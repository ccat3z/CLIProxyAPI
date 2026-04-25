package synthesizer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/limiter"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher/diff"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
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
		// For OAuth: merge per-account excluded models with global provider-level exclusions
		add(perKey)
		if cfg.OAuthExcludedModels != nil {
			providerKey := strings.ToLower(strings.TrimSpace(auth.Provider))
			add(cfg.OAuthExcludedModels[providerKey])
		}
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

// wireLimitsToLimiter registers parsed limit windows with the global limiter.
// It expands the Models list in each window so each model gets its own limit config,
// and registers model prices for cost-based limiting.
// Each model entry in the limits models list is resolved to both its alias and upstream name,
// so limits are enforced regardless of which name the executor uses.
// Stale limits (models that were previously limited but no longer in config) are removed.
func wireLimitsToLimiter(authID string, raw []config.ModelLimitWindow, parsed []config.ParsedModelLimitWindow, modelPrices map[string]usage.ModelPrices, aliasMap map[string]string) {
	lim := limiter.DefaultLimiter()

	// Resolve each model in the limits config to all its known names (alias + upstream name).
	// aliasMap: alias -> upstream-name (both lowercased).
	resolvedModels := make(map[string]struct{})
	for _, w := range parsed {
		for _, m := range w.Models {
			resolvedModels[m] = struct{}{}
			// If m is an alias, also add the upstream name
			if upstream, ok := aliasMap[m]; ok && upstream != m {
				resolvedModels[upstream] = struct{}{}
			}
			// If m is an upstream name, also add any alias pointing to it
			for alias, name := range aliasMap {
				if name == m && alias != m {
					resolvedModels[alias] = struct{}{}
				}
			}
		}
	}

	modelList := make([]string, 0, len(resolvedModels))
	for m := range resolvedModels {
		modelList = append(modelList, m)
	}
	lim.SyncLimitsForAuth(authID, modelList)

	// Register prices for every resolved model. If a model has no prices in
	// modelPrices, register zero prices so stale prices from a previous config
	// are cleared (otherwise cost computation would use outdated prices).
	if usage.UsageStore != nil {
		for m := range resolvedModels {
			p, ok := modelPrices[m]
			if !ok {
				p = usage.ModelPrices{}
			}
			usage.UsageStore.SetModelPrices(authID, m, p)
		}
	}

	if len(parsed) == 0 {
		return
	}
	// Expand windows: one LimitConfig per resolved model name in each window's Models list
	windowsByModel := make(map[string][]limiter.LimitConfig)
	for _, w := range parsed {
		cfg := limiter.LimitConfig{
			Window:       w.Window,
			InputTokens:  w.InputTokens,
			OutputTokens: w.OutputTokens,
			CacheTokens:  w.CacheTokens,
			Price:        w.Price,
		}
		for _, m := range w.Models {
			windowsByModel[m] = append(windowsByModel[m], cfg)
			// Also add limits for the upstream name if m is an alias
			if upstream, ok := aliasMap[m]; ok && upstream != m {
				windowsByModel[upstream] = append(windowsByModel[upstream], cfg)
			}
			// Also add limits for aliases if m is an upstream name
			for alias, name := range aliasMap {
				if name == m && alias != m {
					windowsByModel[alias] = append(windowsByModel[alias], cfg)
				}
			}
		}
	}
	for model, windows := range windowsByModel {
		lim.UpdateLimits(authID, model, windows)
	}
}

// claudeModelPrices builds a model→prices map from ClaudeModel definitions.
// Prices are keyed by both alias and upstream name.
func claudeModelPrices(models []config.ClaudeModel) map[string]usage.ModelPrices {
	out := make(map[string]usage.ModelPrices, len(models)*2)
	for _, m := range models {
		name := strings.ToLower(strings.TrimSpace(m.Name))
		alias := strings.ToLower(strings.TrimSpace(m.Alias))
		if name == "" {
			continue
		}
		if m.InputPriceM > 0 || m.OutputPriceM > 0 || m.CachePriceM > 0 {
			p := usage.ModelPrices{
				InputPriceM:  m.InputPriceM,
				OutputPriceM: m.OutputPriceM,
				CachePriceM:  m.CachePriceM,
			}
			out[name] = p
			if alias != "" {
				out[alias] = p
			}
		}
	}
	return out
}

// openAICompatModelPrices builds a model→prices map from OpenAICompatibilityModel definitions.
// Prices are keyed by both alias and upstream name.
func openAICompatModelPrices(models []config.OpenAICompatibilityModel) map[string]usage.ModelPrices {
	out := make(map[string]usage.ModelPrices, len(models)*2)
	for _, m := range models {
		name := strings.ToLower(strings.TrimSpace(m.Name))
		alias := strings.ToLower(strings.TrimSpace(m.Alias))
		if name == "" {
			continue
		}
		if m.InputPriceM > 0 || m.OutputPriceM > 0 || m.CachePriceM > 0 {
			p := usage.ModelPrices{
				InputPriceM:  m.InputPriceM,
				OutputPriceM: m.OutputPriceM,
				CachePriceM:  m.CachePriceM,
			}
			out[name] = p
			if alias != "" {
				out[alias] = p
			}
		}
	}
	return out
}

// claudeAliasMap builds an alias→name map from ClaudeModel definitions.
func claudeAliasMap(models []config.ClaudeModel) map[string]string {
	out := make(map[string]string, len(models))
	for _, m := range models {
		name := strings.ToLower(strings.TrimSpace(m.Name))
		alias := strings.ToLower(strings.TrimSpace(m.Alias))
		if alias != "" && name != "" {
			out[alias] = name
		}
	}
	return out
}

// openAICompatAliasMap builds an alias→name map from OpenAICompatibilityModel definitions.
func openAICompatAliasMap(models []config.OpenAICompatibilityModel) map[string]string {
	out := make(map[string]string, len(models))
	for _, m := range models {
		name := strings.ToLower(strings.TrimSpace(m.Name))
		alias := strings.ToLower(strings.TrimSpace(m.Alias))
		if alias != "" && name != "" {
			out[alias] = name
		}
	}
	return out
}
