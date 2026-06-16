package management

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/redisqueue"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor/helps"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/limiter"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// GetUsageStatistics returns usage statistics from the SQLite store.
// Accepts an optional ?window=N query parameter (unit: hours, default 24).
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	windowHours := 24
	if w := c.Query("window"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			windowHours = n
		}
	}

	limits := buildLimitsResponse(h)

	if usage.UsageStore != nil {
		from := time.Now().Add(-time.Duration(windowHours) * time.Hour)
		to := time.Now()
		report, err := usage.UsageStore.QueryFullUsageReport(from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to query usage: %v", err)})
			return
		}
		resp := buildPersistResponse(report)
		resp["limits"] = limits
		c.JSON(http.StatusOK, resp)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"usage": gin.H{
			"total_requests":   0,
			"success_count":    0,
			"failure_count":    0,
			"total_tokens":     0,
			"apis":             map[string]any{},
			"requests_by_day":  map[string]any{},
			"requests_by_hour": map[string]any{},
			"tokens_by_day":    map[string]any{},
			"tokens_by_hour":   map[string]any{},
			"cost_by_day":      map[string]any{},
			"cost_by_hour":     map[string]any{},
		},
		"failed_requests": 0,
		"limits":          limits,
	})
}

type usageQueueRecord []byte

func (r usageQueueRecord) MarshalJSON() ([]byte, error) {
	if json.Valid(r) {
		return append([]byte(nil), r...), nil
	}
	return json.Marshal(string(r))
}

// GetUsageQueue pops queued usage records from the usage queue.
func (h *Handler) GetUsageQueue(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}

	count, errCount := parseUsageQueueCount(c.Query("count"))
	if errCount != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errCount.Error()})
		return
	}

	items := redisqueue.PopOldest(count)
	records := make([]usageQueueRecord, 0, len(items))
	for _, item := range items {
		records = append(records, usageQueueRecord(append([]byte(nil), item...)))
	}

	c.JSON(http.StatusOK, records)
}

func parseUsageQueueCount(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1, nil
	}
	count, errCount := strconv.Atoi(value)
	if errCount != nil || count <= 0 {
		return 0, errors.New("count must be a positive integer")
	}
	return count, nil
}

func buildPersistResponse(report *usage.UsageReport) gin.H {
	type tokenStats struct {
		InputTokens     int64 `json:"input_tokens"`
		OutputTokens    int64 `json:"output_tokens"`
		ReasoningTokens int64 `json:"reasoning_tokens"`
		CachedTokens    int64 `json:"cached_tokens"`
		TotalTokens     int64 `json:"total_tokens"`
	}
	type detailEntry struct {
		Timestamp string     `json:"timestamp"`
		LatencyMs int64      `json:"latency_ms"`
		Source    string     `json:"source"`
		AuthIndex string     `json:"auth_index"`
		Tokens    tokenStats `json:"tokens"`
		Failed    bool       `json:"failed"`
		RequestID string     `json:"request_id"`
		Cost      float64    `json:"cost"`
	}
	type modelEntry struct {
		TotalRequests int64         `json:"total_requests"`
		TotalTokens   int64         `json:"total_tokens"`
		Details       []detailEntry `json:"details"`
	}
	type apiEntry struct {
		TotalRequests int64                 `json:"total_requests"`
		TotalTokens   int64                 `json:"total_tokens"`
		Models        map[string]modelEntry `json:"models"`
	}

	apis := make(map[string]apiEntry, len(report.APIs))
	for apiKey, apiReport := range report.APIs {
		models := make(map[string]modelEntry, len(apiReport.Models))
		for modelName, mr := range apiReport.Models {
			details := make([]detailEntry, len(mr.Details))
			for i, d := range mr.Details {
				details[i] = detailEntry{
					Timestamp: d.Timestamp.Format(time.RFC3339Nano),
					LatencyMs: d.LatencyNs / 1_000_000,
					Source:    d.Source,
					AuthIndex: d.AuthIndex,
					Tokens: tokenStats{
						InputTokens:     d.InputTokens,
						OutputTokens:    d.OutputTokens,
						ReasoningTokens: d.ReasoningTokens,
						CachedTokens:    d.CachedTokens,
						TotalTokens:     d.TotalTokens,
					},
					Failed:    d.Failed,
					RequestID: d.RequestID,
					Cost:      d.Cost,
				}
			}
			models[modelName] = modelEntry{
				TotalRequests: mr.TotalRequests,
				TotalTokens:   mr.TotalTokens,
				Details:       details,
			}
		}
		apis[apiKey] = apiEntry{
			TotalRequests: apiReport.TotalRequests,
			TotalTokens:   apiReport.TotalTokens,
			Models:        models,
		}
	}

	return gin.H{
		"usage": gin.H{
			"total_requests":   report.TotalRequests,
			"success_count":    report.SuccessCount,
			"failure_count":    report.FailureCount,
			"total_tokens":     report.TotalTokens,
			"apis":             apis,
			"requests_by_day":  report.RequestsByDay,
			"requests_by_hour": report.RequestsByHour,
			"tokens_by_day":    report.TokensByDay,
			"tokens_by_hour":   report.TokensByHour,
			"cost_by_day":      report.CostByDay,
			"cost_by_hour":     report.CostByHour,
		},
		"failed_requests": report.FailureCount,
	}
}

type limitConfigJSON struct {
	Window       int64    `json:"window"`
	Models       []string `json:"models"`
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	CacheTokens  int64    `json:"cache_tokens"`
	Price        float64  `json:"price"`
}

type limitCurrentJSON struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheTokens  int64   `json:"cache_tokens"`
	Price        float64 `json:"price"`
}

type limitEntryJSON struct {
	Source    string           `json:"source"`
	AuthIndex string           `json:"auth_index"`
	Config    limitConfigJSON  `json:"config"`
	Current   limitCurrentJSON `json:"current"`
}

func buildLimitsResponse(h *Handler) []limitEntryJSON {
	allLimits := limiter.DefaultLimiter().GetAllLimits()
	if len(allLimits) == 0 {
		return []limitEntryJSON{}
	}

	// Build auth lookup map for source/auth_index
	var authList []*coreauth.Auth
	if h != nil && h.authManager != nil {
		authList = h.authManager.List()
	}
	authByID := make(map[string]*coreauth.Auth, len(authList))
	for _, a := range authList {
		authByID[a.ID] = a
	}

	now := time.Now()
	result := make([]limitEntryJSON, 0, len(allLimits))

	for _, entry := range allLimits {
		auth := authByID[entry.AuthID]
		var source, authIndex string
		if auth != nil {
			source = helps.ResolveUsageSource(auth, "")
			authIndex = auth.Index
		}

		for _, lc := range entry.Limits {
			cfg := limitConfigJSON{
				Window:       int64(lc.Window.Seconds()),
				Models:       lc.Models,
				InputTokens:  lc.InputTokens,
				OutputTokens: lc.OutputTokens,
				CacheTokens:  lc.CacheTokens,
				Price:        lc.Price,
			}

			var current limitCurrentJSON
			if usage.UsageStore != nil {
				cutoff := now.Add(-lc.Window)
				summary, err := usage.UsageStore.QueryUsageMulti(entry.AuthID, lc.Models, cutoff, now)
				if err == nil {
					current.InputTokens = summary.InputTokens
					current.OutputTokens = summary.OutputTokens
					current.CacheTokens = summary.CachedTokens
					current.Price = summary.Cost
				}
			}

			result = append(result, limitEntryJSON{
				Source:    source,
				AuthIndex: authIndex,
				Config:    cfg,
				Current:   current,
			})
		}
	}

	return result
}
