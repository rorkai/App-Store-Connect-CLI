package validate

import (
	"context"
	"errors"
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
	IPAPath   string
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
	deep := fs.Bool("deep", false, "Verify IAP attachment through the public API and blockers through a cached Apple web session")
	checkURLs := fs.Bool("check-urls", false, "Check metadata URL destinations with bounded public HTTP requests")
	appleID := fs.String("apple-id", "", "Cached Apple web session to use with --deep")
	ipaPath := fs.String("ipa", "", "Path to the version's .ipa; its UIDeviceFamily decides whether iPad screenshots are required")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc validate --app \"APP_ID\" [--version-id \"VERSION_ID\" | --version \"VERSION\"] [flags]",
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
  - Other-platform and test/debug wording in listing fields (warning; --strict to block)
  - Unfinished-feature wording in listing fields and stale copyright year ranges (info)
  - Deterministic metadata content and keyword hygiene warnings
  - Optional bounded checks for public metadata URL destinations (--check-urls)
  - Required fields and localizations
  - App Store review details completeness
  - Primary category configured
  - Build attached and processed
  - Build encryption declaration readiness
  - App content rights declaration
  - Pricing schedule, base territory price (Free counts), and territory availability
  - Screenshot presence and size compatibility
  - Screenshot, app preview, and asset library placement delivery per locale
  - Required iPad screenshots for builds that run on iPad (--ipa)
  - Subscription review readiness and promotional image guidance
  - Custom product page search keywords missing from this version's keywords
  - Age rating completeness

Deep validation:
  --deep verifies first-of-type consumable, non-consumable, and non-renewing
  IAP attachment to the selected app version through the public API. Missing
  required attachment blocks; incomplete evidence remains unverified. It also
  adds read-only checks from an existing cached Apple web session for
  App Privacy publication, required agreements, and first-of-type subscription
  attachment. It also classifies every actionable finding as api-fixable,
  web-fixable, or manual and returns exact available commands and App Store
  Connect links. Deep validation never starts an interactive login.

iPad screenshots:
  The App Store Connect API does not report whether a build runs on iPad, so
  pass --ipa with the IOS version's .ipa to read UIDeviceFamily locally. When
  it includes iPad (2) and the primary locale has no APP_IPAD_PRO_3GEN_129
  screenshot set, screenshots.required.ipad blocks submission. Without --ipa,
  a version with iPhone screenshots but no primary-locale APP_IPAD_PRO_3GEN_129
  set gets the non-blocking screenshots.required.ipad_unverified info check.
  The IPA's bundle ID must
  match the app's, its CFBundleShortVersionString must match the version, and
  its CFBundleVersion must match the attached build when one is attached.

Default version selection:
  When --version and --version-id are omitted, validate selects the app's
  newest active editable App Store version. If none exists, it checks for a
  DEVELOPER_REMOVED_FROM_SALE version; if that is absent, it falls back to the newest live version.
  Pass --platform when that tier has candidates on more than one platform. The
  selected version is always reported on stderr. A live fallback does not mean the live version is ready for submission;
  its state is evaluated by the same readiness checks and can still block.

Examples:
  asc validate --app "APP_ID"
  asc validate --app "APP_ID" --version-id "VERSION_ID"
  asc validate --app "APP_ID" --version "1.0.0" --platform IOS
  asc validate --app "APP_ID" --version-id "VERSION_ID" --platform IOS --output table
  asc validate --app "APP_ID" --version-id "VERSION_ID" --strict
  asc validate --app "APP_ID" --version-id "VERSION_ID" --deep
  asc validate --app "APP_ID" --version-id "VERSION_ID" --check-urls
  asc validate --app "APP_ID" --version-id "VERSION_ID" --ipa "./App.ipa"
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
			ValidateTestFlightCommand(),
			ValidateIAPCommand(),
			ValidateSubscriptionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Error: unknown subcommand %q\n\n", args[0])
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "")
			}
			trimmedVersion := strings.TrimSpace(*version)
			trimmedVersionID := strings.TrimSpace(*versionID)
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
				IPAPath:   strings.TrimSpace(*ipaPath),
				Output:    *output.Output,
				Pretty:    *output.Pretty,
			})
		},
	}
}

func runValidate(ctx context.Context, opts validateOptions) error {
	var localIPA *LocalIPA
	if opts.IPAPath != "" {
		loaded, loadErr := loadLocalIPA(opts.IPAPath)
		if loadErr != nil {
			return shared.WithDiagnostic(fmt.Errorf("validate: --ipa: %w", loadErr), shared.DiagnosticInvalidInput, "--ipa")
		}
		localIPA = loaded
	}

	var report validation.Report
	var err error
	if strings.TrimSpace(opts.Version) == "" && strings.TrimSpace(opts.VersionID) == "" {
		client, clientErr := clientFactory()
		if clientErr != nil {
			err = clientErr
		} else {
			resolveCtx, cancel := shared.ContextWithTimeout(ctx)
			resolved, resolveErr := shared.ResolveAndAnnounceDefaultAppStoreVersion(resolveCtx, client, opts.AppID, opts.Platform, "--version")
			cancel()
			if resolveErr != nil {
				err = resolveErr
			} else {
				opts.Version = resolved.VersionString
				opts.VersionID = resolved.ID
				opts.Platform = resolved.Platform
			}
		}
	}
	if err == nil {
		report, err = buildReadinessReportFn(ctx, ReadinessOptions{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: opts.VersionID,
			Platform:  opts.Platform,
			Strict:    opts.Strict,
			Deep:      opts.Deep,
			CheckURLs: opts.CheckURLs,
			IPA:       localIPA,
		})
		if ambiguous, ok := errors.AsType[*shared.AmbiguousSelectionError](err); ok {
			message := "validate: " + err.Error()
			fmt.Fprintf(os.Stderr, "Error: %s\n", message)
			reported := shared.WithDiagnostic(shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message), shared.DiagnosticInvalidInput, ambiguous.Flag)
			return shared.NewErrorWithCause(reported, err)
		}
	}
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
	report := validation.Report{
		AppID:         strings.TrimSpace(opts.AppID),
		VersionID:     strings.TrimSpace(opts.VersionID),
		VersionString: strings.TrimSpace(opts.Version),
		Platform:      strings.TrimSpace(opts.Platform),
		Summary:       validation.SummarizeChecks(checks, opts.Strict),
		Remediation:   validation.BuildRemediation(checks, opts.Strict),
		Checks:        checks,
		Strict:        opts.Strict,
	}
	iapCheck, iapFinding := collectIAPAttachmentDeepCheck(context.Background(), nil, report, nil, "First-of-type IAP attachment could not be verified because required agreements block public API access")
	return validation.ApplyDeepValidation(report, validation.DeepReport{Checks: []validation.DeepCheck{iapCheck}}, []validation.CheckResult{*iapFinding})
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
	if resp == nil {
		return "", fmt.Errorf("failed to resolve app store version: empty response")
	}
	pageHasNext := strings.TrimSpace(resp.Links.Next) != ""
	if len(resp.Data) == 0 && !pageHasNext {
		notFound := fmt.Errorf("app store version not found for version %q", version)
		if strings.TrimSpace(platform) != "" {
			notFound = fmt.Errorf("app store version not found for version %q and platform %q", version, platform)
		}
		notFound = shared.WithAppStoreVersionNotFoundDiagnostics(ctx, client, appID, version, platform, notFound)
		return "", shared.WithDiagnostic(shared.NewValidationError(notFound), shared.DiagnosticResourceNotFound, "--version")
	}
	if len(resp.Data) > 1 || pageHasNext {
		ambiguous := shared.AmbiguousAppStoreVersionError(version, platform, resp.Data, "--platform", "--version-id")
		if pageHasNext {
			return "", shared.MarkAmbiguousSelectionSample(ambiguous)
		}
		return "", ambiguous
	}
	return resp.Data[0].ID, nil
}

func fetchScreenshotSets(ctx context.Context, client *asc.Client, localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]validation.ScreenshotSet, error) {
	ctx = withReadinessRequestGate(ctx)
	setsByLocalization := make([][]asc.Resource[asc.AppScreenshotSetAttributes], len(localizations))
	includedByLocalization := make([]map[string][]asc.Resource[asc.AppScreenshotAttributes], len(localizations))
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
			includedByLocalization[index] = asc.IncludedAppScreenshots(response)
			return nil
		})
	}
	if err := runReadinessTasks(ctx, setTasks...); err != nil {
		return nil, err
	}

	type screenshotSetRef struct {
		localization asc.Resource[asc.AppStoreVersionLocalizationAttributes]
		set          asc.Resource[asc.AppScreenshotSetAttributes]
		screenshots  []asc.Resource[asc.AppScreenshotAttributes]
		included     bool
	}
	setRefs := make([]screenshotSetRef, 0)
	for localizationIndex, localizationSets := range setsByLocalization {
		for _, set := range localizationSets {
			screenshots, included := includedByLocalization[localizationIndex][set.ID]
			setRefs = append(setRefs, screenshotSetRef{
				localization: localizations[localizationIndex],
				set:          set,
				screenshots:  screenshots,
				included:     included,
			})
		}
	}

	screenshotsBySet := make([][]validation.Screenshot, len(setRefs))
	screenshotTasks := make([]readinessTask, 0, len(setRefs))
	for index := range setRefs {
		index := index
		screenshotTasks = append(screenshotTasks, func(taskCtx context.Context) error {
			ref := setRefs[index]
			data := ref.screenshots
			if !ref.included {
				response, err := fetchAllScreenshotsForValidation(taskCtx, client, ref.set.ID)
				if err != nil {
					return fmt.Errorf("validate: failed to fetch screenshots for %s: %w", ref.set.ID, err)
				}
				data = response.Data
			}

			screenshots := make([]validation.Screenshot, 0, len(data))
			for _, shot := range data {
				width := 0
				height := 0
				if shot.Attributes.ImageAsset != nil {
					width = shot.Attributes.ImageAsset.Width
					height = shot.Attributes.ImageAsset.Height
				}
				deliveryState, deliveryErrors := screenshotDeliveryState(shot.Attributes.AssetDeliveryState)
				screenshots = append(screenshots, validation.Screenshot{
					ID:             shot.ID,
					FileName:       shot.Attributes.FileName,
					Width:          width,
					Height:         height,
					DeliveryState:  deliveryState,
					DeliveryErrors: deliveryErrors,
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
		return client.GetAppScreenshotSets(requestCtx, localizationID, asc.WithAppScreenshotSetsLimit(200), asc.WithAppScreenshotSetsIncludeScreenshots())
	})
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetAppScreenshotSets(requestCtx, "", asc.WithAppScreenshotSetsNextURL(nextURL))
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
