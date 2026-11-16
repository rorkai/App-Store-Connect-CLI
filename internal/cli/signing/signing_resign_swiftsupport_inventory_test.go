package signing

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestSigningResignSwiftSupportInventoryBindsFinalBytesAndModes(t *testing.T) {
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}

	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, root string)
		match  bool
	}{
		{name: "unchanged", match: true},
		{
			name: "added",
			mutate: func(t *testing.T, root string) {
				writeSigningResignSwiftSupportFixture(t, root, "libswiftUI.dylib", []byte("second runtime"), 0o755)
			},
		},
		{
			name: "dropped",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "SwiftSupport", "iphoneos", "libswiftCore.dylib")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "bytes replaced",
			mutate: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "SwiftSupport", "iphoneos", "libswiftCore.dylib"), []byte("replacement runtime"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "renamed",
			mutate: func(t *testing.T, root string) {
				oldPath := filepath.Join(root, "SwiftSupport", "iphoneos", "libswiftCore.dylib")
				if err := os.Rename(oldPath, filepath.Join(filepath.Dir(oldPath), "libswiftCore-renamed.dylib")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "mode changed",
			mutate: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, "SwiftSupport", "iphoneos", "libswiftCore.dylib"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeSigningResignSwiftSupportFixture(t, root, "libswiftCore.dylib", []byte("canonical runtime"), 0o755)
			if err := validateSigningResignSwiftSupport(context.Background(), root); err != nil {
				t.Fatal(err)
			}
			expected, err := captureSigningResignSwiftSupportInventory(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if test.mutate != nil {
				test.mutate(t, root)
			}
			actual, err := captureSigningResignSwiftSupportInventory(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			err = validateSigningResignSwiftSupportInventory(actual, expected)
			if (err == nil) != test.match {
				t.Fatalf("validateSigningResignSwiftSupportInventory() error = %v, want match=%t", err, test.match)
			}
		})
	}
}

func writeSigningResignSwiftSupportFixture(t *testing.T, root, name string, data []byte, mode os.FileMode) {
	t.Helper()
	pathValue := filepath.Join(root, "SwiftSupport", "iphoneos", name)
	if err := os.MkdirAll(filepath.Dir(pathValue), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathValue, data, mode); err != nil {
		t.Fatal(err)
	}
}

func TestSigningResignSwiftSupportInventoryHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	writeSigningResignSwiftSupportFixture(t, root, "libswiftCore.dylib", []byte("canonical runtime"), 0o755)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := captureSigningResignSwiftSupportInventory(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("captureSigningResignSwiftSupportInventory() error = %v, want cancellation", err)
	}
}

func TestVerifyPackedSigningResignIPAProjectsSwiftSupportMismatchAsClosedVerificationError(t *testing.T) {
	profileData := []byte("replacement profile")
	packedRuntime := []byte("1234567890abcdeg")
	expectedRuntime := []byte("1234567890abcdef")
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	packedData := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: info},
		{name: "Payload/App.app/App", data: executable, mode: 0o755},
		{name: "Payload/App.app/embedded.mobileprovision", data: profileData},
		{name: "SwiftSupport/iphoneos/libswiftCore.dylib", data: packedRuntime, mode: 0o755},
	})
	temporary := t.TempDir()
	packedPath := filepath.Join(temporary, "packed.ipa")
	if err := os.WriteFile(packedPath, packedData, 0o600); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(temporary)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	original := signingResignPreparedTree{
		Archive: signingResignArchive{
			MainPath: "Payload/App.app",
			Targets: []signingResignTarget{{
				Kind:         "application",
				RelativePath: "Payload/App.app",
				BundleID:     "com.example.app",
				Executable:   "App",
				ProfileMode:  0o644,
				Profile: signingResignProfile{
					Data:   profileData,
					SHA256: signingResignSHA256(profileData),
				},
			}},
		},
		SwiftSupport: []signingResignSwiftSupportEntry{{
			RelativePath: "SwiftSupport/iphoneos/libswiftCore.dylib",
			SizeBytes:    int64(len(expectedRuntime)),
			SHA256:       signingResignSHA256(expectedRuntime),
			Mode:         0o755,
		}},
	}
	fileInfo, err := os.Stat(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyPackedSigningResignIPA(context.Background(), packedPath, fileInfo.Size(), stageRoot, filepath.Join(temporary, "tree"), original, "TEAM", strings.Repeat("A", 64))
	if err == nil {
		t.Fatal("verifyPackedSigningResignIPA() returned nil for changed SwiftSupport inventory")
	}
	var operational *signingResignOperationalError
	if !errors.As(err, &operational) {
		t.Fatalf("verifyPackedSigningResignIPA() error = %v, want closed operational error", err)
	}
	if operational.stage != signingResignStageVerification || operational.code != signingResignCodeVerification {
		t.Fatalf("operational stage/code = %v/%v, want verification/verification", operational.stage, operational.code)
	}
	if got := err.Error(); got != "signing resign failed during verification (verification)" {
		t.Fatalf("public verification error = %q, want stable stage/code", got)
	}
	if strings.Contains(err.Error(), "libswiftCore.dylib") || strings.Contains(err.Error(), signingResignSHA256(packedRuntime)) || strings.Contains(err.Error(), signingResignSHA256(expectedRuntime)) {
		t.Fatalf("public verification error leaked inventory details: %q", err)
	}

	if runtime.GOOS != "darwin" {
		t.Skip("command projection is macOS-only")
	}
	originalExecute := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = originalExecute })
	executeSigningResignFn = func(context.Context, *shared.Deps, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{
			Output: signingResignArtifactResult{Path: filepath.Join(temporary, "success-receipt.ipa")},
		}, err
	}
	d, _, _ := newSigningTestDeps()
	command := newSigningResignCommand(d)
	if parseErr := command.FlagSet.Parse([]string{
		"--ipa", filepath.Join(temporary, "private-input.ipa"),
		"--output", filepath.Join(temporary, "private-output.ipa"),
		"--identity", filepath.Join(temporary, "private-identity.p12"),
		"--profiles-manifest", filepath.Join(temporary, "private-profiles.json"),
	}); parseErr != nil {
		t.Fatal(parseErr)
	}
	publicErr := command.Exec(context.Background(), nil)
	if publicErr == nil || errors.Is(publicErr, flag.ErrHelp) {
		t.Fatalf("SigningResignCommand().Exec() error = %v, want operational exit 1", publicErr)
	}
	if got := publicErr.Error(); got != "signing resign: signing resign failed during verification (verification)" {
		t.Fatalf("public command error = %q, want stable verification stage/code", got)
	}
	formatted := errfmt.FormatStderr(publicErr)
	if !strings.Contains(formatted, "signing resign failed during verification (verification)") {
		t.Fatalf("formatted stderr = %q, want stable verification stage/code", formatted)
	}
	for _, secret := range []string{"libswiftCore.dylib", signingResignSHA256(packedRuntime), signingResignSHA256(expectedRuntime), filepath.Join(temporary, "private-input.ipa"), filepath.Join(temporary, "private-output.ipa")} {
		if strings.Contains(publicErr.Error(), secret) || strings.Contains(formatted, secret) {
			t.Fatalf("public command output leaked %q: error=%q stderr=%q", secret, publicErr, formatted)
		}
	}
	if _, statErr := os.Stat(filepath.Join(temporary, "success-receipt.ipa")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed command left a success receipt: stat error = %v", statErr)
	}
}

func TestSigningResignPreservedExternalCodeIncludesWatchKitSupport(t *testing.T) {
	root := t.TempDir()
	if !isSigningResignPreservedExternalCodePath(root, filepath.Join(root, "WatchKitSupport2", "WK")) {
		t.Fatal("WatchKitSupport2/WK must be treated as preserved distribution-side code")
	}
	if isSigningResignPreservedExternalCodePath(root, filepath.Join(root, "WatchKitSupport2", "Other")) {
		t.Fatal("only the WK binary is preserved under WatchKitSupport2")
	}
	if isSigningResignPreservedExternalCodePath(root, filepath.Join(root, "WatchKitSupport2", "nested", "WK")) {
		t.Fatal("nested WatchKitSupport2 entries are not preserved")
	}
}

func TestValidateSigningResignWatchKitSupportEnforcesShapeAndProvenance(t *testing.T) {
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	var verified []string
	runSigningResignToolFn = func(_ context.Context, _ string, args ...string) (signingResignToolOutput, error) {
		verified = append(verified, args[len(args)-1])
		return signingResignToolOutput{}, nil
	}
	root := t.TempDir()
	if err := validateSigningResignWatchKitSupport(context.Background(), root); err != nil {
		t.Fatalf("validateSigningResignWatchKitSupport() error = %v, want absent directory accepted", err)
	}
	wkDir := filepath.Join(root, "WatchKitSupport2")
	if err := os.MkdirAll(wkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wkDir, "WK"), []byte("wk binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignWatchKitSupport(context.Background(), root); err != nil {
		t.Fatalf("validateSigningResignWatchKitSupport() error = %v, want standard WK layout accepted", err)
	}
	if len(verified) != 1 || !strings.Contains(filepath.Base(verified[0]), ".signing-resign-verify-") {
		t.Fatalf("verified = %v, want provenance verification of the WK binary", verified)
	}
	if err := os.WriteFile(filepath.Join(wkDir, "extra"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignWatchKitSupport(context.Background(), root); err == nil {
		t.Fatal("validateSigningResignWatchKitSupport() accepted an unexpected extra entry")
	}
}

func TestValidateSigningResignWatchKitSupportRequiresOwnerExecute(t *testing.T) {
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	root := t.TempDir()
	wkDir := filepath.Join(root, "WatchKitSupport2")
	if err := os.MkdirAll(wkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wkDir, "WK"), []byte("wk binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignWatchKitSupport(context.Background(), root); err == nil || !strings.Contains(err.Error(), "owner-execute") {
		t.Fatalf("validateSigningResignWatchKitSupport() error = %v, want owner-execute rejection", err)
	}
}

func TestCaptureSigningResignPreservedInventoryIncludesWatchKit(t *testing.T) {
	root := t.TempDir()
	inventory, err := captureSigningResignPreservedInventory(context.Background(), root)
	if err != nil || inventory != nil {
		t.Fatalf("captureSigningResignPreservedInventory() = %v, %v, want empty for a plain tree", inventory, err)
	}
	wkDir := filepath.Join(root, "WatchKitSupport2")
	if err := os.MkdirAll(wkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("wk binary")
	if err := os.WriteFile(filepath.Join(wkDir, "WK"), contents, 0o755); err != nil {
		t.Fatal(err)
	}
	inventory, err = captureSigningResignPreservedInventory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 1 {
		t.Fatalf("inventory = %v, want the WK binary captured", inventory)
	}
	entry := inventory[0]
	if entry.RelativePath != "WatchKitSupport2/WK" || entry.SizeBytes != int64(len(contents)) ||
		entry.Mode != 0o755 || entry.SHA256 != signingResignSHA256(contents) {
		t.Fatalf("inventory entry = %+v, want exact WK path, size, mode, and digest", entry)
	}
}

func TestCaptureSigningResignWatchKitInventoryRejectsMissingEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "WatchKitSupport2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := captureSigningResignPreservedInventory(context.Background(), root); err == nil || !strings.Contains(err.Error(), "inspect WatchKitSupport2 entry") {
		t.Fatalf("captureSigningResignPreservedInventory() error = %v, want missing-entry failure", err)
	}
}

func TestSigningResignCombinedPreservedOperationsShareOneStagingRoot(t *testing.T) {
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "SwiftSupport", "iphoneos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SwiftSupport", "iphoneos", "libswiftCore.dylib"), []byte("swift"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "WatchKitSupport2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "WatchKitSupport2", "WK"), []byte("watch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignPreservedExternalDirectories(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	inventory, err := captureSigningResignPreservedInventory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 2 || inventory[0].RelativePath != "SwiftSupport/iphoneos/libswiftCore.dylib" || inventory[1].RelativePath != "WatchKitSupport2/WK" {
		t.Fatalf("inventory = %+v, want both preserved trees in sorted order", inventory)
	}
}

func TestSigningResignPreservedDirectoriesRejectSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "SwiftSupport")); err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignPreservedExternalDirectories(context.Background(), root); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("validate error = %v", err)
	}
	if _, err := captureSigningResignPreservedInventory(context.Background(), root); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("capture error = %v", err)
	}
}
