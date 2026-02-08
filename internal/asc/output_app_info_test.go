package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_AppInfos(t *testing.T) {
	resp := &AppInfosResponse{
		Data: []Resource[AppInfoAttributes]{
			{
				ID: "info-1",
				Attributes: AppInfoAttributes{
					"appStoreState":     "READY_FOR_REVIEW",
					"state":             "READY",
					"appStoreAgeRating": "12+",
					"kidsAgeBand":       "NINE_TO_ELEVEN",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "App Store State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "READY_FOR_REVIEW") {
		t.Fatalf("expected app store state in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppInfos(t *testing.T) {
	resp := &AppInfosResponse{
		Data: []Resource[AppInfoAttributes]{
			{
				ID: "info-1",
				Attributes: AppInfoAttributes{
					"appStoreState": "READY_FOR_REVIEW",
					"state":         "READY",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "info-1") {
		t.Fatalf("expected app info ID in output, got: %s", output)
	}
}
