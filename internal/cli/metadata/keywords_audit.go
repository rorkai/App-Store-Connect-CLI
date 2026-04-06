package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type keywordAuditTermsFlag []string

func (f *keywordAuditTermsFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *keywordAuditTermsFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// MetadataKeywordsAuditCommand returns the keywords audit subcommand.
func MetadataKeywordsAuditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords audit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override for apps with multiple app-infos)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	versionID := fs.String("version-id", "", "App Store version ID")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	blockedTermsFile := fs.String("blocked-terms-file", "", "Optional newline/comma-separated blocked terms file (supports # comments)")
	var blockedTerms keywordAuditTermsFlag
	fs.Var(&blockedTerms, "blocked-term", "Blocked term to flag during keyword audit (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "audit",
		ShortUsage: "asc metadata keywords audit --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\") [--app-info \"APP_INFO_ID\"] [flags]",
		ShortHelp:  "Audit live ASC keywords for ASO-quality issues.",
		LongHelp: `Audit live App Store Connect keyword metadata for ASO-quality issues.

This command fetches version localizations plus matching app-info localizations,
then reports:
  - duplicate phrases within a locale
  - repeated phrases across locales
  - byte budget usage and underfilled keyword fields
  - overlap with localized app name or subtitle
  - blocked terms from flags or a text file
  - malformed keyword separators / empty segments

Examples:
  asc metadata keywords audit --app "APP_ID" --version "1.2.3"
  asc metadata keywords audit --app "APP_ID" --app-info "APP_INFO_ID" --version "1.2.3"
  asc metadata keywords audit --app "APP_ID" --version-id "VERSION_ID" --strict
  asc metadata keywords audit --app "APP_ID" --version "1.2.3" --blocked-term "free"
  asc metadata keywords audit --app "APP_ID" --version "1.2.3" --blocked-terms-file "./blocked-terms.txt"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords audit does not accept positional arguments")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			versionValue := strings.TrimSpace(*version)
			versionIDValue := strings.TrimSpace(*versionID)
			if versionValue == "" && versionIDValue == "" {
				return shared.UsageError("--version or --version-id is required")
			}
			if versionValue != "" && versionIDValue != "" {
				return shared.UsageError("--version and --version-id are mutually exclusive")
			}

			platformValue := strings.TrimSpace(*platform)
			if platformValue != "" {
				normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				platformValue = normalizedPlatform
			}

			resolvedBlockedTerms, err := loadKeywordAuditBlockedTerms(blockedTerms, strings.TrimSpace(*blockedTermsFile))
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("metadata keywords audit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var versionStateValue string
			if versionIDValue != "" {
				versionResource, err := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, resolvedAppID, versionIDValue, platformValue)
				if err != nil {
					return fmt.Errorf("metadata keywords audit: %w", err)
				}
				versionIDValue = strings.TrimSpace(versionResource.ID)
				versionValue = strings.TrimSpace(versionResource.Attributes.VersionString)
				versionStateValue = asc.ResolveAppStoreVersionState(versionResource.Attributes)
				if platformValue == "" {
					platformValue = strings.TrimSpace(string(versionResource.Attributes.Platform))
				}
			} else {
				resolvedVersionID, resolvedVersionState, err := resolveVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
				if err != nil {
					return fmt.Errorf("metadata keywords audit: %w", err)
				}
				versionIDValue = resolvedVersionID
				versionStateValue = resolvedVersionState
			}

			appInfoIDValue, err := resolveMetadataKeywordsAuditAppInfoID(
				requestCtx,
				client,
				resolvedAppID,
				strings.TrimSpace(*appInfoID),
				versionValue,
				platformValue,
				versionStateValue,
			)
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords audit: %w", err)
			}

			versionItems, err := fetchVersionLocalizations(requestCtx, client, versionIDValue)
			if err != nil {
				return fmt.Errorf("metadata keywords audit: %w", err)
			}
			appInfoItems, err := fetchAppInfoLocalizations(requestCtx, client, appInfoIDValue)
			if err != nil {
				return fmt.Errorf("metadata keywords audit: %w", err)
			}

			report := validation.AuditKeywords(validation.KeywordAuditInput{
				AppID:                resolvedAppID,
				VersionID:            versionIDValue,
				VersionString:        versionValue,
				Platform:             platformValue,
				BlockedTerms:         resolvedBlockedTerms,
				VersionLocalizations: mapVersionLocalizationsForAudit(versionItems),
				AppInfoLocalizations: mapAppInfoLocalizationsForAudit(appInfoItems),
			}, *strict)

			if err := shared.PrintOutputWithRenderers(
				report,
				*output.Output,
				*output.Pretty,
				func() error { return printKeywordAuditTable(report) },
				func() error { return printKeywordAuditMarkdown(report) },
			); err != nil {
				return err
			}

			if report.Summary.Blocking > 0 {
				return shared.NewReportedError(fmt.Errorf("metadata keywords audit: found %d blocking issue(s)", report.Summary.Blocking))
			}
			return nil
		},
	}
}

func resolveMetadataKeywordsAuditAppInfoID(
	ctx context.Context,
	client *asc.Client,
	appID string,
	appInfoID string,
	version string,
	platform string,
	versionState string,
) (string, error) {
	return resolveMetadataAppInfoID(ctx, client, appID, appInfoID, version, platform, "", versionState, func(aid, v, p, _ string, infoID string) string {
		return buildMetadataKeywordsAuditAppInfoExample(aid, v, p, infoID)
	})
}

func buildMetadataKeywordsAuditAppInfoExample(appID, version, platform, appInfoID string) string {
	parts := []string{
		"asc metadata keywords audit",
		fmt.Sprintf(`--app %q`, appID),
		fmt.Sprintf(`--version %q`, version),
	}
	if platform != "" {
		parts = append(parts, fmt.Sprintf(`--platform %q`, platform))
	}
	if appInfoID != "" {
		parts = append(parts, fmt.Sprintf(`--app-info %q`, appInfoID))
	}
	return strings.Join(parts, " ")
}

func loadKeywordAuditBlockedTerms(flagValues []string, filePath string) ([]string, error) {
	terms := make([]string, 0, len(flagValues))
	for _, value := range flagValues {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if trimmed == "" {
			return nil, fmt.Errorf("--blocked-term must not be empty")
		}
		terms = append(terms, trimmed)
	}

	if filePath != "" {
		file, err := shared.OpenExistingNoFollow(filePath)
		if err != nil {
			return nil, fmt.Errorf("read --blocked-terms-file %q: %w", filePath, err)
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("read --blocked-terms-file %q: %w", filePath, err)
		}

		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
				continue
			}
			for _, part := range strings.Split(trimmedLine, ",") {
				trimmed := strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
				if trimmed == "" {
					continue
				}
				terms = append(terms, trimmed)
			}
		}
	}

	if filePath != "" && len(terms) == 0 {
		return nil, fmt.Errorf("--blocked-terms-file must include at least one blocked term")
	}
	return terms, nil
}

func mapVersionLocalizationsForAudit(items []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) []validation.VersionLocalization {
	result := make([]validation.VersionLocalization, 0, len(items))
	for _, item := range items {
		result = append(result, validation.VersionLocalization{
			ID:              item.ID,
			Locale:          item.Attributes.Locale,
			Description:     item.Attributes.Description,
			Keywords:        item.Attributes.Keywords,
			WhatsNew:        item.Attributes.WhatsNew,
			PromotionalText: item.Attributes.PromotionalText,
			SupportURL:      item.Attributes.SupportURL,
			MarketingURL:    item.Attributes.MarketingURL,
		})
	}
	return result
}

func mapAppInfoLocalizationsForAudit(items []asc.Resource[asc.AppInfoLocalizationAttributes]) []validation.AppInfoLocalization {
	result := make([]validation.AppInfoLocalization, 0, len(items))
	for _, item := range items {
		result = append(result, validation.AppInfoLocalization{
			ID:               item.ID,
			Locale:           item.Attributes.Locale,
			Name:             item.Attributes.Name,
			Subtitle:         item.Attributes.Subtitle,
			PrivacyPolicyURL: item.Attributes.PrivacyPolicyURL,
		})
	}
	return result
}

func printKeywordAuditTable(report validation.KeywordAuditReport) error {
	asc.RenderTable(
		[]string{"app", "version", "version id", "platform", "strict", "locales", "errors", "warnings", "infos", "blocking", "blocked terms"},
		[][]string{{
			report.AppID,
			report.VersionString,
			report.VersionID,
			report.Platform,
			fmt.Sprintf("%t", report.Strict),
			fmt.Sprintf("%d", len(report.Locales)),
			fmt.Sprintf("%d", report.Summary.Errors),
			fmt.Sprintf("%d", report.Summary.Warnings),
			fmt.Sprintf("%d", report.Summary.Infos),
			fmt.Sprintf("%d", report.Summary.Blocking),
			strings.Join(report.BlockedTerms, ","),
		}},
	)

	localeRows := make([][]string, 0, len(report.Locales))
	for _, locale := range report.Locales {
		localeRows = append(localeRows, []string{
			locale.Locale,
			fmt.Sprintf("%d", locale.KeywordCount),
			fmt.Sprintf("%d", locale.UsedBytes),
			fmt.Sprintf("%d", locale.RemainingBytes),
			fmt.Sprintf("%d", locale.Errors),
			fmt.Sprintf("%d", locale.Warnings),
			fmt.Sprintf("%d", locale.Infos),
			sanitizePlanCell(locale.KeywordField),
		})
	}
	if len(localeRows) == 0 {
		localeRows = append(localeRows, []string{"", "0", "0", "0", "0", "0", "0", ""})
	}
	fmt.Println()
	asc.RenderTable([]string{"locale", "count", "used bytes", "remaining", "errors", "warnings", "infos", "keywords"}, localeRows)

	checkRows := buildKeywordAuditCheckRows(report.Checks)
	fmt.Println()
	asc.RenderTable([]string{"severity", "locale", "related locales", "id", "keyword", "term", "message"}, checkRows)
	return nil
}

func printKeywordAuditMarkdown(report validation.KeywordAuditReport) error {
	asc.RenderMarkdown(
		[]string{"app", "version", "version id", "platform", "strict", "locales", "errors", "warnings", "infos", "blocking", "blocked terms"},
		[][]string{{
			report.AppID,
			report.VersionString,
			report.VersionID,
			report.Platform,
			fmt.Sprintf("%t", report.Strict),
			fmt.Sprintf("%d", len(report.Locales)),
			fmt.Sprintf("%d", report.Summary.Errors),
			fmt.Sprintf("%d", report.Summary.Warnings),
			fmt.Sprintf("%d", report.Summary.Infos),
			fmt.Sprintf("%d", report.Summary.Blocking),
			strings.Join(report.BlockedTerms, ","),
		}},
	)

	localeRows := make([][]string, 0, len(report.Locales))
	for _, locale := range report.Locales {
		localeRows = append(localeRows, []string{
			locale.Locale,
			fmt.Sprintf("%d", locale.KeywordCount),
			fmt.Sprintf("%d", locale.UsedBytes),
			fmt.Sprintf("%d", locale.RemainingBytes),
			fmt.Sprintf("%d", locale.Errors),
			fmt.Sprintf("%d", locale.Warnings),
			fmt.Sprintf("%d", locale.Infos),
			sanitizePlanCell(locale.KeywordField),
		})
	}
	if len(localeRows) == 0 {
		localeRows = append(localeRows, []string{"", "0", "0", "0", "0", "0", "0", ""})
	}
	fmt.Println()
	asc.RenderMarkdown([]string{"locale", "count", "used bytes", "remaining", "errors", "warnings", "infos", "keywords"}, localeRows)

	checkRows := buildKeywordAuditCheckRows(report.Checks)
	fmt.Println()
	asc.RenderMarkdown([]string{"severity", "locale", "related locales", "id", "keyword", "term", "message"}, checkRows)
	return nil
}

func buildKeywordAuditCheckRows(checks []validation.KeywordAuditCheck) [][]string {
	rows := make([][]string, 0, len(checks))
	for _, check := range checks {
		rows = append(rows, []string{
			string(check.Severity),
			check.Locale,
			strings.Join(check.RelatedLocales, ","),
			check.ID,
			check.Keyword,
			check.MatchedTerm,
			sanitizePlanCell(check.Message),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"", "", "", "", "", "", "no findings"})
	}
	return rows
}
