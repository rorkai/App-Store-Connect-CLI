package xcode

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotValidationArtifactRejectsSameSizeSourceMutation(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "Demo.ipa")
	original := bytes.Repeat([]byte{'A'}, 128<<10)
	replacement := bytes.Repeat([]byte{'B'}, len(original))
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}

	previousHook := afterValidationSnapshotCopyForTest
	afterValidationSnapshotCopyForTest = func() {
		if err := os.WriteFile(sourcePath, replacement, 0o600); err != nil {
			t.Fatalf("rewrite source: %v", err)
		}
	}
	t.Cleanup(func() { afterValidationSnapshotCopyForTest = previousHook })

	snapshotPath, cleanup, err := snapshotValidationArtifact(context.Background(), source, info.Size(), ".ipa")
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatalf("snapshotValidationArtifact() path = %q, want mutation error", snapshotPath)
	}
	if !strings.Contains(err.Error(), "validation source changed during snapshot") {
		t.Fatalf("snapshotValidationArtifact() error = %v, want source mutation error", err)
	}
}
