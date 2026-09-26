package asc

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxAssetFileSize = int64(1024 * 1024 * 1024) // 1GB safety guardrail

// UploadAsset uploads a file using the provided upload operations.
func UploadAsset(ctx context.Context, filePath string, operations []UploadOperation) error {
	file, err := openUploadSourceFile(filePath)
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
// Operations run sequentially through the shared upload executor, so a part
// that hits a transient transport failure or retryable status is retried
// instead of leaving the asset partially uploaded.
func UploadAssetFromFile(ctx context.Context, file *os.File, fileSize int64, operations []UploadOperation) error {
	if len(operations) == 0 {
		return fmt.Errorf("no upload operations provided")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	uploadOpts := UploadOptions{
		Client:    clientWithoutRedirects(newUploadClient()),
		RetryOpts: ResolveRetryOptions(),
	}

	for i, op := range operations {
		if strings.TrimSpace(op.Method) == "" {
			return fmt.Errorf("upload operation %d missing method", i)
		}
		if strings.TrimSpace(op.URL) == "" {
			return fmt.Errorf("upload operation %d missing url", i)
		}
		if op.Offset < 0 || op.Length < 0 {
			return fmt.Errorf("upload operation %d has negative offset/length", i)
		}
		if op.Length <= 0 {
			return fmt.Errorf("upload operation %d has non-positive length", i)
		}
		if op.Offset+op.Length > fileSize {
			return fmt.Errorf("upload operation %d exceeds file size", i)
		}
	}

	for i, op := range operations {
		if err := executeUploadOperation(ctx, file, uploadTask{index: i, op: op}, uploadOpts); err != nil {
			return err
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
	return ValidateAssetFileInfo(path, info)
}

// ValidateAssetFileInfo checks the metadata of an already opened asset file.
func ValidateAssetFileInfo(path string, info os.FileInfo) error {
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

// imageFormatExtensions maps the image formats this binary can decode to the
// file extensions that describe them.
var imageFormatExtensions = map[string][]string{
	"png":  {".png"},
	"jpeg": {".jpg", ".jpeg"},
	"gif":  {".gif"},
}

// renameableImageFormats are the formats an operator can fix by renaming the
// file. Asset collection only picks up .png, .jpg, and .jpeg, so suggesting a
// .gif rename would quietly drop the file from an upload directory instead of
// fixing it.
var renameableImageFormats = map[string]bool{
	"png":  true,
	"jpeg": true,
}

// ReadImageDimensions validates and decodes image dimensions from disk.
func ReadImageDimensions(path string) (ImageDimensions, error) {
	dimensions, _, err := ReadImageDimensionsAndFormat(path)
	return dimensions, err
}

// ReadImageDimensionsAndFormat decodes the dimensions together with the
// encoded image format, which is the only reliable description of what a file
// actually contains. Callers that report on a file rather than just sizing it
// need both from a single decode.
func ReadImageDimensionsAndFormat(path string) (ImageDimensions, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return ImageDimensions{}, "", err
	}
	if err := ValidateAssetFileInfo(path, info); err != nil {
		return ImageDimensions{}, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return ImageDimensions{}, "", err
	}
	defer file.Close()

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return ImageDimensions{}, "", fmt.Errorf("decode image dimensions for %q: %w", path, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ImageDimensions{}, "", fmt.Errorf("invalid image dimensions %dx%d for %q", cfg.Width, cfg.Height, path)
	}
	return ImageDimensions{Width: cfg.Width, Height: cfg.Height}, format, nil
}

// ValidateImageFormatMatchesExtension rejects a file whose decoded image
// format contradicts its extension. App Store Connect derives the asset
// content type from the file name, so a JPEG named .png is only rejected
// server-side, after the upload has already been paid for.
//
// Extensions that name no image format are left alone: the check exists to
// catch contradictions, not to police naming. A rename is only suggested when
// the decoded format has an extension asset collection accepts; other
// decodable formats are told to re-export instead.
func ValidateImageFormatMatchesExtension(path, format string) error {
	decoded := strings.ToLower(strings.TrimSpace(format))
	if decoded == "" {
		return nil
	}
	extension := strings.ToLower(filepath.Ext(path))
	expected, known := imageFormatForExtension(extension)
	if !known || expected == decoded {
		return nil
	}

	message := fmt.Sprintf(
		"%q is %s data but has a %s extension",
		path,
		strings.ToUpper(decoded),
		extension,
	)
	if renamed, ok := SuggestedImageFileName(path, decoded); ok {
		message += fmt.Sprintf("; rename it to %s or re-export it as %s", renamed, strings.ToUpper(expected))
	} else {
		message += fmt.Sprintf("; re-export it as %s", strings.ToUpper(expected))
	}
	return errors.New(message)
}

// SuggestedImageFileName returns the name path should carry for its decoded
// format. It reports false when renaming cannot fix the file, because asset
// collection does not pick up that format's extension.
func SuggestedImageFileName(path, format string) (string, bool) {
	decoded := strings.ToLower(strings.TrimSpace(format))
	if !renameableImageFormats[decoded] {
		return "", false
	}
	extensions := imageFormatExtensions[decoded]
	if len(extensions) == 0 {
		return "", false
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + extensions[0], true
}

// ReadImageFormatFrom decodes the encoded image format from an already-opened
// source, for callers that must open the file through a trusted root instead
// of by path.
func ReadImageFormatFrom(reader io.Reader) (string, error) {
	_, format, err := image.DecodeConfig(reader)
	if err != nil {
		return "", fmt.Errorf("decode image format: %w", err)
	}
	return format, nil
}

func imageFormatForExtension(extension string) (string, bool) {
	for format, extensions := range imageFormatExtensions {
		for _, candidate := range extensions {
			if candidate == extension {
				return format, true
			}
		}
	}
	return "", false
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
