package shared

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// OpenWorkingDirectoryFile opens path for reading when it names a regular file
// inside d's working directory, and is the only way a command reads a file
// whose content it sends to a recipient outside the machine. A relative path
// resolves against the working directory; an absolute path must already point
// inside it. Parent traversal, a symlink at any component below the working
// directory, and non-regular files (directories, devices, FIFOs) are refused
// before any content is read.
//
// A refusal is returned as a plain error that names flagName and suggests hint
// (for example "use --message, or copy the file into the project directory");
// callers decide how to report it. A missing or unreadable file inside the
// directory is a read error, not a refusal.
func (d *Deps) OpenWorkingDirectoryFile(flagName, hint, path string) (*os.File, error) {
	refusal := fmt.Errorf("%s must be a regular file inside the working directory; %s", flagName, hint)
	workingDir, err := d.Getwd()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(workingDir, path)
	}
	anchor, relative, ok := relativeToWorkingDirectory(workingDir, filepath.Clean(absolute))
	if !ok {
		return nil, refusal
	}
	root, err := rootfs.New(anchor)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	defer root.Close()
	file, err := root.OpenFile(relative)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("read %s: %w", flagName, err)
		}
		return nil, refusal
	}
	return file, nil
}

// relativeToWorkingDirectory returns the anchor directory and the path of
// absolute beneath it. The working directory is tried as the process reports it
// and then with symlinks resolved, so an absolute path spelled either way inside
// the directory is accepted. A path equal to the anchor, or outside it, is not.
func relativeToWorkingDirectory(workingDir, absolute string) (string, string, bool) {
	anchors := []string{workingDir}
	if physical, err := filepath.EvalSymlinks(workingDir); err == nil && physical != workingDir {
		anchors = append(anchors, physical)
	}
	for _, anchor := range anchors {
		relative, err := filepath.Rel(anchor, absolute)
		if err != nil || relative == "." || relative == ".." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			continue
		}
		return anchor, relative, true
	}
	return "", "", false
}
