package iap

import (
	"context"
	"encoding/json"
	"errors"
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
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --file "./review.png"
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
	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "inAppPurchaseAppStoreReviewScreenshots", "Review screenshot ID")
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

The file must be a PNG or JPEG named .png, .jpg, or .jpeg; any other file is
rejected before anything is uploaded. The command also warns, and still
uploads, when the size matches no documented App Store screenshot size (such
as 1290x2796 for iPhone), the image has an alpha channel, or the image data
does not fully decode. App Store Connect may still reject such an image at
delivery; if it does, delete the failed screenshot and upload a corrected file:
  asc iap review-screenshots delete --screenshot-id "SHOT_ID" --confirm

Examples:
  asc iap review-screenshots create --iap-id "IAP_ID" --file "./review.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				return shared.RequiredFlag("--iap-id")
			}
			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				return shared.RequiredFlag("--file")
			}

			file, info, err := openImageFile(pathValue)
			if err != nil {
				err = fmt.Errorf("iap review-screenshots create: %w", err)
				if errors.Is(err, os.ErrNotExist) {
					return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticFileNotFound, "--file")
				}
				return err
			}
			defer file.Close()
			snapshot, cleanupSnapshot, err := shared.SnapshotImageFile(file, info.Size())
			if err != nil {
				return fmt.Errorf("iap review-screenshots create: %w", err)
			}
			defer cleanupSnapshot()
			if err := shared.PreflightReviewScreenshot(pathValue, snapshot, info.Size()); err != nil {
				return shared.ReviewScreenshotUsageError("--file", "iap review-screenshots create: "+err.Error())
			}

			checksum, err := asc.ComputeChecksumFromReader(snapshot, asc.ChecksumAlgorithmMD5)
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

			if err := asc.UploadAssetFromFile(requestCtx, snapshot, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
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

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "inAppPurchaseAppStoreReviewScreenshots", "Review screenshot ID")
	checksum := fs.String("checksum", "", "Source file checksum (MD5)")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	filePath := fs.String("file", "", "Path to screenshot file; resumes an in-progress upload")
	confirm := fs.Bool("confirm", false, "Accepted with --file; has no effect because update never replaces or deletes a completed screenshot")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap review-screenshots update [flags]",
		ShortHelp:  "Update an in-app purchase review screenshot.",
		LongHelp: `Update an in-app purchase review screenshot.

Examples:
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --file "./review.png"
  asc iap review-screenshots update --screenshot-id "SHOT_ID" --uploaded true --checksum "HASH"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				return shared.RequiredFlag("--screenshot-id")
			}
			fileProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "file" {
					fileProvided = true
				}
			})

			checksumValue := strings.TrimSpace(*checksum)
			if fileProvided {
				if strings.TrimSpace(*filePath) == "" {
					return shared.RequiredFlag("--file")
				}
				if checksumValue != "" || uploaded.IsSet() {
					return shared.UsageError("--file cannot be combined with --checksum or --uploaded")
				}
			} else if *confirm {
				return shared.UsageError("--confirm can only be used with --file")
			}
			if !fileProvided && checksumValue == "" && !uploaded.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			if fileProvided {
				pathValue := strings.TrimSpace(*filePath)
				file, info, err := openImageFile(pathValue)
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: %w", err)
				}
				defer file.Close()
				snapshot, cleanupSnapshot, err := shared.SnapshotImageFile(file, info.Size())
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: %w", err)
				}
				defer cleanupSnapshot()
				if err := shared.PreflightReviewScreenshot(pathValue, snapshot, info.Size()); err != nil {
					return shared.ReviewScreenshotUsageError("--file", "iap review-screenshots update: "+err.Error())
				}

				checksum, err := asc.ComputeChecksumFromReader(snapshot, asc.ChecksumAlgorithmMD5)
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: %w", err)
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: %w", err)
				}

				requestCtx, uploadCancel := contextWithAssetUploadTimeout(ctx)
				defer uploadCancel()
				updated, err := updateIAPReviewScreenshotFromFile(requestCtx, client, id, snapshot, file.Name(), info.Size(), checksum.Hash)
				if err != nil {
					return fmt.Errorf("iap review-screenshots update: %w", err)
				}
				return shared.PrintOutput(updated, *output.Output, *output.Pretty)
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

func updateIAPReviewScreenshotFromFile(ctx context.Context, client *asc.Client, screenshotID string, file *os.File, fileName string, fileSize int64, checksum string) (*asc.InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	screenshotResp, err := client.GetInAppPurchaseAppStoreReviewScreenshot(ctx, screenshotID, asc.WithIAPReviewScreenshotFields([]string{
		"fileSize",
		"uploadOperations",
		"assetDeliveryState",
		"inAppPurchaseV2",
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch screenshot %q: %w", screenshotID, err)
	}
	if screenshotResp == nil {
		return nil, fmt.Errorf("empty screenshot response for %q", screenshotID)
	}

	uploadOps := screenshotResp.Data.Attributes.UploadOperations
	if len(uploadOps) == 0 {
		return nil, completedIAPReviewScreenshotReplacementError(screenshotID, screenshotResp.Data.Relationships, fileName)
	}
	if screenshotResp.Data.Attributes.FileSize != fileSize {
		return nil, shared.WithDiagnostic(shared.NewValidationError(fmt.Errorf("file size %d does not match the existing screenshot upload reservation size %d", fileSize, screenshotResp.Data.Attributes.FileSize)), shared.DiagnosticInvalidInput, "--file")
	}

	if err := asc.UploadAssetFromFile(ctx, file, fileSize, uploadOps); err != nil {
		return nil, fmt.Errorf("upload failed for screenshot %q: %w", screenshotID, err)
	}

	uploaded := true
	if _, err := client.UpdateInAppPurchaseAppStoreReviewScreenshot(ctx, screenshotID, asc.InAppPurchaseAppStoreReviewScreenshotUpdateAttributes{
		Uploaded:           &uploaded,
		SourceFileChecksum: &checksum,
	}); err != nil {
		return nil, fmt.Errorf("failed to commit screenshot %q: %w", screenshotID, err)
	}

	verified, err := waitForIAPReviewScreenshotDelivery(ctx, client, screenshotID)
	if err != nil {
		return nil, fmt.Errorf("screenshot %q: %w", screenshotID, err)
	}

	return verified, nil
}

func completedIAPReviewScreenshotReplacementError(screenshotID string, relationships json.RawMessage, fileName string) error {
	iapID := "IAP_ID"
	if relationshipID, err := relationshipResourceID(relationships, "inAppPurchaseV2"); err == nil {
		iapID = relationshipID
	}
	deleteCommand, createCommand := completedIAPReviewScreenshotReplacementCommands(screenshotID, iapID, fileName)
	manual := fmt.Sprintf("App Store Connect does not support replacing completed screenshot %s through update; run %s, then %s", shellQuotedRemediationArgument(screenshotID, "SCREENSHOT_ID"), deleteCommand, createCommand)
	return shared.WithDiagnostic(shared.NewValidationError(shared.NewErrorWithCause(fmt.Errorf("%s", manual), asc.ErrConflict)), shared.DiagnosticResourceConflict, "--screenshot-id")
}

func completedIAPReviewScreenshotReplacementCommands(screenshotID, iapID, fileName string) (string, string) {
	return fmt.Sprintf(
			"asc iap review-screenshots delete --screenshot-id %s --confirm",
			shellQuotedRemediationArgument(screenshotID, "SCREENSHOT_ID"),
		), fmt.Sprintf(
			"asc iap review-screenshots create --iap-id %s --file %s",
			shellQuotedRemediationArgument(iapID, "IAP_ID"),
			shellQuotedRemediationArgument(fileName, "FILE_PATH"),
		)
}

func shellQuotedRemediationArgument(value, placeholder string) string {
	if quoted, ok := shared.ShellQuote(value); ok {
		return quoted
	}
	return placeholder
}

// IAPReviewScreenshotsDeleteCommand returns the review screenshots delete subcommand.
func IAPReviewScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots delete", flag.ExitOnError)

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "inAppPurchaseAppStoreReviewScreenshots", "Review screenshot ID")
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
				return shared.RequiredFlag("--screenshot-id")
			}
			if !*confirm {
				return shared.RequiredFlag("--confirm")
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
				return struct{}{}, false, shared.ReviewScreenshotDeliveryError("asc iap review-screenshots", screenshotID, state.Errors)
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
