package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue622_MetadataRelationshipEndpoints_GET(t *testing.T) {
	ctx := context.Background()

	const (
		linkagesOK = `{"data":[{"type":"apps","id":"1"}],"links":{}}`
		toOneOK    = `{"data":{"type":"apps","id":"1"},"links":{}}`
	)

	tests := []struct {
		name       string
		wantPath   string
		body       string
		checkQuery func(*testing.T, *http.Request)
		call       func(*Client) error
	}{
		{
			name:     "GetAppPreviewSetAppPreviewsRelationships",
			wantPath: "/v1/appPreviewSets/set-1/relationships/appPreviews",
			body:     linkagesOK,
			checkQuery: func(t *testing.T, req *http.Request) {
				t.Helper()
				if req.URL.Query().Get("limit") != "5" {
					t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
				}
			},
			call: func(client *Client) error {
				_, err := client.GetAppPreviewSetAppPreviewsRelationships(ctx, "set-1", Limit[LinkagesOption](5))
				return err
			},
		},
		{
			name:     "GetAppScreenshotSetAppScreenshotsRelationships",
			wantPath: "/v1/appScreenshotSets/set-1/relationships/appScreenshots",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetAppScreenshotSetAppScreenshotsRelationships(ctx, "set-1")
				return err
			},
		},
		{
			name:     "GetAppStoreVersionBuildRelationship",
			wantPath: "/v1/appStoreVersions/ver-1/relationships/build",
			body:     toOneOK,
			call: func(client *Client) error {
				_, err := client.GetAppStoreVersionBuildRelationship(ctx, "ver-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != tt.wantPath {
					t.Fatalf("expected path %s, got %s", tt.wantPath, req.URL.Path)
				}
				if tt.checkQuery != nil {
					tt.checkQuery(t, req)
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.body))

			if err := tt.call(client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}

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
