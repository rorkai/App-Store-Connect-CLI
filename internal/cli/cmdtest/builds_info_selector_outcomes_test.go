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

func TestRun_BuildsInfoSelectorNotFoundReturnsExitNotFound(t *testing.T) {
	preReleaseVersion := `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}],"links":{}}`
	tests := []struct {
		name               string
		preReleaseVersions string
		args               []string
		wantStderr         []string
		waitCommand        string
	}{
		{
			name:               "latest with no builds",
			preReleaseVersions: emptyCollectionBody,
			args:               []string{"--app", "123456789", "--latest", "--output", "json"},
			wantStderr:         []string{"no builds found for app 123456789", "--app"},
			waitCommand:        "asc builds wait --app 123456789 --latest",
		},
		{
			name:               "latest version without pre-release version",
			preReleaseVersions: emptyCollectionBody,
			args:               []string{"--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "IOS", "--output", "json"},
			wantStderr:         []string{`no pre-release version found for version "1.2.3" on platform IOS`, "--version", "--platform"},
			waitCommand:        "asc builds wait --app 123456789 --latest --version 1.2.3 --platform IOS",
		},
		{
			name:               "latest version with no matching builds",
			preReleaseVersions: preReleaseVersion,
			args:               []string{"--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "IOS", "--output", "json"},
			wantStderr:         []string{`no builds found for app 123456789 matching --version "1.2.3" and --platform IOS`},
			waitCommand:        "asc builds wait --app 123456789 --latest --version 1.2.3 --platform IOS",
		},
		{
			name:               "build number with no match",
			preReleaseVersions: emptyCollectionBody,
			args:               []string{"--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json"},
			wantStderr:         []string{`no build found for app 123456789 with build number "42" for platform IOS`, "--build-number"},
			waitCommand:        "asc builds wait --app 123456789 --build-number 42 --platform IOS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runBuildsInfoSelector(
				t,
				buildsInfoSelectorTransport(t, test.preReleaseVersions, emptyCollectionBody),
				test.args...,
			)

			if code != cmd.ExitNotFound {
				t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitNotFound, code, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
			assertBuildsWaitHint(t, stderr, test.waitCommand)
			if strings.Contains(stderr, "USAGE") {
				t.Fatalf("expected no usage page for a not-found result, got %q", stderr)
			}
		})
	}
}

func assertBuildsWaitHint(t *testing.T, stderr, waitCommand string) {
	t.Helper()
	want := "a just-uploaded build may still be processing; wait for it with `" + waitCommand + "`"
	if !strings.Contains(stderr, want) {
		t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
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
