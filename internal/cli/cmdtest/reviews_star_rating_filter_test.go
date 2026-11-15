package cmdtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

const starRatingReviewsPayload = `{"data":[{"type":"customerReviews","id":"review-1","attributes":{"rating":1,"title":"Bug","body":"Crashes","reviewerNickname":"Tester","createdDate":"2026-01-20T00:00:00Z","territory":"USA"}}]}`

func TestReviewsListEmitsStarRatingFilter(t *testing.T) {
	tests := []struct {
		name       string
		stars      string
		wantRating string
	}{
		{name: "single rating stays supported", stars: "3", wantRating: "3"},
		{name: "multiple ratings", stars: "1,2", wantRating: "1,2"},
		{name: "spacing and duplicates normalized", stars: " 5 , 4 , 5 ", wantRating: "5,4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/customerReviews" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				if got := req.URL.Query().Get("filter[rating]"); got != test.wantRating {
					t.Errorf("filter[rating] = %q, want %q", got, test.wantRating)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, starRatingReviewsPayload)
			}))
			t.Cleanup(server.Close)

			stdout, stderr, code := runWithDeps(t, stubServerClient(t, server), "reviews", "list", "--app", "app-1", "--stars", test.stars, "--output", "json")
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d (stderr routed below)", code, rootcmd.ExitSuccess)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"id":"review-1"`) {
				t.Fatalf("expected review envelope on stdout, got %q", stdout)
			}
			if requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})
	}
}

func TestReviewsListOmitsStarRatingFilterWhenUnset(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, ok := req.URL.Query()["filter[rating]"]; ok {
			t.Errorf("filter[rating] must be absent, got %q", req.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, starRatingReviewsPayload)
	}))
	t.Cleanup(server.Close)

	_, stderr, code := runWithDeps(t, stubServerClient(t, server), "reviews", "list", "--app", "app-1", "--output", "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestReviewsListRejectsInvalidStarRatings(t *testing.T) {
	tests := []struct {
		name  string
		stars string
	}{
		{name: "explicitly empty", stars: ""},
		{name: "whitespace only", stars: "   "},
		{name: "above range", stars: "6"},
		{name: "below range", stars: "0"},
		{name: "one bad element", stars: "1,9"},
		{name: "not a number", stars: "five"},
		{name: "no usable elements", stars: ","},
		{name: "repeated comma", stars: "1,,2"},
		{name: "trailing comma", stars: "1,"},
		{name: "leading comma", stars: ",1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, starRatingReviewsPayload)
			}))
			t.Cleanup(server.Close)

			stdout, stderr, code := runWithDeps(t, stubServerClient(t, server), "reviews", "list", "--app", "app-1", "--stars", test.stars, "--output", "json")
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--stars must be a comma-separated list of star ratings: 1, 2, 3, 4, 5") {
				t.Fatalf("expected --stars guidance listing valid values, got %q", stderr)
			}
			if requests != 0 {
				t.Fatalf("request count = %d, want 0 (validation must precede the request)", requests)
			}
		})
	}
}
