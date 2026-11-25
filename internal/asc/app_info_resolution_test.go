package asc

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestListAppInfoCandidatesForAppFollowsEveryPage(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newTestClient(
		t, func(req *http.Request) {
			requests = append(requests, req.URL.String())
		},
		jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}}],"links":{"next":"HTTPS://api.appstoreconnect.apple.com/v1/apps/app-1/appInfos?cursor=next"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-current","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`),
	)

	got, err := client.ListAppInfoCandidatesForApp(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("ListAppInfoCandidatesForApp() error = %v", err)
	}
	want := []AppInfoCandidate{{ID: "info-current", State: "READY_FOR_DISTRIBUTION"}, {ID: "info-old", State: "REPLACED_WITH_NEW_INFO"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListAppInfoCandidatesForApp() = %+v, want %+v", got, want)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %v, want two pages", requests)
	}
	if !strings.Contains(requests[0], "limit=200") || !strings.Contains(requests[0], "fields%5BappInfos%5D=state") {
		t.Fatalf("first request = %q, want maximum page size and state field", requests[0])
	}
	if !strings.Contains(requests[1], "cursor=next") {
		t.Fatalf("second request = %q, want continuation URL", requests[1])
	}
}

func TestListAppInfoCandidatesForAppRejectsCrossAppContinuation(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newTestClient(
		t, func(req *http.Request) {
			requests = append(requests, req.URL.String())
		},
		jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-2/appInfos?cursor=next"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-foreign","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`),
	)

	_, err := client.ListAppInfoCandidatesForApp(context.Background(), "app-1")
	if err == nil || !strings.Contains(err.Error(), `target app "app-2" does not match selected app "app-1"`) {
		t.Fatalf("ListAppInfoCandidatesForApp() error = %v, want cross-app continuation refusal", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %v, want only the first app-scoped page", requests)
	}
}

func TestAutoResolveAppInfoIDByVersionState(t *testing.T) {
	tests := []struct {
		name         string
		versionState string
		candidates   []AppInfoCandidate
		wantID       string
		wantOK       bool
	}{
		{
			name:         "matches exact shared state",
			versionState: "WAITING_FOR_REVIEW",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "PREPARE_FOR_SUBMISSION"},
				{ID: "info-2", State: "WAITING_FOR_REVIEW"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps pending developer release to pending release",
			versionState: "PENDING_DEVELOPER_RELEASE",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "PREPARE_FOR_SUBMISSION"},
				{ID: "info-2", State: "PENDING_RELEASE"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps pending apple release to pending release",
			versionState: "PENDING_APPLE_RELEASE",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "WAITING_FOR_REVIEW"},
				{ID: "info-2", State: "PENDING_RELEASE"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps replaced with new version to replaced with new info",
			versionState: "REPLACED_WITH_NEW_VERSION",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "READY_FOR_REVIEW"},
				{ID: "info-2", State: "REPLACED_WITH_NEW_INFO"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps ready for sale fallback to ready for distribution",
			versionState: "READY_FOR_SALE",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "READY_FOR_REVIEW"},
				{ID: "info-2", State: "READY_FOR_DISTRIBUTION"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps preorder ready for sale to ready for distribution",
			versionState: "PREORDER_READY_FOR_SALE",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "READY_FOR_REVIEW"},
				{ID: "info-2", State: "READY_FOR_DISTRIBUTION"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "returns false when alias remains ambiguous",
			versionState: "PENDING_DEVELOPER_RELEASE",
			candidates: []AppInfoCandidate{
				{ID: "info-1", State: "PENDING_RELEASE"},
				{ID: "info-2", State: "PENDING_RELEASE"},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := AutoResolveAppInfoIDByVersionState(tt.candidates, tt.versionState)
			if gotOK != tt.wantOK {
				t.Fatalf("AutoResolveAppInfoIDByVersionState() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotID != tt.wantID {
				t.Fatalf("AutoResolveAppInfoIDByVersionState() id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func TestResolveAppStoreVersionStatePrefersAppVersionState(t *testing.T) {
	attrs := AppStoreVersionAttributes{
		AppVersionState: "PREORDER_READY_FOR_SALE",
		AppStoreState:   "READY_FOR_SALE",
	}

	got := ResolveAppStoreVersionState(attrs)
	if got != "PREORDER_READY_FOR_SALE" {
		t.Fatalf("ResolveAppStoreVersionState() = %q, want %q", got, "PREORDER_READY_FOR_SALE")
	}
}
