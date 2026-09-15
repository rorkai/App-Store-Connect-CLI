package validate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
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
	Deep      bool
	CheckURLs bool
	AppleID   string
	Output    string
	Pretty    bool
}

var (
	clientFactory          = shared.GetASCClient
	fetchScreenshotSetsFn  = fetchScreenshotSets
	buildReadinessReportFn = BuildReadinessReport
)

// ValidateCommand returns the asc validate command.
func ValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	deep := fs.Bool("deep", false, "Verify blockers that require a cached Apple web session")
	checkURLs := fs.Bool("check-urls", false, "Check metadata URL destinations with bounded public HTTP requests")
	appleID := fs.String("apple-id", "", "Cached Apple web session to use with --deep")
	output := shared.BindOutputFlags(fs)

	testFlight := wrapValidateSubcommand(ValidateTestFlightCommand(), fs)
	iap := wrapValidateSubcommand(ValidateIAPCommand(), fs)
	subscriptions := wrapValidateSubcommand(ValidateSubscriptionsCommand(), fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc validate --app \"APP_ID\" (--version-id \"VERSION_ID\" | --version \"VERSION\") [flags]",
		ShortHelp:  "Canonical App Store submission readiness report.",
		LongHelp: `Validate pre-submission readiness for an App Store version.

This is the canonical command for App Store submission readiness.
Use it instead of ` + "`asc submit preflight`" + `.

The default validate response includes an ordered remediation plan, so the first step is the next thing to fix.

Placeholder checks preserve shorter Lorem Ipsum product wording and ordinary
localized TODO copy without marker punctuation; only template-like residue is
reported.

Checks:
  - Metadata length limits
  - Placeholder copy in localized listing fields (warning; --strict to block)
  - Deterministic metadata content and keyword hygiene warnings
  - Optional bounded checks for public metadata URL destinations (--check-urls)
  - Required fields and localizations
  - App Store review details completeness
  - Primary category configured
  - Build attached and processed
  - Build encryption declaration readiness
  - App content rights declaration
  - Pricing schedule and territory availability
  - Screenshot presence and size compatibility
  - Subscription review readiness and promotional image guidance
  - Age rating completeness

Deep validation:
  --deep adds read-only checks from an existing cached Apple web session for
  App Privacy publication, required agreements, and first-of-type subscription
  attachment. It also classifies every actionable finding as api-fixable,
  web-fixable, or manual and returns exact available commands and App Store
  Connect links. Deep validation never starts an interactive login.

Examples:
  asc validate --app "APP_ID" --version-id "VERSION_ID"
  asc validate --app "APP_ID" --version "1.0.0" --platform IOS
  asc validate --app "APP_ID" --version-id "VERSION_ID" --platform IOS --output table
  asc validate --app "APP_ID" --version-id "VERSION_ID" --strict
  asc validate --app "APP_ID" --version-id "VERSION_ID" --deep
  asc validate --app "APP_ID" --version-id "VERSION_ID" --check-urls
  asc validate --app "APP_ID" --version "1.0.0" --deep --apple-id "user@example.com"

TestFlight:
  asc validate testflight --app "APP_ID" --build-id "BUILD_ID"

In-App Purchases:
  asc validate iap --app "APP_ID"

Subscriptions:
  asc validate subscriptions --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			testFlight,
			iap,
			subscriptions,
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q\n\n", args[0])
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "")
			}
			trimmedVersion := strings.TrimSpace(*version)
			trimmedVersionID := strings.TrimSpace(*versionID)
			if trimmedVersion == "" && trimmedVersionID == "" {
				return shared.WithDiagnostic(shared.UsageError("--version or --version-id is required"), shared.DiagnosticRequiredInputMissing, "")
			}
			if trimmedVersion != "" && trimmedVersionID != "" {
				return shared.WithDiagnostic(shared.UsageError("--version and --version-id are mutually exclusive"), shared.DiagnosticConflictingInput, "--version-id")
			}
			trimmedAppleID := strings.TrimSpace(*appleID)
			if trimmedAppleID != "" && !*deep {
				return shared.WithDiagnostic(shared.UsageError("--apple-id requires --deep"), shared.DiagnosticInvalidInput, "--apple-id")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.WithDiagnostic(shared.UsageError("--app is required (or set ASC_APP_ID)"), shared.DiagnosticRequiredInputMissing, "--app")
			}

			var normalizedPlatform string
			if strings.TrimSpace(*platform) != "" {
				value, err := shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return shared.WithDiagnostic(fmt.Errorf("validate: %w", err), shared.DiagnosticInvalidInput, "--platform")
				}
				normalizedPlatform = value
			}

			return runValidate(ctx, validateOptions{
				AppID:     resolvedAppID,
				Version:   trimmedVersion,
				VersionID: trimmedVersionID,
				Platform:  normalizedPlatform,
				Strict:    *strict,
				Deep:      *deep,
				CheckURLs: *checkURLs,
				AppleID:   trimmedAppleID,
				Output:    *output.Output,
				Pretty:    *output.Pretty,
			})
		},
	}
}

func wrapValidateSubcommand(cmd *ffcli.Command, parentFlags *flag.FlagSet) *ffcli.Command {
	if cmd == nil || cmd.Exec == nil {
		return cmd
	}

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		if message := validateParentFlagUsageMessage(parentFlags); message != "" {
			return shared.WithDiagnostic(shared.UsageError(message), shared.DiagnosticInvalidInput, "")
		}
		return originalExec(ctx, args)
	}
	return cmd
}

func validateParentFlagUsageMessage(parentFlags *flag.FlagSet) string {
	if parentFlags == nil {
		return ""
	}

	moveAfterSubcommand := make([]string, 0, 4)
	topLevelOnly := make([]string, 0, 8)
	parentFlags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "app", "output", "pretty", "strict":
			moveAfterSubcommand = append(moveAfterSubcommand, "--"+f.Name)
		case "version", "version-id", "platform", "deep", "check-urls", "apple-id":
			topLevelOnly = append(topLevelOnly, "--"+f.Name)
		}
	})

	if len(moveAfterSubcommand) == 0 && len(topLevelOnly) == 0 {
		return ""
	}

	parts := make([]string, 0, 2)
	if len(moveAfterSubcommand) > 0 {
		parts = append(parts, fmt.Sprintf("%s must be passed after the validate subcommand name", formatValidateFlagList(moveAfterSubcommand)))
	}
	if len(topLevelOnly) > 0 {
		parts = append(parts, fmt.Sprintf("%s %s only valid for asc validate", formatValidateFlagList(topLevelOnly), validateFlagVerb(topLevelOnly)))
	}

	return strings.Join(parts, "; ")
}

func formatValidateFlagList(flags []string) string {
	switch len(flags) {
	case 0:
		return ""
	case 1:
		return flags[0]
	case 2:
		return flags[0] + " and " + flags[1]
	default:
		return strings.Join(flags[:len(flags)-1], ", ") + ", and " + flags[len(flags)-1]
	}
}

func validateFlagVerb(flags []string) string {
	if len(flags) == 1 {
		return "is"
	}
	return "are"
}

func runValidate(ctx context.Context, opts validateOptions) error {
	report, err := buildReadinessReportFn(ctx, ReadinessOptions{
		AppID:     opts.AppID,
		Version:   opts.Version,
		VersionID: opts.VersionID,
		Platform:  opts.Platform,
		Strict:    opts.Strict,
		Deep:      opts.Deep,
		CheckURLs: opts.CheckURLs,
	})
	if err != nil {
		if !opts.Deep || !asc.IsRequiredAgreementError(err) {
			return fmt.Errorf("validate: %w", err)
		}
		report = requiredAgreementFallbackReport(opts)
	}
	if opts.Deep {
		deep, findings, deepErr := collectDeepValidation(ctx, report, opts.AppleID)
		if deepErr != nil {
			return fmt.Errorf("validate: deep validation: %w", deepErr)
		}
		report = validation.ApplyDeepValidation(report, deep, findings)
	}

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewValidationReportedError(fmt.Errorf("validate: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}

func requiredAgreementFallbackReport(opts validateOptions) validation.Report {
	message := "App Store Connect API access is blocked by a missing or expired required agreement"
	publicUnavailable := "Retry after the Account Holder resolves the required agreement and App Store Connect API access returns"
	checks := []validation.CheckResult{
		{
			ID:           "agreements.public_api.blocked",
			Severity:     validation.SeverityError,
			Message:      message,
			Remediation:  "Have the Account Holder resolve the required agreement in App Store Connect",
			ResourceType: "agreement",
			Resolution: &validation.Resolution{
				Fixability:         validation.FixabilityManual,
				AppStoreConnectURL: agreementsAppStoreConnectURL,
			},
		},
		{
			ID:           "availability.unverified",
			Severity:     validation.SeverityWarning,
			Message:      "App availability could not be verified because required agreements block public API access",
			Remediation:  publicUnavailable,
			ResourceType: "app",
			ResourceID:   strings.TrimSpace(opts.AppID),
		},
		{
			ID:           "review_details.unverified",
			Severity:     validation.SeverityWarning,
			Message:      "Required App Review fields could not be verified because required agreements block public API access",
			Remediation:  publicUnavailable,
			ResourceType: "appStoreVersion",
			ResourceID:   strings.TrimSpace(opts.VersionID),
		},
	}
	return validation.Report{
		AppID:         strings.TrimSpace(opts.AppID),
		VersionID:     strings.TrimSpace(opts.VersionID),
		VersionString: strings.TrimSpace(opts.Version),
		Platform:      strings.TrimSpace(opts.Platform),
		Summary:       validation.SummarizeChecks(checks, opts.Strict),
		Remediation:   validation.BuildRemediation(checks, opts.Strict),
		Checks:        checks,
		Strict:        opts.Strict,
	}
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
	ctx = withReadinessRequestGate(ctx)
	setsByLocalization := make([][]asc.Resource[asc.AppScreenshotSetAttributes], len(localizations))
	setTasks := make([]readinessTask, 0, len(localizations))
	for index := range localizations {
		index := index
		setTasks = append(setTasks, func(taskCtx context.Context) error {
			localization := localizations[index]
			response, err := fetchAllScreenshotSetsForValidation(taskCtx, client, localization.ID)
			if err != nil {
				return fmt.Errorf("validate: failed to fetch screenshot sets for %s: %w", localization.ID, err)
			}
			setsByLocalization[index] = response.Data
			return nil
		})
	}
	if err := runReadinessTasks(ctx, setTasks...); err != nil {
		return nil, err
	}

	type screenshotSetRef struct {
		localization asc.Resource[asc.AppStoreVersionLocalizationAttributes]
		set          asc.Resource[asc.AppScreenshotSetAttributes]
	}
	setRefs := make([]screenshotSetRef, 0)
	for localizationIndex, localizationSets := range setsByLocalization {
		for _, set := range localizationSets {
			setRefs = append(setRefs, screenshotSetRef{
				localization: localizations[localizationIndex],
				set:          set,
			})
		}
	}

	screenshotsBySet := make([][]validation.Screenshot, len(setRefs))
	screenshotTasks := make([]readinessTask, 0, len(setRefs))
	for index := range setRefs {
		index := index
		screenshotTasks = append(screenshotTasks, func(taskCtx context.Context) error {
			set := setRefs[index].set
			response, err := fetchAllScreenshotsForValidation(taskCtx, client, set.ID)
			if err != nil {
				return fmt.Errorf("validate: failed to fetch screenshots for %s: %w", set.ID, err)
			}

			screenshots := make([]validation.Screenshot, 0, len(response.Data))
			for _, shot := range response.Data {
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
			sort.SliceStable(screenshots, func(i, j int) bool {
				if screenshots[i].FileName != screenshots[j].FileName {
					return screenshots[i].FileName < screenshots[j].FileName
				}
				return screenshots[i].ID < screenshots[j].ID
			})
			screenshotsBySet[index] = screenshots
			return nil
		})
	}
	if err := runReadinessTasks(ctx, screenshotTasks...); err != nil {
		return nil, err
	}

	sets := make([]validation.ScreenshotSet, 0, len(setRefs))
	for index, ref := range setRefs {
		sets = append(sets, validation.ScreenshotSet{
			ID:             ref.set.ID,
			DisplayType:    ref.set.Attributes.ScreenshotDisplayType,
			Locale:         ref.localization.Attributes.Locale,
			LocalizationID: ref.localization.ID,
			Screenshots:    screenshotsBySet[index],
		})
	}
	sort.SliceStable(sets, func(i, j int) bool {
		if sets[i].Locale != sets[j].Locale {
			return sets[i].Locale < sets[j].Locale
		}
		if sets[i].LocalizationID != sets[j].LocalizationID {
			return sets[i].LocalizationID < sets[j].LocalizationID
		}
		if sets[i].DisplayType != sets[j].DisplayType {
			return sets[i].DisplayType < sets[j].DisplayType
		}
		return sets[i].ID < sets[j].ID
	})
	return sets, nil
}

func fetchAllScreenshotSetsForValidation(ctx context.Context, client *asc.Client, localizationID string) (*asc.AppScreenshotSetsResponse, error) {
	firstPage, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppScreenshotSetsResponse, error) {
		return client.GetAppStoreVersionLocalizationScreenshotSets(requestCtx, localizationID, asc.WithAppStoreVersionLocalizationScreenshotSetsLimit(200))
	})
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetAppStoreVersionLocalizationScreenshotSets(requestCtx, "", asc.WithAppStoreVersionLocalizationScreenshotSetsNextURL(nextURL))
		})
	})
	if err != nil {
		return nil, err
	}

	response, ok := paginated.(*asc.AppScreenshotSetsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected screenshot sets pagination response type %T", paginated)
	}
	return response, nil
}

func fetchAllScreenshotsForValidation(ctx context.Context, client *asc.Client, setID string) (*asc.AppScreenshotsResponse, error) {
	firstPage, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppScreenshotsResponse, error) {
		return client.GetAppScreenshots(requestCtx, setID, asc.WithAppScreenshotsLimit(200))
	})
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetAppScreenshots(requestCtx, "", asc.WithAppScreenshotsNextURL(nextURL))
		})
	})
	if err != nil {
		return nil, err
	}

	response, ok := paginated.(*asc.AppScreenshotsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected screenshots pagination response type %T", paginated)
	}
	return response, nil
}

func mapAgeRatingDeclaration(attrs asc.AgeRatingDeclarationAttributes) *validation.AgeRatingDeclaration {
	return &validation.AgeRatingDeclaration{
		Advertising:                         attrs.Advertising,
		Gambling:                            attrs.Gambling,
		HealthOrWellnessTopics:              attrs.HealthOrWellnessTopics,
		LootBox:                             attrs.LootBox,
		MessagingAndChat:                    attrs.MessagingAndChat,
		ParentalControls:                    attrs.ParentalControls,
		AgeAssurance:                        attrs.AgeAssurance,
		SocialMedia:                         nullableAgeRatingBool(attrs.SocialMedia),
		SocialMediaAgeRestricted:            nullableAgeRatingBool(attrs.SocialMediaAgeRestricted),
		UnrestrictedWebAccess:               attrs.UnrestrictedWebAccess,
		UserGeneratedContent:                attrs.UserGeneratedContent,
		AlcoholTobaccoOrDrugUseOrReferences: attrs.AlcoholTobaccoOrDrugUseOrReferences,
		Contests:                            attrs.Contests,
		GamblingSimulated:                   attrs.GamblingSimulated,
		GunsOrOtherWeapons:                  attrs.GunsOrOtherWeapons,
		MedicalOrTreatmentInformation:       attrs.MedicalOrTreatmentInformation,
		ProfanityOrCrudeHumor:               attrs.ProfanityOrCrudeHumor,
		SexualContentGraphicAndNudity:       attrs.SexualContentGraphicAndNudity,
		SexualContentOrNudity:               attrs.SexualContentOrNudity,
		HorrorOrFearThemes:                  attrs.HorrorOrFearThemes,
		MatureOrSuggestiveThemes:            attrs.MatureOrSuggestiveThemes,
		ViolenceCartoonOrFantasy:            attrs.ViolenceCartoonOrFantasy,
		ViolenceRealistic:                   attrs.ViolenceRealistic,
		ViolenceRealisticProlongedGraphicOrSadistic: attrs.ViolenceRealisticProlongedGraphicOrSadistic,
		KidsAgeBand:               attrs.KidsAgeBand,
		AgeRatingOverride:         attrs.AgeRatingOverride,
		AgeRatingOverrideV2:       attrs.AgeRatingOverrideV2,
		KoreaAgeRatingOverride:    attrs.KoreaAgeRatingOverride,
		DeveloperAgeRatingInfoURL: attrs.DeveloperAgeRatingInfoURL,
	}
}

func nullableAgeRatingBool(value *asc.NullableBool) *bool {
	if value == nil {
		return nil
	}
	return value.Value
}
