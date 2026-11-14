package marketplace

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newMarketplaceWebhooksCommand returns the marketplace webhooks command group.
func newMarketplaceWebhooksCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("webhooks", flag.ExitOnError)

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
			newMarketplaceWebhooksListCommand(d),
			newMarketplaceWebhooksGetCommand(d),
			newMarketplaceWebhooksCreateCommand(d),
			newMarketplaceWebhooksUpdateCommand(d),
			newMarketplaceWebhooksDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newMarketplaceWebhooksListCommand returns the webhooks list subcommand.
func newMarketplaceWebhooksListCommand(d *shared.Deps) *ffcli.Command {
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
					return d.UsageErrorf("marketplace webhooks list: %v", err)
				}
				return nil
			}
		},
		ErrorPrefix:      "marketplace webhooks list",
		FetchErrorPrefix: "marketplace webhooks list: failed to fetch",
		Deps:             d,
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
	return withMarketplaceWebhooksDeprecationWarning(d, cmd)
}

// newMarketplaceWebhooksGetCommand returns the webhooks view subcommand.
func newMarketplaceWebhooksGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetMarketplaceWebhook(ctx, id)
		},
	})
	return withMarketplaceWebhooksDeprecationWarning(d, cmd)
}

// newMarketplaceWebhooksCreateCommand returns the webhooks create subcommand.
func newMarketplaceWebhooksCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	output := d.BindOutputFlags(fs)

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
			warnMarketplaceWebhooksDeprecated(d)

			endpointURL := strings.TrimSpace(*url)
			if endpointURL == "" {
				return d.RequiredFlag("--url")
			}
			secretValue := strings.TrimSpace(*secret)
			if secretValue == "" {
				return d.RequiredFlag("--secret")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("marketplace webhooks create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			webhook, err := client.CreateMarketplaceWebhook(requestCtx, endpointURL, secretValue)
			if err != nil {
				return fmt.Errorf("marketplace webhooks create: failed to create: %w", err)
			}

			return d.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// newMarketplaceWebhooksUpdateCommand returns the webhooks update subcommand.
func newMarketplaceWebhooksUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "marketplaceWebhooks", "Marketplace webhook ID")
	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	output := d.BindOutputFlags(fs)

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
			warnMarketplaceWebhooksDeprecated(d)

			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				return d.RequiredFlag("--webhook-id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["url"] && !visited["secret"] {
				fmt.Fprintln(d.IO.ErrOut, "Error: at least one update flag is required")
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

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("marketplace webhooks update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			webhook, err := client.UpdateMarketplaceWebhook(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("marketplace webhooks update: failed to update: %w", err)
			}

			return d.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// newMarketplaceWebhooksDeleteCommand returns the webhooks delete subcommand.
func newMarketplaceWebhooksDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "marketplaceWebhooks", "Marketplace webhook ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

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
			warnMarketplaceWebhooksDeprecated(d)

			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				return d.RequiredFlag("--webhook-id")
			}
			if !*confirm {
				return d.RequiredFlag("--confirm")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("marketplace webhooks delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteMarketplaceWebhook(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("marketplace webhooks delete: failed to delete: %w", err)
			}

			result := &asc.MarketplaceWebhookDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
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

func warnMarketplaceWebhooksDeprecated(d *shared.Deps) {
	fmt.Fprintln(d.IO.ErrOut, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.")
}

// withMarketplaceWebhooksDeprecationWarning prints the deprecation warning before
// any of cmd's own checks run.
func withMarketplaceWebhooksDeprecationWarning(d *shared.Deps, cmd *ffcli.Command) *ffcli.Command {
	exec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		warnMarketplaceWebhooksDeprecated(d)
		return exec(ctx, args)
	}
	return cmd
}
