package web

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestRollbackReviewSubscriptionItemsRunsAfterTheCommandContextEnds covers a
// timed-out or interrupted add: the command's context is already canceled when
// the rollback starts, and the rollback must still remove the item this run
// added and cancel the draft it created.
func TestRollbackReviewSubscriptionItemsRunsAfterTheCommandContextEnds(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.DELETE("/v1/reviewSubmissionItems/item-added"), asctest.JSON(http.StatusNoContent, ``))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/rs-new").JSONBody(`{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"canceled":true}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"platform":"IOS","state":"CANCELING"}}}`))
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			t.Errorf("%s %s sent on an ended context: %v", req.Method, req.URL.Path, err)
		}
		return reg.RoundTrip(req)
	})
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeAppCreateTestPEM(t, keyPath)
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	d, _, _ := newWebTestDeps()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	addErr := errors.New("add subscriptionVersions \"ver-1\": context canceled")
	items := []reviewSubscriptionReviewItem{
		{ItemType: "subscriptionGroupVersions", ResourceID: "gv-1", ReviewSubmissionItemID: "item-existing"},
		{ItemType: "appStoreVersions", ResourceID: "version-1", ReviewSubmissionItemID: "item-added", Added: true},
	}
	err = rollbackReviewSubscriptionItems(ctx, d, client, reviewSubscriptionDraftChoice{ID: "rs-new", Created: true}, items, addErr)

	if !errors.Is(err, addErr) {
		t.Fatalf("rollback error = %v, want it to wrap the add error", err)
	}
	for _, want := range []string{"removed review item item-added", "canceled review submission rs-new, which this run created"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("rollback error = %q, want %q", err, want)
		}
	}
	if got := len(reg.Requests()); got != 2 {
		t.Fatalf("sent %d requests, want the DELETE and the cancel only", got)
	}
}
