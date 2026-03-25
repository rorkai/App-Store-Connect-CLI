package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type subscriptionImageStatus struct {
	HasImage   bool
	Verified   bool
	SkipReason string
}

type metadataCheckStatus struct {
	Verified   bool
	SkipReason string
}

var fetchSubscriptionsFn = fetchSubscriptions

func fetchSubscriptions(ctx context.Context, client *asc.Client, appID string) ([]validation.Subscription, error) {
	groupsCtx, groupsCancel := shared.ContextWithTimeout(ctx)
	groupsResp, err := client.GetSubscriptionGroups(groupsCtx, appID, asc.WithSubscriptionGroupsLimit(200))
	groupsCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription groups: %w", err)
	}

	paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionGroups(pageCtx, appID, asc.WithSubscriptionGroupsNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate subscription groups: %w", err)
	}

	groups, ok := paginatedGroups.(*asc.SubscriptionGroupsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected subscription groups response type %T", paginatedGroups)
	}

	groupLocalizations := make(map[string][]validation.SubscriptionGroupLocalizationInfo)
	groupLocalizationStatuses := make(map[string]metadataCheckStatus)
	groupNames := make(map[string]string)
	for _, group := range groups.Data {
		groupID := strings.TrimSpace(group.ID)
		if groupID == "" {
			continue
		}
		groupNames[groupID] = strings.TrimSpace(group.Attributes.ReferenceName)
	}

	subscriptions := make([]validation.Subscription, 0)
	for _, group := range groups.Data {
		groupID := strings.TrimSpace(group.ID)
		if groupID == "" {
			continue
		}

		subsCtx, subsCancel := shared.ContextWithTimeout(ctx)
		subsResp, err := client.GetSubscriptions(subsCtx, groupID, asc.WithSubscriptionsLimit(200))
		subsCancel()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch subscriptions for group %s: %w", groupID, err)
		}

		paginatedSubs, err := asc.PaginateAll(ctx, subsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
			defer pageCancel()
			return client.GetSubscriptions(pageCtx, groupID, asc.WithSubscriptionsNextURL(nextURL))
		})
		if err != nil {
			return nil, fmt.Errorf("paginate subscriptions: %w", err)
		}

		subsResult, ok := paginatedSubs.(*asc.SubscriptionsResponse)
		if !ok {
			return nil, fmt.Errorf("unexpected subscriptions response type %T", paginatedSubs)
		}

		for _, sub := range subsResult.Data {
			imageStatus, err := subscriptionHasImage(ctx, client, sub.ID)
			if err != nil {
				return nil, fmt.Errorf("fetch subscription images for %s: %w", strings.TrimSpace(sub.ID), err)
			}

			attrs := sub.Attributes
			valSub := validation.Subscription{
				ID:                   sub.ID,
				Name:                 attrs.Name,
				ProductID:            attrs.ProductID,
				State:                attrs.State,
				GroupID:              groupID,
				GroupName:            groupNames[groupID],
				HasImage:             imageStatus.HasImage,
				ImageCheckSkipped:    !imageStatus.Verified,
				ImageCheckSkipReason: imageStatus.SkipReason,
			}

			// Fetch price count for all active subscriptions (used for both
			// diagnostics and territory coverage checks).
			state := strings.ToUpper(strings.TrimSpace(attrs.State))
			if state != "REMOVED_FROM_SALE" && state != "DEVELOPER_REMOVED_FROM_SALE" {
				priceTerritories, priceStatus, err := fetchSubscriptionPriceTerritories(ctx, client, sub.ID)
				if err != nil {
					return nil, fmt.Errorf("fetch subscription prices for %s: %w", strings.TrimSpace(sub.ID), err)
				}
				valSub.PriceTerritories = priceTerritories
				valSub.PriceCount = len(priceTerritories)
				valSub.PriceCheckSkipped = !priceStatus.Verified
				valSub.PriceCheckSkipReason = priceStatus.SkipReason

				if strings.EqualFold(state, "MISSING_METADATA") {
					if _, ok := groupLocalizationStatuses[groupID]; !ok {
						locs, status, err := fetchGroupLocalizations(ctx, client, groupID)
						if err != nil {
							return nil, fmt.Errorf("fetch subscription group localizations for group %s: %w", groupID, err)
						}
						groupLocalizations[groupID] = locs
						groupLocalizationStatuses[groupID] = status
					}
					groupLocalizationStatus := groupLocalizationStatuses[groupID]
					valSub.GroupLocalizations = groupLocalizations[groupID]
					valSub.GroupLocalizationCheckSkipped = !groupLocalizationStatus.Verified
					valSub.GroupLocalizationCheckReason = groupLocalizationStatus.SkipReason

					localizations, localizationStatus, err := fetchSubscriptionLocalizations(ctx, client, sub.ID)
					if err != nil {
						return nil, fmt.Errorf("fetch subscription localizations for %s: %w", strings.TrimSpace(sub.ID), err)
					}
					valSub.Localizations = localizations
					valSub.LocalizationCheckSkipped = !localizationStatus.Verified
					valSub.LocalizationCheckSkipReason = localizationStatus.SkipReason

					reviewScreenshotID, reviewScreenshotStatus, err := fetchSubscriptionReviewScreenshot(ctx, client, sub.ID)
					if err != nil {
						return nil, fmt.Errorf("fetch subscription review screenshot for %s: %w", strings.TrimSpace(sub.ID), err)
					}
					valSub.ReviewScreenshotID = reviewScreenshotID
					valSub.ReviewScreenshotCheckSkipped = !reviewScreenshotStatus.Verified
					valSub.ReviewScreenshotCheckReason = reviewScreenshotStatus.SkipReason

					availabilityID, availabilityTerritories, availabilityStatus, err := fetchSubscriptionAvailabilityTerritories(ctx, client, sub.ID)
					if err != nil {
						return nil, fmt.Errorf("fetch subscription availability for %s: %w", strings.TrimSpace(sub.ID), err)
					}
					valSub.AvailabilityID = availabilityID
					valSub.AvailabilityTerritories = availabilityTerritories
					valSub.AvailabilityCheckSkipped = !availabilityStatus.Verified
					valSub.AvailabilityCheckSkipReason = availabilityStatus.SkipReason

					introOfferCount, introStatus, err := fetchSubscriptionIntroductoryOfferCount(ctx, client, sub.ID)
					if err != nil {
						return nil, fmt.Errorf("fetch subscription introductory offers for %s: %w", strings.TrimSpace(sub.ID), err)
					}
					valSub.IntroductoryOfferCount = introOfferCount
					valSub.IntroductoryOfferCheckSkipped = !introStatus.Verified
					valSub.IntroductoryOfferCheckReason = introStatus.SkipReason

					promoOfferCount, promoStatus, err := fetchSubscriptionPromotionalOfferCount(ctx, client, sub.ID)
					if err != nil {
						return nil, fmt.Errorf("fetch subscription promotional offers for %s: %w", strings.TrimSpace(sub.ID), err)
					}
					valSub.PromotionalOfferCount = promoOfferCount
					valSub.PromotionalOfferCheckSkipped = !promoStatus.Verified
					valSub.PromotionalOfferCheckReason = promoStatus.SkipReason

					winBackOfferCount, winBackStatus, err := fetchSubscriptionWinBackOfferCount(ctx, client, sub.ID)
					if err != nil {
						return nil, fmt.Errorf("fetch subscription win-back offers for %s: %w", strings.TrimSpace(sub.ID), err)
					}
					valSub.WinBackOfferCount = winBackOfferCount
					valSub.WinBackOfferCheckSkipped = !winBackStatus.Verified
					valSub.WinBackOfferCheckReason = winBackStatus.SkipReason
				}
			}

			subscriptions = append(subscriptions, valSub)
		}
	}

	return subscriptions, nil
}

func fetchGroupLocalizations(ctx context.Context, client *asc.Client, groupID string) ([]validation.SubscriptionGroupLocalizationInfo, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionGroupLocalizations(reqCtx, strings.TrimSpace(groupID), asc.WithSubscriptionGroupLocalizationsLimit(200))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription group localizations"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{}, err
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionGroupLocalizations(pageCtx, strings.TrimSpace(groupID), asc.WithSubscriptionGroupLocalizationsNextURL(nextURL))
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription group localizations"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{}, err
	}

	typed, ok := paginated.(*asc.SubscriptionGroupLocalizationsResponse)
	if !ok {
		return nil, metadataCheckStatus{}, fmt.Errorf("unexpected subscription group localizations response type %T", paginated)
	}

	locs := make([]validation.SubscriptionGroupLocalizationInfo, 0, len(typed.Data))
	for _, loc := range typed.Data {
		locs = append(locs, validation.SubscriptionGroupLocalizationInfo{
			Locale: strings.TrimSpace(loc.Attributes.Locale),
			Name:   strings.TrimSpace(loc.Attributes.Name),
		})
	}
	return locs, metadataCheckStatus{Verified: true}, nil
}

// fetchSubscriptionLocalizations fetches localization info for a subscription.
func fetchSubscriptionLocalizations(ctx context.Context, client *asc.Client, subscriptionID string) ([]validation.SubscriptionLocalizationInfo, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionLocalizations(reqCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionLocalizationsLimit(200))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription localizations"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{}, err
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionLocalizations(pageCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionLocalizationsNextURL(nextURL))
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription localizations"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{}, err
	}

	typed, ok := paginated.(*asc.SubscriptionLocalizationsResponse)
	if !ok {
		return nil, metadataCheckStatus{}, fmt.Errorf("unexpected subscription localizations response type %T", paginated)
	}

	locs := make([]validation.SubscriptionLocalizationInfo, 0, len(typed.Data))
	for _, loc := range typed.Data {
		locs = append(locs, validation.SubscriptionLocalizationInfo{
			Locale:      strings.TrimSpace(loc.Attributes.Locale),
			Name:        strings.TrimSpace(loc.Attributes.Name),
			Description: strings.TrimSpace(loc.Attributes.Description),
		})
	}
	return locs, metadataCheckStatus{Verified: true}, nil
}

// fetchSubscriptionPriceTerritories returns the unique territories with prices
// configured for a subscription. It paginates all price resources so scheduled
// price changes for the same territory don't inflate coverage.
func fetchSubscriptionPriceTerritories(ctx context.Context, client *asc.Client, subscriptionID string) ([]string, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionPrices(
		reqCtx,
		strings.TrimSpace(subscriptionID),
		asc.WithSubscriptionPricesInclude([]string{"territory"}),
		asc.WithSubscriptionPricesLimit(200),
	)
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription prices"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{SkipReason: "Validation skipped subscription prices because the App Store Connect endpoint returned an unexpected error"}, nil
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionPrices(pageCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionPricesNextURL(nextURL))
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription prices"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{SkipReason: "Validation skipped subscription prices because the App Store Connect endpoint returned an unexpected error"}, nil
	}

	typed, ok := paginated.(*asc.SubscriptionPricesResponse)
	if !ok {
		return nil, metadataCheckStatus{}, fmt.Errorf("unexpected subscription prices response type %T", paginated)
	}

	territories := make(map[string]struct{}, len(typed.Data))
	for _, price := range typed.Data {
		territoryID, err := subscriptionPriceTerritoryID(price.Relationships)
		if err != nil {
			return nil, metadataCheckStatus{SkipReason: "Validation could not determine unique subscription pricing territories because the API response relationships could not be decoded"}, nil
		}
		territoryID = strings.TrimSpace(territoryID)
		if territoryID == "" {
			return nil, metadataCheckStatus{SkipReason: "Validation could not determine unique subscription pricing territories because the API response omitted territory relationships"}, nil
		}
		territories[territoryID] = struct{}{}
	}

	territoryIDs := make([]string, 0, len(territories))
	for territoryID := range territories {
		territoryIDs = append(territoryIDs, territoryID)
	}
	slices.Sort(territoryIDs)

	return territoryIDs, metadataCheckStatus{Verified: true}, nil
}

func subscriptionPriceTerritoryID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var relationships asc.SubscriptionPriceRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", fmt.Errorf("decode subscription price relationships: %w", err)
	}
	if relationships.Territory == nil {
		return "", nil
	}
	return strings.TrimSpace(relationships.Territory.Data.ID), nil
}

func fetchSubscriptionReviewScreenshot(ctx context.Context, client *asc.Client, subscriptionID string) (string, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(reqCtx, strings.TrimSpace(subscriptionID))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", metadataCheckStatus{}, err
		}
		if asc.IsNotFound(err) {
			return "", metadataCheckStatus{Verified: true}, nil
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription App Review screenshot"); ok {
			return "", metadataCheckStatus{SkipReason: reason}, nil
		}
		return "", metadataCheckStatus{}, err
	}
	if resp == nil {
		return "", metadataCheckStatus{Verified: true}, nil
	}
	return strings.TrimSpace(resp.Data.ID), metadataCheckStatus{Verified: true}, nil
}

func fetchSubscriptionAvailabilityTerritories(ctx context.Context, client *asc.Client, subscriptionID string) (string, []string, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionAvailabilityForSubscription(reqCtx, strings.TrimSpace(subscriptionID))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", nil, metadataCheckStatus{}, err
		}
		if asc.IsNotFound(err) {
			return "", nil, metadataCheckStatus{Verified: true}, nil
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription availability"); ok {
			return "", nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return "", nil, metadataCheckStatus{}, err
	}

	availabilityID := strings.TrimSpace(resp.Data.ID)
	if availabilityID == "" {
		return "", nil, metadataCheckStatus{Verified: true}, nil
	}

	allTerritories := make([]string, 0)
	nextURL := ""
	for {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		var territoryResp *asc.TerritoriesResponse
		if strings.TrimSpace(nextURL) != "" {
			territoryResp, err = client.GetSubscriptionAvailabilityAvailableTerritories(pageCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesNextURL(nextURL))
		} else {
			territoryResp, err = client.GetSubscriptionAvailabilityAvailableTerritories(pageCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
		}
		pageCancel()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return "", nil, metadataCheckStatus{}, err
			}
			if reason, ok := metadataCheckSkipReason(err, "subscription availability territories"); ok {
				return "", nil, metadataCheckStatus{SkipReason: reason}, nil
			}
			return "", nil, metadataCheckStatus{}, err
		}

		for _, territory := range territoryResp.Data {
			allTerritories = append(allTerritories, strings.TrimSpace(territory.ID))
		}

		nextURL = strings.TrimSpace(territoryResp.Links.Next)
		if nextURL == "" {
			break
		}
	}

	return availabilityID, validation.SortedUniqueNonEmptyStrings(allTerritories), metadataCheckStatus{Verified: true}, nil
}

func fetchSubscriptionIntroductoryOfferCount(ctx context.Context, client *asc.Client, subscriptionID string) (int, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionIntroductoryOffers(reqCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionIntroductoryOffersLimit(1))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription introductory offers"); ok {
			return 0, metadataCheckStatus{SkipReason: reason}, nil
		}
		return 0, metadataCheckStatus{}, err
	}
	return len(resp.Data), metadataCheckStatus{Verified: true}, nil
}

func fetchSubscriptionPromotionalOfferCount(ctx context.Context, client *asc.Client, subscriptionID string) (int, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionPromotionalOffers(reqCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionPromotionalOffersLimit(1))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription promotional offers"); ok {
			return 0, metadataCheckStatus{SkipReason: reason}, nil
		}
		return 0, metadataCheckStatus{}, err
	}
	return len(resp.Data), metadataCheckStatus{Verified: true}, nil
}

func fetchSubscriptionWinBackOfferCount(ctx context.Context, client *asc.Client, subscriptionID string) (int, metadataCheckStatus, error) {
	reqCtx, cancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionWinBackOffers(reqCtx, strings.TrimSpace(subscriptionID), asc.WithWinBackOffersLimit(1))
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription win-back offers"); ok {
			return 0, metadataCheckStatus{SkipReason: reason}, nil
		}
		return 0, metadataCheckStatus{}, err
	}
	return len(resp.Data), metadataCheckStatus{Verified: true}, nil
}

func subscriptionHasImage(ctx context.Context, client *asc.Client, subscriptionID string) (subscriptionImageStatus, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetSubscriptionImages(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionImagesLimit(1))
	if err != nil {
		if asc.IsNotFound(err) {
			return subscriptionImageStatus{Verified: true}, nil
		}
		if errors.Is(err, context.Canceled) {
			return subscriptionImageStatus{}, err
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return subscriptionImageStatus{
				Verified:   false,
				SkipReason: "Image verification was skipped because the App Store Connect image endpoint timed out",
			}, nil
		}
		if errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err) {
			return subscriptionImageStatus{
				Verified:   false,
				SkipReason: "Image verification was skipped because this App Store Connect account cannot read subscription image assets",
			}, nil
		}
		if asc.IsRetryable(err) {
			return subscriptionImageStatus{
				Verified:   false,
				SkipReason: "Image verification was skipped because the App Store Connect image endpoint was temporarily unavailable or rate limited",
			}, nil
		}
		var netErr net.Error
		if errors.As(err, &netErr) {
			return subscriptionImageStatus{
				Verified:   false,
				SkipReason: "Image verification was skipped because the App Store Connect image endpoint could not be reached",
			}, nil
		}
		return subscriptionImageStatus{}, err
	}

	return subscriptionImageStatus{
		HasImage: resp != nil && len(resp.Data) > 0,
		Verified: true,
	}, nil
}

func metadataCheckSkipReason(err error, resourceLabel string) (string, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint timed out", resourceLabel), true
	}
	if errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err) {
		return fmt.Sprintf("Validation skipped %s because this App Store Connect account cannot read them", resourceLabel), true
	}
	if asc.IsRetryable(err) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint was temporarily unavailable or rate limited", resourceLabel), true
	}
	if asc.IsNotFound(err) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint returned not found", resourceLabel), true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint could not be reached", resourceLabel), true
	}
	return "", false
}
