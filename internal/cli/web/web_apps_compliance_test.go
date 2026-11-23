package web

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsComplianceListPrintsJSON(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origList := listWebAppComplianceRequirementsFn
	t.Cleanup(func() { listWebAppComplianceRequirementsFn = origList })

	var gotAccountID, gotAppID string
	listWebAppComplianceRequirementsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppComplianceRequirement, error) {
		gotAccountID, gotAppID = accountID, appID
		return []webcore.AppComplianceRequirement{
			{
				AppID: appID, RequirementID: "req-1", RequirementName: "SECURITY_ASSESSMENT", FormName: "SECURITYASF395E",
				Status: "COLLECTED_VERIFY_PENDING", Required: true, CountriesOrRegions: []string{"CHN"}, FormID: "form-1",
				DateSigned: "2026-01-01T00:00:00.000Z", LastUpdatedDate: "2026-01-02T00:00:00.000Z",
				RequiredActions: []string{"APP_BINARY_SUBMISSION"},
			},
			{
				AppID: appID, RequirementID: "req-2", RequirementName: "AI_GENERATED_CONTENT", FormName: "AIGENERATEFD1DE",
				Status: "PENDING_COLLECT", Required: true, CountriesOrRegions: []string{"CHN"},
			},
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsComplianceListCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
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
	if gotAccountID != "account-123" || gotAppID != "app-1" {
		t.Fatalf("unexpected identifiers: account=%q app=%q", gotAccountID, gotAppID)
	}

	want := `[{"appId":"app-1","requirementId":"req-1","requirementName":"SECURITY_ASSESSMENT","formName":"SECURITYASF395E",` +
		`"status":"COLLECTED_VERIFY_PENDING","required":true,"countriesOrRegions":["CHN"],"formId":"form-1",` +
		`"dateSigned":"2026-01-01T00:00:00.000Z","lastUpdatedDate":"2026-01-02T00:00:00.000Z","requiredActions":["APP_BINARY_SUBMISSION"]},` +
		`{"appId":"app-1","requirementId":"req-2","requirementName":"AI_GENERATED_CONTENT","formName":"AIGENERATEFD1DE",` +
		`"status":"PENDING_COLLECT","required":true,"countriesOrRegions":["CHN"]}]` + "\n"
	if stdout != want {
		t.Fatalf("unexpected stdout:\n got %s\nwant %s", stdout, want)
	}
}

func TestWebAppsComplianceSetValidationErrors(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origSet := setWebAppComplianceDeclarationFn
	t.Cleanup(func() { setWebAppComplianceDeclarationFn = origSet })
	setWebAppComplianceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declaration webcore.AppComplianceDeclaration) (*webcore.AppComplianceDeclarationResult, error) {
		t.Fatalf("invalid flags reached App Store Connect: %#v", declaration)
		return nil, nil
	}

	security := []string{"--app", "app-1", "--requirement", "SECURITY_ASSESSMENT"}
	aigc := []string{"--app", "app-1", "--requirement", "AI_GENERATED_CONTENT", "--declared", "true"}
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing app", args: []string{"--requirement", "SECURITY_ASSESSMENT", "--declared", "false"}, wantErr: "--app is required"},
		{name: "missing requirement", args: []string{"--app", "app-1", "--declared", "false"}, wantErr: "--requirement must be one of: SECURITY_ASSESSMENT, AI_GENERATED_CONTENT"},
		{name: "unknown requirement", args: []string{"--app", "app-1", "--requirement", "MEDICAL_DEVICE", "--declared", "false"}, wantErr: "--requirement must be one of: SECURITY_ASSESSMENT, AI_GENERATED_CONTENT"},
		{name: "missing declared", args: security, wantErr: "--declared is required (supported values: true, false)"},
		{name: "evidence with a negative answer", args: append(security[:4:4], "--declared", "false", "--proof", "proof.pdf"), wantErr: "--proof does not apply to SECURITY_ASSESSMENT with --declared false"},
		{name: "evidence of the other requirement", args: append(security[:4:4], "--declared", "true", "--proof", "a.pdf", "--report", "b.pdf", "--filing-name", "Model"), wantErr: "--filing-name does not apply to SECURITY_ASSESSMENT with --declared true"},
		{name: "security assessment without its report", args: append(security[:4:4], "--declared", "true", "--proof", "a.pdf"), wantErr: "--report is required for SECURITY_ASSESSMENT with --declared true"},
		{name: "unknown filing type", args: append(aigc[:6:6], "--filing-type", "MODEL", "--filing-name", "Model", "--filing-number", "1", "--third-party-technology", "false"), wantErr: "--filing-type must be one of: LLM, ALGORITHM"},
		{name: "third-party answer missing", args: append(aigc[:6:6], "--filing-type", "LLM", "--filing-name", "Model", "--filing-number", "1"), wantErr: "--third-party-technology is required for AI_GENERATED_CONTENT with --declared true"},
		{name: "third-party technology without its authorization", args: append(aigc[:6:6], "--filing-type", "LLM", "--filing-name", "Model", "--filing-number", "1", "--third-party-technology", "true"), wantErr: "--third-party-authorization is required with --third-party-technology true"},
		{name: "authorization without third-party technology", args: append(aigc[:6:6], "--filing-type", "LLM", "--filing-name", "Model", "--filing-number", "1", "--third-party-technology", "false", "--third-party-authorization", "a.pdf"), wantErr: "--third-party-authorization does not apply with --third-party-technology false"},
		{name: "save without confirm", args: append(security[:4:4], "--declared", "false"), wantErr: "--confirm is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebAppsComplianceSetCommand(d)
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
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

func TestWebAppsComplianceSetSendsEvidenceAndPrintsReceipt(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origSet := setWebAppComplianceDeclarationFn
	t.Cleanup(func() { setWebAppComplianceDeclarationFn = origSet })

	dir := t.TempDir()
	authorization := filepath.Join(dir, "agreement.pdf")
	if err := os.WriteFile(authorization, []byte("agreement"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got webcore.AppComplianceDeclaration
	var gotDeadline time.Time
	setWebAppComplianceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declaration webcore.AppComplianceDeclaration) (*webcore.AppComplianceDeclarationResult, error) {
		got = declaration
		gotDeadline, _ = ctx.Deadline()
		return &webcore.AppComplianceDeclarationResult{
			AppID: appID, RequirementID: "req-2", RequirementName: declaration.Requirement, Declared: declaration.Declared,
			Status: "COLLECTED_VERIFY_PENDING", FormID: "form-2",
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	// Evidence is uploaded, so the upload budget must bound the save, not the request one.
	requestDeadline := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	uploadDeadline := requestDeadline.Add(time.Hour)
	d.Timeout = func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithDeadline(ctx, requestDeadline)
	}
	d.UploadTimeout = func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithDeadline(ctx, uploadDeadline)
	}
	cmd := newWebAppsComplianceSetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1", "--requirement", "AI_GENERATED_CONTENT", "--declared", "true",
		"--filing-type", "llm", "--filing-name", " Example Model ", "--filing-number", "Example-0000",
		"--third-party-technology", "true", "--third-party-authorization", authorization,
		"--confirm", "--output", "json",
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

	want := webcore.AppComplianceDeclaration{
		Requirement: "AI_GENERATED_CONTENT", Declared: true, FilingType: "LLM",
		FilingName: "Example Model", FilingNumber: "Example-0000", ThirdPartyTechnology: true,
		ThirdPartyAuthorization: &webcore.AppComplianceFile{Name: "agreement.pdf", Data: []byte("agreement")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected declaration:\n got %#v\nwant %#v", got, want)
	}
	if !gotDeadline.Equal(uploadDeadline) {
		t.Fatalf("save deadline = %v, want the upload deadline %v", gotDeadline, uploadDeadline)
	}
	wantStdout := `{"appId":"app-1","requirementId":"req-2","requirementName":"AI_GENERATED_CONTENT","declared":true,` +
		`"status":"COLLECTED_VERIFY_PENDING","formId":"form-2","dryRun":false}` + "\n"
	if stdout != wantStdout {
		t.Fatalf("unexpected stdout:\n got %s\nwant %s", stdout, wantStdout)
	}
}

func TestWebAppsComplianceSetStopsOnUnreadableEvidence(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origSet := setWebAppComplianceDeclarationFn
	t.Cleanup(func() { setWebAppComplianceDeclarationFn = origSet })
	setWebAppComplianceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declaration webcore.AppComplianceDeclaration) (*webcore.AppComplianceDeclarationResult, error) {
		t.Fatalf("declaration with unreadable evidence reached App Store Connect: %#v", declaration)
		return nil, nil
	}

	dir := t.TempDir()
	empty := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(dir, "proof.pdf")
	if err := os.WriteFile(valid, []byte("proof"), 0o600); err != nil {
		t.Fatal(err)
	}
	for wantErr, args := range map[string][]string{
		"open --proof":      {"--proof", filepath.Join(dir, "missing.pdf"), "--report", valid},
		"--report is empty": {"--proof", valid, "--report", empty},
	} {
		d, _, _ := newWebTestDeps()
		cmd := newWebAppsComplianceSetCommand(d)
		if err := cmd.FlagSet.Parse(append([]string{"--app", "app-1", "--requirement", "SECURITY_ASSESSMENT", "--declared", "true", "--dry-run"}, args...)); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("expected %q error, got %v", wantErr, err)
		}
	}
}

func TestWebAppsComplianceSetDryRunSaysTheAnswerWasNotSent(t *testing.T) {
	stubWebDeclarationSession(t, "account-123")
	origSet := setWebAppComplianceDeclarationFn
	t.Cleanup(func() { setWebAppComplianceDeclarationFn = origSet })
	setWebAppComplianceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declaration webcore.AppComplianceDeclaration) (*webcore.AppComplianceDeclarationResult, error) {
		return &webcore.AppComplianceDeclarationResult{
			AppID: appID, RequirementID: "req-1", RequirementName: declaration.Requirement, Declared: declaration.Declared,
			Status: "PENDING_COLLECT", DryRun: declaration.DryRun,
		}, nil
	}

	dir := t.TempDir()
	proof := filepath.Join(dir, "proof.pdf")
	if err := os.WriteFile(proof, []byte("proof"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		args       []string
		wantStderr string
	}{
		"with evidence": {
			args:       []string{"--declared", "true", "--proof", proof, "--report", proof},
			wantStderr: "Dry run: the answer and its evidence files were not sent to Apple; only the requirements list was read.\n",
		},
		"without evidence": {
			args:       []string{"--declared", "false"},
			wantStderr: "Dry run: the answer was not sent to Apple; only the requirements list was read.\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebAppsComplianceSetCommand(d)
			args := append([]string{"--app", "app-1", "--requirement", "SECURITY_ASSESSMENT", "--dry-run", "--output", "json"}, test.args...)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if !strings.Contains(stdout, `"dryRun":true`) {
				t.Fatalf("stdout = %q, want a dry-run receipt", stdout)
			}
		})
	}
}

func TestWebAppsComplianceSetHelpSaysDryRunChecksLocally(t *testing.T) {
	d, _, _ := newWebTestDeps()
	cmd := newWebAppsComplianceSetCommand(d)
	const want = "--dry-run checks the answer and file types locally; Apple's own form checks run only with --confirm"
	if got := cmd.FlagSet.Lookup("dry-run").Usage; !strings.Contains(got, "Apple's own form checks run only with --confirm") {
		t.Fatalf("--dry-run usage = %q, want it to say Apple's form checks run only with --confirm", got)
	}
	help := strings.Join(strings.Fields(strings.ReplaceAll(cmd.LongHelp, "`", "")), " ")
	for _, text := range []string{want, "a first --confirm can still be refused by Apple's form"} {
		if !strings.Contains(help, text) {
			t.Fatalf("long help does not say %q:\n%s", text, cmd.LongHelp)
		}
	}
}

func TestWebAppsComplianceSetReadOnlyRefusesBeforeReadingEvidence(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")
	origSet := setWebAppComplianceDeclarationFn
	t.Cleanup(func() { setWebAppComplianceDeclarationFn = origSet })
	setWebAppComplianceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declaration webcore.AppComplianceDeclaration) (*webcore.AppComplianceDeclarationResult, error) {
		t.Fatalf("read-only mode reached App Store Connect: %#v", declaration)
		return nil, nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebAppsComplianceSetCommand(d)
	missing := filepath.Join(t.TempDir(), "missing.pdf")
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1", "--requirement", "SECURITY_ASSESSMENT", "--declared", "true",
		"--proof", missing, "--report", missing, "--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("error = %v, want readonly.ErrRefused before the evidence files are opened", err)
	}
}
