package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSubscriptionAdjustedEqualizationsSanitizesKnownConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errors":[{"code":"STATE_ERROR.EQUALIZATION_FAILED","detail":"No compatible price point","meta":{"associatedErrors":{"prices":[{"code":"STATE_ERROR.NO_TIER_IN_TERRITORY","detail":"DEU"},{"code":"STATE_ERROR.NO_TIER_IN_TERRITORY","detail":"FRA"}]}}}]}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	result, err := client.GetSubscriptionAdjustedEqualizations(context.Background(), "point-1", "monthly")
	if err != nil {
		t.Fatalf("GetSubscriptionAdjustedEqualizations() error = %v", err)
	}
	if result.Available || result.Status != http.StatusConflict || result.MissingTerritoryCount != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.MissingTerritories) != 2 || result.MissingTerritories[0] != "DEU" || result.MissingTerritories[1] != "FRA" {
		t.Fatalf("unexpected territories: %#v", result.MissingTerritories)
	}
}

func TestGetSubscriptionAdjustedEqualizationsRejectsUpfrontPlanType(t *testing.T) {
	client := &Client{}
	_, err := client.GetSubscriptionAdjustedEqualizations(context.Background(), "point-1", "UPFRONT")
	if err == nil || !strings.Contains(err.Error(), `plan type must be "MONTHLY"`) {
		t.Fatalf("expected MONTHLY-only error, got %v", err)
	}
}
