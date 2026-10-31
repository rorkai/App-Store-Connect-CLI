package validate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/pricing"
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
	Deep      bool
	CheckURLs bool
	Build     *validation.Build
	// IPA is the local binary evidence for iPad support. When nil, a missing
	// iPad screenshot set is reported as informational only.
	IPA *LocalIPA
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
	} else {
		_, err = doReadinessRequest(ctx, func(requestCtx context.Context) (asc.Resource[asc.AppStoreVersionAttributes], error) {
			return shared.ResolveOwnedAppStoreVersionByID(
				requestCtx,
				client,
				strings.TrimSpace(opts.AppID),
				resolvedVersionID,
				strings.TrimSpace(opts.Platform),
			)
		})
		if err != nil {
			return validation.Report{}, fmt.Errorf("failed to verify app store version %q: %w", resolvedVersionID, err)
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
	baseTerritory := ""
	basePriceMissing := false
	hasPaidAppPrice := false
	appPricingKnown := false
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
	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		platform = string(versionData.response.Data.Attributes.Platform)
	}

	var customProductPageLocalizations []validation.CustomProductPageLocalization
	customProductPageKeywordsSkipReason := ""

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
			basePrice, fetchErr := pricing.FetchAppBasePriceStatus(taskCtx, client, priceScheduleID, runReadinessRequest)
			switch {
			case fetchErr != nil:
				if errors.Is(fetchErr, context.Canceled) {
					return fetchErr
				}
				pricingFetchSkipReason = readinessBasePriceSkipReason(fetchErr)
			case !basePrice.Configured:
				// Apple returns a synthetic schedule for apps that never set a
				// price, so only the schedule's prices show it is missing.
				priceScheduleID = ""
				return nil
			default:
				baseTerritory = basePrice.BaseTerritory
				basePriceMissing = !basePrice.HasPrice
			}
			if opts.Deep {
				hasPaidAppPrice, appPricingKnown = fetchCurrentAppPaidPricingEvidence(taskCtx, client, priceScheduleID)
			}
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
			fetchedIAPs, fetchErr := fetchIAPsFn(taskCtx, client, opts.AppID, opts.Deep)
			if fetchErr != nil {
				switch {
				case taskCtx.Err() != nil:
					return taskCtx.Err()
				case opts.Deep:
					iapFetchSkipReason = "The complete IAP catalog could not be verified through the public API"
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
		func(taskCtx context.Context) error {
			// Apple supports custom product pages only on iOS and iPadOS, which
			// share the IOS version platform.
			if !strings.EqualFold(platform, string(asc.PlatformIOS)) {
				customProductPageKeywordsSkipReason = fmt.Sprintf("Custom product page keyword checks were skipped because custom product pages apply only to iOS and iPadOS, and this version's platform is %s", platform)
				return nil
			}
			fetched, fetchErr := fetchCustomProductPageKeywords(taskCtx, client, opts.AppID)
			if fetchErr != nil {
				// The keyword check is advisory, so any failure other than
				// cancellation reads as unverified instead of aborting validation.
				switch {
				case taskCtx.Err() != nil:
					return taskCtx.Err()
				case errors.Is(fetchErr, context.Canceled):
					return fetchErr
				case errors.Is(fetchErr, asc.ErrForbidden) || asc.IsUnauthorized(fetchErr):
					customProductPageKeywordsSkipReason = "Custom product page keyword checks were skipped because this App Store Connect account cannot read custom product pages"
				case asc.IsRetryable(fetchErr):
					customProductPageKeywordsSkipReason = "Custom product page keyword checks were skipped because the App Store Connect custom product page endpoints were temporarily unavailable or rate limited"
				default:
					customProductPageKeywordsSkipReason = fmt.Sprintf("Custom product page keyword checks were skipped because App Store Connect custom product page data could not be read: %v", fetchErr)
				}
				return nil
			}
			customProductPageLocalizations = fetched
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
			PrivacyPolicyText: attrs.PrivacyPolicyText,
		})
	}
	sort.SliceStable(appInfoLocalizations, func(i, j int) bool {
		if appInfoLocalizations[i].Locale != appInfoLocalizations[j].Locale {
			return appInfoLocalizations[i].Locale < appInfoLocalizations[j].Locale
		}
		return appInfoLocalizations[i].ID < appInfoLocalizations[j].ID
	})

	var supportsIPad *bool
	if opts.IPA != nil {
		if err := checkLocalIPAMatchesVersion(
			opts.IPA,
			platform,
			appInfoData.app.Attributes.BundleID,
			versionData.response.Data.Attributes.VersionString,
			attachedBuild,
		); err != nil {
			return validation.Report{}, shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticInvalidInput, "--ipa")
		}
		value := opts.IPA.SupportsIPad()
		supportsIPad = &value
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
		BaseTerritory:               baseTerritory,
		BasePriceMissing:            basePriceMissing,
		AvailabilityID:              availabilityID,
		AvailableTerritories:        availableTerritories,
		AppAvailableTerritories:     appAvailableTerritories,
		PricingTerritories:          pricingTerritories,
		PricingTerritoryCount:       len(pricingTerritories),
		AvailabilityFetchSkipReason: availabilityFetchSkipReason,
		PricingCoverageSkipReason:   pricingCoverageSkipReason,
		ScreenshotSets:              screenshotSets,
		SupportsIPad:                supportsIPad,
		Subscriptions:               subscriptions,
		SubscriptionFetchSkipReason: subscriptionFetchSkipReason,
		IAPs:                        iaps,
		IAPFetchSkipReason:          iapFetchSkipReason,
		AgeRatingDeclaration:        appInfoData.ageRatingDeclaration,
		ReleaseType:                 versionData.response.Data.Attributes.ReleaseType,
		EarliestReleaseDate:         versionData.response.Data.Attributes.EarliestReleaseDate,
		Copyright:                   versionData.response.Data.Attributes.Copyright,
		HasPaidAppPrice:             hasPaidAppPrice,
		AppPricingKnown:             appPricingKnown,

		CustomProductPageLocalizations:      customProductPageLocalizations,
		CustomProductPageKeywordsSkipReason: customProductPageKeywordsSkipReason,
	}, opts.Strict)
	if opts.Deep {
		check, finding := collectIAPAttachmentDeepCheck(ctx, client, report, iaps, iapFetchSkipReason)
		if ctx.Err() != nil {
			return validation.Report{}, ctx.Err()
		}
		findings := []validation.CheckResult{}
		if finding != nil {
			findings = append(findings, *finding)
		}
		report = validation.ApplyDeepValidation(report, validation.DeepReport{Checks: []validation.DeepCheck{check}}, findings)
	}
	if opts.CheckURLs {
		targets := validateURLTargets(versionLocalizations, appInfoLocalizations)
		if len(targets) > 0 {
			urlChecks, checkErr := checkValidateURLs(ctx, newValidateURLChecker(), targets)
			if checkErr != nil {
				return validation.Report{}, checkErr
			}
			report.Checks = append(report.Checks, urlChecks...)
			report.Summary = validation.SummarizeChecks(report.Checks, opts.Strict)
			report.Remediation = validation.BuildRemediation(report.Checks, opts.Strict)
		}
	}

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

// readinessBasePriceSkipReason explains why the base territory price could not
// be verified. Any failure reads as unverified rather than missing so an
// unrelated API problem never reports a false missing price.
func readinessBasePriceSkipReason(err error) string {
	if reason, ok := readinessPricingSkipReason(err); ok {
		return reason
	}
	return "Review app pricing in App Store Connect; readiness could not verify the base territory price automatically because the price schedule response could not be interpreted"
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
