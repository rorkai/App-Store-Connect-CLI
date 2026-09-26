package submit

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestSubmitResolvedVersionRejectsMalformedReviewSubmissionCollectionBeforeMutation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "top-level errors member", body: `{"errors":[],"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`},
		{name: "missing links", body: `{"data":[]}`},
		{name: "null links", body: `{"data":[],"links":null}`},
		{name: "missing required self link", body: `{"data":[],"links":{}}`},
		{name: "null next link", body: `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions","next":null}}`},
		{name: "malformed paging total", body: `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"},"meta":{"paging":{"total":"bad","limit":200}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
					return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
				}
				if req.URL.Path != "/v1/apps/app-1/reviewSubmissions" {
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
				return submitJSONResponse(http.StatusOK, tt.body)
			}))

			_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
				AppID:     "app-1",
				VersionID: "version-1",
				Platform:  "IOS",
			})
			if err == nil || !strings.Contains(err.Error(), "review submission") {
				t.Fatalf("expected malformed review-submissions envelope error, got %v", err)
			}
			if mutations != 0 {
				t.Fatalf("malformed review-submissions envelope allowed %d mutation requests", mutations)
			}
		})
	}
}

func TestSubmitResolvedVersionRejectsMalformedReviewSubmissionItemsCollectionBeforeMutation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "top-level errors member", body: `{"errors":[],"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items"}}`},
		{name: "missing links", body: `{"data":[]}`},
		{name: "null links", body: `{"data":[],"links":null}`},
		{name: "missing required self link", body: `{"data":[],"links":{}}`},
		{name: "null next link", body: `{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":null}}`},
		{name: "malformed paging total", body: `{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items"},"meta":{"paging":{"total":"bad","limit":200}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
					return submitJSONResponse(http.StatusOK, `{
						"data": [{
							"type": "reviewSubmissions",
							"id": "submission-1",
							"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
						}],
						"links": {"self": "/v1/apps/app-1/reviewSubmissions"}
					}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
					return submitJSONResponse(http.StatusOK, tt.body)
				case req.Method != http.MethodGet:
					mutations++
					return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
				AppID:     "app-1",
				VersionID: "version-1",
				Platform:  "IOS",
			})
			if err == nil || !strings.Contains(err.Error(), "review submission") {
				t.Fatalf("expected malformed review-submission-items envelope error, got %v", err)
			}
			if mutations != 0 {
				t.Fatalf("malformed review-submission-items envelope allowed %d mutation requests", mutations)
			}
		})
	}
}

func TestSubmitResolvedVersionRejectsMalformedSubmissionResourceBeforeMutation(t *testing.T) {
	tests := []struct {
		name       string
		resource   string
		wantSubstr string
	}{
		{
			name:       "wrong resource type after valid candidate",
			resource:   `{"type":"appStoreVersions","id":"wrong-type","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}`,
			wantSubstr: "reviewSubmissions",
		},
		{
			name:       "missing state after valid candidate",
			resource:   `{"type":"reviewSubmissions","id":"missing-state","attributes":{"platform":"IOS"}}`,
			wantSubstr: "state",
		},
		{
			name:       "missing platform after valid candidate",
			resource:   `{"type":"reviewSubmissions","id":"missing-platform","attributes":{"state":"READY_FOR_REVIEW"}}`,
			wantSubstr: "platform",
		},
		{
			name: "wrong app store version relationship type",
			resource: `{"type":"reviewSubmissions","id":"wrong-version-relation","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},` +
				`"relationships":{"appStoreVersionForReview":{"data":{"type":"subscriptionVersions","id":"version-1"}}}}`,
			wantSubstr: "appStoreVersionForReview",
		},
		{
			name: "null to-many items relationship",
			resource: `{"type":"reviewSubmissions","id":"null-items","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},` +
				`"relationships":{"items":{"data":null}}}`,
			wantSubstr: "items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
					return submitJSONResponse(http.StatusOK, `{
						"data": [
							{"type":"reviewSubmissions","id":"valid-candidate","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"appStoreVersionForReview":{"data":{"type":"appStoreVersions","id":"version-1"}}}},
							`+tt.resource+`
						],
						"links":{"self":"/v1/apps/app-1/reviewSubmissions"}
					}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/valid-candidate/items":
					return submitJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{"self":"/v1/reviewSubmissions/valid-candidate/items"}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/valid-candidate":
					return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"valid-candidate","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}},"links":{"self":"/v1/reviewSubmissions/valid-candidate"}}`)
				case req.Method != http.MethodGet:
					mutations++
					return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
				AppID:     "app-1",
				VersionID: "version-1",
				Platform:  "IOS",
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("expected malformed submission preflight error containing %q, got %v", tt.wantSubstr, err)
			}
			if mutations != 0 {
				t.Fatalf("malformed submission resource allowed %d mutation requests", mutations)
			}
		})
	}
}

func TestSubmitResolvedVersionRejectsMalformedSubmissionItemBeforeMutation(t *testing.T) {
	tests := []struct {
		name string
		item string
	}{
		{
			name: "wrong resource type",
			item: `{"type":"appStoreVersions","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}`,
		},
		{
			name: "empty resource ID",
			item: `{"type":"reviewSubmissionItems","id":"","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}`,
		},
		{
			name: "wrong app store version relationship type",
			item: `{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"subscriptionVersions","id":"version-1"}}}}`,
		},
		{
			name: "empty app store version relationship ID",
			item: `{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":""}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
					return submitJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
					return submitJSONResponse(http.StatusOK, `{"data":[`+tt.item+`],"links":{"self":"/v1/reviewSubmissions/submission-1/items"}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1":
					return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}},"links":{"self":"/v1/reviewSubmissions/submission-1"}}`)
				case req.Method != http.MethodGet:
					mutations++
					return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
				AppID:     "app-1",
				VersionID: "version-1",
				Platform:  "IOS",
			})
			if err == nil || !strings.Contains(err.Error(), "review submission") {
				t.Fatalf("expected malformed item preflight error, got %v", err)
			}
			if mutations != 0 {
				t.Fatalf("malformed submission item allowed %d mutation requests", mutations)
			}
		})
	}
}

func TestSummarizeReviewSubmissionItemsBoundsPagination(t *testing.T) {
	pageReads := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		pageReads++
		next := "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items?cursor=page-2"
		if pageReads == 2 {
			next = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items?cursor=page-3"
		}
		return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":"`+next+`"}}`)
	}))

	_, err := summarizeReviewSubmissionItemsWithPageLimit(context.Background(), client, "submission-1", "version-1", 2)
	if err == nil || !strings.Contains(err.Error(), "must not exceed 2 pages") {
		t.Fatalf("expected bounded pagination error, got %v", err)
	}
	if pageReads != 2 {
		t.Fatalf("item pages read = %d, want 2", pageReads)
	}
}

func TestSummarizeReviewSubmissionItemsTrimsWhitespaceNext(t *testing.T) {
	pageReads := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		pageReads++
		return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":"  \t  "}}`)
	}))

	if _, err := summarizeReviewSubmissionItemsWithPageLimit(context.Background(), client, "submission-1", "version-1", 2); err != nil {
		t.Fatalf("expected whitespace next link to be terminal, got %v", err)
	}
	if pageReads != 1 {
		t.Fatalf("item pages read = %d, want 1", pageReads)
	}
}

func TestVerifyReviewSubmissionForSubmitRejectsMalformedDetailBeforeItemInspection(t *testing.T) {
	tests := []struct {
		name          string
		resourceType  string
		appType       string
		appID         string
		wantSubstring string
	}{
		{
			name:          "wrong resource type",
			resourceType:  "appStoreVersions",
			appType:       "apps",
			appID:         "app-1",
			wantSubstring: "resource type",
		},
		{
			name:          "wrong app relationship type",
			resourceType:  "reviewSubmissions",
			appType:       "appStoreVersions",
			appID:         "app-1",
			wantSubstring: "app relationship type",
		},
		{
			name:          "empty app relationship ID",
			resourceType:  "reviewSubmissions",
			appType:       "apps",
			appID:         "",
			wantSubstring: "app relationship",
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
                            "type": %q,
                            "id": "submission-1",
                            "attributes": {"state":"READY_FOR_REVIEW","platform":"IOS"},
                            "relationships": {"app": {"data": {"type": %q, "id": %q}}}
                        }
                    }`, tt.resourceType, tt.appType, tt.appID))
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
					itemReads++
					return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items"}}`)
				case req.Method != http.MethodGet:
					return nil, fmt.Errorf("unexpected mutation: %s %s", req.Method, req.URL.RequestURI())
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			err := verifyReviewSubmissionForSubmit(context.Background(), client, "submission-1", "app-1", "IOS", "version-1")
			if err == nil || !strings.Contains(err.Error(), tt.wantSubstring) {
				t.Fatalf("expected detail validation error containing %q, got %v", tt.wantSubstring, err)
			}
			if itemReads != 0 {
				t.Fatalf("malformed detail reached item inspection (%d reads)", itemReads)
			}
		})
	}
}

func TestSubmitResolvedVersionValidatesExistingSubmissionBeforeAddingItem(t *testing.T) {
	itemReads := 0
	addRequests := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission/items":
			itemReads++
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/existing-submission/items"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"CANCELING","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			addRequests++
			return nil, fmt.Errorf("unexpected add after invalid pre-add detail")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Platform:  "IOS",
	})
	if err == nil || !strings.Contains(err.Error(), "READY_FOR_REVIEW") {
		t.Fatalf("expected pre-add detail validation error, got %v", err)
	}
	if itemReads != 1 {
		t.Fatalf("preparation item reads = %d, want 1", itemReads)
	}
	if addRequests != 0 {
		t.Fatalf("invalid existing detail allowed %d add requests", addRequests)
	}
}

func TestSubmitResolvedVersionValidatesCreateReceiptBeforeAddingItem(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		want          string
		wantPreserved bool
	}{
		{
			name:          "wrong resource type",
			response:      `{"data":{"type":"apps","id":"new-submission"}}`,
			want:          "resource type",
			wantPreserved: true,
		},
		{
			name:          "wrong platform",
			response:      `{"data":{"type":"reviewSubmissions","id":"new-submission","attributes":{"platform":"MAC_OS"}}}`,
			want:          "platform",
			wantPreserved: true,
		},
		{
			name:          "wrong app",
			response:      `{"data":{"type":"reviewSubmissions","id":"new-submission","relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}}`,
			want:          "belongs to app",
			wantPreserved: true,
		},
		{
			name:     "empty resource ID",
			response: `{"data":{"type":"reviewSubmissions","id":""}}`,
			want:     "empty ID",
		},
		{
			name:          "top-level errors member",
			response:      `{"errors":[],"data":{"type":"reviewSubmissions","id":"new-submission"}}`,
			want:          "top-level errors",
			wantPreserved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addRequests := 0
			messages := make([]string, 0)
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
					return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
					return submitJSONResponse(http.StatusCreated, tt.response)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
					addRequests++
					return nil, fmt.Errorf("unexpected add after invalid create receipt")
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			_, err := SubmitResolvedVersion(context.Background(), client, SubmitResolvedVersionOptions{
				AppID:     "app-1",
				VersionID: "version-1",
				Platform:  "IOS",
				Emit:      func(message string) { messages = append(messages, message) },
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected create receipt validation error containing %q, got %v", tt.want, err)
			}
			if addRequests != 0 {
				t.Fatalf("invalid create receipt allowed %d add requests", addRequests)
			}
			preserved := false
			for _, message := range messages {
				if strings.Contains(message, "Preserved newly created review submission new-submission") {
					preserved = true
					break
				}
			}
			if preserved != tt.wantPreserved {
				t.Fatalf("preservation warning = %t, want %t; messages = %v", preserved, tt.wantPreserved, messages)
			}
		})
	}
}

func TestSubmitResolvedVersionRejectsTopLevelErrorsInDetailBeforeItemInspection(t *testing.T) {
	itemReads := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission/items":
			itemReads++
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/existing-submission/items"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission":
			return submitJSONResponse(http.StatusOK, `{"errors":[],"data":{"type":"reviewSubmissions","id":"existing-submission","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	err := verifyReviewSubmissionForSubmit(context.Background(), client, "existing-submission", "app-1", "IOS", "version-1")
	if err == nil || !strings.Contains(err.Error(), "top-level errors") {
		t.Fatalf("expected top-level errors rejection, got %v", err)
	}
	if itemReads != 0 {
		t.Fatalf("detail errors response reached item inspection (%d reads)", itemReads)
	}
}

func TestPrepareReviewSubmissionForCreateBoundsDiscoveryPagination(t *testing.T) {
	pageReads := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		pageReads++
		next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"
		if pageReads == 2 {
			next = "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-3"
		}
		return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions","next":"`+next+`"}}`)
	}))

	_, err := prepareReviewSubmissionForCreateWithPageLimit(context.Background(), client, "app-1", "IOS", "version-1", nil, 2)
	if err == nil || !strings.Contains(err.Error(), "must not exceed 2 pages") {
		t.Fatalf("expected bounded discovery error, got %v", err)
	}
	if pageReads != 2 {
		t.Fatalf("submission pages read = %d, want 2", pageReads)
	}
}
