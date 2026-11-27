//go:build darwin || linux

package shared

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestOpenMessageFilePreservesFIFOWithInjectedDirectory(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "templates", "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "templates", "reply")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	writer, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	want := "Selected FIFO content."
	if _, err := io.WriteString(writer, want); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "reply"), []byte("Wrong regular file."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(project, "templates", "child"), filepath.Join(project, "link")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	d := &Deps{Getwd: func() (string, error) { return project, nil }, IO: &IO{ErrOut: &stderr}}
	file, err := d.OpenMessageFile("--file", "copy the file into the project directory", "link/../reply", true)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	body := make([]byte, len(want))
	if _, err := io.ReadFull(file, body); err != nil || string(body) != want {
		t.Fatalf("body=%q error=%v; want %q", body, err, want)
	}
	if !strings.Contains(stderr.String(), "Warning: --file non-regular or descriptor input is deprecated") {
		t.Fatalf("missing stream deprecation warning: %q", stderr.String())
	}
}
