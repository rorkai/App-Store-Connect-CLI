package capabilities

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	statusCLISupported = "cli-supported"
	statusPartial      = "partial"
	statusClientOnly   = "client-only"
	statusExperimental = "experimental-web"
	statusNotPublicAPI = "not-public-api"
)

var allowedCapabilityStatuses = []string{
	statusCLISupported,
	statusPartial,
	statusClientOnly,
	statusExperimental,
	statusNotPublicAPI,
}

// Report is the deterministic capabilities payload.
type Report struct {
	Summary      Summary      `json:"summary"`
	Capabilities []Capability `json:"capabilities"`
	Sources      []string     `json:"sources"`
}

// Summary contains roll-up counts for quick inspection.
type Summary struct {
	Total               int            `json:"total"`
	SchemaEndpointCount int            `json:"schemaEndpointCount"`
	Statuses            map[string]int `json:"statuses"`
	Areas               map[string]int `json:"areas"`
}

// Capability describes one App Store Connect workflow surface.
type Capability struct {
	Area         string   `json:"area"`
	Capability   string   `json:"capability"`
	Status       string   `json:"status"`
	Commands     []string `json:"commands,omitempty"`
	APIResources []string `json:"apiResources,omitempty"`
	Notes        []string `json:"notes,omitempty"`
	NextAction   string   `json:"nextAction,omitempty"`
}

type capabilityFilter struct {
	statuses map[string]struct{}
	areas    map[string]struct{}
}

// Command returns the capabilities command.
func Command() *ffcli.Command {
	fs := flag.NewFlagSet("capabilities", flag.ExitOnError)
	status := fs.String("status", "", "Filter by status: cli-supported, partial, client-only, experimental-web, not-public-api")
	area := fs.String("area", "", "Filter by area (comma-separated, e.g. release,monetization)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "capabilities",
		ShortUsage: "asc capabilities [flags]",
		ShortHelp:  "Show CLI, API, web-only, and public-API-limited capability coverage.",
		LongHelp: `Show CLI, API, web-only, and public-API-limited capability coverage.

This command answers whether high-value App Store Connect workflows are first-class
CLI commands, partially covered, available only through internal client code, routed
through experimental web-session commands, or blocked by Apple's public API.

Examples:
  asc capabilities
  asc capabilities --status not-public-api --output table
  asc capabilities --area release,monetization --output markdown
  asc capabilities --output json --pretty`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected arguments: %s", strings.Join(args, " "))
			}

			filter, err := parseCapabilityFilter(*status, *area)
			if err != nil {
				return err
			}

			report, err := buildReport(filter)
			if err != nil {
				return fmt.Errorf("capabilities: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				report,
				*output.Output,
				*output.Pretty,
				func() error { return renderTable(report) },
				func() error { return renderMarkdown(report) },
			)
		},
	}
}

func parseCapabilityFilter(statusCSV, areaCSV string) (capabilityFilter, error) {
	filter := capabilityFilter{}

	statuses, err := parseCSVSet(statusCSV)
	if err != nil {
		return filter, shared.UsageErrorf("invalid --status: %s", err)
	}
	if len(statuses) > 0 {
		allowed := make(map[string]struct{}, len(allowedCapabilityStatuses))
		for _, status := range allowedCapabilityStatuses {
			allowed[status] = struct{}{}
		}
		for status := range statuses {
			if _, ok := allowed[status]; !ok {
				return filter, shared.UsageErrorf("invalid --status %q (allowed: %s)", status, strings.Join(allowedCapabilityStatuses, ", "))
			}
		}
		filter.statuses = statuses
	}

	areas, err := parseCSVSet(areaCSV)
	if err != nil {
		return filter, shared.UsageErrorf("invalid --area: %s", err)
	}
	if len(areas) > 0 {
		allowed := knownAreas()
		allowedList := sortedKeys(allowed)
		for area := range areas {
			if _, ok := allowed[area]; !ok {
				return filter, shared.UsageErrorf("invalid --area %q (allowed: %s)", area, strings.Join(allowedList, ", "))
			}
		}
		filter.areas = areas
	}

	return filter, nil
}

func parseCSVSet(raw string) (map[string]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	values := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			return nil, fmt.Errorf("empty value")
		}
		values[value] = struct{}{}
	}
	return values, nil
}

func buildReport(filter capabilityFilter) (Report, error) {
	endpointCount, err := schema.EndpointCount()
	if err != nil {
		return Report{}, err
	}

	entries := filterCapabilities(capabilityRows(), filter)
	summary := Summary{
		Total:               len(entries),
		SchemaEndpointCount: endpointCount,
		Statuses:            make(map[string]int),
		Areas:               make(map[string]int),
	}
	for _, entry := range entries {
		summary.Statuses[entry.Status]++
		summary.Areas[entry.Area]++
	}

	return Report{
		Summary:      summary,
		Capabilities: entries,
		Sources: []string{
			"registered CLI command surface",
			"embedded App Store Connect OpenAPI schema index",
			"curated public-API limitations and experimental web-session surfaces",
		},
	}, nil
}

func filterCapabilities(entries []Capability, filter capabilityFilter) []Capability {
	filtered := make([]Capability, 0, len(entries))
	for _, entry := range entries {
		if len(filter.statuses) > 0 {
			if _, ok := filter.statuses[entry.Status]; !ok {
				continue
			}
		}
		if len(filter.areas) > 0 {
			if _, ok := filter.areas[entry.Area]; !ok {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func knownAreas() map[string]struct{} {
	areas := make(map[string]struct{})
	for _, entry := range capabilityRows() {
		areas[entry.Area] = struct{}{}
	}
	return areas
}

func capabilityRows() []Capability {
	return []Capability{
		{
			Area:       "release",
			Capability: "App Store release submission",
			Status:     statusCLISupported,
			Commands:   []string{"asc publish appstore --submit", "asc review submit", "asc validate"},
			APIResources: []string{
				"reviewSubmissions",
				"reviewSubmissionItems",
				"appStoreVersionSubmissions",
			},
			Notes: []string{"High-level publish flow plus lower-level review submission controls are available."},
		},
		{
			Area:       "release",
			Capability: "Release readiness validation",
			Status:     statusCLISupported,
			Commands:   []string{"asc validate", "asc status", "asc review doctor"},
			Notes:      []string{"Aggregates common blocking App Review and version readiness signals."},
		},
		{
			Area:       "builds",
			Capability: "Build upload and processing tracking",
			Status:     statusCLISupported,
			Commands:   []string{"asc builds upload", "asc builds list", "asc builds info", "asc publish testflight"},
			APIResources: []string{
				"buildUploads",
				"builds",
				"preReleaseVersions",
			},
			Notes: []string{"The CLI wraps Apple's supported upload tooling and build-processing APIs."},
		},
		{
			Area:       "builds",
			Capability: "Direct REST build upload",
			Status:     statusNotPublicAPI,
			Notes:      []string{"Apple does not expose direct IPA upload as a normal public App Store Connect REST endpoint."},
			NextAction: "Use asc builds upload, Xcode, Transporter, or asc publish workflows.",
		},
		{
			Area:         "app-management",
			Capability:   "App creation",
			Status:       statusExperimental,
			Commands:     []string{"asc web apps create"},
			APIResources: []string{"apps"},
			Notes:        []string{"The public apps API manages existing apps; creating app records is not exposed as a public REST operation."},
			NextAction:   "Use App Store Connect web UI, or experimental asc web apps create at your own risk.",
		},
		{
			Area:         "app-management",
			Capability:   "Initial app availability bootstrap",
			Status:       statusClientOnly,
			APIResources: []string{"POST /v2/appAvailabilities", "territoryAvailabilities"},
			Notes:        []string{"The internal client can create appAvailabilityV2 records; stable CLI commands currently update existing availability only."},
			NextAction:   "Candidate command: asc pricing availability create.",
		},
		{
			Area:       "app-management",
			Capability: "App pricing and availability updates",
			Status:     statusPartial,
			Commands:   []string{"asc pricing availability view", "asc pricing availability edit", "asc pricing schedule create"},
			APIResources: []string{
				"appAvailabilities",
				"territoryAvailabilities",
				"appPriceSchedules",
				"appPricePoints",
			},
			Notes: []string{"Existing availability records can be edited; initial bootstrap is tracked separately as client-only."},
		},
		{
			Area:       "metadata",
			Capability: "Metadata and localization sync",
			Status:     statusCLISupported,
			Commands:   []string{"asc metadata init", "asc metadata pull", "asc metadata validate", "asc metadata apply", "asc localizations update"},
			APIResources: []string{
				"appInfos",
				"appInfoLocalizations",
				"appStoreVersionLocalizations",
				"searchKeywords",
			},
		},
		{
			Area:       "metadata",
			Capability: "Screenshots and app previews",
			Status:     statusCLISupported,
			Commands:   []string{"asc screenshots upload", "asc screenshots plan", "asc video-previews upload"},
			APIResources: []string{
				"appScreenshotSets",
				"appScreenshots",
				"appPreviewSets",
				"appPreviews",
			},
		},
		{
			Area:       "metadata",
			Capability: "Custom product pages and experiments",
			Status:     statusCLISupported,
			Commands:   []string{"asc product-pages", "asc versions experiments-v2"},
			APIResources: []string{
				"appCustomProductPages",
				"appStoreVersionExperiments",
				"appStoreVersionExperimentTreatments",
			},
		},
		{
			Area:       "privacy",
			Capability: "App privacy data-use declarations",
			Status:     statusExperimental,
			Commands:   []string{"asc web privacy"},
			Notes:      []string{"App privacy data-use resources are not present in the embedded public OpenAPI snapshot."},
			NextAction: "Use App Store Connect web UI, or experimental asc web privacy when web-session risk is acceptable.",
		},
		{
			Area:       "monetization",
			Capability: "Subscriptions and in-app purchases",
			Status:     statusCLISupported,
			Commands:   []string{"asc subscriptions setup", "asc iap setup", "asc subscriptions review submit", "asc iap submit"},
			APIResources: []string{
				"subscriptions",
				"subscriptionGroups",
				"inAppPurchases",
				"inAppPurchaseSubmissions",
				"subscriptionSubmissions",
			},
		},
		{
			Area:       "monetization",
			Capability: "Promoted purchases and offer codes",
			Status:     statusCLISupported,
			Commands:   []string{"asc subscriptions offers", "asc iap offer-codes", "asc iap promoted-purchases"},
			APIResources: []string{
				"promotedPurchases",
				"inAppPurchaseOfferCodes",
				"subscriptionOfferCodes",
				"winBackOffers",
			},
		},
		{
			Area:       "testflight",
			Capability: "TestFlight distribution, feedback, and crashes",
			Status:     statusCLISupported,
			Commands:   []string{"asc testflight groups", "asc testflight testers", "asc testflight feedback", "asc testflight crashes"},
			APIResources: []string{
				"betaGroups",
				"betaTesters",
				"betaFeedbackScreenshotSubmissions",
				"betaFeedbackCrashSubmissions",
			},
		},
		{
			Area:       "testflight",
			Capability: "Sandbox tester lifecycle",
			Status:     statusPartial,
			Commands:   []string{"asc sandbox", "asc web sandbox create"},
			APIResources: []string{
				"sandboxTesters",
				"sandboxTestersClearPurchaseHistoryRequest",
			},
			Notes: []string{"Public API support varies by operation and account; web-session creation exists as an experimental fallback."},
		},
		{
			Area:       "analytics",
			Capability: "Analytics, sales, finance, and performance reports",
			Status:     statusCLISupported,
			Commands:   []string{"asc analytics", "asc finance reports", "asc performance", "asc insights"},
			APIResources: []string{
				"analyticsReportRequests",
				"salesReports",
				"financeReports",
				"diagnosticSignatures",
			},
		},
		{
			Area:       "analytics",
			Capability: "Transaction tax reports",
			Status:     statusNotPublicAPI,
			Notes:      []string{"Apple does not expose Transaction Tax reports through the public App Store Connect API."},
			NextAction: "Download manually from App Store Connect.",
		},
		{
			Area:       "signing",
			Capability: "Signing assets and bundle capabilities",
			Status:     statusCLISupported,
			Commands:   []string{"asc bundle-ids", "asc certificates", "asc profiles", "asc signing"},
			APIResources: []string{
				"bundleIds",
				"bundleIdCapabilities",
				"certificates",
				"profiles",
				"devices",
			},
		},
		{
			Area:       "automation",
			Capability: "Xcode Cloud workflows and artifacts",
			Status:     statusCLISupported,
			Commands:   []string{"asc xcode-cloud run", "asc xcode-cloud status", "asc xcode-cloud artifacts"},
			APIResources: []string{
				"ciProducts",
				"ciWorkflows",
				"ciBuildRuns",
				"ciArtifacts",
			},
		},
		{
			Area:       "automation",
			Capability: "Webhooks",
			Status:     statusCLISupported,
			Commands:   []string{"asc webhooks"},
			APIResources: []string{
				"webhooks",
				"webhookDeliveries",
				"webhookPings",
			},
		},
		{
			Area:       "access",
			Capability: "Users, invitations, actors, and devices",
			Status:     statusCLISupported,
			Commands:   []string{"asc users", "asc actors", "asc devices", "asc account"},
			APIResources: []string{
				"users",
				"userInvitations",
				"actors",
				"devices",
			},
		},
		{
			Area:       "game-center",
			Capability: "Game Center resources",
			Status:     statusCLISupported,
			Commands:   []string{"asc game-center"},
			APIResources: []string{
				"gameCenterAchievements",
				"gameCenterLeaderboards",
				"gameCenterActivities",
				"gameCenterChallenges",
				"gameCenterMatchmakingQueues",
			},
		},
		{
			Area:       "review",
			Capability: "Web-only review rejection inspection",
			Status:     statusExperimental,
			Commands:   []string{"asc web review"},
			Notes:      []string{"Some reviewer-message and rejection-detail surfaces are richer in App Store Connect web-session flows than in public API responses."},
		},
	}
}

func renderTable(report Report) error {
	fmt.Fprintf(os.Stdout, "Total: %d  Schema endpoints: %d\n\n", report.Summary.Total, report.Summary.SchemaEndpointCount)
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "AREA\tCAPABILITY\tSTATUS\tCOMMANDS\tNOTES")
	for _, entry := range report.Capabilities {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			entry.Area,
			entry.Capability,
			entry.Status,
			strings.Join(entry.Commands, ", "),
			strings.Join(entry.Notes, " "),
		)
	}
	return tw.Flush()
}

func renderMarkdown(report Report) error {
	fmt.Fprintf(os.Stdout, "Total: %d  \nSchema endpoints: %d\n\n", report.Summary.Total, report.Summary.SchemaEndpointCount)
	fmt.Fprintln(os.Stdout, "| Area | Capability | Status | Commands | Notes |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, entry := range report.Capabilities {
		fmt.Fprintf(
			os.Stdout,
			"| %s | %s | %s | %s | %s |\n",
			escapeMarkdownTable(entry.Area),
			escapeMarkdownTable(entry.Capability),
			escapeMarkdownTable(entry.Status),
			escapeMarkdownTable(strings.Join(entry.Commands, "<br>")),
			escapeMarkdownTable(strings.Join(entry.Notes, " ")),
		)
	}
	return nil
}

func escapeMarkdownTable(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "|", "\\|")
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
