package submit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

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
				"links": {}
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
				}]
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
			case payload.Data.Attributes.Canceled != nil && *payload.Data.Attributes.Canceled:
				canceledSubmission = true
				return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"DEVELOPER_REMOVED_FROM_SALE"}}}`)
			case payload.Data.Attributes.Submitted != nil && *payload.Data.Attributes.Submitted:
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
