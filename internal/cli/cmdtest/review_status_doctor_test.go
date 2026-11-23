package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewStatusAndDoctorValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "review status missing app",
			args:    []string{"review", "status"},
			wantErr: "--app is required",
		},
		{
			name:    "review doctor missing app",
			args:    []string{"review", "doctor"},
			wantErr: "--app is required",
		},
		{
			name:    "review status whitespace-only app",
			args:    []string{"review", "status", "--app", "   "},
			wantErr: "Error: --app must not be empty",
		},
		{
			name:    "review doctor whitespace-only app",
			args:    []string{"review", "doctor", "--app", "   "},
			wantErr: "Error: --app must not be empty",
		},
		{
			name:    "review status mutually exclusive version flags",
			args:    []string{"review", "status", "--app", "123456789", "--version", "1.2.3", "--version-id", "ver-1"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name:    "review status positional args rejected",
			args:    []string{"review", "status", "--app", "123456789", "1.2.3"},
			wantErr: `unexpected argument "1.2.3"`,
		},
		{
			name:    "review doctor positional args rejected",
			args:    []string{"review", "doctor", "--app", "123456789", "1.2.3"},
			wantErr: `unexpected argument "1.2.3"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestReviewStatusShowsCurrentReviewState(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"WAITING_FOR_REVIEW",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-1",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload["reviewState"] != "WAITING_FOR_REVIEW" {
		t.Fatalf("expected reviewState WAITING_FOR_REVIEW, got %v", payload["reviewState"])
	}
	if payload["nextAction"] != "Wait for App Store review outcome." {
		t.Fatalf("expected wait-for-review next action, got %v", payload["nextAction"])
	}
	if payload["reviewDetailId"] != "detail-1" {
		t.Fatalf("expected reviewDetailId detail-1, got %v", payload["reviewDetailId"])
	}

	latestSubmission, ok := payload["latestSubmission"].(map[string]any)
	if !ok {
		t.Fatalf("expected latestSubmission object, got %T", payload["latestSubmission"])
	}
	if latestSubmission["id"] != "review-sub-1" {
		t.Fatalf("expected latest submission id review-sub-1, got %v", latestSubmission["id"])
	}
}

func TestReviewStatusUsesGenericNextActionForUnhandledSubmissionState(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"PREPARE_FOR_SUBMISSION",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-1",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"CANCELING","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload["reviewState"] != "CANCELING" {
		t.Fatalf("expected reviewState CANCELING, got %v", payload["reviewState"])
	}
	if payload["nextAction"] != "Review the latest submission state in App Store Connect." {
		t.Fatalf("expected generic next action for unhandled submission state, got %v", payload["nextAction"])
	}
}

func TestReviewStatusExplainsCompleteSubmissionWithOnlyRemovedItems(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"DEVELOPER_REJECTED",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-1",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-1",
				"attributes":{"state":"REMOVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if itemRequests := countStatusRequests(reg, "/v1/reviewSubmissions/review-sub-1/items"); itemRequests != 1 {
		t.Fatalf("expected 1 review submission item request, got %d", itemRequests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}
	if payload["nextAction"] != "Create a fresh review submission for the current version." {
		t.Fatalf("expected stale submission next action, got %v", payload["nextAction"])
	}

	blockers, ok := payload["blockers"].([]any)
	if !ok {
		t.Fatalf("expected blockers array, got %T", payload["blockers"])
	}
	if len(blockers) == 0 {
		t.Fatal("expected stale submission blocker")
	}

	found := false
	for _, blocker := range blockers {
		if strings.Contains(blocker.(string), "no longer contains active review items") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stale submission blocker in %v", blockers)
	}
}

func TestReviewStatusPointsCompleteSubmissionWithOnlyRemovedItemsAtDraft(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"DEVELOPER_REJECTED",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-1",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			},
			{
				"type":"reviewSubmissions",
				"id":"draft-1",
				"attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},
				"relationships":{
					"appStoreVersionForReview":{"data":null}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-1",
				"attributes":{"state":"REMOVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	reg.Register(asctest.GET("/v1/reviewSubmissions/draft-1/items", "fields[reviewSubmissionItems]=appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/draft-1/items"}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if itemRequests := countStatusRequests(reg, "/v1/reviewSubmissions/review-sub-1/items"); itemRequests != 1 {
		t.Fatalf("expected 1 review submission item request, got %d", itemRequests)
	}
	if draftItemRequests := countStatusRequests(reg, "/v1/reviewSubmissions/draft-1/items"); draftItemRequests != 1 {
		t.Fatalf("expected 1 draft item request, got %d", draftItemRequests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}
	wantNextAction := "Reuse draft review submission \"draft-1\": run `asc review items add --submission \"draft-1\" --item-type appStoreVersions --item-id \"ver-1\" --if-exists skip`, then `asc review submissions-submit --id \"draft-1\" --confirm`."
	if payload["nextAction"] != wantNextAction {
		t.Fatalf("expected draft reuse next action, got %v", payload["nextAction"])
	}

	blockers, ok := payload["blockers"].([]any)
	if !ok {
		t.Fatalf("expected blockers array, got %T", payload["blockers"])
	}
	if len(blockers) == 0 {
		t.Fatal("expected stale submission blocker")
	}

	found := false
	for _, blocker := range blockers {
		if strings.Contains(blocker.(string), "no longer contains active review items") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stale submission blocker in %v", blockers)
	}
}

func TestReviewStatusDoesNotPointAtDraftHoldingOtherItems(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"DEVELOPER_REJECTED",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-1",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			},
			{
				"type":"reviewSubmissions",
				"id":"draft-1",
				"attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},
				"relationships":{
					"appStoreVersionForReview":{"data":null}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-1",
				"attributes":{"state":"REMOVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	reg.Register(asctest.GET("/v1/reviewSubmissions/draft-1/items", "fields[reviewSubmissionItems]=appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"draft-item-1",
				"relationships":{
					"appCustomProductPageVersion":{
						"data":{"type":"appCustomProductPageVersions","id":"cpp-version-1"}
					}
				}
			}
		],
		"links":{"self":"/v1/reviewSubmissions/draft-1/items"}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if itemRequests := countStatusRequests(reg, "/v1/reviewSubmissions/review-sub-1/items"); itemRequests != 1 {
		t.Fatalf("expected 1 review submission item request, got %d", itemRequests)
	}
	if draftItemRequests := countStatusRequests(reg, "/v1/reviewSubmissions/draft-1/items"); draftItemRequests != 1 {
		t.Fatalf("expected 1 draft item request, got %d", draftItemRequests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}
	wantNextAction := "Draft review submission \"draft-1\" holds other review items, so do not submit it for this version: run `asc review submit --app \"123456789\" --version-id \"ver-1\" --build-id \"BUILD_ID\" --confirm`, which reuses a draft only when it holds nothing but this version and otherwise creates a new submission."
	if payload["nextAction"] != wantNextAction {
		t.Fatalf("expected review submit next action, got %v", payload["nextAction"])
	}

	blockers, ok := payload["blockers"].([]any)
	if !ok {
		t.Fatalf("expected blockers array, got %T", payload["blockers"])
	}
	if len(blockers) == 0 {
		t.Fatal("expected stale submission blocker")
	}

	found := false
	for _, blocker := range blockers {
		if strings.Contains(blocker.(string), "no longer contains active review items") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stale submission blocker in %v", blockers)
	}
}

func TestReviewStatusIgnoresUnrelatedActiveSubmissionItems(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.2.3",
					"appVersionState":"DEVELOPER_REJECTED",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-1",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-1",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-version-removed",
				"attributes":{"state":"REMOVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-1"}
					}
				}
			},
			{
				"type":"reviewSubmissionItems",
				"id":"item-background-active",
				"attributes":{"state":"APPROVED"},
				"relationships":{
					"backgroundAssetVersion":{
						"data":{"type":"backgroundAssetVersions","id":"bg-1"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-1"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if itemRequests := countStatusRequests(reg, "/v1/reviewSubmissions/review-sub-1/items"); itemRequests != 1 {
		t.Fatalf("expected 1 review submission item request, got %d", itemRequests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}
	if payload["nextAction"] != "Create a fresh review submission for the current version." {
		t.Fatalf("expected stale submission next action, got %v", payload["nextAction"])
	}

	blockers, ok := payload["blockers"].([]any)
	if !ok {
		t.Fatalf("expected blockers array, got %T", payload["blockers"])
	}
	if len(blockers) == 0 {
		t.Fatal("expected stale submission blocker")
	}

	found := false
	for _, blocker := range blockers {
		if strings.Contains(blocker.(string), "no longer contains active review items") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stale submission blocker in %v", blockers)
	}
}

func TestReviewStatusFiltersSubmissionsToSelectedVersion(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"ver-2",
			"attributes":{
				"platform":"IOS",
				"versionString":"2.0.0",
				"appVersionState":"PREPARE_FOR_SUBMISSION"
			},
			"relationships":{
				"app":{"data":{"type":"apps","id":"123456789"}}
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-2",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-other-version",
				"attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS","submittedDate":"2026-03-15T02:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			},
			{
				"type":"reviewSubmissions",
				"id":"review-sub-target",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-target/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-1",
				"attributes":{"state":"APPROVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-target"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789", "--version-id", "ver-2")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	includeQuery := ""
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/apps/123456789/reviewSubmissions" {
			includeQuery = req.URL.Query().Get("include")
		}
	}
	if includeQuery != "appStoreVersionForReview" {
		t.Fatalf("expected include=appStoreVersionForReview, got %q", includeQuery)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}

	latestSubmission, ok := payload["latestSubmission"].(map[string]any)
	if !ok {
		t.Fatalf("expected latestSubmission object, got %T", payload["latestSubmission"])
	}
	if latestSubmission["id"] != "review-sub-target" {
		t.Fatalf("expected latest submission id review-sub-target, got %v", latestSubmission["id"])
	}
}

func TestReviewStatusPaginatesSubmissionsToSelectedVersion(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"ver-2",
			"attributes":{
				"platform":"IOS",
				"versionString":"2.0.0",
				"appVersionState":"PREPARE_FOR_SUBMISSION"
			},
			"relationships":{
				"app":{"data":{"type":"apps","id":"123456789"}}
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-page-1",
				"attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS","submittedDate":"2026-03-15T02:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-1"}
					}
				}
			}
		],
		"links":{
			"next":"https://api.appstoreconnect.apple.com/v1/apps/123456789/reviewSubmissions?cursor=page-2&filter%5Bplatform%5D=IOS&include=appStoreVersionForReview&limit=200"
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "cursor=page-2", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-target",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-target/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-1",
				"attributes":{"state":"APPROVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-target"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789", "--version-id", "ver-2")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if reviewSubmissionRequests := countStatusRequests(reg, "/v1/apps/123456789/reviewSubmissions"); reviewSubmissionRequests != 2 {
		t.Fatalf("expected 2 review submission page requests, got %d", reviewSubmissionRequests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}

	latestSubmission, ok := payload["latestSubmission"].(map[string]any)
	if !ok {
		t.Fatalf("expected latestSubmission object, got %T", payload["latestSubmission"])
	}
	if latestSubmission["id"] != "review-sub-target" {
		t.Fatalf("expected latest submission id review-sub-target, got %v", latestSubmission["id"])
	}
}

func TestReviewStatusPaginatesVersionsBeforeSelectingLatestTarget(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-1",
				"attributes":{
					"platform":"IOS",
					"versionString":"1.9.9",
					"appVersionState":"WAITING_FOR_REVIEW",
					"createdDate":"2026-03-15T00:00:00Z"
				}
			}
		],
		"links":{
			"next":"https://api.appstoreconnect.apple.com/v1/apps/123456789/appStoreVersions?cursor=page-2&limit=200"
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "cursor=page-2", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"appStoreVersions",
				"id":"ver-2",
				"attributes":{
					"platform":"IOS",
					"versionString":"2.0.0",
					"appVersionState":"PREPARE_FOR_SUBMISSION",
					"createdDate":"2026-03-16T00:00:00Z"
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-2",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/draft-1/items"}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if versionRequests := countStatusRequests(reg, "/v1/apps/123456789/appStoreVersions"); versionRequests != 2 {
		t.Fatalf("expected 2 app store version page requests, got %d", versionRequests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "NOT_SUBMITTED" {
		t.Fatalf("expected reviewState NOT_SUBMITTED, got %v", payload["reviewState"])
	}

	version, ok := payload["version"].(map[string]any)
	if !ok {
		t.Fatalf("expected version object, got %T", payload["version"])
	}
	if version["id"] != "ver-2" {
		t.Fatalf("expected latest version id ver-2, got %v", version["id"])
	}
	if version["version"] != "2.0.0" {
		t.Fatalf("expected latest version string 2.0.0, got %v", version["version"])
	}
	if payload["reviewDetailId"] != "detail-2" {
		t.Fatalf("expected reviewDetailId detail-2, got %v", payload["reviewDetailId"])
	}
}

func TestReviewStatusPrefersNewestSubmissionForSelectedVersion(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"ver-2",
			"attributes":{
				"platform":"IOS",
				"versionString":"2.0.0",
				"appVersionState":"PREPARE_FOR_SUBMISSION"
			},
			"relationships":{
				"app":{"data":{"type":"apps","id":"123456789"}}
			}
		}
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-2/appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreReviewDetails",
			"id":"detail-2",
			"attributes":{"contactEmail":"dev@example.com"}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/123456789/reviewSubmissions", "filter[platform]=IOS", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissions",
				"id":"review-sub-older",
				"attributes":{"state":"UNRESOLVED_ISSUES","platform":"IOS","submittedDate":"2026-03-15T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					}
				}
			},
			{
				"type":"reviewSubmissions",
				"id":"review-sub-newer",
				"attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-03-16T01:00:00Z"},
				"relationships":{
					"appStoreVersionForReview":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-newer/items", "fields[reviewSubmissionItems]=state,appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{
				"type":"reviewSubmissionItems",
				"id":"item-1",
				"attributes":{"state":"APPROVED"},
				"relationships":{
					"appStoreVersion":{
						"data":{"type":"appStoreVersions","id":"ver-2"}
					},
					"reviewSubmission":{
						"data":{"type":"reviewSubmissions","id":"review-sub-newer"}
					}
				}
			}
		],
		"links":{"next":""}
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "status", "--app", "123456789", "--version-id", "ver-2")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["reviewState"] != "COMPLETE" {
		t.Fatalf("expected reviewState COMPLETE, got %v", payload["reviewState"])
	}

	latestSubmission, ok := payload["latestSubmission"].(map[string]any)
	if !ok {
		t.Fatalf("expected latestSubmission object, got %T", payload["latestSubmission"])
	}
	if latestSubmission["id"] != "review-sub-newer" {
		t.Fatalf("expected latest submission id review-sub-newer, got %v", latestSubmission["id"])
	}
}

func TestReviewStatusAndDoctorRejectVersionIDFromDifferentApp(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "review status version/app mismatch",
			args: []string{"review", "status", "--app", "123456789", "--version-id", "ver-1"},
		},
		{
			name: "review doctor version/app mismatch",
			args: []string{"review", "doctor", "--app", "123456789", "--version-id", "ver-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/appStoreVersions/ver-1", "include=app"), asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"ver-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3"},
					"relationships":{"app":{"data":{"type":"apps","id":"999999999"}}}
				}
			}`))

			stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), test.args...)

			if runErr == nil {
				t.Fatal("expected version/app mismatch error")
			}
			if errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected runtime validation error, got ErrHelp")
			}
			if !strings.Contains(runErr.Error(), `version "ver-1" belongs to app "999999999", not "123456789"`) {
				t.Fatalf("expected mismatch error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func countStatusRequests(reg *asctest.Registry, path string) int {
	count := 0
	for _, req := range reg.Requests() {
		if req.URL.Path == path {
			count++
		}
	}
	return count
}
