package submit

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestSubmitResolvedVersionRejectsUnverifiedConflictSubmission(t *testing.T) {
	var submitted bool
	var canceledCreatedSubmission bool
	itemPage := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/new-submission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-submission","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/new-submission/items":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/new-submission/items"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitJSONResponse(http.StatusConflict, submitAlreadyAddedConflictBody("conflict-submission"))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/conflict-submission":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "conflict-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"app": {"data": {"type": "apps", "id": "app-1"}}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/conflict-submission/items":
			itemPage++
			if req.URL.Query().Get("cursor") == "" {
				return submitJSONResponse(http.StatusOK, `{
					"data": [{
						"type": "reviewSubmissionItems",
						"id": "target-item",
						"relationships": {
							"appStoreVersion": {
								"data": {"type": "appStoreVersions", "id": "version-1"}
							}
						}
					}],
					"links": {"self": "/v1/reviewSubmissions/conflict-submission/items", "next": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/conflict-submission/items?cursor=page-2"}
				}`)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": [{"type": "reviewSubmissionItems", "id": "unrelated-item"}],
				"links": {"self": "/v1/reviewSubmissions/conflict-submission/items"}
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-submission":
			canceledCreatedSubmission = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/conflict-submission":
			submitted = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"conflict-submission"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
	})
	if err == nil {
		t.Fatal("expected mixed conflict submission to be rejected")
	}
	if !strings.Contains(err.Error(), "unrelated review items") {
		t.Fatalf("expected exclusive-target verification error, got %v", err)
	}
	if itemPage != 2 {
		t.Fatalf("expected every conflict-submission item page to be inspected, got %d", itemPage)
	}
	if submitted {
		t.Fatal("must not submit a conflict-derived submission that has unrelated review items")
	}
	if canceledCreatedSubmission {
		t.Fatal("must preserve the newly created submission when conflict recovery is indeterminate")
	}
	messages := strings.Join(result.Messages, "\n")
	if !strings.Contains(messages, "new-submission") || !strings.Contains(messages, "Retry") || !strings.Contains(messages, "asc submit cancel") {
		t.Fatalf("expected retry and explicit-cancel guidance for the preserved submission, got %#v", result.Messages)
	}
}

func TestSubmitResolvedVersionPreservesCreatedSubmissionAfterAmbiguousAddFailure(t *testing.T) {
	var canceledCreatedSubmission bool
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/new-submission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-submission","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/new-submission/items":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/new-submission/items"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return nil, fmt.Errorf("connection closed after sending item request")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-submission":
			canceledCreatedSubmission = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
	})
	if err == nil {
		t.Fatal("expected ambiguous add failure")
	}
	if canceledCreatedSubmission {
		t.Fatal("must not cancel a newly created submission after an ambiguous add failure")
	}
	messages := strings.Join(result.Messages, "\n")
	if !strings.Contains(messages, "new-submission") || !strings.Contains(messages, "Retry") || !strings.Contains(messages, "asc submit cancel") {
		t.Fatalf("expected retry and explicit-cancel guidance for the preserved submission, got %#v", result.Messages)
	}
}

func TestSubmitResolvedVersionPreservesCreatedSubmissionAfterConflictRecovery(t *testing.T) {
	var (
		canceledCreatedSubmission bool
		submittedRecovered        bool
	)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/new-submission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-submission","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/new-submission/items":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/new-submission/items"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitJSONResponse(http.StatusConflict, submitAlreadyAddedConflictBody("conflict-submission"))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/conflict-submission":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "conflict-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"app": {"data": {"type": "apps", "id": "app-1"}}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/conflict-submission/items":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "target-item",
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/existing-submission/items"}
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-submission":
			canceledCreatedSubmission = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-submission"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/conflict-submission":
			submittedRecovered = true
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "conflict-submission",
					"attributes": {"state": "WAITING_FOR_REVIEW"}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
	})
	if err != nil {
		t.Fatalf("SubmitResolvedVersion() error: %v", err)
	}
	if !submittedRecovered || result.SubmissionID != "conflict-submission" {
		t.Fatalf("expected recovered submission to be submitted, got %#v", result)
	}
	if canceledCreatedSubmission {
		t.Fatal("must preserve the newly created submission after conflict recovery")
	}
	messages := strings.Join(result.Messages, "\n")
	if !strings.Contains(messages, "new-submission") || !strings.Contains(messages, "Retry") || !strings.Contains(messages, "asc submit cancel") {
		t.Fatalf("expected retry and explicit-cancel guidance for the preserved submission, got %#v", result.Messages)
	}
}

func TestVerifyReviewSubmissionForSubmitRejectsWrongBinding(t *testing.T) {
	tests := []struct {
		name       string
		attributes string
		appID      string
		want       string
	}{
		{
			name:       "state",
			attributes: `"state": "CANCELING", "platform": "IOS"`,
			appID:      "app-1",
			want:       "not \"READY_FOR_REVIEW\"",
		},
		{
			name:       "platform",
			attributes: `"state": "READY_FOR_REVIEW", "platform": "MAC_OS"`,
			appID:      "app-1",
			want:       "not \"IOS\"",
		},
		{
			name:       "app",
			attributes: `"state": "READY_FOR_REVIEW", "platform": "IOS"`,
			appID:      "app-2",
			want:       "belongs to app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			itemReads := 0
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1":
					return submitJSONResponse(http.StatusOK, fmt.Sprintf(`{
						"data": {
							"type": "reviewSubmissions",
							"id": "submission-1",
							"attributes": {%s},
							"relationships": {
								"app": {"data": {"type": "apps", "id": %q}}
							}
						}
					}`, tt.attributes, tt.appID))
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
					itemReads++
					return submitJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			err := verifyReviewSubmissionForSubmit(
				context.Background(),
				client,
				"submission-1",
				"app-1",
				"IOS",
				"version-1",
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q binding error, got %v", tt.want, err)
			}
			if itemReads != 0 {
				t.Fatalf("expected binding mismatch to fail before item inspection, got %d item reads", itemReads)
			}
		})
	}
}

func TestSubmitResolvedVersionReusesTargetOnlySubmissionWithoutVersionRelationship(t *testing.T) {
	itemReads := 0
	detailReads := 0
	var submitted bool
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "existing-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
				}],
				"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission":
			detailReads++
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "existing-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"app": {"data": {"type": "apps", "id": "app-1"}}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission/items":
			itemReads++
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "target-item",
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/existing-submission/items"}
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/existing-submission":
			submitted = true
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "existing-submission",
					"attributes": {"state": "WAITING_FOR_REVIEW"}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	result, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
	})
	if err != nil {
		t.Fatalf("SubmitResolvedVersion() error: %v", err)
	}
	if result.SubmissionID != "existing-submission" || !submitted {
		t.Fatalf("expected target-only submission to be reused and submitted, got %#v", result)
	}
	if detailReads != 1 {
		t.Fatalf("expected one fresh detail recheck immediately before submit, got %d", detailReads)
	}
	if itemReads != 2 {
		t.Fatalf("expected preparation plus fresh item membership checks, got %d", itemReads)
	}
}

func TestPrepareReviewSubmissionForCreateNeverCancelsExistingSubmission(t *testing.T) {
	var canceled bool
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "other-version-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-2"}
						}
					}
				}],
				"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/other-version-submission/items":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/other-version-submission/items"}}`)
		case req.Method == http.MethodPatch:
			canceled = true
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"other-version-submission"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	var messages []string
	got, err := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1", func(message string) {
		messages = append(messages, message)
	})
	if err != nil {
		t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
	}
	if got.reuseSubmissionID != "" {
		t.Fatalf("must not reuse a submission explicitly bound to another version: %#v", got)
	}
	if canceled {
		t.Fatal("preparation must never implicitly cancel an existing submission")
	}
	if !strings.Contains(strings.Join(messages, "\n"), "asc submit cancel --id other-version-submission --confirm") {
		t.Fatalf("expected explicit cancellation remediation, got %v", messages)
	}
}

func TestReviewSubmissionInspectorCachesFailedInspection(t *testing.T) {
	itemReads := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/reviewSubmissions/submission-1/items" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
		itemReads++
		return submitJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","title":"Invalid request"}]}`)
	}))
	inspector := newReviewSubmissionInspector(client, "version-1")

	for attempt := 0; attempt < 2; attempt++ {
		if _, err := inspector.summarize(context.Background(), "submission-1"); err == nil {
			t.Fatal("expected failed item inspection")
		}
	}
	if itemReads != 1 {
		t.Fatalf("expected failed item inspection to be cached, got %d requests", itemReads)
	}
}
