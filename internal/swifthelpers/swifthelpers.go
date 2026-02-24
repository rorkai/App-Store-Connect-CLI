// Package swifthelpers provides integration with Swift helper tools for
// performance-critical operations on macOS.
//
// The Swift helpers leverage native macOS frameworks for genuine speedups:
//   - Native CryptoKit JWT signing (hardware-accelerated P-256)
//   - Native Security.framework keychain access (no CGO overhead)
//   - Core Image/Metal screenshot framing and image optimization
//   - AVFoundation video encoding
//
// When Swift helpers are not available (Linux, Windows, or not installed),
// the package falls back to pure Go implementations.
package swifthelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Helper names
const (
	JWTSignerBinary       = "asc-jwt-sign"
	KeychainBinary        = "asc-keychain"
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
// 1. Same directory as the current executable
// 2. PATH
// 3. /usr/local/bin
func findHelper(name string) (string, error) {
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
	if !IsAvailable() {
		return nil, fmt.Errorf("swift jwt signer not available on %s", runtime.GOOS)
	}

	helper, err := findHelper(JWTSignerBinary)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, helper,
		"--issuer-id", req.IssuerID,
		"--key-id", req.KeyID,
		"--private-key-path", req.PrivateKeyPath,
		"--output", "json",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("jwt sign failed: %w (output: %s)", err, string(output))
	}

	var resp JWTSignResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse jwt response: %w", err)
	}

	return &resp, nil
}

// KeychainCredential represents stored API credentials
type KeychainCredential struct {
	Name           string `json:"name"`
	KeyID          string `json:"key_id"`
	IssuerID       string `json:"issuer_id"`
	PrivateKeyPath string `json:"private_key_path"`
}

// KeychainStore stores credentials in the macOS keychain.
func KeychainStore(ctx context.Context, cred KeychainCredential) error {
	if !IsAvailable() {
		return fmt.Errorf("swift keychain helper not available on %s", runtime.GOOS)
	}

	helper, err := findHelper(KeychainBinary)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, helper, "store",
		cred.Name,
		"--key-id", cred.KeyID,
		"--issuer-id", cred.IssuerID,
		"--private-key-path", cred.PrivateKeyPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain store failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// KeychainGet retrieves a credential from the macOS keychain.
func KeychainGet(ctx context.Context, name string) (*KeychainCredential, error) {
	if !IsAvailable() {
		return nil, fmt.Errorf("swift keychain helper not available on %s", runtime.GOOS)
	}

	helper, err := findHelper(KeychainBinary)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, helper, "get", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("keychain get failed: %w (output: %s)", err, string(output))
	}

	var cred KeychainCredential
	if err := json.Unmarshal(output, &cred); err != nil {
		return nil, fmt.Errorf("failed to parse credential: %w", err)
	}

	return &cred, nil
}

// KeychainList returns all stored credentials.
func KeychainList(ctx context.Context) ([]KeychainCredential, error) {
	if !IsAvailable() {
		return nil, fmt.Errorf("swift keychain helper not available on %s", runtime.GOOS)
	}

	helper, err := findHelper(KeychainBinary)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, helper, "list", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("keychain list failed: %w (output: %s)", err, string(output))
	}

	var creds []KeychainCredential
	if err := json.Unmarshal(output, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials list: %w", err)
	}

	return creds, nil
}

// KeychainDelete removes a credential from the keychain.
func KeychainDelete(ctx context.Context, name string) error {
	if !IsAvailable() {
		return fmt.Errorf("swift keychain helper not available on %s", runtime.GOOS)
	}

	helper, err := findHelper(KeychainBinary)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, helper, "delete", "--force", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain delete failed: %w (output: %s)", err, string(output))
	}

	return nil
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
	if !IsAvailable() {
		return nil, fmt.Errorf("swift screenshot framer not available on %s", runtime.GOOS)
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

	cmd := exec.CommandContext(ctx, helper, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("screenshot framing failed: %w (output: %s)", err, string(output))
	}

	var resp ScreenshotFrameResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse framing response: %w", err)
	}

	return &resp, nil
}

// BatchFrameScreenshots processes multiple screenshots in batch.
func BatchFrameScreenshots(ctx context.Context, inputDir, outputDir, deviceType string) error {
	if !IsAvailable() {
		return fmt.Errorf("swift screenshot framer not available on %s", runtime.GOOS)
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
	Keychain      string    `json:"keychain_path,omitempty"`
	Screenshot    string    `json:"screenshot_path,omitempty"`
	ImageOptimize string    `json:"image_optimize_path,omitempty"`
	VideoEncode   string    `json:"video_encode_path,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
}

// GetStatus returns the current status of Swift helpers.
func GetStatus() HelperStatus {
	status := HelperStatus{
		Available: IsAvailable(),
		Platform:  runtime.GOOS,
		CheckedAt: time.Now(),
	}

	if !status.Available {
		return status
	}

	if path, err := findHelper(JWTSignerBinary); err == nil {
		status.JWTSigner = path
	}
	if path, err := findHelper(KeychainBinary); err == nil {
		status.Keychain = path
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
	if !IsAvailable() {
		return nil, fmt.Errorf("swift image optimizer not available on %s", runtime.GOOS)
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

	cmd := exec.CommandContext(ctx, helper, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("image optimization failed: %w (output: %s)", err, string(output))
	}

	var result ImageOptimizeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse optimization result: %w", err)
	}

	return &result, nil
}

// BatchOptimizeImages optimizes multiple images in a directory.
func BatchOptimizeImages(ctx context.Context, inputDir, outputDir, preset, format string, recursive bool) error {
	if !IsAvailable() {
		return fmt.Errorf("swift image optimizer not available on %s", runtime.GOOS)
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
	if !IsAvailable() {
		return nil, fmt.Errorf("swift video encoder not available on %s", runtime.GOOS)
	}

	helper, err := findHelper(VideoEncodeBinary)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, helper, "encode",
		"--input", inputPath,
		"--output", outputPath,
		"--preset", preset,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("video encoding failed: %w (output: %s)", err, string(output))
	}

	var result VideoEncodeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse encode result: %w", err)
	}

	return &result, nil
}

// Daemon support for zero-overhead Swift operations
const DefaultDaemonSocketPath = "/tmp/asc-swift-daemon.sock"

// DaemonClient connects to the Swift daemon for fast operations
type DaemonClient struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
}

// NewDaemonClient creates a new daemon client
func NewDaemonClient(socketPath string) *DaemonClient {
	if socketPath == "" {
		socketPath = DefaultDaemonSocketPath
	}
	return &DaemonClient{socketPath: socketPath}
}

// Connect establishes connection to the daemon
func (c *DaemonClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil // Already connected
	}

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	c.conn = conn
	return nil
}

// Close closes the daemon connection
func (c *DaemonClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// IsDaemonRunning checks if the daemon is available
func (c *DaemonClient) IsDaemonRunning() bool {
	if err := c.Connect(); err != nil {
		return false
	}
	defer func() { _ = c.Close() }()
	return true
}

// SignJWTWithDaemon signs a JWT using the daemon (zero subprocess overhead)
func (c *DaemonClient) SignJWTWithDaemon(ctx context.Context, req JWTSignRequest) (*JWTSignResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Build request
	request := map[string]string{
		"cmd":       "jwt_sign",
		"issuer_id": req.IssuerID,
		"key_id":    req.KeyID,
		"key_path":  req.PrivateKeyPath,
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	if _, err := c.conn.Write(requestData); err != nil {
		// Connection might be stale, try reconnecting once
		_ = c.conn.Close()
		c.conn = nil
		if err := c.Connect(); err != nil {
			return nil, err
		}
		if _, err := c.conn.Write(requestData); err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
	}

	// Read response
	// Signal end of request (safe type assertion)
	if unixConn, ok := c.conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}

	responseData, err := io.ReadAll(c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Success   bool   `json:"success"`
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
		Error     string `json:"error"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("daemon signing failed: %s", response.Error)
	}

	return &JWTSignResponse{
		Token:     response.Token,
		ExpiresIn: response.ExpiresIn,
	}, nil
}

// StartDaemon starts the Swift daemon if not already running
func StartDaemon(ctx context.Context, socketPath string) error {
	if socketPath == "" {
		socketPath = DefaultDaemonSocketPath
	}

	// Check if already running
	client := NewDaemonClient(socketPath)
	if client.IsDaemonRunning() {
		return nil // Already running
	}

	helper, err := findHelper("asc-swift-daemon")
	if err != nil {
		return fmt.Errorf("daemon binary not found: %w", err)
	}

	// Start daemon in background
	cmd := exec.CommandContext(ctx, helper, "--socket-path", socketPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait a moment for daemon to start
	time.Sleep(100 * time.Millisecond)

	// Verify it's running
	if !client.IsDaemonRunning() {
		return fmt.Errorf("daemon failed to start")
	}

	return nil
}

// StopDaemon stops the running daemon
func StopDaemon(socketPath string) error {
	if socketPath == "" {
		socketPath = DefaultDaemonSocketPath
	}

	if _, err := os.Stat(socketPath); err != nil {
		return nil // Not running
	}

	return os.Remove(socketPath)
}
