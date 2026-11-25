package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsVietnamLicenseView(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })
	const response = `{"data":{"type":"vietnamGameLicenseDetails","id":"license-1","attributes":{"license":null,"licenseUrl":null},"links":{"self":"detail"}},"links":{"self":"detail"},"meta":{"futureField":true}}`
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/app-1/vietnamGameLicenseDetail" || req.URL.RawQuery != "" || req.ContentLength > 0 {
			t.Errorf("unexpected request: %s %s, body length %d", req.Method, req.URL, req.ContentLength)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resolutions := 0
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolutions++
		return &webcore.AuthSession{Client: &http.Client{Transport: customAppUsersRewriteTransport{target: target, base: server.Client().Transport}}}, "fixture", nil
	}
	args := []string{"apps", "compliance", "vietnam-license", "view"}
	d, stdout, stderr := newWebTestDeps()
	err = NewWebCommand(d).ParseAndRun(context.Background(), append(args, "--app", "app-1", "--output", "json"))
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("view: error = %v, stderr = %q", err, stderr)
	}
	var got, want any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v, %s", err, stdout)
	}
	if err := json.Unmarshal([]byte(response), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("output = %s, want complete Apple envelope %s", stdout, response)
	}
	d, stdout, stderr = newWebTestDeps()
	err = NewWebCommand(d).ParseAndRun(context.Background(), append(args, "--app", "app-1", "--output", "markdown"))
	if err != nil || stderr.Len() != 0 || !strings.Contains(stdout.String(), "license_url") || !strings.Contains(stdout.String(), "license-1") {
		t.Fatalf("markdown: error = %v, stderr = %q, stdout = %q", err, stderr, stdout)
	}
	d, stdout, stderr = newWebTestDeps()
	err = NewWebCommand(d).ParseAndRun(context.Background(), args)
	if err == nil || !strings.Contains(stderr.String(), "--app is required") || stdout.Len() != 0 || requests != 2 || resolutions != 2 {
		t.Fatalf("missing app: error = %v, stderr = %q, stdout = %q, requests = %d, resolutions = %d", err, stderr, stdout, requests, resolutions)
	}
}
