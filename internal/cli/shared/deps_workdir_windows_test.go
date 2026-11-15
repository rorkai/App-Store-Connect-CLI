//go:build windows

package shared

import (
	"path/filepath"
	"testing"
)

func TestDefaultDepsAbsIsFilepathAbsOnWindows(t *testing.T) {
	d := DefaultDeps()
	for _, path := range []string{"", ".", `report.tsv`, `C:report.tsv`, `\report.tsv`, `C:\Users\..\report.tsv`, `\\server\share\report.tsv`, `\\?\C:\report.tsv`, `report.tsv. `, `nul`, `sub/../report.tsv`} {
		want, wantErr := filepath.Abs(path)
		got, err := d.Abs(path)
		if got != want || (err == nil) != (wantErr == nil) {
			t.Errorf("DefaultDeps().Abs(%q) = %q, %v; filepath.Abs = %q, %v", path, got, err, want, wantErr)
		}
	}
}
