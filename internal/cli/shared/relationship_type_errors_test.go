package shared

import (
	"errors"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Captured live from GET /v1/builds/999999999999/relationships/app on
// 2026-09-15 with the build ID substituted.
const missingBuildBody = `{"errors":[{"id":"6f3a1c22-1d7e-4c0e-9f1a-7b2c3d4e5f60","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id '999999999999'"}]}`

const missingRelationshipBody = `{"errors":[{"id":"7c8d9e0f-1a2b-4c3d-8e9f-0a1b2c3d4e5f","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'buildBetaDetails' with id 'build-1'"}]}`

const missingUnrelatedBody = `{"errors":[{"id":"8d9e0f1a-2b3c-4d5e-9f0a-1b2c3d4e5f60","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id 'app-1'"}]}`

func TestDescribeRelationshipLookupFailure(t *testing.T) {
	buildParent := RelationshipParent{
		ResourceType: "builds",
		Label:        "build",
		ID:           "999999999999",
		Hint:         `--build-id expects a build ID (list them with: asc builds list --app "APP_ID")`,
	}

	tests := []struct {
		name             string
		err              error
		relationshipType string
		parent           RelationshipParent
		wantNil          bool
		want             string
	}{
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
		{
			name:             "non not-found failure is unchanged",
			err:              errors.New("network unreachable"),
			relationshipType: "app",
			parent:           buildParent,
			want:             "network unreachable",
		},
		{
			name:             "unknown parent names the resource and hint",
			err:              asc.ParseErrorWithStatus([]byte(missingBuildBody), 404),
			relationshipType: "app",
			parent:           buildParent,
			want:             `build "999999999999" was not found; --build-id expects a build ID (list them with: asc builds list --app "APP_ID")`,
		},
		{
			name:             "unknown parent from a page URL blames the URL",
			err:              asc.ParseErrorWithStatus([]byte(missingBuildBody), 404),
			relationshipType: "individualTesters",
			parent:           RelationshipParent{ResourceType: "builds", Label: "build", Hint: "ignored without an ID"},
			want:             "the build referenced by the requested page URL was not found",
		},
		{
			name:             "missing relationship names the relationship and parent",
			err:              asc.ParseErrorWithStatus([]byte(missingRelationshipBody), 404),
			relationshipType: "buildBetaDetail",
			parent:           RelationshipParent{ResourceType: "builds", Label: "build", ID: "build-1"},
			want:             "buildBetaDetail relationship was not found for build \"build-1\": The specified resource does not exist: There is no resource of type 'buildBetaDetails' with id 'build-1'",
		},
		{
			name:             "missing relationship without a parent id",
			err:              asc.ParseErrorWithStatus([]byte(missingRelationshipBody), 404),
			relationshipType: "buildBetaDetail",
			parent:           RelationshipParent{ResourceType: "builds", Label: "build"},
			want:             "buildBetaDetail relationship was not found: The specified resource does not exist: There is no resource of type 'buildBetaDetails' with id 'build-1'",
		},
		{
			name:             "unclassified not found keeps the api message",
			err:              asc.ParseErrorWithStatus([]byte(missingUnrelatedBody), 404),
			relationshipType: "buildBetaDetail",
			parent:           RelationshipParent{ResourceType: "builds", Label: "build", ID: "build-1"},
			want:             "The specified resource does not exist: There is no resource of type 'apps' with id 'app-1'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DescribeRelationshipLookupFailure(test.err, test.relationshipType, test.parent)
			if test.wantNil {
				if got != nil {
					t.Fatalf("error = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("error = nil, want an error")
			}
			if got.Error() != test.want {
				t.Fatalf("error = %q, want %q", got, test.want)
			}
			if test.err != nil && !errors.Is(got, test.err) {
				t.Fatalf("error %q lost its cause %v", got, test.err)
			}
		})
	}
}

func TestRelationshipTypeFlagUsageListsValues(t *testing.T) {
	got := RelationshipTypeFlagUsage([]string{"app", "builds"})
	want := "Relationship type (required); must be one of: app, builds"
	if got != want {
		t.Fatalf("usage = %q, want %q", got, want)
	}
}
