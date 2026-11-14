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

// BuildsUploadsCommand returns the builds uploads command group.
func BuildsUploadsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("uploads", flag.ExitOnError)
	viewCmd := BuildsUploadsViewCommand()

	return &ffcli.Command{
		Name:       "uploads",
		ShortUsage: "asc builds uploads <subcommand> [flags]",
		ShortHelp:  "Manage build uploads.",
		LongHelp: `Manage build uploads.

Examples:
  asc builds uploads list --app "APP_ID"
  asc builds uploads view --id "UPLOAD_ID"
  asc builds uploads delete --id "UPLOAD_ID" --confirm
  asc builds uploads files list --upload "UPLOAD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsUploadsListCommand(),
			viewCmd,
			BuildsUploadsDeleteCommand(),
			BuildsUploadFilesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsUploadsListCommand returns the builds uploads list subcommand.
func BuildsUploadsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("uploads list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	shortVersion := fs.String("cf-bundle-short-version", "", "Filter by CFBundleShortVersionString(s), comma-separated")
	bundleVersion := fs.String("cf-bundle-version", "", "Filter by CFBundleVersion(s), comma-separated")
	platform := fs.String("platform", "", "Filter by platform(s): IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by upload state(s), comma-separated")
	sort := fs.String("sort", "", "Sort by cfBundleVersion, -cfBundleVersion, uploadedDate, -uploadedDate")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds uploads list [flags]",
		ShortHelp:  "List build uploads for an app.",
		LongHelp: `List build uploads for an app.

Examples:
  asc builds uploads list --app "APP_ID"
  asc builds uploads list --app "APP_ID" --cf-bundle-short-version "1.0.0"
  asc builds uploads list --app "APP_ID" --platform "IOS" --sort "-uploadedDate"
  asc builds uploads list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("builds uploads list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(fs, *next, "builds uploads list", "app", "cf-bundle-short-version", "cf-bundle-version", "platform", "state", "sort"); err != nil {
				return err
			}
			if err := shared.ValidateSort(*sort, "cfBundleVersion", "-cfBundleVersion", "uploadedDate", "-uploadedDate"); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return shared.UsageErrorf("builds uploads list: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildUploadsOption{
				asc.Limit[asc.BuildUploadsOption](*limit),
				asc.NextURL[asc.BuildUploadsOption](*next),
				asc.WithBuildUploadsCFBundleShortVersionStrings(shared.SplitCSV(*shortVersion)),
				asc.WithBuildUploadsCFBundleVersions(shared.SplitCSV(*bundleVersion)),
				asc.WithBuildUploadsPlatforms(platforms),
				asc.WithBuildUploadsStates(shared.SplitCSVUpper(*state)),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithBuildUploadsSort(*sort))
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BuildUploadsOption](200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBuildUploads(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBuildUploads(ctx, resolvedAppID, asc.NextURL[asc.BuildUploadsOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds uploads list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildUploads(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("builds uploads list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsUploadsViewCommand returns the builds uploads view subcommand.
func BuildsUploadsViewCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "uploads view",
		Name:        "view",
		ShortUsage:  "asc builds uploads view --id \"UPLOAD_ID\"",
		ShortHelp:   "View a build upload by ID.",
		LongHelp: `View a build upload by ID.

Examples:
  asc builds uploads view --id "UPLOAD_ID"`,
		IDUsage:            "Build upload ID",
		IDType:             "buildUploads",
		RequiredDiagnostic: true,
		ErrorPrefix:        "builds uploads view",
		FetchErrorPrefix:   "builds uploads view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBuildUpload(ctx, id)
		},
	})
}

// BuildsUploadsDeleteCommand returns the builds uploads delete subcommand.
func BuildsUploadsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("uploads delete", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "buildUploads", "Build upload ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc builds uploads delete --id \"UPLOAD_ID\" --confirm",
		ShortHelp:  "Delete a build upload by ID.",
		LongHelp: `Delete a build upload by ID.

Examples:
  asc builds uploads delete --id "UPLOAD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			uploadID := strings.TrimSpace(*id)
			if uploadID == "" {
				return shared.RequiredFlag("--id")
			}
			if !*confirm {
				return shared.RequiredFlag("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBuildUpload(requestCtx, uploadID); err != nil {
				return fmt.Errorf("builds uploads delete: failed to delete: %w", err)
			}

			result := &asc.BuildUploadDeleteResult{
				ID:      uploadID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BuildsUploadFilesCommand returns the builds upload files command group.
func BuildsUploadFilesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("files", flag.ExitOnError)
	viewCmd := BuildsUploadFilesViewCommand()

	return &ffcli.Command{
		Name:       "files",
		ShortUsage: "asc builds uploads files <subcommand> [flags]",
		ShortHelp:  "Manage build upload files.",
		LongHelp: `Manage build upload files.

Examples:
  asc builds uploads files list --upload "UPLOAD_ID"
  asc builds uploads files view --id "FILE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsUploadFilesListCommand(),
			viewCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsUploadFilesListCommand returns the build upload files list subcommand.
func BuildsUploadFilesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("files list", flag.ExitOnError)

	uploadID := shared.BindResourceIDFlag(fs, "upload", "buildUploads", "Build upload ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds uploads files list [flags]",
		ShortHelp:  "List build upload files for a build upload.",
		LongHelp: `List build upload files for a build upload.

Examples:
  asc builds uploads files list --upload "UPLOAD_ID"
  asc builds uploads files list --upload "UPLOAD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("builds uploads files list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("builds uploads files list: %v", err)
			}

			uploadValue := strings.TrimSpace(*uploadID)
			if uploadValue == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--upload")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads files list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildUploadFilesOption{
				asc.Limit[asc.BuildUploadFilesOption](*limit),
				asc.NextURL[asc.BuildUploadFilesOption](*next),
			}

			if *paginate {
				if uploadValue == "" {
					return shared.RequiredFlag("--upload")
				}

				paginateOpts := append(opts, asc.Limit[asc.BuildUploadFilesOption](200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBuildUploadFiles(ctx, uploadValue, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBuildUploadFiles(ctx, uploadValue, asc.NextURL[asc.BuildUploadFilesOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds uploads files list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildUploadFiles(requestCtx, uploadValue, opts...)
			if err != nil {
				return fmt.Errorf("builds uploads files list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsUploadFilesViewCommand returns the build upload files view subcommand.
func BuildsUploadFilesViewCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "files view",
		Name:        "view",
		ShortUsage:  "asc builds uploads files view --id \"FILE_ID\"",
		ShortHelp:   "View a build upload file by ID.",
		LongHelp: `View a build upload file by ID.

Examples:
  asc builds uploads files view --id "FILE_ID"`,
		IDUsage:            "Build upload file ID",
		IDType:             "buildUploadFiles",
		RequiredDiagnostic: true,
		ErrorPrefix:        "builds uploads files view",
		FetchErrorPrefix:   "builds uploads files view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBuildUploadFile(ctx, id)
		},
	})
}
