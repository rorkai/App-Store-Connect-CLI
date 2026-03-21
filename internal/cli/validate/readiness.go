package validate

import (
	"context"
	"errors"
	"fmt"
	"net"
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

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	refreshRequestCtx := func() {
		if cancel != nil {
			cancel()
		}
		requestCtx, cancel = shared.ContextWithTimeout(ctx)
	}

	resolvedVersionID := strings.TrimSpace(opts.VersionID)
	if resolvedVersionID == "" {
		resolvedVersionID, err = resolveVersionID(requestCtx, client, strings.TrimSpace(opts.AppID), strings.TrimSpace(opts.Version), strings.TrimSpace(opts.Platform))
		if err != nil {
			return validation.Report{}, err
		}
	}

	versionResp, err := client.GetAppStoreVersion(requestCtx, resolvedVersionID)
	if err != nil {
		return validation.Report{}, fmt.Errorf("failed to fetch app store version: %w", err)
	}

	appResp, err := client.GetApp(requestCtx, opts.AppID)
	if err != nil {
		return validation.Report{}, fmt.Errorf("failed to fetch app: %w", err)
	}

	versionLocsResp, err := client.GetAppStoreVersionLocalizations(requestCtx, resolvedVersionID)
	if err != nil {
		return validation.Report{}, fmt.Errorf("failed to fetch version localizations: %w", err)
	}

	appInfosResp, err := client.GetAppInfos(requestCtx, opts.AppID)
	if err != nil {
		return validation.Report{}, fmt.Errorf("failed to fetch app info: %w", err)
	}

	appInfoID := shared.SelectBestAppInfoID(appInfosResp)
	if strings.TrimSpace(appInfoID) == "" {
		return validation.Report{}, fmt.Errorf("failed to select app info for app")
	}

	appInfoLocsResp, err := client.GetAppInfoLocalizations(requestCtx, appInfoID)
	if err != nil {
		return validation.Report{}, fmt.Errorf("failed to fetch app info localizations: %w", err)
	}

	primaryCategoryID := ""
	primaryCategoryResp, err := client.GetAppInfoPrimaryCategoryRelationship(requestCtx, appInfoID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return validation.Report{}, fmt.Errorf("failed to fetch app primary category: %w", err)
		}
	} else {
		primaryCategoryID = primaryCategoryResp.Data.ID
	}

	var ageRatingDecl *validation.AgeRatingDeclaration
	ageRatingResp, err := client.GetAgeRatingDeclarationForAppStoreVersion(requestCtx, resolvedVersionID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return validation.Report{}, fmt.Errorf("failed to fetch age rating declaration: %w", err)
		}
	} else {
		ageRatingDecl = mapAgeRatingDeclaration(ageRatingResp.Data.Attributes)
	}

	var reviewDetails *validation.ReviewDetails
	reviewDetailsResp, err := client.GetAppStoreReviewDetailForVersion(requestCtx, resolvedVersionID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return validation.Report{}, fmt.Errorf("failed to fetch review details: %w", err)
		}
	} else {
		attrs := reviewDetailsResp.Data.Attributes
		reviewDetails = &validation.ReviewDetails{
			ID:                  reviewDetailsResp.Data.ID,
			ContactFirstName:    attrs.ContactFirstName,
			ContactLastName:     attrs.ContactLastName,
			ContactEmail:        attrs.ContactEmail,
			ContactPhone:        attrs.ContactPhone,
			DemoAccountName:     attrs.DemoAccountName,
			DemoAccountPassword: attrs.DemoAccountPassword,
			DemoAccountRequired: attrs.DemoAccountRequired,
			Notes:               attrs.Notes,
		}
	}

	var attachedBuild *validation.Build
	if opts.Build != nil {
		attachedBuild = &validation.Build{
			ID:              strings.TrimSpace(opts.Build.ID),
			Version:         opts.Build.Version,
			ProcessingState: opts.Build.ProcessingState,
			Expired:         opts.Build.Expired,
		}
	} else {
		buildResp, err := client.GetAppStoreVersionBuild(requestCtx, resolvedVersionID)
		if err != nil {
			if !asc.IsNotFound(err) {
				return validation.Report{}, fmt.Errorf("failed to fetch attached build: %w", err)
			}
		} else if strings.TrimSpace(buildResp.Data.ID) != "" {
			attrs := buildResp.Data.Attributes
			attachedBuild = &validation.Build{
				ID:              buildResp.Data.ID,
				Version:         attrs.Version,
				ProcessingState: attrs.ProcessingState,
				Expired:         attrs.Expired,
			}
		}
	}

	priceScheduleID := ""
	pricingFetchSkipReason := ""
	priceScheduleResp, err := client.GetAppPriceSchedule(requestCtx, opts.AppID)
	if err != nil {
		if asc.IsNotFound(err) {
			// Leave priceScheduleID empty so validation reports a missing schedule.
		} else if reason, ok := readinessPricingSkipReason(err); ok {
			pricingFetchSkipReason = reason
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				refreshRequestCtx()
			}
		} else {
			return validation.Report{}, fmt.Errorf("failed to fetch app price schedule: %w", err)
		}
	} else {
		priceScheduleID = priceScheduleResp.Data.ID
	}

	availabilityID := ""
	availableTerritories := 0
	availabilityFetchSkipReason := ""
	pricingCoverageSkipReason := ""
	availabilityID, availableTerritories, err = fetchAvailableTerritoriesFn(requestCtx, client, opts.AppID)
	if err != nil {
		if reason, ok := readinessAvailabilitySkipReason(err); ok {
			availabilityFetchSkipReason = reason
			availabilityID = ""
			availableTerritories = 0
			if coverageReason, coverageOK := availabilityCheckSkipReason(err); coverageOK {
				pricingCoverageSkipReason = coverageReason
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				refreshRequestCtx()
			}
		} else {
			return validation.Report{}, err
		}
	}

	versionLocalizations := make([]validation.VersionLocalization, 0, len(versionLocsResp.Data))
	for _, loc := range versionLocsResp.Data {
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

	appInfoLocalizations := make([]validation.AppInfoLocalization, 0, len(appInfoLocsResp.Data))
	for _, loc := range appInfoLocsResp.Data {
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

	screenshotSets, err := fetchScreenshotSetsFn(requestCtx, client, versionLocsResp.Data)
	if err != nil {
		return validation.Report{}, err
	}

	subscriptions := make([]validation.Subscription, 0)
	subscriptionFetchSkipReason := ""
	fetchedSubscriptions, err := fetchSubscriptionsFn(ctx, client, opts.AppID)
	if err != nil {
		switch {
		case errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err):
			subscriptionFetchSkipReason = "Subscription readiness checks were skipped because this App Store Connect account cannot read subscription resources"
		case asc.IsRetryable(err):
			subscriptionFetchSkipReason = "Subscription readiness checks were skipped because the App Store Connect subscription endpoints were temporarily unavailable or rate limited"
		default:
			return validation.Report{}, fmt.Errorf("failed to fetch subscriptions: %w", err)
		}
	} else {
		subscriptions = fetchedSubscriptions
	}

	iaps := make([]validation.IAP, 0)
	iapFetchSkipReason := ""
	fetchedIAPs, err := fetchIAPsFn(ctx, client, opts.AppID)
	if err != nil {
		switch {
		case errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err):
			iapFetchSkipReason = "IAP readiness checks were skipped because this App Store Connect account cannot read in-app purchase resources"
		case asc.IsRetryable(err):
			iapFetchSkipReason = "IAP readiness checks were skipped because the App Store Connect IAP endpoints were temporarily unavailable or rate limited"
		default:
			return validation.Report{}, fmt.Errorf("failed to fetch in-app purchases: %w", err)
		}
	} else {
		iaps = fetchedIAPs
	}

	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		platform = string(versionResp.Data.Attributes.Platform)
	}

	report := validation.Validate(validation.Input{
		AppID:                       opts.AppID,
		AppInfoID:                   appInfoID,
		VersionID:                   resolvedVersionID,
		VersionString:               versionResp.Data.Attributes.VersionString,
		VersionState:                shared.ResolveAppStoreVersionState(versionResp.Data.Attributes),
		Platform:                    platform,
		PrimaryLocale:               appResp.Data.Attributes.PrimaryLocale,
		VersionLocalizations:        versionLocalizations,
		AppInfoLocalizations:        appInfoLocalizations,
		ReviewDetails:               reviewDetails,
		PrimaryCategoryID:           primaryCategoryID,
		Build:                       attachedBuild,
		PriceScheduleID:             priceScheduleID,
		PricingFetchSkipReason:      pricingFetchSkipReason,
		AvailabilityID:              availabilityID,
		AvailableTerritories:        availableTerritories,
		AvailabilityFetchSkipReason: availabilityFetchSkipReason,
		PricingCoverageSkipReason:   pricingCoverageSkipReason,
		ScreenshotSets:              screenshotSets,
		Subscriptions:               subscriptions,
		SubscriptionFetchSkipReason: subscriptionFetchSkipReason,
		IAPs:                        iaps,
		IAPFetchSkipReason:          iapFetchSkipReason,
		AgeRatingDeclaration:        ageRatingDecl,
		ReleaseType:                 versionResp.Data.Attributes.ReleaseType,
		EarliestReleaseDate:         versionResp.Data.Attributes.EarliestReleaseDate,
		Copyright:                   versionResp.Data.Attributes.Copyright,
	}, opts.Strict)

	return report, nil
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
