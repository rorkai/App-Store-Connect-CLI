package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionsPricesImport_InvalidBooleanReturnsUsageError(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "" +
		"territory,price,preserved\n" +
		"USA,19.99,not-a-bool\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "must be true or false") {
		t.Fatalf("expected boolean validation error, got %q", stderr)
	}
}

func TestSubscriptionsPricesImport_DryRunResolvesASCExportAliasWithoutMutations(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	seenTerritories := map[string]int{}
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected only GET in dry-run, got %s %s", req.Method, req.URL.Path)
		}
		if req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		territory := req.URL.Query().Get("filter[territory]")
		assertSubscriptionPricePointImportQuery(t, req, territory)
		seenTerritories[territory]++

		switch territory {
		case "USA":
			if req.URL.Query().Get("cursor") == "" {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/subscriptions/8000000001/pricePoints?cursor=usa-2"}}`), nil
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"19.99"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "AFG":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-afg","attributes":{"customerPrice":"299.00"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected filter[territory]=%q", territory)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "" +
		"Countries or Regions,Currency Code,Price,start_date,preserved\n" +
		"USA,USD,19.99,2026-03-01,false\n" +
		"Afghanistan,AFN,299.00,2026-03-01,true\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		DryRun  bool `json:"dryRun"`
		Total   int  `json:"total"`
		Created int  `json:"created"`
		Failed  int  `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if !summary.DryRun {
		t.Fatalf("expected dryRun=true in summary")
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if seenTerritories["USA"] == 0 || seenTerritories["AFG"] == 0 {
		t.Fatalf("expected lookups for USA and AFG, got %+v", seenTerritories)
	}
}

func TestSubscriptionsPricesImport_PartialFailureReturnsReportedErrorAndSummary(t *testing.T) {
	setupAuth(t)
	t.Chdir(t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	createCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints":
			if req.URL.Query().Get("filter[territory]") != "USA" {
				t.Fatalf("expected filter[territory]=USA, got %q", req.URL.Query().Get("filter[territory]"))
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"19.99"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			createCount++
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"pp-usa"`) {
				t.Fatalf("expected resolved price point id in payload, got %s", string(payload))
			}
			if !strings.Contains(string(payload), `"planType":"UPFRONT"`) {
				t.Fatalf("expected explicit UPFRONT plan type in payload, got %s", string(payload))
			}
			body := `{"data":{"type":"subscriptionPrices","id":"price-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "" +
		"territory,price\n" +
		"USA,19.99\n" +
		"Atlantis,9.99\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importFailure struct {
		Row int `json:"row"`
	}
	type importSummary struct {
		Total    int             `json:"total"`
		Created  int             `json:"created"`
		Failed   int             `json:"failed"`
		Failures []importFailure `json:"failures"`
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Total != 2 || summary.Created != 1 || summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Row != 2 {
		t.Fatalf("expected one failure at row 2, got %+v", summary.Failures)
	}
	if createCount != 1 {
		t.Fatalf("expected one successful create, got %d", createCount)
	}
}

func TestSubscriptionsPricesImport_SkipsExactExistingPrice(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	postCount := 0
	readCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
			assertSubscriptionPriceImportStateQuery(t, req)
			readCount++
			if req.URL.Query().Get("cursor") == "" {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/subscriptions/8000000001/prices?cursor=page-2"}}`), nil
			}
			body := `{"data":[{"type":"subscriptionPrices","id":"price-existing","attributes":{"startDate":"2026-08-01","preserved":true,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost:
			postCount++
			t.Fatalf("exact existing price must not be posted again")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "territory,price,start_date,preserved,price_point_id\nUSA,19.99,2026-08-01,true,pp-usa\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var summary struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Created != 0 || summary.Skipped != 1 || summary.Failed != 0 || postCount != 0 || readCount != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Results) != 1 || summary.Results[0].Status != "skipped" {
		t.Fatalf("expected skipped row result, got %+v", summary.Results)
	}
}

func TestSubscriptionsPricesImport_ReconcilesAmbiguousCreateWithoutReplay(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "2")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	postCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
			assertSubscriptionPriceImportStateQuery(t, req)
			readCount++
			if readCount == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
			}
			body := `{"data":[{"type":"subscriptionPrices","id":"price-created","attributes":{"startDate":"2026-08-01","preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			postCount++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"ambiguous"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "territory,price,start_date,price_point_id\nUSA,19.99,2026-08-01,pp-usa\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var summary struct {
		Reconciled int `json:"reconciled"`
		Failed     int `json:"failed"`
		Results    []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Reconciled != 1 || summary.Failed != 0 || postCount != 1 || readCount != 2 {
		t.Fatalf("unexpected recovery result: summary=%+v posts=%d reads=%d", summary, postCount, readCount)
	}
	if len(summary.Results) != 1 || summary.Results[0].Status != "reconciled" {
		t.Fatalf("expected reconciled row result, got %+v", summary.Results)
	}
}

func TestSubscriptionsPricesImport_SkipsImmediatePriceWithConcreteEffectiveDate(t *testing.T) {
	setupAuth(t)
	// 17:30 PDT on 2026-09-30: an immediate price carries the Pacific date.
	t.Cleanup(shared.SetPricingNowForTesting(func() time.Time {
		return time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC)
	}))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
			assertSubscriptionPriceImportStateQuery(t, req)
			readCount++
			body := `{"data":[{"type":"subscriptionPrices","id":"price-existing","attributes":{"startDate":"2026-09-30","preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost:
			t.Fatalf("current immediate price must not be posted again")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("territory,price,price_point_id\nUSA,19.99,pp-usa\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var summary struct {
		Skipped int `json:"skipped"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	if summary.Skipped != 1 || readCount != 1 {
		t.Fatalf("expected one indexed skip, summary=%+v reads=%d", summary, readCount)
	}
}

func TestSubscriptionsPricesImport_RetriesTimedOutInitialStateRead(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	t.Setenv("ASC_TIMEOUT", "50ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, errors.New("test transport assertion failed")
		}
		assertSubscriptionPriceImportStateQuery(t, req)
		readCount++
		if readCount == 1 {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		if err := req.Context().Err(); err != nil {
			t.Errorf("expected fresh state-read context, got %v", err)
			return nil, errors.New("test transport assertion failed")
		}
		body := `{"data":[{"type":"subscriptionPrices","id":"price-existing","attributes":{"startDate":"2026-08-01","preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}}}],"links":{}}`
		return jsonHTTPResponse(http.StatusOK, body), nil
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("territory,price,start_date,price_point_id\nUSA,19.99,2026-08-01,pp-usa\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if err := runErr; err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"skipped":1`) || readCount != 2 {
		t.Fatalf("expected fresh-context retry and skip, reads=%d output=%s", readCount, stdout)
	}
}

func TestSubscriptionsPricesImport_RetriesTimedOutPricePointRead(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	t.Setenv("ASC_TIMEOUT", "50ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, errors.New("test transport assertion failed")
		}
		assertSubscriptionPricePointImportQuery(t, req, "USA")
		readCount++
		if readCount == 1 {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		if err := req.Context().Err(); err != nil {
			t.Errorf("expected fresh price-point context, got %v", err)
			return nil, errors.New("test transport assertion failed")
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"19.99"}}],"links":{}}`), nil
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("territory,price\nUSA,19.99\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--dry-run", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if err := runErr; err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"created":1`) || readCount != 2 {
		t.Fatalf("expected fresh-context price-point retry, reads=%d output=%s", readCount, stdout)
	}
}

func TestSubscriptionsPricesImport_RetriesTimedOutSelectorResolution(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	t.Setenv("ASC_TIMEOUT", "50ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	groupReads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			groupReads++
			if groupReads == 1 {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
			if err := req.Context().Err(); err != nil {
				t.Errorf("expected fresh selector context, got %v", err)
				return nil, errors.New("test transport assertion failed")
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.monthly" {
				t.Errorf("unexpected product ID filter: %q", got)
				return nil, errors.New("test transport assertion failed")
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"8000000001","attributes":{"name":"Monthly","productId":"com.example.monthly"}}],"links":{}}`), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, errors.New("unexpected test transport request")
		}
	})

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("territory,price,price_point_id\nUSA,19.99,pp-usa\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "import",
			"--subscription-id", "com.example.monthly", "--app", "app-1", "--input", csvPath, "--dry-run", "--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if err := runErr; err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"subscriptionId":"8000000001"`) || groupReads != 2 {
		t.Fatalf("expected selector retry and resolved ID, reads=%d output=%s", groupReads, stdout)
	}
}

func TestSubscriptionsPricesImport_PrintsFailuresWhenArtifactWriteFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".asc", []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "filter[planType]=UPFRONT", "include=territory,subscriptionPricePoint", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.POST("/v1/subscriptionPrices").JSONBody(`{"data":{"type":"subscriptionPrices","attributes":{"planType":"UPFRONT"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}},"territory":{"data":{"type":"territories","id":"USA"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","detail":"invalid price"}]}`))
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "filter[planType]=UPFRONT", "include=territory,subscriptionPricePoint", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(csvPath, []byte("territory,price,price_point_id\nUSA,19.99,pp-usa\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	var summary struct {
		Failed               int    `json:"failed"`
		FailureArtifactError string `json:"failureArtifactError"`
		Results              []struct {
			Status   string `json:"status"`
			Error    string `json:"error"`
			PlanType string `json:"planType"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v\n%s", err, stdout)
	}
	if summary.Failed != 1 || summary.FailureArtifactError == "" || len(summary.Results) != 1 || summary.Results[0].Status != "failed" || summary.Results[0].Error == "" || summary.Results[0].PlanType != "UPFRONT" {
		t.Fatalf("unexpected failure summary: %+v", summary)
	}
}

func assertSubscriptionPriceImportStateQuery(t *testing.T, req *http.Request) {
	t.Helper()
	if got := req.URL.Query().Get("fields[subscriptionPrices]"); got != "startDate,preserved,planType,territory,subscriptionPricePoint" {
		t.Fatalf("unexpected subscription price fields: %q", got)
	}
	if got := req.URL.Query().Get("include"); got != "territory,subscriptionPricePoint" {
		t.Fatalf("unexpected subscription price include: %q", got)
	}
	if got := req.URL.Query().Get("filter[planType]"); got != "UPFRONT" {
		t.Fatalf("unexpected subscription price plan filter: %q", got)
	}
	if got := req.URL.Query().Get("limit"); got != "200" {
		t.Fatalf("unexpected subscription price limit: %q", got)
	}
}

func assertSubscriptionPricePointImportQuery(t *testing.T, req *http.Request, territory string) {
	t.Helper()
	if got := req.URL.Query().Get("fields[subscriptionPricePoints]"); got != "customerPrice" {
		t.Fatalf("unexpected price point fields: %q", got)
	}
	if got := req.URL.Query().Get("filter[territory]"); got != territory {
		t.Fatalf("unexpected price point territory: %q", got)
	}
	if got := req.URL.Query().Get("limit"); got != "200" {
		t.Fatalf("unexpected price point limit: %q", got)
	}
}

func TestSubscriptionsPricesImport_WritesVersionedFailureArtifact(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Chdir(t.TempDir())

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "filter[planType]=UPFRONT", "include=territory,subscriptionPricePoint", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.POST("/v1/subscriptionPrices").JSONBody(`{"data":{"type":"subscriptionPrices","attributes":{"planType":"UPFRONT"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}},"territory":{"data":{"type":"territories","id":"USA"}}}}}`),
		asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"still unavailable"}]}`))
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "filter[planType]=UPFRONT", "include=territory,subscriptionPricePoint", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	csvPath := filepath.Join(t.TempDir(), "input.csv")
	csvBody := "territory,price,price_point_id\nUSA,19.99,pp-usa\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var summary struct {
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Failed != 1 || summary.FailureArtifactPath == "" {
		t.Fatalf("expected failure artifact, got %+v", summary)
	}
	data, err := os.ReadFile(summary.FailureArtifactPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var artifact struct {
		SchemaVersion int `json:"schemaVersion"`
		Failed        int `json:"failed"`
		Results       []struct {
			Status               string `json:"status"`
			Territory            string `json:"territory"`
			Price                string `json:"price"`
			PricePointID         string `json:"pricePointId"`
			StartDate            string `json:"startDate"`
			PreserveCurrentPrice bool   `json:"preserveCurrentPrice"`
			PlanType             string `json:"planType"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if artifact.SchemaVersion != 1 || artifact.Failed != 1 || len(artifact.Results) != 1 || artifact.Results[0].Status != "failed" {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	result := artifact.Results[0]
	if result.Territory != "USA" || result.Price != "19.99" || result.PricePointID != "pp-usa" || result.StartDate != "" || result.PreserveCurrentPrice || result.PlanType != "UPFRONT" {
		t.Fatalf("artifact is missing desired price state: %+v", result)
	}
}
