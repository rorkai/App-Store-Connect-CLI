package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// BuildsUploadsCommand returns the builds uploads command group.
func BuildsUploadsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("uploads", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "uploads",
		ShortUsage: "asc builds uploads <subcommand> [flags]",
		ShortHelp:  "Manage build uploads.",
		LongHelp: `Manage build uploads.

Examples:
  asc builds uploads list --app "APP_ID"
  asc builds uploads get --id "UPLOAD_ID"
  asc builds uploads delete --id "UPLOAD_ID" --confirm
  asc builds uploads files list --upload "UPLOAD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsUploadsListCommand(),
			BuildsUploadsGetCommand(),
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds uploads list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("builds uploads list: %w", err)
			}
			if err := validateSort(*sort, "cfBundleVersion", "-cfBundleVersion", "uploadedDate", "-uploadedDate"); err != nil {
				return fmt.Errorf("builds uploads list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			platforms, err := normalizeAppStoreVersionPlatforms(splitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("builds uploads list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildUploadsOption{
				asc.WithBuildUploadsLimit(*limit),
				asc.WithBuildUploadsNextURL(*next),
				asc.WithBuildUploadsCFBundleShortVersionStrings(splitCSV(*shortVersion)),
				asc.WithBuildUploadsCFBundleVersions(splitCSV(*bundleVersion)),
				asc.WithBuildUploadsPlatforms(platforms),
				asc.WithBuildUploadsStates(splitCSVUpper(*state)),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithBuildUploadsSort(*sort))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBuildUploadsLimit(200))
				firstPage, err := client.GetBuildUploads(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("builds uploads list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildUploads(ctx, resolvedAppID, asc.WithBuildUploadsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("builds uploads list: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBuildUploads(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("builds uploads list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BuildsUploadsGetCommand returns the builds uploads get subcommand.
func BuildsUploadsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("uploads get", flag.ExitOnError)

	id := fs.String("id", "", "Build upload ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds uploads get --id \"UPLOAD_ID\"",
		ShortHelp:  "Get a build upload by ID.",
		LongHelp: `Get a build upload by ID.

Examples:
  asc builds uploads get --id "UPLOAD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			uploadID := strings.TrimSpace(*id)
			if uploadID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildUpload(requestCtx, uploadID)
			if err != nil {
				return fmt.Errorf("builds uploads get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BuildsUploadsDeleteCommand returns the builds uploads delete subcommand.
func BuildsUploadsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("uploads delete", flag.ExitOnError)

	id := fs.String("id", "", "Build upload ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc builds uploads delete --id \"UPLOAD_ID\" --confirm",
		ShortHelp:  "Delete a build upload by ID.",
		LongHelp: `Delete a build upload by ID.

Examples:
  asc builds uploads delete --id "UPLOAD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			uploadID := strings.TrimSpace(*id)
			if uploadID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBuildUpload(requestCtx, uploadID); err != nil {
				return fmt.Errorf("builds uploads delete: failed to delete: %w", err)
			}

			result := &asc.BuildUploadDeleteResult{
				ID:      uploadID,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

// BuildsUploadFilesCommand returns the builds upload files command group.
func BuildsUploadFilesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("files", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "files",
		ShortUsage: "asc builds uploads files <subcommand> [flags]",
		ShortHelp:  "Manage build upload files.",
		LongHelp: `Manage build upload files.

Examples:
  asc builds uploads files list --upload "UPLOAD_ID"
  asc builds uploads files get --id "FILE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsUploadFilesListCommand(),
			BuildsUploadFilesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsUploadFilesListCommand returns the build upload files list subcommand.
func BuildsUploadFilesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("files list", flag.ExitOnError)

	uploadID := fs.String("upload", "", "Build upload ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds uploads files list [flags]",
		ShortHelp:  "List build upload files for a build upload.",
		LongHelp: `List build upload files for a build upload.

Examples:
  asc builds uploads files list --upload "UPLOAD_ID"
  asc builds uploads files list --upload "UPLOAD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds uploads files list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("builds uploads files list: %w", err)
			}

			uploadValue := strings.TrimSpace(*uploadID)
			if uploadValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --upload is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads files list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildUploadFilesOption{
				asc.WithBuildUploadFilesLimit(*limit),
				asc.WithBuildUploadFilesNextURL(*next),
			}

			if *paginate {
				if uploadValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --upload is required")
					return flag.ErrHelp
				}

				paginateOpts := append(opts, asc.WithBuildUploadFilesLimit(200))
				firstPage, err := client.GetBuildUploadFiles(requestCtx, uploadValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("builds uploads files list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildUploadFiles(ctx, uploadValue, asc.WithBuildUploadFilesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("builds uploads files list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBuildUploadFiles(requestCtx, uploadValue, opts...)
			if err != nil {
				return fmt.Errorf("builds uploads files list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BuildsUploadFilesGetCommand returns the build upload files get subcommand.
func BuildsUploadFilesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("files get", flag.ExitOnError)

	id := fs.String("id", "", "Build upload file ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds uploads files get --id \"FILE_ID\"",
		ShortHelp:  "Get a build upload file by ID.",
		LongHelp: `Get a build upload file by ID.

Examples:
  asc builds uploads files get --id "FILE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fileID := strings.TrimSpace(*id)
			if fileID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds uploads files get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildUploadFile(requestCtx, fileID)
			if err != nil {
				return fmt.Errorf("builds uploads files get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
