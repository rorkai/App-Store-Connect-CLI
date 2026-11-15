package iap

import (
	"context"
	"flag"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const iapLookupAppUsage = "App Store Connect app ID (or ASC_APP_ID env; required when --iap-id uses a product ID or name)"

func addIAPLookupAppFlag(fs *flag.FlagSet) *string {
	return fs.String("app", "", iapLookupAppUsage)
}

func resolveIAPLookupID(ctx context.Context, client *asc.Client, appValue, selector string) (string, error) {
	return resolveIAPLookupIDWith(ctx, shared.DefaultDeps(), client, appValue, selector)
}

func resolveIAPLookupIDWith(ctx context.Context, d *shared.Deps, client *asc.Client, appValue, selector string) (string, error) {
	resolvedAppID := shared.ResolveAppID(appValue)
	if err := d.RequireAppForStableSelector(resolvedAppID, selector, "--iap-id"); err != nil {
		return "", err
	}
	return shared.ResolveIAPID(ctx, client, resolvedAppID, selector)
}

func resolveIAPLookupIDWithTimeout(ctx context.Context, client *asc.Client, appValue, selector string) (string, error) {
	return resolveIAPLookupIDWithTimeoutWith(ctx, shared.DefaultDeps(), client, appValue, selector)
}

func resolveIAPLookupIDWithTimeoutWith(ctx context.Context, d *shared.Deps, client *asc.Client, appValue, selector string) (string, error) {
	lookupCtx, cancel := d.Timeout(ctx)
	defer cancel()

	return resolveIAPLookupIDWith(lookupCtx, d, client, appValue, selector)
}
