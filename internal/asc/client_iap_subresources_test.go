package asc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestCreateInAppPurchaseLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseLocalizations" {
			t.Fatalf("expected path /v1/inAppPurchaseLocalizations, got %s", req.URL.Path)
		}
		var payload InAppPurchaseLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Type != ResourceTypeInAppPurchaseLocalizations {
			t.Fatalf("expected type inAppPurchaseLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships.InAppPurchaseV2.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchaseV2 ID iap-1, got %q", payload.Data.Relationships.InAppPurchaseV2.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := InAppPurchaseLocalizationCreateAttributes{
		Name:        "Name",
		Locale:      "en-US",
		Description: "Description",
	}
	if _, err := client.CreateInAppPurchaseLocalization(context.Background(), "iap-1", attrs); err != nil {
		t.Fatalf("CreateInAppPurchaseLocalization() error: %v", err)
	}
}

func TestUpdateInAppPurchaseLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseLocalizations/loc-1" {
			t.Fatalf("expected path /v1/inAppPurchaseLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload InAppPurchaseLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.ID != "loc-1" {
			t.Fatalf("expected ID loc-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name attribute to be set")
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	if _, err := client.UpdateInAppPurchaseLocalization(context.Background(), "loc-1", InAppPurchaseLocalizationUpdateAttributes{
		Name: &name,
	}); err != nil {
		t.Fatalf("UpdateInAppPurchaseLocalization() error: %v", err)
	}
}

func TestDeleteInAppPurchaseLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseLocalizations/loc-1" {
			t.Fatalf("expected path /v1/inAppPurchaseLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteInAppPurchaseLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteInAppPurchaseLocalization() error: %v", err)
	}
}

func TestGetInAppPurchaseImages_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/inAppPurchases/1/images?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseImages(context.Background(), "1", WithIAPImagesNextURL(next)); err != nil {
		t.Fatalf("GetInAppPurchaseImages() error: %v", err)
	}
}

func TestCreateInAppPurchaseImage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseImages" {
			t.Fatalf("expected path /v1/inAppPurchaseImages, got %s", req.URL.Path)
		}
		var payload InAppPurchaseImageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Relationships.InAppPurchase.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchase ID iap-1, got %q", payload.Data.Relationships.InAppPurchase.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateInAppPurchaseImage(context.Background(), "iap-1", "image.png", 123); err != nil {
		t.Fatalf("CreateInAppPurchaseImage() error: %v", err)
	}
}

func TestGetInAppPurchaseImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseImages/img-1" {
			t.Fatalf("expected path /v1/inAppPurchaseImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetInAppPurchaseImage() error: %v", err)
	}
}

func TestUpdateInAppPurchaseImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseImages/img-1" {
			t.Fatalf("expected path /v1/inAppPurchaseImages/img-1, got %s", req.URL.Path)
		}
		var payload InAppPurchaseImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.SourceFileChecksum == nil {
			t.Fatalf("expected checksum to be set")
		}
		assertAuthorized(t, req)
	}, response)

	checksum := "hash"
	uploaded := true
	if _, err := client.UpdateInAppPurchaseImage(context.Background(), "img-1", InAppPurchaseImageUpdateAttributes{
		SourceFileChecksum: &checksum,
		Uploaded:           &uploaded,
	}); err != nil {
		t.Fatalf("UpdateInAppPurchaseImage() error: %v", err)
	}
}

func TestDeleteInAppPurchaseImage(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseImages/img-1" {
			t.Fatalf("expected path /v1/inAppPurchaseImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteInAppPurchaseImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteInAppPurchaseImage() error: %v", err)
	}
}

func TestGetInAppPurchaseReviewScreenshotForIAP(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/appStoreReviewScreenshot" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/appStoreReviewScreenshot, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseAppStoreReviewScreenshotForIAP(context.Background(), "iap-1"); err != nil {
		t.Fatalf("GetInAppPurchaseAppStoreReviewScreenshotForIAP() error: %v", err)
	}
}

func TestGetInAppPurchaseReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("expected path /v1/inAppPurchaseAppStoreReviewScreenshots/shot-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseAppStoreReviewScreenshot(context.Background(), "shot-1"); err != nil {
		t.Fatalf("GetInAppPurchaseAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestCreateInAppPurchaseReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots" {
			t.Fatalf("expected path /v1/inAppPurchaseAppStoreReviewScreenshots, got %s", req.URL.Path)
		}
		var payload InAppPurchaseAppStoreReviewScreenshotCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Relationships.InAppPurchaseV2.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchaseV2 ID iap-1, got %q", payload.Data.Relationships.InAppPurchaseV2.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateInAppPurchaseAppStoreReviewScreenshot(context.Background(), "iap-1", "review.png", 456); err != nil {
		t.Fatalf("CreateInAppPurchaseAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestUpdateInAppPurchaseReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("expected path /v1/inAppPurchaseAppStoreReviewScreenshots/shot-1, got %s", req.URL.Path)
		}
		var payload InAppPurchaseAppStoreReviewScreenshotUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil {
			t.Fatalf("expected uploaded attribute to be set")
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	checksum := "hash"
	if _, err := client.UpdateInAppPurchaseAppStoreReviewScreenshot(context.Background(), "shot-1", InAppPurchaseAppStoreReviewScreenshotUpdateAttributes{
		Uploaded:           &uploaded,
		SourceFileChecksum: &checksum,
	}); err != nil {
		t.Fatalf("UpdateInAppPurchaseAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestDeleteInAppPurchaseReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("expected path /v1/inAppPurchaseAppStoreReviewScreenshots/shot-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteInAppPurchaseAppStoreReviewScreenshot(context.Background(), "shot-1"); err != nil {
		t.Fatalf("DeleteInAppPurchaseAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestCreateInAppPurchaseAvailability(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAvailabilities" {
			t.Fatalf("expected path /v1/inAppPurchaseAvailabilities, got %s", req.URL.Path)
		}
		var payload InAppPurchaseAvailabilityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if !payload.Data.Attributes.AvailableInNewTerritories {
			t.Fatalf("expected availableInNewTerritories true")
		}
		if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
			t.Fatalf("expected 2 territories, got %d", len(payload.Data.Relationships.AvailableTerritories.Data))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateInAppPurchaseAvailability(context.Background(), "iap-1", true, []string{"USA", "CAN"}); err != nil {
		t.Fatalf("CreateInAppPurchaseAvailability() error: %v", err)
	}
}

func TestGetInAppPurchaseAvailabilityByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAvailabilities/avail-1" {
			t.Fatalf("expected path /v1/inAppPurchaseAvailabilities/avail-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseAvailabilityByID(context.Background(), "avail-1"); err != nil {
		t.Fatalf("GetInAppPurchaseAvailabilityByID() error: %v", err)
	}
}

func TestGetInAppPurchaseAvailabilityAvailableTerritories(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseAvailabilities/avail-1/availableTerritories" {
			t.Fatalf("expected path /v1/inAppPurchaseAvailabilities/avail-1/availableTerritories, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseAvailabilityAvailableTerritories(context.Background(), "avail-1", WithIAPAvailabilityTerritoriesLimit(5)); err != nil {
		t.Fatalf("GetInAppPurchaseAvailabilityAvailableTerritories() error: %v", err)
	}
}

func TestGetInAppPurchaseContent(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/content" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/content, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseContent(context.Background(), "iap-1"); err != nil {
		t.Fatalf("GetInAppPurchaseContent() error: %v", err)
	}
}

func TestGetInAppPurchaseContentByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseContents/content-1" {
			t.Fatalf("expected path /v1/inAppPurchaseContents/content-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseContentByID(context.Background(), "content-1"); err != nil {
		t.Fatalf("GetInAppPurchaseContentByID() error: %v", err)
	}
}

func TestGetInAppPurchasePricePoints_WithTerritory(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/pricePoints, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[territory]") != "USA" {
			t.Fatalf("expected territory filter USA, got %q", req.URL.Query().Get("filter[territory]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePricePoints(context.Background(), "iap-1", WithIAPPricePointsTerritory("USA")); err != nil {
		t.Fatalf("GetInAppPurchasePricePoints() error: %v", err)
	}
}

func TestGetInAppPurchasePricePoints_WithIncludeAndFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/pricePoints, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("include") != "territory" {
			t.Fatalf("expected include=territory, got %q", query.Get("include"))
		}
		if query.Get("fields[inAppPurchasePricePoints]") != "customerPrice,proceeds,territory" {
			t.Fatalf("expected price point fields, got %q", query.Get("fields[inAppPurchasePricePoints]"))
		}
		if query.Get("fields[territories]") != "currency" {
			t.Fatalf("expected territory fields, got %q", query.Get("fields[territories]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePricePoints(
		context.Background(),
		"iap-1",
		WithIAPPricePointsInclude([]string{"territory"}),
		WithIAPPricePointsFields([]string{"customerPrice", "proceeds", "territory"}),
		WithIAPPricePointsTerritoryFields([]string{"currency"}),
	); err != nil {
		t.Fatalf("GetInAppPurchasePricePoints() error: %v", err)
	}
}

func TestGetInAppPurchasePricePointEqualizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePricePoints/price-1/equalizations" {
			t.Fatalf("expected path /v1/inAppPurchasePricePoints/price-1/equalizations, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePricePointEqualizations(context.Background(), "price-1"); err != nil {
		t.Fatalf("GetInAppPurchasePricePointEqualizations() error: %v", err)
	}
}

func TestGetInAppPurchasePriceScheduleManualPrices_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules/schedule-1/manualPrices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceScheduleManualPrices(context.Background(), "schedule-1", WithIAPPriceSchedulePricesLimit(5)); err != nil {
		t.Fatalf("GetInAppPurchasePriceScheduleManualPrices() error: %v", err)
	}
}

func TestGetInAppPurchasePriceScheduleManualPrices_WithQueryOptions(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules/schedule-1/manualPrices, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("include") != "inAppPurchasePricePoint,territory" {
			t.Fatalf("expected include query, got %q", query.Get("include"))
		}
		if query.Get("fields[inAppPurchasePrices]") != "manual,inAppPurchasePricePoint,territory" {
			t.Fatalf("expected fields[inAppPurchasePrices], got %q", query.Get("fields[inAppPurchasePrices]"))
		}
		if query.Get("fields[inAppPurchasePricePoints]") != "customerPrice,proceeds,territory" {
			t.Fatalf("expected fields[inAppPurchasePricePoints], got %q", query.Get("fields[inAppPurchasePricePoints]"))
		}
		if query.Get("fields[territories]") != "currency" {
			t.Fatalf("expected fields[territories], got %q", query.Get("fields[territories]"))
		}
		if query.Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", query.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceScheduleManualPrices(
		context.Background(),
		"schedule-1",
		WithIAPPriceSchedulePricesInclude([]string{"inAppPurchasePricePoint", "territory"}),
		WithIAPPriceSchedulePricesFields([]string{"manual", "inAppPurchasePricePoint", "territory"}),
		WithIAPPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
		WithIAPPriceSchedulePricesTerritoryFields([]string{"currency"}),
		WithIAPPriceSchedulePricesLimit(200),
	); err != nil {
		t.Fatalf("GetInAppPurchasePriceScheduleManualPrices() error: %v", err)
	}
}

func TestGetInAppPurchasePriceScheduleAutomaticPrices_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceScheduleAutomaticPrices(context.Background(), "schedule-1", WithIAPPriceSchedulePricesLimit(5)); err != nil {
		t.Fatalf("GetInAppPurchasePriceScheduleAutomaticPrices() error: %v", err)
	}
}

func TestGetInAppPurchasePriceScheduleByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules/schedule-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceScheduleByID(context.Background(), "schedule-1"); err != nil {
		t.Fatalf("GetInAppPurchasePriceScheduleByID() error: %v", err)
	}
}

func TestGetInAppPurchasePriceSchedule_WithQueryOptions(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/iapPriceSchedule, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("include") != "baseTerritory,manualPrices,automaticPrices" {
			t.Fatalf("expected include query, got %q", query.Get("include"))
		}
		if query.Get("fields[inAppPurchasePriceSchedules]") != "baseTerritory,manualPrices,automaticPrices" {
			t.Fatalf("expected schedule fields query, got %q", query.Get("fields[inAppPurchasePriceSchedules]"))
		}
		if query.Get("fields[territories]") != "currency" {
			t.Fatalf("expected territory fields query, got %q", query.Get("fields[territories]"))
		}
		if query.Get("fields[inAppPurchasePrices]") != "startDate,endDate,manual,inAppPurchasePricePoint,territory" {
			t.Fatalf("expected price fields query, got %q", query.Get("fields[inAppPurchasePrices]"))
		}
		if query.Get("limit[manualPrices]") != "50" {
			t.Fatalf("expected limit[manualPrices]=50, got %q", query.Get("limit[manualPrices]"))
		}
		if query.Get("limit[automaticPrices]") != "50" {
			t.Fatalf("expected limit[automaticPrices]=50, got %q", query.Get("limit[automaticPrices]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceSchedule(
		context.Background(),
		"iap-1",
		WithIAPPriceScheduleInclude([]string{"baseTerritory", "manualPrices", "automaticPrices"}),
		WithIAPPriceScheduleFields([]string{"baseTerritory", "manualPrices", "automaticPrices"}),
		WithIAPPriceScheduleTerritoryFields([]string{"currency"}),
		WithIAPPriceSchedulePriceFields([]string{"startDate", "endDate", "manual", "inAppPurchasePricePoint", "territory"}),
		WithIAPPriceScheduleManualPricesLimit(50),
		WithIAPPriceScheduleAutomaticPricesLimit(50),
	); err != nil {
		t.Fatalf("GetInAppPurchasePriceSchedule() error: %v", err)
	}
}

func TestGetInAppPurchasePriceScheduleByID_WithQueryOptions(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules/schedule-1, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("include") != "baseTerritory,manualPrices" {
			t.Fatalf("expected include query, got %q", query.Get("include"))
		}
		if query.Get("fields[inAppPurchasePrices]") != "startDate,endDate,manual" {
			t.Fatalf("expected price fields query, got %q", query.Get("fields[inAppPurchasePrices]"))
		}
		if query.Get("limit[manualPrices]") != "25" {
			t.Fatalf("expected limit[manualPrices]=25, got %q", query.Get("limit[manualPrices]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceScheduleByID(
		context.Background(),
		"schedule-1",
		WithIAPPriceScheduleInclude([]string{"baseTerritory", "manualPrices"}),
		WithIAPPriceSchedulePriceFields([]string{"startDate", "endDate", "manual"}),
		WithIAPPriceScheduleManualPricesLimit(25),
	); err != nil {
		t.Fatalf("GetInAppPurchasePriceScheduleByID() error: %v", err)
	}
}

func TestGetInAppPurchasePriceScheduleBaseTerritory(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/baseTerritory" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules/schedule-1/baseTerritory, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePriceScheduleBaseTerritory(context.Background(), "schedule-1"); err != nil {
		t.Fatalf("GetInAppPurchasePriceScheduleBaseTerritory() error: %v", err)
	}
}

func TestCreateInAppPurchasePriceSchedule(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
			t.Fatalf("expected path /v1/inAppPurchasePriceSchedules, got %s", req.URL.Path)
		}
		var payload InAppPurchasePriceScheduleCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Relationships.InAppPurchase.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchase ID iap-1, got %q", payload.Data.Relationships.InAppPurchase.Data.ID)
		}
		if len(payload.Data.Relationships.ManualPrices.Data) != 1 {
			t.Fatalf("expected 1 manual price, got %d", len(payload.Data.Relationships.ManualPrices.Data))
		}
		if len(payload.Included) != 1 || payload.Included[0].Relationships.InAppPurchasePricePoint.Data.ID != "price-1" {
			t.Fatalf("expected included price point price-1")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := InAppPurchasePriceScheduleCreateAttributes{
		BaseTerritoryID: "USA",
		Prices: []InAppPurchasePriceSchedulePrice{
			{
				PricePointID: "price-1",
				StartDate:    "2024-03-01",
			},
		},
	}
	if _, err := client.CreateInAppPurchasePriceSchedule(context.Background(), "iap-1", attrs); err != nil {
		t.Fatalf("CreateInAppPurchasePriceSchedule() error: %v", err)
	}
}

func TestGetInAppPurchaseOfferCodes_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/offerCodes?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseOfferCodes(context.Background(), "iap-1", WithIAPOfferCodesNextURL(next)); err != nil {
		t.Fatalf("GetInAppPurchaseOfferCodes() error: %v", err)
	}
}

func TestGetInAppPurchaseOfferCodePrices_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchaseOfferPrices","id":"price-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes/offer-1/prices" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodes/offer-1/prices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseOfferCodePrices(context.Background(), "offer-1", WithIAPOfferCodePricesLimit(5)); err != nil {
		t.Fatalf("GetInAppPurchaseOfferCodePrices() error: %v", err)
	}
}

func TestGetInAppPurchaseOfferCodePrices_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/prices?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseOfferCodePrices(context.Background(), "offer-1", WithIAPOfferCodePricesNextURL(next)); err != nil {
		t.Fatalf("GetInAppPurchaseOfferCodePrices() error: %v", err)
	}
}

func TestCreateInAppPurchaseOfferCode(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodes","id":"code-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodes, got %s", req.URL.Path)
		}
		var payload InAppPurchaseOfferCodeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Relationships.InAppPurchase.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchase ID iap-1, got %q", payload.Data.Relationships.InAppPurchase.Data.ID)
		}
		if len(payload.Included) != 1 || payload.Included[0].Relationships.Territory.Data.ID != "USA" {
			t.Fatalf("expected included territory USA")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := InAppPurchaseOfferCodeCreateAttributes{
		Name: "Spring",
		CustomerEligibilities: []string{
			"NON_SPENDER",
			"ACTIVE_SPENDER",
		},
		Prices: []InAppPurchaseOfferCodePrice{
			{
				TerritoryID:  "USA",
				PricePointID: "price-1",
			},
		},
	}
	if _, err := client.CreateInAppPurchaseOfferCode(context.Background(), "iap-1", attrs); err != nil {
		t.Fatalf("CreateInAppPurchaseOfferCode() error: %v", err)
	}
}

func TestGetInAppPurchaseOfferCode(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseOfferCodes","id":"code-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes/code-1" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodes/code-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseOfferCode(context.Background(), "code-1"); err != nil {
		t.Fatalf("GetInAppPurchaseOfferCode() error: %v", err)
	}
}

func TestUpdateInAppPurchaseOfferCode(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseOfferCodes","id":"code-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes/code-1" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodes/code-1, got %s", req.URL.Path)
		}
		var payload InAppPurchaseOfferCodeUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Active == nil {
			t.Fatalf("expected active attribute to be set")
		}
		assertAuthorized(t, req)
	}, response)

	active := true
	if _, err := client.UpdateInAppPurchaseOfferCode(context.Background(), "code-1", InAppPurchaseOfferCodeUpdateAttributes{
		Active: &active,
	}); err != nil {
		t.Fatalf("UpdateInAppPurchaseOfferCode() error: %v", err)
	}
}

func TestCreateInAppPurchaseSubmission(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseSubmissions","id":"sub-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseSubmissions" {
			t.Fatalf("expected path /v1/inAppPurchaseSubmissions, got %s", req.URL.Path)
		}
		var payload InAppPurchaseSubmissionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Relationships.InAppPurchaseV2.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchaseV2 ID iap-1, got %q", payload.Data.Relationships.InAppPurchaseV2.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateInAppPurchaseSubmission(context.Background(), "iap-1"); err != nil {
		t.Fatalf("CreateInAppPurchaseSubmission() error: %v", err)
	}
}

func TestGetInAppPurchasePromotedPurchase(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/promotedPurchase" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/promotedPurchase, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasePromotedPurchase(context.Background(), "iap-1"); err != nil {
		t.Fatalf("GetInAppPurchasePromotedPurchase() error: %v", err)
	}
}

func TestCreateInAppPurchaseOfferCodeCustomCode_UsesPostPath(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeCustomCodes","id":"cc-1","attributes":{"customCode":"SUMMER26","numberOfCodes":100}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodeCustomCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodeCustomCodes, got %s", req.URL.Path)
		}

		var payload InAppPurchaseOfferCodeCustomCodeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Type != ResourceTypeInAppPurchaseOfferCodeCustomCodes {
			t.Fatalf("expected type %s, got %s", ResourceTypeInAppPurchaseOfferCodeCustomCodes, payload.Data.Type)
		}
		if payload.Data.Attributes.CustomCode != "SUMMER26" {
			t.Fatalf("expected customCode SUMMER26, got %q", payload.Data.Attributes.CustomCode)
		}
		if payload.Data.Attributes.NumberOfCodes != 100 {
			t.Fatalf("expected numberOfCodes 100, got %d", payload.Data.Attributes.NumberOfCodes)
		}
		if payload.Data.Relationships.OfferCode.Data.Type != ResourceTypeInAppPurchaseOfferCodes {
			t.Fatalf("expected relationship type %s, got %s", ResourceTypeInAppPurchaseOfferCodes, payload.Data.Relationships.OfferCode.Data.Type)
		}
		if payload.Data.Relationships.OfferCode.Data.ID != "offer-1" {
			t.Fatalf("expected offerCode ID offer-1, got %q", payload.Data.Relationships.OfferCode.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	req := InAppPurchaseOfferCodeCustomCodeCreateRequest{
		Data: InAppPurchaseOfferCodeCustomCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodeCustomCodes,
			Attributes: InAppPurchaseOfferCodeCustomCodeCreateAttributes{
				CustomCode:    "SUMMER26",
				NumberOfCodes: 100,
			},
			Relationships: InAppPurchaseOfferCodeCustomCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchaseOfferCodes,
						ID:   "offer-1",
					},
				},
			},
		},
	}
	resp, err := client.CreateInAppPurchaseOfferCodeCustomCode(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateInAppPurchaseOfferCodeCustomCode() error: %v", err)
	}
	if resp.Data.ID != "cc-1" {
		t.Fatalf("expected response id cc-1, got %q", resp.Data.ID)
	}
}

func TestCreateInAppPurchaseOfferCodeOneTimeUseCode_UsesPostPath(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-1","attributes":{"numberOfCodes":500,"expirationDate":"2026-09-30"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodeOneTimeUseCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodeOneTimeUseCodes, got %s", req.URL.Path)
		}

		var payload InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Type != ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes {
			t.Fatalf("expected type %s, got %s", ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes, payload.Data.Type)
		}
		if payload.Data.Attributes.NumberOfCodes != 500 {
			t.Fatalf("expected numberOfCodes 500, got %d", payload.Data.Attributes.NumberOfCodes)
		}
		if payload.Data.Attributes.ExpirationDate != "2026-09-30" {
			t.Fatalf("expected expirationDate 2026-09-30, got %q", payload.Data.Attributes.ExpirationDate)
		}
		if payload.Data.Relationships.OfferCode.Data.Type != ResourceTypeInAppPurchaseOfferCodes {
			t.Fatalf("expected relationship type %s, got %s", ResourceTypeInAppPurchaseOfferCodes, payload.Data.Relationships.OfferCode.Data.Type)
		}
		if payload.Data.Relationships.OfferCode.Data.ID != "offer-2" {
			t.Fatalf("expected offerCode ID offer-2, got %q", payload.Data.Relationships.OfferCode.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	req := InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
		Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
			Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
				NumberOfCodes:  500,
				ExpirationDate: "2026-09-30",
			},
			Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchaseOfferCodes,
						ID:   "offer-2",
					},
				},
			},
		},
	}
	resp, err := client.CreateInAppPurchaseOfferCodeOneTimeUseCode(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateInAppPurchaseOfferCodeOneTimeUseCode() error: %v", err)
	}
	if resp.Data.ID != "otuc-1" {
		t.Fatalf("expected response id otuc-1, got %q", resp.Data.ID)
	}
}

func TestCreateInAppPurchaseOfferCodeOneTimeUseCode_PropagatesEnvironment(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-2","attributes":{"numberOfCodes":100,"expirationDate":"2026-12-31","environment":"SANDBOX"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		var payload InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Attributes.Environment != "SANDBOX" {
			t.Fatalf("expected environment SANDBOX, got %q", payload.Data.Attributes.Environment)
		}
		assertAuthorized(t, req)
	}, response)

	req := InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
		Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
			Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
				NumberOfCodes:  100,
				ExpirationDate: "2026-12-31",
				Environment:    "SANDBOX",
			},
			Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchaseOfferCodes,
						ID:   "offer-2",
					},
				},
			},
		},
	}
	resp, err := client.CreateInAppPurchaseOfferCodeOneTimeUseCode(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateInAppPurchaseOfferCodeOneTimeUseCode() error: %v", err)
	}
	if resp.Data.Attributes.Environment != "SANDBOX" {
		t.Fatalf("expected response environment SANDBOX, got %q", resp.Data.Attributes.Environment)
	}
}

func TestCreateInAppPurchaseOfferCodeOneTimeUseCode_NormalizesEnvironment(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-3","attributes":{"numberOfCodes":100,"expirationDate":"2026-12-31","environment":"SANDBOX"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		var payload InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Attributes.Environment != "SANDBOX" {
			t.Fatalf("expected normalized environment SANDBOX, got %q", payload.Data.Attributes.Environment)
		}
		assertAuthorized(t, req)
	}, response)

	req := InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
		Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
			Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
				NumberOfCodes:  100,
				ExpirationDate: "2026-12-31",
				Environment:    " sandbox ",
			},
			Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchaseOfferCodes,
						ID:   "offer-2",
					},
				},
			},
		},
	}
	if _, err := client.CreateInAppPurchaseOfferCodeOneTimeUseCode(context.Background(), req); err != nil {
		t.Fatalf("CreateInAppPurchaseOfferCodeOneTimeUseCode() error: %v", err)
	}
}

func TestCreateInAppPurchaseOfferCodeCustomCode_ValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	tests := []struct {
		name string
		req  InAppPurchaseOfferCodeCustomCodeCreateRequest
	}{
		{
			name: "missing offerCode ID",
			req: InAppPurchaseOfferCodeCustomCodeCreateRequest{
				Data: InAppPurchaseOfferCodeCustomCodeCreateData{
					Type: ResourceTypeInAppPurchaseOfferCodeCustomCodes,
					Attributes: InAppPurchaseOfferCodeCustomCodeCreateAttributes{
						CustomCode:    "SUMMER26",
						NumberOfCodes: 100,
					},
					Relationships: InAppPurchaseOfferCodeCustomCodeCreateRelationships{
						OfferCode: Relationship{
							Data: ResourceData{
								Type: ResourceTypeInAppPurchaseOfferCodes,
								ID:   " ",
							},
						},
					},
				},
			},
		},
		{
			name: "missing customCode",
			req: InAppPurchaseOfferCodeCustomCodeCreateRequest{
				Data: InAppPurchaseOfferCodeCustomCodeCreateData{
					Type: ResourceTypeInAppPurchaseOfferCodeCustomCodes,
					Attributes: InAppPurchaseOfferCodeCustomCodeCreateAttributes{
						CustomCode:    " ",
						NumberOfCodes: 100,
					},
					Relationships: InAppPurchaseOfferCodeCustomCodeCreateRelationships{
						OfferCode: Relationship{
							Data: ResourceData{
								Type: ResourceTypeInAppPurchaseOfferCodes,
								ID:   "offer-1",
							},
						},
					},
				},
			},
		},
		{
			name: "invalid numberOfCodes",
			req: InAppPurchaseOfferCodeCustomCodeCreateRequest{
				Data: InAppPurchaseOfferCodeCustomCodeCreateData{
					Type: ResourceTypeInAppPurchaseOfferCodeCustomCodes,
					Attributes: InAppPurchaseOfferCodeCustomCodeCreateAttributes{
						CustomCode:    "SUMMER26",
						NumberOfCodes: 0,
					},
					Relationships: InAppPurchaseOfferCodeCustomCodeCreateRelationships{
						OfferCode: Relationship{
							Data: ResourceData{
								Type: ResourceTypeInAppPurchaseOfferCodes,
								ID:   "offer-1",
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.CreateInAppPurchaseOfferCodeCustomCode(context.Background(), test.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCreateInAppPurchaseOfferCodeCustomCode_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	req := InAppPurchaseOfferCodeCustomCodeCreateRequest{
		Data: InAppPurchaseOfferCodeCustomCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodeCustomCodes,
			Attributes: InAppPurchaseOfferCodeCustomCodeCreateAttributes{
				CustomCode:    "SUMMER26",
				NumberOfCodes: 100,
			},
			Relationships: InAppPurchaseOfferCodeCustomCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchaseOfferCodes,
						ID:   "offer-1",
					},
				},
			},
		},
	}

	_, err := client.CreateInAppPurchaseOfferCodeCustomCode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestCreateInAppPurchaseOfferCodeOneTimeUseCode_ValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	tests := []struct {
		name string
		req  InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest
	}{
		{
			name: "missing offerCode ID",
			req: InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
				Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
					Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
					Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
						NumberOfCodes:  100,
						ExpirationDate: "2026-12-31",
					},
					Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
						OfferCode: Relationship{
							Data: ResourceData{
								Type: ResourceTypeInAppPurchaseOfferCodes,
								ID:   " ",
							},
						},
					},
				},
			},
		},
		{
			name: "invalid numberOfCodes",
			req: InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
				Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
					Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
					Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
						NumberOfCodes:  0,
						ExpirationDate: "2026-12-31",
					},
					Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
						OfferCode: Relationship{
							Data: ResourceData{
								Type: ResourceTypeInAppPurchaseOfferCodes,
								ID:   "offer-1",
							},
						},
					},
				},
			},
		},
		{
			name: "missing expirationDate",
			req: InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
				Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
					Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
					Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
						NumberOfCodes:  100,
						ExpirationDate: " ",
					},
					Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
						OfferCode: Relationship{
							Data: ResourceData{
								Type: ResourceTypeInAppPurchaseOfferCodes,
								ID:   "offer-1",
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.CreateInAppPurchaseOfferCodeOneTimeUseCode(context.Background(), test.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCreateInAppPurchaseOfferCodeOneTimeUseCode_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	req := InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
		Data: InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
			Attributes: InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
				NumberOfCodes:  100,
				ExpirationDate: "2026-12-31",
			},
			Relationships: InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchaseOfferCodes,
						ID:   "offer-1",
					},
				},
			},
		},
	}

	_, err := client.CreateInAppPurchaseOfferCodeOneTimeUseCode(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}
