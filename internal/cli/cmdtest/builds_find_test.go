package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const buildNumberRequiresPlatformError = "Error: --platform is required with --build-number"

func TestBuildsByBuildNumberRequiresPlatform(t *testing.T) {
	for _, args := range [][]string{
		{"builds", "info", "--app", "123456789", "--build-number", "42", "--output", "json"},
		{"builds", "wait", "--app", "123456789", "--build-number", "42", "--poll-interval", "1ms"},
	} {
		t.Run(args[1], func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			asctest.NewRegistry(t).Install()

			stdout, stderr, runErr := runCommand(t, args)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, buildNumberRequiresPlatformError) {
				t.Fatalf("expected stderr to require --platform, got %q", stderr)
			}
			if strings.Contains(stderr, "Defaulting to IOS") {
				t.Fatalf("expected no implicit IOS default, got %q", stderr)
			}
		})
	}
}

func TestBuildsInfoByBuildNumberSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=IOS", "filter[version]=42", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"PROCESSING"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-42/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-42"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"preReleaseVersions"`) {
		t.Fatalf("expected attached pre-release version output, got %q", stdout)
	}
}

func TestBuildsInfoByBuildNumberExplicitPlatformNarrowsResults(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=TV_OS", "filter[version]=42", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-tv","attributes":{"version":"42","processingState":"VALID"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-tv/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-tv","attributes":{"version":"1.2.3","platform":"TV_OS"}}}`))

	_, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--build-number", "42", "--platform", "TV_OS", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestBuildsInfoByLatestSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-latest","attributes":{"version":"99","processingState":"VALID"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-latest/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-latest","attributes":{"version":"3.4.5","platform":"IOS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-latest"`) {
		t.Fatalf("expected latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestNormalizesLowercasePlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=IOS", "filter[version]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-ios", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-ios","attributes":{"version":"88","processingState":"VALID"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-ios/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "ios", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-ios"`) {
		t.Fatalf("expected normalized-platform latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionWithoutPlatformSelectsNewestAcrossPlatforms(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}},{"type":"preReleaseVersions","id":"prv-macos","attributes":{"version":"1.2.3","platform":"MAC_OS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-ios", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-ios-old","attributes":{"version":"100","uploadedDate":"2026-03-01T10:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-macos", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-macos-new","attributes":{"version":"101","uploadedDate":"2026-03-02T10:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-macos-new/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-macos","attributes":{"version":"1.2.3","platform":"MAC_OS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.2.3", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-macos-new"`) {
		t.Fatalf("expected newest cross-platform latest build, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionCollectsEquivalentPlatformVersions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=1.1,1.1.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}},{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-exact", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"101","uploadedDate":"2026-03-03T10:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-near", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/builds/build-exact/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-exact"`) {
		t.Fatalf("expected exact-version latest build output, got %q", stdout)
	}
	var buildFilters []string
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/builds" {
			buildFilters = append(buildFilters, req.URL.Query().Get("filter[preReleaseVersion]"))
		}
	}
	if len(buildFilters) != 2 || buildFilters[0] != "prv-exact" || buildFilters[1] != "prv-near" {
		t.Fatalf("expected both equivalent platform trains to be inspected, got %v", buildFilters)
	}
}

func TestBuildsInfoByLatestVersionKeepsServerMatchedPreReleaseVersionsWithoutAttributes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=1.1,1.1.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-server","attributes":{}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-server", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-server","attributes":{"version":"101","uploadedDate":"2026-03-03T10:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-server/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-server","attributes":{"version":"1.1","platform":"IOS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-server"`) {
		t.Fatalf("expected server-matched latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionAndPlatformPaginatesPastNearMatches(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=page-2"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=IOS", "filter[version]=1.1", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v1/preReleaseVersions", "cursor=page-2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-exact", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact-ios","attributes":{"version":"101","uploadedDate":"2026-03-03T10:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-exact-ios/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"IOS"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.1", "--platform", "IOS", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-exact-ios"`) {
		t.Fatalf("expected paginated exact-version latest build output, got %q", stdout)
	}
}

func TestBuildsFindAliasIsRemoved(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "find", "--app", "123456789", "--build-number", "42", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Manage builds in App Store Connect.") {
		t.Fatalf("expected builds help after deprecated command removal, got %q", stderr)
	}
	if strings.Contains(stderr, "\n  find\t") || strings.Contains(stderr, "\n  find ") {
		t.Fatalf("expected builds find to stay omitted from help, got %q", stderr)
	}
}
