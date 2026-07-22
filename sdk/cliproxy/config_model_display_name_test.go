package cliproxy

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestBuildConfigModelsDisplayName(t *testing.T) {
	tests := []struct {
		name string
		want string
		got  func() *ModelInfo
	}{
		{
			name: "claude",
			want: "Claude Catalog Name",
			got: func() *ModelInfo {
				return buildClaudeConfigModels(&config.ClaudeKey{Models: []config.ClaudeModel{{
					Name: "claude-upstream", Alias: "claude-catalog", DisplayName: "Claude Catalog Name",
				}}})[0]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got().DisplayName; got != tt.want {
				t.Fatalf("DisplayName = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildConfigModelsDisplayNameFallback(t *testing.T) {
	model := buildClaudeConfigModels(&config.ClaudeKey{Models: []config.ClaudeModel{{
		Name: "claude-upstream", Alias: "claude-catalog",
	}}})[0]
	if model.DisplayName != "claude-upstream" {
		t.Fatalf("DisplayName = %q, want upstream model name", model.DisplayName)
	}
}
