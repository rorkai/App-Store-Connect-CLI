package secureopen

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenExistingRegularFileNoFollow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	const contents = "artifact"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	file, info, err := OpenExistingRegularFileNoFollow(path, "artifact", "--artifact")
	if err != nil {
		t.Fatalf("OpenExistingRegularFileNoFollow() error: %v", err)
	}
	defer file.Close()
	if info.Size() != int64(len(contents)) {
		t.Fatalf("returned size = %d, want %d", info.Size(), len(contents))
	}
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != contents {
		t.Fatalf("returned file contents = %q, want %q", data, contents)
	}
}

func TestOpenExistingRegularFileNoFollowRejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, path string)
		wantErr string
	}{
		{
			name: "empty",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatalf("WriteFile() error: %v", err)
				}
			},
			wantErr: "--artifact must not be empty",
		},
		{
			name: "directory",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("Mkdir() error: %v", err)
				}
			},
			wantErr: "--artifact must be a file",
		},
		{
			name: "symlink",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(path), "target")
				if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
					t.Fatalf("WriteFile() error: %v", err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("Symlink() unavailable: %v", err)
				}
			},
			wantErr: "refusing to read symlink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact")
			tt.prepare(t, path)

			file, _, err := OpenExistingRegularFileNoFollow(path, "artifact", "--artifact")
			if file != nil {
				file.Close()
			}
			if err == nil {
				t.Fatal("expected unsafe path error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
