package pricing

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newPricingScheduleEditCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing schedule edit", flag.ExitOnError)
	app := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	point := shared.BindResourceIDFlag(fs, "price-point", "appPricePoints", "Price point for the app's existing base territory (required)")
	start := fs.String("start-date", "", "Start date (YYYY-MM-DD, US Pacific time; today or later, required)")
	end := fs.String("end-date", "", "Exclusive end date (YYYY-MM-DD); defaults to the next scheduled base price change")
	confirm := fs.Bool("confirm", false, "Confirm editing the base territory price interval")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc pricing schedule edit --app APP_ID --price-point PRICE_POINT_ID --start-date YYYY-MM-DD [--end-date YYYY-MM-DD] --confirm",
		ShortHelp:  "Edit one base territory price interval.",
		LongHelp: `Edit one base territory price interval.

Reads every manual-price page and rebuilds current and future manual intervals,
including manual prices in other territories. Resource IDs may change. An omitted --end-date uses the end of the
interval containing --start-date; an open interval remains open. With an
explicit --end-date, the existing price schedule resumes on that date.

Dates use US Pacific time. Existing intervals must cover today and all future
dates without gaps or overlaps. Ended manual prices, unknown attributes, and
unknown relationships are rejected. Current intervals are resubmitted with an
unbounded start date. A matching price skips the write. Coordinate
schedule edits with other operators: App Store Connect replaces the schedule
when the edit is submitted. Use asc pricing schedule create for a full replacement.

Examples:
  asc pricing schedule edit --app "APP_ID" --price-point "PRICE_POINT_ID" --start-date "YYYY-MM-DD" --confirm
  asc pricing schedule edit --app "APP_ID" --price-point "PRICE_POINT_ID" --start-date "YYYY-MM-DD" --end-date "YYYY-MM-DD" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, _ []string) error {
			appID, pointID := d.ResolveAppID(*app), strings.TrimSpace(*point)
			startDate, endDate := strings.TrimSpace(*start), strings.TrimSpace(*end)
			for _, required := range []struct{ name, value string }{{"--app", appID}, {"--price-point", pointID}, {"--start-date", startDate}} {
				if required.value == "" {
					return d.RequiredFlag(required.name)
				}
			}
			if _, ok := parseAppPriceDateStrict(startDate); !ok {
				return d.UsageError("--start-date must be in YYYY-MM-DD format")
			}
			endSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "end-date" {
					endSet = true
				}
			})
			if endSet {
				if _, ok := parseAppPriceDateStrict(endDate); !ok || endDate == "" {
					return d.UsageError("--end-date must be in YYYY-MM-DD format")
				}
				if endDate <= startDate {
					return d.UsageError("--end-date must be after --start-date")
				}
			}
			if startDate < shared.PricingDate(d.Now()).Format(appPriceDateLayout) {
				return d.UsageError("--start-date must be today or later in US Pacific time")
			}
			if !*confirm {
				return d.RequiredFlag("--confirm")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing schedule edit: %w", err)
			}
			ctx = asc.WithNoRedirectReads(ctx)
			run := appPriceRequestRunner(d.Timeout)
			baseline, err := readPriceScheduleState(ctx, client, run, appID)
			if err != nil {
				return fmt.Errorf("pricing schedule edit: %w", err)
			}
			planned, changed, err := planAppPriceInterval(baseline.prices, baseline.territory, pointID, startDate, endDate, shared.PricingDate(d.Now()).Format(appPriceDateLayout))
			if err != nil {
				return fmt.Errorf("pricing schedule edit: %w", err)
			}
			if !changed {
				fmt.Fprintln(d.IO.ErrOut, "No changes: the requested price already applies throughout the interval.")
				return d.PrintOutput(baseline.schedule, *output.Output, *output.Pretty)
			}
			if err := validateEditPricePoint(ctx, client, run, appID, baseline.territory, pointID); err != nil {
				return fmt.Errorf("pricing schedule edit: %w", err)
			}
			fresh, err := readPriceScheduleState(ctx, client, run, appID)
			if err != nil {
				return fmt.Errorf("pricing schedule edit: verify schedule: %w", err)
			}
			if !samePriceScheduleState(baseline, fresh) {
				return fmt.Errorf("pricing schedule edit: the schedule changed during the edit; read the schedule and retry")
			}
			if startDate < shared.PricingDate(d.Now()).Format(appPriceDateLayout) {
				return d.UsageError("--start-date must be today or later in US Pacific time")
			}
			var result *asc.AppPriceScheduleResponse
			if err := run(ctx, func(callCtx context.Context) error {
				var err error
				result, err = client.CreateAppPriceSchedule(callCtx, appID, asc.AppPriceScheduleCreateAttributes{BaseTerritoryID: baseline.territory, Prices: planned})
				return err
			}); err != nil {
				return fmt.Errorf("pricing schedule edit: save schedule: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

type appPriceScheduleState struct {
	schedule  *asc.AppPriceScheduleResponse
	territory string
	prices    []asc.Resource[asc.AppPriceV2Attributes]
}

func readPriceScheduleState(ctx context.Context, client *asc.Client, run AppPriceRequestRunner, appID string) (*appPriceScheduleState, error) {
	state := &appPriceScheduleState{}
	if err := run(ctx, func(callCtx context.Context) error {
		var err error
		state.schedule, err = client.GetAppPriceSchedule(callCtx, appID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("read schedule: %w", err)
	}
	if state.schedule.Data.ID == "" {
		return nil, fmt.Errorf("schedule ID missing from response")
	}
	var territory *asc.TerritoryResponse
	if err := run(ctx, func(callCtx context.Context) error {
		var err error
		territory, err = client.GetAppPriceScheduleBaseTerritory(callCtx, state.schedule.Data.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("read base territory: %w", err)
	}
	state.territory = territory.Data.ID
	if state.territory == "" {
		return nil, fmt.Errorf("base territory missing from response")
	}
	var err error
	state.prices, err = fetchScheduleEditPrices(ctx, client, run, state.schedule.Data.ID)
	if err != nil {
		return nil, fmt.Errorf("read manual prices: %w", err)
	}
	return state, nil
}

func samePriceScheduleState(a, b *appPriceScheduleState) bool {
	if a.schedule.Data.ID != b.schedule.Data.ID || a.territory != b.territory {
		return false
	}
	type identity struct {
		id               string
		typeName         asc.ResourceType
		attributes       asc.AppPriceV2Attributes
		territory, point string
	}
	identities := func(prices []asc.Resource[asc.AppPriceV2Attributes]) []identity {
		result := make([]identity, 0, len(prices))
		for _, p := range prices {
			result = append(result, identity{p.ID, p.Type, p.Attributes, appPriceRelationshipID(p.Relationships, "territory"), appPriceRelationshipID(p.Relationships, "appPricePoint")})
		}
		sort.Slice(result, func(i, j int) bool { return result[i].id < result[j].id })
		return result
	}
	return slices.Equal(identities(a.prices), identities(b.prices))
}

func fetchScheduleEditPrices(ctx context.Context, client *asc.Client, run AppPriceRequestRunner, scheduleID string) ([]asc.Resource[asc.AppPriceV2Attributes], error) {
	query := url.Values{"include": {"appPricePoint,territory"}, "limit": {"200"}}
	fetch := func(callCtx context.Context, next string) (*asc.AppPricesResponse, error) {
		path := "/v1/appPriceSchedules/" + scheduleID + "/manualPrices"
		if next != "" {
			var err error
			path, err = shared.BindCollectionNextURL(callCtx, client, next, path, query)
			if err != nil {
				return nil, err
			}
		} else {
			path += "?" + query.Encode()
		}
		var body []byte
		err := run(callCtx, func(requestCtx context.Context) error {
			var err error
			body, err = client.RawRequest(requestCtx, http.MethodGet, path, nil)
			return err
		})
		if err != nil {
			return nil, err
		}
		var raw asc.Response[map[string]json.RawMessage]
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		for _, item := range raw.Data {
			for key := range item.Attributes {
				if key != "manual" && key != "startDate" && key != "endDate" {
					return nil, fmt.Errorf("manual price %q has an unknown attribute %q", item.ID, key)
				}
			}
			for _, key := range []string{"manual", "startDate", "endDate"} {
				if _, ok := item.Attributes[key]; !ok {
					return nil, fmt.Errorf("manual price %q is missing %s", item.ID, key)
				}
			}
			var relationships map[string]json.RawMessage
			if err := json.Unmarshal(item.Relationships, &relationships); err != nil {
				return nil, err
			}
			for key := range relationships {
				if key != "territory" && key != "appPricePoint" {
					return nil, fmt.Errorf("manual price %q has an unknown relationship %q", item.ID, key)
				}
			}
			for key, expectedType := range map[string]asc.ResourceType{"territory": asc.ResourceTypeTerritories, "appPricePoint": asc.ResourceTypeAppPricePoints} {
				var relationship asc.Relationship
				if err := json.Unmarshal(relationships[key], &relationship); err != nil {
					return nil, fmt.Errorf("manual price %q has invalid %s linkage", item.ID, key)
				}
				if relationship.Data.Type != expectedType || relationship.Data.ID == "" || strings.TrimSpace(relationship.Data.ID) != relationship.Data.ID {
					return nil, fmt.Errorf("manual price %q has invalid %s linkage", item.ID, key)
				}
			}
		}
		var page asc.AppPricesResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		return &page, nil
	}
	first, err := fetch(ctx, "")
	if err != nil {
		return nil, err
	}
	var prices []asc.Resource[asc.AppPriceV2Attributes]
	err = asc.PaginateEach(ctx, first, func(callCtx context.Context, next string) (asc.PaginatedResponse, error) { return fetch(callCtx, next) }, func(page asc.PaginatedResponse) error {
		prices = append(prices, page.(*asc.AppPricesResponse).Data...)
		return nil
	})
	return prices, err
}

func validateEditPricePoint(ctx context.Context, client *asc.Client, run AppPriceRequestRunner, appID, territory, pointID string) error {
	fetch := func(callCtx context.Context, next string) (*asc.AppPricePointsV3Response, error) {
		opts := []asc.PricePointsOption{asc.WithPricePointsTerritory(territory), asc.Limit[asc.PricePointsOption](200)}
		if next != "" {
			var err error
			next, err = shared.BindCollectionNextURL(callCtx, client, next, "/v1/apps/"+appID+"/appPricePoints", url.Values{"filter[territory]": {territory}, "limit": {"200"}})
			if err != nil {
				return nil, err
			}
			opts = []asc.PricePointsOption{asc.NextURL[asc.PricePointsOption](next)}
		}
		var page *asc.AppPricePointsV3Response
		err := run(callCtx, func(requestCtx context.Context) error {
			var err error
			page, err = client.GetAppPricePoints(requestCtx, appID, opts...)
			return err
		})
		return page, err
	}
	first, err := fetch(ctx, "")
	if err != nil {
		return err
	}
	found := false
	if err := asc.PaginateEach(ctx, first, func(callCtx context.Context, next string) (asc.PaginatedResponse, error) { return fetch(callCtx, next) }, func(page asc.PaginatedResponse) error {
		for _, item := range page.(*asc.AppPricePointsV3Response).Data {
			if item.ID == pointID {
				found = true
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("--price-point must belong to this app's base territory %s", territory)
	}
	return nil
}

type editableAppPrice struct{ id, point, start, end string }

func planAppPriceInterval(resources []asc.Resource[asc.AppPriceV2Attributes], territory, point, start, end, today string) ([]asc.AppPriceSchedulePrice, bool, error) {
	if start < today {
		return nil, false, fmt.Errorf("the start date is before today in US Pacific time")
	}
	var base []editableAppPrice
	seen := map[string]bool{}
	for _, item := range resources {
		if item.Type != asc.ResourceTypeAppPrices || item.ID == "" || strings.TrimSpace(item.ID) != item.ID || seen[item.ID] {
			return nil, false, fmt.Errorf("manual price identity is missing or duplicated")
		}
		seen[item.ID] = true
		itemTerritory := appPriceRelationshipID(item.Relationships, "territory")
		if itemTerritory == "" {
			return nil, false, fmt.Errorf("manual price %q has no territory linkage", item.ID)
		}
		p := editableAppPrice{item.ID, appPriceRelationshipID(item.Relationships, "appPricePoint"), item.Attributes.StartDate, item.Attributes.EndDate}
		if p.point == "" {
			return nil, false, fmt.Errorf("manual price %q has no price point linkage", item.ID)
		}
		for _, value := range []string{p.start, p.end} {
			if _, ok := parseAppPriceDateStrict(value); !ok || strings.TrimSpace(value) != value {
				return nil, false, fmt.Errorf("manual price %q has an invalid date", p.id)
			}
		}
		if p.end != "" && p.start >= p.end {
			return nil, false, fmt.Errorf("manual price %q has an invalid interval", p.id)
		}
		if !item.Attributes.Manual {
			return nil, false, fmt.Errorf("manual price %q is not marked manual", p.id)
		}
		if p.end != "" && p.end <= today {
			return nil, false, fmt.Errorf("ended manual price %q cannot be safely rebuilt", p.id)
		}
		if itemTerritory == territory {
			base = append(base, p)
		}
	}
	sort.Slice(base, func(i, j int) bool { return base[i].start < base[j].start })
	if len(base) == 0 || base[0].start > today {
		return nil, false, fmt.Errorf("base territory has no price covering today")
	}
	for i := 1; i < len(base); i++ {
		if base[i-1].end == "" || base[i].start < base[i-1].end {
			return nil, false, fmt.Errorf("base territory price intervals overlap")
		}
		if base[i].start > base[i-1].end {
			return nil, false, fmt.Errorf("base territory price intervals have a gap")
		}
	}
	if base[len(base)-1].end != "" {
		return nil, false, fmt.Errorf("base territory price intervals do not cover future dates")
	}
	if end == "" {
		for _, p := range base {
			if p.start <= start && (p.end == "" || start < p.end) {
				end = p.end
				break
			}
		}
	}
	changed := false
	affected := map[string]bool{}
	var prefix, suffix *asc.AppPriceSchedulePrice
	for _, p := range base {
		if p.end != "" && p.end <= start || end != "" && p.start >= end {
			continue
		}
		affected[p.id] = true
		changed = changed || p.point != point
		if p.start < start && start > today {
			prefix = &asc.AppPriceSchedulePrice{PricePointID: p.point, StartDate: p.start, EndDate: start}
			if prefix.StartDate < today {
				prefix.StartDate = ""
			}
		}
		if end != "" && (p.end == "" || p.end > end) {
			suffix = &asc.AppPriceSchedulePrice{PricePointID: p.point, StartDate: end, EndDate: p.end}
		}
	}
	if !changed {
		return nil, false, nil
	}
	var result []asc.AppPriceSchedulePrice
	for _, p := range resources {
		if !affected[p.ID] {
			startDate := p.Attributes.StartDate
			if startDate < today {
				startDate = ""
			}
			result = append(result, asc.AppPriceSchedulePrice{PricePointID: appPriceRelationshipID(p.Relationships, "appPricePoint"), StartDate: startDate, EndDate: p.Attributes.EndDate})
		}
	}
	if prefix != nil {
		result = append(result, *prefix)
	}
	result = append(result, asc.AppPriceSchedulePrice{PricePointID: point, StartDate: start, EndDate: end})
	if suffix != nil {
		result = append(result, *suffix)
	}
	return result, true, nil
}
