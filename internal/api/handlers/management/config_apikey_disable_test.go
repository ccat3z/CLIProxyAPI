package management

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestSetConfigAPIKeyExcludedAll(t *testing.T) {
	gotDisable := setConfigAPIKeyExcludedAll([]string{"gpt-5"}, true)
	if len(gotDisable) != 2 || gotDisable[0] != "gpt-5" || gotDisable[1] != "*" {
		t.Fatalf("unexpected disable list: %#v", gotDisable)
	}
	gotEnable := setConfigAPIKeyExcludedAll([]string{"gpt-5", "*"}, false)
	if len(gotEnable) != 1 || gotEnable[0] != "gpt-5" {
		t.Fatalf("unexpected enable list: %#v", gotEnable)
	}
}

func TestToggleConfigAPIKeyExcludedAll_Claude(t *testing.T) {
	cfg := &config.Config{
		ClaudeKey: []config.ClaudeKey{{
			APIKey:  "sk-test",
			BaseURL: "https://example.com/v1",
		}},
	}
	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("claude:apikey", "sk-test", "https://example.com/v1")
	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "claude",
		Attributes: map[string]string{
			"api_key":  "sk-test",
			"base_url": "https://example.com/v1",
			"source":   "config:claude[abc]",
		},
	}

	handled, err := toggleConfigAPIKeyExcludedAll(cfg, auth, true)
	if err != nil || !handled {
		t.Fatalf("toggle disable: handled=%v err=%v", handled, err)
	}
	if len(cfg.ClaudeKey[0].ExcludedModels) != 1 || cfg.ClaudeKey[0].ExcludedModels[0] != "*" {
		t.Fatalf("expected excluded-models [*], got %#v", cfg.ClaudeKey[0].ExcludedModels)
	}

	handled, err = toggleConfigAPIKeyExcludedAll(cfg, auth, false)
	if err != nil || !handled {
		t.Fatalf("toggle enable: handled=%v err=%v", handled, err)
	}
	if len(cfg.ClaudeKey[0].ExcludedModels) != 0 {
		t.Fatalf("expected excluded-models cleared, got %#v", cfg.ClaudeKey[0].ExcludedModels)
	}
}
