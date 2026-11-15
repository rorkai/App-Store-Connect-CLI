package xcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

func TestGenerateExportOptions_AutomaticInfersArchiveTeamAndWritesPlist(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "ARCHIVE123")
	outputPath := filepath.Join(t.TempDir(), "nested", "ExportOptions.plist")

	result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
	})
	if err != nil {
		t.Fatalf("GenerateExportOptions() error: %v", err)
	}

	if result.Path != outputPath {
		t.Fatalf("result path = %q, want %q", result.Path, outputPath)
	}
	if result.ArchivePath != archivePath {
		t.Fatalf("result archive path = %q, want %q", result.ArchivePath, archivePath)
	}
	if result.Method != "app-store-connect" || result.Destination != "export" || result.SigningStyle != "automatic" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.TeamID != "ARCHIVE123" {
		t.Fatalf("result team ID = %q, want inferred archive team", result.TeamID)
	}
	if result.Overwritten {
		t.Fatal("fresh output must not be reported as overwritten")
	}

	payload := readExportOptionsTestPlist(t, outputPath)
	if payload["method"] != "app-store-connect" {
		t.Fatalf("plist method = %#v", payload["method"])
	}
	if payload["destination"] != "export" {
		t.Fatalf("plist destination = %#v", payload["destination"])
	}
	if payload["signingStyle"] != "automatic" {
		t.Fatalf("plist signingStyle = %#v", payload["signingStyle"])
	}
	if payload["teamID"] != "ARCHIVE123" {
		t.Fatalf("plist teamID = %#v, want inferred archive team", payload["teamID"])
	}
	if _, found := payload["provisioningProfiles"]; found {
		t.Fatalf("automatic signing plist unexpectedly contains provisioning profiles: %#v", payload)
	}
}

func TestNormalizeExportOptionsSigningStyle(t *testing.T) {
	for _, tc := range []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "", want: "", wantErr: true},
		{input: " automatic ", want: "automatic"},
		{input: "manual", want: "manual"},
		{input: "heuristic", want: "heuristic", wantErr: true},
	} {
		got, err := NormalizeExportOptionsSigningStyle(tc.input)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Fatalf("NormalizeExportOptionsSigningStyle(%q) = %q, %v; want %q, error=%v", tc.input, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestNormalizeExportOptionsMethod(t *testing.T) {
	for _, tc := range []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "", want: "", wantErr: true},
		{input: " app-store-connect ", want: "app-store-connect"},
		{input: "release-testing", want: "release-testing"},
		{input: "ad-hoc", want: "ad-hoc", wantErr: true},
		{input: "enterprise", want: "enterprise"},
	} {
		got, err := NormalizeExportOptionsMethod(tc.input)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Fatalf("NormalizeExportOptionsMethod(%q) = %q, %v; want %q, error=%v", tc.input, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestGenerateExportOptions_ReleaseTestingWritesModernMethod(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

	result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		Method:       "release-testing",
		Destination:  "export",
		SigningStyle: "automatic",
	})
	if err != nil {
		t.Fatalf("GenerateExportOptions() error: %v", err)
	}
	if result.Method != "release-testing" {
		t.Fatalf("result method = %q, want release-testing", result.Method)
	}
	if payload := readExportOptionsTestPlist(t, outputPath); payload["method"] != "release-testing" {
		t.Fatalf("plist method = %#v, want release-testing", payload["method"])
	}
}

func TestGenerateExportOptions_ReleaseTestingRejectsUploadDestination(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
		Method:      "release-testing",
		Destination: "upload",
	})
	if err == nil || !strings.Contains(err.Error(), "release-testing") || !strings.Contains(err.Error(), "destination=export") {
		t.Fatalf("expected release-testing destination error, got %v", err)
	}
	if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid release-testing options created output: %v", statErr)
	}
}

func TestGenerateExportOptions_ManualReleaseTestingPassesMethodToResolver(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(_ context.Context, _ string, teamID, method string) (manualExportOptions, error) {
		if method != exportOptionsMethodReleaseTesting {
			t.Fatalf("manual generator method = %q, want release-testing", method)
		}
		return manualExportOptions{
			TeamID:             teamID,
			SigningCertificate: "Apple Distribution: Example (TEAM123)",
			ProvisioningProfiles: map[string]string{
				"com.example.demo": "adhoc-profile-uuid",
			},
		}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		Method:       "release-testing",
		SigningStyle: "manual",
	})
	if err != nil {
		t.Fatalf("GenerateExportOptions() error: %v", err)
	}
	if result.Method != "release-testing" || result.ProvisioningProfiles["com.example.demo"] != "adhoc-profile-uuid" {
		t.Fatalf("unexpected release-testing manual result: %#v", result)
	}
}

func TestGenerateExportOptions_ExplicitTeamOverridesArchiveTeam(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "ARCHIVE123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

	result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
		TeamID:      "OVERRIDE456",
	})
	if err != nil {
		t.Fatalf("GenerateExportOptions() error: %v", err)
	}
	if result.TeamID != "OVERRIDE456" {
		t.Fatalf("result team ID = %q, want explicit team", result.TeamID)
	}
	if payload := readExportOptionsTestPlist(t, outputPath); payload["teamID"] != "OVERRIDE456" {
		t.Fatalf("plist teamID = %#v, want explicit team", payload["teamID"])
	}
}

func TestGenerateExportOptions_DestinationRoundTripsThroughDirectUploadDetection(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")

	for _, destination := range []string{"export", "upload"} {
		t.Run(destination, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
			result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
				ArchivePath:  archivePath,
				OutputPath:   outputPath,
				Destination:  destination,
				SigningStyle: "automatic",
			})
			if err != nil {
				t.Fatalf("GenerateExportOptions() error: %v", err)
			}
			if result.Destination != destination {
				t.Fatalf("result destination = %q, want %q", result.Destination, destination)
			}
			if got := isDirectUploadMode(outputPath); got != (destination == "upload") {
				t.Fatalf("isDirectUploadMode() = %t for destination %q", got, destination)
			}
		})
	}
}

func TestDefaultExportOptionsPathForArchive(t *testing.T) {
	got := DefaultExportOptionsPathForArchive(filepath.Join("artifacts", "Demo.xcarchive"))
	want := filepath.Join("artifacts", "Demo-ExportOptions.plist")
	if got != want {
		t.Fatalf("DefaultExportOptionsPathForArchive() = %q, want %q", got, want)
	}
	withSeparator := DefaultExportOptionsPathForArchive(filepath.Join("artifacts", "Demo.xcarchive") + string(filepath.Separator))
	if withSeparator != want {
		t.Fatalf("DefaultExportOptionsPathForArchive(trailing separator) = %q, want %q", withSeparator, want)
	}
}

func TestUniqueExportOptionsPathForArchive(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive") + string(filepath.Separator)
	normalizedArchivePath := filepath.Clean(archivePath)
	wantPrefix := strings.TrimSuffix(normalizedArchivePath, ".xcarchive") + "-ExportOptions-"
	seen := make(map[string]struct{})
	for range 32 {
		got, err := UniqueExportOptionsPathForArchive(archivePath)
		if err != nil {
			t.Fatalf("UniqueExportOptionsPathForArchive() error: %v", err)
		}
		if !strings.HasPrefix(got, wantPrefix) || !strings.HasSuffix(got, ".plist") {
			t.Fatalf("UniqueExportOptionsPathForArchive() = %q, want %q prefix and .plist suffix", got, wantPrefix)
		}
		if _, ok := seen[got]; ok {
			t.Fatalf("UniqueExportOptionsPathForArchive() returned duplicate path %q", got)
		}
		seen[got] = struct{}{}
		if strings.HasPrefix(got, normalizedArchivePath+string(filepath.Separator)) {
			t.Fatalf("generated export options path inside archive: %q", got)
		}
		if _, statErr := os.Lstat(got); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("generated path already exists: %q (%v)", got, statErr)
		}
	}
}

func TestGenerateExportOptions_RejectsMissingAndMalformedArchives(t *testing.T) {
	t.Run("missing archive info", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "Missing.xcarchive")
		if err := os.MkdirAll(archivePath, 0o755); err != nil {
			t.Fatal(err)
		}

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  filepath.Join(t.TempDir(), "ExportOptions.plist"),
		})
		if err == nil || !strings.Contains(err.Error(), "Info.plist") {
			t.Fatalf("expected missing archive Info.plist error, got %v", err)
		}
	})

	t.Run("malformed archive info", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "Malformed.xcarchive")
		if err := os.MkdirAll(archivePath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(archivePath, "Info.plist"), []byte("not a plist"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  filepath.Join(t.TempDir(), "ExportOptions.plist"),
		})
		if err == nil || !strings.Contains(err.Error(), "decode") {
			t.Fatalf("expected malformed archive error, got %v", err)
		}
	})

	t.Run("archive info symlink escapes archive", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		infoPath := filepath.Join(archivePath, "Info.plist")
		outsidePath := filepath.Join(t.TempDir(), "Info.plist")
		if err := os.Rename(infoPath, outsidePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsidePath, infoPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil {
			t.Fatal("expected archive Info.plist symlink escape error")
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unsafe archive created output: %v", statErr)
		}
	})

	t.Run("decodable but invalid archive info", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "Invalid.xcarchive")
		if err := os.MkdirAll(archivePath, 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := plist.Marshal(map[string]any{}, plist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(archivePath, "Info.plist"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err = GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil || !strings.Contains(err.Error(), "ApplicationProperties") {
			t.Fatalf("expected invalid archive metadata error, got %v", err)
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("invalid archive created output: %v", statErr)
		}
	})

	t.Run("missing archived app info", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		if err := os.Remove(filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")); err != nil {
			t.Fatal(err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil || !strings.Contains(err.Error(), "archived app Info.plist") {
			t.Fatalf("expected missing archived app metadata error, got %v", err)
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("invalid archive created output: %v", statErr)
		}
	})

	t.Run("archived app info symlink escapes app bundle", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		infoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
		outsidePath := filepath.Join(t.TempDir(), "Info.plist")
		if err := os.Rename(infoPath, outsidePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsidePath, infoPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil {
			t.Fatal("expected archived app Info.plist symlink escape error")
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unsafe archive created output: %v", statErr)
		}
	})

	t.Run("archived app bundle symlink escapes products", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		appPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app")
		outsidePath := filepath.Join(t.TempDir(), "Demo.app")
		if err := os.Rename(appPath, outsidePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsidePath, appPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil {
			t.Fatal("expected archived app bundle symlink escape error")
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unsafe archive created output: %v", statErr)
		}
	})

	t.Run("products symlink escapes archive", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		productsPath := filepath.Join(archivePath, "Products")
		outsidePath := filepath.Join(t.TempDir(), "Products")
		if err := os.Rename(productsPath, outsidePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsidePath, productsPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil {
			t.Fatal("expected archive Products symlink escape error")
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unsafe archive created output: %v", statErr)
		}
	})

	t.Run("unsupported archived app platform", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
		data, err := plist.Marshal(map[string]any{"CFBundleIdentifier": "com.example.demo"}, plist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(appInfoPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err = GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil || !strings.Contains(err.Error(), "supported platform marker") {
			t.Fatalf("expected unsupported archived app platform error, got %v", err)
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("invalid archive created output: %v", statErr)
		}
	})

	t.Run("mismatched archived app bundle identifier", func(t *testing.T) {
		archivePath := writeExportOptionsTestArchive(t, "TEAM123")
		appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
		data, err := plist.Marshal(map[string]any{
			"CFBundleIdentifier":         "com.example.other",
			"CFBundleSupportedPlatforms": []string{"iphoneos"},
			"DTPlatformName":             "iphoneos",
		}, plist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(appInfoPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

		_, err = GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
		})
		if err == nil || !strings.Contains(err.Error(), "does not match archive bundle identifier") {
			t.Fatalf("expected bundle identifier mismatch error, got %v", err)
		}
		if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("invalid archive created output: %v", statErr)
		}
	})

	for _, applicationPath := range []string{"../../Outside.app", "/tmp/Outside.app"} {
		t.Run("unsafe application path "+strings.ReplaceAll(applicationPath, "/", "_"), func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "Unsafe.xcarchive")
			if err := os.MkdirAll(archivePath, 0o755); err != nil {
				t.Fatal(err)
			}
			data, err := plist.Marshal(map[string]any{
				"ApplicationProperties": map[string]any{
					"ApplicationPath":    applicationPath,
					"CFBundleIdentifier": "com.example.demo",
				},
			}, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(archivePath, "Info.plist"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

			_, err = GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
				ArchivePath: archivePath,
				OutputPath:  outputPath,
			})
			if err == nil || !strings.Contains(err.Error(), "unsafe ApplicationPath") {
				t.Fatalf("expected unsafe ApplicationPath error, got %v", err)
			}
			if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("unsafe archive path created output: %v", statErr)
			}
		})
	}
}

func TestGenerateExportOptions_ExistingOutputRequiresOverwriteAndReplacesAtomically(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
	const original = "do not replace without overwrite"
	if err := os.WriteFile(outputPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
	})
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	if data, readErr := os.ReadFile(outputPath); readErr != nil || string(data) != original {
		t.Fatalf("output changed after refusal: data=%q err=%v", data, readErr)
	}

	result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
		Overwrite:   true,
	})
	if err != nil {
		t.Fatalf("GenerateExportOptions(overwrite) error: %v", err)
	}
	if !result.Overwritten {
		t.Fatal("replacement output must be reported as overwritten")
	}
	after, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("overwrite must atomically replace the destination, not mutate it in place")
	}
	if string(mustReadExportOptionsTestFile(t, outputPath)) == original {
		t.Fatal("overwrite left original output unchanged")
	}
}

func TestGenerateExportOptions_ManualSigningRejectsExistingOutputBeforeLookup(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
	const original = "do not inspect signing assets"
	if err := os.WriteFile(outputPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(context.Context, string, string, string) (manualExportOptions, error) {
		t.Fatal("manual signing lookup must not run for an unusable output destination")
		return manualExportOptions{}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		SigningStyle: "manual",
	})
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	if got := string(mustReadExportOptionsTestFile(t, outputPath)); got != original {
		t.Fatalf("output changed after refusal: %q", got)
	}
}

func TestGenerateExportOptions_FailedAtomicOverwritePreservesExistingOutput(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
	const original = "preserve on write failure"
	if err := os.WriteFile(outputPath, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}

	originalWriter := atomicWriteExportOptionsFileFn
	atomicWriteExportOptionsFileFn = func(path string, _ []byte, mode os.FileMode) error {
		if path != outputPath {
			t.Fatalf("atomic writer path = %q, want %q", path, outputPath)
		}
		if mode != 0o640 {
			t.Fatalf("atomic writer mode = %#o, want %#o", mode, os.FileMode(0o640))
		}
		return errors.New("injected atomic write failure")
	}
	t.Cleanup(func() { atomicWriteExportOptionsFileFn = originalWriter })

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
		Overwrite:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "injected atomic write failure") {
		t.Fatalf("expected injected write failure, got %v", err)
	}
	if got := string(mustReadExportOptionsTestFile(t, outputPath)); got != original {
		t.Fatalf("failed overwrite changed existing output: %q", got)
	}
}

func TestGenerateExportOptions_NewOutputDoesNotReplaceConcurrentFile(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
	const concurrent = "created after destination validation"

	originalWriter := atomicWriteNewExportOptionsFileFn
	atomicWriteNewExportOptionsFileFn = func(path string, data []byte, mode os.FileMode) error {
		if path != outputPath {
			t.Fatalf("atomic writer path = %q, want %q", path, outputPath)
		}
		if err := os.WriteFile(path, []byte(concurrent), 0o600); err != nil {
			return err
		}
		return atomicWriteNewExportOptionsFile(path, data, mode)
	}
	t.Cleanup(func() { atomicWriteNewExportOptionsFileFn = originalWriter })

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath: archivePath,
		OutputPath:  outputPath,
	})
	if err == nil {
		t.Fatal("expected concurrent destination error")
	}
	if got := string(mustReadExportOptionsTestFile(t, outputPath)); got != concurrent {
		t.Fatalf("concurrent output was replaced: %q", got)
	}
}

func TestGenerateExportOptions_RejectsDirectoryAndSymlinkDestinations(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")

	t.Run("directory", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
		if err := os.Mkdir(outputPath, 0o755); err != nil {
			t.Fatal(err)
		}

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
			Overwrite:   true,
		})
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("expected directory rejection, got %v", err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		targetPath := filepath.Join(root, "target.plist")
		const targetContents = "must remain untouched"
		if err := os.WriteFile(targetPath, []byte(targetContents), 0o644); err != nil {
			t.Fatal(err)
		}
		outputPath := filepath.Join(root, "ExportOptions.plist")
		if err := os.Symlink(targetPath, outputPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath: archivePath,
			OutputPath:  outputPath,
			Overwrite:   true,
		})
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink rejection, got %v", err)
		}
		if got := string(mustReadExportOptionsTestFile(t, targetPath)); got != targetContents {
			t.Fatalf("symlink target changed: %q", got)
		}
	})
}

func TestGenerateExportOptions_RejectsInvalidDestinationAndSigningStyleBeforeWriting(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")

	for _, tc := range []struct {
		name      string
		errorHint string
		opts      ExportOptionsGenerateOptions
	}{
		{
			name:      "destination",
			errorHint: "destination",
			opts:      ExportOptionsGenerateOptions{Destination: "invalid", SigningStyle: "automatic"},
		},
		{
			name:      "signing style",
			errorHint: "signing",
			opts:      ExportOptionsGenerateOptions{Destination: "export", SigningStyle: "invalid"},
		},
		{
			name:      "method",
			errorHint: "method",
			opts:      ExportOptionsGenerateOptions{Method: "ad-hoc", Destination: "export", SigningStyle: "automatic"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
			tc.opts.ArchivePath = archivePath
			tc.opts.OutputPath = outputPath
			_, err := GenerateExportOptions(context.Background(), tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.errorHint) {
				t.Fatalf("expected invalid %s error, got %v", tc.name, err)
			}
			if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("invalid %s created output: %v", tc.name, statErr)
			}
		})
	}
}

func TestGenerateExportOptions_ManualGeneratorOutputIsValidatedBeforeReplacingOutput(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
	const original = "preserve this file"
	if err := os.WriteFile(outputPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(_ context.Context, _ string, teamID, _ string) (manualExportOptions, error) {
		return manualExportOptions{
			TeamID:               teamID,
			SigningCertificate:   "Apple Distribution",
			ProvisioningProfiles: map[string]string{"": "profile-uuid"},
		}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		SigningStyle: "manual",
		Overwrite:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Fatalf("expected invalid manual generator output error, got %v", err)
	}
	if got := string(mustReadExportOptionsTestFile(t, outputPath)); got != original {
		t.Fatalf("manual generator validation replaced output: %q", got)
	}
}

func TestGenerateExportOptions_ManualSigningPreflightsOutputParentBeforeLookup(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")
	preflightErr := errors.New("output parent is not writable")

	originalPreflight := preflightExportOptionsParentFn
	preflightExportOptionsParentFn = func(path string) error {
		if path != outputPath {
			t.Fatalf("preflight path = %q, want %q", path, outputPath)
		}
		return preflightErr
	}
	t.Cleanup(func() { preflightExportOptionsParentFn = originalPreflight })

	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(context.Context, string, string, string) (manualExportOptions, error) {
		t.Fatal("manual signing lookup ran before output-parent preflight")
		return manualExportOptions{}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		SigningStyle: "manual",
	})
	if !errors.Is(err, preflightErr) {
		t.Fatalf("expected output-parent preflight error, got %v", err)
	}
	if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("parent preflight failure created output: %v", statErr)
	}
}

func TestPreflightExportOptionsParentCreatesParentAndRemovesProbe(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "nested", "output")
	outputPath := filepath.Join(parent, "ExportOptions.plist")

	if err := preflightExportOptionsParent(outputPath); err != nil {
		t.Fatalf("preflightExportOptionsParent() error: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("read preflight parent: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("preflight left temporary entries: %#v", entries)
	}
}

func TestGenerateExportOptions_ManualSigningWritesResolvedCertificateAndProfiles(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(_ context.Context, _ string, teamID, method string) (manualExportOptions, error) {
		if method != exportOptionsMethodAppStoreConnect {
			t.Fatalf("manual generator method = %q, want app-store-connect", method)
		}
		if teamID != "TEAM123" {
			t.Fatalf("manual generator team ID = %q, want archive team", teamID)
		}
		return manualExportOptions{
			TeamID:                     teamID,
			SigningCertificate:         "Apple Distribution: Example (TEAM123)",
			ICloudContainerEnvironment: "Production",
			ProvisioningProfiles: map[string]string{
				"com.example.demo":        "main-profile-uuid",
				"com.example.demo.widget": "widget-profile-uuid",
			},
		}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	result, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		SigningStyle: "manual",
	})
	if err != nil {
		t.Fatalf("GenerateExportOptions() error: %v", err)
	}
	if result.SigningCertificate != "Apple Distribution: Example (TEAM123)" {
		t.Fatalf("unexpected signing certificate: %q", result.SigningCertificate)
	}
	if result.TeamID != "TEAM123" {
		t.Fatalf("result team ID = %q, want generator team", result.TeamID)
	}
	if result.ProvisioningProfiles["com.example.demo.widget"] != "widget-profile-uuid" {
		t.Fatalf("unexpected provisioning profiles: %#v", result.ProvisioningProfiles)
	}

	payload := readExportOptionsTestPlist(t, outputPath)
	if payload["signingStyle"] != "manual" {
		t.Fatalf("plist signingStyle = %#v", payload["signingStyle"])
	}
	if payload["signingCertificate"] != "Apple Distribution: Example (TEAM123)" {
		t.Fatalf("plist signingCertificate = %#v", payload["signingCertificate"])
	}
	if payload["iCloudContainerEnvironment"] != "Production" {
		t.Fatalf("plist iCloudContainerEnvironment = %#v", payload["iCloudContainerEnvironment"])
	}
	profiles, ok := payload["provisioningProfiles"].(map[string]any)
	if !ok || profiles["com.example.demo"] != "main-profile-uuid" || profiles["com.example.demo.widget"] != "widget-profile-uuid" {
		t.Fatalf("unexpected plist provisioning profiles: %#v", payload["provisioningProfiles"])
	}
}

func TestGenerateExportOptions_ManualSigningRejectsGeneratorTeamMismatch(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "ARCHIVE123")
	outputPath := filepath.Join(t.TempDir(), "ExportOptions.plist")

	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(_ context.Context, _ string, teamID, _ string) (manualExportOptions, error) {
		if teamID != "ARCHIVE123" {
			t.Fatalf("manual generator team ID = %q, want archive team", teamID)
		}
		return manualExportOptions{
			TeamID:             "OTHER456",
			SigningCertificate: "Apple Distribution: Example (OTHER456)",
			ProvisioningProfiles: map[string]string{
				"com.example.demo": "main-profile-uuid",
			},
		}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	_, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
		ArchivePath:  archivePath,
		OutputPath:   outputPath,
		SigningStyle: "manual",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected manual team mismatch error, got %v", err)
	}
	if _, statErr := os.Stat(outputPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("team mismatch created output: %v", statErr)
	}
}

func TestGenerateExportOptions_ManualOutputIsDeterministic(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	originalGenerator := manualExportOptionsGeneratorFn
	manualExportOptionsGeneratorFn = func(_ context.Context, _ string, teamID, _ string) (manualExportOptions, error) {
		return manualExportOptions{
			TeamID:             teamID,
			SigningCertificate: "Apple Distribution: Example (TEAM123)",
			ProvisioningProfiles: map[string]string{
				"com.example.demo.widget": "widget-profile-uuid",
				"com.example.demo":        "main-profile-uuid",
			},
		}, nil
	}
	t.Cleanup(func() { manualExportOptionsGeneratorFn = originalGenerator })

	firstPath := filepath.Join(t.TempDir(), "First.plist")
	secondPath := filepath.Join(t.TempDir(), "Second.plist")
	for _, outputPath := range []string{firstPath, secondPath} {
		if _, err := GenerateExportOptions(context.Background(), ExportOptionsGenerateOptions{
			ArchivePath:  archivePath,
			OutputPath:   outputPath,
			SigningStyle: "manual",
		}); err != nil {
			t.Fatalf("GenerateExportOptions() error: %v", err)
		}
	}
	first := mustReadExportOptionsTestFile(t, firstPath)
	second := mustReadExportOptionsTestFile(t, secondPath)
	if string(first) != string(second) {
		t.Fatalf("manual export options output was not deterministic\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func writeExportOptionsTestArchive(t *testing.T, teamID string) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"ApplicationProperties": map[string]any{
			"ApplicationPath":            "Applications/Demo.app",
			"CFBundleIdentifier":         "com.example.demo",
			"CFBundleShortVersionString": "1.2.3",
			"CFBundleVersion":            "42",
			"Team":                       teamID,
		},
	}
	data, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "Info.plist"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	appBundlePath := filepath.Join(archivePath, "Products", "Applications", "Demo.app")
	if err := os.MkdirAll(appBundlePath, 0o755); err != nil {
		t.Fatal(err)
	}
	appData, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleShortVersionString": "1.2.3",
		"CFBundleVersion":            "42",
		"CFBundleSupportedPlatforms": []string{"iphoneos"},
		"DTPlatformName":             "iphoneos",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appBundlePath, "Info.plist"), appData, 0o644); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func readExportOptionsTestPlist(t *testing.T, path string) map[string]any {
	t.Helper()

	data := mustReadExportOptionsTestFile(t, path)
	payload := map[string]any{}
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode generated plist: %v", err)
	}
	return payload
}

func mustReadExportOptionsTestFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
