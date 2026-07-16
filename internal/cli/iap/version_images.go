package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func IAPVersionImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images", flag.ExitOnError)
	return &ffcli.Command{
		Name: "images", ShortUsage: "asc iap versions images <subcommand> [flags]", ShortHelp: "Manage version-scoped IAP review images.", LongHelp: "Manage version-scoped IAP review images.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{IAPVersionImagesListCommand(), IAPVersionImagesCreateCommand(), IAPVersionImagesViewCommand(), IAPVersionImagesUpdateCommand(), IAPVersionImagesDeleteCommand()}, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func IAPVersionImagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images list", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	imageFields := fs.String("image-fields", "", "fields[inAppPurchaseImages] (comma-separated)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc iap versions images list --version-id "VERSION_ID" [flags]`, ShortHelp: "List images for an IAP version.", LongHelp: "List images for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			if flagSet(fs, "version-id") && strings.TrimSpace(*next) != "" {
				return shared.UsageError("iap versions images list: --next cannot be combined with --version-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap versions images list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageError("iap versions images list: " + err.Error())
			}
			if strings.TrimSpace(*next) != "" && (*limit != 0 || strings.TrimSpace(*imageFields) != "") {
				return shared.UsageError("iap versions images list: --next cannot be combined with --limit or --image-fields")
			}
			fields, err := shared.NormalizeSelection(*imageFields, iapVersionImageFields, "--image-fields")
			if err != nil {
				return shared.UsageError("iap versions images list: " + err.Error())
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions images list: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseVersionImages(requestCtx, id, asc.WithIAPVersionImagesLimit(*limit), asc.WithIAPVersionImagesNextURL(*next), asc.WithIAPVersionImagesFields(fields))
			if err != nil {
				return fmt.Errorf("iap versions images list: failed to fetch: %w", err)
			}
			if *paginate {
				aggregated, err := asc.PaginateAll(requestCtx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseVersionImages(ctx, id, asc.WithIAPVersionImagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap versions images list: %w", err)
				}
				return shared.PrintOutput(aggregated, *output.Output, *output.Pretty)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images create", flag.ExitOnError)
	versionID := fs.String("version-id", "", "In-app purchase version ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc iap versions images create --version-id "VERSION_ID" --file "./image.png"`, ShortHelp: "Upload an image for an IAP version.", LongHelp: "Reserve, upload, commit, and fetch an image for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError()
			}
			path := strings.TrimSpace(*filePath)
			if path == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError()
			}
			file, info, err := openImageFile(path)
			if err != nil {
				return fmt.Errorf("iap versions images create: %w", err)
			}
			defer file.Close()
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions images create: %w", err)
			}
			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()
			reservation, err := client.CreateInAppPurchaseImageV2(requestCtx, vid, info.Name(), info.Size())
			if err != nil {
				return fmt.Errorf("iap versions images create: failed to create: %w", err)
			}
			if reservation == nil {
				return fmt.Errorf("iap versions images create: no upload operations returned")
			}
			reservedID := reservation.Data.ID
			if len(reservation.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("iap versions images create: no upload operations returned for reserved image %q", reservedID)
			}
			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), reservation.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("iap versions images create: upload failed for reserved image %q: %w", reservedID, err)
			}
			uploaded := true
			if _, err := client.UpdateInAppPurchaseImageV2(requestCtx, reservedID, asc.InAppPurchaseImageV2UpdateAttributes{Uploaded: &uploaded}); err != nil {
				return fmt.Errorf("iap versions images create: failed to commit upload for reserved image %q: %w", reservedID, err)
			}
			resp, err := client.GetInAppPurchaseImageV2(requestCtx, reservedID)
			if err != nil {
				return fmt.Errorf("iap versions images create: failed to fetch reserved image %q: %w", reservedID, err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionImagesViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images view", flag.ExitOnError)
	id := fs.String("image-id", "", "Image ID")
	imageFields := fs.String("image-fields", "", "fields[inAppPurchaseImages] (comma-separated)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: `asc iap versions images view --image-id "IMAGE_ID"`, ShortHelp: "View a version-scoped IAP image.", LongHelp: "View a version-scoped IAP image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --image-id is required")
				return shared.MissingRequiredUsageError()
			}
			fields, err := shared.NormalizeSelection(*imageFields, iapVersionImageFields, "--image-fields")
			if err != nil {
				return shared.UsageError("iap versions images view: " + err.Error())
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions images view: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.GetInAppPurchaseImageV2(requestCtx, value, asc.WithIAPImageV2Fields(fields))
			if err != nil {
				return fmt.Errorf("iap versions images view: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionImagesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images update", flag.ExitOnError)
	id := fs.String("image-id", "", "Image ID")
	uploaded := fs.String("uploaded", "", "Set upload completion state: true or false")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: `asc iap versions images update --image-id "IMAGE_ID" --uploaded true`, ShortHelp: "Update a version-scoped IAP image.", LongHelp: "Update the upload completion state for a version-scoped IAP image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --image-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !flagSet(fs, "uploaded") {
				fmt.Fprintln(os.Stderr, "Error: --uploaded is required")
				return shared.MissingRequiredUsageError()
			}
			uploadedValue, err := strconv.ParseBool(strings.TrimSpace(*uploaded))
			if err != nil {
				return shared.UsageError("iap versions images update: --uploaded must be true or false")
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions images update: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resp, err := client.UpdateInAppPurchaseImageV2(requestCtx, value, asc.InAppPurchaseImageV2UpdateAttributes{Uploaded: &uploadedValue})
			if err != nil {
				return fmt.Errorf("iap versions images update: failed to update: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions images delete", flag.ExitOnError)
	id := fs.String("image-id", "", "Image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortUsage: `asc iap versions images delete --image-id "IMAGE_ID" --confirm`, ShortHelp: "Delete a version-scoped IAP image.", LongHelp: "Delete a version-scoped IAP image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectIAPVersionArgs(args); err != nil {
				return err
			}
			value := strings.TrimSpace(*id)
			if value == "" {
				fmt.Fprintln(os.Stderr, "Error: --image-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}
			client, err := iapVersionClientFactory()
			if err != nil {
				return fmt.Errorf("iap versions images delete: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			if err := client.DeleteInAppPurchaseImageV2(requestCtx, value); err != nil {
				return fmt.Errorf("iap versions images delete: failed to delete: %w", err)
			}
			return shared.PrintOutput(&asc.AssetDeleteResult{ID: value, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
