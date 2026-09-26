package submit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubmitResolvedVersionReusesReadySubmissionWithTargetVersion(t *testing.T) {
	var (
		createdSubmission   bool
		addedItem           bool
		canceledSubmission  bool
		submittedSubmission bool
		emittedMessages     []string
	)

	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "existing-submission",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission/items":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/existing-submission/items"}
				}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "existing-submission",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					},
					"relationships": {
						"app": {
							"data": {"type": "apps", "id": "app-1"}
						}
					}
				}
			}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			createdSubmission = true
			return submitJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			addedItem = true
			return submitJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-2"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/existing-submission":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("read patch body: %w", err)
			}
			var payload asc.ReviewSubmissionUpdateRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				return nil, fmt.Errorf("decode patch body: %w", err)
			}
			switch {
			case payload.Data.Attributes.Canceled != nil && payload.Data.Attributes.Canceled.Value != nil && *payload.Data.Attributes.Canceled.Value:
				canceledSubmission = true
				return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"DEVELOPER_REMOVED_FROM_SALE"}}}`)
			case payload.Data.Attributes.Submitted != nil && payload.Data.Attributes.Submitted.Value != nil && *payload.Data.Attributes.Submitted.Value:
				submittedSubmission = true
				return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-29T00:00:00Z"}}}`)
			default:
				return nil, fmt.Errorf("unexpected review submission update payload: %s", string(body))
			}
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
		Emit: func(message string) {
			emittedMessages = append(emittedMessages, message)
		},
	})
	if err != nil {
		t.Fatalf("SubmitResolvedVersion() error: %v", err)
	}

	if got.SubmissionID != "existing-submission" {
		t.Fatalf("expected reused submission ID existing-submission, got %#v", got)
	}
	if !submittedSubmission {
		t.Fatal("expected existing submission to be submitted")
	}
	if canceledSubmission {
		t.Fatal("did not expect reused submission to be canceled first")
	}
	if createdSubmission {
		t.Fatal("did not expect a new review submission to be created")
	}
	if addedItem {
		t.Fatal("did not expect target version to be re-added when already attached")
	}
	wantMessage := "Reusing existing review submission existing-submission because the target version is already attached."
	if !strings.Contains(strings.Join(got.Messages, "\n"), wantMessage) {
		t.Fatalf("expected result messages to include reuse notice, got %#v", got.Messages)
	}
	if !strings.Contains(strings.Join(emittedMessages, "\n"), wantMessage) {
		t.Fatalf("expected emit callback to receive reuse notice, got %#v", emittedMessages)
	}
}

func TestSubmitResolvedVersionFailsClosedWhenReviewSubmissionPreparationFails(t *testing.T) {
	transportErr := errors.New("review submission lookup transport failed")
	tests := []struct {
		name      string
		handler   func(*http.Request) (*http.Response, error)
		wantAPI   bool
		wantCause error
	}{
		{
			name: "initial submission lookup",
			handler: func(req *http.Request) (*http.Response, error) {
				return submitJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"BAD_REQUEST","title":"Invalid request"}]}`)
			},
			wantAPI: true,
		},
		{
			name: "initial transport failure",
			handler: func(req *http.Request) (*http.Response, error) {
				return nil, transportErr
			},
			wantCause: transportErr,
		},
		{
			name: "initial malformed response",
			handler: func(req *http.Request) (*http.Response, error) {
				return submitJSONResponse(http.StatusOK, `{`)
			},
		},
		{
			name: "initial response missing data",
			handler: func(req *http.Request) (*http.Response, error) {
				return submitJSONResponse(http.StatusOK, `{}`)
			},
		},
		{
			name: "initial response has null data",
			handler: func(req *http.Request) (*http.Response, error) {
				return submitJSONResponse(http.StatusOK, `{"data":null}`)
			},
		},
		{
			name: "initial response claims omitted submissions",
			handler: func(req *http.Request) (*http.Response, error) {
				return submitJSONResponse(http.StatusOK, `{
					"data": [],
				"links": {"self": "/v1/apps/app-1/reviewSubmissions"},
					"meta": {"paging": {"total": 1, "limit": 200}}
				}`)
			},
		},
		{
			name: "later submission page",
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Query().Get("cursor") == "" {
					return submitJSONResponse(http.StatusOK, `{
						"data": [],
					"links": {"self": "/v1/apps/app-1/reviewSubmissions", "next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
					}`)
				}
				return submitJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"BAD_REQUEST","title":"Invalid request"}]}`)
			},
			wantAPI: true,
		},
		{
			name: "later submission page missing data",
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Query().Get("cursor") == "" {
					return submitJSONResponse(http.StatusOK, `{
						"data": [],
					"links": {"self": "/v1/apps/app-1/reviewSubmissions", "next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"}
					}`)
				}
				return submitJSONResponse(http.StatusOK, `{}`)
			},
		},
		{
			name: "submission item inspection",
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/apps/app-1/reviewSubmissions" {
					return submitJSONResponse(http.StatusOK, `{
						"data": [{
							"type": "reviewSubmissions",
							"id": "unproven-submission",
							"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
						}],
						"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
					}`)
				}
				return submitJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"BAD_REQUEST","title":"Invalid request"}]}`)
			},
			wantAPI: true,
		},
		{
			name: "submission item response missing data",
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/apps/app-1/reviewSubmissions" {
					return submitJSONResponse(http.StatusOK, `{
						"data": [{
							"type": "reviewSubmissions",
							"id": "unproven-submission",
							"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
						}],
						"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
					}`)
				}
				return submitJSONResponse(http.StatusOK, `{}`)
			},
		},
		{
			name: "submission item response has null data",
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/apps/app-1/reviewSubmissions" {
					return submitJSONResponse(http.StatusOK, `{
						"data": [{
							"type": "reviewSubmissions",
							"id": "unproven-submission",
							"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
						}],
						"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
					}`)
				}
				return submitJSONResponse(http.StatusOK, `{"data":null}`)
			},
		},
		{
			name: "submission item response claims omitted items",
			handler: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/apps/app-1/reviewSubmissions" {
					return submitJSONResponse(http.StatusOK, `{
						"data": [{
							"type": "reviewSubmissions",
							"id": "unproven-submission",
							"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
						}],
						"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
					}`)
				}
				return submitJSONResponse(http.StatusOK, `{
					"data": [],
					"links": {"self": "/v1/apps/app-1/reviewSubmissions"},
					"meta": {"paging": {"total": 1, "limit": 200}}
				}`)
			},
		},
		{
			name: "ready submission missing ID",
			handler: func(req *http.Request) (*http.Response, error) {
				return submitJSONResponse(http.StatusOK, `{
					"data": [{
						"type": "reviewSubmissions",
						"id": "",
						"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
					}],
					"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
				}`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutated := false
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutated = true
					return nil, errors.New("mutation attempted after uncertain preparation")
				}
				return tt.handler(req)
			}))

			_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
				AppID:     "app-1",
				VersionID: "version-1",
				Platform:  "IOS",
			})
			if err == nil {
				t.Fatal("expected preparation failure")
			}
			if mutated {
				t.Fatal("review submission preparation failure must stop before mutation")
			}
			if tt.wantAPI {
				var apiErr *asc.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected API error identity to be preserved, got %T: %v", err, err)
				}
			}
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Fatalf("expected error to preserve %v, got %v", tt.wantCause, err)
			}
		})
	}
}

func TestSubmitResolvedVersionPreflightsBeforeBuildAttachment(t *testing.T) {
	buildLookup := false
	mutated := false
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"BAD_REQUEST","title":"Invalid request"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			buildLookup = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-old"}}`)
		default:
			if req.Method != http.MethodGet {
				mutated = true
			}
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:               "app-1",
		VersionID:           "version-1",
		BuildID:             "build-target",
		Platform:            "IOS",
		EnsureBuildAttached: true,
	})
	if err == nil {
		t.Fatal("expected review submission preflight failure")
	}
	if buildLookup {
		t.Fatal("build attachment lookup must not run before review submission preflight succeeds")
	}
	if mutated {
		t.Fatal("review submission preflight failure must stop before mutation")
	}
}

func TestSubmitResolvedVersionRejectsRepeatedPreparationPageBeforeMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pageReads := 0
	mutated := false
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutated = true
			return nil, errors.New("mutation attempted after repeated pagination URL")
		}
		pageReads++
		if pageReads > 2 {
			cancel()
			return nil, context.Canceled
		}
		return submitJSONResponse(http.StatusOK, `{
			"data": [],
			"links": {"self": "/v1/apps/app-1/reviewSubmissions", "next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=same"}
		}`)
	}))

	_, err := SubmitResolvedVersion(ctx, client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
	})
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected repeated-pagination error, got %v", err)
	}
	if pageReads != 2 {
		t.Fatalf("review submission pages read = %d, want 2", pageReads)
	}
	if mutated {
		t.Fatal("repeated review submission page must stop before mutation")
	}
}

func TestSubmitResolvedVersionSkipsBuildAttachmentWhenAlreadySubmitted(t *testing.T) {
	var (
		buildLookup bool
		buildAttach bool
	)

	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			buildLookup = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-current"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			buildAttach = true
			return submitJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-sub-1"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:                    "app-1",
		VersionID:                "version-1",
		BuildID:                  "build-target",
		Platform:                 "IOS",
		EnsureBuildAttached:      true,
		LookupExistingSubmission: true,
	})
	if err != nil {
		t.Fatalf("SubmitResolvedVersion() error: %v", err)
	}

	if !got.AlreadySubmitted {
		t.Fatalf("expected already submitted result, got %#v", got)
	}
	if got.SubmissionID != "legacy-sub-1" {
		t.Fatalf("expected submission ID legacy-sub-1, got %#v", got)
	}
	if buildLookup {
		t.Fatal("did not expect build lookup before existing-submission short-circuit")
	}
	if buildAttach {
		t.Fatal("did not expect build attachment before existing-submission short-circuit")
	}
	if got.BuildAttachment != nil {
		t.Fatalf("expected build attachment result to be omitted, got %#v", got.BuildAttachment)
	}
}

func TestSubmitResolvedVersionResultJSONOmitsBuildAttachmentWhenUnused(t *testing.T) {
	data, err := json.Marshal(SubmitResolvedVersionResult{
		SubmissionID: "review-sub-1",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	if strings.Contains(string(data), "buildAttachment") {
		t.Fatalf("expected buildAttachment to be omitted when unset, got %s", data)
	}
}

func TestEnsureBuildAttachedAlreadyAttached(t *testing.T) {
	var buildAttach bool

	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			buildAttach = true
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := EnsureBuildAttached(context.Background(), client, " version-1 ", " build-1 ", false)
	if err != nil {
		t.Fatalf("EnsureBuildAttached() error: %v", err)
	}
	if got.VersionID != "version-1" || got.BuildID != "build-1" {
		t.Fatalf("expected trimmed IDs in result, got %#v", got)
	}
	if !got.AlreadyAttached {
		t.Fatalf("expected already attached result, got %#v", got)
	}
	if got.CurrentBuildID != "build-1" {
		t.Fatalf("expected current build ID build-1, got %#v", got)
	}
	if got.Attached || got.WouldAttach {
		t.Fatalf("did not expect attach or dry-run flags, got %#v", got)
	}
	if buildAttach {
		t.Fatal("did not expect attach request when build is already attached")
	}
}

func TestEnsureBuildAttachedDryRunSkipsMutation(t *testing.T) {
	var buildAttach bool

	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-old"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			buildAttach = true
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := EnsureBuildAttached(context.Background(), client, "version-1", "build-new", true)
	if err != nil {
		t.Fatalf("EnsureBuildAttached() error: %v", err)
	}
	if !got.WouldAttach {
		t.Fatalf("expected dry-run would-attach result, got %#v", got)
	}
	if got.CurrentBuildID != "build-old" {
		t.Fatalf("expected current build ID build-old, got %#v", got)
	}
	if got.Attached || got.AlreadyAttached {
		t.Fatalf("did not expect attached/already-attached flags, got %#v", got)
	}
	if buildAttach {
		t.Fatal("did not expect attach request during dry run")
	}
}

func TestEnsureBuildAttachedAttachesWhenNoCurrentBuild(t *testing.T) {
	var buildAttach bool

	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			buildAttach = true
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := EnsureBuildAttached(context.Background(), client, "version-1", "build-new", false)
	if err != nil {
		t.Fatalf("EnsureBuildAttached() error: %v", err)
	}
	if !got.Attached {
		t.Fatalf("expected attached result, got %#v", got)
	}
	if got.CurrentBuildID != "" {
		t.Fatalf("expected empty current build ID when none exists, got %#v", got)
	}
	if !buildAttach {
		t.Fatal("expected attach request when no current build exists")
	}
}

func TestEnsureBuildAttachedUsesFreshDeadlineForMutation(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			time.Sleep(60 * time.Millisecond)
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 70*time.Millisecond {
				t.Fatalf("expected fresh attach deadline, remaining=%s", time.Until(deadline))
			}
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := EnsureBuildAttached(context.Background(), client, "version-1", "build-new", false)
	if err != nil || !result.Attached {
		t.Fatalf("unexpected attach result: result=%+v err=%v", result, err)
	}
}

func TestEnsureBuildAttachedReconcilesAmbiguousMutation(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Setenv("ASC_TIMEOUT", "100ms")
	reads := 0
	patches := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			reads++
			if reads == 1 {
				return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
			}
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 70*time.Millisecond {
				t.Fatalf("expected fresh readback deadline, remaining=%s", time.Until(deadline))
			}
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-new"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			patches++
			time.Sleep(60 * time.Millisecond)
			return submitJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := EnsureBuildAttached(context.Background(), client, "version-1", "build-new", false)
	if err != nil || !result.Attached || reads != 2 || patches != 1 {
		t.Fatalf("unexpected reconcile result: result=%+v reads=%d patches=%d err=%v", result, reads, patches, err)
	}
}

func TestEnsureBuildAttachedReplaysOnlyAfterTwoNegativeReadbacks(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	reads := 0
	patches := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			reads++
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			patches++
			if patches == 1 {
				return submitJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"temporary"}]}`)
			}
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := EnsureBuildAttached(context.Background(), client, "version-1", "build-new", false)
	if err != nil || !result.Attached || reads != 3 || patches != 2 {
		t.Fatalf("unexpected bounded replay: result=%+v reads=%d patches=%d err=%v", result, reads, patches, err)
	}
}

func TestEnsureBuildAttachedPreservesNonTransientFailure(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	reads := 0
	patches := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			reads++
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			patches++
			return submitJSONResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"invalid build"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := EnsureBuildAttached(context.Background(), client, "version-1", "build-new", false)
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected typed non-transient error, result=%+v err=%T %v", result, err, err)
	}
	if reads != 2 || patches != 1 || result.Attached {
		t.Fatalf("unexpected non-transient calls: result=%+v reads=%d patches=%d", result, reads, patches)
	}
}

func TestLookupExistingSubmissionForVersionReturnsTrimmedID(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":" legacy-submission-1 "}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := LookupExistingSubmissionForVersion(context.Background(), client, " version-1 ", 0)
	if err != nil {
		t.Fatalf("LookupExistingSubmissionForVersion() error: %v", err)
	}
	if got != "legacy-submission-1" {
		t.Fatalf("expected trimmed submission ID, got %q", got)
	}
}

func TestLookupExistingSubmissionForVersionServerError(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_SERVER_ERROR","title":"Internal Error"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	_, err := LookupExistingSubmissionForVersion(context.Background(), client, "version-1", 0)
	if err == nil {
		t.Fatal("expected lookup error for server failure")
	}
}

func TestLookupExistingSubmissionForVersionNotFoundReturnsEmptyID(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	got, err := LookupExistingSubmissionForVersion(context.Background(), client, "version-1", 0)
	if err != nil {
		t.Fatalf("LookupExistingSubmissionForVersion() error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty submission ID when lookup returns 404, got %q", got)
	}
}

func TestLookupExistingSubmissionForVersionRejectsEmptyVersionID(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	}))

	_, err := LookupExistingSubmissionForVersion(context.Background(), client, " \t ", 0)
	if err == nil {
		t.Fatal("expected validation error for empty version ID")
	}
	if !strings.Contains(err.Error(), "resolved version ID is empty") {
		t.Fatalf("expected empty version ID validation error, got %v", err)
	}
}
