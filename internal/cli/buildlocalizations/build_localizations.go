package buildlocalizations

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

// BuildLocalizationsCommand returns the build-localizations command group.
func BuildLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build-localizations",
		ShortUsage: "asc build-localizations <subcommand> [flags]",
		ShortHelp:  "Manage build release notes localizations.",
		LongHelp: `Manage localized release notes by build.

Examples:
  asc build-localizations list --build "BUILD_ID"
  asc build-localizations get --id "LOCALIZATION_ID"
  asc build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Bug fixes"
  asc build-localizations update --id "LOCALIZATION_ID" --whats-new "New features"
  asc build-localizations delete --id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildLocalizationsListCommand(),
			BuildLocalizationsGetCommand(),
			BuildLocalizationsCreateCommand(),
			BuildLocalizationsUpdateCommand(),
			BuildLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildLocalizationsListCommand returns the list subcommand.
func BuildLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc build-localizations list [flags]",
		ShortHelp:  "List release note localizations for a build.",
		LongHelp: `List release note localizations for a build.

Examples:
  asc build-localizations list --build "BUILD_ID"
  asc build-localizations list --build "BUILD_ID" --locale "en-US,ja"
  asc build-localizations list --build "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("build-localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("build-localizations list: %w", err)
			}

			build := strings.TrimSpace(*buildID)
			if build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("build-localizations list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			versionID, err := resolveBuildAppStoreVersion(requestCtx, client, build)
			if err != nil {
				return fmt.Errorf("build-localizations list: %w", err)
			}

			opts := []asc.AppStoreVersionLocalizationsOption{
				asc.WithAppStoreVersionLocalizationsLimit(*limit),
				asc.WithAppStoreVersionLocalizationsNextURL(*next),
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithAppStoreVersionLocalizationLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionLocalizationsLimit(200))
				firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, versionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("build-localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("build-localizations list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, versionID, opts...)
			if err != nil {
				return fmt.Errorf("build-localizations list: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildLocalizationsGetCommand returns the get subcommand.
func BuildLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc build-localizations get [flags]",
		ShortHelp:  "Get a localization by ID.",
		LongHelp: `Get a localization by ID.

Examples:
  asc build-localizations get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("build-localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("build-localizations get: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildLocalizationsCreateCommand returns the create subcommand.
func BuildLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "Release notes (whats new)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc build-localizations create [flags]",
		ShortHelp:  "Create a localization for a build.",
		LongHelp: `Create a localization for a build.

Examples:
  asc build-localizations create --build "BUILD_ID" --locale "en-US"
  asc build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Bug fixes"`,
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
				return fmt.Errorf("build-localizations create: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			versionID, err := resolveBuildAppStoreVersion(requestCtx, client, build)
			if err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			attrs := asc.AppStoreVersionLocalizationAttributes{
				Locale: localeValue,
			}
			if whatsNewValue != "" {
				attrs.WhatsNew = whatsNewValue
			}

			resp, err := client.CreateAppStoreVersionLocalization(requestCtx, versionID, attrs)
			if err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildLocalizationsUpdateCommand returns the update subcommand.
func BuildLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	whatsNew := fs.String("whats-new", "", "Release notes (whats new)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc build-localizations update [flags]",
		ShortHelp:  "Update a localization by ID.",
		LongHelp: `Update a localization by ID.

Examples:
  asc build-localizations update --id "LOCALIZATION_ID" --whats-new "New features"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppStoreVersionLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}

			resp, err := client.UpdateAppStoreVersionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("build-localizations update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildLocalizationsDeleteCommand returns the delete subcommand.
func BuildLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc build-localizations delete [flags]",
		ShortHelp:  "Delete a localization by ID.",
		LongHelp: `Delete a localization by ID.

Examples:
  asc build-localizations delete --id "LOCALIZATION_ID" --confirm`,
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
				return fmt.Errorf("build-localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreVersionLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("build-localizations delete: %w", err)
			}

			result := &asc.AppStoreVersionLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func resolveBuildAppStoreVersion(ctx context.Context, client *asc.Client, buildID string) (string, error) {
	resp, err := client.GetBuildAppStoreVersion(ctx, buildID)
	if err != nil {
		if asc.IsNotFound(err) {
			return "", fmt.Errorf("build %s has no associated App Store version", buildID)
		}
		return "", err
	}
	if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
		return "", fmt.Errorf("build %s has no associated App Store version", buildID)
	}
	return resp.Data.ID, nil
}
