package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebDeclarationSession(t *testing.T, accountID string) {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{PublicProviderID: accountID}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
}

func TestWebAppsDeclarationsListValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing app", args: nil, wantErr: "--app is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			cmd := WebAppsDeclarationsListCommand()
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

func TestWebAppsDeclarationsListRejectsPositionalArguments(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := WebAppsDeclarationsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, stderr := captureWebCommandOutput(t, func() {
		err := cmd.Exec(context.Background(), []string{"extra"})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "unexpected argument") {
		t.Fatalf("expected positional argument error, got %q", stderr)
	}
}

func TestWebAppsDeclarationsListRequiresPublicProviderID(t *testing.T) {
	stubWebDeclarationSession(t, "")

	cmd := WebAppsDeclarationsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "public provider/account id") {
		t.Fatalf("expected missing account id error, got %v", err)
	}
}

func TestWebAppsDeclarationsListPrintsJSON(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origList := listWebAppDeclarationsFn
	t.Cleanup(func() { listWebAppDeclarationsFn = origList })

	var gotAccountID, gotAppID string
	listWebAppDeclarationsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppDeclaration, error) {
		gotAccountID = accountID
		gotAppID = appID
		return []webcore.AppDeclaration{
			{
				AppID:           appID,
				RequirementID:   "req-1",
				RequirementName: "MEDICAL_DEVICE",
				Status:          "PENDING_COLLECTION",
				Required:        true,
			},
		}, nil
	}

	cmd := WebAppsDeclarationsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
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
	if gotAccountID != "account-123" || gotAppID != "app-1" {
		t.Fatalf("unexpected identifiers: account=%q app=%q", gotAccountID, gotAppID)
	}

	var decoded []struct {
		AppID           string `json:"appId"`
		RequirementName string `json:"requirementName"`
		Status          string `json:"status"`
		Required        bool   `json:"required"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("decode stdout %q: %v", stdout, err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected one declaration, got %d", len(decoded))
	}
	if decoded[0].AppID != "app-1" || decoded[0].RequirementName != "MEDICAL_DEVICE" || !decoded[0].Required {
		t.Fatalf("unexpected declaration: %#v", decoded[0])
	}
}

func TestWebAppsDeclarationsListPrintsTable(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origList := listWebAppDeclarationsFn
	t.Cleanup(func() { listWebAppDeclarationsFn = origList })

	listWebAppDeclarationsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppDeclaration, error) {
		return []webcore.AppDeclaration{{
			AppID:           appID,
			RequirementID:   "req-1",
			RequirementName: "MEDICAL_DEVICE",
			Status:          "COLLECTED",
			Required:        true,
		}}, nil
	}

	cmd := WebAppsDeclarationsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"Requirement", "Status", "Required", "MEDICAL_DEVICE", "COLLECTED"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table to contain %q, got %q", want, stdout)
		}
	}
}

func TestWebAppsDeclarationsListReportsEmptyResult(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origList := listWebAppDeclarationsFn
	t.Cleanup(func() { listWebAppDeclarationsFn = origList })

	listWebAppDeclarationsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppDeclaration, error) {
		return nil, nil
	}

	cmd := WebAppsDeclarationsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected empty json array, got %q", stdout)
	}
}

func TestWebAppsMedicalDeviceViewPrintsDeclaration(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origGet := viewWebMedicalDeviceDeclarationFn
	t.Cleanup(func() { viewWebMedicalDeviceDeclarationFn = origGet })

	viewWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) (*webcore.MedicalDeviceDeclarationState, error) {
		return &webcore.MedicalDeviceDeclarationState{
			AppID:           appID,
			RequirementID:   "req-1",
			RequirementName: "MEDICAL_DEVICE",
			Status:          "COLLECTED",
			Required:        true,
			Declaration:     "no",
		}, nil
	}

	cmd := WebAppsMedicalDeviceViewCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
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

	var decoded struct {
		AppID       string `json:"appId"`
		Declaration string `json:"declaration"`
		Required    bool   `json:"required"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("decode stdout %q: %v", stdout, err)
	}
	if decoded.AppID != "app-1" || decoded.Declaration != "no" || !decoded.Required {
		t.Fatalf("unexpected declaration state: %#v", decoded)
	}
}

func TestWebAppsMedicalDeviceViewRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := WebAppsMedicalDeviceViewCommand()
	if err := cmd.FlagSet.Parse(nil); err != nil {
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
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected --app error, got %q", stderr)
	}
}

func TestWebAppsMedicalDeviceSetReportsUnchangedDeclaration(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origSet := setWebMedicalDeviceDeclarationFn
	t.Cleanup(func() { setWebMedicalDeviceDeclarationFn = origSet })

	setWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool) (*webcore.MedicalDeviceDeclarationResult, error) {
		return &webcore.MedicalDeviceDeclarationResult{
			AppID:           appID,
			RequirementID:   "req-1",
			RequirementName: "MEDICAL_DEVICE",
			Status:          "COLLECTED",
			Declared:        false,
			Changed:         false,
		}, nil
	}

	cmd := WebAppsMedicalDeviceSetCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--declared", "false", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"changed":false`) {
		t.Fatalf("expected changed=false in stdout, got %q", stdout)
	}
}
