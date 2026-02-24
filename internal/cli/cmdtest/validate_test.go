package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateFixture struct {
	app                  string
	versions             string
	version              string
	appInfos             string
	appInfoLocs          string
	versionLocs          string
	ageRating            string
	reviewDetails        string
	primaryCategory      string
	build                string
	priceSchedule        string
	availabilityV2       string
	availabilityV2Status int
	territories          string
	screenshotSets       map[string]string
	screenshotsBySet     map[string]string
}

func newValidateTestClient(t *testing.T, fixture validateFixture) *asc.Client {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")
	writeECDSAPEM(t, keyPath)

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return jsonResponse(http.StatusMethodNotAllowed, `{"errors":[{"status":405}]}`)
		}

		path := req.URL.Path
		switch {
		case path == "/v1/apps/app-1":
			return jsonResponse(http.StatusOK, fixture.app)
		case path == "/v1/apps/app-1/appStoreVersions":
			if fixture.versions != "" {
				return jsonResponse(http.StatusOK, fixture.versions)
			}
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":404}]}`)
		case path == "/v1/appStoreVersions/ver-1":
			return jsonResponse(http.StatusOK, fixture.version)
		case path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, fixture.appInfos)
		case path == "/v1/appInfos/info-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, fixture.appInfoLocs)
		case path == "/v1/appInfos/info-1/relationships/primaryCategory":
			if fixture.primaryCategory != "" {
				return jsonResponse(http.StatusOK, fixture.primaryCategory)
			}
			return jsonResponse(http.StatusOK, `{"data":null}`)
		case path == "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, fixture.versionLocs)
		case path == "/v1/appStoreVersions/ver-1/ageRatingDeclaration":
			return jsonResponse(http.StatusOK, fixture.ageRating)
		case path == "/v1/appStoreVersions/ver-1/appStoreReviewDetail":
			if fixture.reviewDetails != "" {
				return jsonResponse(http.StatusOK, fixture.reviewDetails)
			}
			return jsonResponse(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`)
		case path == "/v1/appStoreVersions/ver-1/build":
			if fixture.build != "" {
				return jsonResponse(http.StatusOK, fixture.build)
			}
			return jsonResponse(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`)
		case path == "/v1/apps/app-1/appPriceSchedule":
			if fixture.priceSchedule != "" {
				return jsonResponse(http.StatusOK, fixture.priceSchedule)
			}
			return jsonResponse(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`)
		case path == "/v1/apps/app-1/appAvailabilityV2":
			if fixture.availabilityV2Status != 0 {
				return jsonResponse(fixture.availabilityV2Status, fixture.availabilityV2)
			}
			if fixture.availabilityV2 != "" {
				return jsonResponse(http.StatusOK, fixture.availabilityV2)
			}
			return jsonResponse(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`)
		case strings.HasPrefix(path, "/v2/appAvailabilities/") && strings.HasSuffix(path, "/territoryAvailabilities"):
			if fixture.territories != "" {
				return jsonResponse(http.StatusOK, fixture.territories)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasPrefix(path, "/v1/appStoreVersionLocalizations/") && strings.HasSuffix(path, "/appScreenshotSets"):
			localizationID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/appStoreVersionLocalizations/"), "/appScreenshotSets")
			if body, ok := fixture.screenshotSets[localizationID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
		case strings.HasPrefix(path, "/v1/appScreenshotSets/") && strings.HasSuffix(path, "/appScreenshots"):
			setID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/appScreenshotSets/"), "/appScreenshots")
			if body, ok := fixture.screenshotsBySet[setID]; ok {
				return jsonResponse(http.StatusOK, body)
			}
		}

		return jsonResponse(http.StatusNotFound, `{"errors":[{"status":404}]}`)
	})

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func jsonResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func hasCheckWithID(checks []validation.CheckResult, id string) bool {
	for _, check := range checks {
		if check.ID == id {
			return true
		}
	}
	return false
}

func validValidateFixture() validateFixture {
	return validateFixture{
		app:             `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`,
		versions:        `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"}}]}`,
		version:         `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}}`,
		appInfos:        `{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`,
		appInfoLocs:     `{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"My App","subtitle":"Subtitle","privacyPolicyUrl":"https://example.com/privacy"}}]}`,
		versionLocs:     `{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","whatsNew":"Notes","promotionalText":"Promo","supportUrl":"https://support.example.com","marketingUrl":"https://marketing.example.com"}}]}`,
		reviewDetails:   `{"data":{"type":"appStoreReviewDetails","id":"review-detail-1","attributes":{"contactFirstName":"A","contactLastName":"B","contactEmail":"a@example.com","contactPhone":"123","demoAccountName":"","demoAccountPassword":"","demoAccountRequired":false,"notes":"Review notes"}}}`,
		primaryCategory: `{"data":{"type":"appCategories","id":"cat-1"}}`,
		build:           `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0","processingState":"VALID","expired":false}}}`,
		priceSchedule:   `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`,
		availabilityV2:  `{"data":{"type":"appAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`,
		territories:     `{"data":[{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true}}]}`,
		ageRating: `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{
			"advertising":false,
			"gambling":false,
			"healthOrWellnessTopics":false,
			"lootBox":false,
			"messagingAndChat":true,
			"parentalControls":true,
			"ageAssurance":false,
			"unrestrictedWebAccess":false,
			"userGeneratedContent":true,
			"alcoholTobaccoOrDrugUseOrReferences":"NONE",
			"contests":"NONE",
			"gamblingSimulated":"NONE",
			"gunsOrOtherWeapons":"NONE",
			"medicalOrTreatmentInformation":"NONE",
			"profanityOrCrudeHumor":"NONE",
			"sexualContentGraphicAndNudity":"NONE",
			"sexualContentOrNudity":"NONE",
			"horrorOrFearThemes":"NONE",
			"matureOrSuggestiveThemes":"NONE",
			"violenceCartoonOrFantasy":"NONE",
			"violenceRealistic":"NONE",
			"violenceRealisticProlongedGraphicOrSadistic":"NONE"
		}}}`,
		screenshotSets: map[string]string{
			"ver-loc-1": `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`,
		},
		screenshotsBySet: map[string]string{
			"set-1": `{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":1024,"imageAsset":{"width":1242,"height":2688}}}]}`,
		},
	}
}

func TestValidateRequiresAppAndVersionSelector(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"validate", "--version-id", "ver-1"},
			wantErr: "--app is required",
		},
		{
			name:    "missing version id",
			args:    []string{"validate", "--app", "app-1"},
			wantErr: "--version or --version-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestValidateVersionAndVersionIDMutuallyExclusive(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version", "1.0", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestValidateOutputsJSONAndTable(t *testing.T) {
	fixture := validValidateFixture()
	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("expected no issues, got %+v", report.Summary)
	}

	root = RootCommand("1.2.3")
	stdout, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1", "--output", "table"}); err != nil {
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

func TestValidateSupportsVersionLookup(t *testing.T) {
	fixture := validValidateFixture()
	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version", "1.0", "--platform", "IOS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.VersionID != "ver-1" {
		t.Fatalf("expected resolved version ID ver-1, got %q", report.VersionID)
	}
}

func TestValidateStrictExitBehavior(t *testing.T) {
	fixture := validValidateFixture()
	fixture.appInfoLocs = `{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"My App"}}]}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	root = RootCommand("1.2.3")
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1", "--strict"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error with --strict")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %v", err)
		}
	})
}

func TestValidateWarnsWhenPrivacyPolicyURLMissing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.appInfoLocs = `{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"My App","subtitle":"Subtitle"}}]}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 {
		t.Fatalf("expected zero errors, got %+v", report.Summary)
	}
	if !hasCheckWithID(report.Checks, "metadata.recommended.privacy_policy_url") {
		t.Fatalf("expected privacy policy warning, got %+v", report.Checks)
	}
}

func TestValidateFailsForNonEditableVersionState(t *testing.T) {
	fixture := validValidateFixture()
	fixture.version = `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"WAITING_FOR_REVIEW"}}}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !hasCheckWithID(report.Checks, "version.state.editable") {
		t.Fatalf("expected version state check, got %+v", report.Checks)
	}
}

func TestValidateSkipsWhatsNewOnInitialRelease(t *testing.T) {
	fixture := validValidateFixture()
	// Simulate an initial v1.0 release where Apple doesn't allow "What's New".
	// The API can return an empty or missing `whatsNew` field; either way it
	// should not produce a warning.
	fixture.versionLocs = `{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","promotionalText":"Promo","supportUrl":"https://support.example.com","marketingUrl":"https://marketing.example.com"}}]}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1", "--strict"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected no error with --strict, got %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("expected no issues, got %+v", report.Summary)
	}
	for _, check := range report.Checks {
		if check.ID == "metadata.required.whats_new" {
			t.Fatalf("did not expect metadata.required.whats_new check for initial release")
		}
	}
}

func TestValidateMixedWarningAndError(t *testing.T) {
	fixture := validValidateFixture()
	fixture.versionLocs = `{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US","description":"","keywords":"keyword","supportUrl":"https://support.example.com"}}]}`
	fixture.appInfoLocs = `{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"My App"}}]}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error with mixed issues")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %v", err)
		}
	})
}

func TestValidateFailsWhenNoScreenshotSets(t *testing.T) {
	fixture := validValidateFixture()
	fixture.screenshotSets = map[string]string{
		"ver-loc-1": `{"data":[]}`,
	}
	fixture.screenshotsBySet = map[string]string{}

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "screenshots.required.any" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected screenshots.required.any check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenScreenshotSetIsEmpty(t *testing.T) {
	fixture := validValidateFixture()
	fixture.screenshotsBySet = map[string]string{
		"set-1": `{"data":[]}`,
	}

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "screenshots.required.set_nonempty" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected screenshots.required.set_nonempty check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenReviewDetailsMissing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.reviewDetails = ""

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "review_details.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected review_details.missing check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenReviewDetailsMissingContactEmail(t *testing.T) {
	fixture := validValidateFixture()
	fixture.reviewDetails = `{"data":{"type":"appStoreReviewDetails","id":"review-detail-1","attributes":{"contactFirstName":"A","contactLastName":"B","contactEmail":"","contactPhone":"123","demoAccountRequired":false}}}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "review_details.missing_field" && check.Field == "contactEmail" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected review_details.missing_field for contactEmail, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenPrimaryCategoryMissing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.primaryCategory = `{"data":null}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "categories.primary_missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected categories.primary_missing check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenBuildMissing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.build = ""

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "build.required.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected build.required.missing check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenBuildMissingWithNullData(t *testing.T) {
	fixture := validValidateFixture()
	fixture.build = `{"data":null}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "build.required.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected build.required.missing check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenBuildIsProcessing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.build = `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0","processingState":"PROCESSING","expired":false}}}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "build.invalid.processing_state" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected build.invalid.processing_state check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenPriceScheduleMissing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.priceSchedule = ""

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "pricing.schedule.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pricing.schedule.missing check, got %+v", report.Checks)
	}
}

func TestValidateTreatsAppAvailabilityMissingNon404AsMissing(t *testing.T) {
	fixture := validValidateFixture()
	fixture.availabilityV2Status = http.StatusConflict
	fixture.availabilityV2 = `{"errors":[{"code":"RESOURCE_DOES_NOT_EXIST","title":"Resource does not exist","detail":"The resource AppAvailabilities does not exist."}]}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "availability.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected availability.missing check, got %+v", report.Checks)
	}
}

func TestValidateFailsWhenNoTerritoriesAvailable(t *testing.T) {
	fixture := validValidateFixture()
	fixture.territories = `{"data":[{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":false}}]}`

	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "--app", "app-1", "--version-id", "ver-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "availability.territories.none" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected availability.territories.none check, got %+v", report.Checks)
	}
}
