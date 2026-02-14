package promotedpurchases

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

// PromotedPurchasesCommand returns the promoted purchases command with subcommands.
func PromotedPurchasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promoted-purchases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "promoted-purchases",
		ShortUsage: "asc promoted-purchases <subcommand> [flags]",
		ShortHelp:  "Manage promoted purchases for subscriptions and in-app purchases.",
		LongHelp: `Manage promoted purchases for subscriptions and in-app purchases.

Examples:
  asc promoted-purchases list --app "APP_ID"
  asc promoted-purchases get --promoted-purchase-id "PROMO_ID"
  asc promoted-purchases create --app "APP_ID" --product-id "PRODUCT_ID" --product-type SUBSCRIPTION --visible-for-all-users
  asc promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled false
  asc promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm
  asc promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PromotedPurchasesListCommand(),
			PromotedPurchasesGetCommand(),
			PromotedPurchasesCreateCommand(),
			PromotedPurchasesUpdateCommand(),
			PromotedPurchasesDeleteCommand(),
			PromotedPurchasesLinkCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PromotedPurchasesListCommand returns the promoted purchases list subcommand.
func PromotedPurchasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc promoted-purchases list --app APP_ID [flags]",
		ShortHelp:  "List promoted purchases for an app.",
		LongHelp: `List promoted purchases for an app.

Examples:
  asc promoted-purchases list --app "APP_ID"
  asc promoted-purchases list --app "APP_ID" --limit 10
  asc promoted-purchases list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("promoted-purchases list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("promoted-purchases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PromotedPurchasesOption{
				asc.WithPromotedPurchasesLimit(*limit),
				asc.WithPromotedPurchasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithPromotedPurchasesLimit(200))
				firstPage, err := client.GetAppPromotedPurchases(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("promoted-purchases list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPromotedPurchases(ctx, resolvedAppID, asc.WithPromotedPurchasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("promoted-purchases list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppPromotedPurchases(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("promoted-purchases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PromotedPurchasesGetCommand returns the promoted purchases get subcommand.
func PromotedPurchasesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("promoted-purchase-id", "", "Promoted purchase ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc promoted-purchases get --promoted-purchase-id PROMO_ID",
		ShortHelp:  "Get a promoted purchase by ID.",
		LongHelp: `Get a promoted purchase by ID.

Examples:
  asc promoted-purchases get --promoted-purchase-id "PROMO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --promoted-purchase-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("promoted-purchases get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetPromotedPurchase(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("promoted-purchases get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PromotedPurchasesCreateCommand returns the promoted purchases create subcommand.
func PromotedPurchasesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	productID := fs.String("product-id", "", "Product ID (subscription or in-app purchase ID)")
	productType := fs.String("product-type", "", "Product type: SUBSCRIPTION or IN_APP_PURCHASE")
	var visibleForAllUsers shared.OptionalBool
	fs.Var(&visibleForAllUsers, "visible-for-all-users", "Visible for all users: true or false")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the promoted purchase")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc promoted-purchases create --app APP_ID --product-id PRODUCT_ID --product-type SUBSCRIPTION --visible-for-all-users",
		ShortHelp:  "Create a promoted purchase.",
		LongHelp: `Create a promoted purchase for a subscription or in-app purchase.

Examples:
  asc promoted-purchases create --app "APP_ID" --product-id "PRODUCT_ID" --product-type SUBSCRIPTION --visible-for-all-users
  asc promoted-purchases create --app "APP_ID" --product-id "PRODUCT_ID" --product-type IN_APP_PURCHASE --visible-for-all-users --enabled true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			productIDValue := strings.TrimSpace(*productID)
			if productIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*productType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-type is required")
				return flag.ErrHelp
			}

			productTypeValue, err := normalizePromotedPurchaseProductType(*productType)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			if !visibleForAllUsers.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --visible-for-all-users is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("promoted-purchases create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.PromotedPurchaseCreateAttributes{
				VisibleForAllUsers: visibleForAllUsers.Value(),
			}
			if enabled.IsSet() {
				enabledValue := enabled.Value()
				attrs.Enabled = &enabledValue
			}

			relationships := asc.PromotedPurchaseCreateRelationships{
				App: asc.Relationship{
					Data: asc.ResourceData{
						Type: asc.ResourceTypeApps,
						ID:   resolvedAppID,
					},
				},
			}
			switch productTypeValue {
			case promotedPurchaseProductTypeSubscription:
				relationships.Subscription = &asc.Relationship{
					Data: asc.ResourceData{
						Type: asc.ResourceTypeSubscriptions,
						ID:   productIDValue,
					},
				}
			case promotedPurchaseProductTypeInAppPurchase:
				relationships.InAppPurchaseV2 = &asc.Relationship{
					Data: asc.ResourceData{
						Type: asc.ResourceTypeInAppPurchases,
						ID:   productIDValue,
					},
				}
			default:
				return fmt.Errorf("promoted-purchases create: unsupported product type")
			}

			resp, err := client.CreatePromotedPurchase(requestCtx, attrs, relationships)
			if err != nil {
				return fmt.Errorf("promoted-purchases create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PromotedPurchasesUpdateCommand returns the promoted purchases update subcommand.
func PromotedPurchasesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("promoted-purchase-id", "", "Promoted purchase ID")
	var visibleForAllUsers shared.OptionalBool
	fs.Var(&visibleForAllUsers, "visible-for-all-users", "Visible for all users: true or false")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the promoted purchase")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc promoted-purchases update --promoted-purchase-id PROMO_ID [--visible-for-all-users true|false] [--enabled true|false]",
		ShortHelp:  "Update a promoted purchase.",
		LongHelp: `Update a promoted purchase.

Examples:
  asc promoted-purchases update --promoted-purchase-id "PROMO_ID" --visible-for-all-users false
  asc promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --promoted-purchase-id is required")
				return flag.ErrHelp
			}
			if !visibleForAllUsers.IsSet() && !enabled.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("promoted-purchases update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.PromotedPurchaseUpdateAttributes{}
			if visibleForAllUsers.IsSet() {
				value := visibleForAllUsers.Value()
				attrs.VisibleForAllUsers = &value
			}
			if enabled.IsSet() {
				value := enabled.Value()
				attrs.Enabled = &value
			}

			resp, err := client.UpdatePromotedPurchase(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("promoted-purchases update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PromotedPurchasesDeleteCommand returns the promoted purchases delete subcommand.
func PromotedPurchasesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("promoted-purchase-id", "", "Promoted purchase ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc promoted-purchases delete --promoted-purchase-id PROMO_ID --confirm",
		ShortHelp:  "Delete a promoted purchase.",
		LongHelp: `Delete a promoted purchase by ID.

Examples:
  asc promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --promoted-purchase-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("promoted-purchases delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeletePromotedPurchase(requestCtx, idValue); err != nil {
				return fmt.Errorf("promoted-purchases delete: failed to delete: %w", err)
			}

			result := &asc.PromotedPurchaseDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// PromotedPurchasesLinkCommand returns the promoted purchases link subcommand.
func PromotedPurchasesLinkCommand() *ffcli.Command {
	fs := flag.NewFlagSet("link", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	promotedIDs := fs.String("promoted-purchase-id", "", "Comma-separated promoted purchase IDs")
	clear := fs.Bool("clear", false, "Remove all promoted purchases from the app")
	confirm := fs.Bool("confirm", false, "Confirm removal when using --clear")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "link",
		ShortUsage: "asc promoted-purchases link --app APP_ID --promoted-purchase-id PROMO_ID[,PROMO_ID...]",
		ShortHelp:  "Link or clear promoted purchases for an app.",
		LongHelp: `Link promoted purchases to an app.

Examples:
  asc promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"
  asc promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_1,PROMO_2"
  asc promoted-purchases link --app "APP_ID" --clear --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			var promotedPurchaseIDs []string
			if *clear {
				if strings.TrimSpace(*promotedIDs) != "" {
					fmt.Fprintln(os.Stderr, "Error: --clear cannot be used with --promoted-purchase-id")
					return flag.ErrHelp
				}
				if !*confirm {
					fmt.Fprintln(os.Stderr, "Error: --confirm is required with --clear")
					return flag.ErrHelp
				}
				promotedPurchaseIDs = nil
			} else {
				promotedPurchaseIDs = shared.SplitCSV(*promotedIDs)
				if len(promotedPurchaseIDs) == 0 {
					fmt.Fprintln(os.Stderr, "Error: --promoted-purchase-id is required")
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("promoted-purchases link: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.SetAppPromotedPurchases(requestCtx, resolvedAppID, promotedPurchaseIDs); err != nil {
				return fmt.Errorf("promoted-purchases link: failed to link: %w", err)
			}

			action := "linked"
			if *clear {
				action = "cleared"
			}
			result := &asc.AppPromotedPurchasesLinkResult{
				AppID:               resolvedAppID,
				PromotedPurchaseIDs: promotedPurchaseIDs,
				Action:              action,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
