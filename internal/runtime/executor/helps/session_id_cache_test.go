package helps

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func resetSessionIDCache() {
	sessionIDCacheMu.Lock()
	sessionIDCache = make(map[string]sessionIDCacheEntry)
	sessionIDCacheMu.Unlock()
}

func TestCachedSessionIDRequiredEmptyAPIKeyReturnsFreshUUID(t *testing.T) {
	value, errValue := CachedSessionIDRequired(context.Background(), "")
	if errValue != nil {
		t.Fatalf("CachedSessionIDRequired(empty) error = %v", errValue)
	}
	if _, errParse := uuid.Parse(value); errParse != nil {
		t.Fatalf("session id %q is not a UUID: %v", value, errParse)
	}
}

func TestCachedSessionIDRequiredReusesLocalCache(t *testing.T) {
	resetSessionIDCache()

	first, errFirst := CachedSessionIDRequired(context.Background(), "api-key-1")
	if errFirst != nil {
		t.Fatalf("CachedSessionIDRequired() first error = %v", errFirst)
	}
	second, errSecond := CachedSessionIDRequired(context.Background(), "api-key-1")
	if errSecond != nil {
		t.Fatalf("CachedSessionIDRequired() second error = %v", errSecond)
	}
	if first != second {
		t.Fatalf("session id = %q then %q, want local cache reuse", first, second)
	}
	if _, errParse := uuid.Parse(first); errParse != nil {
		t.Fatalf("session id %q is not a UUID: %v", first, errParse)
	}
}

func TestCachedSessionIDRequiredDistinctKeysAreDistinct(t *testing.T) {
	resetSessionIDCache()

	a, errA := CachedSessionIDRequired(context.Background(), "api-key-a")
	if errA != nil {
		t.Fatalf("CachedSessionIDRequired() a error = %v", errA)
	}
	b, errB := CachedSessionIDRequired(context.Background(), "api-key-b")
	if errB != nil {
		t.Fatalf("CachedSessionIDRequired() b error = %v", errB)
	}
	if a == b {
		t.Fatalf("expected distinct session ids for distinct api keys, both = %q", a)
	}
}
