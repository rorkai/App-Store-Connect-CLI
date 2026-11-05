package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAppStatusHistoryFansOutOverVersions(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apps/app-123/appStoreVersions":
			if got := r.URL.Query().Get("fields[appStoreVersions]"); !strings.Contains(got, "versionString") {
				t.Errorf("fields[appStoreVersions] = %q, want versionString", got)
			}
			_, _ = w.Write([]byte(`{"data": [
				{"type": "appStoreVersions", "id": "v-2", "attributes": {"versionString": "2.0", "platform": "IOS"}},
				{"type": "appStoreVersions", "id": "v-1", "attributes": {"versionString": "1.0", "platform": "IOS"}}
			]}`))
		case "/appStoreVersions/v-2/appStoreVersionStateChanges":
			_, _ = w.Write([]byte(`{"data": [
				{"type": "appStoreVersionStateChanges", "id": "c-2", "attributes": {"appStoreState": "READY_FOR_SALE", "date": "2025-02-01T00:00:00Z", "initiator": "Jane Appleseed"}}
			]}`))
		case "/appStoreVersions/v-1/appStoreVersionStateChanges":
			_, _ = w.Write([]byte(`{"data": [
				{"type": "appStoreVersionStateChanges", "id": "c-1", "attributes": {"appVersionState": "READY_FOR_DISTRIBUTION", "date": "2024-02-01T00:00:00Z"}}
			]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppStatusHistory(context.Background(), "app-123", "")
	if err != nil {
		t.Fatalf("GetAppStatusHistory() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("requests = %v, want 3", paths)
	}
	if got.AppID != "app-123" || len(got.Versions) != 2 {
		t.Fatalf("unexpected history: %+v", got)
	}
	if got.Versions[0].VersionID != "v-2" || got.Versions[0].VersionString != "2.0" {
		t.Fatalf("unexpected first version: %+v", got.Versions[0])
	}
	if len(got.Versions[0].Changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(got.Versions[0].Changes))
	}
	change := got.Versions[0].Changes[0]
	if change.ID != "c-2" || change.AppStoreState != "READY_FOR_SALE" || change.Date != "2025-02-01T00:00:00Z" || change.Initiator != "Jane Appleseed" {
		t.Fatalf("unexpected change: %+v", change)
	}
	if got.Versions[1].Changes[0].AppVersionState != "READY_FOR_DISTRIBUTION" {
		t.Fatalf("unexpected second version change: %+v", got.Versions[1].Changes[0])
	}
	if got.Versions[1].Changes[0].Initiator != "" {
		t.Fatalf("expected empty initiator, got %q", got.Versions[1].Changes[0].Initiator)
	}
}

func TestGetAppStatusHistoryScopesToVersionID(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/appStoreVersions/v-7":
			if got := r.URL.Query().Get("include"); got != "app" {
				t.Errorf("include = %q, want app", got)
			}
			foundAppField := false
			for _, field := range strings.Split(r.URL.Query().Get("fields[appStoreVersions]"), ",") {
				if field == "app" {
					foundAppField = true
					break
				}
			}
			if !foundAppField {
				t.Errorf("fields[appStoreVersions] = %q, want exact app relationship field", r.URL.Query().Get("fields[appStoreVersions]"))
			}
			_, _ = w.Write([]byte(`{"data": {"type": "appStoreVersions", "id": "v-7", "attributes": {"versionString": "7.0", "platform": "MAC_OS"}, "relationships": {"app": {"data": {"type": "apps", "id": "app-123"}}}}}`))
		case "/appStoreVersions/v-7/appStoreVersionStateChanges":
			_, _ = w.Write([]byte(`{"data": [{"type": "appStoreVersionStateChanges", "id": "c-7", "attributes": {"appStoreState": "IN_REVIEW", "date": "2025-03-01T00:00:00Z"}}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppStatusHistory(context.Background(), "app-123", "v-7")
	if err != nil {
		t.Fatalf("GetAppStatusHistory() error = %v", err)
	}
	for _, path := range paths {
		if strings.Contains(path, "/apps/") {
			t.Fatalf("expected no app version list request, got %v", paths)
		}
	}
	if len(got.Versions) != 1 || got.Versions[0].VersionID != "v-7" || got.Versions[0].VersionString != "7.0" {
		t.Fatalf("unexpected scoped history: %+v", got)
	}
	if got.Versions[0].Platform != "MAC_OS" || len(got.Versions[0].Changes) != 1 {
		t.Fatalf("unexpected scoped version: %+v", got.Versions[0])
	}
}

func TestGetAppStatusHistoryRejectsVersionNotOwnedByApp(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "another app",
			body:    `{"data": {"type": "appStoreVersions", "id": "v-1", "attributes": {"versionString": "1.0"}, "relationships": {"app": {"data": {"type": "apps", "id": "app-other"}}}}}`,
			wantErr: `version "v-1" belongs to app "app-other", not "app-123"`,
		},
		{
			name:    "missing app relationship",
			body:    `{"data": {"type": "appStoreVersions", "id": "v-1", "attributes": {"versionString": "1.0"}}}`,
			wantErr: `app relationship missing for app store version "v-1"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/appStoreVersions/v-1" {
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			_, err := testWebClient(server).GetAppStatusHistory(context.Background(), "app-123", "v-1")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestGetAppStatusHistoryFollowsStateChangePagination(t *testing.T) {
	var stateChangeRequests int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apps/app-1/appStoreVersions":
			_, _ = w.Write([]byte(`{"data": [{"type": "appStoreVersions", "id": "v-1", "attributes": {"versionString": "1.0"}}]}`))
		case "/appStoreVersions/v-1/appStoreVersionStateChanges":
			stateChangeRequests++
			if r.URL.Query().Get("cursor") == "" {
				_, _ = fmt.Fprintf(w, `{"data": [{"type": "appStoreVersionStateChanges", "id": "c-1", "attributes": {"date": "2024-01-01T00:00:00Z"}}], "links": {"next": "%s/appStoreVersions/v-1/appStoreVersionStateChanges?cursor=NEXT"}}`, server.URL)
				return
			}
			_, _ = w.Write([]byte(`{"data": [{"type": "appStoreVersionStateChanges", "id": "c-2", "attributes": {"date": "2024-02-01T00:00:00Z"}}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppStatusHistory(context.Background(), "app-1", "")
	if err != nil {
		t.Fatalf("GetAppStatusHistory() error = %v", err)
	}
	if stateChangeRequests != 2 {
		t.Fatalf("state change requests = %d, want 2", stateChangeRequests)
	}
	if len(got.Versions) != 1 || len(got.Versions[0].Changes) != 2 {
		t.Fatalf("expected merged pages, got %+v", got)
	}
}

func TestGetAppStatusHistoryRequiresAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
	}))
	defer server.Close()

	if _, err := testWebClient(server).GetAppStatusHistory(context.Background(), " ", ""); err == nil {
		t.Fatal("expected error for empty app id")
	}
}

func TestGetAppStatusHistoryPropagatesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Access denied","detail":"AppStatusHistory role capability required"}]}`))
	}))
	defer server.Close()

	if _, err := testWebClient(server).GetAppStatusHistory(context.Background(), "app-123", ""); err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestGetAppStatusHistoryHandlesEmptyVersionList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppStatusHistory(context.Background(), "app-123", "")
	if err != nil {
		t.Fatalf("GetAppStatusHistory() error = %v", err)
	}
	if got.AppID != "app-123" || len(got.Versions) != 0 {
		t.Fatalf("unexpected history: %+v", got)
	}
}
