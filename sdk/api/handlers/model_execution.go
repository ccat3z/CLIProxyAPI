package handlers

import (
	"net/http"
	"net/url"

	"golang.org/x/net/context"
)

type modelExecutionOptions struct {
	Headers http.Header
	Query   url.Values
}

func modelExecutionResponseProtocol(entryProtocol, exitProtocol string) string {
	if exitProtocol == "" {
		return entryProtocol
	}
	return exitProtocol
}

func modelExecutionHeaders(ctx context.Context, headers http.Header) http.Header {
	if len(headers) > 0 {
		return cloneHeader(headers)
	}
	return headersFromContext(ctx)
}

// modelExecutionQuery prefers an explicitly provided query and otherwise falls
// back to the inbound query embedded in the request context. This lets callers
// observe query parameters for plain HTTP requests even when they do not
// populate execOptions.Query (mirrors modelExecutionHeaders).
func modelExecutionQuery(ctx context.Context, query url.Values) url.Values {
	if len(query) > 0 {
		return cloneURLValues(query)
	}
	return queryFromContext(ctx)
}

func cloneURLValues(src url.Values) url.Values {
	if src == nil {
		return nil
	}
	dst := make(url.Values, len(src))
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	return dst
}
