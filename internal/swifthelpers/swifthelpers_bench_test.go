package swifthelpers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/golang-jwt/jwt/v5"
)

// BenchmarkJWTSigning compares Go (golang-jwt) vs Swift (CryptoKit) JWT signing performance.
//
// Expected result: Swift CryptoKit benefits from hardware-accelerated P-256 on Apple Silicon,
// but subprocess overhead dominates. The benchmark documents the full end-to-end cost
// (subprocess invocation + key loading + signing) vs in-process Go signing.
func BenchmarkJWTSigning(b *testing.B) {
	if runtime.GOOS != "darwin" {
		b.Skip("Swift helpers only available on macOS")
	}

	// Check if Swift helper is available
	_, swiftAvailable := findHelper(JWTSignerBinary)

	// Generate test key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatalf("Failed to generate key: %v", err)
	}

	// Export to temp file for Swift
	tempDir := b.TempDir()
	keyPath := filepath.Join(tempDir, "bench-key.p8")

	privKeyBytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: privKeyBytes}
	keyFile, _ := os.Create(keyPath)
	_ = pem.Encode(keyFile, block)
	_ = keyFile.Close()

	ctx := context.Background()

	// Benchmark Go implementation (in-process, no subprocess)
	b.Run("Go_golang-jwt", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := generateJWTGo("test-key", "test-issuer", privateKey)
			if err != nil {
				b.Fatalf("Go JWT generation failed: %v", err)
			}
		}
	})

	// Benchmark Swift implementation (subprocess + CryptoKit)
	if swiftAvailable == nil {
		b.Run("Swift_CryptoKit_subprocess", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := SignJWT(ctx, JWTSignRequest{
					IssuerID:       "test-issuer",
					KeyID:          "test-key",
					PrivateKeyPath: keyPath,
				})
				if err != nil {
					b.Fatalf("Swift JWT generation failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkKeychainOperations compares Go (99designs/keyring) vs Swift (Security.framework)
// keychain store/get/delete cycles.
//
// The Swift helper uses SecItem* APIs directly via Security.framework, avoiding CGO overhead
// that the Go keyring package incurs through cgo-based bindings.
func BenchmarkKeychainOperations(b *testing.B) {
	if runtime.GOOS != "darwin" {
		b.Skip("Keychain benchmarks only available on macOS")
	}

	if os.Getenv("ASC_BYPASS_KEYCHAIN") == "1" {
		b.Skip("Keychain bypassed via ASC_BYPASS_KEYCHAIN=1")
	}

	_, swiftAvailable := findHelper(KeychainBinary)
	ctx := context.Background()

	// Benchmark Go keyring store+get+delete cycle
	b.Run("Go_99designs_keyring", func(b *testing.B) {
		kr, err := keyring.Open(keyring.Config{
			ServiceName:              "asc-bench-test",
			KeychainTrustApplication: true,
			AllowedBackends:          []keyring.BackendType{keyring.KeychainBackend},
		})
		if err != nil {
			b.Skipf("Go keyring not available: %v", err)
		}

		payload, _ := json.Marshal(map[string]string{
			"key_id":           "bench-key-id",
			"issuer_id":        "bench-issuer-id",
			"private_key_path": "/tmp/bench.p8",
		})

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemKey := fmt.Sprintf("asc-bench-%d", i)

			// Store
			err := kr.Set(keyring.Item{
				Key:   itemKey,
				Data:  payload,
				Label: "ASC Bench Test",
			})
			if err != nil {
				b.Fatalf("Go keyring store failed: %v", err)
			}

			// Get
			_, err = kr.Get(itemKey)
			if err != nil {
				b.Fatalf("Go keyring get failed: %v", err)
			}

			// Delete
			err = kr.Remove(itemKey)
			if err != nil {
				b.Fatalf("Go keyring delete failed: %v", err)
			}
		}
	})

	// Benchmark Swift Security.framework store+get+delete cycle
	if swiftAvailable == nil {
		b.Run("Swift_Security_framework", func(b *testing.B) {
			tempDir := b.TempDir()
			keyPath := filepath.Join(tempDir, "bench.p8")
			_ = os.WriteFile(keyPath, []byte("fake-key-data"), 0o600)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				name := fmt.Sprintf("asc-bench-swift-%d", i)

				// Store
				err := KeychainStore(ctx, KeychainCredential{
					Name:           name,
					KeyID:          "bench-key-id",
					IssuerID:       "bench-issuer-id",
					PrivateKeyPath: keyPath,
				})
				if err != nil {
					b.Fatalf("Swift keychain store failed: %v", err)
				}

				// Get
				_, err = KeychainGet(ctx, name)
				if err != nil {
					b.Fatalf("Swift keychain get failed: %v", err)
				}

				// Delete
				err = KeychainDelete(ctx, name)
				if err != nil {
					b.Fatalf("Swift keychain delete failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkScreenshotFraming compares Swift (CoreImage/Metal) screenshot framing
// against a Go baseline of simple image file copy (since Go has no native CoreImage equivalent).
//
// The Swift helper uses CIFilter composition with Lanczos scaling and Metal-accelerated
// rendering. The Go baseline represents the minimum I/O cost for comparison.
func BenchmarkScreenshotFraming(b *testing.B) {
	if runtime.GOOS != "darwin" {
		b.Skip("Swift helpers only available on macOS")
	}

	_, swiftAvailable := findHelper(ScreenshotFrameBinary)

	tempDir := b.TempDir()

	// Create test screenshots at device-appropriate sizes
	sizes := []struct {
		name   string
		width  int
		height int
		device string
	}{
		{"iPhone_6_7", 1290, 2796, "iphone-16-pro"},
		{"iPad_11", 2388, 1668, "ipad-pro-11"},
	}

	for _, size := range sizes {
		inputPath := filepath.Join(tempDir, fmt.Sprintf("%s_input.png", size.name))
		outputPath := filepath.Join(tempDir, fmt.Sprintf("%s_framed.png", size.name))

		// Create test PNG at device resolution
		if err := createTestPNG(inputPath, size.width, size.height); err != nil {
			b.Skipf("Failed to create test PNG (sips not available): %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Benchmark Swift CoreImage/Metal framing
		if swiftAvailable == nil {
			b.Run(fmt.Sprintf("Swift_CoreImage_%s", size.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = os.Remove(outputPath)
					_, err := FrameScreenshot(ctx, ScreenshotFrameRequest{
						InputPath:  inputPath,
						OutputPath: outputPath,
						DeviceType: size.device,
					})
					if err != nil {
						b.Fatalf("Swift screenshot framing failed: %v", err)
					}
				}
			})
		}

		// Go baseline: read + write (no framing, just I/O cost)
		b.Run(fmt.Sprintf("Go_file_copy_baseline_%s", size.name), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = os.Remove(outputPath)
				data, _ := os.ReadFile(inputPath)
				_ = os.WriteFile(outputPath, data, 0o644)
			}
		})
	}
}

// BenchmarkImageOptimization compares Swift (CoreImage/Metal) image optimization
// against a Go baseline of file copy (Go has no native GPU-accelerated image processing).
//
// The Swift helper uses CIContext backed by MTLDevice for Metal-accelerated processing
// with configurable quality presets (store=95%, preview=85%, thumbnail=75%, aggressive=60%).
func BenchmarkImageOptimization(b *testing.B) {
	if runtime.GOOS != "darwin" {
		b.Skip("Swift helpers only available on macOS")
	}

	_, swiftAvailable := findHelper(ImageOptimizeBinary)

	tempDir := b.TempDir()
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"Small_100x100", 100, 100},
		{"Medium_1000x1000", 1000, 1000},
		{"Large_3000x3000", 3000, 3000},
	}

	for _, size := range sizes {
		inputPath := filepath.Join(tempDir, fmt.Sprintf("%s.png", size.name))
		outputPath := filepath.Join(tempDir, fmt.Sprintf("%s-optimized.png", size.name))

		if err := createTestPNG(inputPath, size.width, size.height); err != nil {
			b.Skipf("Failed to create test PNG (sips not available): %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Benchmark each preset with Swift
		if swiftAvailable == nil {
			for _, preset := range []string{"store", "preview", "thumbnail"} {
				presetOutput := filepath.Join(tempDir, fmt.Sprintf("%s-%s.png", size.name, preset))
				b.Run(fmt.Sprintf("Swift_Metal_%s_%s", size.name, preset), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						_ = os.Remove(presetOutput)
						_, err := OptimizeImage(ctx, ImageOptimizeRequest{
							InputPath:  inputPath,
							OutputPath: presetOutput,
							Preset:     preset,
							Format:     "png",
						})
						if err != nil {
							b.Fatalf("Swift optimization failed: %v", err)
						}
					}
				})
			}
		}

		// Go baseline: file copy (no optimization capability)
		b.Run(fmt.Sprintf("Go_file_copy_baseline_%s", size.name), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = os.Remove(outputPath)
				data, _ := os.ReadFile(inputPath)
				_ = os.WriteFile(outputPath, data, 0o644)
			}
		})
	}
}

// BenchmarkVideoEncoding compares Swift (AVFoundation) video encoding against
// a Go baseline using ffmpeg (if available).
//
// The Swift helper uses AVAssetExportSession with hardware-accelerated H.264 encoding
// on Apple Silicon. Presets: store (6Mbps), preview (4Mbps), compact (2Mbps).
func BenchmarkVideoEncoding(b *testing.B) {
	if runtime.GOOS != "darwin" {
		b.Skip("Swift helpers only available on macOS")
	}

	_, swiftAvailable := findHelper(VideoEncodeBinary)

	tempDir := b.TempDir()

	// Create a test video using ffmpeg (if available)
	inputPath := filepath.Join(tempDir, "test_input.mov")
	if err := createTestVideo(inputPath, 5); err != nil {
		b.Skipf("Failed to create test video (ffmpeg not available): %v", err)
	}

	presets := []string{"store", "preview", "compact"}

	for _, preset := range presets {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Benchmark Swift AVFoundation encoding
		if swiftAvailable == nil {
			b.Run(fmt.Sprintf("Swift_AVFoundation_%s", preset), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					outputPath := filepath.Join(tempDir, fmt.Sprintf("swift_%s_%d.mp4", preset, i))
					_, err := EncodeVideo(ctx, inputPath, outputPath, preset)
					if err != nil {
						b.Fatalf("Swift video encoding failed: %v", err)
					}
					_ = os.Remove(outputPath)
				}
			})
		}

		// Benchmark ffmpeg baseline (if available)
		b.Run(fmt.Sprintf("Go_ffmpeg_%s", preset), func(b *testing.B) {
			ffmpeg, err := exec.LookPath("ffmpeg")
			if err != nil {
				b.Skip("ffmpeg not available for baseline comparison")
			}

			// Map presets to ffmpeg bitrates
			bitrates := map[string]string{
				"store":   "6M",
				"preview": "4M",
				"compact": "2M",
			}
			bitrate := bitrates[preset]

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				outputPath := filepath.Join(tempDir, fmt.Sprintf("ffmpeg_%s_%d.mp4", preset, i))
				cmd := exec.Command(ffmpeg,
					"-y", "-i", inputPath,
					"-c:v", "libx264",
					"-b:v", bitrate,
					"-preset", "fast",
					"-an",
					outputPath,
				)
				cmd.Stdout = nil
				cmd.Stderr = nil
				if err := cmd.Run(); err != nil {
					b.Fatalf("ffmpeg encoding failed: %v", err)
				}
				_ = os.Remove(outputPath)
			}
		})
	}
}

// generateJWTGo generates a JWT using golang-jwt library
func generateJWTGo(keyID, issuerID string, privateKey *ecdsa.PrivateKey) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": issuerID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute * 20).Unix(),
		"aud": "appstoreconnect-v1",
	})
	token.Header["kid"] = keyID
	token.Header["alg"] = "ES256"
	token.Header["typ"] = "JWT"

	return token.SignedString(privateKey)
}

// createTestPNG creates a test PNG file using macOS sips
func createTestPNG(path string, width, height int) error {
	cmd := exec.Command("sips", "-s", "format", "png",
		"-Z", fmt.Sprintf("%d", max(width, height)),
		"/System/Library/CoreServices/DefaultDesktop.heic",
		"--out", path)
	return cmd.Run()
}

// createTestVideo creates a test video file using ffmpeg
func createTestVideo(path string, durationSecs int) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	cmd := exec.Command(ffmpeg,
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%d:size=1920x1080:rate=30", durationSecs),
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "ultrafast",
		path,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
