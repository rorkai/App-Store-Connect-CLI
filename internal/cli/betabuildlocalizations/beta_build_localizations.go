package betabuildlocalizations

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

// BetaBuildLocalizationsCommand returns the beta-build-localizations command group.
func BetaBuildLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-build-localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-build-localizations",
		ShortUsage: "asc beta-build-localizations <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta build localizations.",
		LongHelp: `Manage TestFlight beta build localizations ("What to Test" notes).

Examples:
  asc beta-build-localizations list --build "BUILD_ID"
  asc beta-build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaBuildLocalizationsListCommand(),
			BetaBuildLocalizationsGetCommand(),
			BetaBuildLocalizationsCreateCommand(),
			BetaBuildLocalizationsUpdateCommand(),
			BetaBuildLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaBuildLocalizationsListCommand returns the list subcommand.
func BetaBuildLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc beta-build-localizations list [flags]",
		ShortHelp:  "List beta build localizations for a build.",
		LongHelp: `List beta build localizations for a build.

Examples:
  asc beta-build-localizations list --build "BUILD_ID"
  asc beta-build-localizations list --build "BUILD_ID" --locale "en-US,ja"
  asc beta-build-localizations list --build "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("beta-build-localizations list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("beta-build-localizations list: %w", err)
			}

			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			locales := splitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("beta-build-localizations list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaBuildLocalizationsOption{
				asc.WithBetaBuildLocalizationsLimit(*limit),
				asc.WithBetaBuildLocalizationsNextURL(*next),
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithBetaBuildLocalizationLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaBuildLocalizationsLimit(200))
				firstPage, err := client.GetBetaBuildLocalizations(requestCtx, buildValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("beta-build-localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaBuildLocalizations(ctx, buildValue, asc.WithBetaBuildLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-build-localizations list: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBetaBuildLocalizations(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("beta-build-localizations list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaBuildLocalizationsGetCommand returns the get subcommand.
func BetaBuildLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc beta-build-localizations get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a beta build localization by ID.",
		LongHelp: `Get a beta build localization by ID.

Examples:
  asc beta-build-localizations get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaBuildLocalization(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-build-localizations get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaBuildLocalizationsCreateCommand returns the create subcommand.
func BetaBuildLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc beta-build-localizations create [flags]",
		ShortHelp:  "Create a beta build localization.",
		LongHelp: `Create a beta build localization.

Examples:
  asc beta-build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return fmt.Errorf("beta-build-localizations create: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --whats-new is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaBuildLocalizationAttributes{
				Locale:   localeValue,
				WhatsNew: whatsNewValue,
			}

			resp, err := client.CreateBetaBuildLocalization(requestCtx, buildValue, attrs)
			if err != nil {
				return fmt.Errorf("beta-build-localizations create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaBuildLocalizationsUpdateCommand returns the update subcommand.
func BetaBuildLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc beta-build-localizations update [flags]",
		ShortHelp:  "Update a beta build localization.",
		LongHelp: `Update a beta build localization.

Examples:
  asc beta-build-localizations update --id "LOCALIZATION_ID" --whats-new "Updated notes"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaBuildLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}

			resp, err := client.UpdateBetaBuildLocalization(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("beta-build-localizations update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaBuildLocalizationsDeleteCommand returns the delete subcommand.
func BetaBuildLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc beta-build-localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a beta build localization.",
		LongHelp: `Delete a beta build localization.

Examples:
  asc beta-build-localizations delete --id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaBuildLocalization(requestCtx, idValue); err != nil {
				return fmt.Errorf("beta-build-localizations delete: failed to delete: %w", err)
			}

			result := &asc.BetaBuildLocalizationDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}
