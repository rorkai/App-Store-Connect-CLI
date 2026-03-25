package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetAnalyticsTimeseriesRequestsExpectedGroupPayload(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("unexpected method: %s", req.Method)
				}
				if req.URL.Path != "/analytics/api/v1/data/timeseries" {
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
				group, ok := payload["group"].(map[string]any)
				if !ok {
					t.Fatalf("expected group payload, got %#v", payload["group"])
				}
				if group["dimension"] != "source" || group["metric"] != "pageViewUnique" {
					t.Fatalf("unexpected group payload: %#v", group)
				}
				return analyticsTestJSONResponse(req, `{
					"size": 1,
					"results": [
						{"adamId":"app-1","group":{"key":"Other","title":"App Store Browse"},"data":[{"date":"2025-12-24T00:00:00Z","pageViewUnique":2}]}
					]
				}`), nil
			}),
		},
		baseURL:            analyticsAPIBaseURL,
		minRequestInterval: 0,
	}

	resp, err := client.GetAnalyticsTimeseries(context.Background(), AnalyticsTimeseriesRequest{
		AppID:     "app-1",
		StartDate: "2025-12-24",
		EndDate:   "2026-03-23",
		Measures:  []string{"pageViewUnique"},
		Frequency: "day",
		Group: &AnalyticsTimeseriesGroup{
			Metric:    "pageViewUnique",
			Dimension: "source",
			Rank:      "DESCENDING",
			Limit:     10,
		},
	})
	if err != nil {
		t.Fatalf("GetAnalyticsTimeseries() error = %v", err)
	}
	if resp.Size != 1 || len(resp.Results) != 1 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestGetAnalyticsSourcesListRequestsExpectedEndpointAndPayload(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/analytics/api/v1/data/sources/list" {
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
				if payload["dimension"] != "campaignId" {
					t.Fatalf("unexpected dimension: %#v", payload["dimension"])
				}
				if payload["limit"] != float64(1) {
					t.Fatalf("unexpected limit: %#v", payload["limit"])
				}
				return analyticsTestJSONResponse(req, `{"size":0,"results":[],"meetsThreshold":true}`), nil
			}),
		},
		baseURL:            analyticsAPIBaseURL,
		minRequestInterval: 0,
	}

	resp, err := client.GetAnalyticsSourcesList(context.Background(), AnalyticsSourcesListRequest{
		AppID:     "app-1",
		StartDate: "2025-12-24",
		EndDate:   "2026-03-23",
		Measures:  []string{"impressionsTotal", "totalDownloads"},
		Dimension: "campaignId",
		Frequency: "day",
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("GetAnalyticsSourcesList() error = %v", err)
	}
	if resp.Size != 0 {
		t.Fatalf("expected empty result, got %#v", resp)
	}
}

func TestGetAnalyticsBenchmarksMergesAppAndPercentiles(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/analytics/api/v1/settings/all":
					return analyticsTestJSONResponse(req, `{
						"configuration": {
							"dataStartDate": "2015-04-01T00:00:00Z",
							"dataEndDate": "2026-03-23T00:00:00Z",
							"benchmarkEndDate": "2026-03-02T00:00:00Z"
						}
					}`), nil
				case "/analytics/api/v1/app-info/app-1":
					return analyticsTestJSONResponse(req, `{
						"size": 1,
						"results": [
							{
								"adamId": "app-1",
								"features": [
									{"id":"inAppPurchases.type.subscriptions"}
								]
							}
						]
					}`), nil
				case "/analytics/api/v2/data/dimension-values":
					return analyticsTestJSONResponse(req, `{
						"size": 1,
						"results": [
							{
								"adamId": "app-1",
								"dimension": "peerGroupId",
								"actualSize": 2,
								"values": [
									{"id":"202","category":"GENRE_6027","size":"ALL","memberOf":true,"primary":true},
									{"id":"74","category":"GENRE_6027","size":"ALL","memberOf":true,"primary":true}
								]
							}
						]
					}`), nil
				case "/analytics/api/v2/data/time-series":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("failed to read body: %v", err)
					}
					switch {
					case strings.Contains(string(body), `"benchConversionRate"`):
						return analyticsTestJSONResponse(req, `{
							"size": 1,
							"results": [
								{
									"adamId": "app-1",
									"data": [
										{
											"date": "2026-02-23",
											"benchConversionRate": {"p25":1.42,"p50":3.06,"p75":6.99},
											"benchCrashRate": {"p25":0,"p50":0.07,"p75":0.4}
										}
									]
								}
							]
						}`), nil
					default:
						return analyticsTestJSONResponse(req, `{
							"size": 1,
							"results": [
								{
									"adamId": "app-1",
									"data": [
										{
											"date": "2026-02-23",
											"conversionRate": 0.62,
											"crashRate": 7.69
										}
									]
								}
							]
						}`), nil
					}
				default:
					t.Fatalf("unexpected path: %s", req.URL.Path)
					return nil, nil
				}
			}),
		},
		baseURL:            analyticsAPIBaseURL,
		minRequestInterval: 0,
	}

	resp, err := client.GetAnalyticsBenchmarks(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("GetAnalyticsBenchmarks() error = %v", err)
	}
	if resp.Category != "GENRE_6027" {
		t.Fatalf("unexpected category: %#v", resp)
	}
	if len(resp.PeerGroupIDs) != 2 || resp.PeerGroupIDs[0] != "202" || resp.PeerGroupIDs[1] != "74" {
		t.Fatalf("unexpected peer groups: %#v", resp.PeerGroupIDs)
	}
	var conversion *AnalyticsBenchmarkMetric
	for i := range resp.Metrics {
		if resp.Metrics[i].Key == "conversionRate" {
			conversion = &resp.Metrics[i]
			break
		}
	}
	if conversion == nil {
		t.Fatalf("expected conversionRate metric, got %#v", resp.Metrics)
	}
	if conversion.AppValue == nil || *conversion.AppValue != 0.62 {
		t.Fatalf("unexpected app value: %#v", conversion)
	}
	if conversion.P50 == nil || *conversion.P50 != 3.06 {
		t.Fatalf("unexpected percentile value: %#v", conversion)
	}
}

func analyticsTestJSONResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
