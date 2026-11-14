package asc

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestPrintTable_AppStoreVersionPhasedReleaseIncludesProgress(t *testing.T) {
	resp := &AppStoreVersionPhasedReleaseResponse{
		Data: &Resource[AppStoreVersionPhasedReleaseAttributes]{
			ID: "phase-1",
			Attributes: AppStoreVersionPhasedReleaseAttributes{
				PhasedReleaseState: PhasedReleaseStateActive,
				StartDate:          "2026-02-20",
				CurrentDayNumber:   3,
				TotalPauseDuration: 0,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Progress") {
		t.Fatalf("expected progress header in output, got: %s", output)
	}
	if !strings.Contains(output, "[####------] 3/7") {
		t.Fatalf("expected progress bar in output, got: %s", output)
	}
}

func TestDisplayPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "ios", raw: "IOS", want: "iOS"},
		{name: "macos", raw: "MAC_OS", want: "macOS"},
		{name: "tvos", raw: "TV_OS", want: "tvOS"},
		{name: "visionos", raw: "VISION_OS", want: "visionOS"},
		{name: "unknown passthrough", raw: "CAR_OS", want: "CAR_OS"},
		{name: "empty passthrough", raw: "", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := displayPlatform(tt.raw); got != tt.want {
				t.Fatalf("displayPlatform(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAppStoreVersionsRows_UsesDisplayPlatform(t *testing.T) {
	t.Parallel()

	resp := &AppStoreVersionsResponse{
		Data: []Resource[AppStoreVersionAttributes]{
			{
				ID: "v1",
				Attributes: AppStoreVersionAttributes{
					VersionString: "1.0",
					Platform:      PlatformMacOS,
					AppStoreState: "READY_FOR_SALE",
					CreatedDate:   "2026-03-30T00:00:00Z",
				},
			},
			{
				ID: "v2",
				Attributes: AppStoreVersionAttributes{
					VersionString: "2.0",
					Platform:      Platform("CAR_OS"),
					AppStoreState: "PREPARE_FOR_SUBMISSION",
					CreatedDate:   "2026-03-30T00:00:01Z",
				},
			},
		},
	}

	_, rows := appStoreVersionsRows(resp)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][2] != "macOS" {
		t.Fatalf("expected mapped platform macOS, got %q", rows[0][2])
	}
	if rows[1][2] != "CAR_OS" {
		t.Fatalf("expected unknown platform passthrough CAR_OS, got %q", rows[1][2])
	}
}

func TestAppStoreVersionsRows_ShowsIncludedBuildAndSubmission(t *testing.T) {
	t.Parallel()

	version := func(id, relationships string) Resource[AppStoreVersionAttributes] {
		return Resource[AppStoreVersionAttributes]{
			ID:            id,
			Attributes:    AppStoreVersionAttributes{VersionString: "2.5.3", Platform: PlatformIOS},
			Relationships: json.RawMessage(relationships),
		}
	}
	tests := []struct {
		name        string
		resp        *AppStoreVersionsResponse
		wantHeaders []string
		wantExtra   [][]string
	}{
		{
			name: "build and submission included",
			resp: &AppStoreVersionsResponse{
				Data: []Resource[AppStoreVersionAttributes]{
					version("v1", `{"build":{"data":{"type":"builds","id":"b1"}},"appStoreVersionSubmission":{"data":{"type":"appStoreVersionSubmissions","id":"s1"}}}`),
					version("v2", `{"build":{"data":null},"appStoreVersionSubmission":{"data":null}}`),
				},
				Included: json.RawMessage(`[{"type":"appStoreVersionSubmissions","id":"s1","attributes":{}},{"type":"builds","id":"b1","attributes":{"version":"12"}}]`),
			},
			wantHeaders: []string{"Build ID", "Build Version", "Submission ID"},
			wantExtra:   [][]string{{"b1", "12", "s1"}, {"", "", ""}},
		},
		{
			name: "only build included",
			resp: &AppStoreVersionsResponse{
				Data:     []Resource[AppStoreVersionAttributes]{version("v1", `{"build":{"data":{"type":"builds","id":"b1"}},"appStoreVersionSubmission":{"links":{"self":"x"}}}`)},
				Included: json.RawMessage(`[{"type":"builds","id":"b1","attributes":{"version":"12"}}]`),
			},
			wantHeaders: []string{"Build ID", "Build Version"},
			wantExtra:   [][]string{{"b1", "12"}},
		},
		{
			name:      "nothing included",
			resp:      &AppStoreVersionsResponse{Data: []Resource[AppStoreVersionAttributes]{version("v1", `{"build":{"links":{"self":"x"}}}`)}},
			wantExtra: [][]string{nil},
		},
	}

	base := []string{"ID", "Version", "Platform", "State", "Created"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			headers, rows := appStoreVersionsRows(tt.resp)
			if want := append(slices.Clone(base), tt.wantHeaders...); !slices.Equal(headers, want) {
				t.Fatalf("headers = %v, want %v", headers, want)
			}
			for i, row := range rows {
				if got := row[len(base):]; !slices.Equal(got, tt.wantExtra[i]) {
					t.Fatalf("row %d extra columns = %v, want %v", i, got, tt.wantExtra[i])
				}
			}
		})
	}
}

func TestAppStoreVersionsLatestRows_ShowsIncludedBuild(t *testing.T) {
	t.Parallel()

	headers, rows := appStoreVersionsLatestRows(&AppStoreVersionsLatestResult{
		Items: []Resource[AppStoreVersionAttributes]{{
			ID:            "v1",
			Relationships: json.RawMessage(`{"build":{"data":{"type":"builds","id":"b1"}}}`),
		}},
		Included: json.RawMessage(`[{"type":"builds","id":"b1","attributes":{"version":"12"}}]`),
	})
	if !slices.Contains(headers, "Build Version") || !slices.Contains(rows[0], "12") {
		t.Fatalf("expected included build in latest rows, got headers %v rows %v", headers, rows)
	}
}

func TestPrintTable_AppStoreVersionsLatestResult(t *testing.T) {
	result := &AppStoreVersionsLatestResult{
		Items: []Resource[AppStoreVersionAttributes]{
			{
				ID: "v1",
				Attributes: AppStoreVersionAttributes{
					VersionString: "2.4.1",
					Platform:      PlatformIOS,
					AppStoreState: "READY_FOR_SALE",
					CreatedDate:   "2026-02-20T00:30:00Z",
				},
			},
		},
		TotalCount: 1,
		HasMore:    false,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})
	if !strings.Contains(output, "2.4.1") || !strings.Contains(output, "iOS") {
		t.Fatalf("expected latest-version table row, got %q", output)
	}
}

func TestPreReleaseVersionsRows_UsesDisplayPlatform(t *testing.T) {
	t.Parallel()

	resp := &PreReleaseVersionsResponse{
		Data: []PreReleaseVersion{
			{
				ID: "prv-1",
				Attributes: PrereleaseVersionAttributes{
					Version:  "1.0",
					Platform: PlatformTVOS,
				},
			},
			{
				ID: "prv-2",
				Attributes: PrereleaseVersionAttributes{
					Version:  "2.0",
					Platform: Platform("CAR_OS"),
				},
			},
		},
	}

	_, rows := preReleaseVersionsRows(resp)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][2] != "tvOS" {
		t.Fatalf("expected mapped platform tvOS, got %q", rows[0][2])
	}
	if rows[1][2] != "CAR_OS" {
		t.Fatalf("expected unknown platform passthrough CAR_OS, got %q", rows[1][2])
	}
}

func TestSubmissionAndVersionDetailRows_UseDisplayPlatform(t *testing.T) {
	t.Parallel()

	statusHeaders, statusRows := appStoreVersionSubmissionStatusRows(&AppStoreVersionSubmissionStatusResult{
		ID:            "sub-1",
		VersionID:     "v1",
		VersionString: "1.0",
		Platform:      "VISION_OS",
		State:         "WAITING_FOR_REVIEW",
	})
	if len(statusHeaders) == 0 || len(statusRows) != 1 {
		t.Fatalf("unexpected status rows output: headers=%v rows=%v", statusHeaders, statusRows)
	}
	if statusRows[0][3] != "visionOS" {
		t.Fatalf("expected mapped status platform visionOS, got %q", statusRows[0][3])
	}

	detailHeaders, detailRows := appStoreVersionDetailRows(&AppStoreVersionDetailResult{
		ID:            "v2",
		VersionString: "2.0",
		Platform:      "CAR_OS",
		State:         "DEVELOPER_REJECTED",
	})
	if len(detailHeaders) == 0 || len(detailRows) != 1 {
		t.Fatalf("unexpected detail rows output: headers=%v rows=%v", detailHeaders, detailRows)
	}
	if detailRows[0][2] != "CAR_OS" {
		t.Fatalf("expected unknown detail platform passthrough CAR_OS, got %q", detailRows[0][2])
	}
}

func TestAppStoreVersionDetailRows_IdempotentWriteReceipt(t *testing.T) {
	t.Parallel()

	headers, rows := appStoreVersionDetailRows(&AppStoreVersionDetailResult{
		ID:            "v2",
		VersionString: "2.0",
		IdempotentWriteReceipt: IdempotentWriteReceipt{
			AlreadyExists: true,
			Action:        IdempotentWriteActionSkipped,
		},
	})

	if len(rows) != 1 {
		t.Fatalf("rows = %v, want one row", rows)
	}
	if got, want := headers[len(headers)-2:], []string{"Already Exists", "Action"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt headers = %v, want %v", got, want)
	}
	if got, want := rows[0][len(rows[0])-2:], []string{"true", IdempotentWriteActionSkipped}; !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt cells = %v, want %v", got, want)
	}

	legacyHeaders, legacyRows := appStoreVersionDetailRows(&AppStoreVersionDetailResult{ID: "v3"})
	if slices.Contains(legacyHeaders, "Action") || slices.Contains(legacyHeaders, "Already Exists") {
		t.Fatalf("legacy fail-mode headers = %v, want no receipt columns", legacyHeaders)
	}
	if len(legacyRows) != 1 || len(legacyRows[0]) != len(legacyHeaders) {
		t.Fatalf("legacy rows = %v headers = %v, want aligned unchanged output", legacyRows, legacyHeaders)
	}
}

func TestPrintTable_AppStoreVersionRatingResetCreateResult(t *testing.T) {
	result := &AppStoreVersionRatingResetCreateResult{
		RatingResetRequestID: "reset-123",
		VersionID:            "version-123",
		Scheduled:            true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})
	for _, want := range []string{"Rating Reset Request ID", "Version ID", "Scheduled", "reset-123", "version-123", "true"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestPrintMarkdown_AppStoreVersionRatingResetDeleteResult(t *testing.T) {
	result := &AppStoreVersionRatingResetDeleteResult{
		RatingResetRequestID: "reset-123",
		Cancelled:            true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})
	for _, want := range []string{"Rating Reset Request ID", "Cancelled", "reset-123", "true"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}
