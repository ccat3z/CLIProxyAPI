package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"

	_ "modernc.org/sqlite"
)

// UsageEntry represents a persisted usage record.
type UsageEntry struct {
	AuthID       string
	Model        string
	Timestamp    time.Time
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	Cost         float64
}

// UsageSummary holds aggregated usage statistics for a time range.
type UsageSummary struct {
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	Cost         float64
	EntryCount   int
}

// ModelPrices holds per-unit pricing for cost-based limiting.
type ModelPrices struct {
	InputPriceM  float64
	OutputPriceM float64
	CachePriceM  float64
}

// UsageStore is the global queryable usage store, set when a DB path is configured.
var UsageStore *PersistPlugin

// PersistPlugin implements coreusage.Plugin and persists usage records to SQLite.
// It is append-only — records are never deleted.
type PersistPlugin struct {
	mu         sync.Mutex
	db         *sql.DB
	prices     map[string]ModelPrices // key = "authID|model" -> per-unit pricing
	insertStmt *sql.Stmt
}

const schema = `
CREATE TABLE IF NOT EXISTS usage (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    auth_id        TEXT NOT NULL,
    model          TEXT NOT NULL,
    timestamp      INTEGER NOT NULL,
    input_tokens   INTEGER NOT NULL DEFAULT 0,
    output_tokens  INTEGER NOT NULL DEFAULT 0,
    cached_tokens  INTEGER NOT NULL DEFAULT 0,
    cost           REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_usage_auth_model_ts ON usage(auth_id, model, timestamp);
`

func priceKey(authID, model string) string {
	return authID + "|" + strings.ToLower(model)
}

// SetModelPrices sets per-unit pricing for cost computation on an authID+model pair.
func (p *PersistPlugin) SetModelPrices(authID, model string, mp ModelPrices) {
	if p == nil {
		return
	}
	key := priceKey(authID, model)
	p.mu.Lock()
	p.prices[key] = mp
	p.mu.Unlock()
}

// HandleUsage implements coreusage.Plugin.
func (p *PersistPlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if p == nil || p.db == nil {
		return
	}
	if record.Failed {
		return
	}

	authID := record.AuthID
	model := strings.ToLower(strings.TrimSpace(record.Model))
	if authID == "" || model == "" {
		return
	}

	var cost float64
	key := priceKey(authID, model)
	p.mu.Lock()
	mp := p.prices[key]
	p.mu.Unlock()
	nonCachedInput := record.Detail.InputTokens - record.Detail.CachedTokens
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}
	cost = float64(nonCachedInput)/1_000_000*mp.InputPriceM +
		float64(record.Detail.CachedTokens)/1_000_000*mp.CachePriceM +
		float64(record.Detail.OutputTokens)/1_000_000*mp.OutputPriceM

	ts := record.RequestedAt
	if ts.IsZero() {
		ts = time.Now()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	_, err := p.insertStmt.Exec(authID, model, ts.UnixNano(),
		record.Detail.InputTokens, record.Detail.OutputTokens, record.Detail.CachedTokens, cost)
	if err != nil {
		log.Debugf("persist: failed to insert usage: %v", err)
	}
}

// QueryUsage returns aggregated usage for an authID+model within a time range [from, to).
func (p *PersistPlugin) QueryUsage(authID, model string, from, to time.Time) (UsageSummary, error) {
	var s UsageSummary
	if p == nil || p.db == nil {
		return s, fmt.Errorf("persist: store not initialized")
	}
	model = strings.ToLower(strings.TrimSpace(model))
	row := p.db.QueryRow(
		`SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(cost),0), COUNT(*)
		 FROM usage WHERE auth_id = ? AND model = ? AND timestamp >= ? AND timestamp < ?`,
		authID, model, from.UnixNano(), to.UnixNano())
	err := row.Scan(&s.InputTokens, &s.OutputTokens, &s.CachedTokens, &s.Cost, &s.EntryCount)
	if err != nil {
		return s, fmt.Errorf("persist: query usage: %w", err)
	}
	return s, nil
}

// Close closes the SQLite database.
func (p *PersistPlugin) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.insertStmt != nil {
		p.insertStmt.Close()
		p.insertStmt = nil
	}
	err := p.db.Close()
	p.db = nil
	return err
}

// InitPersistStore opens the SQLite database, runs the schema, sets UsageStore,
// and registers the plugin on the default usage manager.
func InitPersistStore(dbPath string) error {
	if dbPath == "" {
		return nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("persist: open %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return fmt.Errorf("persist: schema init: %w", err)
	}

	insertStmt, err := db.Prepare(
		`INSERT INTO usage (auth_id, model, timestamp, input_tokens, output_tokens, cached_tokens, cost)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		db.Close()
		return fmt.Errorf("persist: prepare insert: %w", err)
	}

	p := &PersistPlugin{
		db:         db,
		prices:     make(map[string]ModelPrices),
		insertStmt: insertStmt,
	}
	UsageStore = p
	coreusage.RegisterPlugin(p)
	log.Infof("persist: usage store initialized at %s", dbPath)
	return nil
}

// ClosePersistStore closes and nils the global UsageStore.
func ClosePersistStore() error {
	if UsageStore == nil {
		return nil
	}
	err := UsageStore.Close()
	UsageStore = nil
	return err
}
