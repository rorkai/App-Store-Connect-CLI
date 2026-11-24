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

// ResolveAppIDWithExactLookup resolves scoped app selectors with the same
// identity checks as ResolveAppIDWithLookup.
func ResolveAppIDWithExactLookup(ctx context.Context, client appLookupClient, appID string) (string, error) {
	return ResolveAppIDWithLookup(ctx, client, appID)
}

// ResolveAppIDWithLookup resolves a numeric app ID, exact bundle ID or exact
// app name. Use apps list to discover apps when the exact selector is unknown.
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

	appID, err := findAppIDByExactBundleID(ctx, client, resolved)
	if err != nil {
		return "", err
	}
	if appID != "" {
		return appID, nil
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

	return "", fmt.Errorf("app %q not found (expected app ID, exact bundle ID, or exact app name); use asc apps list to discover apps", resolved)
}

// ResolveAppIDByExactBundleID resolves a bundle identifier without falling
// back to the app's display name.
func ResolveAppIDByExactBundleID(ctx context.Context, client appLookupClient, bundleID string) (string, error) {
	bundleID = strings.TrimSpace(bundleID)
	if client == nil || bundleID == "" {
		return "", fmt.Errorf("app lookup client and bundle ID are required")
	}
	id, err := findAppIDByExactBundleID(ctx, client, bundleID)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("no app found for bundle ID %q (exact match required)", bundleID)
	}
	return id, nil
}

func findAppIDByExactBundleID(ctx context.Context, client appLookupClient, bundleID string) (string, error) {
	first, err := client.GetApps(ctx, asc.WithAppsBundleIDs([]string{bundleID}), asc.Limit[asc.AppsOption](200))
	if err != nil {
		return "", fmt.Errorf("resolve app by bundle ID: %w", err)
	}
	if first == nil {
		return "", fmt.Errorf("resolve app by bundle ID: empty response")
	}
	// App Store Connect's bundle filter can return prefix matches (observed
	// with apps view on asc 7.3.1, 2026-11-24).
	seen := map[string]struct{}{}
	var matches []AmbiguousCandidate
	err = asc.PaginateEach(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetApps(ctx, asc.NextURL[asc.AppsOption](nextURL))
	}, func(page asc.PaginatedResponse) error {
		for _, app := range page.(*asc.AppsResponse).Data {
			id := strings.TrimSpace(app.ID)
			if id == "" || !strings.EqualFold(strings.TrimSpace(app.Attributes.BundleID), bundleID) {
				continue
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				matches = append(matches, appCandidate(app))
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("resolve app by bundle ID: %w", err)
	}
	if len(matches) > 1 {
		sortAmbiguousCandidatesByID(matches)
		return "", AmbiguousError("app", "--app", bundleID, matches)
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	return "", nil
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
