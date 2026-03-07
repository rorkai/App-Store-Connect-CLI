// Package swifthelpers provides integration with Swift helper tools for
// performance-critical operations on macOS.
//
// The Swift helpers leverage native macOS frameworks for genuine speedups:
//   - Native CryptoKit JWT signing (hardware-accelerated P-256)
//   - Core Image/Metal screenshot framing and image optimization
//   - AVFoundation video encoding
//
// When Swift helpers are not available (Linux, Windows, or not installed),
// the package falls back to pure Go implementations.
package swifthelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Helper names
const (
	JWTSignerBinary       = "asc-jwt-sign"
	ScreenshotFrameBinary = "asc-screenshot-frame"
	ImageOptimizeBinary   = "asc-image-optimize"
	VideoEncodeBinary     = "asc-video-encode"
)

// IsAvailable reports whether Swift helpers are available on this platform.
// Always returns false on non-macOS platforms.
func IsAvailable() bool {
	return runtime.GOOS == "darwin"
}

// findHelper searches for a Swift helper binary in:
// 1. ASC_SWIFT_HELPER_PATH (if configured)
// 2. Same directory as the current executable
// 3. PATH
// 4. /usr/local/bin
func findHelper(name string) (string, error) {
	// Try custom helper directory first when explicitly configured.
	if helperDir := filepath.Clean(GetSwiftHelperPath()); helperDir != "." && helperDir != "" {
		customPath := filepath.Join(helperDir, name)
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
	}

	// Try same directory as current executable
	if exePath, err := os.Executable(); err == nil {
		sameDir := filepath.Join(filepath.Dir(exePath), name)
		if _, err := os.Stat(sameDir); err == nil {
			return sameDir, nil
		}
	}

	// Try PATH
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// Try /usr/local/bin
	localBin := filepath.Join("/usr/local/bin", name)
	if _, err := os.Stat(localBin); err == nil {
		return localBin, nil
	}

	return "", fmt.Errorf("swift helper %s not found", name)
}

func runHelperJSON(ctx context.Context, helper string, args []string, target any, operation string) error {
	cmd := exec.CommandContext(ctx, helper, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%s failed: %w (stderr: %s)", operation, err, strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("%s failed: %w", operation, err)
	}

	if err := json.Unmarshal(output, target); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("failed to parse %s response: %w (stderr: %s)", operation, err, strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("failed to parse %s response: %w", operation, err)
	}

	return nil
}

// JWTSignRequest holds the parameters for JWT signing.
type JWTSignRequest struct {
	IssuerID       string
	KeyID          string
	PrivateKeyPath string
}

// JWTSignResponse is returned after JWT signing.
type JWTSignResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// SignJWT generates a JWT using native CryptoKit when available.
func SignJWT(ctx context.Context, req JWTSignRequest) (*JWTSignResponse, error) {
	if !UseSwiftHelpers() {
		return nil, fmt.Errorf("swift jwt signer disabled or unavailable on %s", runtime.GOOS)
	}

	helper, err := findHelper(JWTSignerBinary)
	if err != nil {
		return nil, err
	}

	var resp JWTSignResponse
	if err := runHelperJSON(ctx, helper, []string{
		"--issuer-id", req.IssuerID,
		"--key-id", req.KeyID,
		"--private-key-path", req.PrivateKeyPath,
		"--output", "json",
	}, &resp, "jwt sign"); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ScreenshotFrameRequest represents a screenshot framing operation
type ScreenshotFrameRequest struct {
	InputPath       string
	OutputPath      string
	DeviceType      string
	BackgroundColor string  // Optional hex color
	Padding         float64 // Optional padding
	ValidateOnly    bool
}

// ScreenshotFrameResponse is returned after framing
type ScreenshotFrameResponse struct {
	Status string `json:"status"`
	Output string `json:"output"`
	Device string `json:"device"`
}

// FrameScreenshot uses Core Image to compose screenshots into device frames.
func FrameScreenshot(ctx context.Context, req ScreenshotFrameRequest) (*ScreenshotFrameResponse, error) {
	if !UseSwiftHelpers() {
		return nil, fmt.Errorf("swift screenshot framer disabled or unavailable on %s", runtime.GOOS)
	}

	helper, err := findHelper(ScreenshotFrameBinary)
	if err != nil {
		return nil, err
	}

	args := []string{
		"frame",
		"--input", req.InputPath,
		"--output", req.OutputPath,
		"--device", req.DeviceType,
	}

	if req.BackgroundColor != "" {
		args = append(args, "--background", req.BackgroundColor)
	}

	if req.Padding != 0 {
		args = append(args, "--padding", fmt.Sprintf("%f", req.Padding))
	}

	if req.ValidateOnly {
		args = append(args, "--validate")
	}

	var resp ScreenshotFrameResponse
	if err := runHelperJSON(ctx, helper, args, &resp, "screenshot framing"); err != nil {
		return nil, err
	}

	return &resp, nil
}

// BatchFrameScreenshots processes multiple screenshots in batch.
func BatchFrameScreenshots(ctx context.Context, inputDir, outputDir, deviceType string) error {
	if !UseSwiftHelpers() {
		return fmt.Errorf("swift screenshot framer disabled or unavailable on %s", runtime.GOOS)
	}

	helper, err := findHelper(ScreenshotFrameBinary)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, helper, "batch",
		"--input-dir", inputDir,
		"--output-dir", outputDir,
		"--device", deviceType,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("batch framing failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// HelperStatus contains information about the Swift helpers
type HelperStatus struct {
	Available     bool      `json:"available"`
	Platform      string    `json:"platform"`
	JWTSigner     string    `json:"jwt_signer_path,omitempty"`
	Screenshot    string    `json:"screenshot_path,omitempty"`
	ImageOptimize string    `json:"image_optimize_path,omitempty"`
	VideoEncode   string    `json:"video_encode_path,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
}

// GetStatus returns the current status of Swift helpers.
func GetStatus() HelperStatus {
	status := HelperStatus{
		Available: UseSwiftHelpers(),
		Platform:  runtime.GOOS,
		CheckedAt: time.Now(),
	}

	if !status.Available {
		return status
	}

	if path, err := findHelper(JWTSignerBinary); err == nil {
		status.JWTSigner = path
	}
	if path, err := findHelper(ScreenshotFrameBinary); err == nil {
		status.Screenshot = path
	}
	if path, err := findHelper(ImageOptimizeBinary); err == nil {
		status.ImageOptimize = path
	}
	if path, err := findHelper(VideoEncodeBinary); err == nil {
		status.VideoEncode = path
	}

	return status
}

// ImageOptimizeRequest represents an image optimization request
type ImageOptimizeRequest struct {
	InputPath  string
	OutputPath string
	Preset     string // store, preview, thumbnail, aggressive
	Format     string // jpeg, png
}

// ImageOptimizeResult is returned after optimization
type ImageOptimizeResult struct {
	Input          string  `json:"input"`
	Output         string  `json:"output"`
	OriginalSize   int64   `json:"original_size"`
	OptimizedSize  int64   `json:"optimized_size"`
	SavingsBytes   int64   `json:"savings_bytes"`
	SavingsPercent float64 `json:"savings_percent"`
	Format         string  `json:"format"`
	Preset         string  `json:"preset"`
}

// OptimizeImage uses Core Image/Metal to optimize images.
func OptimizeImage(ctx context.Context, req ImageOptimizeRequest) (*ImageOptimizeResult, error) {
	if !UseSwiftHelpers() {
		return nil, fmt.Errorf("swift image optimizer disabled or unavailable on %s", runtime.GOOS)
	}

	helper, err := findHelper(ImageOptimizeBinary)
	if err != nil {
		return nil, err
	}

	args := []string{
		"optimize",
		"--input", req.InputPath,
		"--output", req.OutputPath,
		"--preset", req.Preset,
		"--format", req.Format,
	}

	var result ImageOptimizeResult
	if err := runHelperJSON(ctx, helper, args, &result, "image optimization"); err != nil {
		return nil, err
	}

	return &result, nil
}

// BatchOptimizeImages optimizes multiple images in a directory.
func BatchOptimizeImages(ctx context.Context, inputDir, outputDir, preset, format string, recursive bool) error {
	if !UseSwiftHelpers() {
		return fmt.Errorf("swift image optimizer disabled or unavailable on %s", runtime.GOOS)
	}

	helper, err := findHelper(ImageOptimizeBinary)
	if err != nil {
		return err
	}

	args := []string{
		"batch",
		"--input-dir", inputDir,
		"--output-dir", outputDir,
		"--preset", preset,
		"--format", format,
	}

	if recursive {
		args = append(args, "--recursive")
	}

	cmd := exec.CommandContext(ctx, helper, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("batch optimization failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// VideoEncodeResult is returned after video encoding
type VideoEncodeResult struct {
	Input            string  `json:"input"`
	Output           string  `json:"output"`
	Preset           string  `json:"preset"`
	OriginalDuration float64 `json:"original_duration"`
	OriginalSize     int64   `json:"original_file_size"`
	OutputSize       int64   `json:"output_file_size"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// EncodeVideo encodes a video with App Store optimized settings.
func EncodeVideo(ctx context.Context, inputPath, outputPath, preset string) (*VideoEncodeResult, error) {
	if !UseSwiftHelpers() {
		return nil, fmt.Errorf("swift video encoder disabled or unavailable on %s", runtime.GOOS)
	}

	helper, err := findHelper(VideoEncodeBinary)
	if err != nil {
		return nil, err
	}

	var result VideoEncodeResult
	if err := runHelperJSON(ctx, helper, []string{
		"encode",
		"--input", inputPath,
		"--output", outputPath,
		"--preset", preset,
	}, &result, "video encoding"); err != nil {
		return nil, err
	}

	return &result, nil
}
