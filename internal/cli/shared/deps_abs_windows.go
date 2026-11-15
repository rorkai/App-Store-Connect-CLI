//go:build windows

package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

var processGetwd = reflect.ValueOf(os.Getwd).Pointer()

// Abs is filepath.Abs with a relative path resolved against d.Getwd instead
// of the process directory. On Windows filepath.Abs asks the system for the
// full path name, so with the process directory, and for absolute paths, Abs
// is filepath.Abs itself. With another directory, a rooted path takes that
// directory's volume, and a relative path on that volume is joined to it; a
// path relative to another drive's working directory, which Windows keeps per
// drive, still resolves through filepath.Abs.
func (d *Deps) Abs(path string) (string, error) {
	if reflect.ValueOf(d.Getwd).Pointer() == processGetwd || filepath.IsAbs(path) {
		return filepath.Abs(path)
	}
	wd, err := d.Getwd()
	if err != nil {
		return "", err
	}
	volume := filepath.VolumeName(path)
	switch {
	case volume == "" && path != "" && os.IsPathSeparator(path[0]):
		return filepath.Clean(filepath.VolumeName(wd) + path), nil
	case volume == "":
		return filepath.Join(wd, path), nil
	case strings.EqualFold(volume, filepath.VolumeName(wd)):
		return filepath.Join(wd, path[len(volume):]), nil
	default:
		return filepath.Abs(path)
	}
}
