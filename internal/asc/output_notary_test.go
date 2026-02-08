package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_NotarySubmissionStatus(t *testing.T) {
	resp := &NotarySubmissionStatusResponse{
		Data: NotarySubmissionStatusData{
			ID:   "sub-123",
			Type: "submissions",
			Attributes: NotarySubmissionStatusAttributes{
				Status:      NotaryStatusAccepted,
				Name:        "MyApp.zip",
				CreatedDate: "2026-01-15T10:00:00Z",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "sub-123") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Accepted") {
		t.Fatalf("expected status in output, got: %s", output)
	}
	if !strings.Contains(output, "MyApp.zip") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_NotarySubmissionStatus(t *testing.T) {
	resp := &NotarySubmissionStatusResponse{
		Data: NotarySubmissionStatusData{
			ID:   "sub-456",
			Type: "submissions",
			Attributes: NotarySubmissionStatusAttributes{
				Status:      NotaryStatusRejected,
				Name:        "Other.dmg",
				CreatedDate: "2026-02-01T08:30:00Z",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Status") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "sub-456") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Rejected") {
		t.Fatalf("expected status in output, got: %s", output)
	}
}

func TestPrintTable_NotarySubmissionsList(t *testing.T) {
	resp := &NotarySubmissionsListResponse{
		Data: []NotarySubmissionStatusData{
			{
				ID:   "sub-1",
				Type: "submissions",
				Attributes: NotarySubmissionStatusAttributes{
					Status:      NotaryStatusAccepted,
					Name:        "app1.zip",
					CreatedDate: "2026-01-10T10:00:00Z",
				},
			},
			{
				ID:   "sub-2",
				Type: "submissions",
				Attributes: NotarySubmissionStatusAttributes{
					Status:      NotaryStatusInProgress,
					Name:        "app2.pkg",
					CreatedDate: "2026-01-15T14:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "sub-1") {
		t.Fatalf("expected first ID in output, got: %s", output)
	}
	if !strings.Contains(output, "sub-2") {
		t.Fatalf("expected second ID in output, got: %s", output)
	}
	if !strings.Contains(output, "In Progress") {
		t.Fatalf("expected status in output, got: %s", output)
	}
}

func TestPrintMarkdown_NotarySubmissionsList(t *testing.T) {
	resp := &NotarySubmissionsListResponse{
		Data: []NotarySubmissionStatusData{
			{
				ID:   "sub-a",
				Type: "submissions",
				Attributes: NotarySubmissionStatusAttributes{
					Status:      NotaryStatusInvalid,
					Name:        "broken.zip",
					CreatedDate: "2026-01-20T09:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Status") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "sub-a") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Invalid") {
		t.Fatalf("expected status in output, got: %s", output)
	}
}

func TestPrintTable_NotarySubmissionsList_Empty(t *testing.T) {
	resp := &NotarySubmissionsListResponse{
		Data: []NotarySubmissionStatusData{},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected header even for empty list, got: %s", output)
	}
}

func TestPrintTable_NotarySubmissionLogs(t *testing.T) {
	resp := &NotarySubmissionLogsResponse{
		Data: NotarySubmissionLogsData{
			ID:   "sub-log-1",
			Type: "submissionsLog",
			Attributes: NotarySubmissionLogsAttributes{
				DeveloperLogURL: "https://example.com/log.json",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "sub-log-1") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/log.json") {
		t.Fatalf("expected log URL in output, got: %s", output)
	}
}

func TestPrintMarkdown_NotarySubmissionLogs(t *testing.T) {
	resp := &NotarySubmissionLogsResponse{
		Data: NotarySubmissionLogsData{
			ID:   "sub-log-2",
			Type: "submissionsLog",
			Attributes: NotarySubmissionLogsAttributes{
				DeveloperLogURL: "https://example.com/audit.json",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Developer Log URL") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "sub-log-2") {
		t.Fatalf("expected ID in output, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/audit.json") {
		t.Fatalf("expected log URL in output, got: %s", output)
	}
}
