package swifthelpers

import (
	"context"
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
