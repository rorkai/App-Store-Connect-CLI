package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPricingScheduleCreateResolvesPriceUsingNumericMatch(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var pricePointsCalls int
	var resolvedPricePointID string

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPricePoints":
			pricePointsCalls++
			if got := req.URL.Query().Get("filter[territory]"); got != "USA" {
				t.Fatalf("expected filter[territory]=USA, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected limit=200, got %q", got)
			}
			body := `{
				"data":[
					{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPriceSchedules":
			var payload struct {
				Included []struct {
					Relationships struct {
						AppPricePoint struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"appPricePoint"`
					} `json:"relationships"`
				} `json:"included"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode create payload: %v", err)
			}
			if len(payload.Included) == 0 {
				t.Fatalf("expected included app price data in create payload")
			}
			resolvedPricePointID = payload.Included[0].Relationships.AppPricePoint.Data.ID

			body := `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`
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

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "app-1",
			"--price", "0.990",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
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
	if pricePointsCalls != 1 {
		t.Fatalf("expected one price-point lookup call, got %d", pricePointsCalls)
	}
	if resolvedPricePointID != "pp-099" {
		t.Fatalf("expected resolved price point pp-099, got %q", resolvedPricePointID)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}

func TestPricingScheduleCreateNormalizesBaseTerritory(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPricePoints":
			if got := req.URL.Query().Get("filter[territory]"); got != "USA" {
				t.Fatalf("expected normalized filter[territory]=USA, got %q", got)
			}
			body := `{
				"data":[
					{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPriceSchedules":
			var payload struct {
				Data struct {
					Relationships struct {
						BaseTerritory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"baseTerritory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode create payload: %v", err)
			}
			if got := payload.Data.Relationships.BaseTerritory.Data.ID; got != "USA" {
				t.Fatalf("expected normalized base territory USA, got %q", got)
			}

			body := `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`
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

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"pricing", "schedule", "create",
		"--app", "app-1",
		"--price", "0.99",
		"--base-territory", "United States",
		"--start-date", "2026-03-01",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
