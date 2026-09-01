package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertRatingResetWebHeaders(t *testing.T, request *http.Request) {
	t.Helper()
	for name, want := range map[string]string{
		"Accept":           "application/json",
		"Content-Type":     "application/json",
		"Origin":           appStoreBaseURL,
		"Referer":          appStoreBaseURL + "/",
		"X-Requested-With": "XMLHttpRequest",
		"X-CSRF-ITC":       "[asc-ui]",
	} {
		if got := request.Header.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestGetAppStoreVersionRatingResetRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", request.Method)
		}
		if request.URL.EscapedPath() != "/appStoreVersions/version%2F123/resetRatingsRequest" {
			t.Fatalf("path = %q, want escaped version relationship path", request.URL.EscapedPath())
		}
		assertRatingResetWebHeaders(t, request)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"data": {
				"type": "resetRatingsRequests",
				"id": "reset-123",
				"attributes": {"resetDate": "2026-09-01T08:00:00Z"}
			}
		}`))
	}))
	defer server.Close()

	result, err := testWebClient(server).GetAppStoreVersionRatingResetRequest(context.Background(), " version/123 ")
	if err != nil {
		t.Fatalf("GetAppStoreVersionRatingResetRequest() error = %v", err)
	}
	if result.Data.ID != "reset-123" {
		t.Fatalf("id = %q, want reset-123", result.Data.ID)
	}
	if result.Data.Attributes.ResetDate == nil || *result.Data.Attributes.ResetDate != "2026-09-01T08:00:00Z" {
		t.Fatalf("resetDate = %v, want decoded date", result.Data.Attributes.ResetDate)
	}
}

func TestCreateAppStoreVersionRatingResetRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/resetRatingsRequests" {
			t.Fatalf("request = %s %s, want POST /resetRatingsRequests", request.Method, request.URL.Path)
		}
		assertRatingResetWebHeaders(t, request)

		var payload struct {
			Data struct {
				Type          string `json:"type"`
				Relationships struct {
					AppStoreVersion struct {
						Data struct {
							Type string `json:"type"`
							ID   string `json:"id"`
						} `json:"data"`
					} `json:"appStoreVersion"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.Type != ratingResetRequestResourceType {
			t.Fatalf("type = %q, want %q", payload.Data.Type, ratingResetRequestResourceType)
		}
		version := payload.Data.Relationships.AppStoreVersion.Data
		if version.Type != "appStoreVersions" || version.ID != "version-123" {
			t.Fatalf("version relationship = %#v", version)
		}

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte(`{
			"data": {
				"type": "resetRatingsRequests",
				"id": "reset-new",
				"attributes": {"resetDate": null}
			}
		}`))
	}))
	defer server.Close()

	result, err := testWebClient(server).CreateAppStoreVersionRatingResetRequest(context.Background(), " version-123 ")
	if err != nil {
		t.Fatalf("CreateAppStoreVersionRatingResetRequest() error = %v", err)
	}
	if result.Data.ID != "reset-new" {
		t.Fatalf("id = %q, want reset-new", result.Data.ID)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if !strings.Contains(string(encoded), `"resetDate":null`) {
		t.Fatalf("response = %s, want resetDate null preserved", encoded)
	}
	if strings.Contains(string(encoded), `"links"`) {
		t.Fatalf("response = %s, want absent links to stay absent", encoded)
	}
}

func TestDeleteAppStoreVersionRatingResetRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", request.Method)
		}
		if request.URL.EscapedPath() != "/resetRatingsRequests/reset%2F123" {
			t.Fatalf("path = %q, want escaped request path", request.URL.EscapedPath())
		}
		assertRatingResetWebHeaders(t, request)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := testWebClient(server).DeleteAppStoreVersionRatingResetRequest(context.Background(), " reset/123 "); err != nil {
		t.Fatalf("DeleteAppStoreVersionRatingResetRequest() error = %v", err)
	}
}

func TestRatingResetRequestValidationPreventsNetworkCalls(t *testing.T) {
	client := &Client{}
	if _, err := client.GetAppStoreVersionRatingResetRequest(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "version id is required") {
		t.Fatalf("get error = %v, want version validation", err)
	}
	if _, err := client.CreateAppStoreVersionRatingResetRequest(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "version id is required") {
		t.Fatalf("create error = %v, want version validation", err)
	}
	if err := client.DeleteAppStoreVersionRatingResetRequest(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "rating reset request id is required") {
		t.Fatalf("delete error = %v, want request validation", err)
	}
}

func TestRatingResetRequestPropagatesSanitizedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("X-Apple-Request-UUID", "request-123")
		response.WriteHeader(http.StatusConflict)
		_, _ = response.Write([]byte(`{"errors":[{"code":"STATE_ERROR","detail":"sensitive detail"}]}`))
	}))
	defer server.Close()

	_, err := testWebClient(server).CreateAppStoreVersionRatingResetRequest(context.Background(), "version-123")
	if err == nil {
		t.Fatal("expected API error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("error = %v, want 409 APIError", err)
	}
	if strings.Contains(err.Error(), "sensitive detail") {
		t.Fatalf("error leaked response detail: %v", err)
	}
	if !strings.Contains(err.Error(), "request_id=request-123") || !strings.Contains(err.Error(), "STATE_ERROR") {
		t.Fatalf("error = %v, want request id and service code", err)
	}
}

func TestRatingResetRequestRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"data":`))
	}))
	defer server.Close()

	_, err := testWebClient(server).GetAppStoreVersionRatingResetRequest(context.Background(), "version-123")
	if err == nil || !strings.Contains(err.Error(), "failed to parse rating reset response") {
		t.Fatalf("error = %v, want parse context", err)
	}
}
