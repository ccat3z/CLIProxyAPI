package test

import (
	"testing"

	_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator"

	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestOpenAIResponsesToOpenAI_IgnoresBuiltinTools(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5",
		"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}],
		"tools":[{"type":"web_search","search_context_size":"low"}]
	}`)

	out := sdktranslator.TranslateRequest(sdktranslator.FormatOpenAIResponse, sdktranslator.FormatOpenAI, "gpt-5", in, false)

	if got := gjson.GetBytes(out, "tools.#").Int(); got != 0 {
		t.Fatalf("expected 0 tools (builtin tools not supported in Chat Completions), got %d: %s", got, string(out))
	}
}
