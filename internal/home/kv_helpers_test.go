package home

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestHashKeyPart(t *testing.T) {
	first := HashKeyPart("secret-value")
	again := HashKeyPart("secret-value")
	other := HashKeyPart("other-value")
	if first == "" || len(first) != 64 {
		t.Fatalf("HashKeyPart() = %q, want 64 hex chars", first)
	}
	if first != again {
		t.Fatalf("HashKeyPart() is not stable")
	}
	if first == other {
		t.Fatalf("HashKeyPart() returned same hash for different inputs")
	}
	if strings.Contains(first, "secret") || strings.Contains(first, "value") {
		t.Fatalf("HashKeyPart() leaked input: %q", first)
	}
}

func TestCurrentKVClientUnavailableErrors(t *testing.T) {
	t.Cleanup(ClearCurrent)

	disabled := New(config.HomeConfig{Enabled: false})
	SetCurrent(disabled)
	if _, homeMode, errClient := CurrentKVClient(); !homeMode || errClient == nil {
		t.Fatalf("CurrentKVClient(disabled) = homeMode %v err %v, want true error", homeMode, errClient)
	}

	notReady := New(config.HomeConfig{Enabled: true, Host: "127.0.0.1", Port: 1})
	SetCurrent(notReady)
	if _, homeMode, errClient := CurrentKVClient(); !homeMode || errClient == nil {
		t.Fatalf("CurrentKVClient(no heartbeat) = homeMode %v err %v, want true error", homeMode, errClient)
	}
}
