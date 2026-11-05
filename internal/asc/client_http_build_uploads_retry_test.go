package asc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Apple intermittently answers Build Upload API reads with 404 NOT_FOUND for
// apps that certainly exist (fastlane/fastlane#29908). These tests pin the
// bounded retry to that family of paths and to reads only.

const buildUploadsFlakyNotFoundBody = `{"errors":[{"id":"e1","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '6759231657'"}]}`

func newBuildUploadsNotFoundServer(t *testing.T, failures int, attempts *atomic.Int32) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempt := int(attempts.Add(1))
		w.Header().Set("Content-Type", "application/json")
		if attempt <= failures {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, buildUploadsFlakyNotFoundBody)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"type":"buildUploads","id":"bu-1"}]}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestClientDo_RetriesIntermittentNotFoundOnBuildUploads(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 2, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	data, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/buildUploads/bu-1", nil)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if got, want := string(data), `{"data":[{"type":"buildUploads","id":"bu-1"}]}`; got != want {
		t.Fatalf("do() = %q, want %q", got, want)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (404, 404, 200), got %d", got)
	}
}

func TestClientDo_BuildUploadsNotFoundRetryIsBoundedAndSurfacesAppleDetail(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 4, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	_, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/buildUploads/bu-1", nil)
	if err == nil {
		t.Fatal("expected error after exhausting the retry budget")
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("expected 4 attempts (1 + 3 retries), got %d", got)
	}
	if !IsRetryBudgetExhausted(err) {
		t.Fatalf("expected retry budget marker, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError in chain, got %T: %v", err, err)
	}
	if apiErr.Code != "NOT_FOUND" || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected NOT_FOUND/404, got %s/%d", apiErr.Code, apiErr.StatusCode)
	}
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound to see through the retry wrappers: %v", err)
	}
	if !strings.Contains(err.Error(), "There is no resource of type 'apps' with id '6759231657'") {
		t.Fatalf("expected Apple's detail in error, got %v", err)
	}
}

func TestClientDo_DoesNotRetryOtherNotFoundResponses(t *testing.T) {
	setFastRetryEnv(t, "3")

	notFound := func(code, detail string) string {
		return `{"errors":[{"status":"404","code":"` + code + `","title":"The specified resource does not exist","detail":"` + detail + `"}]}`
	}
	const appDetail = "There is no resource of type 'apps' with id '6759231657'"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"app builds", http.MethodGet, "/v1/apps/6759231657/builds", buildUploadsFlakyNotFoundBody},
		{"app", http.MethodGet, "/v1/apps/6759231657", buildUploadsFlakyNotFoundBody},
		{"build", http.MethodGet, "/v1/builds/b-1", buildUploadsFlakyNotFoundBody},
		{"lookalike collection", http.MethodGet, "/v1/buildUploadsArchive/x", buildUploadsFlakyNotFoundBody},
		{"app build uploads subpath", http.MethodGet, "/v1/apps/6759231657/buildUploads/extra", buildUploadsFlakyNotFoundBody},
		{"create upload", http.MethodPost, "/v1/buildUploads", buildUploadsFlakyNotFoundBody},
		{"create upload file", http.MethodPost, "/v1/buildUploadFiles", buildUploadsFlakyNotFoundBody},
		{"update upload file", http.MethodPatch, "/v1/buildUploadFiles/f-1", buildUploadsFlakyNotFoundBody},
		{"delete upload", http.MethodDelete, "/v1/buildUploads/bu-1", buildUploadsFlakyNotFoundBody},
		{"non-NOT_FOUND code", http.MethodGet, "/v1/buildUploads/bu-1", notFound("PATH_ERROR", appDetail)},
		// A genuinely missing upload or file id names that resource, not the app.
		{"missing upload", http.MethodGet, "/v1/buildUploads/bu-missing", notFound("NOT_FOUND", "There is no resource of type 'buildUploads' with id 'bu-missing'")},
		{"missing upload files parent", http.MethodGet, "/v1/buildUploads/bu-missing/buildUploadFiles", notFound("NOT_FOUND", "There is no resource of type 'buildUploads' with id 'bu-missing'")},
		{"missing upload file", http.MethodGet, "/v1/buildUploadFiles/f-missing", notFound("NOT_FOUND", "There is no resource of type 'buildUploadFiles' with id 'f-missing'")},
		{"missing upload without detail", http.MethodGet, "/v1/buildUploads/bu-missing", notFound("NOT_FOUND", "")},
		{"app uploads without detail", http.MethodGet, "/v1/apps/6759231657/buildUploads", notFound("NOT_FOUND", "")},
		{"app uploads naming app", http.MethodGet, "/v1/apps/6759231657/buildUploads", notFound("NOT_FOUND", appDetail)},
		{"app upload linkages without detail", http.MethodGet, "/v1/apps/6759231657/relationships/buildUploads", notFound("NOT_FOUND", "")},
		{"app upload linkages naming app", http.MethodGet, "/v1/apps/6759231657/relationships/buildUploads", notFound("NOT_FOUND", appDetail)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)
			client := newMutationRetryTestClient(t, server.Client())

			var body io.Reader
			if tc.method == http.MethodPost || tc.method == http.MethodPatch {
				body = strings.NewReader(`{"data":{"type":"buildUploads"}}`)
			}
			_, err := client.do(context.Background(), tc.method, server.URL+tc.path, body)
			if err == nil {
				t.Fatal("expected 404 error")
			}
			if got := attempts.Load(); got != 1 {
				t.Fatalf("attempts = %d, want 1", got)
			}
			if IsRetryable(err) || IsRetryBudgetExhausted(err) {
				t.Fatalf("expected a terminal 404, got %v", err)
			}
			if !IsNotFound(err) {
				t.Fatalf("expected not-found error, got %v", err)
			}
		})
	}
}

func TestGetBuildUploadsDoesNotRetryWhenAppIsMissing(t *testing.T) {
	setFastRetryEnv(t, "3")

	var paths []string
	client := newTestClient(
		t, func(req *http.Request) {
			paths = append(paths, req.URL.Path)
		},
		jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody),
		jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody),
	)

	_, err := client.GetBuildUploads(context.Background(), "6759231657")
	if err == nil {
		t.Fatal("expected missing-app error")
	}
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if IsRetryable(err) || IsRetryBudgetExhausted(err) {
		t.Fatalf("expected terminal missing-app error, got %v", err)
	}
	if want := []string{"/v1/apps/6759231657/buildUploads", "/v1/apps/6759231657"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestGetBuildUploadsRetriesWhenAppVerificationFailsTransiently(t *testing.T) {
	setFastRetryEnv(t, "3")

	var paths []string
	client := newTestClient(
		t, func(req *http.Request) {
			paths = append(paths, req.URL.Path)
		},
		jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody),
		jsonResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"Temporary failure"}]}`),
		jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody),
		jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"6759231657"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"bu-1"}]}`),
	)

	response, err := client.GetBuildUploads(context.Background(), "6759231657")
	if err != nil {
		t.Fatalf("GetBuildUploads() error: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "bu-1" {
		t.Fatalf("response data = %#v, want bu-1", response.Data)
	}
	if want := []string{
		"/v1/apps/6759231657/buildUploads",
		"/v1/apps/6759231657",
		"/v1/apps/6759231657/buildUploads",
		"/v1/apps/6759231657",
		"/v1/apps/6759231657/buildUploads",
	}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestGetBuildUploadsMaxRetriesZeroSkipsAppVerification(t *testing.T) {
	setFastRetryEnv(t, "0")

	var paths []string
	client := newTestClient(t, func(req *http.Request) {
		paths = append(paths, req.URL.Path)
	}, jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody))

	_, err := client.GetBuildUploads(context.Background(), "6759231657")
	if err == nil {
		t.Fatal("expected build-uploads error")
	}
	if !IsNotFound(err) || IsRetryable(err) || IsRetryBudgetExhausted(err) {
		t.Fatalf("expected original terminal 404, got %v", err)
	}
	if want := []string{"/v1/apps/6759231657/buildUploads"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestGetAppBuildUploadsRelationshipsRetriesWhenAppExists(t *testing.T) {
	setFastRetryEnv(t, "3")

	var paths []string
	client := newTestClient(
		t, func(req *http.Request) {
			paths = append(paths, req.URL.Path)
		},
		jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody),
		jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"6759231657"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"bu-1"}]}`),
	)

	response, err := client.GetAppBuildUploadsRelationships(context.Background(), "6759231657")
	if err != nil {
		t.Fatalf("GetAppBuildUploadsRelationships() error: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "bu-1" {
		t.Fatalf("response data = %#v, want bu-1", response.Data)
	}
	if want := []string{
		"/v1/apps/6759231657/relationships/buildUploads",
		"/v1/apps/6759231657",
		"/v1/apps/6759231657/relationships/buildUploads",
	}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestGetBuildUploadsNextURLOnlyRetriesWhenAppExists(t *testing.T) {
	setFastRetryEnv(t, "3")

	var paths []string
	client := newTestClient(
		t, func(req *http.Request) {
			paths = append(paths, req.URL.Path)
		},
		jsonResponse(http.StatusNotFound, buildUploadsFlakyNotFoundBody),
		jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"6759231657"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"bu-2"}]}`),
	)

	response, err := client.GetBuildUploads(
		context.Background(),
		"",
		WithBuildUploadsNextURL("https://api.appstoreconnect.apple.com/v1/apps/6759231657/buildUploads?cursor=page-2"),
	)
	if err != nil {
		t.Fatalf("GetBuildUploads() error: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "bu-2" {
		t.Fatalf("response data = %#v, want bu-2", response.Data)
	}
	if want := []string{
		"/v1/apps/6759231657/buildUploads",
		"/v1/apps/6759231657",
		"/v1/apps/6759231657/buildUploads",
	}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestGetBuildUploadsTopLevelNextURLDoesNotVerifyCallerApp(t *testing.T) {
	setFastRetryEnv(t, "3")

	var paths []string
	client := newTestClient(t, func(req *http.Request) {
		paths = append(paths, req.URL.Path)
	}, jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'buildUploads' with id 'bu-missing'"}]}`))

	_, err := client.GetBuildUploads(
		context.Background(),
		"6759231657",
		WithBuildUploadsNextURL("https://api.appstoreconnect.apple.com/v1/buildUploads/bu-missing"),
	)
	if err == nil {
		t.Fatal("expected missing-upload error")
	}
	if !IsNotFound(err) || IsRetryable(err) || IsRetryBudgetExhausted(err) {
		t.Fatalf("expected terminal missing-upload error, got %v", err)
	}
	if want := []string{"/v1/buildUploads/bu-missing"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestAppIDFromBuildUploadsPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/v1/apps/6759231657/buildUploads", "6759231657"},
		{"/v1/apps/6759231657/relationships/buildUploads", "6759231657"},
		{"https://api.appstoreconnect.apple.com/v1/apps/6759231657/buildUploads?cursor=page-2", "6759231657"},
		{"/v1/apps/6759231657/buildUploads/extra", ""},
		{"/v1/apps/6759231657/builds", ""},
		{"/v1/buildUploads/bu-1", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := appIDFromBuildUploadsPath(tc.path); got != tc.want {
				t.Fatalf("appIDFromBuildUploadsPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestNotFoundNamesAppResource(t *testing.T) {
	for _, tc := range []struct {
		detail string
		want   bool
	}{
		{"There is no resource of type 'apps' with id '6759231657'", true},
		{"There is no resource of type \u2018apps\u2019 with id \u20186759231657\u2019", true},
		{"there is no resource of type apps with id 1", true},
		{"There is no resource of type 'buildUploads' with id 'bu-1'", false},
		{"There is no resource of type 'buildUploadFiles' with id 'f-1'", false},
		{"There is no resource of type 'appStoreVersions' with id 'v-1'", false},
		{"", false},
	} {
		t.Run(tc.detail, func(t *testing.T) {
			if got := notFoundNamesAppResource(&APIError{Detail: tc.detail}); got != tc.want {
				t.Fatalf("notFoundNamesAppResource(%q) = %v, want %v", tc.detail, got, tc.want)
			}
		})
	}
}

func TestClientDo_BuildUploadsNotFoundRetryHonorsContextCancellation(t *testing.T) {
	setFastRetryEnv(t, "3")
	t.Setenv("ASC_BASE_DELAY", "5s")
	t.Setenv("ASC_MAX_DELAY", "5s")
	resetConfigCacheForTest()

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 4, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := client.do(ctx, http.MethodGet, server.URL+"/v1/buildUploads/bu-1", nil)
	if err == nil {
		t.Fatal("expected error after cancellation")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("expected cancellation to interrupt the backoff, waited %s", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in chain, got %v", err)
	}
	if !IsNotFound(err) {
		t.Fatalf("expected the last 404 to be preserved, got %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected a single attempt before cancellation, got %d", got)
	}
}

func TestClientDo_RespectsMaxRetriesZeroForBuildUploadsNotFound(t *testing.T) {
	setFastRetryEnv(t, "0")

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 4, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	_, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/buildUploads/bu-1", nil)
	if err == nil {
		t.Fatal("expected 404 error")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected a single attempt with ASC_MAX_RETRIES=0, got %d", got)
	}
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestIsBuildUploadsPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/v1/buildUploads", true},
		{"/v1/buildUploads/bu-1", true},
		{"/v1/buildUploads/bu-1/buildUploadFiles?limit=5", true},
		{"/v1/buildUploads/bu-1/relationships/buildUploadFiles", true},
		{"/v1/buildUploadFiles", true},
		{"/v1/buildUploadFiles/f-1", true},
		{"/v1/apps/6759231657/buildUploads", false},
		{"/v1/apps/6759231657/buildUploads?filter[cfBundleVersion]=22", false},
		{"/v1/apps/6759231657/relationships/buildUploads", false},
		{"https://api.appstoreconnect.apple.com/v1/apps/6759231657/buildUploads?cursor=abc", false},
		{"https://api.appstoreconnect.apple.com/v1/buildUploads/bu-1", true},
		{"", false},
		{"/v1/apps/6759231657", false},
		{"/v1/apps/6759231657/builds", false},
		{"/v1/apps/6759231657/buildUploads/extra", false},
		{"/v1/buildUploadsArchive", false},
		{"/v1/buildUploadFilesX/f-1", false},
		{"/v1/builds/b-1", false},
		{"/v2/buildUploads", false},
		{"https://api.appstoreconnect.apple.com/v1/builds?filter[app]=1", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := isBuildUploadsPath(tc.path); got != tc.want {
				t.Fatalf("isBuildUploadsPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
