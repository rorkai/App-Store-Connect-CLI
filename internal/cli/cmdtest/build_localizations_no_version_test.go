package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Apple answers GET /v1/builds/{id}/appStoreVersion with 200 and a null data
// object for any build that is not attached to an App Store version, which is
// the normal state of a TestFlight-only build.
const buildWithoutAppStoreVersionBody = `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/appStoreVersion"}}`

func runBuildLocalizations(t *testing.T, args []string, register func(reg *asctest.Registry)) (int, string, string, []string) {
	t.Helper()
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	register(reg)

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run(append(append([]string{"build-localizations"}, args...), "--output", "json"), "1.2.3")
	})

	var requests []string
	for _, req := range reg.Requests() {
		requests = append(requests, req.Method+" "+req.URL.Path)
	}
	return exitCode, stdout, stderr, requests
}

func TestBuildLocalizationsListRejectsBuildWithoutAppStoreVersion(t *testing.T) {
	exitCode, stdout, stderr, requests := runBuildLocalizations(t, []string{"list", "--build-id", "build-1"}, func(reg *asctest.Registry) {
		reg.Register(asctest.GET("/v1/builds/build-1/appStoreVersion"), asctest.JSON(http.StatusOK, buildWithoutAppStoreVersionBody))
	})

	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "the selected build is not attached to an App Store version") {
		t.Fatalf("stderr = %q, want explanation that the build has no App Store version", stderr)
	}
	if !strings.Contains(stderr, "asc builds test-notes list") {
		t.Fatalf("stderr = %q, want pointer to asc builds test-notes for TestFlight notes", stderr)
	}
	if strings.Contains(stderr, "USAGE") {
		t.Fatalf("stderr = %q, want concise guidance without the full usage page", stderr)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %v, want only the appStoreVersion lookup", requests)
	}
}

func TestBuildLocalizationsCreateRejectsBuildWithoutAppStoreVersionBeforeMutating(t *testing.T) {
	exitCode, stdout, stderr, requests := runBuildLocalizations(t, []string{"create", "--build-id", "build-1", "--locale", "en-US", "--whats-new", "Bug fixes"}, func(reg *asctest.Registry) {
		reg.Register(asctest.GET("/v1/builds/build-1/appStoreVersion"), asctest.JSON(http.StatusOK, buildWithoutAppStoreVersionBody))
	})

	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "the selected build is not attached to an App Store version") {
		t.Fatalf("stderr = %q, want explanation that the build has no App Store version", stderr)
	}
	if !strings.Contains(stderr, "asc builds test-notes create") {
		t.Fatalf("stderr = %q, want pointer to asc builds test-notes create", stderr)
	}
	if !strings.Contains(stderr, `--locale "LOCALE" --whats-new "NOTES"`) {
		t.Fatalf("stderr = %q, want a complete builds test-notes create command", stderr)
	}
	if len(requests) != 1 || requests[0] != "GET /v1/builds/build-1/appStoreVersion" {
		t.Fatalf("requests = %v, want only the appStoreVersion lookup and no POST", requests)
	}
}

func TestBuildLocalizationsListSurfacesAppleBuildNotFound(t *testing.T) {
	exitCode, stdout, stderr, _ := runBuildLocalizations(t, []string{"list", "--build-id", "missing-build"}, func(reg *asctest.Registry) {
		reg.Register(asctest.GET("/v1/builds/missing-build/appStoreVersion"),
			asctest.JSON(http.StatusNotFound, `{"errors":[{"id":"e1","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'missing-build'"}]}`))
	})

	if exitCode != rootcmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitNotFound, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "There is no resource of type 'builds' with id 'missing-build'") {
		t.Fatalf("stderr = %q, want Apple error detail", stderr)
	}
}
