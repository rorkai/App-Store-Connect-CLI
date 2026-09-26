package suggest

import (
	"slices"
	"strings"
	"testing"
)

func TestCommandsPrefixSuggestion(t *testing.T) {
	got := Commands("buil", []string{"builds", "reviews", "apps"})
	if len(got) == 0 || got[0] != "builds" {
		t.Fatalf("expected prefix suggestion to prioritize builds, got %v", got)
	}
}

func TestCommandsRanksClosestPrefixBeforeSpecializedGroups(t *testing.T) {
	got := Commands("buil", []string{"build-bundles", "build-localizations", "builds"})
	want := []string{"builds", "build-bundles", "build-localizations"}
	if !slices.Equal(got, want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
}

func TestCommandsEditDistanceSuggestion(t *testing.T) {
	got := Commands("revews", []string{"reviews", "crashes", "apps"})
	if len(got) == 0 || got[0] != "reviews" {
		t.Fatalf("expected levenshtein suggestion for reviews, got %v", got)
	}
}

func TestCommandsRejectsEditDistanceAboveContract(t *testing.T) {
	if got := Commands("agxxxments", []string{"agreements"}); got != nil {
		t.Fatalf("Commands() = %v, want no suggestion", got)
	}
	if !withinThreshold("agreements", 2) {
		t.Fatal("long command should accept a two-edit suggestion")
	}
	if withinThreshold("agreements", 3) {
		t.Fatal("long command should reject a three-edit suggestion")
	}
}

func TestCommandsConservativeBehavior(t *testing.T) {
	if got := Commands("", []string{"apps"}); got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
	if got := Commands("zzzzzzzzzz", []string{"apps", "builds", "reviews"}); got != nil {
		t.Fatalf("expected nil for low-confidence suggestions, got %v", got)
	}
}

func TestCommandsAdjacentTransposition(t *testing.T) {
	got := Commands("lsit", []string{"list", "view", "update"})
	if len(got) == 0 || got[0] != "list" {
		t.Fatalf("expected adjacent transposition suggestion, got %v", got)
	}
}

func TestFlagsMatchesIdentifierShorthand(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		want       string
	}{
		{
			name:       "generic ID expands to resource ID",
			input:      "id",
			candidates: []string{"include", "version-id", "output"},
			want:       "version-id",
		},
		{
			name:       "resource ID contracts to generic ID",
			input:      "subscription-id",
			candidates: []string{"id", "output", "pretty"},
			want:       "id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Flags(test.input, test.candidates)
			if len(got) == 0 || got[0] != test.want {
				t.Fatalf("Flags(%q) = %v, want first suggestion %q", test.input, got, test.want)
			}
		})
	}
}

func TestFlagsSuggestsQualifiedIdentifierForTypoOfID(t *testing.T) {
	got := Flags("idd", []string{"app", "iap-id", "limit", "next"})
	if len(got) == 0 || got[0] != "iap-id" {
		t.Fatalf("Flags() = %v, want iap-id first", got)
	}
}

func TestFlagsCapsSuggestionsAcrossMatchingStrategies(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		want       []string
	}{
		{
			name:       "direct matches return at cap",
			input:      "app",
			candidates: []string{"appstore", "application", "apple", "build-app"},
			want:       []string{"apple", "appstore", "application"},
		},
		{
			name:       "suffix matches top up to cap",
			input:      "id",
			candidates: []string{"ids", "version-id", "build-id", "app-id"},
			want:       []string{"ids", "app-id", "build-id"},
		},
		{
			name:       "no match remains empty",
			input:      "zzzzzzzzzz",
			candidates: []string{"output", "pretty"},
			want:       nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Flags(test.input, test.candidates); (got == nil) != (test.want == nil) || !slices.Equal(got, test.want) {
				t.Fatalf("Flags(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestCommandsSubstringSuggestion(t *testing.T) {
	got := Commands("phased", []string{"list", "phased-release", "release"})
	if len(got) == 0 || got[0] != "phased-release" {
		t.Fatalf("expected substring suggestion for phased-release, got %v", got)
	}
	// Two characters are too little signal for a substring match.
	if got := Commands("se", []string{"release", "phased-release"}); got != nil {
		t.Fatalf("expected no substring suggestion for a two-character input, got %v", got)
	}
}

func TestCommandsSubstringSuggestionRequiresForwardHyphenComponents(t *testing.T) {
	if got := Commands("release", []string{"phased-release"}); !slices.Equal(got, []string{"phased-release"}) {
		t.Fatalf("Commands() = %v, want a hyphen-component match", got)
	}
	if got := Commands("hased", []string{"phased-release"}); got != nil {
		t.Fatalf("Commands() = %v, want no mid-component match", got)
	}
	if got := Commands("my-phased-release-command", []string{"phased-release"}); got != nil {
		t.Fatalf("Commands() = %v, want no reverse-containment match", got)
	}
}

func TestCommandsRanksPrefixBeforeSubstringBeforeEdits(t *testing.T) {
	got := Commands("list", []string{"lits", "listen", "app-list-all"})
	want := []string{"listen", "app-list-all", "lits"}
	if !slices.Equal(got, want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
}

func TestEditDistanceCountsAdjacentTranspositionOnce(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"apps", "apps", 0},
		{"app", "apps", 1},
		{"lsit", "list", 1},
		{"lsits", "list", 2},
		{"buidls", "builds", 1},
		{"", "abc", 3},
		{"abc", "", 3},
		{"ca", "abc", 3},
	}
	for _, test := range tests {
		if got := editDistance(test.a, test.b); got != test.want {
			t.Fatalf("editDistance(%q, %q) = %d, want %d", test.a, test.b, got, test.want)
		}
	}
	if !withinThreshold("apps", 1) || withinThreshold("apps", 2) {
		t.Fatalf("unexpected threshold behavior for short command length")
	}
}

func TestDistanceExportsCurrentEditDistance(t *testing.T) {
	if got := Distance("lsit", "list"); got != 1 {
		t.Fatalf("Distance(lsit, list) = %d, want 1", got)
	}
}

// The unknown token is whatever the caller typed, so the ranker must stay
// bounded by the command names it compares against rather than by that token.
// A full edit-distance matrix over a token this long allocates a row per
// character for every candidate.
func TestCommandsStaysBoundedForAnOversizedInput(t *testing.T) {
	candidates := []string{"list", "view", "create", "phased-release"}
	huge := strings.Repeat("q", 200000)

	if got := Commands(huge, candidates); got != nil {
		t.Fatalf("Commands() = %v, want no suggestion for an oversized input", got)
	}

	perCandidate := testing.AllocsPerRun(1, func() {
		_ = editDistance(huge, "phased-release")
	})
	// Three rolling rows, plus slack for the test harness itself.
	if perCandidate > 10 {
		t.Fatalf("editDistance allocated %v times for one candidate, want a handful of rolling rows", perCandidate)
	}
}
