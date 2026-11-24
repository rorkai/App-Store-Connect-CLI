package cmdtest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestInsightsWeeklyValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_VENDOR_NUMBER", "")
	t.Setenv("ASC_ANALYTICS_VENDOR_NUMBER", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"insights", "weekly", "--source", "analytics", "--week", "2026-02-16"},
			wantErr: "--app is required",
		},
		{
			name:    "missing source",
			args:    []string{"insights", "weekly", "--app", "app-1", "--week", "2026-02-16"},
			wantErr: "--source is required",
		},
		{
			name:    "missing week",
			args:    []string{"insights", "weekly", "--app", "app-1", "--source", "analytics"},
			wantErr: "--week is required",
		},
		{
			name:    "sales missing vendor",
			args:    []string{"insights", "weekly", "--app", "app-1", "--source", "sales", "--week", "2026-02-16"},
			wantErr: "--vendor is required for --source sales",
		},
		{
			name:    "currency breakdown requires sales",
			args:    []string{"insights", "weekly", "--app", "app-1", "--source", "analytics", "--week", "2026-02-16", "--by-currency"},
			wantErr: "--by-currency requires --source sales",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestInsightsDailyValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_VENDOR_NUMBER", "")
	t.Setenv("ASC_ANALYTICS_VENDOR_NUMBER", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"insights", "daily", "--vendor", "12345678", "--date", "2026-02-20"},
			wantErr: "--app is required",
		},
		{
			name:    "missing vendor",
			args:    []string{"insights", "daily", "--app", "app-1", "--date", "2026-02-20"},
			wantErr: "--vendor is required",
		},
		{
			name:    "missing date",
			args:    []string{"insights", "daily", "--app", "app-1", "--vendor", "12345678"},
			wantErr: "--date is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestInsightsWeeklySalesJSON(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"apps",
			"id":"app-1",
			"attributes":{
				"name":"Example App",
				"bundleId":"com.example.app",
				"sku":"example-sku-1"
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=WEEKLY", "filter[reportDate]=2026-02-22", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t10\t0.00\t0.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t2\t2.50\t4.00",
		"foo\tother-sku\tapp-999\tother-sku\t99\t9.99\t9.99",
		"",
	}, "\n")))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=WEEKLY", "filter[reportDate]=2026-02-15", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t8\t0.00\t0.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t1\t1.25\t2.00",
		"",
	}, "\n")))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--week", "2026-02-16")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	source, ok := payload["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got %T", payload["source"])
	}
	if source["name"] != "sales" {
		t.Fatalf("expected source.name=sales, got %v", source["name"])
	}
	if source["vendorNumber"] != "12345678" {
		t.Fatalf("expected source.vendorNumber=12345678, got %v", source["vendorNumber"])
	}
	if source["appSku"] != "example-sku-1" {
		t.Fatalf("expected source.appSku=example-sku-1, got %v", source["appSku"])
	}

	week, ok := payload["week"].(map[string]any)
	if !ok {
		t.Fatalf("expected week object, got %T", payload["week"])
	}
	if week["start"] != "2026-02-16" || week["end"] != "2026-02-22" {
		t.Fatalf("unexpected week range: %v", week)
	}

	metrics, ok := payload["metrics"].([]any)
	if !ok {
		t.Fatalf("expected metrics array, got %T", payload["metrics"])
	}
	unitsMetric := findMetric(t, metrics, "units")
	if unitsMetric["status"] != "ok" {
		t.Fatalf("expected units metric status ok, got %v", unitsMetric["status"])
	}
	if unitsMetric["thisWeek"] != 12.0 || unitsMetric["lastWeek"] != 9.0 {
		t.Fatalf("unexpected units totals: %v", unitsMetric)
	}
	monetizedMetric := findMetric(t, metrics, "monetized_units")
	if monetizedMetric["thisWeek"] != 2.0 || monetizedMetric["lastWeek"] != 1.0 {
		t.Fatalf("unexpected monetized units totals: %v", monetizedMetric)
	}
	proceedsMetric := findMetric(t, metrics, "developer_proceeds")
	if proceedsMetric["thisWeek"] != 2.5 || proceedsMetric["lastWeek"] != 1.25 {
		t.Fatalf("unexpected proceeds totals: %v", proceedsMetric)
	}
	unavailableMetric := findMetric(t, metrics, "active_devices")
	if unavailableMetric["status"] != "unavailable" {
		t.Fatalf("expected active_devices metric unavailable, got %v", unavailableMetric["status"])
	}
}

func TestInsightsWeeklySalesReportRowsUnavailableOnFetchError(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"apps",
			"id":"app-1",
			"attributes":{
				"name":"Example App",
				"bundleId":"com.example.app",
				"sku":"example-sku-1"
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=WEEKLY", "filter[reportDate]=2026-02-22", "filter[version]=1_0"), asctest.JSON(http.StatusInternalServerError, `{
		"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Server Error","detail":"temporary failure"}]
	}`))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=WEEKLY", "filter[reportDate]=2026-02-15", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t8\t1.25\t2.00",
		"",
	}, "\n")))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--week", "2026-02-16")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	metrics, ok := payload["metrics"].([]any)
	if !ok {
		t.Fatalf("expected metrics array, got %T", payload["metrics"])
	}

	reportRowsMetric := findMetric(t, metrics, "report_rows")
	if reportRowsMetric["status"] != "unavailable" {
		t.Fatalf("expected report_rows metric unavailable, got %v", reportRowsMetric["status"])
	}
}

func TestInsightsDailySalesJSON(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"apps",
			"id":"app-1",
			"attributes":{
				"name":"Example App",
				"bundleId":"com.example.app",
				"sku":"example-sku-1"
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=DAILY", "filter[reportDate]=2026-02-18", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t \t1\t0.00\t0.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t2.46\t3.99",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tNew\t1\t1.23\t1.99",
		"foo\tother-sku\tapp-999\tother-sku\tRenewal\t99\t9.99\t9.99",
		"",
	}, "\n")))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=DAILY", "filter[reportDate]=2026-02-17", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t \t2\t0.00\t0.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t1.11\t1.49",
		"",
	}, "\n")))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "daily", "--app", "app-1", "--vendor", "12345678", "--date", "2026-02-18")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	source, ok := payload["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got %T", payload["source"])
	}
	if source["name"] != "sales" {
		t.Fatalf("expected source.name=sales, got %v", source["name"])
	}
	if source["appSku"] != "example-sku-1" {
		t.Fatalf("expected source.appSku=example-sku-1, got %v", source["appSku"])
	}

	metrics, ok := payload["metrics"].([]any)
	if !ok {
		t.Fatalf("expected metrics array, got %T", payload["metrics"])
	}
	renewalUnits := findMetric(t, metrics, "renewal_units")
	if renewalUnits["thisDay"] != 1.0 || renewalUnits["previousDay"] != 1.0 {
		t.Fatalf("unexpected renewal units totals: %v", renewalUnits)
	}
	renewalProceeds := findMetric(t, metrics, "renewal_developer_proceeds")
	if renewalProceeds["thisDay"] != 2.46 || renewalProceeds["previousDay"] != 1.11 {
		t.Fatalf("unexpected renewal proceeds totals: %v", renewalProceeds)
	}
	subscriptionUnits := findMetric(t, metrics, "subscription_units")
	if subscriptionUnits["thisDay"] != 2.0 || subscriptionUnits["previousDay"] != 1.0 {
		t.Fatalf("unexpected subscription units totals: %v", subscriptionUnits)
	}
}

func TestInsightsWeeklyAnalyticsJSON(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"analyticsReportRequests",
				"id":"req-1",
				"attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReportRequests/req-1/reports", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Usage"}}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-1/instances", "filter[granularity]=DAILY,WEEKLY,MONTHLY", "filter[processingDate]=2026-02-16,2026-02-17,2026-02-18,2026-02-19,2026-02-20,2026-02-21,2026-02-22,2026-02-09,2026-02-10,2026-02-11,2026-02-12,2026-02-13,2026-02-14,2026-02-15", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"analyticsReportInstances","id":"inst-1","attributes":{"granularity":"DAILY","processingDate":"2026-02-18"}},
			{"type":"analyticsReportInstances","id":"inst-2","attributes":{"granularity":"DAILY","processingDate":"2026-02-10"}}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "analytics", "--week", "2026-02-16")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	source, ok := payload["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got %T", payload["source"])
	}
	if source["name"] != "analytics" {
		t.Fatalf("expected source.name=analytics, got %v", source["name"])
	}
	if source["requestsScanned"] != 1.0 {
		t.Fatalf("expected source.requestsScanned=1, got %v", source["requestsScanned"])
	}

	metrics, ok := payload["metrics"].([]any)
	if !ok {
		t.Fatalf("expected metrics array, got %T", payload["metrics"])
	}
	instancesMetric := findMetric(t, metrics, "instances_available")
	if instancesMetric["status"] != "ok" {
		t.Fatalf("expected instances_available status ok, got %v", instancesMetric["status"])
	}
	if instancesMetric["thisWeek"] != 1.0 || instancesMetric["lastWeek"] != 1.0 {
		t.Fatalf("unexpected instances metrics %v", instancesMetric)
	}
	unavailableMetric := findMetric(t, metrics, "business_conversion_rate")
	if unavailableMetric["status"] != "unavailable" {
		t.Fatalf("expected business_conversion_rate unavailable, got %v", unavailableMetric["status"])
	}
}

func TestInsightsWeeklyTableOutput(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"apps",
			"id":"app-1",
			"attributes":{
				"name":"Example App",
				"bundleId":"com.example.app",
				"sku":"example-sku-1"
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=WEEKLY", "filter[reportDate]=2026-02-22", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t1\t0.00\t0.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t1\t0.10\t0.20",
	}, "\n")))
	reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]=WEEKLY", "filter[reportDate]=2026-02-15", "filter[version]=1_0"), insightsGzipResponse(strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\texample-sku-1\tapp-1\t\t1\t0.00\t0.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t1\t0.10\t0.20",
	}, "\n")))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--week", "2026-02-16", "--output", "table")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "CONTEXT") || !strings.Contains(stdout, "METRICS") {
		t.Fatalf("expected context and metrics sections in table output, got %q", stdout)
	}
}

func TestInsightsWeeklyAnalyticsForbiddenReturnsUnavailable(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusForbidden, `{
		"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"security restriction"}]
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "analytics", "--week", "2026-02-16")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	metrics, ok := payload["metrics"].([]any)
	if !ok || len(metrics) == 0 {
		t.Fatalf("expected metrics array, got %T %v", payload["metrics"], payload["metrics"])
	}
	completedMetric := findMetric(t, metrics, "completed_requests")
	if completedMetric["status"] != "unavailable" {
		t.Fatalf("expected completed_requests unavailable, got %v", completedMetric["status"])
	}
}

func TestInsightsWeeklyAnalyticsNestedForbiddenReturnsUnavailable(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"analyticsReportRequests","id":"req-1","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReportRequests/req-1/reports", "limit=200"), asctest.JSON(http.StatusForbidden, `{
		"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"security restriction"}]
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "analytics", "--week", "2026-02-16")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	metrics, ok := payload["metrics"].([]any)
	if !ok || len(metrics) == 0 {
		t.Fatalf("expected metrics array, got %T %v", payload["metrics"], payload["metrics"])
	}
	reportsMetric := findMetric(t, metrics, "reports_available")
	if reportsMetric["status"] != "unavailable" {
		t.Fatalf("expected reports_available unavailable, got %v", reportsMetric["status"])
	}
}

const insightsSalesCurrencyHeader = "Provider\tSKU\tApple Identifier\tParent Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Currency\tCurrency of Proceeds\tCustomer Price"

func TestInsightsWeeklySalesMoneyTotalsUseUnitsAndCurrency(t *testing.T) {
	tests := []struct {
		name         string
		thisWeek     []string
		lastWeek     []string
		byCurrency   bool
		wantProceeds string
		wantPrice    string
		wantReason   []string
	}{
		{
			name:       "single currency with multi-unit and refund rows",
			byCurrency: true,
			thisWeek: []string{
				"foo\texample-sku-1\tapp-1\t\t \t10\t0.00\tJPY\tJPY\t0.00",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t4\t3.50\tUSD\tUSD\t5.00",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t-1\t3.50\tUSD\tUSD\t-5.00",
			},
			lastWeek: []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t2\t3.50\tUSD\tUSD\t5.00",
			},
			wantProceeds: `{"name":"developer_proceeds","unit":"currency","currency":"USD","thisWeek":10.5,"lastWeek":7,"delta":3.5,"deltaPercent":50,"status":"ok",
				"byCurrency":[{"currency":"USD","thisWeek":10.5,"lastWeek":7,"delta":3.5,"deltaPercent":50}]}`,
			wantPrice: `{"name":"customer_price","unit":"currency","currency":"USD","thisWeek":15,"lastWeek":10,"delta":5,"deltaPercent":50,"status":"ok",
				"byCurrency":[{"currency":"USD","thisWeek":15,"lastWeek":10,"delta":5,"deltaPercent":50}]}`,
		},
		{
			name:       "multiple currencies are never summed",
			byCurrency: true,
			thisWeek: []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t2\t3.50\tUSD\tUSD\t5.00",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t500\tJPY\tJPY\t800",
			},
			lastWeek: []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t3.50\tUSD\tUSD\t5.00",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t2\t500\tJPY\tJPY\t800",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t3.00\tEUR\tEUR\t4.50",
			},
			wantProceeds: `{"name":"developer_proceeds","unit":"currency","status":"unavailable","byCurrency":[
				{"currency":"EUR","thisWeek":0,"lastWeek":3,"delta":-3,"deltaPercent":-100},
				{"currency":"JPY","thisWeek":500,"lastWeek":1000,"delta":-500,"deltaPercent":-50},
				{"currency":"USD","thisWeek":7,"lastWeek":3.5,"delta":3.5,"deltaPercent":100}]}`,
			wantPrice: `{"name":"customer_price","unit":"currency","status":"unavailable","byCurrency":[
				{"currency":"EUR","thisWeek":0,"lastWeek":4.5,"delta":-4.5,"deltaPercent":-100},
				{"currency":"JPY","thisWeek":800,"lastWeek":1600,"delta":-800,"deltaPercent":-50},
				{"currency":"USD","thisWeek":10,"lastWeek":5,"delta":5,"deltaPercent":100}]}`,
			wantReason: []string{"EUR", "JPY", "USD"},
		},
		{
			name: "default preserves the published scalar comparison",
			thisWeek: []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t2\t3\tUSD\tUSD\t5",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t500\tJPY\tJPY\t800",
			},
			lastWeek: []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t3\tUSD\tUSD\t5",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t2\t500\tJPY\tJPY\t800",
			},
			wantProceeds: `{"name":"developer_proceeds","unit":"currency","thisWeek":503,"lastWeek":503,"delta":0,"deltaPercent":0,"status":"ok"}`,
			wantPrice:    `{"name":"customer_price","unit":"currency","thisWeek":805,"lastWeek":805,"delta":0,"deltaPercent":0,"status":"ok"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := registerInsightsSalesReports(t, "WEEKLY", "2026-02-22", tt.thisWeek, "2026-02-15", tt.lastWeek)
			args := []string{"insights", "weekly", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--week", "2026-02-16", "--output", "json"}
			if tt.byCurrency {
				args = append(args, "--by-currency")
			}
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err != nil || stderr != "" {
				t.Fatalf("run error: %v, stderr=%q", err, stderr)
			}
			metrics := decodeInsightsMetrics(t, stdout)
			assertInsightsCurrencyMetric(t, findMetric(t, metrics, "developer_proceeds"), tt.wantProceeds, tt.wantReason)
			assertInsightsCurrencyMetric(t, findMetric(t, metrics, "customer_price"), tt.wantPrice, tt.wantReason)
		})
	}
}

func TestInsightsDailySalesProceedsSplitByCurrency(t *testing.T) {
	reg := registerInsightsSalesReports(t, "DAILY", "2026-02-18", []string{
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t3\t2.50\tUSD\tUSD\t4.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t300\tJPY\tJPY\t450",
	}, "2026-02-17", []string{
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t2.50\tUSD\tUSD\t4.00",
	})
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "daily", "--app", "app-1", "--vendor", "12345678", "--date", "2026-02-18", "--by-currency")
	if err != nil || stderr != "" {
		t.Fatalf("run error: %v, stderr=%q", err, stderr)
	}
	metrics := decodeInsightsMetrics(t, stdout)
	for _, name := range []string{"renewal_developer_proceeds", "subscription_developer_proceeds"} {
		assertInsightsCurrencyMetric(t, findMetric(t, metrics, name), `{"name":"`+name+`","unit":"currency","status":"unavailable","byCurrency":[
			{"currency":"JPY","thisDay":300,"previousDay":0,"delta":300},
			{"currency":"USD","thisDay":7.5,"previousDay":2.5,"delta":5,"deltaPercent":200}]}`, []string{"JPY", "USD"})
	}
}

func TestInsightsWeeklySalesDoesNotCompareIncompleteCurrency(t *testing.T) {
	for _, tc := range []struct {
		name        string
		row         string
		eurComplete bool
	}{
		{
			name:        "zero-unit partial refund",
			row:         "foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t0\t1.17\tUSD\tUSD\t-1.67",
			eurComplete: true,
		},
		{
			name: "unknown proceeds currency",
			row:  "foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t1.17\tUSD\t\t1.67",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := registerInsightsSalesReports(t, "WEEKLY", "2026-02-22", []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t2\t3.50\tUSD\tUSD\t5.00",
				tc.row,
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t2\t3.00\tEUR\tEUR\t4.50",
			}, "2026-02-15", []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t3.50\tUSD\tUSD\t5.00",
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t3.00\tEUR\tEUR\t4.50",
			})
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--week", "2026-02-16", "--output", "json", "--by-currency")
			if err != nil || stderr != "" {
				t.Fatalf("run error: %v, stderr=%q", err, stderr)
			}
			metric := findMetric(t, decodeInsightsMetrics(t, stdout), "developer_proceeds")
			breakdown, ok := metric["byCurrency"].([]any)
			if !ok || len(breakdown) != 2 {
				t.Fatalf("expected EUR and USD comparison, got %v", metric)
			}
			for _, item := range breakdown {
				amount, ok := item.(map[string]any)
				if !ok {
					t.Fatalf("expected currency comparison object, got %v", item)
				}
				if amount["currency"] == "EUR" && tc.eurComplete {
					if amount["thisWeek"] != 6.0 || amount["lastWeek"] != 3.0 || amount["delta"] != 3.0 {
						t.Errorf("expected unaffected EUR comparison of 6 vs 3, got %v", amount)
					}
					continue
				}
				if amount["status"] != "unavailable" || amount["thisWeek"] != nil || amount["delta"] != nil || amount["deltaPercent"] != nil {
					t.Errorf("incomplete current period cannot be compared, got %v", amount)
				}
			}
		})
	}
}

func TestAnalyticsCompareSalesCurrencyOptIn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		byCurrency bool
		want       string
	}{
		{
			name: "default leaves amounts as reported",
			want: `{"name":"developer_proceeds","unit":"currency","baseline":3.5,"comparison":3.5,"delta":0,"deltaPercent":0,"status":"ok"}`,
		},
		{
			name:       "opt in multiplies amounts by units",
			byCurrency: true,
			want: `{"name":"developer_proceeds","unit":"currency","currency":"USD","baseline":3.5,"comparison":7,"delta":3.5,"deltaPercent":100,"status":"ok",
				"byCurrency":[{"currency":"USD","baseline":3.5,"comparison":7,"delta":3.5,"deltaPercent":100}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := registerInsightsSalesReports(t, "DAILY", "2026-02-18", []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t2\t3.50\tUSD\tUSD\t5.00",
			}, "2026-02-17", []string{
				"foo\tcom.example.app.plus\tiap-1\texample-sku-1\tRenewal\t1\t3.50\tUSD\tUSD\t5.00",
			})
			args := []string{"analytics", "compare", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--from", "2026-02-17", "--to", "2026-02-18", "--frequency", "DAILY", "--output", "json"}
			if tc.byCurrency {
				args = append(args, "--by-currency")
			}
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)
			if err != nil || stderr != "" {
				t.Fatalf("run error: %v, stderr=%q", err, stderr)
			}
			assertInsightsCurrencyMetric(t, findMetric(t, decodeInsightsMetrics(t, stdout), "developer_proceeds"), tc.want, nil)
		})
	}
}

func TestInsightsWeeklySalesMarkdownListsEachCurrency(t *testing.T) {
	reg := registerInsightsSalesReports(t, "WEEKLY", "2026-02-22", []string{
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t2\t3.50\tUSD\tUSD\t5.00",
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t500\tJPY\tUSD\t800",
	}, "2026-02-15", []string{
		"foo\tcom.example.app.plus\tiap-1\texample-sku-1\t \t1\t3.50\tUSD\tUSD\t5.00",
	})
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "insights", "weekly", "--app", "app-1", "--source", "sales", "--vendor", "12345678", "--week", "2026-02-16", "--output", "markdown", "--by-currency")
	if err != nil || stderr != "" {
		t.Fatalf("run error: %v, stderr=%q", err, stderr)
	}

	// metric, unit, thisWeek, lastWeek, delta, deltaPercent, status
	var got [][]string
	for line := range strings.SplitSeq(stdout, "\n") {
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) == 8 && (cells[0] == "developer_proceeds" || cells[0] == "customer_price") {
			got = append(got, cells[:7])
		}
	}
	want := [][]string{
		{"developer_proceeds", "USD", "507.00", "3.50", "503.50", "14385.71", "ok"},
		{"customer_price", "currency", "n/a", "n/a", "n/a", "n/a", "unavailable"},
		{"customer_price", "JPY", "800.00", "0.00", "800.00", "n/a", "ok"},
		{"customer_price", "USD", "10.00", "5.00", "5.00", "100.00", "ok"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected money rows\n got: %v\nwant: %v", got, want)
	}
}

func registerInsightsSalesReports(t *testing.T, frequency, thisDate string, thisRows []string, previousDate string, previousRows []string) *asctest.Registry {
	t.Helper()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
		"data":{"type":"apps","id":"app-1","attributes":{"name":"Example App","bundleId":"com.example.app","sku":"example-sku-1"}}
	}`))
	for date, rows := range map[string][]string{thisDate: thisRows, previousDate: previousRows} {
		report := strings.Join(append([]string{insightsSalesCurrencyHeader}, rows...), "\n") + "\n"
		reg.Register(asctest.GET("/v1/salesReports", "filter[vendorNumber]=12345678", "filter[reportType]=SALES", "filter[reportSubType]=SUMMARY", "filter[frequency]="+frequency, "filter[reportDate]="+date, "filter[version]=1_0"), insightsGzipResponse(report))
	}
	return reg
}

func decodeInsightsMetrics(t *testing.T, stdout string) []any {
	t.Helper()
	var payload struct {
		Metrics []any `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	return payload.Metrics
}

// assertInsightsCurrencyMetric compares every field except the reason prose,
// which only has to name the currencies that could not be summed.
func assertInsightsCurrencyMetric(t *testing.T, got map[string]any, wantJSON string, wantReason []string) {
	t.Helper()
	reason, _ := got["reason"].(string)
	for _, currency := range wantReason {
		if !strings.Contains(reason, currency) {
			t.Errorf("expected reason to name %s, got %q", currency, reason)
		}
	}
	if len(wantReason) == 0 && reason != "" {
		t.Errorf("expected no reason, got %q", reason)
	}
	delete(got, "reason")

	var want map[string]any
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Errorf("metric mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func findMetric(t *testing.T, metrics []any, name string) map[string]any {
	t.Helper()
	for _, item := range metrics {
		metric, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if metric["name"] == name {
			return metric
		}
	}
	t.Fatalf("metric %q not found in %v", name, metrics)
	return nil
}

func insightsJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func insightsGzipResponse(report string) asctest.Responder {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write([]byte(report))
	_ = zw.Close()
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(compressed.Bytes())),
			Header:     http.Header{"Content-Type": []string{"application/a-gzip"}},
		}, nil
	}
}
