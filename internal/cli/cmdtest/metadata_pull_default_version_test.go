package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataPullDefaultsToEditableVersionWhenVersionOmitted(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	outputDir := filepath.Join(t.TempDir(), "metadata")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			key := appStoreVersionsQueryKey(req.URL.Query())
			if key != editableVersionStateQuery {
				t.Fatalf("unexpected versions query %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}],"links":{"next":""}}`)
		case "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`)
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US","name":"App Name"}}],"links":{"next":""}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"version-loc-1","attributes":{"locale":"en-US","description":"English description"}}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "pull", "--app", "app-1", "--dir", outputDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	wantNote := "Using version 1.2.3 (PREPARE_FOR_SUBMISSION) for platform IOS; pass --version to override\n"
	if stderr != wantNote {
		t.Fatalf("stderr = %q, want %q", stderr, wantNote)
	}
	if !strings.Contains(stdout, `"version":"1.2.3"`) || !strings.Contains(stdout, `"versionId":"version-1"`) {
		t.Fatalf("expected resolved version in output, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "version", "1.2.3", "en-US.json")); err != nil {
		t.Fatalf("expected version file under resolved version directory: %v", err)
	}
}
