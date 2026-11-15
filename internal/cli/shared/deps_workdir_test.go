package shared

import (
	"os"
	"path/filepath"
	"testing"
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
