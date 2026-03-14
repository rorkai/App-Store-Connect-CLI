package versions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestFetchOptionalBuild_NotFound(t *testing.T) {
	resp, err := fetchOptionalBuild(context.Background(), "VERSION_ID", func(ctx context.Context, versionID string) (*asc.BuildResponse, error) {
		return nil, &asc.APIError{Code: "NOT_FOUND", Title: "Not Found"}
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
}

func TestFetchOptionalBuild_Error(t *testing.T) {
	expected := errors.New("boom")
	_, err := fetchOptionalBuild(context.Background(), "VERSION_ID", func(ctx context.Context, versionID string) (*asc.BuildResponse, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected error %v, got %v", expected, err)
	}
}

func TestFetchOptionalBuild_Success(t *testing.T) {
	resp, err := fetchOptionalBuild(context.Background(), "VERSION_ID", func(ctx context.Context, versionID string) (*asc.BuildResponse, error) {
		return &asc.BuildResponse{
			Data: asc.Resource[asc.BuildAttributes]{
				ID: "BUILD_ID",
				Attributes: asc.BuildAttributes{
					Version: "1.0",
				},
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp == nil || resp.Data.ID != "BUILD_ID" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestFetchOptionalSubmission_NotFound(t *testing.T) {
	resp, err := fetchOptionalSubmission(context.Background(), "VERSION_ID", func(ctx context.Context, versionID string) (*asc.AppStoreVersionSubmissionResourceResponse, error) {
		return nil, &asc.APIError{Code: "NOT_FOUND", Title: "Not Found"}
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
}

func TestFetchOptionalSubmission_Error(t *testing.T) {
	expected := errors.New("boom")
	_, err := fetchOptionalSubmission(context.Background(), "VERSION_ID", func(ctx context.Context, versionID string) (*asc.AppStoreVersionSubmissionResourceResponse, error) {
		return nil, expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected error %v, got %v", expected, err)
	}
}

func TestFetchOptionalSubmission_Success(t *testing.T) {
	resp, err := fetchOptionalSubmission(context.Background(), "VERSION_ID", func(ctx context.Context, versionID string) (*asc.AppStoreVersionSubmissionResourceResponse, error) {
		return &asc.AppStoreVersionSubmissionResourceResponse{
			Data: asc.AppStoreVersionSubmissionResource{
				Type: asc.ResourceTypeAppStoreVersionSubmissions,
				ID:   "SUBMIT_ID",
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp == nil || resp.Data.ID != "SUBMIT_ID" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSplitCompatAppStoreVersionIncludes(t *testing.T) {
	apiIncludes, includeAgeRating := splitCompatAppStoreVersionIncludes([]string{
		"ageRatingDeclaration",
		"appStoreReviewDetail",
		"routingAppCoverage",
	})

	if !includeAgeRating {
		t.Fatal("expected age rating include compatibility flag")
	}
	if len(apiIncludes) != 2 {
		t.Fatalf("expected 2 API includes, got %d (%v)", len(apiIncludes), apiIncludes)
	}
	if apiIncludes[0] != "appStoreReviewDetail" || apiIncludes[1] != "routingAppCoverage" {
		t.Fatalf("unexpected API includes: %v", apiIncludes)
	}
}

func TestAppendAgeRatingDeclarationInclude(t *testing.T) {
	versionResp := &asc.AppStoreVersionResponse{
		Data: asc.Resource[asc.AppStoreVersionAttributes]{
			Type: asc.ResourceTypeAppStoreVersions,
			ID:   "version-1",
			Relationships: json.RawMessage(`{
				"appStoreReviewDetail":{"data":{"type":"appStoreReviewDetails","id":"review-1"}}
			}`),
		},
		Included: json.RawMessage(`[
			{"type":"appStoreReviewDetails","id":"review-1","attributes":{"contactEmail":"qa@example.com"}}
		]`),
	}
	ageRatingResp := &asc.AgeRatingDeclarationResponse{
		Data: asc.Resource[asc.AgeRatingDeclarationAttributes]{
			Type: asc.ResourceTypeAgeRatingDeclarations,
			ID:   "age-1",
			Attributes: asc.AgeRatingDeclarationAttributes{
				Gambling: boolPtr(true),
			},
		},
	}

	if err := appendAgeRatingDeclarationInclude(versionResp, ageRatingResp); err != nil {
		t.Fatalf("appendAgeRatingDeclarationInclude() error: %v", err)
	}

	var relationships map[string]json.RawMessage
	if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
		t.Fatalf("unmarshal relationships: %v", err)
	}
	if _, ok := relationships["ageRatingDeclaration"]; !ok {
		t.Fatal("expected ageRatingDeclaration relationship to be added")
	}
	if _, ok := relationships["appStoreReviewDetail"]; !ok {
		t.Fatal("expected existing relationship to be preserved")
	}

	var included []map[string]any
	if err := json.Unmarshal(versionResp.Included, &included); err != nil {
		t.Fatalf("unmarshal included: %v", err)
	}
	if len(included) != 2 {
		t.Fatalf("expected 2 included resources, got %d", len(included))
	}

	foundAgeRating := false
	for _, item := range included {
		if item["type"] == string(asc.ResourceTypeAgeRatingDeclarations) && item["id"] == "age-1" {
			foundAgeRating = true
		}
	}
	if !foundAgeRating {
		t.Fatal("expected age rating declaration in included resources")
	}
}

func boolPtr(value bool) *bool {
	return &value
}
