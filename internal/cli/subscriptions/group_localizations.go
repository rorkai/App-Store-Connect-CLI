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

// SubscriptionsGroupsLocalizationsCommand returns the group localizations command group.
func SubscriptionsGroupsLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc subscriptions groups localizations <subcommand> [flags]",
		ShortHelp:  "Manage subscription group localizations.",
		LongHelp: `Manage subscription group localizations.

Examples:
  asc subscriptions groups localizations list --group-id "GROUP_ID"
  asc subscriptions groups localizations create --group-id "GROUP_ID" --locale "en-US" --name "Premium"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsLocalizationsListCommand(),
			SubscriptionsGroupsLocalizationsGetCommand(),
			SubscriptionsGroupsLocalizationsCreateCommand(),
			SubscriptionsGroupsLocalizationsUpdateCommand(),
			SubscriptionsGroupsLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGroupsLocalizationsListCommand returns the group localizations list subcommand.
func SubscriptionsGroupsLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Subscription group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions groups localizations list [flags]",
		ShortHelp:  "List subscription group localizations.",
		LongHelp: `List subscription group localizations.

Examples:
  asc subscriptions groups localizations list --group-id "GROUP_ID"
  asc subscriptions groups localizations list --group-id "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions groups localizations list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions groups localizations list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionGroupLocalizationsOption{
				asc.WithSubscriptionGroupLocalizationsLimit(*limit),
				asc.WithSubscriptionGroupLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionGroupLocalizationsLimit(200))
				firstPage, err := client.GetSubscriptionGroupLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions groups localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionGroupLocalizations(ctx, id, asc.WithSubscriptionGroupLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions groups localizations list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionGroupLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsLocalizationsGetCommand returns the group localizations get subcommand.
func SubscriptionsGroupsLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription group localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions groups localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a subscription group localization by ID.",
		LongHelp: `Get a subscription group localization by ID.

Examples:
  asc subscriptions groups localizations get --id "LOC_ID"`,
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
				return fmt.Errorf("subscriptions groups localizations get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionGroupLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsLocalizationsCreateCommand returns the group localizations create subcommand.
func SubscriptionsGroupsLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations create", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Subscription group ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	customAppName := fs.String("custom-app-name", "", "Custom app name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions groups localizations create [flags]",
		ShortHelp:  "Create a subscription group localization.",
		LongHelp: `Create a subscription group localization.

Examples:
  asc subscriptions groups localizations create --group-id "GROUP_ID" --locale "en-US" --name "Premium"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
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
				return fmt.Errorf("subscriptions groups localizations create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupLocalizationCreateAttributes{
				Name:   nameValue,
				Locale: localeValue,
			}
			if customName := strings.TrimSpace(*customAppName); customName != "" {
				attrs.CustomAppName = customName
			}

			resp, err := client.CreateSubscriptionGroupLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsLocalizationsUpdateCommand returns the group localizations update subcommand.
func SubscriptionsGroupsLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription group localization ID")
	name := fs.String("name", "", "Localized name")
	customAppName := fs.String("custom-app-name", "", "Custom app name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions groups localizations update [flags]",
		ShortHelp:  "Update a subscription group localization.",
		LongHelp: `Update a subscription group localization.

Examples:
  asc subscriptions groups localizations update --id "LOC_ID" --name "Premium+"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			customValue := strings.TrimSpace(*customAppName)
			if nameValue == "" && customValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupLocalizationUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if customValue != "" {
				attrs.CustomAppName = &customValue
			}

			resp, err := client.UpdateSubscriptionGroupLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups localizations update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsLocalizationsDeleteCommand returns the group localizations delete subcommand.
func SubscriptionsGroupsLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups localizations delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Subscription group localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions groups localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a subscription group localization.",
		LongHelp: `Delete a subscription group localization.

Examples:
  asc subscriptions groups localizations delete --id "LOC_ID" --confirm`,
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
				return fmt.Errorf("subscriptions groups localizations delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionGroupLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions groups localizations delete: failed to delete: %w", err)
			}

			result := &deleteResult{ID: id, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}
