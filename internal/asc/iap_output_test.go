package asc

import (
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

	if !strings.Contains(output, "| ID | File Name | File Size | State |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "UPLOAD_COMPLETE") {
		t.Fatalf("expected state in output, got: %s", output)
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

	if !strings.Contains(output, "| ID | Customer Price | Proceeds |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "1.40") {
		t.Fatalf("expected proceeds in output, got: %s", output)
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

	if !strings.Contains(output, "| ID | Name | Active | Prod Codes | Sandbox Codes |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "SPRING") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}
