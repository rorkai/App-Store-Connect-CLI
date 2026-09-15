package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

// hostileText carries an OSC window-title sequence, a C1 CSI, a DEL, and a bidi
// override, all of which a terminal or CI log viewer would interpret.
const hostileText = "shot\x1b]0;pwned\x07\u009b2K\u007f\u202egpj.png"

func assertRowsAreInert(t *testing.T, headers []string, rows [][]string) {
	t.Helper()

	for _, header := range headers {
		if HasInterpretedTerminalSequence(header) {
			t.Fatalf("header %q contains interpreted terminal sequences", header)
		}
	}
	for i, row := range rows {
		for j, cell := range row {
			if HasInterpretedTerminalSequence(cell) {
				t.Fatalf("row %d column %d = %q contains interpreted terminal sequences", i, j, cell)
			}
			if strings.Contains(cell, "\x1b") || strings.Contains(cell, "\u202e") {
				t.Fatalf("row %d column %d = %q retained a control character", i, j, cell)
			}
		}
	}
}

func TestAppScreenshotRowsRemoveTerminalControls(t *testing.T) {
	resp := &AppScreenshotsResponse{
		Data: []Resource[AppScreenshotAttributes]{{
			ID: "SHOT\x1b[31mID",
			Attributes: AppScreenshotAttributes{
				FileName:           hostileText,
				FileSize:           1024,
				AssetDeliveryState: &AssetDeliveryState{State: "COMPLETE\u202e"},
			},
		}},
	}

	headers, rows := appScreenshotsRows(resp)
	assertRowsAreInert(t, headers, rows)

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][1] != SanitizeTerminalText(hostileText) {
		t.Fatalf("file name = %q, want %q", rows[0][1], SanitizeTerminalText(hostileText))
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded AppScreenshotsResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if decoded.Data[0].Attributes.FileName != hostileText {
		t.Fatalf("JSON file name = %q, want the original %q", decoded.Data[0].Attributes.FileName, hostileText)
	}
}

func TestAppPreviewRowsRemoveTerminalControls(t *testing.T) {
	resp := &AppPreviewsResponse{
		Data: []Resource[AppPreviewAttributes]{{
			ID: "PREVIEW\x1bID",
			Attributes: AppPreviewAttributes{
				FileName:             hostileText,
				PreviewFrameTimeCode: "00:00:03\u009b2K",
				AssetDeliveryState:   &AssetDeliveryState{State: "FAILED\x1b[0m"},
			},
		}},
	}

	headers, rows := appPreviewsRows(resp)
	assertRowsAreInert(t, headers, rows)
}

func TestAppScreenshotSetRowsRemoveTerminalControls(t *testing.T) {
	headers, rows := appScreenshotSetsRows(&AppScreenshotSetsResponse{
		Data: []Resource[AppScreenshotSetAttributes]{{
			ID:         "SET\x1bID",
			Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_67\u202e"},
		}},
	})
	assertRowsAreInert(t, headers, rows)

	headers, rows = appPreviewSetsRows(&AppPreviewSetsResponse{
		Data: []Resource[AppPreviewSetAttributes]{{
			ID:         "SET\x1bID",
			Attributes: AppPreviewSetAttributes{PreviewType: "IPHONE_67\u009b2K"},
		}},
	})
	assertRowsAreInert(t, headers, rows)
}

func TestAssetListResultRowsRemoveTerminalControls(t *testing.T) {
	screenshotHeaders, screenshotRows := appScreenshotListResultRows(&AppScreenshotListResult{
		VersionLocalizationID: "LOC\x1bID",
		Sets: []AppScreenshotSetWithScreenshots{{
			Set: Resource[AppScreenshotSetAttributes]{
				ID:         "SET\x1bID",
				Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_67\u202e"},
			},
			Screenshots: []Resource[AppScreenshotAttributes]{{
				ID: "SHOT\x1bID",
				Attributes: AppScreenshotAttributes{
					FileName:           hostileText,
					AssetDeliveryState: &AssetDeliveryState{State: "COMPLETE\x07"},
				},
			}},
		}},
	})
	assertRowsAreInert(t, screenshotHeaders, screenshotRows)

	previewHeaders, previewRows := appPreviewListResultRows(&AppPreviewListResult{
		VersionLocalizationID: "LOC\x1bID",
		Sets: []AppPreviewSetWithPreviews{{
			Set: Resource[AppPreviewSetAttributes]{
				ID:         "SET\x1bID",
				Attributes: AppPreviewSetAttributes{PreviewType: "IPHONE_67\u202e"},
			},
			Previews: []Resource[AppPreviewAttributes]{{
				ID: "PREVIEW\x1bID",
				Attributes: AppPreviewAttributes{
					FileName:             hostileText,
					PreviewFrameTimeCode: "00:00:03\x1b[0m",
					AssetDeliveryState:   &AssetDeliveryState{State: "COMPLETE\x07"},
				},
			}},
		}},
	})
	assertRowsAreInert(t, previewHeaders, previewRows)

	customPageHeaders, customPageRows := appScreenshotSetListResultRows(&AppScreenshotSetListResult{
		LocalizationID: "CUSTOM_LOC\x1bID",
		Sets: []AppScreenshotSetWithScreenshots{{
			Set: Resource[AppScreenshotSetAttributes]{
				ID:         "SET\x1bID",
				Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_67\u202e"},
			},
			Screenshots: []Resource[AppScreenshotAttributes]{{
				ID: "SHOT\x1bID",
				Attributes: AppScreenshotAttributes{
					FileName:           hostileText,
					AssetDeliveryState: &AssetDeliveryState{State: "COMPLETE\x07"},
				},
			}},
		}},
	})
	assertRowsAreInert(t, customPageHeaders, customPageRows)
	if err := renderByRegistry(&AppScreenshotSetListResult{}, func(headers []string, rows [][]string) {
		if len(headers) == 0 || len(rows) != 0 {
			t.Fatalf("registered screenshot-set list renderer returned headers=%v rows=%v", headers, rows)
		}
	}); err != nil {
		t.Fatalf("renderByRegistry(AppScreenshotSetListResult) error: %v", err)
	}
}

func TestAppScreenshotSetListResultJSONOutputPreservesNestedScreenshotIDs(t *testing.T) {
	result := &AppScreenshotSetListResult{
		LocalizationID: "custom-localization-1",
		Sets: []AppScreenshotSetWithScreenshots{{
			Set: Resource[AppScreenshotSetAttributes]{
				ID:         "set-1",
				Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_65"},
			},
			Screenshots: []Resource[AppScreenshotAttributes]{{
				ID: "screenshot-1",
				Attributes: AppScreenshotAttributes{
					FileName: "01-home.png",
				},
			}},
		}},
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded AppScreenshotSetListResult
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if decoded.LocalizationID != result.LocalizationID {
		t.Fatalf("localizationId = %q, want %q", decoded.LocalizationID, result.LocalizationID)
	}
	if len(decoded.Sets) != 1 || len(decoded.Sets[0].Screenshots) != 1 || decoded.Sets[0].Screenshots[0].ID != "screenshot-1" {
		t.Fatalf("nested screenshot output = %#v", decoded.Sets)
	}
}

func TestAssetUploadRowsRemoveTerminalControlsAndPreserveJSON(t *testing.T) {
	result := &AppScreenshotUploadResult{
		VersionLocalizationID: "LOC\x1bID",
		SetID:                 "SET\x1bID",
		DisplayType:           "APP_IPHONE_67\u202e",
		Total:                 1,
		FailureArtifactPath:   "./artifacts/failures\x1b[0m.json",
		Results: []AssetUploadResultItem{{
			FileName: hostileText,
			FilePath: "./shots/" + hostileText,
			AssetID:  "ASSET\x1bID",
			State:    "UPLOAD_COMPLETE\u009b2K",
		}},
		Failures: []AssetUploadFailureItem{{
			FileName: hostileText,
			FilePath: "./shots/" + hostileText,
			Error:    "upload failed\x1b]0;pwned\x07",
		}},
	}

	mainHeaders, mainRows := appScreenshotUploadResultMainRows(result)
	assertRowsAreInert(t, mainHeaders, mainRows)

	itemHeaders, itemRows := assetUploadResultItemRows(result.Results)
	assertRowsAreInert(t, itemHeaders, itemRows)

	failureHeaders, failureRows := assetUploadFailureItemRows(result.Failures)
	assertRowsAreInert(t, failureHeaders, failureRows)

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded AppScreenshotUploadResult
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if decoded.Results[0].FileName != hostileText {
		t.Fatalf("JSON result file name = %q, want the original", decoded.Results[0].FileName)
	}
	if decoded.Failures[0].Error != "upload failed\x1b]0;pwned\x07" {
		t.Fatalf("JSON failure error = %q, want the original", decoded.Failures[0].Error)
	}
}

func TestAssetFanoutUploadRowsRemoveTerminalControls(t *testing.T) {
	result := &AppScreenshotFanoutUploadResult{
		AppID:       "123\x1b456",
		Version:     "1.0\u202e",
		VersionID:   "VER\x1bID",
		Platform:    "IOS\x07",
		DisplayType: "APP_IPHONE_67\u009b2K",
		Localizations: []AppScreenshotLocalizationUploadResult{{
			Locale:                "en-US\u202e",
			VersionLocalizationID: "LOC\x1bID",
			SetID:                 "SET\x1bID",
			FailureArtifactPath:   "./failures\x1b[0m.json",
			Results: []AssetUploadResultItem{{
				FileName: hostileText,
				AssetID:  "ASSET\x1bID",
				State:    "UPLOAD_COMPLETE\x1b[0m",
			}},
			Failures: []AssetUploadFailureItem{{
				FileName: hostileText,
				FilePath: "./shots/" + hostileText,
				Error:    "upload failed\u009b2K",
			}},
		}},
	}

	mainHeaders, mainRows := appScreenshotFanoutUploadResultMainRows(result)
	assertRowsAreInert(t, mainHeaders, mainRows)

	localizationHeaders, localizationRows := appScreenshotFanoutUploadLocalizationRows(result)
	assertRowsAreInert(t, localizationHeaders, localizationRows)

	itemHeaders, itemRows := appScreenshotFanoutUploadResultItemRows(result)
	assertRowsAreInert(t, itemHeaders, itemRows)

	failureHeaders, failureRows := appScreenshotFanoutUploadFailureRows(result)
	assertRowsAreInert(t, failureHeaders, failureRows)
}

func TestAssetUploadStateSummaryRemovesTerminalControls(t *testing.T) {
	summary := summarizeAssetUploadStates([]AssetUploadResultItem{
		{FileName: "a.png", State: "UPLOAD_COMPLETE\x1b[0m"},
		{FileName: "b.png", State: "UPLOAD_COMPLETE\x1b[0m"},
		{FileName: "c.png", Skipped: true},
	})
	if HasInterpretedTerminalSequence(summary) {
		t.Fatalf("state summary %q contains interpreted terminal sequences", summary)
	}
	if !strings.Contains(summary, "UPLOAD_COMPLETE[0m=2") {
		t.Fatalf("state summary = %q, want the sanitized state count", summary)
	}
}

func TestAssetPreviewAndDeleteRowsRemoveTerminalControls(t *testing.T) {
	headers, rows := appPreviewUploadResultMainRows(&AppPreviewUploadResult{
		VersionLocalizationID: "LOC\x1bID",
		SetID:                 "SET\x1bID",
		PreviewType:           "IPHONE_67\u202e",
	})
	assertRowsAreInert(t, headers, rows)

	headers, rows = customProductPageScreenshotUploadResultMainRows(&CustomProductPageScreenshotUploadResult{
		CustomProductPageLocalizationID: "LOC\x1bID",
		SetID:                           "SET\x1bID",
		DisplayType:                     "APP_IPHONE_67\u202e",
	})
	assertRowsAreInert(t, headers, rows)

	headers, rows = experimentTreatmentLocalizationScreenshotUploadResultMainRows(&ExperimentTreatmentLocalizationScreenshotUploadResult{
		ExperimentTreatmentLocalizationID: "LOC\x1bID",
		SetID:                             "SET\x1bID",
		DisplayType:                       "APP_IPHONE_67\u202e",
	})
	assertRowsAreInert(t, headers, rows)

	headers, rows = customProductPagePreviewUploadResultMainRows(&CustomProductPagePreviewUploadResult{
		CustomProductPageLocalizationID: "LOC\x1bID",
		SetID:                           "SET\x1bID",
		PreviewType:                     "IPHONE_67\u202e",
	})
	assertRowsAreInert(t, headers, rows)

	headers, rows = assetDeleteResultRows(&AssetDeleteResult{ID: "ASSET\x1bID", Deleted: true})
	assertRowsAreInert(t, headers, rows)
}

func TestAppScreenshotListResultRowsAddLocaleColumnForAllLocalizationResults(t *testing.T) {
	headers, rows := appScreenshotListResultRows(&AppScreenshotListResult{
		Localizations: []AppScreenshotLocalizationListResult{
			{
				Locale:                "de-DE",
				VersionLocalizationID: "loc-de",
			},
			{
				Locale:                "en-US",
				VersionLocalizationID: "loc-en",
				Sets: []AppScreenshotSetWithScreenshots{{
					Set: Resource[AppScreenshotSetAttributes]{
						ID:         "set-en",
						Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_65"},
					},
					Screenshots: []Resource[AppScreenshotAttributes]{{
						ID:         "shot-en",
						Attributes: AppScreenshotAttributes{FileName: "home.png", FileSize: 42},
					}},
				}},
			},
		},
	})

	wantHeaders := []string{"Locale", "Set ID", "Display Type", "Screenshot ID", "File Name", "File Size", "State"}
	if len(headers) != len(wantHeaders) {
		t.Fatalf("headers = %#v, want %#v", headers, wantHeaders)
	}
	for i, want := range wantHeaders {
		if headers[i] != want {
			t.Fatalf("headers = %#v, want %#v", headers, wantHeaders)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want 2 rows", rows)
	}
	if rows[0][0] != "de-DE" || rows[0][1] != "" {
		t.Fatalf("first row = %#v, want empty de-DE placeholder row", rows[0])
	}
	if rows[1][0] != "en-US" || rows[1][1] != "set-en" || rows[1][3] != "shot-en" || rows[1][4] != "home.png" {
		t.Fatalf("second row = %#v, want en-US screenshot row", rows[1])
	}
}

func TestAppScreenshotListResultLocaleRowsRemoveTerminalControls(t *testing.T) {
	headers, rows := appScreenshotListResultRows(&AppScreenshotListResult{
		Localizations: []AppScreenshotLocalizationListResult{{
			Locale:                "en\x1bUS",
			VersionLocalizationID: "LOC\x1bID",
			Sets: []AppScreenshotSetWithScreenshots{{
				Set: Resource[AppScreenshotSetAttributes]{
					ID:         "SET\x1bID",
					Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_67\u202e"},
				},
				Screenshots: []Resource[AppScreenshotAttributes]{{
					ID: "SHOT\x1bID",
					Attributes: AppScreenshotAttributes{
						FileName:           hostileText,
						AssetDeliveryState: &AssetDeliveryState{State: "COMPLETE\x07"},
					},
				}},
			}},
		}},
	})
	assertRowsAreInert(t, headers, rows)
}
