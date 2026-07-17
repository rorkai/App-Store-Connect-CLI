package subscriptions

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

const reviewScreenshotPollInterval = 2 * time.Second

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

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
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
				return shared.MissingRequiredUsageError()
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

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	filePath := fs.String("file", "", "Path to review screenshot file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions review-screenshots create [flags]",
		ShortHelp:  "Upload a review screenshot for a subscription.",
		LongHelp: `Upload a review screenshot for a subscription.

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
				return shared.MissingRequiredUsageError()
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError()
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			file, info, err := openSubscriptionImageFile(pathValue)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}
			defer file.Close()
			checksum, err := asc.ComputeFileChecksum(pathValue, asc.ChecksumAlgorithmMD5)
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

			finalResp, err := createOrResumeSubscriptionReviewScreenshot(ctx, client, id, pathValue, info, checksum.Hash)
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

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
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
				return shared.MissingRequiredUsageError()
			}

			checksumValue := strings.TrimSpace(*checksum)
			if checksumValue == "" && !uploaded.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
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

	screenshotID := fs.String("screenshot-id", "", "Review screenshot ID")
	legacyID := shared.BindDeprecatedStringFlagAlias(fs, "id", "screenshot-id")
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
			if err := legacyID.Apply(screenshotID); err != nil {
				return err
			}
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
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
	_, err := asc.PollUntil(ctx, reviewScreenshotPollInterval, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
			return client.GetSubscriptionAppStoreReviewScreenshot(requestCtx, screenshotID)
		})
		if err != nil {
			return struct{}{}, false, err
		}
		state := resp.Data.Attributes.AssetDeliveryState
		if state != nil && state.State != nil {
			switch strings.ToUpper(*state.State) {
			case "COMPLETE":
				actualChecksum := strings.TrimSpace(resp.Data.Attributes.SourceFileChecksum)
				if !strings.EqualFold(actualChecksum, strings.TrimSpace(expectedChecksum)) {
					return struct{}{}, false, newSubscriptionReviewScreenshotConflictError("screenshot %s checksum changed while waiting for delivery", screenshotID)
				}
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
