package builds

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

// BuildsTestNotesCommand returns the builds test-notes command group.
func BuildsTestNotesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("test-notes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "test-notes",
		ShortUsage: "asc builds test-notes <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight What to Test notes.",
		LongHelp: `Manage TestFlight "What to Test" notes for a build.

Examples:
  asc builds test-notes list --build "BUILD_ID"
  asc builds test-notes get --id "LOCALIZATION_ID"
  asc builds test-notes create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes update --id "LOCALIZATION_ID" --whats-new "Updated instructions"
  asc builds test-notes delete --id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsTestNotesListCommand(),
			BuildsTestNotesGetCommand(),
			BuildsTestNotesCreateCommand(),
			BuildsTestNotesUpdateCommand(),
			BuildsTestNotesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsTestNotesListCommand returns the list subcommand.
func BuildsTestNotesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds test-notes list [flags]",
		ShortHelp:  "List What to Test notes for a build.",
		LongHelp: `List What to Test notes for a build.

Examples:
  asc builds test-notes list --build "BUILD_ID"
  asc builds test-notes list --build "BUILD_ID" --locale "en-US,ja"
  asc builds test-notes list --build "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds test-notes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			build := strings.TrimSpace(*buildID)
			if build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
				resp, err := shared.PaginateWithSpinner(requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaBuildLocalizations(ctx, build, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaBuildLocalizations(ctx, build, asc.WithBetaBuildLocalizationsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds test-notes list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaBuildLocalizations(requestCtx, build, opts...)
			if err != nil {
				return fmt.Errorf("builds test-notes list: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsTestNotesGetCommand returns the get subcommand.
func BuildsTestNotesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds test-notes get [flags]",
		ShortHelp:  "Get a What to Test note by ID.",
		LongHelp: `Get a What to Test note by ID.

Examples:
  asc builds test-notes get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaBuildLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("builds test-notes get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsTestNotesCreateCommand returns the create subcommand.
func BuildsTestNotesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc builds test-notes create [flags]",
		ShortHelp:  "Create What to Test notes for a build.",
		LongHelp: `Create What to Test notes for a build.

Examples:
  asc builds test-notes create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			build := strings.TrimSpace(*buildID)
			if build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --whats-new is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaBuildLocalizationAttributes{
				Locale:   localeValue,
				WhatsNew: whatsNewValue,
			}

			resp, err := client.CreateBetaBuildLocalization(requestCtx, build, attrs)
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsTestNotesUpdateCommand returns the update subcommand.
func BuildsTestNotesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	buildID := fs.String("build", "", "Build ID (alternative to --id, requires --locale)")
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with --build)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc builds test-notes update [flags]",
		ShortHelp:  "Update What to Test notes by ID or build+locale.",
		LongHelp: `Update What to Test notes by ID or by build+locale.

Examples:
  asc builds test-notes update --id "LOCALIZATION_ID" --whats-new "Updated notes"
  asc builds test-notes update --build "BUILD_ID" --locale "en-US" --whats-new "Updated notes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			buildValue := strings.TrimSpace(*buildID)
			localeValue := strings.TrimSpace(*locale)

			if id != "" && (buildValue != "" || localeValue != "") {
				fmt.Fprintln(os.Stderr, "Error: --id cannot be combined with --build or --locale")
				return flag.ErrHelp
			}
			if id == "" {
				if buildValue == "" || localeValue == "" {
					fmt.Fprintln(os.Stderr, "Error: either --id or (--build and --locale) is required")
					return flag.ErrHelp
				}
				if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
					return fmt.Errorf("builds test-notes update: %w", err)
				}
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if id == "" {
				localizations, err := client.GetBetaBuildLocalizations(
					requestCtx,
					buildValue,
					asc.WithBetaBuildLocalizationsLimit(200),
				)
				if err != nil {
					return fmt.Errorf("builds test-notes update: failed to resolve localization: %w", err)
				}
				matches := make([]asc.Resource[asc.BetaBuildLocalizationAttributes], 0, 1)
				if localizations != nil {
					for _, localization := range localizations.Data {
						if !strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), localeValue) {
							continue
						}
						matches = append(matches, localization)
					}
				}
				if len(matches) == 0 {
					return fmt.Errorf("builds test-notes update: no localization found for build %q and locale %q", buildValue, localeValue)
				}
				if len(matches) > 1 {
					return fmt.Errorf("builds test-notes update: multiple localizations found for build %q and locale %q; use --id", buildValue, localeValue)
				}
				id = strings.TrimSpace(matches[0].ID)
				if id == "" {
					return fmt.Errorf("builds test-notes update: resolved localization has empty ID")
				}
			}

			attrs := asc.BetaBuildLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}

			resp, err := client.UpdateBetaBuildLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("builds test-notes update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsTestNotesDeleteCommand returns the delete subcommand.
func BuildsTestNotesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc builds test-notes delete [flags]",
		ShortHelp:  "Delete What to Test notes by ID.",
		LongHelp: `Delete What to Test notes by ID.

Examples:
  asc builds test-notes delete --id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaBuildLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("builds test-notes delete: %w", err)
			}

			result := &asc.BetaBuildLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
