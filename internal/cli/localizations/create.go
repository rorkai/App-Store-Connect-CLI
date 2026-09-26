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

// LocalizationsCreateCommand returns the create localizations subcommand.
func LocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID (required)")
	locale := fs.String("locale", "", "Locale code to create (required; use canonical ASC values like en-US, ja, ar-SA, zh-Hans)")
	description := fs.String("description", "", "App description")
	keywords := fs.String("keywords", "", "Search keywords")
	whatsNew := fs.String("whats-new", "", "What's new text")
	promotionalText := fs.String("promotional-text", "", "Promotional text")
	supportURL := fs.String("support-url", "", "Support URL")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc localizations create --version \"VERSION_ID\" --locale \"LOCALE\" [flags]",
		ShortHelp:  "Create a new locale for an app store version.",
		LongHelp: `Create a new locale for an app store version.

Use canonical App Store Connect locale identifiers when possible. Common accepted
forms include en-US, es-MX, de-DE, ja, ar-SA, zh-Hans, and zh-Hant.

To inspect the shared CLI locale catalog for a version, run:
  asc localizations supported-locales --version "VERSION_ID"

Common failures:
  "ar" is usually rejected; use "ar-SA"
  "de" should usually be "de-DE"
  use "zh-Hans" or "zh-Hant" instead of "zh-Hans-CN" or "zh-Hant-TW"

Examples:
  asc localizations create --version "VERSION_ID" --locale "ja"
  asc localizations create --version "VERSION_ID" --locale "ar-SA" --description "Arabic app" --keywords "arabic,productivity"
  asc localizations create --version "VERSION_ID" --locale "zh-Hans" --description "Simplified Chinese app" --keywords "simplified,chinese"
  asc localizations create --version "VERSION_ID" --locale "de-DE" --description "Meine App" --support-url "https://example.com/support"
  asc localizations create --version "VERSION_ID" --locale "ja" --description "日本語" --if-exists update

--if-exists controls what happens when App Store Connect answers 409 because
the locale already exists on that version. fail (default) returns the error.
skip reads the existing localization back, prints it unchanged, and exits 0.
update applies the same fields to the existing localization with
PATCH /v1/appStoreVersionLocalizations/{id}; when no metadata field was
supplied there is nothing to apply, so update behaves like skip. Any other
409 keeps failing.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("localizations create does not accept positional arguments")
			}

			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return shared.MissingRequiredUsageError("--version")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError("--locale")
			}
			normalizedLocale, err := shared.CanonicalizeAppStoreLocalizationLocale(localeValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			localeValue = normalizedLocale

			ifExistsMode, err := shared.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate)
			if err != nil {
				return err
			}

			attrs := asc.AppStoreVersionLocalizationAttributes{
				Locale:          localeValue,
				Description:     strings.TrimSpace(*description),
				Keywords:        strings.TrimSpace(*keywords),
				WhatsNew:        strings.TrimSpace(*whatsNew),
				PromotionalText: strings.TrimSpace(*promotionalText),
				SupportURL:      strings.TrimSpace(*supportURL),
				MarketingURL:    strings.TrimSpace(*marketingURL),
			}
			if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionLocalization(requestCtx, vid, attrs)
			created := err == nil
			if err != nil {
				existing, handled, resolveErr := shared.ResolveIfExistsConflict(ifExistsMode, err, localizationsCreateExistsCodes, func() (*asc.AppStoreVersionLocalizationResponse, bool, error) {
					return findExistingVersionLocalization(requestCtx, client, vid, localeValue)
				})
				if resolveErr != nil {
					return fmt.Errorf("localizations create: failed to create: %w", resolveErr)
				}
				if !handled {
					return fmt.Errorf("localizations create: failed to create: %w", err)
				}
				outcome := "left unchanged"
				if ifExistsMode == shared.IfExistsUpdate && hasUpdatableVersionLocalizationFields(attrs) {
					updated, updateErr := client.UpdateAppStoreVersionLocalization(requestCtx, existing.Data.ID, attrs)
					if updateErr != nil {
						return fmt.Errorf("localizations create: update existing localization %s: %w", existing.Data.ID, updateErr)
					}
					resp = updated
					outcome = "updated it in place"
				} else {
					// The collection item found by the read-back is not Apple's
					// single-resource envelope, so re-read the localization by ID
					// and print Apple's own response unmodified.
					detail, detailErr := client.GetAppStoreVersionLocalization(requestCtx, existing.Data.ID)
					if detailErr != nil {
						return fmt.Errorf("localizations create: read existing localization %s: %w", existing.Data.ID, detailErr)
					}
					resp = detail
				}
				fmt.Fprintf(os.Stderr, "localizations create: locale %s already exists on version %s as %s; %s (--if-exists %s)\n",
					localeValue, vid, existing.Data.ID, outcome, ifExistsMode)
			}

			// The create-readiness warning describes a locale that was just
			// created from these attributes. When --if-exists resolved a
			// duplicate, nothing was created and the existing localization may
			// already carry the omitted fields, so the warning would be wrong.
			submitOpts := shared.SubmitReadinessOptions{}
			var submitWarningLookupErr error
			if created && strings.TrimSpace(attrs.WhatsNew) == "" {
				submitOpts, submitWarningLookupErr = shared.ResolveSubmitReadinessOptionsForVersion(requestCtx, client, vid, "", "")
				if submitWarningLookupErr != nil {
					submitOpts = shared.SubmitReadinessOptions{}
				}
			}
			warnings := make([]shared.SubmitReadinessCreateWarning, 0, 1)
			if created {
				if warning, ok := shared.SubmitReadinessCreateWarningForLocaleWithOptions(localeValue, attrs, shared.SubmitReadinessCreateModeApplied, submitOpts); ok {
					warnings = append(warnings, warning)
				}
			}

			if err := shared.PrintOutput(resp, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if err := shared.PrintSubmitReadinessCreateWarnings(os.Stderr, warnings); err != nil {
				return err
			}
			if submitWarningLookupErr == nil || len(warnings) > 0 {
				return nil
			}

			localeLabel := strings.TrimSpace(resp.Data.Attributes.Locale)
			if localeLabel == "" {
				localeLabel = localeValue
			}
			if localeLabel == "" {
				localeLabel = "<unknown>"
			}
			_, err = fmt.Fprintf(
				os.Stderr,
				"Warning: locale %s was created without whatsNew, but the CLI could not determine whether this version is an app update: %v\n",
				localeLabel,
				submitWarningLookupErr,
			)
			return err
		},
	}
}

// localizationsCreateExistsCodes lists the Apple 409 codes that mean the locale
// already exists on POST /v1/appStoreVersionLocalizations. Apple answers the
// duplicate with ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE on the
// /data/attributes/locale pointer; the detail names the locale and suggests
// updating instead. STATE_ERROR.* (version not editable) is not an existence
// conflict and keeps failing.
var localizationsCreateExistsCodes = []string{"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}

// findExistingVersionLocalization reads back the localization a 409 conflict
// referred to, keyed by locale. It reports found=false when the version has no
// such locale so the caller can surface the original conflict.
func findExistingVersionLocalization(ctx context.Context, client *asc.Client, versionID, locale string) (*asc.AppStoreVersionLocalizationResponse, bool, error) {
	firstPage, err := client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return nil, false, err
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
	})
	if err != nil {
		return nil, false, err
	}
	localizations, ok := allPages.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return nil, false, fmt.Errorf("unexpected localizations response type: %T", allPages)
	}
	for _, candidate := range localizations.Data {
		if strings.EqualFold(strings.TrimSpace(candidate.Attributes.Locale), locale) {
			return &asc.AppStoreVersionLocalizationResponse{Data: candidate}, true, nil
		}
	}
	return nil, false, nil
}

// hasUpdatableVersionLocalizationFields reports whether the caller supplied any
// attribute the PATCH can carry. A locale-only create has nothing to update, so
// --if-exists update resolves it like skip instead of sending an empty PATCH
// that a non-editable localization could reject.
func hasUpdatableVersionLocalizationFields(attrs asc.AppStoreVersionLocalizationAttributes) bool {
	return strings.TrimSpace(attrs.Description) != "" ||
		strings.TrimSpace(attrs.Keywords) != "" ||
		strings.TrimSpace(attrs.WhatsNew) != "" ||
		strings.TrimSpace(attrs.PromotionalText) != "" ||
		strings.TrimSpace(attrs.SupportURL) != "" ||
		strings.TrimSpace(attrs.MarketingURL) != ""
}
