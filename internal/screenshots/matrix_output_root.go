package screenshots

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// matrixOutputRootBeforeChildRootForTest is a test-only seam placed after the
// parent has anchored the selected child and before the child root is opened.
var matrixOutputRootBeforeChildRootForTest func()

func openMatrixOutputRoot(path string) (rootfs.Root, error) {
	if strings.TrimSpace(path) == "" {
		return rootfs.Root{}, errors.New("matrix output path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return rootfs.Root{}, err
	}
	absPath = filepath.Clean(absPath)
	parentPath := filepath.Dir(absPath)
	name := filepath.Base(absPath)
	parent, err := rootfs.New(parentPath)
	if err != nil {
		return rootfs.Root{}, err
	}
	defer func() { _ = parent.Close() }()
	if err := parent.MkdirAll(".", 0o755); err != nil {
		return rootfs.Root{}, err
	}
	if err := parent.CheckContained(name); err != nil {
		return rootfs.Root{}, err
	}
	if err := parent.MkdirAll(name, 0o755); err != nil {
		return rootfs.Root{}, err
	}
	if err := parent.CheckContained(name); err != nil {
		return rootfs.Root{}, err
	}
	anchoredChild, err := parent.OpenDir(name)
	if err != nil {
		return rootfs.Root{}, err
	}
	defer anchoredChild.Close()
	if matrixOutputRootBeforeChildRootForTest != nil {
		matrixOutputRootBeforeChildRootForTest()
	}
	root, err := rootfs.New(absPath)
	if err != nil {
		return rootfs.Root{}, err
	}
	openedChild, openErr := root.OpenRoot()
	if openErr != nil {
		_ = root.Close()
		return rootfs.Root{}, openErr
	}
	anchoredInfo, anchorErr := anchoredChild.Stat()
	openedInfo, openedErr := openedChild.Stat(".")
	closeErr := openedChild.Close()
	if anchorErr != nil || openedErr != nil || closeErr != nil {
		_ = root.Close()
		return rootfs.Root{}, errors.Join(anchorErr, openedErr, closeErr)
	}
	if !os.SameFile(anchoredInfo, openedInfo) {
		_ = root.Close()
		return rootfs.Root{}, errors.New("matrix output root changed while opening")
	}
	if err := root.MkdirAll(".", 0o755); err != nil {
		_ = root.Close()
		return rootfs.Root{}, err
	}
	return root, nil
}

func relativeMatrixOutputPath(rootPath, path string) (string, error) {
	rootPath = filepath.Clean(rootPath)
	path = filepath.Clean(path)
	relative, err := filepath.Rel(rootPath, path)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %q escapes matrix output root", path)
	}
	return relative, nil
}

func pinMatrixFrameInputFromRetainedRoot(ctx context.Context, rawRoot rootfs.Root, inputPath string, destRoot rootfs.Root, destDir string, index int) (string, error) {
	relative, err := relativeMatrixOutputPath(rawRoot.Path(), inputPath)
	if err != nil {
		return "", err
	}
	src, err := rawRoot.OpenFile(relative)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf(".asc-matrix-frame-input-%d%s", index, filepath.Ext(relative))
	written, writeErr := destRoot.WriteFromPreservingMode(name, &matrixContextReader{ctx: ctx, reader: io.LimitReader(src, maxMatrixArtifactBytes+1)}, 0o600)
	closeErr := src.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if written > maxMatrixArtifactBytes {
		return "", errors.New("raw screenshot exceeds the artifact size limit")
	}
	return filepath.Join(destDir, name), nil
}
