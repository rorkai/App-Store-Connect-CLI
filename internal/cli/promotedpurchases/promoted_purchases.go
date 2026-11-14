package promotedpurchases

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PromotedPurchasesCommand returns the promoted purchases command with subcommands.
func PromotedPurchasesCommand() *ffcli.Command {
	return NewPromotedPurchasesCommand(shared.DefaultDeps())
}

// NewPromotedPurchasesCommand returns the promoted purchases command with
// subcommands built on d.
func NewPromotedPurchasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("promoted-purchases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "promoted-purchases",
		ShortUsage: "asc promoted-purchases <subcommand> [flags]",
		ShortHelp:  "Manage promoted purchases for subscriptions and in-app purchases.",
		LongHelp: `Manage promoted purchases for subscriptions and in-app purchases.

Examples:
  asc promoted-purchases list --app "APP_ID"
  asc promoted-purchases view --promoted-purchase-id "PROMO_ID"
  asc promoted-purchases create --app "APP_ID" --product-id "PRODUCT_ID" --product-type SUBSCRIPTION --visible-for-all-users
  asc promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled false
  asc promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm
  asc promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPromotedPurchasesListCommand(d),
			newPromotedPurchasesGetCommand(d),
			newPromotedPurchasesCreateCommand(d),
			newPromotedPurchasesUpdateCommand(d),
			newPromotedPurchasesDeleteCommand(d),
			newPromotedPurchasesLinkCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPromotedPurchasesListCommand returns the promoted purchases list subcommand.
func newPromotedPurchasesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("promoted-purchases list: %v", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("promoted-purchases list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.PromotedPurchasesOption{
				asc.Limit[asc.PromotedPurchasesOption](*limit),
				asc.NextURL[asc.PromotedPurchasesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.PromotedPurchasesOption](200))
				firstPage, err := client.GetAppPromotedPurchases(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("promoted-purchases list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPromotedPurchases(ctx, resolvedAppID, asc.NextURL[asc.PromotedPurchasesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("promoted-purchases list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppPromotedPurchases(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("promoted-purchases list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPromotedPurchasesGetCommand returns the promoted purchases get subcommand.
func newPromotedPurchasesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc promoted-purchases view --promoted-purchase-id PROMO_ID",
		ShortHelp:   "View a promoted purchase by ID.",
		LongHelp: `View a promoted purchase by ID.

Examples:
  asc promoted-purchases view --promoted-purchase-id "PROMO_ID"`,
		IDFlag:             "promoted-purchase-id",
		IDUsage:            "Promoted purchase ID",
		IDType:             "promotedPurchases",
		RequiredDiagnostic: true,
		ErrorPrefix:        "promoted-purchases view",
		FetchErrorPrefix:   "promoted-purchases view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetPromotedPurchase(ctx, id)
		},
	})
}

// newPromotedPurchasesCreateCommand returns the promoted purchases create subcommand.
func newPromotedPurchasesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	productID := fs.String("product-id", "", "Product ID (subscription or in-app purchase ID)")
	productType := fs.String("product-type", "", "Product type: SUBSCRIPTION or IN_APP_PURCHASE")
	var visibleForAllUsers shared.OptionalBool
	fs.Var(&visibleForAllUsers, "visible-for-all-users", "Visible for all users: true or false")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the promoted purchase")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			productIDValue := strings.TrimSpace(*productID)
			if productIDValue == "" {
				return d.RequiredFlag("--product-id")
			}

			if strings.TrimSpace(*productType) == "" {
				return d.RequiredFlag("--product-type")
			}

			productTypeValue, err := normalizePromotedPurchaseProductType(*productType)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err)
				return flag.ErrHelp
			}

			if !visibleForAllUsers.IsSet() {
				return d.RequiredFlag("--visible-for-all-users")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("promoted-purchases create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPromotedPurchasesUpdateCommand returns the promoted purchases update subcommand.
func newPromotedPurchasesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "promoted-purchase-id", "promotedPurchases", "Promoted purchase ID")
	var visibleForAllUsers shared.OptionalBool
	fs.Var(&visibleForAllUsers, "visible-for-all-users", "Visible for all users: true or false")
	var enabled shared.OptionalBool
	fs.Var(&enabled, "enabled", "Enable or disable the promoted purchase")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--promoted-purchase-id")
			}
			if !visibleForAllUsers.IsSet() && !enabled.IsSet() {
				fmt.Fprintln(d.IO.ErrOut, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("promoted-purchases update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPromotedPurchasesDeleteCommand returns the promoted purchases delete subcommand.
func newPromotedPurchasesDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "promoted-purchase-id", "promotedPurchases", "Promoted purchase ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--confirm")
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--promoted-purchase-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("promoted-purchases delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeletePromotedPurchase(requestCtx, idValue); err != nil {
				return fmt.Errorf("promoted-purchases delete: failed to delete: %w", err)
			}

			result := &asc.PromotedPurchaseDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newPromotedPurchasesLinkCommand returns the promoted purchases link subcommand.
func newPromotedPurchasesLinkCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("link", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	promotedIDs := shared.BindOnceCSVFlag(fs, "promoted-purchase-id", "Comma-separated promoted purchase IDs")
	clear := fs.Bool("clear", false, "Remove all promoted purchases from the app")
	confirm := fs.Bool("confirm", false, "Confirm removal when using --clear")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			var promotedPurchaseIDs []string
			if *clear {
				if strings.TrimSpace(promotedIDs.String()) != "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --clear cannot be used with --promoted-purchase-id")
					return flag.ErrHelp
				}
				if !*confirm {
					fmt.Fprintln(d.IO.ErrOut, "Error: --confirm is required with --clear")
					return shared.MissingRequiredUsageError("--confirm")
				}
				promotedPurchaseIDs = nil
			} else {
				promotedPurchaseIDs = shared.SplitCSV(promotedIDs.String())
				if len(promotedPurchaseIDs) == 0 {
					return d.RequiredFlag("--promoted-purchase-id")
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("promoted-purchases link: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
