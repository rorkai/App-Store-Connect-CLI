package builds

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDSYMSelectionExactVersionDoesNotRequireLatest(t *testing.T) {
	selection, err := parseDSYMSelection(dsymFlagInput{
		AppID:   "123",
		Version: "1.2.3",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !selection.selectsByMarketingVersion() || selection.Resolve.Latest || selection.All || selection.Resolve.Version != "1.2.3" {
		t.Fatalf("selection = %+v, want exact version without --latest", selection)
	}
}

func TestParseDSYMSelectionVersionLatestMatchesLatestFlag(t *testing.T) {
	selection, err := parseDSYMSelection(dsymFlagInput{
		AppID:   "123",
		Version: "latest",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !selection.Resolve.Latest || selection.Resolve.Version != "" || selection.Live || selection.Multi {
		t.Fatalf("selection = %+v, want latest without a version filter", selection)
	}
}

func TestParseDSYMSelectionRejectsMalformedExactVersions(t *testing.T) {
	for _, version := range []string{"foo", "1..2", "-1.2"} {
		t.Run(version, func(t *testing.T) {
			_, err := parseDSYMSelection(dsymFlagInput{AppID: "123", Version: version})
			if err == nil || !strings.Contains(err.Error(), "--version must be a dotted numeric version") {
				t.Fatalf("error = %v, want invalid exact version", err)
			}
		})
	}
}

func TestParseDSYMSelectionRejectsTimeoutWithoutWait(t *testing.T) {
	_, err := parseDSYMSelection(dsymFlagInput{
		BuildID:    "build-1",
		Timeout:    time.Second,
		TimeoutSet: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--timeout and --poll-interval require --wait") {
		t.Fatalf("error = %v, want timeout requiring --wait", err)
	}
}

func TestChooseLiveVersionRequiresPlatformWhenSeveralAreLive(t *testing.T) {
	_, err := chooseLiveVersion([]liveAppVersion{
		{Version: "2.0", Platform: "IOS", CreatedDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		{Version: "1.4", Platform: "MAC_OS", CreatedDate: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "--platform") || !strings.Contains(err.Error(), "IOS 2.0") || !strings.Contains(err.Error(), "MAC_OS 1.4") {
		t.Fatalf("error = %v, want both live platforms", err)
	}
}

func TestCompareMarketingVersions(t *testing.T) {
	got, err := compareMarketingVersions("1.2", "1.2.0")
	if err != nil || got != 0 {
		t.Fatalf("1.2 vs 1.2.0 = %d, %v", got, err)
	}
	got, err = compareMarketingVersions("1.10", "1.9")
	if err != nil || got <= 0 {
		t.Fatalf("1.10 vs 1.9 = %d, %v", got, err)
	}
	got, err = compareMarketingVersions("1.1.9", "1.2.0")
	if err != nil || got >= 0 {
		t.Fatalf("1.1.9 vs 1.2.0 = %d, %v", got, err)
	}
}

func TestSaveSingleDSYMVerifiesExistingFileContent(t *testing.T) {
	for _, remote := range []string{"mac", "ios"} {
		t.Run(remote, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "com.example.app-1.0-42.dSYM.zip")
			if err := os.WriteFile(path, []byte("ios"), 0o600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "3")
				_, _ = io.WriteString(w, remote)
			}))
			defer server.Close()
			target := dsymTarget{ID: "build", AppVersion: "1.0", BuildNumber: "42"}
			bundles := []dsymBundleInfo{{BundleID: "com.example.app", DSYMURL: &server.URL}}
			files, err := saveDSYMBundles(t.Context(), bundles, target, dir, false)
			if remote != "ios" {
				if err == nil || !strings.Contains(err.Error(), "already exists") {
					t.Fatalf("different single-build artifact was trusted: files=%#v err=%v", files, err)
				}
				if len(files) != 0 {
					t.Fatalf("unexpected download receipt: %#v", files)
				}
			} else if err != nil || len(files) != 1 || !files[0].Skipped {
				t.Fatalf("identical existing artifact was not reused: files=%#v err=%v", files, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "ios" {
				t.Fatalf("existing file changed to %q", data)
			}
		})
	}
}
