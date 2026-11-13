package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebRemovedAppsCommandHierarchy(t *testing.T) {
	cmd := WebRemovedAppsCommand()
	if cmd.Name != "removed-apps" {
		t.Fatalf("expected command name %q, got %q", "removed-apps", cmd.Name)
	}
	if cmd.UsageFunc == nil {
		t.Fatal("expected command usage func")
	}
	if len(cmd.Subcommands) != 2 || cmd.Subcommands[0].Name != "list" || cmd.Subcommands[1].Name != "restore" {
		t.Fatalf("expected list subcommand, got %+v", cmd.Subcommands)
	}
}

func TestWebRemovedAppsListOutputsJSON(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		if appleID != "user@example.com" {
			t.Fatalf("expected apple id %q, got %q", "user@example.com", appleID)
		}
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origListRemoved := listRemovedWebAppsFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		listRemovedWebAppsFn = origListRemoved
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	listRemovedWebAppsFn = func(ctx context.Context, client *webcore.Client, opts webcore.RemovedAppsListOptions) (*webcore.RemovedAppsListResponse, error) {
		if opts.Limit != 10 {
			t.Fatalf("expected limit 10, got %d", opts.Limit)
		}
		return &webcore.RemovedAppsListResponse{
			Data: []webcore.RemovedApp{{
				ID:                   "1234567890",
				Name:                 "Throwaway",
				BundleID:             "com.example.throwaway",
				SKU:                  "THROWAWAY",
				PrimaryLocale:        "en-US",
				Removed:              true,
				AppStoreLegacyStatus: "PREPARE_FOR_SUBMISSION",
				DisplayableVersions: []webcore.RemovedAppVersion{{
					ID:            "version-1",
					Platform:      "IOS",
					VersionString: "1.0",
				}},
			}},
		}, nil
	}

	cmd := WebRemovedAppsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--limit", "10", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload webcore.RemovedAppsListResponse
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected JSON output, got error %v and output %q", err, stdout)
	}
	if len(payload.Data) != 1 || payload.Data[0].Name != "Throwaway" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestWebRemovedAppsListTableIncludesCoreColumns(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origListRemoved := listRemovedWebAppsFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		listRemovedWebAppsFn = origListRemoved
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	listRemovedWebAppsFn = func(ctx context.Context, client *webcore.Client, opts webcore.RemovedAppsListOptions) (*webcore.RemovedAppsListResponse, error) {
		return &webcore.RemovedAppsListResponse{
			Data: []webcore.RemovedApp{{
				ID:             "1234567890",
				Name:           "Throwaway",
				BundleID:       "com.example.throwaway",
				VersionSummary: "iOS 1.0",
				Status:         "PREPARE_FOR_SUBMISSION",
				Removed:        true,
			}},
		}, nil
	}

	cmd := WebRemovedAppsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
	for _, want := range []string{"ID", "Name", "Bundle ID", "Version", "Status", "Throwaway", "com.example.throwaway", "iOS 1.0", "PREPARE_FOR_SUBMISSION"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestWebRemovedAppsListValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "limit low", args: []string{"--limit", "0"}, wantErr: "--limit must be between 1 and 200"},
		{name: "next with paginate", args: []string{"--next", "https://appstoreconnect.apple.com/iris/v1/apps?page=2", "--paginate"}, wantErr: "--next cannot be combined with --paginate"},
		{name: "bad next host", args: []string{"--next", "https://example.com/iris/v1/apps?page=2"}, wantErr: "--next must be an App Store Connect web URL"},
		{name: "protocol relative next host", args: []string{"--next", "//example.com/iris/v1/apps?filter[removed]=true"}, wantErr: "--next must be an App Store Connect web URL"},
		{name: "next missing removed filter", args: []string{"--next", "https://appstoreconnect.apple.com/iris/v1/apps?limit=48"}, wantErr: "--next must include filter[removed]=true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebRemovedAppsListCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), cmd.FlagSet.Args())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestWebRemovedAppsRestoreValidationBeforeAuth(t *testing.T) {
	authCalled := false
	restoreSession := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		authCalled = true
		return nil, "", errors.New("auth should not be called")
	})
	t.Cleanup(restoreSession)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"app", nil, "--app is required"},
		{"access", []string{"--app", "123"}, "--access must be limited or full"},
		{"confirm", []string{"--app", "123", "--access", "full"}, "--confirm is required"},
		{"output format", []string{"--app", "123", "--access", "full", "--confirm", "--output", "yaml"}, "--output must be one of"},
		{"pretty human output", []string{"--app", "123", "--access", "full", "--confirm", "--output", "table", "--pretty"}, "--pretty is only valid with JSON output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebRemovedAppsRestoreCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			var gotErr error
			captureWebCommandOutput(t, func() { gotErr = cmd.Exec(context.Background(), cmd.FlagSet.Args()) })
			if gotErr == nil || !strings.Contains(gotErr.Error(), tc.want) {
				t.Fatalf("expected %q in %v", tc.want, gotErr)
			}
		})
	}
	if authCalled {
		t.Fatal("expected validation errors before authentication")
	}
}

func TestWebRemovedAppsRestoreSessionErrorIncludesAuthHint(t *testing.T) {
	restoreSession := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return nil, "", &webcore.APIError{Status: 401}
	})
	t.Cleanup(restoreSession)
	cmd := WebRemovedAppsRestoreCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--access", "full", "--confirm"}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "web removed-apps restore failed: web session is unauthorized or expired") || !strings.Contains(err.Error(), "asc web auth login") {
		t.Fatalf("expected restore auth hint, got %v", err)
	}
}

func TestWebRemovedAppsRestoreCallOrderAndJSON(t *testing.T) {
	restoreSession := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origClient, origRestore, origVerify, origPermission := newWebClientFn, restoreRemovedWebAppFn, getWebAppRemovalStateFn, setRemovedWebAppPermissionFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origClient
		restoreRemovedWebAppFn = origRestore
		getWebAppRemovalStateFn = origVerify
		setRemovedWebAppPermissionFn = origPermission
	})
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	calls := []string{}
	restoreRemovedWebAppFn = func(context.Context, *webcore.Client, string) error { calls = append(calls, "restore"); return nil }
	getWebAppRemovalStateFn = func(context.Context, *webcore.Client, string) (*webcore.AppRemovalState, error) {
		calls = append(calls, "verify")
		return &webcore.AppRemovalState{RemovedKnown: true, Removed: false}, nil
	}
	setRemovedWebAppPermissionFn = func(context.Context, *webcore.Client, string, string) error {
		calls = append(calls, "permission")
		return nil
	}
	cmd := WebRemovedAppsRestoreCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--access", "full", "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if got, want := strings.Join(calls, ","), "restore,verify,permission"; got != want {
		t.Fatalf("call order %q, want %q", got, want)
	}
	var got asc.WebRemovedAppRestoreResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.AppID != "123" || got.Access != "full" || got.Removed || !got.PermissionWritten {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestWebRemovedAppsRestoreHumanRenderers(t *testing.T) {
	restoreSession := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origClient, origRestore, origVerify, origPermission := newWebClientFn, restoreRemovedWebAppFn, getWebAppRemovalStateFn, setRemovedWebAppPermissionFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origClient
		restoreRemovedWebAppFn = origRestore
		getWebAppRemovalStateFn = origVerify
		setRemovedWebAppPermissionFn = origPermission
	})
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	restoreRemovedWebAppFn = func(context.Context, *webcore.Client, string) error { return nil }
	getWebAppRemovalStateFn = func(context.Context, *webcore.Client, string) (*webcore.AppRemovalState, error) {
		return &webcore.AppRemovalState{RemovedKnown: true, Removed: false}, nil
	}
	setRemovedWebAppPermissionFn = func(context.Context, *webcore.Client, string, string) error { return nil }

	for _, tc := range []struct {
		name, format, want string
	}{
		{name: "table", format: "table", want: "App ID"},
		{name: "markdown", format: "markdown", want: "| App ID | Access | Removed | Permission Written |"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebRemovedAppsRestoreCommand()
			if err := cmd.FlagSet.Parse([]string{"--app", "123", "--access", "full", "--confirm", "--output", tc.format}); err != nil {
				t.Fatal(err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatal(err)
				}
			})
			if stderr != "" {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Fatalf("%s output missing %q: %q", tc.name, tc.want, stdout)
			}
			if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
				t.Fatalf("%s output unexpectedly fell back to JSON: %q", tc.name, stdout)
			}
		})
	}

	// The restore receipt uses the same registry-backed renderer for the
	// interactive default as an explicit table selection.
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")
	shared.ResetDefaultOutputFormat()
	t.Cleanup(shared.ResetDefaultOutputFormat)
	cmd := WebRemovedAppsRestoreCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--access", "full", "--confirm"}); err != nil {
		t.Fatal(err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stdout, "App ID") {
		t.Fatalf("TTY-default output missing table header: %q", stdout)
	}
}

func TestWebRemovedAppsRestoreStopsAtFailure(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		verify                               *webcore.AppRemovalState
		restoreErr, verifyErr, permissionErr error
		want                                 string
	}{
		{"restore", nil, errors.New("patch broke"), nil, nil, "app PATCH"},
		{"verify error", nil, nil, errors.New("read broke"), nil, "could not verify removed state"},
		{"still removed", &webcore.AppRemovalState{RemovedKnown: true, Removed: true}, nil, nil, nil, "did not confirm"},
		{"permission", &webcore.AppRemovalState{RemovedKnown: true}, nil, nil, errors.New("write broke"), "app was restored but access update failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreSession := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{}, "cache", nil
			})
			t.Cleanup(restoreSession)
			origClient, origRestore, origVerify, origPermission := newWebClientFn, restoreRemovedWebAppFn, getWebAppRemovalStateFn, setRemovedWebAppPermissionFn
			t.Cleanup(func() {
				newWebClientFn = origClient
				restoreRemovedWebAppFn = origRestore
				getWebAppRemovalStateFn = origVerify
				setRemovedWebAppPermissionFn = origPermission
			})
			newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
			permissionCalled := false
			restoreRemovedWebAppFn = func(context.Context, *webcore.Client, string) error { return tc.restoreErr }
			getWebAppRemovalStateFn = func(context.Context, *webcore.Client, string) (*webcore.AppRemovalState, error) {
				return tc.verify, tc.verifyErr
			}
			setRemovedWebAppPermissionFn = func(context.Context, *webcore.Client, string, string) error {
				permissionCalled = true
				return tc.permissionErr
			}
			cmd := WebRemovedAppsRestoreCommand()
			if err := cmd.FlagSet.Parse([]string{"--app", "123", "--access", "limited", "--confirm"}); err != nil {
				t.Fatal(err)
			}
			var gotErr error
			captureWebCommandOutput(t, func() { gotErr = cmd.Exec(context.Background(), nil) })
			if gotErr == nil || !strings.Contains(gotErr.Error(), tc.want) {
				t.Fatalf("expected %q in %v", tc.want, gotErr)
			}
			if tc.name != "permission" && permissionCalled {
				t.Fatal("permission should be skipped")
			}
		})
	}
}

func TestValidateRemovedAppsNextURLAcceptsSafeRelativeLinks(t *testing.T) {
	for _, next := range []string{
		"/iris/v1/apps?filter[removed]=true&cursor=abc",
		"/apps?filter[removed]=true&cursor=abc",
		"https://appstoreconnect.apple.com/iris/v1/apps?filter[removed]=true&cursor=abc",
	} {
		t.Run(next, func(t *testing.T) {
			if err := validateRemovedAppsNextURL(next); err != nil {
				t.Fatalf("expected %q to validate, got %v", next, err)
			}
		})
	}
}
