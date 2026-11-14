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
  asc marketplace webhooks view --webhook-id "WEBHOOK_ID"`,
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
	var fieldsValue []string
	cmd := shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc marketplace webhooks list [flags]",
		ShortHelp:   "List marketplace webhooks.",
		LongHelp: `List marketplace webhooks.

Examples:
  asc marketplace webhooks list
  asc marketplace webhooks list --limit 10
  asc marketplace webhooks list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"fields"},
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(marketplaceWebhookFieldsList(), ", "))
			return func() (err error) {
				if fieldsValue, err = normalizeMarketplaceWebhookFields(*fields); err != nil {
					return shared.UsageErrorf("marketplace webhooks list: %v", err)
				}
				return nil
			}
		},
		ErrorPrefix:      "marketplace webhooks list",
		FetchErrorPrefix: "marketplace webhooks list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.MarketplaceWebhooksOption{
				asc.Limit[asc.MarketplaceWebhooksOption](limit),
				asc.NextURL[asc.MarketplaceWebhooksOption](next),
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithMarketplaceWebhooksFields(fieldsValue))
			}
			return client.GetMarketplaceWebhooks(ctx, opts...)
		},
	})
	return withMarketplaceWebhooksDeprecationWarning(cmd)
}

// MarketplaceWebhooksGetCommand returns the webhooks view subcommand.
func MarketplaceWebhooksGetCommand() *ffcli.Command {
	cmd := shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc marketplace webhooks view --webhook-id \"WEBHOOK_ID\" [flags]",
		ShortHelp:   "View a marketplace webhook by ID.",
		LongHelp: `View a marketplace webhook by ID.

Examples:
  asc marketplace webhooks view --webhook-id "WEBHOOK_ID"`,
		IDFlag:             "webhook-id",
		IDUsage:            "Marketplace webhook ID",
		IDType:             "marketplaceWebhooks",
		RequiredDiagnostic: true,
		ErrorPrefix:        "marketplace webhooks view",
		FetchErrorPrefix:   "marketplace webhooks view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetMarketplaceWebhook(ctx, id)
		},
	})
	return withMarketplaceWebhooksDeprecationWarning(cmd)
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
				return shared.RequiredFlag("--url")
			}
			secretValue := strings.TrimSpace(*secret)
			if secretValue == "" {
				return shared.RequiredFlag("--secret")
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

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "marketplaceWebhooks", "Marketplace webhook ID")
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
				return shared.RequiredFlag("--webhook-id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["url"] && !visited["secret"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
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

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "marketplaceWebhooks", "Marketplace webhook ID")
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
				return shared.RequiredFlag("--webhook-id")
			}
			if !*confirm {
				return shared.RequiredFlag("--confirm")
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

// withMarketplaceWebhooksDeprecationWarning prints the deprecation warning before
// any of cmd's own checks run.
func withMarketplaceWebhooksDeprecationWarning(cmd *ffcli.Command) *ffcli.Command {
	exec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		warnMarketplaceWebhooksDeprecated()
		return exec(ctx, args)
	}
	return cmd
}
