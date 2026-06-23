package diff

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

type ClaudeModelsSummary struct {
	hash  string
	count int
}

type ExcludedModelsSummary struct {
	hash  string
	count int
}

// SummarizeClaudeModels hashes Claude model aliases for change detection.
func SummarizeClaudeModels(models []config.ClaudeModel) ClaudeModelsSummary {
	if len(models) == 0 {
		return ClaudeModelsSummary{}
	}
	keys := normalizeModelPairs(func(out func(key string)) {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			alias := strings.TrimSpace(model.Alias)
			if name == "" && alias == "" {
				continue
			}
			out(strings.ToLower(name) + "|" + strings.ToLower(alias))
		}
	})
	return ClaudeModelsSummary{
		hash:  hashJoined(keys),
		count: len(keys),
	}
}

// SummarizeExcludedModels normalizes and hashes an excluded-model list.
func SummarizeExcludedModels(list []string) ExcludedModelsSummary {
	if len(list) == 0 {
		return ExcludedModelsSummary{}
	}
	normalized := make([]string, 0, len(list))
	for _, entry := range list {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, strings.ToLower(trimmed))
	}
	if len(normalized) == 0 {
		return ExcludedModelsSummary{}
	}
	return ExcludedModelsSummary{
		hash:  ComputeExcludedModelsHash(normalized),
		count: len(normalized),
	}
}

// expectContains fails the test if target is not present in list.
func expectContains(t *testing.T, list []string, target string) {
	t.Helper()
	for _, entry := range list {
		if entry == target {
			return
		}
	}
	t.Fatalf("expected list to contain %q, got %#v", target, list)
}
