package migrate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type migrateUploadRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn migrateUploadRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestUploadScreenshots_ReordersPlannedFilesBeforeUntouchedRemoteExtras(t *testing.T) {
	dir := t.TempDir()
	existingFile := writeMigrateTestPNG(t, dir, "01-home.png")
	newFile := writeMigrateTestPNG(t, dir, "02-settings.png")
	newFileSize := migrateFileSize(t, newFile)
	relationshipPatchCalled := false

	origTransport := http.DefaultTransport
	http.DefaultTransport = migrateUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return migrateJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"old-home","attributes":{"fileName":"%s"}},{"type":"appScreenshots","id":"old-extra","attributes":{"fileName":"99-legacy.png"}}]}`, filepath.Base(existingFile)))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-extra"},{"type":"appScreenshots","id":"old-home"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return migrateJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-settings","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-settings","length":%d,"offset":0}]}}}`, newFileSize))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return migrateJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-settings":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-settings","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-settings":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-settings","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			wantIDs := []string{"old-home", "new-settings", "old-extra"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return migrateJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newMigrateUploadTestClient(t)
	results, err := uploadScreenshots(
		context.Background(),
		migrateTestDeps(), client,
		"version-1",
		map[string]string{"en-US": "loc-1"},
		[]ScreenshotPlan{{
			Locale:      "en-US",
			DisplayType: "APP_IPHONE_65",
			Files:       []string{existingFile, newFile},
		}},
	)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 screenshot result, got %d", len(results))
	}
	if len(results[0].Skipped) != 1 || results[0].Skipped[0].Path != existingFile {
		t.Fatalf("expected existing file to be skipped, got %#v", results[0].Skipped)
	}
	if len(results[0].Uploaded) != 1 || results[0].Uploaded[0].AssetID != "new-settings" {
		t.Fatalf("expected uploaded screenshot new-settings, got %#v", results[0].Uploaded)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func TestUploadScreenshots_UploadsSetConcurrentlyInPlanOrder(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		writeMigrateTestPNG(t, dir, "01-a.png"),
		writeMigrateTestPNG(t, dir, "02-b.png"),
		writeMigrateTestPNG(t, dir, "03-c.png"),
	}
	size := migrateFileSize(t, files[0])

	var mu sync.Mutex
	inFlight := 0
	release := make(chan struct{})
	var gotOrder []string

	origTransport := http.DefaultTransport
	http.DefaultTransport = migrateUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && (req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots" || req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots"):
			return migrateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			var payload struct {
				Data struct {
					Attributes struct {
						FileName string `json:"fileName"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			id := strings.TrimSuffix(payload.Data.Attributes.FileName, ".png")
			return migrateJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, size))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			mu.Lock()
			inFlight++
			if inFlight == len(files) {
				close(release)
			}
			mu.Unlock()
			select {
			case <-release:
				return migrateJSONResponse(http.StatusOK, `{}`)
			case <-time.After(5 * time.Second):
				t.Errorf("upload PUT %s did not overlap with the other uploads in the set", req.URL.Path)
				return migrateJSONResponse(http.StatusInternalServerError, `{}`)
			}
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return migrateJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return migrateJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			var payload asc.RelationshipRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode relationship patch body: %v", err)
			}
			for _, item := range payload.Data {
				gotOrder = append(gotOrder, item.ID)
			}
			return migrateJSONResponse(http.StatusNoContent, "")
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return migrateJSONResponse(http.StatusNotFound, `{}`)
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	results, err := uploadScreenshots(
		context.Background(),
		migrateTestDeps(), newMigrateUploadTestClient(t),
		"version-1",
		map[string]string{"en-US": "loc-1"},
		[]ScreenshotPlan{{Locale: "en-US", DisplayType: "APP_IPHONE_65", Files: files}},
	)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	wantIDs := []string{"01-a", "02-b", "03-c"}
	if len(results) != 1 || len(results[0].Uploaded) != len(wantIDs) {
		t.Fatalf("unexpected results: %#v", results)
	}
	for i, item := range results[0].Uploaded {
		if item.AssetID != wantIDs[i] || item.FilePath != files[i] {
			t.Fatalf("uploaded[%d] = %#v, want asset %q from %q", i, item, wantIDs[i], files[i])
		}
	}
	if !reflect.DeepEqual(gotOrder, wantIDs) {
		t.Fatalf("relationship order = %v, want %v", gotOrder, wantIDs)
	}
}

func TestUploadScreenshots_FailedUploadDeletesLaterUploadsAndOmitsLaterSkips(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		writeMigrateTestPNG(t, dir, "01-a.png"),
		writeMigrateTestPNG(t, dir, "02-b.png"),
		writeMigrateTestPNG(t, dir, "03-c.png"),
	}
	if err := os.Mkdir(filepath.Join(dir, "dup"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	laterDuplicate := writeMigrateTestPNG(t, filepath.Join(dir, "dup"), "03-c.png")
	size := migrateFileSize(t, files[0])

	var mu sync.Mutex
	var deleted []string
	reordered := false

	origTransport := http.DefaultTransport
	http.DefaultTransport = migrateUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && (req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots" || req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots"):
			return migrateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			var payload struct {
				Data struct {
					Attributes struct {
						FileName string `json:"fileName"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			id := strings.TrimSuffix(payload.Data.Attributes.FileName, ".png")
			return migrateJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, size))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			if req.URL.Path == "/01-a" {
				return migrateJSONResponse(http.StatusForbidden, `{}`)
			}
			return migrateJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return migrateJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return migrateJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`, id))
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			mu.Lock()
			deleted = append(deleted, strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/"))
			mu.Unlock()
			return migrateJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			reordered = true
			return migrateJSONResponse(http.StatusNoContent, "")
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return migrateJSONResponse(http.StatusNotFound, `{}`)
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	results, err := uploadScreenshots(
		context.Background(),
		migrateTestDeps(), newMigrateUploadTestClient(t),
		"version-1",
		map[string]string{"en-US": "loc-1"},
		[]ScreenshotPlan{{Locale: "en-US", DisplayType: "APP_IPHONE_65", Files: append(append([]string(nil), files...), laterDuplicate)}},
	)
	if err == nil || !strings.Contains(err.Error(), files[0]) {
		t.Fatalf("uploadScreenshots() error = %v, want failure for %s", err, files[0])
	}

	sort.Strings(deleted)
	if want := []string{"02-b", "03-c"}; !reflect.DeepEqual(deleted, want) {
		t.Fatalf("deleted = %v, want %v", deleted, want)
	}
	if reordered {
		t.Fatal("reorder PATCH sent after a failed upload")
	}
	if len(results) != 1 || len(results[0].Uploaded) != 0 || len(results[0].Skipped) != 0 {
		t.Fatalf("receipt = %#v, want no uploads or later skips", results)
	}
}

func TestPrepareVersionLocalizationsRejectsOverLimitKeywords(t *testing.T) {
	_, err := prepareVersionLocalizations([]FastlaneLocalization{{
		Locale:   "ja",
		Keywords: strings.Repeat("語", 101),
	}})
	if err == nil {
		t.Fatal("expected localization validation error")
	}
	const wantError = `migrate import: locale "ja": keywords exceed 100 characters`
	if err.Error() != wantError {
		t.Fatalf("prepareVersionLocalizations() error = %q, want %q", err, wantError)
	}
}

func TestPrepareVersionLocalizationsPreservesOrderAndDuplicates(t *testing.T) {
	localizations := []FastlaneLocalization{
		{Locale: "en-US", Description: "first"},
		{Locale: "en-US", Description: "second"},
		{Locale: "fr-FR", Description: "third"},
	}

	prepared, err := prepareVersionLocalizations(localizations)
	if err != nil {
		t.Fatalf("prepareVersionLocalizations() error: %v", err)
	}
	if len(prepared) != len(localizations) {
		t.Fatalf("prepared localization count = %d, want %d", len(prepared), len(localizations))
	}

	gotLocales := make([]string, 0, len(prepared))
	gotDescriptions := make([]string, 0, len(prepared))
	for _, item := range prepared {
		gotLocales = append(gotLocales, item.localization.Locale)
		gotDescriptions = append(gotDescriptions, item.attributes.Description)
	}
	if want := []string{"en-US", "en-US", "fr-FR"}; !reflect.DeepEqual(gotLocales, want) {
		t.Fatalf("prepared locales = %v, want %v", gotLocales, want)
	}
	if want := []string{"first", "second", "third"}; !reflect.DeepEqual(gotDescriptions, want) {
		t.Fatalf("prepared descriptions = %v, want %v", gotDescriptions, want)
	}
}

func TestValidateLocalizationCreateTargetAllowsExistingUnsupportedRoot(t *testing.T) {
	if err := validateLocalizationCreateTarget("nl", "existing-loc"); err != nil {
		t.Fatalf("validateLocalizationCreateTarget() error = %v, want nil for update", err)
	}
}

func TestValidateLocalizationCreateTargetRejectsUnsupportedRootCreate(t *testing.T) {
	err := validateLocalizationCreateTarget("nl", "")
	const wantError = `migrate import: locale "nl": unsupported locale "nl"; did you mean: nl-NL`
	if err == nil || err.Error() != wantError {
		t.Fatalf("validateLocalizationCreateTarget() error = %q, want %q", err, wantError)
	}
}

func newMigrateUploadTestClient(t *testing.T) *asc.Client {
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

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func migrateJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func writeMigrateTestPNG(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func migrateFileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	return info.Size()
}

func TestNormalizeDeliverfilePlatformAcceptsFastlaneSpellings(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "ios", want: "IOS"},
		{value: "IOS", want: "IOS"},
		{value: "osx", want: "MAC_OS"},
		{value: "OSX", want: "MAC_OS"},
		{value: "macos", want: "MAC_OS"},
		{value: "mac", want: "MAC_OS"},
		{value: "mac_os", want: "MAC_OS"},
		{value: "appletvos", want: "TV_OS"},
		{value: "tvos", want: "TV_OS"},
		{value: "tv_os", want: "TV_OS"},
		{value: "xros", want: "VISION_OS"},
		{value: " xrOS ", want: "VISION_OS"},
		{value: "visionos", want: "VISION_OS"},
		{value: "vision_os", want: "VISION_OS"},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := normalizeDeliverfilePlatform(test.value)
			if err != nil {
				t.Fatalf("normalizeDeliverfilePlatform(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("normalizeDeliverfilePlatform(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestNormalizeDeliverfilePlatformRejectsUnknownValue(t *testing.T) {
	_, err := normalizeDeliverfilePlatform("android")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if err.Error() != `unsupported Deliverfile platform "android"` {
		t.Fatalf("error = %q, want unsupported Deliverfile platform message", err)
	}
}

// Each page gets a new budget; finishing one request must release its context.
func TestFetchLocalizationsForPlanRequestContexts(t *testing.T) {
	for _, kind := range []string{"version", "app info"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("ASC_TIMEOUT", "1s")
			parent, cancel := shared.ContextWithTimeout(context.Background())
			defer cancel()
			parentDeadline, _ := parent.Deadline()
			t.Setenv("ASC_TIMEOUT", "30s")
			var requests []context.Context
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = migrateUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				deadline, ok := req.Context().Deadline()
				if !ok || time.Until(deadline) <= 0 || deadline.Sub(parentDeadline) < 20*time.Second {
					t.Fatal("request reused inherited timeout instead of a fresh budget")
				}
				if len(requests) > 0 && requests[len(requests)-1].Err() != context.Canceled {
					t.Fatal("previous request context was not canceled")
				}
				requests = append(requests, req.Context())
				if len(requests) == 1 {
					return migrateJSONResponse(200, fmt.Sprintf(`{"data":[],"links":{"next":%q}}`, "https://api.appstoreconnect.apple.com"+req.URL.Path+"?page=2"))
				}
				return migrateJSONResponse(200, `{"data":[]}`)
			})
			client := newMigrateUploadTestClient(t)
			var err error
			if kind == "version" {
				_, err = fetchVersionLocalizationsForPlan(parent, migrateTestDeps(), client, "VERSION_ID")
			} else {
				_, err = fetchAppInfoLocalizationsForPlan(parent, migrateTestDeps(), client, "INFO_ID")
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(requests) != 2 {
				t.Fatalf("requests=%d, want 2", len(requests))
			}
			if requests[1].Err() != context.Canceled {
				t.Fatal("final request context was not canceled")
			}
		})
	}
}

func TestResolveDeliverfileAppIDRequiresExactBundleID(t *testing.T) {
	for _, test := range []struct {
		name     string
		bundleID string
		want     string
	}{
		{"prefix alone is refused", "com.example.app.preview", ""},
		{"exact bundle resolves", "com.example.app", "123"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=com.example.app", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"123","attributes":{"bundleId":"`+test.bundleID+`"}}]}`))
			got, err := resolveAppID(context.Background(), shared.DefaultDeps(), newMigrateUploadTestClient(t), "",
				DeliverfileConfig{AppIdentifier: "com.example.app"})
			if test.want == "" {
				if got != "" || err == nil || !strings.Contains(err.Error(), "exact match required") {
					t.Fatalf("resolved a prefix candidate: id=%q error=%v", got, err)
				}
			} else if err != nil || got != test.want {
				t.Fatalf("resolved %q, %v; want %q", got, err, test.want)
			}
		})
	}
}
