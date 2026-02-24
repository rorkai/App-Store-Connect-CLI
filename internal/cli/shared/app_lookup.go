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

// ResolveAppIDWithLookup resolves non-numeric app identifiers by exact bundle ID
// or exact app name.
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

	byName, err := client.GetApps(ctx, asc.WithAppsNames([]string{resolved}), asc.WithAppsLimit(2))
	if err != nil {
		return "", fmt.Errorf("resolve app by name: %w", err)
	}
	if len(byName.Data) == 1 {
		return strings.TrimSpace(byName.Data[0].ID), nil
	}
	if len(byName.Data) > 1 {
		return "", fmt.Errorf("multiple apps found for name %q; use --app with App Store Connect app ID", resolved)
	}

	return "", fmt.Errorf("app %q not found (expected app ID, exact bundle ID, or exact app name)", resolved)
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
