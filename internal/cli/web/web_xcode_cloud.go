package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	newCIClientFn = webcore.NewCIClient
	webNowFn      = time.Now
)

// WebXcodeCloudCommand returns the xcode-cloud command group.
func WebXcodeCloudCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "xcode-cloud",
		ShortUsage: "asc web xcode-cloud <subcommand> [flags]",
		ShortHelp:  "[experimental] Xcode Cloud usage and workflow management.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Query Xcode Cloud compute usage (plan quota, monthly/daily breakdowns, products)
using Apple's private CI API. Requires a web session.

` + webWarningText + `

Examples:
  asc web xcode-cloud usage summary --apple-id "user@example.com"
  asc web xcode-cloud usage alert --apple-id "user@example.com" --output table
  asc web xcode-cloud products --apple-id "user@example.com" --output table
  asc web xcode-cloud usage months --apple-id "user@example.com" --output table
  asc web xcode-cloud usage months --product-ids "UUID" --apple-id "user@example.com" --output table
  asc web xcode-cloud usage days --product-ids "UUID" --apple-id "user@example.com"
  asc web xcode-cloud usage workflows --product-id "UUID" --apple-id "user@example.com" --output table
  asc web xcode-cloud workflows create --product-id "UUID" --file ./workflow.json --apple-id "user@example.com"
  asc web xcode-cloud workflows describe --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"
  asc web xcode-cloud workflows options product-config --product-id "UUID" --apple-id "user@example.com"
  asc web xcode-cloud workflows edit --product-id "UUID" --workflow-id "WF-UUID" --patch-file ./workflow.patch.json --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared list --product-id "UUID" --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_VAR --value hello --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudUsageCommand(),
			webXcodeCloudProductsCommand(),
			webXcodeCloudWorkflowsCommand(),
			webXcodeCloudEnvVarsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func webXcodeCloudUsageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud usage", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "usage",
		ShortUsage: "asc web xcode-cloud usage <subcommand> [flags]",
		ShortHelp:  "[experimental] Xcode Cloud usage queries.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Query Xcode Cloud compute usage: plan summary, monthly history, daily breakdown, per-workflow usage.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudUsageSummaryCommand(),
			webXcodeCloudUsageAlertCommand(),
			webXcodeCloudUsageMonthsCommand(),
			webXcodeCloudUsageDaysCommand(),
			webXcodeCloudUsageWorkflowsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func webXcodeCloudUsageSummaryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud usage summary", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "summary",
		ShortUsage: "asc web xcode-cloud usage summary [flags]",
		ShortHelp:  "[experimental] Show Xcode Cloud plan quota.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show current Xcode Cloud plan usage: used/available/total compute minutes and reset date.

` + webWarningText + `

Examples:
  asc web xcode-cloud usage summary --apple-id "user@example.com"
  asc web xcode-cloud usage summary --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud usage summary failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result, err := withWebSpinnerValue("Loading Xcode Cloud usage summary", func() (*webcore.CIUsageSummary, error) {
				return client.GetCIUsageSummary(requestCtx, teamID)
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud usage summary")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIUsageSummaryTable(result) },
				func() error { return renderCIUsageSummaryMarkdown(result) },
			)
		},
	}
}

func webXcodeCloudUsageMonthsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud usage months", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	now := webNowFn()
	defaultEndMonth := int(now.Month())
	defaultEndYear := now.Year()
	startOfWindow := now.AddDate(0, -11, 0)
	defaultStartMonth := int(startOfWindow.Month())
	defaultStartYear := startOfWindow.Year()

	startMonth := fs.Int("start-month", defaultStartMonth, "Start month (1-12)")
	startYear := fs.Int("start-year", defaultStartYear, "Start year")
	endMonth := fs.Int("end-month", defaultEndMonth, "End month (1-12)")
	endYear := fs.Int("end-year", defaultEndYear, "End year")
	productIDs := fs.String("product-ids", "", "Comma-separated Xcode Cloud product IDs to filter (optional)")

	return &ffcli.Command{
		Name:       "months",
		ShortUsage: "asc web xcode-cloud usage months [flags]",
		ShortHelp:  "[experimental] Show monthly Xcode Cloud usage.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show monthly Xcode Cloud compute usage with per-product breakdown.
Defaults to the last 12 months. Use --product-ids to filter the product breakdown.

` + webWarningText + `

Examples:
  asc web xcode-cloud usage months --apple-id "user@example.com"
  asc web xcode-cloud usage months --apple-id "user@example.com" --start-month 1 --start-year 2025 --output table
  asc web xcode-cloud usage months --product-ids "UUID,OTHER_UUID" --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *startMonth < 1 || *startMonth > 12 {
				fmt.Fprintln(os.Stderr, "Error: --start-month must be between 1 and 12")
				return flag.ErrHelp
			}
			if *endMonth < 1 || *endMonth > 12 {
				fmt.Fprintln(os.Stderr, "Error: --end-month must be between 1 and 12")
				return flag.ErrHelp
			}
			requestedProductIDs, err := parseProductIDs(*productIDs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud usage months failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			var result *webcore.CIUsageMonths
			planTotal := 0
			err = withWebSpinner("Loading Xcode Cloud monthly usage", func() error {
				var err error
				result, err = client.GetCIUsageMonths(requestCtx, teamID, *startMonth, *startYear, *endMonth, *endYear)
				if err != nil {
					return err
				}
				if len(requestedProductIDs) > 0 {
					result.ProductUsage = filterProductUsageByIDs(result.ProductUsage, requestedProductIDs)
				}
				switch shared.NormalizeOutputFormat(*output.Output) {
				case "table", "markdown":
					summary, err := client.GetCIUsageSummary(requestCtx, teamID)
					if err == nil && summary != nil {
						planTotal = summary.Plan.Total
					}
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud usage months")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIUsageMonthsTable(result, planTotal) },
				func() error { return renderCIUsageMonthsMarkdown(result, planTotal) },
			)
		},
	}
}

func webXcodeCloudUsageDaysCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud usage days", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	now := webNowFn()
	defaultEnd := now.Format("2006-01-02")
	defaultStart := now.AddDate(0, 0, -30).Format("2006-01-02")

	productIDs := fs.String("product-ids", "", "Comma-separated Xcode Cloud product IDs (required)")
	start := fs.String("start", defaultStart, "Start date (YYYY-MM-DD)")
	end := fs.String("end", defaultEnd, "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "days",
		ShortUsage: "asc web xcode-cloud usage days --product-ids IDS [flags]",
		ShortHelp:  "[experimental] Show daily Xcode Cloud usage for products.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show daily Xcode Cloud compute usage for one or more products with per-workflow breakdown.
The first product ID drives the daily/workflow tables; all product IDs are shown in the scope comparison table.
Defaults to the last 30 days.

` + webWarningText + `

Examples:
  asc web xcode-cloud usage days --product-ids "UUID" --apple-id "user@example.com"
  asc web xcode-cloud usage days --product-ids "UUID" --start 2025-01-01 --end 2025-01-31 --apple-id "user@example.com" --output table
  asc web xcode-cloud usage days --product-ids "UUID,OTHER_ID,ANOTHER_ID" --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestedProductIDs, err := parseProductIDs(*productIDs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}
			if len(requestedProductIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --product-ids is required")
				return flag.ErrHelp
			}
			primaryProductID := requestedProductIDs[0]
			if err := validateDateFlag("--start", *start); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}
			if err := validateDateFlag("--end", *end); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud usage days failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			var result *webcore.CIUsageDays
			var overall *webcore.CIUsageDays
			productNames := map[string]string{}
			planTotal := 0
			err = withWebSpinner("Loading Xcode Cloud daily usage", func() error {
				var err error
				result, err = client.GetCIUsageDays(requestCtx, teamID, primaryProductID, *start, *end)
				if err != nil {
					return err
				}
				switch shared.NormalizeOutputFormat(*output.Output) {
				case "table", "markdown":
					overall, _ = client.GetCIUsageDaysOverall(requestCtx, teamID, *start, *end)
					summary, err := client.GetCIUsageSummary(requestCtx, teamID)
					if err == nil && summary != nil {
						planTotal = summary.Plan.Total
					}
					products, err := client.ListCIProducts(requestCtx, teamID)
					if err == nil {
						productNames = buildProductNameByID(products)
					}
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud usage days")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					return renderCIUsageDaysTable(
						result,
						overall,
						requestedProductIDs,
						productNames,
						planTotal,
					)
				},
				func() error {
					return renderCIUsageDaysMarkdown(
						result,
						overall,
						requestedProductIDs,
						productNames,
						planTotal,
					)
				},
			)
		},
	}
}

// CIWorkflowsResult is the output type for the workflows command.
// It wraps the workflow usage data with product context for clean JSON output.
type CIWorkflowsResult struct {
	ProductID string                    `json:"product_id"`
	Start     string                    `json:"start"`
	End       string                    `json:"end"`
	Workflows []webcore.CIWorkflowUsage `json:"workflows"`
}

func webXcodeCloudUsageWorkflowsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud usage workflows", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	now := webNowFn()
	defaultEnd := now.Format("2006-01-02")
	defaultStart := now.AddDate(0, 0, -30).Format("2006-01-02")

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Specific workflow ID to drill into (optional)")
	start := fs.String("start", defaultStart, "Start date (YYYY-MM-DD)")
	end := fs.String("end", defaultEnd, "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "workflows",
		ShortUsage: "asc web xcode-cloud usage workflows --product-id ID [flags]",
		ShortHelp:  "[experimental] Show per-workflow Xcode Cloud usage.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show Xcode Cloud compute usage broken down by workflow for a product.
Without --workflow-id, lists all workflows and their usage.
With --workflow-id, shows daily breakdown for that specific workflow.
Defaults to the last 30 days.

` + webWarningText + `

Examples:
  asc web xcode-cloud usage workflows --product-id "UUID" --apple-id "user@example.com" --output table
  asc web xcode-cloud usage workflows --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}
			if err := validateDateFlag("--start", *start); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}
			if err := validateDateFlag("--end", *end); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud usage workflows failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			var result *webcore.CIUsageDays
			err = withWebSpinner("Loading Xcode Cloud workflow usage", func() error {
				var err error
				result, err = client.GetCIUsageDays(requestCtx, teamID, pid, *start, *end)
				if err != nil {
					return err
				}

				// Resolve workflow names from the workflows endpoint.
				wfNames := buildWorkflowNameByID(requestCtx, client, teamID, pid)
				populateWorkflowNames(result.WorkflowUsage, wfNames)
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud usage workflows")
			}

			wfID := strings.TrimSpace(*workflowID)
			if wfID != "" {
				// Drill into a specific workflow
				wf := findWorkflowByID(result.WorkflowUsage, wfID)
				if wf == nil {
					return fmt.Errorf("workflow %q not found in product %q", wfID, pid)
				}
				return shared.PrintOutputWithRenderers(
					wf,
					*output.Output,
					*output.Pretty,
					func() error { return renderCIWorkflowDetailTable(wf) },
					func() error { return renderCIWorkflowDetailMarkdown(wf) },
				)
			}

			// List all workflows
			out := &CIWorkflowsResult{
				ProductID: pid,
				Start:     *start,
				End:       *end,
				Workflows: result.WorkflowUsage,
			}
			planTotal := 0
			switch shared.NormalizeOutputFormat(*output.Output) {
			case "table", "markdown":
				summary, _ := withWebSpinnerValue("Loading Xcode Cloud plan summary", func() (*webcore.CIUsageSummary, error) {
					return client.GetCIUsageSummary(requestCtx, teamID)
				})
				if summary != nil {
					planTotal = summary.Plan.Total
				}
			}
			return shared.PrintOutputWithRenderers(
				out,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIWorkflowsListTable(out, planTotal) },
				func() error { return renderCIWorkflowsListMarkdown(out, planTotal) },
			)
		},
	}
}

func buildWorkflowNameByID(ctx context.Context, client *webcore.Client, teamID, productID string) map[string]string {
	names := map[string]string{}
	workflows, err := client.ListCIWorkflows(ctx, teamID, productID)
	if err != nil || workflows == nil {
		return names
	}
	for _, wf := range workflows.Items {
		canonical := strings.ToLower(strings.TrimSpace(wf.ID))
		name := strings.TrimSpace(wf.Content.Name)
		if canonical != "" && name != "" {
			names[canonical] = name
		}
	}
	return names
}

func populateWorkflowNames(workflows []webcore.CIWorkflowUsage, names map[string]string) {
	for i := range workflows {
		if strings.TrimSpace(workflows[i].WorkflowName) != "" {
			continue
		}
		canonical := strings.ToLower(strings.TrimSpace(workflows[i].WorkflowID))
		if name, ok := names[canonical]; ok {
			workflows[i].WorkflowName = name
		}
	}
}

func findWorkflowByID(workflows []webcore.CIWorkflowUsage, id string) *webcore.CIWorkflowUsage {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for i := range workflows {
		if strings.EqualFold(strings.TrimSpace(workflows[i].WorkflowID), id) {
			return &workflows[i]
		}
	}
	return nil
}

func renderCIWorkflowsListTable(result *CIWorkflowsResult, planTotal int) error {
	if result == nil || len(result.Workflows) == 0 {
		fmt.Println("No workflow usage found.")
		return nil
	}
	maxMinutes := maxWorkflowUsageMinutes(result.Workflows)
	fmt.Printf("Product: %s\n", result.ProductID)
	fmt.Printf("Range: %s to %s\n", result.Start, result.End)
	fmt.Printf("Workflows: %d\n\n", len(result.Workflows))
	asc.RenderTable(
		[]string{"Workflow ID", "Workflow Name", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar"},
		buildCIWorkflowUsageRows(result.Workflows, maxMinutes),
	)
	if planTotal > 0 {
		totalMinutes := 0
		for _, wf := range result.Workflows {
			m, _ := normalizeWorkflowUsage(wf)
			totalMinutes += m
		}
		fmt.Printf("\nProduct total: %s\n", formatUsageBarWithValues(totalMinutes, planTotal))
	}
	return nil
}

func renderCIWorkflowsListMarkdown(result *CIWorkflowsResult, planTotal int) error {
	if result == nil || len(result.Workflows) == 0 {
		fmt.Println("No workflow usage found.")
		return nil
	}
	maxMinutes := maxWorkflowUsageMinutes(result.Workflows)
	fmt.Printf("**Product:** %s\n\n", result.ProductID)
	fmt.Printf("**Range:** %s to %s\n\n", result.Start, result.End)
	fmt.Printf("**Workflows:** %d\n\n", len(result.Workflows))
	asc.RenderMarkdown(
		[]string{"Workflow ID", "Workflow Name", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar"},
		buildCIWorkflowUsageRows(result.Workflows, maxMinutes),
	)
	if planTotal > 0 {
		totalMinutes := 0
		for _, wf := range result.Workflows {
			m, _ := normalizeWorkflowUsage(wf)
			totalMinutes += m
		}
		fmt.Printf("\n**Product total:** %s\n", formatUsageBarWithValues(totalMinutes, planTotal))
	}
	return nil
}

func renderCIWorkflowDetailTable(wf *webcore.CIWorkflowUsage) error {
	if wf == nil {
		return nil
	}
	minutes, builds := normalizeWorkflowUsage(*wf)
	maxDayMinutes := maxDayUsageMinutes(wf.Usage)

	fmt.Printf("Workflow: %s (%s)\n", valueOrNA(wf.WorkflowName), wf.WorkflowID)
	fmt.Printf("Current: %d minutes, %d builds\n", minutes, builds)
	fmt.Printf("Previous: %d minutes, %d builds\n\n", wf.PreviousUsageInMinutes, wf.PreviousNumberOfBuilds)

	if len(wf.Usage) == 0 {
		fmt.Println("No daily usage data.")
		return nil
	}
	asc.RenderTable(
		[]string{"Date", "Minutes", "Builds", "Usage Bar"},
		buildCIDayUsageRows(wf.Usage, maxDayMinutes),
	)
	return nil
}

func renderCIWorkflowDetailMarkdown(wf *webcore.CIWorkflowUsage) error {
	if wf == nil {
		return nil
	}
	minutes, builds := normalizeWorkflowUsage(*wf)
	maxDayMinutes := maxDayUsageMinutes(wf.Usage)

	fmt.Printf("**Workflow:** %s (%s)\n\n", valueOrNA(wf.WorkflowName), wf.WorkflowID)
	fmt.Printf("**Current:** %d minutes, %d builds\n\n", minutes, builds)
	fmt.Printf("**Previous:** %d minutes, %d builds\n\n", wf.PreviousUsageInMinutes, wf.PreviousNumberOfBuilds)

	if len(wf.Usage) == 0 {
		fmt.Println("No daily usage data.")
		return nil
	}
	asc.RenderMarkdown(
		[]string{"Date", "Minutes", "Builds", "Usage Bar"},
		buildCIDayUsageRows(wf.Usage, maxDayMinutes),
	)
	return nil
}

func webXcodeCloudProductsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud products", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "products",
		ShortUsage: "asc web xcode-cloud products [flags]",
		ShortHelp:  "[experimental] List Xcode Cloud products.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

List Xcode Cloud products (apps) for the authenticated team.
Use the product IDs with 'usage days' for per-product daily breakdowns.

` + webWarningText + `

Examples:
  asc web xcode-cloud products --apple-id "user@example.com"
  asc web xcode-cloud products --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud products failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result, err := withWebSpinnerValue("Loading Xcode Cloud products", func() (*webcore.CIProductListResponse, error) {
				return client.ListCIProducts(requestCtx, teamID)
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud products")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderCIProductsTable(result) },
				func() error { return renderCIProductsMarkdown(result) },
			)
		},
	}
}

func renderCIUsageSummaryTable(result *webcore.CIUsageSummary) error {
	asc.RenderTable(
		[]string{"Plan", "Usage Bar", "Used", "Available", "Total", "Reset Date", "Reset Date Time", "Manage URL"},
		buildCIUsageSummaryRows(result),
	)
	return nil
}

func renderCIUsageSummaryMarkdown(result *webcore.CIUsageSummary) error {
	asc.RenderMarkdown(
		[]string{"Plan", "Usage Bar", "Used", "Available", "Total", "Reset Date", "Reset Date Time", "Manage URL"},
		buildCIUsageSummaryRows(result),
	)
	return nil
}

func buildCIUsageSummaryRows(result *webcore.CIUsageSummary) [][]string {
	if result == nil {
		result = &webcore.CIUsageSummary{}
	}
	return [][]string{
		{
			valueOrNA(result.Plan.Name),
			formatUsageBarWithValues(result.Plan.Used, result.Plan.Total),
			fmt.Sprintf("%d", result.Plan.Used),
			fmt.Sprintf("%d", result.Plan.Available),
			fmt.Sprintf("%d", result.Plan.Total),
			valueOrNA(result.Plan.ResetDate),
			valueOrNA(result.Plan.ResetDateTime),
			valueOrNA(result.Links["manage"]),
		},
	}
}

func renderCIUsageMonthsTable(result *webcore.CIUsageMonths, planTotal int) error {
	if result == nil {
		result = &webcore.CIUsageMonths{}
	}
	maxMonthMinutes := maxMonthUsageMinutes(result.Usage)

	fmt.Printf("Range: %s\n", formatCIMonthRange(result.Usage, result.Info))
	fmt.Printf("Current: %d minutes (%d builds), avg30=%d\n", result.Info.Current.Used, result.Info.Current.Builds, result.Info.Current.Average30Days)
	fmt.Printf("Previous: %d minutes (%d builds), avg30=%d\n\n", result.Info.Previous.Used, result.Info.Previous.Builds, result.Info.Previous.Average30Days)
	asc.RenderTable([]string{"Year", "Month", "Minutes", "Builds", "Usage Bar"}, buildCIMonthUsageRows(result.Usage, maxMonthMinutes))

	if len(result.ProductUsage) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Product ID", "Product Name", "Bundle ID", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar (Plan)"},
			buildCIProductUsageSummaryRows(result.ProductUsage, planTotal),
		)
	}

	return nil
}

func renderCIUsageMonthsMarkdown(result *webcore.CIUsageMonths, planTotal int) error {
	if result == nil {
		result = &webcore.CIUsageMonths{}
	}
	maxMonthMinutes := maxMonthUsageMinutes(result.Usage)

	fmt.Printf("**Range:** %s\n\n", formatCIMonthRange(result.Usage, result.Info))
	fmt.Printf("**Current:** %d minutes (%d builds), avg30=%d\n\n", result.Info.Current.Used, result.Info.Current.Builds, result.Info.Current.Average30Days)
	fmt.Printf("**Previous:** %d minutes (%d builds), avg30=%d\n\n", result.Info.Previous.Used, result.Info.Previous.Builds, result.Info.Previous.Average30Days)
	asc.RenderMarkdown([]string{"Year", "Month", "Minutes", "Builds", "Usage Bar"}, buildCIMonthUsageRows(result.Usage, maxMonthMinutes))

	if len(result.ProductUsage) > 0 {
		fmt.Println()
		asc.RenderMarkdown(
			[]string{"Product ID", "Product Name", "Bundle ID", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar (Plan)"},
			buildCIProductUsageSummaryRows(result.ProductUsage, planTotal),
		)
	}

	return nil
}

func buildCIMonthUsageRows(usage []webcore.CIMonthUsage, maxMinutes int) [][]string {
	rows := make([][]string, 0, len(usage))
	for _, monthUsage := range usage {
		rows = append(rows, []string{
			fmt.Sprintf("%d", monthUsage.Year),
			fmt.Sprintf("%d", monthUsage.Month),
			fmt.Sprintf("%d", monthUsage.Duration),
			fmt.Sprintf("%d", monthUsage.NumberOfBuilds),
			formatUsageBar(monthUsage.Duration, maxMinutes),
		})
	}
	return rows
}

func buildCIProductUsageSummaryRows(productUsage []webcore.CIProductUsage, planTotal int) [][]string {
	rows := make([][]string, 0)
	for _, product := range productUsage {
		minutes, builds := normalizeProductUsage(product)
		rows = append(rows, []string{
			valueOrNA(product.ProductID),
			valueOrNA(product.ProductName),
			valueOrNA(product.BundleID),
			fmt.Sprintf("%d", minutes),
			fmt.Sprintf("%d", builds),
			fmt.Sprintf("%d", product.PreviousUsageInMinutes),
			fmt.Sprintf("%d", product.PreviousNumberOfBuilds),
			formatUsageBarWithValues(minutes, planTotal),
		})
	}
	return rows
}

func filterProductUsageByIDs(productUsage []webcore.CIProductUsage, productIDs []string) []webcore.CIProductUsage {
	if len(productIDs) == 0 {
		return productUsage
	}
	wanted := map[string]struct{}{}
	for _, id := range productIDs {
		wanted[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	}
	filtered := make([]webcore.CIProductUsage, 0, len(productIDs))
	for _, pu := range productUsage {
		if _, ok := wanted[strings.ToLower(strings.TrimSpace(pu.ProductID))]; ok {
			filtered = append(filtered, pu)
		}
	}
	return filtered
}

func renderCIUsageDaysTable(
	result, overall *webcore.CIUsageDays,
	productIDs []string,
	productNames map[string]string,
	planTotal int,
) error {
	hasOverall := overall != nil
	if result == nil {
		result = &webcore.CIUsageDays{}
	}
	maxDayMinutes := maxDayUsageMinutes(result.Usage)
	maxWorkflowMinutes := maxWorkflowUsageMinutes(result.WorkflowUsage)
	overallCurrent := webcore.CIUsageInfoCurrent{}
	overallPrevious := webcore.CIUsageInfoCurrent{}
	if hasOverall {
		overallCurrent = overall.Info.Current
		overallPrevious = overall.Info.Previous
	}

	fmt.Printf("Range: %s\n", formatCIDayRange(result.Usage, result.Info))
	if hasOverall {
		fmt.Printf("Overall current: %d minutes (%d builds), avg30=%d\n", overallCurrent.Used, overallCurrent.Builds, overallCurrent.Average30Days)
		fmt.Printf("Overall previous: %d minutes (%d builds), avg30=%d\n\n", overallPrevious.Used, overallPrevious.Builds, overallPrevious.Average30Days)
	} else {
		fmt.Printf("Overall usage unavailable; showing selected product scope only.\n\n")
	}
	asc.RenderTable(
		[]string{"Scope", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar (Plan)"},
		buildCIUsageScopeRows(
			result,
			overall,
			productIDs,
			productNames,
			planTotal,
		),
	)
	fmt.Println()
	asc.RenderTable([]string{"Date", "Minutes", "Builds", "Usage Bar"}, buildCIDayUsageRows(result.Usage, maxDayMinutes))

	if len(result.WorkflowUsage) > 0 {
		fmt.Println()
		asc.RenderTable(
			[]string{"Workflow ID", "Workflow Name", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar"},
			buildCIWorkflowUsageRows(result.WorkflowUsage, maxWorkflowMinutes),
		)
	}

	return nil
}

func renderCIUsageDaysMarkdown(
	result, overall *webcore.CIUsageDays,
	productIDs []string,
	productNames map[string]string,
	planTotal int,
) error {
	hasOverall := overall != nil
	if result == nil {
		result = &webcore.CIUsageDays{}
	}
	maxDayMinutes := maxDayUsageMinutes(result.Usage)
	maxWorkflowMinutes := maxWorkflowUsageMinutes(result.WorkflowUsage)
	overallCurrent := webcore.CIUsageInfoCurrent{}
	overallPrevious := webcore.CIUsageInfoCurrent{}
	if hasOverall {
		overallCurrent = overall.Info.Current
		overallPrevious = overall.Info.Previous
	}

	fmt.Printf("**Range:** %s\n\n", formatCIDayRange(result.Usage, result.Info))
	if hasOverall {
		fmt.Printf("**Overall current:** %d minutes (%d builds), avg30=%d\n\n", overallCurrent.Used, overallCurrent.Builds, overallCurrent.Average30Days)
		fmt.Printf("**Overall previous:** %d minutes (%d builds), avg30=%d\n\n", overallPrevious.Used, overallPrevious.Builds, overallPrevious.Average30Days)
	} else {
		fmt.Printf("**Overall usage unavailable; showing selected product scope only.**\n\n")
	}
	asc.RenderMarkdown(
		[]string{"Scope", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar (Plan)"},
		buildCIUsageScopeRows(
			result,
			overall,
			productIDs,
			productNames,
			planTotal,
		),
	)
	fmt.Println()
	asc.RenderMarkdown([]string{"Date", "Minutes", "Builds", "Usage Bar"}, buildCIDayUsageRows(result.Usage, maxDayMinutes))

	if len(result.WorkflowUsage) > 0 {
		fmt.Println()
		asc.RenderMarkdown(
			[]string{"Workflow ID", "Workflow Name", "Minutes", "Builds", "Prev Minutes", "Prev Builds", "Usage Bar"},
			buildCIWorkflowUsageRows(result.WorkflowUsage, maxWorkflowMinutes),
		)
	}

	return nil
}

func buildCIDayUsageRows(usage []webcore.CIDayUsage, maxMinutes int) [][]string {
	rows := make([][]string, 0, len(usage))
	for _, dayUsage := range usage {
		rows = append(rows, []string{
			valueOrNA(dayUsage.Date),
			fmt.Sprintf("%d", dayUsage.Duration),
			fmt.Sprintf("%d", dayUsage.NumberOfBuilds),
			formatUsageBar(dayUsage.Duration, maxMinutes),
		})
	}
	return rows
}

func buildCIWorkflowUsageRows(workflowUsage []webcore.CIWorkflowUsage, maxMinutes int) [][]string {
	rows := make([][]string, 0)
	for _, workflow := range workflowUsage {
		minutes, builds := normalizeWorkflowUsage(workflow)
		rows = append(rows, []string{
			valueOrNA(workflow.WorkflowID),
			valueOrNA(workflow.WorkflowName),
			fmt.Sprintf("%d", minutes),
			fmt.Sprintf("%d", builds),
			fmt.Sprintf("%d", workflow.PreviousUsageInMinutes),
			fmt.Sprintf("%d", workflow.PreviousNumberOfBuilds),
			formatUsageBar(minutes, maxMinutes),
		})
	}
	return rows
}

func renderCIProductsTable(result *webcore.CIProductListResponse) error {
	asc.RenderTable([]string{"Product ID", "Name", "Bundle ID", "Type"}, buildCIProductRows(result))
	return nil
}

func renderCIProductsMarkdown(result *webcore.CIProductListResponse) error {
	asc.RenderMarkdown([]string{"Product ID", "Name", "Bundle ID", "Type"}, buildCIProductRows(result))
	return nil
}

func buildCIProductRows(result *webcore.CIProductListResponse) [][]string {
	if result == nil {
		result = &webcore.CIProductListResponse{}
	}
	rows := make([][]string, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, []string{
			valueOrNA(item.ID),
			valueOrNA(item.Name),
			valueOrNA(item.BundleID),
			valueOrNA(item.Type),
		})
	}
	return rows
}

func formatCIMonthRange(usage []webcore.CIMonthUsage, info webcore.CIUsageInfo) string {
	if info.StartMonth < 1 || info.StartYear < 1 || info.EndMonth < 1 || info.EndYear < 1 {
		if len(usage) > 0 {
			first := usage[0]
			last := usage[len(usage)-1]
			return fmt.Sprintf("%04d-%02d to %04d-%02d", first.Year, first.Month, last.Year, last.Month)
		}
		return "n/a"
	}
	return fmt.Sprintf("%04d-%02d to %04d-%02d", info.StartYear, info.StartMonth, info.EndYear, info.EndMonth)
}

func formatCIDayRange(usage []webcore.CIDayUsage, info webcore.CIUsageInfo) string {
	if info.StartMonth > 0 && info.StartYear > 0 && info.EndMonth > 0 && info.EndYear > 0 {
		return fmt.Sprintf("%04d-%02d to %04d-%02d", info.StartYear, info.StartMonth, info.EndYear, info.EndMonth)
	}
	if len(usage) == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%s to %s", valueOrNA(usage[0].Date), valueOrNA(usage[len(usage)-1].Date))
}

func parseProductIDs(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	seen := map[string]struct{}{}
	ids := make([]string, 0)
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("--product-ids must be a comma-separated list of non-empty product IDs")
		}
		canonical := strings.ToLower(trimmed)
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		ids = append(ids, trimmed)
	}
	return ids, nil
}

func buildProductNameByID(products *webcore.CIProductListResponse) map[string]string {
	names := map[string]string{}
	if products == nil {
		return names
	}
	for _, product := range products.Items {
		canonical := strings.ToLower(strings.TrimSpace(product.ID))
		name := strings.TrimSpace(product.Name)
		if canonical == "" || name == "" {
			continue
		}
		names[canonical] = name
	}
	return names
}

func displayNameForProductID(productID string, names map[string]string) string {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return "n/a"
	}
	if name := strings.TrimSpace(names[strings.ToLower(productID)]); name != "" {
		return name
	}
	return productID
}

func resolveProductUsageSummary(
	productID, primaryProductID string,
	app, overall *webcore.CIUsageDays,
) (webcore.CIUsageInfoCurrent, webcore.CIUsageInfoCurrent) {
	productID = strings.TrimSpace(productID)
	primaryProductID = strings.TrimSpace(primaryProductID)
	if overall != nil {
		if productUsage := findCIProductUsageByID(overall.ProductUsage, productID); productUsage != nil {
			minutes, builds := normalizeProductUsage(*productUsage)
			current := webcore.CIUsageInfoCurrent{
				Used:   minutes,
				Builds: builds,
			}
			previous := webcore.CIUsageInfoCurrent{
				Used:   productUsage.PreviousUsageInMinutes,
				Builds: productUsage.PreviousNumberOfBuilds,
			}
			return current, previous
		}
	}
	if strings.EqualFold(productID, primaryProductID) && app != nil {
		return app.Info.Current, app.Info.Previous
	}
	return webcore.CIUsageInfoCurrent{}, webcore.CIUsageInfoCurrent{}
}

func findCIProductUsageByID(productUsage []webcore.CIProductUsage, productID string) *webcore.CIProductUsage {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil
	}
	for i := range productUsage {
		if strings.EqualFold(strings.TrimSpace(productUsage[i].ProductID), productID) {
			return &productUsage[i]
		}
	}
	return nil
}

func buildCIUsageScopeRows(
	app, overall *webcore.CIUsageDays,
	productIDs []string,
	productNames map[string]string,
	planTotal int,
) [][]string {
	hasOverall := overall != nil
	if overall == nil {
		overall = &webcore.CIUsageDays{}
	}
	if app == nil {
		app = &webcore.CIUsageDays{}
	}
	if !hasOverall && len(productIDs) > 1 {
		// Without overall data we cannot resolve additional products reliably.
		productIDs = productIDs[:1]
	}
	primaryProductID := ""
	if len(productIDs) > 0 {
		primaryProductID = productIDs[0]
	}
	type productScope struct {
		Label    string
		Current  webcore.CIUsageInfoCurrent
		Previous webcore.CIUsageInfoCurrent
	}
	scopes := make([]productScope, 0, len(productIDs))
	for _, productID := range productIDs {
		productID = strings.TrimSpace(productID)
		if productID == "" {
			continue
		}
		current, previous := resolveProductUsageSummary(productID, primaryProductID, app, overall)
		scopes = append(scopes, productScope{
			Label:    displayNameForProductID(productID, productNames),
			Current:  current,
			Previous: previous,
		})
	}

	overallCurrent := overall.Info.Current
	overallPrevious := overall.Info.Previous

	absoluteTotal := planTotal
	if absoluteTotal <= 0 {
		absoluteTotal = overallCurrent.Used
		for _, scope := range scopes {
			if scope.Current.Used > absoluteTotal {
				absoluteTotal = scope.Current.Used
			}
		}
	}

	rows := make([][]string, 0, len(scopes)+1)
	for _, scope := range scopes {
		rows = append(rows, []string{
			scope.Label,
			fmt.Sprintf("%d", scope.Current.Used),
			fmt.Sprintf("%d", scope.Current.Builds),
			fmt.Sprintf("%d", scope.Previous.Used),
			fmt.Sprintf("%d", scope.Previous.Builds),
			formatUsageBarWithValues(scope.Current.Used, absoluteTotal),
		})
	}
	if hasOverall {
		rows = append(rows, []string{
			"Overall Team",
			fmt.Sprintf("%d", overallCurrent.Used),
			fmt.Sprintf("%d", overallCurrent.Builds),
			fmt.Sprintf("%d", overallPrevious.Used),
			fmt.Sprintf("%d", overallPrevious.Builds),
			formatUsageBarWithValues(overallCurrent.Used, absoluteTotal),
		})
	}
	return rows
}

func normalizeProductUsage(product webcore.CIProductUsage) (minutes int, builds int) {
	minutes = product.UsageInMinutes
	builds = product.NumberOfBuilds
	if len(product.Usage) == 0 {
		return minutes, builds
	}
	if minutes == 0 {
		for _, monthUsage := range product.Usage {
			minutes += monthUsage.Duration
		}
	}
	if builds == 0 {
		for _, monthUsage := range product.Usage {
			builds += monthUsage.NumberOfBuilds
		}
	}
	return minutes, builds
}

func normalizeWorkflowUsage(workflow webcore.CIWorkflowUsage) (minutes int, builds int) {
	minutes = workflow.UsageInMinutes
	builds = workflow.NumberOfBuilds
	if len(workflow.Usage) == 0 {
		return minutes, builds
	}
	if minutes == 0 {
		for _, dayUsage := range workflow.Usage {
			minutes += dayUsage.Duration
		}
	}
	if builds == 0 {
		for _, dayUsage := range workflow.Usage {
			builds += dayUsage.NumberOfBuilds
		}
	}
	return minutes, builds
}

func maxMonthUsageMinutes(usage []webcore.CIMonthUsage) int {
	max := 0
	for _, monthUsage := range usage {
		if monthUsage.Duration > max {
			max = monthUsage.Duration
		}
	}
	return max
}

func maxDayUsageMinutes(usage []webcore.CIDayUsage) int {
	max := 0
	for _, dayUsage := range usage {
		if dayUsage.Duration > max {
			max = dayUsage.Duration
		}
	}
	return max
}

func maxWorkflowUsageMinutes(workflows []webcore.CIWorkflowUsage) int {
	max := 0
	for _, workflow := range workflows {
		minutes, _ := normalizeWorkflowUsage(workflow)
		if minutes > max {
			max = minutes
		}
	}
	return max
}

func formatUsageBarWithValues(value, total int) string {
	if total <= 0 {
		return formatUsageBar(value, total)
	}
	return fmt.Sprintf("%s (%d/%dm)", formatUsageBar(value, total), value, total)
}

func formatUsageBar(value, total int) string {
	const barWidth = 16
	if total <= 0 {
		return "[................] n/a"
	}
	if value < 0 {
		value = 0
	}
	if value > total {
		value = total
	}

	percent := (value*100 + total/2) / total
	filled := (value*barWidth + total/2) / total
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	return fmt.Sprintf(
		"[%s%s] %3d%%",
		strings.Repeat("#", filled),
		strings.Repeat(".", barWidth-filled),
		percent,
	)
}

func validateDateFlag(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("%s must be YYYY-MM-DD (got %q)", name, value)
	}
	return nil
}
