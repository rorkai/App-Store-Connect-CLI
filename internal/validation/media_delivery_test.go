package validation

import (
	"strings"
	"testing"
)

func TestMediaDeliveryChecksClassifiesEveryState(t *testing.T) {
	tests := []struct {
		state        string
		wantID       string
		wantSeverity Severity
		wantText     string
	}{
		{state: "COMPLETE"},
		{state: " complete "},
		{state: "FAILED", wantID: CheckMediaDeliveryFailed, wantSeverity: SeverityError, wantText: "failed processing"},
		{state: "AWAITING_UPLOAD", wantID: CheckMediaDeliveryPending, wantSeverity: SeverityWarning, wantText: "AWAITING_UPLOAD"},
		{state: "UPLOAD_COMPLETE", wantID: CheckMediaDeliveryPending, wantSeverity: SeverityWarning, wantText: "UPLOAD_COMPLETE"},
		{state: "PROCESSING", wantID: CheckMediaDeliveryPending, wantSeverity: SeverityWarning, wantText: "PROCESSING"},
		{state: "", wantID: CheckMediaDeliveryUnverified, wantSeverity: SeverityInfo, wantText: "state not returned"},
		{state: "SOMETHING_NEW", wantID: CheckMediaDeliveryUnverified, wantSeverity: SeverityInfo, wantText: "SOMETHING_NEW"},
	}

	for _, test := range tests {
		t.Run("state "+test.state, func(t *testing.T) {
			screenshots := []ScreenshotSet{{
				ID:          "set-1",
				DisplayType: "APP_IPHONE_67",
				Locale:      "fr-FR",
				Screenshots: []Screenshot{{ID: "shot-1", FileName: "one.png", DeliveryState: test.state}},
			}}
			previews := []PreviewSet{{
				ID:          "preview-set-1",
				PreviewType: "IPHONE_67",
				Locale:      "fr-FR",
				Previews:    []Preview{{ID: "preview-1", FileName: "one.mov", DeliveryState: test.state}},
			}}

			checks := mediaDeliveryChecks(screenshots, previews, nil, nil)
			if test.wantID == "" {
				if len(checks) != 0 {
					t.Fatalf("expected no findings, got %+v", checks)
				}
				return
			}
			if len(checks) != 2 {
				t.Fatalf("expected one finding per set, got %+v", checks)
			}
			for index, wantResource := range []struct{ kind, id string }{{"appScreenshotSet", "set-1"}, {"appPreviewSet", "preview-set-1"}} {
				check := checks[index]
				if check.ID != test.wantID || check.Severity != test.wantSeverity {
					t.Fatalf("check %d = %s/%s, want %s/%s", index, check.ID, check.Severity, test.wantID, test.wantSeverity)
				}
				if check.Locale != "fr-FR" || check.ResourceType != wantResource.kind || check.ResourceID != wantResource.id {
					t.Fatalf("check %d lost its locale or set: %+v", index, check)
				}
				if !strings.Contains(check.Message, test.wantText) || check.Remediation == "" {
					t.Fatalf("check %d = %+v, want message containing %q and a remediation", index, check, test.wantText)
				}
			}
		})
	}
}

func TestMediaDeliveryChecksGroupsAssetsAndKeepsAppleErrors(t *testing.T) {
	checks := mediaDeliveryChecks([]ScreenshotSet{{
		ID:          "set-1",
		DisplayType: "APP_IPHONE_67",
		Locale:      "en-US",
		Screenshots: []Screenshot{
			{ID: "shot-1", FileName: "one.png", DeliveryState: "FAILED", DeliveryErrors: []string{"IMAGE_INCORRECT_DIMENSIONS: wrong size"}},
			{ID: "shot-2", DeliveryState: "FAILED"},
			{ID: "shot-3", FileName: "three.png", DeliveryState: "COMPLETE"},
		},
	}}, nil, nil, nil)

	if len(checks) != 1 {
		t.Fatalf("expected one finding for the set, got %+v", checks)
	}
	for _, want := range []string{"2 of 3", "one.png (IMAGE_INCORRECT_DIMENSIONS: wrong size)", "shot-2"} {
		if !strings.Contains(checks[0].Message, want) {
			t.Fatalf("message %q does not contain %q", checks[0].Message, want)
		}
	}
	if strings.Contains(checks[0].Message, "three.png") {
		t.Fatalf("message %q names a delivered screenshot", checks[0].Message)
	}
}

func TestMediaDeliveryChecksReportPlacementStates(t *testing.T) {
	tests := []struct {
		state        string
		wantID       string
		wantSeverity Severity
	}{
		{state: "PARENT_PREPARE_FOR_SUBMISSION"},
		{state: "PARENT_APPROVED"},
		{state: "FAILED", wantID: CheckMediaDeliveryFailed, wantSeverity: SeverityError},
		{state: "ASSET_PROCESSING", wantID: CheckMediaDeliveryPending, wantSeverity: SeverityWarning},
		{state: "", wantID: CheckMediaDeliveryUnverified, wantSeverity: SeverityInfo},
		{state: "SOMETHING_NEW", wantID: CheckMediaDeliveryUnverified, wantSeverity: SeverityInfo},
	}

	for _, test := range tests {
		t.Run("state "+test.state, func(t *testing.T) {
			checks := mediaDeliveryChecks(nil, nil, []MediaPlacement{{
				ID:            "placement-1",
				Locale:        "de-DE",
				PlacementType: "APP_SCREENSHOT",
				State:         test.state,
			}}, nil)
			if test.wantID == "" {
				if len(checks) != 0 {
					t.Fatalf("expected no findings, got %+v", checks)
				}
				return
			}
			if len(checks) != 1 || checks[0].ID != test.wantID || checks[0].Severity != test.wantSeverity {
				t.Fatalf("checks = %+v, want %s/%s", checks, test.wantID, test.wantSeverity)
			}
			if checks[0].Locale != "de-DE" || checks[0].ResourceType != "appAssetLibraryPlacement" || checks[0].ResourceID != "placement-1" {
				t.Fatalf("check lost its locale or placement: %+v", checks[0])
			}
		})
	}
}

func TestMediaDeliveryChecksNeverPassAnUnreadSource(t *testing.T) {
	checks := mediaDeliveryChecks(nil, nil, nil, []MediaReadGap{{
		Media:          "asset library placements",
		Locale:         "ja",
		LocalizationID: "loc-1",
		Reason:         "this account cannot read them",
	}})

	if len(checks) != 1 {
		t.Fatalf("expected one finding, got %+v", checks)
	}
	check := checks[0]
	if check.ID != CheckMediaDeliveryUnverified || check.Severity != SeverityInfo {
		t.Fatalf("check = %s/%s, want an unverified info finding", check.ID, check.Severity)
	}
	if check.Locale != "ja" || check.ResourceID != "loc-1" || !strings.Contains(check.Message, "asset library placements") {
		t.Fatalf("check does not name what was unread: %+v", check)
	}
	if !strings.Contains(check.Remediation, "this account cannot read them") {
		t.Fatalf("remediation %q drops the reason", check.Remediation)
	}
}

func TestValidateBlocksOnFailedMediaAndStrictBlocksOnPendingMedia(t *testing.T) {
	input := Input{
		ScreenshotSets: []ScreenshotSet{{
			ID:          "set-1",
			DisplayType: "APP_IPHONE_67",
			Locale:      "en-US",
			Screenshots: []Screenshot{{ID: "shot-1", Width: 1290, Height: 2796, DeliveryState: "COMPLETE"}},
		}},
		PreviewSets: []PreviewSet{{
			ID:          "preview-set-1",
			PreviewType: "IPHONE_67",
			Locale:      "en-US",
			Previews:    []Preview{{ID: "preview-1", DeliveryState: "PROCESSING"}},
		}},
	}
	withPending := Validate(input, true)
	if !hasMediaCheck(withPending.Checks, CheckMediaDeliveryPending) {
		t.Fatalf("expected a pending finding, got %+v", withPending.Checks)
	}
	lenient := Validate(input, false)
	if lenient.Summary.Blocking >= withPending.Summary.Blocking {
		t.Fatalf("pending media should block only with --strict: lenient=%d strict=%d", lenient.Summary.Blocking, withPending.Summary.Blocking)
	}

	input.PreviewSets[0].Previews[0].DeliveryState = "FAILED"
	withFailed := Validate(input, false)
	if withFailed.Summary.Blocking != lenient.Summary.Blocking+1 {
		t.Fatalf("failed media should block without --strict: failed=%d baseline=%d", withFailed.Summary.Blocking, lenient.Summary.Blocking)
	}
}

func hasMediaCheck(checks []CheckResult, id string) bool {
	for _, check := range checks {
		if check.ID == id {
			return true
		}
	}
	return false
}
