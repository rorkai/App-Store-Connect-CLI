package validate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// ReadinessOptions defines inputs for App Store version readiness validation.
type ReadinessOptions struct {
	AppID     string
	Version   string
	VersionID string
	Platform  string
	Strict    bool
	Build     *validation.Build
}

// BuildReadinessReport fetches live App Store Connect data and returns a
// structured readiness report without printing output.
func BuildReadinessReport(ctx context.Context, opts ReadinessOptions) (validation.Report, error) {
	client, err := clientFactory()
	if err != nil {
		return validation.Report{}, err
	}
	ctx = withReadinessRequestGate(ctx)

	resolvedVersionID := strings.TrimSpace(opts.VersionID)
	if resolvedVersionID == "" {
		resolvedVersionID, err = doReadinessRequest(ctx, func(requestCtx context.Context) (string, error) {
			return resolveVersionID(requestCtx, client, strings.TrimSpace(opts.AppID), strings.TrimSpace(opts.Version), strings.TrimSpace(opts.Platform))
		})
		if err != nil {
			return validation.Report{}, err
		}
	}

	var versionData versionReadinessData
	var appInfoData appInfoReadinessData
	if err := runReadinessTasks(
		ctx,
		func(taskCtx context.Context) error {
			var fetchErr error
			versionData, fetchErr = fetchVersionReadinessData(taskCtx, client, resolvedVersionID, opts.Build)
			return fetchErr
		},
		func(taskCtx context.Context) error {
			var fetchErr error
			appInfoData, fetchErr = fetchAppInfoReadinessData(taskCtx, client, strings.TrimSpace(opts.AppID))
			return fetchErr
		},
	); err != nil {
		return validation.Report{}, err
	}

	var contentRightsDeclaration *string
	if appInfoData.app.Attributes.ContentRightsDeclaration != nil {
		value := strings.TrimSpace(string(*appInfoData.app.Attributes.ContentRightsDeclaration))
		contentRightsDeclaration = &value
	}

	attachedBuild := versionData.build
	priceScheduleID := ""
	pricingFetchSkipReason := ""
	availabilityID := ""
	appAvailableTerritories := []string(nil)
	availableTerritories := 0
	availabilityFetchSkipReason := ""
	pricingTerritories := []string(nil)
	pricingCoverageSkipReason := ""
	var screenshotSets []validation.ScreenshotSet
	var subscriptions []validation.Subscription
	subscriptionFetchSkipReason := ""
	var iaps []validation.IAP
	iapFetchSkipReason := ""

	if err := runReadinessTasks(
		ctx,
		func(taskCtx context.Context) error {
			return resolveMultipleAppInfoAgeRating(
				taskCtx,
				client,
				&appInfoData,
				opts.AppID,
				shared.ResolveAppStoreVersionState(versionData.response.Data.Attributes),
			)
		},
		func(taskCtx context.Context) error {
			return populateBuildEncryptionDeclaration(taskCtx, client, attachedBuild)
		},
		func(taskCtx context.Context) error {
			priceScheduleResp, fetchErr := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AppPriceScheduleResponse, error) {
				return client.GetAppPriceSchedule(requestCtx, opts.AppID)
			})
			if fetchErr != nil {
				if asc.IsNotFound(fetchErr) {
					return nil
				}
				if reason, ok := readinessPricingSkipReason(fetchErr); ok {
					pricingFetchSkipReason = reason
					return nil
				}
				return fmt.Errorf("failed to fetch app price schedule: %w", fetchErr)
			}
			priceScheduleID = priceScheduleResp.Data.ID
			return nil
		},
		func(taskCtx context.Context) error {
			var fetchErr error
			availabilityID, appAvailableTerritories, availableTerritories, fetchErr = fetchAvailableTerritoryDetailsFn(taskCtx, client, opts.AppID)
			if fetchErr == nil {
				return nil
			}
			if reason, ok := readinessAvailabilitySkipReason(fetchErr); ok {
				availabilityFetchSkipReason = reason
				availabilityID = ""
				appAvailableTerritories = nil
				availableTerritories = 0
				return nil
			}
			return fetchErr
		},
		func(taskCtx context.Context) error {
			territories, fetchErr := fetchPricingTerritoriesFn(taskCtx, client)
			if fetchErr != nil {
				if reason, ok := pricingTerritoryCheckSkipReason(fetchErr); ok {
					pricingCoverageSkipReason = reason
					return nil
				}
				return fetchErr
			}
			pricingTerritories = territories
			return nil
		},
		func(taskCtx context.Context) error {
			var fetchErr error
			screenshotSets, fetchErr = fetchScreenshotSetsFn(taskCtx, client, versionData.localizations)
			return fetchErr
		},
		func(taskCtx context.Context) error {
			fetchedSubscriptions, fetchErr := fetchSubscriptionsFn(taskCtx, client, opts.AppID)
			if fetchErr != nil {
				switch {
				case errors.Is(fetchErr, asc.ErrForbidden) || asc.IsUnauthorized(fetchErr):
					subscriptionFetchSkipReason = "Subscription readiness checks were skipped because this App Store Connect account cannot read subscription resources"
				case asc.IsRetryable(fetchErr):
					subscriptionFetchSkipReason = "Subscription readiness checks were skipped because the App Store Connect subscription endpoints were temporarily unavailable or rate limited"
				default:
					return fmt.Errorf("failed to fetch subscriptions: %w", fetchErr)
				}
				return nil
			}
			subscriptions = fetchedSubscriptions
			return nil
		},
		func(taskCtx context.Context) error {
			fetchedIAPs, fetchErr := fetchIAPsFn(taskCtx, client, opts.AppID)
			if fetchErr != nil {
				switch {
				case errors.Is(fetchErr, asc.ErrForbidden) || asc.IsUnauthorized(fetchErr):
					iapFetchSkipReason = "IAP readiness checks were skipped because this App Store Connect account cannot read in-app purchase resources"
				case asc.IsRetryable(fetchErr):
					iapFetchSkipReason = "IAP readiness checks were skipped because the App Store Connect IAP endpoints were temporarily unavailable or rate limited"
				default:
					return fmt.Errorf("failed to fetch in-app purchases: %w", fetchErr)
				}
				return nil
			}
			iaps = fetchedIAPs
			return nil
		},
	); err != nil {
		return validation.Report{}, err
	}

	versionLocalizations := make([]validation.VersionLocalization, 0, len(versionData.localizations))
	for _, loc := range versionData.localizations {
		attrs := loc.Attributes
		versionLocalizations = append(versionLocalizations, validation.VersionLocalization{
			ID:              loc.ID,
			Locale:          attrs.Locale,
			Description:     attrs.Description,
			Keywords:        attrs.Keywords,
			WhatsNew:        attrs.WhatsNew,
			PromotionalText: attrs.PromotionalText,
			SupportURL:      attrs.SupportURL,
			MarketingURL:    attrs.MarketingURL,
		})
	}
	sort.SliceStable(versionLocalizations, func(i, j int) bool {
		if versionLocalizations[i].Locale != versionLocalizations[j].Locale {
			return versionLocalizations[i].Locale < versionLocalizations[j].Locale
		}
		return versionLocalizations[i].ID < versionLocalizations[j].ID
	})

	appInfoLocalizations := make([]validation.AppInfoLocalization, 0, len(appInfoData.localizations))
	for _, loc := range appInfoData.localizations {
		attrs := loc.Attributes
		appInfoLocalizations = append(appInfoLocalizations, validation.AppInfoLocalization{
			ID:                loc.ID,
			Locale:            attrs.Locale,
			Name:              attrs.Name,
			Subtitle:          attrs.Subtitle,
			PrivacyPolicyURL:  attrs.PrivacyPolicyURL,
			PrivacyChoicesURL: attrs.PrivacyChoicesURL,
		})
	}
	sort.SliceStable(appInfoLocalizations, func(i, j int) bool {
		if appInfoLocalizations[i].Locale != appInfoLocalizations[j].Locale {
			return appInfoLocalizations[i].Locale < appInfoLocalizations[j].Locale
		}
		return appInfoLocalizations[i].ID < appInfoLocalizations[j].ID
	})

	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		platform = string(versionData.response.Data.Attributes.Platform)
	}

	report := validation.Validate(validation.Input{
		AppID:                       opts.AppID,
		AppInfoID:                   appInfoData.appInfoID,
		VersionID:                   resolvedVersionID,
		VersionString:               versionData.response.Data.Attributes.VersionString,
		VersionState:                shared.ResolveAppStoreVersionState(versionData.response.Data.Attributes),
		Platform:                    platform,
		PrimaryLocale:               appInfoData.app.Attributes.PrimaryLocale,
		VersionLocalizations:        versionLocalizations,
		AppInfoLocalizations:        appInfoLocalizations,
		ReviewDetails:               versionData.reviewDetails,
		PrimaryCategoryID:           appInfoData.primaryCategoryID,
		ContentRightsDeclaration:    contentRightsDeclaration,
		Build:                       attachedBuild,
		PriceScheduleID:             priceScheduleID,
		PricingFetchSkipReason:      pricingFetchSkipReason,
		AvailabilityID:              availabilityID,
		AvailableTerritories:        availableTerritories,
		AppAvailableTerritories:     appAvailableTerritories,
		PricingTerritories:          pricingTerritories,
		PricingTerritoryCount:       len(pricingTerritories),
		AvailabilityFetchSkipReason: availabilityFetchSkipReason,
		PricingCoverageSkipReason:   pricingCoverageSkipReason,
		ScreenshotSets:              screenshotSets,
		Subscriptions:               subscriptions,
		SubscriptionFetchSkipReason: subscriptionFetchSkipReason,
		IAPs:                        iaps,
		IAPFetchSkipReason:          iapFetchSkipReason,
		AgeRatingDeclaration:        appInfoData.ageRatingDeclaration,
		ReleaseType:                 versionData.response.Data.Attributes.ReleaseType,
		EarliestReleaseDate:         versionData.response.Data.Attributes.EarliestReleaseDate,
		Copyright:                   versionData.response.Data.Attributes.Copyright,
	}, opts.Strict)

	return report, nil
}

func populateBuildEncryptionDeclaration(ctx context.Context, client *asc.Client, build *validation.Build) error {
	if build == nil || client == nil {
		return nil
	}
	if build.UsesNonExemptEncryption == nil || !*build.UsesNonExemptEncryption {
		return nil
	}
	if strings.TrimSpace(build.AppEncryptionDeclarationID) != "" || strings.TrimSpace(build.AppEncryptionDeclarationState) != "" {
		return nil
	}

	declarationResp, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppEncryptionDeclarationResponse, error) {
		return client.GetBuildAppEncryptionDeclaration(requestCtx, strings.TrimSpace(build.ID))
	})
	if err != nil {
		if asc.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to fetch build encryption declaration: %w", err)
	}

	build.AppEncryptionDeclarationID = strings.TrimSpace(declarationResp.Data.ID)
	build.AppEncryptionDeclarationState = strings.TrimSpace(string(declarationResp.Data.Attributes.AppEncryptionDeclarationState))
	return nil
}

func readinessPricingSkipReason(err error) (string, bool) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Review app pricing in App Store Connect; readiness could not verify it automatically because the Pricing and Availability endpoints timed out", true
	case errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err):
		return "Review app pricing in App Store Connect; readiness could not verify it automatically because this account cannot read Pricing and Availability", true
	case asc.IsRetryable(err):
		return "Review app pricing in App Store Connect; readiness could not verify it automatically because the Pricing and Availability endpoints were temporarily unavailable or rate limited", true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return "Review app pricing in App Store Connect; readiness could not verify it automatically because the Pricing and Availability endpoints could not be reached", true
	}
	return "", false
}

func readinessAvailabilitySkipReason(err error) (string, bool) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Review app availability in App Store Connect; readiness could not verify it automatically because the Pricing and Availability endpoints timed out", true
	case errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err):
		return "Review app availability in App Store Connect; readiness could not verify it automatically because this account cannot read Pricing and Availability", true
	case asc.IsRetryable(err):
		return "Review app availability in App Store Connect; readiness could not verify it automatically because the Pricing and Availability endpoints were temporarily unavailable or rate limited", true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return "Review app availability in App Store Connect; readiness could not verify it automatically because the Pricing and Availability endpoints could not be reached", true
	}
	return "", false
}
