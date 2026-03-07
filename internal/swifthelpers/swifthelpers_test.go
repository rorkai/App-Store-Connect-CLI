package swifthelpers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsAvailable(t *testing.T) {
	available := IsAvailable()

	if runtime.GOOS == "darwin" {
		// On macOS, should be true
		if !available {
			t.Error("Expected IsAvailable() to return true on macOS")
		}
	} else {
		// On non-macOS, should be false
		if available {
			t.Error("Expected IsAvailable() to return false on non-macOS")
		}
	}
}

func TestGetStatus(t *testing.T) {
	status := GetStatus()

	if status.Platform != runtime.GOOS {
		t.Errorf("Expected platform %s, got %s", runtime.GOOS, status.Platform)
	}

	if runtime.GOOS == "darwin" {
		// On macOS, status should indicate availability
		if !status.Available {
			t.Error("Expected Available to be true on macOS")
		}

		// Check that paths are populated if helpers are found
		// Note: This test may fail if helpers aren't built
		t.Logf("JWT Signer path: %s", status.JWTSigner)
		t.Logf("Screenshot path: %s", status.Screenshot)
	} else {
		// On non-macOS, all paths should be empty
		if status.JWTSigner != "" || status.Screenshot != "" {
			t.Error("Expected empty paths on non-macOS")
		}
	}
}

func TestFindHelper_NotFound(t *testing.T) {
	// Test with a non-existent helper
	_, err := findHelper("asc-nonexistent-helper")
	if err == nil {
		t.Error("Expected error when finding non-existent helper")
	}
}

func TestFindHelper_UsesCustomHelperPath(t *testing.T) {
	tempDir := t.TempDir()
	helperName := "asc-test-custom-helper"
	helperPath := filepath.Join(tempDir, helperName)

	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("Failed to create fake helper: %v", err)
	}

	t.Setenv(EnvSwiftHelperPath, tempDir)

	got, err := findHelper(helperName)
	if err != nil {
		t.Fatalf("findHelper returned error: %v", err)
	}
	if got != helperPath {
		t.Fatalf("Expected helper path %q, got %q", helperPath, got)
	}
}

func TestUseSwiftHelpers_Disabled(t *testing.T) {
	t.Setenv(EnvDisableSwiftHelpers, "true")
	t.Setenv(EnvPreferSwiftHelpers, "")

	if UseSwiftHelpers() {
		t.Fatal("Expected UseSwiftHelpers to return false when disabled")
	}
}

func TestUseSwiftHelpers_Preferred(t *testing.T) {
	t.Setenv(EnvDisableSwiftHelpers, "")
	t.Setenv(EnvPreferSwiftHelpers, "true")

	if !UseSwiftHelpers() {
		t.Fatal("Expected UseSwiftHelpers to return true when preferred")
	}
}

func TestSignJWT_IgnoresHelperStderrWhenJSONStdoutIsValid(t *testing.T) {
	tempDir := t.TempDir()
	helperPath := filepath.Join(tempDir, JWTSignerBinary)
	script := "#!/bin/sh\n" +
		"echo 'warning on stderr' 1>&2\n" +
		"echo '{\"token\":\"abc123\",\"expires_in\":600}'\n"

	if err := os.WriteFile(helperPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to create fake helper: %v", err)
	}

	t.Setenv(EnvDisableSwiftHelpers, "")
	t.Setenv(EnvPreferSwiftHelpers, "true")
	t.Setenv(EnvSwiftHelperPath, tempDir)

	resp, err := SignJWT(context.Background(), JWTSignRequest{
		IssuerID:       "issuer",
		KeyID:          "key",
		PrivateKeyPath: "/tmp/key.p8",
	})
	if err != nil {
		t.Fatalf("SignJWT returned error: %v", err)
	}
	if resp.Token != "abc123" {
		t.Fatalf("Expected token abc123, got %q", resp.Token)
	}
	if resp.ExpiresIn != 600 {
		t.Fatalf("Expected expires_in 600, got %d", resp.ExpiresIn)
	}
}

func TestSignJWT_NotAvailable(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping on macOS - helper might be available")
	}

	ctx := context.Background()
	req := JWTSignRequest{
		IssuerID:       "test",
		KeyID:          "test",
		PrivateKeyPath: "/test/key.p8",
	}

	_, err := SignJWT(ctx, req)
	if err == nil {
		t.Error("Expected error when signing JWT on non-macOS")
	}
}

func TestFrameScreenshot_NotAvailable(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping on macOS - helper might be available")
	}

	ctx := context.Background()
	req := ScreenshotFrameRequest{
		InputPath:  "test.png",
		OutputPath: "out.png",
		DeviceType: "iphone-16-pro",
	}

	_, err := FrameScreenshot(ctx, req)
	if err == nil {
		t.Error("Expected error when framing screenshot on non-macOS")
	}
}

func TestBatchFrameScreenshots_NotAvailable(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping on macOS - helper might be available")
	}

	ctx := context.Background()

	err := BatchFrameScreenshots(ctx, "./input", "./output", "iphone-16-pro")
	if err == nil {
		t.Error("Expected error when batch framing on non-macOS")
	}
}

func TestHelperStatus(t *testing.T) {
	status := HelperStatus{
		Available:  true,
		Platform:   "darwin",
		JWTSigner:  "/usr/local/bin/asc-jwt-sign",
		Screenshot: "/usr/local/bin/asc-screenshot-frame",
	}

	if !status.Available {
		t.Error("Expected status to be available")
	}

	if status.Platform != "darwin" {
		t.Error("Expected platform to be darwin")
	}
}
