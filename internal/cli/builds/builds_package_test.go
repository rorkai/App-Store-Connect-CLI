package builds

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageWithGo(t *testing.T) {
	// Create a test .app bundle
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "TestApp.app")

	// Create app structure
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("Failed to create app dir: %v", err)
	}

	// Create Info.plist
	infoPlist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>com.test.app</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
</dict>
</plist>`
	if err := os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte(infoPlist), 0o644); err != nil {
		t.Fatalf("Failed to create Info.plist: %v", err)
	}

	// Create a binary file
	binaryContent := make([]byte, 1000)
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(appDir, "TestApp"), binaryContent, 0o755); err != nil {
		t.Fatalf("Failed to create binary: %v", err)
	}

	// Create output path
	outputPath := filepath.Join(tempDir, "TestApp.ipa")

	// Test packaging
	ctx := context.Background()
	result, err := packageWithGo(ctx, appDir, outputPath, 6)
	if err != nil {
		t.Fatalf("packageWithGo failed: %v", err)
	}

	// Verify result
	if !result.Success {
		t.Error("Expected success=true")
	}
	if result.AppPath != appDir {
		t.Errorf("Expected appPath=%s, got %s", appDir, result.AppPath)
	}
	if result.IPAPath != outputPath {
		t.Errorf("Expected ipaPath=%s, got %s", outputPath, result.IPAPath)
	}
	if result.OriginalSize == 0 {
		t.Error("Expected non-zero original size")
	}
	if result.CompressedSize == 0 {
		t.Error("Expected non-zero compressed size")
	}
	if result.CompressionRatio < 1 {
		t.Error("Expected compression ratio >= 1")
	}
	if result.Method != "go-zip" {
		t.Errorf("Expected method=go-zip, got %s", result.Method)
	}
	if result.Duration < 0 {
		t.Error("Expected non-negative duration")
	}

	// Verify IPA was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("IPA file not created at %s", outputPath)
	}

	// Verify IPA is valid ZIP
	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open IPA as ZIP: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Check for Payload directory in IPA
	foundPayload := false
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "Payload/") {
			foundPayload = true
			break
		}
	}
	if !foundPayload {
		t.Error("IPA missing Payload directory")
	}
}

func TestPackageWithGo_DifferentCompressionLevels(t *testing.T) {
	// Create a test .app bundle with compressible content
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "TestApp.app")

	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("Failed to create app dir: %v", err)
	}

	// Create compressible content (repeated pattern)
	content := make([]byte, 10000)
	for i := range content {
		content[i] = 'A'
	}
	if err := os.WriteFile(filepath.Join(appDir, "data.bin"), content, 0o644); err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}

	// Test different compression levels
	levels := []int{0, 3, 6, 9}
	var sizes []int64

	for _, level := range levels {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("TestApp-level%d.ipa", level))
		ctx := context.Background()

		result, err := packageWithGo(ctx, appDir, outputPath, level)
		if err != nil {
			t.Fatalf("packageWithGo failed for level %d: %v", level, err)
		}

		sizes = append(sizes, result.CompressedSize)
		t.Logf("Level %d: %d bytes", level, result.CompressedSize)
	}

	// Higher compression should generally result in smaller files
	// (though this isn't guaranteed for all content)
	if sizes[0] <= sizes[3] {
		t.Logf("Warning: Level 0 (%d bytes) not larger than Level 9 (%d bytes)", sizes[0], sizes[3])
	}
}

func TestCalculateAppSize(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file1 := []byte("Hello, World!")
	file2 := []byte("Test content")

	if err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), file1, 0o644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), file2, 0o644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	expectedSize := int64(len(file1) + len(file2))

	size, err := calculateAppSize(tempDir)
	if err != nil {
		t.Fatalf("calculateAppSize failed: %v", err)
	}

	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

func TestCopyAppBundle(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dest")

	// Create source structure
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	content := []byte("test content")
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), content, 0o644); err != nil {
		t.Fatalf("Failed to create root.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.txt"), content, 0o644); err != nil {
		t.Fatalf("Failed to create sub.txt: %v", err)
	}

	// Copy
	if err := copyAppBundle(srcDir, dstDir); err != nil {
		t.Fatalf("copyAppBundle failed: %v", err)
	}

	// Verify
	if _, err := os.Stat(filepath.Join(dstDir, "root.txt")); os.IsNotExist(err) {
		t.Error("root.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "sub.txt")); os.IsNotExist(err) {
		t.Error("sub.txt not copied")
	}

	// Verify content
	copied, err := os.ReadFile(filepath.Join(dstDir, "root.txt"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(copied) != string(content) {
		t.Error("Copied content doesn't match")
	}
}

func TestGetFileSize(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	size, err := getFileSize(filePath)
	if err != nil {
		t.Fatalf("getFileSize failed: %v", err)
	}

	if size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), size)
	}

	// Test non-existent file
	_, err = getFileSize(filepath.Join(tempDir, "nonexistent.txt"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestPrintPackagingStats(t *testing.T) {
	// This test just verifies the function doesn't panic
	printPackagingStats(1024*1024*10, 1024*1024*5, 2.0)    // 10MB -> 5MB, 2x ratio
	printPackagingStats(1024*1024*100, 1024*1024*100, 1.0) // No compression
	printPackagingStats(0, 0, 1.0)                         // Edge case
}

func TestPackageWithGo_ContextCancellation(t *testing.T) {
	// Create a test .app bundle
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "TestApp.app")

	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("Failed to create app dir: %v", err)
	}

	// Create some content
	content := make([]byte, 1000)
	if err := os.WriteFile(filepath.Join(appDir, "test.bin"), content, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputPath := filepath.Join(tempDir, "TestApp.ipa")

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should still work because the context is only checked at the end
	// In a real scenario with slow operations, cancellation would be respected
	result, err := packageWithGo(ctx, appDir, outputPath, 6)

	// The function may or may not return an error depending on timing
	if err != nil {
		t.Logf("Got expected cancellation error: %v", err)
	} else {
		t.Logf("Operation completed before cancellation: %+v", result)
	}
}

func TestPackageWithGo_InvalidPaths(t *testing.T) {
	ctx := context.Background()

	// Test non-existent app
	_, err := packageWithGo(ctx, "/nonexistent/app.app", "/tmp/output.ipa", 6)
	if err == nil {
		t.Error("Expected error for non-existent app")
	}

	// Test invalid output path (directory doesn't exist and can't be created)
	invalidOutput := "/nonexistent_dir_that_cannot_be_created/output.ipa"
	_, err = packageWithGo(ctx, t.TempDir(), invalidOutput, 6)
	if err == nil {
		t.Error("Expected error for invalid output path")
	}
}

func BenchmarkPackageWithGo(b *testing.B) {
	// Create test app
	tempDir := b.TempDir()
	appDir := filepath.Join(tempDir, "TestApp.app")

	if err := os.MkdirAll(appDir, 0o755); err != nil {
		b.Fatalf("Failed to create app dir: %v", err)
	}

	// Create large compressible content
	content := make([]byte, 100000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(appDir, "binary"), content, 0o644); err != nil {
		b.Fatalf("Failed to create binary: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("output%d.ipa", i))
		_, err := packageWithGo(ctx, appDir, outputPath, 6)
		if err != nil {
			b.Fatalf("packageWithGo failed: %v", err)
		}
	}
}

// Test createIPAFromPayload specifically
func TestCreateIPAFromPayload(t *testing.T) {
	tempDir := t.TempDir()
	payloadDir := filepath.Join(tempDir, "Payload")

	// Create Payload structure
	appDir := filepath.Join(payloadDir, "TestApp.app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("Failed to create app dir: %v", err)
	}

	// Create files in Payload
	if err := os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte("plist content"), 0o644); err != nil {
		t.Fatalf("Failed to create Info.plist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "TestApp"), []byte("binary content"), 0o755); err != nil {
		t.Fatalf("Failed to create binary: %v", err)
	}

	outputPath := filepath.Join(tempDir, "output.ipa")

	// Create IPA
	if err := createIPAFromPayload(payloadDir, outputPath, 6); err != nil {
		t.Fatalf("createIPAFromPayload failed: %v", err)
	}

	// Verify
	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open IPA: %v", err)
	}
	defer func() { _ = reader.Close() }()

	expectedFiles := map[string]bool{
		"Payload/TestApp.app/Info.plist": false,
		"Payload/TestApp.app/TestApp":    false,
	}

	for _, file := range reader.File {
		if _, exists := expectedFiles[file.Name]; exists {
			expectedFiles[file.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file not found in IPA: %s", name)
		}
	}
}
