package limiter

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// UsageEntry records a single token usage event.
type UsageEntry struct {
	Timestamp    time.Time
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
}

// LimitConfig defines a sliding-window token limit.
type LimitConfig struct {
	Window       time.Duration
	InputTokens  int64
	OutputTokens int64
	CacheTokens  int64
	InputPriceM  float64
	OutputPriceM float64
	CachePriceM  float64
	Price        float64
}

// LimitCheckResult describes the outcome of a limit check.
type LimitCheckResult struct {
	Window    time.Duration
	LimitType string // "input_tokens" or "output_tokens"
	Current   int64
	Limit     int64
}

// ModelLimiter tracks token usage per authID+model and enforces sliding window limits.
type ModelLimiter struct {
	mu     sync.RWMutex
	limits map[string][]LimitConfig // key = "authID|model" -> sorted by Window asc
	usage  map[string][]UsageEntry  // key = "authID|model" -> sorted by Timestamp asc

	stopOnce sync.Once
	cancel   context.CancelFunc
}

// NewModelLimiter creates a new ModelLimiter.
func NewModelLimiter() *ModelLimiter {
	return &ModelLimiter{
		limits: make(map[string][]LimitConfig),
		usage:  make(map[string][]UsageEntry),
	}
}

func limitKey(authID, model string) string {
	return authID + "|" + strings.ToLower(model)
}

// UpdateLimits sets or replaces the limit windows for an authID+model pair.
func (l *ModelLimiter) UpdateLimits(authID, model string, windows []LimitConfig) {
	if l == nil || len(windows) == 0 {
		return
	}
	key := limitKey(authID, model)
	l.mu.Lock()
	l.limits[key] = windows
	l.mu.Unlock()
}

// RemoveLimits removes limit configuration and usage entries for an authID+model pair.
func (l *ModelLimiter) RemoveLimits(authID, model string) {
	if l == nil {
		return
	}
	key := limitKey(authID, model)
	l.mu.Lock()
	delete(l.limits, key)
	delete(l.usage, key)
	l.mu.Unlock()
}

// RemoveAllForAuth removes all limit configurations and usage entries for an authID.
func (l *ModelLimiter) RemoveAllForAuth(authID string) {
	if l == nil || authID == "" {
		return
	}
	prefix := authID + "|"
	l.mu.Lock()
	for k := range l.limits {
		if strings.HasPrefix(k, prefix) {
			delete(l.limits, k)
		}
	}
	for k := range l.usage {
		if strings.HasPrefix(k, prefix) {
			delete(l.usage, k)
		}
	}
	l.mu.Unlock()
}

// SyncLimitsForAuth reconciles the limiter state for an authID with the
// currently active set of models. Any model that has limits configured but is
// not in activeModels will have its limits and usage removed. Call this before
// UpdateLimits to ensure stale entries from removed config are cleaned up.
func (l *ModelLimiter) SyncLimitsForAuth(authID string, activeModels []string) {
	if l == nil || authID == "" {
		return
	}
	prefix := authID + "|"
	activeSet := make(map[string]struct{}, len(activeModels))
	for _, m := range activeModels {
		activeSet[limitKey(authID, m)] = struct{}{}
	}
	l.mu.Lock()
	for k := range l.limits {
		if strings.HasPrefix(k, prefix) {
			if _, ok := activeSet[k]; !ok {
				delete(l.limits, k)
				delete(l.usage, k)
			}
		}
	}
	l.mu.Unlock()
}

// HasLimits returns true if any limits are configured for the given authID+model.
func (l *ModelLimiter) HasLimits(authID, model string) bool {
	if l == nil {
		return false
	}
	key := limitKey(authID, model)
	l.mu.RLock()
	_, ok := l.limits[key]
	l.mu.RUnlock()
	return ok
}

// Check performs a pre-emptive limit check. Returns nil if no limits are
// configured or if no limits are exceeded. Returns a LimitCheckResult if any
// window limit is breached.
func (l *ModelLimiter) Check(authID, model string) *LimitCheckResult {
	if l == nil {
		return nil
	}
	key := limitKey(authID, model)

	l.mu.RLock()
	windows, hasLimits := l.limits[key]
	if !hasLimits {
		l.mu.RUnlock()
		return nil
	}
	entries := l.usage[key]
	// Snapshot entries to avoid holding the lock during computation
	snapshot := make([]UsageEntry, len(entries))
	copy(snapshot, entries)
	l.mu.RUnlock()

	now := time.Now()
	for _, w := range windows {
		cutoff := now.Add(-w.Window)
		var inputSum, outputSum, cachedSum int64
		for i := range snapshot {
			e := &snapshot[i]
			if e.Timestamp.Before(cutoff) {
				continue
			}
			inputSum += e.InputTokens
			outputSum += e.OutputTokens
			cachedSum += e.CachedTokens
		}
		if w.InputTokens > 0 && inputSum >= w.InputTokens {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "input_tokens",
				Current:   inputSum,
				Limit:     w.InputTokens,
			}
		}
		if w.OutputTokens > 0 && outputSum >= w.OutputTokens {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "output_tokens",
				Current:   outputSum,
				Limit:     w.OutputTokens,
			}
		}
		if w.CacheTokens > 0 && cachedSum >= w.CacheTokens {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "cache_tokens",
				Current:   cachedSum,
				Limit:     w.CacheTokens,
			}
		}
		if w.Price > 0 {
			nonCachedInput := inputSum - cachedSum
			if nonCachedInput < 0 {
				nonCachedInput = 0
			}
			cost := float64(nonCachedInput)/1_000_000*w.InputPriceM +
				float64(cachedSum)/1_000_000*w.CachePriceM +
				float64(outputSum)/1_000_000*w.OutputPriceM
			if cost >= w.Price {
				return &LimitCheckResult{
					Window:    w.Window,
					LimitType: "price",
					Current:   int64(cost * 1_000_000),
					Limit:     int64(w.Price * 1_000_000),
				}
			}
		}
	}
	return nil
}

// Record adds a usage entry for the given authID+model and prunes old entries.
func (l *ModelLimiter) Record(authID, model string, timestamp time.Time, inputTokens, outputTokens, cachedTokens int64) {
	if l == nil {
		return
	}
	key := limitKey(authID, model)

	l.mu.Lock()
	_, hasLimits := l.limits[key]
	if !hasLimits {
		l.mu.Unlock()
		return
	}
	entry := UsageEntry{
		Timestamp:    timestamp,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CachedTokens: cachedTokens,
	}
	l.usage[key] = append(l.usage[key], entry)

	// Prune entries older than the longest window
	longest := longestWindow(l.limits[key])
	if longest > 0 {
		cutoff := time.Now().Add(-longest)
		entries := l.usage[key]
		i := 0
		for i < len(entries) && entries[i].Timestamp.Before(cutoff) {
			i++
		}
		if i > 0 {
			l.usage[key] = entries[i:]
		}
	}
	l.mu.Unlock()
}

func longestWindow(windows []LimitConfig) time.Duration {
	var longest time.Duration
	for _, w := range windows {
		if w.Window > longest {
			longest = w.Window
		}
	}
	return longest
}

// StartCleanup launches a background goroutine that periodically prunes
// stale usage entries across all keys.
func (l *ModelLimiter) StartCleanup(ctx context.Context, interval time.Duration) {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		ctx, l.cancel = context.WithCancel(ctx)
		go l.cleanupLoop(ctx, interval)
	})
}

// Stop terminates the background cleanup goroutine.
func (l *ModelLimiter) Stop() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		// No cleanup started
	})
	if l.cancel != nil {
		l.cancel()
	}
}

func (l *ModelLimiter) cleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.pruneAll()
		}
	}
}

func (l *ModelLimiter) pruneAll() {
	now := time.Now()
	l.mu.Lock()
	for key, windows := range l.limits {
		longest := longestWindow(windows)
		if longest == 0 {
			continue
		}
		cutoff := now.Add(-longest)
		entries := l.usage[key]
		i := 0
		for i < len(entries) && entries[i].Timestamp.Before(cutoff) {
			i++
		}
		if i > 0 {
			l.usage[key] = entries[i:]
		}
	}
	// Also remove usage entries for keys that no longer have limits
	for key := range l.usage {
		if _, ok := l.limits[key]; !ok {
			delete(l.usage, key)
		}
	}
	l.mu.Unlock()
	log.Debugf("limiter: cleanup completed, tracking %d keys", len(l.usage))
}

// CheckRateLimit checks limits for the given authID+model and returns a
// RateLimitError if any window is exceeded. Returns nil if no limits are
// configured or if usage is within limits. This is a convenience wrapper
// around DefaultLimiter().Check() for use in executors.
func CheckRateLimit(authID, baseModel string) error {
	result := DefaultLimiter().Check(authID, baseModel)
	if result == nil {
		return nil
	}
	return &RateLimitError{result: result}
}

// RateLimitError is returned when a per-key model usage limit is exceeded.
// It implements StatusCode() so the handlers layer can set the correct HTTP status.
type RateLimitError struct {
	result *LimitCheckResult
}

func (e *RateLimitError) Error() string {
	if e.result == nil {
		return "rate limit exceeded"
	}
	if e.result.LimitType == "price" {
		return fmt.Sprintf("rate limit exceeded: price limit (%.3f/%.3f) for window %v",
			float64(e.result.Current)/1_000_000, float64(e.result.Limit)/1_000_000, e.result.Window)
	}
	return fmt.Sprintf("rate limit exceeded: %s limit (%d/%d) for window %v",
		e.result.LimitType, e.result.Current, e.result.Limit, e.result.Window)
}

func (e *RateLimitError) StatusCode() int { return http.StatusTooManyRequests }
