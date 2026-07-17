package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var subscriptionPricePointsClientFactory = shared.GetASCClient

// SubscriptionsPricePointsCommand returns the subscription price points command group.
func SubscriptionsPricePointsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc subscriptions price-points <subcommand> [flags]",
		ShortHelp:  "Manage subscription price points.",
		LongHelp: `Manage subscription price points.

Examples:
  asc subscriptions price-points list --subscription-id "SUB_ID"
  asc subscriptions price-points view --price-point-id "PRICE_POINT_ID"
  asc subscriptions price-points equalizations --price-point-id "PRICE_POINT_ID"
  asc subscriptions price-points adjusted-equalizations --price-point-id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricePointsListCommand(),
			SubscriptionsPricePointsGetCommand(),
			SubscriptionsPricePointsEqualizationsCommand(),
			SubscriptionsPricePointsAdjustedEqualizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricePointsListCommand returns the price points list subcommand.
func SubscriptionsPricePointsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	territory := fs.String("territory", "", "Filter by territory (accepts alpha-2, alpha-3, or exact English country name) to reduce results")
	price := fs.String("price", "", "Filter by exact customer price (e.g., 4.99)")
	minPrice := fs.String("min-price", "", "Filter by minimum customer price")
	maxPrice := fs.String("max-price", "", "Filter by maximum customer price")
	upfrontPricePointIDs := fs.String("upfront-price-point-id", "", "Filter by upfront price point IDs (comma-separated)")
	planTypes := fs.String("plan-type", "", "Filter by plan types (comma-separated)")
	fields := fs.String("fields", "", "Subscription price point fields (comma-separated)")
	territoryFields := fs.String("territory-fields", "", "Territory fields (comma-separated): currency")
	include := fs.String("include", "", "Relationships to include (comma-separated): territory")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	stream := fs.Bool("stream", false, "Stream pages as NDJSON (one JSON object per page, requires --paginate)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions price-points list [flags]",
		ShortHelp:  "List price points for a subscription.",
		LongHelp: `List price points for a subscription.

Use --territory to filter by a specific territory. Without it, all territories
are returned (140K+ results for subscriptions). Filtering by territory reduces
results to ~800 and completes in seconds instead of 20+ minutes.

Use --price to find a specific customer price, or --min-price/--max-price for
a range. These filters are applied client-side after fetching. Combine with
--territory and --paginate for best results. When --fields is also set, the CLI
automatically requests customerPrice so the client-side filter remains correct.
When --territory-fields is set, the CLI automatically includes the territory
relationship so the requested fields are present in the response.

Use --stream with --paginate to emit each page as a separate JSON line (NDJSON)
instead of buffering all pages in memory. This gives immediate feedback and
reduces memory usage for very large result sets.

Examples:
  asc subscriptions price-points list --subscription-id "SUB_ID"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "United States"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "US" --paginate
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "France" --paginate --price "4.99"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "DE" --paginate --min-price "1.00" --max-price "9.99"
  asc subscriptions price-points list --subscription-id "SUB_ID" --paginate --stream`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectUnexpectedArgs(args); err != nil {
				return err
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}
			if strings.TrimSpace(*next) != "" && flagWasProvided(
				fs,
				"subscription-id", "app", "territory", "price", "min-price", "max-price",
				"upfront-price-point-id", "plan-type", "fields", "territory-fields", "include", "limit",
			) {
				return shared.UsageError("--next cannot be combined with owner flags, --limit, API filters, sparse fields, includes, or client-side price filters")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions price-points list: --limit must be between 1 and 200")
			}
			if *stream && !*paginate {
				return shared.UsageError("--stream requires --paginate")
			}

			priceFilter := shared.PriceFilter{
				Price:    strings.TrimSpace(*price),
				MinPrice: strings.TrimSpace(*minPrice),
				MaxPrice: strings.TrimSpace(*maxPrice),
			}
			if err := priceFilter.Validate(); err != nil {
				return shared.UsageError(err.Error())
			}
			if priceFilter.HasFilter() && *stream {
				return shared.UsageError("price filtering is not supported with --stream")
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.UsageError("--subscription-id is required")
			}

			territoryFilter := strings.TrimSpace(*territory)
			if territoryFilter != "" {
				var err error
				territoryFilter, err = ascterritory.Normalize(territoryFilter)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}
			upfrontIDs, err := normalizeOptionalCSVFilter(fs, "upfront-price-point-id", *upfrontPricePointIDs, false)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			plans, err := normalizeOptionalSubscriptionPlanTypes(fs, "plan-type", *planTypes)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			selectedFields, err := normalizeOptionalSelection(fs, "fields", *fields, subscriptionPricePointFields)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if priceFilter.HasFilter() && selectedFields != nil && !containsString(selectedFields, "customerPrice") {
				selectedFields = append(selectedFields, "customerPrice")
			}
			selectedTerritoryFields, err := normalizeOptionalSelection(fs, "territory-fields", *territoryFields, []string{"currency"})
			if err != nil {
				return shared.UsageError(err.Error())
			}
			selectedIncludes, err := normalizeOptionalSelection(fs, "include", *include, []string{"territory"})
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(selectedTerritoryFields) != 0 && !containsString(selectedIncludes, "territory") {
				selectedIncludes = append(selectedIncludes, "territory")
			}
			selectedFields = ensureTerritoryInSparsePricePointFields(selectedFields, selectedIncludes)
			client, err := subscriptionPricePointsClientFactory()
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			baseOpts := []asc.SubscriptionPricePointsOption{
				asc.WithSubscriptionPricePointsTerritory(territoryFilter),
				asc.WithSubscriptionPricePointsUpfrontPricePointIDs(upfrontIDs),
				asc.WithSubscriptionPricePointsPlanTypes(plans),
				asc.WithSubscriptionPricePointsFields(selectedFields),
				asc.WithSubscriptionPricePointsTerritoryFields(selectedTerritoryFields),
				asc.WithSubscriptionPricePointsInclude(selectedIncludes),
			}
			opts := append(baseOpts, asc.WithSubscriptionPricePointsLimit(*limit), asc.WithSubscriptionPricePointsNextURL(*next))

			if *paginate && *stream {
				// Streaming mode: emit each page as a separate JSON line
				paginateOpts := []asc.SubscriptionPricePointsOption{asc.WithSubscriptionPricePointsNextURL(*next)}
				if strings.TrimSpace(*next) == "" {
					paginateOpts = append(baseOpts, asc.WithSubscriptionPricePointsLimit(200))
				}
				firstPageCtx, firstPageCancel := shared.ContextWithTimeout(ctx)
				page, err := client.GetSubscriptionPricePoints(firstPageCtx, id, paginateOpts...)
				firstPageCancel()
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
				}
				if err := asc.PaginateEach(
					ctx,
					page,
					func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
						pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
						defer pageCancel()
						pageOpts := []asc.SubscriptionPricePointsOption{asc.WithSubscriptionPricePointsNextURL(nextURL)}
						return client.GetSubscriptionPricePoints(pageCtx, id, pageOpts...)
					},
					func(page asc.PaginatedResponse) error {
						typed, ok := page.(*asc.SubscriptionPricePointsResponse)
						if !ok {
							return fmt.Errorf("unexpected pagination response type %T", page)
						}
						return shared.PrintStreamPage(typed)
					},
				); err != nil {
					return fmt.Errorf("subscriptions price-points list: %w", err)
				}
				return nil
			}

			if *paginate {
				paginateOpts := []asc.SubscriptionPricePointsOption{asc.WithSubscriptionPricePointsNextURL(*next)}
				if strings.TrimSpace(*next) == "" {
					paginateOpts = append(baseOpts, asc.WithSubscriptionPricePointsLimit(200))
				}
				firstPageCtx, firstPageCancel := shared.ContextWithTimeout(ctx)
				firstPage, err := client.GetSubscriptionPricePoints(firstPageCtx, id, paginateOpts...)
				firstPageCancel()
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
					pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
					defer pageCancel()
					pageOpts := []asc.SubscriptionPricePointsOption{asc.WithSubscriptionPricePointsNextURL(nextURL)}
					return client.GetSubscriptionPricePoints(pageCtx, id, pageOpts...)
				})
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: %w", err)
				}

				if priceFilter.HasFilter() {
					if typed, ok := resp.(*asc.SubscriptionPricePointsResponse); ok {
						filterSubscriptionPricePoints(typed, priceFilter)
						return shared.PrintOutput(typed, *output.Output, *output.Pretty)
					}
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionPricePoints(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
			}

			if priceFilter.HasFilter() {
				filterSubscriptionPricePoints(resp, priceFilter)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func ensureTerritoryInSparsePricePointFields(fields, includes []string) []string {
	if fields == nil || !containsString(includes, "territory") || containsString(fields, "territory") {
		return fields
	}
	return append(fields, "territory")
}

func flagWasProvided(fs *flag.FlagSet, names ...string) bool {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}

	provided := false
	fs.Visit(func(f *flag.Flag) {
		if _, ok := wanted[f.Name]; ok {
			provided = true
		}
	})
	return provided
}

// filterSubscriptionPricePoints removes data entries that don't match the price filter.
func filterSubscriptionPricePoints(resp *asc.SubscriptionPricePointsResponse, pf shared.PriceFilter) {
	filtered := resp.Data[:0]
	for _, item := range resp.Data {
		if pf.MatchesPrice(item.Attributes.CustomerPrice) {
			filtered = append(filtered, item)
		}
	}
	resp.Data = filtered
}

// SubscriptionsPricePointsGetCommand returns the price points get subcommand.
func SubscriptionsPricePointsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points view", flag.ExitOnError)

	pricePointID := fs.String("price-point-id", "", "Subscription price point ID")
	fields := fs.String("fields", "", "Subscription price point fields (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions price-points view --price-point-id \"PRICE_POINT_ID\"",
		ShortHelp:  "View a subscription price point by ID.",
		LongHelp: `View a subscription price point by ID.

Examples:
  asc subscriptions price-points view --price-point-id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selectedFields, err := normalizeSparseFieldsFlag(fs, "", "fields", *fields, subscriptionPricePointFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*pricePointID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions price-points view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPricePoint(requestCtx, id, asc.WithSubscriptionPricePointFields(selectedFields))
			if err != nil {
				return fmt.Errorf("subscriptions price-points view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func SubscriptionsPricePointsEqualizationsCommand() *ffcli.Command {
	return buildSubscriptionPricePointEqualizationsCommand("equalizations", false)
}

// SubscriptionsPricePointsAdjustedEqualizationsCommand returns the adjusted equalizations subcommand.
func SubscriptionsPricePointsAdjustedEqualizationsCommand() *ffcli.Command {
	return buildSubscriptionPricePointEqualizationsCommand("adjusted-equalizations", true)
}

var subscriptionPricePointFields = subscriptionPricePointFieldsList()

func buildSubscriptionPricePointEqualizationsCommand(name string, adjusted bool) *ffcli.Command {
	flagSetName := "price-points " + name
	fs := flag.NewFlagSet(flagSetName, flag.ExitOnError)
	pricePointID := fs.String("price-point-id", "", "Subscription price point ID")
	territory := fs.String("territory", "", "Filter by territory IDs or names (comma-separated)")
	subscriptionIDs := fs.String("subscription-id", "", "Filter by subscription IDs (comma-separated)")
	upfrontPricePointIDUsage := "Filter by upfront price point IDs (comma-separated)"
	if adjusted {
		upfrontPricePointIDUsage = "Required upfront price point IDs (comma-separated)"
	}
	upfrontPricePointIDs := fs.String("upfront-price-point-id", "", upfrontPricePointIDUsage)
	planTypeUsage := "Filter by plan types (comma-separated)"
	if adjusted {
		planTypeUsage = "Required plan type: MONTHLY"
	}
	planTypes := fs.String("plan-type", "", planTypeUsage)
	fields := fs.String("fields", "", "Subscription price point fields (comma-separated)")
	territoryFields := fs.String("territory-fields", "", "Territory fields (comma-separated): currency")
	include := fs.String("include", "", "Relationships to include (comma-separated): territory")
	limit := fs.Int("limit", 0, "Maximum results per page (1-8000)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	adjective := ""
	if adjusted {
		adjective = "adjusted "
	}
	errorPrefix := "subscriptions price-points " + name
	baseUsage := fmt.Sprintf(`asc subscriptions price-points %s --price-point-id "PRICE_POINT_ID"`, name)
	subscriptionExample := baseUsage + ` --subscription-id "SUB_ID" --plan-type MONTHLY`
	requirementHelp := ""
	if adjusted {
		baseUsage += ` --upfront-price-point-id "UPFRONT_PRICE_POINT_ID" --plan-type MONTHLY`
		subscriptionExample = baseUsage + ` --subscription-id "SUB_ID"`
		requirementHelp = `

Apple's live endpoint requires both --upfront-price-point-id and --plan-type
MONTHLY for the first page. An opaque --next URL already carries its query and
does not accept those flags.`
	}

	cmd := &ffcli.Command{
		Name:       name,
		ShortUsage: baseUsage + " [flags]",
		ShortHelp:  fmt.Sprintf("List %sequalized price points for a subscription price point.", adjective),
		LongHelp: fmt.Sprintf(`List %sequalized price points for a subscription price point.

Filters accept comma-separated values and map directly to the App Store Connect
API. Setting --territory-fields automatically includes the territory relationship
so the requested metadata is present in returned price points.%s

Examples:
  %s
  %s --territory "US,France" --include territory
  %s
  %s --paginate`, adjective, requirementHelp, baseUsage, baseUsage, subscriptionExample, baseUsage),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
	}
	usageErrorPrefix := func() string {
		prefix := strings.TrimSpace(strings.SplitN(cmd.ShortUsage, " --", 2)[0])
		return strings.TrimSpace(strings.TrimPrefix(prefix, "asc "))
	}

	cmd.Exec = func(ctx context.Context, args []string) error {
		if len(args) > 0 {
			return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
		}
		if err := shared.ValidateNextURL(*next); err != nil {
			return shared.UsageErrorf("%s: %v", usageErrorPrefix(), err)
		}
		if strings.TrimSpace(*next) != "" && flagWasProvided(
			fs,
			"price-point-id", "territory", "subscription-id", "upfront-price-point-id",
			"plan-type", "fields", "territory-fields", "include", "limit",
		) {
			return shared.UsageError("--next cannot be combined with owner flags, --limit, API filters, sparse fields, or includes")
		}
		if *limit != 0 && (*limit < 1 || *limit > 8000) {
			return shared.UsageErrorf("%s: --limit must be between 1 and 8000", usageErrorPrefix())
		}

		id := strings.TrimSpace(*pricePointID)
		if id == "" && strings.TrimSpace(*next) == "" {
			return shared.UsageError("--price-point-id is required")
		}

		territories, err := normalizeOptionalTerritoryFilter(fs, "territory", *territory)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		subscriptions, err := normalizeOptionalCSVFilter(fs, "subscription-id", *subscriptionIDs, false)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		upfrontIDs, err := normalizeOptionalCSVFilter(fs, "upfront-price-point-id", *upfrontPricePointIDs, false)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		plans, err := normalizeOptionalSubscriptionPlanTypes(fs, "plan-type", *planTypes)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		if adjusted {
			// OpenAPI leaves planType unconstrained, but the live adjusted
			// equalizations endpoint reports MONTHLY as its only supported value.
			for _, plan := range plans {
				if plan != string(asc.SubscriptionPlanTypeMonthly) {
					return shared.UsageError("--plan-type must be MONTHLY for adjusted equalizations")
				}
			}
		}
		selectedFields, err := normalizeOptionalSelection(fs, "fields", *fields, subscriptionPricePointFields)
		if err != nil {
			return shared.UsageError(err.Error())
		}
		selectedTerritoryFields, err := normalizeOptionalSelection(fs, "territory-fields", *territoryFields, []string{"currency"})
		if err != nil {
			return shared.UsageError(err.Error())
		}
		selectedIncludes, err := normalizeOptionalSelection(fs, "include", *include, []string{"territory"})
		if err != nil {
			return shared.UsageError(err.Error())
		}
		if adjusted && strings.TrimSpace(*next) == "" {
			if len(upfrontIDs) == 0 {
				return shared.UsageError("--upfront-price-point-id is required for adjusted equalizations")
			}
			if len(plans) == 0 {
				return shared.UsageError("--plan-type is required for adjusted equalizations")
			}
		}
		if len(selectedTerritoryFields) != 0 && !containsString(selectedIncludes, "territory") {
			selectedIncludes = append(selectedIncludes, "territory")
		}
		selectedFields = ensureTerritoryInSparsePricePointFields(selectedFields, selectedIncludes)
		client, err := subscriptionPricePointsClientFactory()
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}

		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()

		fetchPage := func(pageCtx context.Context, nextURL string, pageLimit int) (asc.PaginatedResponse, error) {
			var opts []asc.SubscriptionPricePointEqualizationsOption
			if strings.TrimSpace(nextURL) != "" {
				opts = []asc.SubscriptionPricePointEqualizationsOption{asc.WithSubscriptionPricePointsNextURL(nextURL)}
			} else {
				opts = []asc.SubscriptionPricePointEqualizationsOption{
					asc.WithSubscriptionPricePointsTerritories(territories),
					asc.WithSubscriptionPricePointsSubscriptions(subscriptions),
					asc.WithSubscriptionPricePointsUpfrontPricePointIDs(upfrontIDs),
					asc.WithSubscriptionPricePointsPlanTypes(plans),
					asc.WithSubscriptionPricePointsFields(selectedFields),
					asc.WithSubscriptionPricePointsTerritoryFields(selectedTerritoryFields),
					asc.WithSubscriptionPricePointsInclude(selectedIncludes),
					asc.WithSubscriptionPricePointsLimit(pageLimit),
				}
			}
			if adjusted {
				return client.GetSubscriptionPricePointAdjustedEqualizations(pageCtx, id, opts...)
			}
			return client.GetSubscriptionPricePointEqualizations(pageCtx, id, opts...)
		}

		if *paginate {
			firstLimit := *limit
			if firstLimit == 0 {
				firstLimit = 8000
			}
			resp, err := shared.PaginateWithSpinner(
				requestCtx,
				func(pageCtx context.Context) (asc.PaginatedResponse, error) {
					return fetchPage(pageCtx, *next, firstLimit)
				},
				func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return fetchPage(pageCtx, nextURL, 0)
				},
			)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		}

		resp, err := fetchPage(requestCtx, *next, *limit)
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}
		return shared.PrintOutput(resp, *output.Output, *output.Pretty)
	}
	return cmd
}

func normalizeOptionalCSVFilter(fs *flag.FlagSet, name, raw string, upper bool) ([]string, error) {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	if !provided {
		return nil, nil
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("invalid value for --%s: cannot be empty", name)
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("invalid value for --%s: cannot contain empty values", name)
		}
	}
	if upper {
		return shared.SplitCSVUpper(raw), nil
	}
	return shared.SplitCSV(raw), nil
}

func normalizeOptionalTerritoryFilter(fs *flag.FlagSet, name, raw string) ([]string, error) {
	values, err := normalizeOptionalCSVFilter(fs, name, raw, false)
	if err != nil || values == nil {
		return values, err
	}
	return shared.NormalizeASCTerritoryCSV(raw)
}

func normalizeOptionalSubscriptionPlanTypes(fs *flag.FlagSet, name, raw string) ([]string, error) {
	values, err := normalizeOptionalCSVFilter(fs, name, raw, true)
	if err != nil || values == nil {
		return values, err
	}
	for _, value := range values {
		if _, err := normalizeSubscriptionPlanType(value); err != nil {
			return nil, err
		}
	}
	return values, nil
}

func normalizeOptionalSelection(fs *flag.FlagSet, name, raw string, allowed []string) ([]string, error) {
	values, err := normalizeOptionalCSVFilter(fs, name, raw, false)
	if err != nil || values == nil {
		return values, err
	}
	return shared.NormalizeSelection(raw, allowed, "--"+name)
}
