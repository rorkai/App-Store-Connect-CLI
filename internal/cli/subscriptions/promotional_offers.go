package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSubscriptionsPromotionalOffersCommand returns the promotional offers command group.
func newSubscriptionsPromotionalOffersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("promotional-offers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "promotional-offers",
		ShortUsage: "asc subscriptions promotional-offers <subcommand> [flags]",
		ShortHelp:  "Manage subscription promotional offers.",
		LongHelp: `Manage subscription promotional offers.

Examples:
  asc subscriptions promotional-offers list --subscription-id "SUB_ID"
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "US"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsPromotionalOffersListCommand(d),
			newSubscriptionsPromotionalOffersGetCommand(d),
			newSubscriptionsPromotionalOffersCreateCommand(d),
			newSubscriptionsPromotionalOffersUpdateCommand(d),
			newSubscriptionsPromotionalOffersDeleteCommand(d),
			newSubscriptionsPromotionalOfferPricesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsPromotionalOffersListCommand returns the promotional offers list subcommand.
func newSubscriptionsPromotionalOffersListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("promotional-offers list", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions promotional-offers list [flags]",
		ShortHelp:  "List promotional offers for a subscription.",
		LongHelp: `List promotional offers for a subscription.

Examples:
  asc subscriptions promotional-offers list --subscription-id "SUB_ID"
  asc subscriptions promotional-offers list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "subscriptions promotional-offers list", 200, "limit", "subscription-fields"); err != nil {
				return err
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--subscription-id")
			}

			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(d, fs, *next, "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
				if err != nil {
					return err
				}
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionPromotionalOffersOption{
				asc.Limit[asc.SubscriptionPromotionalOffersOption](*limit),
				asc.NextURL[asc.SubscriptionPromotionalOffersOption](*next),
				asc.WithSubscriptionPromotionalOffersSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionPromotionalOffersInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			}

			if *paginate {
				paginateOpts := opts
				if strings.TrimSpace(*next) == "" {
					paginateOpts = append(paginateOpts, asc.Limit[asc.SubscriptionPromotionalOffersOption](200))
				}
				firstPage, err := client.GetSubscriptionPromotionalOffers(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions promotional-offers list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionPromotionalOffers(ctx, id, asc.NextURL[asc.SubscriptionPromotionalOffersOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions promotional-offers list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionPromotionalOffers(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsPromotionalOffersGetCommand returns the promotional offers get subcommand.
func newSubscriptionsPromotionalOffersGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("promotional-offers view", flag.ExitOnError)

	offerID := shared.BindResourceIDFlag(fs, "id", "subscriptionPromotionalOffers", "Promotional offer ID")
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions promotional-offers view --id \"OFFER_ID\"",
		ShortHelp:  "View a promotional offer by ID.",
		LongHelp: `View a promotional offer by ID.

Examples:
  asc subscriptions promotional-offers view --id "OFFER_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(d, fs, "", "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*offerID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPromotionalOffer(
				requestCtx, id,
				asc.WithSubscriptionPromotionalOfferSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionPromotionalOfferInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsPromotionalOffersCreateCommand returns the promotional offers create subcommand.
func newSubscriptionsPromotionalOffersCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("promotional-offers create", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	offerCode := fs.String("offer-code", "", "Offer code")
	name := fs.String("name", "", "Offer name")
	offerDuration := fs.String("offer-duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	numberOfPeriods := fs.Int("number-of-periods", 0, "Number of periods (required)")
	prices := shared.BindOnceCSVFlag(fs, "prices", "Promotional offer prices (required): existing PRICE_ID entries, inline TERRITORY entries, or inline TERRITORY:PRICE_POINT_ID entries; territory accepts alpha-2, alpha-3, or exact English country name")
	targetPlanType := d.BindTargetSubscriptionPlanTypeFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions promotional-offers create [flags]",
		ShortHelp:  "Create a promotional offer.",
		LongHelp: `Create a promotional offer.

Examples:
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "US"
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode PAY_AS_YOU_GO --number-of-periods 1 --prices "US:PRICE_POINT_ID"
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode PAY_UP_FRONT --number-of-periods 1 --prices "EXISTING_PROMOTIONAL_OFFER_PRICE_ID"
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "US" --target-subscription-plan-type MONTHLY`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}

			offerCodeValue := strings.TrimSpace(*offerCode)
			if offerCodeValue == "" {
				return d.RequiredFlag("--offer-code")
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}

			duration, err := normalizeSubscriptionOfferDuration(*offerDuration)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			mode, err := normalizeSubscriptionOfferMode(*offerMode)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			if *numberOfPeriods <= 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --number-of-periods is required")
				return requiredPositiveIntegerUsageError(fs, "number-of-periods")
			}

			priceEntries, err := parseSubscriptionPromotionalOfferPrices(prices.String())
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			if len(priceEntries) == 0 {
				return d.RequiredFlag("--prices")
			}

			planType, err := targetPlanType()
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers create: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionPromotionalOfferCreateAttributes{
				Duration:                   duration,
				Name:                       nameValue,
				NumberOfPeriods:            *numberOfPeriods,
				OfferCode:                  offerCodeValue,
				OfferMode:                  mode,
				TargetSubscriptionPlanType: planType,
			}

			resp, err := client.CreateSubscriptionPromotionalOffer(requestCtx, id, attrs, priceEntries)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsPromotionalOffersUpdateCommand returns the promotional offers update subcommand.
func newSubscriptionsPromotionalOffersUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("promotional-offers update", flag.ExitOnError)

	offerID := shared.BindResourceIDFlag(fs, "id", "subscriptionPromotionalOffers", "Promotional offer ID")
	prices := shared.BindOnceCSVFlag(fs, "prices", "Promotional offer price ID(s), comma-separated")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions promotional-offers update [flags]",
		ShortHelp:  "Update a promotional offer's prices.",
		LongHelp: `Update a promotional offer's prices.

Examples:
  asc subscriptions promotional-offers update --id "OFFER_ID" --prices "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			priceIDs := shared.SplitCSV(prices.String())
			if len(priceIDs) == 0 {
				return d.RequiredFlag("--prices")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateSubscriptionPromotionalOffer(requestCtx, id, priceIDs)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsPromotionalOffersDeleteCommand returns the promotional offers delete subcommand.
func newSubscriptionsPromotionalOffersDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "promotional-offers delete",
		Name:        "delete",
		ShortUsage:  "asc subscriptions promotional-offers delete --id \"OFFER_ID\" --confirm",
		ShortHelp:   "Delete a promotional offer.",
		LongHelp: `Delete a promotional offer.

Examples:
  asc subscriptions promotional-offers delete --id "OFFER_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Promotional offer ID",
		IDType:             "subscriptionPromotionalOffers",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteSubscriptionPromotionalOffer(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}

// newSubscriptionsPromotionalOfferPricesCommand returns the promotional offer prices subcommand.
func newSubscriptionsPromotionalOfferPricesCommand(d *shared.Deps) *ffcli.Command {
	var selectedPricePointFields []string
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "promotional-offers prices",
		Name:        "prices",
		ShortUsage:  "asc subscriptions promotional-offers prices --id \"OFFER_ID\" [flags]",
		ShortHelp:   "List prices for a promotional offer.",
		LongHelp: `List prices for a promotional offer.

Examples:
  asc subscriptions promotional-offers prices --id "OFFER_ID"
  asc subscriptions promotional-offers prices --id "OFFER_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Promotional offer ID",
		ParentType:           "subscriptionPromotionalOffers",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"limit", "price-point-fields"},
		Flags: func(fs *flag.FlagSet) func() error {
			pricePointFields := fs.String("price-point-fields", "", "Included subscription price point fields (comma-separated)")
			return func() error {
				var err error
				selectedPricePointFields, err = normalizeSparseFieldsFlag(d, fs, fs.Lookup("next").Value.String(), "price-point-fields", *pricePointFields, subscriptionPricePointFieldsList())
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			if strings.TrimSpace(next) != "" {
				return client.GetSubscriptionPromotionalOfferPrices(ctx, id, asc.NextURL[asc.SubscriptionPromotionalOfferPricesOption](next))
			}
			return client.GetSubscriptionPromotionalOfferPrices(ctx, id,
				asc.Limit[asc.SubscriptionPromotionalOfferPricesOption](limit),
				asc.WithSubscriptionPromotionalOfferPricesPricePointFields(selectedPricePointFields),
				asc.WithSubscriptionPromotionalOfferPricesInclude(includeRelationshipForFields(selectedPricePointFields, "subscriptionPricePoint")),
			)
		},
	})
}
