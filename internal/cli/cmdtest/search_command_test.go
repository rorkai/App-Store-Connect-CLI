package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type searchResponse struct {
	Query   string         `json:"query"`
	Count   int            `json:"count"`
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Command  string   `json:"command"`
	Summary  string   `json:"summary"`
	Usage    string   `json:"usage"`
	Score    int      `json:"score"`
	Matched  []string `json:"matched"`
	Examples []string `json:"examples"`
}

func TestSearchFindsCommandsFromTaskWordsAsJSON(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "external", "testers"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}

	if response.Query != "external testers" {
		t.Fatalf("expected normalized query, got %q", response.Query)
	}
	if response.Count == 0 || len(response.Results) == 0 {
		t.Fatalf("expected search results, got %#v", response)
	}
	if !searchResultsContain(response.Results, "asc testflight testers") {
		t.Fatalf("expected TestFlight tester command in results, got %#v", response.Results)
	}
	for _, result := range response.Results {
		if strings.TrimSpace(result.Command) == "" {
			t.Fatalf("expected command path in result: %#v", result)
		}
		if strings.TrimSpace(result.Summary) == "" {
			t.Fatalf("expected summary in result: %#v", result)
		}
		if result.Score <= 0 {
			t.Fatalf("expected positive score in result: %#v", result)
		}
		if len(result.Matched) == 0 {
			t.Fatalf("expected match reasons in result: %#v", result)
		}
	}
}

func TestSearchUsesAliasesForAgentVocabulary(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "ship", "app", "review"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}

	if !searchResultsContain(response.Results, "asc publish appstore") && !searchResultsContain(response.Results, "asc review submit") {
		t.Fatalf("expected publish or review submission command for ship app review, got %#v", response.Results)
	}
	if !searchResultsMention(response.Results, "alias:ship") {
		t.Fatalf("expected alias match reason for ship query, got %#v", response.Results)
	}
}

func TestSearchRanksExpectedCommandFirst(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"ship app", "asc publish appstore"},
		{"release app", "asc publish appstore"},
		{"upload appstore status", "asc publish appstore"},
		{"ship beta", "asc publish testflight"},
		{"verify TestFlight build upload and App Review status", "asc status"},
		{"app review status", "asc review status"},
		{"duplicate Xcode Cloud artifact", "asc xcode-cloud artifacts"},
		{"phased release status", "asc versions phased-release view"},
		{"phased release overview", "asc versions phased-release view"},
		{"beta review status", "asc builds beta-app-review-submission view"},
		{"update phased release status", "asc versions phased-release update"},
		{"TestFlight App Review status", "asc testflight review view"},
		{"App Review attachment upload status", "asc review attachments-upload"},
		{"list App Review attachment uploads", "asc review attachments-list"},
		{"get App Review attachment upload", "asc review attachments-get"},
		{"delete App Review attachment upload", "asc review attachments-delete"},
		{"TestFlight beta review submission status", "asc testflight review submissions view"},
		{"list beta review submissions status", "asc testflight review submissions list"},
		{"list TestFlight beta review submissions status", "asc testflight review submissions list"},
		{"list TestFlight App Review submissions for build status", "asc testflight review submissions list"},
		{"get TestFlight review submission build status", "asc testflight review submissions build"},
		{"update release pipeline", "asc versions phased-release update"},
		{"App Store Connect and TestFlight system status", "asc system-status"},
		{"TestFlight and App Store account status", "asc account status"},
		{"TestFlight App Store telemetry status", "asc telemetry status"},
		{"disable telemetry status", "asc telemetry disable"},
		{"enable telemetry status", "asc telemetry enable"},
		{"telemetry reset-id status", "asc telemetry reset-id"},
		{"TestFlight and App Store auth status", "asc auth status"},
		{"StoreKit auth status", "asc storekit auth status"},
		{"login StoreKit auth status", "asc storekit auth login"},
		{"Apple Ads auth status", "asc ads auth status"},
		{"discover Apple Ads auth status", "asc ads auth discover"},
		{"Apple Ads account auth status", "asc ads auth status"},
		{"web auth status", "asc web auth status"},
		{"web auth capabilities status", "asc web auth capabilities"},
		{"auth doctor status", "asc auth doctor"},
		{"TestFlight and App Store agreement status", "asc web agreements status"},
		{"Xcode Cloud build and App Review status", "asc xcode-cloud status"},
		{"trigger Xcode Cloud workflow status", "asc xcode-cloud run"},
		{"doctor Xcode Cloud workflow status", "asc xcode-cloud doctor"},
		{"list Xcode Cloud workflows status", "asc xcode-cloud workflows list"},
		{"view Xcode Cloud artifact status", "asc xcode-cloud artifacts view"},
		{"view TestFlight review app status", "asc testflight review app view"},
		{"TestFlight App Store notarization status", "asc notarization status"},
		{"log TestFlight App Store notarization status", "asc notarization log"},
		{"list TestFlight App Store notarization status", "asc notarization list"},
		{"submit TestFlight App Store notarization status", "asc notarization submit"},
		{"TestFlight App Review release dashboard", "asc status"},
		{"TestFlight and App Review dashboard", "asc status"},
		{"App Store App Clip build domain status", "asc app-clips domain-status"},
		{"App Clip domain cache status", "asc app-clips domain-status cache"},
		{"App Clip domain debug status", "asc app-clips domain-status debug"},
		{"analytics overview", "asc web analytics overview"},
		{"TestFlight and phased App Store release dashboard", "asc status"},
		{"release pipeline", "asc status"},
		{"create app", "asc web apps create"},
		{"build status", "asc xcode-cloud status"},
		{"download build", "asc builds dsyms"},
		{"upload build", "asc builds upload"},
		{"upload build app", "asc builds upload"},
		{"accept Apple Developer agreement status", "asc web agreements accept"},
		{"download Apple Developer agreement status", "asc web agreements download"},
		{"download TestFlight beta license agreement", "asc testflight agreements"},
		{"Apple Developer agreement status", "asc web agreements status"},
		{"reset telemetry id status", "asc telemetry reset-id"},
		{"auth issuer id status", "asc auth issuer-id"},
		{"auth issuer-id status", "asc auth issuer-id"},
		{"view beta review app status", "asc testflight review app view"},
		{"TestFlight App Store metadata status", "asc metadata status"},
		{"cancel TestFlight App Store submission status", "asc submit cancel"},
		{"cancel App Store submission status", "asc submit cancel"},
		{"cancel TestFlight review submission status", "asc testflight review submissions view"},
		{"cancel beta review submission status", "asc testflight review submissions view"},
		{"cancel beta app review submission status", "asc testflight review submissions view"},
		{"cancel TestFlight and App Review submission status", "asc submit cancel"},
		{"cancel TestFlight review submission for build status", "asc testflight review submissions build"},
		{"view Xcode Cloud build run", "asc xcode-cloud build-runs view"},
		{"doctor Xcode Cloud build run status", "asc xcode-cloud doctor"},
		{"duplicate Xcode Cloud workflow status", "asc xcode-cloud workflows duplicate"},
		{"approve metadata status", "asc metadata approve"},
		{"validate metadata status", "asc metadata validate"},
		{"metadata keywords plan status", "asc metadata keywords plan"},
		{"metadata keywords apply status", "asc metadata keywords apply"},
		{"metadata keywords push status", "asc metadata keywords push"},
		{"metadata keywords audit status", "asc metadata keywords audit"},
		{"subscriptions analytics dashboard", "asc web analytics subscriptions"},
		{"product pages analytics dashboard", "asc web analytics product-pages"},
		{"in app events analytics dashboard", "asc web analytics in-app-events"},
		{"app clips analytics dashboard", "asc web analytics app-clips"},
		{"TestFlight App Store analytics overview", "asc web analytics overview"},
	}

	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			response := runSearchTopFive(t, test.query)
			if response.Results[0].Command != test.expected {
				t.Fatalf("expected %q first, got %#v", test.expected, response.Results)
			}
		})
	}
}

func TestSearchDoesNotRankBroaderCommandFirst(t *testing.T) {
	tests := []struct {
		query    string
		notFirst string
	}{
		{"duplicate Xcode Cloud build run status", "asc xcode-cloud workflows duplicate"},
		{"app review", "asc publish appstore"},
	}

	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			response := runSearchTopFive(t, test.query)
			if response.Results[0].Command == test.notFirst {
				t.Fatalf("expected a more specific command ahead of %q, got %#v", test.notFirst, response.Results)
			}
		})
	}
}

func runSearchTopFive(t *testing.T, query string) searchResponse {
	t.Helper()

	var code int
	stdout, stderr := captureOutput(t, func() {
		args := append([]string{"search", "--output", "json", "--limit", "5"}, strings.Fields(query)...)
		code = rootcmd.Run(args, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if len(response.Results) == 0 {
		t.Fatalf("expected search results, got %#v", response)
	}
	return response
}

func TestSearchUsesTypoToleranceAsFallback(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "testfligth", "testers"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}

	if !searchResultsContain(response.Results, "asc testflight testers") {
		t.Fatalf("expected TestFlight tester command for typo query, got %#v", response.Results)
	}
	if !searchResultsMention(response.Results, "fuzzy:testflight") {
		t.Fatalf("expected fuzzy match reason for testfligth typo, got %#v", response.Results)
	}
}

func TestSearchSupportsTableOutput(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "table", "build", "upload"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "command") || !strings.Contains(stdout, "summary") {
		t.Fatalf("expected table headers, got %q", stdout)
	}
	if !strings.Contains(stdout, "asc builds upload") {
		t.Fatalf("expected build upload command in table output, got %q", stdout)
	}
}

func TestSearchReturnsEmptyResultsForNoMatches(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "zzzz-not-real"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if response.Count != 0 || len(response.Results) != 0 {
		t.Fatalf("expected empty result set, got %#v", response)
	}
}

func TestSearchDoesNotLeakFlagsAcrossRepeatedRuns(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "--limit", "1", "build"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var limited searchResponse
	if err := json.Unmarshal([]byte(stdout), &limited); err != nil {
		t.Fatalf("failed to unmarshal limited search JSON: %v\nstdout=%s", err, stdout)
	}
	if limited.Count != 1 || len(limited.Results) != 1 {
		t.Fatalf("expected one limited result, got %#v", limited)
	}

	stdout, stderr = captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "build"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var defaultLimit searchResponse
	if err := json.Unmarshal([]byte(stdout), &defaultLimit); err != nil {
		t.Fatalf("failed to unmarshal default-limit search JSON: %v\nstdout=%s", err, stdout)
	}
	if defaultLimit.Count <= 1 || len(defaultLimit.Results) <= 1 {
		t.Fatalf("expected default limit after repeated run, got %#v", defaultLimit)
	}
}

func TestSearchRejectsInvalidInvocationsWithUsageExit(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{args: []string{"search"}, wantErr: "search query is required"},
		{args: []string{"search", "--output", "json", "   "}, wantErr: "search query is required"},
		{args: []string{"search", "--limit", "0", "build"}, wantErr: "--limit must be greater than 0"},
		{args: []string{"search", "--output", "yaml", "builds"}, wantErr: `(got "yaml")`},
		{args: []string{"search", "builds", "--output", "yaml"}, wantErr: `(got "yaml")`},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestSearchAcceptsRootFlagsBeforeSubcommand(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"--profile", "ci", "search", "--output", "json", "--limit", "1", "build"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if response.Count != 1 || len(response.Results) != 1 {
		t.Fatalf("expected one limited result, got %#v", response)
	}
}

func TestSearchFlagValueCanMatchSubcommandName(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "json", "--limit", "3", "completion"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if response.Query != "completion" {
		t.Fatalf("expected completion query, got %q", response.Query)
	}
	if !searchResultsContain(response.Results, "asc completion") {
		t.Fatalf("expected completion command in results, got %#v", response.Results)
	}
}

func TestSearchFindsFirstClassXcodeBuildCommand(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "json", "--limit", "10", "asc", "xcode", "build"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if !searchResultsContain(response.Results, "asc xcode build") {
		t.Fatalf("expected xcode build command in results, got %#v", response.Results)
	}
}

func TestSearchSupportsMixedFlagOrder(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "json", "build", "--limit", "1"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if response.Query != "build" {
		t.Fatalf("expected build query, got %q", response.Query)
	}
	if response.Count != 1 || len(response.Results) != 1 {
		t.Fatalf("expected one limited result, got %#v", response)
	}
	if !searchResultsContain(response.Results, "asc build") {
		t.Fatalf("expected build command in results, got %#v", response.Results)
	}
}

func TestSearchPreservesRootShapedQueryTerms(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "json", "logging", "--debug"}, "1.2.3")
	})

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d with stderr %q", code, rootcmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	if response.Query != "logging --debug" {
		t.Fatalf("query = %q, want %q", response.Query, "logging --debug")
	}
}

func searchResultsContain(results []searchResult, commandPrefix string) bool {
	for _, result := range results {
		if strings.HasPrefix(result.Command, commandPrefix) {
			return true
		}
	}
	return false
}

func searchResultsMention(results []searchResult, match string) bool {
	for _, result := range results {
		for _, item := range result.Matched {
			if item == match {
				return true
			}
		}
	}
	return false
}
