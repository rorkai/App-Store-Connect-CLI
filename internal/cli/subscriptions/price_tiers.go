package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

var subscriptionPriceTierFields = []string{"minQuantity", "subscriptionPricePoint"}

func newSubscriptionsPricingPriceTiersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing price-tiers", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "price-tiers",
		ShortUsage: "asc subscriptions pricing price-tiers <subcommand> [flags]",
		ShortHelp:  "List and equalize subscription quantity price tiers.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsPriceTiersListCommand(d),
			newSubscriptionsPriceTiersEqualizeCommand(d),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newSubscriptionsPriceTiersListCommand(d *shared.Deps) *ffcli.Command {
	return buildSubscriptionPriceTiersListCommand(d, false)
}

func newSubscriptionsPricesAdditionalPriceTiersCommand(d *shared.Deps) *ffcli.Command {
	return buildSubscriptionPriceTiersListCommand(d, true)
}

func buildSubscriptionPriceTiersListCommand(d *shared.Deps, related bool) *ffcli.Command {
	name := "list"
	command := "subscriptions pricing price-tiers list"
	help := "List subscription quantity price tiers."
	if related {
		name = "additional-price-tiers"
		command = "subscriptions pricing prices additional-price-tiers"
		help = "List additional tiers for a subscription price."
	}
	fs := d.NewFlagSet(command, flag.ExitOnError)
	subscriptionID := fs.String("subscription-id", "", "Filter by subscription IDs (comma-separated)")
	var priceID *string
	if related {
		priceID = shared.BindResourceIDFlag(fs, "price-id", "subscriptionPrices", "Subscription price ID")
	}
	territory := fs.String("territory", "", "Filter by territories (comma-separated; country names or codes)")
	fields := fs.String("fields", "", "Subscription price tier fields (comma-separated): minQuantity, subscriptionPricePoint")
	pricePointFields := fs.String("price-point-fields", "", "Included subscription price point fields (comma-separated)")
	include := fs.String("include", "", "Relationships to include: subscriptionPricePoint")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc " + command + " [flags]",
		ShortHelp:  help,
		LongHelp: help + `

Use --price-point-fields to include the price point relationship automatically.
Use --paginate to fetch every page.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			if err := d.ValidatePaginationFlags(ctx, fs, command, 200, "subscription-id", "territory", "fields", "price-point-fields", "include", "price-id", "limit"); err != nil {
				return err
			}
			ids, err := normalizeOptionalCSVFilter(fs, "subscription-id", *subscriptionID, false)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if strings.TrimSpace(*next) == "" {
				if related && strings.TrimSpace(*priceID) == "" {
					return d.RequiredFlag("--price-id")
				}
				if !related && len(ids) == 0 {
					return d.RequiredFlag("--subscription-id")
				}
			}
			territories, err := normalizeOptionalTerritoryFilter(fs, *territory)
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedFields, err := normalizeOptionalSelection(fs, "fields", *fields, subscriptionPriceTierFields)
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedPointFields, err := normalizeOptionalSelection(fs, "price-point-fields", *pricePointFields, subscriptionPricePointFieldsList())
			if err != nil {
				return d.UsageError(err.Error())
			}
			includes, err := normalizeOptionalSelection(fs, "include", *include, []string{"subscriptionPricePoint"})
			if err != nil {
				return d.UsageError(err.Error())
			}
			if len(selectedPointFields) > 0 && !slices.Contains(includes, "subscriptionPricePoint") {
				includes = append(includes, "subscriptionPricePoint")
			}
			if len(selectedFields) > 0 && slices.Contains(includes, "subscriptionPricePoint") && !slices.Contains(selectedFields, "subscriptionPricePoint") {
				selectedFields = append(selectedFields, "subscriptionPricePoint")
			}
			expectedPath := "/v1/subscriptionPriceTiers"
			if related {
				expectedPath, err = subscriptionPriceTierRelationshipPath(strings.TrimSpace(*priceID), *next, "/additionalPriceTiers")
				if err != nil {
					return d.UsageError(fmt.Sprintf("--next: %v", err))
				}
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			ctx = asc.WithNoRedirectReads(ctx)
			nextURL, err := client.ValidateNoRedirectReadContinuation(ctx, *next, expectedPath)
			if err != nil {
				return d.UsageError(fmt.Sprintf("--next: %v", err))
			}
			firstLimit := *limit
			if *paginate && firstLimit == 0 {
				firstLimit = 200
			}
			opts := []asc.SubscriptionPriceTiersOption{
				asc.WithSubscriptionPriceTiersSubscriptions(ids),
				asc.WithSubscriptionPriceTiersTerritories(territories),
				asc.WithSubscriptionPriceTiersFields(selectedFields),
				asc.WithSubscriptionPriceTiersPricePointFields(selectedPointFields),
				asc.WithSubscriptionPriceTiersInclude(includes),
				asc.Limit[asc.SubscriptionPriceTiersOption](firstLimit),
				asc.NextURL[asc.SubscriptionPriceTiersOption](nextURL),
			}
			fetch := func(parent context.Context, opts ...asc.SubscriptionPriceTiersOption) (*asc.SubscriptionPriceTiersResponse, error) {
				requestCtx, cancel := d.Timeout(parent)
				defer cancel()
				if related {
					return client.GetSubscriptionPriceAdditionalPriceTiers(requestCtx, strings.TrimSpace(*priceID), opts...)
				}
				return client.GetSubscriptionPriceTiers(requestCtx, opts...)
			}
			resp, err := fetch(ctx, opts...)
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			if *paginate {
				values := url.Values{}
				setSubscriptionPricesCSVQuery(values, "filter[subscription]", ids)
				setSubscriptionPricesCSVQuery(values, "filter[territory]", territories)
				setSubscriptionPricesCSVQuery(values, "fields[subscriptionPriceTiers]", selectedFields)
				setSubscriptionPricesCSVQuery(values, "fields[subscriptionPricePoints]", selectedPointFields)
				setSubscriptionPricesCSVQuery(values, "include", includes)
				if nextURL != "" {
					parsed, _ := url.Parse(nextURL) // validated above
					saved := parsed.Query()
					for _, key := range []string{"filter[subscription]", "filter[territory]", "fields[subscriptionPriceTiers]", "fields[subscriptionPricePoints]", "include"} {
						if saved.Has(key) {
							values[key] = saved[key]
						}
					}
				}
				scopeQuery := url.Values{}
				for _, key := range []string{"filter[subscription]", "filter[territory]"} {
					if values.Has(key) {
						scopeQuery[key] = values[key]
					}
				}
				all, err := asc.PaginateAll(ctx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					scopedNextURL, scopeErr := client.ValidateNoRedirectReadContinuationQuery(pageCtx, nextURL, expectedPath, scopeQuery)
					if scopeErr != nil {
						return nil, scopeErr
					}
					mergedNextURL, mergeErr := mergeSubscriptionPricesNextQuery(scopedNextURL, values)
					if mergeErr != nil {
						return nil, mergeErr
					}
					return fetch(pageCtx, asc.NextURL[asc.SubscriptionPriceTiersOption](mergedNextURL))
				})
				if err != nil {
					return fmt.Errorf("%s: %w", command, err)
				}
				resp = all.(*asc.SubscriptionPriceTiersResponse)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newSubscriptionsPricesAdditionalPriceTiersLinksCommand(d *shared.Deps) *ffcli.Command {
	const command = "subscriptions pricing prices additional-price-tiers-links"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	priceID := shared.BindResourceIDFlag(fs, "price-id", "subscriptionPrices", "Subscription price ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "additional-price-tiers-links",
		ShortUsage: "asc " + command + " [flags]",
		ShortHelp:  "List additional price tier relationship linkages.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			if err := d.ValidatePaginationFlags(ctx, fs, command, 200, "price-id", "limit"); err != nil {
				return err
			}
			id := strings.TrimSpace(*priceID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--price-id")
			}
			expectedPath, err := subscriptionPriceTierRelationshipPath(id, *next, "/relationships/additionalPriceTiers")
			if err != nil {
				return d.UsageError(fmt.Sprintf("--next: %v", err))
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			ctx = asc.WithNoRedirectReads(ctx)
			nextURL, err := client.ValidateNoRedirectReadContinuation(ctx, *next, expectedPath)
			if err != nil {
				return d.UsageError(fmt.Sprintf("--next: %v", err))
			}
			fetch := func(parent context.Context, opts ...asc.LinkagesOption) (*asc.LinkagesResponse, error) {
				requestCtx, cancel := d.Timeout(parent)
				defer cancel()
				return client.GetSubscriptionPriceAdditionalPriceTiersRelationships(requestCtx, id, opts...)
			}
			firstLimit := *limit
			if *paginate && firstLimit == 0 {
				firstLimit = 200
			}
			resp, err := fetch(ctx, asc.Limit[asc.LinkagesOption](firstLimit), asc.NextURL[asc.LinkagesOption](nextURL))
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			if *paginate {
				all, err := asc.PaginateAll(ctx, resp, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					scopedNextURL, err := client.ValidateNoRedirectReadContinuation(pageCtx, nextURL, expectedPath)
					if err != nil {
						return nil, err
					}
					return fetch(pageCtx, asc.NextURL[asc.LinkagesOption](scopedNextURL))
				})
				if err != nil {
					return fmt.Errorf("%s: %w", command, err)
				}
				resp = all.(*asc.LinkagesResponse)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newSubscriptionsPriceTiersEqualizeCommand(d *shared.Deps) *ffcli.Command {
	const command = "subscriptions pricing price-tiers equalize"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	basePoint := shared.BindResourceIDFlag(fs, "base-price-point", "subscriptionPricePoints", "Base subscription price point ID")
	tiersInput := fs.String("additional-price-tiers", "", "JSON file of {minQuantity,subscriptionPricePointId} rows (or - for stdin)")
	fields := fs.String("fields", "", "Equalization fields (comma-separated): equalizedPrices, priceTierPricePoints")
	pricePointFields := fs.String("price-point-fields", "", "Included subscription price point fields (comma-separated)")
	include := fs.String("include", "", "Relationships to include: priceTierPricePoints")
	pointLimit := fs.Int("price-points-limit", 0, "Maximum included price points (1-50)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "equalize",
		ShortUsage: "asc " + command + " --subscription-id SUB_ID --base-price-point POINT_ID [flags]",
		ShortHelp:  "Compute equalized prices for proposed quantity tiers.",
		LongHelp: `Compute equalized prices for proposed quantity tiers.

The public API returns proposed territory prices without applying a price change.
The optional --additional-price-tiers JSON file contains an array such as
[{"minQuantity":10,"subscriptionPricePointId":"PRICE_POINT_ID"}].
Pass [] to propose no additional tiers.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}
			point := strings.TrimSpace(*basePoint)
			if point == "" {
				return d.RequiredFlag("--base-price-point")
			}
			if flagWasProvided(fs, "price-points-limit") && (*pointLimit < 1 || *pointLimit > 50) {
				return d.UsageError("--price-points-limit must be between 1 and 50")
			}
			selectedFields, err := normalizeOptionalSelection(fs, "fields", *fields, []string{"equalizedPrices", "priceTierPricePoints"})
			if err != nil {
				return d.UsageError(err.Error())
			}
			selectedPoints, err := normalizeOptionalSelection(fs, "price-point-fields", *pricePointFields, subscriptionPricePointFieldsList())
			if err != nil {
				return d.UsageError(err.Error())
			}
			includes, err := normalizeOptionalSelection(fs, "include", *include, []string{"priceTierPricePoints"})
			if err != nil {
				return d.UsageError(err.Error())
			}
			if (len(selectedPoints) > 0 || flagWasProvided(fs, "price-points-limit")) && !slices.Contains(includes, "priceTierPricePoints") {
				includes = append(includes, "priceTierPricePoints")
			}
			if len(selectedFields) > 0 && slices.Contains(includes, "priceTierPricePoints") && !slices.Contains(selectedFields, "priceTierPricePoints") {
				selectedFields = append(selectedFields, "priceTierPricePoints")
			}
			var tiers *[]asc.ProposedSubscriptionPriceTier
			if flagWasProvided(fs, "additional-price-tiers") {
				tiers, err = readSubscriptionPriceTiersInput(d, *tiersInput)
				if err != nil {
					return d.UsageError(err.Error())
				}
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionPriceTierEqualization(requestCtx, id, point, tiers, asc.SubscriptionPriceTierEqualizationQuery{
				Fields:           selectedFields,
				PricePointFields: selectedPoints,
				Include:          includes,
				PricePointsLimit: *pointLimit,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newSubscriptionsPriceChangePreviewCommand(d *shared.Deps) *ffcli.Command {
	const command = "subscriptions pricing price-change-preview"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	input := fs.String("proposed-changes", "", "JSON file of proposed territory changes (or - for stdin)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "price-change-preview",
		ShortUsage: "asc " + command + " --subscription-id SUB_ID [flags]",
		ShortHelp:  "Preview the effect of proposed subscription prices.",
		LongHelp: `Preview the effect of proposed subscription prices.

The optional --proposed-changes JSON file contains an array such as
[{"territoryId":"USA","startDate":"2027-01-01","basePricePointId":"POINT_ID","additionalPriceTiers":[{"minQuantity":10,"subscriptionPricePointId":"TIER_POINT_ID"}]}].
The API returns each territory's change classification without applying prices.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}
			var changes *[]asc.ProposedSubscriptionPriceChange
			if flagWasProvided(fs, "proposed-changes") {
				data, err := readSubscriptionPricingInput(d, *input)
				if err != nil {
					return d.UsageError(err.Error())
				}
				var rows []json.RawMessage
				if err := decodeSubscriptionPricingInput(data, &rows); err != nil {
					return d.UsageError(fmt.Sprintf("--proposed-changes: %v", err))
				}
				var value []asc.ProposedSubscriptionPriceChange
				if rows != nil {
					value = make([]asc.ProposedSubscriptionPriceChange, 0, len(rows))
				}
				for i, rawRow := range rows {
					var row struct {
						TerritoryID          string          `json:"territoryId"`
						StartDate            string          `json:"startDate"`
						BasePricePointID     string          `json:"basePricePointId"`
						AdditionalPriceTiers json.RawMessage `json:"additionalPriceTiers"`
					}
					if err := decodeSubscriptionPricingInput(rawRow, &row); err != nil {
						return d.UsageError(fmt.Sprintf("proposed change %d: %v", i+1, err))
					}
					var properties map[string]json.RawMessage
					if err := json.Unmarshal(rawRow, &properties); err != nil || properties == nil {
						return d.UsageError(fmt.Sprintf("proposed change %d must be an object", i+1))
					}
					for _, field := range []string{"territoryId", "startDate", "basePricePointId"} {
						if bytes.Equal(properties[field], []byte("null")) {
							return d.UsageError(fmt.Sprintf("proposed change %d: %s must be a string", i+1, field))
						}
					}
					change := asc.ProposedSubscriptionPriceChange{
						TerritoryID:      row.TerritoryID,
						StartDate:        row.StartDate,
						BasePricePointID: row.BasePricePointID,
					}
					if change.TerritoryID != "" {
						change.TerritoryID, err = ascterritory.Normalize(change.TerritoryID)
						if err != nil {
							return d.UsageError(err.Error())
						}
					}
					if change.StartDate != "" {
						if _, err := time.Parse("2006-01-02", change.StartDate); err != nil {
							return d.UsageError(fmt.Sprintf("proposed change %d: startDate must use YYYY-MM-DD", i+1))
						}
					}
					if row.AdditionalPriceTiers != nil {
						tiers, err := decodeSubscriptionPriceTiersInput(row.AdditionalPriceTiers)
						if err != nil {
							return d.UsageError(fmt.Sprintf("proposed change %d: %v", i+1, err))
						}
						if *tiers == nil {
							return d.UsageError(fmt.Sprintf("proposed change %d: additionalPriceTiers must be an array", i+1))
						}
						change.AdditionalPriceTiers = tiers
					}
					value = append(value, change)
				}
				changes = &value
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.CreateSubscriptionPriceChangePreview(requestCtx, id, changes)
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func readSubscriptionPriceTiersInput(d *shared.Deps, path string) (*[]asc.ProposedSubscriptionPriceTier, error) {
	data, err := readSubscriptionPricingInput(d, path)
	if err != nil {
		return nil, fmt.Errorf("--additional-price-tiers: %w", err)
	}
	return decodeSubscriptionPriceTiersInput(data)
}

func decodeSubscriptionPriceTiersInput(data []byte) (*[]asc.ProposedSubscriptionPriceTier, error) {
	var rows []struct {
		MinQuantity              *int   `json:"minQuantity"`
		SubscriptionPricePointID string `json:"subscriptionPricePointId"`
	}
	if err := decodeSubscriptionPricingInput(data, &rows); err != nil {
		return nil, fmt.Errorf("--additional-price-tiers: %w", err)
	}
	var tiers []asc.ProposedSubscriptionPriceTier
	if rows != nil {
		tiers = make([]asc.ProposedSubscriptionPriceTier, 0, len(rows))
	}
	for i, row := range rows {
		if row.MinQuantity == nil {
			return nil, fmt.Errorf("price tier %d: minQuantity is required", i+1)
		}
		tiers = append(tiers, asc.ProposedSubscriptionPriceTier{MinQuantity: *row.MinQuantity, SubscriptionPricePointID: strings.TrimSpace(row.SubscriptionPricePointID)})
	}
	if err := asc.ValidateProposedSubscriptionPriceTiers(tiers); err != nil {
		return nil, err
	}
	return &tiers, nil
}

func readSubscriptionPricingInput(d *shared.Deps, path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("input path is required")
	}
	if path == "-" {
		return d.ReadJSONFilePayloadKind(path, shared.JSONPayloadAny)
	}
	workingDir, err := d.Getwd()
	if err != nil {
		return nil, err
	}
	file, err := rootfs.OpenFileIn(workingDir, d.OSPath(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func decodeSubscriptionPricingInput(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("input must contain one JSON value")
	}
	return nil
}

func subscriptionPriceTierRelationshipPath(priceID, nextURL, suffix string) (string, error) {
	const prefix = "/v1/subscriptionPrices/"
	if priceID != "" {
		return prefix + url.PathEscape(priceID) + suffix, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return "", fmt.Errorf("invalid subscription price tier pagination URL")
	}
	path := parsed.Path
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if path != prefix+id+suffix || id == "" || id == "." || id == ".." || strings.Contains(id, "/") {
		return "", fmt.Errorf("pagination URL must target one subscription price%s collection", suffix)
	}
	return prefix + url.PathEscape(id) + suffix, nil
}
