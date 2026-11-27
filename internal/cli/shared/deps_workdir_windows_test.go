//go:build windows

package shared

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	t.Run("message files use Windows path resolution", func(t *testing.T) {
		dir := t.TempDir()
		absolute := filepath.Join(dir, "reply.txt")
		if err := os.WriteFile(absolute, []byte("Reply body."), 0o600); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		d := &Deps{Getwd: func() (string, error) { return dir, nil }, IO: &IO{ErrOut: &stderr}}
		for _, path := range []string{filepath.VolumeName(dir) + "reply.txt", strings.TrimPrefix(absolute, filepath.VolumeName(absolute))} {
			for _, followSymlinks := range []bool{false, true} {
				file, err := d.OpenMessageFile("--body-file", "use --message", path, followSymlinks)
				if err != nil {
					t.Fatalf("open %q: %v", path, err)
				}
				body, readErr := io.ReadAll(file)
				closeErr := file.Close()
				if readErr != nil || closeErr != nil || string(body) != "Reply body." || stderr.Len() != 0 {
					t.Fatalf("path %q: body=%q read=%v close=%v stderr=%q", path, body, readErr, closeErr, stderr.String())
				}
			}
		}
	})
}
