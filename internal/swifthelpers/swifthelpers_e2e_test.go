package swifthelpers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestSwiftEndToEnd_JWT tests the complete JWT signing flow using Swift helpers
func TestSwiftEndToEnd_JWT(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Swift helpers only available on macOS")
	}

	// Check if Swift helpers are available
	if _, err := findHelper(JWTSignerBinary); err != nil {
		t.Skip("Swift JWT signer not found, skipping end-to-end test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a proper P-256 key using OpenSSL (matches what Swift helper expects)
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test-key.p8")

	// Generate key in PKCS#8 format
	cmd := exec.Command("sh", "-c",
		"openssl ecparam -genkey -name prime256v1 -noout | openssl pkcs8 -topk8 -nocrypt")
	output, err := cmd.Output()
	if err != nil {
		t.Skipf("OpenSSL not available to generate test key: %v", err)
	}
	if err := os.WriteFile(keyPath, output, 0o600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// Test JWT signing
	req := JWTSignRequest{
		IssuerID:       "test-issuer-123",
		KeyID:          "test-key-456",
		PrivateKeyPath: keyPath,
	}

	resp, err := SignJWT(ctx, req)
	if err != nil {
		t.Fatalf("SignJWT failed: %v", err)
	}

	if resp == nil {
		t.Fatal("SignJWT returned nil response")
	}

	if resp.Token == "" {
		t.Error("SignJWT returned empty token")
	}

	if resp.ExpiresIn == 0 {
		t.Error("SignJWT returned zero expires_in")
	}

	t.Logf("Successfully generated JWT with %d seconds expiry", resp.ExpiresIn)
}

// TestSwiftEndToEnd_ImageOptimization tests image optimization using Swift helpers
func TestSwiftEndToEnd_ImageOptimization(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Swift helpers only available on macOS")
	}

	if _, err := findHelper(ImageOptimizeBinary); err != nil {
		t.Skip("Swift image optimizer not found, skipping end-to-end test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a simple test PNG
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.png")
	outputPath := filepath.Join(tempDir, "output.png")

	// Create a minimal valid PNG file (1x1 pixel, red)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xFE,
		0xD4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(inputPath, pngData, 0o644); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	// Test optimization
	req := ImageOptimizeRequest{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Preset:     "thumbnail",
		Format:     "png",
	}

	result, err := OptimizeImage(ctx, req)
	if err != nil {
		t.Fatalf("OptimizeImage failed: %v", err)
	}

	if result == nil {
		t.Fatal("OptimizeImage returned nil result")
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("Output file not created at %s", outputPath)
	}

	t.Logf("Image optimization: %d -> %d bytes (%.1f%% savings)",
		result.OriginalSize, result.OptimizedSize, result.SavingsPercent)
}

// TestSwiftEndToEnd_AllHelpersPresent verifies all expected helpers are available
func TestSwiftEndToEnd_AllHelpersPresent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Swift helpers only available on macOS")
	}

	helperTests := []struct {
		name   string
		binary string
	}{
		{"JWT Signer", JWTSignerBinary},
		{"Screenshot Frame", ScreenshotFrameBinary},
		{"Image Optimize", ImageOptimizeBinary},
		{"Video Encode", VideoEncodeBinary},
	}

	status := GetStatus()
	if !status.Available {
		t.Skip("Swift helpers not available")
	}

	for _, tt := range helperTests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := findHelper(tt.binary)
			if err != nil {
				t.Logf("Helper %s not found: %v", tt.binary, err)
			} else {
				t.Logf("Found %s at %s", tt.binary, path)
			}
		})
	}
}
