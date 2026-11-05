package testflight

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestBetaGroupsListBuildIDHelpMatchesOpenAPISnapshot pins the --build-id help
// text to what docs/openapi/latest.json actually documents: there is no GET on
// the build-side betaGroups relationship, but GET /v1/builds/{id} does accept
// include=betaGroups, so the help must not claim that no build-side read exists.
func TestBetaGroupsListBuildIDHelpMatchesOpenAPISnapshot(t *testing.T) {
	pathsIndex := readOpenAPIPathsIndex(t)
	if strings.Contains(pathsIndex, "GET /v1/builds/{id}/relationships/betaGroups") {
		t.Fatal("OpenAPI snapshot now documents a build-side betaGroups relationship GET; revisit the --build-id lookup design and this help text")
	}

	help := betaGroupsListLongHelp(t)

	if strings.Contains(help, "does not expose a build-side GET for beta groups") {
		t.Fatalf("help still claims App Store Connect exposes no build-side GET for beta groups, but the snapshot documents include=betaGroups on GET /v1/builds/{id}: %q", help)
	}
	for _, want := range []string{
		"GET /v1/builds/{id}/relationships/betaGroups",
		"include=betaGroups",
		"limit[betaGroups]",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected --build-id help to mention %q, got %q", want, help)
		}
	}
}

func TestBetaGroupsListHelpDocumentsAppEndpointAndBuildIDConflicts(t *testing.T) {
	if !strings.Contains(readOpenAPIPathsIndex(t), "GET /v1/apps/{id}/betaGroups") {
		t.Fatal("OpenAPI snapshot no longer documents GET /v1/apps/{id}/betaGroups")
	}

	help := betaGroupsListLongHelp(t)
	if strings.Contains(help, "accepts only a page limit") {
		t.Fatalf("help still claims the app-scoped endpoint accepts only a page limit: %q", help)
	}
	if !strings.Contains(help, "fields[betaGroups]") {
		t.Fatalf("expected app-scoped endpoint help to mention fields[betaGroups], got %q", help)
	}

	conflictHelp := betaGroupsListBuildIDConflictHelp(t, help)
	if !strings.Contains(conflictHelp, "only with --app, --internal, and --external") {
		t.Fatalf("expected --build-id help to allowlist --app, --internal, and --external, got %q", conflictHelp)
	}
	if !strings.Contains(conflictHelp, "among query flags") || !strings.Contains(conflictHelp, "Output flags still apply") {
		t.Fatalf("expected --build-id help to qualify the query-flag allowlist and keep output flags, got %q", conflictHelp)
	}

	rejected := slices.Concat(
		betaGroupsListMembershipPageControlFlags,
		betaGroupsListNameSortFlags,
		betaGroupsListQuerySurfaceFlags,
	)
	for _, name := range rejected {
		flagName := "--" + name
		if !helpMentionsFlag(conflictHelp, flagName) {
			t.Errorf("expected --build-id conflict help to mention rejected %s, got %q", flagName, conflictHelp)
		}
	}
	for _, name := range betaGroupsListBuildIDCompatibleFlags {
		flagName := "--" + name
		if !helpMentionsFlag(conflictHelp, flagName) {
			t.Errorf("expected --build-id conflict help to mention compatible %s, got %q", flagName, conflictHelp)
		}
	}

	classified := map[string]string{
		"build-id": "lookup",
		"output":   "output",
		"pretty":   "output",
	}
	for _, name := range betaGroupsListBuildIDCompatibleFlags {
		classified[name] = "compatible"
	}
	for _, name := range betaGroupsListMembershipPageControlFlags {
		classified[name] = "page-control"
	}
	for _, name := range betaGroupsListNameSortFlags {
		classified[name] = "name-sort"
	}
	for _, name := range betaGroupsListQuerySurfaceFlags {
		classified[name] = "query-surface"
	}

	BetaGroupsListCommand().FlagSet.VisitAll(func(value *flag.Flag) {
		if _, ok := classified[value.Name]; !ok {
			t.Errorf("--%s is not classified against the --build-id allowlist or conflict sets", value.Name)
		}
	})
}

func helpMentionsFlag(help, flagName string) bool {
	for start := 0; start <= len(help); {
		offset := strings.Index(help[start:], flagName)
		if offset < 0 {
			return false
		}
		index := start + offset
		end := index + len(flagName)
		if end == len(help) || !isFlagNameContinue(help[end]) {
			return true
		}
		start = end
	}
	return false
}

func isFlagNameContinue(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-'
}

func betaGroupsListBuildIDConflictHelp(t *testing.T, help string) string {
	t.Helper()

	const marker = "--build-id membership lookup"
	start := strings.Index(help, marker)
	if start < 0 {
		t.Fatalf("expected help to contain %q, got %q", marker, help)
	}
	rest := help[start:]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func betaGroupsListLongHelp(t *testing.T) string {
	t.Helper()

	for _, subcommand := range BetaGroupsCommand().Subcommands {
		if subcommand.Name == "list" {
			return subcommand.LongHelp
		}
	}
	t.Fatal("expected beta-groups list subcommand")
	return ""
}

func readOpenAPIPathsIndex(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "paths.txt"))
	if err != nil {
		t.Fatalf("read OpenAPI path index: %v", err)
	}
	return string(data)
}
