package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var keywordPlanFields = []string{"keywords"}

// MetadataKeywordsWarning highlights submit-readiness risk during keyword creates.
type MetadataKeywordsWarning struct {
	Action        string   `json:"action"`
	Locale        string   `json:"locale"`
	Message       string   `json:"message"`
	MissingFields []string `json:"missingFields,omitempty"`
}

// MetadataKeywordsPlanResult describes keyword-only remote changes.
type MetadataKeywordsPlanResult struct {
	AppID               string                           `json:"appId"`
	Version             string                           `json:"version"`
	VersionID           string                           `json:"versionId"`
	Dir                 string                           `json:"dir"`
	DryRun              bool                             `json:"dryRun"`
	Applied             bool                             `json:"applied,omitempty"`
	Total               int                              `json:"total,omitempty"`
	Succeeded           int                              `json:"succeeded,omitempty"`
	Failed              int                              `json:"failed,omitempty"`
	FailureArtifactPath string                           `json:"failureArtifactPath,omitempty"`
	Adds                []PlanItem                       `json:"adds"`
	Updates             []PlanItem                       `json:"updates"`
	APICalls            []PlanAPICall                    `json:"apiCalls,omitempty"`
	Actions             []ApplyAction                    `json:"actions,omitempty"`
	Results             []MetadataKeywordsMutationResult `json:"results,omitempty"`
	Warnings            []MetadataKeywordsWarning        `json:"warnings,omitempty"`
}

type metadataKeywordsPlanOptions struct {
	AppID                string
	Version              string
	Platform             string
	Dir                  string
	DryRun               bool
	Apply                bool
	Confirm              bool
	FailureArtifactScope string
	LocalState           map[string]keywordLocalState
}

// MetadataKeywordsPlanCommand returns the keywords plan subcommand.
func MetadataKeywordsPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords plan", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc metadata keywords plan --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" [flags]",
		ShortHelp:  "Preview keyword-only changes against App Store Connect.",
		LongHelp: `Preview keyword-only changes against App Store Connect.

This command reads local canonical metadata files, looks only at the version
localization ` + "`keywords`" + ` field, and builds a non-mutating plan.

Examples:
  asc metadata keywords plan --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata keywords plan --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetadataKeywordsPlanLikeCommand(ctx, args, "metadata keywords plan", metadataKeywordsPlanOptions{
				AppID:    *appID,
				Version:  *version,
				Platform: *platform,
				Dir:      *dir,
				DryRun:   true,
			}, output)
		},
	}
}

// MetadataKeywordsDiffCommand returns the keywords diff subcommand.
func MetadataKeywordsDiffCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords diff", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "diff",
		ShortUsage: "asc metadata keywords diff --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" [flags]",
		ShortHelp:  "Diff local canonical keywords against App Store Connect.",
		LongHelp: `Diff local canonical keywords against App Store Connect.

This is a keyword-focused alias of the planning flow, intended for human review
of local-vs-remote keyword changes before apply.

Examples:
  asc metadata keywords diff --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata keywords diff --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetadataKeywordsPlanLikeCommand(ctx, args, "metadata keywords diff", metadataKeywordsPlanOptions{
				AppID:    *appID,
				Version:  *version,
				Platform: *platform,
				Dir:      *dir,
				DryRun:   true,
			}, output)
		},
	}
}

func runMetadataKeywordsPlanLikeCommand(
	ctx context.Context,
	args []string,
	errorPrefix string,
	opts metadataKeywordsPlanOptions,
	output shared.OutputFlags,
) error {
	if len(args) > 0 {
		return shared.UsageError(fmt.Sprintf("%s does not accept positional arguments", errorPrefix))
	}
	result, err := executeMetadataKeywordsPlan(ctx, opts)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}
	return shared.PrintOutputWithRenderers(
		result,
		*output.Output,
		*output.Pretty,
		func() error { return printMetadataKeywordsPlanTable(result) },
		func() error { return printMetadataKeywordsPlanMarkdown(result) },
	)
}

func validateMetadataKeywordsRemoteInputs(appID string, version string, dir string, platform string) (string, string, string, string, error) {
	resolvedAppID := shared.ResolveAppID(appID)
	if resolvedAppID == "" {
		return "", "", "", "", shared.UsageError("--app is required (or set ASC_APP_ID)")
	}

	dirValue, versionValue, err := validateMetadataKeywordDirVersion(dir, version)
	if err != nil {
		return "", "", "", "", err
	}

	platformValue := strings.TrimSpace(platform)
	if platformValue != "" {
		normalizedPlatform, platformErr := shared.NormalizeAppStoreVersionPlatform(platformValue)
		if platformErr != nil {
			return "", "", "", "", shared.UsageError(platformErr.Error())
		}
		platformValue = normalizedPlatform
	}

	return resolvedAppID, versionValue, dirValue, platformValue, nil
}

func executeMetadataKeywordsPlan(ctx context.Context, opts metadataKeywordsPlanOptions) (MetadataKeywordsPlanResult, error) {
	resolvedAppID, versionValue, dirValue, platformValue, err := validateMetadataKeywordsRemoteInputs(opts.AppID, opts.Version, opts.Dir, opts.Platform)
	if err != nil {
		return MetadataKeywordsPlanResult{}, err
	}
	if opts.Apply && !opts.Confirm {
		return MetadataKeywordsPlanResult{}, shared.UsageError("--confirm is required")
	}

	localState := opts.LocalState
	if len(localState) == 0 {
		localState, err = loadMetadataKeywordLocalState(dirValue, versionValue)
		if err != nil {
			return MetadataKeywordsPlanResult{}, err
		}
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return MetadataKeywordsPlanResult{}, fmt.Errorf("auth: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	versionIDValue, _, err := resolveVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return MetadataKeywordsPlanResult{}, err
		}
		return MetadataKeywordsPlanResult{}, err
	}

	remoteVersionItems, err := fetchVersionLocalizations(ctx, client, versionIDValue)
	if err != nil {
		return MetadataKeywordsPlanResult{}, err
	}

	localPatches := keywordLocalStateToPatches(localState)
	filteredRemoteItems := filterKeywordRemoteItems(remoteVersionItems, localState)
	remoteVersion := remoteVersionItemsToVersionMap(filteredRemoteItems)
	adds, updates, _, versionCalls := buildScopePlan(
		versionDirName,
		versionValue,
		keywordPlanFields,
		versionToPlanFields(localPatches),
		versionToFieldMap(remoteVersion),
	)
	sortPlanItems(adds)
	sortPlanItems(updates)
	submitOpts := shared.SubmitReadinessOptions{}
	if versionCreateWarningsNeedUpdateContext(localPatches, remoteVersion) {
		readinessCtx, readinessCancel := shared.ContextWithTimeout(ctx)
		submitOpts = shared.ResolveSubmitReadinessOptionsForVersionBestEffort(readinessCtx, client, versionIDValue, resolvedAppID, platformValue)
		readinessCancel()
	}
	warnings := buildMetadataKeywordWarnings(localState, remoteVersion, submitOpts)

	result := MetadataKeywordsPlanResult{
		AppID:     resolvedAppID,
		Version:   versionValue,
		VersionID: versionIDValue,
		Dir:       dirValue,
		DryRun:    !opts.Apply || opts.DryRun,
		Adds:      adds,
		Updates:   updates,
		APICalls:  buildAPICallSummary(scopeCallCounts{}, versionCalls),
		Warnings:  warnings,
	}

	if !opts.Apply {
		return result, nil
	}

	applySummary := applyMetadataKeywordChanges(ctx, client, versionIDValue, versionValue, localPatches, filteredRemoteItems)
	result.DryRun = false
	result.Total = applySummary.Total
	result.Succeeded = applySummary.Succeeded
	result.Failed = applySummary.Failed
	result.Actions = applySummary.Actions
	if applySummary.Failed == 0 {
		result.Applied = true
		return result, nil
	}
	result.Results = applySummary.Results
	artifactPath, err := writeMetadataKeywordsApplyFailureArtifact(result, opts.FailureArtifactScope)
	if err != nil {
		return result, fmt.Errorf("write failure artifact: %w", err)
	}
	result.FailureArtifactPath = artifactPath
	return result, nil
}

func loadMetadataKeywordLocalState(dir, version string) (map[string]keywordLocalState, error) {
	resolvedVersion, err := validatePathSegment("version", version)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}
	versionPath := filepath.Join(strings.TrimSpace(dir), versionDirName, resolvedVersion)
	entries, err := os.ReadDir(versionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, shared.UsageErrorf("no version localization files found in %s", versionPath)
		}
		return nil, fmt.Errorf("failed to read %s: %w", versionPath, err)
	}

	states := make(map[string]keywordLocalState)
	seenLocales := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		rawLocale := strings.TrimSuffix(entry.Name(), ".json")
		if strings.EqualFold(rawLocale, DefaultLocale) {
			continue
		}
		locale, localeErr := validateMetadataKeywordLocale(rawLocale)
		if localeErr != nil {
			return nil, shared.UsageErrorf("invalid metadata keywords file %q: %v", entry.Name(), localeErr)
		}
		if err := recordCanonicalLocaleFile(seenLocales, locale, entry.Name()); err != nil {
			return nil, shared.UsageError(err.Error())
		}

		path := filepath.Join(versionPath, entry.Name())
		localization, readErr := ReadVersionLocalizationFile(path)
		if readErr != nil {
			return nil, shared.UsageErrorf("invalid metadata schema in %s: %v", path, readErr)
		}
		if strings.TrimSpace(localization.Keywords) == "" {
			continue
		}
		states[locale] = buildMetadataKeywordLocalState(locale, path, localization)
	}
	if len(states) == 0 {
		return nil, shared.UsageErrorf("no keyword metadata files with a non-empty keywords field were found in %s", versionPath)
	}
	return states, nil
}

func keywordLocalStateToPatches(states map[string]keywordLocalState) map[string]versionLocalPatch {
	result := make(map[string]versionLocalPatch, len(states))
	for locale, state := range states {
		result[locale] = cloneVersionLocalPatch(state.patch)
	}
	return result
}

func filterKeywordRemoteItems(items []asc.Resource[asc.AppStoreVersionLocalizationAttributes], states map[string]keywordLocalState) []asc.Resource[asc.AppStoreVersionLocalizationAttributes] {
	filtered := make([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], 0, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		if _, ok := states[locale]; !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func remoteVersionItemsToVersionMap(items []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) map[string]VersionLocalization {
	result := make(map[string]VersionLocalization, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		result[locale] = NormalizeVersionLocalization(VersionLocalization{
			Description:     item.Attributes.Description,
			Keywords:        item.Attributes.Keywords,
			MarketingURL:    item.Attributes.MarketingURL,
			PromotionalText: item.Attributes.PromotionalText,
			SupportURL:      item.Attributes.SupportURL,
			WhatsNew:        item.Attributes.WhatsNew,
		})
	}
	return result
}

func buildMetadataKeywordWarnings(states map[string]keywordLocalState, remote map[string]VersionLocalization, submitOpts shared.SubmitReadinessOptions) []MetadataKeywordsWarning {
	patches := keywordLocalStateToPatches(states)
	createWarnings := versionCreateWarningsForPatches(patches, remote, shared.SubmitReadinessCreateModePlanned, submitOpts)
	warnings := make([]MetadataKeywordsWarning, 0, len(createWarnings))
	for _, warning := range createWarnings {
		warnings = append(warnings, MetadataKeywordsWarning{
			Action:        "create",
			Locale:        warning.Locale,
			Message:       shared.FormatSubmitReadinessCreateWarning(warning),
			MissingFields: append([]string(nil), warning.MissingFields...),
		})
	}
	return warnings
}

func printMetadataKeywordsPlanTable(result MetadataKeywordsPlanResult) error {
	fmt.Printf("App ID: %s\n", result.AppID)
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Dry Run: %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Printf("Applied: %t\n\n", result.Applied)
	}
	if result.Total > 0 {
		fmt.Printf("Succeeded: %d\n", result.Succeeded)
		fmt.Printf("Failed: %d\n", result.Failed)
		if result.FailureArtifactPath != "" {
			fmt.Printf("Failure Artifact: %s\n", result.FailureArtifactPath)
		}
		fmt.Println()
	}

	pushResult := PushPlanResult{
		AppID:    result.AppID,
		Version:  result.Version,
		Dir:      result.Dir,
		DryRun:   result.DryRun,
		Applied:  result.Applied,
		Adds:     result.Adds,
		Updates:  result.Updates,
		APICalls: result.APICalls,
		Actions:  result.Actions,
	}
	asc.RenderTable([]string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}, buildPlanRows(pushResult))
	if len(result.APICalls) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Results) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"locale", "action", "status", "localizationId", "error"}, buildMetadataKeywordMutationRows(result.Results))
	} else if len(result.Actions) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"scope", "locale", "version", "action", "localizationId"}, buildApplyActionRows(result.Actions))
	}
	if len(result.Warnings) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"action", "locale", "message", "missingFields"}, buildMetadataKeywordWarningRows(result.Warnings))
	}
	return nil
}

func printMetadataKeywordsPlanMarkdown(result MetadataKeywordsPlanResult) error {
	fmt.Printf("**App ID:** %s\n\n", result.AppID)
	fmt.Printf("**Version:** %s\n\n", result.Version)
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Dry Run:** %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Printf("**Applied:** %t\n\n", result.Applied)
	}
	if result.Total > 0 {
		fmt.Printf("**Succeeded:** %d\n\n", result.Succeeded)
		fmt.Printf("**Failed:** %d\n\n", result.Failed)
		if result.FailureArtifactPath != "" {
			fmt.Printf("**Failure Artifact:** %s\n\n", result.FailureArtifactPath)
		}
	}

	pushResult := PushPlanResult{
		AppID:    result.AppID,
		Version:  result.Version,
		Dir:      result.Dir,
		DryRun:   result.DryRun,
		Applied:  result.Applied,
		Adds:     result.Adds,
		Updates:  result.Updates,
		APICalls: result.APICalls,
		Actions:  result.Actions,
	}
	asc.RenderMarkdown([]string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}, buildPlanRows(pushResult))
	if len(result.APICalls) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Results) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"locale", "action", "status", "localizationId", "error"}, buildMetadataKeywordMutationRows(result.Results))
	} else if len(result.Actions) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"scope", "locale", "version", "action", "localizationId"}, buildApplyActionRows(result.Actions))
	}
	if len(result.Warnings) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"action", "locale", "message", "missingFields"}, buildMetadataKeywordWarningRows(result.Warnings))
	}
	return nil
}

func buildMetadataKeywordWarningRows(warnings []MetadataKeywordsWarning) [][]string {
	rows := make([][]string, 0, len(warnings))
	for _, warning := range warnings {
		rows = append(rows, []string{
			warning.Action,
			warning.Locale,
			warning.Message,
			strings.Join(warning.MissingFields, ","),
		})
	}
	return rows
}
