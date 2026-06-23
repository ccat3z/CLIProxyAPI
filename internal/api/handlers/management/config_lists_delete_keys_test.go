package management

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func writeTestConfigFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if errWrite := os.WriteFile(path, []byte("{}\n"), 0o600); errWrite != nil {
		t.Fatalf("failed to write test config: %v", errWrite)
	}
	return path
}

func TestDeleteClaudeKey_DeletesEmptyBaseURLWhenExplicitlyProvided(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			ClaudeKey: []config.ClaudeKey{
				{APIKey: "shared-key", BaseURL: ""},
				{APIKey: "shared-key", BaseURL: "https://claude.example.com"},
			},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodDelete, "/v0/management/claude-api-key?api-key=shared-key&base-url=", nil)

	h.DeleteClaudeKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := len(h.cfg.ClaudeKey); got != 1 {
		t.Fatalf("claude keys len = %d, want 1", got)
	}
	if got := h.cfg.ClaudeKey[0].BaseURL; got != "https://claude.example.com" {
		t.Fatalf("remaining base-url = %q, want %q", got, "https://claude.example.com")
	}
}
