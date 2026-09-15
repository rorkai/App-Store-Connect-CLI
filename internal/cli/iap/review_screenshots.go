package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const iapReviewScreenshotPollInterval = 2 * time.Second

const iapReviewScreenshotFileDeprecationWarning = "Warning: `--file` is deprecated and unsupported for `asc iap review-screenshots update`; use `asc iap review-screenshots create --iap-id \"IAP_ID\" --file \"./review.png\"` for a replacement upload, then delete the old screenshot, or use `--checksum` and/or `--uploaded` for PATCH updates."

// IAPReviewScreenshotsCommand returns the review screenshots command group.
func IAPReviewScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-screenshots",
		ShortUsage: "asc iap review-screenshots <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase review screenshots.",
		LongHelp: `Manage in-app purchase review screenshots.

Examples:
  asc iap review-screenshots view --iap-id "IAP_ID"
  asc iap review-screenshots create --iap-id "IAP_ID" --file "./review.png"
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --uploaded true --checksum "HASH"
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
	fs := flag.NewFlagSet("review-screenshots view", flag.ExitOnError)

	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	appID := addIAPLookupAppFlag(fs)
	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	iapFields := fs.String("iap-fields", "", "fields[inAppPurchases] for the included in-app purchase (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap review-screenshots view --iap-id \"IAP_ID\"",
		ShortHelp:  "View an in-app purchase review screenshot.",
		LongHelp: `View an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots view --iap-id "IAP_ID"
  asc iap review-screenshots view --screenshot-id "SHOT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			screenshotValue := strings.TrimSpace(*screenshotID)
			if iapValue == "" && screenshotValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id or --screenshot-id is required")
				return shared.MissingRequiredUsageError("")
			}
			fieldValues, err := shared.NormalizeSelection(*iapFields, iapVersionIAPFields, "--iap-fields")
			if err != nil {
				return shared.UsageError("iap review-screenshots view: " + err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap review-screenshots view: %w", err)
			}

			if screenshotValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.GetInAppPurchaseAppStoreReviewScreenshot(requestCtx, screenshotValue, asc.WithIAPReviewScreenshotIAPFields(fieldValues))
				if err != nil {
					return fmt.Errorf("iap review-screenshots view: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseAppStoreReviewScreenshotForIAP(requestCtx, iapValue, asc.WithIAPReviewScreenshotIAPFields(fieldValues))
			if err != nil {
				return fmt.Errorf("iap review-screenshots view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPReviewScreenshotsCreateCommand returns the review screenshots create subcommand.
func IAPReviewScreenshotsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots create", flag.ExitOnError)

	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	appID := addIAPLookupAppFlag(fs)
	filePath := fs.String("file", "", "Path to screenshot file")
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError("--iap-id")
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError("--file")
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

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
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

			// Verify asset delivery — poll until COMPLETE or FAILED
			screenshotID := resp.Data.ID
			verifyCtx, verifyCancel := contextWithAssetUploadTimeout(ctx)
			defer verifyCancel()
			finalResp, verifyErr := waitForIAPReviewScreenshotDelivery(verifyCtx, client, screenshotID)
			if verifyErr != nil {
				return fmt.Errorf("iap review-screenshots create: %w", verifyErr)
			}

			return shared.PrintOutput(finalResp, *output.Output, *output.Pretty)
		},
	}
}

// IAPReviewScreenshotsUpdateCommand returns the review screenshots update subcommand.
func IAPReviewScreenshotsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots update", flag.ExitOnError)

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	checksum := fs.String("checksum", "", "Source file checksum (MD5)")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	fs.String("file", "", "[deprecated] Use --checksum and/or --uploaded; upload replacements with create")
	shared.HideFlagFromHelp(fs.Lookup("file"))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap review-screenshots update [flags]",
		ShortHelp:  "Update an in-app purchase review screenshot.",
		LongHelp: `Update an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --uploaded true --checksum "HASH"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("iap review-screenshots update does not accept positional arguments: %s", strings.Join(args, " "))
			}

			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return shared.MissingRequiredUsageError("--screenshot-id")
			}
			legacyFileProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "file" {
					legacyFileProvided = true
				}
			})
			if legacyFileProvided {
				fmt.Fprintln(os.Stderr, iapReviewScreenshotFileDeprecationWarning)
				return shared.UsageError("`--file` is unsupported for `asc iap review-screenshots update`; use `--checksum` and/or `--uploaded`")
			}

			checksumValue := strings.TrimSpace(*checksum)
			if checksumValue == "" && !uploaded.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.InAppPurchaseAppStoreReviewScreenshotUpdateAttributes{}
			if checksumValue != "" {
				attrs.SourceFileChecksum = &checksumValue
			}
			if uploaded.IsSet() {
				value := uploaded.Value()
				attrs.Uploaded = &value
			}

			updated, err := client.UpdateInAppPurchaseAppStoreReviewScreenshot(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("iap review-screenshots update: failed to update: %w", err)
			}

			return shared.PrintOutput(updated, *output.Output, *output.Pretty)
		},
	}
}

// IAPReviewScreenshotsDeleteCommand returns the review screenshots delete subcommand.
func IAPReviewScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots delete", flag.ExitOnError)

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError("--screenshot-id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// waitForIAPReviewScreenshotDelivery polls until the screenshot reaches
// a terminal delivery state and returns the successful response for output.
func waitForIAPReviewScreenshotDelivery(ctx context.Context, client *asc.Client, screenshotID string) (*asc.InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	var verifiedResp *asc.InAppPurchaseAppStoreReviewScreenshotResponse
	_, err := asc.PollUntil(ctx, iapReviewScreenshotPollInterval, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := client.GetInAppPurchaseAppStoreReviewScreenshot(ctx, screenshotID)
		if err != nil {
			return struct{}{}, false, err
		}
		state := resp.Data.Attributes.AssetDeliveryState
		if state != nil && state.State != nil {
			switch strings.ToUpper(*state.State) {
			case "COMPLETE":
				verifiedResp = resp
				return struct{}{}, true, nil
			case "FAILED":
				errMsgs := make([]string, 0, len(state.Errors))
				for _, e := range state.Errors {
					if e.Code != "" {
						errMsgs = append(errMsgs, e.Code)
					} else if e.Message != "" {
						errMsgs = append(errMsgs, e.Message)
					}
				}
				detail := strings.Join(errMsgs, "; ")
				if detail == "" {
					detail = "unknown error"
				}
				return struct{}{}, false, fmt.Errorf("screenshot %s delivery failed: %s", screenshotID, detail)
			}
		}
		return struct{}{}, false, nil
	})
	if err != nil {
		return nil, err
	}
	if verifiedResp == nil {
		return nil, fmt.Errorf("screenshot %s delivery completed without a verified response", screenshotID)
	}
	return verifiedResp, nil
}
