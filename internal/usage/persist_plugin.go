package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
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

// UsageDetailRow is a single row from the full usage report query.
type UsageDetailRow struct {
	AuthID              string
	Provider            string
	Model               string
	APIKey              string
	AuthIndex           string
	Source              string
	Timestamp           time.Time
	LatencyNs           int64
	Failed              bool
	InputTokens         int64
	OutputTokens        int64
	ReasoningTokens     int64
	CachedTokens        int64
	TotalTokens         int64
	Cost                float64
	RequestID           string
	RequestServiceTier  string
	ResponseServiceTier string
}

// APIReport holds per-API usage data.
type APIReport struct {
	TotalRequests int64
	TotalTokens   int64
	Models        map[string]ModelReport
}

// ModelReport holds per-model usage data.
type ModelReport struct {
	TotalRequests int64
	TotalTokens   int64
	Details       []UsageDetailRow
}

// UsageReport holds the complete usage report for a time range.
type UsageReport struct {
	TotalRequests  int64
	SuccessCount   int64
	FailureCount   int64
	TotalTokens    int64
	APIs           map[string]APIReport
	RequestsByDay  map[string]int64
	RequestsByHour map[string]int64
	TokensByDay    map[string]int64
	TokensByHour   map[string]int64
	CostByDay      map[string]float64
	CostByHour     map[string]float64
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
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    auth_id          TEXT NOT NULL,
    model            TEXT NOT NULL,
    timestamp        INTEGER NOT NULL,
    input_tokens     INTEGER NOT NULL DEFAULT 0,
    output_tokens    INTEGER NOT NULL DEFAULT 0,
    cached_tokens    INTEGER NOT NULL DEFAULT 0,
    cost             REAL NOT NULL DEFAULT 0,
    provider         TEXT NOT NULL DEFAULT '',
    api_key          TEXT NOT NULL DEFAULT '',
    auth_index       TEXT NOT NULL DEFAULT '',
    source           TEXT NOT NULL DEFAULT '',
    latency_ns       INTEGER NOT NULL DEFAULT 0,
    failed           INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens     INTEGER NOT NULL DEFAULT 0,
    request_id       TEXT NOT NULL DEFAULT '',
    request_service_tier  TEXT NOT NULL DEFAULT '',
    response_service_tier TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_usage_auth_model_ts ON usage(auth_id, model, timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage(timestamp);
`

// newColumns lists columns that must be added via migration for existing databases.
var newColumns = []struct {
	name string
	def  string
}{
	{"provider", "TEXT NOT NULL DEFAULT ''"},
	{"api_key", "TEXT NOT NULL DEFAULT ''"},
	{"auth_index", "TEXT NOT NULL DEFAULT ''"},
	{"source", "TEXT NOT NULL DEFAULT ''"},
	{"latency_ns", "INTEGER NOT NULL DEFAULT 0"},
	{"failed", "INTEGER NOT NULL DEFAULT 0"},
	{"reasoning_tokens", "INTEGER NOT NULL DEFAULT 0"},
	{"total_tokens", "INTEGER NOT NULL DEFAULT 0"},
	{"request_id", "TEXT NOT NULL DEFAULT ''"},
	{"request_service_tier", "TEXT NOT NULL DEFAULT ''"},
	{"response_service_tier", "TEXT NOT NULL DEFAULT ''"},
}

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

// ClearStaleModelPrices removes prices for models under authID that are not in currentModels.
func (p *PersistPlugin) ClearStaleModelPrices(authID string, currentModels map[string]struct{}) {
	if p == nil {
		return
	}
	prefix := authID + "|"
	p.mu.Lock()
	for key := range p.prices {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		model := key[len(prefix):]
		if _, ok := currentModels[model]; !ok {
			delete(p.prices, key)
		}
	}
	p.mu.Unlock()
}

// HandleUsage implements coreusage.Plugin.
func (p *PersistPlugin) HandleUsage(_ context.Context, record coreusage.Record) {
	if p == nil || p.db == nil {
		return
	}

	authID := record.AuthID
	model := strings.ToLower(strings.TrimSpace(record.Model))
	if authID == "" || model == "" {
		return
	}

	var cost float64
	if !record.Failed {
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
	}

	ts := record.RequestedAt
	if ts.IsZero() {
		ts = time.Now()
	}

	failed := 0
	if record.Failed {
		failed = 1
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	_, err := p.insertStmt.Exec(
		authID, model, ts.UnixNano(),
		record.Detail.InputTokens, record.Detail.OutputTokens,
		record.Detail.CachedTokens, cost,
		record.Provider, record.APIKey, record.AuthIndex,
		record.Source, record.Latency.Nanoseconds(), failed,
		record.Detail.ReasoningTokens, record.Detail.TotalTokens,
		record.RequestID,
		record.RequestServiceTier, record.ResponseServiceTier,
	)
	if err != nil {
		log.Debugf("persist: failed to insert usage: %v", err)
	}
}

// QueryUsageMulti aggregates usage across multiple models for a given authID.
// If models is empty/nil, it sums usage across all models for the authID (wildcard).
// Otherwise, it sums usage for the specified models only.
func (p *PersistPlugin) QueryUsageMulti(authID string, models []string, from, to time.Time) (UsageSummary, error) {
	var s UsageSummary
	if p == nil || p.db == nil {
		return s, fmt.Errorf("persist: store not initialized")
	}

	var query string
	var args []any

	if len(models) == 0 {
		query = `SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(cost),0), COUNT(*)
			 FROM usage WHERE auth_id = ? AND timestamp >= ? AND timestamp < ?`
		args = []any{authID, from.UnixNano(), to.UnixNano()}
	} else {
		placeholders := make([]string, len(models))
		args = make([]any, 0, len(models)+3)
		args = append(args, authID)
		for i, m := range models {
			placeholders[i] = "?"
			args = append(args, strings.ToLower(strings.TrimSpace(m)))
		}
		args = append(args, from.UnixNano(), to.UnixNano())
		query = fmt.Sprintf(
			`SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cached_tokens),0), COALESCE(SUM(cost),0), COUNT(*)
			 FROM usage WHERE auth_id = ? AND model IN (%s) AND timestamp >= ? AND timestamp < ?`,
			strings.Join(placeholders, ","))
	}

	row := p.db.QueryRow(query, args...)
	err := row.Scan(&s.InputTokens, &s.OutputTokens, &s.CachedTokens, &s.Cost, &s.EntryCount)
	if err != nil {
		return s, fmt.Errorf("persist: query usage multi: %w", err)
	}
	return s, nil
}

// QueryFullUsageReport returns a complete usage report for the time range [from, to).
func (p *PersistPlugin) QueryFullUsageReport(from, to time.Time) (*UsageReport, error) {
	if p == nil || p.db == nil {
		return nil, fmt.Errorf("persist: store not initialized")
	}

	rows, err := p.db.Query(
		`SELECT auth_id, model, timestamp, input_tokens, output_tokens, cached_tokens, cost,
		        provider, api_key, auth_index, source, latency_ns, failed,
		        reasoning_tokens, total_tokens, request_id,
		        request_service_tier, response_service_tier
		 FROM usage WHERE timestamp >= ? AND timestamp < ?`,
		from.UnixNano(), to.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("persist: query full report: %w", err)
	}
	defer func() {
		if errClose := rows.Close(); errClose != nil {
			log.WithError(errClose).Warn("persist: failed to close query rows")
		}
	}()

	report := &UsageReport{
		APIs:           make(map[string]APIReport),
		RequestsByDay:  make(map[string]int64),
		RequestsByHour: make(map[string]int64),
		TokensByDay:    make(map[string]int64),
		TokensByHour:   make(map[string]int64),
		CostByDay:      make(map[string]float64),
		CostByHour:     make(map[string]float64),
	}

	for rows.Next() {
		var (
			authID, model, provider, apiKey, authIndex, source, requestID string
			requestServiceTier, responseServiceTier                       string
			timestampNs, inputTokens, outputTokens, cachedTokens          int64
			latencyNs, reasoningTokens, totalTokens                       int64
			failed                                                        int64
			costFloat                                                     float64
		)
		if errScan := rows.Scan(
			&authID, &model, &timestampNs, &inputTokens, &outputTokens, &cachedTokens, &costFloat,
			&provider, &apiKey, &authIndex, &source, &latencyNs, &failed,
			&reasoningTokens, &totalTokens, &requestID,
			&requestServiceTier, &responseServiceTier,
		); errScan != nil {
			return nil, fmt.Errorf("persist: scan row: %w", errScan)
		}

		ts := time.Unix(0, timestampNs)
		isFailed := failed != 0

		row := UsageDetailRow{
			AuthID:              authID,
			Provider:            provider,
			Model:               model,
			APIKey:              apiKey,
			AuthIndex:           authIndex,
			Source:              source,
			Timestamp:           ts,
			LatencyNs:           latencyNs,
			Failed:              isFailed,
			InputTokens:         inputTokens,
			OutputTokens:        outputTokens,
			ReasoningTokens:     reasoningTokens,
			CachedTokens:        cachedTokens,
			TotalTokens:         totalTokens,
			Cost:                costFloat,
			RequestID:           requestID,
			RequestServiceTier:  requestServiceTier,
			ResponseServiceTier: responseServiceTier,
		}

		report.TotalRequests++
		if isFailed {
			report.FailureCount++
		} else {
			report.SuccessCount++
		}
		report.TotalTokens += totalTokens

		api, ok := report.APIs[authID]
		if !ok {
			api = APIReport{Models: make(map[string]ModelReport)}
		}
		api.TotalRequests++
		api.TotalTokens += totalTokens

		mr, ok := api.Models[model]
		if !ok {
			mr = ModelReport{}
		}
		mr.TotalRequests++
		mr.TotalTokens += totalTokens
		mr.Details = append(mr.Details, row)

		api.Models[model] = mr
		report.APIs[authID] = api

		dayKey := ts.Format("2006-01-02")
		hourKey := ts.Format("15")
		report.RequestsByDay[dayKey]++
		report.RequestsByHour[hourKey]++
		report.TokensByDay[dayKey] += totalTokens
		report.TokensByHour[hourKey] += totalTokens
		report.CostByDay[dayKey] += costFloat
		report.CostByHour[hourKey] += costFloat
	}

	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("persist: iterate rows: %w", errRows)
	}

	return report, nil
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

// migrateSchema adds missing columns to the usage table for existing databases.
func migrateSchema(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(usage)")
	if err != nil {
		return fmt.Errorf("persist: pragma table_info: %w", err)
	}
	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue interface{}
		var pk int
		if errScan := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); errScan != nil {
			rows.Close()
			return fmt.Errorf("persist: scan pragma: %w", errScan)
		}
		existing[name] = true
	}
	if errClose := rows.Close(); errClose != nil {
		return fmt.Errorf("persist: close pragma rows: %w", errClose)
	}
	if errRows := rows.Err(); errRows != nil {
		return fmt.Errorf("persist: pragma rows: %w", errRows)
	}

	for _, col := range newColumns {
		if !existing[col.name] {
			alterSQL := fmt.Sprintf("ALTER TABLE usage ADD COLUMN %s %s", col.name, col.def)
			if _, errExec := db.Exec(alterSQL); errExec != nil {
				return fmt.Errorf("persist: alter add %s: %w", col.name, errExec)
			}
			log.Infof("persist: migrated schema, added column %s", col.name)
		}
	}

	// Add timestamp index if missing
	if _, errExec := db.Exec("CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage(timestamp)"); errExec != nil {
		return fmt.Errorf("persist: create timestamp index: %w", errExec)
	}

	return nil
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

	if err := migrateSchema(db); err != nil {
		db.Close()
		return fmt.Errorf("persist: schema migration: %w", err)
	}

	insertStmt, err := db.Prepare(
		`INSERT INTO usage (auth_id, model, timestamp, input_tokens, output_tokens, cached_tokens, cost,
		                    provider, api_key, auth_index, source, latency_ns, failed,
		                    reasoning_tokens, total_tokens, request_id,
		                    request_service_tier, response_service_tier)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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
