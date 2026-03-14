package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runPhase39InvalidNextURLCases(
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
			next:    "http://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipDefaultExperiences?cursor=AQ",
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

func runPhase39PaginateFromNext(
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

func TestAppClipsAdvancedExperiencesRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runPhase39InvalidNextURLCases(
		t,
		[]string{"app-clips", "advanced-experiences-links"},
		"app-clips advanced-experiences-links: --next",
	)
}

func TestAppClipsAdvancedExperiencesRelationshipsPaginateFromNextWithoutAppClipID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipAdvancedExperiences?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipAdvancedExperiences?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appClipAdvancedExperiences","id":"clip-adv-rel-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appClipAdvancedExperiences","id":"clip-adv-rel-next-2"}],"links":{"next":""}}`

	runPhase39PaginateFromNext(
		t,
		[]string{"app-clips", "advanced-experiences-links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"clip-adv-rel-next-1",
		"clip-adv-rel-next-2",
	)
}

func TestAppClipsDefaultExperiencesRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runPhase39InvalidNextURLCases(
		t,
		[]string{"app-clips", "default-experiences-links"},
		"app-clips default-experiences-links: --next",
	)
}

func TestAppClipsDefaultExperiencesRelationshipsPaginateFromNextWithoutAppClipID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipDefaultExperiences?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipDefaultExperiences?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appClipDefaultExperiences","id":"clip-default-rel-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appClipDefaultExperiences","id":"clip-default-rel-next-2"}],"links":{"next":""}}`

	runPhase39PaginateFromNext(
		t,
		[]string{"app-clips", "default-experiences-links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"clip-default-rel-next-1",
		"clip-default-rel-next-2",
	)
}

func TestAppClipsDefaultExperienceLocalizationsListRejectsInvalidNextURL(t *testing.T) {
	runPhase39InvalidNextURLCases(
		t,
		[]string{"app-clips", "default-experiences", "localizations", "list", "--experience-id", "exp-1"},
		"app-clips default-experiences localizations list: --next",
	)
}

func TestAppClipsDefaultExperienceLocalizationsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appClipDefaultExperiences/exp-1/appClipDefaultExperienceLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appClipDefaultExperiences/exp-1/appClipDefaultExperienceLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"clip-loc-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"clip-loc-next-2"}],"links":{"next":""}}`

	runPhase39PaginateFromNext(
		t,
		[]string{"app-clips", "default-experiences", "localizations", "list", "--experience-id", "exp-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"clip-loc-next-1",
		"clip-loc-next-2",
	)
}

func TestAppEventLocalizationScreenshotsRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runPhase39InvalidNextURLCases(
		t,
		[]string{"app-events", "localizations", "screenshots-links"},
		"app-events localizations screenshots-links: --next",
	)
}

func TestAppEventLocalizationScreenshotsRelationshipsPaginateFromNextWithoutLocalizationID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventScreenshots?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventScreenshots?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appEventScreenshots","id":"event-shot-rel-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appEventScreenshots","id":"event-shot-rel-next-2"}],"links":{"next":""}}`

	runPhase39PaginateFromNext(
		t,
		[]string{"app-events", "localizations", "screenshots-links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"event-shot-rel-next-1",
		"event-shot-rel-next-2",
	)
}

func TestAppEventLocalizationVideoClipsRelationshipsRejectsInvalidNextURL(t *testing.T) {
	runPhase39InvalidNextURLCases(
		t,
		[]string{"app-events", "localizations", "video-clips-links"},
		"app-events localizations video-clips-links: --next",
	)
}

func TestAppEventLocalizationVideoClipsRelationshipsPaginateFromNextWithoutLocalizationID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventVideoClips?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventVideoClips?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appEventVideoClips","id":"event-video-rel-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appEventVideoClips","id":"event-video-rel-next-2"}],"links":{"next":""}}`

	runPhase39PaginateFromNext(
		t,
		[]string{"app-events", "localizations", "video-clips-links"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"event-video-rel-next-1",
		"event-video-rel-next-2",
	)
}
