package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func reviewSubmissionsJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCreateReviewSubmission(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "reviewSubmissions",
			"id": "submission-123",
			"attributes": {
				"platform": "IOS",
				"state": "READY_FOR_REVIEW",
				"submittedDate": "2026-01-20T00:00:00Z"
			},
			"relationships": {
				"app": {
					"data": {
						"type": "apps",
						"id": "app-123"
					}
				}
			}
		}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("expected path /v1/reviewSubmissions, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload ReviewSubmissionCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeReviewSubmissions {
			t.Fatalf("expected type reviewSubmissions, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.Platform != PlatformIOS {
			t.Fatalf("expected platform IOS, got %s", payload.Data.Attributes.Platform)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatal("expected relationships.app to be set")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps {
			t.Fatalf("expected app type apps, got %s", payload.Data.Relationships.App.Data.Type)
		}
		if payload.Data.Relationships.App.Data.ID != "app-123" {
			t.Fatalf("expected app ID app-123, got %s", payload.Data.Relationships.App.Data.ID)
		}
	}, response)

	resp, err := client.CreateReviewSubmission(context.Background(), "app-123", PlatformIOS)
	if err != nil {
		t.Fatalf("CreateReviewSubmission() error: %v", err)
	}

	if resp.Data.ID != "submission-123" {
		t.Fatalf("expected ID submission-123, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.SubmissionState != ReviewSubmissionStateReadyForReview {
		t.Fatalf("expected state %s, got %s", ReviewSubmissionStateReadyForReview, resp.Data.Attributes.SubmissionState)
	}
}

func TestSubmitReviewSubmission(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": {
			"type": "reviewSubmissions",
			"id": "submission-123",
			"attributes": {
				"platform": "IOS",
				"state": "WAITING_FOR_REVIEW",
				"submittedDate": "2026-01-20T00:00:00Z"
			}
		}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions/submission-123" {
			t.Fatalf("expected path /v1/reviewSubmissions/submission-123, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload ReviewSubmissionUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeReviewSubmissions {
			t.Fatalf("expected type reviewSubmissions, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "submission-123" {
			t.Fatalf("expected submission ID submission-123, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes.Submitted == nil || !*payload.Data.Attributes.Submitted {
			t.Fatalf("expected submitted true, got %v", payload.Data.Attributes.Submitted)
		}
	}, response)

	resp, err := client.SubmitReviewSubmission(context.Background(), "submission-123")
	if err != nil {
		t.Fatalf("SubmitReviewSubmission() error: %v", err)
	}

	if resp.Data.Attributes.SubmissionState != ReviewSubmissionStateWaitingForReview {
		t.Fatalf("expected state %s, got %s", ReviewSubmissionStateWaitingForReview, resp.Data.Attributes.SubmissionState)
	}
}

func TestGetReviewSubmissionItem(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": {
			"type": "reviewSubmissionItems",
			"id": "item-123",
			"attributes": {
				"state": "READY_FOR_REVIEW"
			}
		}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissionItems/item-123" {
			t.Fatalf("expected path /v1/reviewSubmissionItems/item-123, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetReviewSubmissionItem(context.Background(), "item-123")
	if err != nil {
		t.Fatalf("GetReviewSubmissionItem() error: %v", err)
	}

	if resp.Data.ID != "item-123" {
		t.Fatalf("expected ID item-123, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.State != "READY_FOR_REVIEW" {
		t.Fatalf("expected state READY_FOR_REVIEW, got %s", resp.Data.Attributes.State)
	}
}

func TestCreateReviewSubmissionItem(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "reviewSubmissionItems",
			"id": "item-123",
			"attributes": {
				"state": "READY_FOR_REVIEW"
			},
			"relationships": {
				"reviewSubmission": {
					"data": {
						"type": "reviewSubmissions",
						"id": "submission-123"
					}
				},
				"appStoreVersion": {
					"data": {
						"type": "appStoreVersions",
						"id": "version-123"
					}
				}
			}
		}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissionItems" {
			t.Fatalf("expected path /v1/reviewSubmissionItems, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload ReviewSubmissionItemCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeReviewSubmissionItems {
			t.Fatalf("expected type reviewSubmissionItems, got %s", payload.Data.Type)
		}
		if payload.Data.Relationships.ReviewSubmission == nil {
			t.Fatal("expected reviewSubmission relationship to be set")
		}
		if payload.Data.Relationships.ReviewSubmission.Data.ID != "submission-123" {
			t.Fatalf("expected submission ID submission-123, got %s", payload.Data.Relationships.ReviewSubmission.Data.ID)
		}
		if payload.Data.Relationships.AppStoreVersion == nil {
			t.Fatal("expected appStoreVersion relationship to be set")
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-123" {
			t.Fatalf("expected version ID version-123, got %s", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
	}, response)

	resp, err := client.CreateReviewSubmissionItem(context.Background(), "submission-123", ReviewSubmissionItemTypeAppStoreVersion, "version-123")
	if err != nil {
		t.Fatalf("CreateReviewSubmissionItem() error: %v", err)
	}

	if resp.Data.ID != "item-123" {
		t.Fatalf("expected ID item-123, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.State != "READY_FOR_REVIEW" {
		t.Fatalf("expected state READY_FOR_REVIEW, got %s", resp.Data.Attributes.State)
	}
}

func TestUpdateReviewSubmissionItem(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": {
			"type": "reviewSubmissionItems",
			"id": "item-123",
			"attributes": {
				"state": "READY_FOR_REVIEW"
			}
		}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissionItems/item-123" {
			t.Fatalf("expected path /v1/reviewSubmissionItems/item-123, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload ReviewSubmissionItemUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeReviewSubmissionItems {
			t.Fatalf("expected type reviewSubmissionItems, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "item-123" {
			t.Fatalf("expected item ID item-123, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes.State == nil || *payload.Data.Attributes.State != "READY_FOR_REVIEW" {
			t.Fatalf("expected state READY_FOR_REVIEW, got %v", payload.Data.Attributes.State)
		}
	}, response)

	state := "READY_FOR_REVIEW"
	resp, err := client.UpdateReviewSubmissionItem(context.Background(), "item-123", ReviewSubmissionItemUpdateAttributes{State: &state})
	if err != nil {
		t.Fatalf("UpdateReviewSubmissionItem() error: %v", err)
	}

	if resp.Data.ID != "item-123" {
		t.Fatalf("expected ID item-123, got %s", resp.Data.ID)
	}
}

func TestDeleteReviewSubmissionItem(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissionItems/item-123" {
			t.Fatalf("expected path /v1/reviewSubmissionItems/item-123, got %s", req.URL.Path)
		}
	}, response)

	if err := client.DeleteReviewSubmissionItem(context.Background(), "item-123"); err != nil {
		t.Fatalf("DeleteReviewSubmissionItem() error: %v", err)
	}
}

func TestGetReviewSubmissionItemsRelationships(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": [
			{
				"type": "reviewSubmissionItems",
				"id": "item-123"
			}
		]
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions/submission-123/relationships/items" {
			t.Fatalf("expected path /v1/reviewSubmissions/submission-123/relationships/items, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetReviewSubmissionItemsRelationships(context.Background(), "submission-123")
	if err != nil {
		t.Fatalf("GetReviewSubmissionItemsRelationships() error: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "item-123" {
		t.Fatalf("expected item ID item-123, got %s", resp.Data[0].ID)
	}
}

func TestReviewSubmissionValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	if _, err := client.GetReviewSubmission(context.Background(), ""); err == nil {
		t.Fatalf("expected submissionID required error, got nil")
	}

	if _, err := client.CreateReviewSubmission(context.Background(), "", PlatformIOS); err == nil {
		t.Fatalf("expected appID required error, got nil")
	}

	if _, err := client.CreateReviewSubmission(context.Background(), "app-123", ""); err == nil {
		t.Fatalf("expected platform required error, got nil")
	}

	if _, err := client.GetReviewSubmissionItems(context.Background(), ""); err == nil {
		t.Fatalf("expected submissionID required error, got nil")
	}

	if _, err := client.GetReviewSubmissionItem(context.Background(), ""); err == nil {
		t.Fatalf("expected itemID required error, got nil")
	}

	if _, err := client.CreateReviewSubmissionItem(context.Background(), "", ReviewSubmissionItemTypeAppStoreVersion, "item-1"); err == nil {
		t.Fatalf("expected submissionID required error, got nil")
	}

	if _, err := client.CreateReviewSubmissionItem(context.Background(), "submission-123", "", "item-1"); err == nil {
		t.Fatalf("expected itemType required error, got nil")
	}

	if _, err := client.CreateReviewSubmissionItem(context.Background(), "submission-123", ReviewSubmissionItemTypeAppStoreVersion, ""); err == nil {
		t.Fatalf("expected itemID required error, got nil")
	}

	if _, err := client.CreateReviewSubmissionItem(context.Background(), "submission-123", "badType", "item-1"); err == nil {
		t.Fatalf("expected unsupported itemType error, got nil")
	}

	if _, err := client.UpdateReviewSubmissionItem(context.Background(), "", ReviewSubmissionItemUpdateAttributes{}); err == nil {
		t.Fatalf("expected itemID required error, got nil")
	}

	if _, err := client.GetReviewSubmissionItemsRelationships(context.Background(), ""); err == nil {
		t.Fatalf("expected submissionID required error, got nil")
	}

	if err := client.DeleteReviewSubmissionItem(context.Background(), ""); err == nil {
		t.Fatalf("expected itemID required error, got nil")
	}
}
