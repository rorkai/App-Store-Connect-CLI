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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	file, err := OpenNewFileNoFollow(path, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, fmt.Errorf("output file already exists: %w", err)
		}
		return 0, err
	}
	defer func() { _ = file.Close() }()

	written, err := io.Copy(file, reader)
	if err != nil {
		return 0, err
	}
	return written, file.Sync()
}

// DecompressGzipFile inflates a gzip file to the destination path.
func DecompressGzipFile(sourcePath, destPath string) (int64, error) {
	in, err := OpenExistingNoFollow(sourcePath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = in.Close() }()

	reader, err := gzip.NewReader(in)
	if err != nil {
		return 0, err
	}
	defer func() { _ = reader.Close() }()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, err
	}
	out, err := OpenNewFileNoFollow(destPath, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, fmt.Errorf("output file already exists: %w", err)
		}
		return 0, err
	}
	defer func() { _ = out.Close() }()

	written, err := io.Copy(out, reader)
	if err != nil {
		return 0, err
	}
	return written, out.Sync()
}
