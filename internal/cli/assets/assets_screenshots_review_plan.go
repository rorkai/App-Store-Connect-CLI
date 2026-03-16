package assets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
}

type screenshotGroupKey struct {
	locale         string
	localizationID string
	displayType    string
}

// AssetsScreenshotsPlanCommand returns the screenshots plan subcommand.
func AssetsScreenshotsPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version string")
	versionID := fs.String("version-id", "", "App Store version ID")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	reviewOutputDir := fs.String("review-output-dir", defaultReviewOutputDir, "Directory containing review artifacts")
	manifestPath := fs.String("manifest-path", "", "Optional manifest path (default: <review-output-dir>/manifest.json)")
	approvalPath := fs.String("approval-path", "", "Optional approvals path (default: <review-output-dir>/approved.json)")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose checksum already exists remotely")
	replace := fs.Bool("replace", false, "Delete existing screenshots in each target set before uploading")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc screenshots plan --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\") [flags]",
		ShortHelp:  "[experimental] Plan screenshot uploads from approved review artifacts.",
		LongHelp: `Plan App Store screenshot uploads from approved review artifacts (experimental).

This reads review manifest + approvals, maps approved ready entries to remote
version localizations, and previews grouped upload intent per display type.

Examples:
  asc screenshots plan --app "123456789" --version "1.2.3"
  asc screenshots plan --app "123456789" --version "1.2.3" --skip-existing
  asc screenshots plan --app "123456789" --version-id "VERSION_ID" --replace`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := executeScreenshotReviewPlan(ctx, screenshotReviewPlanOptions{
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
			if err != nil {
				return err
			}
			if err := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderScreenshotReviewPlanResult(result, false) },
				func() error { return renderScreenshotReviewPlanResult(result, true) },
			); err != nil {
				return err
			}
			if result.ErrorCount > 0 {
				return shared.NewReportedError(fmt.Errorf("screenshots plan: found %d blocking issue(s)", result.ErrorCount))
			}
			return nil
		},
	}
}

// AssetsScreenshotsApplyCommand returns the screenshots apply subcommand.
func AssetsScreenshotsApplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version string")
	versionID := fs.String("version-id", "", "App Store version ID")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	reviewOutputDir := fs.String("review-output-dir", defaultReviewOutputDir, "Directory containing review artifacts")
	manifestPath := fs.String("manifest-path", "", "Optional manifest path (default: <review-output-dir>/manifest.json)")
	approvalPath := fs.String("approval-path", "", "Optional approvals path (default: <review-output-dir>/approved.json)")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose checksum already exists remotely")
	replace := fs.Bool("replace", false, "Delete existing screenshots in each target set before uploading")
	confirm := fs.Bool("confirm", false, "Confirm screenshot uploads (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc screenshots apply --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\") --confirm [flags]",
		ShortHelp:  "[experimental] Apply screenshot uploads from approved review artifacts.",
		LongHelp: `Apply App Store screenshot uploads from approved review artifacts (experimental).

Examples:
  asc screenshots apply --app "123456789" --version "1.2.3" --confirm
  asc screenshots apply --app "123456789" --version "1.2.3" --skip-existing --confirm
  asc screenshots apply --app "123456789" --version-id "VERSION_ID" --replace --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("screenshots apply does not accept positional arguments")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to apply screenshot uploads")
				return flag.ErrHelp
			}

			result, err := executeScreenshotReviewPlan(ctx, screenshotReviewPlanOptions{
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
			})
			if err != nil {
				return err
			}
			if err := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderScreenshotReviewPlanResult(result, false) },
				func() error { return renderScreenshotReviewPlanResult(result, true) },
			); err != nil {
				return err
			}
			if result.ErrorCount > 0 {
				return shared.NewReportedError(fmt.Errorf("screenshots apply: found %d blocking issue(s)", result.ErrorCount))
			}
			return nil
		},
	}
}

func executeScreenshotReviewPlan(ctx context.Context, opts screenshotReviewPlanOptions) (*screenshotReviewPlanResult, error) {
	resolvedAppID := shared.ResolveAppID(opts.AppID)
	if strings.TrimSpace(resolvedAppID) == "" {
		fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
		return nil, flag.ErrHelp
	}

	versionValue := strings.TrimSpace(opts.Version)
	versionIDValue := strings.TrimSpace(opts.VersionID)
	if versionValue == "" && versionIDValue == "" {
		return nil, shared.UsageError("--version or --version-id is required")
	}
	if versionValue != "" && versionIDValue != "" {
		return nil, shared.UsageError("--version and --version-id are mutually exclusive")
	}
	if opts.SkipExisting && opts.Replace {
		fmt.Fprintln(os.Stderr, "Error: --skip-existing and --replace are mutually exclusive")
		return nil, flag.ErrHelp
	}

	normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(opts.Platform)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
	if err != nil {
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	reviewOutputDir, manifestPath, approvalPath, err := resolveReviewArtifactPaths(opts.ReviewOutputDir, opts.ManifestPath, opts.ApprovalPath)
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
		return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
	}

	localizationsResp, err := client.GetAppStoreVersionLocalizations(requestCtx, resolvedVersionID, asc.WithAppStoreVersionLocalizationsLimit(200))
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

		result.ApprovedReadyEntries++
		if coverageByLocale[locale] == nil {
			coverageByLocale[locale] = make(map[string]bool)
		}

		for _, displayType := range entry.DisplayTypes {
			displayValue := strings.TrimSpace(displayType)
			if displayValue == "" {
				continue
			}
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

	for _, locale := range remoteLocales {
		covered := coverageByLocale[locale]
		for _, displayType := range focusedScreenshotDisplayTypes {
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

	for _, key := range groupKeys {
		files := cloneSortedFiles(groupedFiles[key])
		uploadResult, err := uploadScreenshots(requestCtx, client, key.localizationID, key.displayType, files, opts.SkipExisting, opts.Replace, !opts.Apply)
		if err != nil {
			return nil, fmt.Errorf("screenshots %s: %w", reviewPlanVerb(opts.Apply), err)
		}
		result.Groups = append(result.Groups, screenshotReviewPlanGroup{
			Locale:                key.locale,
			VersionLocalizationID: key.localizationID,
			DisplayType:           key.displayType,
			Files:                 files,
			Result:                uploadResult,
		})
	}

	return result, nil
}

func resolveScreenshotPlanVersion(ctx context.Context, client *asc.Client, appID, version, versionID, platform string) (string, string, string, error) {
	if strings.TrimSpace(versionID) != "" {
		resp, err := client.GetAppStoreVersion(ctx, strings.TrimSpace(versionID), asc.WithAppStoreVersionInclude([]string{"app"}))
		if err != nil {
			return "", "", "", err
		}
		relatedAppID, err := asc.AppStoreVersionAppID(resp)
		if err != nil {
			return "", "", "", err
		}
		if !strings.EqualFold(strings.TrimSpace(relatedAppID), strings.TrimSpace(appID)) {
			return "", "", "", fmt.Errorf("version %q belongs to app %q, not %q", strings.TrimSpace(versionID), relatedAppID, appID)
		}
		return strings.TrimSpace(resp.Data.ID), strings.TrimSpace(resp.Data.Attributes.VersionString), strings.TrimSpace(string(resp.Data.Attributes.Platform)), nil
	}

	resolvedVersionID, err := shared.ResolveAppStoreVersionID(ctx, client, appID, strings.TrimSpace(version), strings.TrimSpace(platform))
	if err != nil {
		return "", "", "", err
	}
	return resolvedVersionID, strings.TrimSpace(version), strings.TrimSpace(platform), nil
}

func resolveReviewArtifactPaths(outputDir, manifestPath, approvalPath string) (string, string, string, error) {
	resolvedOutputDir, err := reviewshots.ResolveReviewOutputDir(strings.TrimSpace(outputDir))
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

func renderScreenshotReviewPlanResult(result *screenshotReviewPlanResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	shared.RenderSection("Summary", []string{"field", "value"}, [][]string{
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
	}, markdown)

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
		shared.RenderSection("Groups", []string{"locale", "displayType", "versionLocalizationId", "files", "result"}, rows, markdown)
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
		shared.RenderSection("Issues", []string{"severity", "locale", "displayType", "message", "remediation"}, rows, markdown)
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
