package synthesizer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/limiter"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher/diff"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

// StableIDGenerator generates stable, deterministic IDs for auth entries.
// It uses SHA256 hashing with collision handling via counters.
// It is not safe for concurrent use.
type StableIDGenerator struct {
	counters map[string]int
}

// NewStableIDGenerator creates a new StableIDGenerator instance.
func NewStableIDGenerator() *StableIDGenerator {
	return &StableIDGenerator{counters: make(map[string]int)}
}

// Next generates a stable ID based on the kind and parts.
// Returns the full ID (kind:hash) and the short hash portion.
func (g *StableIDGenerator) Next(kind string, parts ...string) (string, string) {
	if g == nil {
		return kind + ":000000000000", "000000000000"
	}
	hasher := sha256.New()
	hasher.Write([]byte(kind))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		hasher.Write([]byte{0})
		hasher.Write([]byte(trimmed))
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if len(digest) < 12 {
		digest = fmt.Sprintf("%012s", digest)
	}
	short := digest[:12]
	key := kind + ":" + short
	index := g.counters[key]
	g.counters[key] = index + 1
	if index > 0 {
		short = fmt.Sprintf("%s-%d", short, index)
	}
	return fmt.Sprintf("%s:%s", kind, short), short
}

// ApplyAuthExcludedModelsMeta applies excluded models metadata to an auth entry.
// It computes a hash of excluded models and sets the auth_kind attribute.
// For OAuth entries, perKey (from the JSON file's excluded-models field) is merged
// with the global oauth-excluded-models config for the provider.
func ApplyAuthExcludedModelsMeta(auth *coreauth.Auth, cfg *config.Config, perKey []string, authKind string) {
	if auth == nil || cfg == nil {
		return
	}
	authKindKey := strings.ToLower(strings.TrimSpace(authKind))
	seen := make(map[string]struct{})
	add := func(list []string) {
		for _, entry := range list {
			if trimmed := strings.TrimSpace(entry); trimmed != "" {
				key := strings.ToLower(trimmed)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
			}
		}
	}
	if authKindKey == "apikey" {
		add(perKey)
	} else {
		// OAuth entries: per-account excluded models only (global oauth-excluded-models removed in Phase 7 Batch 3)
		add(perKey)
	}
	combined := make([]string, 0, len(seen))
	for k := range seen {
		combined = append(combined, k)
	}
	sort.Strings(combined)
	hash := diff.ComputeExcludedModelsHash(combined)
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	if hash != "" {
		auth.Attributes["excluded_models_hash"] = hash
	}
	// Store the combined excluded models list so that routing can read it at runtime
	if len(combined) > 0 {
		auth.Attributes["excluded_models"] = strings.Join(combined, ",")
	}
	if authKind != "" {
		auth.Attributes["auth_kind"] = authKind
	}
}

// addConfigHeadersToAttrs adds header configuration to auth attributes.
// Headers are prefixed with "header:" in the attributes map.
func addConfigHeadersToAttrs(headers map[string]string, attrs map[string]string) {
	if len(headers) == 0 || attrs == nil {
		return
	}
	for hk, hv := range headers {
		key := strings.TrimSpace(hk)
		val := strings.TrimSpace(hv)
		if key == "" || val == "" {
			continue
		}
		attrs["header:"+key] = val
	}
}

// wireModelPrices registers model prices with the global UsageStore and clears
// stale prices for models no longer in the config.
func wireModelPrices(authID string, modelPrices map[string]usage.ModelPrices) {
	if usage.UsageStore == nil {
		return
	}
	currentModels := make(map[string]struct{}, len(modelPrices))
	for m, p := range modelPrices {
		usage.UsageStore.SetModelPrices(authID, m, p)
		currentModels[m] = struct{}{}
	}
	usage.UsageStore.ClearStaleModelPrices(authID, currentModels)
}

// wireLimitsToLimiter registers parsed limit windows with the global limiter.
// Each ParsedModelLimitWindow becomes one LimitConfig (with its Models list preserved),
// so models in the same window share usage.
func wireLimitsToLimiter(authID string, raw []config.ModelLimitWindow, parsed []config.ParsedModelLimitWindow) {
	lim := limiter.DefaultLimiter()

	if len(parsed) == 0 {
		lim.UpdateLimits(authID, nil)
		return
	}

	configs := make([]limiter.LimitConfig, 0, len(parsed))
	for _, w := range parsed {
		configs = append(configs, limiter.LimitConfig{
			Window:       w.Window,
			InputTokens:  w.InputTokens,
			OutputTokens: w.OutputTokens,
			CacheTokens:  w.CacheTokens,
			Price:        w.Price,
			Models:       w.Models,
		})
	}
	lim.UpdateLimits(authID, configs)
}

// claudeModelPrices builds a model→prices map from ClaudeModel definitions.
func claudeModelPrices(models []config.ClaudeModel) map[string]usage.ModelPrices {
	out := make(map[string]usage.ModelPrices, len(models))
	for _, m := range models {
		name := strings.ToLower(strings.TrimSpace(m.Name))
		if name == "" {
			continue
		}
		if m.InputPriceM > 0 || m.OutputPriceM > 0 || m.CachePriceM > 0 {
			out[name] = usage.ModelPrices{
				InputPriceM:  m.InputPriceM,
				OutputPriceM: m.OutputPriceM,
				CachePriceM:  m.CachePriceM,
			}
		}
	}
	return out
}

// openAICompatModelPrices builds a model→prices map from OpenAICompatibilityModel definitions.
func openAICompatModelPrices(models []config.OpenAICompatibilityModel) map[string]usage.ModelPrices {
	out := make(map[string]usage.ModelPrices, len(models))
	for _, m := range models {
		name := strings.ToLower(strings.TrimSpace(m.Name))
		if name == "" {
			continue
		}
		if m.InputPriceM > 0 || m.OutputPriceM > 0 || m.CachePriceM > 0 {
			out[name] = usage.ModelPrices{
				InputPriceM:  m.InputPriceM,
				OutputPriceM: m.OutputPriceM,
				CachePriceM:  m.CachePriceM,
			}
		}
	}
	return out
}
