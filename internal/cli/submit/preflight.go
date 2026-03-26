package submit

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// checkResult represents the outcome of a single preflight check.
type checkResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Advisory bool   `json:"advisory,omitempty"`
	Message  string `json:"message,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// preflightResult aggregates all preflight check outcomes.
type preflightResult struct {
	AppID     string        `json:"app_id"`
	Version   string        `json:"version"`
	Platform  string        `json:"platform"`
	Checks    []checkResult `json:"checks"`
	PassCount int           `json:"pass_count"`
	FailCount int           `json:"fail_count"`
}

func defaultSubmitPreflightOutputFormat() string {
	if shared.DefaultOutputFormat() == "json" {
		return "json"
	}
	return "text"
}

// SubmitPreflightCommand returns the "submit preflight" subcommand.
func SubmitPreflightCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit preflight", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", defaultSubmitPreflightOutputFormat(), "Output format: text, json", "text", "json")

	return &ffcli.Command{
		Name:       "preflight",
		ShortUsage: "asc submit preflight [flags]",
		ShortHelp:  "Check submission readiness without submitting.",
		LongHelp: `Check all submission requirements upfront and report issues with fix commands.

Examples:
  asc submit preflight --app "123456789" --version "1.0"
  asc submit preflight --app "123456789" --version "1.0" --platform TV_OS
  asc submit preflight --app "123456789" --version "2.0" --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*version) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return flag.ErrHelp
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "text", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit preflight: %w", err)
			}

			result := runPreflight(ctx, client, resolvedAppID, strings.TrimSpace(*version), normalizedPlatform)

			if normalizedOutput == "text" {
				printPreflightText(os.Stdout, result)
				if result.FailCount > 0 {
					return fmt.Errorf("submit preflight: %d issue(s) found", result.FailCount)
				}
				return nil
			}

			if err := shared.PrintOutput(result, normalizedOutput, *output.Pretty); err != nil {
				return err
			}
			if result.FailCount > 0 {
				return fmt.Errorf("submit preflight: %d issue(s) found", result.FailCount)
			}
			return nil
		},
	}
}

// runPreflight executes all checks and collects results. Individual check
// failures do not short-circuit — every check runs independently.
func runPreflight(ctx context.Context, client *asc.Client, appID, version, platform string) *preflightResult {
	result := &preflightResult{
		AppID:    appID,
		Version:  version,
		Platform: platform,
	}

	// 1. Version exists — resolve version ID
	versionID, versionCheck := checkVersionExists(ctx, client, appID, version, platform)
	result.Checks = append(result.Checks, versionCheck)

	// 2. Build attached + 3. Encryption compliance
	if versionID != "" {
		buildID, buildAttrs, buildCheck := checkBuildAttachedWithAttrs(ctx, client, versionID)
		result.Checks = append(result.Checks, buildCheck)
		if buildID != "" {
			result.Checks = append(result.Checks, checkBuildEncryption(ctx, client, buildID, buildAttrs))
		}
	}

	appInfoID, appInfoErr := resolveAppInfoID(ctx, client, appID, versionID)
	if appInfoID != "" {
		result.Checks = append(result.Checks, checkAgeRating(ctx, client, appInfoID, appID))
	} else {
		result.Checks = append(result.Checks, unresolvedAppInfoCheck("Age rating", appInfoErr))
	}

	// 4. Content rights
	result.Checks = append(result.Checks, checkContentRights(ctx, client, appID))

	// 5. Primary category (requires appInfoID)
	if appInfoID != "" {
		result.Checks = append(result.Checks, checkPrimaryCategory(ctx, client, appInfoID, appID))
	} else {
		result.Checks = append(result.Checks, unresolvedAppInfoCheck("Primary category", appInfoErr))
	}

	// 6 & 7. Localizations + screenshots
	if versionID != "" {
		locChecks := checkLocalizations(ctx, client, versionID, appID, version, platform)
		result.Checks = append(result.Checks, locChecks...)
	}
	if advisoryCheck, ok := privacyPublishStateAdvisoryCheck(appID); ok {
		result.Checks = append(result.Checks, advisoryCheck)
	}

	tallyCounts(result)
	return result
}

func tallyCounts(result *preflightResult) {
	result.PassCount = 0
	result.FailCount = 0
	for _, c := range result.Checks {
		if c.Passed {
			result.PassCount++
			continue
		}
		if c.Advisory {
			continue
		}
		result.FailCount++
	}
}

func countAdvisories(checks []checkResult) int {
	count := 0
	for _, check := range checks {
		if check.Advisory {
			count++
		}
	}
	return count
}

func privacyPublishStateAdvisoryCheck(appID string) (checkResult, bool) {
	advisory := validation.PrivacyPublishStateAdvisory(appID)
	if advisory.ID == "" {
		return checkResult{}, false
	}
	return checkResult{
		Name:     "App Privacy",
		Passed:   true,
		Advisory: true,
		Message:  advisory.Message,
		Hint:     advisory.Remediation,
	}, true
}

// --- Individual checks ---

func checkVersionExists(ctx context.Context, client *asc.Client, appID, version, platform string) (string, checkResult) {
	resolveCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	versionID, err := shared.ResolveAppStoreVersionID(resolveCtx, client, appID, version, platform)
	if err != nil {
		return "", checkResult{
			Name:    "Version exists",
			Passed:  false,
			Message: fmt.Sprintf("Version %s not found for platform %s: %v", version, platform, err),
			Hint:    fmt.Sprintf("asc versions create --app %s --version %s --platform %s", appID, version, platform),
		}
	}
	return versionID, checkResult{
		Name:    "Version exists",
		Passed:  true,
		Message: fmt.Sprintf("Version %s found", version),
	}
}

func checkBuildAttachedWithAttrs(ctx context.Context, client *asc.Client, versionID string) (string, *asc.BuildAttributes, checkResult) {
	buildCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	buildResp, err := client.GetAppStoreVersionBuild(buildCtx, versionID)
	if err != nil {
		if asc.IsNotFound(err) {
			return "", nil, checkResult{
				Name:    "Build attached",
				Passed:  false,
				Message: "No build attached to this version",
				Hint:    fmt.Sprintf("asc submit create --version-id %s --build BUILD_ID --confirm", versionID),
			}
		}
		return "", nil, checkResult{
			Name:    "Build attached",
			Passed:  false,
			Message: fmt.Sprintf("Failed to check build: %v", err),
		}
	}

	buildID := strings.TrimSpace(buildResp.Data.ID)
	if buildID == "" {
		return "", nil, checkResult{
			Name:    "Build attached",
			Passed:  false,
			Message: "No build attached to this version",
			Hint:    fmt.Sprintf("asc submit create --version-id %s --build BUILD_ID --confirm", versionID),
		}
	}

	buildVersion := buildResp.Data.Attributes.Version
	msg := "Build attached"
	if buildVersion != "" {
		msg = fmt.Sprintf("Build attached (build %s)", buildVersion)
	}
	return buildID, &buildResp.Data.Attributes, checkResult{
		Name:    "Build attached",
		Passed:  true,
		Message: msg,
	}
}

// checkBuildEncryption verifies encryption compliance using attributes already
// fetched by checkBuildAttachedWithAttrs, avoiding a redundant API call. Only
// makes an additional request when usesNonExemptEncryption=true (to verify the
// encryption declaration is attached).
func checkBuildEncryption(ctx context.Context, client *asc.Client, buildID string, attrs *asc.BuildAttributes) checkResult {
	if attrs == nil || attrs.UsesNonExemptEncryption == nil {
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: "usesNonExemptEncryption not set on build",
			Hint:    fmt.Sprintf("Set Uses Non-Exempt Encryption for build %s in App Store Connect, then rerun asc submit preflight", buildID),
		}
	}

	if !*attrs.UsesNonExemptEncryption {
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  true,
			Message: "No non-exempt encryption used",
		}
	}

	// usesNonExemptEncryption=true — verify declaration is attached.
	declCtx, declCancel := shared.ContextWithTimeout(ctx)
	defer declCancel()

	declarationResp, err := client.GetBuildAppEncryptionDeclaration(declCtx, buildID)
	if err != nil {
		if asc.IsNotFound(err) {
			return checkResult{
				Name:    "Encryption compliance",
				Passed:  false,
				Message: "usesNonExemptEncryption=true but no encryption declaration attached to build",
				Hint:    fmt.Sprintf("asc encryption declarations assign-builds --id DECLARATION_ID --build %s", buildID),
			}
		}
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: fmt.Sprintf("Failed to check encryption declaration: %v", err),
		}
	}
	declarationID := strings.TrimSpace(declarationResp.Data.ID)
	if declarationID == "" {
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: "usesNonExemptEncryption=true but build encryption declaration is missing an ID",
			Hint:    fmt.Sprintf("asc encryption declarations assign-builds --id DECLARATION_ID --build %s", buildID),
		}
	}
	declarationState := declarationResp.Data.Attributes.AppEncryptionDeclarationState
	switch declarationState {
	case asc.AppEncryptionDeclarationStateApproved:
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  true,
			Message: fmt.Sprintf("Encryption declaration approved and attached (%s)", declarationID),
		}
	case asc.AppEncryptionDeclarationStateCreated, asc.AppEncryptionDeclarationStateInReview:
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: fmt.Sprintf("Encryption declaration attached (%s) is still %s", declarationID, declarationState),
			Hint:    "Wait for the encryption declaration to reach APPROVED in App Store Connect, then rerun asc submit preflight",
		}
	case asc.AppEncryptionDeclarationStateRejected, asc.AppEncryptionDeclarationStateInvalid, asc.AppEncryptionDeclarationStateExpired:
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: fmt.Sprintf("Encryption declaration attached (%s) is %s", declarationID, declarationState),
			Hint:    fmt.Sprintf("Attach an approved encryption declaration to build %s in App Store Connect, then rerun asc submit preflight", buildID),
		}
	case "":
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: fmt.Sprintf("Encryption declaration attached (%s) is missing approval state", declarationID),
			Hint:    fmt.Sprintf("asc builds app-encryption-declaration get --id %s", buildID),
		}
	default:
		return checkResult{
			Name:    "Encryption compliance",
			Passed:  false,
			Message: fmt.Sprintf("Encryption declaration attached (%s) has unsupported state %q", declarationID, declarationState),
			Hint:    fmt.Sprintf("asc builds app-encryption-declaration get --id %s", buildID),
		}
	}
}

func resolveAppInfoID(ctx context.Context, client *asc.Client, appID, versionID string) (string, error) {
	infoCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if strings.TrimSpace(versionID) != "" {
		appInfoID, err := client.ResolveAppInfoIDForAppStoreVersion(infoCtx, versionID)
		if err != nil {
			return "", fmt.Errorf("failed to resolve app info for version %s: %w", versionID, err)
		}
		if strings.TrimSpace(appInfoID) == "" {
			return "", fmt.Errorf("no app info found for version %s", versionID)
		}
		return appInfoID, nil
	}

	infos, err := client.GetAppInfos(infoCtx, appID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch app info: %w", err)
	}
	if len(infos.Data) == 0 {
		return "", fmt.Errorf("no app info records found")
	}

	candidates := asc.AppInfoCandidates(infos.Data)
	if len(candidates) == 1 {
		return candidates[0].ID, nil
	}

	// Prefer the app info in PREPARE_FOR_SUBMISSION state — that is the
	// editable draft the submission will use.
	for _, c := range candidates {
		if strings.EqualFold(c.State, "PREPARE_FOR_SUBMISSION") {
			return c.ID, nil
		}
	}

	// Fall back to the first sorted candidate for deterministic app-level checks.
	return candidates[0].ID, nil
}

func unresolvedAppInfoCheck(name string, err error) checkResult {
	msg := "Could not resolve app info for this check"
	if err != nil {
		msg = fmt.Sprintf("Could not resolve app info for this check: %v", err)
	}
	return checkResult{
		Name:    name,
		Passed:  false,
		Message: msg,
	}
}

func checkAgeRating(ctx context.Context, client *asc.Client, appInfoID, appID string) checkResult {
	ratingCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetAgeRatingDeclarationForAppInfo(ratingCtx, appInfoID)
	if err != nil {
		if asc.IsNotFound(err) {
			return checkResult{
				Name:    "Age rating",
				Passed:  false,
				Message: "Age rating declaration not found",
				Hint:    fmt.Sprintf("asc age-rating edit --app %s --gambling false --violence-realistic NONE ...", appID),
			}
		}
		return checkResult{
			Name:    "Age rating",
			Passed:  false,
			Message: fmt.Sprintf("Failed to check age rating: %v", err),
		}
	}

	attrs := resp.Data.Attributes
	missing := ageRatingMissingFields(attrs)
	if len(missing) > 0 {
		return checkResult{
			Name:    "Age rating",
			Passed:  false,
			Message: fmt.Sprintf("Age rating incomplete (missing: %s)", strings.Join(missing, ", ")),
			Hint:    fmt.Sprintf("asc age-rating edit --app %s --gambling false --violence-realistic NONE ...", appID),
		}
	}

	return checkResult{
		Name:    "Age rating",
		Passed:  true,
		Message: "Age rating complete",
	}
}

// ageRatingMissingFields checks which required age rating fields are nil.
// Apple requires all boolean and enum content descriptors to be explicitly set.
func ageRatingMissingFields(a asc.AgeRatingDeclarationAttributes) []string {
	var missing []string

	// Boolean descriptors
	if a.Gambling == nil {
		missing = append(missing, "gambling")
	}
	if a.LootBox == nil {
		missing = append(missing, "lootBox")
	}
	if a.UnrestrictedWebAccess == nil {
		missing = append(missing, "unrestrictedWebAccess")
	}

	// Enum descriptors
	if a.AlcoholTobaccoOrDrugUseOrReferences == nil {
		missing = append(missing, "alcoholTobaccoOrDrugUseOrReferences")
	}
	if a.GamblingSimulated == nil {
		missing = append(missing, "gamblingSimulated")
	}
	if a.HorrorOrFearThemes == nil {
		missing = append(missing, "horrorOrFearThemes")
	}
	if a.MatureOrSuggestiveThemes == nil {
		missing = append(missing, "matureOrSuggestiveThemes")
	}
	if a.ProfanityOrCrudeHumor == nil {
		missing = append(missing, "profanityOrCrudeHumor")
	}
	if a.SexualContentGraphicAndNudity == nil {
		missing = append(missing, "sexualContentGraphicAndNudity")
	}
	if a.SexualContentOrNudity == nil {
		missing = append(missing, "sexualContentOrNudity")
	}
	if a.ViolenceCartoonOrFantasy == nil {
		missing = append(missing, "violenceCartoonOrFantasy")
	}
	if a.ViolenceRealistic == nil {
		missing = append(missing, "violenceRealistic")
	}
	if a.ViolenceRealisticProlongedGraphicOrSadistic == nil {
		missing = append(missing, "violenceRealisticProlongedGraphicOrSadistic")
	}

	return missing
}

func checkContentRights(ctx context.Context, client *asc.Client, appID string) checkResult {
	appCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	appResp, err := client.GetApp(appCtx, appID)
	if err != nil {
		return checkResult{
			Name:    "Content rights",
			Passed:  false,
			Message: fmt.Sprintf("Failed to fetch app: %v", err),
		}
	}

	if appResp.Data.Attributes.ContentRightsDeclaration == nil {
		return checkResult{
			Name:    "Content rights",
			Passed:  false,
			Message: "Content rights declaration not set",
			Hint:    fmt.Sprintf("asc apps update --id %s --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT", appID),
		}
	}

	return checkResult{
		Name:    "Content rights",
		Passed:  true,
		Message: "Content rights set",
	}
}

func checkPrimaryCategory(ctx context.Context, client *asc.Client, appInfoID, appID string) checkResult {
	catCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	catResp, err := client.GetAppInfoPrimaryCategory(catCtx, appInfoID)
	if err != nil {
		if asc.IsNotFound(err) {
			return checkResult{
				Name:    "Primary category",
				Passed:  false,
				Message: "Primary category not set",
				Hint:    fmt.Sprintf("asc app-setup categories set --app %s --primary SPORTS", appID),
			}
		}
		return checkResult{
			Name:    "Primary category",
			Passed:  false,
			Message: fmt.Sprintf("Failed to check primary category: %v", err),
		}
	}

	if strings.TrimSpace(catResp.Data.ID) == "" {
		return checkResult{
			Name:    "Primary category",
			Passed:  false,
			Message: "Primary category not set",
			Hint:    fmt.Sprintf("asc app-setup categories set --app %s --primary SPORTS", appID),
		}
	}

	return checkResult{
		Name:    "Primary category",
		Passed:  true,
		Message: fmt.Sprintf("Primary category set (%s)", catResp.Data.ID),
	}
}

func checkLocalizations(ctx context.Context, client *asc.Client, versionID, appID, version, platform string) []checkResult {
	locCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	localizations, err := client.GetAppStoreVersionLocalizations(locCtx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return []checkResult{
			{
				Name:    "Localization metadata",
				Passed:  false,
				Message: fmt.Sprintf("Failed to fetch localizations: %v", err),
			},
			{
				Name:    "Screenshots",
				Passed:  false,
				Message: fmt.Sprintf("Failed to fetch localizations for screenshot checks: %v", err),
			},
		}
	}

	if len(localizations.Data) == 0 {
		return []checkResult{
			{
				Name:    "Localization metadata",
				Passed:  false,
				Message: "No localizations found for this version",
				Hint:    metadataPushHint(appID, version, platform),
			},
			{
				Name:    "Screenshots",
				Passed:  false,
				Message: "No localizations found for screenshot checks",
				Hint:    screenshotUploadHint("", platform),
			},
		}
	}

	metadataCheck := checkResult{
		Name:    "Localization metadata",
		Passed:  false,
		Message: "Could not determine localization requirements",
	}
	updateCtx, updateCancel := shared.ContextWithTimeout(ctx)
	requireWhatsNew, err := isAppUpdate(updateCtx, client, appID, platform)
	updateCancel()
	if err != nil {
		metadataCheck.Message = fmt.Sprintf("Failed to determine whether whatsNew is required: %v", err)
	} else {
		metadataCheck = checkLocalizationMetadata(localizations.Data, appID, version, platform, shared.SubmitReadinessOptions{
			RequireWhatsNew: requireWhatsNew,
		})
	}

	screenshotCheck := checkScreenshots(ctx, client, localizations.Data, platform)
	return []checkResult{metadataCheck, screenshotCheck}
}

func checkLocalizationMetadata(localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes], appID, version, platform string, opts shared.SubmitReadinessOptions) checkResult {
	issues := shared.SubmitReadinessIssuesByLocaleWithOptions(localizations, opts)
	if len(issues) == 0 {
		return checkResult{
			Name:    "Localization metadata",
			Passed:  true,
			Message: "All localizations include required submission metadata",
		}
	}

	return checkResult{
		Name:    "Localization metadata",
		Passed:  false,
		Message: "Missing submission metadata: " + formatSubmitReadinessIssues(issues),
		Hint:    metadataPushHint(appID, version, platform),
	}
}

func metadataPushHint(appID, version, platform string) string {
	hint := fmt.Sprintf("asc metadata push --app %s --version %s", appID, version)
	if strings.TrimSpace(platform) != "" {
		hint += " --platform " + platform
	}
	return hint + " --dir ./metadata"
}

func formatSubmitReadinessIssues(issues []shared.SubmitReadinessIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, fmt.Sprintf("%s (%s)", issue.Locale, strings.Join(issue.MissingFields, ", ")))
	}
	return strings.Join(parts, "; ")
}

func checkScreenshots(ctx context.Context, client *asc.Client, localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes], platform string) checkResult {
	var fetchErrors []string
	successfulInspections := 0
	for _, loc := range localizations {
		locID := loc.ID

		ssCtx, cancel := shared.ContextWithTimeout(ctx)
		sets, err := client.GetAppStoreVersionLocalizationScreenshotSets(ssCtx, locID, asc.WithAppStoreVersionLocalizationScreenshotSetsLimit(50))
		cancel()

		if err != nil {
			fetchErrors = append(fetchErrors, fmt.Sprintf("%s: %v", loc.Attributes.Locale, err))
			continue
		}
		successfulInspections++

		if len(sets.Data) > 0 {
			return checkResult{
				Name:    "Screenshots",
				Passed:  true,
				Message: fmt.Sprintf("Screenshots uploaded (%d set(s) in %s)", len(sets.Data), loc.Attributes.Locale),
			}
		}
	}

	if len(fetchErrors) > 0 {
		if successfulInspections == 0 {
			return checkResult{
				Name:    "Screenshots",
				Passed:  false,
				Message: fmt.Sprintf("Failed to fetch screenshots for all localizations: %s", strings.Join(fetchErrors, "; ")),
			}
		}
		return checkResult{
			Name:    "Screenshots",
			Passed:  false,
			Message: fmt.Sprintf("Could not fully verify screenshots because some localizations failed to load: %s", strings.Join(fetchErrors, "; ")),
		}
	}

	locID := ""
	if len(localizations) > 0 {
		locID = localizations[0].ID
	}
	hint := screenshotUploadHint(locID, platform)

	return checkResult{
		Name:    "Screenshots",
		Passed:  false,
		Message: "No screenshots uploaded",
		Hint:    hint,
	}
}

func screenshotUploadHint(localizationID, platform string) string {
	deviceType := "DEVICE_TYPE"
	switch strings.TrimSpace(platform) {
	case "IOS":
		deviceType = "IPHONE_65"
	case "MAC_OS":
		deviceType = "DESKTOP"
	case "TV_OS":
		deviceType = "APPLE_TV"
	case "VISION_OS":
		deviceType = "APPLE_VISION_PRO"
	}
	if strings.TrimSpace(localizationID) == "" {
		return fmt.Sprintf("asc screenshots upload --version-localization LOC_ID --path ./screenshots --device-type %s", deviceType)
	}
	return fmt.Sprintf("asc screenshots upload --version-localization %s --path ./screenshots --device-type %s", localizationID, deviceType)
}

// --- Text output ---

func printPreflightText(w io.Writer, result *preflightResult) {
	header := fmt.Sprintf("Preflight check for app %s v%s (%s)", result.AppID, result.Version, result.Platform)
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, strings.Repeat("\u2500", len(header)))

	for _, c := range result.Checks {
		if c.Advisory {
			fmt.Fprintf(w, "! %s\n", c.Message)
			if c.Hint != "" {
				fmt.Fprintf(w, "  Hint: %s\n", c.Hint)
			}
		} else if c.Passed {
			fmt.Fprintf(w, "\u2713 %s\n", c.Message)
		} else {
			fmt.Fprintf(w, "\u2717 %s\n", c.Message)
			if c.Hint != "" {
				fmt.Fprintf(w, "  Hint: %s\n", c.Hint)
			}
		}
	}

	fmt.Fprintln(w)
	advisoryCount := countAdvisories(result.Checks)
	if result.FailCount == 0 && advisoryCount == 0 {
		fmt.Fprintln(w, "Result: All checks passed. Ready to submit.")
	} else if result.FailCount == 0 {
		label := "advisories"
		if advisoryCount == 1 {
			label = "advisory"
		}
		fmt.Fprintf(w, "Result: Required checks passed, but %d %s should be reviewed before submitting.\n", advisoryCount, label)
	} else {
		fmt.Fprintf(w, "Result: %d issue(s) found. Fix them before submitting.\n", result.FailCount)
	}
}
