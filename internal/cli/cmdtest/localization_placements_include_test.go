package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const placementsIncludedJSON = `"included":[{"type":"appAssetLibraryPlacements","id":"placement-1"}]`

// Each row is an operation whose OpenAPI include enum lists placements. A row
// fails if the allowlist rejects the value, the query loses a parameter, or
// Apple's included array is dropped from JSON output.
func TestLocalizationReadsRequestPlacementsAndPrintIncluded(t *testing.T) {
	tests := []struct {
		name       string
		command    []string
		collection bool
		path       string
		flags      []string
		query      string
	}{
		{
			name:       "version localizations list",
			command:    []string{"localizations", "list", "--version", "version-1"},
			collection: true,
			path:       "/v1/appStoreVersions/version-1/appStoreVersionLocalizations",
			flags:      []string{"--include", "placements", "--placements-limit", "50"},
			query:      "include=placements&limit%5Bplacements%5D=50",
		},
		{
			name:    "version localization view",
			command: []string{"localizations", "view", "--id", "loc-1"},
			path:    "/v1/appStoreVersionLocalizations/loc-1",
			flags:   []string{"--include", "appScreenshotSets,placements", "--placements-limit", "1"},
			query:   "include=appScreenshotSets%2Cplacements&limit%5Bplacements%5D=1",
		},
		{
			name:       "app event localizations list",
			command:    []string{"app-events", "localizations", "list", "--event-id", "event-1"},
			collection: true,
			path:       "/v1/appEvents/event-1/localizations",
			flags:      []string{"--include", "placements", "--placements-limit", "10"},
			query:      "include=placements&limit%5Bplacements%5D=10",
		},
		{
			name:    "app event localization view",
			command: []string{"app-events", "localizations", "view", "--localization-id", "loc-1"},
			path:    "/v1/appEventLocalizations/loc-1",
			flags:   []string{"--include", "placements", "--placements-limit", "10"},
			query:   "include=placements&limit%5Bplacements%5D=10",
		},
		{
			name:       "app events list localization fields",
			command:    []string{"app-events", "list", "--app", "app-1"},
			collection: true,
			path:       "/v1/apps/app-1/appEvents",
			flags:      []string{"--include", "localizations", "--localization-fields", "placements"},
			query:      "fields%5BappEventLocalizations%5D=placements&include=localizations",
		},
		{
			name:       "custom page localizations list",
			command:    []string{"product-pages", "custom-pages", "localizations", "list", "--custom-page-version-id", "version-1"},
			collection: true,
			path:       "/v1/appCustomProductPageVersions/version-1/appCustomProductPageLocalizations",
			flags:      []string{"--include", "placements", "--placements-limit", "10"},
			query:      "include=placements&limit%5Bplacements%5D=10",
		},
		{
			name:    "custom page localization view",
			command: []string{"product-pages", "custom-pages", "localizations", "view", "--localization-id", "loc-1"},
			path:    "/v1/appCustomProductPageLocalizations/loc-1",
			flags:   []string{"--include", "placements", "--placements-limit", "10"},
			query:   "include=placements&limit%5Bplacements%5D=10",
		},
		{
			name:       "treatment localizations list",
			command:    []string{"product-pages", "experiments", "treatments", "localizations", "list", "--treatment-id", "treatment-1"},
			collection: true,
			path:       "/v1/appStoreVersionExperimentTreatments/treatment-1/appStoreVersionExperimentTreatmentLocalizations",
			flags:      []string{"--include", "placements", "--placements-limit", "10"},
			query:      "include=placements&limit%5Bplacements%5D=10",
		},
		{
			name:    "treatment localization view",
			command: []string{"product-pages", "experiments", "treatments", "localizations", "view", "--localization-id", "loc-1"},
			path:    "/v1/appStoreVersionExperimentTreatmentLocalizations/loc-1",
			flags:   []string{"--include", "placements", "--placements-limit", "10"},
			query:   "include=placements&limit%5Bplacements%5D=10",
		},
	}

	for _, test := range tests {
		for _, variant := range []struct {
			name         string
			flags        []string
			query        string
			wantIncluded bool
		}{
			{name: "with flags", flags: test.flags, query: test.query, wantIncluded: true},
			{name: "without flags"},
		} {
			t.Run(test.name+" "+variant.name, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

				resource := `{"type":"appEventLocalizations","id":"loc-1","attributes":{}}`
				if test.collection {
					resource = "[" + resource + "]"
				}
				body := `{"data":` + resource + `,"links":{"self":"https://api.appstoreconnect.apple.com` + test.path + `"}`
				if variant.wantIncluded {
					body += "," + placementsIncludedJSON
				}
				body += "}"
				reg := asctest.NewRegistry(t)
				reg.Install()
				reg.Register(asctest.GET(test.path, strings.Split(variant.query, "&")...), asctest.JSON(http.StatusOK, body))

				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)

				args := append(append([]string{}, test.command...), "--output", "json")
				args = append(args, variant.flags...)
				stdout, stderr := captureOutput(t, func() {
					if err := root.Parse(args); err != nil {
						t.Fatalf("parse error: %v", err)
					}
					if err := root.Run(context.Background()); err != nil {
						t.Fatalf("run error: %v", err)
					}
				})

				requests := reg.Requests()
				if requestCount := len(requests); requestCount != 1 {
					t.Fatalf("requests = %d, want 1", requestCount)
				}
				if got := requests[0].URL.RawQuery; got != variant.query {
					t.Fatalf("query = %q, want %q", got, variant.query)
				}
				if stderr != "" {
					t.Fatalf("stderr = %q, want empty", stderr)
				}
				if variant.wantIncluded && !strings.Contains(stdout, placementsIncludedJSON) {
					t.Fatalf("stdout = %q, want %s", stdout, placementsIncludedJSON)
				}
			})
		}
	}
}

// A row fails if a bad value reaches Apple, or a flag that cannot apply to the
// request is dropped without an error.
func TestLocalizationPlacementsFlagsRejectInvalidUseBeforeAuth(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/next?cursor=abc"
	commands := []struct {
		name    string
		args    []string
		include string
		list    bool
	}{
		{name: "version list", args: []string{"localizations", "list", "--version", "version-1"}, include: "searchKeywords", list: true},
		{name: "version view", args: []string{"localizations", "view", "--id", "loc-1"}, include: "searchKeywords"},
		{name: "event list", args: []string{"app-events", "localizations", "list", "--event-id", "event-1"}, include: "appEvent", list: true},
		{name: "event view", args: []string{"app-events", "localizations", "view", "--localization-id", "loc-1"}, include: "appEvent"},
		{name: "custom page list", args: []string{"product-pages", "custom-pages", "localizations", "list", "--custom-page-version-id", "version-1"}, include: "searchKeywords", list: true},
		{name: "custom page view", args: []string{"product-pages", "custom-pages", "localizations", "view", "--localization-id", "loc-1"}, include: "searchKeywords"},
		{name: "treatment list", args: []string{"product-pages", "experiments", "treatments", "localizations", "list", "--treatment-id", "treatment-1"}, include: "appPreviewSets", list: true},
		{name: "treatment view", args: []string{"product-pages", "experiments", "treatments", "localizations", "view", "--localization-id", "loc-1"}, include: "appPreviewSets"},
	}

	for _, command := range commands {
		cases := []struct {
			name string
			args []string
			want string
		}{
			{name: "unknown include", args: []string{"--include", "notARelationship"}, want: "--include must be one of"},
			{name: "empty include", args: []string{"--include", ""}, want: "--include must not be empty"},
			{name: "limit above spec maximum", args: []string{"--include", "placements", "--placements-limit", "51"}, want: "--placements-limit must be between 1 and 50"},
			{name: "explicit zero limit", args: []string{"--include", "placements", "--placements-limit", "0"}, want: "--placements-limit must be between 1 and 50"},
			{name: "limit without include", args: []string{"--placements-limit", "10"}, want: "--placements-limit requires --include placements"},
			{name: "limit with another include", args: []string{"--include", command.include, "--placements-limit", "10"}, want: "--placements-limit requires --include placements"},
		}
		if command.list {
			cases = append(cases,
				struct {
					name string
					args []string
					want string
				}{name: "next with include", args: []string{"--next", next, "--include", "placements"}, want: "--next cannot be combined with --include"},
				struct {
					name string
					args []string
					want string
				}{name: "next with limit", args: []string{"--next", next, "--placements-limit", "10"}, want: "--next cannot be combined with --placements-limit"},
			)
		}
		for _, test := range cases {
			t.Run(command.name+" "+test.name, func(t *testing.T) {
				assertPlacementsUsageError(t, append(append([]string{}, command.args...), test.args...), test.want)
			})
		}
	}

	t.Run("app-info view include", func(t *testing.T) {
		assertPlacementsUsageError(t, []string{"localizations", "view", "--type", "app-info", "--id", "loc-1", "--include", "placements"}, "--include requires --type version")
	})
}

func assertPlacementsUsageError(t *testing.T, args []string, want string) {
	t.Helper()

	clientFactoryCalled := false
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not run during validation")
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if clientFactoryCalled {
		t.Fatal("client factory ran before validation")
	}
}
