package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func makeTestRecord(authID, model string, ts time.Time, inputTokens, outputTokens, cachedTokens int64) coreusage.Record {
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

func TestPersistStoreBasic(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore: %v", err)
	}
	defer ClosePersistStore()

	if UsageStore == nil {
		t.Fatal("UsageStore should not be nil after init")
	}
}

func TestPersistStoreCostFrozenAtRecordTime(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore: %v", err)
	}
	defer ClosePersistStore()

	authID := "claude:abc123"
	model := "claude-sonnet-4-20250514"
	now := time.Now()

	UsageStore.SetModelPrices(authID, model, ModelPrices{InputPriceM: 3.0, OutputPriceM: 15.0, CachePriceM: 0.3})
	UsageStore.HandleUsage(nil, makeTestRecord(authID, model, now, 1000, 500, 200))
	// cost = (800*3 + 200*0.3 + 500*15) / 1M = 0.009960

	// Change prices
	UsageStore.SetModelPrices(authID, model, ModelPrices{InputPriceM: 10.0, OutputPriceM: 50.0, CachePriceM: 1.0})

	// Record another entry with new prices
	UsageStore.HandleUsage(nil, makeTestRecord(authID, model, now.Add(time.Minute), 1000, 500, 200))

	// Query again — first entry should still have original cost
	summary2, _ := UsageStore.QueryUsageMulti(authID, []string{model}, now.Add(-time.Hour), now.Add(time.Hour))
	if summary2.EntryCount != 2 {
		t.Fatalf("EntryCount = %d, want 2", summary2.EntryCount)
	}

	// Second entry cost: (800*10 + 200*1 + 500*50) / 1M = (8000+200+25000)/1M = 0.033200
	expectedCost2 := 0.009960 + 0.033200
	if diff := summary2.Cost - expectedCost2; diff < -0.000001 || diff > 0.000001 {
		t.Errorf("Cost after price change = %.6f, want %.6f", summary2.Cost, expectedCost2)
	}
}

func TestPersistStoreReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore: %v", err)
	}

	authID := "claude:abc123"
	model := "claude-sonnet-4-20250514"
	now := time.Now()

	UsageStore.SetModelPrices(authID, model, ModelPrices{InputPriceM: 3.0, OutputPriceM: 15.0, CachePriceM: 0.3})
	UsageStore.HandleUsage(nil, makeTestRecord(authID, model, now, 1000, 500, 200))
	ClosePersistStore()

	// Reopen
	err = InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore reopen: %v", err)
	}
	defer ClosePersistStore()

	summary, err := UsageStore.QueryUsageMulti(authID, []string{model}, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("QueryUsage after reopen: %v", err)
	}
	if summary.EntryCount != 1 {
		t.Errorf("EntryCount after reopen = %d, want 1", summary.EntryCount)
	}
	if summary.InputTokens != 1000 {
		t.Errorf("InputTokens after reopen = %d, want 1000", summary.InputTokens)
	}
}

func TestPersistStoreEmptyQuery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore: %v", err)
	}
	defer ClosePersistStore()

	summary, err := UsageStore.QueryUsageMulti("nonexistent", []string{"model"}, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("QueryUsage empty: %v", err)
	}
	if summary.EntryCount != 0 {
		t.Errorf("EntryCount = %d, want 0", summary.EntryCount)
	}
}

func TestPersistStoreFailedRecord(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := InitPersistStore(dbPath)
	if err != nil {
		t.Fatalf("InitPersistStore: %v", err)
	}
	defer ClosePersistStore()

	authID := "claude:abc123"
	model := "claude-sonnet-4-20250514"
	now := time.Now()

	r := makeTestRecord(authID, model, now, 1000, 500, 200)
	r.Failed = true
	UsageStore.HandleUsage(nil, r)

	summary, _ := UsageStore.QueryUsageMulti(authID, []string{model}, now.Add(-time.Hour), now.Add(time.Hour))
	if summary.EntryCount != 1 {
		t.Errorf("Failed records should be persisted, got EntryCount = %d", summary.EntryCount)
	}
	if summary.Cost != 0 {
		t.Errorf("Failed records should have zero cost, got Cost = %f", summary.Cost)
	}
}

func TestPersistStoreInitEmptyPath(t *testing.T) {
	err := InitPersistStore("")
	if err != nil {
		t.Errorf("InitPersistStore with empty path should return nil, got: %v", err)
	}
}

func TestPersistStoreCloseWithoutInit(t *testing.T) {
	// ClosePersistStore on nil should not panic
	orig := UsageStore
	UsageStore = nil
	ClosePersistStore()
	UsageStore = orig
}

func TestInitPersistStoreCreatesFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "usage.db")

	// Should fail if parent dir doesn't exist — SQLite doesn't auto-create dirs
	err := InitPersistStore(dbPath)
	if err == nil {
		ClosePersistStore()
		t.Log("SQLite created parent dir automatically")
	} else {
		// Create parent dir and retry
		os.MkdirAll(filepath.Dir(dbPath), 0o755)
		err = InitPersistStore(dbPath)
		if err != nil {
			t.Fatalf("InitPersistStore after mkdir: %v", err)
		}
		defer ClosePersistStore()
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("DB file should exist after init")
	}
}
