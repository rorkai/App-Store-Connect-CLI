package web

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebBundleIDCapabilitiesSyncAppClipValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing bundle id", args: []string{"--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS"}, wantErr: "--bundle-id is required"},
		{name: "missing parent bundle id", args: []string{"--bundle-id", "clip-1", "--capability", "PUSH_NOTIFICATIONS"}, wantErr: "--parent-bundle-id is required"},
		{name: "missing capability", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1"}, wantErr: "--capability is required"},
		{name: "invalid settings json", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--settings-json", `{"key":"BAD"}`}, wantErr: "--settings-json must be a JSON array"},
		{name: "null settings json", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--settings-json", `null`}, wantErr: "--settings-json must be a JSON array"},
		{name: "multiple settings json values", args: []string{"--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS", "--settings-json", `[] []`}, wantErr: "--settings-json must be a JSON array"},
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

func TestWebBundleIDCapabilitiesSyncAppClipCallsPrivateSync(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSync := syncAppClipBundleIDCapabilityFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		syncAppClipBundleIDCapabilityFn = origSync
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var gotReq webcore.AppClipBundleIDCapabilitySyncRequest
	syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		gotReq = req
		return &webcore.AppClipBundleIDCapabilitySyncResult{
			BundleID:       req.BundleID,
			ParentBundleID: req.ParentBundleID,
			Capability:     req.Capability,
			Enabled:        req.Enabled,
		}, nil
	}

	cmd := WebBundleIDCapabilitiesSyncAppClipCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--bundle-id", "clip-1",
		"--parent-bundle-id", "parent-1",
		"--capability", "push_notifications",
		"--settings-json", `[{"key":"PUSH_NOTIFICATION_FEATURES","options":[{"key":"PUSH_NOTIFICATION_FEATURE_BROADCAST","enabled":true}]}]`,
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
	if !strings.Contains(stdout, `"parentBundleId":"parent-1"`) || !strings.Contains(stdout, `"capability":"PUSH_NOTIFICATIONS"`) {
		t.Fatalf("unexpected stdout: %q", stdout)
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

func TestWebBundleIDCapabilitiesEnableHelpUsesSupportedLoginFlags(t *testing.T) {
	cmd := WebBundleIDCapabilitiesEnableCommand()
	if strings.Contains(cmd.LongHelp, "--reauthenticate") {
		t.Fatalf("enable help recommends unsupported web auth login flag: %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, `asc web auth logout --apple-id "user@example.com"`) {
		t.Fatalf("enable help is missing the supported cache-clear command: %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, `asc web auth login --apple-id "user@example.com"`) {
		t.Fatalf("enable help is missing the supported login command: %q", cmd.LongHelp)
	}
}

func TestWebBundleIDCapabilitiesEnableCallsDeveloperPortalClient(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origEnable := enableDeveloperBundleIDCapabilityFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		enableDeveloperBundleIDCapabilityFn = origEnable
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
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
