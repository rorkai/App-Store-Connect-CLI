package shared

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenMessageFilePreservesSymlinkParentTraversal(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.MkdirAll(filepath.Join(project, "templates", "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "templates", "reply.txt"), []byte("Selected reply."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "reply.txt"), []byte("Different reply."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(project, "templates", "child"), filepath.Join(project, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	var stderr bytes.Buffer
	d := &Deps{Getwd: os.Getwd, IO: &IO{ErrOut: &stderr}}
	path := filepath.FromSlash("link/../reply.txt")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := d.OpenMessageFile("--file", "copy the file into the project directory", path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil || !bytes.Equal(body, want) {
		t.Fatalf("body=%q error=%v; want the file selected by OS traversal", body, err)
	}
	warned := strings.Contains(stderr.String(), "Warning: --file outside the working directory or through a symlink is deprecated")
	if wantWarning := string(want) == "Selected reply."; warned != wantWarning {
		t.Fatalf("body=%q warning=%q; want warning=%t", want, stderr.String(), wantWarning)
	}
}

func TestOpenMessageFileWorkingDirectoryAliasDoesNotWarn(t *testing.T) {
	project := t.TempDir()
	alias := filepath.Join(t.TempDir(), "project")
	if err := os.Symlink(project, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "reply.txt"), []byte("Reply body."), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(alias)
	t.Setenv("PWD", alias)
	var stderr bytes.Buffer
	d := &Deps{Getwd: os.Getwd, IO: &IO{ErrOut: &stderr}}
	file, err := d.OpenMessageFile("--file", "copy the file into the project directory", "reply.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil || string(body) != "Reply body." || stderr.Len() != 0 {
		t.Fatalf("body=%q error=%v stderr=%q; want an ordinary in-directory read", body, err, stderr.String())
	}
}
