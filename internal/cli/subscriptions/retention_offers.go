package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const retentionOffersPricesMaxLimit = 50

var retentionOfferIncludedResources = map[string]string{"subscription": "subscriptions", "prices": "retentionOfferPrices"}

var retentionOfferPriceIncludedResources = map[string]string{"territory": "territories", "subscriptionPricePoint": "subscriptionPricePoints"}

func retentionOfferFieldsList() []string {
	return []string{"referenceName", "offerId", "duration", "offerMode", "periodCount", "customerEligibilityPaidSubscriptionDurationInMonths", "customerEligibilityWaitBetweenOffersInMonths", "targetSubscriptionPlanType", "subscription", "prices"}
}

// newSubscriptionsRetentionOffersCommand returns the retention offers command group.
func newSubscriptionsRetentionOffersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("retention", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "retention",
		ShortUsage: "asc subscriptions offers retention <subcommand> [flags]",
		ShortHelp:  "Manage subscription retention offers.",
		LongHelp: `Manage subscription retention offers.

Retention offers are discounts shown with a subscription's retention message
when people start to cancel. Creating an offer does not present it; use
"presented add" to present it with the subscription's retention message.

Examples:
  asc subscriptions offers retention list --subscription-id "SUB_ID"
  asc subscriptions offers retention create --subscription-id "SUB_ID" --reference-name "winter" --offer-id "winter_one_month" --duration ONE_MONTH --offer-mode PAY_UP_FRONT --period-count 1 --eligibility-paid-months 3 --price "SUBSCRIPTION_PRICE_POINT_ID"
  asc subscriptions offers retention presented add --subscription-id "SUB_ID" --retention-offer-id "OFFER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			retentionOffersListCommand(d, "list", "List retention offers for a subscription.", (*asc.Client).GetSubscriptionRetentionOffers),
			retentionOffersLinksCommand(d, "links", "List retention offer relationships for a subscription.", (*asc.Client).GetSubscriptionRetentionOffersRelationships),
			newSubscriptionsRetentionOffersViewCommand(d),
			newSubscriptionsRetentionOffersCreateCommand(d),
			newSubscriptionsRetentionOffersUpdateCommand(d),
			newSubscriptionsRetentionOffersDeleteCommand(d),
			newSubscriptionsRetentionOfferPricesCommand(d),
			newSubscriptionsRetentionMessageOffersCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsRetentionMessageOffersCommand returns the group for the offers
// presented with a subscription's retention message.
func newSubscriptionsRetentionMessageOffersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("presented", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "presented",
		ShortUsage: "asc subscriptions offers retention presented <subcommand> [flags]",
		ShortHelp:  "Manage the retention offers presented with a subscription's retention message.",
		LongHelp: `Manage the retention offers presented with a subscription's retention message.

Creating a retention offer does not present it. Add an offer here to present
it with the subscription's retention message; remove it to stop presenting it.
Apple rejects "presented add" until a retention message is assigned to the
subscription (asc subscriptions retention-messages assign).

Examples:
  asc subscriptions offers retention presented list --subscription-id "SUB_ID"
  asc subscriptions offers retention presented add --subscription-id "SUB_ID" --retention-offer-id "OFFER_ID,OFFER_ID_2"
  asc subscriptions offers retention presented remove --subscription-id "SUB_ID" --retention-offer-id "OFFER_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			retentionOffersListCommand(d, "presented list", "List the retention offers presented with a subscription's retention message.", (*asc.Client).GetSubscriptionRetentionMessageOffers),
			retentionOffersLinksCommand(d, "presented links", "List the relationships of the retention offers presented with a subscription's retention message.", (*asc.Client).GetSubscriptionRetentionMessageOffersRelationships),
			retentionMessageOffersChangeCommand(d, "add"),
			retentionMessageOffersChangeCommand(d, "remove"),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

type retentionOffersFetch func(*asc.Client, context.Context, string, ...asc.RetentionOffersOption) (*asc.RetentionOffersResponse, error)

// retentionOffersListCommand builds "list" and "presented list", which share
// Apple's query parameters and response.
func retentionOffersListCommand(d *shared.Deps, path, shortHelp string, fetch retentionOffersFetch) *ffcli.Command {
	fs := d.NewFlagSet(path, flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	read := bindRetentionOfferReadFlags(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	command := "subscriptions offers retention " + path
	return &ffcli.Command{
		Name:       path[strings.LastIndex(path, " ")+1:],
		ShortUsage: "asc " + command + " --subscription-id SUB_ID [flags]",
		ShortHelp:  shortHelp,
		LongHelp: shortHelp + `

Examples:
  asc ` + command + ` --subscription-id "SUB_ID"
  asc ` + command + ` --subscription-id "SUB_ID" --include prices --price-fields subscriptionPricePoint --prices-limit 10
  asc ` + command + ` --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, command, 200, "fields", "include", "prices-limit"); err != nil {
				return err
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.UsageError("--subscription-id is required")
			}

			opts, err := read.options(d, fs, command, *next)
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			if strings.TrimSpace(*next) == "" {
				if id, err = resolveSubscriptionLookupIDWithTimeoutWith(ctx, d, client, *appID, id); err != nil {
					return err
				}
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts = append(opts, asc.WithRetentionOffersLimit(*limit), asc.WithRetentionOffersNextURL(*next))
			if *paginate {
				if strings.TrimSpace(*next) == "" && *limit == 0 {
					opts = append(opts, asc.WithRetentionOffersLimit(200))
				}
				firstPage, err := fetch(client, requestCtx, id, opts...)
				if err != nil {
					return fmt.Errorf("%s: failed to fetch: %w", command, err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return fetch(client, ctx, id, asc.WithRetentionOffersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("%s: %w", command, err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := fetch(client, requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

type retentionOfferLinksFetch func(*asc.Client, context.Context, string, ...asc.LinkagesOption) (*asc.LinkagesResponse, error)

func retentionOffersLinksCommand(d *shared.Deps, path, shortHelp string, fetch retentionOfferLinksFetch) *ffcli.Command {
	command := "subscriptions offers retention " + path
	cmd := shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: path,
		Name:        path[strings.LastIndex(path, " ")+1:],
		ShortUsage:  "asc " + command + " --subscription-id SUB_ID [flags]",
		ShortHelp:   shortHelp,
		LongHelp: shortHelp + `

Examples:
  asc ` + command + ` --subscription-id "SUB_ID"
  asc ` + command + ` --subscription-id "SUB_ID" --paginate`,
		ParentFlag:       "subscription-id",
		ParentUsage:      "Subscription ID",
		ParentType:       "subscriptions",
		FetchErrorPrefix: command,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return fetch(client, ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
	return cmd
}

type retentionOfferReadFlags struct {
	fields      *string
	include     *string
	pricesLimit *int
	included    *shared.IncludedFieldFlags
}

func bindRetentionOfferReadFlags(fs *flag.FlagSet) *retentionOfferReadFlags {
	return &retentionOfferReadFlags{
		fields:      fs.String("fields", "", "Fields to include: "+strings.Join(retentionOfferFieldsList(), ", ")),
		include:     fs.String("include", "", "Include related resources: prices, subscription"),
		pricesLimit: fs.Int("prices-limit", 0, fmt.Sprintf("Maximum included prices per offer (1-%d; requires --include prices)", retentionOffersPricesMaxLimit)),
		included:    shared.BindIncludedFieldFlags(fs, retentionOfferIncludedResources),
	}
}

func (f *retentionOfferReadFlags) options(d *shared.Deps, fs *flag.FlagSet, command, next string) ([]asc.RetentionOffersOption, error) {
	fields, err := normalizeSelectionFlagWith(d, fs, *f.fields, "--fields", retentionOfferFieldsList())
	if err != nil {
		return nil, err
	}
	include, err := normalizeSelectionFlagWith(d, fs, *f.include, "--include", []string{"prices", "subscription"})
	if err != nil {
		return nil, err
	}
	if *f.pricesLimit != 0 {
		if *f.pricesLimit < 1 || *f.pricesLimit > retentionOffersPricesMaxLimit {
			return nil, d.UsageErrorf("%s: --prices-limit must be between 1 and %d", command, retentionOffersPricesMaxLimit)
		}
		if !shared.HasInclude(include, "prices") {
			return nil, d.UsageErrorf("%s: --prices-limit requires --include prices", command)
		}
	}
	includedFields, err := f.included.Resolve(command, next, include)
	if err != nil {
		return nil, err
	}
	return []asc.RetentionOffersOption{
		asc.WithRetentionOffersFields(fields),
		asc.WithRetentionOffersInclude(include),
		asc.WithRetentionOffersPricesLimit(*f.pricesLimit),
		asc.WithIncludedFields[asc.RetentionOffersOption](includedFields),
	}, nil
}

// newSubscriptionsRetentionOffersViewCommand returns the retention offer view subcommand.
func newSubscriptionsRetentionOffersViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "retentionOffers", "Retention offer ID")
	read := bindRetentionOfferReadFlags(fs)
	output := d.BindOutputFlags(fs)

	const command = "subscriptions offers retention view"
	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc " + command + " --id OFFER_ID [flags]",
		ShortHelp:  "View a retention offer by ID.",
		LongHelp: `View a retention offer by ID.

Examples:
  asc subscriptions offers retention view --id "OFFER_ID"
  asc subscriptions offers retention view --id "OFFER_ID" --include prices,subscription --subscription-fields name,productId`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			opts, err := read.options(d, fs, command, "")
			if err != nil {
				return err
			}
			offerID := strings.TrimSpace(*id)
			if offerID == "" {
				return d.UsageError("--id is required")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetRetentionOffer(requestCtx, offerID, opts...)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsRetentionOffersCreateCommand returns the retention offer create subcommand.
func newSubscriptionsRetentionOffersCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	referenceName := fs.String("reference-name", "", "Reference name (not shown to customers)")
	offerID := fs.String("offer-id", "", "Offer identifier used by StoreKit")
	duration := fs.String("duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	var periodCount, paidMonths, waitMonths optionalInt
	fs.Var(&periodCount, "period-count", "Number of offer periods (required)")
	fs.Var(&paidMonths, "eligibility-paid-months", "Minimum months of paid subscription for eligibility: 1-24, 36, 48, or 60 (required)")
	fs.Var(&waitMonths, "eligibility-wait-months", "Months to wait before redeeming another retention offer: 2-24")
	targetPlanType := d.BindTargetSubscriptionPlanTypeFlag(fs)
	prices := shared.BindOnceCSVFlag(fs, "price", "Subscription price point ID(s), comma-separated, one per territory (required)")
	output := d.BindOutputFlags(fs)

	const command = "subscriptions offers retention create"
	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc " + command + " [flags]",
		ShortHelp:  "Create a retention offer.",
		LongHelp: `Create a retention offer.

Each --price is a subscription price point ID; the price point sets the
territory, so add one per territory. Find them with
"asc subscriptions pricing price-points list". Creating an offer does not
present it; use "asc subscriptions offers retention presented add".

Apple defaults the target plan type to UPFRONT. --target-subscription-plan-type
MONTHLY only works when the subscription offers the Monthly with 12-Month
Commitment plan in each price point's territory.

Examples:
  asc subscriptions offers retention create --subscription-id "SUB_ID" --reference-name "winter" --offer-id "winter_one_month" --duration ONE_MONTH --offer-mode PAY_UP_FRONT --period-count 1 --eligibility-paid-months 3 --eligibility-wait-months 6 --target-subscription-plan-type UPFRONT --price "SUBSCRIPTION_PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			for _, required := range []struct{ name, value string }{
				{"subscription-id", *subscriptionID},
				{"reference-name", *referenceName},
				{"offer-id", *offerID},
				{"duration", *duration},
				{"offer-mode", *offerMode},
				{"period-count", periodCount.String()},
				{"eligibility-paid-months", paidMonths.String()},
				{"price", prices.String()},
			} {
				if strings.TrimSpace(required.value) == "" {
					return d.UsageErrorf("--%s is required", required.name)
				}
			}
			durationValue, ok := subscriptionOfferDurationMap[strings.ToUpper(strings.TrimSpace(*duration))]
			if !ok {
				return d.UsageErrorf("%s: --duration must be one of: %s", command, strings.Join(subscriptionOfferDurationValues, ", "))
			}
			modeValue, ok := subscriptionOfferModeMap[strings.ToUpper(strings.TrimSpace(*offerMode))]
			if !ok {
				return d.UsageErrorf("%s: --offer-mode must be one of: %s", command, strings.Join(subscriptionOfferModeValues, ", "))
			}
			if periodCount.value < 1 {
				return d.UsageErrorf("%s: --period-count must be greater than 0", command)
			}
			if err := validateRetentionOfferEligibility(d, command, paidMonths, waitMonths); err != nil {
				return err
			}
			planType, err := targetPlanType()
			if err != nil {
				return err
			}
			pricePointIDs := shared.SplitUniqueCSV(prices.String())
			if len(pricePointIDs) == 0 {
				return d.UsageError("--price is required")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			subscription, err := resolveSubscriptionLookupIDWithTimeoutWith(ctx, d, client, *appID, strings.TrimSpace(*subscriptionID))
			if err != nil {
				return err
			}

			req := asc.RetentionOfferCreateRequest{
				Data: asc.RetentionOfferCreateData{
					Type: asc.ResourceTypeRetentionOffers,
					Attributes: asc.RetentionOfferCreateAttributes{
						ReferenceName: strings.TrimSpace(*referenceName),
						OfferID:       strings.TrimSpace(*offerID),
						Duration:      durationValue,
						OfferMode:     modeValue,
						PeriodCount:   periodCount.value,
						CustomerEligibilityPaidSubscriptionDurationInMonths: paidMonths.value,
						TargetSubscriptionPlanType:                          planType,
					},
					Relationships: asc.RetentionOfferCreateRelationships{
						Subscription: asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeSubscriptions, ID: subscription}},
					},
				},
			}
			if waitMonths.set {
				req.Data.Attributes.CustomerEligibilityWaitBetweenOffersInMonths = &waitMonths.value
			}
			// Prices don't exist before the offer, so each is created inline in
			// `included` under a `${price-N}` local ID.
			for i, pricePointID := range pricePointIDs {
				localID := fmt.Sprintf("${price-%d}", i+1)
				req.Data.Relationships.Prices.Data = append(req.Data.Relationships.Prices.Data, asc.ResourceData{Type: asc.ResourceTypeRetentionOfferPrices, ID: localID})
				req.Included = append(req.Included, asc.RetentionOfferPriceInlineCreate{
					Type: asc.ResourceTypeRetentionOfferPrices,
					ID:   localID,
					Relationships: asc.RetentionOfferPriceInlineRelationships{
						SubscriptionPricePoint: asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeSubscriptionPricePoints, ID: pricePointID}},
					},
				})
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateRetentionOffer(requestCtx, req)
			if err != nil {
				return fmt.Errorf("%s: failed to create: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newSubscriptionsRetentionOffersUpdateCommand returns the retention offer update subcommand.
func newSubscriptionsRetentionOffersUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "retentionOffers", "Retention offer ID")
	var paidMonths, waitMonths optionalInt
	fs.Var(&paidMonths, "eligibility-paid-months", "Minimum months of paid subscription for eligibility: 1-24, 36, 48, or 60")
	fs.Var(&waitMonths, "eligibility-wait-months", "Months to wait before redeeming another retention offer: 2-24")
	output := d.BindOutputFlags(fs)

	const command = "subscriptions offers retention update"
	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc " + command + " --id OFFER_ID [flags]",
		ShortHelp:  "Update a retention offer's eligibility.",
		LongHelp: `Update a retention offer's eligibility.

The eligibility attributes are the only attributes Apple lets you change.
Apple rejects clearing the wait between offers (null), so set a new value instead.

Examples:
  asc subscriptions offers retention update --id "OFFER_ID" --eligibility-paid-months 6
  asc subscriptions offers retention update --id "OFFER_ID" --eligibility-wait-months 4`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			offerID := strings.TrimSpace(*id)
			if offerID == "" {
				return d.UsageError("--id is required")
			}
			if !paidMonths.set && !waitMonths.set {
				return d.UpdateFlagRequired(fs, &paidMonths, &waitMonths)
			}
			if err := validateRetentionOfferEligibility(d, command, paidMonths, waitMonths); err != nil {
				return err
			}

			var attrs asc.RetentionOfferUpdateAttributes
			if paidMonths.set {
				attrs.CustomerEligibilityPaidSubscriptionDurationInMonths = &paidMonths.value
			}
			if waitMonths.set {
				attrs.CustomerEligibilityWaitBetweenOffersInMonths = &waitMonths.value
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateRetentionOffer(requestCtx, offerID, attrs)
			if err != nil {
				return fmt.Errorf("%s: failed to update: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// validateRetentionOfferEligibility enforces the eligibility values Apple's
// retention messaging guide documents.
func validateRetentionOfferEligibility(d *shared.Deps, command string, paidMonths, waitMonths optionalInt) error {
	if paidMonths.set && (paidMonths.value < 1 || paidMonths.value > 24) && !slices.Contains([]int{36, 48, 60}, paidMonths.value) {
		return d.UsageErrorf("%s: --eligibility-paid-months must be 1-24, 36, 48, or 60", command)
	}
	if waitMonths.set && (waitMonths.value < 2 || waitMonths.value > 24) {
		return d.UsageErrorf("%s: --eligibility-wait-months must be between 2 and 24", command)
	}
	return nil
}

// newSubscriptionsRetentionOffersDeleteCommand returns the retention offer delete subcommand.
func newSubscriptionsRetentionOffersDeleteCommand(d *shared.Deps) *ffcli.Command {
	const command = "subscriptions offers retention delete"
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc " + command + " --id OFFER_ID --confirm",
		ShortHelp:  "Delete a retention offer.",
		LongHelp: `Delete a retention offer.

Examples:
  asc subscriptions offers retention delete --id "OFFER_ID" --confirm`,
		IDFlag:  "id",
		IDUsage: "Retention offer ID",
		IDType:  "retentionOffers",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteRetentionOffer(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}

// newSubscriptionsRetentionOfferPricesCommand returns the retention offer prices group.
func newSubscriptionsRetentionOfferPricesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("prices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions offers retention prices <subcommand> [flags]",
		ShortHelp:  "List retention offer prices.",
		LongHelp: `List retention offer prices.

Examples:
  asc subscriptions offers retention prices list --id "OFFER_ID"
  asc subscriptions offers retention prices links --id "OFFER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsRetentionOfferPricesListCommand(d),
			shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
				FlagSetName: "prices links",
				Name:        "links",
				ShortUsage:  "asc subscriptions offers retention prices links --id OFFER_ID [flags]",
				ShortHelp:   "List price relationships for a retention offer.",
				LongHelp: `List price relationships for a retention offer.

Examples:
  asc subscriptions offers retention prices links --id "OFFER_ID"
  asc subscriptions offers retention prices links --id "OFFER_ID" --paginate`,
				ParentFlag:       "id",
				ParentUsage:      "Retention offer ID",
				ParentType:       "retentionOffers",
				FetchErrorPrefix: "subscriptions offers retention prices links",
				FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
					return client.GetRetentionOfferPricesRelationships(ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
				},
			}),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsRetentionOfferPricesListCommand returns the retention offer prices list subcommand.
func newSubscriptionsRetentionOfferPricesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("prices list", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "retentionOffers", "Retention offer ID")
	territories := fs.String("territory", "", "Filter by territories, comma-separated (alpha-2, alpha-3, or exact English country names)")
	fields := fs.String("fields", "", "Fields to include: territory, subscriptionPricePoint")
	include := fs.String("include", "", "Include related resources: subscriptionPricePoint, territory")
	included := shared.BindIncludedFieldFlags(fs, retentionOfferPriceIncludedResources)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	const command = "subscriptions offers retention prices list"
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc " + command + " --id OFFER_ID [flags]",
		ShortHelp:  "List prices for a retention offer.",
		LongHelp: `List prices for a retention offer.

Examples:
  asc subscriptions offers retention prices list --id "OFFER_ID"
  asc subscriptions offers retention prices list --id "OFFER_ID" --territory "US,France" --include subscriptionPricePoint --price-point-fields customerPrice,proceeds`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, command, 200, "territory", "fields", "include"); err != nil {
				return err
			}

			offerID := strings.TrimSpace(*id)
			if offerID == "" && strings.TrimSpace(*next) == "" {
				return d.UsageError("--id is required")
			}

			fieldsValue, err := normalizeSelectionFlagWith(d, fs, *fields, "--fields", []string{"territory", "subscriptionPricePoint"})
			if err != nil {
				return err
			}
			includeValue, err := normalizeSelectionFlagWith(d, fs, *include, "--include", []string{"subscriptionPricePoint", "territory"})
			if err != nil {
				return err
			}
			includedFields, err := included.Resolve(command, *next, includeValue)
			if err != nil {
				return err
			}
			territoryFilter, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return d.UsageErrorf("%s: %v", command, err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.RetentionOfferPricesOption{
				asc.WithRetentionOfferPricesLimit(*limit),
				asc.WithRetentionOfferPricesNextURL(*next),
				asc.WithRetentionOfferPricesTerritoryFilter(territoryFilter),
				asc.WithRetentionOfferPricesFields(fieldsValue),
				asc.WithRetentionOfferPricesInclude(includeValue),
				asc.WithIncludedFields[asc.RetentionOfferPricesOption](includedFields),
			}
			if *paginate {
				if strings.TrimSpace(*next) == "" && *limit == 0 {
					opts = append(opts, asc.WithRetentionOfferPricesLimit(200))
				}
				firstPage, err := client.GetRetentionOfferPrices(requestCtx, offerID, opts...)
				if err != nil {
					return fmt.Errorf("%s: failed to fetch: %w", command, err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetRetentionOfferPrices(ctx, offerID, asc.WithRetentionOfferPricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("%s: %w", command, err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetRetentionOfferPrices(requestCtx, offerID, opts...)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// retentionMessageOffersChangeCommand builds "presented add" and "presented remove".
func retentionMessageOffersChangeCommand(d *shared.Deps, action string) *ffcli.Command {
	fs := d.NewFlagSet("presented "+action, flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	offerIDs := shared.BindOnceCSVFlag(fs, "retention-offer-id", "Retention offer ID(s), comma-separated (required)")
	var confirm *bool
	output := d.BindOutputFlags(fs)

	command := "subscriptions offers retention presented " + action
	shortHelp := "Present retention offers with a subscription's retention message."
	usage := "asc " + command + " --subscription-id SUB_ID --retention-offer-id OFFER_ID[,OFFER_ID]"
	example := usage
	resultAction := "added"
	change := (*asc.Client).AddSubscriptionRetentionMessageOffers
	if action == "remove" {
		confirm = fs.Bool("confirm", false, "Confirm removal")
		shortHelp = "Stop presenting retention offers with a subscription's retention message."
		usage += " --confirm"
		example = usage
		resultAction = "removed"
		change = (*asc.Client).RemoveSubscriptionRetentionMessageOffers
	}
	example = strings.NewReplacer("SUB_ID", `"SUB_ID"`, "OFFER_ID[,OFFER_ID]", `"OFFER_ID,OFFER_ID_2"`).Replace(example)

	return &ffcli.Command{
		Name:       action,
		ShortUsage: usage,
		ShortHelp:  shortHelp,
		LongHelp: shortHelp + `

Examples:
  ` + example,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.UsageError("--subscription-id is required")
			}
			ids := shared.SplitUniqueCSV(offerIDs.String())
			if len(ids) == 0 {
				return d.UsageError("--retention-offer-id is required")
			}
			if confirm != nil {
				if err := d.RequireConfirm(*confirm); err != nil {
					return err
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			if id, err = resolveSubscriptionLookupIDWithTimeoutWith(ctx, d, client, *appID, id); err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := change(client, requestCtx, id, ids); err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			result := &asc.SubscriptionRetentionMessageOffersResult{SubscriptionID: id, RetentionOfferIDs: ids, Action: resultAction}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
