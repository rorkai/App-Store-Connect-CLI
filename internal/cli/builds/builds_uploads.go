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

// newBuildsUploadsCommand returns the builds uploads command group.
func newBuildsUploadsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("uploads", flag.ExitOnError)
	viewCmd := newBuildsUploadsViewCommand(d)

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
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildsUploadsListCommand(d),
			viewCmd,
			newBuildsUploadsDeleteCommand(d),
			newBuildsUploadFilesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildsUploadsListCommand returns the builds uploads list subcommand.
func newBuildsUploadsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("uploads list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	shortVersion := fs.String("cf-bundle-short-version", "", "Filter by CFBundleShortVersionString(s), comma-separated")
	bundleVersion := fs.String("cf-bundle-version", "", "Filter by CFBundleVersion(s), comma-separated")
	platform := fs.String("platform", "", "Filter by platform(s): IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by upload state(s), comma-separated")
	sort := fs.String("sort", "", "Sort by cfBundleVersion, -cfBundleVersion, uploadedDate, -uploadedDate")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "builds uploads list", 200, "cf-bundle-short-version", "cf-bundle-version", "platform", "state", "sort"); err != nil {
				return err
			}

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			if err := shared.ValidateSort(*sort, "cfBundleVersion", "-cfBundleVersion", "uploadedDate", "-uploadedDate"); err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return d.UsageErrorf("builds uploads list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds uploads list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
				resp, err := d.PaginateWithSpinner(
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
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildUploads(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("builds uploads list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsUploadsViewCommand returns the builds uploads view subcommand.
func newBuildsUploadsViewCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBuildUpload(ctx, id)
		},
	})
}

// newBuildsUploadsDeleteCommand returns the builds uploads delete subcommand.
func newBuildsUploadsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "uploads delete",
		Name:        "delete",
		ShortUsage:  "asc builds uploads delete --id \"UPLOAD_ID\" --confirm",
		ShortHelp:   "Delete a build upload by ID.",
		LongHelp: `Delete a build upload by ID.

Examples:
  asc builds uploads delete --id "UPLOAD_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Build upload ID",
		IDType:             "buildUploads",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBuildUpload(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newBuildsUploadFilesCommand returns the builds upload files command group.
func newBuildsUploadFilesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("files", flag.ExitOnError)
	viewCmd := newBuildsUploadFilesViewCommand(d)

	return &ffcli.Command{
		Name:       "files",
		ShortUsage: "asc builds uploads files <subcommand> [flags]",
		ShortHelp:  "Manage build upload files.",
		LongHelp: `Manage build upload files.

Examples:
  asc builds uploads files list --upload "UPLOAD_ID"
  asc builds uploads files view --id "FILE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildsUploadFilesListCommand(d),
			viewCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildsUploadFilesListCommand returns the build upload files list subcommand.
func newBuildsUploadFilesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("files list", flag.ExitOnError)

	uploadID := shared.BindResourceIDFlag(fs, "upload", "buildUploads", "Build upload ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds uploads files list [flags]",
		ShortHelp:  "List build upload files for a build upload.",
		LongHelp: `List build upload files for a build upload.

Examples:
  asc builds uploads files list --upload "UPLOAD_ID"
  asc builds uploads files list --upload "UPLOAD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "builds uploads files list", 200); err != nil {
				return err
			}

			uploadValue := strings.TrimSpace(*uploadID)
			if uploadValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--upload")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds uploads files list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BuildUploadFilesOption{
				asc.Limit[asc.BuildUploadFilesOption](*limit),
				asc.NextURL[asc.BuildUploadFilesOption](*next),
			}

			if *paginate {
				if uploadValue == "" {
					return d.RequiredFlag("--upload")
				}

				paginateOpts := append(opts, asc.Limit[asc.BuildUploadFilesOption](200))
				resp, err := d.PaginateWithSpinner(
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

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildUploadFiles(requestCtx, uploadValue, opts...)
			if err != nil {
				return fmt.Errorf("builds uploads files list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsUploadFilesViewCommand returns the build upload files view subcommand.
func newBuildsUploadFilesViewCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBuildUploadFile(ctx, id)
		},
	})
}
