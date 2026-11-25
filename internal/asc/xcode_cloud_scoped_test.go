package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestResolveCiWorkflowByNameScopesContinuationWhenMarked(t *testing.T) {
	const path = "/v1/ciProducts/product-1/workflows"
	tests := []struct {
		name    string
		nextURL string
		marked  bool
		wantErr bool
	}{
		{
			name:    "same-origin absolute",
			nextURL: BaseURL + path + "?cursor=next",
			marked:  true,
		},
		{
			name:    "root-relative",
			nextURL: path + "?cursor=next",
			marked:  true,
		},
		{
			name:    "off-collection marked",
			nextURL: "/v1/ciProducts/other/workflows?cursor=next",
			marked:  true,
			wantErr: true,
		},
		{
			name:    "wrong-origin marked",
			nextURL: "https://evil.example" + path + "?cursor=next",
			marked:  true,
			wantErr: true,
		},
		{
			name:    "off-collection unmarked remains compatible",
			nextURL: "/v1/ciProducts/other/workflows?cursor=next",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			client := newTestClient(
				t,
				func(*http.Request) { requestCount++ },
				jsonResponse(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"Deploy"}}],"links":{"next":"`+test.nextURL+`"}}`),
				jsonResponse(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"wf-2","attributes":{"name":"CI Build"}}],"links":{}}`),
			)
			ctx := context.Background()
			if test.marked {
				ctx = WithNoRedirectReads(ctx)
			}

			workflow, err := client.ResolveCiWorkflowByName(ctx, "product-1", "ci build")
			if test.wantErr {
				if err == nil {
					t.Fatal("expected scoped continuation refusal")
				}
				if requestCount != 1 {
					t.Fatalf("request count = %d, want 1 before refusal", requestCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveCiWorkflowByName() error: %v", err)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
			if workflow.ID != "wf-2" {
				t.Fatalf("workflow ID = %q, want wf-2", workflow.ID)
			}
		})
	}
}

func TestResolveGitReferenceByNameScopesContinuationWhenMarked(t *testing.T) {
	const path = "/v1/scmRepositories/repo-1/gitReferences"
	tests := []struct {
		name    string
		nextURL string
		marked  bool
		wantErr bool
	}{
		{
			name:    "same repository",
			nextURL: BaseURL + path + "?cursor=next",
			marked:  true,
		},
		{
			name:    "drifted repository marked",
			nextURL: "/v1/scmRepositories/other/gitReferences?cursor=next",
			marked:  true,
			wantErr: true,
		},
		{
			name:    "wrong origin marked",
			nextURL: "https://evil.example" + path + "?cursor=next",
			marked:  true,
			wantErr: true,
		},
		{
			name:    "drifted repository unmarked remains compatible",
			nextURL: "/v1/scmRepositories/other/gitReferences?cursor=next",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			client := newTestClient(
				t,
				func(*http.Request) { requestCount++ },
				jsonResponse(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"Deploy","canonicalName":"refs/heads/deploy","isDeleted":false}}],"links":{"next":"`+test.nextURL+`"}}`),
				jsonResponse(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-2","attributes":{"name":"main","canonicalName":"refs/heads/main","isDeleted":false}}],"links":{}}`),
			)
			ctx := context.Background()
			if test.marked {
				ctx = WithNoRedirectReads(ctx)
			}

			ref, err := client.ResolveGitReferenceByName(ctx, "repo-1", "main")
			if test.wantErr {
				if err == nil {
					t.Fatal("expected scoped continuation refusal")
				}
				if requestCount != 1 {
					t.Fatalf("request count = %d, want 1 before refusal", requestCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveGitReferenceByName() error: %v", err)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
			if ref.ID != "ref-2" {
				t.Fatalf("reference ID = %q, want ref-2", ref.ID)
			}
		})
	}
}
