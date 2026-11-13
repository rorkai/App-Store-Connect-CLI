package testflight

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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

	testerID := shared.BindResourceIDFlag(fs, "tester-id", "betaTesters", "Beta tester ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID (alias of --tester-id)")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	period := fs.String("period", "", "Reporting period: "+strings.Join(betaTesterUsagePeriodList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc testflight beta-testers metrics --tester-id \"TESTER_ID\" --app \"APP_ID\" [flags]",
		ShortHelp:  "Fetch beta tester usage metrics.",
		LongHelp: `Fetch beta tester usage metrics.

Examples:
  asc testflight beta-testers metrics --tester-id "TESTER_ID" --app "APP_ID"
  asc testflight beta-testers metrics --tester-id "TESTER_ID" --app "APP_ID" --period "P30D"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--limit")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "testflight beta-testers metrics: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(fs, *next, "testflight testers metrics", "period"); err != nil {
				return err
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return shared.WithDiagnostic(
					shared.UsageErrorCtx(ctx, "testflight beta-testers metrics: --tester-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}

			periodValue, err := normalizeBetaTesterUsagePeriod(*period)
			if err != nil {
				return usageErrorFromValidation(ctx, "%v", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextValue := strings.TrimSpace(*next)
			if nextValue == "" && testerValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
				return shared.MissingRequiredUsageError("--tester-id")
			}
			if nextValue == "" && resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-testers metrics: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			return printBetaTesterMetrics(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeBetaTesterUsagePeriod(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, ok := betaTesterUsagePeriods[value]; !ok {
		return "", shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("--period must be one of: %s", strings.Join(betaTesterUsagePeriodList(), ", "))),
			shared.DiagnosticInvalidInput,
			"--period",
		)
	}
	return value, nil
}

func betaTesterUsagePeriodList() []string {
	return []string{"P7D", "P30D", "P90D", "P365D"}
}

// printBetaTesterMetrics renders one tester's usage, whose rows are keyed by
// app rather than tester, so it cannot reuse the app-testers renderer.
func printBetaTesterMetrics(resp *asc.BetaTesterUsagesResponse, format string, pretty bool) error {
	render := func(renderRows func([]string, [][]string) error) func() error {
		return func() error {
			var payload struct {
				Data []struct {
					DataPoints []struct {
						Start  string `json:"start"`
						End    string `json:"end"`
						Values struct {
							SessionCount  *int `json:"sessionCount"`
							CrashCount    *int `json:"crashCount"`
							FeedbackCount *int `json:"feedbackCount"`
						} `json:"values"`
					} `json:"dataPoints"`
					Dimensions struct {
						Apps struct {
							Data *asc.MetricDimensionData `json:"data"`
						} `json:"apps"`
					} `json:"dimensions"`
				} `json:"data"`
			}
			if resp != nil && len(resp.Data) > 0 {
				if err := json.Unmarshal(resp.Data, &payload); err != nil {
					return fmt.Errorf("parse response: %w", err)
				}
			}
			var rows [][]string
			for _, entry := range payload.Data {
				appID := ""
				if entry.Dimensions.Apps.Data != nil {
					appID = entry.Dimensions.Apps.Data.ID
				}
				for _, point := range entry.DataPoints {
					rows = append(rows, []string{appID, point.Start, point.End, formatCount(point.Values.SessionCount), formatCount(point.Values.CrashCount), formatCount(point.Values.FeedbackCount)})
				}
			}
			return renderRows([]string{"App ID", "Start", "End", "Sessions", "Crashes", "Feedback"}, rows)
		}
	}
	return shared.PrintOutputWithRenderers(resp, format, pretty, render(asc.WriteTable), render(asc.WriteMarkdown))
}

func formatCount(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}
