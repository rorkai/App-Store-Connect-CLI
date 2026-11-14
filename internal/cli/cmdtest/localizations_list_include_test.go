package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsListSendsIncludeAndPrintsEnvelope(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=en-US", "include=appScreenshotSets,appPreviewSets"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"included":[{"type":"appScreenshotSets","id":"set-1"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "list",
		"--version", "version-1",
		"--locale", "en-US",
		"--include", "appScreenshotSets,appPreviewSets",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected exactly 1 request, got %d", requestCount)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected localization data in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"included":[{"type":"appScreenshotSets","id":"set-1"}]`) {
		t.Fatalf("expected unmodified included envelope in output, got %q", stdout)
	}
}

func TestLocalizationsListPaginateMergesIncludeAcrossPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=BQ&include=appScreenshotSets&limit=200"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "include=appScreenshotSets", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1"}],"included":[{"type":"appScreenshotSets","id":"set-1"}],"links":{"next":"`+secondURL+`"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "cursor=BQ", "include=appScreenshotSets", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-2"}],"included":[{"type":"appScreenshotSets","id":"set-2"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "list",
		"--version", "version-1",
		"--include", "appScreenshotSets",
		"--paginate",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected exactly 2 requests, got %d", requestCount)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{`"id":"loc-1"`, `"id":"loc-2"`, `"id":"set-1"`, `"id":"set-2"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %s in aggregated output, got %q", want, stdout)
		}
	}
}

func TestLocalizationsListIncludeValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported include value",
			args:    []string{"localizations", "list", "--version", "version-1", "--include", "appScreenshotSets,builds"},
			wantErr: "--include must be one of: appStoreVersion, appScreenshotSets, appPreviewSets, searchKeywords",
		},
		{
			name:    "empty include value",
			args:    []string{"localizations", "list", "--version", "version-1", "--include", ""},
			wantErr: "--include must not be empty",
		},
		{
			name:    "repeated include flag",
			args:    []string{"localizations", "list", "--version", "version-1", "--include", "appScreenshotSets", "--include", "appPreviewSets"},
			wantErr: `--include specified multiple times; pass one comma-separated list, for example --include "appScreenshotSets,appPreviewSets"`,
		},
		{
			name: "include with next",
			args: []string{
				"localizations", "list",
				"--version", "version-1",
				"--include", "appScreenshotSets",
				"--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ",
			},
			wantErr: "--next cannot be combined with --include",
		},
		{
			name:    "include with app-info type",
			args:    []string{"localizations", "list", "--app", "app-1", "--type", "app-info", "--include", "appStoreVersion"},
			wantErr: "--include requires --type version",
		},
		{
			name:    "app-info relationship with app-info type",
			args:    []string{"localizations", "list", "--app", "app-1", "--type", "app-info", "--include", "appInfo"},
			wantErr: "--include requires --type version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			asctest.NewRegistry(t).Install()

			_, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.0.0"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
