package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue621_BuildsTestFlightRelationshipEndpoints_Mutations(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		wantBodyFn func(*testing.T, []byte)
		call       func(*Client) error
	}{
		{
			name:       "AddBuildsToBetaGroup (POST /v1/betaGroups/{id}/relationships/builds)",
			wantMethod: http.MethodPost,
			wantPath:   "/v1/betaGroups/group-1/relationships/builds",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeBuilds || got.Data[0].ID != "build-1" {
					t.Fatalf("unexpected first item: %#v", got.Data[0])
				}
				if got.Data[1].Type != ResourceTypeBuilds || got.Data[1].ID != "build-2" {
					t.Fatalf("unexpected second item: %#v", got.Data[1])
				}
			},
			call: func(client *Client) error {
				return client.AddBuildsToBetaGroup(ctx, "group-1", []string{"build-1", "build-2"})
			},
		},
		{
			name:       "RemoveBuildsFromBetaGroup (DELETE /v1/betaGroups/{id}/relationships/builds)",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaGroups/group-1/relationships/builds",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 {
					t.Fatalf("expected 1 relationship item, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeBuilds || got.Data[0].ID != "build-1" {
					t.Fatalf("unexpected item: %#v", got.Data[0])
				}
			},
			call: func(client *Client) error {
				return client.RemoveBuildsFromBetaGroup(ctx, "group-1", []string{"build-1"})
			},
		},
		{
			name:       "UpdateBuildAppEncryptionDeclarationRelationship (PATCH /v1/builds/{id}/relationships/appEncryptionDeclaration)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/builds/build-1/relationships/appEncryptionDeclaration",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got BuildAppEncryptionDeclarationRelationshipUpdateRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeAppEncryptionDeclarations {
					t.Fatalf("expected type %q, got %q", ResourceTypeAppEncryptionDeclarations, got.Data.Type)
				}
				if got.Data.ID != "decl-1" {
					t.Fatalf("expected id %q, got %q", "decl-1", got.Data.ID)
				}
			},
			call: func(client *Client) error {
				return client.UpdateBuildAppEncryptionDeclarationRelationship(ctx, "build-1", "decl-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != tt.wantMethod {
					t.Fatalf("expected %s, got %s", tt.wantMethod, req.Method)
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
			}, jsonResponse(http.StatusNoContent, ""))

			if err := tt.call(client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}
