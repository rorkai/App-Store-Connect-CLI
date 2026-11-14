package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewsListNormalizesAlpha2Territory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "filter[territory]=USA"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"customerReviews","id":"review-1","attributes":{"rating":5,"title":"Great","body":"Nice","reviewerNickname":"Tester","createdDate":"2026-01-20T00:00:00Z","territory":"US"}}
		]
	}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "list", "--app", "app-1", "--territory", "US"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"review-1"`) {
		t.Fatalf("expected review output, got %q", stdout)
	}
}
