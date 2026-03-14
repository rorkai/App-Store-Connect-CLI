package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runPhase38InvalidNextURLCases(
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
			next:    "http://api.appstoreconnect.apple.com/v1/actors?cursor=AQ",
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
			expectedWarning := ""
			if len(argsPrefix) > 0 {
				switch argsPrefix[0] {
				case "feedback":
					expectedWarning = feedbackRootDeprecationWarning
				case "crashes":
					expectedWarning = crashesRootDeprecationWarning
				}
			}
			if expectedWarning == "" {
				if stderr != "" {
					t.Fatalf("expected empty stderr, got %q", stderr)
				}
			} else {
				requireStderrContainsWarning(t, stderr, expectedWarning)
			}
		})
	}
}

func runPhase38PaginateFromNext(
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

	expectedWarning := ""
	if len(argsPrefix) > 0 {
		switch argsPrefix[0] {
		case "feedback":
			expectedWarning = feedbackRootDeprecationWarning
		case "crashes":
			expectedWarning = crashesRootDeprecationWarning
		}
	}
	if expectedWarning == "" {
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	} else {
		requireStderrContainsWarning(t, stderr, expectedWarning)
	}
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestActorsListRejectsInvalidNextURL(t *testing.T) {
	runPhase38InvalidNextURLCases(
		t,
		[]string{"actors", "list"},
		"actors list: --next",
	)
}

func TestActorsListPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/actors?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/actors?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"actors","id":"actor-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"actors","id":"actor-next-2"}],"links":{"next":""}}`

	runPhase38PaginateFromNext(
		t,
		[]string{"actors", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"actor-next-1",
		"actor-next-2",
	)
}

func TestAndroidIosMappingListRejectsInvalidNextURL(t *testing.T) {
	runPhase38InvalidNextURLCases(
		t,
		[]string{"android-ios-mapping", "list", "--app", "app-1"},
		"android-ios-mapping list: --next",
	)
}

func TestAndroidIosMappingListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/androidToIosAppMappingDetails?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/androidToIosAppMappingDetails?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"androidToIosAppMappingDetails","id":"mapping-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"androidToIosAppMappingDetails","id":"mapping-next-2"}],"links":{"next":""}}`

	runPhase38PaginateFromNext(
		t,
		[]string{"android-ios-mapping", "list", "--app", "app-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"mapping-next-1",
		"mapping-next-2",
	)
}

func TestFeedbackRejectsInvalidNextURL(t *testing.T) {
	runPhase38InvalidNextURLCases(
		t,
		[]string{"feedback"},
		"feedback: --next",
	)
}

func TestFeedbackPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaFeedbackScreenshotSubmissions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaFeedbackScreenshotSubmissions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"feedback-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"feedback-next-2"}],"links":{"next":""}}`

	runPhase38PaginateFromNext(
		t,
		[]string{"feedback"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"feedback-next-1",
		"feedback-next-2",
	)
}

func TestCrashesRejectsInvalidNextURL(t *testing.T) {
	runPhase38InvalidNextURLCases(
		t,
		[]string{"crashes"},
		"crashes: --next",
	)
}

func TestCrashesPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaFeedbackCrashSubmissions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaFeedbackCrashSubmissions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-next-2"}],"links":{"next":""}}`

	runPhase38PaginateFromNext(
		t,
		[]string{"crashes"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"crash-next-1",
		"crash-next-2",
	)
}
