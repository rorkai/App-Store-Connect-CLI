package localizations

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// LocalizationsUpdateCommand returns the update localizations subcommand.
func LocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID (for version localizations)")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID, for app-info localizations)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locType := fs.String("type", shared.LocalizationTypeVersion, "Localization type: version (default) or app-info")
	locale := fs.String("locale", "", "Locale to update (required, e.g., en-US)")

	// App-info fields
	name := fs.String("name", "", "App name (app-info)")
	subtitle := fs.String("subtitle", "", "App subtitle (app-info)")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL (app-info)")
	privacyChoicesURL := fs.String("privacy-choices-url", "", "Privacy choices URL (app-info)")
	privacyPolicyText := fs.String("privacy-policy-text", "", "Privacy policy text (app-info)")

	// Version fields
	description := fs.String("description", "", "App description (version)")
	keywords := fs.String("keywords", "", "Search keywords (version)")
	whatsNew := fs.String("whats-new", "", "What's new text (version)")
	promotionalText := fs.String("promotional-text", "", "Promotional text (version)")
	supportURL := fs.String("support-url", "", "Support URL (version)")
	marketingURL := fs.String("marketing-url", "", "Marketing URL (version)")

	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc localizations update [flags]",
		ShortHelp:  "Update localization fields directly.",
		LongHelp: `Update localization fields directly without file preparation.

For app-info localizations (name, subtitle, privacy URLs):
  asc localizations update --app "APP_ID" --type app-info --locale "en-US" --subtitle "My App"

For version localizations (description, keywords, whatsNew):
  asc localizations update --version "VERSION_ID" --locale "en-US" --description "Updated description"

At least one field flag must be provided.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return fmt.Errorf("localizations update: %w", err)
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			switch normalizedType {
			case shared.LocalizationTypeAppInfo:
				return updateAppInfoLocalization(ctx, updateAppInfoParams{
					appID:             *appID,
					appInfoID:         *appInfoID,
					locale:            localeValue,
					name:              *name,
					subtitle:          *subtitle,
					privacyPolicyURL:  *privacyPolicyURL,
					privacyChoicesURL: *privacyChoicesURL,
					privacyPolicyText: *privacyPolicyText,
					output:            output,
				})
			case shared.LocalizationTypeVersion:
				return updateVersionLocalization(ctx, updateVersionParams{
					versionID:       *versionID,
					locale:          localeValue,
					description:     *description,
					keywords:        *keywords,
					whatsNew:        *whatsNew,
					promotionalText: *promotionalText,
					supportURL:      *supportURL,
					marketingURL:    *marketingURL,
					output:          output,
				})
			default:
				return fmt.Errorf("localizations update: unsupported type %q", normalizedType)
			}
		},
	}
}

type updateAppInfoParams struct {
	appID, appInfoID, locale                                               string
	name, subtitle, privacyPolicyURL, privacyChoicesURL, privacyPolicyText string
	output                                                                 shared.OutputFlags
}

func updateAppInfoLocalization(ctx context.Context, p updateAppInfoParams) error {
	if !hasAnyAppInfoField(p) {
		fmt.Fprintln(os.Stderr, "Error: at least one app-info field is required (--name, --subtitle, --privacy-policy-url, --privacy-choices-url, --privacy-policy-text)")
		return flag.ErrHelp
	}

	resolvedAppID := shared.ResolveAppID(p.appID)
	if resolvedAppID == "" {
		fmt.Fprintln(os.Stderr, "Error: --app is required for app-info localizations (or set ASC_APP_ID)")
		return flag.ErrHelp
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	appInfo, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(p.appInfoID))
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	// Find existing localization ID for the locale
	existing, err := client.GetAppInfoLocalizations(requestCtx, appInfo, asc.WithAppInfoLocalizationsLimit(200))
	if err != nil {
		return fmt.Errorf("localizations update: failed to fetch localizations: %w", err)
	}

	var localizationID string
	for _, item := range existing.Data {
		if strings.EqualFold(strings.TrimSpace(item.Attributes.Locale), p.locale) {
			localizationID = item.ID
			break
		}
	}
	if localizationID == "" {
		return fmt.Errorf("localizations update: no existing localization found for locale %q", p.locale)
	}

	attrs := asc.AppInfoLocalizationAttributes{
		Name:              p.name,
		Subtitle:          p.subtitle,
		PrivacyPolicyURL:  p.privacyPolicyURL,
		PrivacyChoicesURL: p.privacyChoicesURL,
		PrivacyPolicyText: p.privacyPolicyText,
	}

	resp, err := client.UpdateAppInfoLocalization(requestCtx, localizationID, attrs)
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	return shared.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

func hasAnyAppInfoField(p updateAppInfoParams) bool {
	return p.name != "" || p.subtitle != "" || p.privacyPolicyURL != "" || p.privacyChoicesURL != "" || p.privacyPolicyText != ""
}

type updateVersionParams struct {
	versionID, locale                                                          string
	description, keywords, whatsNew, promotionalText, supportURL, marketingURL string
	output                                                                     shared.OutputFlags
}

func updateVersionLocalization(ctx context.Context, p updateVersionParams) error {
	if !hasAnyVersionField(p) {
		fmt.Fprintln(os.Stderr, "Error: at least one version field is required (--description, --keywords, --whats-new, --promotional-text, --support-url, --marketing-url)")
		return flag.ErrHelp
	}

	vid := strings.TrimSpace(p.versionID)
	if vid == "" {
		fmt.Fprintln(os.Stderr, "Error: --version is required for version localizations")
		return flag.ErrHelp
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	// Find existing localization ID for the locale
	existing, err := client.GetAppStoreVersionLocalizations(requestCtx, vid, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return fmt.Errorf("localizations update: failed to fetch localizations: %w", err)
	}

	var localizationID string
	for _, item := range existing.Data {
		if strings.EqualFold(strings.TrimSpace(item.Attributes.Locale), p.locale) {
			localizationID = item.ID
			break
		}
	}
	if localizationID == "" {
		return fmt.Errorf("localizations update: no existing localization found for locale %q", p.locale)
	}

	attrs := asc.AppStoreVersionLocalizationAttributes{
		Description:     p.description,
		Keywords:        p.keywords,
		WhatsNew:        p.whatsNew,
		PromotionalText: p.promotionalText,
		SupportURL:      p.supportURL,
		MarketingURL:    p.marketingURL,
	}

	resp, err := client.UpdateAppStoreVersionLocalization(requestCtx, localizationID, attrs)
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	return shared.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

func hasAnyVersionField(p updateVersionParams) bool {
	return p.description != "" || p.keywords != "" || p.whatsNew != "" || p.promotionalText != "" || p.supportURL != "" || p.marketingURL != ""
}
