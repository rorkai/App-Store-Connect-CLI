package webhooks

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

const webhooksMaxLimit = 200

// WebhooksCommand returns the webhooks command group.
func WebhooksCommand() *ffcli.Command {
	fs := flag.NewFlagSet("webhooks", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "webhooks",
		ShortUsage: "asc webhooks <subcommand> [flags]",
		ShortHelp:  "Manage App Store Connect webhooks.",
		LongHelp: `Manage App Store Connect webhooks.

Examples:
  asc webhooks list --app "APP_ID"
  asc webhooks get --webhook-id "WEBHOOK_ID"
  asc webhooks create --app "APP_ID" --name "Build Updates" --url "https://example.com/webhook" --secret "secret123" --events "SUBSCRIPTION.CREATED,SUBSCRIPTION.UPDATED" --enabled true
  asc webhooks update --webhook-id "WEBHOOK_ID" --url "https://new-url.com/webhook" --enabled false
  asc webhooks delete --webhook-id "WEBHOOK_ID" --confirm
  asc webhooks deliveries --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries relationships --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries redeliver --delivery-id "DELIVERY_ID"
  asc webhooks ping --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebhooksListCommand(),
			WebhooksGetCommand(),
			WebhooksCreateCommand(),
			WebhooksUpdateCommand(),
			WebhooksDeleteCommand(),
			WebhookDeliveriesCommand(),
			WebhookPingCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebhooksListCommand returns the webhooks list subcommand.
func WebhooksListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc webhooks list [flags]",
		ShortHelp:  "List webhooks for an app.",
		LongHelp: `List webhooks for an app.

Examples:
  asc webhooks list --app "APP_ID"
  asc webhooks list --app "APP_ID" --limit 10
  asc webhooks list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > webhooksMaxLimit) {
				return fmt.Errorf("webhooks list: --limit must be between 1 and %d", webhooksMaxLimit)
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("webhooks list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.WebhooksOption{
				asc.WithWebhooksLimit(*limit),
				asc.WithWebhooksNextURL(*next),
			}

			if *paginate {
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithWebhooksLimit(webhooksMaxLimit))
				firstPage, err := client.GetAppWebhooks(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("webhooks list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppWebhooks(ctx, resolvedAppID, asc.WithWebhooksNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("webhooks list: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			webhooks, err := client.GetAppWebhooks(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("webhooks list: failed to fetch: %w", err)
			}

			return printOutput(webhooks, *output, *pretty)
		},
	}
}

// WebhooksGetCommand returns the webhooks get subcommand.
func WebhooksGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Webhook ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc webhooks get --webhook-id \"WEBHOOK_ID\" [flags]",
		ShortHelp:  "Get a webhook by ID.",
		LongHelp: `Get a webhook by ID.

Examples:
  asc webhooks get --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			webhook, err := client.GetWebhook(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("webhooks get: failed to fetch: %w", err)
			}

			return printOutput(webhook, *output, *pretty)
		},
	}
}

// WebhooksCreateCommand returns the webhooks create subcommand.
func WebhooksCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	name := fs.String("name", "", "Webhook name")
	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	events := fs.String("events", "", "Webhook event types (comma-separated)")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the webhook: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc webhooks create --app APP_ID --name NAME --url URL --secret SECRET --events EVENTS --enabled [true|false] [flags]",
		ShortHelp:  "Create a webhook.",
		LongHelp: `Create a webhook.

Examples:
  asc webhooks create --app "APP_ID" --name "Build Updates" --url "https://example.com/webhook" --secret "secret123" --events "SUBSCRIPTION.CREATED,SUBSCRIPTION.UPDATED" --enabled true`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*url) == "" {
				fmt.Fprintln(os.Stderr, "Error: --url is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*secret) == "" {
				fmt.Fprintln(os.Stderr, "Error: --secret is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*events) == "" {
				fmt.Fprintln(os.Stderr, "Error: --events is required")
				return flag.ErrHelp
			}
			if !enabled.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --enabled is required")
				return flag.ErrHelp
			}

			eventTypes, err := normalizeWebhookEvents(*events)
			if err != nil {
				return fmt.Errorf("webhooks create: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.WebhookCreateAttributes{
				Enabled:    enabled.Value(),
				EventTypes: eventTypes,
				Name:       strings.TrimSpace(*name),
				Secret:     strings.TrimSpace(*secret),
				URL:        strings.TrimSpace(*url),
			}

			webhook, err := client.CreateWebhook(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("webhooks create: failed to create: %w", err)
			}

			return printOutput(webhook, *output, *pretty)
		},
	}
}

// WebhooksUpdateCommand returns the webhooks update subcommand.
func WebhooksUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Webhook ID")
	name := fs.String("name", "", "Webhook name")
	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	events := fs.String("events", "", "Webhook event types (comma-separated)")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the webhook: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc webhooks update --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "Update a webhook.",
		LongHelp: `Update a webhook.

Examples:
  asc webhooks update --webhook-id "WEBHOOK_ID" --url "https://new-url.com/webhook"
  asc webhooks update --webhook-id "WEBHOOK_ID" --enabled false`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}

			attrs := asc.WebhookUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				value := strings.TrimSpace(*name)
				attrs.Name = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*url) != "" {
				value := strings.TrimSpace(*url)
				attrs.URL = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*secret) != "" {
				value := strings.TrimSpace(*secret)
				attrs.Secret = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*events) != "" {
				eventTypes, err := normalizeWebhookEvents(*events)
				if err != nil {
					return fmt.Errorf("webhooks update: %w", err)
				}
				attrs.EventTypes = eventTypes
				hasUpdate = true
			}
			if enabled.IsSet() {
				value := enabled.Value()
				attrs.Enabled = &value
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: --name, --url, --secret, --events, or --enabled is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			webhook, err := client.UpdateWebhook(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("webhooks update: failed to update: %w", err)
			}

			return printOutput(webhook, *output, *pretty)
		},
	}
}

// WebhooksDeleteCommand returns the webhooks delete subcommand.
func WebhooksDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Webhook ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc webhooks delete --webhook-id WEBHOOK_ID --confirm [flags]",
		ShortHelp:  "Delete a webhook.",
		LongHelp: `Delete a webhook.

Examples:
  asc webhooks delete --webhook-id "WEBHOOK_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteWebhook(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("webhooks delete: failed to delete: %w", err)
			}

			result := &asc.WebhookDeleteResult{ID: trimmedID, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}

// WebhookDeliveriesCommand returns the webhook deliveries command.
func WebhookDeliveriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("deliveries", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Webhook ID")
	createdAfter := fs.String("created-after", "", "Filter deliveries created after or equal to a timestamp")
	createdBefore := fs.String("created-before", "", "Filter deliveries created before a timestamp")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "deliveries",
		ShortUsage: "asc webhooks deliveries --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "List webhook deliveries.",
		LongHelp: `List webhook deliveries.

Examples:
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --created-after "2026-01-01T00:00:00Z"
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --limit 10
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebhookDeliveriesRelationshipsCommand(),
			WebhookDeliveriesRedeliverCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}
			filterCount := 0
			if strings.TrimSpace(*createdAfter) != "" {
				filterCount++
			}
			if strings.TrimSpace(*createdBefore) != "" {
				filterCount++
			}
			if trimmedNext == "" {
				if filterCount == 0 {
					fmt.Fprintln(os.Stderr, "Error: --created-after or --created-before is required")
					return flag.ErrHelp
				}
				if filterCount > 1 {
					fmt.Fprintln(os.Stderr, "Error: only one of --created-after or --created-before can be used")
					return flag.ErrHelp
				}
			}
			if *limit != 0 && (*limit < 1 || *limit > webhooksMaxLimit) {
				return fmt.Errorf("webhooks deliveries: --limit must be between 1 and %d", webhooksMaxLimit)
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("webhooks deliveries: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks deliveries: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.WebhookDeliveriesOption{
				asc.WithWebhookDeliveriesLimit(*limit),
				asc.WithWebhookDeliveriesNextURL(*next),
			}
			if strings.TrimSpace(*createdAfter) != "" {
				values := splitCSV(*createdAfter)
				if len(values) == 0 {
					fmt.Fprintln(os.Stderr, "Error: --created-after must include at least one value")
					return flag.ErrHelp
				}
				opts = append(opts, asc.WithWebhookDeliveriesCreatedAfter(values))
			}
			if strings.TrimSpace(*createdBefore) != "" {
				values := splitCSV(*createdBefore)
				if len(values) == 0 {
					fmt.Fprintln(os.Stderr, "Error: --created-before must include at least one value")
					return flag.ErrHelp
				}
				opts = append(opts, asc.WithWebhookDeliveriesCreatedBefore(values))
			}

			if *paginate {
				if trimmedID == "" {
					fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithWebhookDeliveriesLimit(webhooksMaxLimit))
				firstPage, err := client.GetWebhookDeliveries(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("webhooks deliveries: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetWebhookDeliveries(ctx, trimmedID, asc.WithWebhookDeliveriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("webhooks deliveries: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			deliveries, err := client.GetWebhookDeliveries(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("webhooks deliveries: failed to fetch: %w", err)
			}

			return printOutput(deliveries, *output, *pretty)
		},
	}
}

// WebhookDeliveriesRelationshipsCommand returns the webhook deliveries relationships subcommand.
func WebhookDeliveriesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Webhook ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc webhooks deliveries relationships --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "List webhook delivery relationships.",
		LongHelp: `List webhook delivery relationships.

Examples:
  asc webhooks deliveries relationships --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries relationships --webhook-id "WEBHOOK_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > webhooksMaxLimit) {
				return fmt.Errorf("webhooks deliveries relationships: --limit must be between 1 and %d", webhooksMaxLimit)
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("webhooks deliveries relationships: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks deliveries relationships: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				if trimmedID == "" {
					fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithLinkagesLimit(webhooksMaxLimit))
				firstPage, err := client.GetWebhookDeliveriesRelationships(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("webhooks deliveries relationships: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetWebhookDeliveriesRelationships(ctx, trimmedID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("webhooks deliveries relationships: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetWebhookDeliveriesRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("webhooks deliveries relationships: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// WebhookDeliveriesRedeliverCommand returns the webhook deliveries redeliver subcommand.
func WebhookDeliveriesRedeliverCommand() *ffcli.Command {
	fs := flag.NewFlagSet("redeliver", flag.ExitOnError)

	deliveryID := fs.String("delivery-id", "", "Webhook delivery ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "redeliver",
		ShortUsage: "asc webhooks deliveries redeliver --delivery-id DELIVERY_ID [flags]",
		ShortHelp:  "Redeliver a webhook delivery.",
		LongHelp: `Redeliver a webhook delivery.

Examples:
  asc webhooks deliveries redeliver --delivery-id "DELIVERY_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*deliveryID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --delivery-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks deliveries redeliver: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateWebhookDelivery(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("webhooks deliveries redeliver: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// WebhookPingCommand returns the webhook ping subcommand.
func WebhookPingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ping", flag.ExitOnError)

	webhookID := fs.String("webhook-id", "", "Webhook ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "ping",
		ShortUsage: "asc webhooks ping --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "Create a webhook ping.",
		LongHelp: `Create a webhook ping.

Examples:
  asc webhooks ping --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --webhook-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("webhooks ping: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateWebhookPing(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("webhooks ping: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
