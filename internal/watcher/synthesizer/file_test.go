package synthesizer

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher/diff"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestNewFileSynthesizer(t *testing.T) {
	synth := NewFileSynthesizer()
	if synth == nil {
		t.Fatal("expected non-nil synthesizer")
	}
}

func TestFileSynthesizer_Synthesize_NilContext(t *testing.T) {
	synth := NewFileSynthesizer()
	auths, err := synth.Synthesize(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auths) != 0 {
		t.Fatalf("expected empty auths, got %d", len(auths))
	}
}

func TestFileSynthesizer_Synthesize_EmptyConfig(t *testing.T) {
	synth := NewFileSynthesizer()
	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}
	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auths) != 0 {
		t.Fatalf("expected empty auths, got %d", len(auths))
	}
}

func TestSynthesizeAuthFile_ValidAuthFile(t *testing.T) {
	tempDir := t.TempDir()

	authData := map[string]any{
		"type":      "claude",
		"email":     "test@example.com",
		"proxy_url": "http://proxy.local",
		"prefix":    "test-prefix",
		"headers": map[string]string{
			" X-Test ": " value ",
			"X-Empty":  "  ",
		},
		"disable_cooling": true,
		"request_retry":   2,
	}
	data, _ := json.Marshal(authData)

	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IDGenerator: NewStableIDGenerator(),
	}

	authFile := filepath.Join(tempDir, "claude-auth.json")
	auths := SynthesizeAuthFile(ctx, authFile, data)
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth, got %d", len(auths))
	}

	if auths[0].Provider != "claude" {
		t.Errorf("expected provider claude, got %s", auths[0].Provider)
	}
	if auths[0].Label != "test@example.com" {
		t.Errorf("expected label test@example.com, got %s", auths[0].Label)
	}
	if auths[0].Prefix != "test-prefix" {
		t.Errorf("expected prefix test-prefix, got %s", auths[0].Prefix)
	}
	if got := auths[0].Attributes["header:X-Test"]; got != "value" {
		t.Errorf("expected header:X-Test value, got %q", got)
	}
	if _, ok := auths[0].Attributes["header:X-Empty"]; ok {
		t.Errorf("expected header:X-Empty to be absent, got %q", auths[0].Attributes["header:X-Empty"])
	}
	if v, ok := auths[0].Metadata["disable_cooling"].(bool); !ok || !v {
		t.Errorf("expected disable_cooling true, got %v", auths[0].Metadata["disable_cooling"])
	}
	if v, ok := auths[0].Metadata["request_retry"].(float64); !ok || int(v) != 2 {
		t.Errorf("expected request_retry 2, got %v", auths[0].Metadata["request_retry"])
	}
	if auths[0].Status != coreauth.StatusActive {
		t.Errorf("expected status active, got %s", auths[0].Status)
	}
}

func TestSynthesizeAuthFile_IgnoresGeminiProviderFile(t *testing.T) {
	tempDir := t.TempDir()

	authData := map[string]any{
		"type":  "gemini",
		"email": "gemini@example.com",
	}
	data, _ := json.Marshal(authData)

	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}

	authFile := filepath.Join(tempDir, "gemini-auth.json")
	auths := SynthesizeAuthFile(ctx, authFile, data)
	if len(auths) != 0 {
		t.Fatalf("expected Gemini auth file to be ignored, got %d auths", len(auths))
	}
}

func TestSynthesizeAuthFile_SkipsInvalidFiles(t *testing.T) {
	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}

	// Invalid payloads should return no auths
	if auths := SynthesizeAuthFile(ctx, "not-json.txt", []byte("text content")); len(auths) != 0 {
		t.Fatalf("expected 0 auths for non-JSON, got %d", len(auths))
	}
	if auths := SynthesizeAuthFile(ctx, "invalid.json", []byte("not valid json")); len(auths) != 0 {
		t.Fatalf("expected 0 auths for invalid JSON, got %d", len(auths))
	}
	if auths := SynthesizeAuthFile(ctx, "empty.json", []byte("")); len(auths) != 0 {
		t.Fatalf("expected 0 auths for empty file, got %d", len(auths))
	}
	if auths := SynthesizeAuthFile(ctx, "no-type.json", []byte(`{"email": "test@example.com"}`)); len(auths) != 0 {
		t.Fatalf("expected 0 auths for no-type, got %d", len(auths))
	}

	// Valid payload
	tempDir := t.TempDir()
	validData, _ := json.Marshal(map[string]any{"type": "claude", "email": "valid@example.com"})
	authFile := filepath.Join(tempDir, "valid.json")
	auths := SynthesizeAuthFile(ctx, authFile, validData)
	if len(auths) != 1 {
		t.Fatalf("only valid auth file should be processed, got %d", len(auths))
	}
	if auths[0].Label != "valid@example.com" {
		t.Errorf("expected label valid@example.com, got %s", auths[0].Label)
	}
}

func TestSynthesizeAuthFile_RelativeID(t *testing.T) {
	tempDir := t.TempDir()

	authData := map[string]any{"type": "claude"}
	data, _ := json.Marshal(authData)

	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}

	authFile := filepath.Join(tempDir, "my-auth.json")
	auths := SynthesizeAuthFile(ctx, authFile, data)
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth, got %d", len(auths))
	}

	// ID should be the full path
	if auths[0].ID != authFile {
		t.Errorf("expected ID %s, got %s", authFile, auths[0].ID)
	}
}

func TestSynthesizeAuthFile_PrefixValidation(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		wantPrefix string
	}{
		{"valid prefix", "myprefix", "myprefix"},
		{"prefix with slashes trimmed", "/myprefix/", "myprefix"},
		{"prefix with spaces trimmed", "  myprefix  ", "myprefix"},
		{"prefix with internal slash rejected", "my/prefix", ""},
		{"empty prefix", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			authData := map[string]any{
				"type":   "claude",
				"prefix": tt.prefix,
			}
			data, _ := json.Marshal(authData)

			ctx := &SynthesisContext{
				Config:      &config.Config{},
				Now:         time.Now(),
				IDGenerator: NewStableIDGenerator(),
			}

			authFile := filepath.Join(tempDir, "auth.json")
			auths := SynthesizeAuthFile(ctx, authFile, data)
			if len(auths) != 1 {
				t.Fatalf("expected 1 auth, got %d", len(auths))
			}
			if auths[0].Prefix != tt.wantPrefix {
				t.Errorf("expected prefix %q, got %q", tt.wantPrefix, auths[0].Prefix)
			}
		})
	}
}

func TestSynthesizeAuthFile_PriorityParsing(t *testing.T) {
	tests := []struct {
		name     string
		priority any
		want     string
		hasValue bool
	}{
		{
			name:     "string with spaces",
			priority: " 10 ",
			want:     "10",
			hasValue: true,
		},
		{
			name:     "number",
			priority: 8,
			want:     "8",
			hasValue: true,
		},
		{
			name:     "invalid string",
			priority: "1x",
			hasValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			authData := map[string]any{
				"type":     "claude",
				"priority": tt.priority,
			}
			data, _ := json.Marshal(authData)

			ctx := &SynthesisContext{
				Config:      &config.Config{},
				Now:         time.Now(),
				IDGenerator: NewStableIDGenerator(),
			}

			authFile := filepath.Join(tempDir, "auth.json")
			auths := SynthesizeAuthFile(ctx, authFile, data)
			if len(auths) != 1 {
				t.Fatalf("expected 1 auth, got %d", len(auths))
			}

			value, ok := auths[0].Attributes["priority"]
			if tt.hasValue {
				if !ok {
					t.Fatal("expected priority attribute to be set")
				}
				if value != tt.want {
					t.Fatalf("expected priority %q, got %q", tt.want, value)
				}
				return
			}
			if ok {
				t.Fatalf("expected priority attribute to be absent, got %q", value)
			}
		})
	}
}

func TestSynthesizeAuthFile_PerAuthExcludedModels(t *testing.T) {
	tempDir := t.TempDir()
	authData := map[string]any{
		"type":            "claude",
		"excluded_models": []string{"custom-model", "MODEL-B"},
	}
	data, _ := json.Marshal(authData)

	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Now(),
		IDGenerator: NewStableIDGenerator(),
	}

	authFile := filepath.Join(tempDir, "auth.json")
	auths := SynthesizeAuthFile(ctx, authFile, data)
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth, got %d", len(auths))
	}

	got := auths[0].Attributes["excluded_models_hash"]
	want := diff.ComputeExcludedModelsHash([]string{"custom-model", "model-b"})
	if got != want {
		t.Fatalf("expected excluded_models_hash %q, got %q", want, got)
	}
	if auths[0].Attributes["auth_kind"] != "oauth" {
		t.Fatalf("expected auth_kind=oauth, got %s", auths[0].Attributes["auth_kind"])
	}
}

func TestSynthesizeAuthFile_NoteParsing(t *testing.T) {
	tests := []struct {
		name     string
		note     any
		want     string
		hasValue bool
	}{
		{
			name:     "valid string note",
			note:     "hello world",
			want:     "hello world",
			hasValue: true,
		},
		{
			name:     "string note with whitespace",
			note:     "  trimmed note  ",
			want:     "trimmed note",
			hasValue: true,
		},
		{
			name:     "empty string note",
			note:     "",
			hasValue: false,
		},
		{
			name:     "whitespace only note",
			note:     "   ",
			hasValue: false,
		},
		{
			name:     "non-string note ignored",
			note:     12345,
			hasValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			authData := map[string]any{
				"type": "claude",
				"note": tt.note,
			}
			data, _ := json.Marshal(authData)

			ctx := &SynthesisContext{
				Config:      &config.Config{},
				Now:         time.Now(),
				IDGenerator: NewStableIDGenerator(),
			}

			authFile := filepath.Join(tempDir, "auth.json")
			auths := SynthesizeAuthFile(ctx, authFile, data)
			if len(auths) != 1 {
				t.Fatalf("expected 1 auth, got %d", len(auths))
			}

			value, ok := auths[0].Attributes["note"]
			if tt.hasValue {
				if !ok {
					t.Fatal("expected note attribute to be set")
				}
				if value != tt.want {
					t.Fatalf("expected note %q, got %q", tt.want, value)
				}
				return
			}
			if ok {
				t.Fatalf("expected note attribute to be absent, got %q", value)
			}
		})
	}
}
