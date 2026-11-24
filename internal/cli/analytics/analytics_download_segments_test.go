package analytics

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func TestPublishAnalyticsDirectoryWithoutNoReplaceRename(t *testing.T) {
	previous := renameNoReplaceInRoot
	t.Cleanup(func() { renameNoReplaceInRoot = previous })
	renameNoReplaceInRoot = func(*os.Root, string, string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}
	files := []string{"segment-001.txt.gz", "segment-001.txt"}
	stage := func(t *testing.T) (string, *os.Root) {
		t.Helper()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "staging"), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range files {
			if err := os.WriteFile(filepath.Join(dir, "staging", name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		parent, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = parent.Close() })
		return dir, parent
	}

	t.Run("new destination", func(t *testing.T) {
		dir, parent := stage(t)
		stagingGone, err := publishAnalyticsDirectory(parent, "staging", "report", files)
		if err != nil || stagingGone {
			t.Fatalf("publish: stagingGone=%v err=%v, want the files moved out of a remaining staging directory", stagingGone, err)
		}
		for _, name := range files {
			data, err := os.ReadFile(filepath.Join(dir, "report", name))
			if err != nil || string(data) != name {
				t.Errorf("published %s = %q, %v", name, data, err)
			}
		}
		left, err := os.ReadDir(filepath.Join(dir, "staging"))
		if err != nil || len(left) != 0 {
			t.Errorf("staging holds %v (%v), want it empty", left, err)
		}
	})

	t.Run("existing destination", func(t *testing.T) {
		dir, parent := stage(t)
		kept := filepath.Join(dir, "report", "segment-001.txt")
		if err := os.Mkdir(filepath.Join(dir, "report"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(kept, []byte("earlier download"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := publishAnalyticsDirectory(parent, "staging", "report", files); !errors.Is(err, os.ErrExist) {
			t.Fatalf("publish error = %v, want os.ErrExist", err)
		}
		if data, err := os.ReadFile(kept); err != nil || string(data) != "earlier download" {
			t.Errorf("existing file = %q, %v, want it untouched", data, err)
		}
		if entries, err := os.ReadDir(filepath.Join(dir, "report")); err != nil || len(entries) != 1 {
			t.Errorf("existing directory holds %v (%v), want only its own file", entries, err)
		}
	})
}
