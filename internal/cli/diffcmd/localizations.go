package diffcmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type localizationDiffEndpoint struct {
	Kind      string `json:"kind"`
	Path      string `json:"path,omitempty"`
	VersionID string `json:"versionId,omitempty"`
}

type localizationDiffItem struct {
	Key    string `json:"key"`
	Locale string `json:"locale"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

type localizationDiffPlan struct {
	Scope     string                   `json:"scope"`
	AppID     string                   `json:"appId"`
	Direction string                   `json:"direction"`
	Source    localizationDiffEndpoint `json:"source"`
	Target    localizationDiffEndpoint `json:"target"`
	Adds      []localizationDiffItem   `json:"adds"`
	Updates   []localizationDiffItem   `json:"updates"`
	Deletes   []localizationDiffItem   `json:"deletes"`
}

// DiffLocalizationsCommand compares localization metadata between two sources.
func DiffLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	path := fs.String("path", "", "Local .strings directory or file (source)")
	fromVersion := shared.BindResourceIDFlag(fs, "from-version", "appStoreVersions", "Remote source app store version ID")
	version := shared.BindResourceIDFlag(fs, "version", "appStoreVersions", "Remote target app store version ID (when using --path)")
	toVersion := shared.BindResourceIDFlag(fs, "to-version", "appStoreVersions", "Remote target app store version ID (when using --from-version)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc diff localizations [flags]",
		ShortHelp:  "Diff localization metadata from local files or remote versions.",
		LongHelp: `Diff localization metadata from local files or remote versions.

Modes:
  Local vs remote:
    asc diff localizations --app "APP_ID" --path "./metadata/localizations" --version "VERSION_ID"

  Remote vs remote:
    asc diff localizations --app "APP_ID" --from-version "VERSION_ID_A" --to-version "VERSION_ID_B"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			sourcePath := strings.TrimSpace(*path)
			sourceVersion := strings.TrimSpace(*fromVersion)
			targetVersion := strings.TrimSpace(*version)
			targetToVersion := strings.TrimSpace(*toVersion)

			hasPath := sourcePath != ""
			hasFromVersion := sourceVersion != ""
			if hasPath && hasFromVersion {
				return shared.UsageError("--path and --from-version are mutually exclusive")
			}
			if !hasPath && !hasFromVersion {
				return shared.UsageError("either --path or --from-version is required")
			}

			var plan localizationDiffPlan
			var sourceValues map[string]map[string]string
			var targetValues map[string]map[string]string

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if hasPath {
				if targetVersion == "" {
					return shared.UsageError("--version is required when using --path")
				}
				if targetToVersion != "" {
					return shared.UsageError("--to-version cannot be used with --path; use --version")
				}

				localValues, err := readAndValidateLocalLocalizations(sourcePath)
				if err != nil {
					return shared.UsageError(err.Error())
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("diff localizations: %w", err)
				}

				if err := validateVersionBelongsToApp(requestCtx, client, targetVersion, resolvedAppID); err != nil {
					return err
				}

				remoteValues, err := fetchVersionLocalizations(requestCtx, client, targetVersion)
				if err != nil {
					return fmt.Errorf("diff localizations: %w", err)
				}

				sourceValues = localValues
				targetValues = remoteValues
				plan = buildLocalizationDiffPlan(
					resolvedAppID,
					localizationDiffEndpoint{Kind: "local", Path: sourcePath},
					localizationDiffEndpoint{Kind: "remote", VersionID: targetVersion},
					sourceValues,
					targetValues,
				)
			} else {
				if targetToVersion == "" {
					return shared.UsageError("--to-version is required when using --from-version")
				}
				if targetVersion != "" {
					return shared.UsageError("--version cannot be used with --from-version; use --to-version")
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("diff localizations: %w", err)
				}

				if err := validateVersionBelongsToApp(requestCtx, client, sourceVersion, resolvedAppID); err != nil {
					return err
				}
				if targetToVersion != sourceVersion {
					if err := validateVersionBelongsToApp(requestCtx, client, targetToVersion, resolvedAppID); err != nil {
						return err
					}
				}

				fromValues, err := fetchVersionLocalizations(requestCtx, client, sourceVersion)
				if err != nil {
					return fmt.Errorf("diff localizations: %w", err)
				}
				toValues, err := fetchVersionLocalizations(requestCtx, client, targetToVersion)
				if err != nil {
					return fmt.Errorf("diff localizations: %w", err)
				}

				sourceValues = fromValues
				targetValues = toValues
				plan = buildLocalizationDiffPlan(
					resolvedAppID,
					localizationDiffEndpoint{Kind: "remote", VersionID: sourceVersion},
					localizationDiffEndpoint{Kind: "remote", VersionID: targetToVersion},
					sourceValues,
					targetValues,
				)
			}

			return shared.PrintOutputWithRenderers(
				plan,
				*output.Output,
				*output.Pretty,
				func() error {
					renderLocalizationDiffTable(plan)
					return nil
				},
				func() error {
					renderLocalizationDiffMarkdown(plan)
					return nil
				},
			)
		},
	}
}

func readAndValidateLocalLocalizations(inputPath string) (map[string]map[string]string, error) {
	valuesByLocale, err := shared.ReadLocalizationStrings(inputPath, nil)
	if err != nil {
		return nil, err
	}

	normalized := make(map[string]map[string]string, len(valuesByLocale))
	for locale, values := range valuesByLocale {
		if err := shared.ValidateVersionLocalizationKeys(locale, values); err != nil {
			return nil, err
		}
		normalized[locale] = normalizeLocalizationValues(values)
	}

	return normalized, nil
}

func fetchVersionLocalizations(ctx context.Context, client *asc.Client, versionID string) (map[string]map[string]string, error) {
	firstPage, err := client.GetAppStoreVersionLocalizations(
		ctx,
		versionID,
		asc.Limit[asc.AppStoreVersionLocalizationsOption](200),
	)
	if err != nil {
		return nil, err
	}

	resp := firstPage
	if firstPage != nil && firstPage.Links.Next != "" {
		paginated, err := asc.PaginateAll(
			ctx,
			firstPage,
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetAppStoreVersionLocalizations(
					ctx,
					"",
					asc.NextURL[asc.AppStoreVersionLocalizationsOption](nextURL),
				)
			},
		)
		if err != nil {
			return nil, err
		}

		typed, ok := paginated.(*asc.AppStoreVersionLocalizationsResponse)
		if !ok {
			return nil, fmt.Errorf("unexpected pagination response type")
		}
		resp = typed
	}

	valuesByLocale := make(map[string]map[string]string)
	if resp == nil {
		return valuesByLocale, nil
	}

	for _, item := range resp.Data {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		if _, exists := valuesByLocale[locale]; exists {
			return nil, fmt.Errorf("duplicate locale %q in remote version %q", locale, versionID)
		}
		valuesByLocale[locale] = normalizeLocalizationValues(shared.MapVersionLocalizationStrings(item.Attributes))
	}

	return valuesByLocale, nil
}

func normalizeLocalizationValues(values map[string]string) map[string]string {
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized[key] = trimmed
	}
	return normalized
}

func validateVersionBelongsToApp(ctx context.Context, client *asc.Client, versionID, appID string) error {
	resolvedVersionID := strings.TrimSpace(versionID)
	expectedAppID := strings.TrimSpace(appID)

	actualAppID, err := resolveVersionAppID(ctx, client, resolvedVersionID)
	if err != nil {
		return fmt.Errorf("diff localizations: failed to resolve app for version %q: %w", resolvedVersionID, err)
	}
	if actualAppID == "" {
		return shared.UsageErrorf("could not determine owning app for version %q", resolvedVersionID)
	}
	if actualAppID != expectedAppID {
		return shared.UsageErrorf("version %q belongs to app %q, expected --app %q", resolvedVersionID, actualAppID, expectedAppID)
	}

	return nil
}

func resolveVersionAppID(ctx context.Context, client *asc.Client, versionID string) (string, error) {
	resp, err := client.GetAppStoreVersion(
		ctx,
		versionID,
		asc.WithAppStoreVersionInclude([]string{"app"}),
	)
	if err != nil {
		return "", err
	}
	return extractVersionAppID(resp)
}

func extractVersionAppID(resp *asc.AppStoreVersionResponse) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("empty app store version response")
	}

	if len(resp.Data.Relationships) > 0 && string(resp.Data.Relationships) != "null" {
		var relationships struct {
			App *struct {
				Data *asc.ResourceData `json:"data"`
			} `json:"app"`
		}
		if err := json.Unmarshal(resp.Data.Relationships, &relationships); err != nil {
			return "", fmt.Errorf("parse app store version relationships: %w", err)
		}
		if relationships.App != nil && relationships.App.Data != nil {
			if appID := strings.TrimSpace(relationships.App.Data.ID); appID != "" {
				return appID, nil
			}
		}
	}

	if len(resp.Included) > 0 && string(resp.Included) != "null" {
		var included []struct {
			Type asc.ResourceType `json:"type"`
			ID   string           `json:"id"`
		}
		if err := json.Unmarshal(resp.Included, &included); err != nil {
			return "", fmt.Errorf("parse included resources: %w", err)
		}
		for _, resource := range included {
			if resource.Type != asc.ResourceTypeApps {
				continue
			}
			if appID := strings.TrimSpace(resource.ID); appID != "" {
				return appID, nil
			}
		}
	}

	return "", nil
}

func buildLocalizationDiffPlan(
	appID string,
	source localizationDiffEndpoint,
	target localizationDiffEndpoint,
	sourceValues map[string]map[string]string,
	targetValues map[string]map[string]string,
) localizationDiffPlan {
	plan := localizationDiffPlan{
		Scope:     "localizations",
		AppID:     appID,
		Direction: "source-to-target",
		Source:    source,
		Target:    target,
		Adds:      make([]localizationDiffItem, 0),
		Updates:   make([]localizationDiffItem, 0),
		Deletes:   make([]localizationDiffItem, 0),
	}

	localesMap := make(map[string]struct{})
	for locale := range sourceValues {
		localesMap[locale] = struct{}{}
	}
	for locale := range targetValues {
		localesMap[locale] = struct{}{}
	}

	locales := make([]string, 0, len(localesMap))
	for locale := range localesMap {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	fields := shared.VersionLocalizationKeys()

	for _, locale := range locales {
		sourceFields := sourceValues[locale]
		targetFields := targetValues[locale]

		for _, field := range fields {
			sourceValue, sourceOK := sourceFields[field]
			targetValue, targetOK := targetFields[field]
			key := fmt.Sprintf("%s:%s", locale, field)

			switch {
			case !sourceOK && targetOK:
				plan.Adds = append(plan.Adds, localizationDiffItem{
					Key:    key,
					Locale: locale,
					Field:  field,
					Reason: "field exists in target but not in source",
					To:     targetValue,
				})
			case sourceOK && !targetOK:
				plan.Deletes = append(plan.Deletes, localizationDiffItem{
					Key:    key,
					Locale: locale,
					Field:  field,
					Reason: "field exists in source but not in target",
					From:   sourceValue,
				})
			case sourceOK && targetOK && sourceValue != targetValue:
				plan.Updates = append(plan.Updates, localizationDiffItem{
					Key:    key,
					Locale: locale,
					Field:  field,
					Reason: "field value differs",
					From:   sourceValue,
					To:     targetValue,
				})
			}
		}
	}

	return plan
}

func renderLocalizationDiffTable(plan localizationDiffPlan) {
	headers := []string{"change", "key", "locale", "field", "reason", "from", "to"}
	asc.RenderTable(headers, buildLocalizationDiffRows(plan))
}

func renderLocalizationDiffMarkdown(plan localizationDiffPlan) {
	headers := []string{"change", "key", "locale", "field", "reason", "from", "to"}
	asc.RenderMarkdown(headers, buildLocalizationDiffRows(plan))
}

func buildLocalizationDiffRows(plan localizationDiffPlan) [][]string {
	rows := make([][]string, 0, len(plan.Adds)+len(plan.Updates)+len(plan.Deletes))

	// Key and Locale can come from repository-controlled local files or remote
	// data, so they get the same cell sanitization as the values. Field and
	// Reason are internal constants.
	for _, item := range plan.Adds {
		rows = append(rows, []string{
			"add",
			sanitizeDiffCell(item.Key),
			sanitizeDiffCell(item.Locale),
			item.Field,
			item.Reason,
			"",
			sanitizeDiffCell(item.To),
		})
	}
	for _, item := range plan.Updates {
		rows = append(rows, []string{
			"update",
			sanitizeDiffCell(item.Key),
			sanitizeDiffCell(item.Locale),
			item.Field,
			item.Reason,
			sanitizeDiffCell(item.From),
			sanitizeDiffCell(item.To),
		})
	}
	for _, item := range plan.Deletes {
		rows = append(rows, []string{
			"delete",
			sanitizeDiffCell(item.Key),
			sanitizeDiffCell(item.Locale),
			item.Field,
			item.Reason,
			sanitizeDiffCell(item.From),
			"",
		})
	}

	if len(rows) == 0 {
		rows = append(rows, []string{"none", "", "", "", "no changes", "", ""})
	}
	return rows
}

// sanitizeDiffCell prepares a localization value for human table or Markdown
// output. Newlines become a visible escape, terminal control and bidi sequences
// are removed before truncation so a clipped cell cannot leave a partial escape
// behind, and long values are shortened on a rune boundary.
func sanitizeDiffCell(value string) string {
	normalized := asc.SanitizeTerminalText(strings.ReplaceAll(value, "\n", "\\n"))
	const maxLen = 80
	const suffix = "..."
	runes := []rune(normalized)
	if len(runes) <= maxLen {
		return normalized
	}
	suffixLen := len([]rune(suffix))
	if maxLen <= suffixLen {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-suffixLen]) + suffix
}
