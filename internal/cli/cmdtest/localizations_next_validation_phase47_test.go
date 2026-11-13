package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalizationsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"testflight", "app-localizations", "list"}, wantErr: "testflight app-localizations list: --next"},
	})
}

func TestLocalizationsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"testflight", "app-localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaAppLocalizations?cursor=AQ&limit=200",
			dataType: "betaAppLocalizations",
		},
	})
}

func TestBuildLocalizationsListPaginateFromNext(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			wantURL := "https://api.appstoreconnect.apple.com/v1/builds/build-1/appStoreVersion"
			if req.Method != http.MethodGet || req.URL.String() != wantURL {
				t.Fatalf("unexpected resolve request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":{"type":"appStoreVersions","id":"version-1"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first localization request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"build-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second localization request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"build-localization-next-2"}],"links":{"next":""}}`
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

	args := []string{"build-localizations", "list", "--build-id", "build-1", "--paginate", "--next", firstURL}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-localization-next-1"`) || !strings.Contains(stdout, `"id":"build-localization-next-2"`) {
		t.Fatalf("expected paginated build localizations in output, got %q", stdout)
	}
}
