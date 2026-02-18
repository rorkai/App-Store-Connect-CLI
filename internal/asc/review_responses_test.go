package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func newReviewResponsesTestClient(t *testing.T, check func(*http.Request), response *http.Response) *Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if check != nil {
			check(req)
		}
		return response, nil
	})

	return &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func reviewResponsesJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCreateCustomerReviewResponse(t *testing.T) {
	response := reviewResponsesJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "customerReviewResponses",
			"id": "response-123",
			"attributes": {
				"responseBody": "Thanks for the feedback!",
				"lastModifiedDate": "2026-01-20T00:00:00Z",
				"state": "PUBLISHED"
			}
		}
	}`)

	client := newReviewResponsesTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/customerReviewResponses" {
			t.Fatalf("expected path /v1/customerReviewResponses, got %s", req.URL.Path)
		}

		// Verify request body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload CustomerReviewResponseCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeCustomerReviewResponses {
			t.Fatalf("expected type customerReviewResponses, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.ResponseBody != "Thanks for the feedback!" {
			t.Fatalf("expected responseBody 'Thanks for the feedback!', got %s", payload.Data.Attributes.ResponseBody)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Review == nil {
			t.Fatal("expected relationships.review to be set")
		}
		if payload.Data.Relationships.Review.Data.ID != "review-456" {
			t.Fatalf("expected review ID 'review-456', got %s", payload.Data.Relationships.Review.Data.ID)
		}
	}, response)

	resp, err := client.CreateCustomerReviewResponse(context.Background(), "review-456", "Thanks for the feedback!")
	if err != nil {
		t.Fatalf("CreateCustomerReviewResponse() error: %v", err)
	}

	if resp.Data.ID != "response-123" {
		t.Fatalf("expected ID response-123, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.ResponseBody != "Thanks for the feedback!" {
		t.Fatalf("expected responseBody 'Thanks for the feedback!', got %s", resp.Data.Attributes.ResponseBody)
	}
	if resp.Data.Attributes.State != "PUBLISHED" {
		t.Fatalf("expected state PUBLISHED, got %s", resp.Data.Attributes.State)
	}
}

func TestCreateCustomerReviewResponse_ValidationErrors(t *testing.T) {
	client := newReviewResponsesTestClient(t, nil, nil)

	// Missing review ID
	_, err := client.CreateCustomerReviewResponse(context.Background(), "", "response")
	if err == nil {
		t.Fatalf("expected error for missing reviewID, got nil")
	}

	// Missing response body
	_, err = client.CreateCustomerReviewResponse(context.Background(), "review-123", "")
	if err == nil {
		t.Fatalf("expected error for missing responseBody, got nil")
	}

	// Whitespace-only values
	_, err = client.CreateCustomerReviewResponse(context.Background(), "   ", "response")
	if err == nil {
		t.Fatalf("expected error for whitespace reviewID, got nil")
	}

	_, err = client.CreateCustomerReviewResponse(context.Background(), "review-123", "   ")
	if err == nil {
		t.Fatalf("expected error for whitespace responseBody, got nil")
	}
}

func TestGetCustomerReviewResponse(t *testing.T) {
	response := reviewResponsesJSONResponse(http.StatusOK, `{
		"data": {
			"type": "customerReviewResponses",
			"id": "response-123",
			"attributes": {
				"responseBody": "We appreciate your feedback!",
				"lastModifiedDate": "2026-01-20T00:00:00Z",
				"state": "PUBLISHED"
			}
		}
	}`)

	client := newReviewResponsesTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/customerReviewResponses/response-123" {
			t.Fatalf("expected path /v1/customerReviewResponses/response-123, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetCustomerReviewResponse(context.Background(), "response-123")
	if err != nil {
		t.Fatalf("GetCustomerReviewResponse() error: %v", err)
	}

	if resp.Data.ID != "response-123" {
		t.Fatalf("expected ID response-123, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.ResponseBody != "We appreciate your feedback!" {
		t.Fatalf("expected responseBody 'We appreciate your feedback!', got %s", resp.Data.Attributes.ResponseBody)
	}
}

func TestGetCustomerReviewResponse_ValidationErrors(t *testing.T) {
	client := newReviewResponsesTestClient(t, nil, nil)

	// Missing response ID
	_, err := client.GetCustomerReviewResponse(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for missing responseID, got nil")
	}

	// Whitespace-only ID
	_, err = client.GetCustomerReviewResponse(context.Background(), "   ")
	if err == nil {
		t.Fatalf("expected error for whitespace responseID, got nil")
	}
}

func TestDeleteCustomerReviewResponse(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	client := newReviewResponsesTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/customerReviewResponses/response-123" {
			t.Fatalf("expected path /v1/customerReviewResponses/response-123, got %s", req.URL.Path)
		}
	}, response)

	err := client.DeleteCustomerReviewResponse(context.Background(), "response-123")
	if err != nil {
		t.Fatalf("DeleteCustomerReviewResponse() error: %v", err)
	}
}

func TestDeleteCustomerReviewResponse_ValidationErrors(t *testing.T) {
	client := newReviewResponsesTestClient(t, nil, nil)

	// Missing response ID
	err := client.DeleteCustomerReviewResponse(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for missing responseID, got nil")
	}

	// Whitespace-only ID
	err = client.DeleteCustomerReviewResponse(context.Background(), "   ")
	if err == nil {
		t.Fatalf("expected error for whitespace responseID, got nil")
	}
}

func TestGetCustomerReviewResponseForReview(t *testing.T) {
	response := reviewResponsesJSONResponse(http.StatusOK, `{
		"data": {
			"type": "customerReviewResponses",
			"id": "response-789",
			"attributes": {
				"responseBody": "Thank you for your review!",
				"lastModifiedDate": "2026-01-20T00:00:00Z",
				"state": "PUBLISHED"
			}
		}
	}`)

	client := newReviewResponsesTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/customerReviews/review-456/response" {
			t.Fatalf("expected path /v1/customerReviews/review-456/response, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetCustomerReviewResponseForReview(context.Background(), "review-456")
	if err != nil {
		t.Fatalf("GetCustomerReviewResponseForReview() error: %v", err)
	}

	if resp.Data.ID != "response-789" {
		t.Fatalf("expected ID response-789, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.ResponseBody != "Thank you for your review!" {
		t.Fatalf("expected responseBody 'Thank you for your review!', got %s", resp.Data.Attributes.ResponseBody)
	}
}

func TestGetCustomerReviewResponseRelationshipForReview(t *testing.T) {
	response := reviewResponsesJSONResponse(http.StatusOK, `{
		"data": {
			"type": "customerReviewResponses",
			"id": "response-789"
		}
	}`)

	client := newReviewResponsesTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/customerReviews/review-456/relationships/response" {
			t.Fatalf("expected path /v1/customerReviews/review-456/relationships/response, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetCustomerReviewResponseRelationshipForReview(context.Background(), "review-456")
	if err != nil {
		t.Fatalf("GetCustomerReviewResponseRelationshipForReview() error: %v", err)
	}

	if resp.Data.ID != "response-789" {
		t.Fatalf("expected ID response-789, got %s", resp.Data.ID)
	}
}

func TestGetCustomerReviewResponseForReview_ValidationErrors(t *testing.T) {
	client := newReviewResponsesTestClient(t, nil, nil)

	// Missing review ID
	_, err := client.GetCustomerReviewResponseForReview(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for missing reviewID, got nil")
	}

	// Whitespace-only ID
	_, err = client.GetCustomerReviewResponseForReview(context.Background(), "   ")
	if err == nil {
		t.Fatalf("expected error for whitespace reviewID, got nil")
	}
}
