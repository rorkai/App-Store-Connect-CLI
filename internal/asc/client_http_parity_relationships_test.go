package asc

import (
	"context"
	"net/http"
	"testing"
)

func runSimpleGetTest(t *testing.T, expectedPath string, call func(*Client) error) {
	t.Helper()

	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := call(client); err != nil {
		t.Fatalf("request error: %v", err)
	}
}

func TestParityRelatedGETs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "GetBuildBetaAppReviewSubmission",
			path: "/v1/builds/build-1/betaAppReviewSubmission",
			call: func(client *Client) error {
				_, err := client.GetBuildBetaAppReviewSubmission(ctx, "build-1")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSimpleGetTest(t, test.path, test.call)
		})
	}
}

func TestGetAppBetaTesterUsagesMetrics_OmitsAppFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"value":"1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/apps/app-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("period") != "P7D" {
			t.Fatalf("expected period=P7D, got %q", values.Get("period"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		if values.Get("filter[apps]") != "" {
			t.Fatalf("expected filter[apps] to be empty, got %q", values.Get("filter[apps]"))
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetAppBetaTesterUsagesMetrics(
		context.Background(),
		"app-1",
		WithBetaTesterUsagesPeriod("P7D"),
		Limit[BetaTesterUsagesOption](5),
		WithBetaTesterUsagesAppID("ignored"),
	)
	if err != nil {
		t.Fatalf("GetAppBetaTesterUsagesMetrics() error: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected response data")
	}
}
