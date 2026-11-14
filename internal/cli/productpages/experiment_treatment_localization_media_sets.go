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

const treatmentScreenshotSetsListCommand = "asc product-pages experiments treatments localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\" --include-screenshots --paginate"

// newExperimentTreatmentLocalizationPreviewSetsCommand returns the preview sets command group.
func newExperimentTreatmentLocalizationPreviewSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc product-pages experiments treatments localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for a treatment localization.",
		LongHelp: `Manage preview sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations preview-sets list --localization-id "LOCALIZATION_ID"
  asc product-pages experiments treatments localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newExperimentTreatmentLocalizationPreviewSetsListCommand(d),
			newExperimentTreatmentLocalizationPreviewSetsUploadCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newExperimentTreatmentLocalizationPreviewSetsListCommand returns the preview sets list subcommand.
func newExperimentTreatmentLocalizationPreviewSetsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations preview-sets list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview sets for a treatment localization.",
		LongHelp: `List preview sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--localization-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("experiments treatments localizations preview-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("experiments treatments localizations preview-sets list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption{
				asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption](*limit),
				asc.NextURL[asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption](productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations preview-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(ctx, trimmedID, asc.NextURL[asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentLocalizationPreviewSetsUploadCommand returns the preview sets upload subcommand.
func newExperimentTreatmentLocalizationPreviewSetsUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations preview-sets upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc product-pages experiments treatments localizations preview-sets upload --localization-id \"LOCALIZATION_ID\" --path \"./previews\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload previews for a treatment localization.",
		LongHelp: `Upload previews for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65"
  asc product-pages experiments treatments localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews/en-US.mov" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			result, err := executeExperimentTreatmentLocalizationPreviewUpload(ctx, d, *localizationID, *path, *deviceType)
			if result != nil {
				if printErr := d.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets upload: %w", err)
			}
			return nil
		},
	}
}

// newExperimentTreatmentLocalizationScreenshotSetsCommand returns the screenshot sets command group.
func newExperimentTreatmentLocalizationScreenshotSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc product-pages experiments treatments localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for a treatment localization.",
		LongHelp: `Manage screenshot sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc product-pages experiments treatments localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc product-pages experiments treatments localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newExperimentTreatmentLocalizationScreenshotSetsListCommand(d),
			newExperimentTreatmentLocalizationScreenshotSetsUploadCommand(d),
			newExperimentTreatmentLocalizationScreenshotSetsSyncCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newExperimentTreatmentLocalizationScreenshotSetsListCommand returns the screenshot sets list subcommand.
func newExperimentTreatmentLocalizationScreenshotSetsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations screenshot-sets list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	includeScreenshots := fs.Bool("include-screenshots", false, "Include screenshot IDs and metadata for each set (requires --localization-id and --paginate)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot sets for a treatment localization.",
		LongHelp: `List screenshot sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc product-pages experiments treatments localizations screenshot-sets list --localization-id "LOCALIZATION_ID" --include-screenshots --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if *includeScreenshots {
				if trimmedID == "" {
					return d.RequiredFlag("--localization-id")
				}
				if trimmedNext != "" {
					return d.UsageError("experiments treatments localizations screenshot-sets list: --include-screenshots cannot be combined with --next")
				}
				if !*paginate {
					return d.UsageError("experiments treatments localizations screenshot-sets list: --include-screenshots requires --paginate")
				}
			}
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--localization-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("experiments treatments localizations screenshot-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("experiments treatments localizations screenshot-sets list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
			}

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption{
				asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption](*limit),
				asc.NextURL[asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption](*next),
			}
			if *includeScreenshots {
				opts = append(opts, asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsIncludeScreenshots())
			}

			if *paginate {
				paginateOpts := make([]asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption, 0, len(opts)+2)
				paginateOpts = append(paginateOpts, opts...)
				paginateOpts = append(
					paginateOpts,
					asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption](productPagesMaxLimit),
					asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRequestContext(d.Timeout),
				)
				resp, err := client.GetAllAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
				}

				if *includeScreenshots {
					result, err := screenshotSetListResult(ctx, d, client, trimmedID, resp)
					if err != nil {
						return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
					}
					return d.PrintOutput(result, *output.Output, *output.Pretty)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: failed to fetch: %w", err)
			}
			if *includeScreenshots {
				result, err := screenshotSetListResult(ctx, d, client, trimmedID, resp)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
				}
				return d.PrintOutput(result, *output.Output, *output.Pretty)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentLocalizationScreenshotSetsUploadCommand returns the screenshot sets upload subcommand.
func newExperimentTreatmentLocalizationScreenshotSetsUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations screenshot-sets upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	uploadFlags := assets.BindScreenshotSetUploadFlags(fs, treatmentScreenshotSetsListCommand)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc product-pages experiments treatments localizations screenshot-sets upload --localization-id \"LOCALIZATION_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for a treatment localization.",
		LongHelp: `Upload screenshots for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc product-pages experiments treatments localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			ctx, err := uploadFlags.Apply(ctx, "experiments treatments localizations screenshot-sets upload")
			if err != nil {
				return err
			}
			result, err := executeExperimentTreatmentLocalizationScreenshotUpload(ctx, d, *localizationID, *path, *deviceType, false)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets upload: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentLocalizationScreenshotSetsSyncCommand returns the screenshot sets sync subcommand.
func newExperimentTreatmentLocalizationScreenshotSetsSyncCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations screenshot-sets sync", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	confirm := fs.Bool("confirm", false, "Confirm sync (deletes existing media in the matching set before upload)")
	pollInterval := shared.BindMediaPollIntervalFlag(fs)
	uploadFlags := assets.BindScreenshotSetUploadFlags(fs, treatmentScreenshotSetsListCommand)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc product-pages experiments treatments localizations screenshot-sets sync --localization-id \"LOCALIZATION_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\" --confirm",
		ShortHelp:  "Sync screenshots for a treatment localization.",
		LongHelp: `Sync screenshots for a treatment localization.

This replaces existing screenshots in the matching display-type set with files from --path.

Examples:
  asc product-pages experiments treatments localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ctx, pollErr := pollInterval.Apply(ctx)
			if pollErr != nil {
				return pollErr
			}
			if !*confirm {
				fmt.Fprintln(d.IO.ErrOut, "Error: --confirm is required to sync")
				return shared.MissingRequiredUsageError("--confirm")
			}
			ctx, err := uploadFlags.Apply(ctx, "experiments treatments localizations screenshot-sets sync")
			if err != nil {
				return err
			}

			result, err := executeExperimentTreatmentLocalizationScreenshotUpload(ctx, d, *localizationID, *path, *deviceType, true)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets sync: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func executeExperimentTreatmentLocalizationPreviewUpload(
	ctx context.Context,
	d *shared.Deps,
	localizationID, path, deviceType string,
) (*asc.ExperimentTreatmentLocalizationPreviewUploadResult, error) {
	upload, err := uploadLocalizationPreviews(ctx, d, localizationPreviewUploadOptions{
		localizationID:           localizationID,
		path:                     path,
		deviceType:               deviceType,
		invalidDeviceTypeIsUsage: true,
		access: localizationPreviewSetAccess{
			list: func(ctx context.Context, client *asc.Client, localizationID string) (*asc.AppPreviewSetsResponse, error) {
				return client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(ctx, localizationID)
			},
			create: func(ctx context.Context, client *asc.Client, localizationID, previewType string) (*asc.AppPreviewSetResponse, error) {
				return client.CreateAppPreviewSetForExperimentTreatmentLocalization(ctx, localizationID, previewType)
			},
		},
	})
	if upload == nil {
		return nil, err
	}
	return &asc.ExperimentTreatmentLocalizationPreviewUploadResult{
		ExperimentTreatmentLocalizationID: upload.localizationID,
		SetID:                             upload.set.ID,
		PreviewType:                       upload.set.Attributes.PreviewType,
		Results:                           upload.results,
		Failures:                          upload.failures,
	}, err
}

func executeExperimentTreatmentLocalizationScreenshotUpload(
	ctx context.Context,
	d *shared.Deps,
	localizationID, path, deviceType string,
	sync bool,
) (*asc.ExperimentTreatmentLocalizationScreenshotUploadResult, error) {
	trimmedLocalizationID := strings.TrimSpace(localizationID)
	trimmedPath := strings.TrimSpace(path)
	trimmedDeviceType := strings.TrimSpace(deviceType)
	return assets.ExecuteScreenshotSetUpload(ctx, assets.ScreenshotSetUploadOptions[*asc.ExperimentTreatmentLocalizationScreenshotUploadResult]{
		LocalizationID:           localizationID,
		Path:                     path,
		DeviceType:               deviceType,
		Replace:                  sync,
		InspectCommand:           fmt.Sprintf("asc product-pages experiments treatments localizations screenshot-sets list --localization-id %q --include-screenshots --paginate --output json", trimmedLocalizationID),
		ReplaceCommand:           shellquote.Join("asc", "product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "sync", "--localization-id", trimmedLocalizationID, "--path", trimmedPath, "--device-type", trimmedDeviceType, "--confirm"),
		InvalidDeviceTypeIsUsage: true,
		ClientFactory:            d.Client,
		RequestContext:           d.Timeout,
		UploadContext:            assets.ContextWithAssetUploadTimeout,
		Access: assets.ScreenshotSetAccess{
			List: func(ctx context.Context, client *asc.Client, localizationID string, requestContext asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error) {
				return client.GetAllAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx, localizationID, asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRequestContext(requestContext))
			},
			Create: func(ctx context.Context, client *asc.Client, localizationID, displayType string) (*asc.AppScreenshotSetResponse, error) {
				return client.CreateAppScreenshotSetForExperimentTreatmentLocalization(ctx, localizationID, displayType)
			},
		},
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], results []asc.AssetUploadResultItem) *asc.ExperimentTreatmentLocalizationScreenshotUploadResult {
			return &asc.ExperimentTreatmentLocalizationScreenshotUploadResult{
				ExperimentTreatmentLocalizationID: localizationID,
				SetID:                             set.ID,
				DisplayType:                       set.Attributes.ScreenshotDisplayType,
				Results:                           results,
			}
		},
	})
}
