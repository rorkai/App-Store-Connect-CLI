package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/offercodes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsOfferCodesCommand returns the offer codes command group.
func SubscriptionsOfferCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "offer-codes",
		ShortUsage: "asc subscriptions offer-codes <subcommand> [flags]",
		ShortHelp:  "Manage subscription offer codes.",
		LongHelp: `Manage subscription offer codes.

Examples:
  asc subscriptions offers offer-codes list --subscription-id "SUB_ID"
  asc subscriptions offers offer-codes list --subscription-id "SUB_ID" --output table
  asc subscriptions offers offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "US"
  asc subscriptions offers offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode PAY_AS_YOU_GO --number-of-periods 1 --prices "US:PRICE_POINT_ID"
  asc subscriptions offers offer-codes generate --offer-code-id "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"
  asc subscriptions offers offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID"
  asc subscriptions offers offer-codes values --batch-id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.csv" --format csv`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsOfferCodesListCommand(),
			SubscriptionsOfferCodesGetCommand(),
			SubscriptionsOfferCodesCreateCommand(),
			SubscriptionsOfferCodesUpdateCommand(),
			SubscriptionsOfferCodesCustomCodesCommand(),
			SubscriptionsOfferCodesOneTimeCodesCommand(),
			SubscriptionsOfferCodesGenerateCommand(),
			SubscriptionsOfferCodesValuesCommand(),
			SubscriptionsOfferCodesPricesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsOfferCodesListCommand returns the offer codes list subcommand.
func SubscriptionsOfferCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes list", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions offer-codes list [flags]",
		ShortHelp:  "List offer codes for a subscription.",
		LongHelp: `List offer codes for a subscription.

Examples:
  asc subscriptions offers offer-codes list --subscription-id "SUB_ID"
  asc subscriptions offers offer-codes list --subscription-id "SUB_ID" --output table
  asc subscriptions offers offer-codes list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageErrorCtx(ctx, "subscriptions offer-codes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "subscriptions offer-codes list: %v", err)
			}
			if err := validateNextExclusiveFlags(fs, *next, "subscription-id", "app", "limit", "subscription-fields"); err != nil {
				return err
			}
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(fs, *next, "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--subscription-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodesOption{
				asc.Limit[asc.SubscriptionOfferCodesOption](*limit),
				asc.NextURL[asc.SubscriptionOfferCodesOption](*next),
				asc.WithSubscriptionOfferCodesSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionOfferCodesInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			}

			if *paginate {
				paginateOpts := opts
				if strings.TrimSpace(*next) == "" {
					paginateOpts = append(paginateOpts, asc.Limit[asc.SubscriptionOfferCodesOption](200))
				}
				firstPage, err := client.GetSubscriptionOfferCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodes(ctx, id, asc.NextURL[asc.SubscriptionOfferCodesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes list: %w", err)
				}

				return printSubscriptionOfferCodesListOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionOfferCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes list: failed to fetch: %w", err)
			}

			return printSubscriptionOfferCodesListOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func printSubscriptionOfferCodesListOutput(data any, format string, pretty bool) error {
	resp, ok := data.(*asc.SubscriptionOfferCodesResponse)
	if !ok {
		return shared.PrintOutput(data, format, pretty)
	}
	return shared.PrintOutputWithRenderers(
		resp,
		format,
		pretty,
		func() error {
			if err := asc.PrintTable(resp); err != nil {
				return err
			}
			printSubscriptionOfferCodesFollowUps(resp)
			return nil
		},
		func() error {
			return asc.PrintMarkdown(resp)
		},
	)
}

func printSubscriptionOfferCodesFollowUps(resp *asc.SubscriptionOfferCodesResponse) {
	if resp == nil || len(resp.Data) == 0 {
		return
	}
	id := strings.TrimSpace(resp.Data[0].ID)
	if id == "" {
		return
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Follow-up commands:")
	fmt.Fprintf(os.Stdout, "  asc subscriptions offers offer-codes one-time-codes list --offer-code-id %q\n", id)
	fmt.Fprintln(os.Stdout, `  asc subscriptions offers offer-codes values --batch-id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.csv" --format csv`)
}

// SubscriptionsOfferCodesGetCommand returns the offer codes get subcommand.
func SubscriptionsOfferCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes view", flag.ExitOnError)

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "subscriptionOfferCodes", "Offer code ID")
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions offer-codes view --offer-code-id \"OFFER_CODE_ID\"",
		ShortHelp:  "View an offer code by ID.",
		LongHelp: `View an offer code by ID.

Examples:
  asc subscriptions offer-codes view --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(fs, "", "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*offerCodeID)
			if id == "" {
				return shared.RequiredFlag("--offer-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionOfferCode(
				requestCtx, id,
				asc.WithSubscriptionOfferCodeSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionOfferCodeInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsOfferCodesCreateCommand returns the offer codes create subcommand.
func SubscriptionsOfferCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes create", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	name := fs.String("name", "", "Offer code name")
	offerEligibility := fs.String("offer-eligibility", "", "Offer eligibility: "+strings.Join(subscriptionOfferEligibilityValues, ", "))
	customerEligibilities := fs.String("customer-eligibilities", "", "Customer eligibilities: "+strings.Join(subscriptionCustomerEligibilityValues, ", "))
	offerDuration := fs.String("offer-duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	numberOfPeriods := fs.Int("number-of-periods", 0, "Number of periods (required)")
	prices := fs.String("prices", "", "Offer code prices (required): TERRITORY entries for FREE_TRIAL or TERRITORY:PRICE_POINT_ID entries for paid modes; territory accepts alpha-2, alpha-3, or exact English country name")
	var autoRenewEnabled shared.OptionalBool
	fs.Var(&autoRenewEnabled, "auto-renew-enabled", "Enable auto-renew: true or false")
	targetPlanType := shared.BindTargetSubscriptionPlanTypeFlag(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions offer-codes create [flags]",
		ShortHelp:  "Create an offer code.",
		LongHelp: `Create an offer code.

Examples:
  asc subscriptions offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "US"
  asc subscriptions offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode PAY_AS_YOU_GO --number-of-periods 1 --prices "US:PRICE_POINT_ID"
  asc subscriptions offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "US" --target-subscription-plan-type MONTHLY`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.RequiredFlag("--subscription-id")
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return shared.RequiredFlag("--name")
			}

			eligibility, err := normalizeSubscriptionOfferEligibility(*offerEligibility, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			customerEligibilityValues, err := normalizeSubscriptionCustomerEligibilities(*customerEligibilities)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			duration, err := normalizeSubscriptionOfferDuration(*offerDuration)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			mode, err := normalizeSubscriptionOfferMode(*offerMode)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			if *numberOfPeriods <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --number-of-periods is required")
				return requiredPositiveIntegerUsageError(fs, "number-of-periods")
			}

			priceEntries, err := parseSubscriptionOfferCodePrices(*prices, mode)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if len(priceEntries) == 0 {
				return shared.RequiredFlag("--prices")
			}

			planType, err := targetPlanType()
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes create: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionOfferCodeCreateAttributes{
				Name:                       nameValue,
				OfferEligibility:           eligibility,
				CustomerEligibilities:      customerEligibilityValues,
				Duration:                   duration,
				OfferMode:                  mode,
				NumberOfPeriods:            *numberOfPeriods,
				TargetSubscriptionPlanType: planType,
			}
			if autoRenewEnabled.IsSet() {
				value := autoRenewEnabled.Value()
				attrs.AutoRenewEnabled = &value
			}

			resp, err := client.CreateSubscriptionOfferCode(requestCtx, id, attrs, priceEntries)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsOfferCodesUpdateCommand returns the offer codes update subcommand.
func SubscriptionsOfferCodesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes update", flag.ExitOnError)

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "subscriptionOfferCodes", "Offer code ID")
	var active shared.OptionalBool
	fs.Var(&active, "active", "Enable or disable the offer code: true or false")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions offer-codes update [flags]",
		ShortHelp:  "Update an offer code.",
		LongHelp: `Update an offer code.

Examples:
  asc subscriptions offer-codes update --offer-code-id "OFFER_CODE_ID" --active false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerCodeID)
			if id == "" {
				return shared.RequiredFlag("--offer-code-id")
			}
			if !active.IsSet() {
				return shared.RequiredFlag("--active")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			value := active.Value()
			attrs := asc.SubscriptionOfferCodeUpdateAttributes{
				Active: &value,
			}

			resp, err := client.UpdateSubscriptionOfferCode(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsOfferCodesCustomCodesCommand returns the offer code custom codes subcommand.
func SubscriptionsOfferCodesCustomCodesCommand() *ffcli.Command {
	return shared.RewriteCommandTreePath(
		offercodes.OfferCodeCustomCodesCommand(),
		"asc offer-codes custom-codes",
		"asc subscriptions offer-codes custom-codes",
	)
}

// SubscriptionsOfferCodesOneTimeCodesCommand returns the offer code one-time use codes command group.
func SubscriptionsOfferCodesOneTimeCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "one-time-codes",
		ShortUsage: "asc subscriptions offer-codes one-time-codes <subcommand> [flags]",
		ShortHelp:  "Manage one-time use code batches for an offer code.",
		LongHelp: `Manage one-time use code batches for an offer code.

Examples:
  asc subscriptions offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID"
  asc subscriptions offer-codes one-time-codes view --batch-id "ONE_TIME_USE_CODE_ID"
  asc subscriptions offer-codes one-time-codes update --batch-id "ONE_TIME_USE_CODE_ID" --active false --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsOfferCodesOneTimeCodesListCommand(),
			SubscriptionsOfferCodesOneTimeCodesGetCommand(),
			SubscriptionsOfferCodesOneTimeCodesUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsOfferCodesOneTimeCodesListCommand returns the offer code one-time use codes list subcommand.
func SubscriptionsOfferCodesOneTimeCodesListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "offer-codes one-time-codes list",
		Name:        "list",
		ShortUsage:  "asc subscriptions offer-codes one-time-codes list --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:   "List one-time use code batches for an offer code.",
		LongHelp: `List one-time use code batches for an offer code.

Examples:
  asc subscriptions offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID"
  asc subscriptions offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		ParentFlag:           "offer-code-id",
		ParentUsage:          "Offer code ID",
		ParentType:           "subscriptionOfferCodes",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "subscriptions offer-codes one-time-codes list",
		FetchErrorPrefix:     "subscriptions offer-codes one-time-codes list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, offerCodeID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionOfferCodeOneTimeUseCodes(ctx, offerCodeID,
				asc.Limit[asc.SubscriptionOfferCodeOneTimeUseCodesOption](limit),
				asc.NextURL[asc.SubscriptionOfferCodeOneTimeUseCodesOption](next),
			)
		},
	})
}

// SubscriptionsOfferCodesOneTimeCodesGetCommand returns the offer code one-time use codes get subcommand.
func SubscriptionsOfferCodesOneTimeCodesGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "offer-codes one-time-codes view",
		Name:        "view",
		ShortUsage:  "asc subscriptions offer-codes one-time-codes view --batch-id \"ONE_TIME_USE_CODE_ID\"",
		ShortHelp:   "View a one-time use code batch by ID.",
		LongHelp: `View a one-time use code batch by ID.

Examples:
  asc subscriptions offer-codes one-time-codes view --batch-id "ONE_TIME_USE_CODE_ID"`,
		IDFlag:             "batch-id",
		IDUsage:            "One-time use code batch ID",
		IDType:             "subscriptionOfferCodeOneTimeUseCodes",
		RequiredDiagnostic: true,
		ErrorPrefix:        "subscriptions offer-codes one-time-codes view",
		FetchErrorPrefix:   "subscriptions offer-codes one-time-codes view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscriptionOfferCodeOneTimeUseCode(ctx, id)
		},
	})
}

// SubscriptionsOfferCodesOneTimeCodesUpdateCommand returns the one-time use code batch update subcommand.
func SubscriptionsOfferCodesOneTimeCodesUpdateCommand() *ffcli.Command {
	return offercodes.NewActiveUpdateCommand(offercodes.ActiveUpdateCommandConfig{
		FlagSetName: "offer-codes one-time-codes update",
		Name:        "update",
		ShortUsage:  "asc subscriptions offer-codes one-time-codes update --batch-id \"ONE_TIME_USE_CODE_ID\" --active false --confirm",
		ShortHelp:   "Turn off a one-time use code batch.",
		LongHelp: `Turn off a one-time use code batch.

Turning a batch off is permanent: Apple rejects --active true on an inactive
batch, so --active false requires --confirm.

Examples:
  asc subscriptions offer-codes one-time-codes update --batch-id "ONE_TIME_USE_CODE_ID" --active false --confirm`,
		IDFlag:      "batch-id",
		IDUsage:     "One-time use code batch ID (required)",
		IDType:      "subscriptionOfferCodeOneTimeUseCodes",
		ErrorPrefix: "subscriptions offer-codes one-time-codes update",
		ConfirmOff:  "a one-time use code batch",
		Update: func(ctx context.Context, client *asc.Client, id string, active *bool) (any, error) {
			return client.UpdateSubscriptionOfferCodeOneTimeUseCode(ctx, id, asc.SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes{Active: active})
		},
	})
}

// SubscriptionsOfferCodesPricesCommand returns the offer code prices subcommand.
func SubscriptionsOfferCodesPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes prices", flag.ExitOnError)

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "subscriptionOfferCodes", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	pricePointFields := fs.String("price-point-fields", "", "Included subscription price point fields (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions offer-codes prices --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List prices for an offer code.",
		LongHelp: `List prices for an offer code.

Examples:
  asc subscriptions offer-codes prices --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageErrorCtx(ctx, "subscriptions offer-codes prices: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "subscriptions offer-codes prices: %v", err)
			}
			if err := validateNextExclusiveFlags(fs, *next, "offer-code-id", "limit", "price-point-fields"); err != nil {
				return err
			}
			selectedPricePointFields, err := normalizeSparseFieldsFlag(fs, *next, "price-point-fields", *pricePointFields, subscriptionPricePointFieldsList())
			if err != nil {
				return err
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--offer-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodePricesOption{
				asc.Limit[asc.SubscriptionOfferCodePricesOption](*limit),
				asc.NextURL[asc.SubscriptionOfferCodePricesOption](*next),
				asc.WithSubscriptionOfferCodePricesPricePointFields(selectedPricePointFields),
				asc.WithSubscriptionOfferCodePricesInclude(includeRelationshipForFields(selectedPricePointFields, "subscriptionPricePoint")),
			}

			if *paginate {
				paginateOpts := opts
				if strings.TrimSpace(*next) == "" {
					paginateOpts = append(paginateOpts, asc.Limit[asc.SubscriptionOfferCodePricesOption](200))
				}
				firstPage, err := client.GetSubscriptionOfferCodePrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodePrices(ctx, id, asc.NextURL[asc.SubscriptionOfferCodePricesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes prices: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionOfferCodePrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes prices: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsOfferCodesGenerateCommand returns the nested generate shim under subscriptions.
func SubscriptionsOfferCodesGenerateCommand() *ffcli.Command {
	return shared.RewriteCommandTreePath(
		offercodes.OfferCodesGenerateCommand(),
		"asc offer-codes",
		"asc subscriptions offer-codes",
	)
}

// SubscriptionsOfferCodesValuesCommand returns the nested values shim under subscriptions.
func SubscriptionsOfferCodesValuesCommand() *ffcli.Command {
	return shared.RewriteCommandTreePath(
		offercodes.OfferCodesValuesCommand(),
		"asc offer-codes",
		"asc subscriptions offer-codes",
	)
}
