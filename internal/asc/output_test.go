package asc

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	err = fn()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close error: %v", closeErr)
	}
	os.Stdout = orig

	var buf bytes.Buffer
	if _, readErr := io.Copy(&buf, r); readErr != nil {
		t.Fatalf("read error: %v", readErr)
	}
	if err != nil {
		t.Fatalf("function error: %v", err)
	}

	return buf.String()
}

func assertRenderedNonJSONContains(t *testing.T, render func(any) error, data any, want ...string) {
	t.Helper()

	output := captureStdout(t, func() error {
		return render(data)
	})
	trimmed := strings.TrimSpace(output)

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		t.Fatalf("expected non-JSON rendered output, got JSON: %s", output)
	}

	for _, token := range want {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
}

func TestPrintTableAndMarkdown_RepresentativeResponses(t *testing.T) {
	renderers := []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	}

	tests := []struct {
		name string
		data any
		want []string
	}{
		{
			name: "feedback with screenshots",
			data: &FeedbackResponse{
				Data: []Resource[FeedbackAttributes]{
					{
						ID: "1",
						Attributes: FeedbackAttributes{
							CreatedDate: "2026-01-20T00:00:00Z",
							Email:       "tester@example.com",
							Comment:     "Looks good",
							Screenshots: []FeedbackScreenshotImage{
								{URL: "https://example.com/shot.png"},
							},
						},
					},
				},
			},
			want: []string{"Created", "Email", "Screenshots", "tester@example.com", "https://example.com/shot.png"},
		},
		{
			name: "apps",
			data: &AppsResponse{
				Data: []Resource[AppAttributes]{
					{
						ID: "123",
						Attributes: AppAttributes{
							Name:     "Demo App",
							BundleID: "com.example.demo",
							SKU:      "SKU-1",
						},
					},
				},
			},
			want: []string{"ID", "Bundle ID", "SKU", "Demo App", "com.example.demo"},
		},
		{
			name: "builds",
			data: &BuildsResponse{
				Data: []Resource[BuildAttributes]{
					{
						ID: "1",
						Attributes: BuildAttributes{
							Version:         "1.2.3",
							UploadedDate:    "2026-01-20T00:00:00Z",
							ProcessingState: "PROCESSING",
						},
					},
				},
			},
			want: []string{"ID", "Processing", "1.2.3", "PROCESSING"},
		},
		{
			name: "app events",
			data: &AppEventsResponse{
				Data: []Resource[AppEventAttributes]{
					{
						ID: "evt-1",
						Attributes: AppEventAttributes{
							ReferenceName: "Holiday Event",
							EventState:    "LIVE",
						},
					},
				},
			},
			want: []string{"ID", "Reference Name", "Holiday Event", "LIVE"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, renderer := range renderers {
				renderer := renderer
				t.Run(renderer.name, func(t *testing.T) {
					assertRenderedNonJSONContains(t, renderer.fn, tc.data, tc.want...)
				})
			}
		})
	}
}

func TestPrintTableAndMarkdown_StripsControlChars(t *testing.T) {
	resp := &FeedbackResponse{
		Data: []Resource[FeedbackAttributes]{
			{
				ID: "1",
				Attributes: FeedbackAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Email:       "test\x1b[2J@example.com",
					Comment:     "ok\x1b[31mRED\x1b[0m",
				},
			},
		},
	}

	renderers := []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	}

	for _, renderer := range renderers {
		renderer := renderer
		t.Run(renderer.name, func(t *testing.T) {
			output := captureStdout(t, func() error {
				return renderer.fn(resp)
			})
			if strings.Contains(output, "\x1b") {
				t.Fatalf("expected control characters to be stripped, got: %q", output)
			}
		})
	}
}

func TestPrintTableAndMarkdown_BuildsEmptyStillShowsHeaders(t *testing.T) {
	resp := &BuildsResponse{Data: []Resource[BuildAttributes]{}}
	renderers := []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	}

	for _, renderer := range renderers {
		renderer := renderer
		t.Run(renderer.name, func(t *testing.T) {
			assertRenderedNonJSONContains(t, renderer.fn, resp, "ID", "Version", "Processing")
		})
	}
}

func TestPrintTableAndMarkdown_UnregisteredTypeFallsBackToJSON(t *testing.T) {
	type unregisteredOutput struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	value := unregisteredOutput{Name: "fallback", Count: 7}
	renderers := []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	}

	for _, renderer := range renderers {
		renderer := renderer
		t.Run(renderer.name, func(t *testing.T) {
			output := captureStdout(t, func() error {
				return renderer.fn(value)
			})

			var parsed map[string]any
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Fatalf("expected JSON fallback output, got parse error: %v (output=%q)", err, output)
			}
			if parsed["name"] != "fallback" {
				t.Fatalf("expected name=fallback, got %#v", parsed["name"])
			}
			if parsed["count"] != float64(7) {
				t.Fatalf("expected count=7, got %#v", parsed["count"])
			}
		})
	}
}

func TestPrintPrettyJSON_Indentation(t *testing.T) {
	resp := &ReviewsResponse{
		Data: []Resource[ReviewAttributes]{
			{
				ID: "1",
				Attributes: ReviewAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Rating:      5,
					Title:       "Great app",
					Body:        "Nice work",
					Territory:   "US",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintPrettyJSON(resp)
	})

	if !strings.Contains(output, "\n  \"data\"") {
		t.Fatalf("expected pretty JSON indentation, got: %s", output)
	}
}

func TestPrintPrettyJSON_PerfPowerMetricsUsesRawData(t *testing.T) {
	resp := &PerfPowerMetricsResponse{
		Data: json.RawMessage(`{"metrics":[{"name":"cpu"}]}`),
	}

	output := captureStdout(t, func() error {
		return PrintPrettyJSON(resp)
	})

	if !strings.Contains(output, "\n  \"metrics\"") {
		t.Fatalf("expected pretty-printed raw metrics JSON, got: %s", output)
	}
}

func TestPrintJSON_CustomProductPageUploadResultUsesCustomLocalizationID(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{
			name: "screenshot upload result",
			data: &CustomProductPageScreenshotUploadResult{
				CustomProductPageLocalizationID: "CPP_LOC_123",
				SetID:                           "SET_123",
				DisplayType:                     "APP_IPHONE_65",
				Results: []AssetUploadResultItem{
					{FileName: "shot.png", AssetID: "SHOT_123", State: "COMPLETE"},
				},
			},
		},
		{
			name: "preview upload result",
			data: &CustomProductPagePreviewUploadResult{
				CustomProductPageLocalizationID: "CPP_LOC_123",
				SetID:                           "SET_123",
				PreviewType:                     "IPHONE_65",
				Results: []AssetUploadResultItem{
					{FileName: "preview.mov", AssetID: "PREVIEW_123", State: "COMPLETE"},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			output := captureStdout(t, func() error {
				return PrintJSON(tc.data)
			})

			var parsed map[string]any
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Fatalf("failed to parse output JSON: %v", err)
			}
			if got := parsed["customProductPageLocalizationId"]; got != "CPP_LOC_123" {
				t.Fatalf("expected customProductPageLocalizationId=CPP_LOC_123, got %#v", got)
			}
			if _, exists := parsed["versionLocalizationId"]; exists {
				t.Fatalf("did not expect versionLocalizationId in output JSON: %s", output)
			}
		})
	}
}

func TestPrintTable_SkippedAssetUploadResultShowsSkippedState(t *testing.T) {
	resp := &AppScreenshotUploadResult{
		VersionLocalizationID: "LOC_123",
		SetID:                 "SET_123",
		DisplayType:           "APP_IPHONE_65",
		Results: []AssetUploadResultItem{
			{FileName: "shot.png", FilePath: "/tmp/shot.png", Skipped: true},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "skipped") {
		t.Fatalf("expected skipped state in table output, got: %s", output)
	}
}

func TestPrintTableAndMarkdown_BuildUploadResultIncludesOperations(t *testing.T) {
	resp := &BuildUploadResult{
		UploadID: "UPLOAD_123",
		FileID:   "FILE_123",
		FileName: "app.ipa",
		FileSize: 1024,
		Operations: []UploadOperation{
			{
				Method: "PUT",
				URL:    "https://example.com/upload",
				Length: 1024,
				Offset: 0,
			},
		},
	}

	renderers := []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	}

	for _, renderer := range renderers {
		renderer := renderer
		t.Run(renderer.name, func(t *testing.T) {
			assertRenderedNonJSONContains(t, renderer.fn, resp, "UPLOAD_123", "PUT")
		})
	}
}
