package insights

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNormalizeWeekStart(t *testing.T) {
	parsed, err := normalizeWeekStart("2026-02-16")
	if err != nil {
		t.Fatalf("normalizeWeekStart error: %v", err)
	}
	if parsed.Format("2006-01-02") != "2026-02-16" {
		t.Fatalf("unexpected week start %q", parsed.Format("2006-01-02"))
	}

	if _, err := normalizeWeekStart("2026-2-16"); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestAnalyticsReportRequestIsActiveUsesCurrentAttribute(t *testing.T) {
	active := false
	stopped := true
	if !analyticsReportRequestIsActive(asc.AnalyticsReportRequestAttributes{StoppedDueToInactivity: &active}) {
		t.Fatal("expected stoppedDueToInactivity=false to be active")
	}
	if analyticsReportRequestIsActive(asc.AnalyticsReportRequestAttributes{StoppedDueToInactivity: &stopped}) {
		t.Fatal("expected stoppedDueToInactivity=true to be inactive")
	}
	if !analyticsReportRequestIsActive(asc.AnalyticsReportRequestAttributes{}) {
		t.Fatal("expected absent stoppedDueToInactivity to remain compatible and active")
	}
}

func TestParseSalesReportMetrics(t *testing.T) {
	report := strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tProduct Type Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\tChromism12345\t1500196580\t\t1F\t \t10\t0.00\t0.00",
		"foo\tChromism12345\t1500196580\t\t3F\t \t4\t0.00\t0.00",
		"foo\tChromism12345\t1500196580\t\t7F\t \t6\t0.00\t0.00",
		"foo\tChromism12345\t1500196580\t\tF1\t \t2\t0.00\t0.00",
		"foo\tcom.rudrankriyam.chroma_plus\t1619633372\tChromism12345\tIAY\tRenewal\t3\t0.75\t1.00",
		"foo\tcom.rudrankriyam.chroma_plus\t1619633372\tChromism12345\tIAY\tNew\t2\t1.25\t2.00",
		"foo\tother\t999999\tOtherSKU\t1F\tRenewal\t500\t9.99\t9.99",
		"",
	}, "\n")

	compressed := gzipText(t, report)
	metrics, err := ParseSalesReportMetrics(bytes.NewReader(compressed), salesScope{
		AppID:  "1500196580",
		AppSKU: "Chromism12345",
	})
	if err != nil {
		t.Fatalf("ParseSalesReportMetrics error: %v", err)
	}

	if metrics.RowCount != 6 {
		t.Fatalf("expected RowCount=6, got %d", metrics.RowCount)
	}
	if !metrics.UnitsColumnPresent || metrics.UnitsTotal != 27 {
		t.Fatalf("unexpected units totals: %+v", metrics)
	}
	if !metrics.DownloadUnitsAvailable {
		t.Fatal("expected download units to be available")
	}
	if metrics.DownloadUnitsTotal != 12 {
		t.Fatalf("unexpected download units totals: %+v", metrics)
	}
	if metrics.MonetizedUnitsTotal != 5 {
		t.Fatalf("unexpected monetized units totals: %+v", metrics)
	}
	// Money columns are per unit: 3 x 0.75 + 2 x 1.25 and 3 x 1.00 + 2 x 2.00.
	// This report has no currency columns, so totals land under the empty code.
	if !metrics.DeveloperProceedsColumnPresent || metrics.DeveloperProceedsByCurrency[""] != 4.75 {
		t.Fatalf("unexpected developer proceeds totals: %+v", metrics)
	}
	if !metrics.CustomerPriceColumnPresent || metrics.CustomerPriceByCurrency[""] != 7 {
		t.Fatalf("unexpected customer price totals: %+v", metrics)
	}
	if !metrics.SubscriptionColumnPresent || metrics.SubscriptionRows != 2 || metrics.SubscriptionUnitsTotal != 5 {
		t.Fatalf("unexpected subscription totals: %+v", metrics)
	}
	if metrics.RenewalRows != 1 || metrics.RenewalUnitsTotal != 3 || metrics.RenewalDeveloperProceedsByCurrency[""] != 2.25 {
		t.Fatalf("unexpected renewal totals: %+v", metrics)
	}
}

func TestIsInitialAppDownloadProductType(t *testing.T) {
	tests := []struct {
		name        string
		productType string
		want        bool
	}{
		{name: "app", productType: "1", want: true},
		{name: "app bundle", productType: "1-B", want: true},
		{name: "custom iOS app", productType: "1E", want: true},
		{name: "custom iPadOS app", productType: "1EP", want: true},
		{name: "custom universal app", productType: "1EU", want: true},
		{name: "universal app", productType: "1F", want: true},
		{name: "iPad app", productType: "1T", want: true},
		{name: "Mac app", productType: "F1", want: true},
		{name: "Mac app bundle", productType: "F1-B", want: true},
		{name: "redownload", productType: "3", want: false},
		{name: "universal redownload", productType: "3F", want: false},
		{name: "update", productType: "7", want: false},
		{name: "universal update", productType: "7F", want: false},
		{name: "iPad update", productType: "7T", want: false},
		{name: "Mac update", productType: "F7", want: false},
		{name: "in-app purchase", productType: "IA1", want: false},
		{name: "unknown", productType: "future", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInitialAppDownloadProductType(tt.productType); got != tt.want {
				t.Fatalf("isInitialAppDownloadProductType(%q) = %v, want %v", tt.productType, got, tt.want)
			}
		})
	}
}

func TestParseSalesReportMetricsMarksDownloadUnitsUnavailableWithoutProductType(t *testing.T) {
	report := strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits",
		"foo\tChromism12345\t1500196580\t\t10",
	}, "\n")

	metrics, err := ParseSalesReportMetrics(bytes.NewReader(gzipText(t, report)), salesScope{
		AppID:  "1500196580",
		AppSKU: "Chromism12345",
	})
	if err != nil {
		t.Fatalf("ParseSalesReportMetrics error: %v", err)
	}
	if metrics.DownloadUnitsAvailable {
		t.Fatal("expected download units to be unavailable without Product Type Identifier")
	}
	if metrics.DownloadUnitsTotal != 0 {
		t.Fatalf("expected no inferred download units, got %.2f", metrics.DownloadUnitsTotal)
	}
	if metrics.UnitsTotal != 10 {
		t.Fatalf("expected all units to remain available, got %.2f", metrics.UnitsTotal)
	}
}

func TestContainsDate(t *testing.T) {
	window := weekWindowFromStart(time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC))

	if !containsDate(window, time.Date(2026, 2, 16, 15, 0, 0, 0, time.UTC)) {
		t.Fatal("expected first day to be in range")
	}
	if !containsDate(window, time.Date(2026, 2, 22, 23, 59, 0, 0, time.UTC)) {
		t.Fatal("expected last day to be in range")
	}
	if containsDate(window, time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected next week date to be out of range")
	}
}

func TestWeekWindowProcessingDates(t *testing.T) {
	thisWeek := weekWindowFromStart(time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC))
	previousWeek := weekWindowFromStart(time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC))

	dates := weekWindowProcessingDates(thisWeek, previousWeek)
	if len(dates) != 14 {
		t.Fatalf("expected 14 processing dates, got %d (%v)", len(dates), dates)
	}
	if dates[0] != "2026-02-16" || dates[6] != "2026-02-22" {
		t.Fatalf("unexpected this-week dates: %v", dates[:7])
	}
	if dates[7] != "2026-02-09" || dates[13] != "2026-02-15" {
		t.Fatalf("unexpected previous-week dates: %v", dates[7:])
	}

	if overlapping := weekWindowProcessingDates(thisWeek, thisWeek); len(overlapping) != 7 {
		t.Fatalf("expected duplicate windows to collapse to 7 dates, got %d", len(overlapping))
	}
}

func TestCollectAnalyticsMetricsFiltersDatesAndFollowsNextPage(t *testing.T) {
	thisWeek := weekWindowFromStart(time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC))
	previousWeek := weekWindowFromStart(time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC))

	var instancePages int
	client := newInsightsTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/analyticsReportRequests"):
			return jsonInsightsResponse(req, `{"data":[{"type":"analyticsReportRequests","id":"request-1","attributes":{}}],"links":{}}`), nil
		case strings.HasSuffix(req.URL.Path, "/reports"):
			return jsonInsightsResponse(req, `{"data":[{"type":"analyticsReports","id":"report-1","attributes":{}}],"links":{}}`), nil
		case strings.HasSuffix(req.URL.Path, "/instances"):
			instancePages++
			if instancePages == 1 {
				query := req.URL.Query()
				if got := query.Get("filter[granularity]"); got != "DAILY,WEEKLY,MONTHLY" {
					t.Errorf("filter[granularity] = %q, want DAILY,WEEKLY,MONTHLY", got)
				}
				gotDates := strings.Split(query.Get("filter[processingDate]"), ",")
				if len(gotDates) != 14 {
					t.Errorf("filter[processingDate] = %q, want 14 dates", query.Get("filter[processingDate]"))
				}
				if !slices.Contains(gotDates, "2026-02-09") || !slices.Contains(gotDates, "2026-02-22") {
					t.Errorf("filter[processingDate] = %q, want both comparison windows", query.Get("filter[processingDate]"))
				}
				if got := query.Get("limit"); got != "200" {
					t.Errorf("limit = %q, want 200", got)
				}
				return jsonInsightsResponse(req, `{
					"data":[{"type":"analyticsReportInstances","id":"instance-1","attributes":{"granularity":"DAILY","processingDate":"2026-02-17"}}],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/instances?cursor=PAGE2"}
				}`), nil
			}
			if got := req.URL.Query().Get("cursor"); got != "PAGE2" {
				t.Errorf("second instances page cursor = %q, want PAGE2", got)
			}
			return jsonInsightsResponse(req, `{
				"data":[{"type":"analyticsReportInstances","id":"instance-2","attributes":{"granularity":"WEEKLY","processingDate":"2026-02-15"}}],
				"links":{}
			}`), nil
		default:
			t.Errorf("unexpected request path %q", req.URL.Path)
			return jsonInsightsResponse(req, `{"data":[],"links":{}}`), nil
		}
	})

	metrics, requestCount, err := collectAnalyticsMetrics(context.Background(), client, "123", thisWeek, previousWeek)
	if err != nil {
		t.Fatalf("collectAnalyticsMetrics error: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 active request, got %d", requestCount)
	}
	if instancePages != 2 {
		t.Fatalf("expected both instance pages to be fetched, got %d", instancePages)
	}

	instancesMetric := findWeeklyMetric(t, metrics, "instances_available")
	if instancesMetric.ThisWeek == nil || *instancesMetric.ThisWeek != 1 {
		t.Fatalf("unexpected this-week instances: %+v", instancesMetric)
	}
	if instancesMetric.LastWeek == nil || *instancesMetric.LastWeek != 1 {
		t.Fatalf("unexpected last-week instances from the second page: %+v", instancesMetric)
	}
}

func TestCollectAnalyticsMetricsFollowsRequestAndReportPages(t *testing.T) {
	thisWeek := weekWindowFromStart(time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC))
	previousWeek := weekWindowFromStart(time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC))

	var requestPages, reportPages int
	client := newInsightsTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/analyticsReportRequests"):
			requestPages++
			if req.URL.Query().Get("cursor") == "REQUESTS2" {
				return jsonInsightsResponse(req, `{"data":[{"type":"analyticsReportRequests","id":"request-2","attributes":{}}],"links":{}}`), nil
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Errorf("requests limit = %q, want 200", got)
			}
			return jsonInsightsResponse(req, `{
				"data":[{"type":"analyticsReportRequests","id":"request-1","attributes":{}}],
				"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/123/analyticsReportRequests?cursor=REQUESTS2"}
			}`), nil
		case strings.HasSuffix(req.URL.Path, "/request-1/reports"):
			reportPages++
			if req.URL.Query().Get("cursor") == "REPORTS2" {
				return jsonInsightsResponse(req, `{
					"data":[{"type":"analyticsReports","id":"report-2","attributes":{}}],
					"links":{}
				}`), nil
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Errorf("reports limit = %q, want 200", got)
			}
			return jsonInsightsResponse(req, `{
				"data":[{"type":"analyticsReports","id":"report-1","attributes":{}}],
				"links":{"next":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports?cursor=REPORTS2"}
			}`), nil
		case strings.HasSuffix(req.URL.Path, "/request-2/reports"):
			return jsonInsightsResponse(req, `{"data":[],"links":{}}`), nil
		case strings.HasSuffix(req.URL.Path, "/report-1/instances"):
			return jsonInsightsResponse(req, `{
				"data":[{"type":"analyticsReportInstances","id":"instance-1","attributes":{"granularity":"DAILY","processingDate":"2026-02-17"}}],
				"links":{}
			}`), nil
		case strings.HasSuffix(req.URL.Path, "/report-2/instances"):
			return jsonInsightsResponse(req, `{
				"data":[{"type":"analyticsReportInstances","id":"instance-2","attributes":{"granularity":"DAILY","processingDate":"2026-02-10"}}],
				"links":{}
			}`), nil
		default:
			t.Errorf("unexpected request path %q", req.URL.Path)
			return jsonInsightsResponse(req, `{"data":[],"links":{}}`), nil
		}
	})

	metrics, requestCount, err := collectAnalyticsMetrics(context.Background(), client, "123", thisWeek, previousWeek)
	if err != nil {
		t.Fatalf("collectAnalyticsMetrics error: %v", err)
	}
	if requestPages != 2 {
		t.Fatalf("request page requests = %d, want 2", requestPages)
	}
	if reportPages != 2 {
		t.Fatalf("report page requests = %d, want 2", reportPages)
	}
	if requestCount != 2 {
		t.Fatalf("active request count = %d, want 2", requestCount)
	}

	reportsMetric := findWeeklyMetric(t, metrics, "reports_available")
	if reportsMetric.ThisWeek == nil || *reportsMetric.ThisWeek != 1 {
		t.Fatalf("unexpected this-week reports: %+v", reportsMetric)
	}
	if reportsMetric.LastWeek == nil || *reportsMetric.LastWeek != 1 {
		t.Fatalf("unexpected last-week reports from second page: %+v", reportsMetric)
	}

	instancesMetric := findWeeklyMetric(t, metrics, "instances_available")
	if instancesMetric.ThisWeek == nil || *instancesMetric.ThisWeek != 1 {
		t.Fatalf("unexpected this-week instances: %+v", instancesMetric)
	}
	if instancesMetric.LastWeek == nil || *instancesMetric.LastWeek != 1 {
		t.Fatalf("unexpected last-week instances from second-page report: %+v", instancesMetric)
	}
}

func TestFetchAnalyticsReportInstancesRejectsEquivalentRepeatedNext(t *testing.T) {
	const instancesPath = "/v1/analyticsReports/report-1/instances"

	tests := []struct {
		name      string
		firstNext string
		pageNext  string
	}{
		{
			name:      "whitespace",
			firstNext: instancesPath + "?cursor=abc",
			pageNext:  "  " + instancesPath + "?cursor=abc  ",
		},
		{
			name:      "relative and same-host absolute",
			firstNext: instancesPath + "?cursor=abc",
			pageNext:  asc.BaseURL + instancesPath + "?cursor=abc",
		},
		{
			name:      "reordered query parameters",
			firstNext: instancesPath + "?cursor=abc&limit=200",
			pageNext:  instancesPath + "?limit=200&cursor=abc",
		},
		{
			name:      "combined absolute whitespace and reordered query parameters",
			firstNext: instancesPath + "?cursor=abc&limit=200",
			pageNext:  " " + asc.BaseURL + instancesPath + "?limit=200&cursor=abc ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instancePages := 0
			client := newInsightsTestClient(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != instancesPath {
					t.Errorf("request path = %q, want %q", req.URL.Path, instancesPath)
				}
				instancePages++
				switch instancePages {
				case 1:
					return jsonInsightsResponse(req, `{"data":[{"type":"analyticsReportInstances","id":"instance-1"}],"links":{"next":"`+tt.firstNext+`"}}`), nil
				case 2:
					return jsonInsightsResponse(req, `{"data":[{"type":"analyticsReportInstances","id":"instance-2"}],"links":{"next":"`+tt.pageNext+`"}}`), nil
				default:
					return jsonInsightsResponse(req, `{"data":[],"links":{}}`), nil
				}
			})

			_, err := fetchAnalyticsReportInstances(context.Background(), client, "report-1")
			if err == nil || !strings.Contains(err.Error(), "detected repeated analytics report instance pagination URL") {
				t.Fatalf("fetchAnalyticsReportInstances() error = %v, want repeated-pagination error", err)
			}
			if instancePages != 2 {
				t.Fatalf("instance page requests = %d, want 2 without a third fetch", instancePages)
			}
		})
	}
}

func findWeeklyMetric(t *testing.T, metrics []weeklyMetric, name string) weeklyMetric {
	t.Helper()

	for _, metric := range metrics {
		if metric.Name == name {
			return metric
		}
	}
	t.Fatalf("metric %q not found in %+v", name, metrics)
	return weeklyMetric{}
}

type insightsRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn insightsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newInsightsTestClient(t *testing.T, fn insightsRoundTripFunc) *asc.Client {
	t.Helper()

	oldTransport := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
	})

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(insightsTestPrivateKeyPEM(t)))
	if err != nil {
		t.Fatalf("failed to create ASC client: %v", err)
	}
	return client
}

func insightsTestPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func jsonInsightsResponse(req *http.Request, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func gzipText(t *testing.T, value string) []byte {
	t.Helper()

	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write([]byte(value)); err != nil {
		t.Fatalf("gzip write error: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close error: %v", err)
	}
	return out.Bytes()
}

// salesSummaryHeader is the column order of Apple's Sales Summary report.
const salesSummaryHeader = "Provider\tProvider Country\tSKU\tDeveloper\tTitle\tVersion\tProduct Type Identifier\tUnits\tDeveloper Proceeds\tBegin Date\tEnd Date\tCustomer Currency\tCountry Code\tCurrency of Proceeds\tApple Identifier\tCustomer Price\tPromo Code\tParent Identifier\tSubscription\tPeriod\tCategory\tCMB\tDevice\tSupported Platforms\tProceeds Reason\tPreserved Pricing\tClient\tOrder Type"

// salesSummaryRow fills one Sales Summary row for app 1500196580 (SKU Chromism12345).
func salesSummaryRow(productType, units, proceeds, customerCurrency, proceedsCurrency, customerPrice, parent, subscription string) string {
	appleID := "1500196580"
	sku := "Chromism12345"
	if parent != "" {
		appleID = "1619633372"
		sku = "com.rudrankriyam.chroma_plus"
	}
	return strings.Join([]string{
		"APPLE", "US", sku, "Developer", "Chroma", "1.0", productType, units, proceeds,
		"02/16/2026", "02/22/2026", customerCurrency, "US", proceedsCurrency, appleID, customerPrice,
		"", parent, subscription, "", "Productivity", "", "iPhone", "iOS", "", "", "", "",
	}, "\t")
}

func TestSalesMoneyMetricsRoundToCentsAndRefuseBlankCurrency(t *testing.T) {
	// 3 x 0.70 + 1 x 0.10 is 2.1999999999999997 in float64 and 2.20 in the report.
	lastWeek := []string{
		salesSummaryRow("1F", "1", "1.10", "USD", "USD", "1.50", "", ""),
	}
	tests := []struct {
		name     string
		thisWeek []string
		want     map[string]string
	}{
		{
			name: "amounts and deltas are whole cents",
			thisWeek: []string{
				salesSummaryRow("1F", "3", "0.70", "USD", "USD", "0.99", "", ""),
				salesSummaryRow("1F", "1", "0.10", "USD", "USD", "0.20", "", ""),
			},
			want: map[string]string{
				"developer_proceeds": `{"name":"developer_proceeds","unit":"currency","currency":"USD","thisWeek":2.2,"lastWeek":1.1,
					"delta":1.1,"deltaPercent":100,"status":"ok",
					"byCurrency":[{"currency":"USD","thisWeek":2.2,"lastWeek":1.1,"delta":1.1,"deltaPercent":100}]}`,
				"customer_price": `{"name":"customer_price","unit":"currency","currency":"USD","thisWeek":3.17,"lastWeek":1.5,
					"delta":1.67,"deltaPercent":111.33333333333333,"status":"ok",
					"byCurrency":[{"currency":"USD","thisWeek":3.17,"lastWeek":1.5,"delta":1.67,"deltaPercent":111.33333333333333}]}`,
			},
		},
		{
			name: "a blank currency of proceeds on a paid row makes proceeds unavailable",
			thisWeek: []string{
				salesSummaryRow("1F", "3", "0.70", "USD", "USD", "0.99", "", ""),
				salesSummaryRow("1F", "2", "0.70", "USD", "", "0.99", "", ""),
				// A free row has no amount, so its blank currency does not matter.
				salesSummaryRow("1F", "5", "0.00", "", "", "0.00", "", ""),
			},
			want: map[string]string{
				"developer_proceeds": `{"name":"developer_proceeds","unit":"currency","status":"unavailable",
					"reason":"developer proceeds cannot be totaled: 1 row this week has an amount but a blank Currency of Proceeds"}`,
				"customer_price": `{"name":"customer_price","unit":"currency","currency":"USD","thisWeek":4.95,"lastWeek":1.5,
					"delta":3.45,"deltaPercent":230.00000000000003,"status":"ok",
					"byCurrency":[{"currency":"USD","thisWeek":4.95,"lastWeek":1.5,"delta":3.45,"deltaPercent":230.00000000000003}]}`,
			},
		},
		{
			name: "a blank customer currency makes customer price unavailable",
			thisWeek: []string{
				salesSummaryRow("1F", "3", "0.70", "", "USD", "0.99", "", ""),
				salesSummaryRow("1F", "1", "0.70", "", "USD", "0.99", "", ""),
			},
			want: map[string]string{
				"developer_proceeds": `{"name":"developer_proceeds","unit":"currency","currency":"USD","thisWeek":2.8,"lastWeek":1.1,
					"delta":1.7,"deltaPercent":154.54545454545453,"status":"ok",
					"byCurrency":[{"currency":"USD","thisWeek":2.8,"lastWeek":1.1,"delta":1.7,"deltaPercent":154.54545454545453}]}`,
				"customer_price": `{"name":"customer_price","unit":"currency","status":"unavailable",
					"reason":"customer price cannot be totaled: 2 rows this week have an amount but a blank Customer Currency"}`,
			},
		},
	}

	scope := salesScope{AppID: "1500196580", AppSKU: "Chromism12345"}
	parse := func(rows []string) salesWeekMetrics {
		t.Helper()
		report := salesSummaryHeader + "\n" + strings.Join(rows, "\n") + "\n"
		metrics, err := ParseSalesReportMetrics(bytes.NewReader(gzipText(t, report)), scope)
		if err != nil {
			t.Fatalf("ParseSalesReportMetrics error: %v", err)
		}
		return metrics
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := collectSalesMetrics(parse(tt.thisWeek), nil, parse(lastWeek), nil)
			for name, want := range tt.want {
				assertJSONEqual(t, findWeeklyMetric(t, metrics, name), want)
			}
		})
	}
}

func TestDailySubscriptionProceedsRefuseBlankCurrency(t *testing.T) {
	// Subscription and renewal proceeds come from the same rows; a blank
	// Currency of Proceeds on a renewal makes both unavailable for the day.
	report := strings.Join([]string{
		salesSummaryHeader,
		salesSummaryRow("IAY", "3", "0.70", "USD", "USD", "0.99", "Chromism12345", "Renewal"),
		salesSummaryRow("IAY", "1", "0.70", "USD", "", "0.99", "Chromism12345", "Renewal"),
		"",
	}, "\n")
	thisDay, err := ParseSalesReportMetrics(bytes.NewReader(gzipText(t, report)), salesScope{AppID: "1500196580", AppSKU: "Chromism12345"})
	if err != nil {
		t.Fatalf("ParseSalesReportMetrics error: %v", err)
	}
	metrics := dailySalesMetrics(thisDay, salesWeekMetrics{UnitsColumnPresent: true, DeveloperProceedsColumnPresent: true, SubscriptionColumnPresent: true}, "")
	for _, name := range []string{"renewal_developer_proceeds", "subscription_developer_proceeds"} {
		label := strings.ReplaceAll(strings.TrimSuffix(name, "_developer_proceeds"), "_", " ")
		want := `{"name":"` + name + `","unit":"currency","status":"unavailable",
			"reason":"` + label + ` developer proceeds cannot be totaled: 1 row on the selected day has an amount but a blank Currency of Proceeds"}`
		for _, metric := range metrics {
			if metric.Name == name {
				assertJSONEqual(t, metric, want)
			}
		}
	}
}

func assertJSONEqual(t *testing.T, value any, wantJSON string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var got, want any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON mismatch\n got: %s\nwant: %s", data, wantJSON)
	}
}
