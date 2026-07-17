package subscriptions

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
)

// SubscriptionsPromotedPurchasesCommand returns the canonical nested promoted purchases tree.
func SubscriptionsPromotedPurchasesCommand() *ffcli.Command {
	cmd := promotedpurchases.PromotedPurchasesCommand()
	if cmd != nil {
		lookupAppID := addSubscriptionPromotedPurchaseLookupAppFlag(cmd)
		cmd.ShortUsage = "asc subscriptions promoted-purchases <subcommand> [flags]"
		promotedpurchases.ConfigureScopedPromotedPurchasesCommand(cmd, promotedpurchases.ScopedPromotedPurchasesCommandConfig{
			PathPrefix:         "asc subscriptions promoted-purchases",
			ProductType:        "SUBSCRIPTION",
			ProductSingular:    "a subscription",
			ProductPlural:      "subscriptions",
			OwnerIDFlag:        "subscription-id",
			OwnerIDUsage:       "Subscription ID, product ID, or exact current name",
			OwnerIDPlaceholder: "SUBSCRIPTION_SELECTOR",
			ResolveOwnerID: func(ctx context.Context, client *asc.Client, selector string) (string, error) {
				return resolveSubscriptionLookupIDWithTimeout(ctx, client, *lookupAppID, selector)
			},
			FetchForOwner: func(ctx context.Context, client *asc.Client, subscriptionID string, opts ...asc.PromotedPurchaseGetOption) (*asc.PromotedPurchaseResponse, error) {
				return client.GetSubscriptionPromotedPurchase(ctx, subscriptionID, opts...)
			},
			RootShortHelp: "Manage promoted purchases for subscriptions.",
			RootLongHelp: `Manage promoted purchases for subscriptions.

Only promoted purchases attached to subscriptions are listed or modified.
Link operations preserve any in-app purchase promoted purchases already
attached to the app.

Examples:
  asc subscriptions promoted-purchases list --app "APP_ID"
  asc subscriptions promoted-purchases view --promoted-purchase-id "PROMO_ID"
  asc subscriptions promoted-purchases view --app "APP_ID" --subscription-id "SUBSCRIPTION_SELECTOR"
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true
  asc subscriptions promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled false
  asc subscriptions promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm
  asc subscriptions promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"`,
		})
		configureSubscriptionsPromotedPurchasesCreate(cmd)
	}
	return cmd
}

func addSubscriptionPromotedPurchaseLookupAppFlag(cmd *ffcli.Command) *string {
	for _, subcommand := range cmd.Subcommands {
		if subcommand != nil && subcommand.Name == "view" && subcommand.FlagSet != nil {
			return addSubscriptionLookupAppFlag(subcommand.FlagSet)
		}
	}
	appID := ""
	return &appID
}

func configureSubscriptionsPromotedPurchasesCreate(cmd *ffcli.Command) {
	promotedpurchases.ConfigureFixedProductTypeCreateCommand(cmd, promotedpurchases.FixedProductTypeCreateConfig{
		ShortUsage: "asc subscriptions promoted-purchases create --app APP_ID --product-id PRODUCT_ID --visible-for-all-users",
		ShortHelp:  "Create a promoted purchase for a subscription.",
		LongHelp: `Create a promoted purchase for a subscription.

Examples:
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true --enabled true`,
		ProductType:    "SUBSCRIPTION",
		ProductIDUsage: "Subscription ID",
	})
}
