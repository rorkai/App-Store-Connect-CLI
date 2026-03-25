package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewAnalyticsClientUsesAnalyticsBaseURL(t *testing.T) {
	client := NewAnalyticsClient(&AuthSession{Client: &http.Client{}})
	if client.baseURL != analyticsAPIBaseURL {
		t.Fatalf("expected analytics base URL %q, got %q", analyticsAPIBaseURL, client.baseURL)
	}
}

func TestGetAnalyticsMeasuresRequestsExpectedEndpointAndPayload(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("unexpected method: %s", req.Method)
				}
				if req.URL.Path != "/analytics/api/v1/data/app/detail/measures" {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				if got := req.Header.Get("Referer"); !strings.Contains(got, "/apps/app-1/analytics") {
					t.Fatalf("expected analytics referer, got %q", got)
				}
				if got := req.Header.Get("X-Requested-By"); got != "appstoreconnect.apple.com" {
					t.Fatalf("expected X-Requested-By header, got %q", got)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("failed to read body: %v", err)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to decode body: %v", err)
				}
				if payload["startTime"] != "2025-12-24T00:00:00Z" || payload["endTime"] != "2026-03-23T00:00:00Z" {
					t.Fatalf("unexpected dates: %#v", payload)
				}
				measures, ok := payload["measures"].([]any)
				if !ok || len(measures) != 2 {
					t.Fatalf("unexpected measures payload: %#v", payload["measures"])
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"size": 2,
						"results": [
							{"adamId":"app-1","measure":"units","total":94,"percentChange":0.093,"data":[{"date":"2025-12-24T00:00:00Z","value":1}]},
							{"adamId":"app-1","measure":"redownloads","total":32,"percentChange":0.778,"data":[{"date":"2025-12-24T00:00:00Z","value":0}]}
						]
					}`)),
					Request: req,
				}, nil
			}),
		},
		baseURL:            analyticsAPIBaseURL,
		minRequestInterval: 0,
	}

	resp, err := client.GetAnalyticsMeasures(context.Background(), AnalyticsMeasuresRequest{
		AppID:     "app-1",
		StartDate: "2025-12-24",
		EndDate:   "2026-03-23",
		Measures:  []string{"units", "redownloads"},
		Frequency: "day",
	})
	if err != nil {
		t.Fatalf("GetAnalyticsMeasures() error = %v", err)
	}
	if resp.Size != 2 || len(resp.Results) != 2 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Results[0].Measure != "units" || resp.Results[1].Measure != "redownloads" {
		t.Fatalf("unexpected parsed measures: %#v", resp.Results)
	}
}

func TestGetAnalyticsCohortsRequestsStringAdamID(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/analytics/api/v1/data/cohorts" {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("failed to read body: %v", err)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to decode body: %v", err)
				}
				if _, ok := payload["adamId"].(string); !ok {
					t.Fatalf("expected string adamId in cohorts payload, got %#v", payload["adamId"])
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"results": {
							"date": ["2025-12-24T00:00:00Z", "2025-12-24T00:00:00Z"],
							"period": ["d1", "d7"],
							"cohort-download-to-paid-rate": [2.4, 3.42]
						}
					}`)),
					Request: req,
				}, nil
			}),
		},
		baseURL:            analyticsAPIBaseURL,
		minRequestInterval: 0,
	}

	resp, err := client.GetAnalyticsCohorts(context.Background(), AnalyticsCohortsRequest{
		AppID:     "app-1",
		StartDate: "2025-12-24",
		EndDate:   "2026-03-23",
		Measures:  []string{"cohort-download-to-paid-rate"},
		Periods:   []string{"d1", "d7"},
		Frequency: "day",
	})
	if err != nil {
		t.Fatalf("GetAnalyticsCohorts() error = %v", err)
	}
	if got := len(resp.Results["date"]); got != 2 {
		t.Fatalf("expected 2 cohort rows, got %d", got)
	}
}

func TestGetAnalyticsMeasuresUsesClientBaseURL(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "analytics.example.test" {
					t.Fatalf("unexpected host: %s", req.URL.Host)
				}
				if req.URL.Path != "/custom-base/data/app/detail/measures" {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"size":0,"results":[]}`)),
					Request:    req,
				}, nil
			}),
		},
		baseURL:            "https://analytics.example.test/custom-base",
		minRequestInterval: 0,
	}

	if _, err := client.GetAnalyticsMeasures(context.Background(), AnalyticsMeasuresRequest{
		AppID:     "app-1",
		StartDate: "2025-12-24",
		EndDate:   "2026-03-23",
		Measures:  []string{"units"},
		Frequency: "day",
	}); err != nil {
		t.Fatalf("GetAnalyticsMeasures() error = %v", err)
	}
}

func TestMonthlyRecurringRevenueRangeExcludesPartialStartMonth(t *testing.T) {
	start, end, ok, err := monthlyRecurringRevenueRange("2025-12-24", "2026-03-23")
	if err != nil {
		t.Fatalf("monthlyRecurringRevenueRange() error = %v", err)
	}
	if !ok {
		t.Fatal("expected a complete-month range")
	}
	if start != "2026-01-01" || end != "2026-02-28" {
		t.Fatalf("unexpected monthly range: %s to %s", start, end)
	}
}
