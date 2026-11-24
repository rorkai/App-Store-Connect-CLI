package asc

import (
	"context"
	"net/http"
)

type noRedirectReadsContextKey struct{}

// WithNoRedirectReads prevents Client.do GETs from following redirects.
func WithNoRedirectReads(ctx context.Context) context.Context {
	return context.WithValue(ctx, noRedirectReadsContextKey{}, true)
}

func noRedirectReads(ctx context.Context) bool {
	value, _ := ctx.Value(noRedirectReadsContextKey{}).(bool)
	return value
}

func clientWithoutRedirects(client *http.Client) *http.Client {
	if client == nil {
		client = newDefaultHTTPClient(ResolveTimeout())
	}
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		// A redirect can disclose a signed URL through Referer and 307/308 can
		// replay an upload body to an attacker-controlled destination.
		return http.ErrUseLastResponse
	}
	return &safeClient
}
