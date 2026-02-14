package asc

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
)

const maxAssetFileSize = int64(1024 * 1024 * 1024) // 1GB safety guardrail

// UploadAsset uploads a file using the provided upload operations.
func UploadAsset(ctx context.Context, filePath string, operations []UploadOperation) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	return UploadAssetFromFile(ctx, file, info.Size(), operations)
}

// UploadAssetFromFile uploads a file using the provided upload operations.
func UploadAssetFromFile(ctx context.Context, file *os.File, fileSize int64, operations []UploadOperation) error {
	if len(operations) == 0 {
		return fmt.Errorf("no upload operations provided")
	}

	client := &http.Client{Timeout: ResolveUploadTimeout()}

	for i, op := range operations {
		method := strings.ToUpper(strings.TrimSpace(op.Method))
		if method == "" {
			return fmt.Errorf("upload operation %d missing method", i)
		}
		if strings.TrimSpace(op.URL) == "" {
			return fmt.Errorf("upload operation %d missing url", i)
		}
		if op.Offset < 0 || op.Length < 0 {
			return fmt.Errorf("upload operation %d has negative offset/length", i)
		}
		if op.Offset+op.Length > fileSize {
			return fmt.Errorf("upload operation %d exceeds file size", i)
		}

		reader := io.NewSectionReader(file, op.Offset, op.Length)
		req, err := http.NewRequestWithContext(ctx, method, op.URL, reader)
		if err != nil {
			return fmt.Errorf("upload operation %d: %w", i, err)
		}
		req.ContentLength = op.Length
		for _, header := range op.RequestHeaders {
			req.Header.Set(header.Name, header.Value)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("upload operation %d failed: %w", i, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("upload operation %d failed with status %d", i, resp.StatusCode)
		}
	}

	return nil
}

// ValidateAssetFile validates that a file exists and is safe to read.
func ValidateAssetFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to read symlink %q", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("expected regular file: %q", path)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("file is empty: %q", path)
	}
	if info.Size() > maxAssetFileSize {
		return fmt.Errorf("file size exceeds %d bytes: %q", maxAssetFileSize, path)
	}
	return nil
}

// ValidateImageFile validates that a file exists and is safe to read.
func ValidateImageFile(path string) error {
	return ValidateAssetFile(path)
}

// ImageDimensions represents decoded image dimensions.
type ImageDimensions struct {
	Width  int
	Height int
}

// ReadImageDimensions validates and decodes image dimensions from disk.
func ReadImageDimensions(path string) (ImageDimensions, error) {
	if err := ValidateImageFile(path); err != nil {
		return ImageDimensions{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return ImageDimensions{}, err
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return ImageDimensions{}, fmt.Errorf("decode image dimensions for %q: %w", path, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ImageDimensions{}, fmt.Errorf("invalid image dimensions %dx%d for %q", cfg.Width, cfg.Height, path)
	}
	return ImageDimensions{Width: cfg.Width, Height: cfg.Height}, nil
}

// ComputeChecksum computes a checksum for a file on disk.
func ComputeChecksum(path string, algorithm ChecksumAlgorithm) (*Checksum, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ComputeChecksumFromReader(file, algorithm)
}

// ComputeChecksumFromReader computes a checksum for an io.Reader.
func ComputeChecksumFromReader(reader io.Reader, algorithm ChecksumAlgorithm) (*Checksum, error) {
	var hasher hash.Hash
	switch algorithm {
	case ChecksumAlgorithmMD5:
		hasher = md5.New()
	case ChecksumAlgorithmSHA256:
		hasher = sha256.New()
	default:
		return nil, fmt.Errorf("unsupported checksum algorithm %q", algorithm)
	}

	if _, err := io.Copy(hasher, reader); err != nil {
		return nil, err
	}

	return &Checksum{
		Hash:      hex.EncodeToString(hasher.Sum(nil)),
		Algorithm: algorithm,
	}, nil
}
