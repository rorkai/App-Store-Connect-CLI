package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPricingTiersCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "tiers"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected --app required error, got %q", stderr)
	}
}

func TestPricingTiersCommand_Success(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appPricePoints") {
			body := `{
				"data":[
					{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"appPricePoints","id":"pp-299","attributes":{"customerPrice":"2.99","proceeds":"2.10"}},
					{"type":"appPricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.40"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	t.Setenv("HOME", t.TempDir())

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "tiers",
			"--app", "app-1",
			"--territory", "USA",
			"--refresh",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, "pp-099") {
		t.Fatalf("expected pp-099 in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "pp-199") {
		t.Fatalf("expected pp-199 in output, got %q", stdout)
	}
}

func TestPricingScheduleCreate_TierFlag(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var resolvedPricePointID string

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appPricePoints"):
			body := `{
				"data":[
					{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"appPricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.40"}},
					{"type":"appPricePoints","id":"pp-299","attributes":{"customerPrice":"2.99","proceeds":"2.10"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/appPriceSchedules"):
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "pp-199") {
				resolvedPricePointID = "pp-199"
			}
			resp := `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	t.Setenv("HOME", t.TempDir())

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "app-1",
			"--tier", "2",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
			"--refresh",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if resolvedPricePointID != "pp-199" {
		t.Fatalf("expected tier 2 to resolve to pp-199, got %q", resolvedPricePointID)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}

func TestPricingScheduleCreate_TierAndPricePointMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "APP",
			"--price-point", "PP",
			"--tier", "5",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestPricingScheduleCreate_AllThreeMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "APP",
			"--price-point", "PP",
			"--tier", "5",
			"--price", "4.99",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestPricingScheduleCreate_TierRequiresTerritory(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "APP",
			"--tier", "5",
			"--start-date", "2026-03-01",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--base-territory is required") {
		t.Fatalf("expected base-territory required error, got %q", stderr)
	}
}
