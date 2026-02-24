package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateTestFlightFixture struct {
	app              string
	build            string
	buildApp         string
	betaReviewDetail string
	buildLocs        string
}

func newValidateTestFlightClient(t *testing.T, fixture validateTestFlightFixture) *asc.Client {
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
		switch path {
		case "/v1/apps/app-1":
			return jsonResponse(http.StatusOK, fixture.app)
		case "/v1/builds/build-1":
			if fixture.build != "" {
				return jsonResponse(http.StatusOK, fixture.build)
			}
			return jsonResponse(http.StatusNotFound, notFound)
		case "/v1/builds/build-1/app":
			return jsonResponse(http.StatusOK, fixture.buildApp)
		case "/v1/apps/app-1/betaAppReviewDetail":
			if fixture.betaReviewDetail != "" {
				return jsonResponse(http.StatusOK, fixture.betaReviewDetail)
			}
			return jsonResponse(http.StatusNotFound, notFound)
		case "/v1/builds/build-1/betaBuildLocalizations":
			if fixture.buildLocs != "" {
				return jsonResponse(http.StatusOK, fixture.buildLocs)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		}

		return jsonResponse(http.StatusNotFound, notFound)
	})

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func validValidateTestFlightFixture() validateTestFlightFixture {
	return validateTestFlightFixture{
		app:              `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`,
		build:            `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0","processingState":"VALID","expired":false}}}`,
		buildApp:         `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`,
		betaReviewDetail: `{"data":{"type":"betaAppReviewDetails","id":"beta-detail-1","attributes":{"contactFirstName":"A","contactLastName":"B","contactEmail":"a@example.com","contactPhone":"123","demoAccountRequired":false}}}`,
		buildLocs:        `{"data":[{"type":"betaBuildLocalizations","id":"bbl-1","attributes":{"locale":"en-US","whatsNew":"Test this build"}}]}`,
	}
}

func TestValidateTestFlightRequiresAppAndBuild(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"validate", "testflight", "--build", "build-1"},
			wantErr: "--app is required",
		},
		{
			name:    "missing build",
			args:    []string{"validate", "testflight", "--app", "app-1"},
			wantErr: "--build is required",
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

func TestValidateTestFlightOutputsJSONAndTable(t *testing.T) {
	fixture := validValidateTestFlightFixture()
	client := newValidateTestFlightClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "testflight", "--app", "app-1", "--build", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.TestFlightReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("expected no issues, got %+v", report.Summary)
	}

	root = RootCommand("1.2.3")
	stdout, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "testflight", "--app", "app-1", "--build", "build-1", "--output", "table"}); err != nil {
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

func TestValidateTestFlightFailsWhenBetaReviewDetailsMissing(t *testing.T) {
	fixture := validValidateTestFlightFixture()
	fixture.betaReviewDetail = ""

	client := newValidateTestFlightClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "testflight", "--app", "app-1", "--build", "build-1"}); err != nil {
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

	var report validation.TestFlightReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "testflight.review_details.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected testflight.review_details.missing check, got %+v", report.Checks)
	}
}

func TestValidateTestFlightFailsWhenWhatsNewMissing(t *testing.T) {
	fixture := validValidateTestFlightFixture()
	fixture.buildLocs = `{"data":[{"type":"betaBuildLocalizations","id":"bbl-1","attributes":{"locale":"en-US","whatsNew":""}}]}`

	client := newValidateTestFlightClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "testflight", "--app", "app-1", "--build", "build-1"}); err != nil {
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

	var report validation.TestFlightReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "testflight.whats_new.missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected testflight.whats_new.missing check, got %+v", report.Checks)
	}
}

func TestValidateTestFlightFailsWhenBuildAppMismatch(t *testing.T) {
	fixture := validValidateTestFlightFixture()
	fixture.buildApp = `{"data":{"type":"apps","id":"app-2"}}`

	client := newValidateTestFlightClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"validate", "testflight", "--app", "app-1", "--build", "build-1"}); err != nil {
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

	var report validation.TestFlightReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID == "testflight.build.app_mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected testflight.build.app_mismatch check, got %+v", report.Checks)
	}
}
