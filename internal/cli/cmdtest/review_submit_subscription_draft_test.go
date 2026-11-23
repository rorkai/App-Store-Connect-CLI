package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestReviewSubmitSkippedSubscriptionDraftNamesSubmissionsSubmit covers the
// draft `asc web review subscriptions attach --review-items` builds when the
// app already has an approved subscription: only subscription and subscription
// group versions, no app version. review submit skips it, and must tell the
// caller to submit it on its own, not to cancel what attach just staged. A
// draft with any other item keeps the cancel advice.
func TestReviewSubmitSkippedSubscriptionDraftNamesSubmissionsSubmit(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	registerSubmitPreflightAppInfos(reg, completeAgeRatingAttributes)
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=UNRESOLVED_ISSUES", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=READY_FOR_REVIEW", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"draft-subs","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}},{"type":"reviewSubmissions","id":"draft-mixed","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/draft-subs/items", "fields[reviewSubmissionItems]=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-s","relationships":{"appStoreVersion":{"data":null},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"sub-ver-1"}}}},{"type":"reviewSubmissionItems","id":"item-g","relationships":{"appStoreVersion":{"data":null},"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"group-ver-1"}}}}],"links":{"self":"/v1/reviewSubmissions/draft-subs/items"}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/draft-mixed/items", "fields[reviewSubmissionItems]=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-s2","relationships":{"appStoreVersion":{"data":null},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"sub-ver-2"}}}},{"type":"reviewSubmissionItems","id":"item-e","relationships":{"appStoreVersion":{"data":null},"appEvent":{"data":{"type":"appEvents","id":"event-1"}}}}],"links":{"self":"/v1/reviewSubmissions/draft-mixed/items"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersions/version-1/relationships/build").JSONBody(`{"data":{"type":"builds","id":"build-1"}}`), asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.POST("/v1/reviewSubmissions").JSONBody(`{"data":{"type":"reviewSubmissions","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`))
	reg.Register(asctest.POST("/v1/reviewSubmissionItems").JSONBody(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"review-sub-1"}},"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-sub-1").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"submitted":true}}}`), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-04-08T00:00:00Z"}}}`))

	_, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "submit",
		"--app", "app-1",
		"--version", "1.2.3",
		"--build-id", "build-1",
		"--confirm",
		"--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	for _, want := range []string{
		"Skipped review submission draft-subs because it holds only subscription review items and not version version-1. Submit it on its own with `asc review submissions-submit --id draft-subs --confirm`.",
		"Skipped stale review submission draft-mixed because it is not exclusively usable for version version-1. Cancel it explicitly with `asc submit cancel --id draft-mixed --confirm`",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "--id draft-subs --confirm` if you intend to replace it") || strings.Contains(stderr, "cancel --id draft-subs") {
		t.Fatalf("stderr = %q, must not tell the caller to cancel the subscription-only draft", stderr)
	}
}
