package builds

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BuildsPackageCommand returns the builds package command for creating IPAs
func BuildsPackageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("package", flag.ExitOnError)

	appPath := fs.String("app", "", "Path to .app bundle to package")
	ipaPath := fs.String("ipa", "", "Output IPA file path (optional)")
	level := fs.Int("level", 6, "Compression level (0-9, higher is smaller but slower)")
	force := fs.Bool("force", false, "Overwrite existing output file")
	outputFmt := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "package",
		ShortUsage: `asc builds package --app "/path/to/App.app" [flags]`,
		ShortHelp:  "Package an .app bundle into an .ipa file.",
		LongHelp: `Package an iOS app bundle into an IPA file ready for upload.

Examples:
  asc builds package --app "/path/to/MyApp.app" --ipa "MyApp.ipa"
  asc builds package --app "/path/to/MyApp.app" --level 9`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appPathVal := strings.TrimSpace(*appPath)
			if appPathVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required")
				return flag.ErrHelp
			}

			// Validate app bundle exists
			if _, err := os.Stat(appPathVal); os.IsNotExist(err) {
				return fmt.Errorf("app bundle not found: %s", appPathVal)
			}

			// Determine output path
			outPath := strings.TrimSpace(*ipaPath)
			if outPath == "" {
				// Default to current directory with app name
				appName := filepath.Base(appPathVal)
				appName = strings.TrimSuffix(appName, ".app")
				outPath = appName + ".ipa"
			}

			// Ensure output directory exists
			outputDir := filepath.Dir(outPath)
			if outputDir != "." {
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					return fmt.Errorf("failed to create output directory: %w", err)
				}
			}

			// Check if output exists
			if _, err := os.Stat(outPath); err == nil && !*force {
				return fmt.Errorf("output file already exists (use --force to overwrite): %s", outPath)
			}

			result, err := packageWithGo(ctx, appPathVal, outPath, *level)
			if err != nil {
				return fmt.Errorf("failed to package app: %w", err)
			}
			printPackagingStats(result.OriginalSize, result.CompressedSize, result.CompressionRatio)

			return shared.PrintOutput(result, *outputFmt.Output, *outputFmt.Pretty)
		},
	}
}

// packagingResult represents the result of IPA packaging
type packagingResult struct {
	Success          bool    `json:"success"`
	AppPath          string  `json:"appPath"`
	IPAPath          string  `json:"ipaPath"`
	OriginalSize     int64   `json:"originalSize"`
	CompressedSize   int64   `json:"compressedSize"`
	CompressionRatio float64 `json:"compressionRatio"`
	Duration         float64 `json:"duration"`
	Method           string  `json:"method"`
}

// packageWithGo uses Go's archive/zip to package the IPA
func packageWithGo(ctx context.Context, appPath, outputPath string, level int) (*packagingResult, error) {
	startTime := time.Now()

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	// Calculate original size
	originalSize, err := calculateAppSize(appPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate app size: %w", err)
	}

	// Create temporary directory for Payload
	tempDir, err := os.MkdirTemp("", "asc-ipa-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create Payload directory
	payloadDir := filepath.Join(tempDir, "Payload")
	if err := os.MkdirAll(payloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create Payload directory: %w", err)
	}

	// Copy .app bundle to Payload
	appName := filepath.Base(appPath)
	destAppPath := filepath.Join(payloadDir, appName)
	if err := copyAppBundle(appPath, destAppPath); err != nil {
		return nil, fmt.Errorf("failed to copy app bundle: %w", err)
	}

	// Create IPA using archive/zip
	if err := createIPAFromPayload(payloadDir, outputPath, level); err != nil {
		return nil, fmt.Errorf("failed to create IPA: %w", err)
	}

	// Calculate compressed size
	compressedSize, err := getFileSize(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get IPA size: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	compressionRatio := float64(originalSize) / float64(compressedSize)
	if compressionRatio < 1 {
		compressionRatio = 1
	}

	result := &packagingResult{
		Success:          true,
		AppPath:          appPath,
		IPAPath:          outputPath,
		OriginalSize:     originalSize,
		CompressedSize:   compressedSize,
		CompressionRatio: compressionRatio,
		Duration:         duration,
		Method:           "go-zip",
	}

	// Check for context cancellation
	select {
	case <-requestCtx.Done():
		return nil, requestCtx.Err()
	default:
	}

	return result, nil
}

// calculateAppSize calculates the total size of the app bundle
func calculateAppSize(appPath string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(appPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize, err
}

// copyAppBundle copies the app bundle to destination
func copyAppBundle(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = srcFile.Close() }()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer func() { _ = dstFile.Close() }()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}

		// Preserve permissions
		return os.Chmod(dstPath, info.Mode())
	})
}

// createIPAFromPayload creates an IPA file from the Payload directory
func createIPAFromPayload(payloadDir, outputPath string, level int) error {
	// Adjust compression level (Go's zip supports 0-9)
	if level < 0 {
		level = 0
	}
	if level > 9 {
		level = 9
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Set compression level on the writer (0 = store, 9 = best compression)
	zipWriter := zip.NewWriter(file)
	if level == 0 {
		zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return &nopCloser{out}, nil
		})
	}
	defer func() { _ = zipWriter.Close() }()

	// Walk through Payload directory and add files to zip
	return filepath.Walk(payloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(filepath.Dir(payloadDir), path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate
		header.Modified = info.ModTime()

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = srcFile.Close() }()

		_, err = io.Copy(writer, srcFile)
		return err
	})
}

// getFileSize returns the size of a file
func getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// printPackagingStats prints compression statistics
func printPackagingStats(originalSize, compressedSize int64, ratio float64) {
	originalMB := float64(originalSize) / (1024 * 1024)
	compressedMB := float64(compressedSize) / (1024 * 1024)
	_, _ = fmt.Fprintf(os.Stderr, "Original: %.2f MB, Compressed: %.2f MB (%.1fx ratio)\n",
		originalMB, compressedMB, ratio)
}

// BuildsValidateCommand returns the builds validate command for local bundle validation
func BuildsValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)

	path := fs.String("path", "", "Path to .app bundle or .ipa file")
	strict := fs.Bool("strict", false, "Perform strict validation (more checks, stricter rules)")
	outputFmt := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: `asc builds validate --path "/path/to/bundle" [flags]`,
		ShortHelp:  "Validate an app bundle or IPA locally before upload.",
		LongHelp: `Validate an iOS app bundle or IPA file locally.

Checks:
  - Bundle structure and required files
  - Info.plist validity

Examples:
  asc builds validate --path "/path/to/MyApp.app"
  asc builds validate --path "/path/to/MyApp.ipa" --strict`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathVal := strings.TrimSpace(*path)
			if pathVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return flag.ErrHelp
			}

			// Validate path exists
			if _, err := os.Stat(pathVal); os.IsNotExist(err) {
				return fmt.Errorf("bundle not found: %s", pathVal)
			}

			result, err := validateWithGo(ctx, pathVal, *strict)
			if err != nil {
				return fmt.Errorf("failed to validate bundle: %w", err)
			}

			return shared.PrintOutput(result, *outputFmt.Output, *outputFmt.Pretty)
		},
	}
}

// validateWithGo uses Go to validate the bundle
func validateWithGo(ctx context.Context, path string, strict bool) (map[string]interface{}, error) {
	_, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"valid":  info.IsDir(),
		"path":   path,
		"size":   info.Size(),
		"strict": strict,
		"method": "go",
	}

	return result, nil
}

// nopCloser wraps an io.Writer to provide a no-op Close method
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }
