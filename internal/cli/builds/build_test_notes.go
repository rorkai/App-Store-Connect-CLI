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

const legacyLocalizationIDWarning = "Warning: `--id` is deprecated. Use `--localization-id`."

type testNotesBuildSelectorFlags struct {
	buildSelectorFlags
}

// BuildsTestNotesCommand returns the builds test-notes command group.
func BuildsTestNotesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("test-notes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "test-notes",
		ShortUsage: "asc builds test-notes <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight What to Test notes.",
		LongHelp: `Manage TestFlight "What to Test" notes for a build.

Build selector modes:
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --build-number NUM [--version VER] [--platform PLATFORM]

Examples:
  asc builds test-notes list --build-id "BUILD_ID"
  asc builds test-notes view --app "123456789" --latest --locale "en-US"
  asc builds test-notes create --app "123456789" --build-number "42" --version "1.2.3" --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes update --build-id "BUILD_ID" --locale "en-US" --whats-new "Updated instructions"
  asc builds test-notes delete --build-id "BUILD_ID" --locale "en-US" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsTestNotesListCommand(),
			BuildsTestNotesViewCommand(),
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

	selectors := bindTestNotesBuildSelectorFlags(fs)
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds test-notes list [--build-id BUILD_ID | --app APP --latest [--version VER] [--platform PLATFORM] | --app APP --build-number NUM [--version VER] [--platform PLATFORM]] [flags]",
		ShortHelp:  "List What to Test notes for a build.",
		LongHelp: `List What to Test notes for a build.

Build selector modes (one of):
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --build-number NUM [--version VER] [--platform PLATFORM]

Examples:
  asc builds test-notes list --build-id "BUILD_ID"
  asc builds test-notes list --app "123456789" --latest --locale "en-US,ja"
  asc builds test-notes list --app "123456789" --build-number "42"
  asc builds test-notes list --build-id "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds test-notes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}
			if strings.TrimSpace(*next) == "" {
				if err := validateResolveBuildOptions(selectors.resolveOptions()); err != nil {
					return fmt.Errorf("builds test-notes list: %w", err)
				}
			} else if err := selectors.validateNextPageSelectorFlags(); err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			opts := []asc.BetaBuildLocalizationsOption{
				asc.WithBetaBuildLocalizationsLimit(*limit),
				asc.WithBetaBuildLocalizationsNextURL(*next),
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithBetaBuildLocalizationLocales(locales))
			}
			if strings.TrimSpace(*next) == "" {
				buildResp, err := selectors.resolveBuild(ctx, client)
				if err != nil {
					return fmt.Errorf("builds test-notes list: %w", err)
				}
				opts = append(opts, asc.WithBetaBuildLocalizationBuildIDs([]string{buildResp.Data.ID}))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaBuildLocalizationsLimit(200))
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.ListBetaBuildLocalizations(ctx, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.ListBetaBuildLocalizations(ctx, asc.WithBetaBuildLocalizationsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds test-notes list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.ListBetaBuildLocalizations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("builds test-notes list: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsTestNotesViewCommand returns the view subcommand.
func BuildsTestNotesViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(fs)
	localizationID := fs.String("localization-id", "", "Localization ID (low-level escape hatch)")
	legacyLocalizationID := bindHiddenLocalizationIDFlag(fs)
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with build selectors)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc builds test-notes view [flags]",
		ShortHelp:  "View What to Test notes for a build and locale.",
		LongHelp: `View What to Test notes for a build selector and locale.

Selector modes:
  --localization-id LOCALIZATION_ID
  --locale LOCALE with one of:
    --build-id BUILD_ID
    --app APP --latest [--version VER] [--platform PLATFORM]
    --app APP --build-number NUM [--version VER] [--platform PLATFORM]

Examples:
  asc builds test-notes view --build-id "BUILD_ID" --locale "en-US"
  asc builds test-notes view --app "123456789" --latest --locale "en-US"
  asc builds test-notes view --app "123456789" --build-number "42" --version "1.2.3" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if err := applyLegacyLocalizationIDAlias(localizationID, legacyLocalizationID); err != nil {
				return err
			}

			id := strings.TrimSpace(*localizationID)
			localeValue := strings.TrimSpace(*locale)
			if err := validateTestNotesLocalizationTarget(id, localeValue, selectors); err != nil {
				return fmt.Errorf("builds test-notes view: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes view: %w", err)
			}

			if id != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.GetBetaBuildLocalization(requestCtx, id)
				if err != nil {
					return fmt.Errorf("builds test-notes view: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := resolveTestNotesLocalization(ctx, client, selectors, localeValue)
			if err != nil {
				return fmt.Errorf("builds test-notes view: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsTestNotesCreateCommand returns the create subcommand.
func BuildsTestNotesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(fs)
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc builds test-notes create [--build-id BUILD_ID | --app APP --latest [--version VER] [--platform PLATFORM] | --app APP --build-number NUM [--version VER] [--platform PLATFORM]] [flags]",
		ShortHelp:  "Create What to Test notes for a build.",
		LongHelp: `Create What to Test notes for a build.

Build selector modes (one of):
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --build-number NUM [--version VER] [--platform PLATFORM]

Examples:
  asc builds test-notes create --build-id "BUILD_ID" --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes create --app "123456789" --latest --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes create --app "123456789" --build-number "42" --version "1.2.3" --locale "en-US" --whats-new "Test instructions"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError()
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}
			if err := validateResolveBuildOptions(selectors.resolveOptions()); err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --whats-new is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}
			buildResp, err := selectors.resolveBuild(ctx, client)
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			attrs := asc.BetaBuildLocalizationAttributes{
				Locale:   localeValue,
				WhatsNew: whatsNewValue,
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBetaBuildLocalization(requestCtx, buildResp.Data.ID, attrs)
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

	selectors := bindTestNotesBuildSelectorFlags(fs)
	localizationID := fs.String("localization-id", "", "Localization ID (low-level escape hatch)")
	legacyLocalizationID := bindHiddenLocalizationIDFlag(fs)
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with build selectors)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc builds test-notes update [flags]",
		ShortHelp:  "Update What to Test notes for a build and locale.",
		LongHelp: `Update What to Test notes for a build selector and locale.

Selector modes:
  --localization-id LOCALIZATION_ID
  --locale LOCALE with one of:
    --build-id BUILD_ID
    --app APP --latest [--version VER] [--platform PLATFORM]
    --app APP --build-number NUM [--version VER] [--platform PLATFORM]

Examples:
  asc builds test-notes update --build-id "BUILD_ID" --locale "en-US" --whats-new "Updated notes"
  asc builds test-notes update --app "123456789" --build-number "42" --version "1.2.3" --locale "en-US" --whats-new "Updated notes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if err := applyLegacyLocalizationIDAlias(localizationID, legacyLocalizationID); err != nil {
				return err
			}

			id := strings.TrimSpace(*localizationID)
			localeValue := strings.TrimSpace(*locale)
			if err := validateTestNotesLocalizationTarget(id, localeValue, selectors); err != nil {
				return fmt.Errorf("builds test-notes update: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes update: %w", err)
			}

			if id == "" {
				localization, err := resolveTestNotesLocalization(ctx, client, selectors, localeValue)
				if err != nil {
					return fmt.Errorf("builds test-notes update: %w", err)
				}
				id = strings.TrimSpace(localization.Data.ID)
			}

			attrs := asc.BetaBuildLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

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

	selectors := bindTestNotesBuildSelectorFlags(fs)
	localizationID := fs.String("localization-id", "", "Localization ID (low-level escape hatch)")
	legacyLocalizationID := bindHiddenLocalizationIDFlag(fs)
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with build selectors)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc builds test-notes delete [flags]",
		ShortHelp:  "Delete What to Test notes for a build and locale.",
		LongHelp: `Delete What to Test notes for a build selector and locale.

Selector modes:
  --localization-id LOCALIZATION_ID
  --locale LOCALE with one of:
    --build-id BUILD_ID
    --app APP --latest [--version VER] [--platform PLATFORM]
    --app APP --build-number NUM [--version VER] [--platform PLATFORM]

Examples:
  asc builds test-notes delete --build-id "BUILD_ID" --locale "en-US" --confirm
  asc builds test-notes delete --app "123456789" --build-number "42" --version "1.2.3" --locale "en-US" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if err := applyLegacyLocalizationIDAlias(localizationID, legacyLocalizationID); err != nil {
				return err
			}

			id := strings.TrimSpace(*localizationID)
			localeValue := strings.TrimSpace(*locale)
			if err := validateTestNotesLocalizationTarget(id, localeValue, selectors); err != nil {
				return fmt.Errorf("builds test-notes delete: %w", err)
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds test-notes delete: %w", err)
			}

			if id == "" {
				localization, err := resolveTestNotesLocalization(ctx, client, selectors, localeValue)
				if err != nil {
					return fmt.Errorf("builds test-notes delete: %w", err)
				}
				id = strings.TrimSpace(localization.Data.ID)
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

func bindTestNotesBuildSelectorFlags(fs *flag.FlagSet) testNotesBuildSelectorFlags {
	return testNotesBuildSelectorFlags{
		buildSelectorFlags: bindBuildSelectorFlags(fs, buildSelectorFlagOptions{
			buildIDUsage:     "Build ID",
			appUsage:         "App ID, bundle ID, or app name (or ASC_APP_ID)",
			latestUsage:      "Resolve the latest matching build for --app context",
			versionUsage:     "App version string (e.g., 1.2.3)",
			buildNumberUsage: "Build number (CFBundleVersion)",
			platformUsage:    "Platform: IOS, MAC_OS, TV_OS, VISION_OS",
		}),
	}
}

func (f testNotesBuildSelectorFlags) hasAnyInputs() bool {
	opts := f.resolveOptions()
	return opts.BuildID != "" ||
		opts.AppID != "" ||
		opts.Version != "" ||
		opts.BuildNumber != "" ||
		opts.Platform != "" ||
		opts.Latest
}

func (f testNotesBuildSelectorFlags) resolveBuild(ctx context.Context, client *asc.Client) (*asc.BuildResponse, error) {
	opts := f.resolveOptions()
	if err := validateResolveBuildOptions(opts); err != nil {
		return nil, err
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	return ResolveBuild(requestCtx, client, opts)
}

func bindHiddenLocalizationIDFlag(fs *flag.FlagSet) *trackedStringFlag {
	value := &trackedStringFlag{}
	fs.Var(value, "id", "DEPRECATED: use --localization-id")
	shared.HideFlagFromHelp(fs.Lookup("id"))
	return value
}

func applyLegacyLocalizationIDAlias(localizationID *string, legacyLocalizationID *trackedStringFlag) error {
	return applyLegacyStringAlias(localizationID, legacyLocalizationID, "--id", "--localization-id", legacyLocalizationIDWarning)
}

func validateTestNotesLocalizationTarget(localizationID, locale string, selectors testNotesBuildSelectorFlags) error {
	localizationIDValue := strings.TrimSpace(localizationID)
	localeValue := strings.TrimSpace(locale)
	if localizationIDValue != "" {
		if selectors.hasAnyInputs() || localeValue != "" {
			return shared.UsageError("--localization-id cannot be combined with build selectors or --locale")
		}
		return nil
	}
	if localeValue == "" || !selectors.hasAnyInputs() {
		return shared.UsageError("either --localization-id or (--locale and a build selector) is required")
	}
	if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
		return err
	}
	return validateResolveBuildOptions(selectors.resolveOptions())
}

func resolveTestNotesLocalization(ctx context.Context, client *asc.Client, selectors testNotesBuildSelectorFlags, locale string) (*asc.BetaBuildLocalizationResponse, error) {
	buildResp, err := selectors.resolveBuild(ctx, client)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	localizations, err := client.ListBetaBuildLocalizations(
		requestCtx,
		asc.WithBetaBuildLocalizationBuildIDs([]string{buildResp.Data.ID}),
		asc.WithBetaBuildLocalizationLocales([]string{strings.TrimSpace(locale)}),
		asc.WithBetaBuildLocalizationsLimit(200),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve localization: %w", err)
	}
	if len(localizations.Data) == 0 {
		return nil, fmt.Errorf("no localization found for build %q and locale %q", buildResp.Data.ID, locale)
	}
	if len(localizations.Data) > 1 {
		return nil, fmt.Errorf("multiple localizations found for build %q and locale %q; use --localization-id", buildResp.Data.ID, locale)
	}

	match := localizations.Data[0]
	if strings.TrimSpace(match.ID) == "" {
		return nil, fmt.Errorf("resolved localization has empty ID")
	}

	return &asc.BetaBuildLocalizationResponse{
		Data:  match,
		Links: localizations.Links,
	}, nil
}
