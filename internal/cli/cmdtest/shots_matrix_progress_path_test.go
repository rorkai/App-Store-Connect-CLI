package cmdtest

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShotsMatrixRejectsProgressArtifactAliasesBeforeExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
	for _, test := range []struct {
		name, target string
		alias        bool
	}{
		{name: "review manifest", target: "review/manifest.json"},
		{name: "review HTML", target: "review/index.html"},
		{name: "review lock", target: "review/.asc-matrix-review.lock"},
		{name: "raw screenshot", target: "raw/en-US/phone/light/default/home.png"},
		{name: "framed screenshot", target: "framed/en-US/phone/light/default/home.png"},
		{name: "physical parent alias", target: "raw/en-US/phone/light/default/home.png", alias: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir, binDir := t.TempDir(), t.TempDir()
			calls := filepath.Join(dir, "provider-calls")
			t.Setenv("MATRIX_ALIAS_CALLS", calls)
			t.Setenv("PATH", binDir)
			for _, tool := range []string{"xcrun", "axe", "kou"} {
				writeShotsMatrixExecutable(t, filepath.Join(binDir, tool), "#!/bin/sh\nprintf 'called\\n' >> \"$MATRIX_ALIAS_CALLS\"\nexit 7\n")
			}
			base := `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`
			matrix := `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`
			basePath, matrixPath := filepath.Join(dir, "base.json"), filepath.Join(dir, "matrix.json")
			writeFile(t, basePath, base)
			writeFile(t, matrixPath, matrix)
			progressPath := filepath.Join(dir, filepath.FromSlash(test.target))
			if err := os.MkdirAll(filepath.Dir(progressPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if test.alias {
				alias := filepath.Join(t.TempDir(), "selected")
				if err := os.Symlink(dir, alias); err != nil {
					t.Fatal(err)
				}
				progressPath = filepath.Join(alias, filepath.FromSlash(test.target))
			}
			stdout, stderr, code := runWithDeps(t, nil, "screenshots", "matrix", "--plan", matrixPath, "--progress-file", progressPath, "--output", "json")
			if code != 2 || stdout != "" || !strings.Contains(stderr, "--progress-file") || !strings.Contains(stderr, "matrix output") {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want progress/output conflict before execution", code, stdout, stderr)
			}
			for _, path := range []string{calls, progressPath} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("refused matrix touched %q: %v", path, err)
				}
			}
			for path, want := range map[string]string{basePath: base, matrixPath: matrix} {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Fatalf("input %q changed: %q, %v", path, data, err)
				}
			}
		})
	}
}
