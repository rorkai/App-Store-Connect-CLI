package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsTestNotesViewLatestByAppLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"builds","id":"build-latest","attributes":{"processingState":"VALID","uploadedDate":"2026-03-05T12:00:00Z"}}]
			}`))
	reg.Register(asctest.GET("/v1/betaBuildLocalizations", "filter[build]=build-latest", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Latest notes"}}]
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "test-notes", "view",
		"--app", "123456789",
		"--latest",
		"--locale", "en-US",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected localization output, got %q", stdout)
	}
}

func TestBuildsTestNotesUpdateByBuildNumberAndLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=IOS", "filter[version]=42", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}]
			}`))
	reg.Register(asctest.GET("/v1/betaBuildLocalizations", "filter[build]=build-42", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-42","attributes":{"locale":"en-US","whatsNew":"Old notes"}}]
			}`))
	reg.Register(asctest.PATCH("/v1/betaBuildLocalizations/loc-42").JSONBody(`{"data":{"type":"betaBuildLocalizations","id":"loc-42","attributes":{"whatsNew":"Updated notes"}}}`),
		asctest.JSON(http.StatusOK, `{
				"data":{"type":"betaBuildLocalizations","id":"loc-42","attributes":{"locale":"en-US","whatsNew":"Updated notes"}}
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "test-notes", "update",
		"--app", "123456789",
		"--build-number", "42",
		"--platform", "IOS",
		"--locale", "en-US",
		"--whats-new", "Updated notes",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-42"`) {
		t.Fatalf("expected updated localization output, got %q", stdout)
	}
}

func TestBuildsTestNotesCreateUpdatesExistingLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{
				"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}
			}`))
	reg.Register(asctest.GET("/v1/builds/build-1/app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"123456789"}}`))
	reg.Register(asctest.GET("/v1/betaAppLocalizations", "filter[app]=123456789", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"betaAppLocalizations","id":"bal-en","attributes":{"locale":"en-US"}}]
			}`))
	reg.Register(asctest.GET("/v1/builds/build-1/betaBuildLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-en","attributes":{"locale":"en-US","whatsNew":"Old notes"}}]
			}`))
	reg.Register(asctest.PATCH("/v1/betaBuildLocalizations/loc-en").JSONBody(`{"data":{"type":"betaBuildLocalizations","id":"loc-en","attributes":{"whatsNew":"Updated notes"}}}`),
		asctest.JSON(http.StatusOK, `{
				"data":{"type":"betaBuildLocalizations","id":"loc-en","attributes":{"locale":"en-US","whatsNew":"Updated notes"}}
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "test-notes", "create",
		"--build-id", "build-1",
		"--locale", "en-US",
		"--whats-new", "Updated notes",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-en"`) {
		t.Fatalf("expected existing localization output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected five requests, got %d", requestCount)
	}
}

func TestBuildsTestNotesRemovedSelectorAliasesHaveGuidance(t *testing.T) {
	tests := []struct {
		name string
		args []string
		flag string
	}{
		{
			name: "list build alias",
			args: []string{"builds", "test-notes", "list", "--build", "BUILD_123"},
			flag: "--build",
		},
		{
			name: "view build alias",
			args: []string{"builds", "test-notes", "view", "--build", "BUILD_123", "--locale", "en-US"},
			flag: "--build",
		},
		{
			name: "view localization id alias",
			args: []string{"builds", "test-notes", "view", "--id", "loc-1"},
			flag: "--id",
		},
		{
			name: "create build alias",
			args: []string{"builds", "test-notes", "create", "--build", "BUILD_123", "--locale", "en-US", "--whats-new", "Notes"},
			flag: "--build",
		},
		{
			name: "update localization id alias",
			args: []string{"builds", "test-notes", "update", "--id", "loc-1", "--whats-new", "Notes"},
			flag: "--id",
		},
		{
			name: "delete localization id alias",
			args: []string{"builds", "test-notes", "delete", "--id", "loc-1", "--confirm"},
			flag: "--id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertRemovedFlagGuidance(t, test.args, test.flag)
		})
	}
}
