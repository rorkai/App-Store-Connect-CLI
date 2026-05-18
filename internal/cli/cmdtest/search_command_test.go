package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
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

func TestSearchSupportsLimitFlag(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "json", "--limit", "2", "build"}, "1.2.3")
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
	if response.Count > 2 || len(response.Results) > 2 {
		t.Fatalf("expected at most 2 results, got %#v", response)
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

func TestSearchRejectsNonPositiveLimit(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--limit", "0", "build"}, "1.2.3")
	})

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--limit must be greater than 0") {
		t.Fatalf("expected limit validation error, got %q", stderr)
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

func TestSearchRequiresQuery(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search"}); err != nil {
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
	if !strings.Contains(stderr, "search query is required") {
		t.Fatalf("expected missing query error, got %q", stderr)
	}
}

func TestSearchRejectsBlankQuery(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "json", "   "}, "1.2.3")
	})

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "search query is required") {
		t.Fatalf("expected missing query error, got %q", stderr)
	}
}

func TestSearchInvalidOutputExitsWithUsageCode(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "--output", "yaml", "builds"}, "1.2.3")
	})

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected unsupported format error, got %q", stderr)
	}
}

func TestSearchInvalidMixedOrderOutputExitsWithUsageCode(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"search", "builds", "--output", "yaml"}, "1.2.3")
	})

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected unsupported format error, got %q", stderr)
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
