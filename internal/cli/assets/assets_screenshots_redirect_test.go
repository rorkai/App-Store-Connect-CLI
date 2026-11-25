package assets

import (
	"context"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	reviewshots "github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestUploadScreenshotsFanoutRefusesRedirectBeforeWrites(t *testing.T) {
	var requests, redirected, writes atomic.Int32
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			http.Redirect(w, r, "/fixture/off-collection", http.StatusTemporaryRedirect)
		case "/fixture/off-collection":
			redirected.Add(1)
			http.Error(w, "unexpected off-collection read", http.StatusInternalServerError)
		default:
			if r.Method != http.MethodGet {
				writes.Add(1)
			}
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	deps := newAssetsUploadTestDeps()
	_, err := uploadScreenshotsFanout(context.Background(), screenshotUploadFanoutConfig{
		Deps:                  deps,
		Client:                client,
		VersionID:             "version-1",
		LocaleAssets:          []screenshotLocaleAssetFiles{{Locale: "en-US", Files: []string{"ignored.png"}}},
		LocaleAssetsCanonical: true,
		DisplayType:           asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"),
		Replace:               true,
		RequestContext:        deps.Timeout,
	})
	if err == nil {
		t.Fatal("uploadScreenshotsFanout() error = nil, want refused redirect")
	}
	if redirected.Load() != 0 || writes.Load() != 0 {
		t.Fatalf("redirected=%d writes=%d, want zero", redirected.Load(), writes.Load())
	}
	if requests.Load() != 1 {
		t.Fatalf("requests=%d, want one collection read", requests.Load())
	}
}

func TestUploadScreenshotsScopedSetReadRefusesRedirectBeforeReplace(t *testing.T) {
	root := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, root, "01-home.png", 1242, 2688)
	var requests, redirected, writes atomic.Int32
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			http.Redirect(w, r, "/fixture/off-collection", http.StatusTemporaryRedirect)
		case "/fixture/off-collection":
			redirected.Add(1)
			http.Error(w, "unexpected off-collection read", http.StatusInternalServerError)
		default:
			if r.Method != http.MethodGet {
				writes.Add(1)
			}
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	deps := newAssetsUploadTestDeps()
	_, err := uploadScreenshots(
		asc.WithNoRedirectReads(context.Background()), deps, client, "loc-en", "APP_IPHONE_65", []string{filePath}, false, true, false,
	)
	if err == nil {
		t.Fatal("uploadScreenshots() error = nil, want refused redirect")
	}
	if redirected.Load() != 0 || writes.Load() != 0 {
		t.Fatalf("redirected=%d writes=%d, want zero", redirected.Load(), writes.Load())
	}
	if requests.Load() != 1 {
		t.Fatalf("requests=%d, want one screenshot-set collection read", requests.Load())
	}
}

func TestResolveScreenshotPlanVersionRefusesRedirectBeforeVersionSelection(t *testing.T) {
	var requests, redirected atomic.Int32
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			http.Redirect(w, r, "/fixture/off-collection", http.StatusTemporaryRedirect)
		case "/fixture/off-collection":
			redirected.Add(1)
			http.Error(w, "unexpected off-collection read", http.StatusInternalServerError)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	_, _, _, err := resolveScreenshotPlanVersion(context.Background(), client, "123456789", "1.2.3", "", "IOS")
	if err == nil {
		t.Fatal("resolveScreenshotPlanVersion() error = nil, want refused redirect")
	}
	if redirected.Load() != 0 {
		t.Fatalf("redirected=%d, want zero", redirected.Load())
	}
	if requests.Load() != 1 {
		t.Fatalf("requests=%d, want one version collection read", requests.Load())
	}
}

func TestExecuteScreenshotReviewPlanRefusesRedirectBeforeWrites(t *testing.T) {
	for _, apply := range []bool{false, true} {
		t.Run(map[bool]string{false: "plan", true: "apply"}[apply], func(t *testing.T) {
			setupAssetsPlanAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			outputDir := t.TempDir()
			filePath := writeAssetsTestPNGWithSize(t, outputDir, "01-home.png", 1242, 2688)
			writeAssetsReviewManifest(t, filepath.Join(outputDir, defaultReviewManifestFile), reviewshots.ReviewManifest{
				GeneratedAt: "2026-03-16T00:00:00Z",
				FramedDir:   outputDir,
				OutputDir:   outputDir,
				Entries: []reviewshots.ReviewEntry{{
					Key:               "ready-entry",
					ScreenshotID:      "home",
					Locale:            "en-US",
					FramedPath:        filePath,
					FramedRelative:    "01-home.png",
					DisplayTypes:      []string{"APP_IPHONE_65"},
					ValidAppStoreSize: true,
					Status:            "ready",
				}},
			})
			if err := reviewshots.SaveApprovals(filepath.Join(outputDir, defaultReviewApprovalFile), map[string]bool{"ready-entry": true}); err != nil {
				t.Fatal(err)
			}

			var requests, redirected, writes atomic.Int32
			client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch r.URL.Path {
				case "/v1/appStoreVersions/version-123":
					writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-123","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
				case "/v1/appStoreVersions/version-123/appStoreVersionLocalizations":
					http.Redirect(w, r, "/fixture/off-collection", http.StatusTemporaryRedirect)
				case "/fixture/off-collection":
					redirected.Add(1)
					http.Error(w, "unexpected off-collection read", http.StatusInternalServerError)
				default:
					if r.Method != http.MethodGet {
						writes.Add(1)
					}
					http.Error(w, "unexpected request", http.StatusNotFound)
				}
			}))
			d := newAssetsClientTestDeps(func() (*asc.Client, error) { return client, nil })

			_, err := executeScreenshotReviewPlan(context.Background(), d, screenshotReviewPlanOptions{
				AppID:           "123456789",
				VersionID:       "version-123",
				Platform:        "IOS",
				ReviewOutputDir: outputDir,
				Apply:           apply,
				Confirm:         apply,
			})
			if err == nil {
				t.Fatal("executeScreenshotReviewPlan() error = nil, want refused redirect")
			}
			if redirected.Load() != 0 || writes.Load() != 0 {
				t.Fatalf("redirected=%d writes=%d, want zero", redirected.Load(), writes.Load())
			}
			if requests.Load() != 2 {
				t.Fatalf("requests=%d, want version and localization reads", requests.Load())
			}
		})
	}
}
