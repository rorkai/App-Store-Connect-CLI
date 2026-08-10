package release

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	routingcoveragecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
)

const validReleaseRoutingCoverageGeoJSON = `{"type":"MultiPolygon","coordinates":[[[[77.5,12.9],[77.7,12.9],[77.7,13.1],[77.5,12.9]]]]}`

func prepareReleaseRoutingCoverage(t *testing.T, path string) routingcoveragecli.PreparedRoutingCoverageFile {
	t.Helper()
	t.Chdir(filepath.Dir(path))
	prepared, err := routingcoveragecli.PrepareRoutingCoverageFile(path)
	if err != nil {
		t.Fatalf("PrepareRoutingCoverageFile() error: %v", err)
	}
	return prepared
}

func TestApplyRoutingCoverageStepReusesMatchingCompleteAsset(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	checksum, err := asc.ComputeFileChecksum(coveragePath, asc.ChecksumAlgorithmMD5)
	if err != nil {
		t.Fatalf("compute routing coverage checksum: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/VERSION_123/routingAppCoverage" {
			t.Fatalf("unexpected mutation while reusing routing coverage: %s %s", req.Method, req.URL.String())
		}
		return releaseJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"fileName":"coverage.geojson","sourceFileChecksum":%q,"assetDeliveryState":{"state":"COMPLETE"}}}}`, checksum.Hash))
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepareReleaseRoutingCoverage(t, coveragePath), false)
	if err != nil {
		t.Fatalf("applyRoutingCoverageStep() error: %v", err)
	}
	if outcome.Status != "skipped" || !outcome.Persist {
		t.Fatalf("expected persisted reuse outcome, got %#v", outcome)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok || details.Action != "reuse" || details.CoverageID != "COVERAGE_123" {
		t.Fatalf("unexpected reuse details: %#v", outcome.Details)
	}
}

func TestWaitForRoutingCoverageDeliveryIncludesSchemaErrorDescriptions(t *testing.T) {
	client, _ := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/routingAppCoverages/COVERAGE_123" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"FAILED","errors":[{"code":"FILE_INVALID","description":"The GeoJSON is invalid."},{"description":"Try another file."}]}}}}`)
	}))

	state, err := waitForRoutingCoverageDelivery(context.Background(), client, "COVERAGE_123")
	if state != "FAILED" {
		t.Fatalf("waitForRoutingCoverageDelivery() state = %q, want FAILED", state)
	}
	const want = "routing coverage COVERAGE_123 delivery failed: FILE_INVALID: The GeoJSON is invalid.; Try another file."
	if err == nil || err.Error() != want {
		t.Fatalf("waitForRoutingCoverageDelivery() error = %v, want %q", err, want)
	}
}

func TestApplyRoutingCoverageStepRevalidatesBeforeReusingCompleteAsset(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON+"\n"), 0o600); err != nil {
		t.Fatalf("change routing coverage fixture: %v", err)
	}

	client, _ := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/VERSION_123/routingAppCoverage" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		writeReleaseTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"sourceFileChecksum":%q,"assetDeliveryState":{"state":"COMPLETE"}}}}`, prepared.Checksum))
	}))

	_, err := applyPreparedRoutingCoverageStep(context.Background(), client, "VERSION_123", prepared, false)
	if err == nil || !strings.Contains(err.Error(), "file changed after validation") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want changed-file diagnostic", err)
	}
}

func TestApplyRoutingCoverageStepRevalidatesAfterWaitingForMatchingAsset(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)

	client, _ := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			writeReleaseTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"sourceFileChecksum":%q,"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`, prepared.Checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON+"\n"), 0o600); err != nil {
				t.Errorf("change routing coverage fixture: %v", err)
				http.Error(w, "fixture write failed", http.StatusInternalServerError)
				return
			}
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))

	_, err := applyPreparedRoutingCoverageStep(context.Background(), client, "VERSION_123", prepared, false)
	if err == nil || !strings.Contains(err.Error(), "file changed after validation") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want changed-file diagnostic", err)
	}
}

func TestApplyRoutingCoverageStepRefreshesWhenMatchingRelationshipOmitsDeliveryState(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)
	requestPaths := []string{}

	client, _ := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestPaths = append(requestPaths, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			writeReleaseTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"sourceFileChecksum":%q}}}`, prepared.Checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), client, "VERSION_123", prepared, false)
	if err != nil {
		t.Fatalf("applyPreparedRoutingCoverageStep() error: %v", err)
	}
	if outcome.Status != "skipped" || !outcome.Persist {
		t.Fatalf("expected persisted reuse outcome, got %#v", outcome)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok || details.Action != "reuse" || details.DeliveryState != "COMPLETE" {
		t.Fatalf("unexpected reuse details: %#v", outcome.Details)
	}
	wantPaths := "GET /v1/appStoreVersions/VERSION_123/routingAppCoverage,GET /v1/routingAppCoverages/COVERAGE_123"
	if strings.Join(requestPaths, ",") != wantPaths {
		t.Fatalf("unexpected reconciliation requests: %v", requestPaths)
	}
}

func TestApplyRoutingCoverageStepReplacesMatchingAwaitingUploadWhenRelationshipOmitsDeliveryState(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)
	requestPaths := []string{}

	var serverURL string
	client, serverURL := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestPaths = append(requestPaths, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			writeReleaseTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":%q}}}`, prepared.Checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"","assetDeliveryState":{"state":"AWAITING_UPLOAD"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			writeReleaseTestJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"length":%d,"offset":0}]}}}`, serverURL+"/upload/coverage", len(validReleaseRoutingCoverageGeoJSON)))
		case req.Method == http.MethodPut && req.URL.Path == "/upload/coverage":
			if _, err := io.Copy(io.Discard, req.Body); err != nil {
				t.Errorf("read upload body: %v", err)
				http.Error(w, "read failed", http.StatusInternalServerError)
				return
			}
			writeReleaseTestJSON(w, http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outcome, err := applyPreparedRoutingCoverageStep(ctx, client, "VERSION_123", prepared, false)
	if err != nil {
		t.Fatalf("applyPreparedRoutingCoverageStep() error: %v", err)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if outcome.Status != "ok" || !outcome.Persist || !ok || details.Action != "replace" || details.CoverageID != "COVERAGE_NEW" {
		t.Fatalf("unexpected replacement outcome: %#v", outcome)
	}
	wantPaths := "GET /v1/appStoreVersions/VERSION_123/routingAppCoverage,GET /v1/routingAppCoverages/COVERAGE_OLD,DELETE /v1/routingAppCoverages/COVERAGE_OLD,POST /v1/routingAppCoverages,PUT /upload/coverage,PATCH /v1/routingAppCoverages/COVERAGE_NEW,GET /v1/routingAppCoverages/COVERAGE_NEW"
	if strings.Join(requestPaths, ",") != wantPaths {
		t.Fatalf("unexpected reconciliation requests: %v", requestPaths)
	}
}

func TestApplyRoutingCoverageStepRejectsMatchingCoverageWhenFullResourceOmitsDeliveryState(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)
	requestPaths := []string{}

	client, _ := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestPaths = append(requestPaths, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			writeReleaseTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"sourceFileChecksum":%q}}}`, prepared.Checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := applyPreparedRoutingCoverageStep(ctx, client, "VERSION_123", prepared, false)
	if err == nil || !strings.Contains(err.Error(), "response is missing a delivery state") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want missing-state diagnostic", err)
	}
	wantPaths := "GET /v1/appStoreVersions/VERSION_123/routingAppCoverage,GET /v1/routingAppCoverages/COVERAGE_123"
	if strings.Join(requestPaths, ",") != wantPaths {
		t.Fatalf("unexpected reconciliation requests: %v", requestPaths)
	}
}

func TestApplyRoutingCoverageStepDryRunPlansReplacementWithoutMutation(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/VERSION_123/routingAppCoverage" {
			t.Fatalf("unexpected mutation during routing coverage dry-run: %s %s", req.Method, req.URL.String())
		}
		return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"different","assetDeliveryState":{"state":"COMPLETE"}}}}`)
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepareReleaseRoutingCoverage(t, coveragePath), true)
	if err != nil {
		t.Fatalf("applyRoutingCoverageStep() error: %v", err)
	}
	if outcome.Status != "dry-run" || outcome.Persist {
		t.Fatalf("expected non-persisted dry-run outcome, got %#v", outcome)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok || details.Action != "replace" {
		t.Fatalf("unexpected dry-run details: %#v", outcome.Details)
	}
}

func TestApplyRoutingCoverageStepTreatsNullRelationshipAsMissing(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/VERSION_123/routingAppCoverage" {
			t.Fatalf("unexpected mutation during routing coverage dry-run: %s %s", req.Method, req.URL.String())
		}
		return releaseJSONResponse(http.StatusOK, `{"data":null}`)
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepareReleaseRoutingCoverage(t, coveragePath), true)
	if err != nil {
		t.Fatalf("applyRoutingCoverageStep() error: %v", err)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok || details.Action != "create" || details.CoverageID != "" {
		t.Fatalf("unexpected null-relationship plan: %#v", outcome.Details)
	}
}

func TestApplyRoutingCoverageStepCleansFailedReservation(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	deleted := false
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			return releaseJSONResponse(http.StatusOK, `{"data":null}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"uploadOperations":[]}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			deleted = true
			return releaseJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	_, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepareReleaseRoutingCoverage(t, coveragePath), false)
	if err == nil || !strings.Contains(err.Error(), "no upload operations returned") {
		t.Fatalf("applyRoutingCoverageStep() error = %v, want missing upload operations", err)
	}
	if !deleted {
		t.Fatal("expected failed routing coverage reservation to be deleted")
	}
}

func TestApplyRoutingCoverageStepReportsNewIDWhenReservationCleanupFails(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"old-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			return releaseJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"uploadOperations":[]}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			return releaseJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"cleanup unavailable"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepareReleaseRoutingCoverage(t, coveragePath), false)
	if err == nil || !strings.Contains(err.Error(), "also failed to delete routing coverage reservation COVERAGE_NEW") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want retained-reservation diagnostic", err)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok || details.Action != "replace" || details.CoverageID != "COVERAGE_NEW" {
		t.Fatalf("expected cleanup error details to preserve the new coverage ID, got %#v", outcome.Details)
	}
}

func TestApplyRoutingCoverageStepRevalidatesBeforeDeletingExistingCoverage(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON+"\n"), 0o600); err != nil {
		t.Fatalf("change routing coverage fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	deleted := false
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"old-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			deleted = true
			return releaseJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	_, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepared, false)
	if err == nil || !strings.Contains(err.Error(), "file changed after validation") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want changed-file diagnostic", err)
	}
	if deleted {
		t.Fatal("existing routing coverage was deleted before the prepared file was revalidated")
	}
}

func TestReplaceRoutingCoverageRejectsEmptyVersionBeforeDeletingExistingCoverage(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)

	originalTransport := http.DefaultTransport
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	_, err := routingcoveragecli.ReplaceRoutingCoverageWithPreparedFile(context.Background(), newReleaseTestClient(t), " \t ", "COVERAGE_OLD", prepared)
	if err == nil || !strings.Contains(err.Error(), "version ID is required") {
		t.Fatalf("ReplaceRoutingCoverageWithPreparedFile() error = %v, want version ID diagnostic", err)
	}
}

func TestApplyRoutingCoverageStepUploadsSnapshotAfterDeletingExistingCoverage(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	originalContent := []byte(validReleaseRoutingCoverageGeoJSON)
	changedContent := []byte(strings.Replace(validReleaseRoutingCoverageGeoJSON, "77.5", "78.5", 1))
	if len(changedContent) != len(originalContent) {
		t.Fatalf("fixture sizes differ: changed=%d original=%d", len(changedContent), len(originalContent))
	}
	if err := os.WriteFile(coveragePath, originalContent, 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)

	var uploaded []byte
	var serverURL string
	client, serverURL := newReleaseTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"old-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			if err := os.WriteFile(coveragePath, changedContent, 0o600); err != nil {
				t.Errorf("change routing coverage fixture: %v", err)
				http.Error(w, "fixture write failed", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			writeReleaseTestJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"length":%d,"offset":0}]}}}`, serverURL+"/upload/coverage", len(originalContent)))
		case req.Method == http.MethodPut && req.URL.Path == "/upload/coverage":
			var err error
			uploaded, err = io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read upload body: %v", err)
				http.Error(w, "read failed", http.StatusInternalServerError)
				return
			}
			writeReleaseTestJSON(w, http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			writeReleaseTestJSON(w, http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))

	_, err := applyPreparedRoutingCoverageStep(context.Background(), client, "VERSION_123", prepared, false)
	if err != nil {
		t.Fatalf("applyPreparedRoutingCoverageStep() error: %v", err)
	}
	if string(uploaded) != string(originalContent) {
		t.Fatalf("uploaded content = %q, want prepared snapshot %q", uploaded, originalContent)
	}
}

func TestUploadPreparedRoutingCoverageFileDoesNotDeleteAfterAmbiguousCommitResponse(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	prepared := prepareReleaseRoutingCoverage(t, coveragePath)

	originalTransport := http.DefaultTransport
	deleted := false
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			return releaseJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/coverage","length":%d,"offset":0}]}}}`, len(validReleaseRoutingCoverageGeoJSON)))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return releaseJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			deleted = true
			return releaseJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	_, err := routingcoveragecli.UploadPreparedRoutingCoverageFile(context.Background(), newReleaseTestClient(t), "VERSION_123", prepared)
	if err == nil || !strings.Contains(err.Error(), "committed routing coverage response is missing an ID") {
		t.Fatalf("UploadPreparedRoutingCoverageFile() error = %v, want missing-ID diagnostic", err)
	}
	if deleted {
		t.Fatal("routing coverage was deleted after an ambiguous successful commit response")
	}
}

func TestApplyRoutingCoverageStepReportsNewIDAfterAmbiguousReplacementCommit(t *testing.T) {
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
			return releaseJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			return releaseJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_NEW","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/coverage","length":%d,"offset":0}]}}}`, len(validReleaseRoutingCoverageGeoJSON)))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return releaseJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_NEW":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	outcome, err := applyPreparedRoutingCoverageStep(context.Background(), newReleaseTestClient(t), "VERSION_123", prepared, false)
	if err == nil || !strings.Contains(err.Error(), "committed routing coverage response is missing an ID") {
		t.Fatalf("applyPreparedRoutingCoverageStep() error = %v, want missing-ID diagnostic", err)
	}
	details, ok := outcome.Details.(routingCoverageStepDetails)
	if !ok || details.Action != "replace" || details.CoverageID != "COVERAGE_NEW" {
		t.Fatalf("expected replacement error details to preserve the new coverage ID, got %#v", outcome.Details)
	}
}

func TestVerifyResumedCheckpointRechecksRoutingCoverageInput(t *testing.T) {
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	opts := checkpointBindingOptions()
	opts.RoutingCoverageFile = "/tmp/coverage.geojson"
	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:        true,
			stepApplyMetadata:        true,
			stepApplyRoutingCoverage: true,
			stepAttachBuild:          true,
			stepValidateReadiness:    true,
		},
	}
	messages := []string{}
	if err := verifyResumedCheckpointBinding(context.Background(), client, opts, &checkpoint, func(message string) {
		messages = append(messages, message)
	}); err != nil {
		t.Fatalf("verifyResumedCheckpointBinding() error: %v", err)
	}
	if checkpoint.Completed[stepApplyRoutingCoverage] || checkpoint.Completed[stepValidateReadiness] {
		t.Fatalf("expected routing coverage and readiness to be rechecked, got %#v", checkpoint.Completed)
	}
	if !checkpoint.Completed[stepEnsureVersion] || !checkpoint.Completed[stepAttachBuild] {
		t.Fatalf("expected remotely verified steps to survive, got %#v", checkpoint.Completed)
	}
	if !strings.Contains(strings.Join(messages, "\n"), stepApplyRoutingCoverage) {
		t.Fatalf("expected routing coverage recheck diagnostic, got %v", messages)
	}
}
