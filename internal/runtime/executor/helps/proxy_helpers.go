package helps

import (
	"context"
	"net/http"
	"time"
)

// NewHTTPClient creates an HTTP client for upstream requests.
//
// If the context carries a "cliproxy.roundtripper" http.RoundTripper value
// (populated by an SDK RoundTripperProvider), it is used as the client
// transport; otherwise the default transport is used.
//
// Parameters:
//   - ctx: The context optionally carrying a RoundTripper
//   - timeout: The client timeout (0 means no timeout)
//
// Returns:
//   - *http.Client: An HTTP client configured with the context transport
func NewHTTPClient(ctx context.Context, timeout time.Duration) *http.Client {
	httpClient := &http.Client{}
	if timeout > 0 {
		httpClient.Timeout = timeout
	}

	if rt, ok := ctx.Value("cliproxy.roundtripper").(http.RoundTripper); ok && rt != nil {
		httpClient.Transport = rt
	}

	return httpClient
}
