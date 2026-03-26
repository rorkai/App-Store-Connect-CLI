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
	locale := fs.String("locale", "", "Locale code to create (required, e.g., en-US, ja, es-MX)")
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

Examples:
  asc localizations create --version "VERSION_ID" --locale "ja"
  asc localizations create --version "VERSION_ID" --locale "es-MX" --description "Mi app" --keywords "palabra,clave"
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
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return shared.UsageError(err.Error())
			}

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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
