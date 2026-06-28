package suggest

import "testing"

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
