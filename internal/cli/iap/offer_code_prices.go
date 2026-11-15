package iap

import (
	"context"
	"net/url"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPOfferCodesPricesCommand returns the offer code prices subcommand.
func IAPOfferCodesPricesCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "offer-codes prices",
		Name:        "prices",
		ShortUsage:  "asc iap offer-codes prices --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:   "List prices for an offer code.",
		LongHelp: `List prices for an offer code.

Examples:
  asc iap offer-codes prices --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes prices --offer-code-id "OFFER_CODE_ID" --paginate`,
		ParentFlag:           "offer-code-id",
		ParentUsage:          "Offer code ID",
		ParentType:           "inAppPurchaseOfferCodes",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			nextURL, err := shared.MergeNextURLQuery(next, offerCodePricesRelationshipQuery())
			if err != nil {
				return nil, err
			}
			return client.GetInAppPurchaseOfferCodePrices(ctx, id,
				asc.Limit[asc.IAPOfferCodePricesOption](limit),
				asc.NextURL[asc.IAPOfferCodePricesOption](nextURL),
				asc.WithIAPOfferCodePricesFields([]string{"territory", "pricePoint"}),
				asc.WithIAPOfferCodePricesInclude([]string{"territory", "pricePoint"}),
			)
		},
	})
}

func offerCodePricesRelationshipQuery() url.Values {
	return url.Values{
		"fields[inAppPurchaseOfferPrices]": []string{"territory,pricePoint"},
		"include":                          []string{"territory,pricePoint"},
	}
}
