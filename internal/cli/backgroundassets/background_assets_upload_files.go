package backgroundassets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BackgroundAssetsUploadFilesCommand returns the upload files command group.
func BackgroundAssetsUploadFilesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload-files", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "upload-files",
		ShortUsage: "asc background-assets upload-files <subcommand> [flags]",
		ShortHelp:  "Manage background asset upload files.",
		LongHelp: `Manage background asset upload files.

Examples:
  asc background-assets upload-files list --version-id "VERSION_ID"
  asc background-assets upload-files get --upload-file-id "UPLOAD_FILE_ID"
  asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.zip" --asset-type ASSET
  asc background-assets upload-files update --upload-file-id "UPLOAD_FILE_ID" --uploaded true --file "./asset.zip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BackgroundAssetsUploadFilesListCommand(),
			BackgroundAssetsUploadFilesGetCommand(),
			BackgroundAssetsUploadFilesCreateCommand(),
			BackgroundAssetsUploadFilesUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BackgroundAssetsUploadFilesListCommand returns the upload files list subcommand.
func BackgroundAssetsUploadFilesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Background asset version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc background-assets upload-files list --version-id \"VERSION_ID\"",
		ShortHelp:  "List upload files for a background asset version.",
		LongHelp: `List upload files for a background asset version.

Examples:
  asc background-assets upload-files list --version-id "VERSION_ID"
  asc background-assets upload-files list --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionIDValue := strings.TrimSpace(*versionID)
			if versionIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > backgroundAssetsMaxLimit) {
				return fmt.Errorf("background-assets upload-files list: --limit must be between 1 and %d", backgroundAssetsMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("background-assets upload-files list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets upload-files list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BackgroundAssetUploadFilesOption{
				asc.WithBackgroundAssetUploadFilesLimit(*limit),
				asc.WithBackgroundAssetUploadFilesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBackgroundAssetUploadFilesLimit(backgroundAssetsMaxLimit))
				firstPage, err := client.GetBackgroundAssetUploadFiles(requestCtx, versionIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("background-assets upload-files list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBackgroundAssetUploadFiles(ctx, versionIDValue, asc.WithBackgroundAssetUploadFilesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("background-assets upload-files list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBackgroundAssetUploadFiles(requestCtx, versionIDValue, opts...)
			if err != nil {
				return fmt.Errorf("background-assets upload-files list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsUploadFilesGetCommand returns the upload files get subcommand.
func BackgroundAssetsUploadFilesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	uploadFileID := fs.String("upload-file-id", "", "Background asset upload file ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc background-assets upload-files get --upload-file-id \"UPLOAD_FILE_ID\"",
		ShortHelp:  "Get a background asset upload file by ID.",
		LongHelp: `Get a background asset upload file by ID.

Examples:
  asc background-assets upload-files get --upload-file-id "UPLOAD_FILE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			uploadFileIDValue := strings.TrimSpace(*uploadFileID)
			if uploadFileIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --upload-file-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets upload-files get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBackgroundAssetUploadFile(requestCtx, uploadFileIDValue)
			if err != nil {
				return fmt.Errorf("background-assets upload-files get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BackgroundAssetsUploadFilesCreateCommand returns the upload files create subcommand.
func BackgroundAssetsUploadFilesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Background asset version ID")
	filePath := fs.String("file", "", "Path to upload file")
	assetType := fs.String("asset-type", "", "Asset type: "+strings.Join(backgroundAssetUploadFileAssetTypeValues, ", "))
	checksum := fs.Bool("checksum", false, "Verify source file checksums before committing")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc background-assets upload-files create --version-id \"VERSION_ID\" --file \"./asset.zip\" --asset-type ASSET",
		ShortHelp:  "Create and upload a background asset file.",
		LongHelp: `Create and upload a background asset file.

Examples:
  asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.zip" --asset-type ASSET
  asc background-assets upload-files create --version-id "VERSION_ID" --file "./manifest.json" --asset-type MANIFEST --checksum`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionIDValue := strings.TrimSpace(*versionID)
			if versionIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*assetType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --asset-type is required")
				return flag.ErrHelp
			}

			typeValue, err := normalizeBackgroundAssetUploadFileAssetType(*assetType)
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: %w", err)
			}

			info, err := os.Lstat(pathValue)
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: %w", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("background-assets upload-files create: refusing to read symlink %q", pathValue)
			}
			if info.IsDir() {
				return fmt.Errorf("background-assets upload-files create: %q is a directory", pathValue)
			}
			if info.Size() <= 0 {
				return fmt.Errorf("background-assets upload-files create: file size must be greater than 0")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBackgroundAssetUploadFile(requestCtx, versionIDValue, filepath.Base(pathValue), info.Size(), typeValue)
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("background-assets upload-files create: no upload operations returned")
			}

			uploadCtx, uploadCancel := shared.ContextWithUploadTimeout(ctx)
			err = asc.ExecuteUploadOperations(uploadCtx, pathValue, resp.Data.Attributes.UploadOperations)
			uploadCancel()
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: upload failed: %w", err)
			}

			var checksums *asc.Checksums
			sourceChecksums := resp.Data.Attributes.SourceFileChecksums
			if *checksum {
				if sourceChecksums == nil || (sourceChecksums.File == nil && sourceChecksums.Composite == nil) {
					fmt.Fprintln(os.Stderr, "Warning: --checksum requested but API provided no checksums to verify; skipping")
				} else {
					computed, err := asc.VerifySourceFileChecksums(pathValue, sourceChecksums)
					if err != nil {
						return fmt.Errorf("background-assets upload-files create: checksum verification failed: %w", err)
					}
					checksums = computed
				}
			} else if sourceChecksums != nil {
				checksums = sourceChecksums
			}

			uploaded := true
			updateAttrs := asc.BackgroundAssetUploadFileUpdateAttributes{
				SourceFileChecksums: checksums,
				Uploaded:            &uploaded,
			}

			commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
			commitResp, err := client.UpdateBackgroundAssetUploadFile(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: failed to commit upload: %w", err)
			}

			return shared.PrintOutput(commitResp, *output, *pretty)
		},
	}
}

// BackgroundAssetsUploadFilesUpdateCommand returns the upload files update subcommand.
func BackgroundAssetsUploadFilesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	uploadFileID := fs.String("upload-file-id", "", "Background asset upload file ID")
	uploaded := fs.String("uploaded", "", "Mark upload as complete (true/false)")
	filePath := fs.String("file", "", "Path to file for checksum verification")
	checksum := fs.Bool("checksum", false, "Verify source file checksums before committing")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc background-assets upload-files update --upload-file-id \"UPLOAD_FILE_ID\" --uploaded true",
		ShortHelp:  "Update a background asset upload file.",
		LongHelp: `Update a background asset upload file.

Examples:
  asc background-assets upload-files update --upload-file-id "UPLOAD_FILE_ID" --uploaded true
  asc background-assets upload-files update --upload-file-id "UPLOAD_FILE_ID" --uploaded true --file "./asset.zip" --checksum`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			uploadFileIDValue := strings.TrimSpace(*uploadFileID)
			if uploadFileIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --upload-file-id is required")
				return flag.ErrHelp
			}

			uploadedValue := strings.TrimSpace(*uploaded)
			if uploadedValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --uploaded is required")
				return flag.ErrHelp
			}
			uploadedBool, err := parseBool(uploadedValue, "--uploaded")
			if err != nil {
				return fmt.Errorf("background-assets upload-files update: %w", err)
			}

			pathValue := strings.TrimSpace(*filePath)
			if *checksum && pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --checksum requires --file")
				return flag.ErrHelp
			}

			if pathValue != "" {
				info, err := os.Lstat(pathValue)
				if err != nil {
					return fmt.Errorf("background-assets upload-files update: %w", err)
				}
				if info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("background-assets upload-files update: refusing to read symlink %q", pathValue)
				}
				if info.IsDir() {
					return fmt.Errorf("background-assets upload-files update: %q is a directory", pathValue)
				}
				if info.Size() <= 0 {
					return fmt.Errorf("background-assets upload-files update: file size must be greater than 0")
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("background-assets upload-files update: %w", err)
			}

			var checksums *asc.Checksums
			if pathValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.GetBackgroundAssetUploadFile(requestCtx, uploadFileIDValue)
				if err != nil {
					return fmt.Errorf("background-assets upload-files update: failed to fetch: %w", err)
				}

				sourceChecksums := resp.Data.Attributes.SourceFileChecksums
				if *checksum {
					if sourceChecksums == nil || (sourceChecksums.File == nil && sourceChecksums.Composite == nil) {
						fmt.Fprintln(os.Stderr, "Warning: --checksum requested but API provided no checksums to verify; skipping")
					} else {
						computed, err := asc.VerifySourceFileChecksums(pathValue, sourceChecksums)
						if err != nil {
							return fmt.Errorf("background-assets upload-files update: checksum verification failed: %w", err)
						}
						checksums = computed
					}
				} else if sourceChecksums != nil {
					checksums = sourceChecksums
				}
			}

			updateAttrs := asc.BackgroundAssetUploadFileUpdateAttributes{
				SourceFileChecksums: checksums,
				Uploaded:            &uploadedBool,
			}

			commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
			commitResp, err := client.UpdateBackgroundAssetUploadFile(commitCtx, uploadFileIDValue, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("background-assets upload-files update: failed to update: %w", err)
			}

			return shared.PrintOutput(commitResp, *output, *pretty)
		},
	}
}
