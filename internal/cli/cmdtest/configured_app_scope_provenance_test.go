package cmdtest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const (
	configuredAppBundleCollision   = "com.example.saved"
	configuredAppBundle            = "com.example.fixture"
	configuredBundleCollisionError = `no app found for bundle ID "` + configuredAppBundleCollision + `" (exact match required)`
)

func configureAppSelector(t *testing.T, source, value string) {
	t.Helper()
	setupAuth(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	if source == "environment" {
		t.Setenv("ASC_APP_ID", value)
		return
	}
	previous, hadPrevious := os.LookupEnv("ASC_APP_ID")
	if err := os.Unsetenv("ASC_APP_ID"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadPrevious {
			_ = os.Setenv("ASC_APP_ID", previous)
		} else {
			_ = os.Unsetenv("ASC_APP_ID")
		}
	})
	if err := os.WriteFile(configPath, []byte(`{"app_id":"`+value+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func configureSavedAppCollision(t *testing.T, source string) {
	configureAppSelector(t, source, configuredAppBundleCollision)
}

func configuredBundleCollisionRegistry(t *testing.T) *asctest.Registry {
	t.Helper()
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]="+configuredAppBundleCollision, "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	return reg
}

func configuredBundlePositiveRegistry(t *testing.T) *asctest.Registry {
	t.Helper()
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]="+configuredAppBundle, "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-configured","attributes":{"bundleId":"`+configuredAppBundle+`","name":"Configured Fixture"}}]}`))
	reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=app-configured", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"product-configured"}]}`))
	reg.Register(asctest.GET("/v1/ciProducts/product-configured/workflows", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"workflow-configured","attributes":{"name":"CI"}}],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"workflow-configured"}},"sourceBranchOrTag":{"data":{"type":"scmGitReferences","id":"ref-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-configured","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-14T00:00:00Z"}}}`))
	return reg
}

func assertConfiguredBundleCollisionStopsBeforeContent(t *testing.T, args ...string) {
	t.Helper()
	for _, source := range []string{"environment", "saved configuration"} {
		t.Run(source, func(t *testing.T) {
			configureSavedAppCollision(t, source)
			reg := configuredBundleCollisionRegistry(t)

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err == nil {
				t.Fatalf("expected scoped app lookup failure, got %v", err)
			}
			if !strings.Contains(err.Error(), configuredBundleCollisionError) {
				t.Fatalf("error = %v, want %q", err, configuredBundleCollisionError)
			}
			if stdout != "" {
				t.Fatalf("expected no stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected no stderr, got %q", stderr)
			}
			if requests := reg.Requests(); len(requests) != 1 {
				t.Fatalf("expected only the exact bundle lookup, got %d requests: %v", len(requests), requests)
			}
		})
	}
}

func TestConfiguredBundleCollisionStopsBeforeNamedXcodeCloudRun(t *testing.T) {
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"xcode-cloud", "run", "--workflow", "CI", "--branch", "main")
}

func TestConfiguredBundleCollisionStopsBeforeXcodeCloudProducts(t *testing.T) {
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"xcode-cloud", "products", "list")
}

func TestConfiguredBundleCollisionStopsBeforeXcodeCloudWorkflows(t *testing.T) {
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"xcode-cloud", "workflows", "list")
}

func TestConfiguredBundleReachesNamedXcodeCloudRun(t *testing.T) {
	for _, source := range []string{"environment", "saved configuration"} {
		t.Run(source, func(t *testing.T) {
			configureAppSelector(t, source, configuredAppBundle)
			reg := configuredBundlePositiveRegistry(t)

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
				"xcode-cloud", "run", "--workflow", "CI", "--git-reference-id", "ref-1")
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" || !strings.Contains(stdout, `"buildRunId":"run-configured"`) {
				t.Fatalf("stdout=%q stderr=%q, want configured build run", stdout, stderr)
			}
			if requests := reg.Requests(); len(requests) != 4 {
				t.Fatalf("expected app, product, workflow and build requests, got %d: %v", len(requests), requests)
			}
		})
	}
}

func TestConfiguredBundleCollisionStopsBeforeScreenshotsList(t *testing.T) {
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"screenshots", "list", "--version", "1.2.3", "--output", "json")
}

func TestConfiguredBundleCollisionStopsBeforeScreenshotsUploadReplace(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePNG(t, filepath.Join(localeDir, "01-home.png"), 1284, 2778)

	args := []string{"screenshots", "upload", "--version-id", "version-1", "--path", root, "--device-type", "IPHONE_65", "--output", "json"}
	t.Run("ordinary upload", func(t *testing.T) {
		assertConfiguredBundleCollisionStopsBeforeContent(t, args...)
	})
	for _, source := range []string{"environment", "saved configuration"} {
		t.Run("replace with "+source, func(t *testing.T) {
			configureSavedAppCollision(t, source)
			reg := asctest.NewRegistry(t)
			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), append(args, "--replace", "--confirm")...)
			if err == nil || !strings.Contains(err.Error(), "--replace requires a canonical numeric app ID") || stdout != "" {
				t.Fatalf("expected numeric app refusal, got stdout=%q err=%v", stdout, err)
			}
			if requests := reg.Requests(); len(requests) != 0 {
				t.Fatalf("unexpected requests before numeric app selection: %v", requests)
			}
		})
	}
}

func TestConfiguredBundleCollisionStopsBeforeScreenshotsApply(t *testing.T) {
	reviewDir, _ := writeScreenshotReviewArtifacts(t)
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"screenshots", "apply", "--version", "1.2.3", "--review-output-dir", reviewDir,
		"--confirm", "--output", "json")
}

func TestConfiguredBundleCollisionStopsBeforeTestFlightConfigExport(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "testflight.yaml")
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"testflight", "config", "export", "--output", outputPath)
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("expected no export file, stat error = %v", err)
	}
}

func TestConfiguredBundleCollisionStopsBeforePublishTestFlight(t *testing.T) {
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"publish", "testflight",
		"--workspace", "Demo.xcworkspace", "--scheme", "Demo",
		"--version", "1.2.3", "--build-number", "42", "--upload-only",
		"--output", "json")
}

func TestConfiguredBundleCollisionStopsBeforePublishAppStore(t *testing.T) {
	assertConfiguredBundleCollisionStopsBeforeContent(t,
		"publish", "appstore",
		"--workspace", "Demo.xcworkspace", "--scheme", "Demo",
		"--version", "1.2.3", "--build-number", "42",
		"--dry-run", "--output", "json")
}

func TestConfiguredBundleReachesPublishAppStoreDryRun(t *testing.T) {
	for _, source := range []string{"environment", "saved configuration"} {
		t.Run(source, func(t *testing.T) {
			configureAppSelector(t, source, configuredAppBundle)
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps", "filter[bundleId]="+configuredAppBundle, "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-configured","attributes":{"bundleId":"`+configuredAppBundle+`"}}]}`))

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
				"publish", "appstore",
				"--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--build-number", "42",
				"--dry-run", "--output", "json")
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" || !strings.Contains(stdout, "\"dryRun\":true") {
				t.Fatalf("stdout=%q stderr=%q, want configured publish dry-run", stdout, stderr)
			}
			if requests := reg.Requests(); len(requests) != 1 {
				t.Fatalf("expected only exact bundle lookup before dry-run planning, got %d: %v", len(requests), requests)
			}
		})
	}
}

func TestExplicitAppNameReachesPublishAppStoreDryRun(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Friendly App", "limit=200"),
		asctest.JSON(http.StatusOK, "{\"data\":[],\"links\":{\"next\":\"\"}}"))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Friendly App", "limit=200"),
		asctest.JSON(http.StatusOK, "{\"data\":[{\"type\":\"apps\",\"id\":\"app-friendly\",\"attributes\":{\"bundleId\":\"com.example.friendly\",\"name\":\"Friendly App\"}}],\"links\":{\"next\":\"\"}}"))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
		"publish", "appstore", "--app", "Friendly App",
		"--workspace", "Demo.xcworkspace", "--scheme", "Demo",
		"--version", "1.2.3", "--build-number", "42",
		"--dry-run", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" || !strings.Contains(stdout, "\"dryRun\":true") {
		t.Fatalf("stdout=%q stderr=%q, want explicit-name publish dry-run", stdout, stderr)
	}
	if requests := reg.Requests(); len(requests) != 2 {
		t.Fatalf("expected exact bundle then name lookup, got %d: %v", len(requests), requests)
	}
}
