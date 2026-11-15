package webhooks

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const webhooksMaxLimit = 200

// NewWebhooksCommand returns the webhooks command group built on d.
func NewWebhooksCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("webhooks", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "webhooks",
		ShortUsage: "asc webhooks <subcommand> [flags]",
		ShortHelp:  "Manage webhooks in App Store Connect.",
		LongHelp: `Manage webhooks in App Store Connect.

Examples:
  asc webhooks list --app "APP_ID"
  asc webhooks view --webhook-id "WEBHOOK_ID"
  asc webhooks create --app "APP_ID" --name "Build Updates" --url "https://example.com/webhook" --secret "secret123" --events "BUILD_UPLOAD_STATE_UPDATED" --enabled true
  asc webhooks update --webhook-id "WEBHOOK_ID" --url "https://new-url.com/webhook" --enabled false
  asc webhooks delete --webhook-id "WEBHOOK_ID" --confirm
  asc webhooks serve --port 8787 --dir ./webhook-events
  asc webhooks deliveries --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries links --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries redeliver --delivery-id "DELIVERY_ID"
  asc webhooks ping --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebhooksListCommand(d),
			newWebhooksGetCommand(d),
			newWebhooksCreateCommand(d),
			newWebhooksUpdateCommand(d),
			newWebhooksDeleteCommand(d),
			newWebhooksServeCommand(d),
			newWebhookDeliveriesCommand(d),
			newWebhookPingCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebhooksListCommand returns the webhooks list subcommand.
func newWebhooksListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:       d,
		Name:       "list",
		ShortUsage: "asc webhooks list [flags]",
		ShortHelp:  "List webhooks for an app.",
		LongHelp: `List webhooks for an app.

Examples:
  asc webhooks list --app "APP_ID"
  asc webhooks list --app "APP_ID" --limit 10
  asc webhooks list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		LimitMax:             webhooksMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"limit"},
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppWebhooks(ctx, appID, asc.Limit[asc.WebhooksOption](limit), asc.NextURL[asc.WebhooksOption](next))
		},
	})
}

// newWebhooksGetCommand returns the webhooks view subcommand.
func newWebhooksGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc webhooks view --webhook-id \"WEBHOOK_ID\" [flags]",
		ShortHelp:  "View a webhook by ID.",
		LongHelp: `View a webhook by ID.

Examples:
  asc webhooks view --webhook-id "WEBHOOK_ID"`,
		IDFlag:             "webhook-id",
		IDUsage:            "Webhook ID",
		IDType:             "webhooks",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetWebhook(ctx, id)
		},
	})
}

// newWebhooksCreateCommand returns the webhooks create subcommand.
func newWebhooksCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	name := fs.String("name", "", "Webhook name")
	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	events := shared.BindOnceCSVFlag(fs, "events", "Webhook event types (comma-separated)")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the webhook: true or false")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc webhooks create --app APP_ID --name NAME --url URL --secret SECRET --events EVENTS --enabled [true|false] [flags]",
		ShortHelp:  "Create a webhook.",
		LongHelp: `Create a webhook.

Examples:
  asc webhooks create --app "APP_ID" --name "Build Updates" --url "https://example.com/webhook" --secret "secret123" --events "BUILD_UPLOAD_STATE_UPDATED" --enabled true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*name) == "" {
				return d.RequiredFlag("--name")
			}
			if strings.TrimSpace(*url) == "" {
				return d.RequiredFlag("--url")
			}
			if strings.TrimSpace(*secret) == "" {
				return d.RequiredFlag("--secret")
			}
			if strings.TrimSpace(events.String()) == "" {
				return d.RequiredFlag("--events")
			}
			if !enabled.IsSet() {
				return d.RequiredFlag("--enabled")
			}

			eventTypes, err := normalizeWebhookEvents(events.String())
			if err != nil {
				return fmt.Errorf("webhooks create: %w", d.UsageError(err.Error()))
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("webhooks create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// newWebhooksUpdateCommand returns the webhooks update subcommand.
func newWebhooksUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "webhooks", "Webhook ID")
	name := fs.String("name", "", "Webhook name")
	url := fs.String("url", "", "Webhook endpoint URL")
	secret := fs.String("secret", "", "Webhook secret")
	events := shared.BindOnceCSVFlag(fs, "events", "Webhook event types (comma-separated)")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the webhook: true or false")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc webhooks update --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "Update a webhook.",
		LongHelp: `Update a webhook.

Examples:
  asc webhooks update --webhook-id "WEBHOOK_ID" --url "https://new-url.com/webhook"
  asc webhooks update --webhook-id "WEBHOOK_ID" --enabled false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				return d.RequiredFlag("--webhook-id")
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
			if strings.TrimSpace(events.String()) != "" {
				eventTypes, err := normalizeWebhookEvents(events.String())
				if err != nil {
					return fmt.Errorf("webhooks update: %w", d.UsageError(err.Error()))
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
				return d.UpdateFlagRequired(fs, name, url, secret, events, &enabled)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("webhooks update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			webhook, err := client.UpdateWebhook(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("webhooks update: failed to update: %w", err)
			}

			return d.PrintOutput(webhook, *output.Output, *output.Pretty)
		},
	}
}

// newWebhooksDeleteCommand returns the webhooks delete subcommand.
func newWebhooksDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc webhooks delete --webhook-id WEBHOOK_ID --confirm [flags]",
		ShortHelp:  "Delete a webhook.",
		LongHelp: `Delete a webhook.

Examples:
  asc webhooks delete --webhook-id "WEBHOOK_ID" --confirm`,
		IDFlag:             "webhook-id",
		IDUsage:            "Webhook ID",
		IDType:             "webhooks",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteWebhook(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newWebhookDeliveriesCommand returns the webhook deliveries command.
func newWebhookDeliveriesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("deliveries", flag.ExitOnError)

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "webhooks", "Webhook ID")
	createdAfter := fs.String("created-after", "", "Filter deliveries created after or equal to a timestamp")
	createdBefore := fs.String("created-before", "", "Filter deliveries created before a timestamp")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "deliveries",
		ShortUsage: "asc webhooks deliveries --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "List webhook deliveries.",
		LongHelp: `List webhook deliveries.

Examples:
  asc webhooks deliveries --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --created-after "2026-01-01T00:00:00Z"
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --created-after "2026-01-01T00:00:00Z" --created-before "2026-01-02T00:00:00Z"
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --limit 10
  asc webhooks deliveries --webhook-id "WEBHOOK_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebhookDeliveriesRelationshipsCommand(d),
			newWebhookDeliveriesRedeliverCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "webhooks deliveries", webhooksMaxLimit, "created-after", "created-before"); err != nil {
				return err
			}
			trimmedID := strings.TrimSpace(*webhookID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--webhook-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("webhooks deliveries: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.WebhookDeliveriesOption{
				asc.Limit[asc.WebhookDeliveriesOption](*limit),
				asc.NextURL[asc.WebhookDeliveriesOption](*next),
			}
			if strings.TrimSpace(*createdAfter) != "" {
				values := shared.SplitCSV(*createdAfter)
				if len(values) == 0 {
					fmt.Fprintln(d.IO.ErrOut, "Error: --created-after must include at least one value")
					return flag.ErrHelp
				}
				opts = append(opts, asc.WithWebhookDeliveriesCreatedAfter(values))
			}
			if strings.TrimSpace(*createdBefore) != "" {
				values := shared.SplitCSV(*createdBefore)
				if len(values) == 0 {
					fmt.Fprintln(d.IO.ErrOut, "Error: --created-before must include at least one value")
					return flag.ErrHelp
				}
				opts = append(opts, asc.WithWebhookDeliveriesCreatedBefore(values))
			}

			if *paginate {
				if trimmedID == "" {
					return d.RequiredFlag("--webhook-id")
				}
				paginateOpts := append(opts, asc.Limit[asc.WebhookDeliveriesOption](webhooksMaxLimit))
				firstPage, err := client.GetWebhookDeliveries(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("webhooks deliveries: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetWebhookDeliveries(ctx, trimmedID, asc.NextURL[asc.WebhookDeliveriesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("webhooks deliveries: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			deliveries, err := client.GetWebhookDeliveries(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("webhooks deliveries: failed to fetch: %w", err)
			}

			return d.PrintOutput(deliveries, *output.Output, *output.Pretty)
		},
	}
}

// newWebhookDeliveriesRelationshipsCommand returns the webhook deliveries links subcommand.
func newWebhookDeliveriesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("links", flag.ExitOnError)

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "webhooks", "Webhook ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc webhooks deliveries links --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "List webhook delivery relationships.",
		LongHelp: `List webhook delivery relationships.

Examples:
  asc webhooks deliveries links --webhook-id "WEBHOOK_ID"
  asc webhooks deliveries links --webhook-id "WEBHOOK_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "webhooks deliveries links", webhooksMaxLimit); err != nil {
				return err
			}
			trimmedID := strings.TrimSpace(*webhookID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--webhook-id")
			}
			if trimmedID == "" && trimmedNext != "" {
				derivedID, err := extractWebhookIDFromNextURL(trimmedNext)
				if err != nil {
					return d.UsageErrorf("webhooks deliveries links: %v", err)
				}
				trimmedID = derivedID
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("webhooks deliveries links: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				if trimmedID == "" {
					return d.RequiredFlag("--webhook-id")
				}
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](webhooksMaxLimit))
				firstPage, err := client.GetWebhookDeliveriesRelationships(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("webhooks deliveries links: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetWebhookDeliveriesRelationships(ctx, trimmedID, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("webhooks deliveries links: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetWebhookDeliveriesRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("webhooks deliveries links: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newWebhookDeliveriesRedeliverCommand returns the webhook deliveries redeliver subcommand.
func newWebhookDeliveriesRedeliverCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("redeliver", flag.ExitOnError)

	deliveryID := fs.String("delivery-id", "", "Webhook delivery ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "redeliver",
		ShortUsage: "asc webhooks deliveries redeliver --delivery-id DELIVERY_ID [flags]",
		ShortHelp:  "Redeliver a webhook delivery.",
		LongHelp: `Redeliver a webhook delivery.

Examples:
  asc webhooks deliveries redeliver --delivery-id "DELIVERY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*deliveryID)
			if trimmedID == "" {
				return d.RequiredFlag("--delivery-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("webhooks deliveries redeliver: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateWebhookDelivery(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("webhooks deliveries redeliver: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newWebhookPingCommand returns the webhook ping subcommand.
func newWebhookPingCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("ping", flag.ExitOnError)

	webhookID := shared.BindResourceIDFlag(fs, "webhook-id", "webhooks", "Webhook ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "ping",
		ShortUsage: "asc webhooks ping --webhook-id WEBHOOK_ID [flags]",
		ShortHelp:  "Create a webhook ping.",
		LongHelp: `Create a webhook ping.

Examples:
  asc webhooks ping --webhook-id "WEBHOOK_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*webhookID)
			if trimmedID == "" {
				return d.RequiredFlag("--webhook-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("webhooks ping: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateWebhookPing(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("webhooks ping: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
