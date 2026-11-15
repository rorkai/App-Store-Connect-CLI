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

func newBackgroundAssetsUploadFilesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload-files", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "upload-files",
		ShortUsage: "asc background-assets upload-files <subcommand> [flags]",
		ShortHelp:  "Manage background asset upload files.",
		LongHelp: `Manage background asset upload files.

Examples:
  asc background-assets upload-files list --version-id "VERSION_ID"
  asc background-assets upload-files view --upload-file-id "UPLOAD_FILE_ID"
  asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.zip" --asset-type ASSET
  asc background-assets upload-files update --upload-file-id "UPLOAD_FILE_ID" --uploaded true --file "./asset.zip" --checksum`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBackgroundAssetsUploadFilesListCommand(d),
			newBackgroundAssetsUploadFilesGetCommand(d),
			newBackgroundAssetsUploadFilesCreateCommand(d),
			newBackgroundAssetsUploadFilesUpdateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newBackgroundAssetsUploadFilesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc background-assets upload-files list --version-id \"VERSION_ID\"",
		ShortHelp:  "List upload files for a background asset version.",
		LongHelp: `List upload files for a background asset version.

Examples:
  asc background-assets upload-files list --version-id "VERSION_ID"
  asc background-assets upload-files list --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Background asset version ID",
		ParentType:           "backgroundAssetVersions",
		RequiredDiagnostic:   true,
		LimitMax:             backgroundAssetsMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		FetchPage: func(ctx context.Context, client *asc.Client, versionID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetBackgroundAssetUploadFiles(ctx, versionID,
				asc.Limit[asc.BackgroundAssetUploadFilesOption](limit),
				asc.NextURL[asc.BackgroundAssetUploadFilesOption](next),
			)
		},
	})
}

func newBackgroundAssetsUploadFilesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc background-assets upload-files view --upload-file-id \"UPLOAD_FILE_ID\"",
		ShortHelp:  "View a background asset upload file by ID.",
		LongHelp: `View a background asset upload file by ID.

Examples:
  asc background-assets upload-files view --upload-file-id "UPLOAD_FILE_ID"`,
		IDFlag:             "upload-file-id",
		IDUsage:            "Background asset upload file ID",
		IDType:             "backgroundAssetUploadFiles",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetUploadFile(ctx, id)
		},
	})
}

func newBackgroundAssetsUploadFilesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "backgroundAssetVersions", "Background asset version ID")
	filePath := fs.String("file", "", "Path to upload file")
	assetType := fs.String("asset-type", "", "Asset type: "+strings.Join(backgroundAssetUploadFileAssetTypeValues, ", "))
	checksum := fs.Bool("checksum", false, "Verify source file checksums before committing")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--version-id")
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				return d.RequiredFlag("--file")
			}

			if strings.TrimSpace(*assetType) == "" {
				return d.RequiredFlag("--asset-type")
			}

			typeValue, err := normalizeBackgroundAssetUploadFileAssetType(*assetType)
			if err != nil {
				return d.UsageErrorf("background-assets upload-files create: %v", err)
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

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateBackgroundAssetUploadFile(requestCtx, versionIDValue, filepath.Base(pathValue), info.Size(), typeValue)
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("background-assets upload-files create: no upload operations returned")
			}

			uploadCtx, uploadCancel := d.UploadTimeout(ctx)
			err = asc.ExecuteUploadOperations(uploadCtx, pathValue, resp.Data.Attributes.UploadOperations)
			uploadCancel()
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: upload failed: %w", err)
			}

			var checksums *asc.Checksums
			sourceChecksums := resp.Data.Attributes.SourceFileChecksums
			if *checksum {
				if sourceChecksums == nil || (sourceChecksums.File == nil && sourceChecksums.Composite == nil) {
					fmt.Fprintln(d.IO.ErrOut, "Warning: --checksum requested but API provided no checksums to verify; skipping")
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

			commitCtx, commitCancel := d.UploadTimeout(ctx)
			commitResp, err := client.UpdateBackgroundAssetUploadFile(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("background-assets upload-files create: failed to commit upload: %w", err)
			}

			return d.PrintOutput(commitResp, *output.Output, *output.Pretty)
		},
	}
}

func newBackgroundAssetsUploadFilesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	uploadFileID := shared.BindResourceIDFlag(fs, "upload-file-id", "backgroundAssetUploadFiles", "Background asset upload file ID")
	uploaded := fs.String("uploaded", "", "Mark upload as complete (true/false)")
	filePath := fs.String("file", "", "Path to file for checksum verification (requires --checksum)")
	checksum := fs.Bool("checksum", false, "Verify source file checksums before committing")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--upload-file-id")
			}

			uploadedValue := strings.TrimSpace(*uploaded)
			if uploadedValue == "" {
				return d.RequiredFlag("--uploaded")
			}
			uploadedBool, err := shared.ParseBoolFlag(uploadedValue, "--uploaded")
			if err != nil {
				return d.UsageErrorf("background-assets upload-files update: %v", err)
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue != "" && !*checksum {
				return d.UsageError("--file requires --checksum")
			}
			if *checksum && pathValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --checksum requires --file")
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

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("background-assets upload-files update: %w", err)
			}

			var checksums *asc.Checksums
			if pathValue != "" {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				resp, err := client.GetBackgroundAssetUploadFile(requestCtx, uploadFileIDValue)
				if err != nil {
					return fmt.Errorf("background-assets upload-files update: failed to fetch: %w", err)
				}

				// --file requires --checksum, so verification always runs here.
				sourceChecksums := resp.Data.Attributes.SourceFileChecksums
				if sourceChecksums == nil || (sourceChecksums.File == nil && sourceChecksums.Composite == nil) {
					fmt.Fprintln(d.IO.ErrOut, "Warning: --checksum requested but API provided no checksums to verify; skipping")
				} else {
					computed, err := asc.VerifySourceFileChecksums(pathValue, sourceChecksums)
					if err != nil {
						return fmt.Errorf("background-assets upload-files update: checksum verification failed: %w", err)
					}
					checksums = computed
				}
			}

			updateAttrs := asc.BackgroundAssetUploadFileUpdateAttributes{
				SourceFileChecksums: checksums,
				Uploaded:            &uploadedBool,
			}

			commitCtx, commitCancel := d.UploadTimeout(ctx)
			commitResp, err := client.UpdateBackgroundAssetUploadFile(commitCtx, uploadFileIDValue, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("background-assets upload-files update: failed to update: %w", err)
			}

			return d.PrintOutput(commitResp, *output.Output, *output.Pretty)
		},
	}
}
