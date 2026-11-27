package productpages

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type customPageScreenshotSyncOptions struct {
	Replace     bool
	Confirm     bool
	Incremental bool
	DryRun      bool
}

const customPageScreenshotSetsListCommand = "asc product-pages custom-pages localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\" --include-screenshots --paginate"

// newCustomPageLocalizationsScreenshotSetsUploadCommand returns the screenshot sets upload subcommand.
func newCustomPageLocalizationsScreenshotSetsUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations screenshot-sets upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	uploadFlags := assets.BindScreenshotSetUploadFlags(fs, customPageScreenshotSetsListCommand)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets upload --localization-id \"LOCALIZATION_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for a custom product page localization.",
		LongHelp: `Upload screenshots for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc product-pages custom-pages localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			ctx, err := uploadFlags.Apply(ctx, "custom-pages localizations screenshot-sets upload")
			if err != nil {
				return err
			}
			result, err := executeCustomPageScreenshotUpload(ctx, d, *localizationID, *path, *deviceType, customPageScreenshotSyncOptions{})
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets upload: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsScreenshotSetsSyncCommand returns the screenshot sets sync subcommand.
func newCustomPageLocalizationsScreenshotSetsSyncCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations screenshot-sets sync", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	confirm := fs.Bool("confirm", false, "Confirm sync deletions")
	incremental := fs.Bool("incremental", false, "Keep matching complete screenshots, upload missing files, delete obsolete assets, and restore local file order")
	dryRun := fs.Bool("dry-run", false, "Preview incremental sync without changing remote assets (requires --incremental)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	uploadFlags := assets.BindScreenshotSetUploadFlags(fs, customPageScreenshotSetsListCommand)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets sync --localization-id \"LOCALIZATION_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\" (--confirm [--incremental] | --incremental --dry-run)",
		ShortHelp:  "Sync screenshots for a custom product page localization.",
		LongHelp: `Sync screenshots for a custom product page localization.

This replaces existing screenshots in the matching display-type set with files from --path.

With --incremental, matching COMPLETE screenshots keep their asset IDs. Only
obsolete screenshots are deleted, missing files are uploaded, and filename order
is restored. Preview the diff with --incremental --dry-run, then apply it with
--incremental --confirm. Incomplete assets or ambiguous matches stop the sync.
Missing files are uploaded and verified before obsolete assets are deleted when
space permits. A full set clears only the slots needed for those uploads first.
Incremental sync waits for processing and cannot use --no-wait.

Examples:
  asc product-pages custom-pages localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --confirm
  asc product-pages custom-pages localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --incremental --dry-run
  asc product-pages custom-pages localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --incremental --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *dryRun && !*incremental {
				return d.UsageError("--dry-run requires --incremental")
			}
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			ctx, err := uploadFlags.Apply(ctx, "custom-pages localizations screenshot-sets sync")
			if err != nil {
				return err
			}

			result, err := executeCustomPageScreenshotUpload(ctx, d, *localizationID, *path, *deviceType, customPageScreenshotSyncOptions{Replace: true, Confirm: *confirm, Incremental: *incremental, DryRun: *dryRun})
			if result != nil {
				if printErr := d.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets sync: %w", err)
			}
			return nil
		},
	}
}

// newCustomPageLocalizationsPreviewSetsUploadCommand returns the preview sets upload subcommand.
func newCustomPageLocalizationsPreviewSetsUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations preview-sets upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets upload --localization-id \"LOCALIZATION_ID\" --path \"./previews\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload previews for a custom product page localization.",
		LongHelp: `Upload previews for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65"
  asc product-pages custom-pages localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews/en-US.mov" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			result, err := executeCustomPagePreviewUpload(ctx, d, *localizationID, *path, *deviceType, false, false)
			if result != nil {
				if printErr := d.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets upload: %w", err)
			}
			return nil
		},
	}
}

// newCustomPageLocalizationsPreviewSetsSyncCommand returns the preview sets sync subcommand.
func newCustomPageLocalizationsPreviewSetsSyncCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations preview-sets sync", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	confirm := fs.Bool("confirm", false, "Confirm sync (deletes existing media in the matching set before upload)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets sync --localization-id \"LOCALIZATION_ID\" --path \"./previews\" --device-type \"IPHONE_65\" --confirm",
		ShortHelp:  "Sync previews for a custom product page localization.",
		LongHelp: `Sync previews for a custom product page localization.

This replaces existing previews in the matching preview-type set with files from --path.

Examples:
  asc product-pages custom-pages localizations preview-sets sync --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			result, err := executeCustomPagePreviewUpload(ctx, d, *localizationID, *path, *deviceType, true, *confirm)
			if result != nil {
				if printErr := d.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets sync: %w", err)
			}
			return nil
		},
	}
}

func executeCustomPageScreenshotUpload(
	ctx context.Context,
	d *shared.Deps,
	localizationID, path, deviceType string,
	options customPageScreenshotSyncOptions,
) (*asc.CustomProductPageScreenshotUploadResult, error) {
	trimmedLocalizationID := strings.TrimSpace(localizationID)
	trimmedPath := strings.TrimSpace(path)
	trimmedDeviceType := strings.TrimSpace(deviceType)
	return assets.ExecuteScreenshotSetUpload(ctx, d, assets.ScreenshotSetUploadOptions[*asc.CustomProductPageScreenshotUploadResult]{
		LocalizationID: localizationID,
		Path:           path,
		DeviceType:     deviceType,
		Replace:        options.Replace,
		Incremental:    options.Incremental,
		DryRun:         options.DryRun,
		RequireConfirm: func() error { return d.RequireConfirm(options.Confirm) },
		InspectCommand: fmt.Sprintf("asc product-pages custom-pages localizations screenshot-sets list --localization-id %q --include-screenshots --paginate --output json", trimmedLocalizationID),
		ReplaceCommand: shellquote.Join("asc", "product-pages", "custom-pages", "localizations", "screenshot-sets", "sync", "--localization-id", trimmedLocalizationID, "--path", trimmedPath, "--device-type", trimmedDeviceType, "--confirm"),
		ClientFactory:  d.Client,
		RequestContext: contextWithCustomPageMediaUploadTimeout,
		UploadContext:  contextWithCustomPageMediaUploadTimeout,
		Access: assets.ScreenshotSetAccess{
			List: func(ctx context.Context, client *asc.Client, localizationID string, requestContext asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error) {
				return client.GetAllAppCustomProductPageLocalizationScreenshotSets(ctx, localizationID, asc.WithAppCustomProductPageLocalizationScreenshotSetsRequestContext(requestContext))
			},
			Create: func(ctx context.Context, client *asc.Client, localizationID, displayType string) (*asc.AppScreenshotSetResponse, error) {
				return client.CreateAppScreenshotSetForCustomProductPageLocalization(ctx, localizationID, displayType)
			},
		},
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], results []asc.AssetUploadResultItem) *asc.CustomProductPageScreenshotUploadResult {
			return &asc.CustomProductPageScreenshotUploadResult{
				CustomProductPageLocalizationID: localizationID,
				SetID:                           set.ID,
				DisplayType:                     set.Attributes.ScreenshotDisplayType,
				Results:                         results,
			}
		},
	})
}

func executeCustomPagePreviewUpload(
	ctx context.Context,
	d *shared.Deps,
	localizationID, path, deviceType string,
	sync, confirm bool,
) (*asc.CustomProductPagePreviewUploadResult, error) {
	upload, err := uploadLocalizationPreviews(ctx, d, localizationPreviewUploadOptions{
		localizationID: localizationID,
		path:           path,
		deviceType:     deviceType,
		sync:           sync,
		confirm:        confirm,
		access: localizationPreviewSetAccess{
			list: func(ctx context.Context, client *asc.Client, localizationID string) (*asc.AppPreviewSetsResponse, error) {
				return client.GetAppCustomProductPageLocalizationPreviewSets(ctx, localizationID)
			},
			create: func(ctx context.Context, client *asc.Client, localizationID, previewType string) (*asc.AppPreviewSetResponse, error) {
				return client.CreateAppPreviewSetForCustomProductPageLocalization(ctx, localizationID, previewType)
			},
		},
	})
	if upload == nil {
		return nil, err
	}
	return &asc.CustomProductPagePreviewUploadResult{
		CustomProductPageLocalizationID: upload.localizationID,
		SetID:                           upload.set.ID,
		PreviewType:                     upload.set.Attributes.PreviewType,
		Results:                         upload.results,
		Failures:                        upload.failures,
	}, err
}

type localizationPreviewSetAccess struct {
	list   func(ctx context.Context, client *asc.Client, localizationID string) (*asc.AppPreviewSetsResponse, error)
	create func(ctx context.Context, client *asc.Client, localizationID, previewType string) (*asc.AppPreviewSetResponse, error)
}

type localizationPreviewUploadOptions struct {
	localizationID           string
	path                     string
	deviceType               string
	sync                     bool
	confirm                  bool
	invalidDeviceTypeIsUsage bool
	access                   localizationPreviewSetAccess
}

type localizationPreviewUpload struct {
	localizationID string
	set            asc.Resource[asc.AppPreviewSetAttributes]
	results        []asc.AssetUploadResultItem
	failures       []asc.AssetUploadFailureItem
}

func uploadLocalizationPreviews(ctx context.Context, d *shared.Deps, opts localizationPreviewUploadOptions) (*localizationPreviewUpload, error) {
	trimmedLocalizationID := strings.TrimSpace(opts.localizationID)
	if trimmedLocalizationID == "" {
		return nil, d.RequiredFlag("--localization-id")
	}
	trimmedPath := strings.TrimSpace(opts.path)
	if trimmedPath == "" {
		return nil, d.RequiredFlag("--path")
	}
	trimmedDeviceType := strings.TrimSpace(opts.deviceType)
	if trimmedDeviceType == "" {
		return nil, d.RequiredFlag("--device-type")
	}
	if opts.sync {
		if err := d.RequireConfirm(opts.confirm); err != nil {
			return nil, err
		}
	}

	previewType, err := assets.NormalizePreviewType(trimmedDeviceType)
	if err != nil {
		if opts.invalidDeviceTypeIsUsage {
			return nil, d.UsageError(err.Error())
		}
		return nil, assets.InvalidDeviceTypeError(err)
	}
	files, err := collectCustomPageMediaFiles(trimmedPath)
	if err != nil {
		return nil, assets.ClassifyMediaInputError(err, "--path")
	}
	if err := assets.ValidatePreviewFiles(files, true); err != nil {
		return nil, assets.ClassifyMediaInputError(err, "--path")
	}

	client, err := d.Client()
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := contextWithCustomPageMediaUploadTimeout(ctx)
	defer cancel()

	set, setCreated, err := ensureLocalizationPreviewSet(requestCtx, client, opts.access, trimmedLocalizationID, previewType)
	if err != nil {
		return nil, err
	}
	if opts.sync {
		if err := deleteAllPreviewsInSet(requestCtx, client, set.ID); err != nil {
			return nil, err
		}
	}

	results, failures, err := assets.UploadPreviewFiles(requestCtx, ctx, client, set.ID, setCreated, files)

	return &localizationPreviewUpload{
		localizationID: trimmedLocalizationID,
		set:            set,
		results:        results,
		failures:       failures,
	}, err
}

func contextWithCustomPageMediaUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return assets.ContextWithAssetUploadTimeout(ctx)
}

func collectCustomPageMediaFiles(path string) ([]string, error) {
	return assets.CollectAssetFiles(path)
}

func ensureLocalizationPreviewSet(ctx context.Context, client *asc.Client, access localizationPreviewSetAccess, localizationID, previewType string) (asc.Resource[asc.AppPreviewSetAttributes], bool, error) {
	resp, err := access.list(ctx, client, localizationID)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, false, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.PreviewType, previewType) {
			return set, false, nil
		}
	}
	created, err := access.create(ctx, client, localizationID, previewType)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, false, err
	}
	return created.Data, true, nil
}

func deleteAllPreviewsInSet(ctx context.Context, client *asc.Client, setID string) error {
	resp, err := client.GetAppPreviews(ctx, setID)
	if err != nil {
		return err
	}
	for _, preview := range resp.Data {
		if err := client.DeleteAppPreview(ctx, preview.ID); err != nil {
			return err
		}
	}
	return nil
}
