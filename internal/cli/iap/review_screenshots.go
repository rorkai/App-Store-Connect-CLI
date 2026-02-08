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

// IAPReviewScreenshotsCommand returns the review screenshots command group.
func IAPReviewScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-screenshots",
		ShortUsage: "asc iap review-screenshots <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase review screenshots.",
		LongHelp: `Manage in-app purchase review screenshots.

Examples:
  asc iap review-screenshots get --iap-id "IAP_ID"
  asc iap review-screenshots create --iap-id "IAP_ID" --file "./review.png"
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --file "./review.png"
  asc iap review-screenshots delete --screenshot-id "SHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPReviewScreenshotsGetCommand(),
			IAPReviewScreenshotsCreateCommand(),
			IAPReviewScreenshotsUpdateCommand(),
			IAPReviewScreenshotsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPReviewScreenshotsGetCommand returns the review screenshots get subcommand.
func IAPReviewScreenshotsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots get", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap review-screenshots get --iap-id \"IAP_ID\"",
		ShortHelp:  "Get an in-app purchase review screenshot.",
		LongHelp: `Get an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots get --iap-id "IAP_ID"
  asc iap review-screenshots get --screenshot-id "SHOT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			screenshotValue := strings.TrimSpace(*screenshotID)
			if iapValue == "" && screenshotValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id or --screenshot-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap review-screenshots get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if screenshotValue != "" {
				resp, err := client.GetInAppPurchaseAppStoreReviewScreenshot(requestCtx, screenshotValue)
				if err != nil {
					return fmt.Errorf("iap review-screenshots get: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchaseAppStoreReviewScreenshotForIAP(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap review-screenshots get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// IAPReviewScreenshotsCreateCommand returns the review screenshots create subcommand.
func IAPReviewScreenshotsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots create", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	filePath := fs.String("file", "", "Path to screenshot file")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap review-screenshots create --iap-id \"IAP_ID\" --file \"./review.png\"",
		ShortHelp:  "Upload an in-app purchase review screenshot.",
		LongHelp: `Upload an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots create --iap-id "IAP_ID" --file "./review.png"`,
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
				return fmt.Errorf("iap review-screenshots create: %w", err)
			}
			defer file.Close()

			checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("iap review-screenshots create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap review-screenshots create: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseAppStoreReviewScreenshot(requestCtx, iapValue, info.Name(), info.Size())
			if err != nil {
				return fmt.Errorf("iap review-screenshots create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("iap review-screenshots create: no upload operations returned")
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("iap review-screenshots create: upload failed: %w", err)
			}

			uploaded := true
			if _, err := client.UpdateInAppPurchaseAppStoreReviewScreenshot(requestCtx, resp.Data.ID, asc.InAppPurchaseAppStoreReviewScreenshotUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksum.Hash,
			}); err != nil {
				return fmt.Errorf("iap review-screenshots create: failed to commit upload: %w", err)
			}

			finalResp, err := client.GetInAppPurchaseAppStoreReviewScreenshot(requestCtx, resp.Data.ID)
			if err != nil {
				return fmt.Errorf("iap review-screenshots create: failed to fetch: %w", err)
			}

			return shared.PrintOutput(finalResp, *output, *pretty)
		},
	}
}

// IAPReviewScreenshotsUpdateCommand returns the review screenshots update subcommand.
func IAPReviewScreenshotsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots update", flag.ExitOnError)

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	filePath := fs.String("file", "", "Path to screenshot file")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap review-screenshots update --screenshot-id \"SHOT_ID\" --file \"./review.png\"",
		ShortHelp:  "Re-upload an in-app purchase review screenshot.",
		LongHelp: `Re-upload an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --file "./review.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			screenshotValue := strings.TrimSpace(*screenshotID)
			if screenshotValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return flag.ErrHelp
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			file, info, err := openImageFile(pathValue)
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: %w", err)
			}
			defer file.Close()

			checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			screenshotResp, err := client.GetInAppPurchaseAppStoreReviewScreenshot(requestCtx, screenshotValue)
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: failed to fetch: %w", err)
			}
			if screenshotResp == nil {
				return fmt.Errorf("iap review-screenshots update: empty screenshot response")
			}

			uploadOps := screenshotResp.Data.Attributes.UploadOperations
			targetScreenshotID := screenshotValue
			createdReplacement := false
			if len(uploadOps) == 0 {
				iapID, err := relationshipResourceID(screenshotResp.Data.Relationships, "inAppPurchaseV2")
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: %w", err)
				}

				created, err := client.CreateInAppPurchaseAppStoreReviewScreenshot(requestCtx, iapID, info.Name(), info.Size())
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: failed to create: %w", err)
				}
				if created == nil || len(created.Data.Attributes.UploadOperations) == 0 {
					return fmt.Errorf("iap review-screenshots update: no upload operations returned")
				}

				uploadOps = created.Data.Attributes.UploadOperations
				targetScreenshotID = created.Data.ID
				createdReplacement = true
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), uploadOps); err != nil {
				return fmt.Errorf("iap review-screenshots update: upload failed: %w", err)
			}

			uploaded := true
			updated, err := client.UpdateInAppPurchaseAppStoreReviewScreenshot(requestCtx, targetScreenshotID, asc.InAppPurchaseAppStoreReviewScreenshotUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksum.Hash,
			})
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: failed to commit upload: %w", err)
			}

			if createdReplacement {
				if err := client.DeleteInAppPurchaseAppStoreReviewScreenshot(requestCtx, screenshotValue); err != nil {
					return fmt.Errorf("iap review-screenshots update: failed to delete previous screenshot: %w", err)
				}
			}

			return shared.PrintOutput(updated, *output, *pretty)
		},
	}
}

// IAPReviewScreenshotsDeleteCommand returns the review screenshots delete subcommand.
func IAPReviewScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots delete", flag.ExitOnError)

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc iap review-screenshots delete --screenshot-id \"SHOT_ID\" --confirm",
		ShortHelp:  "Delete an in-app purchase review screenshot.",
		LongHelp: `Delete an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots delete --screenshot-id "SHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			screenshotValue := strings.TrimSpace(*screenshotID)
			if screenshotValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap review-screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteInAppPurchaseAppStoreReviewScreenshot(requestCtx, screenshotValue); err != nil {
				return fmt.Errorf("iap review-screenshots delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{
				ID:      screenshotValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
