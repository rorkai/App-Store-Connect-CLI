package validate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateOptions struct {
	AppID     string
	Version   string
	VersionID string
	Platform  string
	Strict    bool
	Output    string
	Pretty    bool
}

var clientFactory = shared.GetASCClient

// ValidateCommand returns the asc validate command.
func ValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	versionID := fs.String("version-id", "", "App Store version ID")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc validate --app \"APP_ID\" (--version-id \"VERSION_ID\" | --version \"VERSION\") [flags]",
		ShortHelp:  "Validate App Store version readiness before submission.",
		LongHelp: `Validate pre-submission readiness for an App Store version.

Checks:
  - Metadata length limits
  - Required fields and localizations
  - App Store review details completeness
  - Primary category configured
  - Build attached and processed
  - Pricing schedule and territory availability
  - Screenshot presence and size compatibility
  - Age rating completeness

Examples:
  asc validate --app "APP_ID" --version-id "VERSION_ID"
  asc validate --app "APP_ID" --version "1.0.0" --platform IOS
  asc validate --app "APP_ID" --version-id "VERSION_ID" --platform IOS --output table
  asc validate --app "APP_ID" --version-id "VERSION_ID" --strict

TestFlight:
  asc validate testflight --app "APP_ID" --build "BUILD_ID"

In-App Purchases:
  asc validate iap --app "APP_ID"

Subscriptions:
  asc validate subscriptions --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ValidateTestFlightCommand(),
			ValidateIAPCommand(),
			ValidateSubscriptionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q\n\n", args[0])
				return flag.ErrHelp
			}
			trimmedVersion := strings.TrimSpace(*version)
			trimmedVersionID := strings.TrimSpace(*versionID)
			if trimmedVersion == "" && trimmedVersionID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version or --version-id is required")
				return flag.ErrHelp
			}
			if trimmedVersion != "" && trimmedVersionID != "" {
				return shared.UsageError("--version and --version-id are mutually exclusive")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			var normalizedPlatform string
			if strings.TrimSpace(*platform) != "" {
				value, err := shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return fmt.Errorf("validate: %w", err)
				}
				normalizedPlatform = value
			}

			return runValidate(ctx, validateOptions{
				AppID:     resolvedAppID,
				Version:   trimmedVersion,
				VersionID: trimmedVersionID,
				Platform:  normalizedPlatform,
				Strict:    *strict,
				Output:    *output.Output,
				Pretty:    *output.Pretty,
			})
		},
	}
}

func runValidate(ctx context.Context, opts validateOptions) error {
	client, err := clientFactory()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resolvedVersionID := opts.VersionID
	if resolvedVersionID == "" {
		resolvedVersionID, err = resolveVersionID(requestCtx, client, opts.AppID, opts.Version, opts.Platform)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}
	}

	versionResp, err := client.GetAppStoreVersion(requestCtx, resolvedVersionID)
	if err != nil {
		return fmt.Errorf("validate: failed to fetch app store version: %w", err)
	}

	appResp, err := client.GetApp(requestCtx, opts.AppID)
	if err != nil {
		return fmt.Errorf("validate: failed to fetch app: %w", err)
	}

	versionLocsResp, err := client.GetAppStoreVersionLocalizations(requestCtx, resolvedVersionID)
	if err != nil {
		return fmt.Errorf("validate: failed to fetch version localizations: %w", err)
	}

	appInfosResp, err := client.GetAppInfos(requestCtx, opts.AppID)
	if err != nil {
		return fmt.Errorf("validate: failed to fetch app info: %w", err)
	}

	appInfoID := shared.SelectBestAppInfoID(appInfosResp)
	if strings.TrimSpace(appInfoID) == "" {
		return fmt.Errorf("validate: failed to select app info for app")
	}

	appInfoLocsResp, err := client.GetAppInfoLocalizations(requestCtx, appInfoID)
	if err != nil {
		return fmt.Errorf("validate: failed to fetch app info localizations: %w", err)
	}

	primaryCategoryID := ""
	primaryCategoryResp, err := client.GetAppInfoPrimaryCategoryRelationship(requestCtx, appInfoID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate: failed to fetch app primary category: %w", err)
		}
	} else {
		primaryCategoryID = primaryCategoryResp.Data.ID
	}

	var ageRatingDecl *validation.AgeRatingDeclaration
	ageRatingResp, err := client.GetAgeRatingDeclarationForAppStoreVersion(requestCtx, resolvedVersionID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate: failed to fetch age rating declaration: %w", err)
		}
	} else {
		ageRatingDecl = mapAgeRatingDeclaration(ageRatingResp.Data.Attributes)
	}

	var reviewDetails *validation.ReviewDetails
	reviewDetailsResp, err := client.GetAppStoreReviewDetailForVersion(requestCtx, resolvedVersionID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate: failed to fetch review details: %w", err)
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
	buildResp, err := client.GetAppStoreVersionBuild(requestCtx, resolvedVersionID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate: failed to fetch attached build: %w", err)
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

	priceScheduleID := ""
	priceScheduleResp, err := client.GetAppPriceSchedule(requestCtx, opts.AppID)
	if err != nil {
		if !asc.IsNotFound(err) {
			return fmt.Errorf("validate: failed to fetch app price schedule: %w", err)
		}
	} else {
		priceScheduleID = priceScheduleResp.Data.ID
	}

	availabilityID := ""
	availableTerritories := 0
	availabilityResp, err := client.GetAppAvailabilityV2(requestCtx, opts.AppID)
	if err != nil {
		// ASC can report missing app availability with non-404 errors
		// (e.g. "resource does not exist"). Treat those as "missing" rather than
		// aborting validation.
		if !shared.IsAppAvailabilityMissing(err) {
			return fmt.Errorf("validate: failed to fetch app availability: %w", err)
		}
	} else {
		availabilityID = availabilityResp.Data.ID
		if strings.TrimSpace(availabilityID) != "" {
			nextURL := ""
			for {
				var territoryResp *asc.TerritoryAvailabilitiesResponse
				if strings.TrimSpace(nextURL) != "" {
					territoryResp, err = client.GetTerritoryAvailabilities(requestCtx, availabilityID, asc.WithTerritoryAvailabilitiesNextURL(nextURL))
				} else {
					territoryResp, err = client.GetTerritoryAvailabilities(requestCtx, availabilityID, asc.WithTerritoryAvailabilitiesLimit(200))
				}
				if err != nil {
					return fmt.Errorf("validate: failed to fetch territory availabilities: %w", err)
				}

				for _, territoryAvailability := range territoryResp.Data {
					if territoryAvailability.Attributes.Available {
						availableTerritories++
					}
				}

				if availableTerritories > 0 {
					break
				}

				nextURL = strings.TrimSpace(territoryResp.Links.Next)
				if nextURL == "" {
					break
				}
			}
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
			ID:               loc.ID,
			Locale:           attrs.Locale,
			Name:             attrs.Name,
			Subtitle:         attrs.Subtitle,
			PrivacyPolicyURL: attrs.PrivacyPolicyURL,
		})
	}

	screenshotSets, err := fetchScreenshotSets(requestCtx, client, versionLocsResp.Data)
	if err != nil {
		return err
	}

	platform := opts.Platform
	if platform == "" {
		platform = string(versionResp.Data.Attributes.Platform)
	}

	report := validation.Validate(validation.Input{
		AppID:                opts.AppID,
		AppInfoID:            appInfoID,
		VersionID:            resolvedVersionID,
		VersionString:        versionResp.Data.Attributes.VersionString,
		VersionState:         shared.ResolveAppStoreVersionState(versionResp.Data.Attributes),
		Platform:             platform,
		PrimaryLocale:        appResp.Data.Attributes.PrimaryLocale,
		VersionLocalizations: versionLocalizations,
		AppInfoLocalizations: appInfoLocalizations,
		ReviewDetails:        reviewDetails,
		PrimaryCategoryID:    primaryCategoryID,
		Build:                attachedBuild,
		PriceScheduleID:      priceScheduleID,
		AvailabilityID:       availabilityID,
		AvailableTerritories: availableTerritories,
		ScreenshotSets:       screenshotSets,
		AgeRatingDeclaration: ageRatingDecl,
	}, opts.Strict)

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewReportedError(fmt.Errorf("validate: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}

func resolveVersionID(ctx context.Context, client *asc.Client, appID, version, platform string) (string, error) {
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsVersionStrings([]string{version}),
		asc.WithAppStoreVersionsLimit(20),
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}

	resp, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to resolve app store version: %w", err)
	}
	if resp == nil || len(resp.Data) == 0 {
		if strings.TrimSpace(platform) != "" {
			return "", fmt.Errorf("app store version not found for version %q and platform %q", version, platform)
		}
		return "", fmt.Errorf("app store version not found for version %q", version)
	}
	if len(resp.Data) > 1 {
		if strings.TrimSpace(platform) != "" {
			return "", fmt.Errorf("multiple app store versions found for version %q and platform %q (use --version-id)", version, platform)
		}
		return "", fmt.Errorf("multiple app store versions found for version %q (use --platform or --version-id)", version)
	}
	return resp.Data[0].ID, nil
}

func fetchScreenshotSets(ctx context.Context, client *asc.Client, localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]validation.ScreenshotSet, error) {
	var sets []validation.ScreenshotSet
	for _, loc := range localizations {
		resp, err := client.GetAppStoreVersionLocalizationScreenshotSets(ctx, loc.ID)
		if err != nil {
			return nil, fmt.Errorf("validate: failed to fetch screenshot sets for %s: %w", loc.ID, err)
		}
		for _, set := range resp.Data {
			screenshotsResp, err := client.GetAppScreenshots(ctx, set.ID)
			if err != nil {
				return nil, fmt.Errorf("validate: failed to fetch screenshots for %s: %w", set.ID, err)
			}
			screenshots := make([]validation.Screenshot, 0, len(screenshotsResp.Data))
			for _, shot := range screenshotsResp.Data {
				width := 0
				height := 0
				if shot.Attributes.ImageAsset != nil {
					width = shot.Attributes.ImageAsset.Width
					height = shot.Attributes.ImageAsset.Height
				}
				screenshots = append(screenshots, validation.Screenshot{
					ID:       shot.ID,
					FileName: shot.Attributes.FileName,
					Width:    width,
					Height:   height,
				})
			}
			sets = append(sets, validation.ScreenshotSet{
				ID:             set.ID,
				DisplayType:    set.Attributes.ScreenshotDisplayType,
				Locale:         loc.Attributes.Locale,
				LocalizationID: loc.ID,
				Screenshots:    screenshots,
			})
		}
	}
	return sets, nil
}

func mapAgeRatingDeclaration(attrs asc.AgeRatingDeclarationAttributes) *validation.AgeRatingDeclaration {
	return &validation.AgeRatingDeclaration{
		Advertising:                                 attrs.Advertising,
		Gambling:                                    attrs.Gambling,
		HealthOrWellnessTopics:                      attrs.HealthOrWellnessTopics,
		LootBox:                                     attrs.LootBox,
		MessagingAndChat:                            attrs.MessagingAndChat,
		ParentalControls:                            attrs.ParentalControls,
		AgeAssurance:                                attrs.AgeAssurance,
		UnrestrictedWebAccess:                       attrs.UnrestrictedWebAccess,
		UserGeneratedContent:                        attrs.UserGeneratedContent,
		AlcoholTobaccoOrDrugUseOrReferences:         attrs.AlcoholTobaccoOrDrugUseOrReferences,
		Contests:                                    attrs.Contests,
		GamblingSimulated:                           attrs.GamblingSimulated,
		GunsOrOtherWeapons:                          attrs.GunsOrOtherWeapons,
		MedicalOrTreatmentInformation:               attrs.MedicalOrTreatmentInformation,
		ProfanityOrCrudeHumor:                       attrs.ProfanityOrCrudeHumor,
		SexualContentGraphicAndNudity:               attrs.SexualContentGraphicAndNudity,
		SexualContentOrNudity:                       attrs.SexualContentOrNudity,
		HorrorOrFearThemes:                          attrs.HorrorOrFearThemes,
		MatureOrSuggestiveThemes:                    attrs.MatureOrSuggestiveThemes,
		ViolenceCartoonOrFantasy:                    attrs.ViolenceCartoonOrFantasy,
		ViolenceRealistic:                           attrs.ViolenceRealistic,
		ViolenceRealisticProlongedGraphicOrSadistic: attrs.ViolenceRealisticProlongedGraphicOrSadistic,
		KidsAgeBand:                                 attrs.KidsAgeBand,
		AgeRatingOverride:                           attrs.AgeRatingOverride,
		AgeRatingOverrideV2:                         attrs.AgeRatingOverrideV2,
		KoreaAgeRatingOverride:                      attrs.KoreaAgeRatingOverride,
		DeveloperAgeRatingInfoURL:                   attrs.DeveloperAgeRatingInfoURL,
	}
}
