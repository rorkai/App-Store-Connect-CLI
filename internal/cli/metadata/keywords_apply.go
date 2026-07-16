package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MetadataKeywordsMutationResult describes one remote keyword mutation attempt.
type MetadataKeywordsMutationResult struct {
	Locale         string `json:"locale"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	LocalizationID string `json:"localizationId,omitempty"`
	Error          string `json:"error,omitempty"`
}

type metadataKeywordsApplyFailureArtifact struct {
	AppID       string                           `json:"appId"`
	Version     string                           `json:"version"`
	VersionID   string                           `json:"versionId"`
	Dir         string                           `json:"dir"`
	Total       int                              `json:"total"`
	Succeeded   int                              `json:"succeeded"`
	Failed      int                              `json:"failed"`
	GeneratedAt string                           `json:"generatedAt"`
	Results     []MetadataKeywordsMutationResult `json:"results"`
}

type metadataKeywordsApplySummary struct {
	Total     int
	Succeeded int
	Failed    int
	Actions   []ApplyAction
	Results   []MetadataKeywordsMutationResult
}

func shouldPrintMetadataKeywordsPlanResult(result MetadataKeywordsPlanResult, err error) bool {
	if err == nil {
		return true
	}
	return result.Total > 0 || result.Succeeded > 0 || result.Failed > 0 || len(result.Results) > 0 || result.FailureArtifactPath != ""
}

// MetadataKeywordsApplyCommand returns the keywords apply subcommand.
func MetadataKeywordsApplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords apply", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	confirm := fs.Bool("confirm", false, "Confirm remote keyword mutations")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc metadata keywords apply --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" --confirm [flags]",
		ShortHelp:  "Apply keyword-only metadata changes to App Store Connect.",
		LongHelp: `Apply keyword-only metadata changes to App Store Connect.

This command mutates only the version-localization ` + "`keywords`" + ` field.
Other version metadata fields remain untouched by updates performed here.

Examples:
  asc metadata keywords apply --app "APP_ID" --version "1.2.3" --dir "./metadata" --confirm
  asc metadata keywords apply --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords apply does not accept positional arguments")
			}
			result, err := executeMetadataKeywordsPlan(ctx, metadataKeywordsPlanOptions{
				AppID:                *appID,
				Version:              *version,
				Platform:             *platform,
				Dir:                  *dir,
				DryRun:               false,
				Apply:                true,
				Confirm:              *confirm,
				FailureArtifactScope: "metadata-keywords-apply",
			})
			if err != nil && errors.Is(err, flag.ErrHelp) {
				return err
			}
			if !shouldPrintMetadataKeywordsPlanResult(result, err) {
				return fmt.Errorf("metadata keywords apply: %w", err)
			}
			if printErr := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataKeywordsPlanTable(result) },
				func() error { return printMetadataKeywordsPlanMarkdown(result) },
			); printErr != nil {
				return printErr
			}
			if err != nil {
				return fmt.Errorf("metadata keywords apply: %w", err)
			}
			if result.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("metadata keywords apply: %d locale(s) failed", result.Failed))
			}
			return nil
		},
	}
}

func applyMetadataKeywordChanges(
	ctx context.Context,
	client *asc.Client,
	versionID string,
	version string,
	local map[string]versionLocalPatch,
	remoteItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
) metadataKeywordsApplySummary {
	remoteByLocale := make(map[string]remoteLocalizationState, len(remoteItems))
	for _, item := range remoteItems {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		remoteByLocale[locale] = remoteLocalizationState{
			id: item.ID,
			fields: versionFields(VersionLocalization{
				Description:     item.Attributes.Description,
				Keywords:        item.Attributes.Keywords,
				MarketingURL:    item.Attributes.MarketingURL,
				PromotionalText: item.Attributes.PromotionalText,
				SupportURL:      item.Attributes.SupportURL,
				WhatsNew:        item.Attributes.WhatsNew,
			}),
		}
	}

	locales := sortedLocaleUnion(local, remoteByLocale)
	summary := metadataKeywordsApplySummary{
		Actions: make([]ApplyAction, 0, len(locales)),
		Results: make([]MetadataKeywordsMutationResult, 0, len(locales)),
	}

	for _, locale := range locales {
		localPatch, localExists := local[locale]
		remoteState, remoteExists := remoteByLocale[locale]
		if !localExists {
			continue
		}

		remoteFields := cloneStringMap(remoteState.fields)
		adds, updates := countIntentChanges(keywordPlanFields, localPatch.setFields, remoteFields)
		if adds == 0 && updates == 0 {
			continue
		}

		result := MetadataKeywordsMutationResult{Locale: locale}
		summary.Total++

		switch {
		case !remoteExists:
			result.Action = "create"

			createLoc := localPatch.localization
			if hasVersionContent(localPatch.createLocalization) {
				createLoc = localPatch.createLocalization
			}

			createCtx, createCancel := shared.ContextWithTimeout(ctx)
			resp, createErr := client.CreateAppStoreVersionLocalization(createCtx, versionID, versionAttributes(locale, createLoc, true))
			createCancel()
			if createErr != nil {
				result.Status = "failed"
				result.Error = fmt.Sprintf(
					"create version localization %s (fields: %s): %v",
					locale,
					formatAttemptedFieldMap(keywordPlanFields, localPatch.setFields),
					createErr,
				)
				summary.Failed++
				summary.Results = append(summary.Results, result)
				continue
			}

			result.Status = "succeeded"
			result.LocalizationID = strings.TrimSpace(resp.Data.ID)
			summary.Succeeded++
			summary.Actions = append(summary.Actions, ApplyAction{
				Scope:          versionDirName,
				Locale:         locale,
				Version:        version,
				Action:         "create",
				LocalizationID: result.LocalizationID,
			})
		case remoteExists:
			result.Action = "update"

			updateCtx, updateCancel := shared.ContextWithTimeout(ctx)
			resp, updateErr := client.UpdateAppStoreVersionLocalization(updateCtx, remoteState.id, versionAttributes(locale, localPatch.localization, false))
			updateCancel()
			if updateErr != nil {
				result.Status = "failed"
				result.LocalizationID = remoteState.id
				result.Error = fmt.Sprintf(
					"update version localization %s (fields: %s): %v",
					locale,
					formatAttemptedFieldMap(keywordPlanFields, localPatch.setFields),
					updateErr,
				)
				summary.Failed++
				summary.Results = append(summary.Results, result)
				continue
			}

			result.Status = "succeeded"
			result.LocalizationID = strings.TrimSpace(resp.Data.ID)
			if result.LocalizationID == "" {
				result.LocalizationID = remoteState.id
			}
			summary.Succeeded++
			summary.Actions = append(summary.Actions, ApplyAction{
				Scope:          versionDirName,
				Locale:         locale,
				Version:        version,
				Action:         "update",
				LocalizationID: result.LocalizationID,
			})
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}

func writeMetadataKeywordsApplyFailureArtifact(result MetadataKeywordsPlanResult, scope string) (string, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "metadata-keywords-apply"
	}

	artifact := metadataKeywordsApplyFailureArtifact{
		AppID:       result.AppID,
		Version:     result.Version,
		VersionID:   result.VersionID,
		Dir:         result.Dir,
		Total:       result.Total,
		Succeeded:   result.Succeeded,
		Failed:      result.Failed,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Results:     append([]MetadataKeywordsMutationResult(nil), result.Results...),
	}

	data, err := encodeCanonicalJSON(artifact)
	if err != nil {
		return "", err
	}

	path := filepath.Join(
		".asc",
		"reports",
		scope,
		fmt.Sprintf("failures-%d.json", time.Now().UTC().UnixNano()),
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := writeFileNoFollow(path, data); err != nil {
		return "", err
	}
	return path, nil
}

func buildMetadataKeywordMutationRows(results []MetadataKeywordsMutationResult) [][]string {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Locale,
			result.Action,
			result.Status,
			result.LocalizationID,
			result.Error,
		})
	}
	return rows
}
