package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	editableVersionStateQuery = "filter[appVersionState]=DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED,WAITING_FOR_REVIEW"
	removedEditableStateQuery = "filter[appStoreState]=DEVELOPER_REMOVED_FROM_SALE"
	liveVersionStateQuery     = "filter[appStoreState]=READY_FOR_SALE"
	// The live tier queries both state spellings; see shared.LiveAppStoreVersionStateFilter.
	liveVersionModernStateQuery = "filter[appVersionState]=READY_FOR_DISTRIBUTION"
)

func runValidateWithFixture(t *testing.T, fixture validateFixture, args ...string) (string, string, error) {
	t.Helper()
	client := newValidateTestClient(t, fixture)
	return runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, args...)
}

func TestValidateDefaultsToEditableVersionWhenVersionOmitted(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery: `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z","copyright":"2026 Test Company"}}],"links":{"next":""}}`,
	}

	stdout, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--output", "json")
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	wantNote := "Using version 1.0 (PREPARE_FOR_SUBMISSION) for platform IOS; pass --version to override\n"
	if stderr != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr, wantNote)
	}
	var report struct {
		VersionID     string `json:"versionId"`
		VersionString string `json:"versionString"`
		Platform      string `json:"platform"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\nstdout=%q", err, stdout)
	}
	if report.VersionID != "ver-1" || report.VersionString != "1.0" || report.Platform != "IOS" {
		t.Fatalf("report = %+v, want ver-1/1.0/IOS", report)
	}
}

func TestValidateDefaultsToLiveVersionWhenNoEditableVersion(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery:   `{"data":[],"links":{"next":""}}`,
		removedEditableStateQuery:   `{"data":[],"links":{"next":""}}`,
		liveVersionStateQuery:       `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":""}}`,
		liveVersionModernStateQuery: `{"data":[],"links":{"next":""}}`,
	}
	fixture.version = `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"READY_FOR_DISTRIBUTION","copyright":"2026 Test Company"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`

	stdout, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--output", "json")
	if runErr == nil {
		t.Fatal("expected live version readiness to remain blocked")
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; err=%v", got, rootcmd.ExitError, runErr)
	}
	wantNote := "Using version 1.0 (READY_FOR_DISTRIBUTION) for platform IOS; pass --version to override\n"
	if stderr != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr, wantNote)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\nstdout=%q", err, stdout)
	}
	if !hasCheckWithID(report.Checks, "version.state.editable") {
		t.Fatalf("expected live version state blocker, got %#v", report.Checks)
	}
}

func TestValidateDefaultsToReadyForReviewWithoutStateBlocker(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery: `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"READY_FOR_REVIEW","createdDate":"2026-02-01T00:00:00Z","copyright":"2026 Test Company"}}],"links":{"next":""}}`,
	}
	fixture.version = `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"READY_FOR_REVIEW","copyright":"2026 Test Company"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`

	stdout, _, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--output", "json")
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\nstdout=%q", err, stdout)
	}
	if hasCheckWithID(report.Checks, "version.state.editable") {
		t.Fatalf("did not expect READY_FOR_REVIEW state blocker, got %#v", report.Checks)
	}
}

func TestValidateDeepDefaultVersionReturnsAgreementFallbackDuringDiscovery(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "off")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery: `{"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED","title":"A required agreement is missing or has expired","detail":"This request requires an in-effect agreement."}]}`,
	}
	fixture.versionsStatusByQuery = map[string]int{editableVersionStateQuery: 403}
	var paths []string
	fixture.requestObserver = func(req *http.Request) { paths = append(paths, req.URL.Path) }

	stdout, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--deep", "--output", "json")
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; err=%v", got, rootcmd.ExitError, runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no selection note", stderr)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\nstdout=%q", err, stdout)
	}
	if report.VersionID != "" || report.VersionString != "" || report.Platform != "" {
		t.Fatalf("discovery fallback invented version fields: %#v", report)
	}
	if !hasCheckWithID(report.Checks, "agreements.public_api.blocked") {
		t.Fatalf("agreement fallback missing from %#v", report.Checks)
	}
	if len(paths) != 1 || paths[0] != "/v1/apps/app-1/appStoreVersions" {
		t.Fatalf("requests = %#v, want only default-version discovery", paths)
	}
}

func TestValidateDefaultVersionDiscoveryAgreementErrorWithoutDeepStaysAPIError(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery: `{"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED","title":"A required agreement is missing or has expired","detail":"This request requires an in-effect agreement."}]}`,
	}
	fixture.versionsStatusByQuery = map[string]int{editableVersionStateQuery: 403}

	stdout, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--output", "json")
	if runErr == nil || !asc.IsRequiredAgreementError(runErr) {
		t.Fatalf("expected required-agreement API error, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestValidateDeepDefaultVersionDoesNotFallbackForSelectionErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "off")
	tests := []struct {
		name       string
		versions   map[string]string
		wantExit   int
		wantStderr string
	}{
		{
			name: "ambiguous",
			versions: map[string]string{
				editableVersionStateQuery: `{"data":[
					{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}},
					{"type":"appStoreVersions","id":"ver-mac","attributes":{"platform":"MAC_OS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
				],"links":{"next":""}}`,
			},
			wantExit:   rootcmd.ExitUsage,
			wantStderr: "--platform",
		},
		{
			name: "not found",
			versions: map[string]string{
				editableVersionStateQuery:   `{"data":[],"links":{"next":""}}`,
				removedEditableStateQuery:   `{"data":[],"links":{"next":""}}`,
				liveVersionStateQuery:       `{"data":[],"links":{"next":""}}`,
				liveVersionModernStateQuery: `{"data":[],"links":{"next":""}}`,
			},
			wantExit: rootcmd.ExitNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := validValidateFixture()
			fixture.versions = ""
			fixture.versionsByQuery = test.versions
			stdout, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--deep", "--output", "json")
			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d; err=%v", got, test.wantExit, runErr)
			}
			if stdout != "" {
				t.Fatalf("unexpected fallback report: %q", stdout)
			}
			if test.wantStderr != "" && !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestValidateDefaultVersionRequiresPlatformWhenAmbiguous(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery: `{"data":[
			{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}},
			{"type":"appStoreVersions","id":"ver-mac","attributes":{"platform":"MAC_OS","versionString":"2.0","appVersionState":"DEVELOPER_REJECTED","createdDate":"2026-01-01T00:00:00Z"}}
		],"links":{"next":""}}`,
		editableVersionStateQuery + "&filter[platform]=IOS": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}],"links":{"next":""}}`,
	}

	stdout, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1")
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected reported usage error, got %v", runErr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("expected no report, got %q", stdout)
	}
	for _, want := range []string{"IOS", "version 1.0 (ver-1)", "MAC_OS", "version 2.0 (ver-mac)", "--platform"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected %q in stderr %q", want, stderr)
		}
	}
	if strings.Contains(stderr, "pass --version") || strings.Contains(stderr, "Usage:") {
		t.Fatalf("stderr advertises an ineffective selector or dumps full help: %q", stderr)
	}

	_, stderr, runErr = runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--platform", "IOS")
	if runErr != nil {
		t.Fatalf("Run() with --platform error = %v", runErr)
	}
	if !strings.HasPrefix(stderr, "Using version 1.0 (PREPARE_FOR_SUBMISSION) for platform IOS;") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestValidateDefaultVersionErrorsWhenNoVersionExists(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		editableVersionStateQuery:   `{"data":[],"links":{"next":""}}`,
		removedEditableStateQuery:   `{"data":[],"links":{"next":""}}`,
		liveVersionStateQuery:       `{"data":[],"links":{"next":""}}`,
		liveVersionModernStateQuery: `{"data":[],"links":{"next":""}}`,
	}

	stdout, _, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1")
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime error, got %v", runErr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitNotFound)
	}
	if !strings.Contains(runErr.Error(), `no editable or live App Store version found for app "app-1"`) {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected no report, got %q", stdout)
	}
}

func TestValidateExplicitVersionSkipsDefaultResolution(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	fixture := validValidateFixture()
	fixture.versions = ""
	fixture.versionsByQuery = map[string]string{
		"filter[versionString]=1.0": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","copyright":"2026 Test Company"}}]}`,
	}

	_, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1", "--version", "1.0")
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected no default-version note for explicit --version, got %q", stderr)
	}
}
