package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommunityWallSourceFileIsCanonical(t *testing.T) {
	sourcePath := filepath.Join("..", "..", "..", "docs", "wall-of-apps.json")
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read wall source: %v", err)
	}

	entries, err := parseCommunityWallSourceEntries(raw, sourcePath)
	if err != nil {
		t.Fatalf("parse wall source: %v", err)
	}

	rendered, err := renderCommunityWallSourceEntries(entries)
	if err != nil {
		t.Fatalf("render wall source: %v", err)
	}

	if string(raw) != rendered {
		t.Fatalf("docs/wall-of-apps.json is not in canonical order/format")
	}
}
