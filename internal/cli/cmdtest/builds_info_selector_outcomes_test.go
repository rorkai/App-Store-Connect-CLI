package cmdtest

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// buildsInfoSelectorTransport answers the read-only lookups behind app-scoped
// builds info selectors with a fixed pre-release version list and build list.
func buildsInfoSelectorTransport(t *testing.T, preReleaseVersionsBody, buildsBody string) http.RoundTripper {
	t.Helper()
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s %s", req.Method, req.URL.Path)
		}
		var body string
		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			body = preReleaseVersionsBody
		case "/v1/builds":
			body = buildsBody
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
}

func runBuildsInfoSelector(t *testing.T, transport http.RoundTripper, args ...string) (int, string, string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installDefaultTransport(t, transport)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run(append([]string{"builds", "info"}, args...), "1.2.3")
	})
	return code, stdout, stderr
}

const emptyCollectionBody = `{"data":[],"links":{}}`

func TestRun_BuildsInfoLatestWithNoBuildsReturnsExitNotFound(t *testing.T) {
	code, stdout, stderr := runBuildsInfoSelector(
		t,
		buildsInfoSelectorTransport(t, emptyCollectionBody, emptyCollectionBody),
		"--app", "123456789", "--latest", "--output", "json",
	)

	if code != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitNotFound, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "no builds found for app 123456789") {
		t.Fatalf("expected not-found message in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "--app") {
		t.Fatalf("expected stderr to name the selector flag to check, got %q", stderr)
	}
	if strings.Contains(stderr, "USAGE") {
		t.Fatalf("expected no usage page for a not-found result, got %q", stderr)
	}
}

func TestRun_BuildsInfoLatestVersionWithoutPreReleaseVersionReturnsExitNotFound(t *testing.T) {
	code, stdout, stderr := runBuildsInfoSelector(
		t,
		buildsInfoSelectorTransport(t, emptyCollectionBody, emptyCollectionBody),
		"--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "IOS", "--output", "json",
	)

	if code != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitNotFound, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `no pre-release version found for version "1.2.3" on platform IOS`) {
		t.Fatalf("expected pre-release not-found message in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "--version") || !strings.Contains(stderr, "--platform") {
		t.Fatalf("expected stderr to name --version and --platform, got %q", stderr)
	}
}

func TestRun_BuildsInfoLatestVersionWithNoMatchingBuildsReturnsExitNotFound(t *testing.T) {
	preReleaseVersions := `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}],"links":{}}`
	code, stdout, stderr := runBuildsInfoSelector(
		t,
		buildsInfoSelectorTransport(t, preReleaseVersions, emptyCollectionBody),
		"--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "IOS", "--output", "json",
	)

	if code != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitNotFound, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `no builds found for app 123456789 matching --version "1.2.3" and --platform IOS`) {
		t.Fatalf("expected filtered not-found message in stderr, got %q", stderr)
	}
}

func TestRun_BuildsInfoBuildNumberWithNoMatchReturnsExitNotFound(t *testing.T) {
	code, stdout, stderr := runBuildsInfoSelector(
		t,
		buildsInfoSelectorTransport(t, emptyCollectionBody, emptyCollectionBody),
		"--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json",
	)

	if code != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitNotFound, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `no build found for app 123456789 with build number "42" for platform IOS`) {
		t.Fatalf("expected build-number not-found message in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "--build-number") {
		t.Fatalf("expected stderr to name --build-number, got %q", stderr)
	}
}

func TestRun_BuildsInfoBuildNumberAmbiguousReturnsExitUsage(t *testing.T) {
	builds := `{"data":[
		{"type":"builds","id":"build-a","attributes":{"version":"42","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}},
		{"type":"builds","id":"build-b","attributes":{"version":"42","uploadedDate":"2026-03-12T00:00:00Z","processingState":"VALID","expired":false}}
	],"links":{}}`
	code, stdout, stderr := runBuildsInfoSelector(
		t,
		buildsInfoSelectorTransport(t, emptyCollectionBody, builds),
		"--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json",
	)

	if code != cmd.ExitUsage {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitUsage, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertUsageDiagnosticFirstLine(t, stderr, `2 builds match build number "42" for platform IOS for app 123456789; pass --build-id with one of:`)
}
