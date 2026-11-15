package subscriptions

import (
	"context"
	"flag"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const subscriptionLookupAppUsage = "App Store Connect app ID (or ASC_APP_ID env; required when --subscription-id uses a product ID or name)"

func addSubscriptionLookupAppFlag(fs *flag.FlagSet) *string {
	return fs.String("app", "", subscriptionLookupAppUsage)
}

func resolveSubscriptionLookupID(ctx context.Context, client *asc.Client, appValue, selector string) (string, error) {
	resolvedAppID := shared.ResolveAppID(appValue)
	if err := shared.RequireAppForStableSelector(resolvedAppID, selector, "--subscription-id"); err != nil {
		return "", err
	}
	return shared.ResolveSubscriptionID(ctx, client, resolvedAppID, selector)
}

func resolveSubscriptionLookupIDWithTimeout(ctx context.Context, client *asc.Client, appValue, selector string) (string, error) {
	return resolveSubscriptionLookupIDWithTimeoutWith(ctx, shared.DefaultDeps(), client, appValue, selector)
}

func resolveSubscriptionLookupIDWithTimeoutWith(ctx context.Context, d *shared.Deps, client *asc.Client, appValue, selector string) (string, error) {
	lookupCtx, cancel := d.Timeout(ctx)
	defer cancel()

	return resolveSubscriptionLookupID(lookupCtx, client, appValue, selector)
}
