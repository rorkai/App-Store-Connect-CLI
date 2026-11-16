package web

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsMedicalDeviceRegionSetRejectsUnsupportedInputBeforeAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "region.json")
	if err := os.WriteFile(path, []byte(`{"declaration":false,"contactInformation":[]}`), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	called := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		called = true
		return nil, "", errors.New("session must not be resolved")
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsMedicalDeviceRegionSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--region", "GBR",
		"--input", path,
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
	if !strings.Contains(stderr, `unsupported --input field "contactInformation"`) {
		t.Fatalf("expected unsupported-input error, got %q", stderr)
	}
	if called {
		t.Fatal("session was resolved before input validation")
	}
}

func TestWebAppsMedicalDeviceRegionSetParsesRootfsInputAndPrintsReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "region.json")
	if err := os.WriteFile(path, []byte(`{
		"declaration": false
	}`), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origSet := setWebMedicalDeviceRegionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		setWebMedicalDeviceRegionFn = origSet
	})
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{PublicProviderID: "account-123"}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	var gotAccountID, gotAppID, gotRegion string
	var gotOptions webcore.MedicalDeviceRegionOptions
	setWebMedicalDeviceRegionFn = func(ctx context.Context, client *webcore.Client, accountID, appID, region string, options webcore.MedicalDeviceRegionOptions) (*webcore.MedicalDeviceRegionResult, error) {
		gotAccountID, gotAppID, gotRegion, gotOptions = accountID, appID, region, options
		return &webcore.MedicalDeviceRegionResult{
			AppID:           appID,
			RequirementID:   "req-123",
			RequirementName: "MEDICAL_DEVICE",
			Status:          "COLLECTED",
			Region:          region,
			Declared:        false,
			Changed:         true,
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsMedicalDeviceRegionSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--region", "EU",
		"--input", path,
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
	if !strings.Contains(stdout, `"region":"EEA"`) || !strings.Contains(stdout, `"changed":true`) {
		t.Fatalf("unexpected receipt: %q", stdout)
	}
	if gotAccountID != "account-123" || gotAppID != "app-1" || gotRegion != "EEA" {
		t.Fatalf("unexpected request identity: account=%q app=%q region=%q", gotAccountID, gotAppID, gotRegion)
	}
	if gotOptions.Declaration || gotOptions.RegistrationNumber != "" || len(gotOptions.SupportInfo) != 0 {
		t.Fatalf("unexpected parsed options: %#v", gotOptions)
	}
}
