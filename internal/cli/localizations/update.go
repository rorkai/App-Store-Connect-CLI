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
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)

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
  asc localizations update --version "VERSION_ID" --locale "ja" --description "Updated" --if-exists skip

--if-exists controls a 409 whose code is ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE
("Entity with locale: ... already exists. Try updating."). fail (default)
returns that conflict unchanged. skip reads the localization back and exits 0
without applying fields. update sends the same field write again. Any other
409, including a state conflict, keeps failing.

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

			ifExistsMode, err := shared.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate)
			if err != nil {
				return err
			}

			switch normalizedType {
			case shared.LocalizationTypeAppInfo:
				return updateAppInfoLocalization(ctx, updateAppInfoParams{
					localizationID:    localizationIDValue,
					appID:             *appID,
					appInfoID:         *appInfoID,
					locale:            localeValue,
					name:              *name,
					subtitle:          *subtitle,
					privacyPolicyURL:  *privacyPolicyURL,
					privacyChoicesURL: *privacyChoicesURL,
					privacyPolicyText: *privacyPolicyText,
					output:            output,
					ifExists:          ifExistsMode,
				})
			case shared.LocalizationTypeVersion:
				return updateVersionLocalization(ctx, updateVersionParams{
					localizationID:  localizationIDValue,
					versionID:       *versionID,
					locale:          localeValue,
					description:     *description,
					keywords:        *keywords,
					whatsNew:        *whatsNew,
					promotionalText: *promotionalText,
					supportURL:      *supportURL,
					marketingURL:    *marketingURL,
					output:          output,
					ifExists:        ifExistsMode,
				})
			default:
				return fmt.Errorf("localizations update: unsupported type %q", normalizedType)
			}
		},
	}
}

type updateAppInfoParams struct {
	localizationID                                                         string
	appID, appInfoID, locale                                               string
	name, subtitle, privacyPolicyURL, privacyChoicesURL, privacyPolicyText string
	output                                                                 shared.OutputFlags
	ifExists                                                               shared.IfExistsMode
}

func updateAppInfoLocalization(ctx context.Context, p updateAppInfoParams) error {
	if !hasAnyAppInfoField(p) {
		fmt.Fprintln(os.Stderr, "Error: at least one app-info field is required (--name, --subtitle, --privacy-policy-url, --privacy-choices-url, --privacy-policy-text)")
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

	attrs := asc.AppInfoLocalizationAttributes{
		Name:              p.name,
		Subtitle:          p.subtitle,
		PrivacyPolicyURL:  p.privacyPolicyURL,
		PrivacyChoicesURL: p.privacyChoicesURL,
		PrivacyPolicyText: p.privacyPolicyText,
	}

	resp, err := client.UpdateAppInfoLocalization(requestCtx, localizationID, attrs)
	if err != nil {
		resolved, handled, resolveErr := resolveLocalizationUpdate(p.ifExists, err, func() (*asc.AppInfoLocalizationResponse, bool, error) {
			got, getErr := client.GetAppInfoLocalization(requestCtx, localizationID)
			if getErr != nil {
				if asc.IsNotFound(getErr) {
					return nil, false, nil
				}
				return nil, false, getErr
			}
			if got == nil || strings.TrimSpace(got.Data.ID) == "" {
				return nil, false, nil
			}
			return got, true, nil
		}, func() (*asc.AppInfoLocalizationResponse, error) {
			return client.UpdateAppInfoLocalization(requestCtx, localizationID, attrs)
		})
		if resolveErr != nil || !handled {
			if resolveErr != nil {
				err = resolveErr
			}
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
		resp = resolved
		fmt.Fprintf(os.Stderr, "localizations update: app-info localization %s already exists; %s (--if-exists %s)\n", resolved.Data.ID, localizationUpdateOutcome(p.ifExists), p.ifExists)
	}

	return shared.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

func hasAnyAppInfoField(p updateAppInfoParams) bool {
	return p.name != "" || p.subtitle != "" || p.privacyPolicyURL != "" || p.privacyChoicesURL != "" || p.privacyPolicyText != ""
}

type updateVersionParams struct {
	localizationID                                                             string
	versionID, locale                                                          string
	description, keywords, whatsNew, promotionalText, supportURL, marketingURL string
	output                                                                     shared.OutputFlags
	ifExists                                                                   shared.IfExistsMode
}

func updateVersionLocalization(ctx context.Context, p updateVersionParams) error {
	if !hasAnyVersionField(p) {
		fmt.Fprintln(os.Stderr, "Error: at least one version field is required (--description, --keywords, --whats-new, --promotional-text, --support-url, --marketing-url)")
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

	resp, err := client.UpdateAppStoreVersionLocalization(requestCtx, localizationID, attrs)
	if err != nil {
		resolved, handled, resolveErr := resolveLocalizationUpdate(p.ifExists, err, func() (*asc.AppStoreVersionLocalizationResponse, bool, error) {
			got, getErr := client.GetAppStoreVersionLocalization(requestCtx, localizationID)
			if getErr != nil {
				if asc.IsNotFound(getErr) {
					return nil, false, nil
				}
				return nil, false, getErr
			}
			if got == nil || strings.TrimSpace(got.Data.ID) == "" {
				return nil, false, nil
			}
			return got, true, nil
		}, func() (*asc.AppStoreVersionLocalizationResponse, error) {
			return client.UpdateAppStoreVersionLocalization(requestCtx, localizationID, attrs)
		})
		if resolveErr != nil || !handled {
			if resolveErr != nil {
				err = resolveErr
			}
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
		resp = resolved
		fmt.Fprintf(os.Stderr, "localizations update: version localization %s already exists; %s (--if-exists %s)\n", resolved.Data.ID, localizationUpdateOutcome(p.ifExists), p.ifExists)
	}

	return shared.PrintOutput(resp, *p.output.Output, *p.output.Pretty)
}

func hasAnyVersionField(p updateVersionParams) bool {
	return p.description != "" || p.keywords != "" || p.whatsNew != "" || p.promotionalText != "" || p.supportURL != "" || p.marketingURL != ""
}

func appInfoAttemptedFields(p updateAppInfoParams) []string {
	fields := make([]string, 0, 5)
	if p.name != "" {
		fields = append(fields, "name")
	}
	if p.subtitle != "" {
		fields = append(fields, "subtitle")
	}
	if p.privacyPolicyURL != "" {
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
	if p.promotionalText != "" {
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

// localizationUpdateExistsCodes is the Apple 409 code for a locale that already
// exists. The detail "Entity with locale: 'ja' already exists. Try updating."
// is the recorded duplicate-locale body; a state 409 is not in this list.
var localizationUpdateExistsCodes = []string{"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}

func localizationUpdateOutcome(mode shared.IfExistsMode) string {
	if mode == shared.IfExistsUpdate {
		return "updated"
	}
	return "left unchanged"
}

func resolveLocalizationUpdate[T any](mode shared.IfExistsMode, updateErr error, lookup func() (T, bool, error), retry func() (T, error)) (T, bool, error) {
	var zero T
	existing, handled, err := shared.ResolveIfExistsConflict(mode, updateErr, localizationUpdateExistsCodes, lookup)
	if err != nil || !handled {
		if err == nil {
			err = updateErr
		}
		return zero, false, err
	}
	if mode != shared.IfExistsUpdate {
		return existing, true, nil
	}
	updated, err := retry()
	if err != nil {
		return zero, false, err
	}
	return updated, true, nil
}
