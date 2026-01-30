package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// SubscriptionsLocalizationsCommand returns the subscription localizations command group.
func SubscriptionsLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc subscriptions localizations <subcommand> [flags]",
		ShortHelp:  "Manage subscription localizations.",
		LongHelp: `Manage subscription localizations.

Examples:
  asc subscriptions localizations list --subscription-id "SUB_ID"
  asc subscriptions localizations create --subscription-id "SUB_ID" --locale "en-US" --name "Pro"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsLocalizationsListCommand(),
			SubscriptionsLocalizationsGetCommand(),
			SubscriptionsLocalizationsCreateCommand(),
			SubscriptionsLocalizationsUpdateCommand(),
			SubscriptionsLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsLocalizationsListCommand returns the localizations list subcommand.
func SubscriptionsLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions localizations list [flags]",
		ShortHelp:  "List subscription localizations.",
		LongHelp: `List subscription localizations.

Examples:
  asc subscriptions localizations list --subscription-id "SUB_ID"
  asc subscriptions localizations list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions localizations list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions localizations list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionLocalizationsOption{
				asc.WithSubscriptionLocalizationsLimit(*limit),
				asc.WithSubscriptionLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionLocalizationsLimit(200))
				firstPage, err := client.GetSubscriptionLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionLocalizations(ctx, id, asc.WithSubscriptionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions localizations list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions localizations list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsLocalizationsGetCommand returns the localizations get subcommand.
func SubscriptionsLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a subscription localization by ID.",
		LongHelp: `Get a subscription localization by ID.

Examples:
  asc subscriptions localizations get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions localizations get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsLocalizationsCreateCommand returns the localizations create subcommand.
func SubscriptionsLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions localizations create [flags]",
		ShortHelp:  "Create a subscription localization.",
		LongHelp: `Create a subscription localization.

Examples:
  asc subscriptions localizations create --subscription-id "SUB_ID" --locale "en-US" --name "Pro"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionLocalizationCreateAttributes{
				Name:   nameValue,
				Locale: localeValue,
			}
			if desc := strings.TrimSpace(*description); desc != "" {
				attrs.Description = desc
			}

			resp, err := client.CreateSubscriptionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions localizations create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsLocalizationsUpdateCommand returns the localizations update subcommand.
func SubscriptionsLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription localization ID")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions localizations update [flags]",
		ShortHelp:  "Update a subscription localization.",
		LongHelp: `Update a subscription localization.

Examples:
  asc subscriptions localizations update --id "LOC_ID" --name "Pro+"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			descriptionValue := strings.TrimSpace(*description)
			if nameValue == "" && descriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions localizations update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionLocalizationUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if descriptionValue != "" {
				attrs.Description = &descriptionValue
			}

			resp, err := client.UpdateSubscriptionLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions localizations update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsLocalizationsDeleteCommand returns the localizations delete subcommand.
func SubscriptionsLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a subscription localization.",
		LongHelp: `Delete a subscription localization.

Examples:
  asc subscriptions localizations delete --id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
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
				return fmt.Errorf("subscriptions localizations delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions localizations delete: failed to delete: %w", err)
			}

			result := &deleteResult{ID: id, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}
