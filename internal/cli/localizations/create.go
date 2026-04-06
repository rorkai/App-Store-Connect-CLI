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
  asc localizations create --version "VERSION_ID" --locale "de-DE" --description "Meine App" --support-url "https://example.com/support"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("localizations create does not accept positional arguments")
			}

			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			normalizedLocale, err := shared.CanonicalizeAppStoreLocalizationLocale(localeValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			localeValue = normalizedLocale

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppStoreVersionLocalizationAttributes{
				Locale:          localeValue,
				Description:     strings.TrimSpace(*description),
				Keywords:        strings.TrimSpace(*keywords),
				WhatsNew:        strings.TrimSpace(*whatsNew),
				PromotionalText: strings.TrimSpace(*promotionalText),
				SupportURL:      strings.TrimSpace(*supportURL),
				MarketingURL:    strings.TrimSpace(*marketingURL),
			}

			resp, err := client.CreateAppStoreVersionLocalization(requestCtx, vid, attrs)
			if err != nil {
				return fmt.Errorf("localizations create: failed to create: %w", err)
			}

			submitOpts := shared.SubmitReadinessOptions{}
			var submitWarningLookupErr error
			if strings.TrimSpace(attrs.WhatsNew) == "" {
				submitOpts, submitWarningLookupErr = shared.ResolveSubmitReadinessOptionsForVersion(requestCtx, client, vid, "", "")
				if submitWarningLookupErr != nil {
					submitOpts = shared.SubmitReadinessOptions{}
				}
			}
			warnings := make([]shared.SubmitReadinessCreateWarning, 0, 1)
			if warning, ok := shared.SubmitReadinessCreateWarningForLocaleWithOptions(localeValue, attrs, shared.SubmitReadinessCreateModeApplied, submitOpts); ok {
				warnings = append(warnings, warning)
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
