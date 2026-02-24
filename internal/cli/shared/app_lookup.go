package shared

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type appLookupClient interface {
	GetApps(ctx context.Context, opts ...asc.AppsOption) (*asc.AppsResponse, error)
}

// ResolveAppIDWithLookup takes an already-resolved app identifier and, when
// non-numeric, looks it up by exact bundle ID, exact app name, then legacy
// fuzzy-name matching when unique.
func ResolveAppIDWithLookup(ctx context.Context, client appLookupClient, appID string) (string, error) {
	resolved := strings.TrimSpace(appID)
	if resolved == "" {
		return "", nil
	}
	if isNumericAppID(resolved) {
		return resolved, nil
	}
	if client == nil {
		return "", fmt.Errorf("app lookup client is required for non-numeric --app values")
	}

	byBundle, err := client.GetApps(ctx, asc.WithAppsBundleIDs([]string{resolved}), asc.WithAppsLimit(2))
	if err != nil {
		return "", fmt.Errorf("resolve app by bundle ID: %w", err)
	}
	if len(byBundle.Data) == 1 {
		return strings.TrimSpace(byBundle.Data[0].ID), nil
	}
	if len(byBundle.Data) > 1 {
		return "", fmt.Errorf("multiple apps found for bundle ID %q; use --app with App Store Connect app ID", resolved)
	}

	nameMatchIDs, err := findExactAppNameMatches(ctx, client, resolved, true)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(nameMatchIDs) == 1 {
		return nameMatchIDs[0], nil
	}
	if len(nameMatchIDs) > 1 {
		return "", fmt.Errorf("multiple apps found for name %q (%s); use --app with App Store Connect app ID", resolved, strings.Join(nameMatchIDs, ", "))
	}

	// ASC name filtering is fuzzy in practice; full-scan fallback preserves exact-name semantics.
	nameMatchIDs, err = findExactAppNameMatches(ctx, client, resolved, false)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(nameMatchIDs) == 1 {
		return nameMatchIDs[0], nil
	}
	if len(nameMatchIDs) > 1 {
		return "", fmt.Errorf("multiple apps found for name %q (%s); use --app with App Store Connect app ID", resolved, strings.Join(nameMatchIDs, ", "))
	}

	// Backward compatibility: if no exact name match exists, keep legacy behavior
	// by accepting a unique fuzzy name-filter result.
	fuzzyMatches, err := findFuzzyAppNameMatches(ctx, client, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(fuzzyMatches) == 1 {
		return fuzzyMatches[0], nil
	}
	if len(fuzzyMatches) > 1 {
		return "", fmt.Errorf("multiple apps found for name %q (%s); use --app with App Store Connect app ID", resolved, strings.Join(fuzzyMatches, ", "))
	}
	return "", fmt.Errorf("app %q not found (expected app ID, exact bundle ID, or exact app name)", resolved)
}

func findExactAppNameMatches(ctx context.Context, client appLookupClient, name string, useNameFilter bool) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" || client == nil {
		return nil, nil
	}

	opts := []asc.AppsOption{asc.WithAppsLimit(200)}
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
	matchIDs := make([]string, 0, 1)
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
			matchIDs = append(matchIDs, id)
		}
	}

	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetApps(ctx, asc.WithAppsNextURL(nextURL))
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

	sort.Strings(matchIDs)
	return matchIDs, nil
}

func findFuzzyAppNameMatches(ctx context.Context, client appLookupClient, name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" || client == nil {
		return nil, nil
	}

	resp, err := client.GetApps(ctx, asc.WithAppsNames([]string{name}), asc.WithAppsLimit(2))
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	seen := map[string]struct{}{}
	ids := make([]string, 0, len(resp.Data))
	for _, app := range resp.Data {
		id := strings.TrimSpace(app.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func isNumericAppID(value string) bool {
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
