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
