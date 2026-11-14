package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func runBuildsInfoSelector(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run(append([]string{"builds", "info"}, args...), "1.2.3")
	})
	return code, stdout, stderr
}

const emptyCollectionBody = `{"data":[],"links":{}}`

var (
	buildsByNumberIOS42      = asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=IOS", "filter[version]=42", "limit=200", "sort=-uploadedDate")
	preReleaseVersionIOS123  = asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=IOS", "filter[version]=1.2.3", "limit=200")
	emptyCollectionResponder = asctest.JSON(http.StatusOK, emptyCollectionBody)
)

func TestRun_BuildsInfoSelectorNotFoundReturnsExitNotFound(t *testing.T) {
	preReleaseVersion := `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}],"links":{}}`
	tests := []struct {
		name        string
		register    func(reg *asctest.Registry)
		args        []string
		wantStderr  []string
		waitCommand string
	}{
		{
			name: "latest with no builds",
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=200", "sort=-uploadedDate"), emptyCollectionResponder)
			},
			args:        []string{"--app", "123456789", "--latest", "--output", "json"},
			wantStderr:  []string{"no builds found for app 123456789", "--app"},
			waitCommand: "asc builds wait --app 123456789 --latest",
		},
		{
			name: "latest version without pre-release version",
			register: func(reg *asctest.Registry) {
				reg.Register(preReleaseVersionIOS123, emptyCollectionResponder)
			},
			args:        []string{"--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "IOS", "--output", "json"},
			wantStderr:  []string{`no pre-release version found for version "1.2.3" on platform IOS`, "--version", "--platform"},
			waitCommand: "asc builds wait --app 123456789 --latest --version 1.2.3 --platform IOS",
		},
		{
			name: "latest version with no matching builds",
			register: func(reg *asctest.Registry) {
				reg.Register(preReleaseVersionIOS123, asctest.JSON(http.StatusOK, preReleaseVersion))
				reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-1", "limit=1", "sort=-uploadedDate"), emptyCollectionResponder)
			},
			args:        []string{"--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "IOS", "--output", "json"},
			wantStderr:  []string{`no builds found for app 123456789 matching --version "1.2.3" and --platform IOS`},
			waitCommand: "asc builds wait --app 123456789 --latest --version 1.2.3 --platform IOS",
		},
		{
			name: "build number with no match",
			register: func(reg *asctest.Registry) {
				reg.Register(buildsByNumberIOS42, emptyCollectionResponder)
			},
			args:        []string{"--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json"},
			wantStderr:  []string{`no build found for app 123456789 with build number "42" for platform IOS`, "--build-number"},
			waitCommand: "asc builds wait --app 123456789 --build-number 42 --platform IOS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			test.register(reg)

			code, stdout, stderr := runBuildsInfoSelector(t, test.args...)

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
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(buildsByNumberIOS42, asctest.JSON(http.StatusOK, builds))

	code, stdout, stderr := runBuildsInfoSelector(t, "--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json")

	if code != cmd.ExitUsage {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitUsage, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertUsageDiagnosticFirstLine(t, stderr, `2 builds match build number "42" for platform IOS for app 123456789; pass --build-id with one of:`)
}
