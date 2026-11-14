package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMetadataPullDefaultsToEditableVersionWhenVersionOmitted(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	outputDir := filepath.Join(t.TempDir(), "metadata")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", editableVersionStateQuery, "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US","name":"App Name"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"version-loc-1","attributes":{"locale":"en-US","description":"English description"}}],"links":{"next":""}}`))

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

func TestMetadataPullDefaultVersionSelectionErrorsDoNotWriteFiles(t *testing.T) {
	tests := []struct {
		name     string
		versions map[string]string
		wantExit int
	}{
		{
			name: "ambiguous",
			versions: map[string]string{
				editableVersionStateQuery: `{"data":[
					{"type":"appStoreVersions","id":"version-ios","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}},
					{"type":"appStoreVersions","id":"version-mac","attributes":{"versionString":"1.2.3","platform":"MAC_OS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
				],"links":{"next":""}}`,
			},
			wantExit: rootcmd.ExitUsage,
		},
		{
			name: "missing",
			versions: map[string]string{
				editableVersionStateQuery:   `{"data":[],"links":{"next":""}}`,
				removedEditableStateQuery:   `{"data":[],"links":{"next":""}}`,
				liveVersionStateQuery:       `{"data":[],"links":{"next":""}}`,
				liveVersionModernStateQuery: `{"data":[],"links":{"next":""}}`,
			},
			wantExit: rootcmd.ExitNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			outputDir := filepath.Join(t.TempDir(), "metadata")

			reg := asctest.NewRegistry(t)
			reg.Install()
			for _, query := range []string{editableVersionStateQuery, removedEditableStateQuery, liveVersionStateQuery, liveVersionModernStateQuery} {
				if body, ok := test.versions[query]; ok {
					reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", query, "limit=200"), asctest.JSON(http.StatusOK, body))
				}
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse([]string{"metadata", "pull", "--app", "app-1", "--dir", outputDir}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d; err=%v", got, test.wantExit, runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
				t.Fatalf("selection failure created output path %q: %v", outputDir, err)
			}
		})
	}
}
