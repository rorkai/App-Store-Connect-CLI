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

// newSubscriptionsReviewScreenshotsCommand returns the review screenshots command group.
func newSubscriptionsReviewScreenshotsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-screenshots",
		ShortUsage: "asc subscriptions review-screenshots <subcommand> [flags]",
		ShortHelp:  "Manage subscription App Store review screenshots.",
		LongHelp: `Manage subscription App Store review screenshots.

Examples:
  asc subscriptions review-screenshots view --screenshot-id "SHOT_ID"
  asc subscriptions review-screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsReviewScreenshotsGetCommand(d),
			newSubscriptionsReviewScreenshotsCreateCommand(d),
			newSubscriptionsReviewScreenshotsUpdateCommand(d),
			newSubscriptionsReviewScreenshotsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsReviewScreenshotsGetCommand returns the review screenshots get subcommand.
func newSubscriptionsReviewScreenshotsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-screenshots view", flag.ExitOnError)

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "subscriptionAppStoreReviewScreenshots", "Review screenshot ID")
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions review-screenshots view --screenshot-id \"SHOT_ID\"",
		ShortHelp:  "View a review screenshot by ID.",
		LongHelp: `View a review screenshot by ID.

Examples:
  asc subscriptions review-screenshots view --screenshot-id "SHOT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(d, fs, "", "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				return d.RequiredFlag("--screenshot-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAppStoreReviewScreenshot(
				requestCtx, id,
				asc.WithSubscriptionAppStoreReviewScreenshotSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionAppStoreReviewScreenshotInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsReviewScreenshotsCreateCommand returns the review screenshots create subcommand.
func newSubscriptionsReviewScreenshotsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-screenshots create", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	filePath := fs.String("file", "", "Path to review screenshot file")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				return d.RequiredFlag("--file")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
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
			if err := d.PreflightReviewScreenshot(pathValue, snapshot, info.Size()); err != nil {
				return d.ReviewScreenshotUsageError("--file", shared.RewriteUsageMessage(ctx, "subscriptions review-screenshots create: "+err.Error()))
			}
			checksum, err := asc.ComputeChecksumFromReader(io.NewSectionReader(snapshot, 0, info.Size()), asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: checksum failed: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}

			finalResp, err := createOrResumeSubscriptionReviewScreenshot(ctx, d, client, id, snapshot, info, checksum.Hash)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}

			return d.PrintOutput(finalResp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsReviewScreenshotsUpdateCommand returns the review screenshots update subcommand.
func newSubscriptionsReviewScreenshotsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-screenshots update", flag.ExitOnError)

	screenshotID := shared.BindResourceIDFlag(fs, "screenshot-id", "subscriptionAppStoreReviewScreenshots", "Review screenshot ID")
	checksum := fs.String("checksum", "", "Source file checksum (MD5)")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions review-screenshots update [flags]",
		ShortHelp:  "Update a review screenshot.",
		LongHelp: `Update a review screenshot.

Examples:
  asc subscriptions review-screenshots update --screenshot-id "SHOT_ID" --uploaded true --checksum "HASH"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				return d.RequiredFlag("--screenshot-id")
			}

			checksumValue := strings.TrimSpace(*checksum)
			if checksumValue == "" && !uploaded.IsSet() {
				return d.UpdateFlagRequired(fs, checksum, &uploaded)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsReviewScreenshotsDeleteCommand returns the review screenshots delete subcommand.
func newSubscriptionsReviewScreenshotsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "review-screenshots delete",
		Name:        "delete",
		ShortUsage:  "asc subscriptions review-screenshots delete --screenshot-id \"SHOT_ID\" --confirm",
		ShortHelp:   "Delete a review screenshot.",
		LongHelp: `Delete a review screenshot.

Examples:
  asc subscriptions review-screenshots delete --screenshot-id "SHOT_ID" --confirm`,
		IDFlag:             "screenshot-id",
		IDUsage:            "Review screenshot ID",
		IDType:             "subscriptionAppStoreReviewScreenshots",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscriptionAppStoreReviewScreenshot(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}

// waitForSubscriptionReviewScreenshotDelivery polls until the screenshot reaches
// a terminal delivery state and returns the successful response for output.
func waitForSubscriptionReviewScreenshotDelivery(ctx context.Context, d *shared.Deps, client *asc.Client, screenshotID, expectedChecksum string) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
	var verifiedResp *asc.SubscriptionAppStoreReviewScreenshotResponse
	lastState := ""
	_, err := asc.PollMediaUntil(ctx, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
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
