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
)

func TestSubscriptionsPricesImport_InvalidBooleanReturnsUsageError(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

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
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "sub-1", "--input", csvPath, "--dry-run"}); err != nil {
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
		if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		territory := req.URL.Query().Get("filter[territory]")
		seenTerritories[territory]++

		switch territory {
		case "USA":
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
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "sub-1", "--input", csvPath, "--dry-run"}); err != nil {
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	createCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			if req.URL.Query().Get("filter[territory]") != "USA" {
				t.Fatalf("expected filter[territory]=USA, got %q", req.URL.Query().Get("filter[territory]"))
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"19.99"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			createCount++
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"pp-usa"`) {
				t.Fatalf("expected resolved price point id in payload, got %s", string(payload))
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
		if err := root.Parse([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "sub-1", "--input", csvPath}); err != nil {
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
