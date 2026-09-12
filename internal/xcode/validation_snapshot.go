package xcode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

var afterValidationSnapshotCopyForTest func()

// snapshotValidationArtifact copies a safely opened artifact into a private,
// correctly suffixed path for altool. Apple's validation stack identifies the
// archive from its pathname and may open it from a child process, so an
// inherited descriptor path such as /dev/fd/3 is not sufficient.
func snapshotValidationArtifact(ctx context.Context, source *os.File, size int64, suffix string) (string, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if source == nil {
		return "", nil, fmt.Errorf("validation source is nil")
	}
	if size <= 0 {
		return "", nil, fmt.Errorf("validation source size must be positive")
	}
	if suffix != ".ipa" && suffix != ".pkg" {
		return "", nil, fmt.Errorf("unsupported validation artifact suffix %q", suffix)
	}

	sourceInfo, err := source.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("inspect validation source: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() || sourceInfo.Size() != size {
		return "", nil, fmt.Errorf("validation source changed before snapshot")
	}

	directory, err := os.MkdirTemp("", ".asc-xcode-validate-*")
	if err != nil {
		return "", nil, fmt.Errorf("create validation snapshot directory: %w", err)
	}
	removeDirectory := func() { _ = os.Remove(directory) }

	trustedDirectory, err := rootfs.New(directory)
	if err != nil {
		removeDirectory()
		return "", nil, fmt.Errorf("anchor validation snapshot directory: %w", err)
	}
	openedDirectory, err := trustedDirectory.OpenRoot()
	if err != nil {
		_ = trustedDirectory.Close()
		removeDirectory()
		return "", nil, fmt.Errorf("open validation snapshot directory: %w", err)
	}

	snapshot, snapshotName, err := secureopen.CreateTempNoFollowInRootWithCreator(
		openedDirectory,
		".",
		".artifact-*"+suffix,
		0o600,
		secureopen.OpenNewPrivateFileNoFollowInRoot,
	)
	if err != nil {
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		removeDirectory()
		return "", nil, fmt.Errorf("create validation snapshot: %w", err)
	}
	cleanup := func() {
		_ = snapshot.Close()
		_ = openedDirectory.Remove(snapshotName)
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		removeDirectory()
	}

	if err := secureopen.PreparePrivateFile(snapshot, 0o600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("secure validation snapshot: %w", err)
	}

	copiedHash := sha256.New()
	written, err := copyValidationSnapshotWithContext(ctx, io.MultiWriter(snapshot, copiedHash), io.NewSectionReader(source, 0, size))
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("copy validation snapshot: %w", err)
	}
	if written != size {
		cleanup()
		return "", nil, fmt.Errorf("copy validation snapshot: copied %d of %d bytes", written, size)
	}
	if afterValidationSnapshotCopyForTest != nil {
		afterValidationSnapshotCopyForTest()
	}
	verifiedHash := sha256.New()
	verified, err := copyValidationSnapshotWithContext(ctx, verifiedHash, io.NewSectionReader(source, 0, size))
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("verify validation source after snapshot: %w", err)
	}
	if verified != size || !bytes.Equal(copiedHash.Sum(nil), verifiedHash.Sum(nil)) {
		cleanup()
		return "", nil, fmt.Errorf("validation source changed during snapshot")
	}

	afterSourceInfo, err := source.Stat()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("reinspect validation source: %w", err)
	}
	if !afterSourceInfo.Mode().IsRegular() || afterSourceInfo.Size() != size {
		cleanup()
		return "", nil, fmt.Errorf("validation source changed during snapshot")
	}
	if err := snapshot.Sync(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("sync validation snapshot: %w", err)
	}
	snapshotInfo, err := snapshot.Stat()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("inspect validation snapshot: %w", err)
	}
	if !snapshotInfo.Mode().IsRegular() || snapshotInfo.Size() != size {
		cleanup()
		return "", nil, fmt.Errorf("validation snapshot size is invalid")
	}
	if err := snapshot.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close validation snapshot: %w", err)
	}

	snapshotPath := filepath.Join(directory, snapshotName)
	namedInfo, err := os.Lstat(snapshotPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("inspect named validation snapshot: %w", err)
	}
	if namedInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(snapshotInfo, namedInfo) || !namedInfo.Mode().IsRegular() || namedInfo.Size() != size {
		cleanup()
		return "", nil, fmt.Errorf("validation snapshot changed before use")
	}

	return snapshotPath, cleanup, nil
}

func copyValidationSnapshotWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 64<<10)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if err := ctx.Err(); err != nil {
				return written, err
			}
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}
