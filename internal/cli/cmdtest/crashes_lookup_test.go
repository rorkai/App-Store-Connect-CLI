package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestCrashesNextURLSkipsAppLookupForNonNumericApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-lookup/betaFeedbackCrashSubmissions?cursor=AQ"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-lookup/betaFeedbackCrashSubmissions", "cursor=AQ"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-next"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"testflight", "crashes", "list",
		"--next", nextURL,
		"--app", "com.example.crashes",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"crash-next"`) {
		t.Fatalf("expected next-page crash output, got %q", stdout)
	}
}
