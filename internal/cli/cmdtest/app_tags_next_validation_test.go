package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runAppTagsInvalidNextURLCases(
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
			next:    "http://api.appstoreconnect.apple.com/v1/apps/app-1/relationships/appTags?cursor=AQ",
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

func runAppTagsPaginateFromNext(
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

func TestAppTagsRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runAppTagsInvalidNextURLCases(
		t,
		[]string{"app-tags", "links"},
		"app-tags links: --next",
	)
}

func TestAppTagsRelationshipsPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/relationships/appTags?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/relationships/appTags?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appTags","id":"app-tag-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appTags","id":"app-tag-link-next-2"}],"links":{"next":""}}`

	runAppTagsPaginateFromNext(
		t,
		[]string{"app-tags", "links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"app-tag-link-next-1",
		"app-tag-link-next-2",
	)
}

func TestAppTagsTerritoriesRejectsInvalidNextURL(t *testing.T) {
	runAppTagsInvalidNextURLCases(
		t,
		[]string{"app-tags", "territories"},
		"app-tags territories: --next",
	)
}

func TestAppTagsTerritoriesPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/territories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/territories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"territories","id":"app-tag-territory-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"territories","id":"app-tag-territory-next-2"}],"links":{"next":""}}`

	runAppTagsPaginateFromNext(
		t,
		[]string{"app-tags", "territories"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"app-tag-territory-next-1",
		"app-tag-territory-next-2",
	)
}

func TestAppTagsTerritoriesRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runAppTagsInvalidNextURLCases(
		t,
		[]string{"app-tags", "territories-links"},
		"app-tags territories-links: --next",
	)
}

func TestAppTagsTerritoriesRelationshipsPaginateFromNextWithoutID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/relationships/territories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/relationships/territories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"territories","id":"app-tag-territory-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"territories","id":"app-tag-territory-link-next-2"}],"links":{"next":""}}`

	runAppTagsPaginateFromNext(
		t,
		[]string{"app-tags", "territories-links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"app-tag-territory-link-next-1",
		"app-tag-territory-link-next-2",
	)
}
