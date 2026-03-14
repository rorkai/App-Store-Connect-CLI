package shared

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type appResolutionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f appResolutionRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newAppResolutionTestClient(t *testing.T, transport appResolutionRoundTripFunc) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}

	return client
}

func appResolutionJSONResponse(body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func captureAppResolutionStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stderr = writePipe
	defer func() {
		os.Stderr = oldStderr
	}()

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("writePipe.Close() error: %v", err)
	}

	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("io.ReadAll() error: %v", err)
	}
	if err := readPipe.Close(); err != nil {
		t.Fatalf("readPipe.Close() error: %v", err)
	}

	return string(output)
}

func TestResolveAppStoreVersionIDAndState_PrefersAppVersionState(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			t.Fatalf("expected /v1/apps/app-1/appStoreVersions, got %s", req.URL.Path)
		}

		query := req.URL.Query()
		if query.Get("filter[versionString]") != "1.2.3" {
			t.Fatalf("expected filter[versionString]=1.2.3, got %q", query.Get("filter[versionString]"))
		}
		if query.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
		}
		if query.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", query.Get("limit"))
		}

		return appResolutionJSONResponse(`{"data":[{"type":"appStoreVersions","id":"ver-123","attributes":{"appVersionState":"PREORDER_READY_FOR_SALE","appStoreState":"READY_FOR_SALE"}}]}`)
	})

	versionID, versionState, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.2.3", "IOS")
	if err != nil {
		t.Fatalf("ResolveAppStoreVersionIDAndState() error: %v", err)
	}
	if versionID != "ver-123" {
		t.Fatalf("expected version ID ver-123, got %q", versionID)
	}
	if versionState != "PREORDER_READY_FOR_SALE" {
		t.Fatalf("expected state PREORDER_READY_FOR_SALE, got %q", versionState)
	}
}

func TestResolveAppStoreVersionIDAndState_FallsBackToTrimmedAppStoreState(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[{"type":"appStoreVersions","id":"ver-456","attributes":{"appVersionState":"   ","appStoreState":" READY_FOR_SALE "}}]}`)
	})

	_, versionState, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.2.3", "IOS")
	if err != nil {
		t.Fatalf("ResolveAppStoreVersionIDAndState() error: %v", err)
	}
	if versionState != "READY_FOR_SALE" {
		t.Fatalf("expected fallback state READY_FOR_SALE, got %q", versionState)
	}
}

func TestResolveAppInfoID_ExplicitOverride(t *testing.T) {
	id, err := ResolveAppInfoID(context.Background(), nil, "app-1", "explicit-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "explicit-id" {
		t.Fatalf("expected explicit-id, got %q", id)
	}
}

func TestResolveAppInfoID_SingleAppInfo(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_SALE"}}]}`)
	})

	id, err := ResolveAppInfoID(context.Background(), client, "app-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "info-1" {
		t.Fatalf("expected info-1, got %q", id)
	}
}

func TestResolveAppInfoID_AutoSelectsEditableFromMultiple(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[
			{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_SALE"}},
			{"type":"appInfos","id":"info-editable","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
		]}`)
	})

	var (
		id  string
		err error
	)
	stderr := captureAppResolutionStderr(t, func() {
		id, err = ResolveAppInfoID(context.Background(), client, "app-1", "")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "info-editable" {
		t.Fatalf("expected info-editable, got %q", id)
	}
	if !strings.Contains(stderr, "auto-selected info-editable (PREPARE_FOR_SUBMISSION)") {
		t.Fatalf("expected PREPARE_FOR_SUBMISSION selection message, got %q", stderr)
	}
}

func TestResolveAppInfoID_ReturnsErrorWhenMultipleRemainAmbiguous(t *testing.T) {
	testCases := []struct {
		name           string
		responseBody   string
		wantSubstrings []string
	}{
		{
			name: "non-live without prepare for submission",
			responseBody: `{"data":[
				{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_SALE"}},
				{"type":"appInfos","id":"info-review","attributes":{"state":"IN_REVIEW"}}
			]}`,
			wantSubstrings: []string{`multiple app infos found for app "app-1"`, "READY_FOR_SALE", "IN_REVIEW"},
		},
		{
			name: "all live candidates",
			responseBody: `{"data":[
				{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_SALE"}},
				{"type":"appInfos","id":"info-2","attributes":{"state":"READY_FOR_DISTRIBUTION"}}
			]}`,
			wantSubstrings: []string{`multiple app infos found for app "app-1"`, "READY_FOR_SALE", "READY_FOR_DISTRIBUTION"},
		},
		{
			name: "multiple prepare for submission candidates",
			responseBody: `{"data":[
				{"type":"appInfos","id":"info-ios","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
				{"type":"appInfos","id":"info-macos","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
			]}`,
			wantSubstrings: []string{`multiple app infos found for app "app-1"`, "info-ios", "info-macos", "PREPARE_FOR_SUBMISSION"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
				return appResolutionJSONResponse(tc.responseBody)
			})

			_, err := ResolveAppInfoID(context.Background(), client, "app-1", "")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			for _, want := range tc.wantSubstrings {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected error to contain %q, got %v", want, err)
				}
			}
		})
	}
}

func TestResolveAppInfoID_NoAppInfos(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[]}`)
	})

	_, err := ResolveAppInfoID(context.Background(), client, "app-1", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no app info found") {
		t.Fatalf("expected 'no app info found' error, got: %v", err)
	}
}
