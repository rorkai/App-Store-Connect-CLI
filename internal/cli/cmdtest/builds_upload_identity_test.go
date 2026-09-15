package cmdtest

import (
	"archive/zip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"howett.net/plist"
)

func TestBuildsUploadResolvesExactBundleIDAndChecksIPAIdentityBeforeReservation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var reservations int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps" && req.URL.Query().Get("filter[bundleId]") == "com.example.demo":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			atomic.AddInt32(&reservations, 1)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read reservation request: %v", err)
			}
			if !strings.Contains(string(body), `"id":"123456789"`) {
				t.Fatalf("expected resolved app ID in reservation body: %s", body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":1,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[]}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"builds", "upload",
		"--app", "com.example.demo",
		"--ipa", ipaPath,
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, _ = captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if got := atomic.LoadInt32(&reservations); got != 1 {
		t.Fatalf("expected one upload reservation, got %d", got)
	}
}

func TestBuildsUploadAutoDetectsIPAPlatformBeforeReservation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	ipaPath := writeBuildUploadIPAWithPlatform(t, "com.example.demo", "appletvos", []string{"AppleTVOS"})

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read reservation request: %v", err)
			}
			if !strings.Contains(string(body), `"platform":"TV_OS"`) {
				t.Fatalf("expected auto-detected TV_OS platform in reservation body: %s", body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"TV_OS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":1,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[]}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"builds", "upload",
		"--app", "123456789",
		"--ipa", ipaPath,
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestBuildsUploadRejectsExplicitIPAPlatformMismatchBeforeNetwork(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	ipaPath := writeBuildUploadIPAWithPlatform(t, "com.example.demo", "xros", []string{"XROS"})

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"builds", "upload",
		"--app", "123456789",
		"--ipa", ipaPath,
		"--platform", "IOS",
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected platform mismatch error")
	}
	if !strings.Contains(err.Error(), "--platform IOS does not match IPA platform VISION_OS") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
}

func TestBuildsUploadRejectsMissingOrAmbiguousExactAppBeforeReservation(t *testing.T) {
	tests := []struct {
		name     string
		appInput string
		appList  string
		wantErr  string
	}{
		{
			name:     "missing",
			appInput: "Missing App",
			appList:  `{"data":[]}`,
			wantErr:  `app "Missing App" not found (expected app ID, exact bundle ID, or exact app name)`,
		},
		{
			name:     "ambiguous exact name",
			appInput: "Duplicate App",
			appList: `{"data":[
				{"type":"apps","id":"111","attributes":{"name":"Duplicate App","bundleId":"com.example.one"}},
				{"type":"apps","id":"222","attributes":{"name":"Duplicate App","bundleId":"com.example.two"}}
			]}`,
			wantErr: "2 apps match \"Duplicate App\"; pass --app with one of:\n  111  Duplicate App  com.example.one\n  222  Duplicate App  com.example.two",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			ipaPath := writeBuildUploadIPA(t, "com.example.demo")

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			var reservations int32
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps":
					if req.URL.Query().Get("filter[bundleId]") != "" {
						return jsonResponse(http.StatusOK, `{"data":[]}`)
					}
					return jsonResponse(http.StatusOK, test.appList)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
					atomic.AddInt32(&reservations, 1)
					return nil, http.ErrUseLastResponse
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse([]string{
				"builds", "upload",
				"--app", test.appInput,
				"--ipa", ipaPath,
				"--dry-run",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := root.Run(context.Background())
			if err == nil {
				t.Fatal("expected app lookup failure, got nil")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("unexpected lookup error: %v", err)
			}
			if got := atomic.LoadInt32(&reservations); got != 0 {
				t.Fatalf("expected no upload reservation, got %d", got)
			}
		})
	}
}

func TestBuildsUploadRejectsBundleMismatchBeforeReservation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	ipaPath := writeBuildUploadIPA(t, "com.example.ipa")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var reservations int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Selected App","bundleId":"com.example.selected"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			atomic.AddInt32(&reservations, 1)
			return nil, http.ErrUseLastResponse
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"builds", "upload",
		"--app", "123456789",
		"--ipa", ipaPath,
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected bundle mismatch, got nil")
	}
	if !strings.Contains(err.Error(), `IPA bundle ID "com.example.ipa" does not match selected app "Selected App" bundle ID "com.example.selected"`) {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
	if got := atomic.LoadInt32(&reservations); got != 0 {
		t.Fatalf("expected no upload reservation, got %d", got)
	}
}

func TestBuildsUploadRequiresTopLevelIPABundleIDBeforeAppLookup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	ipaPath := writeBuildUploadIPA(t, "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"builds", "upload",
		"--app", "123456789",
		"--ipa", ipaPath,
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected missing CFBundleIdentifier failure, got nil")
	}
	if !strings.Contains(err.Error(), "IPA top-level app Info.plist is missing CFBundleIdentifier") {
		t.Fatalf("unexpected missing bundle ID error: %v", err)
	}
}

func writeBuildUploadIPA(t *testing.T, bundleID string) string {
	t.Helper()
	return writeBuildUploadIPAWithPlatform(t, bundleID, "", nil)
}

func writeBuildUploadIPAWithPlatform(t *testing.T, bundleID, platformName string, supportedPlatforms []string) string {
	t.Helper()

	plistData, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         bundleID,
		"CFBundleShortVersionString": "1.0.0",
		"CFBundleVersion":            "42",
		"DTPlatformName":             platformName,
		"CFBundleSupportedPlatforms": supportedPlatforms,
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("marshal Info.plist: %v", err)
	}

	path := filepath.Join(t.TempDir(), "app.ipa")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create IPA: %v", err)
	}
	zipWriter := zip.NewWriter(file)
	entry, err := zipWriter.Create("Payload/Demo.app/Info.plist")
	if err != nil {
		t.Fatalf("create Info.plist entry: %v", err)
	}
	if _, err := entry.Write(plistData); err != nil {
		t.Fatalf("write Info.plist entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close IPA: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close IPA file: %v", err)
	}
	return path
}
