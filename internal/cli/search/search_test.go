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

func TestScoreTermDoesNotStackExactCommandWithSelfReferentialHelpText(t *testing.T) {
	doc := commandDoc{
		Command:    "asc search",
		Summary:    "Search asc commands and examples.",
		Usage:      "asc search [flags] <query>",
		LongHelp:   "Search asc commands and examples.\n\nExamples:\n  asc search \"external testers\"",
		Examples:   []string{`asc search "external testers"`},
		PathTokens: []string{"search"},
		FlagTokens: []string{"search"},
	}

	score, matched := scoreTerm(doc, "search", "query:search")

	if score != 120 {
		t.Fatalf("expected exact command score only, got %d with matches %v", score, matched)
	}

	for _, unexpected := range []string{"summary:search", "usage:search", "flag:search", "example:search", "help:search"} {
		if slices.Contains(matched, unexpected) {
			t.Fatalf("expected exact command match to skip %q, got matches %v", unexpected, matched)
		}
	}
}

func TestScoreCommandDocsIgnoresLeadingASCToken(t *testing.T) {
	docs := []commandDoc{
		{
			Command:    "asc builds upload",
			Summary:    "Upload a build.",
			Usage:      "asc builds upload [flags]",
			PathTokens: []string{"builds", "upload"},
			AllTokens:  []string{"builds", "upload"},
		},
		{
			Command:    "asc apps list",
			Summary:    "List apps.",
			Usage:      "asc apps list [flags]",
			PathTokens: []string{"apps", "list"},
			AllTokens:  []string{"apps", "list"},
		},
	}

	results := scoreCommandDocs(docs, "asc build upload")

	if len(results) != 1 {
		t.Fatalf("expected only the build upload result, got %#v", results)
	}
	if results[0].result.Command != "asc builds upload" {
		t.Fatalf("expected build upload result, got %#v", results[0].result)
	}
	if slices.Contains(results[0].result.Matched, "command:asc") {
		t.Fatalf("expected leading asc token to be ignored, got matches %v", results[0].result.Matched)
	}
}
