package shared

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDefaultDepsGetwdMatchesProcess(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	t.Chdir(first)
	d := DefaultDeps()

	for _, test := range []struct {
		name string
		dir  string
	}{
		{name: "directory when the deps were built", dir: first},
		{name: "directory changed after the deps were built", dir: second},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(test.dir)
			want, wantErr := os.Getwd()
			got, err := d.Getwd()
			if err != nil || wantErr != nil {
				t.Fatalf("d.Getwd() error = %v, os.Getwd() error = %v", err, wantErr)
			}
			if got != want {
				t.Fatalf("d.Getwd() = %q, os.Getwd() = %q", got, want)
			}
			if resolved, _ := filepath.EvalSymlinks(got); resolved != mustEvalSymlinks(t, test.dir) {
				t.Fatalf("d.Getwd() = %q, want %q", got, test.dir)
			}
		})
	}
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return resolved
}

func TestDepsAbsMatchesFilepathAbs(t *testing.T) {
	processDir, depsDir := t.TempDir(), t.TempDir()
	errGetwd := errors.New("getwd failed")
	for _, path := range []string{"", ".", "report.tsv", "./sub/report.tsv", "sub/../report.tsv", "sub/", "..", "../other/report.tsv", filepath.Join(depsDir, "sub", "..", "report.tsv")} {
		t.Run(path, func(t *testing.T) {
			t.Chdir(depsDir)
			wantInDepsDir, wantErr := filepath.Abs(path)
			if wantErr != nil {
				t.Fatalf("filepath.Abs(%q) error: %v", path, wantErr)
			}
			t.Chdir(processDir)
			wantInProcessDir, _ := filepath.Abs(path)

			got, err := DefaultDeps().Abs(path)
			if err != nil || got != wantInProcessDir {
				t.Fatalf("DefaultDeps().Abs(%q) = %q, %v; want %q", path, got, err, wantInProcessDir)
			}
			if got := DefaultDeps().OSPath(path); got != path {
				t.Fatalf("DefaultDeps().OSPath(%q) = %q, want the path as given", path, got)
			}
			injected := &Deps{Getwd: func() (string, error) { return depsDir, nil }}
			got, err = injected.Abs(path)
			if err != nil || got != wantInDepsDir {
				t.Fatalf("Abs(%q) with Getwd %q = %q, %v; want %q (filepath.Abs in that directory)", path, depsDir, got, err, wantInDepsDir)
			}
			wantOSPath := wantInDepsDir
			if wantInDepsDir == wantInProcessDir {
				wantOSPath = path
			}
			if got := injected.OSPath(path); got != wantOSPath {
				t.Fatalf("OSPath(%q) with Getwd %q = %q, want %q", path, depsDir, got, wantOSPath)
			}
			failing := &Deps{Getwd: func() (string, error) { return "", errGetwd }}
			got, err = failing.Abs(path)
			if filepath.IsAbs(path) {
				if err != nil || got != wantInDepsDir {
					t.Fatalf("Abs(%q) with a failing Getwd = %q, %v; want %q", path, got, err, wantInDepsDir)
				}
			} else if !errors.Is(err, errGetwd) || got != "" {
				t.Fatalf("Abs(%q) with a failing Getwd = %q, %v; want Getwd's error", path, got, err)
			}
		})
	}
}

func TestDepsReportWritersMatchProcessVariants(t *testing.T) {
	gzipped := func(t *testing.T, content string) []byte {
		var buffer bytes.Buffer
		writer := gzip.NewWriter(&buffer)
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("gzip: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("gzip: %v", err)
		}
		return buffer.Bytes()
	}
	reports := map[string]string{"process": "process report\n", "deps": "deps report\n"}

	for _, test := range []struct {
		name   string
		write  func(d *Deps, content []byte) (int64, error)
		output string
		source bool
		// artifact rows write one new file with a generated name into the
		// output directory on every call, whose JSON inputPath is the report.
		artifact bool
	}{
		{
			name: "WriteStreamToFile",
			write: func(d *Deps, content []byte) (int64, error) {
				if d == nil {
					return WriteStreamToFile("report.tsv.gz", bytes.NewReader(content))
				}
				return d.WriteStreamToFile("report.tsv.gz", bytes.NewReader(content))
			},
			output: "report.tsv.gz",
		},
		{
			name: "DecompressGzipFile",
			write: func(d *Deps, _ []byte) (int64, error) {
				if d == nil {
					return DefaultDeps().DecompressGzipFile("report.tsv.gz", "report.tsv")
				}
				return d.DecompressGzipFile("report.tsv.gz", "report.tsv")
			},
			output: "report.tsv",
			source: true,
		},
		{
			name: "FinalizeLocalizationUploadResult",
			write: func(d *Deps, content []byte) (int64, error) {
				reader, err := gzip.NewReader(bytes.NewReader(content))
				if err != nil {
					return 0, err
				}
				input, err := io.ReadAll(reader)
				if err != nil {
					return 0, err
				}
				result := &asc.LocalizationUploadResult{
					InputPath: string(input),
					Results:   []asc.LocalizationUploadLocaleResult{{Locale: "en-US", Action: "update", Status: "failed", Error: "timeout"}},
				}
				if d == nil {
					FinalizeLocalizationUploadResult(result, "publish")
				} else {
					d.FinalizeLocalizationUploadResult(result, "publish")
				}
				if result.FailureArtifactError != "" {
					return 0, errors.New(result.FailureArtifactError)
				}
				if !strings.HasPrefix(result.FailureArtifactPath, filepath.Join(".asc", "reports", "localizations-upload")+string(filepath.Separator)) {
					return 0, errors.New("artifact path " + result.FailureArtifactPath + " is not the relative report path")
				}
				return 0, nil
			},
			output:   filepath.Join(".asc", "reports", "localizations-upload"),
			artifact: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The deps directory is reached through a symlink, so only a write
			// anchored at d.Getwd() itself is accepted there.
			depsDir := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink(t.TempDir(), depsDir); err != nil {
				t.Skipf("symlinks are unavailable: %v", err)
			}
			dirs := map[string]string{"process": t.TempDir(), "deps": depsDir}
			want := map[string]string{}
			for name, dir := range dirs {
				want[name] = string(gzipped(t, reports[name]))
				if test.source {
					if err := os.WriteFile(filepath.Join(dir, "report.tsv.gz"), []byte(want[name]), 0o600); err != nil {
						t.Fatalf("write source: %v", err)
					}
					want[name] = reports[name]
				}
			}
			t.Chdir(dirs["process"])
			d := &Deps{Getwd: func() (string, error) { return dirs["deps"], nil }, Now: time.Now}
			run := map[string]func() (int64, error){
				"process": func() (int64, error) { return test.write(nil, gzipped(t, reports["process"])) },
				"deps":    func() (int64, error) { return test.write(d, gzipped(t, reports["deps"])) },
			}

			if test.artifact {
				for _, name := range []string{"process", "deps"} {
					if _, err := run[name](); err != nil {
						t.Fatalf("%s form error = %v", name, err)
					}
				}
				for name, dir := range dirs {
					entries, err := os.ReadDir(filepath.Join(dir, test.output))
					if err != nil || len(entries) != 1 {
						t.Fatalf("%s in the %s directory = %v, %v; want one artifact", test.output, name, entries, err)
					}
					data, err := os.ReadFile(filepath.Join(dir, test.output, entries[0].Name()))
					var artifact localizationUploadFailureArtifact
					if err != nil || json.Unmarshal(data, &artifact) != nil || artifact.InputPath != reports[name] || artifact.Failed != 1 {
						t.Fatalf("artifact in the %s directory = %s, %v; want the %s form's failures", name, data, err, name)
					}
				}
				return
			}
			for _, name := range []string{"process", "deps"} {
				size, err := run[name]()
				if err != nil || size != int64(len(want[name])) {
					t.Fatalf("%s form = %d, %v; want %d, nil", name, size, err, len(want[name]))
				}
			}
			for name, dir := range dirs {
				data, err := os.ReadFile(filepath.Join(dir, test.output))
				if err != nil || string(data) != want[name] {
					t.Fatalf("%s in the %s directory = %q, %v; want %q", test.output, name, data, err, want[name])
				}
			}
			_, processErr := run["process"]()
			_, depsErr := run["deps"]()
			var processExists, depsExists *OutputFileExistsError
			if !errors.As(processErr, &processExists) || !errors.As(depsErr, &depsExists) || strings.ReplaceAll(depsErr.Error(), dirs["deps"], dirs["process"]) != processErr.Error() {
				t.Fatalf("second write: deps form error %v, process form error %v; want the same output-exists error", depsErr, processErr)
			}
		})
	}
}
