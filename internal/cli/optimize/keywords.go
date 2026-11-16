package optimize

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

// Keyword hygiene limits shared by every `asc optimize keywords` subcommand.
// They keep one invocation bounded and reviewable instead of turning a single
// command into an unbounded crawl of Apple's public endpoints.
const (
	keywordMinLength = 2
	keywordMaxLength = 60
	keywordMaxWords  = 4
	keywordMaxCount  = 100
)

// Keyword evidence status values mirror the official Apple Ads source status
// vocabulary so an unavailable source is never confused with an empty result.
const (
	keywordStatusAvailable   = "available"
	keywordStatusEmpty       = "empty"
	keywordStatusUnavailable = "unavailable"
)

// newKeywordsItunesClient returns the public App Store client the keyword
// commands use, sending through d's transport.
func newKeywordsItunesClient(d *shared.Deps) *itunes.Client {
	client := itunes.NewClient()
	client.HTTPClient = &http.Client{Transport: d.HTTPTransport()}
	return client
}

// newKeywordsCommand returns the keyword evaluation group.
func newKeywordsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("keywords", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "keywords",
		ShortUsage: "asc optimize keywords <subcommand> [flags]",
		ShortHelp:  "Evaluate App Store keyword candidates.",
		LongHelp: `Evaluate App Store keyword candidates against official Apple data.

The rank and score commands evaluate keywords you already have. The discover
command produces only the candidates Apple itself suggests through its official
Ads endpoints; nothing here invents a keyword. Every computed value is reported
next to the raw inputs it was derived from, and an unavailable source is
reported instead of replaced.

Typical loop: discover official candidates, score them to decide which are
worth pursuing, then rank the ones that were adopted.`,
		FlagSet:     fs,
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newKeywordsRankCommand(d), newKeywordsScoreCommand(d), newKeywordsDiscoverCommand(d)},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// normalizeKeywordList splits a comma-separated keyword flag into canonical
// keywords: trimmed, lowercased, internally whitespace-collapsed, and
// deduplicated while preserving the caller's order.
func normalizeKeywordList(d *shared.Deps, raw string) ([]string, error) {
	keywords := make([]string, 0, strings.Count(raw, ",")+1)
	seen := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		keyword := strings.ToLower(strings.Join(strings.Fields(entry), " "))
		if keyword == "" {
			continue
		}
		if length := len([]rune(keyword)); length < keywordMinLength || length > keywordMaxLength {
			return nil, d.UsageErrorf(
				"--keywords entry %q must be between %d and %d characters",
				keyword, keywordMinLength, keywordMaxLength,
			)
		}
		if words := strings.Count(keyword, " ") + 1; words > keywordMaxWords {
			return nil, d.UsageErrorf(
				"--keywords entry %q must contain at most %d space-separated words",
				keyword, keywordMaxWords,
			)
		}
		if _, duplicate := seen[keyword]; duplicate {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	if len(keywords) == 0 {
		return nil, d.UsageError("--keywords must contain at least one keyword")
	}
	if len(keywords) > keywordMaxCount {
		return nil, d.UsageErrorf("--keywords accepts at most %d keywords per invocation", keywordMaxCount)
	}
	return keywords, nil
}

// normalizeKeywordCountry validates an ISO alpha-2 storefront and returns the
// lowercase form the public iTunes endpoints expect.
func normalizeKeywordCountry(d *shared.Deps, country string) (string, error) {
	normalized, err := itunes.NormalizeCountryCode(country)
	if err != nil || normalized == "" {
		return "", d.UsageErrorf("--country %q is not a supported App Store storefront", strings.TrimSpace(country))
	}
	return normalized, nil
}

// normalizeKeywordAppID resolves and validates the public App Store app ID.
func normalizeKeywordAppID(d *shared.Deps, appID string) (string, error) {
	resolved := strings.TrimSpace(d.ResolveAppID(appID))
	if resolved == "" {
		return "", d.UsageError("--app is required (or set ASC_APP_ID)")
	}
	for _, digit := range resolved {
		if digit < '0' || digit > '9' {
			return "", d.UsageError("--app must be a numeric App Store app ID")
		}
	}
	parsed, err := strconv.ParseInt(resolved, 10, 64)
	if err != nil || parsed <= 0 {
		return "", d.UsageError("--app must be a positive App Store app ID representable as a 64-bit integer")
	}
	return resolved, nil
}

// keywordFanOutResult pairs one keyword with the outcome of its lookup. A
// failed keyword keeps its slot so partial evidence stays attributable.
type keywordFanOutResult[T any] struct {
	Keyword string
	Value   T
	Err     error
}

// fanOutKeywords runs one bounded-concurrency lookup per keyword. Individual
// failures are accumulated rather than cancelling the remaining keywords, and
// results are returned in the caller's keyword order.
func fanOutKeywords[T any](
	ctx context.Context,
	keywords []string,
	workers int,
	run func(context.Context, string) (T, error),
) []keywordFanOutResult[T] {
	if workers < 1 {
		workers = 1
	}

	results := make([]keywordFanOutResult[T], len(keywords))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for index, keyword := range keywords {
		results[index].Keyword = keyword
		wg.Add(1)
		go func(index int, keyword string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				results[index].Err = ctx.Err()
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			results[index].Value, results[index].Err = run(ctx, keyword)
		}(index, keyword)
	}

	wg.Wait()
	return results
}

// representativeKeywordError selects one stable error to report when every
// keyword failed. Server failures outrank client failures because they best
// describe an endpoint outage, and within a class the lowest status wins so
// the reported cause does not depend on goroutine completion order.
func representativeKeywordError(errs []error) error {
	var (
		selected       error
		selectedStatus int
	)
	for _, err := range errs {
		if err == nil {
			continue
		}
		status := 0
		var statusError interface{ HTTPStatusCode() int }
		if errors.As(err, &statusError) {
			status = statusError.HTTPStatusCode()
		}
		switch {
		case selected == nil,
			status > 0 && selectedStatus == 0,
			status >= 500 && selectedStatus > 0 && selectedStatus < 500,
			status > 0 && status/100 == selectedStatus/100 && status < selectedStatus:
			selected = err
			selectedStatus = status
		}
	}
	return selected
}
