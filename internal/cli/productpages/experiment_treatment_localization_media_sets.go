package productpages

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var experimentTreatmentLocalizationMediaClientFactory = shared.GetASCClient

// ExperimentTreatmentLocalizationPreviewSetsCommand returns the preview sets command group.
func ExperimentTreatmentLocalizationPreviewSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc product-pages experiments treatments localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for a treatment localization.",
		LongHelp: `Manage preview sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ExperimentTreatmentLocalizationPreviewSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentTreatmentLocalizationPreviewSetsListCommand returns the preview sets list subcommand.
func ExperimentTreatmentLocalizationPreviewSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations preview-sets list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError("--localization-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return shared.UsageErrorf("experiments treatments localizations preview-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("experiments treatments localizations preview-sets list: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption{
				asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsLimit(*limit),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations preview-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(ctx, trimmedID, asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentLocalizationScreenshotSetsCommand returns the screenshot sets command group.
func ExperimentTreatmentLocalizationScreenshotSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-sets", flag.ExitOnError)

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
			ExperimentTreatmentLocalizationScreenshotSetsListCommand(),
			ExperimentTreatmentLocalizationScreenshotSetsUploadCommand(),
			ExperimentTreatmentLocalizationScreenshotSetsSyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentTreatmentLocalizationScreenshotSetsListCommand returns the screenshot sets list subcommand.
func ExperimentTreatmentLocalizationScreenshotSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations screenshot-sets list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	includeScreenshots := fs.Bool("include-screenshots", false, "Include screenshot IDs and metadata for each set (requires --localization-id and --paginate)")
	output := shared.BindOutputFlags(fs)

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
					fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
					return shared.MissingRequiredUsageError("--localization-id")
				}
				if trimmedNext != "" {
					return shared.UsageError("experiments treatments localizations screenshot-sets list: --include-screenshots cannot be combined with --next")
				}
				if !*paginate {
					return shared.UsageError("experiments treatments localizations screenshot-sets list: --include-screenshots requires --paginate")
				}
			}
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError("--localization-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return shared.UsageErrorf("experiments treatments localizations screenshot-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("experiments treatments localizations screenshot-sets list: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
			}

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption{
				asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsLimit(*limit),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := make([]asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption, 0, len(opts)+2)
				paginateOpts = append(paginateOpts, opts...)
				paginateOpts = append(
					paginateOpts,
					asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsLimit(productPagesMaxLimit),
					asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRequestContext(shared.ContextWithTimeout),
				)
				resp, err := client.GetAllAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
				}

				if *includeScreenshots {
					result, err := screenshotSetListResult(ctx, client, trimmedID, resp)
					if err != nil {
						return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
					}
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: failed to fetch: %w", err)
			}
			if *includeScreenshots {
				result, err := screenshotSetListResult(ctx, client, trimmedID, resp)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentLocalizationScreenshotSetsUploadCommand returns the screenshot sets upload subcommand.
func ExperimentTreatmentLocalizationScreenshotSetsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations screenshot-sets upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	output := shared.BindOutputFlags(fs)

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
			result, err := executeExperimentTreatmentLocalizationScreenshotUpload(ctx, *localizationID, *path, *deviceType, false)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets upload: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentLocalizationScreenshotSetsSyncCommand returns the screenshot sets sync subcommand.
func ExperimentTreatmentLocalizationScreenshotSetsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations screenshot-sets sync", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	confirm := fs.Bool("confirm", false, "Confirm sync (deletes existing media in the matching set before upload)")
	output := shared.BindOutputFlags(fs)

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
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to sync")
				return shared.MissingRequiredUsageError("--confirm")
			}

			result, err := executeExperimentTreatmentLocalizationScreenshotUpload(ctx, *localizationID, *path, *deviceType, true)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets sync: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func executeExperimentTreatmentLocalizationScreenshotUpload(
	ctx context.Context,
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
		ClientFactory:            experimentTreatmentLocalizationMediaClientFactory,
		RequestContext:           shared.ContextWithTimeout,
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
