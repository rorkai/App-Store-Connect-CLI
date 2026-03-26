package validate

import (
	"context"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// SetClientFactory replaces the ASC client factory for tests.
// It returns a restore function to reset the previous handler.
func SetClientFactory(fn func() (*asc.Client, error)) func() {
	previous := clientFactory
	if fn == nil {
		clientFactory = shared.GetASCClient
	} else {
		clientFactory = fn
	}
	return func() {
		clientFactory = previous
	}
}

// SetFetchSubscriptionsFunc replaces the subscription fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchSubscriptionsFunc(fn func(context.Context, *asc.Client, string) ([]validation.Subscription, error)) func() {
	previous := fetchSubscriptionsFn
	if fn == nil {
		fetchSubscriptionsFn = fetchSubscriptions
	} else {
		fetchSubscriptionsFn = fn
	}
	return func() {
		fetchSubscriptionsFn = previous
	}
}

// SetFetchIAPsFunc replaces the IAP fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchIAPsFunc(fn func(context.Context, *asc.Client, string) ([]validation.IAP, error)) func() {
	previous := fetchIAPsFn
	if fn == nil {
		fetchIAPsFn = fetchIAPs
	} else {
		fetchIAPsFn = fn
	}
	return func() {
		fetchIAPsFn = previous
	}
}

// SetFetchAvailableTerritoriesFunc replaces the availability fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchAvailableTerritoriesFunc(fn func(context.Context, *asc.Client, string) (string, int, error)) func() {
	previousDetails := fetchAvailableTerritoryDetailsFn
	if fn == nil {
		fetchAvailableTerritoryDetailsFn = fetchAvailableTerritoryDetails
	} else {
		fetchAvailableTerritoryDetailsFn = func(ctx context.Context, client *asc.Client, appID string) (string, []string, int, error) {
			availabilityID, availableTerritories, err := fn(ctx, client, appID)
			return availabilityID, nil, availableTerritories, err
		}
	}
	return func() {
		fetchAvailableTerritoryDetailsFn = previousDetails
	}
}

// SetFetchAppBuildCountFunc replaces the app build-count fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchAppBuildCountFunc(fn func(context.Context, *asc.Client, string) (int, bool, string, error)) func() {
	previous := fetchAppBuildCountFn
	if fn == nil {
		fetchAppBuildCountFn = fetchAppBuildCount
	} else {
		fetchAppBuildCountFn = func(ctx context.Context, client *asc.Client, appID string) (int, metadataCheckStatus, error) {
			count, verified, skipReason, err := fn(ctx, client, appID)
			return count, metadataCheckStatus{Verified: verified, SkipReason: skipReason}, err
		}
	}
	return func() {
		fetchAppBuildCountFn = previous
	}
}

// SetFetchScreenshotSetsFunc replaces the screenshot-set fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchScreenshotSetsFunc(fn func(context.Context, *asc.Client, []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]validation.ScreenshotSet, error)) func() {
	previous := fetchScreenshotSetsFn
	if fn == nil {
		fetchScreenshotSetsFn = fetchScreenshotSets
	} else {
		fetchScreenshotSetsFn = fn
	}
	return func() {
		fetchScreenshotSetsFn = previous
	}
}
