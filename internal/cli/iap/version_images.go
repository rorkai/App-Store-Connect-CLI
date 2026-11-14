package iap

import (
	"context"
	"flag"
	"fmt"
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
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	imageFields := fs.String("image-fields", "", "fields[inAppPurchaseImages] (comma-separated)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortUsage: `asc iap versions images list --version-id "VERSION_ID" [flags]`, ShortHelp: "List images for an IAP version.", LongHelp: "List images for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--version-id")
			}
			if err := rejectIAPVersionNextFlagConflicts(
				fs, *next, "iap versions images list", "version-id", "limit", "image-fields",
			); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap versions images list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageError("iap versions images list: " + err.Error())
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
			firstPageLimit := *limit
			if *paginate && firstPageLimit == 0 {
				firstPageLimit = 200
			}
			resp, err := client.GetInAppPurchaseVersionImages(requestCtx, id, asc.Limit[asc.IAPVersionImagesOption](firstPageLimit), asc.WithIAPVersionImagesNextURL(*next), asc.WithIAPVersionImagesFields(fields))
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
	versionID := shared.BindResourceIDFlag(fs, "version-id", "inAppPurchaseVersions", "In-app purchase version ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "create", ShortUsage: `asc iap versions images create --version-id "VERSION_ID" --file "./image.png"`, ShortHelp: "Upload an image for an IAP version.", LongHelp: "Reserve, upload, commit, and fetch an image for an IAP version.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				return shared.RequiredFlag("--version-id")
			}
			path := strings.TrimSpace(*filePath)
			if path == "" {
				return shared.RequiredFlag("--file")
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
			if _, err := client.UpdateInAppPurchaseImageV2(requestCtx, reservedID, asc.InAppPurchaseImageV2UpdateAttributes{Uploaded: &asc.NullableBool{Value: &uploaded}}); err != nil {
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
	id := shared.BindResourceIDFlag(fs, "image-id", "inAppPurchaseImages", "Image ID")
	imageFields := fs.String("image-fields", "", "fields[inAppPurchaseImages] (comma-separated)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortUsage: `asc iap versions images view --image-id "IMAGE_ID"`, ShortHelp: "View a version-scoped IAP image.", LongHelp: "View a version-scoped IAP image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			value := strings.TrimSpace(*id)
			if value == "" {
				return shared.RequiredFlag("--image-id")
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
	id := shared.BindResourceIDFlag(fs, "image-id", "inAppPurchaseImages", "Image ID")
	uploaded := fs.String("uploaded", "", "Set upload completion state: true or false")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "update", ShortUsage: `asc iap versions images update --image-id "IMAGE_ID" --uploaded true`, ShortHelp: "Update a version-scoped IAP image.", LongHelp: "Update the upload completion state for a version-scoped IAP image.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			value := strings.TrimSpace(*id)
			if value == "" {
				return shared.RequiredFlag("--image-id")
			}
			if !flagSet(fs, "uploaded") {
				return shared.RequiredFlag("--uploaded")
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
			resp, err := client.UpdateInAppPurchaseImageV2(requestCtx, value, asc.InAppPurchaseImageV2UpdateAttributes{Uploaded: &asc.NullableBool{Value: &uploadedValue}})
			if err != nil {
				return fmt.Errorf("iap versions images update: failed to update: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func IAPVersionImagesDeleteCommand() *ffcli.Command {
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return iapVersionClientFactory() }

	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName:        "versions images delete",
		Name:               "delete",
		ShortUsage:         `asc iap versions images delete --image-id "IMAGE_ID" --confirm`,
		ShortHelp:          "Delete a version-scoped IAP image.",
		LongHelp:           "Delete a version-scoped IAP image.",
		IDFlag:             "image-id",
		IDUsage:            "Image ID",
		IDType:             "inAppPurchaseImages",
		RequiredDiagnostic: true,
		ErrorPrefix:        "iap versions images delete",
		DeleteErrorPrefix:  "iap versions images delete: failed to delete",
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteInAppPurchaseImageV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
