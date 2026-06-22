package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	homekv "github.com/router-for-me/CLIProxyAPI/v7/internal/home"
)

// resetAntigravityCreditsRetryState clears all package-level antigravity credits retry state.
// Intended for test isolation.
func resetAntigravityCreditsRetryState() {
	antigravityCreditsFailureByAuth = sync.Map{}
	antigravityShortCooldownByAuth = sync.Map{}
	antigravityCreditsBalanceByAuth = sync.Map{}
	antigravityCreditsHintRefreshByID = sync.Map{}
}

type fakeAntigravityKVClient struct {
	values       map[string][]byte
	getErr       error
	setErr       error
	setNXErr     error
	delErr       error
	setNXResult  bool
	getCount     int
	setCount     int
	setNXCount   int
	delCount     int
	lastSetTTL   time.Duration
	lastSetNXTTL time.Duration
	lastSetNXKey string
	lastSetKey   string
}

func newFakeAntigravityKVClient() *fakeAntigravityKVClient {
	return &fakeAntigravityKVClient{
		values:      make(map[string][]byte),
		setNXResult: true,
	}
}

func (c *fakeAntigravityKVClient) KVGet(_ context.Context, key string) ([]byte, bool, error) {
	c.getCount++
	if c.getErr != nil {
		return nil, false, c.getErr
	}
	value, ok := c.values[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (c *fakeAntigravityKVClient) KVSet(_ context.Context, key string, value []byte, opts homekv.KVSetOptions) (bool, error) {
	c.setCount++
	c.lastSetKey = key
	c.lastSetTTL = opts.EX
	if c.setErr != nil {
		return false, c.setErr
	}
	c.values[key] = append([]byte(nil), value...)
	return true, nil
}

func (c *fakeAntigravityKVClient) KVSetNX(_ context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	c.setNXCount++
	c.lastSetNXKey = key
	c.lastSetNXTTL = ttl
	if c.setNXErr != nil {
		return false, c.setNXErr
	}
	if _, ok := c.values[key]; ok {
		return false, nil
	}
	if c.setNXResult {
		c.values[key] = append([]byte(nil), value...)
		return true, nil
	}
	return false, nil
}

func (c *fakeAntigravityKVClient) KVDel(_ context.Context, keys ...string) (int64, error) {
	c.delCount++
	if c.delErr != nil {
		return 0, c.delErr
	}
	var deleted int64
	for _, key := range keys {
		if _, ok := c.values[key]; ok {
			delete(c.values, key)
			deleted++
		}
	}
	return deleted, nil
}

func useFakeAntigravityKVClient(t *testing.T, client *fakeAntigravityKVClient, homeMode bool, errClient error) {
	t.Helper()
	previous := currentAntigravityKVClient
	currentAntigravityKVClient = func() (antigravityKVClient, bool, error) {
		return client, homeMode, errClient
	}
	t.Cleanup(func() {
		currentAntigravityKVClient = previous
	})
}

func mustAntigravityJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, errMarshal := json.Marshal(value)
	if errMarshal != nil {
		t.Fatalf("marshal value: %v", errMarshal)
	}
	return raw
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
