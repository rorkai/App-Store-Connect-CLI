package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type appLookupClient interface {
	GetApps(ctx context.Context, opts ...asc.AppsOption) (*asc.AppsResponse, error)
}

// ResolveAppIDWithExactLookup takes an already-resolved app identifier and
// looks it up by exact bundle ID and exact app name only. If no exact match
// exists and the value is numeric, it falls back to treating it as an App
// Store Connect app ID.
func ResolveAppIDWithExactLookup(ctx context.Context, client appLookupClient, appID string) (string, error) {
	resolved := strings.TrimSpace(appID)
	if resolved == "" {
		return "", nil
	}
	if IsNumericAppID(resolved) {
		return resolved, nil
	}
	if client == nil {
		return "", fmt.Errorf("app lookup client is required for non-numeric --app values")
	}

	byBundle, err := client.GetApps(ctx, asc.WithAppsBundleIDs([]string{resolved}), asc.Limit[asc.AppsOption](2))
	if err != nil {
		return "", fmt.Errorf("resolve app by bundle ID: %w", err)
	}
	if byBundle == nil {
		return "", fmt.Errorf("resolve app by bundle ID: empty response")
	}
	bundlePageHasNext := strings.TrimSpace(byBundle.Links.Next) != ""
	if bundlePageHasNext && len(byBundle.Data) == 0 {
		return "", MarkAmbiguousSelectionSample(AmbiguousError("app", "--app", resolved, nil))
	}
	if len(byBundle.Data) == 1 {
		if bundlePageHasNext {
			return "", MarkAmbiguousSelectionSample(AmbiguousError("app", "--app", resolved, AppCandidates(byBundle.Data)))
		}
		return strings.TrimSpace(byBundle.Data[0].ID), nil
	}
	if len(byBundle.Data) > 1 {
		ambiguous := AmbiguousError("app", "--app", resolved, AppCandidates(byBundle.Data))
		if bundlePageHasNext {
			ambiguous = MarkAmbiguousSelectionSample(ambiguous)
		}
		return "", ambiguous
	}

	nameMatches, err := findExactAppNameMatches(ctx, client, resolved, true)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(nameMatches) == 1 {
		return nameMatches[0].ID, nil
	}
	if len(nameMatches) > 1 {
		return "", AmbiguousError("app", "--app", resolved, nameMatches)
	}

	// ASC name filtering is fuzzy in practice; full-scan fallback preserves exact-name semantics.
	nameMatches, err = findExactAppNameMatches(ctx, client, resolved, false)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(nameMatches) == 1 {
		return nameMatches[0].ID, nil
	}
	if len(nameMatches) > 1 {
		return "", AmbiguousError("app", "--app", resolved, nameMatches)
	}

	return "", fmt.Errorf("app %q not found (expected app ID, exact bundle ID, or exact app name)", resolved)
}

// ResolveAppIDWithLookup takes an already-resolved app identifier and, when
// non-numeric, looks it up by exact bundle ID, exact app name, then legacy
// fuzzy-name matching when unique.
func ResolveAppIDWithLookup(ctx context.Context, client appLookupClient, appID string) (string, error) {
	resolved := strings.TrimSpace(appID)
	if resolved == "" {
		return "", nil
	}
	if IsNumericAppID(resolved) {
		return resolved, nil
	}
	if client == nil {
		return "", fmt.Errorf("app lookup client is required for non-numeric --app values")
	}

	byBundle, err := client.GetApps(ctx, asc.WithAppsBundleIDs([]string{resolved}), asc.Limit[asc.AppsOption](2))
	if err != nil {
		return "", fmt.Errorf("resolve app by bundle ID: %w", err)
	}
	if byBundle == nil {
		return "", fmt.Errorf("resolve app by bundle ID: empty response")
	}
	bundlePageHasNext := strings.TrimSpace(byBundle.Links.Next) != ""
	if bundlePageHasNext && len(byBundle.Data) == 0 {
		return "", MarkAmbiguousSelectionSample(AmbiguousError("app", "--app", resolved, nil))
	}
	if len(byBundle.Data) == 1 {
		if bundlePageHasNext {
			return "", MarkAmbiguousSelectionSample(AmbiguousError("app", "--app", resolved, AppCandidates(byBundle.Data)))
		}
		return strings.TrimSpace(byBundle.Data[0].ID), nil
	}
	if len(byBundle.Data) > 1 {
		ambiguous := AmbiguousError("app", "--app", resolved, AppCandidates(byBundle.Data))
		if bundlePageHasNext {
			ambiguous = MarkAmbiguousSelectionSample(ambiguous)
		}
		return "", ambiguous
	}

	nameMatches, err := findExactAppNameMatches(ctx, client, resolved, true)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(nameMatches) == 1 {
		return nameMatches[0].ID, nil
	}
	if len(nameMatches) > 1 {
		return "", AmbiguousError("app", "--app", resolved, nameMatches)
	}

	// ASC name filtering is fuzzy in practice; full-scan fallback preserves exact-name semantics.
	nameMatches, err = findExactAppNameMatches(ctx, client, resolved, false)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(nameMatches) == 1 {
		return nameMatches[0].ID, nil
	}
	if len(nameMatches) > 1 {
		return "", AmbiguousError("app", "--app", resolved, nameMatches)
	}

	// Backward compatibility: if no exact name match exists, keep legacy behavior
	// by accepting a unique fuzzy name-filter result.
	fuzzyMatches, fuzzyMatchesAreSample, err := findFuzzyAppNameMatches(ctx, client, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(fuzzyMatches) == 1 {
		if fuzzyMatchesAreSample {
			return "", MarkAmbiguousSelectionSample(AmbiguousError("app", "--app", resolved, fuzzyMatches))
		}
		return fuzzyMatches[0].ID, nil
	}
	if len(fuzzyMatches) > 1 {
		ambiguous := AmbiguousError("app", "--app", resolved, fuzzyMatches)
		if fuzzyMatchesAreSample {
			ambiguous = MarkAmbiguousSelectionSample(ambiguous)
		}
		return "", ambiguous
	}
	if fuzzyMatchesAreSample {
		return "", MarkAmbiguousSelectionSample(AmbiguousError("app", "--app", resolved, fuzzyMatches))
	}
	return "", fmt.Errorf("app %q not found (expected app ID, exact bundle ID, or exact app name)", resolved)
}

func findExactAppNameMatches(ctx context.Context, client appLookupClient, name string, useNameFilter bool) ([]AmbiguousCandidate, error) {
	name = strings.TrimSpace(name)
	if name == "" || client == nil {
		return nil, nil
	}

	opts := []asc.AppsOption{asc.Limit[asc.AppsOption](200)}
	if useNameFilter {
		opts = append(opts, asc.WithAppsNames([]string{name}))
	}

	firstPage, err := client.GetApps(ctx, opts...)
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, nil
	}

	seen := map[string]struct{}{}
	matches := make([]AmbiguousCandidate, 0, 1)
	collect := func(resp *asc.AppsResponse) {
		if resp == nil {
			return
		}
		for _, app := range resp.Data {
			if !strings.EqualFold(strings.TrimSpace(app.Attributes.Name), name) {
				continue
			}
			id := strings.TrimSpace(app.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			matches = append(matches, appCandidate(app))
		}
	}

	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetApps(ctx, asc.NextURL[asc.AppsOption](nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.AppsResponse)
			if !ok {
				return fmt.Errorf("unexpected apps page type %T", page)
			}
			collect(resp)
			return nil
		},
	); err != nil {
		return nil, err
	}

	sortAmbiguousCandidatesByID(matches)
	return matches, nil
}

func findFuzzyAppNameMatches(ctx context.Context, client appLookupClient, name string) ([]AmbiguousCandidate, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" || client == nil {
		return nil, false, nil
	}

	resp, err := client.GetApps(ctx, asc.WithAppsNames([]string{name}), asc.Limit[asc.AppsOption](2))
	if err != nil {
		return nil, false, err
	}
	if resp == nil {
		return nil, false, nil
	}

	seen := map[string]struct{}{}
	matches := make([]AmbiguousCandidate, 0, len(resp.Data))
	for _, app := range resp.Data {
		id := strings.TrimSpace(app.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		matches = append(matches, appCandidate(app))
	}
	sortAmbiguousCandidatesByID(matches)
	return matches, strings.TrimSpace(resp.Links.Next) != "", nil
}

// IsNumericAppID reports whether a value is a non-empty decimal app ID.
func IsNumericAppID(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
