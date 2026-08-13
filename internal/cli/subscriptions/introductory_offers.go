package subscriptions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const subscriptionIntroductoryOfferCreateTimeout = 5 * time.Minute

const subscriptionIntroductoryOfferCreateSelectorGuidance = `Try:
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --territory "USA" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --all-territories --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
For help:
  asc subscriptions offers introductory create --help
`

// SubscriptionsIntroductoryOffersCommand returns the introductory offers command group.
func SubscriptionsIntroductoryOffersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "introductory-offers",
		ShortUsage: "asc subscriptions introductory-offers <subcommand> [flags]",
		ShortHelp:  "Manage subscription introductory offers.",
		LongHelp: `Manage subscription introductory offers.

Examples:
  asc subscriptions offers introductory list --subscription-id "SUB_ID"
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --territory "USA" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions offers introductory import --subscription-id "SUB_ID" --input "./offers.csv" --offer-duration ONE_WEEK --offer-mode FREE_TRIAL --number-of-periods 1 --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsIntroductoryOffersListCommand(),
			SubscriptionsIntroductoryOffersGetCommand(),
			SubscriptionsIntroductoryOffersCreateCommand(),
			SubscriptionsIntroductoryOffersImportCommand(),
			SubscriptionsIntroductoryOffersUpdateCommand(),
			SubscriptionsIntroductoryOffersDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsIntroductoryOffersListCommand returns the introductory offers list subcommand.
func SubscriptionsIntroductoryOffersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	pricePointFields := fs.String("price-point-fields", "", "Included subscription price point fields (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions introductory-offers list [flags]",
		ShortHelp:  "List introductory offers for a subscription.",
		LongHelp: `List introductory offers for a subscription.

Examples:
  asc subscriptions introductory-offers list --subscription-id "SUB_ID"
  asc subscriptions introductory-offers list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions introductory-offers list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions introductory-offers list: %w", err)
			}
			if err := validateNextExclusiveFlags(fs, *next, "subscription-id", "app", "limit", "subscription-fields", "price-point-fields"); err != nil {
				return err
			}
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(fs, *next, "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}
			selectedPricePointFields, err := normalizeSparseFieldsFlag(fs, *next, "price-point-fields", *pricePointFields, subscriptionPricePointFieldsList())
			if err != nil {
				return err
			}
			include := includeRelationshipForFields(selectedSubscriptionFields, "subscription")
			include = appendIncludeForFields(include, selectedPricePointFields, "subscriptionPricePoint")

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionIntroductoryOffersOption{
				asc.WithSubscriptionIntroductoryOffersLimit(*limit),
				asc.WithSubscriptionIntroductoryOffersNextURL(*next),
				asc.WithSubscriptionIntroductoryOffersSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionIntroductoryOffersPricePointFields(selectedPricePointFields),
				asc.WithSubscriptionIntroductoryOffersInclude(include),
			}

			if *paginate {
				paginateOpts := opts
				if strings.TrimSpace(*next) == "" {
					paginateOpts = append(paginateOpts, asc.WithSubscriptionIntroductoryOffersLimit(200))
				}
				firstPage, err := client.GetSubscriptionIntroductoryOffers(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions introductory-offers list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionIntroductoryOffers(ctx, id, asc.WithSubscriptionIntroductoryOffersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions introductory-offers list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionIntroductoryOffers(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsIntroductoryOffersGetCommand returns the introductory offers get subcommand.
func SubscriptionsIntroductoryOffersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers view", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	offerID := fs.String("id", "", "Introductory offer ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions introductory-offers view --subscription-id \"SUB_ID\" --id \"OFFER_ID\"",
		ShortHelp:  "View an introductory offer by subscription and offer ID.",
		LongHelp: `View an introductory offer by subscription and offer ID.

The subscription selector accepts a subscription ID, product ID, or exact current name.
Product IDs and names require --app to resolve the subscription.

Examples:
  asc subscriptions introductory-offers view --subscription-id "SUB_ID" --id "OFFER_ID"
  asc subscriptions introductory-offers view --app "APP_ID" --subscription-id "com.example.monthly" --id "OFFER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			subID := strings.TrimSpace(*subscriptionID)
			if subID == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers view: %w", err)
			}

			subID, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, subID)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := findSubscriptionIntroductoryOffer(requestCtx, client, subID, id)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func findSubscriptionIntroductoryOffer(ctx context.Context, client *asc.Client, subscriptionID, offerID string) (*asc.SubscriptionIntroductoryOfferResponse, error) {
	firstPage, err := client.GetSubscriptionIntroductoryOffers(ctx, subscriptionID, asc.WithSubscriptionIntroductoryOffersLimit(200))
	if err != nil {
		return nil, err
	}

	errOfferFound := errors.New("introductory offer found")
	var found *asc.SubscriptionIntroductoryOfferResponse
	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetSubscriptionIntroductoryOffers(ctx, subscriptionID, asc.WithSubscriptionIntroductoryOffersNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.SubscriptionIntroductoryOffersResponse)
		if !ok {
			return fmt.Errorf("unexpected page type %T", page)
		}
		for _, offer := range resp.Data {
			if offer.ID == offerID {
				found = &asc.SubscriptionIntroductoryOfferResponse{Data: offer}
				return errOfferFound
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errOfferFound) {
		return nil, err
	}
	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf("introductory offer %q not found for subscription %q: %w", offerID, subscriptionID, asc.ErrNotFound)
}

// SubscriptionsIntroductoryOffersCreateCommand returns the introductory offers create subcommand.
func SubscriptionsIntroductoryOffersCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	offerDuration := fs.String("offer-duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	numberOfPeriods := fs.Int("number-of-periods", 0, "Number of periods (required)")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD)")
	endDate := fs.String("end-date", "", "End date (YYYY-MM-DD)")
	territory := fs.String("territory", "", "Territory for the offer (accepts alpha-2, alpha-3, or exact English country name; required unless --all-territories)")
	allTerritories := fs.Bool("all-territories", false, "Create introductory offers for all current subscription availability territories")
	pricePoint := fs.String("price-point", "", "Subscription price point ID")
	dryRun := fs.Bool("dry-run", false, "Resolve territories and print summary without creating offers")
	continueOnError := fs.Bool("continue-on-error", true, "Continue creating offers after a territory fails")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: `asc subscriptions offers introductory create --subscription-id "SUB_ID" (--territory "USA" | --all-territories) [flags]`,
		ShortHelp:  "Create an introductory offer.",
		LongHelp: `Create an introductory offer.

Examples:
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --territory "USA" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --all-territories --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --all-territories --dry-run --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1

Timeouts:
  An explicit ASC_TIMEOUT caps the full create operation. Without an override, the operation uses a 5m fallback while individual requests retain the standard request timeout.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
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
				return shared.MissingRequiredUsageError()
			}

			var normalizedStartDate string
			if strings.TrimSpace(*startDate) != "" {
				normalizedStartDate, err = shared.NormalizeDate(*startDate, "--start-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			var normalizedEndDate string
			if strings.TrimSpace(*endDate) != "" {
				normalizedEndDate, err = shared.NormalizeDate(*endDate, "--end-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			territoryProvided := false
			fs.Visit(func(parsed *flag.Flag) {
				if parsed.Name == "territory" {
					territoryProvided = true
				}
			})
			territoryID := strings.TrimSpace(*territory)
			if territoryProvided && territoryID == "" {
				return subscriptionIntroductoryOfferSelectorUsageError(
					shared.UsageErrorInvalidValue,
					"invalid value for --territory: cannot be empty",
				)
			}
			if territoryProvided == *allTerritories {
				kind := shared.UsageErrorMissingRequired
				if territoryProvided {
					kind = shared.UsageErrorInvalidValue
				}
				return subscriptionIntroductoryOfferSelectorUsageError(
					kind,
					"exactly one of --territory or --all-territories is required",
				)
			}

			legacyAllTerritories := territoryProvided && strings.EqualFold(territoryID, "ALL")
			useAllTerritories := *allTerritories || legacyAllTerritories
			if useAllTerritories && strings.TrimSpace(*pricePoint) != "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point cannot be used with --all-territories or --territory ALL")
				return flag.ErrHelp
			}
			if legacyAllTerritories {
				fmt.Fprintln(os.Stderr, "Warning: `--territory ALL` is deprecated. Use `--all-territories`.")
				territoryID = ""
			} else if territoryProvided {
				territoryID, err = ascterritory.Normalize(territoryID)
				if err != nil {
					return subscriptionIntroductoryOfferSelectorUsageError(shared.UsageErrorInvalidValue, err.Error())
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers create: %w", err)
			}

			operationCtx, operationCancel := contextWithSubscriptionIntroductoryOfferCreateTimeout(ctx)
			defer operationCancel()

			id, err = resolveSubscriptionLookupIDWithTimeout(operationCtx, client, *appID, id)
			if err != nil {
				return err
			}

			attrs := asc.SubscriptionIntroductoryOfferCreateAttributes{
				Duration:        duration,
				OfferMode:       mode,
				NumberOfPeriods: *numberOfPeriods,
			}
			if normalizedStartDate != "" {
				attrs.StartDate = normalizedStartDate
			}
			if normalizedEndDate != "" {
				attrs.EndDate = normalizedEndDate
			}

			if useAllTerritories {
				return createSubscriptionIntroductoryOffersForAllTerritories(
					operationCtx,
					client,
					id,
					attrs,
					*dryRun,
					*continueOnError,
					*output.Output,
					*output.Pretty,
				)
			}

			requestCtx, cancel := shared.ContextWithTimeout(operationCtx)
			defer cancel()

			resp, err := client.CreateSubscriptionIntroductoryOffer(
				requestCtx,
				id,
				attrs,
				territoryID,
				strings.TrimSpace(*pricePoint),
			)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func subscriptionIntroductoryOfferSelectorUsageError(kind shared.UsageErrorKind, message string) error {
	fmt.Fprintf(os.Stderr, "Error: %s\n%s", strings.TrimSpace(message), subscriptionIntroductoryOfferCreateSelectorGuidance)
	return shared.NewReportedUsageError(kind, message)
}

type subscriptionIntroductoryOfferCreateBulkSummary struct {
	SubscriptionID  string                                                  `json:"subscriptionId"`
	AvailabilityID  string                                                  `json:"availabilityId,omitempty"`
	AllTerritories  bool                                                    `json:"allTerritories"`
	DryRun          bool                                                    `json:"dryRun"`
	ContinueOnError bool                                                    `json:"continueOnError"`
	Total           int                                                     `json:"total"`
	Created         int                                                     `json:"created"`
	Skipped         int                                                     `json:"skipped"`
	Failed          int                                                     `json:"failed"`
	Skips           []subscriptionIntroductoryOfferCreateBulkSummarySkip    `json:"skips,omitempty"`
	Failures        []subscriptionIntroductoryOfferCreateBulkSummaryFailure `json:"failures,omitempty"`
}

type subscriptionIntroductoryOfferCreateBulkSummarySkip struct {
	Territory string `json:"territory"`
	Reason    string `json:"reason"`
}

type subscriptionIntroductoryOfferCreateBulkSummaryFailure struct {
	Territory string `json:"territory"`
	Error     string `json:"error"`
}

func createSubscriptionIntroductoryOffersForAllTerritories(
	ctx context.Context,
	client *asc.Client,
	subscriptionID string,
	attrs asc.SubscriptionIntroductoryOfferCreateAttributes,
	dryRun bool,
	continueOnError bool,
	output string,
	pretty bool,
) error {
	availabilityID, territories, err := fetchIntroductoryOfferAvailabilityTerritories(ctx, client, subscriptionID)
	if err != nil {
		return fmt.Errorf("subscriptions introductory-offers create: %w", err)
	}

	existing, err := fetchIntroductoryOfferTerritories(ctx, client, subscriptionID)
	if err != nil {
		return fmt.Errorf("subscriptions introductory-offers create: %w", err)
	}

	summary := &subscriptionIntroductoryOfferCreateBulkSummary{
		SubscriptionID:  subscriptionID,
		AvailabilityID:  availabilityID,
		AllTerritories:  true,
		DryRun:          dryRun,
		ContinueOnError: continueOnError,
		Total:           len(territories),
	}

	var operationErr error
	for _, territoryID := range territories {
		if _, ok := existing[territoryID]; ok {
			appendSubscriptionIntroductoryOfferCreateBulkSkip(summary, territoryID, "introductory offer already exists for territory")
			continue
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			appendSubscriptionIntroductoryOfferCreateBulkFailure(summary, territoryID, ctxErr)
			operationErr = ctxErr
			break
		}

		if dryRun {
			summary.Created++
			continue
		}

		createCtx, createCancel := shared.ContextWithTimeout(ctx)
		_, err := client.CreateSubscriptionIntroductoryOffer(createCtx, subscriptionID, attrs, territoryID, "")
		createCancel()
		if err != nil {
			appendSubscriptionIntroductoryOfferCreateBulkFailure(summary, territoryID, err)
			if ctxErr := ctx.Err(); ctxErr != nil {
				operationErr = ctxErr
				break
			}
			if !continueOnError {
				break
			}
			continue
		}

		summary.Created++
	}

	if err := shared.PrintOutputWithRenderers(
		summary,
		output,
		pretty,
		func() error { return renderSubscriptionIntroductoryOfferCreateBulkSummary(summary, false) },
		func() error { return renderSubscriptionIntroductoryOfferCreateBulkSummary(summary, true) },
	); err != nil {
		return err
	}
	if operationErr != nil {
		return shared.NewReportedError(fmt.Errorf("subscriptions introductory-offers create: operation stopped: %w", operationErr))
	}
	if summary.Failed > 0 {
		return shared.NewReportedError(fmt.Errorf("subscriptions introductory-offers create: %d territor%s failed", summary.Failed, pluralizeIntroductoryOfferCreateTerritories(summary.Failed)))
	}
	return nil
}

func contextWithSubscriptionIntroductoryOfferCreateTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithResolvedTimeout(ctx, subscriptionIntroductoryOfferCreateTimeout)
}

func fetchIntroductoryOfferAvailabilityTerritories(ctx context.Context, client *asc.Client, subscriptionID string) (string, []string, error) {
	availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
	availability, err := client.GetSubscriptionAvailabilityForSubscription(availabilityCtx, subscriptionID)
	availabilityCancel()
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch subscription availability: %w", err)
	}

	availabilityID := strings.TrimSpace(availability.Data.ID)
	if availabilityID == "" {
		return "", nil, fmt.Errorf("subscription availability readback returned empty id")
	}

	territoriesCtx, territoriesCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetSubscriptionAvailabilityAvailableTerritories(territoriesCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
	territoriesCancel()
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch subscription availability territories: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionAvailabilityAvailableTerritories(pageCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesNextURL(nextURL))
	})
	if err != nil {
		return "", nil, fmt.Errorf("paginate subscription availability territories: %w", err)
	}

	typed, ok := allPages.(*asc.TerritoriesResponse)
	if !ok {
		return "", nil, fmt.Errorf("unexpected subscription availability territories response type %T", allPages)
	}

	territories := make([]string, 0, len(typed.Data))
	seen := make(map[string]struct{}, len(typed.Data))
	for _, territory := range typed.Data {
		id := strings.ToUpper(strings.TrimSpace(territory.ID))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		territories = append(territories, id)
	}
	return availabilityID, territories, nil
}

func fetchIntroductoryOfferTerritories(ctx context.Context, client *asc.Client, subscriptionID string) (map[string]struct{}, error) {
	offersCtx, offersCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetSubscriptionIntroductoryOffers(offersCtx, subscriptionID, asc.WithSubscriptionIntroductoryOffersLimit(200))
	offersCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing introductory offers: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionIntroductoryOffers(pageCtx, subscriptionID, asc.WithSubscriptionIntroductoryOffersNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate existing introductory offers: %w", err)
	}

	typed, ok := allPages.(*asc.SubscriptionIntroductoryOffersResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected introductory offers response type %T", allPages)
	}

	existing := make(map[string]struct{}, len(typed.Data))
	for _, offer := range typed.Data {
		territoryID := introductoryOfferTerritoryID(offer)
		if territoryID == "" {
			continue
		}
		existing[territoryID] = struct{}{}
	}
	return existing, nil
}

func introductoryOfferTerritoryID(offer asc.Resource[asc.SubscriptionIntroductoryOfferAttributes]) string {
	if len(offer.Relationships) != 0 {
		var relationships struct {
			Territory *struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			} `json:"territory"`
		}
		if err := json.Unmarshal(offer.Relationships, &relationships); err == nil && relationships.Territory != nil {
			if territoryID := strings.ToUpper(strings.TrimSpace(relationships.Territory.Data.ID)); territoryID != "" {
				return territoryID
			}
		}
	}
	return introductoryOfferTerritoryIDFromEncodedID(offer.ID)
}

func introductoryOfferTerritoryIDFromEncodedID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}

	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(id)
		if err != nil {
			return ""
		}
	}

	// ASC currently omits territory relationships from introductory offer lists,
	// but the opaque offer ID contains the App Store territory code under "i".
	var payload struct {
		Territory string `json:"i"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return ""
	}

	territoryID, err := ascterritory.Normalize(payload.Territory)
	if err != nil {
		return strings.ToUpper(strings.TrimSpace(payload.Territory))
	}
	return territoryID
}

func appendSubscriptionIntroductoryOfferCreateBulkSkip(summary *subscriptionIntroductoryOfferCreateBulkSummary, territoryID, reason string) {
	if summary == nil {
		return
	}
	summary.Skipped++
	summary.Skips = append(summary.Skips, subscriptionIntroductoryOfferCreateBulkSummarySkip{
		Territory: territoryID,
		Reason:    reason,
	})
}

func appendSubscriptionIntroductoryOfferCreateBulkFailure(summary *subscriptionIntroductoryOfferCreateBulkSummary, territoryID string, err error) {
	if summary == nil || err == nil {
		return
	}
	summary.Failed++
	summary.Failures = append(summary.Failures, subscriptionIntroductoryOfferCreateBulkSummaryFailure{
		Territory: territoryID,
		Error:     err.Error(),
	})
}

func renderSubscriptionIntroductoryOfferCreateBulkSummary(summary *subscriptionIntroductoryOfferCreateBulkSummary, markdown bool) error {
	if summary == nil {
		return fmt.Errorf("summary is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Subscription ID", "Availability ID", "Dry Run", "Total", "Created", "Skipped", "Failed"},
		[][]string{{
			summary.SubscriptionID,
			summary.AvailabilityID,
			fmt.Sprintf("%t", summary.DryRun),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Skipped),
			fmt.Sprintf("%d", summary.Failed),
		}},
	)

	if len(summary.Skips) > 0 {
		rows := make([][]string, 0, len(summary.Skips))
		for _, skip := range summary.Skips {
			rows = append(rows, []string{skip.Territory, skip.Reason})
		}
		render([]string{"Skipped Territory", "Reason"}, rows)
	}

	if len(summary.Failures) > 0 {
		rows := make([][]string, 0, len(summary.Failures))
		for _, failure := range summary.Failures {
			rows = append(rows, []string{failure.Territory, failure.Error})
		}
		render([]string{"Failed Territory", "Error"}, rows)
	}

	return nil
}

func pluralizeIntroductoryOfferCreateTerritories(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// SubscriptionsIntroductoryOffersUpdateCommand returns the introductory offers update subcommand.
func SubscriptionsIntroductoryOffersUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers update", flag.ExitOnError)

	offerID := fs.String("id", "", "Introductory offer ID")
	endDate := fs.String("end-date", "", "End date (YYYY-MM-DD)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions introductory-offers update [flags]",
		ShortHelp:  "Update an introductory offer.",
		LongHelp: `Update an introductory offer.

Examples:
  asc subscriptions introductory-offers update --id "OFFER_ID" --end-date "2026-02-01"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*endDate) == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
			}

			normalizedEndDate, err := shared.NormalizeDate(*endDate, "--end-date")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionIntroductoryOfferUpdateAttributes{
				EndDate: &normalizedEndDate,
			}

			resp, err := client.UpdateSubscriptionIntroductoryOffer(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsIntroductoryOffersDeleteCommand returns the introductory offers delete subcommand.
func SubscriptionsIntroductoryOffersDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers delete", flag.ExitOnError)

	offerID := fs.String("id", "", "Introductory offer ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions introductory-offers delete --id \"OFFER_ID\" --confirm",
		ShortHelp:  "Delete an introductory offer.",
		LongHelp: `Delete an introductory offer.

Examples:
  asc subscriptions introductory-offers delete --id "OFFER_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionIntroductoryOffer(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions introductory-offers delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{ID: id, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
