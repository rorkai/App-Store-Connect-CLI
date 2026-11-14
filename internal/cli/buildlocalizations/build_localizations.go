package buildlocalizations

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewBuildLocalizationsCommand returns the build-localizations command group
// built on d.
func NewBuildLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("build-localizations", flag.ExitOnError)

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
			newBuildLocalizationsListCommand(d),
			newBuildLocalizationsGetCommand(d),
			newBuildLocalizationsCreateCommand(d),
			newBuildLocalizationsUpdateCommand(d),
			newBuildLocalizationsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildLocalizationsListCommand returns the list subcommand built on d.
func newBuildLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
  asc build-localizations list --build-id "BUILD_ID" --paginate
  asc build-localizations list --next "NEXT_URL"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return d.UsageError("build-localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("build-localizations list: %v", err)
			}

			build := strings.TrimSpace(*buildID)
			nextURL := strings.TrimSpace(*next)
			if build == "" && nextURL == "" {
				return d.RequiredFlag("--build-id")
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return d.UsageErrorf("build-localizations list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-localizations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			// A links.next URL already names its version, so --build-id is
			// only resolved when given, to reject a page of another version.
			versionID := ""
			if build != "" {
				versionID, err = resolveBuildAppStoreVersion(requestCtx, d, client, "list", build)
				if err != nil {
					if shared.IsReportedUsageError(err) {
						return err
					}
					return fmt.Errorf("build-localizations list: %w", err)
				}
				if nextURL != "" && !isVersionLocalizationsPage(nextURL, versionID) {
					return d.UsageErrorf("build-localizations list: --next is not a page of the App Store version attached to --build-id %q", build)
				}
			}

			opts := []asc.AppStoreVersionLocalizationsOption{
				asc.Limit[asc.AppStoreVersionLocalizationsOption](*limit),
				asc.NextURL[asc.AppStoreVersionLocalizationsOption](*next),
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithAppStoreVersionLocalizationLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppStoreVersionLocalizationsOption](200))
				firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, versionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("build-localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizations(ctx, versionID, asc.NextURL[asc.AppStoreVersionLocalizationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("build-localizations list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, versionID, opts...)
			if err != nil {
				return fmt.Errorf("build-localizations list: failed to fetch: %w", err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildLocalizationsGetCommand returns the get subcommand built on d.
func newBuildLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "id", "appStoreVersionLocalizations", "Localization ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-localizations view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("build-localizations view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildLocalizationsCreateCommand returns the create subcommand built on d.
func newBuildLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	whatsNew := fs.String("whats-new", "", "Release notes (whats new), up to 4000 characters")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--build-id")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return d.UsageErrorf("build-localizations create: %v", err)
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)

			attrs := asc.AppStoreVersionLocalizationAttributes{
				Locale: localeValue,
			}
			if whatsNewValue != "" {
				attrs.WhatsNew = whatsNewValue
			}
			if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-localizations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			versionID, err := resolveBuildAppStoreVersion(requestCtx, d, client, "create", build)
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

			if err := d.PrintOutput(resp, *output.Output, *output.Pretty); err != nil {
				return err
			}
			return shared.PrintSubmitReadinessCreateWarnings(d.IO.ErrOut, warnings)
		},
	}
}

// newBuildLocalizationsUpdateCommand returns the update subcommand built on d.
func newBuildLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "id", "appStoreVersionLocalizations", "Localization ID")
	whatsNew := fs.String("whats-new", "", "Release notes (whats new), up to 4000 characters")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}

			whatsNewValue := strings.TrimSpace(*whatsNew)
			if whatsNewValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("--whats-new")
			}

			attrs := asc.AppStoreVersionLocalizationAttributes{
				WhatsNew: whatsNewValue,
			}
			if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-localizations update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreVersionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("build-localizations update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildLocalizationsDeleteCommand returns the delete subcommand built on d.
func newBuildLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc build-localizations delete [flags]",
		ShortHelp:   "Delete a localization by ID.",
		LongHelp: `Delete a localization by ID.

Examples:
  asc build-localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Localization ID",
		IDType:             "appStoreVersionLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "build-localizations delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppStoreVersionLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppStoreVersionLocalizationDeleteResult{ID: id, Deleted: true}
		},
	})
}

// resolveBuildAppStoreVersion returns the App Store version a build is attached
// to. Apple answers GET /v1/builds/{id}/appStoreVersion with a null data object
// for a build that is not attached to any version, which is the normal state of
// a TestFlight-only build, so that case is reported as a usage error with
// guidance instead of a generic failure. API errors, including an unknown build
// ID, are returned unchanged so the service detail and classification survive.
func isVersionLocalizationsPage(nextURL, versionID string) bool {
	parsed, err := url.Parse(nextURL)
	if err != nil {
		return false
	}
	return parsed.Path == "/v1/appStoreVersions/"+url.PathEscape(versionID)+"/appStoreVersionLocalizations"
}

func resolveBuildAppStoreVersion(ctx context.Context, d *shared.Deps, client *asc.Client, subcommand, buildID string) (string, error) {
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
		fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", message)
		return "", shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message)
	}
	return resp.Data.ID, nil
}
