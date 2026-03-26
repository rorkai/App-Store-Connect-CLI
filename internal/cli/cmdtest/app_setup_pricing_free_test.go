package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAppSetupPricingSetFreeResolvesBaseTerritoryAndPaginatedPricePoint(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var pricePointsCalls int
	var resolvedBaseTerritoryID string
	var resolvedPricePointID string
	var resolvedStartDate string

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPriceSchedule":
			body := `{"data":{"type":"appPriceSchedules","id":"sched-current"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPriceSchedules/sched-current/baseTerritory":
			body := `{"data":{"type":"territories","id":"CAN"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPricePoints":
			pricePointsCalls++
			switch pricePointsCalls {
			case 1:
				if got := req.URL.Query().Get("filter[territory]"); got != "CAN" {
					t.Fatalf("expected filter[territory]=CAN, got %q", got)
				}
				if got := req.URL.Query().Get("limit"); got != "200" {
					t.Fatalf("expected limit=200, got %q", got)
				}
				body := `{
					"data":[
						{"type":"appPricePoints","id":"pp-paid","attributes":{"customerPrice":"0.99"}}
					],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appPricePoints?cursor=AQ&limit=200&filter%5Bterritory%5D=CAN"}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case 2:
				if got := req.URL.Query().Get("cursor"); got != "AQ" {
					t.Fatalf("expected cursor=AQ on paginated request, got %q", got)
				}
				if got := req.URL.Query().Get("filter[territory]"); got != "CAN" {
					t.Fatalf("expected paginated request to preserve territory filter, got %q", got)
				}
				body := `{
					"data":[
						{"type":"appPricePoints","id":"pp-free","attributes":{"customerPrice":"0.00"}}
					],
					"links":{"next":""}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected extra price-point lookup %d", pricePointsCalls)
				return nil, nil
			}

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
				Included []struct {
					Attributes struct {
						StartDate string `json:"startDate"`
					} `json:"attributes"`
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
				t.Fatalf("decode create payload: %v", err)
			}
			resolvedBaseTerritoryID = payload.Data.Relationships.BaseTerritory.Data.ID
			if len(payload.Included) == 0 {
				t.Fatalf("expected included manual price")
			}
			resolvedPricePointID = payload.Included[0].Relationships.AppPricePoint.Data.ID
			resolvedStartDate = payload.Included[0].Attributes.StartDate

			body := `{"data":{"type":"appPriceSchedules","id":"sched-new","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-setup", "pricing", "set",
			"--app", "app-1",
			"--free",
			"--start-date", "2026-03-01",
			"--output", "json",
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
	if pricePointsCalls != 2 {
		t.Fatalf("expected 2 paginated price-point lookups, got %d", pricePointsCalls)
	}
	if resolvedBaseTerritoryID != "CAN" {
		t.Fatalf("expected inferred base territory CAN, got %q", resolvedBaseTerritoryID)
	}
	if resolvedPricePointID != "pp-free" {
		t.Fatalf("expected resolved free price point pp-free, got %q", resolvedPricePointID)
	}
	if resolvedStartDate != "2026-03-01" {
		t.Fatalf("expected start date 2026-03-01, got %q", resolvedStartDate)
	}
	if !strings.Contains(stdout, `"id":"sched-new"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}
