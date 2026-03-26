package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListReviewSubscriptionsParsesIncludedSubscriptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/subscriptionGroups" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("include"); got != "subscriptions" {
			t.Fatalf("expected include subscriptions, got %q", got)
		}
		if got := query.Get("sort"); got != "referenceName" {
			t.Fatalf("expected sort referenceName, got %q", got)
		}
		if got := query.Get("limit[subscriptions]"); got != "1000" {
			t.Fatalf("expected subscriptions limit 1000, got %q", got)
		}
		if got := query.Get("fields[subscriptions]"); !strings.Contains(got, "submitWithNextAppStoreVersion") || !strings.Contains(got, "isAppStoreReviewInProgress") {
			t.Fatalf("expected subscription review fields, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "group-1",
				"type": "subscriptionGroups",
				"attributes": {"referenceName": "Premium"},
				"relationships": {
					"subscriptions": {
						"data": [
							{"type": "subscriptions", "id": "sub-1"},
							{"type": "subscriptions", "id": "sub-2"}
						]
					}
				}
			}],
			"included": [
				{
					"id": "sub-1",
					"type": "subscriptions",
					"attributes": {
						"productId": "com.example.monthly",
						"name": "Monthly",
						"state": "READY_TO_SUBMIT",
						"isAppStoreReviewInProgress": false,
						"submitWithNextAppStoreVersion": true
					}
				},
				{
					"id": "sub-2",
					"type": "subscriptions",
					"attributes": {
						"productId": "com.example.yearly",
						"name": "Yearly",
						"state": "DEVELOPER_ACTION_NEEDED",
						"isAppStoreReviewInProgress": true,
						"submitWithNextAppStoreVersion": false
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListReviewSubscriptions(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListReviewSubscriptions() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two subscriptions, got %d", len(got))
	}
	if got[0].ID != "sub-1" || got[0].GroupID != "group-1" || got[0].GroupReferenceName != "Premium" {
		t.Fatalf("unexpected first subscription identity: %#v", got[0])
	}
	if !got[0].SubmitWithNextAppStoreVersion {
		t.Fatalf("expected first subscription to be attached, got %#v", got[0])
	}
	if !got[1].IsAppStoreReviewInProgress {
		t.Fatalf("expected second subscription review progress flag, got %#v", got[1])
	}
}

func TestListReviewSubscriptionsAggregatesPagination(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RawQuery {
		case "fields%5Bsubscriptions%5D=productId%2Cname%2Cstate%2CisAppStoreReviewInProgress%2CsubmitWithNextAppStoreVersion&include=subscriptions&limit=300&limit%5Bsubscriptions%5D=1000&sort=referenceName":
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"data": [{
					"id": "group-1",
					"type": "subscriptionGroups",
					"attributes": {"referenceName": "Alpha"},
					"relationships": {
						"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
					}
				}],
				"included": [{
					"id": "sub-1",
					"type": "subscriptions",
					"attributes": {"productId": "com.example.alpha", "name": "Alpha", "state": "READY_TO_SUBMIT"}
				}],
				"links": {
					"next": "%s/apps/app-123/subscriptionGroups?cursor=page-2"
				}
			}`, server.URL)))
		case "cursor=page-2":
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "group-2",
					"type": "subscriptionGroups",
					"attributes": {"referenceName": "Beta"},
					"relationships": {
						"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-2"}]}
					}
				}],
				"included": [{
					"id": "sub-2",
					"type": "subscriptions",
					"attributes": {"productId": "com.example.beta", "name": "Beta", "state": "APPROVED"}
				}],
				"links": {}
			}`))
		default:
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListReviewSubscriptions(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListReviewSubscriptions() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two subscriptions across pages, got %#v", got)
	}
	if got[0].ID != "sub-1" || got[1].ID != "sub-2" {
		t.Fatalf("unexpected paginated subscriptions: %#v", got)
	}
}

func TestCreateSubscriptionSubmissionSendsHiddenAttachPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriptionSubmissions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var payload struct {
			Data struct {
				Type          string `json:"type"`
				Attributes    map[string]bool
				Relationships map[string]struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload.Data.Type != "subscriptionSubmissions" {
			t.Fatalf("expected type subscriptionSubmissions, got %q", payload.Data.Type)
		}
		if !payload.Data.Attributes["submitWithNextAppStoreVersion"] {
			t.Fatalf("expected hidden attach flag to be true, got %#v", payload.Data.Attributes)
		}
		relationship := payload.Data.Relationships["subscription"].Data
		if relationship.Type != "subscriptions" || relationship.ID != "sub-1" {
			t.Fatalf("unexpected subscription relationship: %#v", relationship)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "submission-1",
				"type": "subscriptionSubmissions",
				"attributes": {"submitWithNextAppStoreVersion": true},
				"relationships": {
					"subscription": {"data": {"type": "subscriptions", "id": "sub-1"}}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.CreateSubscriptionSubmission(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("CreateSubscriptionSubmission() error = %v", err)
	}
	if got.ID != "submission-1" || got.SubscriptionID != "sub-1" || !got.SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected submission payload: %#v", got)
	}
}

func TestDeleteSubscriptionSubmissionUsesSubscriptionIDPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/subscriptionSubmissions/sub-1" {
			t.Fatalf("unexpected delete path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := testWebClient(server)
	if err := client.DeleteSubscriptionSubmission(context.Background(), "sub-1"); err != nil {
		t.Fatalf("DeleteSubscriptionSubmission() error = %v", err)
	}
}

func TestReviewSubscriptionJSONPreservesFalseBooleans(t *testing.T) {
	payload := ReviewSubscription{
		ID:                            "sub-1",
		IsAppStoreReviewInProgress:    false,
		SubmitWithNextAppStoreVersion: false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"isAppStoreReviewInProgress":false`) {
		t.Fatalf("expected false review progress flag in JSON, got %s", text)
	}
	if !strings.Contains(text, `"submitWithNextAppStoreVersion":false`) {
		t.Fatalf("expected false attach flag in JSON, got %s", text)
	}
}

func TestReviewSubscriptionSubmissionJSONPreservesFalseBooleans(t *testing.T) {
	payload := ReviewSubscriptionSubmission{
		ID:                            "submission-1",
		SubmitWithNextAppStoreVersion: false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `"submitWithNextAppStoreVersion":false`) {
		t.Fatalf("expected false attach flag in JSON, got %s", text)
	}
}

func TestListReviewSubscriptionsHandlesMissingIncludedSubscriptionResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "group-1",
				"type": "subscriptionGroups",
				"attributes": {"referenceName": "Premium"},
				"relationships": {
					"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}
				}
			}],
			"included": []
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListReviewSubscriptions(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListReviewSubscriptions() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one subscription, got %#v", got)
	}
	if got[0].ID != "sub-1" || got[0].GroupID != "group-1" || got[0].GroupReferenceName != "Premium" {
		t.Fatalf("unexpected decoded subscription identity: %#v", got[0])
	}
	if got[0].ProductID != "" || got[0].Name != "" || got[0].State != "" {
		t.Fatalf("expected missing included fields to remain empty, got %#v", got[0])
	}
}

func TestCreateSubscriptionSubmissionFallsBackToRequestedIDWhenRelationshipMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriptionSubmissions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "submission-1",
				"type": "subscriptionSubmissions",
				"attributes": {"submitWithNextAppStoreVersion": true},
				"relationships": {}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.CreateSubscriptionSubmission(context.Background(), "sub-expected")
	if err != nil {
		t.Fatalf("CreateSubscriptionSubmission() error = %v", err)
	}
	if got.ID != "submission-1" {
		t.Fatalf("expected submission id submission-1, got %#v", got)
	}
	if got.SubscriptionID != "sub-expected" {
		t.Fatalf("expected fallback subscription id sub-expected, got %#v", got)
	}
	if !got.SubmitWithNextAppStoreVersion {
		t.Fatalf("expected submitWithNextAppStoreVersion true, got %#v", got)
	}
}
