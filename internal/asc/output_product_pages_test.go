package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_AppCustomProductPages(t *testing.T) {
	resp := &AppCustomProductPagesResponse{
		Data: []Resource[AppCustomProductPageAttributes]{
			{
				ID: "page-1",
				Attributes: AppCustomProductPageAttributes{
					Name:    "Summer Campaign",
					URL:     "https://example.com/page",
					Visible: ptrBool(true),
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Visible") || !strings.Contains(output, "Name") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Summer Campaign") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPages(t *testing.T) {
	resp := &AppCustomProductPagesResponse{
		Data: []Resource[AppCustomProductPageAttributes]{
			{
				ID: "page-1",
				Attributes: AppCustomProductPageAttributes{
					Name:    "Summer Campaign",
					URL:     "https://example.com/page",
					Visible: ptrBool(true),
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Visible") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Summer Campaign") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_AppCustomProductPages_Empty(t *testing.T) {
	resp := &AppCustomProductPagesResponse{Data: []Resource[AppCustomProductPageAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Visible") || !strings.Contains(output, "Name") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPages_Empty(t *testing.T) {
	resp := &AppCustomProductPagesResponse{Data: []Resource[AppCustomProductPageAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Visible") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppCustomProductPageVersions(t *testing.T) {
	resp := &AppCustomProductPageVersionsResponse{
		Data: []Resource[AppCustomProductPageVersionAttributes]{
			{
				ID: "version-1",
				Attributes: AppCustomProductPageVersionAttributes{
					Version:  "1.0",
					State:    "READY_FOR_REVIEW",
					DeepLink: "https://example.com/link",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version") || !strings.Contains(output, "State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "version-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPageVersions(t *testing.T) {
	resp := &AppCustomProductPageVersionsResponse{
		Data: []Resource[AppCustomProductPageVersionAttributes]{
			{
				ID: "version-1",
				Attributes: AppCustomProductPageVersionAttributes{
					Version:  "1.0",
					State:    "READY_FOR_REVIEW",
					DeepLink: "https://example.com/link",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deep Link") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "READY_FOR_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintTable_AppCustomProductPageVersions_Empty(t *testing.T) {
	resp := &AppCustomProductPageVersionsResponse{Data: []Resource[AppCustomProductPageVersionAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version") || !strings.Contains(output, "State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPageVersions_Empty(t *testing.T) {
	resp := &AppCustomProductPageVersionsResponse{Data: []Resource[AppCustomProductPageVersionAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deep Link") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppCustomProductPageLocalizations(t *testing.T) {
	resp := &AppCustomProductPageLocalizationsResponse{
		Data: []Resource[AppCustomProductPageLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppCustomProductPageLocalizationAttributes{
					Locale:          "en-US",
					PromotionalText: "Promo copy",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Locale") || !strings.Contains(output, "Promotional Text") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Promo copy") {
		t.Fatalf("expected promo text in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPageLocalizations(t *testing.T) {
	resp := &AppCustomProductPageLocalizationsResponse{
		Data: []Resource[AppCustomProductPageLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppCustomProductPageLocalizationAttributes{
					Locale:          "en-US",
					PromotionalText: "Promo copy",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Promotional Text") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_AppCustomProductPageLocalizations_Empty(t *testing.T) {
	resp := &AppCustomProductPageLocalizationsResponse{Data: []Resource[AppCustomProductPageLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Locale") || !strings.Contains(output, "Promotional Text") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPageLocalizations_Empty(t *testing.T) {
	resp := &AppCustomProductPageLocalizationsResponse{Data: []Resource[AppCustomProductPageLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Promotional Text") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppKeywords(t *testing.T) {
	resp := &AppKeywordsResponse{
		Data: []Resource[AppKeywordAttributes]{
			{
				ID: "keyword-1",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "keyword-1") {
		t.Fatalf("expected keyword in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppKeywords(t *testing.T) {
	resp := &AppKeywordsResponse{
		Data: []Resource[AppKeywordAttributes]{
			{
				ID: "keyword-1",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "keyword-1") {
		t.Fatalf("expected keyword in output, got: %s", output)
	}
}

func TestPrintTable_AppKeywords_Empty(t *testing.T) {
	resp := &AppKeywordsResponse{Data: []Resource[AppKeywordAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppKeywords_Empty(t *testing.T) {
	resp := &AppKeywordsResponse{Data: []Resource[AppKeywordAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppPreviewSets(t *testing.T) {
	resp := &AppPreviewSetsResponse{
		Data: []Resource[AppPreviewSetAttributes]{
			{
				ID: "preview-set-1",
				Attributes: AppPreviewSetAttributes{
					PreviewType: "IPHONE_65",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Preview Type") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "preview-set-1") {
		t.Fatalf("expected set ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppPreviewSets(t *testing.T) {
	resp := &AppPreviewSetsResponse{
		Data: []Resource[AppPreviewSetAttributes]{
			{
				ID: "preview-set-1",
				Attributes: AppPreviewSetAttributes{
					PreviewType: "IPHONE_65",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Preview Type") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "IPHONE_65") {
		t.Fatalf("expected preview type in output, got: %s", output)
	}
}

func TestPrintTable_AppPreviewSets_Empty(t *testing.T) {
	resp := &AppPreviewSetsResponse{Data: []Resource[AppPreviewSetAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Preview Type") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppPreviewSets_Empty(t *testing.T) {
	resp := &AppPreviewSetsResponse{Data: []Resource[AppPreviewSetAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Preview Type") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppScreenshotSets(t *testing.T) {
	resp := &AppScreenshotSetsResponse{
		Data: []Resource[AppScreenshotSetAttributes]{
			{
				ID: "screenshot-set-1",
				Attributes: AppScreenshotSetAttributes{
					ScreenshotDisplayType: "APP_IPHONE_65",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Display Type") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "screenshot-set-1") {
		t.Fatalf("expected set ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppScreenshotSets(t *testing.T) {
	resp := &AppScreenshotSetsResponse{
		Data: []Resource[AppScreenshotSetAttributes]{
			{
				ID: "screenshot-set-1",
				Attributes: AppScreenshotSetAttributes{
					ScreenshotDisplayType: "APP_IPHONE_65",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Display Type") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "APP_IPHONE_65") {
		t.Fatalf("expected display type in output, got: %s", output)
	}
}

func TestPrintTable_AppScreenshotSets_Empty(t *testing.T) {
	resp := &AppScreenshotSetsResponse{Data: []Resource[AppScreenshotSetAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Display Type") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppScreenshotSets_Empty(t *testing.T) {
	resp := &AppScreenshotSetsResponse{Data: []Resource[AppScreenshotSetAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Display Type") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperiments(t *testing.T) {
	resp := &AppStoreVersionExperimentsResponse{
		Data: []Resource[AppStoreVersionExperimentAttributes]{
			{
				ID: "exp-1",
				Attributes: AppStoreVersionExperimentAttributes{
					Name:              "Icon Test",
					TrafficProportion: ptrInt(25),
					State:             "IN_REVIEW",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Traffic") || !strings.Contains(output, "State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Icon Test") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperiments(t *testing.T) {
	resp := &AppStoreVersionExperimentsResponse{
		Data: []Resource[AppStoreVersionExperimentAttributes]{
			{
				ID: "exp-1",
				Attributes: AppStoreVersionExperimentAttributes{
					Name:              "Icon Test",
					TrafficProportion: ptrInt(25),
					State:             "IN_REVIEW",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Traffic Proportion") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "IN_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperiments_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentsResponse{Data: []Resource[AppStoreVersionExperimentAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Traffic") || !strings.Contains(output, "State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperiments_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentsResponse{Data: []Resource[AppStoreVersionExperimentAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Traffic Proportion") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperimentsV2(t *testing.T) {
	resp := &AppStoreVersionExperimentsV2Response{
		Data: []Resource[AppStoreVersionExperimentV2Attributes]{
			{
				ID: "exp-2",
				Attributes: AppStoreVersionExperimentV2Attributes{
					Name:              "Icon Test V2",
					Platform:          PlatformIOS,
					TrafficProportion: ptrInt(40),
					State:             "PREPARE_FOR_SUBMISSION",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Platform") || !strings.Contains(output, "Traffic") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "IOS") {
		t.Fatalf("expected platform in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperimentsV2(t *testing.T) {
	resp := &AppStoreVersionExperimentsV2Response{
		Data: []Resource[AppStoreVersionExperimentV2Attributes]{
			{
				ID: "exp-2",
				Attributes: AppStoreVersionExperimentV2Attributes{
					Name:              "Icon Test V2",
					Platform:          PlatformIOS,
					TrafficProportion: ptrInt(40),
					State:             "PREPARE_FOR_SUBMISSION",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Platform") || !strings.Contains(output, "Traffic Proportion") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Icon Test V2") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperimentsV2_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentsV2Response{Data: []Resource[AppStoreVersionExperimentV2Attributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Platform") || !strings.Contains(output, "Traffic") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperimentsV2_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentsV2Response{Data: []Resource[AppStoreVersionExperimentV2Attributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Platform") || !strings.Contains(output, "Traffic Proportion") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperimentTreatments(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentsResponse{
		Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{
			{
				ID: "treat-1",
				Attributes: AppStoreVersionExperimentTreatmentAttributes{
					Name:         "Variant A",
					AppIconName:  "Icon A",
					PromotedDate: "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "App Icon Name") || !strings.Contains(output, "Promoted Date") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Variant A") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperimentTreatments(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentsResponse{
		Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{
			{
				ID: "treat-1",
				Attributes: AppStoreVersionExperimentTreatmentAttributes{
					Name:         "Variant A",
					AppIconName:  "Icon A",
					PromotedDate: "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "App Icon Name") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Variant A") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperimentTreatments_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "App Icon Name") || !strings.Contains(output, "Promoted Date") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperimentTreatments_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "App Icon Name") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperimentTreatmentLocalizations(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentLocalizationsResponse{
		Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{
			{
				ID: "tloc-1",
				Attributes: AppStoreVersionExperimentTreatmentLocalizationAttributes{
					Locale: "fr-FR",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Locale") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "fr-FR") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperimentTreatmentLocalizations(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentLocalizationsResponse{
		Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{
			{
				ID: "tloc-1",
				Attributes: AppStoreVersionExperimentTreatmentLocalizationAttributes{
					Locale: "fr-FR",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Locale") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "fr-FR") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionExperimentTreatmentLocalizations_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentLocalizationsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Locale") {
		t.Fatalf("expected header in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionExperimentTreatmentLocalizations_Empty(t *testing.T) {
	resp := &AppStoreVersionExperimentTreatmentLocalizationsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Locale") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppCustomProductPageDeleteResult(t *testing.T) {
	result := &AppCustomProductPageDeleteResult{ID: "page-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "page-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppCustomProductPageDeleteResult(t *testing.T) {
	result := &AppCustomProductPageDeleteResult{ID: "page-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deleted") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "page-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func ptrInt(value int) *int {
	return &value
}
