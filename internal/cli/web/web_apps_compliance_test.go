package web

import (
	"context"
	"testing"

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
