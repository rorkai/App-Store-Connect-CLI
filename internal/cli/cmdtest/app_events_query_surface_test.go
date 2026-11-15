package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type appEventsListQueryCapture struct {
	calls int
	path  string
	query url.Values
}

func appEventsListQueryStub(t *testing.T) (*appEventsListQueryCapture, func() (*asc.Client, error)) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	captured := &appEventsListQueryCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured.calls++
		captured.path = req.URL.Path
		captured.query = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.appstoreconnect.apple.com" {
			t.Errorf("request URL = %s, want official App Store Connect host", req.URL.String())
		}
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return captured, func() (*asc.Client, error) {
		return client, nil
	}
}

func TestAppEventsListQuerySurfaceEmitsSupportedParameters(t *testing.T) {
	captured, client := appEventsListQueryStub(t)

	stdout, stderr, err := runRootWithDeps(
		t, client,
		"app-events", "list",
		"--app", "app-123",
		"--event-state", "approved,archived",
		"--id", "event-1,event-2",
		"--fields", "referenceName,eventState,localizations",
		"--localization-fields", "locale,name",
		"--include", "localizations",
		"--localizations-limit", "10",
		"--limit", "25",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if captured.path != "/v1/apps/app-123/appEvents" {
		t.Fatalf("path = %q, want /v1/apps/app-123/appEvents", captured.path)
	}
	want := url.Values{
		"filter[eventState]":            {"APPROVED,ARCHIVED"},
		"filter[id]":                    {"event-1,event-2"},
		"fields[appEvents]":             {"referenceName,eventState,localizations"},
		"fields[appEventLocalizations]": {"locale,name"},
		"include":                       {"localizations"},
		"limit[localizations]":          {"10"},
		"limit":                         {"25"},
	}
	if got := captured.query.Encode(); got != want.Encode() {
		t.Fatalf("query = %q, want %q", got, want.Encode())
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("stdout = %q, want empty data envelope", stdout)
	}
}

func TestAppEventsListQuerySurfaceRejectsInvalidValuesBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "event state",
			args: []string{"--event-state", "NOT_A_STATE"},
			want: "--event-state",
		},
		{
			name: "event state explicitly empty",
			args: []string{"--event-state", ""},
			want: "--event-state must not be empty",
		},
		{
			name: "event state contains empty value",
			args: []string{"--event-state", "APPROVED,"},
			want: "--event-state must not contain empty values",
		},
		{
			name: "id explicitly empty",
			args: []string{"--id", ""},
			want: "--id must not be empty",
		},
		{
			name: "id contains empty value",
			args: []string{"--id", "event-1,"},
			want: "--id must not contain empty values",
		},
		{
			name: "event fields",
			args: []string{"--fields", "notAField"},
			want: "--fields",
		},
		{
			name: "event fields explicitly empty",
			args: []string{"--fields", "  "},
			want: "--fields must not be empty",
		},
		{
			name: "event fields contains empty value",
			args: []string{"--fields", "referenceName,,eventState"},
			want: "--fields must not contain empty values",
		},
		{
			name: "localization fields",
			args: []string{"--localization-fields", "notAField"},
			want: "--localization-fields",
		},
		{
			name: "localization fields explicitly empty",
			args: []string{"--localization-fields", ""},
			want: "--localization-fields must not be empty",
		},
		{
			name: "localization fields contains empty value",
			args: []string{"--localization-fields", "locale,,name"},
			want: "--localization-fields must not contain empty values",
		},
		{
			name: "include",
			args: []string{"--include", "notARelationship"},
			want: "--include",
		},
		{
			name: "include explicitly empty",
			args: []string{"--include", ""},
			want: "--include must not be empty",
		},
		{
			name: "include contains empty value",
			args: []string{"--include", "localizations,"},
			want: "--include must not contain empty values",
		},
		{
			name: "localization limit",
			args: []string{"--localizations-limit", "51"},
			want: "--localizations-limit must be between 1 and 50",
		},
		{
			name: "explicit zero localization limit",
			args: []string{"--localizations-limit", "0"},
			want: "--localizations-limit must be between 1 and 50",
		},
		{
			name: "localization fields require include",
			args: []string{"--localization-fields", "locale"},
			want: "--localization-fields requires --include localizations",
		},
		{
			name: "localization limit requires include",
			args: []string{"--localizations-limit", "10"},
			want: "--localizations-limit requires --include localizations",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			stdout, stderr, code := runWithDeps(t, func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}, append([]string{"app-events", "list", "--app", "app-123"}, test.args...)...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if clientFactoryCalled {
				t.Fatal("client factory ran before query validation")
			}
		})
	}
}

func TestAppEventsListQuerySurfaceRejectsNextConflictsBeforeAuth(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-123/appEvents?cursor=next"
	tests := []struct {
		name  string
		flag  string
		value string
	}{
		{name: "event state", flag: "--event-state", value: "APPROVED"},
		{name: "id", flag: "--id", value: "event-1"},
		{name: "fields", flag: "--fields", value: "referenceName"},
		{name: "localization fields", flag: "--localization-fields", value: "locale"},
		{name: "include", flag: "--include", value: "localizations"},
		{name: "localization limit", flag: "--localizations-limit", value: "10"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			args := []string{"app-events", "list", "--next", nextURL, test.flag, test.value}
			stdout, stderr, code := runWithDeps(t, func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}, args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "app-events list: --next cannot be combined with " + test.flag
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestAppEventsListQuerySurfaceAllowsAppWithNextAndUsesOpaqueURL(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-123/appEvents?cursor=opaque&limit=42&unexpected=kept"
	captured, client := appEventsListQueryStub(t)

	stdout, stderr, err := runRootWithDeps(
		t, client,
		"app-events", "list",
		"--app", "app-123",
		"--next", nextURL,
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if captured.calls != 1 {
		t.Fatalf("request count = %d, want 1", captured.calls)
	}
	if captured.path != "/v1/apps/app-123/appEvents" {
		t.Fatalf("path = %q, want /v1/apps/app-123/appEvents", captured.path)
	}
	want := url.Values{
		"cursor":     {"opaque"},
		"limit":      {"42"},
		"unexpected": {"kept"},
	}
	if got := captured.query.Encode(); got != want.Encode() {
		t.Fatalf("query = %q, want opaque next query %q", got, want.Encode())
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("stdout = %q, want response from next URL", stdout)
	}
}
