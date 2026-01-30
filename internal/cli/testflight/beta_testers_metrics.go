package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var betaTesterUsagePeriods = map[string]struct{}{
	"P7D":   {},
	"P30D":  {},
	"P90D":  {},
	"P365D": {},
}

// BetaTestersMetricsCommand returns the beta-testers metrics subcommand.
func BetaTestersMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)

	testerID := fs.String("tester-id", "", "Beta tester ID")
	aliasID := fs.String("id", "", "Beta tester ID (alias of --tester-id)")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	period := fs.String("period", "", "Reporting period: "+strings.Join(betaTesterUsagePeriodList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc testflight beta-testers metrics --tester-id \"TESTER_ID\" --app \"APP_ID\" [flags]",
		ShortHelp:  "Fetch beta tester usage metrics.",
		LongHelp: `Fetch beta tester usage metrics.

Examples:
  asc testflight beta-testers metrics --tester-id "TESTER_ID" --app "APP_ID"
  asc testflight beta-testers metrics --tester-id "TESTER_ID" --app "APP_ID" --period "P30D"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight beta-testers metrics: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-testers metrics: %w", err)
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return fmt.Errorf("testflight beta-testers metrics: --tester-id and --id must match")
			}

			periodValue, err := normalizeBetaTesterUsagePeriod(*period)
			if err != nil {
				return fmt.Errorf("testflight beta-testers metrics: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			nextValue := strings.TrimSpace(*next)
			if nextValue == "" && testerValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
				return flag.ErrHelp
			}
			if nextValue == "" && resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-testers metrics: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterUsagesOption{
				asc.WithBetaTesterUsagesLimit(*limit),
				asc.WithBetaTesterUsagesNextURL(*next),
				asc.WithBetaTesterUsagesPeriod(periodValue),
				asc.WithBetaTesterUsagesAppID(resolvedAppID),
			}

			resp, err := client.GetBetaTesterUsagesMetrics(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers metrics: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

func normalizeBetaTesterUsagePeriod(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, ok := betaTesterUsagePeriods[value]; !ok {
		return "", fmt.Errorf("--period must be one of: %s", strings.Join(betaTesterUsagePeriodList(), ", "))
	}
	return value, nil
}

func betaTesterUsagePeriodList() []string {
	return []string{"P7D", "P30D", "P90D", "P365D"}
}
