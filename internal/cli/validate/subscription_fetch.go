package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
	ctx = withReadinessRequestGate(ctx)
	groupsResp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionGroupsResponse, error) {
		return client.GetSubscriptionGroups(requestCtx, appID, asc.WithSubscriptionGroupsLimit(200))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription groups: %w", err)
	}

	paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroups(requestCtx, appID, asc.WithSubscriptionGroupsNextURL(nextURL))
		})
	})
	if err != nil {
		return nil, fmt.Errorf("paginate subscription groups: %w", err)
	}

	groups, ok := paginatedGroups.(*asc.SubscriptionGroupsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected subscription groups response type %T", paginatedGroups)
	}

	type groupSubscriptions struct {
		id            string
		name          string
		subscriptions []asc.Resource[asc.SubscriptionAttributes]
	}
	groupResults := make([]groupSubscriptions, len(groups.Data))
	groupTasks := make([]readinessTask, 0, len(groups.Data))
	for index, group := range groups.Data {
		groupID := strings.TrimSpace(group.ID)
		groupResults[index] = groupSubscriptions{id: groupID, name: strings.TrimSpace(group.Attributes.ReferenceName)}
		if groupID == "" {
			continue
		}
		index := index
		groupTasks = append(groupTasks, func(taskCtx context.Context) error {
			subscriptions, fetchErr := fetchSubscriptionsForGroup(taskCtx, client, groupID)
			if fetchErr != nil {
				return fetchErr
			}
			groupResults[index].subscriptions = subscriptions
			return nil
		})
	}
	if err := runReadinessTasks(ctx, groupTasks...); err != nil {
		return nil, err
	}

	type subscriptionRef struct {
		groupIndex int
		groupID    string
		groupName  string
		resource   asc.Resource[asc.SubscriptionAttributes]
	}
	refs := make([]subscriptionRef, 0)
	for groupIndex, group := range groupResults {
		for _, subscription := range group.subscriptions {
			refs = append(refs, subscriptionRef{
				groupIndex: groupIndex,
				groupID:    group.id,
				groupName:  group.name,
				resource:   subscription,
			})
		}
	}

	enrichments := make([]subscriptionEnrichment, len(refs))
	groupMetadata := make([]subscriptionGroupMetadata, len(groupResults))
	groupMetadataScheduled := make([]bool, len(groupResults))
	metadataTasks := make([]readinessTask, 0, len(refs)*3)
	for index := range refs {
		index := index
		ref := refs[index]
		subscriptionID := strings.TrimSpace(ref.resource.ID)
		metadataTasks = append(metadataTasks, func(taskCtx context.Context) error {
			status, fetchErr := subscriptionHasImage(taskCtx, client, subscriptionID)
			if fetchErr != nil {
				return fmt.Errorf("fetch subscription images for %s: %w", subscriptionID, fetchErr)
			}
			enrichments[index].image = status
			return nil
		})

		state := strings.ToUpper(strings.TrimSpace(ref.resource.Attributes.State))
		if state == "REMOVED_FROM_SALE" || state == "DEVELOPER_REMOVED_FROM_SALE" {
			continue
		}
		metadataTasks = append(metadataTasks, func(taskCtx context.Context) error {
			territories, status, fetchErr := fetchSubscriptionPriceTerritories(taskCtx, client, subscriptionID)
			if fetchErr != nil {
				return fmt.Errorf("fetch subscription prices for %s: %w", subscriptionID, fetchErr)
			}
			enrichments[index].priceTerritories = territories
			enrichments[index].priceStatus = status
			return nil
		})

		if state != "MISSING_METADATA" {
			continue
		}
		if !groupMetadataScheduled[ref.groupIndex] {
			groupMetadataScheduled[ref.groupIndex] = true
			groupIndex := ref.groupIndex
			groupID := ref.groupID
			metadataTasks = append(metadataTasks, func(taskCtx context.Context) error {
				localizations, status, fetchErr := fetchGroupLocalizations(taskCtx, client, groupID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription group localizations for group %s: %w", groupID, fetchErr)
				}
				groupMetadata[groupIndex] = subscriptionGroupMetadata{localizations: localizations, status: status}
				return nil
			})
		}

		metadataTasks = append(
			metadataTasks,
			func(taskCtx context.Context) error {
				localizations, status, fetchErr := fetchSubscriptionLocalizations(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription localizations for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].localizations = localizations
				enrichments[index].localizationStatus = status
				return nil
			},
			func(taskCtx context.Context) error {
				id, status, fetchErr := fetchSubscriptionReviewScreenshot(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription review screenshot for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].reviewScreenshotID = id
				enrichments[index].reviewScreenshotStatus = status
				return nil
			},
			func(taskCtx context.Context) error {
				id, territories, availableInNew, status, fetchErr := fetchSubscriptionAvailabilityTerritories(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription availability for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].availabilityID = id
				enrichments[index].availabilityTerritories = territories
				enrichments[index].availabilityInNewTerritories = availableInNew
				enrichments[index].availabilityStatus = status
				return nil
			},
			func(taskCtx context.Context) error {
				plans, status, fetchErr := fetchSubscriptionPlanAvailabilities(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription plan availabilities for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].planAvailabilities = plans
				enrichments[index].planAvailabilityStatus = status
				return nil
			},
			func(taskCtx context.Context) error {
				count, status, fetchErr := fetchSubscriptionIntroductoryOfferCount(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription introductory offers for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].introductoryOfferCount = count
				enrichments[index].introductoryOfferStatus = status
				return nil
			},
			func(taskCtx context.Context) error {
				count, status, fetchErr := fetchSubscriptionPromotionalOfferCount(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription promotional offers for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].promotionalOfferCount = count
				enrichments[index].promotionalOfferStatus = status
				return nil
			},
			func(taskCtx context.Context) error {
				count, status, fetchErr := fetchSubscriptionWinBackOfferCount(taskCtx, client, subscriptionID)
				if fetchErr != nil {
					return fmt.Errorf("fetch subscription win-back offers for %s: %w", subscriptionID, fetchErr)
				}
				enrichments[index].winBackOfferCount = count
				enrichments[index].winBackOfferStatus = status
				return nil
			},
		)
	}
	if err := runReadinessTasks(ctx, metadataTasks...); err != nil {
		return nil, err
	}

	subscriptions := make([]validation.Subscription, 0, len(refs))
	for index, ref := range refs {
		attrs := ref.resource.Attributes
		enrichment := enrichments[index]
		valSub := validation.Subscription{
			ID:                   ref.resource.ID,
			Name:                 attrs.Name,
			ProductID:            attrs.ProductID,
			State:                attrs.State,
			GroupID:              ref.groupID,
			GroupName:            ref.groupName,
			HasImage:             enrichment.image.HasImage,
			ImageCheckSkipped:    !enrichment.image.Verified,
			ImageCheckSkipReason: enrichment.image.SkipReason,
			SubscriptionPeriod:   attrs.SubscriptionPeriod,
		}

		state := strings.ToUpper(strings.TrimSpace(attrs.State))
		if state != "REMOVED_FROM_SALE" && state != "DEVELOPER_REMOVED_FROM_SALE" {
			valSub.PriceTerritories = enrichment.priceTerritories
			valSub.PriceCount = len(enrichment.priceTerritories)
			valSub.PriceCheckSkipped = !enrichment.priceStatus.Verified
			valSub.PriceCheckSkipReason = enrichment.priceStatus.SkipReason

			if state == "MISSING_METADATA" {
				group := groupMetadata[ref.groupIndex]
				valSub.GroupLocalizations = group.localizations
				valSub.GroupLocalizationCheckSkipped = !group.status.Verified
				valSub.GroupLocalizationCheckReason = group.status.SkipReason
				valSub.Localizations = enrichment.localizations
				valSub.LocalizationCheckSkipped = !enrichment.localizationStatus.Verified
				valSub.LocalizationCheckSkipReason = enrichment.localizationStatus.SkipReason
				valSub.ReviewScreenshotID = enrichment.reviewScreenshotID
				valSub.ReviewScreenshotCheckSkipped = !enrichment.reviewScreenshotStatus.Verified
				valSub.ReviewScreenshotCheckReason = enrichment.reviewScreenshotStatus.SkipReason
				valSub.AvailabilityID = enrichment.availabilityID
				valSub.AvailabilityTerritories = enrichment.availabilityTerritories
				valSub.AvailabilityInNewTerritories = enrichment.availabilityInNewTerritories
				valSub.AvailabilityCheckSkipped = !enrichment.availabilityStatus.Verified
				valSub.AvailabilityCheckSkipReason = enrichment.availabilityStatus.SkipReason
				valSub.PlanAvailabilities = enrichment.planAvailabilities
				valSub.PlanAvailabilityCheckSkipped = !enrichment.planAvailabilityStatus.Verified
				valSub.PlanAvailabilityCheckReason = enrichment.planAvailabilityStatus.SkipReason
				valSub.IntroductoryOfferCount = enrichment.introductoryOfferCount
				valSub.IntroductoryOfferCheckSkipped = !enrichment.introductoryOfferStatus.Verified
				valSub.IntroductoryOfferCheckReason = enrichment.introductoryOfferStatus.SkipReason
				valSub.PromotionalOfferCount = enrichment.promotionalOfferCount
				valSub.PromotionalOfferCheckSkipped = !enrichment.promotionalOfferStatus.Verified
				valSub.PromotionalOfferCheckReason = enrichment.promotionalOfferStatus.SkipReason
				valSub.WinBackOfferCount = enrichment.winBackOfferCount
				valSub.WinBackOfferCheckSkipped = !enrichment.winBackOfferStatus.Verified
				valSub.WinBackOfferCheckReason = enrichment.winBackOfferStatus.SkipReason
			}
		}
		subscriptions = append(subscriptions, valSub)
	}
	sort.SliceStable(subscriptions, func(i, j int) bool {
		if subscriptions[i].GroupID != subscriptions[j].GroupID {
			return subscriptions[i].GroupID < subscriptions[j].GroupID
		}
		if subscriptions[i].ProductID != subscriptions[j].ProductID {
			return subscriptions[i].ProductID < subscriptions[j].ProductID
		}
		return subscriptions[i].ID < subscriptions[j].ID
	})

	return subscriptions, nil
}

type subscriptionGroupMetadata struct {
	localizations []validation.SubscriptionGroupLocalizationInfo
	status        metadataCheckStatus
}

type subscriptionEnrichment struct {
	image                        subscriptionImageStatus
	priceTerritories             []string
	priceStatus                  metadataCheckStatus
	localizations                []validation.SubscriptionLocalizationInfo
	localizationStatus           metadataCheckStatus
	reviewScreenshotID           string
	reviewScreenshotStatus       metadataCheckStatus
	availabilityID               string
	availabilityTerritories      []string
	availabilityInNewTerritories *bool
	availabilityStatus           metadataCheckStatus
	planAvailabilities           []validation.SubscriptionPlanAvailabilityInfo
	planAvailabilityStatus       metadataCheckStatus
	introductoryOfferCount       int
	introductoryOfferStatus      metadataCheckStatus
	promotionalOfferCount        int
	promotionalOfferStatus       metadataCheckStatus
	winBackOfferCount            int
	winBackOfferStatus           metadataCheckStatus
}

func fetchSubscriptionsForGroup(ctx context.Context, client *asc.Client, groupID string) ([]asc.Resource[asc.SubscriptionAttributes], error) {
	response, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionsResponse, error) {
		return client.GetSubscriptions(requestCtx, groupID, asc.WithSubscriptionsLimit(200))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscriptions for group %s: %w", groupID, err)
	}

	paginated, err := asc.PaginateAll(ctx, response, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetSubscriptions(requestCtx, groupID, asc.WithSubscriptionsNextURL(nextURL))
		})
	})
	if err != nil {
		return nil, fmt.Errorf("paginate subscriptions: %w", err)
	}
	typed, ok := paginated.(*asc.SubscriptionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected subscriptions response type %T", paginated)
	}
	return typed.Data, nil
}

func fetchGroupLocalizations(ctx context.Context, client *asc.Client, groupID string) ([]validation.SubscriptionGroupLocalizationInfo, metadataCheckStatus, error) {
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionGroupLocalizationsResponse, error) {
		return client.GetSubscriptionGroupLocalizations(requestCtx, strings.TrimSpace(groupID), asc.WithSubscriptionGroupLocalizationsLimit(200))
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

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroupLocalizations(requestCtx, strings.TrimSpace(groupID), asc.WithSubscriptionGroupLocalizationsNextURL(nextURL))
		})
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
	sort.SliceStable(locs, func(i, j int) bool {
		if locs[i].Locale != locs[j].Locale {
			return locs[i].Locale < locs[j].Locale
		}
		return locs[i].Name < locs[j].Name
	})
	return locs, metadataCheckStatus{Verified: true}, nil
}

// fetchSubscriptionLocalizations fetches localization info for a subscription.
func fetchSubscriptionLocalizations(ctx context.Context, client *asc.Client, subscriptionID string) ([]validation.SubscriptionLocalizationInfo, metadataCheckStatus, error) {
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionLocalizationsResponse, error) {
		return client.GetSubscriptionLocalizations(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionLocalizationsLimit(200))
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

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionLocalizations(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionLocalizationsNextURL(nextURL))
		})
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
	sort.SliceStable(locs, func(i, j int) bool {
		if locs[i].Locale != locs[j].Locale {
			return locs[i].Locale < locs[j].Locale
		}
		if locs[i].Name != locs[j].Name {
			return locs[i].Name < locs[j].Name
		}
		return locs[i].Description < locs[j].Description
	})
	return locs, metadataCheckStatus{Verified: true}, nil
}

// fetchSubscriptionPriceTerritories returns the unique territories with prices
// configured for a subscription. It paginates all price resources so scheduled
// price changes for the same territory don't inflate coverage.
func fetchSubscriptionPriceTerritories(ctx context.Context, client *asc.Client, subscriptionID string) ([]string, metadataCheckStatus, error) {
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricesResponse, error) {
		return client.GetSubscriptionPrices(
			requestCtx,
			strings.TrimSpace(subscriptionID),
			asc.WithSubscriptionPricesInclude([]string{"territory"}),
			asc.WithSubscriptionPricesLimit(200),
		)
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

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionPrices(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionPricesNextURL(nextURL))
		})
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
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
		return client.GetSubscriptionAppStoreReviewScreenshotForSubscription(requestCtx, strings.TrimSpace(subscriptionID))
	})
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

func fetchSubscriptionAvailabilityTerritories(ctx context.Context, client *asc.Client, subscriptionID string) (string, []string, *bool, metadataCheckStatus, error) {
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionAvailabilityResponse, error) {
		return client.GetSubscriptionAvailabilityForSubscription(requestCtx, strings.TrimSpace(subscriptionID))
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", nil, nil, metadataCheckStatus{}, err
		}
		if asc.IsNotFound(err) {
			return "", nil, nil, metadataCheckStatus{Verified: true}, nil
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription availability"); ok {
			return "", nil, nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return "", nil, nil, metadataCheckStatus{}, err
	}

	availabilityID := strings.TrimSpace(resp.Data.ID)
	if availabilityID == "" {
		return "", nil, nil, metadataCheckStatus{Verified: true}, nil
	}
	availableInNew := resp.Data.Attributes.AvailableInNewTerritories

	allTerritories := make([]string, 0)
	nextURL := ""
	for {
		territoryResp, requestErr := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.TerritoriesResponse, error) {
			if strings.TrimSpace(nextURL) != "" {
				return client.GetSubscriptionAvailabilityAvailableTerritories(requestCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesNextURL(nextURL))
			}
			return client.GetSubscriptionAvailabilityAvailableTerritories(requestCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
		})
		err = requestErr
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return "", nil, nil, metadataCheckStatus{}, err
			}
			if reason, ok := metadataCheckSkipReason(err, "subscription availability territories"); ok {
				return "", nil, nil, metadataCheckStatus{SkipReason: reason}, nil
			}
			return "", nil, nil, metadataCheckStatus{}, err
		}

		for _, territory := range territoryResp.Data {
			allTerritories = append(allTerritories, strings.ToUpper(strings.TrimSpace(territory.ID)))
		}

		nextURL = strings.TrimSpace(territoryResp.Links.Next)
		if nextURL == "" {
			break
		}
	}

	return availabilityID, validation.SortedUniqueNonEmptyStrings(allTerritories), &availableInNew, metadataCheckStatus{Verified: true}, nil
}

func fetchSubscriptionPlanAvailabilities(ctx context.Context, client *asc.Client, subscriptionID string) ([]validation.SubscriptionPlanAvailabilityInfo, metadataCheckStatus, error) {
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionPlanAvailabilitiesResponse, error) {
		return client.GetSubscriptionPlanAvailabilitiesForSubscription(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionPlanAvailabilitiesLimit(200))
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription plan availabilities"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{}, err
	}
	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionPlanAvailabilitiesForSubscription(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionPlanAvailabilitiesNextURL(nextURL))
		})
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, metadataCheckStatus{}, err
		}
		if reason, ok := metadataCheckSkipReason(err, "subscription plan availabilities"); ok {
			return nil, metadataCheckStatus{SkipReason: reason}, nil
		}
		return nil, metadataCheckStatus{}, err
	}
	typed, ok := paginated.(*asc.SubscriptionPlanAvailabilitiesResponse)
	if !ok {
		return nil, metadataCheckStatus{}, fmt.Errorf("unexpected subscription plan availabilities response type %T", paginated)
	}

	plans := make([]validation.SubscriptionPlanAvailabilityInfo, 0, len(typed.Data))
	for _, plan := range typed.Data {
		planID := strings.TrimSpace(plan.ID)
		territoryResp, fetchErr := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.LinkagesResponse, error) {
			return client.GetSubscriptionPlanAvailabilityAvailableTerritoriesRelationships(requestCtx, planID, asc.WithLinkagesLimit(200))
		})
		if fetchErr != nil {
			if errors.Is(fetchErr, context.Canceled) {
				return nil, metadataCheckStatus{}, fetchErr
			}
			if reason, ok := metadataCheckSkipReason(fetchErr, "subscription plan availability territories"); ok {
				return nil, metadataCheckStatus{SkipReason: reason}, nil
			}
			return nil, metadataCheckStatus{}, fetchErr
		}
		all, fetchErr := asc.PaginateAll(ctx, territoryResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
				return client.GetSubscriptionPlanAvailabilityAvailableTerritoriesRelationships(requestCtx, planID, asc.WithLinkagesNextURL(nextURL))
			})
		})
		if fetchErr != nil {
			if errors.Is(fetchErr, context.Canceled) {
				return nil, metadataCheckStatus{}, fetchErr
			}
			if reason, ok := metadataCheckSkipReason(fetchErr, "subscription plan availability territories"); ok {
				return nil, metadataCheckStatus{SkipReason: reason}, nil
			}
			return nil, metadataCheckStatus{}, fetchErr
		}
		links, ok := all.(*asc.LinkagesResponse)
		if !ok {
			return nil, metadataCheckStatus{}, fmt.Errorf("unexpected subscription plan territory response type %T", all)
		}
		territories := make([]string, 0, len(links.Data))
		for _, territory := range links.Data {
			territories = append(territories, strings.ToUpper(strings.TrimSpace(territory.ID)))
		}
		plans = append(plans, validation.SubscriptionPlanAvailabilityInfo{ID: planID, PlanType: string(plan.Attributes.PlanType), AvailableInNewTerritories: plan.Attributes.AvailableInNewTerritories, Territories: validation.SortedUniqueNonEmptyStrings(territories)})
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].PlanType != plans[j].PlanType {
			return plans[i].PlanType < plans[j].PlanType
		}
		return plans[i].ID < plans[j].ID
	})
	return plans, metadataCheckStatus{Verified: true}, nil
}

func fetchSubscriptionIntroductoryOfferCount(ctx context.Context, client *asc.Client, subscriptionID string) (int, metadataCheckStatus, error) {
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionIntroductoryOffersResponse, error) {
		return client.GetSubscriptionIntroductoryOffers(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionIntroductoryOffersLimit(1))
	})
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
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionPromotionalOffersResponse, error) {
		return client.GetSubscriptionPromotionalOffers(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionPromotionalOffersLimit(1))
	})
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
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.WinBackOffersResponse, error) {
		return client.GetSubscriptionWinBackOffers(requestCtx, strings.TrimSpace(subscriptionID), asc.WithWinBackOffersLimit(1))
	})
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
	resp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.SubscriptionImagesResponse, error) {
		return client.GetSubscriptionImages(requestCtx, strings.TrimSpace(subscriptionID), asc.WithSubscriptionImagesLimit(1))
	})
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
		var netErr net.Error
		if errors.As(err, &netErr) {
			return subscriptionImageStatus{
				Verified:   false,
				SkipReason: "Image verification was skipped because the App Store Connect image endpoint could not be reached",
			}, nil
		}
		if asc.IsRetryable(err) {
			return subscriptionImageStatus{
				Verified:   false,
				SkipReason: "Image verification was skipped because the App Store Connect image endpoint was temporarily unavailable or rate limited",
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
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint could not be reached", resourceLabel), true
	}
	if asc.IsRetryable(err) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint was temporarily unavailable or rate limited", resourceLabel), true
	}
	if asc.IsNotFound(err) {
		return fmt.Sprintf("Validation skipped %s because the App Store Connect endpoint returned not found", resourceLabel), true
	}
	return "", false
}
