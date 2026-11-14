package asc

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetAppStoreReviewAttachmentsForReviewDetail_UsesNextURLWithoutReviewDetail(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments?cursor=abc"
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"`+next+`"}}`))

	if _, err := client.GetAppStoreReviewAttachmentsForReviewDetail(
		context.Background(),
		"",
		NextURL[AppStoreReviewAttachmentsOption](next),
	); err != nil {
		t.Fatalf("GetAppStoreReviewAttachmentsForReviewDetail() error: %v", err)
	}
}

func TestGetAppStoreReviewAttachmentsForReviewDetail_RejectsNextURLWithSelectorOrQueryOptions(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments?cursor=abc"
	tests := []struct {
		name           string
		reviewDetailID string
		opts           []AppStoreReviewAttachmentsOption
	}{
		{name: "review detail ID", reviewDetailID: "detail-1"},
		{name: "fields", opts: []AppStoreReviewAttachmentsOption{WithAppStoreReviewAttachmentsFields([]string{"fileName"})}},
		{name: "review detail fields", opts: []AppStoreReviewAttachmentsOption{WithAppStoreReviewAttachmentReviewDetailFields([]string{"notes"})}},
		{name: "include", opts: []AppStoreReviewAttachmentsOption{WithAppStoreReviewAttachmentsInclude([]string{"appStoreReviewDetail"})}},
		{name: "limit", opts: []AppStoreReviewAttachmentsOption{Limit[AppStoreReviewAttachmentsOption](25)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := newTestClient(t, func(*http.Request) { requests++ }, jsonResponse(http.StatusOK, `{"data":[]}`))
			opts := append([]AppStoreReviewAttachmentsOption{NextURL[AppStoreReviewAttachmentsOption](next)}, test.opts...)

			_, err := client.GetAppStoreReviewAttachmentsForReviewDetail(context.Background(), test.reviewDetailID, opts...)
			if err == nil || !strings.Contains(err.Error(), "next URL cannot be combined with") {
				t.Fatalf("error = %v, want next URL conflict", err)
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
		})
	}
}
