package optimize

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSearchPlanCommandValidatesRequiredFlagsBeforeAuthentication(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubSearchPlanResolversToFail(t)
	command := newSearchPlanCommand(newOptimizeTestDeps(nil))
	err := command.ParseAndRun(context.Background(), nil)
	if err == nil || err.Error() != "--app is required (or set ASC_APP_ID)" {
		t.Fatalf("error = %v, want exact --app usage error", err)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestSearchPlanCommandRejectsInvalidCountryGenreLocaleAndWindow(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "country", args: []string{"--app", "1", "--version", "4.4.4", "--ad-account", "2", "--country", "USA", "--genre", "PRODUCTIVITY_UTILITIES", "--locale", "en-US"}, want: "--country must be an ISO alpha-2 code"},
		{name: "genre", args: []string{"--app", "1", "--version", "4.4.4", "--ad-account", "2", "--country", "US", "--genre", "bad genre", "--locale", "en-US"}, want: "--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES"},
		{name: "locale", args: []string{"--app", "1", "--version", "4.4.4", "--ad-account", "2", "--country", "US", "--genre", "PRODUCTIVITY_UTILITIES", "--locale", "english"}, want: "--locale invalid locale \"english\": must match pattern like en or en-US"},
		{name: "window", args: []string{"--app", "1", "--version", "4.4.4", "--ad-account", "2", "--country", "US", "--genre", "PRODUCTIVITY_UTILITIES", "--locale", "en-US", "--window", "31d"}, want: "--window must be a whole number of days from 2d through 30d"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stubSearchPlanResolversToFail(t)
			command := newSearchPlanCommand(newOptimizeTestDeps(nil))
			err := command.ParseAndRun(context.Background(), test.args)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func stubSearchPlanResolversToFail(t *testing.T) {
	t.Helper()
	previousMetadata := resolveSearchMetadataForPlan
	previousAds := collectSearchDataForPlan
	t.Cleanup(func() {
		resolveSearchMetadataForPlan = previousMetadata
		collectSearchDataForPlan = previousAds
	})
	resolveSearchMetadataForPlan = func(context.Context, *shared.Deps, string, string, string, string, string) (resolvedSearchMetadata, error) {
		t.Fatal("metadata resolver ran before usage validation")
		return resolvedSearchMetadata{}, nil
	}
	collectSearchDataForPlan = func(context.Context, *shared.Deps, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		t.Fatal("Apple Ads resolver ran before usage validation")
		return ads.SearchOptimizationData{}, nil
	}
}

func TestOptimizeGroupsReturnHelp(t *testing.T) {
	if err := NewOptimizeCommand(newOptimizeTestDeps(nil)).Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("optimize Exec error = %v, want flag.ErrHelp", err)
	}
}

func TestSearchPlanCommandRendersSupportedOutputsAndWritesArtifacts(t *testing.T) {
	previousMetadata := resolveSearchMetadataForPlan
	previousAds := collectSearchDataForPlan
	t.Cleanup(func() {
		resolveSearchMetadataForPlan = previousMetadata
		collectSearchDataForPlan = previousAds
	})
	resolveSearchMetadataForPlan = func(_ context.Context, _ *shared.Deps, appSelector, version, platform, appInfoID, locale string) (resolvedSearchMetadata, error) {
		if appSelector != "123456789" || version != "4.4.4" || platform != "IOS" || appInfoID != "info-live" || locale != "en-US" {
			t.Fatalf("metadata inputs = (%q, %q, %q, %q, %q)", appSelector, version, platform, appInfoID, locale)
		}
		return resolvedSearchMetadata{
			AppID: "123456789", VersionID: "version-1", AppInfoID: "info-live", Platform: "IOS",
			Metadata: searchMetadataSnapshot{Name: "Focus Keeper", Subtitle: "Habit tracker", Keywords: "focus,timer"},
		}, nil
	}
	collectSearchDataForPlan = func(_ context.Context, _ *shared.Deps, profile, account string, request ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		if profile != "Ads" || account != "987654321" || request.AppID != "123456789" || request.Country != "US" || request.Genre != "PRODUCTIVITY_UTILITIES" {
			t.Fatalf("Ads inputs = (%q, %q, %+v)", profile, account, request)
		}
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{
				{Name: "keyword_suggestions", Status: "available", Count: 1},
				{Name: "phrase_suggestions", Status: "unavailable", Error: "request unavailable"},
				{Name: "search_term_performance", Status: "empty"},
			},
			Suggestions:  []ads.SearchSuggestion{{Text: "daily habits", Popularity: intPtr(72), Kind: "keyword"}},
			Popularities: []ads.SearchPopularity{{Term: "daily habits", Popularity100: intPtr(72), Popularity5: intPtr(4), RankInGenre: intPtr(8)}},
		}, nil
	}

	baseArgs := []string{
		"--app", "123456789", "--version", "4.4.4", "--app-info", "info-live", "--ad-account", "987654321", "--ads-profile", "Ads",
		"--country", "us", "--genre", "productivity_utilities", "--locale", "en-US",
	}
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			d := newOptimizeTestDeps(nil)
			args := append(append([]string(nil), baseArgs...), "--output", format)
			if format == "json" {
				args = append(args, "--out-dir", filepath.Join(t.TempDir(), "plan"))
			}
			stdout := captureSearchPlanStdout(t, d, func() error {
				return newSearchPlanCommand(d).ParseAndRun(context.Background(), args)
			})
			for _, want := range []string{"daily habits", "phrase_suggestions", "unavailable", "request unavailable"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
			normalizedOutput := strings.ToLower(stdout)
			expectedByFormat := map[string][]string{
				"json":     {"metadata_candidate", "untested_candidate"},
				"table":    {"search optimization plan", "focus keeper", "popularity", "genre rank", "sources", "notices", "metadata · test"},
				"markdown": {"popularity 1-5", "popularity 1-100", "genre rank", "sources", "notices", "metadata_candidate", "untested_candidate"},
			}
			for _, want := range expectedByFormat[format] {
				if !strings.Contains(normalizedOutput, strings.ToLower(want)) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
			if format == "table" {
				planIndex := strings.Index(stdout, "SEARCH PLAN\n")
				sourcesIndex := strings.Index(stdout, "SOURCES\n")
				if planIndex < 0 || sourcesIndex < 0 || planIndex > sourcesIndex {
					t.Fatalf("table must lead with the search plan before diagnostics:\n%s", stdout)
				}
			}
			if format == "json" {
				outIndex := len(args) - 1
				if _, err := os.Stat(filepath.Join(args[outIndex], searchPlanReportArtifact)); err != nil {
					t.Fatalf("report artifact: %v", err)
				}
			}
		})
	}
}

// newOptimizeTestDeps returns deps with buffered streams whose public App
// Store requests go through transport.
func newOptimizeTestDeps(transport http.RoundTripper) *shared.Deps {
	return &shared.Deps{
		IO:            &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
		Now:           time.Now,
		Timeout:       shared.ContextWithTimeout,
		Getwd:         os.Getwd,
		HTTPTransport: func() http.RoundTripper { return transport },
	}
}

// captureSearchPlanStdout runs a command built on d and returns what it wrote
// to d's stdout.
func captureSearchPlanStdout(t *testing.T, d *shared.Deps, run func() error) string {
	t.Helper()
	var stdout bytes.Buffer
	d.IO = &shared.IO{Out: &stdout, ErrOut: io.Discard}
	if err := run(); err != nil {
		t.Fatalf("run error = %v", err)
	}
	return stdout.String()
}
