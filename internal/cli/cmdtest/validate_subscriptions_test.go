package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateSubscriptionsFixture struct {
	groups                     string
	availabilityV2             string
	availabilityV2Status       int
	territories                string
	territoriesByQuery         map[string]string
	territoryStatusByQuery     map[string]int
	subscriptionsByGroup       map[string]string
	groupLocalizationsByGroup  map[string]string
	groupLocalizationStatus    map[string]int
	imagesBySubscription       map[string]string
	imageStatusBySubscription  map[string]int
	imageErrorBySubscription   map[string]error
	localizationsBySub         map[string]string
	localizationsStatusBySub   map[string]int
	pricesBySubscription       map[string]string
	expectedPriceInclude       string
	pricesStatusBySubscription map[string]int
	priceErrorBySubscription   map[string]error
	subscriptionGroupsStatus   int
}

func newValidateSubscriptionsClient(t *testing.T, fixture validateSubscriptionsFixture) *asc.Client {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")
	writeECDSAPEM(t, keyPath)

	notFound := `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return jsonResponse(http.StatusMethodNotAllowed, `{"errors":[{"status":405}]}`)
		}

		path := req.URL.Path
		switch {
		case path == "/v1/apps/app-1/subscriptionGroups":
			if fixture.subscriptionGroupsStatus != 0 {
				return jsonResponse(fixture.subscriptionGroupsStatus, apiErrorJSONForStatus(fixture.subscriptionGroupsStatus))
			}
			return jsonResponse(http.StatusOK, fixture.groups)
		case path == "/v1/apps/app-1/appAvailabilityV2":
			if fixture.availabilityV2Status != 0 {
				return jsonResponse(fixture.availabilityV2Status, fixture.availabilityV2)
			}
			if fixture.availabilityV2 != "" {
				return jsonResponse(http.StatusOK, fixture.availabilityV2)
			}
			return jsonResponse(http.StatusNotFound, notFound)
		case strings.HasPrefix(path, "/v2/appAvailabilities/") && strings.HasSuffix(path, "/territoryAvailabilities"):
			if status, ok := fixture.territoryStatusByQuery[req.URL.RawQuery]; ok {
				return jsonResponse(status, apiErrorJSONForStatus(status))
			}
			if body, ok := fixture.territoriesByQuery[req.URL.RawQuery]; ok {
				return jsonResponse(http.StatusOK, body)
			}
			if fixture.territories != "" {
				return jsonResponse(http.StatusOK, fixture.territories)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasPrefix(path, "/v1/subscriptionGroups/") && strings.HasSuffix(path, "/subscriptionGroupLocalizations"):
			groupID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/subscriptionGroups/"), "/subscriptionGroupLocalizations")
			if status, ok := fixture.groupLocalizationStatus[groupID]; ok {
				return jsonResponse(status, apiErrorJSONForStatus(status))
			}
			if body, ok := fixture.groupLocalizationsByGroup[groupID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasPrefix(path, "/v1/subscriptionGroups/") && strings.HasSuffix(path, "/subscriptions"):
			groupID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/subscriptionGroups/"), "/subscriptions")
			if body, ok := fixture.subscriptionsByGroup[groupID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasPrefix(path, "/v1/subscriptions/") && strings.HasSuffix(path, "/subscriptionLocalizations"):
			subscriptionID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/subscriptions/"), "/subscriptionLocalizations")
			if status, ok := fixture.localizationsStatusBySub[subscriptionID]; ok {
				return jsonResponse(status, apiErrorJSONForStatus(status))
			}
			if body, ok := fixture.localizationsBySub[subscriptionID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasPrefix(path, "/v1/subscriptions/") && strings.HasSuffix(path, "/prices"):
			subscriptionID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/subscriptions/"), "/prices")
			if fixture.expectedPriceInclude != "" && req.URL.Query().Get("include") != fixture.expectedPriceInclude {
				t.Fatalf("expected include=%q, got %q", fixture.expectedPriceInclude, req.URL.Query().Get("include"))
			}
			if err, ok := fixture.priceErrorBySubscription[subscriptionID]; ok {
				return nil, err
			}
			if status, ok := fixture.pricesStatusBySubscription[subscriptionID]; ok {
				return jsonResponse(status, apiErrorJSONForStatus(status))
			}
			if body, ok := fixture.pricesBySubscription[subscriptionID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasPrefix(path, "/v1/subscriptions/") && strings.HasSuffix(path, "/images"):
			subscriptionID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/subscriptions/"), "/images")
			if err, ok := fixture.imageErrorBySubscription[subscriptionID]; ok {
				return nil, err
			}
			if status, ok := fixture.imageStatusBySubscription[subscriptionID]; ok {
				return jsonResponse(status, apiErrorJSONForStatus(status))
			}
			if body, ok := fixture.imagesBySubscription[subscriptionID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		default:
			return jsonResponse(http.StatusNotFound, notFound)
		}
	})

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func validValidateSubscriptionsFixture() validateSubscriptionsFixture {
	return validateSubscriptionsFixture{
		groups: `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Group"}}]}`,
		subscriptionsByGroup: map[string]string{
			"group-1": `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"APPROVED"}}]}`,
		},
		imagesBySubscription: map[string]string{
			"sub-1": `{"data":[{"type":"subscriptionImages","id":"image-1","attributes":{"fileName":"monthly.png","fileSize":1024}}]}`,
		},
	}
}

func TestValidateSubscriptionsRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected --app required error, got %q", stderr)
	}
}

func TestValidateSubscriptionsOutputsJSONAndTable(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("expected no issues, got %+v", report.Summary)
	}

	root = RootCommand("1.2.3")
	stdout, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Severity") {
		t.Fatalf("expected table output to include headers, got %q", stdout)
	}
}

func TestValidateSubscriptionsWarnsPartialSubscriptionPricingCoverage(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.availabilityV2 = `{"data":{"type":"appAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`
	fixture.territories = `{"data":[` +
		`{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true}},` +
		`{"type":"territoryAvailabilities","id":"ta-2","attributes":{"available":true}}` +
		`]}`
	fixture.expectedPriceInclude = "territory"
	fixture.pricesBySubscription = map[string]string{
		"sub-1": `{"data":[
			{"type":"subscriptionPrices","id":"price-old","attributes":{"startDate":"2026-01-01"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},
			{"type":"subscriptionPrices","id":"price-current","attributes":{"startDate":"2026-02-01"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}
		]}`,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected warning-only validate subscriptions run, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected pricing coverage warning, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsCountsAvailableTerritoriesAcrossPages(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.availabilityV2 = `{"data":{"type":"appAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`
	fixture.territories = `{"data":[{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true}}],"links":{"next":"https://api.appstoreconnect.apple.com/v2/appAvailabilities/avail-1/territoryAvailabilities?cursor=page-2"}}`
	fixture.territoriesByQuery = map[string]string{
		"cursor=page-2": `{"data":[{"type":"territoryAvailabilities","id":"ta-2","attributes":{"available":true}}],"links":{"next":""}}`,
	}
	fixture.expectedPriceInclude = "territory"
	fixture.pricesBySubscription = map[string]string{
		"sub-1": `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"startDate":"2026-01-01"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected warning-only validate subscriptions run, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected pricing coverage warning after paginated territory count, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsPricingCoverageWhenAvailabilityForbidden(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.availabilityV2Status = http.StatusForbidden
	fixture.availabilityV2 = apiErrorJSONForStatus(http.StatusForbidden)

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected availability permission failure to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.pricing_coverage.unverified") {
		t.Fatalf("expected pricing coverage skip info check, got %+v", report.Checks)
	}
	if hasCheckWithID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("did not expect pricing coverage warning when availability could not be read, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsPricingCoverageWhenAvailabilityRateLimited(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.availabilityV2Status = http.StatusTooManyRequests
	fixture.availabilityV2 = apiErrorJSONForStatus(http.StatusTooManyRequests)

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected availability rate-limit to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	var skipCheck *validation.CheckResult
	for i := range report.Checks {
		if report.Checks[i].ID == "subscriptions.pricing_coverage.unverified" {
			skipCheck = &report.Checks[i]
			break
		}
	}
	if skipCheck == nil {
		t.Fatalf("expected pricing coverage skip info check, got %+v", report.Checks)
	}
	if !strings.Contains(skipCheck.Remediation, "temporarily unavailable or rate limited") {
		t.Fatalf("expected retryable remediation, got %+v", *skipCheck)
	}
	if hasCheckWithID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("did not expect pricing coverage warning when availability is retryable, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsPricingCoverageWhenPaginatedAvailabilityRateLimited(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.availabilityV2 = `{"data":{"type":"appAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`
	fixture.territories = `{"data":[{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true}}],"links":{"next":"https://api.appstoreconnect.apple.com/v2/appAvailabilities/avail-1/territoryAvailabilities?cursor=page-2"}}`
	fixture.territoryStatusByQuery = map[string]int{
		"cursor=page-2": http.StatusTooManyRequests,
	}
	fixture.expectedPriceInclude = "territory"
	fixture.pricesBySubscription = map[string]string{
		"sub-1": `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"startDate":"2026-01-01"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected paginated availability rate-limit to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	var skipCheck *validation.CheckResult
	for i := range report.Checks {
		if report.Checks[i].ID == "subscriptions.pricing_coverage.unverified" {
			skipCheck = &report.Checks[i]
			break
		}
	}
	if skipCheck == nil {
		t.Fatalf("expected pricing coverage skip info check, got %+v", report.Checks)
	}
	if !strings.Contains(skipCheck.Remediation, "temporarily unavailable or rate limited") {
		t.Fatalf("expected retryable remediation, got %+v", *skipCheck)
	}
	if hasCheckWithID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("did not expect pricing coverage warning when paginated availability is retryable, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSurfacesSkippedPricingVerificationForApprovedSubscriptions(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.expectedPriceInclude = "territory"
	fixture.pricesStatusBySubscription = map[string]int{
		"sub-1": http.StatusForbidden,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected skipped pricing verification to stay non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.pricing.unverified") {
		t.Fatalf("expected pricing verification info check, got %+v", report.Checks)
	}
	if hasCheckWithID(report.Checks, "subscriptions.diagnostics.pricing_unverified") {
		t.Fatalf("did not expect missing-metadata pricing diagnostic for approved subscription, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsGroupLocalizationProbeForHealthySubscriptions(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.groupLocalizationsByGroup = map[string]string{
		"group-1": `{"data":invalid}`,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("expected no issues, got %+v", report.Summary)
	}
}

func TestValidateSubscriptionsWarnsAndStrictFails(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.subscriptionsByGroup["group-1"] = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"READY_TO_SUBMIT"}}]}`

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected no error (warning-only), got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Warnings == 0 {
		t.Fatalf("expected warnings, got %+v", report.Summary)
	}

	root = RootCommand("1.2.3")
	var runErr error
	stdout, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1", "--strict"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatalf("expected error with --strict")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var strictReport validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &strictReport); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range strictReport.Checks {
		if check.ID == "subscriptions.review_readiness.needs_attention" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected subscriptions.review_readiness.needs_attention check, got %+v", strictReport.Checks)
	}
}

func TestValidateSubscriptionsMissingMetadataDiagnosticsWarnByDefault(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.subscriptionsByGroup["group-1"] = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"MISSING_METADATA"}}]}`

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected MISSING_METADATA diagnostics to stay warning-only by default, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings == 0 {
		t.Fatalf("expected warnings without blocking errors, got %+v", report.Summary)
	}

	for _, check := range report.Checks {
		if check.ID == "subscriptions.diagnostics.localization_missing" && check.Severity != validation.SeverityWarning {
			t.Fatalf("expected missing-metadata diagnostics to be warnings, got %+v", check)
		}
	}

	root = RootCommand("1.2.3")
	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1", "--strict"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected warning-only missing-metadata diagnostics to fail under --strict")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
}

func TestValidateSubscriptionsWarnsWhenSubscriptionImageMissing(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.imagesBySubscription["sub-1"] = `{"data":[]}`

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr != nil {
		t.Fatalf("expected warning-only behavior, got %v", runErr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Warnings == 0 {
		t.Fatalf("expected warnings, got %+v", report.Summary)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "subscriptions.images.recommended" {
			found = true
			if !strings.Contains(strings.ToLower(check.Remediation), "offer") {
				t.Fatalf("expected remediation to explain why the image matters, got %+v", check)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected subscriptions.images.recommended check, got %+v", report.Checks)
	}

	root = RootCommand("1.2.3")
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1", "--strict"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected warning to become blocking with --strict")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
}

func TestValidateSubscriptionsSkipsImageWarningWhenImageEndpointForbidden(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.imageStatusBySubscription = map[string]int{
		"sub-1": http.StatusForbidden,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected image probe failure to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 || report.Summary.Infos == 0 {
		t.Fatalf("expected informational skipped-image check only, got %+v", report.Summary)
	}
	if hasCheckWithID(report.Checks, "subscriptions.images.recommended") {
		t.Fatalf("expected no promotional-image recommendation when probe is skipped, got %+v", report.Checks)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.images.unverified") {
		t.Fatalf("expected subscriptions.images.unverified check, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsImageWarningWhenImageEndpointTimesOut(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.imageErrorBySubscription = map[string]error{
		"sub-1": &url.Error{Op: "Get", URL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/images", Err: context.DeadlineExceeded},
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected image probe timeout to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 || report.Summary.Infos == 0 {
		t.Fatalf("expected informational skipped-image check only, got %+v", report.Summary)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.images.unverified") {
		t.Fatalf("expected subscriptions.images.unverified check, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsImageWarningWhenImageEndpointIsRetryable(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.imageStatusBySubscription = map[string]int{
		"sub-1": http.StatusTooManyRequests,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected retryable image probe failure to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 || report.Summary.Infos == 0 {
		t.Fatalf("expected informational skipped-image check only, got %+v", report.Summary)
	}
	if hasCheckWithID(report.Checks, "subscriptions.images.recommended") {
		t.Fatalf("expected no promotional-image recommendation when probe is skipped, got %+v", report.Checks)
	}
	foundUnverified := false
	for _, check := range report.Checks {
		if check.ID == "subscriptions.images.unverified" {
			foundUnverified = true
			if !strings.Contains(strings.ToLower(check.Remediation), "rate limited") {
				t.Fatalf("expected retryable remediation to mention rate limiting, got %+v", check)
			}
		}
	}
	if !foundUnverified {
		t.Fatalf("expected subscriptions.images.unverified check, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsSkipsImageWarningWhenImageEndpointTransportFails(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.imageErrorBySubscription = map[string]error{
		"sub-1": &url.Error{
			Op:  "Get",
			URL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/images",
			Err: &net.DNSError{
				Err:       "dial tcp: i/o timeout",
				Name:      "api.appstoreconnect.apple.com",
				IsTimeout: true,
			},
		},
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected transport image probe failure to be non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 || report.Summary.Infos == 0 {
		t.Fatalf("expected informational skipped-image check only, got %+v", report.Summary)
	}
	if hasCheckWithID(report.Checks, "subscriptions.images.recommended") {
		t.Fatalf("expected no promotional-image recommendation when probe is skipped, got %+v", report.Checks)
	}
	foundUnverified := false
	for _, check := range report.Checks {
		if check.ID == "subscriptions.images.unverified" {
			foundUnverified = true
			if !strings.Contains(strings.ToLower(check.Remediation), "could not be reached") {
				t.Fatalf("expected transport remediation to mention endpoint reachability, got %+v", check)
			}
		}
	}
	if !foundUnverified {
		t.Fatalf("expected subscriptions.images.unverified check, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsTreatsMetadataProbeFailuresAsInformational(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.subscriptionsByGroup["group-1"] = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"MISSING_METADATA"}}]}`
	fixture.groupLocalizationStatus = map[string]int{
		"group-1": http.StatusForbidden,
	}
	fixture.localizationsStatusBySub = map[string]int{
		"sub-1": http.StatusForbidden,
	}
	fixture.pricesStatusBySubscription = map[string]int{
		"sub-1": http.StatusForbidden,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected metadata probe failures to stay non-blocking, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.SubscriptionsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 {
		t.Fatalf("expected no blocking errors, got %+v", report.Summary)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.diagnostics.group_localization_unverified") {
		t.Fatalf("expected group localization unverified check, got %+v", report.Checks)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.diagnostics.localization_unverified") {
		t.Fatalf("expected localization unverified check, got %+v", report.Checks)
	}
	if !hasCheckWithID(report.Checks, "subscriptions.diagnostics.pricing_unverified") {
		t.Fatalf("expected pricing unverified check, got %+v", report.Checks)
	}
	if hasCheckWithID(report.Checks, "subscriptions.diagnostics.group_localization_missing") || hasCheckWithID(report.Checks, "subscriptions.diagnostics.localization_missing") {
		t.Fatalf("expected no false missing-metadata checks, got %+v", report.Checks)
	}
	if hasCheckWithID(report.Checks, "subscriptions.diagnostics.pricing_missing") {
		t.Fatalf("expected no false pricing-missing check, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsPropagatesCanceledPriceProbe(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.subscriptionsByGroup["group-1"] = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"MISSING_METADATA"}}]}`
	fixture.priceErrorBySubscription = map[string]error{
		"sub-1": context.Canceled,
	}

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout when pricing probe is canceled, got %q", stdout)
	}
	if runErr == nil {
		t.Fatal("expected canceled price probe to abort validation")
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", runErr)
	}
}

func TestValidateSubscriptionsFailsWhenSubscriptionGroupsForbidden(t *testing.T) {
	fixture := validValidateSubscriptionsFixture()
	fixture.subscriptionGroupsStatus = http.StatusForbidden

	client := newValidateSubscriptionsClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "subscriptions", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected validate subscriptions to fail when subscription groups cannot be read")
	}
}
