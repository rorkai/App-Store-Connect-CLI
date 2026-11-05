package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFindReviewIAPReturnsFirstMatchingAppScopedIAP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/inAppPurchases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Query().Get("limit"); got != "300" {
			t.Fatalf("expected limit=300, got %q", got)
		}
		fields := r.URL.Query().Get("fields[inAppPurchases]")
		fieldTokens := map[string]bool{}
		for _, field := range strings.Split(fields, ",") {
			fieldTokens[strings.TrimSpace(field)] = true
		}
		for _, want := range []string{"productId", "referenceName", "state"} {
			if !fieldTokens[want] {
				t.Fatalf("expected fields to contain %q, got %q", want, fields)
			}
		}
		// Apple's iris flavor rejects these names with PARAMETER_ERROR.INVALID
		// (verified against /apps/{APP_ID}/inAppPurchases) — guard against
		// re-adding them in a future patch.
		for _, forbidden := range []string{"name", "inAppPurchaseType", "isAppStoreReviewInProgress", "submitWithNextAppStoreVersion"} {
			if fieldTokens[forbidden] {
				t.Fatalf("fields must not include %q (iris rejects it): %q", forbidden, fields)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "iap-1",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "com.example.removeads",
						"referenceName": "Remove Ads",
						"state": "READY_TO_SUBMIT"
					}
				}
			],
			"links": {
				"next": ""
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-1")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found")
	}
	if got.ID != "iap-1" || got.ProductID != "com.example.removeads" || got.ReferenceName != "Remove Ads" || got.State != "READY_TO_SUBMIT" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}
}

func TestFindReviewIAPMatchesByReferenceName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "iap-reference-name",
				"type": "inAppPurchases",
				"attributes": {
					"productId": "com.example.lifetime",
					"referenceName": "Lifetime Access",
					"state": "READY_TO_SUBMIT"
				}
			}],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "lifetime access")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found by reference name")
	}
	if got.ID != "iap-reference-name" || got.ReferenceName != "Lifetime Access" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}
}

func TestFindReviewIAPRejectsAmbiguousProductID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"iap-1","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"First"}},
				{"id":"iap-2","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"Second"}}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "com.example.duplicate")
	if err == nil {
		t.Fatal("expected ambiguous product ID to fail")
	}
	if found {
		t.Fatal("ambiguous product ID must not report a match")
	}
	if !strings.Contains(err.Error(), "matches 2 in-app purchases by product ID") {
		t.Fatalf("expected ambiguity diagnostic, got %q", err)
	}
	var ambiguous *ReviewIAPAmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected typed ambiguity error, got %T", err)
	}
	if len(ambiguous.Matches) != 2 || ambiguous.Matches[0].ID != "iap-1" || ambiguous.Matches[1].ID != "iap-2" {
		t.Fatalf("ambiguity must retain recovery IDs, got %#v", ambiguous.Matches)
	}
}

func TestFindReviewIAPRejectsAmbiguousReferenceName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"iap-1","type":"inAppPurchases","attributes":{"productId":"com.example.first","referenceName":"Duplicate Name"}},
				{"id":"iap-2","type":"inAppPurchases","attributes":{"productId":"com.example.second","referenceName":"duplicate name"}}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "duplicate name")
	if err == nil {
		t.Fatal("expected ambiguous reference name to fail")
	}
	if found {
		t.Fatal("ambiguous reference name must not report a match")
	}
	if !strings.Contains(err.Error(), "matches 2 in-app purchases by reference name") {
		t.Fatalf("expected ambiguity diagnostic, got %q", err)
	}
}

func TestFindReviewIAPPrefersExactResourceIDOverProductIDFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "resource-from-product-fallback",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "iap-2",
						"referenceName": "Collision Product",
						"state": "READY_TO_SUBMIT"
					}
				},
				{
					"id": "iap-2",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "com.example.target",
						"referenceName": "Exact Resource",
						"state": "READY_TO_SUBMIT"
					}
				}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-2")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found")
	}
	if got.ID != "iap-2" || got.ProductID != "com.example.target" {
		t.Fatalf("expected exact resource-id match to win, got %#v", got)
	}
}

func TestFindReviewIAPPrefersExactResourceIDOnLaterPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "resource-from-product-fallback",
					"type": "inAppPurchases",
					"attributes": {"productId": "iap-2"}
				}],
				"links": {"next": "/apps/app-123/inAppPurchases?page=2"}
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "iap-2",
					"type": "inAppPurchases",
					"attributes": {"productId": "com.example.target"}
				}],
				"links": {"next": ""}
			}`))
		default:
			t.Fatalf("unexpected extra request %d: %s", calls, r.URL.String())
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-2")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found")
	}
	if got.ID != "iap-2" || got.ProductID != "com.example.target" {
		t.Fatalf("expected later exact resource-id match to win, got %#v", got)
	}
	if calls != 2 {
		t.Fatalf("expected two paginated requests, got %d", calls)
	}
}

func TestFindReviewIAPRejectsAmbiguousProductIDAcrossPages(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "iap-1",
					"type": "inAppPurchases",
					"attributes": {"productId": "com.example.duplicate", "referenceName": "First"}
				}],
				"links": {"next": "/apps/app-123/inAppPurchases?page=2"}
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "iap-2",
					"type": "inAppPurchases",
					"attributes": {"productId": "com.example.duplicate", "referenceName": "Second"}
				}],
				"links": {"next": ""}
			}`))
		default:
			t.Fatalf("unexpected extra request %d: %s", calls, r.URL.String())
		}
	}))
	defer server.Close()

	_, found, err := testWebClient(server).FindReviewIAP(context.Background(), "app-123", "com.example.duplicate")
	if err == nil || !strings.Contains(err.Error(), "matches 2 in-app purchases by product ID") {
		t.Fatalf("expected cross-page ambiguity error, got found=%t err=%v", found, err)
	}
	if found {
		t.Fatal("ambiguous cross-page product ID must not report a match")
	}
	if calls != 2 {
		t.Fatalf("expected both pages to be scanned, got %d requests", calls)
	}
}

func TestFindReviewIAPMatchesMaxBinRedactedSchema(t *testing.T) {
	const (
		publicASCIAPID = "1234567890"
		irisResourceID = "ae6d89d7-15c5-4a3d-9041-663a4d40638e"
		productID      = "com.example.app.pro.lifetime"
	)

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/apps/app-123/inAppPurchases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields[inAppPurchases]"); got != reviewIAPFields {
			t.Fatalf("expected iris-safe fields %q, got %q", reviewIAPFields, got)
		}
		if got := r.URL.Query().Get("sort"); got != "referenceName" {
			t.Fatalf("expected sort=referenceName, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "` + irisResourceID + `",
				"type": "inAppPurchases",
				"attributes": {
					"productId": "` + productID + `",
					"referenceName": "Pro Lifetime",
					"state": "READY_TO_SUBMIT"
				}
			}],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", productID)
	if err != nil {
		t.Fatalf("FindReviewIAP(productID) error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found by bundle-style productId")
	}
	if got.ID != irisResourceID || got.ProductID != productID || got.ReferenceName != "Pro Lifetime" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}

	_, found, err = client.FindReviewIAP(context.Background(), "app-123", publicASCIAPID)
	if err != nil {
		t.Fatalf("FindReviewIAP(public numeric id) error = %v", err)
	}
	if found {
		t.Fatalf("public numeric ASC IAP id %q must not match Iris schema that only exposes id=%q and productId=%q", publicASCIAPID, irisResourceID, productID)
	}
	if requests != 2 {
		t.Fatalf("expected two lookup requests, got %d", requests)
	}
}

func TestFindReviewIAPStopsAfterMatchingPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 1 {
			t.Fatalf("expected lookup to stop after finding target, got request %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "iap-1",
				"type": "inAppPurchases",
				"attributes": {"productId": "com.example.removeads"}
			}],
			"links": {
				"next": "https://example.com/should-not-fetch"
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-1")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found || got.ID != "iap-1" {
		t.Fatalf("expected target IAP, got found=%t payload=%#v", found, got)
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}

func TestFindReviewIAPReturnsFalseWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "missing-iap")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if found {
		t.Fatal("expected missing IAP")
	}
}

func TestFindReviewIAPRejectsUnexpectedResourceType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"sub-1","type":"subscriptions","attributes":{"productId":"com.example.wrong","referenceName":"Wrong type"}}]}`))
	}))
	defer server.Close()

	_, found, err := testWebClient(server).FindReviewIAP(context.Background(), "app-123", "com.example.wrong")
	if err == nil || !strings.Contains(err.Error(), "unexpected resource type") {
		t.Fatalf("expected unexpected-type error, got found=%t err=%v", found, err)
	}
	if found {
		t.Fatal("unexpected resource type must not be reported as a match")
	}
}

func TestFindReviewIAPRejectsEmptyIDs(t *testing.T) {
	for _, tc := range []struct{ name, appID, iapID string }{
		{name: "empty app id", appID: "   ", iapID: "iap-1"},
		{name: "empty iap id", appID: "app-123", iapID: "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, found, err := (&Client{}).FindReviewIAP(context.Background(), tc.appID, tc.iapID)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if found {
				t.Fatal("expected found=false")
			}
		})
	}
}

func TestCreateReviewIAPSubmissionItemValidatesResponse(t *testing.T) {
	tests := []struct{ name, body, wantErr string }{
		{"valid", `{"data":{"type":"reviewSubmissionItems","id":"item-1","relationships":{"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}}`, ""},
		{"relationship omitted", `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`, ""},
		{"missing id", `{"data":{"type":"reviewSubmissionItems"}}`, "resource id"},
		{"wrong resource type", `{"data":{"type":"inAppPurchaseSubmissions","id":"item-1"}}`, "unexpected resource type"},
		{"missing resource type", `{"data":{"id":"item-1"}}`, "unexpected resource type"},
		{"wrong version", `{"data":{"type":"reviewSubmissionItems","id":"item-1","relationships":{"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"version-other"}}}}}`, "version-other"},
		{"null version", `{"data":{"type":"reviewSubmissionItems","id":"item-1","relationships":{"inAppPurchaseVersion":{"data":null}}}}`, "no resource data"},
		{"wrong draft", `{"data":{"type":"reviewSubmissionItems","id":"item-1","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"draft-other"}}}}}`, "draft-other"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/reviewSubmissionItems" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			id, err := testWebClient(server).CreateReviewIAPSubmissionItem(context.Background(), "draft-1", "version-1")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "before retrying") {
					t.Fatalf("error=%v, want uncertain-write warning and %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || id != "item-1" {
				t.Fatalf("id=%q error=%v", id, err)
			}
		})
	}
}

func TestCreateReviewIAPSubmissionItemPreservesSanitizedPortalReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"code":"STATE_ERROR","title":"Attachment refused","detail":"The IAP is not ready\u001b[31m"}]}`))
	}))
	defer server.Close()
	_, err := testWebClient(server).CreateReviewIAPSubmissionItem(context.Background(), "draft-1", "version-1")
	if err == nil || !strings.Contains(err.Error(), "status 422") || !strings.Contains(err.Error(), "Attachment refused: The IAP is not ready[31m") || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("expected status and sanitized portal reason, got %v", err)
	}
}
