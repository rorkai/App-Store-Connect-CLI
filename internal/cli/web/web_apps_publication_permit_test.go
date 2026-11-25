package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsChinaPublicationPermitView(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })
	response := `{"data":{"type":"apps","id":"app-1","relationships":{"appPublicationPermit":{"data":null,"links":{"related":"https://appstoreconnect.apple.com/iris/v1/apps/app-1/appPublicationPermit"}}}},"links":{"self":"https://appstoreconnect.apple.com/iris/v1/apps/app-1"},"meta":{"unknown":true}}`
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		query := url.Values{"fields[apps]": {"appPublicationPermit"}, "include": {"appPublicationPermit"}}
		if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/app-1" || req.URL.Query().Encode() != query.Encode() {
			t.Errorf("request = %s %s, want observed publication permit GET", req.Method, req.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := customAppUsersRewriteTransport{target: target, base: server.Client().Transport}
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "fixture", nil
	}
	for _, status := range []string{"absent", "present"} {
		if status == "present" {
			response = strings.Replace(response, `"data":null`, `"data":{"type":"appPublicationPermits","id":"permit-1"}`, 1)
		}
		for _, output := range []string{"json", "markdown"} {
			d, stdout, stderr := newWebTestDeps()
			err := NewWebCommand(d).ParseAndRun(context.Background(), []string{"apps", "compliance", "china-publication-permit", "view", "--app", "app-1", "--output", output})
			if err != nil || stderr.Len() != 0 {
				t.Fatalf("error=%v stderr=%q", err, stderr)
			}
			if output == "json" {
				var got, want any
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(response), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("response envelope changed: %s", stdout)
				}
			} else if !strings.Contains(strings.Join(strings.Fields(stdout.String()), " "), "| app-1 | "+status+" |") {
				t.Fatalf("markdown=%q", stdout)
			}
		}
	}
	for _, tc := range []struct {
		args       []string
		diagnostic string
	}{
		{nil, "--app is required"},
		{[]string{"--app", "app-1", "--output", "csv"}, "--output"},
		{[]string{"--app", "app-1", "stray"}, "unexpected positional arguments"},
	} {
		d, stdout, stderr := newWebTestDeps()
		args := append([]string{"apps", "compliance", "china-publication-permit", "view"}, tc.args...)
		err = NewWebCommand(d).ParseAndRun(context.Background(), args)
		if err == nil || !strings.Contains(stderr.String(), tc.diagnostic) || stdout.Len() != 0 || requests != 4 {
			t.Fatalf("invalid args %v: error=%v stderr=%q stdout=%q requests=%d", tc.args, err, stderr, stdout, requests)
		}
	}
	response = `{"data":{"type":"apps","id":"app-1","relationships":{}}}`
	d, stdout, _ := newWebTestDeps()
	err = NewWebCommand(d).ParseAndRun(context.Background(), []string{"apps", "compliance", "china-publication-permit", "view", "--app", "app-1"})
	if err == nil || !strings.Contains(err.Error(), "missing app or permit relationship") || stdout.Len() != 0 {
		t.Fatalf("missing relationship: error=%v stdout=%q", err, stdout)
	}
}
