package routingcoverage

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const validRoutingCoverageGeoJSON = `{"type":"MultiPolygon","coordinates":[[[[77.5,12.9],[77.7,12.9],[77.7,13.1],[77.5,12.9]]]]}`

func TestPrepareRoutingCoverageFileValidatesGeoJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "malformed JSON", content: `{"type":`, want: "invalid JSON"},
		{name: "wrong geometry type", content: `{"type":"Polygon","coordinates":[]}`, want: "MultiPolygon"},
		{name: "no polygons", content: `{"type":"MultiPolygon","coordinates":[]}`, want: "at least one Polygon"},
		{name: "short ring", content: `{"type":"MultiPolygon","coordinates":[[[[77.5,12.9],[77.7,12.9],[77.5,12.9]]]]}`, want: "at least four coordinate points"},
		{name: "open ring", content: `{"type":"MultiPolygon","coordinates":[[[[77.5,12.9],[77.7,12.9],[77.7,13.1],[77.6,12.8]]]]}`, want: "start and end coordinate points"},
		{name: "null coordinate component", content: `{"type":"MultiPolygon","coordinates":[[[[null,12.9],[77.7,12.9],[77.7,13.1],[null,12.9]]]]}`, want: "coordinate component 0 must be a number"},
		{name: "longitude above range", content: `{"type":"MultiPolygon","coordinates":[[[[181,12.9],[77.7,12.9],[77.7,13.1],[181,12.9]]]]}`, want: "longitude must be between -180 and 180"},
		{name: "longitude below range", content: `{"type":"MultiPolygon","coordinates":[[[[-181,12.9],[77.7,12.9],[77.7,13.1],[-181,12.9]]]]}`, want: "longitude must be between -180 and 180"},
		{name: "latitude above range", content: `{"type":"MultiPolygon","coordinates":[[[[77.5,91],[77.7,12.9],[77.7,13.1],[77.5,91]]]]}`, want: "latitude must be between -90 and 90"},
		{name: "latitude below range", content: `{"type":"MultiPolygon","coordinates":[[[[77.5,-91],[77.7,12.9],[77.7,13.1],[77.5,-91]]]]}`, want: "latitude must be between -90 and 90"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "coverage.geojson")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			t.Chdir(filepath.Dir(path))

			_, err := PrepareRoutingCoverageFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("PrepareRoutingCoverageFile() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestPrepareRoutingCoverageFileRejectsNonGeoJSONExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(path, []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := PrepareRoutingCoverageFile(path)
	if err == nil || !strings.Contains(err.Error(), ".geojson") {
		t.Fatalf("PrepareRoutingCoverageFile() error = %v, want .geojson extension error", err)
	}
}

func TestPrepareRoutingCoverageFileFingerprintsValidatedSnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "coverage.geojson")
	originalContent := validRoutingCoverageGeoJSON
	changedContent := strings.Replace(originalContent, "MultiPolygon", "InvalidShape", 1)
	if len(changedContent) != len(originalContent) {
		t.Fatalf("fixture sizes differ: changed=%d original=%d", len(changedContent), len(originalContent))
	}
	if err := os.WriteFile(path, []byte(originalContent), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Chdir(root)

	prepared, err := prepareRoutingCoverageFile(path, func(reader io.Reader, algorithm asc.ChecksumAlgorithm) (*asc.Checksum, error) {
		if err := os.WriteFile(path, []byte(changedContent), 0o600); err != nil {
			return nil, err
		}
		return asc.ComputeChecksumFromReader(reader, algorithm)
	})
	if err != nil {
		t.Fatalf("PrepareRoutingCoverageFile() error: %v", err)
	}
	expected, err := asc.ComputeChecksumFromReader(strings.NewReader(originalContent), asc.ChecksumAlgorithmMD5)
	if err != nil {
		t.Fatalf("compute expected checksum: %v", err)
	}
	if prepared.Checksum != expected.Hash {
		t.Fatalf("prepared checksum = %q, want validated snapshot checksum %q", prepared.Checksum, expected.Hash)
	}
}

func TestPrepareRoutingCoverageFileAcceptsOutOfWorkingDirectoryRegularFiles(t *testing.T) {
	base := t.TempDir()
	workingDir := filepath.Join(base, "work")
	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(workingDir, 0o700); err != nil {
		t.Fatalf("create working directory: %v", err)
	}
	if err := os.MkdirAll(inputDir, 0o700); err != nil {
		t.Fatalf("create input directory: %v", err)
	}
	coveragePath := filepath.Join(inputDir, "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Chdir(workingDir)

	for _, path := range []string{
		coveragePath,
		filepath.Join("..", "inputs", "coverage.geojson"),
	} {
		prepared, err := PrepareRoutingCoverageFile(path)
		if err != nil {
			t.Fatalf("PrepareRoutingCoverageFile(%q) error: %v", path, err)
		}
		if prepared.Path != coveragePath {
			t.Fatalf("PrepareRoutingCoverageFile(%q) path = %q, want %q", path, prepared.Path, coveragePath)
		}
	}
}

func TestPrepareRoutingCoverageFileAcceptsAbsoluteFileThroughHostTempAlias(t *testing.T) {
	tempAlias := filepath.Join(string(filepath.Separator), "tmp")
	aliasInfo, err := os.Lstat(tempAlias)
	if err != nil {
		t.Skipf("inspect host temp alias: %v", err)
	}
	if aliasInfo.Mode()&os.ModeSymlink == 0 {
		t.Skipf("host temp path %q is not a symlink alias", tempAlias)
	}

	inputDir, err := os.MkdirTemp(tempAlias, "asc-routing-coverage-")
	if err != nil {
		t.Fatalf("create input directory through host temp alias: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(inputDir) })
	coveragePath := filepath.Join(inputDir, "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	workingDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve unrelated working directory: %v", err)
	}
	t.Chdir(workingDir)

	prepared, err := PrepareRoutingCoverageFile(coveragePath)
	if err != nil {
		t.Fatalf("PrepareRoutingCoverageFile(%q) error: %v", coveragePath, err)
	}
	if prepared.Path != coveragePath {
		t.Fatalf("PrepareRoutingCoverageFile(%q) path = %q, want %q", coveragePath, prepared.Path, coveragePath)
	}
	if err := RevalidatePreparedRoutingCoverageFile(prepared); err != nil {
		t.Fatalf("RevalidatePreparedRoutingCoverageFile() error: %v", err)
	}

	finalLink := filepath.Join(inputDir, "final-link.geojson")
	if err := os.Symlink(coveragePath, finalLink); err != nil {
		t.Fatalf("create final symlink: %v", err)
	}
	parentTarget := filepath.Join(inputDir, "parent-target")
	if err := os.Mkdir(parentTarget, 0o700); err != nil {
		t.Fatalf("create parent target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parentTarget, "coverage.geojson"), []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write parent target fixture: %v", err)
	}
	parentLink := filepath.Join(inputDir, "parent-link")
	if err := os.Symlink(parentTarget, parentLink); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}

	for _, path := range []string{finalLink, filepath.Join(parentLink, "coverage.geojson")} {
		if _, err := PrepareRoutingCoverageFile(path); !errors.Is(err, rootfs.ErrSymlink) {
			t.Fatalf("PrepareRoutingCoverageFile(%q) error = %v, want rootfs.ErrSymlink", path, err)
		}
	}
}

func TestPrepareRoutingCoverageFileRejectsOutOfWorkingDirectorySymlinks(t *testing.T) {
	base := t.TempDir()
	workingDir := filepath.Join(base, "work")
	inputDir := filepath.Join(base, "inputs")
	outsideDir := filepath.Join(base, "outside")
	for _, dir := range []string{workingDir, inputDir, outsideDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create directory %q: %v", dir, err)
		}
	}
	targetPath := filepath.Join(outsideDir, "coverage.geojson")
	if err := os.WriteFile(targetPath, []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	finalLink := filepath.Join(inputDir, "coverage.geojson")
	if err := os.Symlink(targetPath, finalLink); err != nil {
		t.Skipf("create final symlink: %v", err)
	}
	parentLink := filepath.Join(base, "linked-inputs")
	if err := os.Symlink(outsideDir, parentLink); err != nil {
		t.Skipf("create parent symlink: %v", err)
	}
	t.Chdir(workingDir)

	tests := []struct {
		name string
		path string
	}{
		{name: "absolute final symlink", path: finalLink},
		{name: "parent-relative symlinked parent", path: filepath.Join("..", "linked-inputs", "coverage.geojson")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PrepareRoutingCoverageFile(tt.path)
			if !errors.Is(err, rootfs.ErrSymlink) {
				t.Fatalf("PrepareRoutingCoverageFile(%q) error = %v, want rootfs.ErrSymlink", tt.path, err)
			}
		})
	}
}

func TestPrepareRoutingCoverageFileRejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "coverage.geojson"), []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}
	t.Chdir(root)

	_, err := PrepareRoutingCoverageFile(filepath.Join("linked", "coverage.geojson"))
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("PrepareRoutingCoverageFile() error = %v, want rootfs.ErrSymlink", err)
	}
}

func TestPrepareRoutingCoverageFileRejectsAbsoluteSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "coverage.geojson"), []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}
	t.Chdir(root)

	_, err := PrepareRoutingCoverageFile(filepath.Join(root, "linked", "coverage.geojson"))
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("PrepareRoutingCoverageFile() error = %v, want rootfs.ErrSymlink", err)
	}
}

func TestPreparedRoutingCoverageFileRechecksRootedParent(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "coverage.geojson"), []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Chdir(root)
	prepared, err := PrepareRoutingCoverageFile(filepath.Join("source", "coverage.geojson"))
	if err != nil {
		t.Fatalf("PrepareRoutingCoverageFile() error: %v", err)
	}

	if err := os.Rename(sourceDir, filepath.Join(root, "original")); err != nil {
		t.Fatalf("move source directory: %v", err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "coverage.geojson"), []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write replacement fixture: %v", err)
	}
	if err := os.Symlink(outside, sourceDir); err != nil {
		t.Fatalf("replace source with symlink: %v", err)
	}

	file, err := prepared.openSource()
	if file != nil {
		_ = file.Close()
	}
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("openSource() error = %v, want rootfs.ErrSymlink", err)
	}
}

func TestVerifyPreparedRoutingCoverageSourceRejectsAppendDuringChecksum(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "coverage.geojson")
	if err := os.WriteFile(path, []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Chdir(root)
	prepared, err := PrepareRoutingCoverageFile(path)
	if err != nil {
		t.Fatalf("PrepareRoutingCoverageFile() error: %v", err)
	}
	source, err := prepared.openSource()
	if err != nil {
		t.Fatalf("openSource() error: %v", err)
	}
	defer source.Close()

	err = verifyPreparedRoutingCoverageSourceWithChecksum(source, prepared, func(file *os.File, size int64) (*asc.Checksum, error) {
		checksum, checksumErr := checksumOpenedFile(file, size)
		if checksumErr != nil {
			return nil, checksumErr
		}
		appender, appendErr := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if appendErr != nil {
			return nil, appendErr
		}
		if _, appendErr = appender.WriteString("\n"); appendErr != nil {
			_ = appender.Close()
			return nil, appendErr
		}
		if appendErr = appender.Close(); appendErr != nil {
			return nil, appendErr
		}
		return checksum, nil
	})
	if err == nil || !strings.Contains(err.Error(), "file changed after validation") {
		t.Fatalf("verifyPreparedRoutingCoverageSourceWithChecksum() error = %v, want changed-file diagnostic", err)
	}
}

func TestRoutingCoverageCommandShape(t *testing.T) {
	cmd := RoutingCoverageCommand()
	if cmd == nil {
		t.Fatal("expected routing-coverage command")
		return
	}
	if cmd.Name != "routing-coverage" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 4 {
		t.Fatalf("expected 4 subcommands, got %d", len(cmd.Subcommands))
	}
}

func TestRoutingCoverageGetCommand_MissingVersionID(t *testing.T) {
	cmd := RoutingCoverageGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestRoutingCoverageInfoCommand_MissingID(t *testing.T) {
	cmd := RoutingCoverageInfoCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestRoutingCoverageCreateCommand_MissingRequiredFlags(t *testing.T) {
	t.Run("missing version-id", func(t *testing.T) {
		cmd := RoutingCoverageCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--file", "coverage.geojson"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		cmd := RoutingCoverageCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--version-id", "VERSION_ID"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestRoutingCoverageDeleteCommandValidation(t *testing.T) {
	t.Run("missing id", func(t *testing.T) {
		cmd := RoutingCoverageDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("missing confirm", func(t *testing.T) {
		cmd := RoutingCoverageDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "COVERAGE_ID"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestCommandWrapper(t *testing.T) {
	if got := RoutingCoverageCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}
