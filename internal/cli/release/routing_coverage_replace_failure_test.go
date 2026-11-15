package release

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestApplyRoutingCoverageStepReportsDeletedCoverageAfterFailedReplacement
// proves a failed replacement never reports the already-deleted coverage as the
// current one. The replacement deletes the previous coverage before creating
// its successor, so a create failure leaves the version with no routing
// coverage at all.
func TestApplyRoutingCoverageStepReportsDeletedCoverageAfterFailedReplacement(t *testing.T) {
	d := releaseTestDeps()
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)

	originalTransport := http.DefaultTransport
	deleted := false
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			if deleted {
				return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
			}
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"old-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			deleted = true
			return releaseJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			return releaseJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"reservation unavailable"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), d, newReleaseTestClient(t), "VERSION_123", prepared, false)
	if err == nil {
		t.Fatal("applyPreparedRoutingCoverageStep() error = nil, want create failure")
	}
	if !strings.Contains(err.Error(), "failed to create") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want the create failure", err)
	}
	if !strings.Contains(err.Error(), "no routing coverage") || !strings.Contains(err.Error(), "COVERAGE_OLD") {
		t.Fatalf("error does not report that the previous coverage was deleted: %v", err)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok {
		t.Fatalf("expected routing coverage details, got %#v", outcome.Details)
	}
	if details.CoverageID != "" {
		t.Fatalf("failed replacement reported a deleted coverage as current: %#v", details)
	}
	if details.DeliveryState != "" {
		t.Fatalf("failed replacement reported the deleted coverage delivery state: %#v", details)
	}
	if details.Action != "replace_failed" {
		t.Fatalf("expected a failed replacement action, got %#v", details)
	}
	if details.PreviousCoverageID != "COVERAGE_OLD" {
		t.Fatalf("expected the deleted coverage to be reported as previous, got %#v", details)
	}
}

// TestApplyRoutingCoverageStepKeepsSurvivingCoverageWhenReplacementNeverDeleted
// proves the same reporting stays accurate when the replacement failed before
// the previous coverage was deleted: the version still has that coverage, and
// nothing claims otherwise.
func TestApplyRoutingCoverageStepKeepsSurvivingCoverageWhenReplacementNeverDeleted(t *testing.T) {
	d := releaseTestDeps()
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)

	originalTransport := http.DefaultTransport
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"old-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			return releaseJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"delete unavailable"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), d, newReleaseTestClient(t), "VERSION_123", prepared, false)
	if err == nil {
		t.Fatal("applyPreparedRoutingCoverageStep() error = nil, want delete failure")
	}
	if strings.Contains(err.Error(), "no routing coverage") {
		t.Fatalf("error claims the surviving coverage was deleted: %v", err)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok {
		t.Fatalf("expected routing coverage details, got %#v", outcome.Details)
	}
	if details.Action != "replace_failed" {
		t.Fatalf("expected a failed replacement action, got %#v", details)
	}
	if details.CoverageID != "COVERAGE_OLD" || details.DeliveryState != "COMPLETE" {
		t.Fatalf("expected the surviving coverage to stay reported as current, got %#v", details)
	}
	if details.PreviousCoverageID != "" {
		t.Fatalf("surviving coverage was also reported as previous: %#v", details)
	}
}
