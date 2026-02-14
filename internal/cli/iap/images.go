package iap

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

// IAPImagesCommand returns the images command group.
func IAPImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc iap images <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase images.",
		LongHelp: `Manage in-app purchase images.

Examples:
  asc iap images list --iap-id "IAP_ID"
  asc iap images get --image-id "IMAGE_ID"
  asc iap images create --iap-id "IAP_ID" --file "./image.png"
  asc iap images update --image-id "IMAGE_ID" --file "./image.png"
  asc iap images delete --image-id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPImagesListCommand(),
			IAPImagesGetCommand(),
			IAPImagesCreateCommand(),
			IAPImagesUpdateCommand(),
			IAPImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPImagesListCommand returns the images list subcommand.
func IAPImagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images list", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap images list --iap-id \"IAP_ID\"",
		ShortHelp:  "List images for an in-app purchase.",
		LongHelp: `List images for an in-app purchase.

Examples:
  asc iap images list --iap-id "IAP_ID"
  asc iap images list --iap-id "IAP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap images list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap images list: %w", err)
			}

			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap images list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPImagesOption{
				asc.WithIAPImagesLimit(*limit),
				asc.WithIAPImagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPImagesLimit(200))
				firstPage, err := client.GetInAppPurchaseImages(requestCtx, iapValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap images list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseImages(ctx, iapValue, asc.WithIAPImagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap images list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseImages(requestCtx, iapValue, opts...)
			if err != nil {
				return fmt.Errorf("iap images list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPImagesGetCommand returns the images get subcommand.
func IAPImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images get", flag.ExitOnError)

	imageID := fs.String("image-id", "", "Image ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap images get --image-id \"IMAGE_ID\"",
		ShortHelp:  "Get an in-app purchase image by ID.",
		LongHelp: `Get an in-app purchase image by ID.

Examples:
  asc iap images get --image-id "IMAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			imageValue := strings.TrimSpace(*imageID)
			if imageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --image-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseImage(requestCtx, imageValue)
			if err != nil {
				return fmt.Errorf("iap images get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPImagesCreateCommand returns the images create subcommand.
func IAPImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images create", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap images create --iap-id \"IAP_ID\" --file \"./image.png\"",
		ShortHelp:  "Upload an in-app purchase image.",
		LongHelp: `Upload an in-app purchase image.

Examples:
  asc iap images create --iap-id "IAP_ID" --file "./image.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			file, info, err := openImageFile(pathValue)
			if err != nil {
				return fmt.Errorf("iap images create: %w", err)
			}
			defer file.Close()

			checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("iap images create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap images create: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseImage(requestCtx, iapValue, info.Name(), info.Size())
			if err != nil {
				return fmt.Errorf("iap images create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("iap images create: no upload operations returned")
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("iap images create: upload failed: %w", err)
			}

			uploaded := true
			if _, err := client.UpdateInAppPurchaseImage(requestCtx, resp.Data.ID, asc.InAppPurchaseImageUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksum.Hash,
			}); err != nil {
				return fmt.Errorf("iap images create: failed to commit upload: %w", err)
			}

			finalResp, err := client.GetInAppPurchaseImage(requestCtx, resp.Data.ID)
			if err != nil {
				return fmt.Errorf("iap images create: failed to fetch: %w", err)
			}

			return shared.PrintOutput(finalResp, *output.Output, *output.Pretty)
		},
	}
}

// IAPImagesUpdateCommand returns the images update subcommand.
func IAPImagesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images update", flag.ExitOnError)

	imageID := fs.String("image-id", "", "Image ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap images update --image-id \"IMAGE_ID\" --file \"./image.png\"",
		ShortHelp:  "Re-upload an in-app purchase image.",
		LongHelp: `Re-upload an in-app purchase image.

Examples:
  asc iap images update --image-id "IMAGE_ID" --file "./image.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			imageValue := strings.TrimSpace(*imageID)
			if imageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --image-id is required")
				return flag.ErrHelp
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			file, info, err := openImageFile(pathValue)
			if err != nil {
				return fmt.Errorf("iap images update: %w", err)
			}
			defer file.Close()

			checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("iap images update: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap images update: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			imageResp, err := client.GetInAppPurchaseImage(requestCtx, imageValue)
			if err != nil {
				return fmt.Errorf("iap images update: failed to fetch: %w", err)
			}
			if imageResp == nil {
				return fmt.Errorf("iap images update: empty image response")
			}

			uploadOps := imageResp.Data.Attributes.UploadOperations
			targetImageID := imageValue
			createdReplacement := false
			if len(uploadOps) == 0 {
				iapID, err := relationshipResourceID(imageResp.Data.Relationships, "inAppPurchase")
				if err != nil {
					return fmt.Errorf("iap images update: %w", err)
				}

				created, err := client.CreateInAppPurchaseImage(requestCtx, iapID, info.Name(), info.Size())
				if err != nil {
					return fmt.Errorf("iap images update: failed to create: %w", err)
				}
				if created == nil || len(created.Data.Attributes.UploadOperations) == 0 {
					return fmt.Errorf("iap images update: no upload operations returned")
				}

				uploadOps = created.Data.Attributes.UploadOperations
				targetImageID = created.Data.ID
				createdReplacement = true
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), uploadOps); err != nil {
				return fmt.Errorf("iap images update: upload failed: %w", err)
			}

			uploaded := true
			updated, err := client.UpdateInAppPurchaseImage(requestCtx, targetImageID, asc.InAppPurchaseImageUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksum.Hash,
			})
			if err != nil {
				return fmt.Errorf("iap images update: failed to commit upload: %w", err)
			}

			if createdReplacement {
				if err := client.DeleteInAppPurchaseImage(requestCtx, imageValue); err != nil {
					return fmt.Errorf("iap images update: failed to delete previous image: %w", err)
				}
			}

			return shared.PrintOutput(updated, *output.Output, *output.Pretty)
		},
	}
}

// IAPImagesDeleteCommand returns the images delete subcommand.
func IAPImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images delete", flag.ExitOnError)

	imageID := fs.String("image-id", "", "Image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc iap images delete --image-id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete an in-app purchase image.",
		LongHelp: `Delete an in-app purchase image.

Examples:
  asc iap images delete --image-id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			imageValue := strings.TrimSpace(*imageID)
			if imageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --image-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteInAppPurchaseImage(requestCtx, imageValue); err != nil {
				return fmt.Errorf("iap images delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{
				ID:      imageValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
