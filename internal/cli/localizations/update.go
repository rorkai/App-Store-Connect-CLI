package localizations

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
)

// LocalizationsUpdateCommand returns the update localizations subcommand.
func LocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization resource ID (skips parent and locale lookup)")
	versionID := fs.String("version", "", "App Store version ID (for version localizations)")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID, for app-info localizations)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locType := fs.String("type", shared.LocalizationTypeVersion, "Localization type: version (default) or app-info")
	locale := fs.String("locale", "", "Locale to update when resolving by parent (reuse exact ASC locale like en-US, ar-SA, zh-Hans)")

	// App-info fields
	name := fs.String("name", "", "App name (app-info)")
	subtitle := fs.String("subtitle", "", "App subtitle (app-info)")
	clearSubtitle := fs.Bool("clear-subtitle", false, "Clear the app subtitle (app-info)")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL (app-info)")
	clearPrivacyPolicyURL := fs.Bool("clear-privacy-policy-url", false, "Clear the privacy policy URL (app-info)")
	privacyChoicesURL := fs.String("privacy-choices-url", "", "Privacy choices URL (app-info)")
	privacyPolicyText := fs.String("privacy-policy-text", "", "Privacy policy text (app-info)")

	// Version fields
	description := fs.String("description", "", "App description (version)")
	keywords := fs.String("keywords", "", "Search keywords (version)")
	whatsNew := fs.String("whats-new", "", "What's new text (version)")
	promotionalText := fs.String("promotional-text", "", "Promotional text (version)")
	clearPromotionalText := fs.Bool("clear-promotional-text", false, "Clear the promotional text (version)")
	supportURL := fs.String("support-url", "", "Support URL (version)")
	marketingURL := fs.String("marketing-url", "", "Marketing URL (version)")
	confirm := fs.Bool("confirm", false, "Confirm clearing nullable localization fields")

	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc localizations update [flags]",
		ShortHelp:  "Update localization fields directly.",
		LongHelp: `Update localization fields directly without file preparation.

Use --id to update a known localization resource directly. Otherwise, identify
the parent version or app and pass the exact locale already stored in App Store
Connect.

Pass the exact locale value already stored in App Store Connect. Common accepted
forms include en-US, de-DE, ja, ar-SA, zh-Hans, and zh-Hant.

Common failures:
  "ar" is usually stored as "ar-SA"
  "de" is usually stored as "de-DE"
  "zh-Hans-CN" and "zh-Hant-TW" are usually stored as "zh-Hans" and "zh-Hant"

If a version locale is rejected or not found, run:
  asc localizations supported-locales --version "VERSION_ID"
  asc localizations list --version "VERSION_ID"

For app-info localizations, inspect configured locales with:
  asc localizations list --app "APP_ID" --type app-info

For app-info localizations (name, subtitle, privacy URLs):
  asc localizations update --type app-info --id "LOCALIZATION_ID" --subtitle "Arabic subtitle"
  asc localizations update --app "APP_ID" --type app-info --locale "ar-SA" --subtitle "Arabic subtitle"

For version localizations (description, keywords, whatsNew):
  asc localizations update --id "LOCALIZATION_ID" --description "Simplified Chinese description"
  asc localizations update --version "VERSION_ID" --locale "zh-Hans" --description "Simplified Chinese description"

To clear optional listing text that App Store Connect models as nullable:
  asc localizations update --type app-info --id "LOCALIZATION_ID" --clear-subtitle --confirm
  asc localizations update --app "APP_ID" --type app-info --locale "ar-SA" --clear-privacy-policy-url --confirm
  asc localizations update --version "VERSION_ID" --locale "zh-Hans" --clear-promotional-text --confirm

Each --clear-* flag rejects the matching set flag, and a clear flag alone is a
valid update with --confirm. An empty or omitted set flag never clears a field.

At least one field flag must be provided.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			localizationIDValue := strings.TrimSpace(*localizationID)
			if localizationIDValue != "" && (strings.TrimSpace(*versionID) != "" || strings.TrimSpace(*appID) != "" || strings.TrimSpace(*appInfoID) != "" || strings.TrimSpace(*locale) != "") {
				fmt.Fprintln(os.Stderr, "Error: --id cannot be combined with --version, --app, --app-info, or --locale")
				return flag.ErrHelp
			}
			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return fmt.Errorf("localizations update: %w", err)
			}
			if err := validateClearFlags(fs, normalizedType); err != nil {
				return err
			}
			if (*clearSubtitle || *clearPrivacyPolicyURL || *clearPromotionalText) && !*confirm {
				return shared.UsageError("--confirm is required to clear localization fields")
			}

			localeValue := ""
			if localizationIDValue == "" {
				localeValue = strings.TrimSpace(*locale)
				if localeValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --locale is required when --id is not provided")
					return shared.MissingRequiredUsageError("--locale")
				}
				localeValue, err = shared.CanonicalizeAppStoreLocalizationLocale(localeValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			switch normalizedType {
			case shared.LocalizationTypeAppInfo:
				return updateAppInfoLocalization(ctx, updateAppInfoParams{
					localizationID:        localizationIDValue,
					appID:                 *appID,
					appInfoID:             *appInfoID,
					locale:                localeValue,
					name:                  *name,
					subtitle:              *subtitle,
					privacyPolicyURL:      *privacyPolicyURL,
					privacyChoicesURL:     *privacyChoicesURL,
					privacyPolicyText:     *privacyPolicyText,
					clearSubtitle:         *clearSubtitle,
					clearPrivacyPolicyURL: *clearPrivacyPolicyURL,
					output:                output,
				})
			case shared.LocalizationTypeVersion:
				return updateVersionLocalization(ctx, updateVersionParams{
					localizationID:       localizationIDValue,
					versionID:            *versionID,
					locale:               localeValue,
					description:          *description,
					keywords:             *keywords,
					whatsNew:             *whatsNew,
					promotionalText:      *promotionalText,
					supportURL:           *supportURL,
					marketingURL:         *marketingURL,
					clearPromotionalText: *clearPromotionalText,
					output:               output,
				})
			default:
				return fmt.Errorf("localizations update: unsupported type %q", normalizedType)
			}
		},
	}
}

// clearFlagRules pairs each clear flag with the set flag it excludes and the
// localization type that supports it.
var clearFlagRules = []struct {
	clearFlag          string
	setFlag            string
	supportedLocType   string
	supportedTypeLabel string
}{
	{clearFlag: "clear-subtitle", setFlag: "subtitle", supportedLocType: shared.LocalizationTypeAppInfo, supportedTypeLabel: "app-info"},
	{clearFlag: "clear-privacy-policy-url", setFlag: "privacy-policy-url", supportedLocType: shared.LocalizationTypeAppInfo, supportedTypeLabel: "app-info"},
	{clearFlag: "clear-promotional-text", setFlag: "promotional-text", supportedLocType: shared.LocalizationTypeVersion, supportedTypeLabel: "version"},
}

func validateClearFlags(fs *flag.FlagSet, normalizedType string) error {
	for _, rule := range clearFlagRules {
		if !flagProvided(fs, rule.clearFlag) || fs.Lookup(rule.clearFlag).Value.String() != "true" {
			continue
		}
		if normalizedType != rule.supportedLocType {
			return shared.UsageErrorf(
				"localizations update: --%s requires --type %s",
				rule.clearFlag,
				rule.supportedTypeLabel,
			)
		}
		if flagProvided(fs, rule.setFlag) {
			return shared.UsageErrorf(
				"localizations update: --%s and --%s are mutually exclusive",
				rule.setFlag,
				rule.clearFlag,
			)
		}
	}
	return nil
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	if fs == nil {
		return false
	}
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

type updateAppInfoParams struct {
	localizationID                                                         string
	appID, appInfoID, locale                                               string
	name, subtitle, privacyPolicyURL, privacyChoicesURL, privacyPolicyText string
	clearSubtitle, clearPrivacyPolicyURL                                   bool
	output                                                                 shared.OutputFlags
}

func updateAppInfoLocalization(ctx context.Context, p updateAppInfoParams) error {
	if !hasAnyAppInfoField(p) {
		fmt.Fprintln(os.Stderr, "Error: at least one app-info field is required (--name, --subtitle, --privacy-policy-url, --privacy-choices-url, --privacy-policy-text, --clear-subtitle, --clear-privacy-policy-url)")
		return shared.MissingRequiredUsageError("")
	}

	localizationID := strings.TrimSpace(p.localizationID)
	resolvedAppID := ""
	if localizationID == "" {
		resolvedAppID = shared.ResolveAppID(p.appID)
		if resolvedAppID == "" {
			fmt.Fprintln(os.Stderr, "Error: --app is required for app-info localizations (or set ASC_APP_ID)")
			return shared.MissingRequiredUsageError("--app")
		}
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if localizationID == "" {
		appInfo, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(p.appInfoID))
		if err != nil {
			return fmt.Errorf("localizations update: %w", err)
		}

		// Find existing localization ID for the locale
		existing, err := client.GetAppInfoLocalizations(requestCtx, appInfo, asc.WithAppInfoLocalizationsLimit(200))
		if err != nil {
			return fmt.Errorf("localizations update: failed to fetch localizations: %w", err)
		}

		for _, item := range existing.Data {
			if strings.EqualFold(strings.TrimSpace(item.Attributes.Locale), p.locale) {
				localizationID = item.ID
				break
			}
		}
		if localizationID == "" {
			return fmt.Errorf("localizations update: no existing localization found for locale %q", p.locale)
		}
	}

	resp, err := patchAppInfoLocalization(requestCtx, client, localizationID, p)
	if err != nil {
		selector := p.locale
		if selector == "" {
			selector = localizationID
		}
		return fmt.Errorf(
			"localizations update: update app-info localization %q via PATCH /v1/appInfoLocalizations/%s (fields: %s): %w",
			selector,
			localizationID,
			formatAttemptedFields(appInfoAttemptedFields(p)),
			err,
		)
	}

	return shared.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

// patchAppInfoLocalization sends nullable attributes when the caller clears a
// field so the request encodes JSON null, and otherwise keeps the omitempty
// attribute payload used by set-only updates.
func patchAppInfoLocalization(ctx context.Context, client *asc.Client, localizationID string, p updateAppInfoParams) (*asc.AppInfoLocalizationResponse, error) {
	if p.clearSubtitle || p.clearPrivacyPolicyURL {
		fields := setLocalizationFields(map[string]string{
			"name":              p.name,
			"subtitle":          p.subtitle,
			"privacyPolicyUrl":  p.privacyPolicyURL,
			"privacyChoicesUrl": p.privacyChoicesURL,
			"privacyPolicyText": p.privacyPolicyText,
		})
		if p.clearSubtitle {
			fields["subtitle"] = asc.NullableString{}
		}
		if p.clearPrivacyPolicyURL {
			fields["privacyPolicyUrl"] = asc.NullableString{}
		}
		return client.UpdateAppInfoLocalizationNullableFields(ctx, localizationID, fields)
	}

	return client.UpdateAppInfoLocalization(ctx, localizationID, asc.AppInfoLocalizationAttributes{
		Name:              p.name,
		Subtitle:          p.subtitle,
		PrivacyPolicyURL:  p.privacyPolicyURL,
		PrivacyChoicesURL: p.privacyChoicesURL,
		PrivacyPolicyText: p.privacyPolicyText,
	})
}

// setLocalizationFields keeps the omitempty semantics of the attribute structs:
// only non-empty values reach the request payload.
func setLocalizationFields(values map[string]string) map[string]asc.NullableString {
	fields := make(map[string]asc.NullableString, len(values))
	for field, value := range values {
		if value == "" {
			continue
		}
		fields[field] = asc.NullableString{Value: &value}
	}
	return fields
}

func hasAnyAppInfoField(p updateAppInfoParams) bool {
	return p.name != "" || p.subtitle != "" || p.privacyPolicyURL != "" || p.privacyChoicesURL != "" ||
		p.privacyPolicyText != "" || p.clearSubtitle || p.clearPrivacyPolicyURL
}

type updateVersionParams struct {
	localizationID                                                             string
	versionID, locale                                                          string
	description, keywords, whatsNew, promotionalText, supportURL, marketingURL string
	clearPromotionalText                                                       bool
	output                                                                     shared.OutputFlags
}

func updateVersionLocalization(ctx context.Context, p updateVersionParams) error {
	if !hasAnyVersionField(p) {
		fmt.Fprintln(os.Stderr, "Error: at least one version field is required (--description, --keywords, --whats-new, --promotional-text, --support-url, --marketing-url, --clear-promotional-text)")
		return shared.MissingRequiredUsageError("")
	}

	localizationID := strings.TrimSpace(p.localizationID)
	vid := strings.TrimSpace(p.versionID)
	if localizationID == "" && vid == "" {
		fmt.Fprintln(os.Stderr, "Error: --version is required for version localizations")
		return shared.MissingRequiredUsageError("--version")
	}
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Description:     p.description,
		Keywords:        p.keywords,
		WhatsNew:        p.whatsNew,
		PromotionalText: p.promotionalText,
		SupportURL:      p.supportURL,
		MarketingURL:    p.marketingURL,
	}
	if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
		return shared.UsageError(err.Error())
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if localizationID == "" {
		// Find existing localization ID for the locale
		existing, err := client.GetAppStoreVersionLocalizations(requestCtx, vid, asc.WithAppStoreVersionLocalizationsLimit(200))
		if err != nil {
			return fmt.Errorf("localizations update: failed to fetch localizations: %w", err)
		}

		for _, item := range existing.Data {
			if strings.EqualFold(strings.TrimSpace(item.Attributes.Locale), p.locale) {
				localizationID = item.ID
				break
			}
		}
		if localizationID == "" {
			return fmt.Errorf("localizations update: no existing localization found for locale %q", p.locale)
		}
	}

	resp, err := patchVersionLocalization(requestCtx, client, localizationID, p, attrs)
	if err != nil {
		selector := p.locale
		if selector == "" {
			selector = localizationID
		}
		return fmt.Errorf(
			"localizations update: update version localization %q via PATCH /v1/appStoreVersionLocalizations/%s (fields: %s): %w",
			selector,
			localizationID,
			formatAttemptedFields(versionAttemptedFields(p)),
			err,
		)
	}

	return shared.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

// patchVersionLocalization sends nullable attributes when the caller clears a
// field so the request encodes JSON null, and otherwise keeps the omitempty
// attribute payload used by set-only updates.
func patchVersionLocalization(
	ctx context.Context,
	client *asc.Client,
	localizationID string,
	p updateVersionParams,
	attrs asc.AppStoreVersionLocalizationAttributes,
) (*asc.AppStoreVersionLocalizationResponse, error) {
	if p.clearPromotionalText {
		fields := setLocalizationFields(map[string]string{
			"description":  p.description,
			"keywords":     p.keywords,
			"whatsNew":     p.whatsNew,
			"supportUrl":   p.supportURL,
			"marketingUrl": p.marketingURL,
		})
		fields["promotionalText"] = asc.NullableString{}
		return client.UpdateAppStoreVersionLocalizationNullableFields(ctx, localizationID, fields)
	}

	return client.UpdateAppStoreVersionLocalization(ctx, localizationID, attrs)
}

func hasAnyVersionField(p updateVersionParams) bool {
	return p.description != "" || p.keywords != "" || p.whatsNew != "" || p.promotionalText != "" ||
		p.supportURL != "" || p.marketingURL != "" || p.clearPromotionalText
}

func appInfoAttemptedFields(p updateAppInfoParams) []string {
	fields := make([]string, 0, 5)
	if p.name != "" {
		fields = append(fields, "name")
	}
	if p.subtitle != "" || p.clearSubtitle {
		fields = append(fields, "subtitle")
	}
	if p.privacyPolicyURL != "" || p.clearPrivacyPolicyURL {
		fields = append(fields, "privacyPolicyUrl")
	}
	if p.privacyChoicesURL != "" {
		fields = append(fields, "privacyChoicesUrl")
	}
	if p.privacyPolicyText != "" {
		fields = append(fields, "privacyPolicyText")
	}
	return fields
}

func versionAttemptedFields(p updateVersionParams) []string {
	fields := make([]string, 0, 6)
	if p.description != "" {
		fields = append(fields, "description")
	}
	if p.keywords != "" {
		fields = append(fields, "keywords")
	}
	if p.marketingURL != "" {
		fields = append(fields, "marketingUrl")
	}
	if p.promotionalText != "" || p.clearPromotionalText {
		fields = append(fields, "promotionalText")
	}
	if p.supportURL != "" {
		fields = append(fields, "supportUrl")
	}
	if p.whatsNew != "" {
		fields = append(fields, "whatsNew")
	}
	return fields
}

func formatAttemptedFields(fields []string) string {
	if len(fields) == 0 {
		return "none"
	}
	values := append([]string(nil), fields...)
	sort.Strings(values)
	return strings.Join(values, ", ")
}
