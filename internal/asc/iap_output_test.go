package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintTable_InAppPurchaseImages(t *testing.T) {
	resp := &InAppPurchaseImagesResponse{
		Data: []Resource[InAppPurchaseImageAttributes]{
			{
				ID: "img-1",
				Attributes: InAppPurchaseImageAttributes{
					FileName: "image.png",
					FileSize: 123,
					State:    "UPLOAD_COMPLETE",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") || !strings.Contains(output, "State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "image.png") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintMarkdown_InAppPurchaseImages(t *testing.T) {
	resp := &InAppPurchaseImagesResponse{
		Data: []Resource[InAppPurchaseImageAttributes]{
			{
				ID: "img-1",
				Attributes: InAppPurchaseImageAttributes{
					FileName: "image.png",
					FileSize: 123,
					State:    "UPLOAD_COMPLETE",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "File Name") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "UPLOAD_COMPLETE") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintTable_InAppPurchaseLocalization(t *testing.T) {
	resp := &InAppPurchaseLocalizationResponse{
		Data: Resource[InAppPurchaseLocalizationAttributes]{
			ID: "loc-1",
			Attributes: InAppPurchaseLocalizationAttributes{
				Locale:      "en-US",
				Name:        "Coins",
				Description: "Premium coins",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Locale") || !strings.Contains(output, "Name") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Coins") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_InAppPurchaseLocalization(t *testing.T) {
	resp := &InAppPurchaseLocalizationResponse{
		Data: Resource[InAppPurchaseLocalizationAttributes]{
			ID: "loc-1",
			Attributes: InAppPurchaseLocalizationAttributes{
				Locale:      "en-US",
				Name:        "Coins",
				Description: "Premium coins",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Locale") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Premium coins") {
		t.Fatalf("expected description in output, got: %s", output)
	}
}

func TestPrintTable_InAppPurchasePricePoints(t *testing.T) {
	resp := &InAppPurchasePricePointsResponse{
		Data: []Resource[InAppPurchasePricePointAttributes]{
			{
				ID: "price-1",
				Attributes: InAppPurchasePricePointAttributes{
					CustomerPrice: "1.99",
					Proceeds:      "1.40",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Customer Price") || !strings.Contains(output, "Proceeds") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "price-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_InAppPurchasePricePoints(t *testing.T) {
	resp := &InAppPurchasePricePointsResponse{
		Data: []Resource[InAppPurchasePricePointAttributes]{
			{
				ID: "price-1",
				Attributes: InAppPurchasePricePointAttributes{
					CustomerPrice: "1.99",
					Proceeds:      "1.40",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Customer Price") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "1.40") {
		t.Fatalf("expected proceeds in output, got: %s", output)
	}
}

func TestPrintTable_InAppPurchasePrices(t *testing.T) {
	relationships := json.RawMessage(`{"territory":{"data":{"type":"territories","id":"USA"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"PRICE_POINT_1"}}}`)
	resp := &InAppPurchasePricesResponse{
		Data: []Resource[InAppPurchasePriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
				Attributes: InAppPurchasePriceAttributes{
					StartDate: "2026-01-01",
					EndDate:   "2026-02-01",
					Manual:    true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Territory") || !strings.Contains(output, "Price Point") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "USA") {
		t.Fatalf("expected territory in output, got: %s", output)
	}
}

func TestPrintMarkdown_InAppPurchasePrices(t *testing.T) {
	relationships := json.RawMessage(`{"territory":{"data":{"type":"territories","id":"USA"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"PRICE_POINT_1"}}}`)
	resp := &InAppPurchasePricesResponse{
		Data: []Resource[InAppPurchasePriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
				Attributes: InAppPurchasePriceAttributes{
					StartDate: "2026-01-01",
					EndDate:   "2026-02-01",
					Manual:    true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Territory") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "PRICE_POINT_1") {
		t.Fatalf("expected price point in output, got: %s", output)
	}
}

func TestPrintTable_InAppPurchaseOfferCodes(t *testing.T) {
	resp := &InAppPurchaseOfferCodesResponse{
		Data: []Resource[InAppPurchaseOfferCodeAttributes]{
			{
				ID: "code-1",
				Attributes: InAppPurchaseOfferCodeAttributes{
					Name:                "SPRING",
					Active:              true,
					ProductionCodeCount: 10,
					SandboxCodeCount:    2,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Prod Codes") || !strings.Contains(output, "Sandbox Codes") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "SPRING") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_InAppPurchaseOfferCodes(t *testing.T) {
	resp := &InAppPurchaseOfferCodesResponse{
		Data: []Resource[InAppPurchaseOfferCodeAttributes]{
			{
				ID: "code-1",
				Attributes: InAppPurchaseOfferCodeAttributes{
					Name:                "SPRING",
					Active:              true,
					ProductionCodeCount: 10,
					SandboxCodeCount:    2,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Prod Codes") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "SPRING") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}
