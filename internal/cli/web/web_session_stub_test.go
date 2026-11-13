package web

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebSessionTransport(t *testing.T, transport roundTripFunc) {
	t.Helper()
	original := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = original })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}
}

func webJSONResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
