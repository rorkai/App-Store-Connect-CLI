package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListCustomAppUsersPreservesRawAppleEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/apps/app-1/customAppUsers" || r.URL.RawQuery != "limit=100" {
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
  "data": [{"type":"customAppUsers","id":"recipient-1","attributes":{"appleId":"account@example.com","future":true},"links":{"self":"/customAppUsers/recipient-1"},"unknownResource":{"keep":"yes"}}],
  "links": {"self":"/apps/app-1/customAppUsers"},
  "meta": {"paging":{"total":1,"limit":100}},
  "unknownTopLevel": {"keep": true}
}`)
	}))
	defer server.Close()

	result, err := testWebClient(server).listCustomAppUsers(context.Background(), "app-1", false)
	if err != nil {
		t.Fatalf("ListCustomAppUsers() error = %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "recipient-1" || result.Data[0].AppleID != "account@example.com" {
		t.Fatalf("unexpected data: %+v", result.Data)
	}
	if !strings.Contains(string(result.Raw), `"unknownTopLevel"`) || !strings.Contains(string(result.Data[0].Raw), `"future":true`) {
		t.Fatalf("raw response omitted unknown members: envelope=%s resource=%s", result.Raw, result.Data[0].Raw)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	assertCompactJSONEqual(t, encoded, result.Raw)
}

func TestListCustomAppUsersPaginatedAggregatesOnlySelectedAppCollection(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"type":"customAppUsers","id":"recipient-1","attributes":{"appleId":"one@example.com"}}],"links":{"self":"/iris/v1/apps/app-1/customAppUsers","next":"?cursor=two"},"meta":{"paging":{"total":2,"limit":100}},"unknownTopLevel":{"keep":true}}`)
		case "two":
			_, _ = io.WriteString(w, `{"data":[{"type":"customAppUsers","id":"recipient-2","attributes":{"appleId":"two@example.com"},"future":42}],"links":{"self":"/iris/v1/apps/app-1/customAppUsers?cursor=two"},"meta":{"paging":{"total":2,"limit":100}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	client.baseURL += "/iris/v1"
	result, err := client.ListCustomAppUsersPaginated(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("ListCustomAppUsersPaginated() error = %v", err)
	}
	if len(requests) != 2 || requests[0] != "/iris/v1/apps/app-1/customAppUsers?limit=100" || requests[1] != "/iris/v1/apps/app-1/customAppUsers?cursor=two" {
		t.Fatalf("requests = %v, want first page and selected-app next page", requests)
	}
	if len(result.Data) != 2 || result.Data[1].ID != "recipient-2" {
		t.Fatalf("unexpected aggregate: %+v", result.Data)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"unknownTopLevel":{"keep":true}`) || !strings.Contains(string(encoded), `"future":42`) {
		t.Fatalf("aggregate omitted raw members: %s", encoded)
	}
	if strings.Contains(string(encoded), `"next"`) {
		t.Fatalf("aggregate retained continuation link: %s", encoded)
	}
}

func TestListCustomAppUsersRejectsMalformedOrForeignCollection(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "null total", body: `{"data":[],"links":{"self":"/apps/app-1/customAppUsers"},"meta":{"paging":{"total":null}}}`, want: "non-negative integer"},
		{name: "foreign next", body: `{"data":[],"links":{"self":"/apps/app-1/customAppUsers","next":"/apps/app-2/customAppUsers"}}`, want: "outside selected app"},
		{name: "null data", body: `{"data":null,"links":{"self":"/apps/app-1/customAppUsers"}}`, want: "non-null data"},
		{name: "wrong resource type", body: `{"data":[{"type":"customAppOrganizations","id":"org-1","attributes":{"appleId":"one@example.com"}}],"links":{"self":"/apps/app-1/customAppUsers"}}`, want: "resource type"},
		{name: "foreign collection", body: `{"data":[],"links":{"self":"/apps/app-2/customAppUsers"}}`, want: "outside selected app"},
		{name: "foreign host", body: `{"data":[],"links":{"self":"https://example.invalid/iris/v1/apps/app-1/customAppUsers"}}`, want: "does not match client host"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			_, err := testWebClient(server).ListCustomAppUsersPaginated(context.Background(), "app-1")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCustomAppUserMutationsUseScopedHeadersAndObservedBodies(t *testing.T) {
	var createBody map[string]any
	var deleteBody map[string]any
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if got := r.Header.Get("X-CSRF-ITC"); got != "[asc-ui]" {
			t.Fatalf("X-CSRF-ITC = %q, want [asc-ui]", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("Referer"); got != appStoreBaseURL+"/apps/app-1/distribution/pricing" {
			t.Fatalf("Referer = %q, want app pricing route", got)
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if r.Method == http.MethodPost {
			if err := json.Unmarshal(payload, &createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"type":"customAppUsers","id":"recipient-1","attributes":{"appleId":"account@example.com"},"unknown":true}}`)
			return
		}
		if r.Method == http.MethodDelete {
			if err := json.Unmarshal(payload, &deleteBody); err != nil {
				t.Fatalf("decode delete body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected method %s", r.Method)
	}))
	defer server.Close()

	client := testWebClient(server)
	created, err := client.CreateCustomAppUser(context.Background(), "app-1", "account@example.com")
	if err != nil || created == nil || created.ID != "recipient-1" {
		t.Fatalf("CreateCustomAppUser() = %+v, %v", created, err)
	}
	if err := client.DeleteCustomAppUser(context.Background(), "app-1", "recipient-1"); err != nil {
		t.Fatalf("DeleteCustomAppUser() error = %v", err)
	}
	if len(methods) != 2 || methods[0] != http.MethodPost || methods[1] != http.MethodDelete {
		t.Fatalf("methods = %v, want POST then DELETE", methods)
	}
	createData := createBody["data"].(map[string]any)
	if createData["type"] != customAppUserResourceType || createData["relationships"].(map[string]any)["app"].(map[string]any)["data"].(map[string]any)["id"] != "app-1" {
		t.Fatalf("unexpected create body: %#v", createBody)
	}
	if deleteBody["data"].(map[string]any)["type"] != customAppUserResourceType || deleteBody["data"].(map[string]any)["id"] != "recipient-1" {
		t.Fatalf("unexpected delete body: %#v", deleteBody)
	}
}

func TestCreateCustomAppUserDoesNotRetryUncertainProviderError(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"errors":[{"status":"503","title":"temporary"}]}`)
	}))
	defer server.Close()

	_, err := testWebClient(server).CreateCustomAppUser(context.Background(), "app-1", "account@example.com")
	if err == nil || !IsCustomAppUserWriteUncertain(err) {
		t.Fatalf("error = %v, want uncertain provider error", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("error = %v, want APIError 503", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one mutation attempt", requests)
	}
}

func TestListCustomAppUsersAllowsPartialFirstPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"data":[{"type":"customAppUsers","id":"one","attributes":{"appleId":"one@example.com"}}],"links":{"self":"/apps/app-1/customAppUsers","next":"/apps/app-1/customAppUsers?cursor=two"},"meta":{"paging":{"total":2}}}`)
	}))
	defer server.Close()
	result, err := testWebClient(server).listCustomAppUsers(context.Background(), "app-1", false)
	if err != nil || calls != 1 || result == nil || len(result.Data) != 1 || result.GetLinks().Next == "" {
		t.Fatalf("partial list result=%+v calls=%d err=%v", result, calls, err)
	}
}

func TestListCustomAppUsersPreservesObjectContinuationDiagnostics(t *testing.T) {
	for _, field := range []string{"href", "url"} {
		t.Run(field, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, `{"data":[],"links":{"self":"/apps/app-1/customAppUsers","next":{"`+field+`":"/apps/app-1/customAppUsers?cursor=next"}},"meta":{"paging":{"total":1}}}`)
			}))
			defer server.Close()
			result, err := testWebClient(server).listCustomAppUsers(context.Background(), "app-1", false)
			if err != nil {
				t.Fatal(err)
			}
			if result.GetLinks().Next != "/apps/app-1/customAppUsers?cursor=next" {
				t.Fatalf("continuation lost: %+v", result.GetLinks())
			}
		})
	}
}
