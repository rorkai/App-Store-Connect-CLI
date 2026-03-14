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
		ShortHelp:  "DEPRECATED: use `asc builds test-notes ...`.",
		LongHelp: `Deprecated compatibility layer for TestFlight What to Test notes.

Canonical build-scoped workflows now live under ` + "`asc builds test-notes ...`" + `.

Legacy-only behaviors still remain here during the transition:
- ` + "`beta-build-localizations list --global`" + `
- ` + "`beta-build-localizations get --app ... --latest`" + `
- ` + "`beta-build-localizations create --app ... --latest`" + `
- ` + "`beta-build-localizations build get`" + `

Examples:
  asc builds test-notes list --build "BUILD_ID"
  asc builds test-notes view --id "LOCALIZATION_ID"
  asc builds test-notes create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaBuildLocalizationsLeafCommand(
				BetaBuildLocalizationsListCommand(),
				"asc builds test-notes list",
				"Warning: `asc beta-build-localizations list` is deprecated. Use `asc builds test-notes list` for build-scoped workflows. `--global` remains legacy-only during transition.",
			),
			deprecatedBetaBuildLocalizationsLeafCommand(
				BetaBuildLocalizationsGetCommand(),
				"asc builds test-notes view",
				"Warning: `asc beta-build-localizations get` is deprecated. Use `asc builds test-notes view` for ID-based lookups. `--latest` remains legacy-only during transition.",
			),
			BetaBuildLocalizationsBuildCommand(),
			deprecatedBetaBuildLocalizationsLeafCommand(
				BetaBuildLocalizationsCreateCommand(),
				"asc builds test-notes create",
				"Warning: `asc beta-build-localizations create` is deprecated. Use `asc builds test-notes create` for build-scoped workflows. `--latest` remains legacy-only during transition.",
			),
			deprecatedBetaBuildLocalizationsLeafCommand(
				BetaBuildLocalizationsUpdateCommand(),
				"asc builds test-notes update",
				"Warning: `asc beta-build-localizations update` is deprecated. Use `asc builds test-notes update`.",
			),
			deprecatedBetaBuildLocalizationsLeafCommand(
				BetaBuildLocalizationsDeleteCommand(),
				"asc builds test-notes delete",
				"Warning: `asc beta-build-localizations delete` is deprecated. Use `asc builds test-notes delete`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: `asc beta-build-localizations` is deprecated. Use `asc builds test-notes ...` for canonical build-scoped workflows.")
			return flag.ErrHelp
		},
	}
}

// BetaBuildLocalizationsListCommand returns the list subcommand.
func BetaBuildLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	global := fs.Bool("global", false, "List beta build localizations across all builds (top-level endpoint)")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc beta-build-localizations list [flags]",
		ShortHelp:  "List beta build localizations for a build or globally.",
		LongHelp: `List beta build localizations for a build or globally.

Examples:
  asc beta-build-localizations list --build "BUILD_ID"
  asc beta-build-localizations list --build "BUILD_ID" --locale "en-US,ja"
  asc beta-build-localizations list --build "BUILD_ID" --paginate
  asc beta-build-localizations list --global
  asc beta-build-localizations list --global --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("beta-build-localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("beta-build-localizations list: %w", err)
			}

			buildValue := strings.TrimSpace(*buildID)

			// Reject --global + --build combination
			if *global && buildValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --global and --build are mutually exclusive")
				return flag.ErrHelp
			}

			// Require one of --build or --global (unless --next is provided)
			if !*global && buildValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build or --global is required")
				return flag.ErrHelp
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("beta-build-localizations list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations list: %w", err)
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

			if *global {
				if *paginate {
					paginateOpts := append(opts, asc.WithBetaBuildLocalizationsLimit(200))
					firstPage, err := client.ListBetaBuildLocalizations(requestCtx, paginateOpts...)
					if err != nil {
						return fmt.Errorf("beta-build-localizations list: failed to fetch: %w", err)
					}

					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.ListBetaBuildLocalizations(ctx, asc.WithBetaBuildLocalizationsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("beta-build-localizations list: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := client.ListBetaBuildLocalizations(requestCtx, opts...)
				if err != nil {
					return fmt.Errorf("beta-build-localizations list: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaBuildLocalizations(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("beta-build-localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaBuildLocalizationsGetCommand returns the get subcommand.
func BetaBuildLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required with --latest)")
	latest := fs.Bool("latest", false, "Resolve latest build for --app context")
	state := fs.String("state", "", "Latest-build state filter: PROCESSING,VALID,FAILED,INVALID,COMPLETE(alias of VALID), or all (requires --latest)")
	locale := fs.String("locale", "", "Locale for latest-build lookup (e.g., en-US); required when latest build has multiple localizations")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc beta-build-localizations get --id \"LOCALIZATION_ID\" | --app \"APP_ID\" --latest [flags]",
		ShortHelp:  "Get a beta build localization by ID.",
		LongHelp: `Get a beta build localization by ID or by latest build for an app.

Examples:
  asc beta-build-localizations get --id "LOCALIZATION_ID"
  asc beta-build-localizations get --app "123456789" --latest --state "PROCESSING,COMPLETE"
  asc beta-build-localizations get --app "123456789" --latest --state "PROCESSING,COMPLETE" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := strings.TrimSpace(*appID)
			stateValue := strings.TrimSpace(*state)
			localeValue := strings.TrimSpace(*locale)
			normalizedStateValues, err := normalizeLatestBuildProcessingStateFilter(stateValue)
			if err != nil {
				return err
			}

			if idValue != "" {
				if appValue != "" || *latest || stateValue != "" || localeValue != "" {
					fmt.Fprintln(os.Stderr, "Error: --id is mutually exclusive with --app, --latest, --state, and --locale")
					return flag.ErrHelp
				}
			} else {
				if appValue == "" && !*latest {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				if *latest && appValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required with --latest")
					return flag.ErrHelp
				}
				if !*latest && appValue != "" {
					fmt.Fprintln(os.Stderr, "Error: --latest is required with --app")
					return flag.ErrHelp
				}
			}
			if stateValue != "" && !*latest {
				fmt.Fprintln(os.Stderr, "Error: --state requires --latest")
				return flag.ErrHelp
			}
			if localeValue != "" && !*latest {
				fmt.Fprintln(os.Stderr, "Error: --locale requires --latest")
				return flag.ErrHelp
			}
			if localeValue != "" {
				if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
					return fmt.Errorf("beta-build-localizations get: %w", err)
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if idValue == "" {
				buildValue, err := resolveLatestBuildIDForBetaBuildLocalizations(requestCtx, client, appValue, normalizedStateValues)
				if err != nil {
					return fmt.Errorf("beta-build-localizations get: %w", err)
				}

				opts := []asc.BetaBuildLocalizationsOption{
					asc.WithBetaBuildLocalizationsLimit(200),
				}
				if localeValue != "" {
					opts = append(opts, asc.WithBetaBuildLocalizationLocales([]string{localeValue}))
				}

				localizations, err := client.GetBetaBuildLocalizations(requestCtx, buildValue, opts...)
				if err != nil {
					return fmt.Errorf("beta-build-localizations get: failed to fetch latest-build localizations: %w", err)
				}
				if len(localizations.Data) == 0 {
					if localeValue != "" {
						return fmt.Errorf("beta-build-localizations get: no localization found for latest build %q and locale %q", buildValue, localeValue)
					}
					return fmt.Errorf("beta-build-localizations get: no localization found for latest build %q", buildValue)
				}
				if localeValue == "" && len(localizations.Data) > 1 {
					return fmt.Errorf(
						"beta-build-localizations get: latest build %q has %d localizations; pass --locale to disambiguate",
						buildValue,
						len(localizations.Data),
					)
				}

				resp := &asc.BetaBuildLocalizationResponse{
					Data:  localizations.Data[0],
					Links: localizations.Links,
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaBuildLocalization(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-build-localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaBuildLocalizationsCreateCommand returns the create subcommand.
func BetaBuildLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required with --latest)")
	latest := fs.Bool("latest", false, "Resolve latest build for --app context")
	state := fs.String("state", "", "Latest-build state filter: PROCESSING,VALID,FAILED,INVALID,COMPLETE(alias of VALID), or all (requires --latest)")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	upsert := fs.Bool("upsert", false, "Create-or-update by locale (idempotent)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc beta-build-localizations create [flags]",
		ShortHelp:  "Create a beta build localization.",
		LongHelp: `Create a beta build localization.

Examples:
  asc beta-build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions"
  asc beta-build-localizations create --app "123456789" --latest --state "PROCESSING,COMPLETE" --locale "en-US" --whats-new "Test instructions"
  asc beta-build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Test instructions" --upsert`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			appValue := strings.TrimSpace(*appID)
			stateValue := strings.TrimSpace(*state)
			normalizedStateValues, err := normalizeLatestBuildProcessingStateFilter(stateValue)
			if err != nil {
				return err
			}

			if buildValue == "" && appValue == "" && !*latest {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if buildValue != "" && (appValue != "" || *latest || stateValue != "") {
				fmt.Fprintln(os.Stderr, "Error: --build is mutually exclusive with --app, --latest, and --state")
				return flag.ErrHelp
			}
			if *latest && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required with --latest")
				return flag.ErrHelp
			}
			if !*latest && appValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --latest is required with --app")
				return flag.ErrHelp
			}
			if stateValue != "" && !*latest {
				fmt.Fprintln(os.Stderr, "Error: --state requires --latest")
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if buildValue == "" {
				buildValue, err = resolveLatestBuildIDForBetaBuildLocalizations(requestCtx, client, appValue, normalizedStateValues)
				if err != nil {
					return fmt.Errorf("beta-build-localizations create: %w", err)
				}
			}

			attrs := asc.BetaBuildLocalizationAttributes{
				Locale:   localeValue,
				WhatsNew: whatsNewValue,
			}

			var resp *asc.BetaBuildLocalizationResponse
			if *upsert {
				resp, err = shared.UpsertBetaBuildLocalization(requestCtx, client, buildValue, localeValue, whatsNewValue)
				if err != nil {
					return fmt.Errorf("beta-build-localizations create: failed to upsert: %w", err)
				}
			} else {
				resp, err = client.CreateBetaBuildLocalization(requestCtx, buildValue, attrs)
				if err != nil {
					return fmt.Errorf("beta-build-localizations create: failed to create: %w", err)
				}
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaBuildLocalizationsUpdateCommand returns the update subcommand.
func BetaBuildLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc beta-build-localizations update [flags]",
		ShortHelp:  "Update a beta build localization.",
		LongHelp: `Update a beta build localization.

Examples:
  asc beta-build-localizations update --id "LOCALIZATION_ID" --whats-new "Updated notes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaBuildLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}

			resp, err := client.UpdateBetaBuildLocalization(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("beta-build-localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaBuildLocalizationsDeleteCommand returns the delete subcommand.
func BetaBuildLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc beta-build-localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a beta build localization.",
		LongHelp: `Delete a beta build localization.

Examples:
  asc beta-build-localizations delete --id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaBuildLocalization(requestCtx, idValue); err != nil {
				return fmt.Errorf("beta-build-localizations delete: failed to delete: %w", err)
			}

			result := &asc.BetaBuildLocalizationDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
