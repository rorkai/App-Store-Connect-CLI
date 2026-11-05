package optimize

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNormalizeKeywordListTrimsLowercasesCollapsesAndDedupes(t *testing.T) {
	got, err := normalizeKeywordList(" Focus Timer , focus   timer ,HABIT tracker, ,focus timer")
	if err != nil {
		t.Fatalf("normalizeKeywordList error = %v", err)
	}
	want := []string{"focus timer", "habit tracker"}
	if len(got) != len(want) {
		t.Fatalf("keywords = %v, want %v", got, want)
	}
	for index, keyword := range want {
		if got[index] != keyword {
			t.Fatalf("keywords = %v, want %v", got, want)
		}
	}
}

func TestNormalizeKeywordListRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "empty",
			raw:  "  , ",
			want: "--keywords must contain at least one keyword",
		},
		{
			name: "too short",
			raw:  "focus,a",
			want: `--keywords entry "a" must be between 2 and 60 characters`,
		},
		{
			name: "too long",
			raw:  "focus," + strings.Repeat("a", 61),
			want: fmt.Sprintf("--keywords entry %q must be between 2 and 60 characters", strings.Repeat("a", 61)),
		},
		{
			name: "too many words",
			raw:  "one two three four five",
			want: `--keywords entry "one two three four five" must contain at most 4 space-separated words`,
		},
		{
			name: "too many keywords",
			raw:  strings.Join(manyKeywords(101), ","),
			want: "--keywords accepts at most 100 keywords per invocation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeKeywordList(test.raw)
			if err == nil {
				t.Fatalf("normalizeKeywordList(%q) = %v, want error", test.raw, got)
			}
			if err.Error() != test.want {
				t.Fatalf("error = %q, want %q", err.Error(), test.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestNormalizeKeywordListAcceptsBoundaryValues(t *testing.T) {
	boundary := []string{
		"ab",
		strings.Repeat("a", 60),
		"one two three four",
	}
	got, err := normalizeKeywordList(strings.Join(boundary, ","))
	if err != nil {
		t.Fatalf("normalizeKeywordList error = %v", err)
	}
	if len(got) != len(boundary) {
		t.Fatalf("keywords = %v, want %v", got, boundary)
	}

	if _, err := normalizeKeywordList(strings.Join(manyKeywords(100), ",")); err != nil {
		t.Fatalf("100 keywords must be accepted, got %v", err)
	}
}

func TestFanOutKeywordsBoundsConcurrencyAndPreservesOrder(t *testing.T) {
	keywords := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}

	for _, workers := range []int{1, 3} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			var (
				mu        sync.Mutex
				active    int
				maxActive int
			)
			results := fanOutKeywords(context.Background(), keywords, workers, func(_ context.Context, keyword string) (string, error) {
				mu.Lock()
				active++
				if active > maxActive {
					maxActive = active
				}
				mu.Unlock()
				time.Sleep(20 * time.Millisecond)
				mu.Lock()
				active--
				mu.Unlock()
				return strings.ToUpper(keyword), nil
			})

			if len(results) != len(keywords) {
				t.Fatalf("results = %d, want %d", len(results), len(keywords))
			}
			for index, result := range results {
				if result.Keyword != keywords[index] {
					t.Fatalf("results[%d].Keyword = %q, want %q", index, result.Keyword, keywords[index])
				}
				if result.Err != nil {
					t.Fatalf("results[%d].Err = %v", index, result.Err)
				}
				if result.Value != strings.ToUpper(keywords[index]) {
					t.Fatalf("results[%d].Value = %q", index, result.Value)
				}
			}
			if maxActive > workers {
				t.Fatalf("max concurrent keyword lookups = %d, want at most %d", maxActive, workers)
			}
			if workers > 1 && maxActive < 2 {
				t.Fatalf("max concurrent keyword lookups = %d, want parallel execution", maxActive)
			}
		})
	}
}

func TestFanOutKeywordsAccumulatesFailuresWithoutCancellingPeers(t *testing.T) {
	keywords := []string{"alpha", "bravo", "charlie"}
	results := fanOutKeywords(context.Background(), keywords, 3, func(_ context.Context, keyword string) (string, error) {
		if keyword == "bravo" {
			return "", errors.New("bravo failed")
		}
		return keyword, nil
	})

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Fatalf("peer failures leaked: %+v", results)
	}
	if results[1].Err == nil || results[1].Err.Error() != "bravo failed" {
		t.Fatalf("results[1].Err = %v, want bravo failure", results[1].Err)
	}
}

func TestRepresentativeKeywordErrorPrefersServerFailuresThenLowestStatus(t *testing.T) {
	tests := []struct {
		name string
		errs []error
		want string
	}{
		{
			name: "no failures",
			errs: []error{nil, nil},
			want: "",
		},
		{
			name: "server failure outranks client failure",
			errs: []error{statusErrorStub(429), statusErrorStub(503), statusErrorStub(500)},
			want: "stub request returned status 500",
		},
		{
			name: "lowest client status within class",
			errs: []error{statusErrorStub(429), statusErrorStub(404)},
			want: "stub request returned status 404",
		},
		{
			name: "falls back to the first failure",
			errs: []error{nil, errors.New("network unreachable"), errors.New("second")},
			want: "network unreachable",
		},
		{
			name: "status failure outranks plain failure",
			errs: []error{errors.New("network unreachable"), statusErrorStub(500)},
			want: "stub request returned status 500",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := representativeKeywordError(test.errs)
			if test.want == "" {
				if got != nil {
					t.Fatalf("error = %v, want nil", got)
				}
				return
			}
			if got == nil || got.Error() != test.want {
				t.Fatalf("error = %v, want %q", got, test.want)
			}
		})
	}
}

func TestKeywordsGroupReturnsHelpAndRegistersRank(t *testing.T) {
	group := KeywordsCommand()
	if group.FlagSet == nil {
		t.Fatal("keywords group is missing a FlagSet")
	}
	if group.UsageFunc == nil {
		t.Fatal("keywords group is missing UsageFunc")
	}
	if err := group.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("keywords Exec error = %v, want flag.ErrHelp", err)
	}

	var names []string
	for _, sub := range group.Subcommands {
		names = append(names, sub.Name)
		if sub.UsageFunc == nil {
			t.Fatalf("subcommand %q is missing UsageFunc", sub.Name)
		}
	}
	if len(names) != 3 || names[0] != "rank" || names[1] != "score" || names[2] != "discover" {
		t.Fatalf("keywords subcommands = %v, want [rank score discover]", names)
	}
}

func TestOptimizeGroupRegistersKeywords(t *testing.T) {
	var names []string
	for _, sub := range OptimizeCommand().Subcommands {
		names = append(names, sub.Name)
	}
	found := false
	for _, name := range names {
		if name == "keywords" {
			found = true
		}
	}
	if !found {
		t.Fatalf("optimize subcommands = %v, want keywords", names)
	}
}

func manyKeywords(count int) []string {
	keywords := make([]string, 0, count)
	for index := range count {
		keywords = append(keywords, fmt.Sprintf("keyword%03d", index))
	}
	return keywords
}

type stubStatusError int

func statusErrorStub(status int) error { return stubStatusError(status) }

func (e stubStatusError) Error() string {
	return fmt.Sprintf("stub request returned status %d", int(e))
}

func (e stubStatusError) HTTPStatusCode() int { return int(e) }
