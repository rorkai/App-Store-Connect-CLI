package asc

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestGetBuildBetaAppReviewSubmission_NullDataReturnsNotFound(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/betaAppReviewSubmission"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/betaAppReviewSubmission" {
			t.Fatalf("expected path /v1/builds/build-1/betaAppReviewSubmission, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetBuildBetaAppReviewSubmission(context.Background(), "build-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	var missingErr MissingBuildBetaAppReviewSubmissionError
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected MissingBuildBetaAppReviewSubmissionError, got %T", err)
	}
	if missingErr.BuildID != "build-1" {
		t.Fatalf("expected build id build-1, got %q", missingErr.BuildID)
	}
}
