package assets

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshotcatalog"
	reviewshots "github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

const (
	defaultReviewOutputDir    = "./screenshots/review"
	defaultReviewManifestFile = "manifest.json"
	defaultReviewApprovalFile = "approved.json"
)

type screenshotReviewPlanIssue struct {
	Severity    string `json:"severity"`
	Key         string `json:"key,omitempty"`
	Locale      string `json:"locale,omitempty"`
	DisplayType string `json:"displayType,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type screenshotReviewPlanGroup struct {
	Locale                string                        `json:"locale"`
	VersionLocalizationID string                        `json:"versionLocalizationId"`
	DisplayType           string                        `json:"displayType"`
	Files                 []string                      `json:"files"`
	Result                asc.AppScreenshotUploadResult `json:"result,omitempty"`
}

type screenshotReviewPlanResult struct {
	AppID                string                      `json:"appId"`
	Version              string                      `json:"version"`
	VersionID            string                      `json:"versionId"`
	Platform             string                      `json:"platform"`
	ReviewOutputDir      string                      `json:"reviewOutputDir"`
	ManifestPath         string                      `json:"manifestPath"`
	ApprovalPath         string                      `json:"approvalPath"`
	SkipExisting         bool                        `json:"skipExisting"`
	Replace              bool                        `json:"replace"`
	Applied              bool                        `json:"applied,omitempty"`
	ApprovedReadyEntries int                         `json:"approvedReadyEntries"`
	PlannedGroups        int                         `json:"plannedGroups"`
	ErrorCount           int                         `json:"errorCount"`
	WarningCount         int                         `json:"warningCount"`
	Issues               []screenshotReviewPlanIssue `json:"issues,omitempty"`
	Groups               []screenshotReviewPlanGroup `json:"groups,omitempty"`
}

type screenshotReviewPlanOptions struct {
	AppID           string
	Version         string
	VersionID       string
	Platform        string
	ReviewOutputDir string
	ManifestPath    string
	ApprovalPath    string
	SkipExisting    bool
	Replace         bool
	Apply           bool
	Confirm         bool
}

type screenshotGroupKey struct {
	locale         string
	localizationID string
	displayType    string
}

// NewAssetsScreenshotsPlanCommand returns the screenshots plan subcommand.
func NewAssetsScreenshotsPlanCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("plan", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version ID or numeric version string")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (defaults to IOS for version strings; IDs use their actual platform)")
	reviewOutputDir := fs.String("review-output-dir", defaultReviewOutputDir, "Directory containing review artifacts")
	manifestPath := fs.String("manifest-path", "", "Optional manifest path (default: <review-output-dir>/manifest.json)")
	approvalPath := fs.String("approval-path", "", "Optional approvals path (default: <review-output-dir>/approved.json)")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose checksum already exists remotely")
	replace := fs.Bool("replace", false, "Delete existing screenshots in each target set before uploading")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc screenshots plan --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\") [flags]",
		ShortHelp:  "Plan screenshot uploads from approved review artifacts.",
		LongHelp: `Plan App Store screenshot uploads from approved review artifacts.

This reads review manifest + approvals, maps approved ready entries to remote
version localizations, and previews grouped upload intent per display type.

Examples:
  asc screenshots plan --app "123456789" --version "1.2.3"
  asc screenshots plan --app "123456789" --version "1.2.3" --skip-existing
  asc screenshots plan --app "123456789" --version "VERSION_ID" --replace`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := executeScreenshotReviewPlan(ctx, d, screenshotReviewPlanOptions{
				AppID:           *appID,
				Version:         *version,
				VersionID:       *versionID,
				Platform:        *platform,
				ReviewOutputDir: *reviewOutputDir,
				ManifestPath:    *manifestPath,
				ApprovalPath:    *approvalPath,
				SkipExisting:    *skipExisting,
				Replace:         *replace,
				Apply:           false,
			})
			if printErr := printScreenshotReviewPlanResult(d, result, output); printErr != nil {
				return printErr
			}
			if err != nil {
				return err
			}
			if result.ErrorCount > 0 {
				return shared.NewValidationReportedError(fmt.Errorf("screenshots plan: found %d blocking issue(s)", result.ErrorCount))
			}
			return nil
		},
	}
}

// NewAssetsScreenshotsApplyCommand returns the screenshots apply subcommand.
func NewAssetsScreenshotsApplyCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apply", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version ID or numeric version string")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (defaults to IOS for version strings; IDs use their actual platform)")
	reviewOutputDir := fs.String("review-output-dir", defaultReviewOutputDir, "Directory containing review artifacts")
	manifestPath := fs.String("manifest-path", "", "Optional manifest path (default: <review-output-dir>/manifest.json)")
	approvalPath := fs.String("approval-path", "", "Optional approvals path (default: <review-output-dir>/approved.json)")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose checksum already exists remotely")
	replace := fs.Bool("replace", false, "Delete existing screenshots in each target set before uploading")
	confirm := fs.Bool("confirm", false, "Confirm screenshot uploads (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc screenshots apply --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\") --confirm [flags]",
		ShortHelp:  "Apply screenshot uploads from approved review artifacts.",
		LongHelp: `Apply App Store screenshot uploads from approved review artifacts.

Examples:
  asc screenshots apply --app "123456789" --version "1.2.3" --confirm
  asc screenshots apply --app "123456789" --version "1.2.3" --skip-existing --confirm
  asc screenshots apply --app "123456789" --version "VERSION_ID" --replace --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := executeScreenshotReviewPlan(ctx, d, screenshotReviewPlanOptions{
				AppID:           *appID,
				Version:         *version,
				VersionID:       *versionID,
				Platform:        *platform,
				ReviewOutputDir: *reviewOutputDir,
				ManifestPath:    *manifestPath,
				ApprovalPath:    *approvalPath,
				SkipExisting:    *skipExisting,
				Replace:         *replace,
				Apply:           true,
				Confirm:         *confirm,
			})
			if printErr := printScreenshotReviewPlanResult(d, result, output); printErr != nil {
				return printErr
			}
			if err != nil {
				return err
			}
			if result.ErrorCount > 0 {
				return shared.NewValidationReportedError(fmt.Errorf("screenshots apply: found %d blocking issue(s)", result.ErrorCount))
			}
			return nil
		},
	}
}

func executeScreenshotReviewPlan(ctx context.Context, d *shared.Deps, opts screenshotReviewPlanOptions) (*screenshotReviewPlanResult, error) {
	appSelector := d.ResolveAppSelector(opts.AppID)
	resolvedAppID := appSelector.Value
	if strings.TrimSpace(resolvedAppID) == "" {
		fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
		return nil, shared.MissingRequiredUsageError("--app")
	}

	versionValue := strings.TrimSpace(opts.Version)
	versionIDValue := strings.TrimSpace(opts.VersionID)
	if versionValue == "" && versionIDValue == "" {
		return nil, d.UsageError("--version or --version-id is required")
	}
	if opts.Apply {
		if err := d.RequireConfirm(opts.Confirm); err != nil {
			return nil, err
		}
	}
	if versionValue != "" && versionIDValue != "" {
		return nil, d.UsageError("--version and --version-id are mutually exclusive")
	}
	if opts.SkipExisting && opts.Replace {
		fmt.Fprintln(d.IO.ErrOut, "Error: --skip-existing and --replace are mutually exclusive")
		return nil, flag.ErrHelp
	}

	versionValue, versionIDValue, err := normalizeScreenshotVersionSelector(versionValue, versionIDValue)
	if err != nil {
		return nil, d.UsageError(err.Error())
	}

	normalizedPlatform, err := resolveAppScopedScreenshotPlatform(versionValue, opts.Platform)
	if err != nil {
		return nil, d.UsageError(err.Error())
	}

	client, err := d.Client()
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()
	requestCtx = asc.WithNoRedirectReads(requestCtx)

	resolvedAppID, err = shared.ResolveScopedAppID(requestCtx, client, appSelector)
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	reviewOutputDir, manifestPath, approvalPath, err := resolveReviewArtifactPaths(d, opts.ReviewOutputDir, opts.ManifestPath, opts.ApprovalPath)
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	manifest, err := reviewshots.LoadReviewManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}
	approvals, err := reviewshots.LoadApprovals(approvalPath)
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	resolvedVersionID, resolvedVersion, resolvedPlatform, err := resolveScreenshotPlanVersion(requestCtx, client, resolvedAppID, versionValue, versionIDValue, normalizedPlatform)
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), screenshotVersionSelectorError(err, opts.Version))
	}

	localizationsResp, err := client.GetAppStoreVersionLocalizations(requestCtx, resolvedVersionID, asc.Limit[asc.AppStoreVersionLocalizationsOption](200))
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: fetch version localizations: %w", reviewPlanVerb(opts.Apply), err)
	}

	result := &screenshotReviewPlanResult{
		AppID:           resolvedAppID,
		Version:         resolvedVersion,
		VersionID:       resolvedVersionID,
		Platform:        resolvedPlatform,
		ReviewOutputDir: reviewOutputDir,
		ManifestPath:    manifestPath,
		ApprovalPath:    approvalPath,
		SkipExisting:    opts.SkipExisting,
		Replace:         opts.Replace,
		Applied:         opts.Apply,
		Issues:          make([]screenshotReviewPlanIssue, 0),
		Groups:          make([]screenshotReviewPlanGroup, 0),
	}

	localizationByLocale := make(map[string]string, len(localizationsResp.Data))
	remoteLocales := make([]string, 0, len(localizationsResp.Data))
	for _, item := range localizationsResp.Data {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		localizationByLocale[locale] = strings.TrimSpace(item.ID)
		remoteLocales = append(remoteLocales, locale)
	}
	sort.Strings(remoteLocales)

	groupedFiles := make(map[screenshotGroupKey][]string)
	coverageByLocale := make(map[string]map[string]bool)
	for _, entry := range manifest.Entries {
		if !approvals[strings.TrimSpace(entry.Key)] {
			continue
		}
		if entry.Status != "ready" {
			appendScreenshotReviewIssue(result, "error", entry.Key, entry.Locale, "", "approved review entry is not ready for upload", "Regenerate or reframe the screenshot until the review status is ready.")
			continue
		}
		if !entry.ValidAppStoreSize || len(entry.DisplayTypes) == 0 {
			appendScreenshotReviewIssue(result, "error", entry.Key, entry.Locale, "", "approved review entry does not match a supported App Store screenshot size", "Reframe the screenshot to a supported App Store size and regenerate the review manifest.")
			continue
		}
		locale := strings.TrimSpace(entry.Locale)
		if locale == "" {
			appendScreenshotReviewIssue(result, "error", entry.Key, "", "", "approved review entry is missing a locale", "Place the screenshot under a locale-specific review path and regenerate the review manifest.")
			continue
		}
		localizationID := strings.TrimSpace(localizationByLocale[locale])
		if localizationID == "" {
			appendScreenshotReviewIssue(result, "error", entry.Key, locale, "", "no matching App Store version localization exists for this locale", "Create the missing version localization before applying screenshot uploads.")
			continue
		}

		canonicalDisplayTypes := make([]string, 0, len(entry.DisplayTypes))
		seenDisplayTypes := make(map[string]struct{}, len(entry.DisplayTypes))
		displayTypesValid := true
		for _, displayType := range entry.DisplayTypes {
			rawDisplayType := strings.TrimSpace(displayType)
			if rawDisplayType == "" {
				appendScreenshotReviewIssue(
					result,
					"error",
					entry.Key,
					locale,
					displayType,
					"approved review entry contains an empty screenshot display type",
					"Regenerate the review manifest with a supported App Store screenshot size.",
				)
				displayTypesValid = false
				continue
			}
			displayValue := asc.CanonicalScreenshotDisplayTypeForAPI(rawDisplayType)
			if !asc.IsValidScreenshotDisplayType(displayValue) {
				appendScreenshotReviewIssue(
					result,
					"error",
					entry.Key,
					locale,
					rawDisplayType,
					fmt.Sprintf("approved review entry contains unsupported screenshot display type %q", rawDisplayType),
					"Regenerate the review manifest with a supported App Store screenshot size.",
				)
				displayTypesValid = false
				continue
			}
			if _, exists := seenDisplayTypes[displayValue]; exists {
				continue
			}
			seenDisplayTypes[displayValue] = struct{}{}
			canonicalDisplayTypes = append(canonicalDisplayTypes, displayValue)
		}
		if !displayTypesValid || len(canonicalDisplayTypes) == 0 {
			continue
		}
		// Manifests written before the iPad slot correction list both the
		// retired 12.9" slot and its successor for the same size; upload to the
		// current slot only.
		canonicalDisplayTypes = screenshotcatalog.PreferCurrentDisplayTypes(canonicalDisplayTypes)

		result.ApprovedReadyEntries++
		if coverageByLocale[locale] == nil {
			coverageByLocale[locale] = make(map[string]bool)
		}
		for _, displayValue := range canonicalDisplayTypes {
			groupKey := screenshotGroupKey{
				locale:         locale,
				localizationID: localizationID,
				displayType:    displayValue,
			}
			groupedFiles[groupKey] = append(groupedFiles[groupKey], strings.TrimSpace(entry.FramedPath))
			coverageByLocale[locale][displayValue] = true
		}
	}

	if result.ApprovedReadyEntries == 0 {
		appendScreenshotReviewIssue(result, "error", "", "", "", "no approved ready screenshots were found in the review artifacts", "Run `asc screenshots review-approve` to approve ready screenshots before planning uploads.")
	}

	focusedDisplayTypes := focusedScreenshotDisplayTypesForPlatform(resolvedPlatform)
	for _, locale := range remoteLocales {
		covered := coverageByLocale[locale]
		for _, displayType := range focusedDisplayTypes {
			if covered != nil && covered[displayType] {
				continue
			}
			appendScreenshotReviewIssue(result, "warning", "", locale, displayType, "approved review artifacts do not cover this focused screenshot slot", "Add an approved screenshot for this locale and display type before release.")
		}
	}

	groupKeys := make([]screenshotGroupKey, 0, len(groupedFiles))
	for key := range groupedFiles {
		groupKeys = append(groupKeys, key)
	}
	sort.Slice(groupKeys, func(i, j int) bool {
		if groupKeys[i].locale == groupKeys[j].locale {
			if groupKeys[i].displayType == groupKeys[j].displayType {
				return groupKeys[i].localizationID < groupKeys[j].localizationID
			}
			return groupKeys[i].displayType < groupKeys[j].displayType
		}
		return groupKeys[i].locale < groupKeys[j].locale
	})

	result.PlannedGroups = len(groupKeys)

	for _, key := range groupKeys {
		files := cloneSortedFiles(groupedFiles[key])
		if err := validateScreenshotDimensions(files, key.displayType); err != nil {
			appendScreenshotReviewIssue(
				result,
				"error",
				"",
				key.locale,
				key.displayType,
				err.Error(),
				"Regenerate the approved screenshot artifact for this screenshot slot before uploading.",
			)
		}
	}

	blockingIssues := result.ErrorCount > 0
	if blockingIssues {
		for _, key := range groupKeys {
			result.Groups = append(result.Groups, screenshotReviewPlanGroup{
				Locale:                key.locale,
				VersionLocalizationID: key.localizationID,
				DisplayType:           key.displayType,
				Files:                 cloneSortedFiles(groupedFiles[key]),
			})
		}
		return result, nil
	}

	// Screenshot uploads must not inherit the short per-request deadline used
	// for the lookups above: each group reserves, uploads, commits and then
	// polls asset delivery. Derive the upload budget from the un-deadlined
	// parent so uploadScreenshots can apply the upload timeout per group.
	uploadParentCtx := asc.WithNoRedirectReads(shared.ContextWithoutTimeout(ctx))
	for _, key := range groupKeys {
		files := cloneSortedFiles(groupedFiles[key])
		uploadResult, err := uploadScreenshots(uploadParentCtx, d, client, key.localizationID, key.displayType, files, opts.SkipExisting, opts.Replace, !opts.Apply)
		if err == nil || len(uploadResult.Results) > 0 {
			result.Groups = append(result.Groups, screenshotReviewPlanGroup{
				Locale:                key.locale,
				VersionLocalizationID: key.localizationID,
				DisplayType:           key.displayType,
				Files:                 files,
				Result:                uploadResult,
			})
		}
		if err != nil {
			appendScreenshotReviewIssue(result, "error", "", key.locale, key.displayType, err.Error(), "")
			return result, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
		}
	}

	return result, nil
}

// printScreenshotReviewPlanResult renders whatever the run produced, including
// completed screenshot receipts from a group that failed partway through.
func printScreenshotReviewPlanResult(d *shared.Deps, result *screenshotReviewPlanResult, output shared.OutputFlags) error {
	if result == nil {
		return nil
	}
	return d.PrintOutputWithRenderers(
		result,
		*output.Output,
		*output.Pretty,
		func() error { return renderScreenshotReviewPlanResult(d, result, false) },
		func() error { return renderScreenshotReviewPlanResult(d, result, true) },
	)
}

func resolveScreenshotPlanVersion(ctx context.Context, client *asc.Client, appID, version, versionID, platform string) (string, string, string, error) {
	ctx = asc.WithNoRedirectReads(ctx)
	version = strings.TrimSpace(version)
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		resolvedVersionID, err := shared.ResolveAppStoreVersionID(ctx, client, appID, version, strings.TrimSpace(platform))
		if err != nil {
			return "", "", "", err
		}
		versionID = strings.TrimSpace(resolvedVersionID)
		if versionID == "" {
			return "", "", "", fmt.Errorf("version %q did not return a resource ID", version)
		}
	}

	versionData, err := shared.ResolveOwnedAppStoreVersionByID(ctx, client, appID, versionID, platform)
	if err != nil {
		return "", "", "", err
	}
	resolvedVersion := strings.TrimSpace(versionData.Attributes.VersionString)
	if version != "" && resolvedVersion != version {
		return "", "", "", fmt.Errorf("version ID %q has version %q, expected %q", versionID, resolvedVersion, version)
	}
	return versionID, resolvedVersion, strings.TrimSpace(string(versionData.Attributes.Platform)), nil
}

func resolveReviewArtifactPaths(d *shared.Deps, outputDir, manifestPath, approvalPath string) (string, string, string, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir != "" {
		outputDir = d.OSPath(outputDir)
	}
	resolvedOutputDir, err := reviewshots.ResolveReviewOutputDir(outputDir)
	if err != nil {
		return "", "", "", err
	}

	resolve := func(value, defaultName string) string {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return filepath.Join(resolvedOutputDir, defaultName)
		}
		if filepath.IsAbs(trimmed) {
			return trimmed
		}
		return filepath.Join(resolvedOutputDir, trimmed)
	}

	return resolvedOutputDir, resolve(manifestPath, defaultReviewManifestFile), resolve(approvalPath, defaultReviewApprovalFile), nil
}

func appendScreenshotReviewIssue(result *screenshotReviewPlanResult, severity, key, locale, displayType, message, remediation string) {
	result.Issues = append(result.Issues, screenshotReviewPlanIssue{
		Severity:    severity,
		Key:         strings.TrimSpace(key),
		Locale:      strings.TrimSpace(locale),
		DisplayType: strings.TrimSpace(displayType),
		Message:     message,
		Remediation: remediation,
	})
	if severity == "error" {
		result.ErrorCount++
		return
	}
	result.WarningCount++
}

func cloneSortedFiles(files []string) []string {
	cloned := append([]string(nil), files...)
	sort.Strings(cloned)
	return cloned
}

func renderScreenshotReviewPlanResult(d *shared.Deps, result *screenshotReviewPlanResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	renderSection := func(title string, headers []string, rows [][]string) error {
		heading := d.Bold(strings.ToUpper(title)) + "\n"
		renderRows := asc.WriteTableTo
		if markdown {
			heading = fmt.Sprintf("### %s\n\n", title)
			renderRows = asc.WriteMarkdownTo
		}
		if _, err := fmt.Fprint(d.IO.Out, heading); err != nil {
			return err
		}
		if err := renderRows(d.IO.Out, headers, rows); err != nil {
			return err
		}
		_, err := fmt.Fprintln(d.IO.Out)
		return err
	}

	if err := renderSection("Summary", []string{"field", "value"}, [][]string{
		{"appId", result.AppID},
		{"version", result.Version},
		{"versionId", result.VersionID},
		{"platform", result.Platform},
		{"reviewOutputDir", result.ReviewOutputDir},
		{"manifestPath", result.ManifestPath},
		{"approvalPath", result.ApprovalPath},
		{"mode", map[bool]string{true: "apply", false: "plan"}[result.Applied]},
		{"approvedReadyEntries", fmt.Sprintf("%d", result.ApprovedReadyEntries)},
		{"plannedGroups", fmt.Sprintf("%d", result.PlannedGroups)},
		{"errorCount", fmt.Sprintf("%d", result.ErrorCount)},
		{"warningCount", fmt.Sprintf("%d", result.WarningCount)},
	}); err != nil {
		return err
	}

	if len(result.Groups) > 0 {
		rows := make([][]string, 0, len(result.Groups))
		for _, group := range result.Groups {
			rows = append(rows, []string{
				group.Locale,
				group.DisplayType,
				group.VersionLocalizationID,
				fmt.Sprintf("%d", len(group.Files)),
				summarizeUploadStates(group.Result.Results),
			})
		}
		if err := renderSection("Groups", []string{"locale", "displayType", "versionLocalizationId", "files", "result"}, rows); err != nil {
			return err
		}
	}

	if len(result.Issues) > 0 {
		rows := make([][]string, 0, len(result.Issues))
		for _, issue := range result.Issues {
			rows = append(rows, []string{
				issue.Severity,
				issue.Locale,
				issue.DisplayType,
				issue.Message,
				shared.OrNA(issue.Remediation),
			})
		}
		if err := renderSection("Issues", []string{"severity", "locale", "displayType", "message", "remediation"}, rows); err != nil {
			return err
		}
	}

	return nil
}

func summarizeUploadStates(results []asc.AssetUploadResultItem) string {
	if len(results) == 0 {
		return "n/a"
	}
	counts := make(map[string]int)
	states := make([]string, 0, len(results))
	for _, item := range results {
		state := strings.TrimSpace(item.State)
		if state == "" {
			state = "uploaded"
		}
		if _, ok := counts[state]; !ok {
			states = append(states, state)
		}
		counts[state]++
	}
	sort.Strings(states)

	parts := make([]string, 0, len(states))
	for _, state := range states {
		parts = append(parts, fmt.Sprintf("%s=%d", state, counts[state]))
	}
	return strings.Join(parts, ", ")
}

func reviewPlanVerb(apply bool) string {
	if apply {
		return "apply"
	}
	return "plan"
}
