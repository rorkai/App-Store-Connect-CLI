package testflight

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	// betaTesterResolveChunkSize bounds how many tester IDs are packed into a
	// single filter[id] lookup on GET /v1/betaTesters.
	betaTesterResolveChunkSize = 50
	// betaTesterResolveMaxIDs caps how many unique tester IDs --resolve-testers
	// resolves, bounding the API fan-out on very large metric responses.
	betaTesterResolveMaxIDs = 500
)

func newTestFlightMetricsAppTestersCommand(d *shared.Deps) *ffcli.Command {
	cmd := rewriteCommandTree(
		newTestFlightMetricsBetaTesterUsagesCommand(d),
		"asc testflight metrics beta-tester-usages",
		"asc testflight metrics app-testers",
		map[string]string{
			"beta-tester-usages": "app-testers",
		},
		[]textReplacement{
			{old: "beta tester usage metrics for an app", new: "app tester usage metrics"},
			{old: "Beta tester usage metrics for an app", new: "App tester usage metrics"},
			{old: "beta tester", new: "tester"},
			{old: "Beta tester", new: "Tester"},
		},
	)
	cmd.ShortHelp = "Fetch TestFlight app tester usage metrics."
	cmd.LongHelp = `Fetch TestFlight app tester usage metrics.

Examples:
  asc testflight metrics app-testers --app "APP_ID"
  asc testflight metrics app-testers --app "APP_ID" --period "P30D"
  asc testflight metrics app-testers --app "APP_ID" --filter-tester "TESTER_ID"
  asc testflight metrics app-testers --app "APP_ID" --resolve-testers`
	if groupByFlag := cmd.FlagSet.Lookup("group-by"); groupByFlag != nil {
		groupByFlag.Usage = "Group results by dimension (testers)"
		groupByFlag.DefValue = "testers"
	}
	cmd.UsageFunc = d.DefaultUsageFunc
	return cmd
}

func newTestFlightMetricsGroupTestersCommand(d *shared.Deps) *ffcli.Command {
	cmd := rewriteCommandTree(
		newTestFlightMetricsTestersCommand(d),
		"asc testflight metrics testers",
		"asc testflight metrics group-testers",
		map[string]string{
			"testers": "group-testers",
		},
		[]textReplacement{
			{old: "Fetch TestFlight beta tester usage metrics.", new: "Fetch TestFlight group tester usage metrics."},
			{old: "Fetch TestFlight beta tester usage metrics", new: "Fetch TestFlight group tester usage metrics"},
			{old: "metrics testers", new: "metrics group-testers"},
		},
	)
	cmd.UsageFunc = d.DefaultUsageFunc
	return cmd
}

// newTestFlightMetricsBetaTesterUsagesCommand fetches app-level beta tester usage metrics.
func newTestFlightMetricsBetaTesterUsagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics beta-tester-usages", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	period := fs.String("period", "", "Reporting period: "+strings.Join(betaTesterUsagePeriodList(), ", "))
	groupBy := fs.String("group-by", "testers", "Group results by dimension (testers)")
	filterTester := shared.BindResourceIDFlag(fs, "filter-tester", "betaTesters", "Filter by beta tester ID")
	resolveTesters := fs.Bool("resolve-testers", false, "Resolve tester IDs to email and name (extra API calls)")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "beta-tester-usages",
		ShortUsage: "asc testflight metrics beta-tester-usages --app \"APP_ID\" [flags]",
		ShortHelp:  "Fetch TestFlight beta tester usage metrics for an app.",
		LongHelp: `Fetch TestFlight beta tester usage metrics for an app.

Requires either --group-by or --filter-tester (or both).

Examples:
  asc testflight metrics beta-tester-usages --app "APP_ID"
  asc testflight metrics beta-tester-usages --app "APP_ID" --period "P30D"
  asc testflight metrics beta-tester-usages --app "APP_ID" --filter-tester "TESTER_ID"
  asc testflight metrics beta-tester-usages --app "APP_ID" --resolve-testers`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight metrics app-testers", 200, "period", "group-by", "filter-tester"); err != nil {
				return err
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextValue := strings.TrimSpace(*next)
			if nextValue == "" && resolvedAppID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			periodValue, err := normalizeBetaTesterUsagePeriod(*period)
			if err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--period")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterUsagesOption{
				asc.Limit[asc.BetaTesterUsagesOption](*limit),
				asc.NextURL[asc.BetaTesterUsagesOption](*next),
				asc.WithBetaTesterUsagesPeriod(periodValue),
				asc.WithBetaTesterUsagesGroupBy(normalizeBetaTesterUsageGroupBy(*groupBy)),
				asc.WithBetaTesterUsagesFilterBetaTesters(*filterTester),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BetaTesterUsagesOption](200))
				firstPage, err := client.GetAppBetaTesterUsagesMetrics(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight metrics beta-tester-usages: failed to fetch: %w", err)
				}

				combined, err := paginateBetaTesterUsages(requestCtx, client, resolvedAppID, firstPage)
				if err != nil {
					return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
				}

				if *resolveTesters {
					if err := resolveBetaTesterUsageTesters(ctx, d, client, combined); err != nil {
						return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
					}
				}

				return d.PrintOutput(combined, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppBetaTesterUsagesMetrics(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("testflight metrics beta-tester-usages: failed to fetch: %w", err)
			}

			if *resolveTesters {
				page, err := parseBetaTesterUsagesPage(resp.Data)
				if err != nil {
					return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
				}
				if err := resolveBetaTesterUsageTesters(ctx, d, client, page); err != nil {
					return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
				}
				return d.PrintOutput(page, *output.Output, *output.Pretty)
			}

			return printAppTesterUsages(d, resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeBetaTesterUsageGroupBy(value string) string {
	switch strings.TrimSpace(value) {
	case "", "testers", "betaTesters":
		return "betaTesters"
	default:
		return strings.TrimSpace(value)
	}
}

func paginateBetaTesterUsages(ctx context.Context, client *asc.Client, appID string, firstPage *asc.BetaTesterUsagesResponse) (*asc.BetaTesterUsagesPage, error) {
	if firstPage == nil {
		return nil, nil
	}

	combined := &asc.BetaTesterUsagesPage{}
	seenNext := make(map[string]struct{})
	pageNumber := 1
	current := firstPage
	var mergedIncluded []json.RawMessage

	for {
		parsed, err := parseBetaTesterUsagesPage(current.Data)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", pageNumber, err)
		}

		combined.Data = append(combined.Data, parsed.Data...)
		if len(parsed.Included) > 0 {
			var pageIncluded []json.RawMessage
			if err := json.Unmarshal(parsed.Included, &pageIncluded); err != nil {
				return nil, fmt.Errorf("page %d: parse included: %w", pageNumber, err)
			}
			mergedIncluded = append(mergedIncluded, pageIncluded...)
		}
		if len(combined.Meta) == 0 && len(parsed.Meta) > 0 {
			combined.Meta = parsed.Meta
		}

		next := strings.TrimSpace(parsed.Links.Next)
		if next == "" {
			break
		}
		if _, ok := seenNext[next]; ok {
			return combined, fmt.Errorf("page %d: %w", pageNumber+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[next] = struct{}{}
		pageNumber++

		nextPage, err := client.GetAppBetaTesterUsagesMetrics(ctx, appID, asc.NextURL[asc.BetaTesterUsagesOption](next))
		if err != nil {
			return combined, fmt.Errorf("page %d: %w", pageNumber, err)
		}
		current = nextPage
	}

	if len(mergedIncluded) > 0 {
		encoded, err := json.Marshal(mergedIncluded)
		if err != nil {
			return nil, fmt.Errorf("encode included resources: %w", err)
		}
		combined.Included = encoded
	}

	return combined, nil
}

func printAppTesterUsages(d *shared.Deps, resp *asc.BetaTesterUsagesResponse, format string, pretty bool) error {
	resolved, err := shared.ValidateOutputFormat(format, pretty)
	if err != nil {
		return err
	}
	if resolved == "table" || resolved == "markdown" {
		page := &asc.BetaTesterUsagesPage{}
		if resp != nil && len(resp.Data) > 0 {
			page, err = parseBetaTesterUsagesPage(resp.Data)
			if err != nil {
				return err
			}
		}
		return d.PrintOutput(page, resolved, pretty)
	}
	return d.PrintOutput(resp, format, pretty)
}

func parseBetaTesterUsagesPage(data json.RawMessage) (*asc.BetaTesterUsagesPage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	var page asc.BetaTesterUsagesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &page, nil
}

// resolveBetaTesterUsageTesters fills page.Testers with tester details for the
// unique tester IDs referenced by the page's dimensions.betaTesters.data
// values. Testers already present in the page's included resources are used
// as-is; the remainder are batch-fetched from GET /v1/betaTesters via
// filter[id]. Resolution is capped at betaTesterResolveMaxIDs unique IDs; IDs
// beyond the cap are skipped with a stderr warning.
func resolveBetaTesterUsageTesters(ctx context.Context, d *shared.Deps, client *asc.Client, page *asc.BetaTesterUsagesPage) error {
	if page == nil {
		return nil
	}

	ids, err := betaTesterUsageTesterIDs(page.Data)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Fprintln(d.IO.ErrOut, "Warning: --resolve-testers found no tester IDs in the metrics response; nothing to resolve")
		return nil
	}
	if len(ids) > betaTesterResolveMaxIDs {
		fmt.Fprintf(d.IO.ErrOut, "Warning: --resolve-testers resolves at most %d unique testers; skipping resolution for %d of %d\n", betaTesterResolveMaxIDs, len(ids)-betaTesterResolveMaxIDs, len(ids))
		ids = ids[:betaTesterResolveMaxIDs]
	}

	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}

	testers := make(map[string]asc.BetaTesterUsageTesterInfo, len(ids))
	addTester := func(resource asc.Resource[asc.BetaTesterAttributes]) {
		id := strings.TrimSpace(resource.ID)
		if id == "" {
			return
		}
		if _, ok := wanted[id]; !ok {
			return
		}
		testers[id] = asc.BetaTesterUsageTesterInfo{
			ID:         id,
			Email:      resource.Attributes.Email,
			FirstName:  resource.Attributes.FirstName,
			LastName:   resource.Attributes.LastName,
			State:      string(resource.Attributes.State),
			InviteType: string(resource.Attributes.InviteType),
		}
	}

	if len(page.Included) > 0 {
		var included []asc.Resource[asc.BetaTesterAttributes]
		if err := json.Unmarshal(page.Included, &included); err != nil {
			return fmt.Errorf("parse included resources: %w", err)
		}
		for _, resource := range included {
			if resource.Type != asc.ResourceTypeBetaTesters {
				continue
			}
			addTester(resource)
		}
	}

	var missing []string
	for _, id := range ids {
		if _, ok := testers[id]; !ok {
			missing = append(missing, id)
		}
	}

	for chunk := range slices.Chunk(missing, betaTesterResolveChunkSize) {
		fetched, err := fetchBetaTestersByIDs(ctx, d, client, chunk)
		if err != nil {
			return fmt.Errorf("resolve testers: %w", err)
		}
		for _, resource := range fetched {
			addTester(resource)
		}
	}

	unresolved := 0
	for _, id := range ids {
		if _, ok := testers[id]; !ok {
			unresolved++
		}
	}
	if unresolved > 0 {
		fmt.Fprintf(d.IO.ErrOut, "Warning: --resolve-testers could not resolve %d of %d tester ID(s)\n", unresolved, len(ids))
	}

	page.Testers = testers
	return nil
}

// betaTesterUsageTesterIDs extracts the unique beta tester IDs referenced by
// usage metric rows, in first-seen order.
func betaTesterUsageTesterIDs(rows []json.RawMessage) ([]string, error) {
	seen := make(map[string]struct{}, len(rows))
	ids := make([]string, 0, len(rows))
	for i, row := range rows {
		var parsed struct {
			Dimensions struct {
				BetaTesters struct {
					Data *asc.MetricDimensionData `json:"data"`
				} `json:"betaTesters"`
			} `json:"dimensions"`
		}
		if err := json.Unmarshal(row, &parsed); err != nil {
			return nil, fmt.Errorf("parse data[%d] dimensions: %w", i, err)
		}
		if parsed.Dimensions.BetaTesters.Data == nil {
			continue
		}
		id := strings.TrimSpace(parsed.Dimensions.BetaTesters.Data.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

// fetchBetaTestersByIDs fetches beta testers matching the given IDs from
// GET /v1/betaTesters using filter[id], following pagination and applying the
// shared request timeout to every request.
func fetchBetaTestersByIDs(ctx context.Context, d *shared.Deps, client *asc.Client, ids []string) ([]asc.Resource[asc.BetaTesterAttributes], error) {
	firstPage, err := func() (*asc.BetaTestersResponse, error) {
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		return client.GetBetaTesters(requestCtx, "", asc.WithBetaTestersIDs(ids), asc.Limit[asc.BetaTestersOption](200))
	}()
	if err != nil {
		return nil, err
	}

	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		return client.GetBetaTesters(requestCtx, "", asc.NextURL[asc.BetaTestersOption](nextURL))
	})
	if err != nil {
		return nil, err
	}

	testers, ok := all.(*asc.BetaTestersResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected beta testers response type %T", all)
	}
	return testers.Data, nil
}
