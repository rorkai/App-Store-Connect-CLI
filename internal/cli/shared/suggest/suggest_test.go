package suggest

import (
	"slices"
	"testing"
)

func TestCommandsPrefixSuggestion(t *testing.T) {
	got := Commands("buil", []string{"builds", "reviews", "apps"})
	if len(got) == 0 || got[0] != "builds" {
		t.Fatalf("expected prefix suggestion to prioritize builds, got %v", got)
	}
}

func TestCommandsEditDistanceSuggestion(t *testing.T) {
	got := Commands("revews", []string{"reviews", "crashes", "apps"})
	if len(got) == 0 || got[0] != "reviews" {
		t.Fatalf("expected levenshtein suggestion for reviews, got %v", got)
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
			want:       []string{"apple", "application", "appstore"},
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

func TestLevenshteinAndThresholdHelpers(t *testing.T) {
	if d := levenshtein("apps", "apps"); d != 0 {
		t.Fatalf("expected equal strings distance 0, got %d", d)
	}
	if d := levenshtein("app", "apps"); d != 1 {
		t.Fatalf("expected distance 1, got %d", d)
	}
	if !withinThreshold("apps", 1) || withinThreshold("apps", 2) {
		t.Fatalf("unexpected threshold behavior for short command length")
	}
	if min := min3(3, 2, 4); min != 2 {
		t.Fatalf("expected min3 to return 2, got %d", min)
	}
}
