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

// SubscriptionsImagesCommand returns the subscription images command group.
func SubscriptionsImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc subscriptions images <subcommand> [flags]",
		ShortHelp:  "Manage subscription images.",
		LongHelp: `Manage subscription images.

Examples:
  asc subscriptions images list --subscription-id "SUB_ID"
  asc subscriptions images create --subscription-id "SUB_ID" --file "./image.png"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsImagesListCommand(),
			SubscriptionsImagesGetCommand(),
			SubscriptionsImagesCreateCommand(),
			SubscriptionsImagesUpdateCommand(),
			SubscriptionsImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsImagesListCommand returns the images list subcommand.
func SubscriptionsImagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions images list [flags]",
		ShortHelp:  "List subscription images.",
		LongHelp: `List subscription images.

Examples:
  asc subscriptions images list --subscription-id "SUB_ID"
  asc subscriptions images list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions images list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions images list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions images list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionImagesOption{
				asc.WithSubscriptionImagesLimit(*limit),
				asc.WithSubscriptionImagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionImagesLimit(200))
				firstPage, err := client.GetSubscriptionImages(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions images list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionImages(ctx, id, asc.WithSubscriptionImagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions images list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionImages(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions images list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsImagesGetCommand returns the images get subcommand.
func SubscriptionsImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images get", flag.ExitOnError)

	imageID := fs.String("id", "", "Subscription image ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions images get --id \"IMAGE_ID\"",
		ShortHelp:  "Get a subscription image by ID.",
		LongHelp: `Get a subscription image by ID.

Examples:
  asc subscriptions images get --id "IMAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions images get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions images get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsImagesCreateCommand returns the images create subcommand.
func SubscriptionsImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	filePath := fs.String("file", "", "Path to image file")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions images create [flags]",
		ShortHelp:  "Upload a subscription image.",
		LongHelp: `Upload a subscription image.

Examples:
  asc subscriptions images create --subscription-id "SUB_ID" --file "./image.png"`,
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
				return fmt.Errorf("subscriptions images create: %w", err)
			}
			defer file.Close()

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions images create: %w", err)
			}

			requestCtx, cancel := contextWithUploadTimeout(ctx)
			defer cancel()

			resp, err := client.CreateSubscriptionImage(requestCtx, id, info.Name(), info.Size())
			if err != nil {
				return fmt.Errorf("subscriptions images create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("subscriptions images create: no upload operations returned")
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("subscriptions images create: upload failed: %w", err)
			}

			checksum, err := asc.ComputeFileChecksum(pathValue, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("subscriptions images create: checksum failed: %w", err)
			}

			uploaded := true
			updateAttrs := asc.SubscriptionImageUpdateAttributes{
				SourceFileChecksum: &checksum.Hash,
				Uploaded:           &uploaded,
			}

			commitResp, err := client.UpdateSubscriptionImage(requestCtx, resp.Data.ID, updateAttrs)
			if err != nil {
				return fmt.Errorf("subscriptions images create: failed to commit upload: %w", err)
			}
			if commitResp != nil {
				return printOutput(commitResp, *output, *pretty)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsImagesUpdateCommand returns the images update subcommand.
func SubscriptionsImagesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images update", flag.ExitOnError)

	imageID := fs.String("id", "", "Subscription image ID")
	checksum := fs.String("checksum", "", "Source file checksum (MD5)")
	var uploaded shared.OptionalBool
	fs.Var(&uploaded, "uploaded", "Mark upload complete: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions images update [flags]",
		ShortHelp:  "Update a subscription image.",
		LongHelp: `Update a subscription image.

Examples:
  asc subscriptions images update --id "IMAGE_ID" --uploaded true --checksum "HASH"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
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
				return fmt.Errorf("subscriptions images update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionImageUpdateAttributes{}
			if checksumValue != "" {
				attrs.SourceFileChecksum = &checksumValue
			}
			if uploaded.IsSet() {
				value := uploaded.Value()
				attrs.Uploaded = &value
			}

			resp, err := client.UpdateSubscriptionImage(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions images update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsImagesDeleteCommand returns the images delete subcommand.
func SubscriptionsImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Subscription image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a subscription image.",
		LongHelp: `Delete a subscription image.

Examples:
  asc subscriptions images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
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
				return fmt.Errorf("subscriptions images delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionImage(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions images delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{ID: id, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}
