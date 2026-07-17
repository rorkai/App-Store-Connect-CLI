package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
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

func TestReviewSubmissionsResponsePreservesSchemaMetadata(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": [{
			"type": "reviewSubmissions",
			"id": "submission-123",
			"relationships": {
				"items": {
					"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-123/relationships/items", "related": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-123/items"},
					"meta": {"paging": {"total": 1, "limit": 50, "nextCursor": "item-next"}},
					"data": [{"type": "reviewSubmissionItems", "id": "item-123"}]
				}
			},
			"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-123"}
		}],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/app-123/reviewSubmissions", "first": "https://api.appstoreconnect.apple.com/v1/apps/app-123/reviewSubmissions?cursor=first", "next": "https://api.appstoreconnect.apple.com/v1/apps/app-123/reviewSubmissions?cursor=next"},
		"meta": {"paging": {"total": 1, "limit": 50}}
	}`)
	client := newTestClient(t, func(req *http.Request) {}, response)

	resp, err := client.GetReviewSubmissions(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetReviewSubmissions() error: %v", err)
	}
	if len(resp.Data) != 1 || !strings.Contains(string(resp.Data[0].Links), `"self"`) {
		t.Fatalf("resource links were not preserved: %+v", resp.Data)
	}
	if !strings.Contains(string(resp.Meta), `"paging"`) {
		t.Fatalf("response meta was not preserved: %s", resp.Meta)
	}
	if !strings.Contains(resp.Links.First, "cursor=first") {
		t.Fatalf("paged document first link was not preserved: %+v", resp.Links)
	}
	items := resp.Data[0].Relationships.Items
	if items == nil || !strings.Contains(string(items.Links), `"related"`) || !strings.Contains(string(items.Meta), `"nextCursor"`) {
		t.Fatalf("items relationship links/meta were not preserved: %+v", items)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	for _, want := range []string{`"first"`, `"related"`, `"nextCursor"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("re-encoded response omitted %s: %s", want, encoded)
		}
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
		if payload.Data.Attributes.Submitted == nil || payload.Data.Attributes.Submitted.Value == nil || !*payload.Data.Attributes.Submitted.Value {
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

func TestUpdateReviewSubmissionUsesExactNullableSchema(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"submission-123"}}`)
	client := newTestClient(t, func(req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload struct {
			Data struct {
				Attributes map[string]json.RawMessage `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got := string(payload.Data.Attributes["platform"]); got != `"MAC_OS"` {
			t.Fatalf("platform = %s, want MAC_OS", got)
		}
		if got := string(payload.Data.Attributes["submitted"]); got != "false" {
			t.Fatalf("submitted = %s, want false", got)
		}
		if got := string(payload.Data.Attributes["canceled"]); got != "null" {
			t.Fatalf("canceled = %s, want null", got)
		}
	}, response)

	platform := PlatformMacOS
	submitted := false
	_, err := client.UpdateReviewSubmission(context.Background(), "submission-123", ReviewSubmissionUpdateAttributes{
		Platform:  &NullablePlatform{Value: &platform},
		Submitted: &NullableBool{Value: &submitted},
		Canceled:  &NullableBool{},
	})
	if err != nil {
		t.Fatalf("UpdateReviewSubmission() error: %v", err)
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

func TestCreateReviewSubmissionItem_SupportedItemTypes(t *testing.T) {
	tests := []struct {
		name            string
		itemType        ReviewSubmissionItemType
		itemID          string
		wantType        ResourceType
		getRelationship func(ReviewSubmissionItemCreateRelationships) *Relationship
	}{
		{
			name:     "in-app purchase version",
			itemType: ReviewSubmissionItemTypeInAppPurchaseVersion,
			itemID:   "iap-version-123",
			wantType: ResourceTypeInAppPurchaseVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.InAppPurchaseVersion
			},
		},
		{
			name:     "subscription version",
			itemType: ReviewSubmissionItemTypeSubscriptionVersion,
			itemID:   "subscription-version-123",
			wantType: ResourceTypeSubscriptionVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.SubscriptionVersion
			},
		},
		{
			name:     "subscription group version",
			itemType: ReviewSubmissionItemTypeSubscriptionGroupVersion,
			itemID:   "subscription-group-version-123",
			wantType: ResourceTypeSubscriptionGroupVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.SubscriptionGroupVersion
			},
		},
		{
			name:     "app store version",
			itemType: ReviewSubmissionItemTypeAppStoreVersion,
			itemID:   "version-123",
			wantType: ResourceTypeAppStoreVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.AppStoreVersion
			},
		},
		{
			name:     "app custom product page version",
			itemType: ReviewSubmissionItemTypeAppCustomProductPageVersion,
			itemID:   "cppv-123",
			wantType: ResourceTypeAppCustomProductPageVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.AppCustomProductPageVersion
			},
		},
		{
			name:     "app event",
			itemType: ReviewSubmissionItemTypeAppEvent,
			itemID:   "event-123",
			wantType: ResourceTypeAppEvents,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.AppEvent
			},
		},
		{
			name:     "app store version experiment",
			itemType: ReviewSubmissionItemTypeAppStoreVersionExperiment,
			itemID:   "experiment-123",
			wantType: ResourceTypeAppStoreVersionExperiments,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.AppStoreVersionExperiment
			},
		},
		{
			name:     "app store version experiment v2",
			itemType: ReviewSubmissionItemTypeAppStoreVersionExperimentV2,
			itemID:   "experiment-v2-123",
			wantType: ResourceTypeAppStoreVersionExperiments,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.AppStoreVersionExperimentV2
			},
		},
		{
			name:     "background asset version",
			itemType: ReviewSubmissionItemTypeBackgroundAssetVersion,
			itemID:   "asset-123",
			wantType: ResourceTypeBackgroundAssetVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.BackgroundAssetVersion
			},
		},
		{
			name:     "game center achievement version",
			itemType: ReviewSubmissionItemTypeGameCenterAchievementVersion,
			itemID:   "achievement-123",
			wantType: ResourceTypeGameCenterAchievementVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.GameCenterAchievementVersion
			},
		},
		{
			name:     "game center activity version",
			itemType: ReviewSubmissionItemTypeGameCenterActivityVersion,
			itemID:   "activity-123",
			wantType: ResourceTypeGameCenterActivityVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.GameCenterActivityVersion
			},
		},
		{
			name:     "game center challenge version",
			itemType: ReviewSubmissionItemTypeGameCenterChallengeVersion,
			itemID:   "challenge-123",
			wantType: ResourceTypeGameCenterChallengeVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.GameCenterChallengeVersion
			},
		},
		{
			name:     "game center leaderboard set version",
			itemType: ReviewSubmissionItemTypeGameCenterLeaderboardSetVersion,
			itemID:   "leaderboard-set-123",
			wantType: ResourceTypeGameCenterLeaderboardSetVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.GameCenterLeaderboardSetVersion
			},
		},
		{
			name:     "game center leaderboard version",
			itemType: ReviewSubmissionItemTypeGameCenterLeaderboardVersion,
			itemID:   "leaderboard-123",
			wantType: ResourceTypeGameCenterLeaderboardVersions,
			getRelationship: func(relationships ReviewSubmissionItemCreateRelationships) *Relationship {
				return relationships.GameCenterLeaderboardVersion
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := reviewSubmissionsJSONResponse(http.StatusCreated, `{
				"data": {
					"type": "reviewSubmissionItems",
					"id": "item-123",
					"attributes": {
						"state": "READY_FOR_REVIEW"
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

				if payload.Data.Relationships.ReviewSubmission == nil {
					t.Fatal("expected reviewSubmission relationship to be set")
				}
				if payload.Data.Relationships.ReviewSubmission.Data.ID != "submission-123" {
					t.Fatalf("expected submission ID submission-123, got %s", payload.Data.Relationships.ReviewSubmission.Data.ID)
				}
				if got := countReviewSubmissionItemCreateRelationships(payload.Data.Relationships); got != 1 {
					t.Fatalf("expected exactly one item relationship, got %d", got)
				}

				relationship := test.getRelationship(payload.Data.Relationships)
				if relationship == nil {
					t.Fatalf("expected relationship for item type %q", test.itemType)
					return
				}
				if relationship.Data.Type != test.wantType {
					t.Fatalf("expected relationship type %q, got %q", test.wantType, relationship.Data.Type)
				}
				if relationship.Data.ID != test.itemID {
					t.Fatalf("expected relationship id %q, got %q", test.itemID, relationship.Data.ID)
				}
			}, response)

			resp, err := client.CreateReviewSubmissionItem(context.Background(), "submission-123", test.itemType, test.itemID)
			if err != nil {
				t.Fatalf("CreateReviewSubmissionItem() error: %v", err)
			}

			if resp.Data.ID != "item-123" {
				t.Fatalf("expected ID item-123, got %s", resp.Data.ID)
			}
			if resp.Data.Attributes.State != "READY_FOR_REVIEW" {
				t.Fatalf("expected state READY_FOR_REVIEW, got %s", resp.Data.Attributes.State)
			}
		})
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

		var payload struct {
			Data struct {
				Type       ResourceType               `json:"type"`
				ID         string                     `json:"id"`
				Attributes map[string]json.RawMessage `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeReviewSubmissionItems {
			t.Fatalf("expected type reviewSubmissionItems, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "item-123" {
			t.Fatalf("expected item ID item-123, got %s", payload.Data.ID)
		}
		if got := string(payload.Data.Attributes["resolved"]); got != "false" {
			t.Fatalf("resolved = %s, want false", got)
		}
		if got := string(payload.Data.Attributes["removed"]); got != "null" {
			t.Fatalf("removed = %s, want null", got)
		}
		if _, ok := payload.Data.Attributes["state"]; ok {
			t.Fatalf("unsupported state attribute was sent: %s", body)
		}
	}, response)

	resolved := false
	resp, err := client.UpdateReviewSubmissionItem(context.Background(), "item-123", ReviewSubmissionItemUpdateAttributes{
		Resolved: &NullableBool{Value: &resolved},
		Removed:  &NullableBool{},
	})
	if err != nil {
		t.Fatalf("UpdateReviewSubmissionItem() error: %v", err)
	}

	if resp.Data.ID != "item-123" {
		t.Fatalf("expected ID item-123, got %s", resp.Data.ID)
	}
}

func TestReviewSubmissionItemMutationsDecode441VersionRelationships(t *testing.T) {
	body := `{
		"data":{
			"type":"reviewSubmissionItems",
			"id":"item-441",
			"relationships":{
				"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iapv-1"}},
				"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"subv-1"}},
				"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"sgv-1"}}
			}
		},
		"included":[
			{"type":"inAppPurchaseVersions","id":"iapv-1"},
			{"type":"subscriptionVersions","id":"subv-1"},
			{"type":"subscriptionGroupVersions","id":"sgv-1"}
		]
	}`
	tests := []struct {
		name   string
		method string
		call   func(*Client) (*ReviewSubmissionItemResponse, error)
	}{
		{
			name:   "create",
			method: http.MethodPost,
			call: func(client *Client) (*ReviewSubmissionItemResponse, error) {
				return client.CreateReviewSubmissionItem(context.Background(), "submission-1", ReviewSubmissionItemTypeSubscriptionVersion, "subv-1")
			},
		},
		{
			name:   "update",
			method: http.MethodPatch,
			call: func(client *Client) (*ReviewSubmissionItemResponse, error) {
				resolved := true
				return client.UpdateReviewSubmissionItem(context.Background(), "item-441", ReviewSubmissionItemUpdateAttributes{Resolved: &NullableBool{Value: &resolved}})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != test.method {
					t.Fatalf("method = %q, want %q", req.Method, test.method)
				}
			}, reviewSubmissionsJSONResponse(http.StatusOK, body))

			response, err := test.call(client)
			if err != nil {
				t.Fatalf("call error: %v", err)
			}
			relationships := response.Data.Relationships
			if relationships == nil || relationships.InAppPurchaseVersion == nil || relationships.SubscriptionVersion == nil || relationships.SubscriptionGroupVersion == nil {
				t.Fatalf("relationships = %#v, want all 4.4.1 version targets", relationships)
			}
			var included []ResourceData
			if err := json.Unmarshal(response.Included, &included); err != nil || len(included) != 3 {
				t.Fatalf("included = %s, err = %v; want three 4.4.1 version resources", response.Included, err)
			}
		})
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
		],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-123/relationships/items", "first": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-123/relationships/items?cursor=first"},
		"meta": {"paging": {"total": 1, "limit": 50}}
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
	if !strings.Contains(resp.Links.First, "cursor=first") {
		t.Fatalf("linkage first link was not preserved: %+v", resp.Links)
	}
	if !strings.Contains(string(resp.Meta), `"paging"`) {
		t.Fatalf("linkage meta was not preserved: %s", resp.Meta)
	}
}

func TestGetReviewSubmissionItems(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": [
			{
				"type": "reviewSubmissionItems",
				"id": "item-456"
			}
		],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-456/items", "first": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-456/items?cursor=first"},
		"meta": {"paging": {"total": 1, "limit": 50}}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions/submission-456/items" {
			t.Fatalf("expected path /v1/reviewSubmissions/submission-456/items, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetReviewSubmissionItems(context.Background(), "submission-456")
	if err != nil {
		t.Fatalf("GetReviewSubmissionItems() error: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Data))
	}
	if !strings.Contains(resp.Links.First, "cursor=first") {
		t.Fatalf("items first link was not preserved: %+v", resp.Links)
	}
	if !strings.Contains(string(resp.Meta), `"paging"`) {
		t.Fatalf("items meta was not preserved: %s", resp.Meta)
	}
	if resp.Data[0].ID != "item-456" {
		t.Fatalf("expected item ID item-456, got %s", resp.Data[0].ID)
	}
}

func TestGetReviewSubmissionItems_WithIncludeAndFields(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": [
			{
				"type": "reviewSubmissionItems",
				"id": "item-456"
			}
		]
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions/submission-456/items" {
			t.Fatalf("expected path /v1/reviewSubmissions/submission-456/items, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("include"); got != "appStoreVersion,backgroundAssetVersion" {
			t.Fatalf("expected include query, got %q", got)
		}
		if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); got != "state,appStoreVersion,backgroundAssetVersion" {
			t.Fatalf("expected fields[reviewSubmissionItems] query, got %q", got)
		}
	}, response)

	_, err := client.GetReviewSubmissionItems(
		context.Background(),
		"submission-456",
		WithReviewSubmissionItemsInclude([]string{"appStoreVersion", "backgroundAssetVersion"}),
		WithReviewSubmissionItemsFields([]string{"state", "appStoreVersion", "backgroundAssetVersion"}),
	)
	if err != nil {
		t.Fatalf("GetReviewSubmissionItems() error: %v", err)
	}
}

func TestGetReviewSubmissionItems_With441VersionSparseFields(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data":[{
			"type":"reviewSubmissionItems",
			"id":"item-441",
			"relationships":{
				"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iapv-1"}},
				"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"subv-1"}},
				"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"sgv-1"}}
			},
			"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissionItems/item-441"}
		}],
		"included":[
			{"type":"inAppPurchaseVersions","id":"iapv-1"},
			{"type":"subscriptionVersions","id":"subv-1"},
			{"type":"subscriptionGroupVersions","id":"sgv-1"}
		],
		"meta":{"paging":{"total":1,"limit":200}}
	}`)

	client := newTestClient(t, func(req *http.Request) {
		want := url.Values{
			"fields[inAppPurchaseVersions]":     {"version,state,inAppPurchase,image,images,localizations"},
			"fields[subscriptionVersions]":      {"version,state,subscription,image,images,localizations"},
			"fields[subscriptionGroupVersions]": {"version,state,subscriptionGroup,localizations"},
		}
		if got := req.URL.Query(); !reflect.DeepEqual(got, want) {
			t.Fatalf("query = %#v, want %#v", got, want)
		}
	}, response)

	result, err := client.GetReviewSubmissionItems(
		context.Background(),
		"submission-456",
		WithReviewSubmissionItemsInAppPurchaseVersionFields([]string{" version ", "state", "inAppPurchase", "image", "images", "localizations"}),
		WithReviewSubmissionItemsSubscriptionVersionFields([]string{"version", "state", "subscription", "image", "images", "localizations"}),
		WithReviewSubmissionItemsSubscriptionGroupVersionFields([]string{"version", "state", "subscriptionGroup", "localizations"}),
	)
	if err != nil {
		t.Fatalf("GetReviewSubmissionItems() error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].Relationships == nil {
		t.Fatalf("response = %#v, want one item with relationships", result)
	}
	relationships := result.Data[0].Relationships
	if relationships.InAppPurchaseVersion == nil || relationships.InAppPurchaseVersion.Data.ID != "iapv-1" ||
		relationships.SubscriptionVersion == nil || relationships.SubscriptionVersion.Data.ID != "subv-1" ||
		relationships.SubscriptionGroupVersion == nil || relationships.SubscriptionGroupVersion.Data.ID != "sgv-1" {
		t.Fatalf("relationships = %#v, want all 4.4.1 version targets", relationships)
	}
	var included []ResourceData
	if err := json.Unmarshal(result.Included, &included); err != nil {
		t.Fatalf("decode included: %v", err)
	}
	if len(included) != 3 || included[0].Type != ResourceTypeInAppPurchaseVersions || included[1].Type != ResourceTypeSubscriptionVersions || included[2].Type != ResourceTypeSubscriptionGroupVersions {
		t.Fatalf("included = %#v, want all 4.4.1 version resources", included)
	}
	if !strings.Contains(string(result.Meta), `"total":1`) {
		t.Fatalf("meta = %s, want paging metadata", result.Meta)
	}
	if !strings.Contains(string(result.Data[0].Links), `"self"`) {
		t.Fatalf("resource links = %s, want self link", result.Data[0].Links)
	}
}

func TestGetReviewSubmissions_WithInclude(t *testing.T) {
	response := reviewSubmissionsJSONResponse(http.StatusOK, `{
		"data": [
			{
				"type": "reviewSubmissions",
				"id": "submission-456",
				"relationships": {
					"appStoreVersionForReview": {
						"data": {
							"type": "appStoreVersions",
							"id": "version-123"
						}
					}
				}
			}
		],
		"included": [
			{
				"type": "appStoreVersions",
				"id": "version-123",
				"attributes": {
					"versionString": "1.2.3",
					"platform": "IOS"
				}
			}
		]
	}`)

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/reviewSubmissions" {
			t.Fatalf("expected path /v1/apps/app-123/reviewSubmissions, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("include"); got != "appStoreVersionForReview" {
			t.Fatalf("expected include=appStoreVersionForReview, got %q", got)
		}
	}, response)

	resp, err := client.GetReviewSubmissions(context.Background(), "app-123", WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}))
	if err != nil {
		t.Fatalf("GetReviewSubmissions() error: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 submission, got %d", len(resp.Data))
	}
	if resp.Data[0].Relationships == nil || resp.Data[0].Relationships.AppStoreVersionForReview == nil {
		t.Fatal("expected appStoreVersionForReview relationship to be populated")
	}
	if resp.Data[0].Relationships.AppStoreVersionForReview.Data.ID != "version-123" {
		t.Fatalf("expected version relationship ID version-123, got %s", resp.Data[0].Relationships.AppStoreVersionForReview.Data.ID)
	}
	if len(resp.Included) == 0 {
		t.Fatal("expected included appStoreVersion payload")
	}
}

func TestReviewSubmissionGetOperationsSend441ItemFieldsAndIncludeItems(t *testing.T) {
	wantFields := "state,inAppPurchaseVersion,subscriptionVersion,subscriptionGroupVersion"
	tests := []struct {
		name string
		path string
		body string
		call func(*Client) error
	}{
		{
			name: "app related list",
			path: "/v1/apps/app-1/reviewSubmissions",
			body: `{"data":[]}`,
			call: func(client *Client) error {
				_, err := client.GetReviewSubmissions(context.Background(), "app-1", WithReviewSubmissionsItemFields(strings.Split(wantFields, ",")), WithReviewSubmissionsInclude([]string{"items"}))
				return err
			},
		},
		{
			name: "top-level list",
			path: "/v1/reviewSubmissions",
			body: `{"data":[]}`,
			call: func(client *Client) error {
				_, err := client.ListReviewSubmissions(context.Background(), WithReviewSubmissionsItemFields(strings.Split(wantFields, ",")), WithReviewSubmissionsInclude([]string{"items"}))
				return err
			},
		},
		{
			name: "detail",
			path: "/v1/reviewSubmissions/submission-1",
			body: `{"data":{"type":"reviewSubmissions","id":"submission-1"}}`,
			call: func(client *Client) error {
				_, err := client.GetReviewSubmission(context.Background(), "submission-1", WithReviewSubmissionItemFields(strings.Split(wantFields, ",")), WithReviewSubmissionInclude([]string{"items"}))
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.URL.Path != test.path {
					t.Fatalf("path = %q, want %q", req.URL.Path, test.path)
				}
				if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); got != wantFields {
					t.Fatalf("fields[reviewSubmissionItems] = %q, want %q", got, wantFields)
				}
				if got := req.URL.Query().Get("include"); got != "items" {
					t.Fatalf("include = %q, want items", got)
				}
			}, reviewSubmissionsJSONResponse(http.StatusOK, test.body))

			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
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

func countReviewSubmissionItemCreateRelationships(relationships ReviewSubmissionItemCreateRelationships) int {
	count := 0
	for _, relationship := range []*Relationship{
		relationships.InAppPurchaseVersion,
		relationships.SubscriptionVersion,
		relationships.SubscriptionGroupVersion,
		relationships.AppStoreVersion,
		relationships.AppCustomProductPageVersion,
		relationships.AppEvent,
		relationships.AppStoreVersionExperiment,
		relationships.AppStoreVersionExperimentV2,
		relationships.BackgroundAssetVersion,
		relationships.GameCenterAchievementVersion,
		relationships.GameCenterActivityVersion,
		relationships.GameCenterChallengeVersion,
		relationships.GameCenterLeaderboardSetVersion,
		relationships.GameCenterLeaderboardVersion,
	} {
		if relationship != nil {
			count++
		}
	}
	return count
}
