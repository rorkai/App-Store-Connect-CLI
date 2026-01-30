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

// SubscriptionsCommand returns the subscriptions command group.
func SubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscriptions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc subscriptions <subcommand> [flags]",
		ShortHelp:  "Manage subscription groups and subscriptions.",
		LongHelp: `Manage subscription groups and subscriptions.

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions groups create --app "APP_ID" --reference-name "Premium"
  asc subscriptions list --group "GROUP_ID"
  asc subscriptions create --group "GROUP_ID" --ref-name "Monthly" --product-id "com.example.sub.monthly"
  asc subscriptions prices add --id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions availability set --id "SUB_ID" --territory "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsCommand(),
			SubscriptionsListCommand(),
			SubscriptionsCreateCommand(),
			SubscriptionsGetCommand(),
			SubscriptionsUpdateCommand(),
			SubscriptionsDeleteCommand(),
			SubscriptionsPricesCommand(),
			SubscriptionsAvailabilityCommand(),
			SubscriptionsLocalizationsCommand(),
			SubscriptionsImagesCommand(),
			SubscriptionsIntroductoryOffersCommand(),
			SubscriptionsPromotionalOffersCommand(),
			SubscriptionsOfferCodesCommand(),
			SubscriptionsPricePointsCommand(),
			SubscriptionsReviewScreenshotsCommand(),
			SubscriptionsSubmitCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGroupsCommand returns the subscriptions groups command group.
func SubscriptionsGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "groups",
		ShortUsage: "asc subscriptions groups <subcommand> [flags]",
		ShortHelp:  "Manage subscription groups.",
		LongHelp: `Manage subscription groups.

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions groups create --app "APP_ID" --reference-name "Premium"
  asc subscriptions groups get --id "GROUP_ID"
  asc subscriptions groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGroupsListCommand(),
			SubscriptionsGroupsCreateCommand(),
			SubscriptionsGroupsGetCommand(),
			SubscriptionsGroupsUpdateCommand(),
			SubscriptionsGroupsDeleteCommand(),
			SubscriptionsGroupsLocalizationsCommand(),
			SubscriptionsGroupsSubmitCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGroupsListCommand returns the groups list subcommand.
func SubscriptionsGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions groups list [flags]",
		ShortHelp:  "List subscription groups for an app.",
		LongHelp: `List subscription groups for an app.

Examples:
  asc subscriptions groups list --app "APP_ID"
  asc subscriptions groups list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions groups list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions groups list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionGroupsOption{
				asc.WithSubscriptionGroupsLimit(*limit),
				asc.WithSubscriptionGroupsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionGroupsLimit(200))
				firstPage, err := client.GetSubscriptionGroups(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions groups list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionGroups(ctx, resolvedAppID, asc.WithSubscriptionGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions groups list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionGroups(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions groups list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsCreateCommand returns the groups create subcommand.
func SubscriptionsGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions groups create [flags]",
		ShortHelp:  "Create a subscription group.",
		LongHelp: `Create a subscription group.

Examples:
  asc subscriptions groups create --app "APP_ID" --reference-name "Premium"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupCreateAttributes{
				ReferenceName: name,
			}

			resp, err := client.CreateSubscriptionGroup(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsGetCommand returns the groups get subcommand.
func SubscriptionsGroupsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups get", flag.ExitOnError)

	groupID := fs.String("id", "", "Subscription group ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions groups get --id \"GROUP_ID\"",
		ShortHelp:  "Get a subscription group by ID.",
		LongHelp: `Get a subscription group by ID.

Examples:
  asc subscriptions groups get --id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionGroup(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions groups get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsUpdateCommand returns the groups update subcommand.
func SubscriptionsGroupsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups update", flag.ExitOnError)

	groupID := fs.String("id", "", "Subscription group ID")
	referenceName := fs.String("reference-name", "", "Reference name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions groups update [flags]",
		ShortHelp:  "Update a subscription group.",
		LongHelp: `Update a subscription group.

Examples:
  asc subscriptions groups update --id "GROUP_ID" --reference-name "Premium"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGroupUpdateAttributes{
				ReferenceName: &name,
			}

			resp, err := client.UpdateSubscriptionGroup(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions groups update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsDeleteCommand returns the groups delete subcommand.
func SubscriptionsGroupsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups delete", flag.ExitOnError)

	groupID := fs.String("id", "", "Subscription group ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:  "Delete a subscription group.",
		LongHelp: `Delete a subscription group.

Examples:
  asc subscriptions groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
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
				return fmt.Errorf("subscriptions groups delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionGroup(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions groups delete: failed to delete: %w", err)
			}

			result := &asc.SubscriptionGroupDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

// SubscriptionsListCommand returns the subscriptions list subcommand.
func SubscriptionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group", "", "Subscription group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions list [flags]",
		ShortHelp:  "List subscriptions in a group.",
		LongHelp: `List subscriptions in a group.

Examples:
  asc subscriptions list --group "GROUP_ID"
  asc subscriptions list --group "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionsOption{
				asc.WithSubscriptionsLimit(*limit),
				asc.WithSubscriptionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionsLimit(200))
				firstPage, err := client.GetSubscriptions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptions(ctx, id, asc.WithSubscriptionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsCreateCommand returns the subscriptions create subcommand.
func SubscriptionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	groupID := fs.String("group", "", "Subscription group ID")
	refName := fs.String("ref-name", "", "Reference name")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.sub)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions create [flags]",
		ShortHelp:  "Create a subscription.",
		LongHelp: `Create a subscription.

Examples:
  asc subscriptions create --group "GROUP_ID" --ref-name "Monthly" --product-id "com.example.sub.monthly"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*refName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --ref-name is required")
				return flag.ErrHelp
			}

			product := strings.TrimSpace(*productID)
			if product == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionCreateAttributes{
				Name:      name,
				ProductID: product,
			}

			resp, err := client.CreateSubscription(requestCtx, group, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGetCommand returns the subscriptions get subcommand.
func SubscriptionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions get --id \"SUB_ID\"",
		ShortHelp:  "Get a subscription by ID.",
		LongHelp: `Get a subscription by ID.

Examples:
  asc subscriptions get --id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsUpdateCommand returns the subscriptions update subcommand.
func SubscriptionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	refName := fs.String("ref-name", "", "Reference name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions update [flags]",
		ShortHelp:  "Update a subscription.",
		LongHelp: `Update a subscription.

Examples:
  asc subscriptions update --id "SUB_ID" --ref-name "New Name"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*refName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionUpdateAttributes{
				Name: &name,
			}

			resp, err := client.UpdateSubscription(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsDeleteCommand returns the subscriptions delete subcommand.
func SubscriptionsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions delete --id \"SUB_ID\" --confirm",
		ShortHelp:  "Delete a subscription.",
		LongHelp: `Delete a subscription.

Examples:
  asc subscriptions delete --id "SUB_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
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
				return fmt.Errorf("subscriptions delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscription(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions delete: failed to delete: %w", err)
			}

			result := &asc.SubscriptionDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

// SubscriptionsPricesCommand returns the subscriptions prices command group.
func SubscriptionsPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions prices <subcommand> [flags]",
		ShortHelp:  "Manage subscription pricing.",
		LongHelp: `Manage subscription pricing.

Examples:
  asc subscriptions prices add --id "SUB_ID" --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricesAddCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricesAddCommand returns the subscriptions prices add subcommand.
func SubscriptionsPricesAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices add", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	pricePointID := fs.String("price-point", "", "Subscription price point ID")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD)")
	preserved := fs.Bool("preserved", false, "Preserve existing prices")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc subscriptions prices add [flags]",
		ShortHelp:  "Add a subscription price.",
		LongHelp: `Add a subscription price.

Examples:
  asc subscriptions prices add --id "SUB_ID" --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			pricePoint := strings.TrimSpace(*pricePointID)
			if pricePoint == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions prices add: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionPriceCreateAttributes{
				StartDate: strings.TrimSpace(*startDate),
			}
			if *preserved {
				attrs.Preserved = preserved
			}

			resp, err := client.CreateSubscriptionPrice(requestCtx, id, pricePoint, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions prices add: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsAvailabilityCommand returns the subscriptions availability command group.
func SubscriptionsAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc subscriptions availability <subcommand> [flags]",
		ShortHelp:  "Manage subscription availability.",
		LongHelp: `Manage subscription availability.

Examples:
  asc subscriptions availability set --id "SUB_ID" --territory "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsAvailabilitySetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsAvailabilitySetCommand returns the availability set subcommand.
func SubscriptionsAvailabilitySetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("availability set", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID")
	territories := fs.String("territory", "", "Territory IDs, comma-separated")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new territories automatically")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc subscriptions availability set [flags]",
		ShortHelp:  "Set subscription availability in territories.",
		LongHelp: `Set subscription availability in territories.

Examples:
  asc subscriptions availability set --id "SUB_ID" --territory "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			territoryIDs := parseCommaSeparatedIDs(*territories)
			if len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions availability set: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionAvailabilityAttributes{
				AvailableInNewTerritories: *availableInNew,
			}

			resp, err := client.CreateSubscriptionAvailability(requestCtx, id, territoryIDs, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions availability set: failed to set: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
