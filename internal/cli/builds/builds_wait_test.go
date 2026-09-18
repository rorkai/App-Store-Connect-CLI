package builds

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type buildsWaitRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildsWaitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newBuildsWaitTestClient(t *testing.T, transport buildsWaitRoundTripFunc) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if data == nil {
		t.Fatal("failed to encode PEM")
	}
	keyPath := filepath.Join(t.TempDir(), "key.p8")
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func buildsWaitJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func buildsWaitServiceUnavailable() (*http.Response, error) {
	return buildsWaitJSONResponse(http.StatusServiceUnavailable, `{
		"errors": [
			{"status": "503", "code": "SERVICE_UNAVAILABLE", "title": "unavailable"}
		]
	}`)
}

func captureBuildsWaitStderr(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stderr = writer

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		_ = reader.Close()
		done <- buf.String()
	}()

	fn()

	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close error: %v", closeErr)
	}
	os.Stderr = orig

	return <-done
}

func TestWaitForBuildProcessingStateToleratesTransientLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		if calls <= 2 {
			return buildsWaitServiceUnavailable()
		}
		return buildsWaitJSONResponse(http.StatusOK, `{
			"data": {
				"type": "builds",
				"id": "build-1",
				"attributes": {"version": "42", "processingState": "VALID"}
			}
		}`)
	})

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureBuildsWaitStderr(t, func() {
		buildResp, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, shared.BuildProcessingFailureContext{})
	})
	if err != nil {
		t.Fatalf("waitForBuildProcessingState() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.Attributes.ProcessingState != asc.BuildProcessingStateValid {
		t.Fatalf("expected VALID build after transient failures, got %#v", buildResp)
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

func TestWaitForBuildProcessingStateFailsAfterConsecutiveTransientLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildsWaitServiceUnavailable()
	})

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, shared.BuildProcessingFailureContext{})
	})
	if err == nil {
		t.Fatal("expected error once transient failures exceed the ceiling, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 6 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive transient failure error, got %v", err)
	}
	if calls != asc.DefaultMaxConsecutivePollFailures+1 {
		t.Fatalf("expected %d lookups, got %d", asc.DefaultMaxConsecutivePollFailures+1, calls)
	}
}

func TestWaitForBuildProcessingStateReturnsTerminalFailure(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/app", "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusNotFound, buildsWaitNotFoundBody)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, shared.BuildProcessingFailureContext{})
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if !strings.Contains(err.Error(), "build processing failed with state FAILED") {
		t.Fatalf("expected terminal FAILED error, got %v", err)
	}
}

const buildsWaitNotFoundBody = `{
	"errors": [
		{"status": "404", "code": "NOT_FOUND", "title": "The specified resource does not exist"}
	]
}`

const buildsWaitProcessingDetail = `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`

// buildsWaitNoLinkedUploadsBody answers the build lookup that resolves which
// upload a build came from, reporting no linkage.
const buildsWaitNoLinkedUploadsBody = `{"data": [], "links": {}}`

const buildsWaitPreReleaseVersionBody = `{
	"data": {
		"type": "preReleaseVersions",
		"id": "pre-1",
		"attributes": {"version": "1.2.3", "platform": "IOS"}
	}
}`

type buildsWaitDiagnosticsStub struct {
	calls     int
	appIDs    []string
	uploadIDs []string
}

// stubBuildsWaitProcessingDetails records diagnostics lookups so tests can
// prove when, and for which upload, processing details are fetched.
func stubBuildsWaitProcessingDetails(t *testing.T, details string, lookupErr error) *buildsWaitDiagnosticsStub {
	t.Helper()

	stub := &buildsWaitDiagnosticsStub{}
	t.Cleanup(shared.SetBuildUploadFailureDiagnosticsForTesting(func(_ context.Context, _ *asc.Client, appID string, upload *asc.BuildUploadResponse) (string, error) {
		stub.calls++
		stub.appIDs = append(stub.appIDs, appID)
		if upload != nil {
			stub.uploadIDs = append(stub.uploadIDs, upload.Data.ID)
		}
		return details, lookupErr
	}))
	return stub
}

func TestWaitForBuildProcessingStateFailureIncludesProcessingDetails(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	var uploadQuery url.Values
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/app":
			return buildsWaitJSONResponse(http.StatusOK, `{"data": {"type": "apps", "id": "app-1"}}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "preReleaseVersions",
					"id": "pre-1",
					"attributes": {"version": "1.2.3", "platform": "IOS"}
				}
			}`)
		case "/v1/builds":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitNoLinkedUploadsBody)
		case "/v1/apps/app-1/buildUploads":
			uploadQuery = req.URL.Query()
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "buildUploads",
						"id": "upload-1",
						"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, shared.BuildProcessingFailureContext{})
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	message := err.Error()
	if !strings.Contains(message, "build processing failed with state FAILED") {
		t.Fatalf("expected the state error to be preserved, got %v", err)
	}
	if !strings.Contains(message, "App Store Connect processing details: "+buildsWaitProcessingDetail) {
		t.Fatalf("expected processing details suffix, got %v", err)
	}
	if diagnostics.calls != 1 {
		t.Fatalf("diagnostics lookups = %d, want 1", diagnostics.calls)
	}
	if want := []string{"app-1"}; !slices.Equal(diagnostics.appIDs, want) {
		t.Fatalf("diagnostics app IDs = %v, want %v", diagnostics.appIDs, want)
	}
	if want := []string{"upload-1"}; !slices.Equal(diagnostics.uploadIDs, want) {
		t.Fatalf("diagnostics upload IDs = %v, want %v", diagnostics.uploadIDs, want)
	}
	for key, want := range map[string]string{
		"filter[cfBundleVersion]":            "42",
		"filter[cfBundleShortVersionString]": "1.2.3",
		"filter[platform]":                   "IOS",
	} {
		if got := uploadQuery.Get(key); got != want {
			t.Fatalf("build uploads %s = %q, want %q", key, got, want)
		}
	}
}

// Retrying the same build number produces several uploads with identical
// versions, so the upload linked to the waited build must win.
func TestWaitForBuildProcessingStateFailurePrefersUploadLinkedToBuild(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	var buildsQuery url.Values
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitPreReleaseVersionBody)
		case "/v1/builds":
			buildsQuery = req.URL.Query()
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "builds",
						"id": "build-retry",
						"attributes": {"version": "42"},
						"relationships": {"buildUpload": {"data": {"type": "buildUploads", "id": "upload-retry"}}}
					},
					{
						"type": "builds",
						"id": "build-1",
						"attributes": {"version": "42"},
						"relationships": {"buildUpload": {"data": {"type": "buildUploads", "id": "upload-waited"}}}
					}
				],
				"links": {}
			}`)
		case "/v1/buildUploads/upload-waited":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "buildUploads",
					"id": "upload-waited",
					"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	failureContext := shared.BuildProcessingFailureContext{
		AppID:        "app-1",
		ShortVersion: "1.2.3",
		Platform:     "IOS",
	}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, failureContext)
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if !strings.Contains(err.Error(), "App Store Connect processing details: "+buildsWaitProcessingDetail) {
		t.Fatalf("expected processing details suffix, got %v", err)
	}
	if want := []string{"upload-waited"}; !slices.Equal(diagnostics.uploadIDs, want) {
		t.Fatalf("diagnostics upload IDs = %v, want %v", diagnostics.uploadIDs, want)
	}
	for key, want := range map[string]string{
		"filter[app]":     "app-1",
		"filter[version]": "42",
		"include":         "buildUpload",
	} {
		if got := buildsQuery.Get(key); got != want {
			t.Fatalf("builds lookup %s = %q, want %q", key, got, want)
		}
	}
}

func TestWaitForBuildProcessingStateFailureFallsBackWhenLinkedUploadLookupFails(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitPreReleaseVersionBody)
		case "/v1/builds":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "builds",
						"id": "build-1",
						"attributes": {"version": "42"},
						"relationships": {"buildUpload": {"data": {"type": "buildUploads", "id": "upload-missing"}}}
					}
				],
				"links": {}
			}`)
		case "/v1/buildUploads/upload-missing":
			return buildsWaitJSONResponse(http.StatusNotFound, buildsWaitNotFoundBody)
		case "/v1/apps/app-1/buildUploads":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "buildUploads",
						"id": "upload-1",
						"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	failureContext := shared.BuildProcessingFailureContext{AppID: "app-1"}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, failureContext)
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if !strings.Contains(err.Error(), "App Store Connect processing details: "+buildsWaitProcessingDetail) {
		t.Fatalf("expected processing details suffix, got %v", err)
	}
	if want := []string{"upload-1"}; !slices.Equal(diagnostics.uploadIDs, want) {
		t.Fatalf("diagnostics upload IDs = %v, want %v", diagnostics.uploadIDs, want)
	}
}

// Processing details are reported per app, build number, marketing version,
// and platform, so an upload must not be guessed from an incomplete identity.
func TestWaitForBuildProcessingStateFailureSkipsUploadMatchWithUnknownMarketingVersion(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusNotFound, buildsWaitNotFoundBody)
		case "/v1/builds":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitNoLinkedUploadsBody)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	failureContext := shared.BuildProcessingFailureContext{AppID: "app-1"}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, failureContext)
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if got := err.Error(); got != "build processing failed with state FAILED" {
		t.Fatalf("error = %q, want the unmodified state error", got)
	}
	if diagnostics.calls != 0 {
		t.Fatalf("diagnostics lookups = %d, want 0", diagnostics.calls)
	}
}

func TestWaitForBuildProcessingStateFailureSkipsAppLookupWhenSelectorProvidesApp(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitPreReleaseVersionBody)
		case "/v1/builds":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitNoLinkedUploadsBody)
		case "/v1/apps/app-1/buildUploads":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "buildUploads",
						"id": "upload-1",
						"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	failureContext := shared.BuildProcessingFailureContext{
		AppID:        "app-1",
		ShortVersion: "1.2.3",
		Platform:     "IOS",
	}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, failureContext)
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if !strings.Contains(err.Error(), "App Store Connect processing details: "+buildsWaitProcessingDetail) {
		t.Fatalf("expected processing details suffix, got %v", err)
	}
	if diagnostics.calls != 1 {
		t.Fatalf("diagnostics lookups = %d, want 1", diagnostics.calls)
	}
}

// Build numbers repeat across platforms, and App Store Connect stores only the
// uploaded spelling of a marketing version, so the build's own pre-release
// version and platform must drive the upload match.
func TestWaitForBuildProcessingStateFailurePrefersBuildReportedVersionAndPlatform(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	var uploadQuery url.Values
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "FAILED"}
				}
			}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "preReleaseVersions",
					"id": "pre-1",
					"attributes": {"version": "1.2", "platform": "MAC_OS"}
				}
			}`)
		case "/v1/builds":
			return buildsWaitJSONResponse(http.StatusOK, buildsWaitNoLinkedUploadsBody)
		case "/v1/apps/app-1/buildUploads":
			uploadQuery = req.URL.Query()
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "buildUploads",
						"id": "upload-1",
						"attributes": {"cfBundleShortVersionString": "1.2", "cfBundleVersion": "42", "platform": "MAC_OS"}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	failureContext := shared.BuildProcessingFailureContext{
		AppID:        "app-1",
		ShortVersion: "1.2.0",
	}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, failureContext)
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if !strings.Contains(err.Error(), "App Store Connect processing details: "+buildsWaitProcessingDetail) {
		t.Fatalf("expected processing details suffix, got %v", err)
	}
	for key, want := range map[string]string{
		"filter[cfBundleShortVersionString]": "1.2",
		"filter[platform]":                   "MAC_OS",
	} {
		if got := uploadQuery.Get(key); got != want {
			t.Fatalf("build uploads %s = %q, want %q", key, got, want)
		}
	}
	if diagnostics.calls != 1 {
		t.Fatalf("diagnostics lookups = %d, want 1", diagnostics.calls)
	}
}

func TestWaitForBuildProcessingStateFailureKeepsStateErrorWhenDetailsUnavailable(t *testing.T) {
	tests := []struct {
		name         string
		uploadsBody  string
		uploadStatus int
		details      string
		lookupErr    error
		wantCalls    int
	}{
		{
			name:         "no matching upload",
			uploadStatus: http.StatusOK,
			uploadsBody:  `{"data": [], "links": {}}`,
		},
		{
			name:         "upload lookup fails",
			uploadStatus: http.StatusNotFound,
			uploadsBody:  buildsWaitNotFoundBody,
		},
		{
			name:         "diagnostics lookup fails",
			uploadStatus: http.StatusOK,
			uploadsBody: `{
				"data": [
					{
						"type": "buildUploads",
						"id": "upload-1",
						"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
					}
				],
				"links": {}
			}`,
			lookupErr: errors.New("processing details unavailable"),
			wantCalls: 1,
		},
		{
			name:         "details already reported",
			uploadStatus: http.StatusOK,
			uploadsBody: `{
				"data": [
					{
						"type": "buildUploads",
						"id": "upload-1",
						"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
					}
				],
				"links": {}
			}`,
			details:   "build processing failed with state FAILED",
			wantCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "0")

			diagnostics := stubBuildsWaitProcessingDetails(t, test.details, test.lookupErr)

			client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/builds/build-1":
					return buildsWaitJSONResponse(http.StatusOK, `{
						"data": {
							"type": "builds",
							"id": "build-1",
							"attributes": {"version": "42", "processingState": "FAILED"}
						}
					}`)
				case "/v1/builds/build-1/preReleaseVersion":
					return buildsWaitJSONResponse(http.StatusOK, buildsWaitPreReleaseVersionBody)
				case "/v1/builds":
					return buildsWaitJSONResponse(http.StatusOK, buildsWaitNoLinkedUploadsBody)
				case "/v1/apps/app-1/buildUploads":
					return buildsWaitJSONResponse(test.uploadStatus, test.uploadsBody)
				default:
					return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
				}
			})

			failureContext := shared.BuildProcessingFailureContext{
				AppID:        "app-1",
				ShortVersion: "1.2.3",
				Platform:     "IOS",
			}

			var err error
			captureBuildsWaitStderr(t, func() {
				_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false, failureContext)
			})
			if err == nil {
				t.Fatal("expected terminal FAILED error, got nil")
			}
			if got := err.Error(); got != "build processing failed with state FAILED" {
				t.Fatalf("error = %q, want the unmodified state error", got)
			}
			if diagnostics.calls != test.wantCalls {
				t.Fatalf("diagnostics lookups = %d, want %d", diagnostics.calls, test.wantCalls)
			}
		})
	}
}

func TestWaitForBuildProcessingStateInvalidFetchesDetailsOnlyWhenFailing(t *testing.T) {
	tests := []struct {
		name          string
		failOnInvalid bool
		wantErr       bool
	}{
		{name: "invalid tolerated"},
		{name: "invalid fails", failOnInvalid: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "0")

			diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

			client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/builds/build-1":
					return buildsWaitJSONResponse(http.StatusOK, `{
						"data": {
							"type": "builds",
							"id": "build-1",
							"attributes": {"version": "42", "processingState": "INVALID"}
						}
					}`)
				case "/v1/builds/build-1/preReleaseVersion":
					if !test.failOnInvalid {
						t.Errorf("unexpected pre-release version lookup for a tolerated INVALID build")
					}
					return buildsWaitJSONResponse(http.StatusOK, buildsWaitPreReleaseVersionBody)
				case "/v1/builds":
					if !test.failOnInvalid {
						t.Errorf("unexpected upload linkage lookup for a tolerated INVALID build")
					}
					return buildsWaitJSONResponse(http.StatusOK, buildsWaitNoLinkedUploadsBody)
				case "/v1/apps/app-1/buildUploads":
					if !test.failOnInvalid {
						t.Errorf("unexpected build upload lookup for a tolerated INVALID build")
					}
					return buildsWaitJSONResponse(http.StatusOK, `{
						"data": [
							{
								"type": "buildUploads",
								"id": "upload-1",
								"attributes": {"cfBundleShortVersionString": "1.2.3", "cfBundleVersion": "42", "platform": "IOS"}
							}
						],
						"links": {}
					}`)
				default:
					return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
				}
			})

			failureContext := shared.BuildProcessingFailureContext{
				AppID:        "app-1",
				ShortVersion: "1.2.3",
				Platform:     "IOS",
			}

			var buildResp *asc.BuildResponse
			var err error
			captureBuildsWaitStderr(t, func() {
				buildResp, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, test.failOnInvalid, failureContext)
			})

			if !test.wantErr {
				if err != nil {
					t.Fatalf("waitForBuildProcessingState() error: %v", err)
				}
				if buildResp == nil || buildResp.Data.Attributes.ProcessingState != asc.BuildProcessingStateInvalid {
					t.Fatalf("expected the INVALID build to be returned, got %#v", buildResp)
				}
				if diagnostics.calls != 0 {
					t.Fatalf("diagnostics lookups = %d, want 0", diagnostics.calls)
				}
				return
			}

			if err == nil {
				t.Fatal("expected terminal INVALID error, got nil")
			}
			message := err.Error()
			if !strings.Contains(message, "build processing failed with state INVALID") {
				t.Fatalf("expected the state error to be preserved, got %v", err)
			}
			if !strings.Contains(message, "App Store Connect processing details: "+buildsWaitProcessingDetail) {
				t.Fatalf("expected processing details suffix, got %v", err)
			}
			if diagnostics.calls != 1 {
				t.Fatalf("diagnostics lookups = %d, want 1", diagnostics.calls)
			}
		})
	}
}

func TestWaitForBuildProcessingStateValidSkipsProcessingDetails(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	diagnostics := stubBuildsWaitProcessingDetails(t, buildsWaitProcessingDetail, nil)

	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildsWaitJSONResponse(http.StatusOK, `{
			"data": {
				"type": "builds",
				"id": "build-1",
				"attributes": {"version": "42", "processingState": "VALID"}
			}
		}`)
	})

	failureContext := shared.BuildProcessingFailureContext{
		AppID:        "app-1",
		ShortVersion: "1.2.3",
		Platform:     "IOS",
	}

	var buildResp *asc.BuildResponse
	var err error
	captureBuildsWaitStderr(t, func() {
		buildResp, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, true, failureContext)
	})
	if err != nil {
		t.Fatalf("waitForBuildProcessingState() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.Attributes.ProcessingState != asc.BuildProcessingStateValid {
		t.Fatalf("expected the VALID build to be returned, got %#v", buildResp)
	}
	if diagnostics.calls != 0 {
		t.Fatalf("diagnostics lookups = %d, want 0", diagnostics.calls)
	}
}

func TestWaitForBuildDiscoveryToleratesTransientLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		if calls <= 2 {
			return buildsWaitServiceUnavailable()
		}
		return buildsWaitJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "PROCESSING", "uploadedDate": "2026-03-16T12:00:05Z"}
				}
			],
			"links": {}
		}`)
	})

	selector := appBuildWaitSelector{
		AppID:       "123456789",
		BuildNumber: "42",
		Platform:    "IOS",
	}

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureBuildsWaitStderr(t, func() {
		buildResp, err = waitForBuildDiscovery(context.Background(), client, selector, time.Millisecond)
	})
	if err != nil {
		t.Fatalf("waitForBuildDiscovery() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.ID != "build-1" {
		t.Fatalf("expected discovered build-1 after transient failures, got %#v", buildResp)
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

func TestWaitForBuildDiscoveryFailsAfterConsecutiveTransientLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildsWaitServiceUnavailable()
	})

	selector := appBuildWaitSelector{
		AppID:       "123456789",
		BuildNumber: "42",
		Platform:    "IOS",
	}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildDiscovery(context.Background(), client, selector, time.Millisecond)
	})
	if err == nil {
		t.Fatal("expected error once transient failures exceed the ceiling, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 6 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive transient failure error, got %v", err)
	}
	if calls != asc.DefaultMaxConsecutivePollFailures+1 {
		t.Fatalf("expected %d lookups, got %d", asc.DefaultMaxConsecutivePollFailures+1, calls)
	}
}

// The --since build-number path stops as soon as ambiguity is proven, so the
// error must describe retained candidates as a sample instead of a total.
func TestResolveBuildByNumberSelectionSinceReportsCandidatesWithoutClaimingLaterPages(t *testing.T) {
	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildsWaitJSONResponse(http.StatusOK, `{"data":[
			{"type":"builds","id":"build-ios","attributes":{"version":"42","uploadedDate":"2026-03-02T18:01:00Z","processingState":"VALID"}},
			{"type":"builds","id":"build-macos","attributes":{"version":"42","uploadedDate":"2026-03-02T18:00:30Z","processingState":"VALID"}},
			{"type":"builds","id":"build-third","attributes":{"version":"42","uploadedDate":"2026-03-02T18:00:00Z","processingState":"VALID"}}
		],"links":{}}`)
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelectionSince(
		context.Background(),
		client,
		"123456789",
		"42",
		"",
		"IOS",
		[]asc.BuildsOption{asc.WithBuildsVersion("42")},
		&since,
		false,
	)
	if err == nil {
		t.Fatal("expected an ambiguity error for two matching builds")
	}
	message := err.Error()
	for _, want := range []string{"multiple builds match", "pass --build-id with one of these sample matches:", "build-ios", "build-macos", "listed builds are a sample"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in %q", want, message)
		}
	}
	if strings.Contains(message, "later pages") {
		t.Fatalf("no later pages exist for this response: %q", message)
	}
	if strings.Contains(message, "build-third") {
		t.Fatalf("resolver should stop after proving ambiguity: %q", message)
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want 1", calls)
	}
}

func TestResolveBuildByNumberSelectionSinceDoesNotClaimUninspectedNextPageMatches(t *testing.T) {
	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildsWaitJSONResponse(http.StatusOK, `{"data":[
			{"type":"builds","id":"build-ios","attributes":{"version":"42","uploadedDate":"2026-03-02T18:01:00Z","processingState":"VALID"}},
			{"type":"builds","id":"build-macos","attributes":{"version":"42","uploadedDate":"2026-03-02T18:00:30Z","processingState":"VALID"}}
		],"links":{"next":"https://api.appstoreconnect.apple.com/v1/builds?cursor=older"}}`)
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelectionSince(
		context.Background(), client, "123456789", "42", "", "IOS",
		[]asc.BuildsOption{asc.WithBuildsVersion("42")}, &since, false,
	)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	message := err.Error()
	if strings.Contains(message, "matching builds exist on later pages") || !strings.Contains(message, "additional matches may exist") {
		t.Fatalf("must not classify uninspected next-page rows: %q", message)
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want 1", calls)
	}
}

func TestResolveBuildByNumberSelectionSinceRejectsRepeatedNextURL(t *testing.T) {
	calls := 0
	const repeatedNext = "https://api.appstoreconnect.apple.com/v1/builds?cursor=older"
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		switch calls {
		case 1:
			return buildsWaitJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[
				{"type":"builds","id":"build-1","attributes":{"version":"42","uploadedDate":"2026-03-02T18:01:00Z","processingState":"VALID"}}
			],"links":{"next":%q}}`, repeatedNext))
		case 2:
			// The repeated page must be rejected before this duplicate row can
			// be interpreted as an ambiguity.
			return buildsWaitJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[
				{"type":"builds","id":"build-1","attributes":{"version":"42","uploadedDate":"2026-03-02T18:01:00Z","processingState":"VALID"}}
			],"links":{"next":%q}}`, repeatedNext))
		default:
			return nil, context.Canceled
		}
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelectionSince(
		context.Background(), client, "123456789", "42", "", "IOS",
		[]asc.BuildsOption{asc.WithBuildsVersion("42")}, &since, false,
	)
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected repeated pagination URL error, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("requests = %d, want 2 before repeated-link rejection", calls)
	}
}

func TestResolveBuildByNumberSelectionSinceRejectsEquivalentReorderedNextURL(t *testing.T) {
	calls := 0
	firstNext := "/v1/builds?cursor=older&filter%5Bapp%5D=123456789"
	secondNext := "https://api.appstoreconnect.apple.com/v1/builds?filter%5Bapp%5D=123456789&cursor=older"
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		switch calls {
		case 1:
			return buildsWaitJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[
				{"type":"builds","id":"build-new","attributes":{"version":"42","uploadedDate":"2026-03-02T18:01:00Z","processingState":"VALID"}}
			],"links":{"next":%q}}`, firstNext))
		case 2:
			// The reordered query string and absolute spelling are the same
			// request as the provider-relative firstNext.
			return buildsWaitJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[
				{"type":"builds","id":"build-old","attributes":{"version":"42","uploadedDate":"2026-03-02T16:59:00Z","processingState":"VALID"}}
			],"links":{"next":%q}}`, secondNext))
		default:
			return nil, context.Canceled
		}
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelectionSince(
		context.Background(), client, "123456789", "42", "", "IOS",
		[]asc.BuildsOption{asc.WithBuildsVersion("42")}, &since, false,
	)
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected equivalent repeated pagination URL error, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("requests = %d, want 2 before reordered-link rejection", calls)
	}
}

func TestResolveBuildByNumberSelectionSinceStopsAtPageSafetyLimit(t *testing.T) {
	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildsWaitJSONResponse(http.StatusOK, fmt.Sprintf(
			`{"data":[],"links":{"next":"/v1/builds?cursor=%d"}}`,
			calls,
		))
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelectionSince(
		context.Background(), client, "123456789", "42", "", "IOS",
		[]asc.BuildsOption{asc.WithBuildsVersion("42")}, &since, false,
	)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d-page safety limit", resolveBuildSinceMaxPages)) {
		t.Fatalf("expected page safety limit error, got %v", err)
	}
	if calls != resolveBuildSinceMaxPages {
		t.Fatalf("requests = %d, want %d before page-limit rejection", calls, resolveBuildSinceMaxPages)
	}
}

func TestResolveBuildByNumberSelectionSinceBoundsPreReleaseVersionPagination(t *testing.T) {
	preReleaseCalls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/preReleaseVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		preReleaseCalls++
		return buildsWaitJSONResponse(http.StatusOK, fmt.Sprintf(
			`{"data":[],"links":{"next":"/v1/preReleaseVersions?cursor=%d"}}`,
			preReleaseCalls,
		))
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelection(
		context.Background(),
		client,
		buildNumberSelectionOptions{
			AppID:       "123456789",
			Version:     "1.2.3",
			BuildNumber: "42",
			Platform:    "IOS",
			Since:       &since,
		},
		false,
	)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d-page safety limit", resolveBuildSinceMaxPages)) {
		t.Fatalf("expected pre-release pagination safety-limit error, got %v", err)
	}
	if preReleaseCalls != resolveBuildSinceMaxPages {
		t.Fatalf("pre-release requests = %d, want %d before page-limit rejection", preReleaseCalls, resolveBuildSinceMaxPages)
	}
}
