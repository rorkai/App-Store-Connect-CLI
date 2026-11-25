package localizations

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// LocalizationsUpdateCommand returns the update localizations subcommand.
func newLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization resource ID (skips parent and locale lookup)")
	shared.BindFlagAlias(fs, "localization-id", "id")
	versionID := shared.BindResourceIDFlag(fs, "version", "appStoreVersions", "App Store version ID, or a version string with --app (for version localizations)")
	shared.BindFlagAlias(fs, "version-id", "version")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Platform for version strings: IOS, MAC_OS, TV_OS, VISION_OS")
	appInfoID := shared.BindResourceIDFlag(fs, "app-info", "appInfos", "App Info ID (optional override)")
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

	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc localizations update [flags]",
		ShortHelp:  "Update localization fields directly.",
		LongHelp: `Update localization fields directly without file preparation.

Use --id to update a known localization resource directly. Otherwise, identify
the parent version or app and pass the exact locale already stored in App Store
Connect.

For version localizations, --version accepts a version ID, or a version string
such as 1.2.3 with --app (or ASC_APP_ID). Use --platform when the string exists
on multiple platforms. With a version ID, the app selected by --app,
ASC_APP_ID, or the CLI config must own that version before updating.

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			localizationIDValue := strings.TrimSpace(*localizationID)
			if localizationIDValue != "" && (strings.TrimSpace(*versionID) != "" || strings.TrimSpace(*appID) != "" || strings.TrimSpace(*appInfoID) != "" || strings.TrimSpace(*locale) != "" || flagProvided(fs, "platform")) {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id cannot be combined with --version, --app, --app-info, --locale, or --platform")
				return flag.ErrHelp
			}
			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return d.UsageErrorf("localizations update: %v", err)
			}
			if flagProvided(fs, "platform") && strings.TrimSpace(*platform) == "" {
				return d.UsageError("--platform must not be empty")
			}
			versionFields, appInfoFields := activeProvidedFlags(fs, versionFieldNames...), activeProvidedFlags(fs, appInfoFieldNames...)
			if len(versionFields) > 0 && len(appInfoFields) > 0 {
				conflicting := appInfoFields[0]
				if normalizedType == shared.LocalizationTypeAppInfo {
					conflicting = versionFields[0]
				}
				return shared.WithDiagnostic(
					d.UsageErrorf(
						"localizations update: version fields (%s) and app-info fields (%s) cannot be combined in one command; update version fields with --type version and app-info fields with --type app-info in separate commands",
						strings.Join(versionFields, ", "),
						strings.Join(appInfoFields, ", "),
					),
					shared.DiagnosticConflictingInput,
					conflicting,
				)
			}

			if normalizedType == shared.LocalizationTypeAppInfo && flagProvided(fs, "platform") {
				return d.UsageError("--platform requires --type version")
			}
			if normalizedType == shared.LocalizationTypeAppInfo && flagProvided(fs, "version") {
				return d.UsageError("--version requires --type version")
			}
			if normalizedType == shared.LocalizationTypeVersion && flagProvided(fs, "app-info") {
				return d.UsageError("--app-info requires --type app-info")
			}
			if err := validateClearFlags(d, fs, normalizedType); err != nil {
				return err
			}
			if (*clearSubtitle || *clearPrivacyPolicyURL || *clearPromotionalText) && !*confirm {
				return d.UsageError("--confirm is required to clear localization fields")
			}

			localeValue := ""
			if localizationIDValue == "" {
				localeValue = strings.TrimSpace(*locale)
				if localeValue == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --locale is required when --id is not provided")
					return shared.MissingRequiredUsageError("--locale")
				}
				localeValue, err = shared.CanonicalizeAppStoreLocalizationLocale(localeValue)
				if err != nil {
					return d.UsageError(err.Error())
				}
			}

			switch normalizedType {
			case shared.LocalizationTypeAppInfo:
				params := updateAppInfoParams{
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
				}
				if len(versionFields) > 0 {
					return shared.WithDiagnostic(
						d.UsageError("--type version is required for version update flags"),
						shared.DiagnosticRequiredInputMissing,
						"--type",
					)
				}
				if !hasAnyAppInfoField(params) {
					return d.UpdateFlagRequired(fs, name, subtitle, clearSubtitle, privacyPolicyURL, clearPrivacyPolicyURL, privacyChoicesURL, privacyPolicyText)
				}
				return updateAppInfoLocalization(ctx, d, params)
			case shared.LocalizationTypeVersion:
				params := updateVersionParams{
					localizationID:       localizationIDValue,
					versionID:            *versionID,
					appID:                *appID,
					platform:             *platform,
					locale:               localeValue,
					description:          *description,
					keywords:             *keywords,
					whatsNew:             *whatsNew,
					promotionalText:      *promotionalText,
					supportURL:           *supportURL,
					marketingURL:         *marketingURL,
					clearPromotionalText: *clearPromotionalText,
					output:               output,
				}
				if len(appInfoFields) > 0 {
					return shared.WithDiagnostic(
						d.UsageError("--type app-info is required for app-info update flags"),
						shared.DiagnosticRequiredInputMissing,
						"--type",
					)
				}
				if !hasAnyVersionField(params) {
					return d.UpdateFlagRequired(fs, description, keywords, whatsNew, promotionalText, clearPromotionalText, supportURL, marketingURL)
				}
				return updateVersionLocalization(ctx, d, params)
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

func validateClearFlags(d *shared.Deps, fs *flag.FlagSet, normalizedType string) error {
	for _, rule := range clearFlagRules {
		if !flagProvided(fs, rule.clearFlag) || fs.Lookup(rule.clearFlag).Value.String() != "true" {
			continue
		}
		if normalizedType != rule.supportedLocType {
			return d.UsageErrorf(
				"localizations update: --%s requires --type %s",
				rule.clearFlag,
				rule.supportedTypeLabel,
			)
		}
		if flagProvided(fs, rule.setFlag) {
			return d.UsageErrorf(
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

var appInfoFieldNames = []string{
	"name",
	"subtitle",
	"privacy-policy-url",
	"privacy-choices-url",
	"privacy-policy-text",
	"clear-subtitle",
	"clear-privacy-policy-url",
}

var versionFieldNames = []string{
	"description",
	"keywords",
	"whats-new",
	"promotional-text",
	"clear-promotional-text",
	"support-url",
	"marketing-url",
}

func activeProvidedFlags(fs *flag.FlagSet, names ...string) []string {
	var active []string
	for _, name := range names {
		if !flagProvided(fs, name) {
			continue
		}
		if strings.HasPrefix(name, "clear-") && fs.Lookup(name).Value.String() != "true" {
			continue
		}
		active = append(active, "--"+name)
	}
	return active
}

type updateAppInfoParams struct {
	localizationID                                                         string
	appID, appInfoID, locale                                               string
	name, subtitle, privacyPolicyURL, privacyChoicesURL, privacyPolicyText string
	clearSubtitle, clearPrivacyPolicyURL                                   bool
	output                                                                 shared.OutputFlags
}

func updateAppInfoLocalization(ctx context.Context, d *shared.Deps, p updateAppInfoParams) error {
	localizationID := strings.TrimSpace(p.localizationID)
	appSelector := d.ResolveAppSelector(p.appID)
	resolvedAppID := appSelector.Value
	if localizationID == "" {
		if resolvedAppID == "" {
			fmt.Fprintln(d.IO.ErrOut, "Error: --app is required for app-info localizations (or set ASC_APP_ID)")
			return shared.MissingRequiredUsageError("--app")
		}
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	if resolvedAppID != "" {
		resolvedAppID, err = shared.ResolveScopedAppID(requestCtx, client, appSelector)
		if err != nil {
			return fmt.Errorf("localizations update: resolve app: %w", err)
		}
		requestCtx = asc.WithNoRedirectReads(requestCtx)
		if localizationID != "" {
			if err := verifyAppInfoLocalizationOwnership(requestCtx, client, localizationID, resolvedAppID); err != nil {
				return fmt.Errorf("localizations update: %w", err)
			}
		}
	}

	if localizationID == "" {
		appInfo, err := d.ResolveOwnedAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(p.appInfoID))
		if err != nil {
			return fmt.Errorf("localizations update: %w", err)
		}

		// Find existing localization ID for the locale
		existing, err := client.GetAppInfoLocalizations(requestCtx, appInfo, asc.Limit[asc.AppInfoLocalizationsOption](200))
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
			return fmt.Errorf("localizations update: no existing localization found for locale %q; create it with asc apps rename --app %q --app-info %q --locale %q --name \"APP_NAME\", then retry this update", p.locale, resolvedAppID, appInfo, p.locale)
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

	return d.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

type appInfoLocalizationOwnershipResponse struct {
	Data struct {
		Type          asc.ResourceType
		ID            string
		Relationships struct {
			AppInfo asc.Relationship
		}
	}
}

func verifyAppInfoLocalizationOwnership(ctx context.Context, client *asc.Client, localizationID, appID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("app info localization ID is required")
	}
	body, err := client.RawRequestWithQuery(
		ctx,
		"GET",
		fmt.Sprintf("/v1/appInfoLocalizations/%s", url.PathEscape(localizationID)),
		url.Values{
			"include":                      {"appInfo"},
			"fields[appInfoLocalizations]": {"appInfo"},
		},
		nil,
	)
	if err != nil {
		return err
	}
	var response appInfoLocalizationOwnershipResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decode app info localization %q: %w", localizationID, err)
	}
	if response.Data.Type != asc.ResourceTypeAppInfoLocalizations {
		return fmt.Errorf("expected resource type %q, got %q when reading app info localization %q", asc.ResourceTypeAppInfoLocalizations, response.Data.Type, localizationID)
	}
	if response.Data.ID != localizationID {
		return fmt.Errorf("expected app info localization %q, got %q", localizationID, response.Data.ID)
	}
	if response.Data.Relationships.AppInfo.Data.Type != asc.ResourceTypeAppInfos || strings.TrimSpace(response.Data.Relationships.AppInfo.Data.ID) == "" {
		return fmt.Errorf("app info relationship missing or invalid for app info localization %q", localizationID)
	}
	appInfoID := strings.TrimSpace(response.Data.Relationships.AppInfo.Data.ID)
	appInfo, err := client.GetAppInfo(ctx, appInfoID, asc.WithAppInfoInclude([]string{"app"}))
	if err != nil {
		return fmt.Errorf("read app info %q for localization %q: %w", appInfoID, localizationID, err)
	}
	if appInfo.Data.Type != asc.ResourceTypeAppInfos {
		return fmt.Errorf("expected app info resource type %q, got %q for app info %q", asc.ResourceTypeAppInfos, appInfo.Data.Type, appInfoID)
	}
	if appInfo.Data.ID != appInfoID {
		return fmt.Errorf("expected app info %q, got %q when reading localization %q", appInfoID, appInfo.Data.ID, localizationID)
	}
	ownerAppID, err := asc.AppInfoAppID(appInfo)
	if err != nil {
		return fmt.Errorf("resolve owner for app info localization %q: %w", localizationID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(ownerAppID), strings.TrimSpace(appID)) {
		return fmt.Errorf("app info localization %q belongs to app %q, not %q", localizationID, ownerAppID, appID)
	}
	return nil
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

	return client.UpdateAppInfoLocalization(ctx, localizationID, asc.AppInfoLocalizationUpdateAttributes{
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
	versionID, locale, appID, platform                                         string
	description, keywords, whatsNew, promotionalText, supportURL, marketingURL string
	clearPromotionalText                                                       bool
	output                                                                     shared.OutputFlags
}

func updateVersionLocalization(ctx context.Context, d *shared.Deps, p updateVersionParams) error {
	localizationID := strings.TrimSpace(p.localizationID)
	vid := strings.TrimSpace(p.versionID)
	if localizationID == "" && vid == "" {
		fmt.Fprintln(d.IO.ErrOut, "Error: --version is required for version localizations")
		return shared.MissingRequiredUsageError("--version")
	}
	normalizedPlatform, err := normalizeLocalizationVersionSelector(d, vid, p.appID, p.platform)
	if localizationID == "" && err != nil {
		return err
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
		return d.UsageError(err.Error())
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	if localizationID == "" {
		vid, err = ResolveVersionIDForMutation(requestCtx, d, client, vid, p.appID, normalizedPlatform)
		if err != nil {
			return fmt.Errorf("localizations update: %w", err)
		}
		// Find existing localization ID for the locale
		existing, err := client.GetAppStoreVersionLocalizations(asc.WithNoRedirectReads(requestCtx), vid, asc.Limit[asc.AppStoreVersionLocalizationsOption](200))
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
	} else if err := verifyVersionLocalizationOwner(requestCtx, d, client, localizationID); err != nil {
		return fmt.Errorf("localizations update: %w", err)
	}

	resp, err := patchVersionLocalization(requestCtx, client, localizationID, p, attrs)
	if err != nil {
		selector := p.locale
		if selector == "" {
			selector = localizationID
		}
		fields := versionAttemptedFields(p)
		return withVersionLocalizationConflictFacts(ctx, d, client, localizationID, vid, p.locale, fields, fmt.Errorf(
			"localizations update: update version localization %q via PATCH /v1/appStoreVersionLocalizations/%s (fields: %s): %w",
			selector,
			localizationID,
			formatAttemptedFields(fields),
			err,
		))
	}

	return d.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
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

	return client.UpdateAppStoreVersionLocalization(ctx, localizationID, attrs.UpdateAttributes())
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
