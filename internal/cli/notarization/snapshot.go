package notarization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

var (
	snapshotCreatedForTest    func(string)
	duringSnapshotCopyForTest func()
)

// snapshotNotarizationArtifact copies the opened source into a private,
// operation-owned file and hashes the bytes written to that file. The caller
// must call the returned cleanup function after it is done with the snapshot.
// Subsequent changes to the source path or inode cannot change the bytes used
// for the submission hash or S3 upload.
func snapshotNotarizationArtifact(ctx context.Context, source *os.File, size int64) (*os.File, int64, string, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, "", nil, err
	}
	if source == nil {
		return nil, 0, "", nil, fmt.Errorf("notarization source is nil")
	}
	if size <= 0 {
		return nil, 0, "", nil, fmt.Errorf("notarization source size must be positive")
	}

	info, err := source.Stat()
	if err != nil {
		return nil, 0, "", nil, fmt.Errorf("inspect notarization source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, "", nil, fmt.Errorf("notarization source is not a regular file")
	}
	if info.Size() != size {
		return nil, 0, "", nil, fmt.Errorf("notarization source size changed before snapshot")
	}

	directory, err := os.MkdirTemp("", ".asc-notarization-snapshot-*")
	if err != nil {
		return nil, 0, "", nil, fmt.Errorf("create notarization snapshot directory: %w", err)
	}
	cleanupDirectory := func() { _ = os.Remove(directory) }
	if snapshotCreatedForTest != nil {
		snapshotCreatedForTest(directory)
	}

	trustedDirectory, err := rootfs.New(directory)
	if err != nil {
		cleanupDirectory()
		return nil, 0, "", nil, fmt.Errorf("anchor notarization snapshot directory: %w", err)
	}
	openedDirectory, err := trustedDirectory.OpenRoot()
	if err != nil {
		_ = trustedDirectory.Close()
		cleanupDirectory()
		return nil, 0, "", nil, fmt.Errorf("open notarization snapshot directory: %w", err)
	}

	snapshot, snapshotName, err := secureopen.CreateTempNoFollowInRootWithCreator(
		openedDirectory,
		".",
		".artifact-*.snapshot",
		0o600,
		secureopen.OpenNewPrivateFileNoFollowInRoot,
	)
	if err != nil {
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		cleanupDirectory()
		return nil, 0, "", nil, fmt.Errorf("create notarization snapshot: %w", err)
	}
	closeAndRemove := func() {
		_ = snapshot.Close()
		_ = openedDirectory.Remove(snapshotName)
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		cleanupDirectory()
	}

	if err := secureopen.PreparePrivateFile(snapshot, 0o600); err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("secure notarization snapshot: %w", err)
	}

	hash := sha256.New()
	written, err := copySnapshotWithContext(
		ctx,
		io.MultiWriter(snapshot, hash),
		io.NewSectionReader(source, 0, size),
		duringSnapshotCopyForTest,
	)
	if err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("copy notarization snapshot: %w", err)
	}
	if written != size {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("copy notarization snapshot: copied %d of %d bytes", written, size)
	}

	// A size change during the copy means the source was not a stable input for
	// this operation. Same-size rewrites remain harmless because all later
	// stages consume the completed snapshot rather than the source descriptor.
	afterInfo, err := source.Stat()
	if err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("reinspect notarization source after snapshot: %w", err)
	}
	if !afterInfo.Mode().IsRegular() || afterInfo.Size() != size {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("notarization source size changed during snapshot")
	}

	if err := snapshot.Sync(); err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("sync notarization snapshot: %w", err)
	}
	snapshotInfo, err := snapshot.Stat()
	if err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("inspect notarization snapshot: %w", err)
	}
	if !snapshotInfo.Mode().IsRegular() || snapshotInfo.Size() != size {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("notarization snapshot size is invalid")
	}
	if err := snapshot.Close(); err != nil {
		_ = openedDirectory.Remove(snapshotName)
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		cleanupDirectory()
		return nil, 0, "", nil, fmt.Errorf("close notarization snapshot: %w", err)
	}

	snapshot, err = secureopen.OpenExistingNoFollowInRoot(openedDirectory, snapshotName)
	if err != nil {
		_ = openedDirectory.Remove(snapshotName)
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		cleanupDirectory()
		return nil, 0, "", nil, fmt.Errorf("reopen notarization snapshot: %w", err)
	}
	reopenedInfo, err := snapshot.Stat()
	if err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("inspect reopened notarization snapshot: %w", err)
	}
	if !os.SameFile(snapshotInfo, reopenedInfo) || !reopenedInfo.Mode().IsRegular() || reopenedInfo.Size() != size {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("notarization snapshot changed while reopening")
	}
	if err := openedDirectory.Remove(snapshotName); err != nil {
		closeAndRemove()
		return nil, 0, "", nil, fmt.Errorf("unlink notarization snapshot: %w", err)
	}

	cleanup := func() {
		_ = snapshot.Close()
		_ = openedDirectory.Close()
		_ = trustedDirectory.Close()
		cleanupDirectory()
	}
	return snapshot, size, hex.EncodeToString(hash.Sum(nil)), cleanup, nil
}

func copySnapshotWithContext(ctx context.Context, destination io.Writer, source io.Reader, hook func()) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	buffer := make([]byte, 64<<10)
	var written int64
	for {
		if hook != nil {
			hook()
		}
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
