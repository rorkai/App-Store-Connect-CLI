package asc

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestPerformanceOverviewRejectsInvalidPayload(t *testing.T) {
	for _, body := range []string{"not-json", "null", "[]"} {
		t.Run(body, func(t *testing.T) {
			client := newTestClient(t, nil, jsonResponse(http.StatusOK, body))
			if _, err := client.GetPerformanceOverviewForApp(context.Background(), "123", nil); err == nil {
				t.Fatalf("accepted invalid overview %s", body)
			}
		})
	}
}

func TestPerformanceOverviewEmptyAndAPIError(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
	}, jsonResponse(http.StatusOK, `{}`))
	resp, err := client.GetPerformanceOverviewForApp(context.Background(), "123", nil)
	if err != nil || string(resp.Data) != "{}" {
		t.Fatalf("empty overview: %v %v", resp, err)
	}
	client = newTestClient(t, nil, jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Not permitted"}]}`))
	if _, err := client.GetPerformanceOverviewForApp(context.Background(), "123", nil); err == nil || !strings.Contains(err.Error(), "Not permitted") {
		t.Fatalf("API error lost: %v", err)
	}
}
