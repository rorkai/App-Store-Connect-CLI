//go:build !windows

package shared

import "path/filepath"

// Abs is filepath.Abs with a relative path resolved against d.Getwd instead
// of the process directory: on Unix, filepath.Abs cleans an absolute path and
// joins a relative one to the working directory.
func (d *Deps) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	wd, err := d.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, path), nil
}
