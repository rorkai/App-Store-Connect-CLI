package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxReviewsPromotedListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"sandbox", "list"}, wantErr: "sandbox list: --next"},
		{args: []string{"subscriptions", "promoted-purchases", "list"}, wantErr: "subscriptions promoted-purchases list: --next"},
		{args: []string{"reviews", "list"}, wantErr: "reviews: --next"},
		{args: []string{"reviews", "summarizations"}, wantErr: "reviews summarizations: --next"},
	})
}

func TestSandboxReviewsPromotedListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"sandbox", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=AQ&limit=200",
			dataType: "sandboxTesters",
		},
		{
			args:     []string{"reviews", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews?cursor=AQ&limit=200",
			dataType: "customerReviews",
		},
		{
			args:     []string{"reviews", "summarizations"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviewSummarizations?cursor=AQ&limit=200",
			dataType: "customerReviewSummarizations",
		},
	})
}

func TestPromotedPurchasesListPaginateFromNextWithoutApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"promotedPurchases","id":"promo-next-1","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-next-1"}}}}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"promotedPurchases","id":"promo-next-2","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-next-2"}}}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "promoted-purchases", "list", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"promo-next-1"`) || !strings.Contains(stdout, `"id":"promo-next-2"`) {
		t.Fatalf("expected paginated promoted purchases in output, got %q", stdout)
	}
}
