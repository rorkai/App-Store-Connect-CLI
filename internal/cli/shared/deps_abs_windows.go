//go:build windows

package shared

import (
	"os"
	"path/filepath"
	"reflect"
)

var processGetwd = reflect.ValueOf(os.Getwd).Pointer()

// Abs is filepath.Abs with a relative path resolved against d.Getwd instead
// of the process directory. On Windows filepath.Abs asks the system for the
// full path name, which also resolves rooted and drive-relative paths and
// normalises the last element, so with the process directory, and for paths
// that depend on the process's drive state, Abs is filepath.Abs itself. Another
// directory has a relative path joined to it.
func (d *Deps) Abs(path string) (string, error) {
	if reflect.ValueOf(d.Getwd).Pointer() == processGetwd || filepath.VolumeName(path) != "" || (path != "" && os.IsPathSeparator(path[0])) {
		return filepath.Abs(path)
	}
	wd, err := d.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, path), nil
}
