package marketplace

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

// MarketplaceWebhooksCommand returns the marketplace webhooks command group.
func MarketplaceWebhooksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("webhooks", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "webhooks",
		ShortUsage: "asc marketplace webhooks <subcommand> [flags]",
		ShortHelp:  "Manage marketplace webhooks.",
		LongHelp: `Manage marketplace webhooks.

Examples:
  asc marketplace webhooks list
  asc marketplace webhooks get --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MarketplaceWebhooksListCommand(),
			MarketplaceWebhooksGetCommand(),
			MarketplaceWebhooksCreateCommand(),
			MarketplaceWebhooksUpdateCommand(),
			MarketplaceWebhooksDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MarketplaceWebhooksListCommand returns the webhooks list subcommand.
func MarketplaceWebhooksListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	fields := fs.String("fields", "", "Fields to include: "+strings.Join(marketplaceWebhookFieldsList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc marketplace webhooks list [flags]",
		ShortHelp:  "List marketplace webhooks.",
		LongHelp: `List marketplace webhooks.

Examples:
  asc marketplace webhooks list
  asc marketplace webhooks list --limit 10
  asc marketplace webhooks list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			warnMarketplaceWebhooksDeprecated()

			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("marketplace webhooks list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("marketplace webhooks list: %w", err)
			}

			fieldsValue, err := normalizeMarketplaceWebhookFields(*fields)
			if err != nil {
				return fmt.Errorf("marketplace webhooks list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace webhooks list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.MarketplaceWebhooksOption{
				asc.WithMarketplaceWebhooksLimit(*limit),
				asc.WithMarketplaceWebhooksNextURL(*next),
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithMarketplaceWebhooksFields(fieldsValue))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithMarketplaceWebhooksLimit(200))
				firstPage, err := client.GetMarketplaceWebhooks(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("marketplace webhooks list: failed to fetch: %w", err)
				}

				webhooks, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetMarketplaceWebhooks(ctx, asc.WithMarketplaceWebhooksNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("marketplace webhooks list: %w", err)
				}

				return shared.PrintOutput(webhooks, *output.Output, *output.Pretty)
			}

			webhooks, err := client.GetMarketplaceWebhooks(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("marketplace webhooks list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(webhooks, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceWebhooksGetCommand returns the webhooks get subcommand.
func MarketplaceWebhooksGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Marketplace webhook ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc marketplace webhooks get --webhook-id \"WEBHOOK_ID\" [flags]",
		ShortHelp:  "Get a marketplace webhook by ID.",
		LongHelp: `Get a marketplace webhook by ID.

Examples:
  asc marketplace webhooks get --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			warnMarketplaceWebhooksDeprecated()

			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace webhooks get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			webhook, err := client.GetMarketplaceWebhook(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("marketplace webhooks get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceWebhooksCreateCommand returns the webhooks create subcommand.
func MarketplaceWebhooksCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc marketplace webhooks create --url \"URL\" --secret \"SECRET\" [flags]",
		ShortHelp:  "Create a marketplace webhook.",
		LongHelp: `Create a marketplace webhook.

Examples:
  asc marketplace webhooks create --url "https://example.com/webhook" --secret "placeholder123"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			warnMarketplaceWebhooksDeprecated()

			endpointURL := strings.TrimSpace(*url)
			if endpointURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --url is required")
				return flag.ErrHelp
			}
			secretValue := strings.TrimSpace(*secret)
			if secretValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --secret is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace webhooks create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			webhook, err := client.CreateMarketplaceWebhook(requestCtx, endpointURL, secretValue)
			if err != nil {
				return fmt.Errorf("marketplace webhooks create: failed to create: %w", err)
			}

			return shared.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceWebhooksUpdateCommand returns the webhooks update subcommand.
func MarketplaceWebhooksUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Marketplace webhook ID")
	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc marketplace webhooks update --webhook-id \"WEBHOOK_ID\" [flags]",
		ShortHelp:  "Update a marketplace webhook.",
		LongHelp: `Update a marketplace webhook.

Examples:
  asc marketplace webhooks update --webhook-id "WEBHOOK_ID" --url "https://example.com/webhook"
  asc marketplace webhooks update --webhook-id "WEBHOOK_ID" --secret "new-secret"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			warnMarketplaceWebhooksDeprecated()

			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["url"] && !visited["secret"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			attrs := asc.MarketplaceWebhookUpdateAttributes{}
			if visited["url"] {
				value := strings.TrimSpace(*url)
				attrs.EndpointURL = &value
			}
			if visited["secret"] {
				value := strings.TrimSpace(*secret)
				attrs.Secret = &value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace webhooks update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			webhook, err := client.UpdateMarketplaceWebhook(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("marketplace webhooks update: failed to update: %w", err)
			}

			return shared.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceWebhooksDeleteCommand returns the webhooks delete subcommand.
func MarketplaceWebhooksDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Marketplace webhook ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc marketplace webhooks delete --webhook-id \"WEBHOOK_ID\" --confirm",
		ShortHelp:  "Delete a marketplace webhook.",
		LongHelp: `Delete a marketplace webhook.

Examples:
  asc marketplace webhooks delete --webhook-id "WEBHOOK_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			warnMarketplaceWebhooksDeprecated()

			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace webhooks delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteMarketplaceWebhook(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("marketplace webhooks delete: failed to delete: %w", err)
			}

			result := &asc.MarketplaceWebhookDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeMarketplaceWebhookFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range marketplaceWebhookFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(marketplaceWebhookFieldsList(), ", "))
		}
	}
	return fields, nil
}

func marketplaceWebhookFieldsList() []string {
	return []string{"endpointUrl"}
}

func warnMarketplaceWebhooksDeprecated() {
	fmt.Fprintln(os.Stderr, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.")
}
