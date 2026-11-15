package xcodecloud

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var errArtifactRead = errors.New("artifact read failed")

type failingArtifactReader struct {
	delivered bool
}

func (r *failingArtifactReader) Read(p []byte) (int, error) {
	if r.delivered {
		return 0, errArtifactRead
	}
	r.delivered = true
	return copy(p, "replacement"), nil
}

func TestWriteArtifactFileOverwritePreservesExistingFileOnReadFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original artifact: %v", err)
	}

	_, err := writeArtifactFile(target, &failingArtifactReader{}, true)
	if !errors.Is(err, errArtifactRead) {
		t.Fatalf("writeArtifactFile error = %v, want %v", err, errArtifactRead)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read preserved artifact: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("preserved artifact = %q, want %q", got, "original")
	}
}

func TestWriteArtifactFileOverwriteReplacesCompletedFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatalf("write original artifact: %v", err)
	}

	written, err := writeArtifactFile(target, strings.NewReader("replacement"), true)
	if err != nil {
		t.Fatalf("writeArtifactFile error: %v", err)
	}
	if written != int64(len("replacement")) {
		t.Fatalf("bytes written = %d, want %d", written, len("replacement"))
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read replacement artifact: %v", err)
	}
	if string(got) != "replacement" {
		t.Fatalf("replacement artifact = %q, want %q", got, "replacement")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat replacement artifact: %v", err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o600 {
		t.Fatalf("replacement mode = %o, want 600", gotMode)
	}
}

func TestArtifactsDownloadExistingDestinationFailsBeforeClientCreation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(target, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing artifact: %v", err)
	}

	clientCalls := 0
	d := &shared.Deps{
		Client: func() (*asc.Client, error) {
			clientCalls++
			return nil, errors.New("client factory should not be called")
		},
		IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
	}

	err := newXcodeCloudArtifactsDownloadCommand(d).ParseAndRun(context.Background(), []string{
		"--id", "artifact-1",
		"--path", target,
	})
	if err == nil || !strings.Contains(err.Error(), "output file already exists") {
		t.Fatalf("command error = %v, want existing destination error", err)
	}
	if clientCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientCalls)
	}
}

func TestArtifactsDownloadTrailingSeparatorFailsBeforeClientOrFilesystemSideEffects(t *testing.T) {
	separators := []string{string(os.PathSeparator)}
	if os.PathSeparator != '/' {
		separators = append(separators, "/")
	}

	for _, separator := range separators {
		t.Run(strconv.Quote(separator), func(t *testing.T) {
			parent := t.TempDir()
			createdPath := filepath.Join(parent, "result")
			target := createdPath + separator
			clientCalls := 0
			d := &shared.Deps{
				Client: func() (*asc.Client, error) {
					clientCalls++
					return nil, errors.New("client factory should not be called")
				},
				IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
			}

			err := newXcodeCloudArtifactsDownloadCommand(d).ParseAndRun(context.Background(), []string{
				"--id", "artifact-1",
				"--path", target,
			})
			if err == nil || !strings.Contains(err.Error(), strconv.Quote(target)) {
				t.Fatalf("command error = %v, want exact destination %s", err, strconv.Quote(target))
			}
			if clientCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientCalls)
			}
			if _, statErr := os.Lstat(createdPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("destination-shaped directory was created, stat error = %v", statErr)
			}
		})
	}
}
