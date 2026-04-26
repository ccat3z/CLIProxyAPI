package limiter

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

// LimitConfig defines a sliding-window token limit.
type LimitConfig struct {
	Window       time.Duration
	InputTokens  int64
	OutputTokens int64
	CacheTokens  int64
	Price        float64
}

// LimitCheckResult describes the outcome of a limit check.
type LimitCheckResult struct {
	Window    time.Duration
	LimitType string // "input_tokens", "output_tokens", "cache_tokens", or "price"
	Current   int64
	Limit     int64
}

// ModelLimiter tracks token usage per authID+model and enforces sliding window limits.
// Usage data is queried from usage.UsageStore (SQLite). When no store is configured,
// Check returns nil (no limiting).
type ModelLimiter struct {
	mu     sync.RWMutex
	limits map[string][]LimitConfig // key = "authID|model" -> sorted by Window asc
}

// NewModelLimiter creates a new ModelLimiter.
func NewModelLimiter() *ModelLimiter {
	return &ModelLimiter{
		limits: make(map[string][]LimitConfig),
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

// RemoveLimits removes limit configuration for an authID+model pair.
func (l *ModelLimiter) RemoveLimits(authID, model string) {
	if l == nil {
		return
	}
	key := limitKey(authID, model)
	l.mu.Lock()
	delete(l.limits, key)
	l.mu.Unlock()
}

// RemoveAllForAuth removes all limit configurations for an authID.
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
	l.mu.Unlock()
}

// SyncLimitsForAuth reconciles the limiter state for an authID with the
// currently active set of models. Any model that has limits configured but is
// not in activeModels will have its limits removed. Call this before
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
			}
		}
	}
	l.mu.Unlock()
}

// ClearAll removes all limits. Used for testing.
func (l *ModelLimiter) ClearAll() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.limits = make(map[string][]LimitConfig)
	l.mu.Unlock()
}

// LimitEntry describes a configured limit with its associated auth and model.
type LimitEntry struct {
	AuthID string
	Model  string
	Limits []LimitConfig
}

// GetAllLimits returns a snapshot of all configured limits grouped by authID+model.
func (l *ModelLimiter) GetAllLimits() []LimitEntry {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	entries := make([]LimitEntry, 0, len(l.limits))
	for key, windows := range l.limits {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		entries = append(entries, LimitEntry{
			AuthID: parts[0],
			Model:  parts[1],
			Limits: windows,
		})
	}
	return entries
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

// Check performs a pre-emptive limit check by querying usage.UsageStore.
// Returns nil if no limits are configured, no store is available, or usage is
// within limits. Returns a LimitCheckResult if any window limit is breached.
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
	l.mu.RUnlock()

	if usage.UsageStore == nil {
		return nil
	}

	now := time.Now()
	for _, w := range windows {
		cutoff := now.Add(-w.Window)

		summary, err := usage.UsageStore.QueryUsage(authID, model, cutoff, now)
		if err != nil {
			return nil
		}

		if w.InputTokens > 0 && summary.InputTokens >= w.InputTokens {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "input_tokens",
				Current:   summary.InputTokens,
				Limit:     w.InputTokens,
			}
		}
		if w.OutputTokens > 0 && summary.OutputTokens >= w.OutputTokens {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "output_tokens",
				Current:   summary.OutputTokens,
				Limit:     w.OutputTokens,
			}
		}
		if w.CacheTokens > 0 && summary.CachedTokens >= w.CacheTokens {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "cache_tokens",
				Current:   summary.CachedTokens,
				Limit:     w.CacheTokens,
			}
		}
		if w.Price > 0 && summary.Cost >= w.Price {
			return &LimitCheckResult{
				Window:    w.Window,
				LimitType: "price",
				Current:   int64(summary.Cost * 1_000_000),
				Limit:     int64(w.Price * 1_000_000),
			}
		}
	}
	return nil
}

// CheckRateLimit checks limits for the given authID+model and returns a
// RateLimitError if any window is exceeded. Returns nil if no limits are
// configured or if usage is within limits.
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
