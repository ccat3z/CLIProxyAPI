package limiter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

// setupTestStore creates a temporary UsageStore for testing and returns a cleanup function.
func setupTestStore(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	err := usage.InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore: %v", err)
	}
	return func() {
		usage.ClosePersistStore()
		DefaultLimiter().ClearAll()
	}
}

// makeRecord creates a coreusage.Record for injecting into the UsageStore.
func makeRecord(authID, model string, ts time.Time, inputTokens, outputTokens, cachedTokens int64) coreusage.Record {
	return coreusage.Record{
		AuthID:      authID,
		Model:       model,
		RequestedAt: ts,
		Detail: coreusage.Detail{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			CachedTokens: cachedTokens,
		},
	}
}

func TestModelLimiter_Check_NoLimits(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := NewModelLimiter()
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected nil with no limits, got %+v", result)
	}
}

func TestModelLimiter_Check_NoStore(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"model-a"}},
	})
	// No UsageStore — Check returns nil
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected nil without store, got %+v", result)
	}
}

func TestModelLimiter_Check_WithinLimits(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 500, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 500, 200, 0))

	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected nil when within limits, got %+v", result)
	}
}

func TestModelLimiter_Check_InputExceeded(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 500, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 1000, 100, 0))

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected limit exceeded, got nil")
	}
	if result.LimitType != "input_tokens" {
		t.Fatalf("expected input_tokens, got %s", result.LimitType)
	}
	if result.Current != 1000 {
		t.Fatalf("expected current 1000, got %d", result.Current)
	}
	if result.Limit != 1000 {
		t.Fatalf("expected limit 1000, got %d", result.Limit)
	}
}

func TestModelLimiter_Check_OutputExceeded(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, OutputTokens: 500, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 100, 500, 0))

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected limit exceeded, got nil")
	}
	if result.LimitType != "output_tokens" {
		t.Fatalf("expected output_tokens, got %s", result.LimitType)
	}
}

func TestModelLimiter_Check_MultipleWindows(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 500, OutputTokens: 200, Models: []string{"model-a"}},
		{Window: 24 * time.Hour, InputTokens: 2000, OutputTokens: 1000, Models: []string{"model-a"}},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now.Add(-30*time.Minute), 300, 100, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now.Add(-20*time.Minute), 300, 100, 0))

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected short window exceeded, got nil")
	}
	if result.Window != 1*time.Hour {
		t.Fatalf("expected 1h window, got %v", result.Window)
	}
}

func TestModelLimiter_Check_SlidingWindow(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0, Models: []string{"model-a"}},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now.Add(-2*time.Hour), 800, 0, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now.Add(-30*time.Minute), 500, 0, 0))

	result := l.Check("auth1", "model-a")
	if result != nil {
		t.Fatalf("expected nil (only 500 in window), got %+v", result)
	}
}

func TestModelLimiter_RemoveAllForAuth(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"model-a"}},
	})
	l.UpdateLimits("auth2", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 3000, Models: []string{"model-a"}},
	})

	l.RemoveAllForAuth("auth1")

	if l.HasLimits("auth1") {
		t.Fatal("expected auth1 limits removed")
	}
	if !l.HasLimits("auth2") {
		t.Fatal("expected auth2 limits preserved")
	}
}

func TestModelLimiter_DifferentAuths_Independent(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"model-a"}},
	})
	l.UpdateLimits("auth2", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 1000, 0, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth2", "model-a", time.Now(), 1000, 0, 0))

	if result := l.Check("auth1", "model-a"); result == nil {
		t.Fatal("expected auth1 exceeded")
	}
	if result := l.Check("auth2", "model-a"); result != nil {
		t.Fatal("expected auth2 within limits")
	}
}

func TestModelLimiter_CaseInsensitiveModel(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"Kimi-K2"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "kimi-k2", time.Now(), 500, 0, 0))

	result := l.Check("auth1", "KIMI-K2")
	if result != nil {
		t.Fatalf("expected within limits (case insensitive), got %+v", result)
	}
}

func TestModelLimiter_Check_CacheTokensExceeded(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, CacheTokens: 500, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 600, 0, 500))

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected cache_tokens limit exceeded, got nil")
	}
	if result.LimitType != "cache_tokens" {
		t.Fatalf("expected cache_tokens, got %s", result.LimitType)
	}
	if result.Current != 500 {
		t.Fatalf("expected current 500, got %d", result.Current)
	}
	if result.Limit != 500 {
		t.Fatalf("expected limit 500, got %d", result.Limit)
	}
}

func TestModelLimiter_Check_PriceExceeded(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	usage.UsageStore.SetModelPrices("auth1", "model-a", usage.ModelPrices{InputPriceM: 3.0, OutputPriceM: 15.0})
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, Price: 0.01, Models: []string{"model-a"}},
	})

	// 10k input tokens at $3/M = $0.03, exceeds $0.01
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 10000, 0, 0))

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected price limit exceeded, got nil")
	}
	if result.LimitType != "price" {
		t.Fatalf("expected price, got %s", result.LimitType)
	}
}

func TestModelLimiter_Check_PriceCostCalculation(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	usage.UsageStore.SetModelPrices("auth1", "model-a", usage.ModelPrices{InputPriceM: 3.0, CachePriceM: 0.3, OutputPriceM: 15.0})
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, Price: 1.0, Models: []string{"model-a"}},
	})

	// 1M input (500k non-cached + 500k cached) at $3/M + $0.3/M, 100k output at $15/M
	// = $1.5 + $0.15 + $1.5 = $3.15, exceeds $1.0
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 1000000, 100000, 500000))

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected price limit exceeded, got nil")
	}
	if result.LimitType != "price" {
		t.Fatalf("expected price, got %s", result.LimitType)
	}
}

func TestModelLimiter_Check_PriceWithinLimits(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	usage.UsageStore.SetModelPrices("auth1", "model-a", usage.ModelPrices{InputPriceM: 3.0, OutputPriceM: 15.0})
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, Price: 10.0, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 100000, 10000, 0))

	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected within limits, got %+v", result)
	}
}

func TestModelLimiter_Check_CachedExceedsInput(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	usage.UsageStore.SetModelPrices("auth1", "model-a", usage.ModelPrices{InputPriceM: 3.0, CachePriceM: 0.3, OutputPriceM: 15.0})
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, Price: 1.0, Models: []string{"model-a"}},
	})

	// Cached tokens exceed input tokens — nonCachedInput should clamp to 0
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 100000, 10000, 200000))

	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected within limits (cached clamped), got %+v", result)
	}
}

// --- New tests for shared-window and wildcard behavior ---

func TestModelLimiter_Check_SharedWindow(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	// models: [a, b] share a window with 1000 input_tokens limit
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"model-a", "model-b"}},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now, 600, 0, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-b", now, 500, 0, 0))

	// Combined 600+500=1100 >= 1000, should trigger for either model
	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected shared window limit exceeded for model-a")
	}
	if result.Current != 1100 {
		t.Fatalf("expected current 1100 (a+b), got %d", result.Current)
	}

	result = l.Check("auth1", "model-b")
	if result == nil {
		t.Fatal("expected shared window limit exceeded for model-b")
	}
}

func TestModelLimiter_Check_SharedWindow_WithinLimits(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 2000, Models: []string{"model-a", "model-b"}},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now, 600, 0, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-b", now, 500, 0, 0))

	// Combined 1100 < 2000, should not trigger
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected within shared limit, got %+v", result)
	}
}

func TestModelLimiter_Check_Wildcard(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	// Empty Models = wildcard: matches any model, aggregates all
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: nil},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now, 600, 0, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-b", now, 500, 0, 0))

	// Wildcard matches any model; combined 1100 >= 1000
	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected wildcard limit exceeded")
	}
	if result.Current != 1100 {
		t.Fatalf("expected current 1100 (all models), got %d", result.Current)
	}

	result = l.Check("auth1", "model-c")
	if result == nil {
		t.Fatal("expected wildcard limit exceeded for model-c too")
	}
}

func TestModelLimiter_Check_Wildcard_WithinLimits(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 2000, Models: nil},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now, 500, 0, 0))

	if result := l.Check("auth1", "model-b"); result != nil {
		t.Fatalf("expected within wildcard limit, got %+v", result)
	}
}

func TestModelLimiter_Check_ModelNotInConfig(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"model-a"}},
	})

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-b", time.Now(), 5000, 0, 0))

	// model-b is not in the config's models list, should not match
	if result := l.Check("auth1", "model-b"); result != nil {
		t.Fatalf("expected nil (model not in config), got %+v", result)
	}
}

func TestModelLimiter_Check_MultipleConfigsForAuth(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 500, Models: []string{"model-a"}},
		{Window: 1 * time.Hour, InputTokens: 2000, Models: nil}, // wildcard
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now, 600, 0, 0))

	// model-a matches both configs; the specific one triggers first (600 >= 500)
	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected specific config limit exceeded")
	}
	if result.Limit != 500 {
		t.Fatalf("expected limit from specific config (500), got %d", result.Limit)
	}

	// model-b only matches wildcard; 600 < 2000, within limits
	if result := l.Check("auth1", "model-b"); result != nil {
		t.Fatalf("expected within wildcard limit for model-b, got %+v", result)
	}
}

func TestModelLimiter_UpdateLimits_ReplacesAll(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, Models: []string{"model-a"}},
	})
	l.UpdateLimits("auth1", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, Models: []string{"model-b"}},
	})

	// Only model-b config should exist
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 2000, 0, 0))
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected model-a no longer limited after UpdateLimits replace, got %+v", result)
	}

	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-b", time.Now(), 6000, 0, 0))
	if result := l.Check("auth1", "model-b"); result == nil {
		t.Fatal("expected model-b limit exceeded")
	}
}
