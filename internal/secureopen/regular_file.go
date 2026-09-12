package secureopen

import (
	"fmt"
	"os"
)

// OpenExistingRegularFileNoFollow opens an existing non-empty regular file
// without following a symlink in its final path component. The returned file
// handle pins the inode checked during validation; callers own the handle.
//
// artifactName is used in errors that describe stat/open failures, while
// flagName is used for command-facing validation errors.
func OpenExistingRegularFileNoFollow(path, artifactName, flagName string) (*os.File, os.FileInfo, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat %s: %w", artifactName, err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, regularFileSymlinkError(path, flagName)
	}
	if err := validateNonEmptyRegularFile(pathInfo, flagName); err != nil {
		return nil, nil, err
	}

	file, err := OpenExistingNoFollow(path)
	if err != nil {
		if latestInfo, statErr := os.Lstat(path); statErr == nil && latestInfo.Mode()&os.ModeSymlink != 0 {
			return nil, nil, regularFileSymlinkError(path, flagName)
		}
		return nil, nil, fmt.Errorf("failed to open %s: %w", artifactName, err)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("failed to stat opened %s: %w", artifactName, err)
	}
	if !os.SameFile(pathInfo, fileInfo) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%s changed while being opened", flagName)
	}
	if err := validateNonEmptyRegularFile(fileInfo, flagName); err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, fileInfo, nil
}

func validateNonEmptyRegularFile(fileInfo os.FileInfo, flagName string) error {
	if fileInfo.IsDir() {
		return fmt.Errorf("%s must be a file", flagName)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", flagName)
	}
	if fileInfo.Size() == 0 {
		return fmt.Errorf("%s must not be empty", flagName)
	}
	return nil
}

func regularFileSymlinkError(path, flagName string) error {
	return fmt.Errorf("refusing to read symlink %q from %s", path, flagName)
}
