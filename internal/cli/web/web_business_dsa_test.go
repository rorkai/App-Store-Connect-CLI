package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebBusinessDSAStatus(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })

	const response = `{"accountId":"private-account","traderDeclaration":"TRADER","sellerInfo":[{"id":"private-seller","verificationStatus":"VERIFIED","isEuCovered":false,"verificationStatusLocKey":"private-key","email":"private-contact@example.com","address":{"line":"private-address"}}]}`
	var responseBody string
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.Host != "appstoreconnect.apple.com" || req.URL.Path != "/v1/accounts/private-account/sellerInfo" || req.URL.RawQuery != "" {
			t.Errorf("request = %s %s", req.Method, req.URL)
		}
		_, _ = io.WriteString(w, responseBody)
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := customAppUsersRewriteTransport{target: target, base: server.Client().Transport}
	var sessionResolutions int
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		sessionResolutions++
		return &webcore.AuthSession{PublicProviderID: "private-account", Client: &http.Client{Transport: transport}}, "fixture", nil
	}

	for _, tc := range []struct {
		name, body, output, want, diagnostic, requestError string
		extra                                              []string
	}{
		{name: "sanitized JSON", body: response, output: "json", want: `{"traderDeclaration":"TRADER","sellerInfo":[{"verificationStatus":"VERIFIED","isEuCovered":false}]}`},
		{name: "table", body: response, output: "table", want: "VERIFIED"},
		{name: "malformed response", body: `{"traderDeclaration":"private-contact@example.com",`, output: "json", requestError: "DSA status returned a malformed response"},
		{name: "different account", body: strings.Replace(response, "private-account", "private-other-account", 1), output: "json", requestError: "DSA status response identifies another account"},
		{name: "missing declaration", body: `{"accountId":"private-account"}`, output: "json", requestError: "DSA status response is missing traderDeclaration"},
		{name: "invalid output before auth", output: "csv", diagnostic: "--output"},
		{name: "unexpected arguments before auth", output: "json", extra: []string{"stray"}, diagnostic: "unexpected positional arguments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			responseBody = tc.body
			beforeRequests, beforeSessions := requests, sessionResolutions
			d, stdout, stderr := newWebTestDeps()
			args := []string{"business", "dsa", "status", "--output", tc.output}
			err := NewWebCommand(d).ParseAndRun(context.Background(), append(args, tc.extra...))
			if tc.diagnostic != "" {
				if shared.ClassifyUsageError(err) == "" || !strings.Contains(stderr.String(), tc.diagnostic) || stdout.Len() != 0 || requests != beforeRequests || sessionResolutions != beforeSessions {
					t.Fatalf("error = %v, stderr = %q, stdout = %q, requests = %d, sessions = %d", err, stderr, stdout, requests-beforeRequests, sessionResolutions-beforeSessions)
				}
				return
			}
			if requests != beforeRequests+1 || sessionResolutions != beforeSessions+1 {
				t.Fatalf("requests = %d, sessions = %d", requests-beforeRequests, sessionResolutions-beforeSessions)
			}
			if tc.requestError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.requestError) || strings.Contains(err.Error(), "private-") || stdout.Len() != 0 {
					t.Fatalf("error = %v, stdout = %q", err, stdout)
				}
				return
			}
			got := strings.TrimSpace(stdout.String())
			if err != nil || stderr.Len() != 0 || strings.Contains(got, "private-") {
				t.Fatalf("error = %v, stderr = %q, stdout = %q", err, stderr, stdout)
			}
			if tc.output == "json" && got != tc.want || tc.output == "table" && (!strings.Contains(got, tc.want) || !strings.Contains(got, "TRADER") || !strings.Contains(got, "false")) {
				t.Fatalf("stdout = %q, want %q", got, tc.want)
			}
		})
	}
}
