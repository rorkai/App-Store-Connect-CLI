package shared

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ResolveVendorNumber resolves the vendor number for reports.
func ResolveVendorNumber(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	vendorEnv, vendorSet := os.LookupEnv("ASC_VENDOR_NUMBER")
	analyticsEnv, analyticsSet := os.LookupEnv("ASC_ANALYTICS_VENDOR_NUMBER")
	if vendorSet || analyticsSet {
		if env := strings.TrimSpace(vendorEnv); env != "" {
			return env
		}
		if env := strings.TrimSpace(analyticsEnv); env != "" {
			return env
		}
		return ""
	}
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	if value := strings.TrimSpace(cfg.VendorNumber); value != "" {
		return value
	}
	return strings.TrimSpace(cfg.AnalyticsVendorNumber)
}

// ResolveReportOutputPaths returns compressed/decompressed paths for reports.
func ResolveReportOutputPaths(outputPath, defaultCompressed, decompressedExt string, decompress bool) (string, string) {
	compressed := strings.TrimSpace(outputPath)
	if compressed == "" {
		compressed = defaultCompressed
	}
	if !decompress {
		return compressed, ""
	}
	if before, ok := strings.CutSuffix(compressed, ".gz"); ok {
		return compressed, before
	}
	if strings.HasSuffix(compressed, decompressedExt) {
		return compressed + ".gz", compressed
	}
	return compressed, compressed + decompressedExt
}

// WriteStreamToFile writes a reader to a file securely.
func WriteStreamToFile(path string, reader io.Reader) (int64, error) {
	return createNewOutputFile(path, reader)
}

// DecompressGzipFile inflates a gzip file to the destination path.
func DecompressGzipFile(sourcePath, destPath string) (int64, error) {
	in, err := rootfs.OpenFile(sourcePath)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	reader, err := gzip.NewReader(in)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	return createNewOutputFile(destPath, reader)
}

// createNewOutputFile writes a complete stream below the filesystem root. The
// rooted traversal rejects symlinks in every parent component, and atomic
// no-replace publication keeps a failed stream from leaving a partial output.
func createNewOutputFile(path string, reader io.Reader) (int64, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("resolve output path %q: %w", path, err)
	}
	rootPath := filepath.VolumeName(absolute) + string(filepath.Separator)
	workingDir, _ := os.Getwd()
	for _, candidate := range []string{workingDir, os.TempDir()} {
		candidate, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		candidate = filepath.Clean(candidate)
		if !pathWithinOutputRoot(candidate, absolute) {
			continue
		}
		if len(candidate) > len(rootPath) {
			rootPath = candidate
		}
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return 0, err
	}
	defer root.Close()

	relative, err := filepath.Rel(root.Path(), absolute)
	if err != nil {
		return 0, fmt.Errorf("resolve output path %q: %w", path, err)
	}
	written, err := root.CreateNewFrom(relative, reader, 0o600)
	if errors.Is(err, os.ErrExist) {
		return 0, &OutputFileExistsError{err: err}
	}
	return written, err
}

// OutputFileExistsError reports that a command refused to replace an existing
// output file. Choosing another path or removing the file is the fix, so it is
// a validation outcome rather than an asc defect.
type OutputFileExistsError struct {
	err error
}

func (e *OutputFileExistsError) Error() string { return "output file already exists: " + e.err.Error() }

func (e *OutputFileExistsError) Unwrap() error { return e.err }

func (e *OutputFileExistsError) ValidationFailure() bool { return true }

func (e *OutputFileExistsError) Diagnostic() Diagnostic {
	return Diagnostic{Code: DiagnosticResourceConflict}
}

func pathWithinOutputRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
