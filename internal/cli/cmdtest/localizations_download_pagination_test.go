package cmdtest

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsDownloadWritesEveryPageWithoutPaginateFlag(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=page-2"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English"}}],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "cursor=page-2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"日本語"}}],"links":{"next":""}}`))

	dir := t.TempDir()
	_, stderr, err := runCommand(t, []string{"localizations", "download", "--version", "version-1", "--path", dir})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, name := range []string{"en-US.strings", "ja.strings"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to be written: %v", name, err)
		}
	}
}
