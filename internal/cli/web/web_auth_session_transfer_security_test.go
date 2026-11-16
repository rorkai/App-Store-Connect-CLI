package web

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteWebSessionBundleUsesPrivateFilePolicy(t *testing.T) {
	d, _, _ := newWebTestDeps()
	path := filepath.Join(t.TempDir(), "session.json")
	const payload = "{\"kind\":\"asc-web-session\"}\n"

	overwritten, err := writeWebSessionBundle(d, path, []byte(payload), false)
	if err != nil {
		t.Fatalf("writeWebSessionBundle() error = %v", err)
	}
	if overwritten {
		t.Fatal("writeWebSessionBundle() reported an overwrite for a new file")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != payload {
		t.Fatalf("bundle contents = %q, want %q", got, payload)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("bundle permissions = %#o, want owner-only", info.Mode().Perm())
		}
	}
}
