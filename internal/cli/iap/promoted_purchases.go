package iap

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPPromotedPurchasesCommand returns the canonical nested promoted purchases tree.
func IAPPromotedPurchasesCommand() *ffcli.Command {
	cmd := shared.RewriteCommandTreePath(
		promotedpurchases.PromotedPurchasesCommand(),
		"asc promoted-purchases",
		"asc iap promoted-purchases",
	)
	if cmd != nil {
		promotedpurchases.ConfigureScopedPromotedPurchasesCommand(cmd, promotedpurchases.ScopedPromotedPurchasesCommandConfig{
			PathPrefix:      "asc iap promoted-purchases",
			ProductType:     "IN_APP_PURCHASE",
			ProductSingular: "an in-app purchase",
			ProductPlural:   "in-app purchases",
			RootShortHelp:   "Manage promoted purchases for in-app purchases.",
			RootLongHelp: `Manage promoted purchases for in-app purchases.

Only promoted purchases attached to in-app purchases are listed or modified.
Link operations preserve any subscription promoted purchases already attached
to the app.

Examples:
  asc iap promoted-purchases list --app "APP_ID"
  asc iap promoted-purchases get --promoted-purchase-id "PROMO_ID"
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true
  asc iap promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled false
  asc iap promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm
  asc iap promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"`,
		})
		configureIAPPromotedPurchasesCreate(cmd)
	}
	return cmd
}

func configureIAPPromotedPurchasesCreate(cmd *ffcli.Command) {
	promotedpurchases.ConfigureFixedProductTypeCreateCommand(cmd, promotedpurchases.FixedProductTypeCreateConfig{
		ShortUsage: "asc iap promoted-purchases create --app APP_ID --product-id PRODUCT_ID --visible-for-all-users",
		ShortHelp:  "Create a promoted purchase for an in-app purchase.",
		LongHelp: `Create a promoted purchase for an in-app purchase.

Examples:
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true --enabled true`,
		ProductType:    "IN_APP_PURCHASE",
		ProductIDUsage: "In-app purchase ID",
	})
}
