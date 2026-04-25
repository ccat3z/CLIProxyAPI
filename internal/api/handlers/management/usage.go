package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

type usageExportPayload struct {
	Version    int                      `json:"version"`
	ExportedAt time.Time                `json:"exported_at"`
	Usage      usage.StatisticsSnapshot `json:"usage"`
}

type usageImportPayload struct {
	Version int                      `json:"version"`
	Usage   usage.StatisticsSnapshot `json:"usage"`
}

// GetUsageStatistics returns usage statistics, preferring the SQLite store when available.
// Accepts an optional ?window=N query parameter (unit: hours, default 24).
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	windowHours := 24
	if w := c.Query("window"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			windowHours = n
		}
	}

	if usage.UsageStore != nil {
		from := time.Now().Add(-time.Duration(windowHours) * time.Hour)
		to := time.Now()
		report, err := usage.UsageStore.QueryFullUsageReport(from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to query usage: %v", err)})
			return
		}
		c.JSON(http.StatusOK, buildPersistResponse(report))
		return
	}

	// Fallback to in-memory LoggerPlugin
	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, gin.H{
		"usage":           snapshot,
		"failed_requests": snapshot.FailureCount,
	})
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

// ExportUsageStatistics returns a complete usage snapshot for backup/migration.
func (h *Handler) ExportUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, usageExportPayload{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Usage:      snapshot,
	})
}

// ImportUsageStatistics merges a previously exported usage snapshot into memory.
func (h *Handler) ImportUsageStatistics(c *gin.Context) {
	if h == nil || h.usageStats == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usage statistics unavailable"})
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var payload usageImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.Version != 0 && payload.Version != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported version"})
		return
	}

	result := h.usageStats.MergeSnapshot(payload.Usage)
	snapshot := h.usageStats.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"added":           result.Added,
		"skipped":         result.Skipped,
		"total_requests":  snapshot.TotalRequests,
		"failed_requests": snapshot.FailureCount,
	})
}
