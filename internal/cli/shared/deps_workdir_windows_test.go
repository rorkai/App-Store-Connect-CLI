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

func TestDepsAbsResolvesAgainstInjectedDirectoryOnWindows(t *testing.T) {
	d := &Deps{Getwd: func() (string, error) { return `C:\repo`, nil }}
	for path, want := range map[string]string{
		``:                 `C:\repo`,
		`out\report.tsv`:   `C:\repo\out\report.tsv`,
		`..\report.tsv`:    `C:\report.tsv`,
		`\reports\out`:     `C:\reports\out`,
		`C:out\report.tsv`: `C:\repo\out\report.tsv`,
		`c:out`:            `C:\repo\out`,
		`D:\reports\out`:   `D:\reports\out`,
		`\\server\share\x`: `\\server\share\x`,
	} {
		if got, err := d.Abs(path); err != nil || got != want {
			t.Errorf("Abs(%q) with Getwd C:\\repo = %q, %v; want %q", path, got, err, want)
		}
	}
}
