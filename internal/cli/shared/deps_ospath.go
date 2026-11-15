package shared

import "path/filepath"

// OSPath is the path to hand to the operating system for path, a path given
// relative to d's working directory: path itself when filepath.Abs resolves it
// to the same place as d.Abs, so the operating system sees and reports the
// path as given, and d.Abs(path) otherwise.
func (d *Deps) OSPath(path string) string {
	resolved, err := d.Abs(path)
	if err != nil {
		return path
	}
	if process, err := filepath.Abs(path); err == nil && process == resolved {
		return path
	}
	return resolved
}
