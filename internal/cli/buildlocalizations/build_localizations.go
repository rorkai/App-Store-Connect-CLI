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

These commands manage the App Store version localizations (What's New text)
of the App Store version a build is attached to. A build that is not
attached to an App Store version, such as a TestFlight-only build, is rejected.
For TestFlight What to Test notes use asc builds test-notes.

Examples:
  asc build-localizations list --build-id "BUILD_ID"
  asc build-localizations view --id "LOCALIZATION_ID"
  asc build-localizations create --build-id "BUILD_ID" --locale "en-US" --whats-new "Bug fixes"
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

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc build-localizations list [flags]",
		ShortHelp:  "List release note localizations for a build.",
		LongHelp: `List release note localizations for a build.

Lists the App Store version localizations of the App Store version the build
is attached to. For TestFlight What to Test notes use asc builds test-notes list.

Examples:
  asc build-localizations list --build-id "BUILD_ID"
  asc build-localizations list --build-id "BUILD_ID" --locale "en-US,ja"
  asc build-localizations list --build-id "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("build-localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("build-localizations list: %v", err)
			}

			build := strings.TrimSpace(*buildID)
			if build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
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

			versionID, err := resolveBuildAppStoreVersion(requestCtx, client, "list", build)
			if err != nil {
				if shared.IsReportedUsageError(err) {
					return err
				}
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
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, versionID, opts...)
			if err != nil {
				return fmt.Errorf("build-localizations list: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildLocalizationsGetCommand returns the get subcommand.
func BuildLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc build-localizations view [flags]",
		ShortHelp:  "View a localization by ID.",
		LongHelp: `View a localization by ID.

Examples:
  asc build-localizations view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-localizations view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("build-localizations view: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildLocalizationsCreateCommand returns the create subcommand.
func BuildLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "Release notes (whats new), up to 4000 characters")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc build-localizations create [flags]",
		ShortHelp:  "Create a localization for a build.",
		LongHelp: `Create a localization for a build.

Creates an App Store version localization on the App Store version the build
is attached to. For TestFlight What to Test notes use asc builds test-notes create.

Release notes are limited to 4000 characters and are checked before the
request is sent.

Examples:
  asc build-localizations create --build-id "BUILD_ID" --locale "en-US"
  asc build-localizations create --build-id "BUILD_ID" --locale "en-US" --whats-new "Bug fixes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			build := strings.TrimSpace(*buildID)
			if build == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError("--locale")
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)

			attrs := asc.AppStoreVersionLocalizationAttributes{
				Locale: localeValue,
			}
			if whatsNewValue != "" {
				attrs.WhatsNew = whatsNewValue
			}
			if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			versionID, err := resolveBuildAppStoreVersion(requestCtx, client, "create", build)
			if err != nil {
				if shared.IsReportedUsageError(err) {
					return err
				}
				return fmt.Errorf("build-localizations create: %w", err)
			}

			resp, err := client.CreateAppStoreVersionLocalization(requestCtx, versionID, attrs)
			if err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			submitOpts := shared.SubmitReadinessOptions{}
			if strings.TrimSpace(attrs.WhatsNew) == "" {
				submitOpts = shared.ResolveSubmitReadinessOptionsForVersionBestEffort(requestCtx, client, versionID, "", "")
			}
			warnings := make([]shared.SubmitReadinessCreateWarning, 0, 1)
			if warning, ok := shared.SubmitReadinessCreateWarningForLocaleWithOptions(localeValue, attrs, shared.SubmitReadinessCreateModeApplied, submitOpts); ok {
				warnings = append(warnings, warning)
			}

			if err := shared.PrintOutput(resp, *output.Output, *output.Pretty); err != nil {
				return err
			}
			return shared.PrintSubmitReadinessCreateWarnings(os.Stderr, warnings)
		},
	}
}

// BuildLocalizationsUpdateCommand returns the update subcommand.
func BuildLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	whatsNew := fs.String("whats-new", "", "Release notes (whats new), up to 4000 characters")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc build-localizations update [flags]",
		ShortHelp:  "Update a localization by ID.",
		LongHelp: `Update a localization by ID.

Release notes are limited to 4000 characters and are checked before the
request is sent.

Examples:
  asc build-localizations update --id "LOCALIZATION_ID" --whats-new "New features"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("--whats-new")
			}

			attrs := asc.AppStoreVersionLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}
			if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreVersionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("build-localizations update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildLocalizationsDeleteCommand returns the delete subcommand.
func BuildLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// resolveBuildAppStoreVersion returns the App Store version a build is attached
// to. Apple answers GET /v1/builds/{id}/appStoreVersion with a null data object
// for a build that is not attached to any version, which is the normal state of
// a TestFlight-only build, so that case is reported as a usage error with
// guidance instead of a generic failure. API errors, including an unknown build
// ID, are returned unchanged so the service detail and classification survive.
func resolveBuildAppStoreVersion(ctx context.Context, client *asc.Client, subcommand, buildID string) (string, error) {
	resp, err := client.GetBuildAppStoreVersion(ctx, buildID)
	if err != nil {
		return "", err
	}
	if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
		testNotesCommand := fmt.Sprintf(`asc builds test-notes %s --build-id "BUILD_ID"`, subcommand)
		if subcommand == "create" {
			testNotesCommand += ` --locale "LOCALE" --whats-new "NOTES"`
		}
		message := fmt.Sprintf(
			"build-localizations %s: the selected build is not attached to an App Store version. build-localizations manages the App Store version's What's New text; for TestFlight What to Test notes use `%s`.",
			subcommand, testNotesCommand,
		)
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
		return "", shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
	}
	return resp.Data.ID, nil
}
