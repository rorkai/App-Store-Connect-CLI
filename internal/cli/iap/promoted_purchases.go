package iap

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
)

// IAPPromotedPurchasesCommand returns the canonical nested promoted purchases tree.
func IAPPromotedPurchasesCommand() *ffcli.Command {
	cmd := promotedpurchases.PromotedPurchasesCommand()
	if cmd != nil {
		lookupAppID := addIAPPromotedPurchaseLookupAppFlag(cmd)
		cmd.ShortUsage = "asc iap promoted-purchases <subcommand> [flags]"
		promotedpurchases.ConfigureScopedPromotedPurchasesCommand(cmd, promotedpurchases.ScopedPromotedPurchasesCommandConfig{
			PathPrefix:         "asc iap promoted-purchases",
			ProductType:        "IN_APP_PURCHASE",
			ProductSingular:    "an in-app purchase",
			ProductPlural:      "in-app purchases",
			OwnerIDFlag:        "iap-id",
			OwnerIDUsage:       "In-app purchase ID, product ID, or exact current name",
			OwnerIDPlaceholder: "IAP_SELECTOR",
			ResolveOwnerID: func(ctx context.Context, client *asc.Client, selector string) (string, error) {
				return resolveIAPLookupIDWithTimeout(ctx, client, *lookupAppID, selector)
			},
			FetchForOwner: func(ctx context.Context, client *asc.Client, iapID string, opts ...asc.PromotedPurchaseGetOption) (*asc.PromotedPurchaseResponse, error) {
				return client.GetInAppPurchasePromotedPurchase(ctx, iapID, opts...)
			},
			RootShortHelp: "Manage promoted purchases for in-app purchases.",
			RootLongHelp: `Manage promoted purchases for in-app purchases.

Only promoted purchases attached to in-app purchases are listed or modified.
Link operations preserve any subscription promoted purchases already attached
to the app.

Examples:
  asc iap promoted-purchases list --app "APP_ID"
  asc iap promoted-purchases view --promoted-purchase-id "PROMO_ID"
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true
  asc iap promoted-purchases update --promoted-purchase-id "PROMO_ID" --enabled false
  asc iap promoted-purchases delete --promoted-purchase-id "PROMO_ID" --confirm
  asc iap promoted-purchases link --app "APP_ID" --promoted-purchase-id "PROMO_ID"`,
		})
		configureIAPPromotedPurchasesCreate(cmd)
	}
	return cmd
}

func addIAPPromotedPurchaseLookupAppFlag(cmd *ffcli.Command) *string {
	for _, subcommand := range cmd.Subcommands {
		if subcommand != nil && subcommand.Name == "view" && subcommand.FlagSet != nil {
			return addIAPLookupAppFlag(subcommand.FlagSet)
		}
	}
	appID := ""
	return &appID
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
