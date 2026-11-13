package subscriptions

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsReviewScreenshotsCommand returns the review screenshots command group.
func SubscriptionsReviewScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-screenshots",
		ShortUsage: "asc subscriptions review-screenshots <subcommand> [flags]",
		ShortHelp:  "Manage subscription App Store review screenshots.",
		LongHelp: `Manage subscription App Store review screenshots.

Examples:
  asc subscriptions review-screenshots view --screenshot-id "SHOT_ID"
  asc subscriptions review-screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsReviewScreenshotsGetCommand(),
			SubscriptionsReviewScreenshotsCreateCommand(),
			SubscriptionsReviewScreenshotsUpdateCommand(),
			SubscriptionsReviewScreenshotsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsReviewScreenshotsGetCommand returns the review screenshots get subcommand.
func SubscriptionsReviewScreenshotsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots view", flag.ExitOnError)

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "subscriptionAppStoreReviewScreenshots", "Review screenshot ID")
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions review-screenshots view --screenshot-id \"SHOT_ID\"",
		ShortHelp:  "View a review screenshot by ID.",
		LongHelp: `View a review screenshot by ID.

Examples:
  asc subscriptions review-screenshots view --screenshot-id "SHOT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(fs, "", "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return shared.MissingRequiredUsageError("--screenshot-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAppStoreReviewScreenshot(
				requestCtx, id,
				asc.WithSubscriptionAppStoreReviewScreenshotSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionAppStoreReviewScreenshotInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsReviewScreenshotsCreateCommand returns the review screenshots create subcommand.
func SubscriptionsReviewScreenshotsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots create", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	filePath := fs.String("file", "", "Path to review screenshot file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions review-screenshots create [flags]",
		ShortHelp:  "Upload a review screenshot for a subscription.",
		LongHelp: `Upload a review screenshot for a subscription.

The file must be a PNG or JPEG named .png, .jpg, or .jpeg; any other file is
rejected before anything is uploaded. The command also warns, and still
uploads, when the size matches no documented App Store screenshot size (such
as 1290x2796 for iPhone), the image has an alpha channel, or the image data
does not fully decode. App Store Connect may still reject such an image at
delivery; if it does, delete the failed screenshot and upload a corrected file:
  asc subscriptions review-screenshots delete --screenshot-id "SHOT_ID" --confirm

Examples:
  asc subscriptions review-screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("subscriptions review screenshots create does not accept positional arguments: %s", strings.Join(args, " "))
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError("--subscription-id")
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError("--file")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			snapshot, info, cleanupSnapshot, err := snapshotSubscriptionReviewScreenshot(pathValue)
			if err != nil {
				err = fmt.Errorf("subscriptions review-screenshots create: %w", err)
				if errors.Is(err, os.ErrNotExist) {
					return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticFileNotFound, "--file")
				}
				return err
			}
			defer cleanupSnapshot()
			if err := shared.PreflightReviewScreenshot(pathValue, snapshot, info.Size()); err != nil {
				return shared.ReviewScreenshotUsageError("--file", shared.RewriteUsageMessage(ctx, "subscriptions review-screenshots create: "+err.Error()))
			}
			checksum, err := asc.ComputeChecksumFromReader(io.NewSectionReader(snapshot, 0, info.Size()), asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: checksum failed: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			finalResp, err := createOrResumeSubscriptionReviewScreenshot(ctx, client, id, snapshot, info, checksum.Hash)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}

			return shared.PrintOutput(finalResp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsReviewScreenshotsUpdateCommand returns the review screenshots update subcommand.
func SubscriptionsReviewScreenshotsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots update", flag.ExitOnError)

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "subscriptionAppStoreReviewScreenshots", "Review screenshot ID")
	checksum := fs.String("checksum", "", "Source file checksum (MD5)")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions review-screenshots update [flags]",
		ShortHelp:  "Update a review screenshot.",
		LongHelp: `Update a review screenshot.

Examples:
  asc subscriptions review-screenshots update --screenshot-id "SHOT_ID" --uploaded true --checksum "HASH"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return shared.MissingRequiredUsageError("--screenshot-id")
			}

			checksumValue := strings.TrimSpace(*checksum)
			if checksumValue == "" && !uploaded.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionAppStoreReviewScreenshotUpdateAttributes{}
			if checksumValue != "" {
				attrs.SourceFileChecksum = &checksumValue
			}
			if uploaded.IsSet() {
				value := uploaded.Value()
				attrs.Uploaded = &value
			}

			resp, err := client.UpdateSubscriptionAppStoreReviewScreenshot(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsReviewScreenshotsDeleteCommand returns the review screenshots delete subcommand.
func SubscriptionsReviewScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots delete", flag.ExitOnError)

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "subscriptionAppStoreReviewScreenshots", "Review screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions review-screenshots delete --screenshot-id \"SHOT_ID\" --confirm",
		ShortHelp:  "Delete a review screenshot.",
		LongHelp: `Delete a review screenshot.

Examples:
  asc subscriptions review-screenshots delete --screenshot-id "SHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return shared.MissingRequiredUsageError("--screenshot-id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionAppStoreReviewScreenshot(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions review-screenshots delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{ID: id, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// waitForSubscriptionReviewScreenshotDelivery polls until the screenshot reaches
// a terminal delivery state and returns the successful response for output.
func waitForSubscriptionReviewScreenshotDelivery(ctx context.Context, client *asc.Client, screenshotID, expectedChecksum string) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
	var verifiedResp *asc.SubscriptionAppStoreReviewScreenshotResponse
	lastState := ""
	_, err := asc.PollMediaUntil(ctx, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
			return client.GetSubscriptionAppStoreReviewScreenshot(requestCtx, screenshotID)
		})
		if err != nil {
			return struct{}{}, false, err
		}
		state := resp.Data.Attributes.AssetDeliveryState
		if state != nil && state.State != nil {
			lastState = strings.ToUpper(*state.State)
			switch lastState {
			case "COMPLETE":
				actualChecksum := strings.TrimSpace(resp.Data.Attributes.SourceFileChecksum)
				if !strings.EqualFold(actualChecksum, strings.TrimSpace(expectedChecksum)) {
					return struct{}{}, false, newSubscriptionReviewScreenshotConflictError("screenshot %s checksum changed while waiting for delivery", screenshotID)
				}
				verifiedResp = resp
				return struct{}{}, true, nil
			case "FAILED":
				return struct{}{}, false, subscriptionReviewScreenshotDeliveryError(resp)
			}
		}
		return struct{}{}, false, nil
	}, asc.PollOptions{})
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			stateDetail := ""
			if lastState != "" {
				stateDetail = " after delivery state " + lastState
			}
			return nil, shared.MarkUploadDeadline(fmt.Errorf("timed out waiting for screenshot %s delivery%s: %w; the screenshot was uploaded and App Store Connect may still finish processing it; rerun the same command to resume waiting", screenshotID, stateDetail, err))
		}
		return nil, err
	}
	if verifiedResp == nil {
		return nil, fmt.Errorf("screenshot %s delivery completed without a verified response", screenshotID)
	}
	return verifiedResp, nil
}
