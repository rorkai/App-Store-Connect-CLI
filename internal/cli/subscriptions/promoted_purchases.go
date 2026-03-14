package subscriptions

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsPromotedPurchasesCommand returns the canonical nested promoted purchases tree.
func SubscriptionsPromotedPurchasesCommand() *ffcli.Command {
	cmd := shared.RewriteCommandTreePath(
		promotedpurchases.PromotedPurchasesCommand(),
		"asc promoted-purchases",
		"asc subscriptions promoted-purchases",
	)
	if cmd != nil {
		promotedpurchases.ConfigureScopedPromotedPurchasesCommand(cmd, promotedpurchases.ScopedPromotedPurchasesCommandConfig{
			PathPrefix:      "asc subscriptions promoted-purchases",
			ProductType:     "SUBSCRIPTION",
			ProductSingular: "a subscription",
			ProductPlural:   "subscriptions",
			RootShortHelp:   "Manage promoted purchases for subscriptions.",
			RootLongHelp: `Manage promoted purchases for subscriptions.

Only promoted purchases attached to subscriptions are listed or modified.
Link operations preserve any in-app purchase promoted purchases already
attached to the app.

Examples:
  asc subscriptions promoted-purchases list --app "APP_ID"
  asc subscriptions promoted-purchases get --promoted-purchase-id "PROMO_ID"
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true
  asc subscriptions promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled false
  asc subscriptions promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm
  asc subscriptions promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"`,
		})
		configureSubscriptionsPromotedPurchasesCreate(cmd)
	}
	return cmd
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
