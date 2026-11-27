package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// OpenMessageFile opens message content. Outside-directory
// files remain supported with a deprecation warning. followSymlinks preserves
// legacy symlink support only for callers that already allowed it; those reads
// also warn. Regular files are opened through rootfs; legacy streams and OS
// descriptors retain their original opening semantics. Read errors
// retain their causes for the caller's existing error classification.
func (d *Deps) OpenMessageFile(flagName, hint, path string, followSymlinks bool) (*os.File, error) {
	workingDir, err := d.Getwd()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	absolute, err := d.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	if followSymlinks {
		source := d.OSPath(path)
		info, statErr := os.Stat(source)
		stream := statErr == nil && !info.Mode().IsRegular() && !info.IsDir()
		if stream || messageFileDescriptorPath(absolute) {
			// Legacy Slack and batch inputs accepted OS streams and descriptors.
			// These cannot satisfy rootfs regular-file identity checks; ordinary
			// file inputs below retain the rooted traversal.
			file, err := os.Open(source)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", flagName, err)
			}
			fmt.Fprintf(d.IO.ErrOut, "Warning: %s non-regular or descriptor input is deprecated and will be refused in a future release; %s\n", flagName, hint)
			return file, nil
		}
	}
	openPath := absolute
	if followSymlinks {
		// Windows resolves drive-relative paths and dot segments before opening.
		// Unix walks the original components, so link/../file can name another file.
		source := absolute
		if runtime.GOOS != "windows" {
			source = d.OSPath(path)
		}
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", flagName, err)
		}
		openPath, err = d.Abs(resolved)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", flagName, err)
		}
	}
	file, err := rootfs.OpenFileIn(workingDir, openPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	if messageFileNeedsWarning(workingDir, absolute, openPath, followSymlinks) {
		fmt.Fprintf(d.IO.ErrOut, "Warning: %s outside the working directory or through a symlink is deprecated and will be refused in a future release; %s\n", flagName, hint)
	}
	return file, nil
}

func messageFileNeedsWarning(workingDir, lexicalPath, openPath string, followed bool) bool {
	physicalDir, err := filepath.EvalSymlinks(workingDir)
	if err != nil {
		return true
	}
	if !followed || workingDir != physicalDir {
		openPath, err = filepath.EvalSymlinks(openPath)
		if err != nil {
			return true
		}
	}
	if !pathWithinDirectory(physicalDir, openPath) {
		return true
	}
	if !followed {
		return false
	}
	if !pathWithinDirectory(workingDir, lexicalPath) {
		// Trusted root aliases such as /var and /private/var are not symlink
		// inputs below the working directory.
		if rootfs.CheckContainedPath(lexicalPath) != nil {
			return true
		}
		physicalLexical, err := filepath.EvalSymlinks(lexicalPath)
		if err != nil {
			return true
		}
		selected, err := filepath.Rel(physicalLexical, openPath)
		return err != nil || selected != "."
	}
	relative, _ := filepath.Rel(workingDir, lexicalPath)
	selected, err := filepath.Rel(filepath.Join(physicalDir, relative), openPath)
	return err != nil || selected != "."
}

func pathWithinDirectory(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && filepath.IsLocal(relative)
}

// Descriptor paths refer to an already-open operator-selected input. Resolving
// their symlinks may lose the descriptor (or fail for a pipe on Linux).
func messageFileDescriptorPath(path string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if path == "/dev/stdin" || path == "/dev/stdout" || path == "/dev/stderr" {
		return true
	}
	for _, prefix := range []string{"/dev/fd/", "/proc/self/fd/"} {
		if suffix, ok := strings.CutPrefix(path, prefix); ok {
			_, err := strconv.ParseUint(suffix, 10, 64)
			return err == nil
		}
	}
	return false
}
