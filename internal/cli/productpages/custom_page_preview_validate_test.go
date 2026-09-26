package productpages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestCustomPagePreviewSyncValidatesFilesBeforeContactingApple(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("not a preview"), 0o600); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	called := false
	original := customPageMediaClientFactory
	customPageMediaClientFactory = func() (*asc.Client, error) {
		called = true
		return nil, context.Canceled
	}
	t.Cleanup(func() { customPageMediaClientFactory = original })

	_, err := executeCustomPagePreviewUpload(context.Background(), "loc-1", dir, "IPHONE_65", true)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ds_store") {
		t.Fatalf("error = %v, want a .DS_Store validation failure", err)
	}
	if called {
		t.Fatal("sync contacted Apple before validating replacement files")
	}
}
