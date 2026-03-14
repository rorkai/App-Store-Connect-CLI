package asc

import "testing"

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
