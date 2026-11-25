package shared

import (
	"context"
	"errors"
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

func TestResolveOwnedAppStoreVersionByIDRejectsRedirectBeforeScopedValidation(t *testing.T) {
	requests := 0
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if requests > 1 {
			t.Fatalf("ownership redirect was followed: %s", req.URL.RequestURI())
		}
		response := &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Status:     http.StatusText(http.StatusTemporaryRedirect),
			Header: http.Header{
				"Location": []string{"/v1/apps/foreign/appStoreVersions"},
			},
			Body:    io.NopCloser(strings.NewReader("")),
			Request: req,
		}
		return response, nil
	})

	_, err := ResolveOwnedAppStoreVersionByID(context.Background(), client, "app-1", "version-1", "IOS")
	if err == nil {
		t.Fatal("ResolveOwnedAppStoreVersionByID() succeeded after a redirect response")
	}
	if requests != 1 {
		t.Fatalf("ownership requests = %d, want one request and no redirect follow-up", requests)
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

func TestResolveAppStoreVersionIDAndStateRejectsSingleSample(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[{"type":"appStoreVersions","id":"ver-456","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=next"}}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.2.3", "IOS")
	if err == nil || !strings.Contains(err.Error(), "sample matches") {
		t.Fatalf("expected incomplete-page ambiguity, got %v", err)
	}
}

func TestResolveAppStoreVersionIDAndStateRejectsEmptySample(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=next"}}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "1.2.3", "IOS")
	if err == nil || !strings.Contains(err.Error(), "sample matches") {
		t.Fatalf("expected incomplete-page ambiguity, got %v", err)
	}
}

func TestResolveAppStoreVersionIDAndState_EmptyCollectionIsNotFound(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[]}`)
	})

	_, _, err := ResolveAppStoreVersionIDAndState(context.Background(), client, "app-1", "9.9.9", "IOS")
	if !errors.Is(err, asc.ErrNotFound) {
		t.Fatalf("ResolveAppStoreVersionIDAndState() error = %v, want asc.ErrNotFound", err)
	}
}

func TestResolveAppInfoID_ExplicitDirectID(t *testing.T) {
	id, err := DefaultDeps().ResolveAppInfoID(context.Background(), nil, "", "explicit-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "explicit-id" {
		t.Fatalf("expected explicit-id, got %q", id)
	}
}

func TestResolveOwnedAppInfoID_PreservesDirectIDOnlyTarget(t *testing.T) {
	id, err := DefaultDeps().ResolveOwnedAppInfoID(context.Background(), nil, "", "explicit-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "explicit-id" {
		t.Fatalf("expected explicit-id, got %q", id)
	}
}

func TestResolveAppInfoIDRejectsMalformedOwnedResource(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "matching resource",
			body: `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
		},
		{
			name: "wrong resource type",
			body: `{"data":{"type":"apps","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `expected resource type "appInfos", got "apps"`,
		},
		{
			name: "wrong resource ID",
			body: `{"data":{"type":"appInfos","id":"info-2","relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`,
			want: `expected app info "info-1", got "info-2"`,
		},
		{
			name: "wrong app relationship type",
			body: `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"builds","id":"123456789"}}}}}`,
			want: `expected app relationship type "apps", got "builds"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/v1/appInfos/info-1" || req.URL.Query().Get("include") != "app" {
					t.Fatalf("unexpected ownership request: %s", req.URL.RequestURI())
				}
				return appResolutionJSONResponse(test.body)
			})

			id, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "info-1")
			if test.want == "" {
				if err != nil || id != "info-1" {
					t.Fatalf("ResolveAppInfoID() = %q, %v; want info-1", id, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ResolveAppInfoID() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveAppInfoIDResolvesConfiguredBundleBeforeOwnership(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			if got := req.URL.Query().Get("filter[bundleId]"); got != "com.example.owner" {
				t.Fatalf("bundle lookup filter = %q, want com.example.owner", got)
			}
			return appResolutionJSONResponse(`{"data":[{"type":"apps","id":"123456789","attributes":{"bundleId":"com.example.owner"}}]}`)
		case "/v1/appInfos/info-1":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("ownership lookup must include app, got %q", req.URL.RawQuery)
			}
			return appResolutionJSONResponse(`{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		default:
			t.Fatalf("unexpected request: %s", req.URL.RequestURI())
			return nil, nil
		}
	})

	deps := DefaultDeps()
	deps.LookupEnv = func(name string) (string, bool) {
		if name == "ASC_APP_ID" {
			return "com.example.owner", true
		}
		return "", false
	}
	resolvedAppID := deps.ResolveAppID("")
	if resolvedAppID != "com.example.owner" {
		t.Fatalf("ResolveAppID() = %q, want com.example.owner", resolvedAppID)
	}
	id, err := deps.ResolveAppInfoID(context.Background(), client, resolvedAppID, "info-1")
	if err != nil {
		t.Fatalf("ResolveAppInfoID() error = %v", err)
	}
	if id != "info-1" {
		t.Fatalf("ResolveAppInfoID() = %q, want info-1", id)
	}
}

func TestResolveAppInfoID_SingleAppInfo(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_SALE"}}]}`)
	})

	id, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "info-1" {
		t.Fatalf("expected info-1, got %q", id)
	}
}

func TestResolveAppInfoIDRejectsSingleSample(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_SALE"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appInfos?cursor=next"}}`)
	})

	_, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
	if err == nil || !strings.Contains(err.Error(), "sample matches") {
		t.Fatalf("expected incomplete-page ambiguity, got %v", err)
	}
}

func TestResolveAppInfoIDRejectsEmptySample(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appInfos?cursor=next"}}`)
	})

	_, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
	if err == nil || !strings.Contains(err.Error(), "sample matches") {
		t.Fatalf("expected incomplete-page ambiguity, got %v", err)
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
		id, err = DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
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

func TestResolveAppInfoID_SkipsReplacedAppInfo(t *testing.T) {
	client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		return appResolutionJSONResponse(`{"data":[
			{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
			{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}}
		]}`)
	})

	id, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
	if err != nil || id != "info-live" {
		t.Fatalf("ResolveAppInfoID() = %q, %v; want info-live", id, err)
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
			wantSubstrings: []string{`2 app infos match app "123456789"; pass --app-info with one of:`, "READY_FOR_SALE", "IN_REVIEW"},
		},
		{
			name: "all live candidates",
			responseBody: `{"data":[
				{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_SALE"}},
				{"type":"appInfos","id":"info-2","attributes":{"state":"READY_FOR_DISTRIBUTION"}}
			]}`,
			wantSubstrings: []string{`2 app infos match app "123456789"; pass --app-info with one of:`, "READY_FOR_SALE", "READY_FOR_DISTRIBUTION"},
		},
		{
			name: "multiple prepare for submission candidates",
			responseBody: `{"data":[
				{"type":"appInfos","id":"info-ios","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
				{"type":"appInfos","id":"info-macos","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
			]}`,
			wantSubstrings: []string{`2 app infos match app "123456789"; pass --app-info with one of:`, "info-ios", "info-macos", "PREPARE_FOR_SUBMISSION"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
				return appResolutionJSONResponse(tc.responseBody)
			})

			_, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
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

	_, err := DefaultDeps().ResolveAppInfoID(context.Background(), client, "123456789", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no app info found") {
		t.Fatalf("expected 'no app info found' error, got: %v", err)
	}
}
