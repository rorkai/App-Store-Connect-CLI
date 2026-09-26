package search

import (
	"flag"
	"slices"
	"strings"
	"testing"
)

func TestParseInterspersedSearchFlagsPreservesErrorPrecedence(t *testing.T) {
	t.Run("escaped profile query", func(t *testing.T) {
		fs := flag.NewFlagSet("search", flag.ContinueOnError)
		fs.Int("limit", defaultLimit, "")
		args := []string{"--profile", "staging"}

		got, err := parseInterspersedSearchFlags(fs, args)
		if err != nil || !slices.Equal(got, args) {
			t.Fatalf("parseInterspersedSearchFlags() = %q, %v, want escaped query unchanged", got, err)
		}
	})

	t.Run("earlier invalid value", func(t *testing.T) {
		fs := flag.NewFlagSet("search", flag.ContinueOnError)
		fs.Int("limit", defaultLimit, "")

		_, err := parseInterspersedSearchFlags(fs, []string{
			"query", "--limit", "nope", "--profile", "staging",
		})
		if err == nil || !strings.Contains(err.Error(), `invalid value "nope" for --limit`) {
			t.Fatalf("parseInterspersedSearchFlags() error = %v, want the invalid-limit error", err)
		}
	})

	t.Run("misplaced root profile", func(t *testing.T) {
		fs := flag.NewFlagSet("search", flag.ContinueOnError)
		fs.Int("limit", defaultLimit, "")

		_, err := parseInterspersedSearchFlags(fs, []string{"query", "--profile", "staging"})
		if err == nil || err.Error() != "`--profile` must appear before positional arguments" {
			t.Fatalf("parseInterspersedSearchFlags() error = %v, want the profile-placement error", err)
		}
	})
}

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

func TestCommandPathScorePrefersWholeSegmentsOverHyphenatedComponents(t *testing.T) {
	tests := []struct {
		name string
		doc  commandDoc
		term string
		want int
	}{
		{
			name: "whole path segment",
			doc:  commandDoc{Command: "asc web apps create", PathTokens: []string{"web", "apps", "create"}},
			term: "app",
			want: exactPathTokenScore,
		},
		{
			name: "hyphenated component",
			doc:  commandDoc{Command: "asc app-events create", PathTokens: []string{"app-events", "create"}},
			term: "app",
			want: compoundPathTokenScore,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := commandPathScore(test.doc, test.term); got != test.want {
				t.Fatalf("commandPathScore(%q, %q) = %d, want %d", test.doc.Command, test.term, got, test.want)
			}
		})
	}
}

func TestExactLeafQueryTokenMatchesWholeLeafOnly(t *testing.T) {
	if token, ok := exactLeafQueryToken("asc xcode-cloud status", []string{"build", "status"}); !ok || token != "status" {
		t.Fatalf("exactLeafQueryToken() = %q, %t, want status, true", token, ok)
	}
	if token, ok := exactLeafQueryToken("asc app-clips domain-status", []string{"build", "status"}); ok {
		t.Fatalf("exactLeafQueryToken() = %q, true, want compound leaf not to match", token)
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
		Command:       "asc search",
		Summary:       "Search asc commands and examples.",
		Usage:         "asc search [flags] <query>",
		Examples:      []string{`asc search "external testers"`},
		PathTokens:    []string{"search"},
		SummaryTokens: []string{"search"},
		UsageTokens:   []string{"search"},
		ExampleTokens: []string{"search"},
		HelpTokens:    []string{"search"},
		FlagTokens:    []string{"search"},
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

func TestTokenContainsDoesNotMatchSubstringInsideToken(t *testing.T) {
	if tokenContains([]string{"relationships"}, "ship") {
		t.Fatal("expected ship not to match relationships")
	}
	if tokenContains([]string{"real"}, "zzzz-not-real") {
		t.Fatal("expected a hyphenated query not to match one of its components")
	}
}

func TestTokenContainsMatchesSingularPluralAndHyphenatedComponents(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		term   string
	}{
		{name: "singular query", tokens: []string{"builds"}, term: "build"},
		{name: "plural query", tokens: []string{"build"}, term: "builds"},
		{name: "ies plural", tokens: []string{"categories"}, term: "category"},
		{name: "es plural", tokens: []string{"classes"}, term: "class"},
		{name: "singular ending s", tokens: []string{"statuses"}, term: "status"},
		{name: "hyphenated component", tokens: []string{"app-store-releases"}, term: "app"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !tokenContains(test.tokens, test.term) {
				t.Fatalf("expected %q to match tokens %v", test.term, test.tokens)
			}
		})
	}
}

func TestScopedAuthActionIntentPrefersEverySupportedActionOverStatus(t *testing.T) {
	tests := []struct {
		name     string
		query    []string
		expected string
	}{
		{name: "auth init", query: []string{"auth", "init", "status"}, expected: "asc auth init"},
		{name: "auth login", query: []string{"auth", "login", "status"}, expected: "asc auth login"},
		{name: "auth export-to-config", query: []string{"auth", "export-to-config", "status"}, expected: "asc auth export-to-config"},
		{name: "auth export alias", query: []string{"auth", "export", "status"}, expected: "asc auth export-to-config"},
		{name: "auth switch", query: []string{"auth", "switch", "status"}, expected: "asc auth switch"},
		{name: "auth logout", query: []string{"auth", "logout", "status"}, expected: "asc auth logout"},
		{name: "auth doctor", query: []string{"auth", "doctor", "status"}, expected: "asc auth doctor"},
		{name: "auth issuer-id", query: []string{"auth", "issuer-id", "status"}, expected: "asc auth issuer-id"},
		{name: "auth issuer id split", query: []string{"auth", "issuer", "id", "status"}, expected: "asc auth issuer-id"},
		{name: "auth token", query: []string{"auth", "token", "status"}, expected: "asc auth token"},
		{name: "StoreKit login", query: []string{"storekit", "auth", "login", "status"}, expected: "asc storekit auth login"},
		{name: "StoreKit switch", query: []string{"storekit", "auth", "switch", "status"}, expected: "asc storekit auth switch"},
		{name: "StoreKit doctor", query: []string{"storekit", "auth", "doctor", "status"}, expected: "asc storekit auth doctor"},
		{name: "StoreKit logout", query: []string{"storekit", "auth", "logout", "status"}, expected: "asc storekit auth logout"},
		{name: "Ads login", query: []string{"ads", "auth", "login", "status"}, expected: "asc ads auth login"},
		{name: "Ads discover", query: []string{"ads", "auth", "discover", "status"}, expected: "asc ads auth discover"},
		{name: "Ads switch", query: []string{"ads", "auth", "switch", "status"}, expected: "asc ads auth switch"},
		{name: "Ads token", query: []string{"ads", "auth", "token", "status"}, expected: "asc ads auth token"},
		{name: "Ads doctor", query: []string{"ads", "auth", "doctor", "status"}, expected: "asc ads auth doctor"},
		{name: "Ads logout", query: []string{"ads", "auth", "logout", "status"}, expected: "asc ads auth logout"},
		{name: "web login", query: []string{"web", "auth", "login", "status"}, expected: "asc web auth login"},
		{name: "web capabilities", query: []string{"web", "auth", "capabilities", "status"}, expected: "asc web auth capabilities"},
		{name: "web logout", query: []string{"web", "auth", "logout", "status"}, expected: "asc web auth logout"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, _, ok := scopedAuthActionIntent(test.query)
			if !ok {
				t.Fatal("expected an auth action intent")
			}
			if target != test.expected {
				t.Fatalf("scopedAuthActionIntent() = %q, want %q", target, test.expected)
			}
		})
	}
}

func TestScopedCanonicalIntentPrefersMostSpecificNamedLeaf(t *testing.T) {
	tests := []struct {
		name     string
		query    []string
		expected string
	}{
		{name: "metadata flat sibling", query: []string{"metadata", "plan", "status"}, expected: "asc metadata plan"},
		{name: "metadata nested keywords plan", query: []string{"metadata", "keywords", "plan", "status"}, expected: "asc metadata keywords plan"},
		{name: "metadata nested keywords apply", query: []string{"metadata", "keywords", "apply", "status"}, expected: "asc metadata keywords apply"},
		{name: "metadata nested keywords push", query: []string{"metadata", "keywords", "push", "status"}, expected: "asc metadata keywords push"},
		{name: "metadata nested keywords audit", query: []string{"metadata", "keywords", "audit", "status"}, expected: "asc metadata keywords audit"},
		{name: "metadata keywords group", query: []string{"metadata", "keywords", "status"}, expected: "asc metadata keywords"},
		{name: "analytics compound split", query: []string{"product", "pages", "analytics", "dashboard"}, expected: "asc web analytics product-pages"},
		{name: "analytics compound hyphenated", query: []string{"product-pages", "analytics", "dashboard"}, expected: "asc web analytics product-pages"},
		{name: "analytics in app events split", query: []string{"in", "app", "events", "analytics", "dashboard"}, expected: "asc web analytics in-app-events"},
		{name: "analytics app clips split", query: []string{"app", "clips", "analytics", "dashboard"}, expected: "asc web analytics app-clips"},
		{name: "analytics overview fallback", query: []string{"analytics", "overview"}, expected: "asc web analytics overview"},
		{name: "beta cancellation stays on TestFlight", query: []string{"cancel", "beta", "review", "submission", "status"}, expected: "asc testflight review submissions view"},
		{name: "beta app review cancellation stays on TestFlight", query: []string{"cancel", "beta", "app", "review", "submission", "status"}, expected: "asc testflight review submissions view"},
		{name: "cross-surface cancellation stays on App Store", query: []string{"cancel", "testflight", "app", "store", "submission", "status"}, expected: "asc submit cancel"},
		{name: "cross-surface App Review cancellation stays on App Store", query: []string{"cancel", "testflight", "and", "app", "review", "submission", "status"}, expected: "asc submit cancel"},
		{name: "explicit beta and App Review cancellation is cross-surface", query: []string{"cancel", "testflight", "beta", "and", "app", "review", "submission", "status"}, expected: "asc submit cancel"},
		{name: "implicit TestFlight App Review cancellation stays scoped", query: []string{"cancel", "testflight", "app", "review", "submission", "status"}, expected: "asc testflight review submissions view"},
		{name: "status conjunction does not imply cross-surface cancellation", query: []string{"cancel", "testflight", "app", "review", "submission", "and", "status"}, expected: "asc testflight review submissions view"},
		{name: "agreement download", query: []string{"download", "apple", "developer", "agreement", "status"}, expected: "asc web agreements download"},
		{name: "Xcode Cloud workflow duplicate", query: []string{"duplicate", "xcode", "cloud", "workflow", "status"}, expected: "asc xcode-cloud workflows duplicate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, _, ok := scopedCanonicalIntent(test.query)
			if !ok {
				t.Fatalf("expected scoped intent for %v", test.query)
			}
			if target != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, target)
			}
		})
	}
}

func TestScopedCanonicalIntentLeavesAppReviewDashboardForAggregateStatus(t *testing.T) {
	target, _, ok := scopedCanonicalIntent([]string{"testflight", "and", "app", "review", "dashboard"})
	if ok {
		t.Fatalf("expected aggregate dashboard scoring, got scoped target %q", target)
	}
}

func TestScopedCanonicalIntentRequiresWorkflowForXcodeCloudDuplicate(t *testing.T) {
	for _, query := range [][]string{
		{"duplicate", "xcode", "cloud", "artifact"},
		{"duplicate", "xcode", "cloud", "build", "run", "status"},
	} {
		t.Run(strings.Join(query, "-"), func(t *testing.T) {
			target, reason, _ := scopedCanonicalIntent(query)
			if target == "asc xcode-cloud workflows duplicate" || reason == "canonical:xcode-cloud-workflow-duplicate" {
				t.Fatalf("expected non-workflow Xcode Cloud routing, got duplicate target %q", target)
			}
		})
	}
}

func TestScopedCanonicalIntentLeavesTestFlightAgreementDownloadUnscoped(t *testing.T) {
	for _, query := range [][]string{
		{"download", "testflight", "beta", "license", "agreement"},
		{"download", "beta", "license", "agreement"},
	} {
		t.Run(strings.Join(query, "-"), func(t *testing.T) {
			target, reason, _ := scopedCanonicalIntent(query)
			if target == "asc web agreements download" || reason == "canonical:agreement-download" {
				t.Fatalf("expected TestFlight agreement scoring, got scoped target %q", target)
			}
		})
	}
}
