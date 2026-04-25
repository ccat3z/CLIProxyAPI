package limiter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 500},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 500},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, OutputTokens: 500},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 500, OutputTokens: 200},
		{Window: 24 * time.Hour, InputTokens: 2000, OutputTokens: 1000},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})

	now := time.Now()
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now.Add(-2*time.Hour), 800, 0, 0))
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", now.Add(-30*time.Minute), 500, 0, 0))

	result := l.Check("auth1", "model-a")
	if result != nil {
		t.Fatalf("expected nil (only 500 in window), got %+v", result)
	}
}

func TestModelLimiter_RemoveLimits(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})

	l.RemoveLimits("auth1", "model-a")

	if l.HasLimits("auth1", "model-a") {
		t.Fatal("expected limits removed")
	}
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatal("expected nil check after removal")
	}
}

func TestModelLimiter_RemoveAllForAuth(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.UpdateLimits("auth1", "model-b", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 2000, OutputTokens: 0},
	})
	l.UpdateLimits("auth2", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 3000, OutputTokens: 0},
	})

	l.RemoveAllForAuth("auth1")

	if l.HasLimits("auth1", "model-a") {
		t.Fatal("expected auth1|model-a limits removed")
	}
	if l.HasLimits("auth1", "model-b") {
		t.Fatal("expected auth1|model-b limits removed")
	}
	if !l.HasLimits("auth2", "model-a") {
		t.Fatal("expected auth2|model-a limits preserved")
	}
}

func TestModelLimiter_DifferentAuths_Independent(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.UpdateLimits("auth2", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, OutputTokens: 0},
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

func TestModelLimiter_SyncLimitsForAuth_RemovesStaleModels(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.UpdateLimits("auth1", "model-b", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 2000, OutputTokens: 0},
	})

	l.SyncLimitsForAuth("auth1", []string{"model-a"})

	if !l.HasLimits("auth1", "model-a") {
		t.Fatal("expected model-a limits preserved")
	}
	if l.HasLimits("auth1", "model-b") {
		t.Fatal("expected model-b limits removed after sync")
	}
}

func TestModelLimiter_SyncLimitsForAuth_EmptyModelsRemovesAll(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})

	l.SyncLimitsForAuth("auth1", nil)

	if l.HasLimits("auth1", "model-a") {
		t.Fatal("expected all limits removed after sync with empty models")
	}
}

func TestModelLimiter_SyncLimitsForAuth_PreservesOtherAuths(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.UpdateLimits("auth2", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 2000, OutputTokens: 0},
	})

	l.SyncLimitsForAuth("auth1", nil)

	if l.HasLimits("auth1", "model-a") {
		t.Fatal("expected auth1 limits removed")
	}
	if !l.HasLimits("auth2", "model-a") {
		t.Fatal("expected auth2 limits preserved")
	}
}

func TestModelLimiter_CaseInsensitiveModel(t *testing.T) {
	cleanup := setupTestStore(t)
	defer cleanup()

	l := DefaultLimiter()
	l.UpdateLimits("auth1", "Kimi-K2", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, CacheTokens: 500, OutputTokens: 0},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, Price: 0.01},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, Price: 1.0},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, Price: 10.0},
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
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, Price: 1.0},
	})

	// Cached tokens exceed input tokens — nonCachedInput should clamp to 0
	usage.UsageStore.HandleUsage(nil, makeRecord("auth1", "model-a", time.Now(), 100000, 10000, 200000))

	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected within limits (cached clamped), got %+v", result)
	}
}
