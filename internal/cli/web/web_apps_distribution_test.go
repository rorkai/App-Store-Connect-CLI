package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebAppsDistributionSession(t *testing.T) {
	t.Helper()

	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
}

func TestWebAppsDistributionViewPrintsAppleAttributes(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origGet := getWebAppDistributionFn
	t.Cleanup(func() { getWebAppDistributionFn = origGet })

	var gotAppID string
	getWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppDistribution, error) {
		gotAppID = appID
		return &webcore.AppDistribution{
			AppID:                 appID,
			Name:                  "Example",
			BundleID:              "com.example.app",
			DistributionType:      "CUSTOM",
			EducationDiscountType: "NOT_APPLICABLE",
		}, nil
	}

	cmd := WebAppsDistributionViewCommand()
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
	if gotAppID != "app-1" {
		t.Fatalf("appID = %q, want app-1", gotAppID)
	}

	var out struct {
		AppID                 string `json:"appId"`
		BundleID              string `json:"bundleId"`
		DistributionType      string `json:"distributionType"`
		EducationDiscountType string `json:"educationDiscountType"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v; stdout=%q", err, stdout)
	}
	if out.AppID != "app-1" || out.BundleID != "com.example.app" {
		t.Fatalf("unexpected identity fields: %+v", out)
	}
	if out.DistributionType != "CUSTOM" {
		t.Fatalf("distributionType = %q, want CUSTOM", out.DistributionType)
	}
	if out.EducationDiscountType != "NOT_APPLICABLE" {
		t.Fatalf("educationDiscountType = %q, want NOT_APPLICABLE", out.EducationDiscountType)
	}
}

func TestWebAppsDistributionViewTableRendersUnknownForMissingAttributes(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origGet := getWebAppDistributionFn
	t.Cleanup(func() { getWebAppDistributionFn = origGet })

	getWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppDistribution, error) {
		return &webcore.AppDistribution{AppID: appID}, nil
	}

	cmd := WebAppsDistributionViewCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-2", "--output", "table"}); err != nil {
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
	for _, want := range []string{"distribution_type", "unknown", "education_discount_type", "app-2"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q; stdout=%q", want, stdout)
		}
	}
}

func TestWebAppsDistributionViewRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := WebAppsDistributionViewCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	_, stderr := captureWebCommandOutput(t, func() {
		err = cmd.Exec(context.Background(), nil)
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if want := "Error: --app is required (or set ASC_APP_ID)\n"; !strings.HasPrefix(stderr, want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, want)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp usage contract", err)
	}
	if kind := shared.ClassifyUsageError(err); kind != shared.UsageErrorMissingRequired {
		t.Fatalf("usage kind = %q, want %q", kind, shared.UsageErrorMissingRequired)
	}
}

func TestWebAppsRegistersDistributionAndHistory(t *testing.T) {
	registered := map[string]*ffcli.Command{}
	for _, sub := range WebAppsCommand().Subcommands {
		registered[sub.Name] = sub
	}
	if registered["history"] == nil {
		t.Fatal("expected asc web apps history to be registered")
	}
	distribution := registered["distribution"]
	if distribution == nil {
		t.Fatal("expected asc web apps distribution to be registered")
	}
	for _, sub := range distribution.Subcommands {
		if sub.Name == "set" {
			return
		}
	}
	t.Fatal("expected distribution set subcommand")
}

func TestWebAppsDistributionSetRequiresConfirmationBeforeSession(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	var resolveCalls int
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}

	cmd := WebAppsDistributionSetCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--method", "private"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	_, stderr := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", runErr)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("stderr = %q, want confirmation diagnostic", stderr)
	}
	if resolveCalls != 0 {
		t.Fatalf("session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestWebAppsDistributionSetRejectsInvalidOutputBeforeSession(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	var resolveCalls int
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}

	cmd := WebAppsDistributionSetCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--method", "private", "--confirm", "--output", "table", "--pretty"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("error = %v, want invalid output error", err)
	}
	if resolveCalls != 0 {
		t.Fatalf("session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestWebAppsDistributionSetCallsClientAndPrintsReceipt(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origSet := setWebAppDistributionFn
	t.Cleanup(func() { setWebAppDistributionFn = origSet })
	var gotRequest webcore.AppDistributionSetRequest
	setWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, request webcore.AppDistributionSetRequest) (*asc.WebAppDistributionSetResult, error) {
		gotRequest = request
		return &asc.WebAppDistributionSetResult{
			AppID:                 request.AppID,
			DistributionType:      request.DistributionType,
			EducationDiscountType: webcore.AppDistributionEducationNotApplicable,
			Changed:               true,
			Verified:              true,
			Status:                "verified",
		}, nil
	}

	cmd := WebAppsDistributionSetCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--method", "private", "--confirm", "--output", "json"}); err != nil {
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
	if gotRequest.AppID != "app-1" || gotRequest.DistributionType != webcore.AppDistributionTypeCustom || gotRequest.EducationDiscountType != "" {
		t.Fatalf("unexpected request: %+v", gotRequest)
	}
	var result asc.WebAppDistributionSetResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if result.AppID != "app-1" || !result.Changed || !result.Verified || result.Status != "verified" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestWebAppsDistributionSetRejectsEducationForPrivate(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	var resolveCalls int
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}

	cmd := WebAppsDistributionSetCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--method", "private", "--education-discount", "discounted", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", runErr)
	}
	if !strings.Contains(stderr, "cannot be used with --method private") {
		t.Fatalf("stderr = %q, want private education diagnostic", stderr)
	}
	if resolveCalls != 0 {
		t.Fatalf("session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestWebAppsDistributionSetPrintsUncertainReceiptBeforeReturningError(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origSet := setWebAppDistributionFn
	t.Cleanup(func() { setWebAppDistributionFn = origSet })
	setWebAppDistributionFn = func(ctx context.Context, client *webcore.Client, request webcore.AppDistributionSetRequest) (*asc.WebAppDistributionSetResult, error) {
		return &asc.WebAppDistributionSetResult{
			AppID:                 request.AppID,
			DistributionType:      request.DistributionType,
			EducationDiscountType: webcore.AppDistributionEducationNotApplicable,
			Changed:               true,
			Verified:              false,
			Status:                "uncertain",
		}, errors.New("provider write outcome is uncertain")
	}

	cmd := WebAppsDistributionSetCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--method", "private", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected uncertain write error")
	}
	if !strings.Contains(stdout, `"status":"uncertain"`) {
		t.Fatalf("stdout = %q, want uncertain receipt", stdout)
	}
}
