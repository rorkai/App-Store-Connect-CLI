package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func expectedLocalizationsStderr(argsPrefix []string) string {
	if len(argsPrefix) >= 2 && argsPrefix[0] == "beta-app-localizations" && argsPrefix[1] == "list" {
		return betaAppLocalizationsListDeprecationWarning
	}
	if len(argsPrefix) >= 2 && argsPrefix[0] == "beta-build-localizations" && argsPrefix[1] == "list" {
		return "Warning: `asc beta-build-localizations list` is deprecated. Use `asc builds test-notes list`"
	}
	return ""
}

func runLocalizationsInvalidNextURLCases(
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
			next:    "http://api.appstoreconnect.apple.com/v1/betaAppLocalizations?cursor=AQ",
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
			if wantWarning := expectedLocalizationsStderr(argsPrefix); wantWarning != "" {
				if !strings.Contains(stderr, wantWarning) {
					t.Fatalf("expected deprecation warning %q, got %q", wantWarning, stderr)
				}
			} else if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func runLocalizationsPaginateFromNext(
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

	if wantWarning := expectedLocalizationsStderr(argsPrefix); wantWarning != "" {
		if !strings.Contains(stderr, wantWarning) {
			t.Fatalf("expected deprecation warning %q, got %q", wantWarning, stderr)
		}
	} else if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestBetaAppLocalizationsListRejectsInvalidNextURL(t *testing.T) {
	runLocalizationsInvalidNextURLCases(
		t,
		[]string{"beta-app-localizations", "list"},
		"beta-app-localizations list: --next",
	)
}

func TestBetaAppLocalizationsListPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/betaAppLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/betaAppLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"betaAppLocalizations","id":"beta-app-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"betaAppLocalizations","id":"beta-app-localization-next-2"}],"links":{"next":""}}`

	runLocalizationsPaginateFromNext(
		t,
		[]string{"beta-app-localizations", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"beta-app-localization-next-1",
		"beta-app-localization-next-2",
	)
}

func TestBetaBuildLocalizationsListRejectsInvalidNextURL(t *testing.T) {
	runLocalizationsInvalidNextURLCases(
		t,
		[]string{"beta-build-localizations", "list"},
		"beta-build-localizations list: --next",
	)
}

func TestBetaBuildLocalizationsListPaginateFromNextWithoutBuild(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/betaBuildLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/betaBuildLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"betaBuildLocalizations","id":"beta-build-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"betaBuildLocalizations","id":"beta-build-localization-next-2"}],"links":{"next":""}}`

	runLocalizationsPaginateFromNext(
		t,
		[]string{"beta-build-localizations", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"beta-build-localization-next-1",
		"beta-build-localization-next-2",
	)
}

func TestBuildLocalizationsListRejectsInvalidNextURL(t *testing.T) {
	runLocalizationsInvalidNextURLCases(
		t,
		[]string{"build-localizations", "list", "--build", "build-1"},
		"build-localizations list: --next",
	)
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

	args := []string{"build-localizations", "list", "--build", "build-1", "--paginate", "--next", firstURL}

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
