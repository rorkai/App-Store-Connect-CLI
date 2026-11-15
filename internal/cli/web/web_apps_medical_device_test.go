package web

import (
	"context"
	"errors"
	"flag"
	"slices"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsMedicalDeviceSetValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing app", args: []string{"--declared", "false"}, wantErr: "--app is required"},
		{name: "missing declared", args: []string{"--app", "app-1"}, wantErr: "--declared is required"},
		{name: "missing confirmation", args: []string{"--app", "app-1", "--declared", "true"}, wantErr: "--confirm is required"},
		{name: "blank regions", args: []string{"--app", "app-1", "--declared", "true", "--countries-or-regions", " ", "--confirm"}, wantErr: "--countries-or-regions must not be empty"},
		{name: "regions with false", args: []string{"--app", "app-1", "--declared", "false", "--countries-or-regions", "EEA"}, wantErr: "--countries-or-regions requires --declared true"},
		{name: "blank regions with false", args: []string{"--app", "app-1", "--declared", "false", "--countries-or-regions", ""}, wantErr: "--countries-or-regions must not be empty"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebAppsMedicalDeviceSetCommand(d)
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
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

func TestWebAppsMedicalDeviceSetResolvesSessionWithoutRequestDeadline(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	hadDeadline := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		_, hadDeadline = ctx.Deadline()
		return nil, "", resolveErr
	}

	d, _, _ := newWebTestDeps()

	cmd := newWebAppsMedicalDeviceSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "false",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected resolveSession error %v, got %v", resolveErr, err)
	}
	if hadDeadline {
		t.Fatal("expected session resolution to run without the request deadline")
	}
}

func TestWebAppsMedicalDeviceSetUpdatesDeclarationWithoutConfirmation(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSet := setWebMedicalDeviceDeclarationFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		setWebMedicalDeviceDeclarationFn = origSet
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{PublicProviderID: "account-123"}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var gotAccountID string
	var gotAppID string
	var gotDeclared bool
	setWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool) (*webcore.MedicalDeviceDeclarationResult, error) {
		gotAccountID = accountID
		gotAppID = appID
		gotDeclared = declared
		return &webcore.MedicalDeviceDeclarationResult{
			AppID:              appID,
			RequirementID:      "req-123",
			RequirementName:    "MEDICAL_DEVICE",
			Status:             "COLLECTED",
			Declared:           false,
			CountriesOrRegions: []string{"EEA", "GBR", "USA"},
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()

	cmd := newWebAppsMedicalDeviceSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "false",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"appId":"app-1"`) {
		t.Fatalf("expected app id in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, `"declared":false`) {
		t.Fatalf("expected declared=false in stdout, got %q", stdout)
	}
	if gotAccountID != "account-123" {
		t.Fatalf("expected account-123, got %q", gotAccountID)
	}
	if gotAppID != "app-1" {
		t.Fatalf("expected app-1, got %q", gotAppID)
	}
	if gotDeclared {
		t.Fatal("expected declared=false")
	}
}

func TestWebAppsMedicalDeviceSetTruePassesSelectedRegions(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSetWithOptions := setWebMedicalDeviceDeclarationWithOptionsFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		setWebMedicalDeviceDeclarationWithOptionsFn = origSetWithOptions
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{PublicProviderID: "account-123"}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var gotDeclared bool
	var gotRegions []string
	setWebMedicalDeviceDeclarationWithOptionsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool, options webcore.MedicalDeviceDeclarationOptions) (*webcore.MedicalDeviceDeclarationResult, error) {
		gotDeclared = declared
		gotRegions = options.CountriesOrRegions
		return &webcore.MedicalDeviceDeclarationResult{AppID: appID, Declared: declared, Changed: true}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()

	cmd := newWebAppsMedicalDeviceSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "true",
		"--countries-or-regions", "GBR,EEA",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !gotDeclared || !slices.Equal(gotRegions, []string{"EEA", "GBR"}) {
		t.Fatalf("unexpected options: declared=%t regions=%v", gotDeclared, gotRegions)
	}
	if !strings.Contains(stdout, `"declared":true`) {
		t.Fatalf("expected declared=true in stdout, got %q", stdout)
	}
}

func TestWebAppsMedicalDeviceSetRejectsInvalidRegionBeforeSession(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	called := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", errors.New("session must not be resolved")
	}

	d, depsStdout, depsStderr := newWebTestDeps()

	cmd := newWebAppsMedicalDeviceSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "true",
		"--countries-or-regions", "CAN",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "unsupported medical device country/region") {
		t.Fatalf("expected invalid-region error, got %q", stderr)
	}
	if called {
		t.Fatal("session was resolved before region validation")
	}
}

func TestWebAppsMedicalDeviceSetRequiresPublicProviderID(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}

	d, _, _ := newWebTestDeps()

	cmd := newWebAppsMedicalDeviceSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--declared", "false",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "public provider/account id") {
		t.Fatalf("expected missing account id error, got %v", err)
	}
}
