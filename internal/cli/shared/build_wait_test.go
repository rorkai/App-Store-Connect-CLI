package shared

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type buildWaitRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildWaitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type transientBuildWaitError struct{}

func (transientBuildWaitError) Error() string   { return "temporary network failure" }
func (transientBuildWaitError) Timeout() bool   { return true }
func (transientBuildWaitError) Temporary() bool { return true }

func newBuildWaitTestClient(t *testing.T, transport buildWaitRoundTripFunc) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func newBuildWaitServerTestClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	httpClient := server.Client()
	serverTransport := httpClient.Transport
	httpClient.Transport = buildWaitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		routedReq := req.Clone(req.Context())
		routedURL := *req.URL
		routedURL.Scheme = serverURL.Scheme
		routedURL.Host = serverURL.Host
		routedReq.URL = &routedURL
		routedReq.Host = serverURL.Host
		return serverTransport.RoundTrip(routedReq)
	})

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func buildWaitJSONResponse(body string) (*http.Response, error) {
	return buildWaitJSONStatusResponse(http.StatusOK, body)
}

func buildWaitJSONStatusResponse(statusCode int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestWaitForBuildByNumberOrUploadFailureRejectsStaleBuildFromDifferentUpload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uploadCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			uploadCalls++
			return buildWaitJSONResponse(`{
				"data": {
					"type": "buildUploads",
					"id": "upload-current",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"state": {
							"state": "PROCESSING"
						}
					}
				}
			}`)
		case "/v1/preReleaseVersions":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2.3",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			if got := req.URL.Query().Get("include"); got != "buildUpload" {
				t.Fatalf("expected include=buildUpload when upload ID is known, got %q", got)
			}
			cancel()
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "stale-build",
						"attributes": {
							"version": "42",
							"uploadedDate": "2026-03-16T12:00:05Z"
						},
						"relationships": {
							"buildUpload": {
								"data": {
									"type": "buildUploads",
									"id": "stale-upload"
								}
							}
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	_, err := WaitForBuildByNumberOrUploadFailure(ctx, client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation after rejecting stale build, got %v", err)
	}
	if uploadCalls == 0 {
		t.Fatal("expected build upload lookup before accepting a discovered build")
	}
}

func TestWaitForBuildByNumberOrUploadFailureReturnsBuildLinkedFromUpload(t *testing.T) {
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONResponse(`{
				"data": {
					"type": "buildUploads",
					"id": "upload-current",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS"
					},
					"relationships": {
						"build": {
							"data": {
								"type": "builds",
								"id": "build-123"
							}
						}
					}
				}
			}`)
		case "/v1/builds/build-123":
			return buildWaitJSONResponse(`{
				"data": {
					"type": "builds",
					"id": "build-123",
					"attributes": {
						"version": "42",
						"processingState": "PROCESSING"
					}
				}
			}`)
		case "/v1/preReleaseVersions", "/v1/builds":
			t.Fatalf("did not expect build discovery list request once upload links a build: %s", req.URL.Path)
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	buildResp, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil {
		t.Fatal("expected linked build response")
		return
	}
	if buildResp.Data.ID != "build-123" {
		t.Fatalf("expected linked build ID build-123, got %q", buildResp.Data.ID)
	}
}

func TestWaitForBuildByNumberOrUploadFailureIncludesProcessingDiagnostics(t *testing.T) {
	restoreDiagnostics := SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		return `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`, nil
	})
	t.Cleanup(restoreDiagnostics)

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads/upload-current" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildWaitJSONResponse(`{
			"data": {
				"type": "buildUploads",
				"id": "upload-current",
				"attributes": {
					"cfBundleShortVersionString": "1.2.3",
					"cfBundleVersion": "42",
					"platform": "IOS",
					"state": {
						"state": "FAILED",
						"errors": [
							{"code": "90626"}
						]
					}
				}
			}
		}`)
	})

	_, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err == nil {
		t.Fatal("expected build upload failure, got nil")
	}
	if !strings.Contains(err.Error(), `build upload "upload-current" failed with state FAILED: 90626`) {
		t.Fatalf("expected original upload failure details, got %v", err)
	}
	if !strings.Contains(err.Error(), `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`) {
		t.Fatalf("expected enriched processing details, got %v", err)
	}
}

// builds upload --wait, publish --wait, and xcode export --wait wait on
// processing through this helper and must report the same processing details
// as builds wait, resolving the app from the build when the caller has none.
func TestWaitForBuildProcessingWithDetailsIncludesProcessingDetails(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	var diagnosedUploads []string
	t.Cleanup(SetBuildUploadFailureDiagnosticsForTesting(func(_ context.Context, _ *asc.Client, appID string, upload *asc.BuildUploadResponse) (string, error) {
		diagnosedUploads = append(diagnosedUploads, appID+"/"+upload.Data.ID)
		return "ITMS-90000: processing details", nil
	}))

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildWaitJSONResponse(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"FAILED"}}}`)
		case "/v1/builds/build-1/app":
			return buildWaitJSONResponse(`{"data":{"type":"apps","id":"app-1"}}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildWaitJSONResponse(`{"data":{"type":"preReleaseVersions","id":"pre-1","attributes":{"version":"1.2.3","platform":"IOS"}}}`)
		case "/v1/builds":
			return buildWaitJSONResponse(`{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42"},"relationships":{"buildUpload":{"data":{"type":"buildUploads","id":"upload-1"}}}}],"links":{}}`)
		case "/v1/buildUploads/upload-1":
			return buildWaitJSONResponse(`{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	build, err := WaitForBuildProcessingWithDetails(context.Background(), client, "", "build-1", time.Millisecond)
	if err == nil {
		t.Fatal("expected processing failure, got nil")
	}
	if want := "build processing failed: FAILED; App Store Connect processing details: ITMS-90000: processing details"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	if build == nil || build.Data.ID != "build-1" {
		t.Fatalf("build = %#v, want the failed build", build)
	}
	if want := []string{"app-1/upload-1"}; !slices.Equal(diagnosedUploads, want) {
		t.Fatalf("diagnosed uploads = %v, want %v", diagnosedUploads, want)
	}
}

func TestWaitForBuildProcessingWithDetailsKeepsStateErrorWhenLookupsFail(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnosticsCalls := 0
	t.Cleanup(SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		diagnosticsCalls++
		return "processing details", nil
	}))

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/builds/build-1" {
			return buildWaitJSONResponse(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"INVALID"}}}`)
		}
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"forbidden"}]}`)),
		}, nil
	})

	_, err := WaitForBuildProcessingWithDetails(context.Background(), client, "app-1", "build-1", time.Millisecond)
	if err == nil || err.Error() != "build processing failed: INVALID" {
		t.Fatalf("error = %v, want the unmodified state error", err)
	}
	if diagnosticsCalls != 0 {
		t.Fatalf("diagnostics lookups = %d, want 0", diagnosticsCalls)
	}
}

func TestWaitForBuildProcessingWithDetailsLeavesValidBuildUntouched(t *testing.T) {
	requests := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildWaitJSONResponse(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`)
	})

	build, err := WaitForBuildProcessingWithDetails(context.Background(), client, "app-1", "build-1", time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build == nil || build.Data.Attributes.ProcessingState != "VALID" || requests != 1 {
		t.Fatalf("build = %#v after %d requests, want one VALID lookup", build, requests)
	}
}

func TestEnrichBuildProcessingFailureSkipsLookupWithoutBundleVersion(t *testing.T) {
	diagnosticsCalls := 0
	t.Cleanup(SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		diagnosticsCalls++
		return "processing details", nil
	}))

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s", req.URL.Path)
	})

	baseErr := errors.New("build processing failed with state FAILED")
	err := EnrichBuildProcessingFailure(context.Background(), client, BuildProcessingFailureContext{AppID: "app-1"}, baseErr)
	if !errors.Is(err, baseErr) {
		t.Fatalf("error = %v, want the base error", err)
	}
	if got := err.Error(); got != baseErr.Error() {
		t.Fatalf("error = %q, want %q", got, baseErr.Error())
	}
	if diagnosticsCalls != 0 {
		t.Fatalf("diagnostics lookups = %d, want 0", diagnosticsCalls)
	}
}

func TestBuildUploadFailureErrorIncludesRecoveryGuidance(t *testing.T) {
	tests := []struct {
		name        string
		codes       []string
		message     string
		description string
		want        []string
	}{
		{
			name:  "closed version train",
			codes: []string{"90062", "90186", "90478"},
			want:  []string{"increase the marketing version", "CFBundleShortVersionString"},
		},
		{
			name:  "duplicate build number",
			codes: []string{"90189"},
			want:  []string{"increase the build number", "CFBundleVersion"},
		},
		{
			name:        "bundle identifier mismatch",
			codes:       []string{"90054", "90055"},
			description: "The bundle identifier does not match the selected app.",
			want:        []string{"bundle identifier", "selected app"},
		},
		{
			name:        "invalid build number format",
			codes:       []string{"90054"},
			description: "The value for CFBundleVersion must be a period-separated list of at most three non-negative integers.",
			want:        []string{"CFBundleVersion", "period-separated list"},
		},
		{
			name:        "missing privacy purpose string",
			codes:       []string{"90683"},
			message:     "Privacy validation failed.",
			description: "Missing Info.plist value. A value for NSCameraUsageDescription must be present.",
			want:        []string{"NSCameraUsageDescription", "Info.plist"},
		},
		{
			name:  "unsupported SDK",
			codes: []string{"90725"},
			want:  []string{"supported SDK", "toolchain"},
		},
		{
			name:  "missing background task identifiers",
			codes: []string{"90771"},
			want:  []string{"BGTaskSchedulerPermittedIdentifiers", "Info.plist"},
		},
		{
			name:  "missing app icons",
			codes: []string{"90391", "90713"},
			want:  []string{"required app icons", "CFBundleIconName"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := "FAILED"
			details := make([]asc.StateDetail, 0, len(tt.codes))
			for _, code := range tt.codes {
				details = append(details, asc.StateDetail{
					Code:        code,
					Description: tt.description,
					Message:     tt.message,
				})
			}
			upload := &asc.BuildUploadResponse{}
			upload.Data.ID = "upload-1"
			upload.Data.Attributes.State = &asc.AppMediaAssetState{State: &state, Errors: details}

			err := buildUploadFailureError(upload)
			if err == nil {
				t.Fatal("expected failure error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected %q in %q", want, err)
				}
			}
			for _, code := range tt.codes {
				if !strings.Contains(err.Error(), code) {
					t.Fatalf("expected original code %q in %q", code, err)
				}
			}
		})
	}
}

func TestBuildUploadFailureErrorCombinesIndependentVersionRecoveries(t *testing.T) {
	state := "FAILED"
	upload := &asc.BuildUploadResponse{}
	upload.Data.ID = "upload-1"
	upload.Data.Attributes.State = &asc.AppMediaAssetState{
		State: &state,
		Errors: []asc.StateDetail{
			{
				Code:        "90054",
				Description: "The value for CFBundleVersion must be a period-separated list of at most three non-negative integers.",
			},
			{
				Code:        "90055",
				Description: "The bundle identifier does not match the selected app.",
			},
		},
	}

	err := buildUploadFailureError(upload)
	if err == nil {
		t.Fatal("expected failure error")
	}
	for _, want := range []string{"CFBundleVersion", "period-separated list", "bundle identifier", "selected app"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in %q", want, err)
		}
	}
}

func TestBuildUploadFailureErrorRecognizesIndividualCodeFromFamily(t *testing.T) {
	for _, code := range []string{"90062", "90186", "90478"} {
		t.Run(code, func(t *testing.T) {
			state := "FAILED"
			upload := &asc.BuildUploadResponse{}
			upload.Data.ID = "upload-1"
			upload.Data.Attributes.State = &asc.AppMediaAssetState{
				State:  &state,
				Errors: []asc.StateDetail{{Code: code}},
			}

			err := buildUploadFailureError(upload)
			if err == nil || !strings.Contains(err.Error(), "increase the marketing version") {
				t.Fatalf("expected closed-version guidance for %s, got %v", code, err)
			}
		})
	}
}

func TestBuildUploadFailureErrorLeavesUnknownFailuresUnchanged(t *testing.T) {
	state := "FAILED"
	upload := &asc.BuildUploadResponse{}
	upload.Data.ID = "upload-1"
	upload.Data.Attributes.State = &asc.AppMediaAssetState{
		State:  &state,
		Errors: []asc.StateDetail{{Code: "UNKNOWN", Description: "Server-provided detail", Message: "Server-provided detail"}},
	}

	err := buildUploadFailureError(upload)
	if err == nil {
		t.Fatal("expected failure error")
	}
	want := `build upload "upload-1" failed with state FAILED: UNKNOWN (Server-provided detail)`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestBuildUploadFailureErrorPreservesDescriptionWhenMessageIsEmpty(t *testing.T) {
	state := "FAILED"
	upload := &asc.BuildUploadResponse{}
	upload.Data.ID = "upload-1"
	upload.Data.Attributes.State = &asc.AppMediaAssetState{
		State:  &state,
		Errors: []asc.StateDetail{{Code: "90054", Description: "raw App Store Connect description"}},
	}

	err := buildUploadFailureError(upload)
	if err == nil {
		t.Fatal("expected failure error")
	}
	if !strings.Contains(err.Error(), "raw App Store Connect description") {
		t.Fatalf("expected raw description in %q", err)
	}
}

func TestBuildUploadFailureErrorDoesNotGuessForMixedCodes(t *testing.T) {
	state := "FAILED"
	upload := &asc.BuildUploadResponse{}
	upload.Data.ID = "upload-1"
	upload.Data.Attributes.State = &asc.AppMediaAssetState{
		State: &state,
		Errors: []asc.StateDetail{
			{Code: "90189"},
			{Code: "UNKNOWN"},
		},
	}

	err := buildUploadFailureError(upload)
	if err == nil {
		t.Fatal("expected failure error")
	}
	if strings.Contains(err.Error(), "recovery:") {
		t.Fatalf("mixed errors must not receive speculative guidance: %v", err)
	}
}

func TestWaitForBuildByNumberOrUploadFailureFallsBackWhenUploadLookupFails(t *testing.T) {
	// Build Upload API 404s are retried; exhaust the budget quickly so the
	// fallback is exercised behind the retry wrappers without real backoff.
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONStatusResponse(http.StatusNotFound, `{
				"errors": [
					{"status": "404", "code": "NOT_FOUND", "title": "not found"}
				]
			}`)
		case "/v1/preReleaseVersions":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2.3",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "build-123",
						"attributes": {
							"version": "42",
							"processingState": "PROCESSING"
						},
						"relationships": {
							"buildUpload": {
								"data": {
									"type": "buildUploads",
									"id": "upload-current"
								}
							}
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	buildResp, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil {
		t.Fatal("expected build response after falling back to build discovery")
		return
	}
	if buildResp.Data.ID != "build-123" {
		t.Fatalf("expected build ID build-123, got %q", buildResp.Data.ID)
	}
}

func TestWaitForBuildByNumberOrUploadFailureStopsAfterExhaustedBuildUploadNotFound(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	uploadCalls := 0
	fallbackCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			uploadCalls++
			return buildWaitJSONStatusResponse(http.StatusNotFound, `{
				"errors": [{
					"status": "404",
					"code": "NOT_FOUND",
					"title": "The specified resource does not exist",
					"detail": "There is no resource of type 'apps' with id 'app-1'"
				}]
			}`)
		case "/v1/preReleaseVersions":
			fallbackCalls++
			return buildWaitJSONResponse(`{"data":[],"links":{}}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err := WaitForBuildByNumberOrUploadFailure(ctx, client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err == nil {
		t.Fatal("expected exhausted build-upload lookup error")
	}
	if uploadCalls != 2 {
		t.Fatalf("build-upload lookup made %d requests, want one attempt plus one retry", uploadCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("build discovery replayed %d times after terminal upload lookup", fallbackCalls)
	}
	if !asc.IsNotFound(err) || !asc.IsRetryBudgetExhausted(err) {
		t.Fatalf("error = %v, want exhausted NOT_FOUND", err)
	}
	if !strings.Contains(err.Error(), "There is no resource of type 'apps' with id 'app-1'") {
		t.Fatalf("error lost Apple's detail: %v", err)
	}
}

func TestWaitForBuildByNumberOrUploadFailureFallsBackWhenLinkedBuildLookupFails(t *testing.T) {
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONResponse(`{
				"data": {
					"type": "buildUploads",
					"id": "upload-current",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"state": {
							"state": "PROCESSING"
						}
					},
					"relationships": {
						"build": {
							"data": {
								"type": "builds",
								"id": "build-123"
							}
						}
					}
				}
			}`)
		case "/v1/builds/build-123":
			return buildWaitJSONStatusResponse(http.StatusNotFound, `{
				"errors": [
					{"status": "404", "code": "NOT_FOUND", "title": "not found"}
				]
			}`)
		case "/v1/preReleaseVersions":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2.3",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "build-123",
						"attributes": {
							"version": "42",
							"processingState": "PROCESSING"
						},
						"relationships": {
							"buildUpload": {
								"data": {
									"type": "buildUploads",
									"id": "upload-current"
								}
							}
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	buildResp, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil {
		t.Fatal("expected build response after falling back from linked build lookup")
		return
	}
	if buildResp.Data.ID != "build-123" {
		t.Fatalf("expected build ID build-123, got %q", buildResp.Data.ID)
	}
}

func TestWaitForBuildByNumberOrUploadFailureReturnsUploadLookupErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONStatusResponse(http.StatusUnauthorized, `{
				"errors": [
					{"status": "401", "code": "UNAUTHORIZED", "title": "unauthorized"}
				]
			}`)
		case "/v1/preReleaseVersions":
			cancel()
			return buildWaitJSONResponse(`{"data":[]}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	_, err := WaitForBuildByNumberOrUploadFailure(ctx, client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err == nil {
		t.Fatal("expected upload lookup error, got nil")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized upload lookup error, got %v", err)
	}
}

func TestWaitForBuildByNumberOrUploadFailureReturnsMalformedUploadRelationships(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONResponse(`{
				"data": {
					"type": "buildUploads",
					"id": "upload-current",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"state": {
							"state": "PROCESSING"
						}
					},
					"relationships": {
						"build": "bad-shape"
					}
				}
			}`)
		case "/v1/preReleaseVersions":
			cancel()
			return buildWaitJSONResponse(`{"data":[]}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	_, err := WaitForBuildByNumberOrUploadFailure(ctx, client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err == nil {
		t.Fatal("expected malformed relationship error, got nil")
	}
	if !strings.Contains(err.Error(), `parse build upload "upload-current" relationships`) {
		t.Fatalf("expected malformed relationship error, got %v", err)
	}
}

func TestWaitForBuildByNumberOrUploadFailureToleratesTransientLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	preReleaseCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			preReleaseCalls++
			if preReleaseCalls <= 2 {
				return buildWaitJSONStatusResponse(http.StatusServiceUnavailable, `{
					"errors": [
						{"status": "503", "code": "SERVICE_UNAVAILABLE", "title": "unavailable"}
					]
				}`)
			}
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2.3",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "build-123",
						"attributes": {
							"version": "42",
							"processingState": "PROCESSING"
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureStderr(t, func() {
		buildResp, err = WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "", "1.2.3", "42", "IOS", time.Millisecond)
	})
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil {
		t.Fatal("expected build response after tolerating transient failures")
		return
	}
	if buildResp.Data.ID != "build-123" {
		t.Fatalf("expected build ID build-123, got %q", buildResp.Data.ID)
	}
	for _, want := range []string{
		"transient App Store Connect error while waiting (1/5)",
		"transient App Store Connect error while waiting (2/5)",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestWaitForBuildByNumberOrUploadFailureFailsAfterConsecutiveTransientLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	preReleaseCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/preReleaseVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		preReleaseCalls++
		return buildWaitJSONStatusResponse(http.StatusServiceUnavailable, `{
			"errors": [
				{"status": "503", "code": "SERVICE_UNAVAILABLE", "title": "unavailable"}
			]
		}`)
	})

	var err error
	captureStderr(t, func() {
		_, err = WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "", "1.2.3", "42", "IOS", time.Millisecond)
	})
	if err == nil {
		t.Fatal("expected error once transient failures exceed the ceiling, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 6 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive transient failure error, got %v", err)
	}
	if preReleaseCalls != asc.DefaultMaxConsecutivePollFailures+1 {
		t.Fatalf("expected %d lookups, got %d", asc.DefaultMaxConsecutivePollFailures+1, preReleaseCalls)
	}
}

func TestWaitForBuildByNumberOrUploadFailureMatchesEquivalentVersionFormat(t *testing.T) {
	resetEquivalentVersionNotes()

	var versionFilters []string
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			requested := req.URL.Query().Get("filter[version]")
			versionFilters = append(versionFilters, requested)
			if requested != "1.2" {
				return buildWaitJSONResponse(`{"data": [], "links": {}}`)
			}
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "build-123",
						"attributes": {
							"version": "42",
							"processingState": "PROCESSING"
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureStderr(t, func() {
		buildResp, err = WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "", "1.2.0", "42", "IOS", time.Millisecond)
	})
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil {
		t.Fatal("expected build response for equivalent version format")
		return
	}
	if buildResp.Data.ID != "build-123" {
		t.Fatalf("expected build ID build-123, got %q", buildResp.Data.ID)
	}
	if len(versionFilters) != 2 || versionFilters[0] != "1.2.0" || versionFilters[1] != "1.2" {
		t.Fatalf("expected requested format to be queried before the equivalent form, got %v", versionFilters)
	}
	if !strings.Contains(stderr, `note: matched version "1.2" for requested "1.2.0"`) {
		t.Fatalf("expected equivalent version note, got %q", stderr)
	}
}

func TestWaitForBuildByNumberOrUploadFailureFiltersNearMatchesAcrossPages(t *testing.T) {
	resetEquivalentVersionNotes()

	var buildFilters []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", req.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}

		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			if got := req.URL.Query().Get("filter[version]"); got != "1.2.0" && req.URL.Query().Get("cursor") == "" {
				t.Errorf("filter[version] = %q, want 1.2.0", got)
			}
			if req.URL.Query().Get("cursor") == "page-2" {
				_, _ = io.WriteString(w, `{
					"data": [
						{
							"type": "preReleaseVersions",
							"id": "prv-exact",
							"attributes": {"version": "1.2.0", "platform": "IOS"}
						}
					],
					"links": {}
				}`)
				return
			}
			_, _ = io.WriteString(w, `{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-near-match",
						"attributes": {"version": "1.2", "platform": "IOS"}
					}
				],
				"links": {"next": "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=page-2"}
			}`)
		case "/v1/builds":
			buildFilters = append(buildFilters, req.URL.Query().Get("filter[preReleaseVersion]"))
			_, _ = io.WriteString(w, `{
				"data": [
					{
						"type": "builds",
						"id": "build-exact",
						"attributes": {"version": "42", "processingState": "PROCESSING"}
					}
				],
				"links": {}
			}`)
		default:
			t.Errorf("unexpected path: %s", req.URL.Path)
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)
	client := newBuildWaitServerTestClient(t, server)

	buildResp, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "", "1.2.0", "42", "IOS", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.ID != "build-exact" {
		t.Fatalf("expected build-exact, got %#v", buildResp)
	}
	if len(buildFilters) != 1 || buildFilters[0] != "prv-exact" {
		t.Fatalf("expected exact pre-release version filter, got %v", buildFilters)
	}
}

func TestWaitForBuildByNumberOrUploadFailurePrefersRequestedVersionFormat(t *testing.T) {
	resetEquivalentVersionNotes()

	var versionFilters []string
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			requested := req.URL.Query().Get("filter[version]")
			versionFilters = append(versionFilters, requested)
			if requested != "1.2.0" {
				return buildWaitJSONResponse(`{"data": [], "links": {}}`)
			}
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2.0",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "build-123",
						"attributes": {
							"version": "42",
							"processingState": "PROCESSING"
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureStderr(t, func() {
		buildResp, err = WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "", "1.2.0", "42", "IOS", time.Millisecond)
	})
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.ID != "build-123" {
		t.Fatalf("expected build-123, got %#v", buildResp)
	}
	if len(versionFilters) != 1 || versionFilters[0] != "1.2.0" {
		t.Fatalf("expected only the requested format to be queried, got %v", versionFilters)
	}
	if strings.Contains(stderr, "note: matched version") {
		t.Fatalf("did not expect an equivalent version note, got %q", stderr)
	}
}

func TestVerifyBuildUploadAfterCommitIgnoresRetryableLookupErrorsUntilBuildLinks(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	lookupCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads/upload-current" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		lookupCalls++
		if lookupCalls == 1 {
			return nil, transientBuildWaitError{}
		}
		return buildWaitJSONResponse(`{
			"data": {
				"type": "buildUploads",
				"id": "upload-current",
				"attributes": {
					"cfBundleShortVersionString": "1.2.3",
					"cfBundleVersion": "42",
					"platform": "IOS",
					"state": {"state": "UPLOADED"}
				},
				"relationships": {
					"build": {
						"data": {
							"type": "builds",
							"id": "build-123"
						}
					}
				}
			}
		}`)
	})

	_, err := VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("VerifyBuildUploadAfterCommit() error: %v", err)
	}
	if lookupCalls < 2 {
		t.Fatalf("expected retryable lookup error to be retried, got %d lookup(s)", lookupCalls)
	}
}

func TestVerifyBuildUploadAfterCommitIgnoresRetryDelayBeyondVerificationBudget(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "5s")
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	lookupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads/upload-current" {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		lookupCalls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{
			"errors": [{"status": "429", "code": "RATE_LIMIT_EXCEEDED", "title": "Too many requests"}]
		}`)
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		redirected := req.Clone(req.Context())
		redirected.URL.Scheme = serverURL.Scheme
		redirected.URL.Host = serverURL.Host
		redirected.Host = serverURL.Host
		return server.Client().Do(redirected)
	})

	verifyTimeout := 30 * time.Millisecond
	_, err = VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, verifyTimeout)
	if err != nil {
		t.Fatalf("VerifyBuildUploadAfterCommit() error: %v", err)
	}
	if lookupCalls != 1 {
		t.Fatalf("expected one best-effort upload lookup before honoring Retry-After, got %d", lookupCalls)
	}
}

func TestVerifyBuildUploadAfterCommitStopsAfterExhaustedBuildUploadNotFound(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	lookupCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/buildUploads/upload-current" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		lookupCalls++
		return buildWaitJSONStatusResponse(http.StatusNotFound, `{
			"errors": [{
				"status": "404",
				"code": "NOT_FOUND",
				"title": "The specified resource does not exist",
				"detail": "There is no resource of type 'apps' with id 'app-1'"
			}]
		}`)
	})

	verifyTimeout := 150 * time.Millisecond
	started := time.Now()
	_, err := VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, verifyTimeout)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("VerifyBuildUploadAfterCommit() error: %v", err)
	}
	if lookupCalls != 2 {
		t.Fatalf("build-upload verification made %d requests, want one attempt plus one retry", lookupCalls)
	}
	if elapsed >= verifyTimeout/2 {
		t.Fatalf("verification replayed a terminal lookup for %v (budget %v)", elapsed, verifyTimeout)
	}
}

func TestResolveBuildStatusBundleIDReturnsAppBundleIDWhenSupported(t *testing.T) {
	previous := buildStatusBundleIDSupportedFn
	buildStatusBundleIDSupportedFn = func(context.Context) bool { return true }
	t.Cleanup(func() {
		buildStatusBundleIDSupportedFn = previous
	})

	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildWaitJSONResponse(`{
			"data": {
				"type": "apps",
				"id": "app-1",
				"attributes": {
					"name": "Demo",
					"bundleId": "com.example.demo",
					"sku": "demo"
				}
			}
		}`)
	})

	bundleID := resolveBuildStatusBundleID(context.Background(), client, "app-1")
	if bundleID != "com.example.demo" {
		t.Fatalf("expected resolved bundle ID com.example.demo, got %q", bundleID)
	}
}

func TestBuildStatusPrivateKeyPathFallsBackToStoredPEMWhenPathMissing(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	CleanupTempPrivateKeys()
	t.Cleanup(CleanupTempPrivateKeys)

	resolvedPath, err := buildStatusPrivateKeyPath(ResolvedAuthCredentials{
		KeyPath: filepath.Join(tempDir, "missing.p8"),
		KeyPEM:  string(keyData),
	})
	if err != nil {
		t.Fatalf("buildStatusPrivateKeyPath() error: %v", err)
	}
	if resolvedPath == filepath.Join(tempDir, "missing.p8") {
		t.Fatalf("expected fallback temp path, got missing configured path %q", resolvedPath)
	}
	if _, err := os.Stat(resolvedPath); err != nil {
		t.Fatalf("Stat(%q) error: %v", resolvedPath, err)
	}
	if _, err := asc.NewClient("KEY123", "ISS456", resolvedPath); err != nil {
		t.Fatalf("expected fallback private key path to be usable, got %v", err)
	}
}

func TestBuildStatusPrivateKeyPathPrefersStoredPEMOverExistingKeyPath(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey-stale.p8")
	if err := os.WriteFile(keyPath, []byte("stale-key"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	validKeyPath := filepath.Join(tempDir, "AuthKey-valid.p8")
	writeECDSAPEM(t, validKeyPath)

	keyData, err := os.ReadFile(validKeyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	CleanupTempPrivateKeys()
	t.Cleanup(CleanupTempPrivateKeys)

	resolvedPath, err := buildStatusPrivateKeyPath(ResolvedAuthCredentials{
		KeyPath: keyPath,
		KeyPEM:  string(keyData),
	})
	if err != nil {
		t.Fatalf("buildStatusPrivateKeyPath() error: %v", err)
	}
	if resolvedPath == keyPath {
		t.Fatalf("expected PEM-backed temp path instead of configured key path %q", resolvedPath)
	}
	if _, err := asc.NewClient("KEY123", "ISS456", resolvedPath); err != nil {
		t.Fatalf("expected PEM-backed fallback path to be usable, got %v", err)
	}
}

func TestBuildStatusPrivateKeyPathDecodesStoredBase64PEM(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey-valid.p8")
	writeECDSAPEM(t, keyPath)

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	CleanupTempPrivateKeys()
	t.Cleanup(CleanupTempPrivateKeys)

	resolvedPath, err := buildStatusPrivateKeyPath(ResolvedAuthCredentials{
		KeyPEM: base64.StdEncoding.EncodeToString(keyData),
	})
	if err != nil {
		t.Fatalf("buildStatusPrivateKeyPath() error: %v", err)
	}
	if resolvedPath == "" {
		t.Fatal("expected decoded temp key path, got empty path")
	}
	resolvedData, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", resolvedPath, err)
	}
	if string(resolvedData) != string(keyData) {
		t.Fatalf("expected decoded PEM data, got %q", string(resolvedData))
	}
	if _, err := asc.NewClient("KEY123", "ISS456", resolvedPath); err != nil {
		t.Fatalf("expected base64-decoded private key path to be usable, got %v", err)
	}
}
