package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runAppsInvalidNextURLCases(
	t *testing.T,
	argsPrefix []string,
	wantErrPrefix string,
) {
	t.Helper()

	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/apps?cursor=AQ",
			wantErr: wantErrPrefix + " must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: wantErrPrefix + " must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, argsPrefix...), "--next", test.next)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func runAppsPaginateFromNext(
	t *testing.T,
	argsPrefix []string,
	firstURL string,
	secondURL string,
	firstBody string,
	secondBody string,
	wantIDs ...string,
) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

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
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(firstBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(secondBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	args := append(append([]string{}, argsPrefix...), "--paginate", "--next", firstURL)

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
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestAppsListRejectsInvalidNextURL(t *testing.T) {
	runAppsInvalidNextURLCases(
		t,
		[]string{"apps", "list"},
		"apps: --next",
	)
}

func TestAppsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"apps","id":"app-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"apps","id":"app-next-2"}],"links":{"next":""}}`

	runAppsPaginateFromNext(
		t,
		[]string{"apps", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"app-next-1",
		"app-next-2",
	)
}

func TestAppsSearchKeywordsListRejectsInvalidNextURL(t *testing.T) {
	runAppsInvalidNextURLCases(
		t,
		[]string{"apps", "search-keywords", "list"},
		"apps search-keywords list: --next",
	)
}

func TestAppsSearchKeywordsListPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/searchKeywords?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/searchKeywords?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appKeywords","id":"app-keyword-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appKeywords","id":"app-keyword-next-2"}],"links":{"next":""}}`

	runAppsPaginateFromNext(
		t,
		[]string{"apps", "search-keywords", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"app-keyword-next-1",
		"app-keyword-next-2",
	)
}

func TestAppsAppEncryptionDeclarationsListRejectsInvalidNextURL(t *testing.T) {
	runAppsInvalidNextURLCases(
		t,
		[]string{"apps", "app-encryption-declarations", "list"},
		"apps app-encryption-declarations list: --next",
	)
}

func TestAppsAppEncryptionDeclarationsListPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appEncryptionDeclarations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appEncryptionDeclarations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appEncryptionDeclarations","id":"app-encryption-declaration-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appEncryptionDeclarations","id":"app-encryption-declaration-next-2"}],"links":{"next":""}}`

	runAppsPaginateFromNext(
		t,
		[]string{"apps", "app-encryption-declarations", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"app-encryption-declaration-next-1",
		"app-encryption-declaration-next-2",
	)
}

func TestAppInfoTerritoryAgeRatingsListRejectsInvalidNextURL(t *testing.T) {
	runAppsInvalidNextURLCases(
		t,
		[]string{"apps", "info", "territory-age-ratings", "list"},
		"apps info territory-age-ratings list: --next",
	)
}

func TestAppInfoTerritoryAgeRatingsListPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-1/territoryAgeRatings?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-1/territoryAgeRatings?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appInfoTerritoryAgeRatings","id":"territory-age-rating-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appInfoTerritoryAgeRatings","id":"territory-age-rating-next-2"}],"links":{"next":""}}`

	runAppsPaginateFromNext(
		t,
		[]string{"apps", "info", "territory-age-ratings", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"territory-age-rating-next-1",
		"territory-age-rating-next-2",
	)
}
