package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestValidateDateFlagValidDates(t *testing.T) {
	tests := []string{"2026-01-01", "2025-12-31", "2000-06-15"}
	for _, d := range tests {
		if err := validateDateFlag("--start", d); err != nil {
			t.Fatalf("validateDateFlag(%q) unexpected error: %v", d, err)
		}
	}
}

func TestValidateDateFlagRejectsEmpty(t *testing.T) {
	err := validateDateFlag("--start", "")
	if err == nil {
		t.Fatal("expected error for empty date")
	}
	if !strings.Contains(err.Error(), "--start is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDateFlagRejectsInvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"wrong format", "01-01-2026"},
		{"not a date", "foobar"},
		{"month-day only", "01-01"},
		{"slash separator", "2026/01/01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDateFlag("--end", tt.value)
			if err == nil {
				t.Fatalf("expected error for %q", tt.value)
			}
			if !strings.Contains(err.Error(), "must be YYYY-MM-DD") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWebXcodeCloudCommandHierarchy(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	if cmd.Name != "xcode-cloud" {
		t.Fatalf("expected command name %q, got %q", "xcode-cloud", cmd.Name)
	}
	if len(cmd.Subcommands) != 4 {
		t.Fatalf("expected 4 subcommands (usage, products, workflows, env-vars), got %d", len(cmd.Subcommands))
	}

	names := map[string]bool{}
	for _, sub := range cmd.Subcommands {
		names[sub.Name] = true
	}
	if !names["usage"] {
		t.Fatal("expected 'usage' subcommand")
	}
	if !names["products"] {
		t.Fatal("expected 'products' subcommand")
	}
	if !names["workflows"] {
		t.Fatal("expected 'workflows' subcommand")
	}
	if !names["env-vars"] {
		t.Fatal("expected 'env-vars' subcommand")
	}
}

func TestWebXcodeCloudUsageSubcommands(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	usageCmd := findSub(cmd, "usage")
	if usageCmd == nil {
		t.Fatal("could not find 'usage' subcommand")
	}
	if len(usageCmd.Subcommands) != 5 {
		t.Fatalf("expected 5 usage subcommands, got %d", len(usageCmd.Subcommands))
	}
	usageNames := map[string]bool{}
	for _, sub := range usageCmd.Subcommands {
		usageNames[sub.Name] = true
	}
	for _, expected := range []string{"summary", "alert", "months", "days", "workflows"} {
		if !usageNames[expected] {
			t.Fatalf("expected %q usage subcommand", expected)
		}
	}
}

func TestWebXcodeCloudSubcommandsResolveSessionWithinTimeoutContext(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	tests := []struct {
		name  string
		build func() *ffcli.Command
		args  []string
	}{
		{
			name:  "usage summary",
			build: webXcodeCloudUsageSummaryCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "usage alert",
			build: webXcodeCloudUsageAlertCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "usage months",
			build: webXcodeCloudUsageMonthsCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "usage days",
			build: webXcodeCloudUsageDaysCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-ids", "product-123"},
		},
		{
			name:  "usage workflows",
			build: webXcodeCloudUsageWorkflowsCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123"},
		},
		{
			name:  "products",
			build: webXcodeCloudProductsCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "workflows create",
			build: webXcodeCloudWorkflowCreateCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123", "--file", workflowCreateTestPayload(t)},
		},
		{
			name:  "workflows options team-config",
			build: webXcodeCloudWorkflowOptionsTeamConfigCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "workflows options build-versions",
			build: webXcodeCloudWorkflowOptionsBuildVersionsCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "workflows options product-config",
			build: webXcodeCloudWorkflowOptionsProductConfigCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123"},
		},
		{
			name:  "workflows options schemes",
			build: webXcodeCloudWorkflowOptionsSchemesCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123"},
		},
		{
			name:  "workflows options test-destinations",
			build: webXcodeCloudWorkflowOptionsTestDestinationsCommand,
			args:  []string{"--apple-id", "user@example.com", "--xcode-version", "latest:stable"},
		},
		{
			name:  "workflows options slack-provider",
			build: webXcodeCloudWorkflowOptionsSlackProviderCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "workflows options slack-channels",
			build: webXcodeCloudWorkflowOptionsSlackChannelsCommand,
			args:  []string{"--apple-id", "user@example.com"},
		},
		{
			name:  "workflows describe",
			build: webXcodeCloudWorkflowDescribeCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123", "--workflow-id", "wf-123"},
		},
		{
			name:  "workflows enable",
			build: webXcodeCloudWorkflowEnableCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123", "--workflow-id", "wf-123"},
		},
		{
			name:  "workflows disable",
			build: webXcodeCloudWorkflowDisableCommand,
			args:  []string{"--apple-id", "user@example.com", "--product-id", "prod-123", "--workflow-id", "wf-123", "--confirm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hadDeadline := false
			resolveSessionFn = func(
				ctx context.Context,
				appleID, password, twoFactorCode string,
			) (*webcore.AuthSession, string, error) {
				_, hadDeadline = ctx.Deadline()
				return nil, "", resolveErr
			}

			cmd := tt.build()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			err := cmd.Exec(context.Background(), nil)
			if !errors.Is(err, resolveErr) {
				t.Fatalf("expected resolveSession error %v, got %v", resolveErr, err)
			}
			if !hadDeadline {
				t.Fatal("expected resolveSession to receive a timeout context")
			}
		})
	}
}

func workflowCreateTestPayload(t *testing.T) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "workflow-create-*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := file.WriteString(`{"name":"WF"}`); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return file.Name()
}

func TestWebXcodeCloudUsageSummaryOutputTableUsesHumanRenderer(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body := `{"plan":{"name":"Plan","reset_date":"2026-03-27","reset_date_time":"2026-03-27T07:26:10Z","available":1500,"used":0,"total":1500},"links":{"manage":"https://developer.apple.com/xcode-cloud/"}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudUsageSummaryCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, `"plan"`) {
		t.Fatalf("expected table output, got json: %q", stdout)
	}
	for _, token := range []string{"Plan", "Available", "1500"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected table output to include %q, got %q", token, stdout)
		}
	}
	for _, token := range []string{"Usage Bar", "0/1500m"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected table output to include %q, got %q", token, stdout)
		}
	}
}

func TestFormatUsageBar(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		total    int
		contains []string
	}{
		{
			name:     "half usage",
			value:    50,
			total:    100,
			contains: []string{"50%", "########"},
		},
		{
			name:     "empty total",
			value:    10,
			total:    0,
			contains: []string{"n/a"},
		},
		{
			name:     "clamps over total",
			value:    150,
			total:    100,
			contains: []string{"100%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUsageBar(tt.value, tt.total)
			for _, token := range tt.contains {
				if !strings.Contains(got, token) {
					t.Fatalf("expected %q to contain %q", got, token)
				}
			}
		})
	}
}

func TestResolveProductUsageSummaryPrefersOverallProductUsage(t *testing.T) {
	app := &webcore.CIUsageDays{
		Info: webcore.CIUsageInfo{
			Current:  webcore.CIUsageInfoCurrent{Used: 1, Builds: 1, Average30Days: 1},
			Previous: webcore.CIUsageInfoCurrent{Used: 2, Builds: 2, Average30Days: 2},
		},
	}
	overall := &webcore.CIUsageDays{
		ProductUsage: []webcore.CIProductUsage{
			{
				ProductID:              "prod-1",
				UsageInMinutes:         56,
				NumberOfBuilds:         7,
				PreviousUsageInMinutes: 134,
				PreviousNumberOfBuilds: 15,
			},
		},
	}

	current, previous := resolveProductUsageSummary("prod-1", "prod-1", app, overall)
	if current.Used != 56 || current.Builds != 7 {
		t.Fatalf("expected current from overall product usage, got %+v", current)
	}
	if previous.Used != 134 || previous.Builds != 15 {
		t.Fatalf("expected previous from overall product usage, got %+v", previous)
	}
}

func TestResolveProductUsageSummaryFallsBackToNestedUsage(t *testing.T) {
	overall := &webcore.CIUsageDays{
		ProductUsage: []webcore.CIProductUsage{
			{
				ProductID: "prod-1",
				Usage: []webcore.CIMonthUsage{
					{Month: 1, Year: 2026, Duration: 9, NumberOfBuilds: 3},
					{Month: 2, Year: 2026, Duration: 6, NumberOfBuilds: 2},
				},
				PreviousUsageInMinutes: 4,
				PreviousNumberOfBuilds: 1,
			},
		},
	}

	current, previous := resolveProductUsageSummary("prod-1", "prod-1", nil, overall)
	if current.Used != 15 || current.Builds != 5 {
		t.Fatalf("expected current usage derived from nested usage, got %+v", current)
	}
	if previous.Used != 4 || previous.Builds != 1 {
		t.Fatalf("expected previous usage from explicit fields, got %+v", previous)
	}
}

func TestNormalizeProductUsageMixedAggregatesDoesNotDoubleCountBuilds(t *testing.T) {
	product := webcore.CIProductUsage{
		UsageInMinutes: 0,
		NumberOfBuilds: 7,
		Usage: []webcore.CIMonthUsage{
			{Month: 1, Year: 2026, Duration: 12, NumberOfBuilds: 3},
			{Month: 2, Year: 2026, Duration: 8, NumberOfBuilds: 4},
		},
	}

	minutes, builds := normalizeProductUsage(product)
	if minutes != 20 {
		t.Fatalf("minutes = %d, want 20", minutes)
	}
	if builds != 7 {
		t.Fatalf("builds = %d, want 7 (no double-count)", builds)
	}
}

func TestNormalizeWorkflowUsageMixedAggregatesDoesNotDoubleCountBuilds(t *testing.T) {
	workflow := webcore.CIWorkflowUsage{
		UsageInMinutes: 0,
		NumberOfBuilds: 5,
		Usage: []webcore.CIDayUsage{
			{Date: "2026-01-01", Duration: 6, NumberOfBuilds: 2},
			{Date: "2026-01-02", Duration: 4, NumberOfBuilds: 3},
		},
	}

	minutes, builds := normalizeWorkflowUsage(workflow)
	if minutes != 10 {
		t.Fatalf("minutes = %d, want 10", minutes)
	}
	if builds != 5 {
		t.Fatalf("builds = %d, want 5 (no double-count)", builds)
	}
}

func TestBuildCIUsageScopeRowsIncludesBothScopes(t *testing.T) {
	app := &webcore.CIUsageDays{
		Info: webcore.CIUsageInfo{
			Current:  webcore.CIUsageInfoCurrent{Used: 7, Builds: 1},
			Previous: webcore.CIUsageInfoCurrent{Used: 12, Builds: 2},
		},
	}
	overall := &webcore.CIUsageDays{
		Info: webcore.CIUsageInfo{
			Current:  webcore.CIUsageInfoCurrent{Used: 103, Builds: 11},
			Previous: webcore.CIUsageInfoCurrent{Used: 187, Builds: 25},
		},
		ProductUsage: []webcore.CIProductUsage{
			{ProductID: "prod-1", UsageInMinutes: 7, NumberOfBuilds: 1, PreviousUsageInMinutes: 12, PreviousNumberOfBuilds: 2},
			{ProductID: "prod-2", UsageInMinutes: 44, NumberOfBuilds: 3, PreviousUsageInMinutes: 22, PreviousNumberOfBuilds: 1},
		},
	}
	productNames := map[string]string{
		"prod-1": "Chroma",
		"prod-2": "Gradients",
	}

	rows := buildCIUsageScopeRows(app, overall, []string{"prod-1", "prod-2"}, productNames, 1500)
	if len(rows) != 3 {
		t.Fatalf("expected 3 scope rows, got %d", len(rows))
	}
	if rows[0][0] != "Chroma" || rows[1][0] != "Gradients" || rows[2][0] != "Overall Team" {
		t.Fatalf("unexpected scope labels: %v", rows)
	}
	if !strings.Contains(rows[0][5], "/1500m") || !strings.Contains(rows[1][5], "/1500m") {
		t.Fatalf("expected absolute plan denominator in usage bars, got %v", rows)
	}
}

func TestParseProductIDs(t *testing.T) {
	t.Run("valid csv dedupes while preserving order", func(t *testing.T) {
		ids, err := parseProductIDs("prod-1, prod-2, prod-3, prod-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 3 || ids[0] != "prod-1" || ids[1] != "prod-2" || ids[2] != "prod-3" {
			t.Fatalf("unexpected ids: %v", ids)
		}
	})

	t.Run("rejects empty entries", func(t *testing.T) {
		_, err := parseProductIDs("prod-2,,prod-3")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "--product-ids") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestWebXcodeCloudUsageDaysProductIDsValidation(t *testing.T) {
	t.Run("accepts valid product IDs", func(t *testing.T) {
		origResolveSession := resolveSessionFn
		t.Cleanup(func() {
			resolveSessionFn = origResolveSession
		})
		resolveErr := errors.New("stop after validation")
		resolveSessionFn = func(
			ctx context.Context,
			appleID, password, twoFactorCode string,
		) (*webcore.AuthSession, string, error) {
			return nil, "", resolveErr
		}

		cmd := webXcodeCloudUsageDaysCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-1,prod-2,prod-3",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, resolveErr) {
			t.Fatalf("expected resolve session error %v, got %v", resolveErr, err)
		}
	})

	t.Run("requires product IDs", func(t *testing.T) {
		cmd := webXcodeCloudUsageDaysCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		_, stderr := captureOutput(t, func() {
			err := cmd.Exec(context.Background(), nil)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "Error: --product-ids is required") {
			t.Fatalf("unexpected stderr: %q", stderr)
		}
	})

	t.Run("rejects invalid product IDs", func(t *testing.T) {
		cmd := webXcodeCloudUsageDaysCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-2,,prod-3",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		_, stderr := captureOutput(t, func() {
			err := cmd.Exec(context.Background(), nil)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "Error: --product-ids must be a comma-separated list of non-empty product IDs") {
			t.Fatalf("unexpected stderr: %q", stderr)
		}
	})
}

func TestWebXcodeCloudUsageDaysFlagSet(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	daysCmd := findSub(findSub(cmd, "usage"), "days")
	if daysCmd == nil {
		t.Fatal("could not find 'usage days' subcommand")
	}

	fs := daysCmd.FlagSet
	if fs == nil {
		t.Fatal("expected flag set on days command")
	}

	for _, name := range []string{"product-ids", "start", "end"} {
		if fs.Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}

func TestWebXcodeCloudUsageMonthsFlagSet(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	monthsCmd := findSub(findSub(cmd, "usage"), "months")
	if monthsCmd == nil {
		t.Fatal("could not find 'usage months' subcommand")
	}

	fs := monthsCmd.FlagSet
	for _, name := range []string{"start-month", "start-year", "end-month", "end-year", "product-ids"} {
		if fs.Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}

func TestWebXcodeCloudUsageMonthsDefaultsLast12Months(t *testing.T) {
	origNowFn := webNowFn
	t.Cleanup(func() {
		webNowFn = origNowFn
	})
	fixedNow := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)
	webNowFn = func() time.Time { return fixedNow }

	cmd := webXcodeCloudUsageMonthsCommand()
	fs := cmd.FlagSet
	if fs == nil {
		t.Fatal("expected flag set on months command")
	}
	startMonth := fs.Lookup("start-month")
	startYear := fs.Lookup("start-year")
	endMonth := fs.Lookup("end-month")
	endYear := fs.Lookup("end-year")
	if startMonth == nil || startYear == nil || endMonth == nil || endYear == nil {
		t.Fatal("expected start/end month/year flags")
	}

	expectedStart := fixedNow.AddDate(0, -11, 0)
	if got := startMonth.DefValue; got != strconv.Itoa(int(expectedStart.Month())) {
		t.Fatalf("start-month default = %s, want %d", got, int(expectedStart.Month()))
	}
	if got := startYear.DefValue; got != strconv.Itoa(expectedStart.Year()) {
		t.Fatalf("start-year default = %s, want %d", got, expectedStart.Year())
	}
	if got := endMonth.DefValue; got != strconv.Itoa(int(fixedNow.Month())) {
		t.Fatalf("end-month default = %s, want %d", got, int(fixedNow.Month()))
	}
	if got := endYear.DefValue; got != strconv.Itoa(fixedNow.Year()) {
		t.Fatalf("end-year default = %s, want %d", got, fixedNow.Year())
	}
}

func TestFilterProductUsageByIDs(t *testing.T) {
	products := []webcore.CIProductUsage{
		{ProductID: "prod-1", ProductName: "App One", UsageInMinutes: 10},
		{ProductID: "prod-2", ProductName: "App Two", UsageInMinutes: 20},
		{ProductID: "prod-3", ProductName: "App Three", UsageInMinutes: 30},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		result := filterProductUsageByIDs(products, nil)
		if len(result) != 3 {
			t.Fatalf("expected 3 products, got %d", len(result))
		}
	})

	t.Run("filters to matching IDs", func(t *testing.T) {
		result := filterProductUsageByIDs(products, []string{"prod-1", "prod-3"})
		if len(result) != 2 {
			t.Fatalf("expected 2 products, got %d", len(result))
		}
		if result[0].ProductID != "prod-1" || result[1].ProductID != "prod-3" {
			t.Fatalf("unexpected products: %+v", result)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		result := filterProductUsageByIDs(products, []string{"PROD-2"})
		if len(result) != 1 || result[0].ProductID != "prod-2" {
			t.Fatalf("expected prod-2, got %+v", result)
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		result := filterProductUsageByIDs(products, []string{"nonexistent"})
		if len(result) != 0 {
			t.Fatalf("expected 0 products, got %d", len(result))
		}
	})
}

func TestWebXcodeCloudUsageMonthsProductIDsValidation(t *testing.T) {
	t.Run("rejects invalid product IDs", func(t *testing.T) {
		cmd := webXcodeCloudUsageMonthsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-2,,prod-3",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		_, stderr := captureOutput(t, func() {
			err := cmd.Exec(context.Background(), nil)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "Error: --product-ids must be a comma-separated list of non-empty product IDs") {
			t.Fatalf("unexpected stderr: %q", stderr)
		}
	})
}

func TestWebXcodeCloudUsageMonthsOutputTableWithProductFilter(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	requestCount := 0
	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requestCount++
					var body string
					if strings.Contains(req.URL.Path, "/usage/summary") {
						body = `{"plan":{"name":"Plan","total":1500,"used":130,"available":1370}}`
					} else {
						body = `{
							"usage":[{"month":1,"year":2026,"duration":100,"number_of_builds":5},{"month":2,"year":2026,"duration":30,"number_of_builds":2}],
							"product_usage":[
								{"product_id":"prod-1","product_name":"App One","usage_in_minutes":80,"number_of_builds":4,"previous_usage_in_minutes":50,"previous_number_of_builds":3},
								{"product_id":"prod-2","product_name":"App Two","usage_in_minutes":50,"number_of_builds":3,"previous_usage_in_minutes":20,"previous_number_of_builds":1}
							],
							"info":{"start_month":1,"start_year":2026,"end_month":2,"end_year":2026,"current":{"builds":7,"used":130,"average_30_days":65},"previous":{"builds":4,"used":70,"average_30_days":35}}
						}`
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	t.Run("without product filter shows all products", func(t *testing.T) {
		requestCount = 0
		cmd := webXcodeCloudUsageMonthsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, "App One") || !strings.Contains(stdout, "App Two") {
			t.Fatalf("expected both products in output, got %q", stdout)
		}
		if !strings.Contains(stdout, "/1500m") {
			t.Fatalf("expected plan total in usage bar, got %q", stdout)
		}
		if requestCount != 2 {
			t.Fatalf("expected 2 API requests (months + summary), got %d", requestCount)
		}
	})

	t.Run("with product filter shows only matching products", func(t *testing.T) {
		requestCount = 0
		cmd := webXcodeCloudUsageMonthsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-1",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, "App One") {
			t.Fatalf("expected prod-1 in output, got %q", stdout)
		}
		if strings.Contains(stdout, "App Two") {
			t.Fatalf("expected prod-2 to be filtered out, got %q", stdout)
		}
	})

	t.Run("json output skips summary fetch", func(t *testing.T) {
		requestCount = 0
		cmd := webXcodeCloudUsageMonthsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, `"usage"`) {
			t.Fatalf("expected json usage payload, got %q", stdout)
		}
		if requestCount != 1 {
			t.Fatalf("expected 1 API request (months only) for json output, got %d", requestCount)
		}
	})
}

func TestWebXcodeCloudUsageMonthsTableDoesNotFailWhenSummaryUnavailable(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.Contains(req.URL.Path, "/usage/summary") {
						return &http.Response{
							StatusCode: http.StatusForbidden,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
							Request:    req,
						}, nil
					}
					body := `{
						"usage":[{"month":1,"year":2026,"duration":100,"number_of_builds":5}],
						"product_usage":[{"product_id":"prod-1","product_name":"App One","usage_in_minutes":80,"number_of_builds":4}],
						"info":{"start_month":1,"start_year":2026,"end_month":1,"end_year":2026}
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudUsageMonthsCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if !strings.Contains(stdout, "App One") {
		t.Fatalf("expected table output despite summary failure, got %q", stdout)
	}
}

func TestWebXcodeCloudUsageDaysOutputBehavior(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	t.Run("json output skips team-wide and lookup requests", func(t *testing.T) {
		productCalls := 0
		overallCalls := 0
		summaryCalls := 0
		productsCalls := 0

		resolveSessionFn = func(
			ctx context.Context,
			appleID, password, twoFactorCode string,
		) (*webcore.AuthSession, string, error) {
			return &webcore.AuthSession{
				PublicProviderID: "team-uuid",
				Client: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						path := req.URL.Path
						body := "{}"
						switch {
						case strings.Contains(path, "/products/prod-1/usage/days"):
							productCalls++
							body = `{
								"usage":[{"date":"2026-01-15","duration":5,"number_of_builds":1}],
								"workflow_usage":[],
								"info":{"current":{"builds":1,"used":5,"average_30_days":5},"previous":{"builds":0,"used":0,"average_30_days":0}}
							}`
						case strings.Contains(path, "/usage/days"):
							overallCalls++
							body = `{"usage":[],"workflow_usage":[],"info":{}}`
						case strings.Contains(path, "/usage/summary"):
							summaryCalls++
							body = `{"plan":{"total":1500}}`
						case strings.Contains(path, "/products-v4"):
							productsCalls++
							body = `{"items":[{"id":"prod-1","name":"App One"}]}`
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}),
				},
			}, "cache", nil
		}

		cmd := webXcodeCloudUsageDaysCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, `"usage"`) {
			t.Fatalf("expected json usage payload, got %q", stdout)
		}
		if productCalls != 1 {
			t.Fatalf("expected 1 product-days request, got %d", productCalls)
		}
		if overallCalls != 0 || summaryCalls != 0 || productsCalls != 0 {
			t.Fatalf("expected no team-wide/summary/products requests in json mode, got overall=%d summary=%d products=%d", overallCalls, summaryCalls, productsCalls)
		}
	})

	t.Run("table output falls back when product lookup fails", func(t *testing.T) {
		resolveSessionFn = func(
			ctx context.Context,
			appleID, password, twoFactorCode string,
		) (*webcore.AuthSession, string, error) {
			return &webcore.AuthSession{
				PublicProviderID: "team-uuid",
				Client: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						path := req.URL.Path
						switch {
						case strings.Contains(path, "/products/prod-1/usage/days"):
							body := `{
								"usage":[{"date":"2026-01-15","duration":5,"number_of_builds":1}],
								"workflow_usage":[],
								"info":{"current":{"builds":1,"used":5,"average_30_days":5},"previous":{"builds":0,"used":0,"average_30_days":0}}
							}`
							return &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(body)),
								Request:    req,
							}, nil
						case strings.Contains(path, "/usage/days"):
							body := `{
								"usage":[],
								"workflow_usage":[],
								"product_usage":[{"product_id":"prod-1","usage_in_minutes":5,"number_of_builds":1,"previous_usage_in_minutes":0,"previous_number_of_builds":0}],
								"info":{"current":{"builds":1,"used":5,"average_30_days":5},"previous":{"builds":0,"used":0,"average_30_days":0}}
							}`
							return &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(body)),
								Request:    req,
							}, nil
						case strings.Contains(path, "/usage/summary"):
							body := `{"plan":{"total":1500}}`
							return &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(body)),
								Request:    req,
							}, nil
						case strings.Contains(path, "/products-v4"):
							// Product-name lookup should be best effort and not fail the command.
							return &http.Response{
								StatusCode: http.StatusForbidden,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403","detail":"forbidden"}]}`)),
								Request:    req,
							}, nil
						default:
							return &http.Response{
								StatusCode: http.StatusNotFound,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
								Request:    req,
							}, nil
						}
					}),
				},
			}, "cache", nil
		}

		cmd := webXcodeCloudUsageDaysCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-1",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, "prod-1") {
			t.Fatalf("expected product-id fallback label in output, got %q", stdout)
		}
		if !strings.Contains(stdout, "Overall Team") {
			t.Fatalf("expected overall row in output, got %q", stdout)
		}
	})

	t.Run("table output degrades when overall and summary fail", func(t *testing.T) {
		resolveSessionFn = func(
			ctx context.Context,
			appleID, password, twoFactorCode string,
		) (*webcore.AuthSession, string, error) {
			return &webcore.AuthSession{
				PublicProviderID: "team-uuid",
				Client: &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						path := req.URL.Path
						switch {
						case strings.Contains(path, "/products/prod-1/usage/days"):
							body := `{
								"usage":[{"date":"2026-01-15","duration":5,"number_of_builds":1}],
								"workflow_usage":[],
								"info":{"current":{"builds":1,"used":5,"average_30_days":5},"previous":{"builds":0,"used":0,"average_30_days":0}}
							}`
							return &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(body)),
								Request:    req,
							}, nil
						case strings.Contains(path, "/usage/days"), strings.Contains(path, "/usage/summary"), strings.Contains(path, "/products-v4"):
							return &http.Response{
								StatusCode: http.StatusForbidden,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
								Request:    req,
							}, nil
						default:
							return &http.Response{
								StatusCode: http.StatusNotFound,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
								Request:    req,
							}, nil
						}
					}),
				},
			}, "cache", nil
		}

		cmd := webXcodeCloudUsageDaysCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-ids", "prod-1,prod-2",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, "Overall usage unavailable") {
			t.Fatalf("expected fallback note when overall fails, got %q", stdout)
		}
		if strings.Contains(stdout, "Overall Team") {
			t.Fatalf("did not expect overall team row without overall data, got %q", stdout)
		}
		if !strings.Contains(stdout, "prod-1") {
			t.Fatalf("expected selected product scope row, got %q", stdout)
		}
	})
}

func TestWebXcodeCloudUsageWorkflowsFlagSet(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	workflowsCmd := findSub(findSub(cmd, "usage"), "workflows")
	if workflowsCmd == nil {
		t.Fatal("could not find 'usage workflows' subcommand")
	}

	fs := workflowsCmd.FlagSet
	for _, name := range []string{"product-id", "workflow-id", "start", "end"} {
		if fs.Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}

func TestWebXcodeCloudUsageWorkflowsRequiresProductID(t *testing.T) {
	cmd := webXcodeCloudUsageWorkflowsCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "Error: --product-id is required") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestFindWorkflowByID(t *testing.T) {
	workflows := []webcore.CIWorkflowUsage{
		{WorkflowID: "wf-1", WorkflowName: "Build"},
		{WorkflowID: "wf-2", WorkflowName: "Test"},
	}

	t.Run("finds by exact ID", func(t *testing.T) {
		wf := findWorkflowByID(workflows, "wf-1")
		if wf == nil || wf.WorkflowName != "Build" {
			t.Fatalf("expected Build workflow, got %+v", wf)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		wf := findWorkflowByID(workflows, "WF-2")
		if wf == nil || wf.WorkflowName != "Test" {
			t.Fatalf("expected Test workflow, got %+v", wf)
		}
	})

	t.Run("returns nil for missing", func(t *testing.T) {
		wf := findWorkflowByID(workflows, "wf-999")
		if wf != nil {
			t.Fatalf("expected nil, got %+v", wf)
		}
	})

	t.Run("returns nil for empty ID", func(t *testing.T) {
		wf := findWorkflowByID(workflows, "")
		if wf != nil {
			t.Fatalf("expected nil, got %+v", wf)
		}
	})
}

func TestWebXcodeCloudUsageWorkflowsListOutput(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var body string
					if strings.Contains(req.URL.Path, "/usage/summary") {
						body = `{"plan":{"name":"Plan","total":1500,"used":100,"available":1400}}`
					} else {
						body = `{
							"usage":[{"date":"2026-01-15","duration":30,"number_of_builds":3}],
							"workflow_usage":[
								{"workflow_id":"wf-1","workflow_name":"Build","usage_in_minutes":20,"number_of_builds":2,"previous_usage_in_minutes":10,"previous_number_of_builds":1},
								{"workflow_id":"wf-2","workflow_name":"Test","usage_in_minutes":10,"number_of_builds":1,"previous_usage_in_minutes":5,"previous_number_of_builds":1}
							],
							"info":{}
						}`
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	t.Run("lists all workflows", func(t *testing.T) {
		cmd := webXcodeCloudUsageWorkflowsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-id", "prod-1",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, "Build") || !strings.Contains(stdout, "Test") {
			t.Fatalf("expected both workflows in output, got %q", stdout)
		}
		if !strings.Contains(stdout, "wf-1") || !strings.Contains(stdout, "wf-2") {
			t.Fatalf("expected workflow IDs in output, got %q", stdout)
		}
		if !strings.Contains(stdout, "Workflows: 2") {
			t.Fatalf("expected workflow count, got %q", stdout)
		}
		if !strings.Contains(stdout, "/1500m") {
			t.Fatalf("expected plan total in output, got %q", stdout)
		}
	})

	t.Run("drills into specific workflow", func(t *testing.T) {
		cmd := webXcodeCloudUsageWorkflowsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-id", "prod-1",
			"--workflow-id", "wf-1",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, _ := captureOutput(t, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		if !strings.Contains(stdout, "Build") {
			t.Fatalf("expected workflow name in output, got %q", stdout)
		}
		if !strings.Contains(stdout, "Current: 20 minutes") {
			t.Fatalf("expected current usage, got %q", stdout)
		}
		if !strings.Contains(stdout, "Previous: 10 minutes") {
			t.Fatalf("expected previous usage, got %q", stdout)
		}
		// Should NOT show the other workflow
		if strings.Contains(stdout, "Test") {
			t.Fatalf("expected only Build workflow, got %q", stdout)
		}
	})

	t.Run("workflow not found returns error", func(t *testing.T) {
		cmd := webXcodeCloudUsageWorkflowsCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--apple-id", "user@example.com",
			"--product-id", "prod-1",
			"--workflow-id", "nonexistent",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error for missing workflow")
		}
		if !strings.Contains(err.Error(), `workflow "nonexistent" not found`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestWebXcodeCloudUsageWorkflowsJSONSkipsSummaryFetch(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	summaryCalls := 0
	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/usage/summary"):
						summaryCalls++
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"plan":{"total":1500}}`)),
							Request:    req,
						}, nil
					case strings.Contains(req.URL.Path, "/workflows-v15"):
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"items":[{"id":"wf-1","content":{"name":"Build"}}]}`)),
							Request:    req,
						}, nil
					default:
						body := `{
							"usage":[{"date":"2026-01-15","duration":30,"number_of_builds":3}],
							"workflow_usage":[{"workflow_id":"wf-1","usage_in_minutes":20,"number_of_builds":2}],
							"info":{}
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudUsageWorkflowsCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"workflows"`) {
		t.Fatalf("expected workflows json output, got %q", stdout)
	}
	if summaryCalls != 0 {
		t.Fatalf("expected no summary request in json mode, got %d", summaryCalls)
	}
}

func TestPopulateWorkflowNames(t *testing.T) {
	workflows := []webcore.CIWorkflowUsage{
		{WorkflowID: "wf-1", WorkflowName: ""},
		{WorkflowID: "wf-2", WorkflowName: "Already Named"},
		{WorkflowID: "wf-3", WorkflowName: ""},
	}
	names := map[string]string{
		"wf-1": "TestFlight Deploy",
		"wf-2": "Should Not Override",
		"wf-3": "PR Check",
	}

	populateWorkflowNames(workflows, names)

	if workflows[0].WorkflowName != "TestFlight Deploy" {
		t.Fatalf("expected wf-1 name to be populated, got %q", workflows[0].WorkflowName)
	}
	if workflows[1].WorkflowName != "Already Named" {
		t.Fatalf("expected wf-2 name to be preserved, got %q", workflows[1].WorkflowName)
	}
	if workflows[2].WorkflowName != "PR Check" {
		t.Fatalf("expected wf-3 name to be populated, got %q", workflows[2].WorkflowName)
	}
}

func TestWebXcodeCloudAllCommandsHaveUsageFunc(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	if cmd.UsageFunc == nil {
		t.Fatal("expected UsageFunc on xcode-cloud command")
	}
	for _, sub := range cmd.Subcommands {
		if sub.UsageFunc == nil {
			t.Fatalf("expected UsageFunc on %q subcommand", sub.Name)
		}
		for _, subsub := range sub.Subcommands {
			if subsub.UsageFunc == nil {
				t.Fatalf("expected UsageFunc on %q subcommand", subsub.Name)
			}
		}
	}
}

func findSub(cmd *ffcli.Command, name string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	for _, sub := range cmd.Subcommands {
		if sub.Name == name {
			return sub
		}
	}
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, stderr
}
