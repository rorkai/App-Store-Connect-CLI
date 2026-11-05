package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebBundleIDCapabilitiesSyncAppClipValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing bundle id", args: []string{"--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm"}, wantErr: "--bundle-id is required"},
		{name: "missing parent bundle id", args: []string{"--bundle-id", "clip-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm"}, wantErr: "--parent-bundle-id is required"},
		{name: "missing capability", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--confirm"}, wantErr: "--capability is required"},
		{name: "invalid settings json", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--settings-json", `{"key":"BAD"}`, "--confirm"}, wantErr: "--settings-json must be a JSON array"},
		{name: "null settings json", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--settings-json", `null`, "--confirm"}, wantErr: "--settings-json must be a JSON array"},
		{name: "multiple settings json values", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--settings-json", `[] []`, "--confirm"}, wantErr: "--settings-json must be a JSON array"},
		{name: "missing confirm", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS"}, wantErr: "Error: --confirm is required"},
		{name: "explicit confirm false", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm=false"}, wantErr: "Error: --confirm is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

// TestWebBundleIDCapabilitiesSyncAppClipWithoutConfirmFailsBeforeAnyRequest is
// the transition test for the --confirm requirement: the pre-existing
// invocation shape still parses, but it must fail with migration guidance
// before a web session is resolved or any HTTP request is attempted.
func TestWebBundleIDCapabilitiesSyncAppClipWithoutConfirmFailsBeforeAnyRequest(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSync := syncAppClipBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		syncAppClipBundleIDCapabilityFn = origSync
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run when --confirm is missing")
		return nil, "", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		t.Fatal("web client must not be created when --confirm is missing")
		return nil
	}
	syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		t.Fatal("sync must not run when --confirm is missing")
		return nil, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("session must not be persisted when --confirm is missing")
		return nil
	}

	cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--bundle-id", "clip-1",
		"--parent-bundle-id", "parent-1",
		"--capability", "PUSH_NOTIFICATIONS",
		"--output", "json",
	}); err != nil {
		t.Fatalf("legacy invocation without --confirm must still parse, got %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, webBundleIDSyncAppClipConfirmMigrationWarning) {
		t.Fatalf("expected migration warning in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("expected --confirm usage error in stderr, got %q", stderr)
	}
	if strings.Index(stderr, "Warning:") > strings.Index(stderr, "Error:") {
		t.Fatalf("expected migration warning before the usage error, got %q", stderr)
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipCallsPrivateSync(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSync := syncAppClipBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		syncAppClipBundleIDCapabilityFn = origSync
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error { return nil }

	var gotReq webcore.AppClipBundleIDCapabilitySyncRequest
	syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		gotReq = req
		return &webcore.AppClipBundleIDCapabilitySyncResult{
			BundleID:       req.BundleID,
			ParentBundleID: req.ParentBundleID,
			Capability:     req.Capability,
			Enabled:        req.Enabled,
			Changed:        true,
			Status:         "synced",
		}, nil
	}

	cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--bundle-id", "clip-1",
		"--parent-bundle-id", "parent-1",
		"--capability", "push_notifications",
		"--settings-json", `[{"key":"PUSH_NOTIFICATION_FEATURES","options":[{"key":"PUSH_NOTIFICATION_FEATURE_BROADCAST","enabled":true}]}]`,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stderr, "Warning:") || !strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("expected provisioning profile invalidation warning after a real write, got %q", stderr)
	}
	var receipt struct {
		BundleID       string `json:"bundleId"`
		ParentBundleID string `json:"parentBundleId"`
		Capability     string `json:"capability"`
		Enabled        bool   `json:"enabled"`
		Changed        bool   `json:"changed"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout is not a JSON receipt: %v; stdout=%q", err, stdout)
	}
	if receipt.BundleID != "clip-1" || receipt.ParentBundleID != "parent-1" || receipt.Capability != "PUSH_NOTIFICATIONS" || !receipt.Enabled || !receipt.Changed || receipt.Status != "synced" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if gotReq.BundleID != "clip-1" || gotReq.ParentBundleID != "parent-1" || gotReq.Capability != "PUSH_NOTIFICATIONS" {
		t.Fatalf("unexpected sync request: %+v", gotReq)
	}
	if !gotReq.SettingsProvided {
		t.Fatal("expected settings to be marked as explicitly provided")
	}
	if len(gotReq.Settings) != 1 || gotReq.Settings[0].Key != "PUSH_NOTIFICATION_FEATURES" {
		t.Fatalf("expected parsed settings, got %+v", gotReq.Settings)
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipAlreadySyncedReceiptHasNoInvalidationWarning(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSync := syncAppClipBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		syncAppClipBundleIDCapabilityFn = origSync
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(session *webcore.AuthSession) error { return nil }
	syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		return &webcore.AppClipBundleIDCapabilitySyncResult{
			BundleID:       req.BundleID,
			ParentBundleID: req.ParentBundleID,
			Capability:     req.Capability,
			Enabled:        true,
			Changed:        false,
			Status:         "already-synced",
		}, nil
	}

	for _, output := range []string{"json", "table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
			if err := cmd.FlagSet.Parse([]string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm", "--output", output}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected no warning when nothing changed, got %q", stderr)
			}
			switch output {
			case "json":
				if !strings.Contains(stdout, `"changed":false`) || !strings.Contains(stdout, `"status":"already-synced"`) {
					t.Fatalf("unexpected json receipt: %q", stdout)
				}
			default:
				if !strings.Contains(stdout, "Changed") || !strings.Contains(stdout, "false") || !strings.Contains(stdout, "already-synced") {
					t.Fatalf("expected Changed/Status columns in %s output, got %q", output, stdout)
				}
			}
		})
	}
}

// TestWebBundleIDCapabilitiesSyncAppClipPersistsRefreshedSession drives the
// real web client against an in-process transport so refreshed Set-Cookie
// values from the mutation land in the session jar that gets persisted.
func TestWebBundleIDCapabilitiesSyncAppClipPersistsRefreshedSession(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		persistWebSessionFn = origPersist
	})
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "1ms")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	irisURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(irisURL, []*http.Cookie{{Name: "myacinfo", Value: "stale", Path: "/", Domain: ".apple.com", Secure: true}})

	var patchCount int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		header := http.Header{"Content-Type": []string{"application/json"}}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/bundleIds/clip-1":
			return &http.Response{StatusCode: http.StatusOK, Header: header, Request: r, Body: io.NopCloser(strings.NewReader(`{"data":{"id":"clip-1","type":"bundleIds","attributes":{"name":"Clip","identifier":"com.example.app.Clip"}}}`))}, nil
		case r.Method == http.MethodPatch && r.URL.Path == "/iris/v1/bundleIds/clip-1":
			patchCount++
			if got := r.Header.Get("Cookie"); !strings.Contains(got, "myacinfo=stale") {
				t.Errorf("expected cached cookie on PATCH, got %q", got)
			}
			header.Add("Set-Cookie", "myacinfo=refreshed; Path=/; Domain=.apple.com; Secure")
			return &http.Response{StatusCode: http.StatusOK, Header: header, Request: r, Body: io.NopCloser(strings.NewReader(`{"data":{"id":"clip-1","type":"bundleIds"}}`))}, nil
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			return &http.Response{StatusCode: http.StatusNotFound, Header: header, Request: r, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
	})
	session := &webcore.AuthSession{UserEmail: "user@example.com", Client: &http.Client{Jar: jar, Transport: transport}}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return session, "cache", nil
	}
	newWebClientFn = webcore.NewClient

	var persisted []*webcore.AuthSession
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = append(persisted, session)
		return nil
	}

	cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
	if err := cmd.FlagSet.Parse([]string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if patchCount != 1 {
		t.Fatalf("expected one PATCH, got %d", patchCount)
	}
	if !strings.Contains(stdout, `"changed":true`) {
		t.Fatalf("expected changed receipt, got %q", stdout)
	}
	if len(persisted) != 1 || persisted[0] != session {
		t.Fatalf("expected the resolved session to be persisted exactly once after the mutation, got %d persist calls", len(persisted))
	}
	var refreshed string
	for _, cookie := range persisted[0].Client.Jar.Cookies(irisURL) {
		if cookie.Name == "myacinfo" {
			refreshed = cookie.Value
		}
	}
	if refreshed != "refreshed" {
		t.Fatalf("expected persisted jar to hold the refreshed myacinfo cookie, got %q", refreshed)
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipWarnsWhenSessionPersistFails(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSync := syncAppClipBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		syncAppClipBundleIDCapabilityFn = origSync
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		return &webcore.AppClipBundleIDCapabilitySyncResult{BundleID: req.BundleID, ParentBundleID: req.ParentBundleID, Capability: req.Capability, Enabled: true, Changed: false, Status: "already-synced"}, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error { return errors.New("disk full") }

	cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
	if err := cmd.FlagSet.Parse([]string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("persist failure must not fail the command, got %v", err)
		}
	})
	if !strings.Contains(stdout, `"status":"already-synced"`) {
		t.Fatalf("expected receipt on stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "failed to persist refreshed web session") || !strings.Contains(stderr, "disk full") {
		t.Fatalf("expected persistence warning on stderr, got %q", stderr)
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipDoesNotPersistSessionWhenSyncFails(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSync := syncAppClipBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		syncAppClipBundleIDCapabilityFn = origSync
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		return nil, errors.New("boom")
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("session must not be persisted when the sync fails")
		return nil
	}

	cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
	if err := cmd.FlagSet.Parse([]string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, _ = captureWebCommandOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected sync error, got %v", err)
		}
	})
}

func TestWebBundleIDCapabilitiesEnableValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing bundle id", args: []string{"--capability", "PRIVATE_CLOUD_COMPUTE", "--confirm"}, wantErr: "--bundle-id is required"},
		{name: "missing capability", args: []string{"--bundle-id", "bundle-1", "--confirm"}, wantErr: "--capability is required"},
		{name: "missing confirm", args: []string{"--bundle-id", "bundle-1", "--capability", "PRIVATE_CLOUD_COMPUTE"}, wantErr: "--confirm is required"},
		{name: "unsupported capability", args: []string{"--bundle-id", "bundle-1", "--capability", "ICLOUD", "--confirm"}, wantErr: "unsupported Developer Portal capability"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebBundleIDCapabilitiesEnableCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestWebBundleIDCapabilitiesRegistersDisableWithSharedFlags(t *testing.T) {
	var disable *ffcli.Command
	for _, subcommand := range WebBundleIDCapabilitiesCommand().Subcommands {
		if subcommand.Name == "disable" {
			disable = subcommand
			break
		}
	}
	if disable == nil {
		t.Fatal("capabilities command did not register disable")
	}
	for _, name := range []string{"bundle-id", "capability", "confirm", "developer-team", "output", "pretty"} {
		if disable.FlagSet.Lookup(name) == nil {
			t.Fatalf("disable command missing --%s", name)
		}
	}
}

func TestWebBundleIDCapabilitiesDisableValidatesBeforeSession(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run for invalid disable input")
		return nil, "", nil
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing bundle id", args: []string{"--capability", "PRIVATE_CLOUD_COMPUTE", "--confirm"}, want: "--bundle-id is required"},
		{name: "missing capability", args: []string{"--bundle-id", "bundle-1", "--confirm"}, want: "--capability is required"},
		{name: "missing confirm", args: []string{"--bundle-id", "bundle-1", "--capability", "PRIVATE_CLOUD_COMPUTE"}, want: "--confirm is required"},
		{name: "unsupported capability", args: []string{"--bundle-id", "bundle-1", "--capability", "ICLOUD", "--confirm"}, want: "unsupported Developer Portal capability"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebBundleIDCapabilitiesDisableCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tc.want)
			}
		})
	}
}

func TestWebBundleIDCapabilitiesDisablePrintsReceiptAndWarning(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origDisable := disableDeveloperBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		disableDeveloperBundleIDCapabilityFn = origDisable
		persistWebSessionFn = origPersist
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	disableDeveloperBundleIDCapabilityFn = func(_ context.Context, _ *webcore.Client, req webcore.DeveloperBundleIDCapabilityDisableRequest) (*asc.DeveloperBundleIDCapabilityDisableResult, error) {
		return &asc.DeveloperBundleIDCapabilityDisableResult{BundleID: req.BundleID, Capability: req.Capability, Changed: true, Status: "disabled"}, nil
	}

	cmd := WebBundleIDCapabilitiesDisableCommand()
	if err := cmd.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--capability", "private_cloud_compute", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"status":"disabled"`) || !strings.Contains(stdout, `"changed":true`) {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("stderr missing profile warning: %q", stderr)
	}
}

func TestWebBundleIDCapabilitiesDisableUnverifiedPersistsAndPrintsNoReceipt(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origDisable := disableDeveloperBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		disableDeveloperBundleIDCapabilityFn = origDisable
		persistWebSessionFn = origPersist
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	disableDeveloperBundleIDCapabilityFn = func(context.Context, *webcore.Client, webcore.DeveloperBundleIDCapabilityDisableRequest) (*asc.DeveloperBundleIDCapabilityDisableResult, error) {
		return nil, &webcore.DeveloperBundleIDCapabilityUnverifiedError{Err: errors.New("write may have been applied; no automatic retry was sent")}
	}
	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}

	cmd := WebBundleIDCapabilitiesDisableCommand()
	if err := cmd.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--capability", "PRIVATE_CLOUD_COMPUTE", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, stderr := captureWebCommandOutput(t, func() { runErr = cmd.Exec(context.Background(), nil) })
	if runErr == nil || !strings.Contains(runErr.Error(), "no automatic retry was sent") {
		t.Fatalf("error = %v, want unverified diagnostic", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected no success receipt on an unverified write, got %q", stdout)
	}
	if !strings.Contains(stderr, "may have changed the App ID") || !strings.Contains(stderr, "can invalidate existing provisioning profiles") {
		t.Fatalf("stderr missing possible-change warning: %q", stderr)
	}
	if persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1", persistCalls)
	}
}

func TestWebBundleIDCapabilitiesEnablePersistsTeamWhenPortalResponseIsAmbiguous(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origEnable := enableDeveloperBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		enableDeveloperBundleIDCapabilityFn = origEnable
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	enableDeveloperBundleIDCapabilityFn = func(context.Context, *webcore.Client, webcore.DeveloperBundleIDCapabilityEnableRequest) (*webcore.DeveloperBundleIDCapabilityEnableResult, error) {
		return nil, errors.New("failed to read Developer Portal capability response")
	}
	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}

	cmd := WebBundleIDCapabilitiesEnableCommand()
	if err := cmd.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--capability", "PRIVATE_CLOUD_COMPUTE", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, _ = captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "failed to read Developer Portal capability response") {
		t.Fatalf("expected the ambiguous enable error to propagate, got %v", runErr)
	}
	if persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1 so a later retry without --developer-team still targets the team that may have enabled the capability", persistCalls)
	}
}

func TestWebBundleIDCapabilitiesEnableCallsDeveloperPortalClient(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origEnable := enableDeveloperBundleIDCapabilityFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		enableDeveloperBundleIDCapabilityFn = origEnable
		persistWebSessionFn = origPersist
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		return nil
	}

	var gotReq webcore.DeveloperBundleIDCapabilityEnableRequest
	enableDeveloperBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.DeveloperBundleIDCapabilityEnableRequest) (*webcore.DeveloperBundleIDCapabilityEnableResult, error) {
		gotReq = req
		return &webcore.DeveloperBundleIDCapabilityEnableResult{
			BundleID:   req.BundleID,
			Capability: req.Capability,
			Enabled:    true,
			Changed:    true,
			Status:     "enabled",
		}, nil
	}

	cmd := WebBundleIDCapabilitiesEnableCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--bundle-id", "bundle-1",
		"--capability", "private_cloud_compute",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"capability":"PRIVATE_CLOUD_COMPUTE"`) || !strings.Contains(stdout, `"status":"enabled"`) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if gotReq.BundleID != "bundle-1" || gotReq.Capability != "PRIVATE_CLOUD_COMPUTE" {
		t.Fatalf("unexpected enable request: %+v", gotReq)
	}
}
