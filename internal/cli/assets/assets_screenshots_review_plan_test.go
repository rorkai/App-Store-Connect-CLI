package assets

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	reviewshots "github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestExecuteScreenshotReviewPlanPlanModeReturnsBeforeUploadWhenBlockingIssuesExist(t *testing.T) {
	setupAssetsPlanAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := t.TempDir()
	filePath := writeAssetsTestPNG(t, outputDir, "01-home.png")

	manifestPath := filepath.Join(outputDir, defaultReviewManifestFile)
	manifest := reviewshots.ReviewManifest{
		GeneratedAt: "2026-03-16T00:00:00Z",
		FramedDir:   outputDir,
		OutputDir:   outputDir,
		Entries: []reviewshots.ReviewEntry{
			{
				Key:               "ready-entry",
				ScreenshotID:      "home",
				Locale:            "en-US",
				FramedPath:        filePath,
				FramedRelative:    "01-home.png",
				DisplayTypes:      []string{"APP_IPHONE_65"},
				ValidAppStoreSize: true,
				Status:            "ready",
			},
			{
				Key:               "blocked-entry",
				ScreenshotID:      "details",
				Locale:            "en-US",
				FramedPath:        filePath,
				FramedRelative:    "01-home.png",
				DisplayTypes:      []string{"APP_IPHONE_65"},
				ValidAppStoreSize: true,
				Status:            "invalid-size",
			},
		},
	}
	writeAssetsReviewManifest(t, manifestPath, manifest)
	if err := reviewshots.SaveApprovals(filepath.Join(outputDir, defaultReviewApprovalFile), map[string]bool{
		"ready-entry":   true,
		"blocked-entry": true,
	}); err != nil {
		t.Fatalf("SaveApprovals() error: %v", err)
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			return assetsJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "123456789"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionLocalizations":
			return assetsJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "appStoreVersionLocalizations",
						"id": "loc-en",
						"attributes": {
							"locale": "en-US"
						}
					}
				],
				"links": {}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			t.Fatal("plan mode with blocking issues must not fetch screenshot sets")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	result, err := executeScreenshotReviewPlan(context.Background(), screenshotReviewPlanOptions{
		AppID:           "123456789",
		VersionID:       "version-123",
		Platform:        "IOS",
		ReviewOutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("executeScreenshotReviewPlan() error: %v", err)
	}

	if result.ErrorCount == 0 {
		t.Fatal("expected blocking issue count to be reported")
	}
	if result.PlannedGroups != 1 {
		t.Fatalf("expected one planned group, got %d", result.PlannedGroups)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("expected one returned group without uploads, got %d", len(result.Groups))
	}
	if result.Groups[0].Result.SetID != "" || len(result.Groups[0].Result.Results) != 0 {
		t.Fatalf("expected plan mode to skip upload results when blocking issues exist, got %+v", result.Groups[0].Result)
	}
}

func TestExecuteScreenshotReviewPlanUsesPlatformSpecificCoverageWarnings(t *testing.T) {
	setupAssetsPlanAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	outputDir := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, outputDir, "01-desktop.png", 2880, 1800)

	manifestPath := filepath.Join(outputDir, defaultReviewManifestFile)
	manifest := reviewshots.ReviewManifest{
		GeneratedAt: "2026-03-16T00:00:00Z",
		FramedDir:   outputDir,
		OutputDir:   outputDir,
		Entries: []reviewshots.ReviewEntry{
			{
				Key:               "desktop-entry",
				ScreenshotID:      "desktop-home",
				Locale:            "en-US",
				FramedPath:        filePath,
				FramedRelative:    "01-desktop.png",
				DisplayTypes:      []string{"APP_DESKTOP"},
				ValidAppStoreSize: true,
				Status:            "ready",
			},
		},
	}
	writeAssetsReviewManifest(t, manifestPath, manifest)
	if err := reviewshots.SaveApprovals(filepath.Join(outputDir, defaultReviewApprovalFile), map[string]bool{
		"desktop-entry": true,
	}); err != nil {
		t.Fatalf("SaveApprovals() error: %v", err)
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			return assetsJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "MAC_OS"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "123456789"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionLocalizations":
			return assetsJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "appStoreVersionLocalizations",
						"id": "loc-en",
						"attributes": {
							"locale": "en-US"
						}
					}
				],
				"links": {}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	result, err := executeScreenshotReviewPlan(context.Background(), screenshotReviewPlanOptions{
		AppID:           "123456789",
		VersionID:       "version-123",
		Platform:        "MAC_OS",
		ReviewOutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("executeScreenshotReviewPlan() error: %v", err)
	}

	if result.ErrorCount != 0 {
		t.Fatalf("expected no blocking issues, got %d", result.ErrorCount)
	}
	if result.WarningCount != 0 {
		t.Fatalf("expected no iOS-focused coverage warnings for MAC_OS, got %d with issues %+v", result.WarningCount, result.Issues)
	}
	if result.PlannedGroups != 1 {
		t.Fatalf("expected one planned group, got %d", result.PlannedGroups)
	}
}

func TestResolveScreenshotPlanVersionRejectsPlatformMismatchForVersionID(t *testing.T) {
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			return assetsJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "123456789"
							}
						}
					}
				}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	_, _, _, err := resolveScreenshotPlanVersion(
		context.Background(),
		client,
		"123456789",
		"",
		"version-123",
		"MAC_OS",
	)
	if err == nil {
		t.Fatal("expected platform mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), `version "version-123" is on platform "IOS", not "MAC_OS"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func setupAssetsPlanAuth(t *testing.T) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("encode pem: nil")
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "KEY_ID")
	t.Setenv("ASC_ISSUER_ID", "ISSUER_ID")
	t.Setenv("ASC_PRIVATE_KEY", string(pemBytes))
}

func writeAssetsReviewManifest(t *testing.T, path string, manifest reviewshots.ReviewManifest) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
}

func writeAssetsTestPNGWithSize(t *testing.T, dir, name string, width, height int) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	// Distinct names get distinct pixels: uploads reject duplicate screenshots.
	sum := crc32.ChecksumIEEE([]byte(name))
	img.Set(0, 0, color.RGBA{R: byte(sum), G: byte(sum >> 8), B: byte(sum >> 16), A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}
