package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue622_MetadataRelationshipEndpoints_PATCH(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		wantPath   string
		wantBodyFn func(*testing.T, []byte)
		call       func(*Client) error
	}{
		{
			name:     "UpdateAppPreviewSetAppPreviewsRelationship",
			wantPath: "/v1/appPreviewSets/set-1/relationships/appPreviews",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeAppPreviews || got.Data[0].ID != "pre-1" {
					t.Fatalf("unexpected first item: %#v", got.Data[0])
				}
				if got.Data[1].Type != ResourceTypeAppPreviews || got.Data[1].ID != "pre-2" {
					t.Fatalf("unexpected second item: %#v", got.Data[1])
				}
			},
			call: func(client *Client) error {
				return client.UpdateAppPreviewSetAppPreviewsRelationship(ctx, "set-1", []string{"pre-1", "pre-2"})
			},
		},
		{
			name:     "UpdateAppScreenshotSetAppScreenshotsRelationship",
			wantPath: "/v1/appScreenshotSets/set-1/relationships/appScreenshots",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeAppScreenshots || got.Data[0].ID != "shot-1" {
					t.Fatalf("unexpected first item: %#v", got.Data[0])
				}
				if got.Data[1].Type != ResourceTypeAppScreenshots || got.Data[1].ID != "shot-2" {
					t.Fatalf("unexpected second item: %#v", got.Data[1])
				}
			},
			call: func(client *Client) error {
				return client.UpdateAppScreenshotSetAppScreenshotsRelationship(ctx, "set-1", []string{"shot-1", "shot-2"})
			},
		},
		{
			name:     "UpdateAppScreenshotSetAppScreenshotsRelationship empty clears relationship",
			wantPath: "/v1/appScreenshotSets/set-1/relationships/appScreenshots",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got map[string]json.RawMessage
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if string(got["data"]) != "[]" {
					t.Fatalf("expected empty relationship data array, got %s", body)
				}
			},
			call: func(client *Client) error {
				return client.UpdateAppScreenshotSetAppScreenshotsRelationship(ctx, "set-1", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodPatch {
					t.Fatalf("expected PATCH, got %s", req.Method)
				}
				if req.URL.Path != tt.wantPath {
					t.Fatalf("expected path %s, got %s", tt.wantPath, req.URL.Path)
				}

				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				tt.wantBodyFn(t, body)

				assertAuthorized(t, req)
			}, jsonResponse(http.StatusNoContent, "")) // relationship PATCH endpoints return 204

			if err := tt.call(client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}
