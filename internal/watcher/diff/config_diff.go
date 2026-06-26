package diff

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

// BuildConfigChangeDetails computes a redacted, human-readable list of config changes.
// Secrets are never printed; only structural or non-sensitive fields are surfaced.
func BuildConfigChangeDetails(oldCfg, newCfg *config.Config) []string {
	changes := make([]string, 0, 16)
	if oldCfg == nil || newCfg == nil {
		return changes
	}

	// Simple scalars
	if oldCfg.Port != newCfg.Port {
		changes = append(changes, fmt.Sprintf("port: %d -> %d", oldCfg.Port, newCfg.Port))
	}
	if oldCfg.Debug != newCfg.Debug {
		changes = append(changes, fmt.Sprintf("debug: %t -> %t", oldCfg.Debug, newCfg.Debug))
	}
	if oldCfg.LoggingToFile != newCfg.LoggingToFile {
		changes = append(changes, fmt.Sprintf("logging-to-file: %t -> %t", oldCfg.LoggingToFile, newCfg.LoggingToFile))
	}
	if oldCfg.UsageStatisticsEnabled != newCfg.UsageStatisticsEnabled {
		changes = append(changes, fmt.Sprintf("usage-statistics-enabled: %t -> %t", oldCfg.UsageStatisticsEnabled, newCfg.UsageStatisticsEnabled))
	}
	if oldCfg.DisableCooling != newCfg.DisableCooling {
		changes = append(changes, fmt.Sprintf("disable-cooling: %t -> %t", oldCfg.DisableCooling, newCfg.DisableCooling))
	}
	if oldCfg.TransientErrorCooldownSeconds != newCfg.TransientErrorCooldownSeconds {
		changes = append(changes, fmt.Sprintf("transient-error-cooldown-seconds: %d -> %d", oldCfg.TransientErrorCooldownSeconds, newCfg.TransientErrorCooldownSeconds))
	}
	if oldCfg.DisableClaudeCloakMode != newCfg.DisableClaudeCloakMode {
		changes = append(changes, fmt.Sprintf("disable-claude-cloak-mode: %t -> %t", oldCfg.DisableClaudeCloakMode, newCfg.DisableClaudeCloakMode))
	}
	if oldCfg.DisableImageGeneration != newCfg.DisableImageGeneration {
		changes = append(changes, fmt.Sprintf("disable-image-generation: %v -> %v", oldCfg.DisableImageGeneration, newCfg.DisableImageGeneration))
	}
	if oldCfg.RequestLog != newCfg.RequestLog {
		changes = append(changes, fmt.Sprintf("request-log: %t -> %t", oldCfg.RequestLog, newCfg.RequestLog))
	}
	if oldCfg.LogsMaxTotalSizeMB != newCfg.LogsMaxTotalSizeMB {
		changes = append(changes, fmt.Sprintf("logs-max-total-size-mb: %d -> %d", oldCfg.LogsMaxTotalSizeMB, newCfg.LogsMaxTotalSizeMB))
	}
	if oldCfg.ErrorLogsMaxFiles != newCfg.ErrorLogsMaxFiles {
		changes = append(changes, fmt.Sprintf("error-logs-max-files: %d -> %d", oldCfg.ErrorLogsMaxFiles, newCfg.ErrorLogsMaxFiles))
	}
	if oldCfg.RequestRetry != newCfg.RequestRetry {
		changes = append(changes, fmt.Sprintf("request-retry: %d -> %d", oldCfg.RequestRetry, newCfg.RequestRetry))
	}
	if oldCfg.MaxRetryCredentials != newCfg.MaxRetryCredentials {
		changes = append(changes, fmt.Sprintf("max-retry-credentials: %d -> %d", oldCfg.MaxRetryCredentials, newCfg.MaxRetryCredentials))
	}
	if oldCfg.MaxRetryInterval != newCfg.MaxRetryInterval {
		changes = append(changes, fmt.Sprintf("max-retry-interval: %d -> %d", oldCfg.MaxRetryInterval, newCfg.MaxRetryInterval))
	}
	if oldCfg.WebsocketAuth != newCfg.WebsocketAuth {
		changes = append(changes, fmt.Sprintf("ws-auth: %t -> %t", oldCfg.WebsocketAuth, newCfg.WebsocketAuth))
	}
	if oldCfg.ForceModelPrefix != newCfg.ForceModelPrefix {
		changes = append(changes, fmt.Sprintf("force-model-prefix: %t -> %t", oldCfg.ForceModelPrefix, newCfg.ForceModelPrefix))
	}
	if oldCfg.NonStreamKeepAliveInterval != newCfg.NonStreamKeepAliveInterval {
		changes = append(changes, fmt.Sprintf("nonstream-keepalive-interval: %d -> %d", oldCfg.NonStreamKeepAliveInterval, newCfg.NonStreamKeepAliveInterval))
	}

	if oldCfg.Routing.Strategy != newCfg.Routing.Strategy {
		changes = append(changes, fmt.Sprintf("routing.strategy: %s -> %s", oldCfg.Routing.Strategy, newCfg.Routing.Strategy))
	}
	if !reflect.DeepEqual(oldCfg.Payload, newCfg.Payload) {
		changes = appendPayloadConfigChanges(changes, oldCfg.Payload, newCfg.Payload)
	}

	// API keys (redacted) and counts
	if len(oldCfg.APIKeys) != len(newCfg.APIKeys) {
		changes = append(changes, fmt.Sprintf("api-keys count: %d -> %d", len(oldCfg.APIKeys), len(newCfg.APIKeys)))
	} else if !reflect.DeepEqual(trimStrings(oldCfg.APIKeys), trimStrings(newCfg.APIKeys)) {
		changes = append(changes, "api-keys: values updated (count unchanged, redacted)")
	}

	// Claude keys (do not print key material)
	if len(oldCfg.ClaudeKey) != len(newCfg.ClaudeKey) {
		changes = append(changes, fmt.Sprintf("claude-api-key count: %d -> %d", len(oldCfg.ClaudeKey), len(newCfg.ClaudeKey)))
	} else {
		for i := range oldCfg.ClaudeKey {
			o := oldCfg.ClaudeKey[i]
			n := newCfg.ClaudeKey[i]
			if strings.TrimSpace(o.BaseURL) != strings.TrimSpace(n.BaseURL) {
				changes = append(changes, fmt.Sprintf("claude[%d].base-url: %s -> %s", i, strings.TrimSpace(o.BaseURL), strings.TrimSpace(n.BaseURL)))
			}
			if strings.TrimSpace(o.Prefix) != strings.TrimSpace(n.Prefix) {
				changes = append(changes, fmt.Sprintf("claude[%d].prefix: %s -> %s", i, strings.TrimSpace(o.Prefix), strings.TrimSpace(n.Prefix)))
			}
			if strings.TrimSpace(o.APIKey) != strings.TrimSpace(n.APIKey) {
				changes = append(changes, fmt.Sprintf("claude[%d].api-key: updated", i))
			}
			if !equalStringMap(o.Headers, n.Headers) {
				changes = append(changes, fmt.Sprintf("claude[%d].headers: updated", i))
			}
			oldModels := SummarizeClaudeModels(o.Models)
			newModels := SummarizeClaudeModels(n.Models)
			if oldModels.hash != newModels.hash {
				changes = append(changes, fmt.Sprintf("claude[%d].models: updated (%d -> %d entries)", i, oldModels.count, newModels.count))
			}
			oldExcluded := SummarizeExcludedModels(o.ExcludedModels)
			newExcluded := SummarizeExcludedModels(n.ExcludedModels)
			if oldExcluded.hash != newExcluded.hash {
				changes = append(changes, fmt.Sprintf("claude[%d].excluded-models: updated (%d -> %d entries)", i, oldExcluded.count, newExcluded.count))
			}
			if o.Cloak != nil && n.Cloak != nil {
				if strings.TrimSpace(o.Cloak.Mode) != strings.TrimSpace(n.Cloak.Mode) {
					changes = append(changes, fmt.Sprintf("claude[%d].cloak.mode: %s -> %s", i, o.Cloak.Mode, n.Cloak.Mode))
				}
				if o.Cloak.StrictMode != n.Cloak.StrictMode {
					changes = append(changes, fmt.Sprintf("claude[%d].cloak.strict-mode: %t -> %t", i, o.Cloak.StrictMode, n.Cloak.StrictMode))
				}
				if len(o.Cloak.SensitiveWords) != len(n.Cloak.SensitiveWords) {
					changes = append(changes, fmt.Sprintf("claude[%d].cloak.sensitive-words: %d -> %d", i, len(o.Cloak.SensitiveWords), len(n.Cloak.SensitiveWords)))
				}
			}
		}
	}

	// Remote management (never print the key)
	if oldCfg.RemoteManagement.AllowRemote != newCfg.RemoteManagement.AllowRemote {
		changes = append(changes, fmt.Sprintf("remote-management.allow-remote: %t -> %t", oldCfg.RemoteManagement.AllowRemote, newCfg.RemoteManagement.AllowRemote))
	}
	if oldCfg.RemoteManagement.DisableControlPanel != newCfg.RemoteManagement.DisableControlPanel {
		changes = append(changes, fmt.Sprintf("remote-management.disable-control-panel: %t -> %t", oldCfg.RemoteManagement.DisableControlPanel, newCfg.RemoteManagement.DisableControlPanel))
	}
	if oldCfg.RemoteManagement.DisableAutoUpdatePanel != newCfg.RemoteManagement.DisableAutoUpdatePanel {
		changes = append(changes, fmt.Sprintf("remote-management.disable-auto-update-panel: %t -> %t", oldCfg.RemoteManagement.DisableAutoUpdatePanel, newCfg.RemoteManagement.DisableAutoUpdatePanel))
	}
	if oldCfg.RemoteManagement.DisableConfigAPI != newCfg.RemoteManagement.DisableConfigAPI {
		changes = append(changes, fmt.Sprintf("remote-management.disable-config-api: %t -> %t", oldCfg.RemoteManagement.DisableConfigAPI, newCfg.RemoteManagement.DisableConfigAPI))
	}
	oldPanelRepo := strings.TrimSpace(oldCfg.RemoteManagement.PanelGitHubRepository)
	newPanelRepo := strings.TrimSpace(newCfg.RemoteManagement.PanelGitHubRepository)
	if oldPanelRepo != newPanelRepo {
		changes = append(changes, fmt.Sprintf("remote-management.panel-github-repository: %s -> %s", oldPanelRepo, newPanelRepo))
	}
	if oldCfg.RemoteManagement.SecretKey != newCfg.RemoteManagement.SecretKey {
		switch {
		case oldCfg.RemoteManagement.SecretKey == "" && newCfg.RemoteManagement.SecretKey != "":
			changes = append(changes, "remote-management.secret-key: created")
		case oldCfg.RemoteManagement.SecretKey != "" && newCfg.RemoteManagement.SecretKey == "":
			changes = append(changes, "remote-management.secret-key: deleted")
		default:
			changes = append(changes, "remote-management.secret-key: updated")
		}
	}

	// OpenAI compatibility providers (summarized)
	if compat := DiffOpenAICompatibility(oldCfg.OpenAICompatibility, newCfg.OpenAICompatibility); len(compat) > 0 {
		changes = append(changes, "openai-compatibility:")
		for _, c := range compat {
			changes = append(changes, "  "+c)
		}
	}

	return changes
}

func trimStrings(in []string) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = strings.TrimSpace(in[i])
	}
	return out
}

func appendPayloadConfigChanges(changes []string, oldPayload, newPayload config.PayloadConfig) []string {
	changes = appendPayloadRuleChanges(changes, "default", oldPayload.Default, newPayload.Default)
	changes = appendPayloadRuleChanges(changes, "default-raw", oldPayload.DefaultRaw, newPayload.DefaultRaw)
	changes = appendPayloadRuleChanges(changes, "override", oldPayload.Override, newPayload.Override)
	changes = appendPayloadRuleChanges(changes, "override-raw", oldPayload.OverrideRaw, newPayload.OverrideRaw)
	changes = appendPayloadFilterRuleChanges(changes, "filter", oldPayload.Filter, newPayload.Filter)
	return changes
}

func appendPayloadRuleChanges(changes []string, section string, oldRules, newRules []config.PayloadRule) []string {
	if reflect.DeepEqual(oldRules, newRules) {
		return changes
	}
	return append(changes, fmt.Sprintf("payload.%s: updated (%d -> %d rules)", section, len(oldRules), len(newRules)))
}

func appendPayloadFilterRuleChanges(changes []string, section string, oldRules, newRules []config.PayloadFilterRule) []string {
	if reflect.DeepEqual(oldRules, newRules) {
		return changes
	}
	return append(changes, fmt.Sprintf("payload.%s: updated (%d -> %d rules)", section, len(oldRules), len(newRules)))
}

func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
