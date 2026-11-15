package builds

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type testNotesBuildSelectorFlags struct {
	buildSelectorFlags
}

// newBuildsTestNotesCommand returns the builds test-notes command group.
func newBuildsTestNotesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("test-notes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "test-notes",
		ShortUsage: "asc builds test-notes <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight What to Test notes.",
		LongHelp: `Manage TestFlight "What to Test" notes for a build.

Build selector modes:
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --build-number NUM --platform PLATFORM [--version VER]

Examples:
  asc builds test-notes list --build-id "BUILD_ID"
  asc builds test-notes view --app "123456789" --latest --locale "en-US"
  asc builds test-notes create --app "123456789" --build-number "42" --platform IOS --version "1.2.3" --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes update --build-id "BUILD_ID" --locale "en-US" --whats-new "Updated instructions"
  asc builds test-notes delete --build-id "BUILD_ID" --locale "en-US" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildsTestNotesListCommand(d),
			newBuildsTestNotesViewCommand(d),
			newBuildsTestNotesCreateCommand(d),
			newBuildsTestNotesUpdateCommand(d),
			newBuildsTestNotesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildsTestNotesListCommand returns the list subcommand.
func newBuildsTestNotesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(d, fs)
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds test-notes list [--build-id BUILD_ID | --app APP --latest [--version VER] [--platform PLATFORM] | --app APP --build-number NUM --platform PLATFORM [--version VER]] [flags]",
		ShortHelp:  "List What to Test notes for a build.",
		LongHelp: `List What to Test notes for a build.

Build selector modes (one of):
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --build-number NUM --platform PLATFORM [--version VER]

Examples:
  asc builds test-notes list --build-id "BUILD_ID"
  asc builds test-notes list --app "123456789" --latest --locale "en-US,ja"
  asc builds test-notes list --app "123456789" --build-number "42" --platform IOS
  asc builds test-notes list --build-id "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "builds test-notes list", 200, "locale"); err != nil {
				return err
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return d.UsageErrorf("builds test-notes list: %v", err)
			}
			if strings.TrimSpace(*next) == "" {
				if err := validateResolveBuildOptions(d, selectors.resolveOptions()); err != nil {
					return fmt.Errorf("builds test-notes list: %w", err)
				}
			} else if err := selectors.validateNextPageSelectorFlags(); err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds test-notes list: %w", err)
			}

			opts := []asc.BetaBuildLocalizationsOption{
				asc.Limit[asc.BetaBuildLocalizationsOption](*limit),
				asc.NextURL[asc.BetaBuildLocalizationsOption](*next),
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
				paginateOpts := append(opts, asc.Limit[asc.BetaBuildLocalizationsOption](200))
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				resp, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.ListBetaBuildLocalizations(ctx, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.ListBetaBuildLocalizations(ctx, asc.NextURL[asc.BetaBuildLocalizationsOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds test-notes list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.ListBetaBuildLocalizations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("builds test-notes list: failed to fetch: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsTestNotesViewCommand returns the view subcommand.
func newBuildsTestNotesViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(d, fs)
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "betaBuildLocalizations", "Localization ID (low-level escape hatch)")
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with build selectors)")
	output := d.BindOutputFlags(fs)

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
    --app APP --build-number NUM --platform PLATFORM [--version VER]

Examples:
  asc builds test-notes view --build-id "BUILD_ID" --locale "en-US"
  asc builds test-notes view --app "123456789" --latest --locale "en-US"
  asc builds test-notes view --app "123456789" --build-number "42" --platform IOS --version "1.2.3" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			localeValue := strings.TrimSpace(*locale)
			if err := validateTestNotesLocalizationTarget("builds test-notes view", id, localeValue, selectors); err != nil {
				return fmt.Errorf("builds test-notes view: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds test-notes view: %w", err)
			}

			if id != "" {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				resp, err := client.GetBetaBuildLocalization(requestCtx, id)
				if err != nil {
					return fmt.Errorf("builds test-notes view: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := resolveTestNotesLocalization(ctx, client, selectors, localeValue)
			if err != nil {
				return fmt.Errorf("builds test-notes view: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsTestNotesCreateCommand returns the create subcommand.
func newBuildsTestNotesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(d, fs)
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc builds test-notes create [--build-id BUILD_ID | --app APP --latest [--version VER] [--platform PLATFORM] | --app APP --build-number NUM --platform PLATFORM [--version VER]] [flags]",
		ShortHelp:  "Create What to Test notes for a build.",
		LongHelp: `Create What to Test notes for a build.

Build selector modes (one of):
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --build-number NUM --platform PLATFORM [--version VER]

Examples:
  asc builds test-notes create --build-id "BUILD_ID" --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes create --app "123456789" --latest --locale "en-US" --whats-new "Test instructions"
  asc builds test-notes create --app "123456789" --build-number "42" --platform IOS --version "1.2.3" --locale "en-US" --whats-new "Test instructions"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return d.UsageErrorf("builds test-notes create: %v", err)
			}
			if err := validateResolveBuildOptions(d, selectors.resolveOptions()); err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				return d.RequiredFlag("--whats-new")
			}
			whatsNewValue, normalizeErr := shared.NormalizeTestNotesForCommand(d.IO.ErrOut, whatsNewValue)
			if normalizeErr != nil {
				return fmt.Errorf("builds test-notes create: %w", normalizeErr)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}
			buildResp, err := selectors.resolveBuild(ctx, client)
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := shared.UpsertBetaBuildLocalization(requestCtx, client, buildResp.Data.ID, localeValue, whatsNewValue, shared.UpsertBetaBuildLocalizationOptions{Diagnostics: d.IO.ErrOut})
			if err != nil {
				return fmt.Errorf("builds test-notes create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsTestNotesUpdateCommand returns the update subcommand.
func newBuildsTestNotesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(d, fs)
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "betaBuildLocalizations", "Localization ID (low-level escape hatch)")
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with build selectors)")
	whatsNew := fs.String("whats-new", "", "What to Test notes")
	output := d.BindOutputFlags(fs)

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
    --app APP --build-number NUM --platform PLATFORM [--version VER]

Examples:
  asc builds test-notes update --build-id "BUILD_ID" --locale "en-US" --whats-new "Updated notes"
  asc builds test-notes update --app "123456789" --build-number "42" --platform IOS --version "1.2.3" --locale "en-US" --whats-new "Updated notes"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			localeValue := strings.TrimSpace(*locale)
			if err := validateTestNotesLocalizationTarget("builds test-notes update", id, localeValue, selectors); err != nil {
				return fmt.Errorf("builds test-notes update: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				return d.UpdateFlagRequired(fs, whatsNew)
			}
			whatsNewValue, normalizeErr := shared.NormalizeTestNotesForCommand(d.IO.ErrOut, whatsNewValue)
			if normalizeErr != nil {
				return fmt.Errorf("builds test-notes update: %w", normalizeErr)
			}

			client, err := d.Client()
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

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaBuildLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("builds test-notes update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsTestNotesDeleteCommand returns the delete subcommand.
func newBuildsTestNotesDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)

	selectors := bindTestNotesBuildSelectorFlags(d, fs)
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "betaBuildLocalizations", "Localization ID (low-level escape hatch)")
	locale := fs.String("locale", "", "Locale (e.g., en-US, required with build selectors)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

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
    --app APP --build-number NUM --platform PLATFORM [--version VER]

Examples:
  asc builds test-notes delete --build-id "BUILD_ID" --locale "en-US" --confirm
  asc builds test-notes delete --app "123456789" --build-number "42" --platform IOS --version "1.2.3" --locale "en-US" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			localeValue := strings.TrimSpace(*locale)
			if err := validateTestNotesLocalizationTarget("builds test-notes delete", id, localeValue, selectors); err != nil {
				return fmt.Errorf("builds test-notes delete: %w", err)
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
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

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteBetaBuildLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("builds test-notes delete: %w", err)
			}

			result := &asc.DeleteResult{
				ID:      id,
				Deleted: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func bindTestNotesBuildSelectorFlags(d *shared.Deps, fs *flag.FlagSet) testNotesBuildSelectorFlags {
	return testNotesBuildSelectorFlags{
		buildSelectorFlags: bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{
			buildIDUsage:     "Build ID",
			appUsage:         "App ID, bundle ID, or app name (or ASC_APP_ID)",
			latestUsage:      "Resolve the latest matching build for --app context",
			versionUsage:     "App version string (e.g., 1.2.3)",
			buildNumberUsage: "Build number (CFBundleVersion)",
			platformUsage:    "Platform (required with --build-number): IOS, MAC_OS, TV_OS, VISION_OS",
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
	if err := validateResolveBuildOptions(f.d, opts); err != nil {
		return nil, err
	}

	requestCtx, cancel := f.d.Timeout(ctx)
	defer cancel()

	return ResolveBuild(requestCtx, f.d, client, opts)
}

func validateTestNotesLocalizationTarget(command, localizationID, locale string, selectors testNotesBuildSelectorFlags) error {
	localizationIDValue := strings.TrimSpace(localizationID)
	localeValue := strings.TrimSpace(locale)
	if localizationIDValue != "" {
		if selectors.hasAnyInputs() || localeValue != "" {
			return selectors.d.UsageError("--localization-id cannot be combined with build selectors or --locale")
		}
		return nil
	}
	if localeValue == "" || !selectors.hasAnyInputs() {
		return selectors.d.UsageError("either --localization-id or (--locale and a build selector) is required")
	}
	if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
		return selectors.d.UsageErrorf("%s: %v", command, err)
	}
	return validateResolveBuildOptions(selectors.d, selectors.resolveOptions())
}

func resolveTestNotesLocalization(ctx context.Context, client *asc.Client, selectors testNotesBuildSelectorFlags, locale string) (*asc.BetaBuildLocalizationResponse, error) {
	buildResp, err := selectors.resolveBuild(ctx, client)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := selectors.d.Timeout(ctx)
	defer cancel()

	localizations, err := client.ListBetaBuildLocalizations(
		requestCtx,
		asc.WithBetaBuildLocalizationBuildIDs([]string{buildResp.Data.ID}),
		asc.WithBetaBuildLocalizationLocales([]string{strings.TrimSpace(locale)}),
		asc.Limit[asc.BetaBuildLocalizationsOption](200),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve localization: %w", err)
	}
	if localizations == nil {
		return nil, fmt.Errorf("empty localization response")
	}
	pageHasNext := strings.TrimSpace(localizations.Links.Next) != ""
	if len(localizations.Data) == 0 && !pageHasNext {
		return nil, fmt.Errorf("no localization found for build %q and locale %q", buildResp.Data.ID, locale)
	}
	if len(localizations.Data) > 1 || pageHasNext {
		ambiguous := &shared.AmbiguousSelectionError{
			Kind:        "build localization",
			Description: fmt.Sprintf("build %q and locale %q", buildResp.Data.ID, strings.TrimSpace(locale)),
			Flag:        "--localization-id",
			Candidates: shared.LocalizationCandidates(localizations.Data, func(attributes asc.BetaBuildLocalizationAttributes) string {
				return attributes.Locale
			}),
			Hint: "Use --localization-id instead of the build and locale selectors.",
		}
		if pageHasNext {
			return nil, shared.MarkAmbiguousSelectionSample(ambiguous)
		}
		return nil, ambiguous
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
