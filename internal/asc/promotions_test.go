package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateAppStoreVersionPromotion_RequiresTreatment(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatal("unexpected request")
	}, jsonResponse(http.StatusOK, `{"data":{}}`))

	_, err := client.CreateAppStoreVersionPromotion(context.Background(), "version-123", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateAppStoreVersionPromotion_WithTreatment(t *testing.T) {
	resp := AppStoreVersionPromotionResponse{
		Data: Resource[AppStoreVersionPromotionAttributes]{
			Type: ResourceTypeAppStoreVersionPromotions,
			ID:   "promo-456",
		},
	}
	body, _ := json.Marshal(resp)

	client := newTestClient(t, func(req *http.Request) {
		assertAuthorized(t, req)

		var createReq AppStoreVersionPromotionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		rel := createReq.Data.Relationships.AppStoreVersionExperimentTreatment
		if rel.Data.ID != "treatment-456" {
			t.Errorf("expected treatment ID treatment-456, got %s", rel.Data.ID)
		}
		if rel.Data.Type != ResourceTypeAppStoreVersionExperimentTreatments {
			t.Errorf("expected type appStoreVersionExperimentTreatments, got %s", rel.Data.Type)
		}
		if createReq.Data.Relationships.AppStoreVersion.Data.ID != "version-123" {
			t.Errorf("expected version ID version-123, got %s", createReq.Data.Relationships.AppStoreVersion.Data.ID)
		}
	}, jsonResponse(http.StatusCreated, string(body)))

	result, err := client.CreateAppStoreVersionPromotion(context.Background(), "version-123", "treatment-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data.ID != "promo-456" {
		t.Errorf("expected ID promo-456, got %s", result.Data.ID)
	}
}
