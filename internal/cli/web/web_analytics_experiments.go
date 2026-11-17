package web

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// These views are only used for human-readable output; JSON prints the complete Apple envelope.
type analyticsExperimentListView struct {
	Results []struct {
		TestID    string `json:"testId"`
		TestName  string `json:"testName"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Status    string `json:"status"`
	} `json:"results"`
}

type analyticsExperimentInterval struct {
	Central *float64 `json:"central"`
}

type analyticsExperimentResultsView struct {
	Baseline   string `json:"baseline"`
	Treatments []struct {
		ID                string                       `json:"id"`
		Name              string                       `json:"name"`
		ConversionRate    *analyticsExperimentInterval `json:"conversionRate"`
		Improvement       *analyticsExperimentInterval `json:"improvement"`
		UniqueImpressions *int64                       `json:"uniqueImpressions"`
		Confidence        *float64                     `json:"confidence"`
		Outcome           string                       `json:"outcome"`
	} `json:"treatments"`
}

func newWebAnalyticsExperimentsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "experiments",
		ShortUsage: "asc web analytics product-pages experiments <subcommand> [flags]",
		ShortHelp:  "Read product page optimization experiments and results.",
		LongHelp: `WEB SESSION WORKFLOWS

Read Product Page Optimization analytics using an authenticated Apple web session.
These analytics endpoints are separate from the public experiment management API.

Examples:
  asc web analytics product-pages experiments list --app "123456789"
  asc web analytics product-pages experiments results --experiment "EXPERIMENT_UUID" --output table`,
		FlagSet:     d.NewFlagSet("web analytics product-pages experiments", flag.ExitOnError),
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newWebAnalyticsExperimentsListCommand(d), newWebAnalyticsExperimentResultsCommand(d)},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newWebAnalyticsExperimentsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics product-pages experiments list", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web analytics product-pages experiments list [flags]",
		ShortHelp:  "List product page optimization experiments with analytics data.",
		LongHelp: `WEB SESSION WORKFLOWS

List experiment IDs, names, dates, and lifecycle status from Apple's analytics web API.
JSON preserves Apple's complete response. No date filter or pagination is supported.
`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			resolvedAppID, err := resolveAnalyticsAppIDFlag(d, *appID)
			if err != nil {
				return err
			}
			if strings.IndexFunc(resolvedAppID, func(r rune) bool { return r < '0' || r > '9' }) != -1 {
				return d.UsageError("--app must be a numeric App ID")
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			payload, err := withWebSpinnerValueWith(d, "Loading product page experiments", func() (json.RawMessage, error) { return client.GetAnalyticsExperiments(requestCtx, resolvedAppID) })
			if err != nil {
				return withWebAuthHint(err, "analytics product-pages experiments list")
			}
			var result analyticsExperimentListView
			if err := json.Unmarshal(payload, &result); err != nil {
				return fmt.Errorf("failed to parse experiment list: %w", err)
			}
			rows := make([][]string, 0, len(result.Results))
			for _, experiment := range result.Results {
				rows = append(rows, []string{experiment.TestID, experiment.TestName, experiment.StartDate, experiment.EndDate, experiment.Status})
			}
			return d.PrintOutputRows(payload, *output.Output, *output.Pretty, []string{"Experiment ID", "Name", "Start", "End", "Status"}, rows)
		},
	}
}

func newWebAnalyticsExperimentResultsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics product-pages experiments results", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	experimentID := fs.String("experiment", "", "Analytics experiment UUID (from experiments list)")
	baseline := fs.String("baseline", webcore.AnalyticsExperimentDefaultBaseline, "Baseline: default (original page) or treatment UUID")
	return &ffcli.Command{
		Name:       "results",
		ShortUsage: "asc web analytics product-pages experiments results [flags]",
		ShortHelp:  "Read conversion, improvement, confidence, and daily experiment results.",
		LongHelp: `WEB SESSION WORKFLOWS

Read all results for an experiment using an Apple web session. --experiment is the
analytics testId returned by experiments list. --baseline selects the original
product page (default) or a treatment UUID from the results.

JSON preserves Apple's complete envelope, including interval bounds and daily data.
Table and Markdown summarize treatments and display fractional rates as percentages.
Lifecycle status and statistical outcome are separate; no winner is inferred.
The API returns the experiment's full range, without --start or --end filters.
`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			if strings.TrimSpace(*experimentID) == "" {
				return d.UsageError("--experiment is required")
			}
			id, err := webcore.NormalizeAnalyticsExperimentID(*experimentID)
			if err != nil {
				return d.UsageError("--experiment must be a valid UUID")
			}
			normalizedBaseline, err := webcore.NormalizeAnalyticsExperimentBaseline(*baseline)
			if err != nil {
				return d.UsageError("--baseline must be default or a treatment UUID")
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			payload, err := withWebSpinnerValueWith(d, "Loading product page experiment results", func() (json.RawMessage, error) {
				return client.GetAnalyticsExperimentResults(requestCtx, id, normalizedBaseline)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics product-pages experiments results")
			}
			var result analyticsExperimentResultsView
			if err := json.Unmarshal(payload, &result); err != nil {
				return fmt.Errorf("failed to parse experiment results: %w", err)
			}
			rows := make([][]string, 0, len(result.Treatments))
			for _, treatment := range result.Treatments {
				impressions := "-"
				if treatment.UniqueImpressions != nil {
					impressions = strconv.FormatInt(*treatment.UniqueImpressions, 10)
				}
				improvement, confidence := analyticsExperimentIntervalString(treatment.Improvement), analyticsExperimentPercentString(treatment.Confidence)
				if treatment.ID == result.Baseline {
					improvement, confidence = "-", "-"
				}
				rows = append(rows, []string{treatment.ID, treatment.Name, impressions, analyticsExperimentIntervalString(treatment.ConversionRate), improvement, confidence, treatment.Outcome})
			}
			return d.PrintOutputRows(payload, *output.Output, *output.Pretty, []string{"Treatment ID", "Name", "Unique Impressions", "Conversion Rate", "Improvement", "Confidence", "Outcome"}, rows)
		},
	}
}

func analyticsExperimentIntervalString(interval *analyticsExperimentInterval) string {
	if interval == nil {
		return "-"
	}
	return analyticsExperimentPercentString(interval.Central)
}

func analyticsExperimentPercentString(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", *value*100)
}
