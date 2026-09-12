package notarization

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSnapshotNotarizationArtifactPreservesBytesAndCleansUp(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "artifact.zip")
	original := bytes.Repeat([]byte{'A'}, 256<<10)
	replacement := bytes.Repeat([]byte{'B'}, 256<<10)
	if len(original) != len(replacement) {
		t.Fatalf("test contents differ in size: %d and %d", len(original), len(replacement))
	}
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatal(err)
	}

	var snapshotDirectory string
	previousCreatedHook := snapshotCreatedForTest
	snapshotCreatedForTest = func(path string) { snapshotDirectory = path }
	t.Cleanup(func() { snapshotCreatedForTest = previousCreatedHook })

	snapshot, size, gotHash, cleanup, err := snapshotNotarizationArtifact(context.Background(), source, info.Size())
	if err != nil {
		t.Fatalf("snapshotNotarizationArtifact() error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("snapshotNotarizationArtifact() cleanup = nil")
	}
	if snapshotDirectory == "" {
		t.Fatal("snapshot directory hook was not called")
	}

	if runtime.GOOS != "windows" {
		if info, err := os.Stat(snapshotDirectory); err != nil {
			t.Fatalf("snapshot directory stat: %v", err)
		} else if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("snapshot directory permissions = %#o, want owner-only", info.Mode().Perm())
		}
		if info, err := snapshot.Stat(); err != nil {
			t.Fatalf("snapshot stat: %v", err)
		} else if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("snapshot permissions = %#o, want owner-only", info.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(snapshotDirectory)
	if err != nil {
		t.Fatalf("read snapshot directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("snapshot directory entries = %d, want unlinked snapshot", len(entries))
	}

	if err := rewriteFileInPlace(sourcePath, replacement); err != nil {
		t.Fatal(err)
	}
	gotContents, err := io.ReadAll(snapshot)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !bytes.Equal(gotContents, original) {
		t.Fatalf("snapshot contents = %q, want original contents", gotContents[:min(len(gotContents), 64)])
	}
	if size != int64(len(original)) {
		t.Fatalf("snapshot size = %d, want %d", size, len(original))
	}
	wantDigest := sha256.Sum256(original)
	if gotHash != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("snapshot hash = %q, want %q", gotHash, hex.EncodeToString(wantDigest[:]))
	}

	cleanup()
	if _, err := os.Lstat(snapshotDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot directory after cleanup: %v, want not found", err)
	}
}

func TestSnapshotNotarizationArtifactCancellationCleansUp(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "artifact.zip")
	if err := os.WriteFile(sourcePath, bytes.Repeat([]byte("snapshot bytes\n"), 8<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var snapshotDirectory string
	previousCreatedHook := snapshotCreatedForTest
	previousCopyHook := duringSnapshotCopyForTest
	snapshotCreatedForTest = func(path string) { snapshotDirectory = path }
	duringSnapshotCopyForTest = cancel
	t.Cleanup(func() {
		snapshotCreatedForTest = previousCreatedHook
		duringSnapshotCopyForTest = previousCopyHook
	})

	snapshot, _, _, cleanup, err := snapshotNotarizationArtifact(ctx, source, info.Size())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("snapshotNotarizationArtifact() error = %v, want context.Canceled", err)
	}
	if snapshot != nil || cleanup != nil {
		t.Fatalf("snapshotNotarizationArtifact() returned snapshot=%v cleanup=%v after cancellation", snapshot, cleanup != nil)
	}
	if snapshotDirectory == "" {
		t.Fatal("snapshot directory hook was not called")
	}
	if _, err := os.Lstat(snapshotDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot directory after cancellation: %v, want not found", err)
	}
}

func TestSnapshotNotarizationArtifactRejectsSourceSizeChangeDuringCopy(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "artifact.zip")
	if err := os.WriteFile(sourcePath, bytes.Repeat([]byte("snapshot bytes\n"), 8<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatal(err)
	}

	var snapshotDirectory string
	previousCreatedHook := snapshotCreatedForTest
	previousCopyHook := duringSnapshotCopyForTest
	snapshotCreatedForTest = func(path string) { snapshotDirectory = path }
	duringSnapshotCopyForTest = func() {
		if err := os.Truncate(sourcePath, 0); err != nil {
			t.Errorf("truncate source during snapshot: %v", err)
		}
		duringSnapshotCopyForTest = nil
	}
	t.Cleanup(func() {
		snapshotCreatedForTest = previousCreatedHook
		duringSnapshotCopyForTest = previousCopyHook
	})

	snapshot, _, _, cleanup, err := snapshotNotarizationArtifact(context.Background(), source, info.Size())
	if err == nil {
		t.Fatal("snapshotNotarizationArtifact() error = nil, want source size failure")
	}
	if snapshot != nil || cleanup != nil {
		t.Fatalf("snapshotNotarizationArtifact() returned snapshot=%v cleanup=%v after size failure", snapshot, cleanup != nil)
	}
	if snapshotDirectory == "" {
		t.Fatal("snapshot directory hook was not called")
	}
	if _, err := os.Lstat(snapshotDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot directory after size failure: %v, want not found", err)
	}
}

func TestSnapshotNotarizationArtifactRejectsSourceSizeChange(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "artifact.zip")
	if err := os.WriteFile(sourcePath, []byte("snapshot bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if _, _, _, cleanup, err := snapshotNotarizationArtifact(context.Background(), source, info.Size()+1); err == nil {
		t.Fatal("snapshotNotarizationArtifact() error = nil, want size mismatch")
	} else if cleanup != nil {
		t.Fatal("snapshotNotarizationArtifact() returned cleanup after size mismatch")
	}
}

func rewriteFileInPlace(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
