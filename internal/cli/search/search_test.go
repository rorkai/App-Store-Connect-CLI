package search

import (
	"slices"
	"testing"
)

func TestScoreCommandDocSkipsSelfReferentialAliases(t *testing.T) {
	doc := commandDoc{
		Command:    "asc foo",
		PathTokens: []string{"tester"},
	}

	score, matched := scoreCommandDoc(doc, []string{"tester"})

	if score != 60 {
		t.Fatalf("expected direct path-token score only, got %d with matches %v", score, matched)
	}
	if slices.Contains(matched, "alias:tester") {
		t.Fatalf("expected self alias to be skipped, got matches %v", matched)
	}
}

func TestScoreTermDoesNotStackExactCommandAndPathTokenScores(t *testing.T) {
	doc := commandDoc{
		Command:    "asc search",
		PathTokens: []string{"search"},
	}

	score, matched := scoreTerm(doc, "search", "query:search")

	if score != 120 {
		t.Fatalf("expected exact command score only, got %d with matches %v", score, matched)
	}
}
