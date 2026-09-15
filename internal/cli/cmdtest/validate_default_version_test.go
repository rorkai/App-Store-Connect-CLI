package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
)

const (
	editableVersionStateQuery = "filter[appVersionState]=DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED,WAITING_FOR_REVIEW"
	liveVersionStateQuery     = "filter[appStoreState]=READY_FOR_SALE"
	// The live tier queries both state spellings; see defaultLiveAppVersionStates.
	liveVersionModernStateQuery = "filter[appVersionState]=READY_FOR_DISTRIBUTION"
)

func runValidateWithFixture(t *testing.T, fixture validateFixture, args ...string) (string, string, error) {
	t.Helper()
	client := newValidateTestClient(t, fixture)
	restore := validate.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)

	root := RootCommand("1.2.3")
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
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
		liveVersionStateQuery:       `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":""}}`,
		liveVersionModernStateQuery: `{"data":[],"links":{"next":""}}`,
	}

	_, stderr, runErr := runValidateWithFixture(t, fixture, "validate", "--app", "app-1")
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	wantNote := "Using version 1.0 (READY_FOR_DISTRIBUTION) for platform IOS; pass --version to override\n"
	if stderr != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr, wantNote)
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
	for _, want := range []string{"IOS 1.0 (PREPARE_FOR_SUBMISSION)", "MAC_OS 2.0 (DEVELOPER_REJECTED)", "--platform"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected %q in stderr %q", want, stderr)
		}
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
