package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestModelLimiter_Check_NoLimits(t *testing.T) {
	l := NewModelLimiter()
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected nil with no limits, got %+v", result)
	}
}

func TestModelLimiter_Check_WithinLimits(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 500},
	})

	l.Record("auth1", "model-a", time.Now(), 500, 200)

	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatalf("expected nil when within limits, got %+v", result)
	}
}

func TestModelLimiter_Check_InputExceeded(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 500},
	})

	l.Record("auth1", "model-a", time.Now(), 1000, 100)

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
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, OutputTokens: 500},
	})

	l.Record("auth1", "model-a", time.Now(), 100, 500)

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected limit exceeded, got nil")
	}
	if result.LimitType != "output_tokens" {
		t.Fatalf("expected output_tokens, got %s", result.LimitType)
	}
}

func TestModelLimiter_Check_MultipleWindows(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 500, OutputTokens: 200},
		{Window: 24 * time.Hour, InputTokens: 2000, OutputTokens: 1000},
	})

	now := time.Now()
	// Record usage that fits in the 24h window but exceeds the 1h window
	l.Record("auth1", "model-a", now.Add(-30*time.Minute), 300, 100)
	l.Record("auth1", "model-a", now.Add(-20*time.Minute), 300, 100)

	result := l.Check("auth1", "model-a")
	if result == nil {
		t.Fatal("expected short window exceeded, got nil")
	}
	if result.Window != 1*time.Hour {
		t.Fatalf("expected 1h window, got %v", result.Window)
	}
}

func TestModelLimiter_Check_SlidingWindow(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})

	now := time.Now()
	// Old entry outside the window
	l.Record("auth1", "model-a", now.Add(-2*time.Hour), 800, 0)
	// Recent entry inside the window
	l.Record("auth1", "model-a", now.Add(-30*time.Minute), 500, 0)

	result := l.Check("auth1", "model-a")
	if result != nil {
		t.Fatalf("expected nil (only 500 in window), got %+v", result)
	}
}

func TestModelLimiter_Record_SkipsUntracked(t *testing.T) {
	l := NewModelLimiter()
	// No limits configured, record should be skipped
	l.Record("auth1", "model-a", time.Now(), 1000, 500)

	l.mu.RLock()
	_, ok := l.usage[limitKey("auth1", "model-a")]
	l.mu.RUnlock()
	if ok {
		t.Fatal("expected no usage entry for untracked key")
	}
}

func TestModelLimiter_RemoveLimits(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.Record("auth1", "model-a", time.Now(), 500, 0)

	l.RemoveLimits("auth1", "model-a")

	if l.HasLimits("auth1", "model-a") {
		t.Fatal("expected limits removed")
	}
	if result := l.Check("auth1", "model-a"); result != nil {
		t.Fatal("expected nil check after removal")
	}
}

func TestModelLimiter_RemoveAllForAuth(t *testing.T) {
	l := NewModelLimiter()
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
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.UpdateLimits("auth2", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 5000, OutputTokens: 0},
	})

	l.Record("auth1", "model-a", time.Now(), 1000, 0)
	l.Record("auth2", "model-a", time.Now(), 1000, 0)

	if result := l.Check("auth1", "model-a"); result == nil {
		t.Fatal("expected auth1 exceeded")
	}
	if result := l.Check("auth2", "model-a"); result != nil {
		t.Fatal("expected auth2 within limits")
	}
}

func TestModelLimiter_ConcurrentAccess(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 100000, OutputTokens: 100000},
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Record("auth1", "model-a", time.Now(), 100, 50)
			l.Check("auth1", "model-a")
		}()
	}
	wg.Wait()
}

func TestModelLimiter_SyncLimitsForAuth_RemovesStaleModels(t *testing.T) {
	l := NewModelLimiter()
	// Set up limits for two models
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.UpdateLimits("auth1", "model-b", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 2000, OutputTokens: 0},
	})
	l.Record("auth1", "model-a", time.Now(), 500, 0)
	l.Record("auth1", "model-b", time.Now(), 300, 0)

	// Sync with only model-a: model-b should be removed
	l.SyncLimitsForAuth("auth1", []string{"model-a"})

	if !l.HasLimits("auth1", "model-a") {
		t.Fatal("expected model-a limits preserved")
	}
	if l.HasLimits("auth1", "model-b") {
		t.Fatal("expected model-b limits removed after sync")
	}
	// Usage for model-b should also be gone
	l.mu.RLock()
	_, hasUsage := l.usage[limitKey("auth1", "model-b")]
	l.mu.RUnlock()
	if hasUsage {
		t.Fatal("expected model-b usage removed after sync")
	}
}

func TestModelLimiter_SyncLimitsForAuth_EmptyModelsRemovesAll(t *testing.T) {
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "model-a", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})
	l.Record("auth1", "model-a", time.Now(), 500, 0)

	// Sync with no models: all limits should be removed
	l.SyncLimitsForAuth("auth1", nil)

	if l.HasLimits("auth1", "model-a") {
		t.Fatal("expected all limits removed after sync with empty models")
	}
}

func TestModelLimiter_SyncLimitsForAuth_PreservesOtherAuths(t *testing.T) {
	l := NewModelLimiter()
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
	l := NewModelLimiter()
	l.UpdateLimits("auth1", "Kimi-K2", []LimitConfig{
		{Window: 1 * time.Hour, InputTokens: 1000, OutputTokens: 0},
	})

	l.Record("auth1", "kimi-k2", time.Now(), 500, 0)

	result := l.Check("auth1", "KIMI-K2")
	if result != nil {
		t.Fatalf("expected within limits (case insensitive), got %+v", result)
	}
}
