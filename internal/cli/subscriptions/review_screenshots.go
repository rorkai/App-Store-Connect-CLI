package subscriptions

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

// SubscriptionsReviewScreenshotsCommand returns the review screenshots command group.
func SubscriptionsReviewScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-screenshots",
		ShortUsage: "asc subscriptions review-screenshots <subcommand> [flags]",
		ShortHelp:  "Manage subscription App Store review screenshots.",
		LongHelp: `Manage subscription App Store review screenshots.

Examples:
  asc subscriptions review-screenshots get --id "SHOT_ID"
  asc subscriptions review-screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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
	fs := flag.NewFlagSet("review-screenshots get", flag.ExitOnError)

	screenshotID := fs.String("id", "", "Review screenshot ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions review-screenshots get --id \"SHOT_ID\"",
		ShortHelp:  "Get a review screenshot by ID.",
		LongHelp: `Get a review screenshot by ID.

Examples:
  asc subscriptions review-screenshots get --id "SHOT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAppStoreReviewScreenshot(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsReviewScreenshotsCreateCommand returns the review screenshots create subcommand.
func SubscriptionsReviewScreenshotsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	filePath := fs.String("file", "", "Path to review screenshot file")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions review-screenshots create [flags]",
		ShortHelp:  "Upload a review screenshot for a subscription.",
		LongHelp: `Upload a review screenshot for a subscription.

Examples:
  asc subscriptions review-screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			file, info, err := openSubscriptionImageFile(pathValue)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}
			defer file.Close()

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: %w", err)
			}

			requestCtx, cancel := contextWithUploadTimeout(ctx)
			defer cancel()

			resp, err := client.CreateSubscriptionAppStoreReviewScreenshot(requestCtx, id, info.Name(), info.Size())
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("subscriptions review-screenshots create: no upload operations returned")
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: upload failed: %w", err)
			}

			checksum, err := asc.ComputeFileChecksum(pathValue, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: checksum failed: %w", err)
			}

			uploaded := true
			updateAttrs := asc.SubscriptionAppStoreReviewScreenshotUpdateAttributes{
				SourceFileChecksum: &checksum.Hash,
				Uploaded:           &uploaded,
			}

			commitResp, err := client.UpdateSubscriptionAppStoreReviewScreenshot(requestCtx, resp.Data.ID, updateAttrs)
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots create: failed to commit upload: %w", err)
			}
			if commitResp != nil {
				return printOutput(commitResp, *output, *pretty)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsReviewScreenshotsUpdateCommand returns the review screenshots update subcommand.
func SubscriptionsReviewScreenshotsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots update", flag.ExitOnError)

	screenshotID := fs.String("id", "", "Review screenshot ID")
	checksum := fs.String("checksum", "", "Source file checksum (MD5)")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions review-screenshots update [flags]",
		ShortHelp:  "Update a review screenshot.",
		LongHelp: `Update a review screenshot.

Examples:
  asc subscriptions review-screenshots update --id "SHOT_ID" --uploaded true --checksum "HASH"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			checksumValue := strings.TrimSpace(*checksum)
			if checksumValue == "" && !uploaded.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
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

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsReviewScreenshotsDeleteCommand returns the review screenshots delete subcommand.
func SubscriptionsReviewScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-screenshots delete", flag.ExitOnError)

	screenshotID := fs.String("id", "", "Review screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions review-screenshots delete --id \"SHOT_ID\" --confirm",
		ShortHelp:  "Delete a review screenshot.",
		LongHelp: `Delete a review screenshot.

Examples:
  asc subscriptions review-screenshots delete --id "SHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions review-screenshots delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionAppStoreReviewScreenshot(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions review-screenshots delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{ID: id, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}
