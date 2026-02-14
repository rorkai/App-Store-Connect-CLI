package testflight

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type betaTesterUsagesPage struct {
	Data  []json.RawMessage `json:"data"`
	Links asc.Links         `json:"links"`
	Meta  json.RawMessage   `json:"meta,omitempty"`
}

// TestFlightMetricsBetaTesterUsagesCommand fetches app-level beta tester usage metrics.
func TestFlightMetricsBetaTesterUsagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics beta-tester-usages", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	period := fs.String("period", "", "Reporting period: "+strings.Join(betaTesterUsagePeriodList(), ", "))
	groupBy := fs.String("group-by", "betaTesters", "Group results by dimension (betaTesters)")
	filterTester := fs.String("filter-tester", "", "Filter by beta tester ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "beta-tester-usages",
		ShortUsage: "asc testflight metrics beta-tester-usages --app \"APP_ID\" [flags]",
		ShortHelp:  "Fetch TestFlight beta tester usage metrics for an app.",
		LongHelp: `Fetch TestFlight beta tester usage metrics for an app.

Requires either --group-by or --filter-tester (or both).

Examples:
  asc testflight metrics beta-tester-usages --app "APP_ID"
  asc testflight metrics beta-tester-usages --app "APP_ID" --period "P30D"
  asc testflight metrics beta-tester-usages --app "APP_ID" --filter-tester "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
			}

			periodValue, err := normalizeBetaTesterUsagePeriod(*period)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextValue := strings.TrimSpace(*next)
			if nextValue == "" && resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterUsagesOption{
				asc.WithBetaTesterUsagesLimit(*limit),
				asc.WithBetaTesterUsagesNextURL(*next),
				asc.WithBetaTesterUsagesPeriod(periodValue),
				asc.WithBetaTesterUsagesGroupBy(*groupBy),
				asc.WithBetaTesterUsagesFilterBetaTesters(*filterTester),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaTesterUsagesLimit(200))
				firstPage, err := client.GetAppBetaTesterUsagesMetrics(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight metrics beta-tester-usages: failed to fetch: %w", err)
				}

				combined, err := paginateBetaTesterUsages(requestCtx, client, resolvedAppID, firstPage)
				if err != nil {
					return fmt.Errorf("testflight metrics beta-tester-usages: %w", err)
				}

				return shared.PrintOutput(combined, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppBetaTesterUsagesMetrics(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("testflight metrics beta-tester-usages: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func paginateBetaTesterUsages(ctx context.Context, client *asc.Client, appID string, firstPage *asc.BetaTesterUsagesResponse) (*betaTesterUsagesPage, error) {
	if firstPage == nil {
		return nil, nil
	}

	combined := &betaTesterUsagesPage{}
	seenNext := make(map[string]struct{})
	pageNumber := 1
	current := firstPage

	for {
		parsed, err := parseBetaTesterUsagesPage(current.Data)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", pageNumber, err)
		}

		combined.Data = append(combined.Data, parsed.Data...)
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

		nextPage, err := client.GetAppBetaTesterUsagesMetrics(ctx, appID, asc.WithBetaTesterUsagesNextURL(next))
		if err != nil {
			return combined, fmt.Errorf("page %d: %w", pageNumber, err)
		}
		current = nextPage
	}

	return combined, nil
}

func parseBetaTesterUsagesPage(data json.RawMessage) (*betaTesterUsagesPage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	var page betaTesterUsagesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &page, nil
}
