package diff

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestBuildConfigChangeDetails(t *testing.T) {
	oldCfg := &config.Config{
		Port:    8080,
		AuthDir: "/tmp/auth-old",
		RemoteManagement: config.RemoteManagement{
			AllowRemote:            false,
			SecretKey:              "old",
			DisableControlPanel:    false,
			DisableAutoUpdatePanel: false,
			PanelGitHubRepository:  "repo-old",
		},
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name: "compat-a",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "k1"},
				},
				Models: []config.OpenAICompatibilityModel{{Name: "m1"}},
			},
		},
	}

	newCfg := &config.Config{
		Port:    9090,
		AuthDir: "/tmp/auth-new",
		RemoteManagement: config.RemoteManagement{
			AllowRemote:            true,
			SecretKey:              "new",
			DisableControlPanel:    true,
			DisableAutoUpdatePanel: true,
			PanelGitHubRepository:  "repo-new",
		},
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name: "compat-a",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "k1"},
				},
				Models: []config.OpenAICompatibilityModel{{Name: "m1"}, {Name: "m2"}},
			},
			{
				Name: "compat-b",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "k2"},
				},
			},
		},
	}

	details := BuildConfigChangeDetails(oldCfg, newCfg)

	expectContains(t, details, "port: 8080 -> 9090")
	expectContains(t, details, "auth-dir: /tmp/auth-old -> /tmp/auth-new")
	expectContains(t, details, "remote-management.allow-remote: false -> true")
	expectContains(t, details, "remote-management.disable-auto-update-panel: false -> true")
	expectContains(t, details, "remote-management.secret-key: updated")
	expectContains(t, details, "openai-compatibility:")
	expectContains(t, details, "  provider added: compat-b (api-keys=1, models=0)")
	expectContains(t, details, "  provider updated: compat-a (models 1 -> 2)")
}

func TestBuildConfigChangeDetails_NoChanges(t *testing.T) {
	cfg := &config.Config{
		Port: 8080,
	}
	if details := BuildConfigChangeDetails(cfg, cfg); len(details) != 0 {
		t.Fatalf("expected no change entries, got %v", details)
	}
}

func TestBuildConfigChangeDetails_ClaudeHeaders(t *testing.T) {
	oldCfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c1", Headers: map[string]string{"H": "1"}, ExcludedModels: []string{"a"}},
		},
	}
	newCfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c1", Headers: map[string]string{"H": "2"}, ExcludedModels: []string{"a", "b"}},
		},
	}

	details := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, details, "claude[0].headers: updated")
	expectContains(t, details, "claude[0].excluded-models: updated (1 -> 2 entries)")
}

func TestBuildConfigChangeDetails_ModelPrefixes(t *testing.T) {
	oldCfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c1", Prefix: "old-c", BaseURL: "http://c", ProxyURL: "http://cp"},
		},
	}
	newCfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c1", Prefix: "new-c", BaseURL: "http://c", ProxyURL: "http://cp"},
		},
	}

	changes := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, changes, "claude[0].prefix: old-c -> new-c")
}

func TestBuildConfigChangeDetails_NilSafe(t *testing.T) {
	if details := BuildConfigChangeDetails(nil, &config.Config{}); len(details) != 0 {
		t.Fatalf("expected empty change list when old nil, got %v", details)
	}
	if details := BuildConfigChangeDetails(&config.Config{}, nil); len(details) != 0 {
		t.Fatalf("expected empty change list when new nil, got %v", details)
	}
}

func TestBuildConfigChangeDetails_SecretsAndCounts(t *testing.T) {
	oldCfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"a"},
		},
		RemoteManagement: config.RemoteManagement{
			SecretKey: "",
		},
	}
	newCfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"a", "b", "c"},
		},
		RemoteManagement: config.RemoteManagement{
			SecretKey: "new-secret",
		},
	}

	details := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, details, "api-keys count: 1 -> 3")
	expectContains(t, details, "remote-management.secret-key: created")
}

func TestBuildConfigChangeDetails_FlagsAndKeys(t *testing.T) {
	oldCfg := &config.Config{
		Port:                   1000,
		AuthDir:                "/old",
		Debug:                  false,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		DisableCooling:         false,
		RequestRetry:           1,
		MaxRetryCredentials:    1,
		MaxRetryInterval:       1,
		WebsocketAuth:          false,
		ClaudeKey:              []config.ClaudeKey{{APIKey: "c1"}},
		RemoteManagement:       config.RemoteManagement{DisableControlPanel: false, PanelGitHubRepository: "old/repo", SecretKey: "keep"},
		SDKConfig: sdkconfig.SDKConfig{
			RequestLog:                 false,
			ProxyURL:                   "http://old-proxy",
			APIKeys:                    []string{"key-1"},
			ForceModelPrefix:           false,
			NonStreamKeepAliveInterval: 0,
		},
	}
	newCfg := &config.Config{
		Port:                   2000,
		AuthDir:                "/new",
		Debug:                  true,
		LoggingToFile:          true,
		UsageStatisticsEnabled: true,
		DisableCooling:         true,
		RequestRetry:           2,
		MaxRetryCredentials:    3,
		MaxRetryInterval:       3,
		WebsocketAuth:          true,
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c1", BaseURL: "http://new", ProxyURL: "http://p", Headers: map[string]string{"H": "1"}, ExcludedModels: []string{"a"}},
			{APIKey: "c2"},
		},
		RemoteManagement: config.RemoteManagement{
			DisableControlPanel:    true,
			DisableAutoUpdatePanel: true,
			PanelGitHubRepository:  "new/repo",
			SecretKey:              "",
		},
		SDKConfig: sdkconfig.SDKConfig{
			RequestLog:                 true,
			ProxyURL:                   "http://new-proxy",
			APIKeys:                    []string{" key-1 ", "key-2"},
			ForceModelPrefix:           true,
			NonStreamKeepAliveInterval: 5,
			DisableImageGeneration:     config.DisableImageGenerationAll,
		},
	}

	details := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, details, "debug: false -> true")
	expectContains(t, details, "logging-to-file: false -> true")
	expectContains(t, details, "usage-statistics-enabled: false -> true")
	expectContains(t, details, "disable-cooling: false -> true")
	expectContains(t, details, "disable-image-generation: false -> true")
	expectContains(t, details, "request-log: false -> true")
	expectContains(t, details, "request-retry: 1 -> 2")
	expectContains(t, details, "max-retry-credentials: 1 -> 3")
	expectContains(t, details, "max-retry-interval: 1 -> 3")
	expectContains(t, details, "proxy-url: http://old-proxy -> http://new-proxy")
	expectContains(t, details, "ws-auth: false -> true")
	expectContains(t, details, "force-model-prefix: false -> true")
	expectContains(t, details, "nonstream-keepalive-interval: 0 -> 5")
	expectContains(t, details, "api-keys count: 1 -> 2")
	expectContains(t, details, "claude-api-key count: 1 -> 2")
	expectContains(t, details, "remote-management.disable-control-panel: false -> true")
	expectContains(t, details, "remote-management.disable-auto-update-panel: false -> true")
	expectContains(t, details, "remote-management.panel-github-repository: old/repo -> new/repo")
	expectContains(t, details, "remote-management.secret-key: deleted")
}

func TestBuildConfigChangeDetails_AllBranches(t *testing.T) {
	oldCfg := &config.Config{
		Port:                   1,
		AuthDir:                "/a",
		Debug:                  false,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		DisableCooling:         false,
		RequestRetry:           1,
		MaxRetryCredentials:    1,
		MaxRetryInterval:       1,
		WebsocketAuth:          false,
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c-old", BaseURL: "http://c-old", ProxyURL: "http://cp-old", Headers: map[string]string{"H": "1"}, ExcludedModels: []string{"x"}},
		},
		RemoteManagement: config.RemoteManagement{
			AllowRemote:            false,
			DisableControlPanel:    false,
			DisableAutoUpdatePanel: false,
			PanelGitHubRepository:  "old/repo",
			SecretKey:              "old",
		},
		SDKConfig: sdkconfig.SDKConfig{
			RequestLog: false,
			ProxyURL:   "http://old-proxy",
			APIKeys:    []string{" keyA "},
		},
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name: "prov-old",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "k1"},
				},
				Models: []config.OpenAICompatibilityModel{{Name: "m1"}},
			},
		},
	}
	newCfg := &config.Config{
		Port:                   2,
		AuthDir:                "/b",
		Debug:                  true,
		LoggingToFile:          true,
		UsageStatisticsEnabled: true,
		DisableCooling:         true,
		RequestRetry:           2,
		MaxRetryCredentials:    3,
		MaxRetryInterval:       3,
		WebsocketAuth:          true,
		ClaudeKey: []config.ClaudeKey{
			{APIKey: "c-new", BaseURL: "http://c-new", ProxyURL: "http://cp-new", Headers: map[string]string{"H": "2"}, ExcludedModels: []string{"x", "y"}},
		},
		RemoteManagement: config.RemoteManagement{
			AllowRemote:            true,
			DisableControlPanel:    true,
			DisableAutoUpdatePanel: true,
			PanelGitHubRepository:  "new/repo",
			SecretKey:              "",
		},
		SDKConfig: sdkconfig.SDKConfig{
			RequestLog:             true,
			ProxyURL:               "http://new-proxy",
			APIKeys:                []string{"keyB"},
			DisableImageGeneration: config.DisableImageGenerationAll,
		},
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name: "prov-old",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{
					{APIKey: "k1"},
					{APIKey: "k2"},
				},
				Models: []config.OpenAICompatibilityModel{{Name: "m1"}, {Name: "m2"}},
			},
			{
				Name:          "prov-new",
				APIKeyEntries: []config.OpenAICompatibilityAPIKey{{APIKey: "k3"}},
			},
		},
	}

	changes := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, changes, "port: 1 -> 2")
	expectContains(t, changes, "auth-dir: /a -> /b")
	expectContains(t, changes, "debug: false -> true")
	expectContains(t, changes, "logging-to-file: false -> true")
	expectContains(t, changes, "usage-statistics-enabled: false -> true")
	expectContains(t, changes, "disable-cooling: false -> true")
	expectContains(t, changes, "disable-image-generation: false -> true")
	expectContains(t, changes, "request-retry: 1 -> 2")
	expectContains(t, changes, "max-retry-credentials: 1 -> 3")
	expectContains(t, changes, "max-retry-interval: 1 -> 3")
	expectContains(t, changes, "proxy-url: http://old-proxy -> http://new-proxy")
	expectContains(t, changes, "ws-auth: false -> true")
	expectContains(t, changes, "api-keys: values updated (count unchanged, redacted)")
	expectContains(t, changes, "claude[0].base-url: http://c-old -> http://c-new")
	expectContains(t, changes, "claude[0].proxy-url: http://cp-old -> http://cp-new")
	expectContains(t, changes, "claude[0].api-key: updated")
	expectContains(t, changes, "claude[0].headers: updated")
	expectContains(t, changes, "claude[0].excluded-models: updated (1 -> 2 entries)")
	expectContains(t, changes, "remote-management.allow-remote: false -> true")
	expectContains(t, changes, "remote-management.disable-control-panel: false -> true")
	expectContains(t, changes, "remote-management.disable-auto-update-panel: false -> true")
	expectContains(t, changes, "remote-management.panel-github-repository: old/repo -> new/repo")
	expectContains(t, changes, "remote-management.secret-key: deleted")
	expectContains(t, changes, "openai-compatibility:")
}

func TestFormatProxyURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "<none>"},
		{name: "invalid", in: "http://[::1", want: "<redacted>"},
		{name: "fullURLRedactsUserinfoAndPath", in: "http://user:pass@example.com:8080/path?x=1#frag", want: "http://example.com:8080"},
		{name: "socks5RedactsUserinfoAndPath", in: "socks5://user:pass@192.168.1.1:1080/path?x=1", want: "socks5://192.168.1.1:1080"},
		{name: "socks5HostPort", in: "socks5://proxy.example.com:1080/", want: "socks5://proxy.example.com:1080"},
		{name: "hostPortNoScheme", in: "example.com:1234/path?x=1", want: "example.com:1234"},
		{name: "relativePathRedacted", in: "/just/path", want: "<redacted>"},
		{name: "schemeAndHost", in: "https://example.com", want: "https://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatProxyURL(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBuildConfigChangeDetails_RemoteManagementSecretUpdated(t *testing.T) {
	oldCfg := &config.Config{
		RemoteManagement: config.RemoteManagement{
			SecretKey: "old",
		},
	}
	newCfg := &config.Config{
		RemoteManagement: config.RemoteManagement{
			SecretKey: "new",
		},
	}

	changes := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, changes, "remote-management.secret-key: updated")
}

func TestBuildConfigChangeDetails_CountBranches(t *testing.T) {
	oldCfg := &config.Config{}
	newCfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{{APIKey: "c"}},
	}

	changes := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, changes, "claude-api-key count: 0 -> 1")
}

func TestTrimStrings(t *testing.T) {
	out := trimStrings([]string{" a ", "b", "  c"})
	if len(out) != 3 || out[0] != "a" || out[1] != "b" || out[2] != "c" {
		t.Fatalf("unexpected trimmed strings: %v", out)
	}
}
