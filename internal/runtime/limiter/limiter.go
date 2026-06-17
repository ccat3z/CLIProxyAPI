package limiter

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
)

// LimitConfig defines a sliding-window token limit for a set of models.
// Models lists the model names this config applies to; empty/nil means wildcard (all models).
// When Models has multiple entries, their combined usage is checked against the limits.
type LimitConfig struct {
	Window       time.Duration
	InputTokens  int64
	OutputTokens int64
	CacheTokens  int64
	Price        float64
	Models       []string // empty = wildcard (all models for authID)
}

// LimitCheckResult describes the outcome of a limit check.
type LimitCheckResult struct {
	Window    time.Duration
	LimitType string // "input_tokens", "output_tokens", "cache_tokens", or "price"
	Current   int64
	Limit     int64
}

// ModelLimiter tracks token usage per authID and enforces sliding window limits.
// Usage data is queried from usage.UsageStore (SQLite). When no store is configured,
// Check returns nil (no limiting).
type ModelLimiter struct {
	mu     sync.RWMutex
	limits map[string][]LimitConfig // key = authID
}

// NewModelLimiter creates a new ModelLimiter.
func NewModelLimiter() *ModelLimiter {
	return &ModelLimiter{
		limits: make(map[string][]LimitConfig),
	}
}

// UpdateLimits replaces all limit configs for an authID.
func (l *ModelLimiter) UpdateLimits(authID string, configs []LimitConfig) {
	if l == nil || authID == "" {
		return
	}
	l.mu.Lock()
	if len(configs) == 0 {
		delete(l.limits, authID)
	} else {
		l.limits[authID] = configs
	}
	l.mu.Unlock()
}

// RemoveAllForAuth removes all limit configurations for an authID.
func (l *ModelLimiter) RemoveAllForAuth(authID string) {
	if l == nil || authID == "" {
		return
	}
	l.mu.Lock()
	delete(l.limits, authID)
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

// LimitEntry describes a configured limit with its associated auth and models.
type LimitEntry struct {
	AuthID string
	Models []string
	Limits []LimitConfig
}

// GetAllLimits returns a snapshot of all configured limits grouped by authID.
func (l *ModelLimiter) GetAllLimits() []LimitEntry {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	entries := make([]LimitEntry, 0, len(l.limits))
	for authID, configs := range l.limits {
		entries = append(entries, LimitEntry{
			AuthID: authID,
			Limits: configs,
		})
	}
	return entries
}

// HasLimits returns true if any limits are configured for the given authID.
func (l *ModelLimiter) HasLimits(authID string) bool {
	if l == nil {
		return false
	}
	l.mu.RLock()
	_, ok := l.limits[authID]
	l.mu.RUnlock()
	return ok
}

// modelMatches returns true if the given model is covered by the config's Models list.
// Empty Models means wildcard (matches everything).
func modelMatches(configModels []string, model string) bool {
	if len(configModels) == 0 {
		return true
	}
	lower := strings.ToLower(model)
	for _, m := range configModels {
		if strings.ToLower(m) == lower {
			return true
		}
	}
	return false
}

// Check performs a pre-emptive limit check by querying usage.UsageStore.
// It iterates all configs for the authID; for each config where the requested model
// matches (either explicitly listed or via wildcard), it queries the aggregate usage
// across the config's models and checks against limits.
// Returns nil if no limits apply, no store is available, or usage is within limits.
// Returns a LimitCheckResult if any window limit is breached.
func (l *ModelLimiter) Check(authID, model string) *LimitCheckResult {
	if l == nil {
		return nil
	}

	l.mu.RLock()
	configs, hasLimits := l.limits[authID]
	if !hasLimits {
		l.mu.RUnlock()
		return nil
	}
	configsCopy := make([]LimitConfig, len(configs))
	copy(configsCopy, configs)
	l.mu.RUnlock()

	if usage.UsageStore == nil {
		return nil
	}

	now := time.Now()
	for _, w := range configsCopy {
		if !modelMatches(w.Models, model) {
			continue
		}

		cutoff := now.Add(-w.Window)
		summary, err := usage.UsageStore.QueryUsageMulti(authID, w.Models, cutoff, now)
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
