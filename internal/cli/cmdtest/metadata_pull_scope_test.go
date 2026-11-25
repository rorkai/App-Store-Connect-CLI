package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMetadataPullDefaultVersionRefusesForeignContinuationBeforeContentReads(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	outputDir := filepath.Join(t.TempDir(), "metadata")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", editableVersionStateQuery, "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/other/appStoreVersions?cursor=foreign"}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "pull", "--app", "123456789", "--dir", outputDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected foreign version continuation to fail closed")
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want generic error; err=%v", rootcmd.ExitCodeFromError(runErr), runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty before metadata reads or writes", stdout)
	}
	if requests := reg.Requests(); len(requests) != 1 || requests[0].URL.Path != "/v1/apps/123456789/appStoreVersions" {
		t.Fatalf("requests=%v, want only the selected-app version collection page", requests)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("selection failure created output path %q: %v", outputDir, err)
	}
}
