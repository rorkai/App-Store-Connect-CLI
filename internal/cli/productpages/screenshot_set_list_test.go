package productpages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type screenshotSetDeadlineRoundTripper struct {
	next      http.RoundTripper
	deadlines *[]time.Duration
}

func (t screenshotSetDeadlineRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if deadline, ok := req.Context().Deadline(); ok {
		*t.deadlines = append(*t.deadlines, time.Until(deadline))
	}
	return t.next.RoundTrip(req)
}

func TestScreenshotSetListResultFetchesNestedScreenshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || (req.URL.Path != "/v1/appScreenshotSets/set-1/appScreenshots" && req.URL.Path != "/v1/appScreenshotSets/set-1/relationships/appScreenshots") {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots" {
			_, _ = io.WriteString(w, `{"data":[{"type":"appScreenshots","id":"screenshot-1"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"type":"appScreenshots","id":"screenshot-1","attributes":{"fileName":"01-home.png"}}]}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	httpClient := server.Client()
	httpClient.Transport = customPageBaseURLRoundTripper{target: serverURL, next: httpClient.Transport}
	client := newCustomPageTestClientWithHTTPClient(t, httpClient)

	result, err := screenshotSetListResult(context.Background(), client, "custom-loc-1", &asc.AppScreenshotSetsResponse{
		Data: []asc.Resource[asc.AppScreenshotSetAttributes]{
			{ID: "set-1", Attributes: asc.AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_65"}},
		},
	})
	if err != nil {
		t.Fatalf("screenshotSetListResult() error: %v", err)
	}
	if result.LocalizationID != "custom-loc-1" || len(result.Sets) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Sets[0].Screenshots) != 1 || result.Sets[0].Screenshots[0].ID != "screenshot-1" {
		t.Fatalf("nested screenshots = %#v", result.Sets[0].Screenshots)
	}
}

func TestScreenshotSetListResultRenewsNestedRequestTimeouts(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "500ms")

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || !strings.HasPrefix(req.URL.Path, "/v1/appScreenshotSets/set-") {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		callCount++
		if callCount == 1 {
			time.Sleep(250 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	})

	deadlines := make([]time.Duration, 0, 4)
	httpClient := &http.Client{Transport: screenshotSetDeadlineRoundTripper{
		deadlines: &deadlines,
		next: customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			return response.Result(), nil
		}),
	}}
	client := newCustomPageTestClientWithHTTPClient(t, httpClient)

	var err error
	synctest.Test(t, func(*testing.T) {
		_, err = screenshotSetListResult(context.Background(), client, "custom-loc-1", &asc.AppScreenshotSetsResponse{
			Data: []asc.Resource[asc.AppScreenshotSetAttributes]{
				{ID: "set-1"},
				{ID: "set-2"},
			},
		})
	})
	if err != nil {
		t.Fatalf("screenshotSetListResult() error: %v", err)
	}
	if len(deadlines) != 4 {
		t.Fatalf("recorded %d nested request deadlines, want 4", len(deadlines))
	}
	if deadlines[1] != 500*time.Millisecond {
		t.Fatalf("second nested request inherited the first request deadline: %s remaining, want 500ms", deadlines[1])
	}
}
