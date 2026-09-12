//go:build darwin

package xcode

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/certificateutil"
	legacyexportoptions "github.com/bitrise-io/go-xcode/exportoptions"
	"github.com/bitrise-io/go-xcode/v2/exportoptionsgenerator"
	"github.com/bitrise-io/go-xcode/v2/plistutil"
	"github.com/bitrise-io/go-xcode/v2/profileutil"
	"github.com/fullsailor/pkcs7"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"howett.net/plist"
)

func TestManualExportOptionsResolverMethodUsesLegacyAdHocProfileClassification(t *testing.T) {
	if got := manualExportOptionsResolverMethod(exportOptionsMethodReleaseTesting); got != legacyexportoptions.MethodAdHoc {
		t.Fatalf("release-testing resolver method = %q, want %q", got, legacyexportoptions.MethodAdHoc)
	}
	if got := manualExportOptionsResolverMethod(exportOptionsMethodAppStoreConnect); got != legacyexportoptions.MethodAppStoreConnect {
		t.Fatalf("app-store-connect resolver method = %q, want %q", got, legacyexportoptions.MethodAppStoreConnect)
	}
}

func TestManualExportOptionsResolverOptionsUseProductionCloudKitForReleaseTesting(t *testing.T) {
	releaseTesting := manualExportOptionsResolverOptions("TEAM123", exportOptionsMethodReleaseTesting)
	if releaseTesting.TeamID != "TEAM123" || releaseTesting.ContainerEnvironment != "Production" {
		t.Fatalf("release-testing resolver options = %#v, want team and Production CloudKit", releaseTesting)
	}
	appStore := manualExportOptionsResolverOptions("TEAM123", exportOptionsMethodAppStoreConnect)
	if appStore.TeamID != "TEAM123" || appStore.ContainerEnvironment != "" {
		t.Fatalf("app-store-connect resolver options = %#v, want team and implied CloudKit environment", appStore)
	}
}

func TestCaptureBitriseStdout(t *testing.T) {
	wantErr := errors.New("generator sentinel")
	captured, err := captureBitriseStdout(func() error {
		fmt.Fprint(os.Stdout, "Checking if project uses CloudKit")
		log.NewLogger().Warnf("profile diagnostic")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("captureBitriseStdout() error = %v, want %v", err, wantErr)
	}
	if !strings.Contains(captured, "Checking if project uses CloudKit") || !strings.Contains(captured, "profile diagnostic") {
		t.Fatalf("captureBitriseStdout() output = %q", captured)
	}
}

func TestGenerateManualExportOptionsCapturesArchiveReaderStdout(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	wantErr := errors.New("archive reader sentinel")
	originalReader := readArchiveExportInfoFn
	originalStdout := os.Stdout
	captured := false
	readArchiveExportInfoFn = func(string) (exportoptionsgenerator.ArchiveInfo, error) {
		captured = os.Stdout != originalStdout
		fmt.Fprint(os.Stdout, "Fetching entitlements from executable")
		return exportoptionsgenerator.ArchiveInfo{}, wantErr
	}
	t.Cleanup(func() { readArchiveExportInfoFn = originalReader })

	_, err := generateManualExportOptions(t.Context(), archivePath, "TEAM123", exportOptionsMethodReleaseTesting)
	if !errors.Is(err, wantErr) {
		t.Fatalf("generateManualExportOptions() error = %v, want %v", err, wantErr)
	}
	if !captured {
		t.Fatal("archive reader ran before Bitrise stdout capture was installed")
	}
}

func TestGenerateManualExportOptionsSupportsMacOSAppStore(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	originalReader := readMacArchiveExportInfoFn
	originalProfiles := installedMacProvisioningProfileInfosFn
	originalCertificates := installedCodesignIdentityInfosFn
	originalInstallerCertificates := installedInstallerIdentityInfosFn
	readMacArchiveExportInfoFn = func(context.Context, string) (macArchiveExportInfo, error) {
		return macArchiveExportInfo{
			ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
				AppBundleID: "com.example.demo",
				EntitlementsByBundleID: map[string]plistutil.PlistData{
					"com.example.demo":       {"com.apple.developer.team-identifier": "TEAM123"},
					"com.example.demo.share": {"com.apple.developer.team-identifier": "TEAM123"},
				},
			},
			EmbeddedProfiles: map[string]profileutil.ProvisioningProfileInfoModel{
				"com.example.demo":       {UUID: "MAIN-UUID", BundleID: "com.example.demo", TeamID: "TEAM123"},
				"com.example.demo.share": {UUID: "EMBEDDED-SHARE-UUID", BundleID: "com.example.demo.share", TeamID: "TEAM123"},
			},
		}, nil
	}
	installedMacProvisioningProfileInfosFn = func(context.Context) ([]profileutil.ProvisioningProfileInfoModel, error) {
		return []profileutil.ProvisioningProfileInfoModel{
			{
				UUID: "MAIN-UUID", Name: "Main Mac Profile", BundleID: "com.example.demo", TeamID: "TEAM123",
				Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
				ExpirationDate: time.Now().Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT-SERIAL"}},
				Entitlements: plistutil.PlistData{"com.apple.developer.team-identifier": "TEAM123"},
			},
			{
				UUID: "SHARE-UUID", Name: "Share Mac Profile", BundleID: "com.example.demo.*", TeamID: "TEAM123",
				Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
				ExpirationDate: time.Now().Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT-SERIAL"}},
				Entitlements: plistutil.PlistData{"com.apple.developer.team-identifier": "TEAM123"},
			},
		}, nil
	}
	installedCodesignIdentityInfosFn = func(context.Context) ([]certificateutil.CertificateInfoModel, error) {
		return []certificateutil.CertificateInfoModel{{CommonName: "Mac App Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "CERT-SERIAL"}}, nil
	}
	installedInstallerIdentityInfosFn = func(context.Context) ([]certificateutil.CertificateInfoModel, error) {
		return []certificateutil.CertificateInfoModel{{CommonName: "Mac Installer Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "INSTALLER-SERIAL"}}, nil
	}
	t.Cleanup(func() {
		readMacArchiveExportInfoFn = originalReader
		installedMacProvisioningProfileInfosFn = originalProfiles
		installedCodesignIdentityInfosFn = originalCertificates
		installedInstallerIdentityInfosFn = originalInstallerCertificates
	})

	manual, err := generateManualExportOptions(t.Context(), archivePath, "TEAM123", exportOptionsMethodAppStoreConnect)
	if err != nil {
		t.Fatalf("generateManualExportOptions() error: %v", err)
	}
	if manual.SigningCertificate != "Mac App Distribution: Example (TEAM123)" {
		t.Fatalf("signing certificate = %q", manual.SigningCertificate)
	}
	if manual.InstallerSigningCertificate != "Mac Installer Distribution: Example (TEAM123)" {
		t.Fatalf("installer signing certificate = %q", manual.InstallerSigningCertificate)
	}
	if got := manual.ProvisioningProfiles["com.example.demo"]; got != "MAIN-UUID" {
		t.Fatalf("main profile = %q", got)
	}
	if got := manual.ProvisioningProfiles["com.example.demo.share"]; got != "SHARE-UUID" {
		t.Fatalf("extension profile = %q", got)
	}
}

func TestGenerateManualExportOptionsRejectsMacOSReleaseTesting(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	_, err := generateManualExportOptions(t.Context(), archivePath, "TEAM123", exportOptionsMethodReleaseTesting)
	if err == nil || !strings.Contains(err.Error(), "macOS archives only supports method") || !strings.Contains(err.Error(), exportOptionsMethodAppStoreConnect) {
		t.Fatalf("expected macOS method rejection, got %v", err)
	}
}

func TestReadArchiveExportInfoSupportsMacOSArchive(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	binDir := t.TempDir()
	codesignPath := filepath.Join(binDir, "codesign")
	pathCanary := filepath.Join(t.TempDir(), "ambient-codesign-ran")
	codesignScript := []byte("#!/bin/sh\nprintf invoked > \"$ASC_PATH_CANARY\"\n")
	if err := os.WriteFile(codesignPath, codesignScript, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ASC_PATH_CANARY", pathCanary)
	originalCodesignCommand := macCodesignCommandContextFn
	codesignCalls := 0
	macCodesignCommandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "/usr/bin/codesign" {
			t.Fatalf("codesign path = %q, want /usr/bin/codesign", name)
		}
		codesignCalls++
		return exec.CommandContext(ctx, "/usr/bin/true")
	}
	t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

	info, err := readArchiveExportInfo(archivePath)
	if err != nil {
		t.Fatalf("readArchiveExportInfo() error: %v", err)
	}
	if info.AppBundleID != "com.example.demo" {
		t.Fatalf("app bundle ID = %q", info.AppBundleID)
	}
	if len(info.EntitlementsByBundleID) != 10 {
		t.Fatalf("entitlements = %#v, want every executable bundle in the archive", info.EntitlementsByBundleID)
	}
	for _, bundleID := range []string{
		"com.example.demo.helper",
		"com.example.demo.login",
		"com.example.demo.network-extension",
		"com.example.demo.driver-extension",
		"com.example.demo.bundle",
		"com.example.demo.plugin",
		"com.example.demo.quicklook",
		"com.example.demo.metadata-importer",
	} {
		if _, ok := info.EntitlementsByBundleID[bundleID]; !ok {
			t.Fatalf("nested bundle %q is absent from entitlements: %#v", bundleID, info.EntitlementsByBundleID)
		}
	}
	if _, err := os.Stat(pathCanary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ambient PATH codesign was invoked: %v", err)
	}
	if codesignCalls != 10 {
		t.Fatalf("system codesign invocations = %d, want 10", codesignCalls)
	}
}

func TestReadMacExecutableEntitlementsUsesImmutableSnapshotAndSystemCodesign(t *testing.T) {
	directory := t.TempDir()
	executablePath := filepath.Join(directory, "Demo")
	if err := os.WriteFile(executablePath, []byte("original executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Open(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	defer executable.Close()

	originalCodesignCommand := macCodesignCommandContextFn
	var snapshotDirectory string
	macCodesignCommandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "/usr/bin/codesign" {
			t.Fatalf("codesign path = %q, want /usr/bin/codesign", name)
		}
		if len(args) != 4 || args[0] != "-d" || args[1] != "--entitlements" || args[2] != ":-" {
			t.Fatalf("codesign args = %#v", args)
		}
		snapshotPath := args[3]
		snapshotDirectory = filepath.Dir(snapshotPath)
		if snapshotPath == executablePath || strings.HasPrefix(snapshotPath, directory+string(filepath.Separator)) {
			t.Fatalf("codesign input = %q, want private snapshot", snapshotPath)
		}
		if err := os.Rename(executablePath, executablePath+".original"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(executablePath, []byte("replacement executable"), 0o755); err != nil {
			t.Fatal(err)
		}
		snapshot, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(snapshot) != "original executable" {
			t.Fatalf("snapshot = %q, want original executable", snapshot)
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", `printf '%s' '<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>com.apple.developer.team-identifier</key><string>TEAM123</string></dict></plist>'`)
	}
	t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

	entitlements, err := readMacExecutableEntitlements(t.Context(), executable)
	if err != nil {
		t.Fatalf("readMacExecutableEntitlements() error: %v", err)
	}
	if entitlements["com.apple.developer.team-identifier"] != "TEAM123" {
		t.Fatalf("entitlements = %#v", entitlements)
	}
	if _, err := os.Stat(snapshotDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot directory remains after codesign: %v", err)
	}
}

func TestReadMacExecutableEntitlementsPreservesCancellation(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "Demo")
	if err := os.WriteFile(executablePath, []byte("executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Open(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	defer executable.Close()

	originalCodesignCommand := macCodesignCommandContextFn
	macCodesignCommandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 10")
	}
	t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err = readMacExecutableEntitlements(ctx, executable)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readMacExecutableEntitlements() error = %v, want context deadline exceeded", err)
	}
}

func TestReadMacArchiveExportInfoRejectsSymlinkedNestedBundle(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	appPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app")
	outside := filepath.Join(t.TempDir(), "Outside.xpc")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(appPath, "Contents", "XPCServices", "Linked.xpc")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	originalCodesignCommand := macCodesignCommandContextFn
	macCodesignCommandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/usr/bin/true")
	}
	t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

	_, err := readMacArchiveExportInfo(t.Context(), archivePath)
	if err == nil || !strings.Contains(err.Error(), "symlinked macOS executable bundle") {
		t.Fatalf("readMacArchiveExportInfo() error = %v, want symlink rejection", err)
	}
}

func TestReadMacArchiveExportInfoRejectsOversizedMetadataAndProfile(t *testing.T) {
	for _, test := range []struct {
		name string
		path func(string) string
		size int
	}{
		{
			name: "archive Info.plist",
			path: func(archivePath string) string { return filepath.Join(archivePath, "Info.plist") },
			size: infoplist.MaxBytes + 1,
		},
		{
			name: "application Info.plist",
			path: func(archivePath string) string {
				return filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Contents", "Info.plist")
			},
			size: infoplist.MaxBytes + 1,
		},
		{
			name: "embedded profile",
			path: func(archivePath string) string {
				return filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Contents", "embedded.provisionprofile")
			},
			size: int(macEmbeddedProfileMaxBytes) + 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			archivePath := writeMacExportOptionsTestArchive(t)
			if err := os.WriteFile(test.path(archivePath), make([]byte, test.size), 0o644); err != nil {
				t.Fatal(err)
			}
			originalCodesignCommand := macCodesignCommandContextFn
			macCodesignCommandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
				return exec.CommandContext(ctx, "/usr/bin/true")
			}
			t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

			_, err := readMacArchiveExportInfo(t.Context(), archivePath)
			if err == nil || !strings.Contains(err.Error(), "size limit") {
				t.Fatalf("readMacArchiveExportInfo() error = %v, want size limit", err)
			}
		})
	}
}

func TestReadMacArchiveExportInfoHonorsCanceledContext(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	originalCodesignCommand := macCodesignCommandContextFn
	macCodesignCommandContextFn = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("codesign command was created after cancellation")
		return nil
	}
	t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

	if _, err := readMacArchiveExportInfo(ctx, archivePath); !errors.Is(err, context.Canceled) {
		t.Fatalf("readMacArchiveExportInfo() error = %v, want context.Canceled", err)
	}
}

func TestDiscoverNestedMacBundlePathsHonorsCanceledContext(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	archiveRoot, err := rootfs.New(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveRoot.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = discoverNestedMacBundlePaths(ctx, archiveRoot, "Products/Applications/Demo.app")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("discoverNestedMacBundlePaths() error = %v, want context.Canceled", err)
	}
}

func TestReadMacArchiveExportInfoRejectsArchiveRootSwap(t *testing.T) {
	archivePath := writeMacExportOptionsTestArchive(t)
	outsideArchive := writeMacExportOptionsTestArchive(t)
	originalHook := afterMacArchiveInitialLstatFn
	afterMacArchiveInitialLstatFn = func() {
		if err := os.Rename(archivePath, archivePath+".original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideArchive, archivePath); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { afterMacArchiveInitialLstatFn = originalHook })
	originalCodesignCommand := macCodesignCommandContextFn
	macCodesignCommandContextFn = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("codesign command was created after archive root replacement")
		return nil
	}
	t.Cleanup(func() { macCodesignCommandContextFn = originalCodesignCommand })

	_, err := readMacArchiveExportInfo(t.Context(), archivePath)
	if err == nil || !strings.Contains(err.Error(), "same non-symlinked directory") {
		t.Fatalf("readMacArchiveExportInfo() error = %v, want archive root identity rejection", err)
	}
}

func TestInstalledMacProvisioningProfileInfosUsesBoundedNoFollowInventory(t *testing.T) {
	t.Run("reads both native profile directories", func(t *testing.T) {
		home := t.TempDir()
		modern := filepath.Join(home, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles")
		legacy := filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles")
		for _, directory := range []string{modern, legacy} {
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(modern, "modern.provisionprofile"), signedMacProvisioningProfile(t, "MODERN-UUID"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(legacy, "legacy.mobileprovision"), signedMacProvisioningProfile(t, "LEGACY-UUID"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modern, "ignored.txt"), []byte("not a profile"), 0o600); err != nil {
			t.Fatal(err)
		}
		originalHome := macUserHomeDirFn
		macUserHomeDirFn = func() (string, error) { return home, nil }
		t.Cleanup(func() { macUserHomeDirFn = originalHome })

		profiles, err := installedMacProvisioningProfileInfos(t.Context())
		if err != nil {
			t.Fatalf("installedMacProvisioningProfileInfos() error: %v", err)
		}
		if len(profiles) != 2 || profiles[0].UUID != "LEGACY-UUID" || profiles[1].UUID != "MODERN-UUID" {
			t.Fatalf("profiles = %#v, want sorted native macOS profiles from both directories", profiles)
		}
		if profiles[0].Type != profileutil.ProfileTypeMacOs || profiles[1].Type != profileutil.ProfileTypeMacOs {
			t.Fatalf("profile types = %q, %q, want macOS", profiles[0].Type, profiles[1].Type)
		}
	})

	t.Run("rejects profile symlink", func(t *testing.T) {
		directory := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.provisionprofile")
		if err := os.WriteFile(outside, signedMacProvisioningProfile(t, "OUTSIDE-UUID"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(directory, "linked.provisionprofile")); err != nil {
			t.Fatal(err)
		}
		_, err := readInstalledMacProvisioningProfiles(t.Context(), directory)
		if err == nil || !strings.Contains(err.Error(), "symlinked") {
			t.Fatalf("readInstalledMacProvisioningProfiles() error = %v, want symlink rejection", err)
		}
	})

	t.Run("rejects oversized profile", func(t *testing.T) {
		directory := t.TempDir()
		path := filepath.Join(directory, "oversized.provisionprofile")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(macEmbeddedProfileMaxBytes + 1); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		_, err = readInstalledMacProvisioningProfiles(t.Context(), directory)
		if err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("readInstalledMacProvisioningProfiles() error = %v, want size limit", err)
		}
	})

	t.Run("honors canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := readInstalledMacProvisioningProfiles(ctx, t.TempDir())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("readInstalledMacProvisioningProfiles() error = %v, want context.Canceled", err)
		}
	})
}

func TestResolveMacManualExportOptionsFiltersProfilesAndRequiresSharedCertificate(t *testing.T) {
	now := time.Now()
	archiveInfo := macArchiveExportInfo{
		ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
			AppBundleID: "com.example.demo",
			EntitlementsByBundleID: map[string]plistutil.PlistData{
				"com.example.demo":       {"com.apple.developer.team-identifier": "TEAM123"},
				"com.example.demo.share": {"com.apple.developer.team-identifier": "TEAM123"},
			},
		},
		EmbeddedProfiles: map[string]profileutil.ProvisioningProfileInfoModel{
			"com.example.demo":       {UUID: "embedded-main"},
			"com.example.demo.share": {UUID: "embedded-share"},
		},
	}
	profile := func(uuid, bundleID, teamID string, profileType profileutil.ProfileType, method legacyexportoptions.Method, expiry time.Time, serial string) profileutil.ProvisioningProfileInfoModel {
		return profileutil.ProvisioningProfileInfoModel{
			UUID: uuid, Name: uuid, BundleID: bundleID, TeamID: teamID, Type: profileType, ExportType: method,
			ExpirationDate: expiry, DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: serial}},
			Entitlements: plistutil.PlistData{"com.apple.developer.team-identifier": "TEAM123"},
		}
	}
	profiles := []profileutil.ProvisioningProfileInfoModel{
		profile("ios", "com.example.demo", "TEAM123", profileutil.ProfileTypeIos, legacyexportoptions.MethodAppStoreConnect, now.Add(time.Hour), "SHARED"),
		profile("dev", "com.example.demo", "TEAM123", profileutil.ProfileTypeMacOs, legacyexportoptions.MethodDevelopment, now.Add(time.Hour), "SHARED"),
		profile("wrong-team", "com.example.demo", "OTHER", profileutil.ProfileTypeMacOs, legacyexportoptions.MethodAppStoreConnect, now.Add(time.Hour), "SHARED"),
		profile("expired", "com.example.demo", "TEAM123", profileutil.ProfileTypeMacOs, legacyexportoptions.MethodAppStoreConnect, now.Add(-time.Hour), "SHARED"),
		profile("main", "com.example.demo", "TEAM123", profileutil.ProfileTypeMacOs, legacyexportoptions.MethodAppStoreConnect, now.Add(time.Hour), "SHARED"),
		profile("share-a", "com.example.demo.share", "TEAM123", profileutil.ProfileTypeMacOs, legacyexportoptions.MethodAppStoreConnect, now.Add(time.Hour), "SHARED"),
	}
	certificates := []certificateutil.CertificateInfoModel{{CommonName: "Mac App Distribution", TeamID: "TEAM123", Serial: "SHARED"}}
	certificates[0].CommonName = "Mac App Distribution: Example (TEAM123)"
	installers := []certificateutil.CertificateInfoModel{{CommonName: "Mac Installer Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "INSTALLER"}}
	manual, err := resolveMacManualExportOptions(archiveInfo, profiles, certificates, installers, "TEAM123")
	if err != nil {
		t.Fatalf("resolveMacManualExportOptions() error: %v", err)
	}
	if manual.ProvisioningProfiles["com.example.demo"] != "main" || manual.ProvisioningProfiles["com.example.demo.share"] != "share-a" {
		t.Fatalf("profile mapping = %#v", manual.ProvisioningProfiles)
	}

	profiles[len(profiles)-1].DeveloperCertificates = []certificateutil.CertificateInfoModel{{Serial: "OTHER"}}
	if _, err := resolveMacManualExportOptions(archiveInfo, profiles, certificates, installers, "TEAM123"); err == nil || !strings.Contains(err.Error(), "could not find one installed macOS distribution certificate") {
		t.Fatalf("expected shared certificate failure, got %v", err)
	}
}

func TestResolveMacManualExportOptionsAllowsArchiveWithoutEmbeddedProfiles(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {"com.apple.security.app-sandbox": true},
		},
	}}
	certificates := []certificateutil.CertificateInfoModel{{
		CommonName: "Apple Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "APP",
	}}
	installers := []certificateutil.CertificateInfoModel{{
		CommonName: "Mac Installer Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "INSTALLER",
	}}

	manual, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, installers, "TEAM123")
	if err != nil {
		t.Fatalf("resolveMacManualExportOptions() error: %v", err)
	}
	if len(manual.ProvisioningProfiles) != 0 {
		t.Fatalf("profile mapping = %#v, want no profiles for an archive without embedded profiles", manual.ProvisioningProfiles)
	}
	if manual.SigningCertificate != certificates[0].CommonName || manual.InstallerSigningCertificate != installers[0].CommonName {
		t.Fatalf("manual signing identities = %#v", manual)
	}
}

func TestResolveMacManualExportOptionsRequiresProfileForRestrictedEntitlement(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {"keychain-access-groups": []string{"TEAM123.com.example.shared"}},
		},
	}}
	certificates := []certificateutil.CertificateInfoModel{{
		CommonName: "Apple Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "APP",
	}}
	installers := []certificateutil.CertificateInfoModel{{
		CommonName: "Mac Installer Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "INSTALLER",
	}}

	_, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, installers, "TEAM123")
	if err == nil || !strings.Contains(err.Error(), "required App Store provisioning profile set") {
		t.Fatalf("expected restricted-entitlement profile error, got %v", err)
	}
}

func TestMacBundleRequiresProvisioningProfileForApplicationGroups(t *testing.T) {
	if !macBundleRequiresProvisioningProfile(plistutil.PlistData{
		"com.apple.security.application-groups": []string{"TEAM123.com.example.shared"},
	}, false) {
		t.Fatal("application groups must require a macOS provisioning profile")
	}
}

func TestMacBundleDoesNotRequireProvisioningProfileForScriptingTargets(t *testing.T) {
	if macBundleRequiresProvisioningProfile(plistutil.PlistData{
		"com.apple.security.app-sandbox": true,
		"com.apple.security.scripting-targets": map[string]any{
			"com.apple.mail": []string{"com.apple.mail.compose"},
		},
	}, false) {
		t.Fatal("App Sandbox scripting targets must not require a macOS provisioning profile")
	}
}

func TestSelectMacProvisioningProfileMatchesEntitlementValues(t *testing.T) {
	now := time.Now()
	profile := func(uuid string, entitlements plistutil.PlistData) profileutil.ProvisioningProfileInfoModel {
		return profileutil.ProvisioningProfileInfoModel{
			UUID: uuid, Name: uuid, BundleID: "com.example.demo", TeamID: "TEAM123",
			Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
			ExpirationDate: now.Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT"}},
			Entitlements: entitlements,
		}
	}

	tests := []struct {
		name         string
		target       any
		wrong        any
		matching     any
		matchingUUID string
	}{
		{
			name:         "keychain access groups",
			target:       []string{"TEAM123.com.example.shared"},
			wrong:        []string{"TEAM123.com.example.other"},
			matching:     []string{"TEAM123.com.example.shared"},
			matchingUUID: "matching-keychain",
		},
		{
			name:         "application groups",
			target:       []string{"TEAM123.com.example.shared"},
			wrong:        []string{"TEAM123.com.example.other"},
			matching:     []string{"TEAM123.com.example.shared"},
			matchingUUID: "matching-groups",
		},
		{
			name:         "iCloud containers",
			target:       []string{"iCloud.com.example.demo"},
			wrong:        []string{"iCloud.com.example.other"},
			matching:     []string{"iCloud.com.example.demo"},
			matchingUUID: "matching-icloud",
		},
	}
	keys := []string{
		"keychain-access-groups",
		"com.apple.security.application-groups",
		"com.apple.developer.icloud-container-identifiers",
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetEntitlements := plistutil.PlistData{keys[i]: tt.target}
			profiles := []profileutil.ProvisioningProfileInfoModel{
				profile("aaa-wrong", plistutil.PlistData{keys[i]: tt.wrong}),
				profile(tt.matchingUUID, plistutil.PlistData{keys[i]: tt.matching}),
			}

			selected, ok := selectMacProvisioningProfile("com.example.demo", nil, profiles, "CERT", "TEAM123", targetEntitlements)
			if !ok || selected.UUID != tt.matchingUUID {
				t.Fatalf("selected profile = %#v, %t; want %q", selected, ok, tt.matchingUUID)
			}

			if selected, ok := selectMacProvisioningProfile("com.example.demo", nil, profiles[:1], "CERT", "TEAM123", targetEntitlements); ok {
				t.Fatalf("selected mismatching profile %#v", selected)
			}
		})
	}
}

func TestSelectMacProvisioningProfileAllowsAppStoreEntitlementTransitions(t *testing.T) {
	now := time.Now()
	profile := profileutil.ProvisioningProfileInfoModel{
		UUID: "app-store", Name: "app-store", BundleID: "com.example.demo", TeamID: "TEAM123",
		Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
		ExpirationDate: now.Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT"}},
		Entitlements: plistutil.PlistData{
			"com.apple.developer.icloud-container-environment":      "Production",
			"com.apple.developer.aps-environment":                   "production",
			"com.apple.developer.devicecheck.appattest-environment": "production",
		},
	}
	targetEntitlements := plistutil.PlistData{
		"com.apple.developer.icloud-container-environment":      "Development",
		"com.apple.developer.aps-environment":                   "development",
		"com.apple.developer.devicecheck.appattest-environment": "development",
	}

	selected, ok := selectMacProvisioningProfile("com.example.demo", nil, []profileutil.ProvisioningProfileInfoModel{profile}, "CERT", "TEAM123", targetEntitlements)
	if !ok || selected.UUID != "app-store" {
		t.Fatalf("selected profile = %#v, %t; want App Store profile", selected, ok)
	}
}

func TestSelectMacProvisioningProfileAllowsCloudKitProfileValueShapes(t *testing.T) {
	now := time.Now()
	profile := profileutil.ProvisioningProfileInfoModel{
		UUID: "cloudkit", Name: "cloudkit", BundleID: "com.example.demo", TeamID: "TEAM123",
		Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
		ExpirationDate: now.Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT"}},
		Entitlements: plistutil.PlistData{
			"com.apple.developer.icloud-container-environment": []any{"Production", "Development"},
			"com.apple.developer.icloud-services":              "*",
		},
	}
	targetEntitlements := plistutil.PlistData{
		"com.apple.developer.icloud-container-environment": "Development",
		"com.apple.developer.icloud-services":              []any{"CloudKit", "CloudDocuments"},
	}

	selected, ok := selectMacProvisioningProfile("com.example.demo", nil, []profileutil.ProvisioningProfileInfoModel{profile}, "CERT", "TEAM123", targetEntitlements)
	if !ok || selected.UUID != "cloudkit" {
		t.Fatalf("selected profile = %#v, %t; want CloudKit profile", selected, ok)
	}
}

func TestSelectMacProvisioningProfileRejectsUnsafeCloudKitProfileValueShapes(t *testing.T) {
	now := time.Now()
	profile := func(entitlements plistutil.PlistData) profileutil.ProvisioningProfileInfoModel {
		return profileutil.ProvisioningProfileInfoModel{
			UUID: "unsafe", Name: "unsafe", BundleID: "com.example.demo", TeamID: "TEAM123",
			Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
			ExpirationDate: now.Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT"}},
			Entitlements: entitlements,
		}
	}

	tests := []struct {
		name    string
		profile any
		target  any
		key     string
	}{
		{name: "reverse environment transition", key: "com.apple.developer.icloud-container-environment", profile: []any{"Development"}, target: "Production"},
		{name: "unknown profile environment", key: "com.apple.developer.icloud-container-environment", profile: []any{"Production", "Staging"}, target: "Development"},
		{name: "unknown iCloud service", key: "com.apple.developer.icloud-services", profile: "*", target: []any{"CloudKit", "Unknown"}},
		{name: "App Clip-only iCloud service", key: "com.apple.developer.icloud-services", profile: "*", target: []any{"CloudKit-Anonymous"}},
		{name: "target iCloud service wildcard", key: "com.apple.developer.icloud-services", profile: "*", target: []any{"*"}},
		{name: "scalar target iCloud service", key: "com.apple.developer.icloud-services", profile: "*", target: "CloudKit"},
		{name: "non-string environment", key: "com.apple.developer.icloud-container-environment", profile: []any{"Production", 42}, target: "Production"},
		{name: "empty environment set", key: "com.apple.developer.icloud-container-environment", profile: []any{}, target: "Development"},
		{name: "non-string iCloud service", key: "com.apple.developer.icloud-services", profile: "*", target: []any{"CloudKit", 42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := profile(plistutil.PlistData{tt.key: tt.profile})
			if selected, ok := selectMacProvisioningProfile("com.example.demo", nil, []profileutil.ProvisioningProfileInfoModel{candidate}, "CERT", "TEAM123", plistutil.PlistData{tt.key: tt.target}); ok {
				t.Fatalf("selected unsafe profile %#v", selected)
			}
		})
	}
}

func TestMacProfileEntitlementTransitionsRejectWrongDirectionAndUnknownValues(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		profile any
		target  any
	}{
		{
			name:    "CloudKit production archive to development profile",
			key:     "com.apple.developer.icloud-container-environment",
			profile: "Development",
			target:  "Production",
		},
		{
			name:    "push production archive to development profile",
			key:     "com.apple.developer.aps-environment",
			profile: "development",
			target:  "production",
		},
		{
			name:    "unknown CloudKit value",
			key:     "com.apple.developer.icloud-container-environment",
			profile: "Production",
			target:  "Staging",
		},
		{
			name:    "unknown push value",
			key:     "com.apple.developer.aps-environment",
			profile: "production",
			target:  "staging",
		},
		{
			name:    "App Attest production archive to development profile",
			key:     "com.apple.developer.devicecheck.appattest-environment",
			profile: "development",
			target:  "production",
		},
		{
			name:    "unknown App Attest value",
			key:     "com.apple.developer.devicecheck.appattest-environment",
			profile: "production",
			target:  "staging",
		},
		{
			name:    "matching unknown App Attest value",
			key:     "com.apple.developer.devicecheck.appattest-environment",
			profile: "staging",
			target:  "staging",
		},
		{
			name:    "noncanonical CloudKit case",
			key:     "com.apple.developer.icloud-container-environment",
			profile: "production",
			target:  "development",
		},
		{
			name:    "noncanonical push case",
			key:     "com.apple.developer.aps-environment",
			profile: "production",
			target:  "DEVELOPMENT",
		},
		{
			name:    "noncanonical App Attest whitespace",
			key:     "com.apple.developer.devicecheck.appattest-environment",
			profile: "production",
			target:  " development ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if macProfileEntitlementValuePermitsForExport(tt.key, tt.profile, tt.target) {
				t.Fatalf("profile value %#v permitted target %#v for %q", tt.profile, tt.target, tt.key)
			}
		})
	}
}

func TestSelectMacProvisioningProfileAllowsWildcardEntitlementValues(t *testing.T) {
	now := time.Now()
	profiles := []profileutil.ProvisioningProfileInfoModel{{
		UUID: "wildcard", Name: "wildcard", BundleID: "com.example.*", TeamID: "TEAM123",
		Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
		ExpirationDate: now.Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "CERT"}},
		Entitlements: plistutil.PlistData{
			"keychain-access-groups": []string{"TEAM123.*"},
		},
	}}
	targetEntitlements := plistutil.PlistData{
		"keychain-access-groups": []string{"TEAM123.com.example.shared"},
	}

	selected, ok := selectMacProvisioningProfile("com.example.demo", nil, profiles, "CERT", "TEAM123", targetEntitlements)
	if !ok || selected.UUID != "wildcard" {
		t.Fatalf("selected profile = %#v, %t; want wildcard profile", selected, ok)
	}
}

func TestMacProfileEntitlementValueRejectsUnsafeWildcardsAndLists(t *testing.T) {
	tests := []struct {
		name    string
		profile any
		target  any
	}{
		{name: "bare wildcard", profile: "*", target: "TEAM123.com.example.shared"},
		{name: "partial suffix wildcard", profile: "TEAM123*", target: "TEAM123.com.example.shared"},
		{name: "middle wildcard", profile: "TEAM*.shared", target: "TEAM123.com.example.shared"},
		{name: "mixed profile list", profile: []any{"TEAM123.*", 42}, target: []string{"TEAM123.com.example.shared"}},
		{name: "wildcard target", profile: []string{"TEAM123.*"}, target: []string{"TEAM123.*"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if macProfileEntitlementValuePermits(tt.profile, tt.target) {
				t.Fatalf("profile value %#v permitted target %#v", tt.profile, tt.target)
			}
		})
	}
}

func TestResolveMacManualExportOptionsUsesSelectedCertificateFingerprints(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {"keychain-access-groups": []string{"TEAM123.com.example.shared"}},
		},
	}}
	profiles := []profileutil.ProvisioningProfileInfoModel{{
		UUID: "matching", Name: "matching", BundleID: "com.example.demo", TeamID: "TEAM123",
		Type: profileutil.ProfileTypeMacOs, ExportType: legacyexportoptions.MethodAppStoreConnect,
		ExpirationDate: time.Now().Add(time.Hour), DeveloperCertificates: []certificateutil.CertificateInfoModel{{Serial: "BBB"}},
		Entitlements: plistutil.PlistData{"keychain-access-groups": []string{"TEAM123.com.example.shared"}},
	}}
	certificates := []certificateutil.CertificateInfoModel{
		{CommonName: "Mac App Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "AAA", SHA1Fingerprint: "APP-WRONG"},
		{CommonName: "Mac App Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "BBB", SHA1Fingerprint: "APP-MATCHING"},
	}
	installers := []certificateutil.CertificateInfoModel{{
		CommonName: "Mac Installer Distribution: Example (TEAM123)", TeamID: "TEAM123", Serial: "INSTALLER", SHA1Fingerprint: "INSTALLER-SHA1",
	}}

	manual, err := resolveMacManualExportOptions(archiveInfo, profiles, certificates, installers, "TEAM123")
	if err != nil {
		t.Fatalf("resolveMacManualExportOptions() error: %v", err)
	}
	if manual.SigningCertificate != "APP-MATCHING" {
		t.Fatalf("signing certificate = %q, want selected certificate fingerprint", manual.SigningCertificate)
	}
	if manual.InstallerSigningCertificate != "INSTALLER-SHA1" {
		t.Fatalf("installer signing certificate = %q, want selected certificate fingerprint", manual.InstallerSigningCertificate)
	}
}

func TestInstalledIdentityInfosPreservesExactPrivateKeyIdentities(t *testing.T) {
	const (
		firstIdentity   = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		secondIdentity  = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
		certificateOnly = "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	)
	for _, policy := range []string{"codesigning", "macappstore"} {
		t.Run(policy, func(t *testing.T) {
			originalCommand := macSecurityCommandContextFn
			macSecurityCommandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
				if name != "/usr/bin/security" {
					t.Fatalf("security path = %q", name)
				}
				if got := strings.Join(args, " "); got != "find-identity -v -p "+policy {
					t.Fatalf("security arguments = %q", got)
				}
				output := fmt.Sprintf("  1) %s \"Shared Name\"\n  2) %s \"Shared Name\"\n     2 valid identities found\n", firstIdentity, secondIdentity)
				return exec.CommandContext(ctx, "/usr/bin/printf", "%s", output)
			}
			t.Cleanup(func() { macSecurityCommandContextFn = originalCommand })

			certificates := []certificateutil.CertificateInfoModel{
				{CommonName: "Shared Name", SHA1Fingerprint: certificateOnly},
				{CommonName: "Shared Name", SHA1Fingerprint: strings.ToLower(secondIdentity)},
				{CommonName: "Shared Name", SHA1Fingerprint: firstIdentity},
			}
			identities, err := installedIdentityInfos(t.Context(), policy, func(context.Context) ([]certificateutil.CertificateInfoModel, error) {
				return certificates, nil
			})
			if err != nil {
				t.Fatalf("installedIdentityInfos() error: %v", err)
			}
			if len(identities) != 2 || !strings.EqualFold(identities[0].SHA1Fingerprint, firstIdentity) || !strings.EqualFold(identities[1].SHA1Fingerprint, secondIdentity) {
				t.Fatalf("identities = %#v, want exact private-key-backed fingerprints in security order", identities)
			}
		})
	}
}

func TestInstalledIdentityInfosRejectsUnmatchedIdentityFingerprint(t *testing.T) {
	const identity = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	originalCommand := macSecurityCommandContextFn
	macSecurityCommandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/usr/bin/printf", "%s", "1) "+identity+" \"Identity\"\n")
	}
	t.Cleanup(func() { macSecurityCommandContextFn = originalCommand })

	_, err := installedIdentityInfos(t.Context(), "codesigning", func(context.Context) ([]certificateutil.CertificateInfoModel, error) {
		return []certificateutil.CertificateInfoModel{{CommonName: "Identity", SHA1Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "did not match an accessible certificate") {
		t.Fatalf("installedIdentityInfos() error = %v, want unmatched identity rejection", err)
	}
}

func TestInstalledCodesignIdentityInfosReadsCertificateMetadataWithContext(t *testing.T) {
	certificate, certificatePEM := testInstalledIdentityCertificate(t, "Mac App Distribution: Example (TEAM123)")
	originalCommand := macSecurityCommandContextFn
	macSecurityCommandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "/usr/bin/security" {
			t.Fatalf("security path = %q", name)
		}
		switch got := strings.Join(args, " "); got {
		case "find-identity -v -p codesigning":
			return exec.CommandContext(ctx, "/usr/bin/printf", "%s", "1) "+certificate.SHA1Fingerprint+" \""+certificate.CommonName+"\"\n")
		case "find-certificate -a -p":
			return exec.CommandContext(ctx, "/usr/bin/printf", "%s", string(certificatePEM))
		default:
			t.Fatalf("security arguments = %q", got)
			return nil
		}
	}
	t.Cleanup(func() { macSecurityCommandContextFn = originalCommand })

	identities, err := installedCodesignIdentityInfos(t.Context())
	if err != nil {
		t.Fatalf("installedCodesignIdentityInfos() error: %v", err)
	}
	if len(identities) != 1 || !strings.EqualFold(identities[0].SHA1Fingerprint, certificate.SHA1Fingerprint) || identities[0].CommonName != certificate.CommonName {
		t.Fatalf("identities = %#v, want certificate metadata for exact identity", identities)
	}
}

func TestInstalledCodesignIdentityInfosCancelsCertificateMetadataEnumeration(t *testing.T) {
	const identity = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	originalCommand := macSecurityCommandContextFn
	macSecurityCommandContextFn = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		switch got := strings.Join(args, " "); got {
		case "find-identity -v -p codesigning":
			return exec.CommandContext(ctx, "/usr/bin/printf", "%s", "1) "+identity+" \"Identity\"\n")
		case "find-certificate -a -p":
			return exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 10")
		default:
			t.Fatalf("security arguments = %q", got)
			return nil
		}
	}
	t.Cleanup(func() { macSecurityCommandContextFn = originalCommand })

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := installedCodesignIdentityInfos(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("installedCodesignIdentityInfos() error = %v, want context deadline exceeded", err)
	}
}

func TestInstalledIdentityFingerprintsRejectsMalformedAndOversizedOutput(t *testing.T) {
	if _, err := parseInstalledIdentityFingerprints([]byte("1) NOT-A-SHA1 \"Identity\"\n")); err == nil {
		t.Fatal("expected malformed identity fingerprint rejection")
	}

	originalCommand := macSecurityCommandContextFn
	originalRun := macSecurityRunCommandFn
	macSecurityCommandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/usr/bin/true")
	}
	macSecurityRunCommandFn = func(command *exec.Cmd) error {
		_, err := command.Stdout.Write(make([]byte, macSecurityOutputMaxBytes+1))
		return err
	}
	t.Cleanup(func() {
		macSecurityCommandContextFn = originalCommand
		macSecurityRunCommandFn = originalRun
	})
	if _, err := installedIdentityFingerprints(t.Context(), "codesigning"); err == nil || !strings.Contains(err.Error(), "output exceeded") {
		t.Fatalf("installedIdentityFingerprints() error = %v, want bounded-output rejection", err)
	}
}

func TestInstalledIdentityFingerprintsPreservesCancellation(t *testing.T) {
	originalCommand := macSecurityCommandContextFn
	macSecurityCommandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 10")
	}
	t.Cleanup(func() { macSecurityCommandContextFn = originalCommand })

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := installedIdentityFingerprints(ctx, "macappstore")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("installedIdentityFingerprints() error = %v, want context deadline exceeded", err)
	}
}

func TestResolveMacManualExportOptionsPairsInstallerWithInferredApplicationTeam(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {"com.apple.security.app-sandbox": true},
		},
	}, SigningIdentity: "Apple Distribution: B"}
	certificates := []certificateutil.CertificateInfoModel{
		{CommonName: "Apple Distribution: A", TeamID: "TEAMA", Serial: "APP-A"},
		{CommonName: "Apple Distribution: B", TeamID: "TEAMB", Serial: "APP-B"},
	}
	installers := []certificateutil.CertificateInfoModel{
		{CommonName: "3rd Party Mac Developer Installer: A", TeamID: "TEAMB", Serial: "INSTALLER-B"},
		{CommonName: "3rd Party Mac Developer Installer: Z", TeamID: "TEAMA", Serial: "INSTALLER-A"},
	}

	manual, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, installers, "")
	if err != nil {
		t.Fatalf("resolveMacManualExportOptions() error: %v", err)
	}
	if manual.TeamID != "TEAMB" || manual.SigningCertificate != "Apple Distribution: B" || manual.InstallerSigningCertificate != "3rd Party Mac Developer Installer: A" {
		t.Fatalf("manual identities were not paired by inferred team: %#v", manual)
	}
}

func TestResolveMacManualExportOptionsRejectsAmbiguousInstalledTeams(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {"com.apple.security.app-sandbox": true},
		},
	}}
	certificates := []certificateutil.CertificateInfoModel{
		{CommonName: "Apple Distribution: A", TeamID: "TEAMA", Serial: "APP-A"},
		{CommonName: "Apple Distribution: B", TeamID: "TEAMB", Serial: "APP-B"},
	}
	installers := []certificateutil.CertificateInfoModel{
		{CommonName: "3rd Party Mac Developer Installer: A", TeamID: "TEAMA", Serial: "INSTALLER-A"},
		{CommonName: "3rd Party Mac Developer Installer: B", TeamID: "TEAMB", Serial: "INSTALLER-B"},
	}

	_, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, installers, "")
	if err == nil || !strings.Contains(err.Error(), "multiple installed teams") || !strings.Contains(err.Error(), "--team-id") {
		t.Fatalf("resolveMacManualExportOptions() error = %v, want ambiguous-team guidance", err)
	}
}

func TestResolveMacManualExportOptionsInfersTeamFromEntitlements(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {
				"com.apple.developer.team-identifier": "TEAMB",
				"com.apple.security.app-sandbox":      true,
			},
		},
	}}
	certificates := []certificateutil.CertificateInfoModel{
		{CommonName: "Apple Distribution: A", TeamID: "TEAMA", Serial: "APP-A"},
		{CommonName: "Apple Distribution: B", TeamID: "TEAMB", Serial: "APP-B"},
	}
	installers := []certificateutil.CertificateInfoModel{
		{CommonName: "3rd Party Mac Developer Installer: A", TeamID: "TEAMA", Serial: "INSTALLER-A"},
		{CommonName: "3rd Party Mac Developer Installer: B", TeamID: "TEAMB", Serial: "INSTALLER-B"},
	}

	manual, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, installers, "")
	if err != nil {
		t.Fatalf("resolveMacManualExportOptions() error: %v", err)
	}
	if manual.TeamID != "TEAMB" || manual.SigningCertificate != "Apple Distribution: B" || manual.InstallerSigningCertificate != "3rd Party Mac Developer Installer: B" {
		t.Fatalf("manual identities were not selected using the archived entitlement team: %#v", manual)
	}
}

func TestResolveMacManualExportOptionsInfersTeamFromArchivedIdentityName(t *testing.T) {
	archiveInfo := macArchiveExportInfo{
		ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
			AppBundleID: "com.example.demo",
			EntitlementsByBundleID: map[string]plistutil.PlistData{
				"com.example.demo": {"com.apple.security.app-sandbox": true},
			},
		},
		SigningIdentity: "Apple Development: Example (TEAM123456)",
	}
	certificates := []certificateutil.CertificateInfoModel{
		{CommonName: "Apple Distribution: Other", TeamID: "OTHER12345", Serial: "APP-A"},
		{CommonName: "Apple Distribution: Example", TeamID: "TEAM123456", Serial: "APP-B"},
	}
	installers := []certificateutil.CertificateInfoModel{
		{CommonName: "Mac Installer Distribution: Example", TeamID: "TEAM123456", Serial: "INSTALLER-B"},
	}

	manual, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, installers, "")
	if err != nil {
		t.Fatalf("resolveMacManualExportOptions() error: %v", err)
	}
	if manual.TeamID != "TEAM123456" || manual.SigningCertificate != "Apple Distribution: Example" {
		t.Fatalf("manual identity was not selected using the archived identity team: %#v", manual)
	}
}

func TestResolveMacManualExportOptionsRejectsConflictingArchiveTeams(t *testing.T) {
	archiveInfo := macArchiveExportInfo{
		ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
			AppBundleID: "com.example.demo",
			EntitlementsByBundleID: map[string]plistutil.PlistData{
				"com.example.demo": {"com.apple.developer.team-identifier": "TEAMB"},
			},
		},
		EmbeddedProfiles: map[string]profileutil.ProvisioningProfileInfoModel{
			"com.example.demo": {TeamID: "TEAMA"},
		},
	}

	_, err := resolveMacManualExportOptions(archiveInfo, nil, nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "conflicting team identifiers") || !strings.Contains(err.Error(), "TEAMA, TEAMB") {
		t.Fatalf("resolveMacManualExportOptions() error = %v, want conflicting archive teams", err)
	}
}

func TestResolveMacManualExportOptionsRequiresInstallerIdentity(t *testing.T) {
	archiveInfo := macArchiveExportInfo{ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
		AppBundleID: "com.example.demo",
		EntitlementsByBundleID: map[string]plistutil.PlistData{
			"com.example.demo": {},
		},
	}}
	certificates := []certificateutil.CertificateInfoModel{{CommonName: "Mac App Distribution: Example (TEAM123)", TeamID: "TEAM123"}}

	_, err := resolveMacManualExportOptions(archiveInfo, nil, certificates, nil, "TEAM123")
	if err == nil || !strings.Contains(err.Error(), "installer certificate") {
		t.Fatalf("expected installer identity error, got %v", err)
	}
	wrongTeam := []certificateutil.CertificateInfoModel{{CommonName: "Mac Installer Distribution: Other", TeamID: "OTHER"}}
	_, err = resolveMacManualExportOptions(archiveInfo, nil, certificates, wrongTeam, "TEAM123")
	if err == nil || !strings.Contains(err.Error(), "installer certificate") {
		t.Fatalf("expected wrong-team installer identity error, got %v", err)
	}
}

func TestManualExportOptionsFromHashPreservesCloudKitEnvironment(t *testing.T) {
	manual, err := manualExportOptionsFromHash(map[string]interface{}{
		"teamID":                      "TEAM123",
		"signingCertificate":          "Apple Distribution: Example (TEAM123)",
		"installerSigningCertificate": "Mac Installer Distribution",
		"provisioningProfiles":        map[string]string{"com.example.demo": "profile-uuid"},
		"iCloudContainerEnvironment":  "Production",
	})
	if err != nil {
		t.Fatalf("manualExportOptionsFromHash() error: %v", err)
	}

	payload := buildPlatformExportOptionsPayload(ExportOptionsGenerateOptions{
		Destination:  exportOptionsDestinationExport,
		SigningStyle: exportOptionsSigningStyleManual,
	}, manual.TeamID, manual)
	if fmt.Sprint(payload["iCloudContainerEnvironment"]) != "Production" {
		t.Fatalf("iCloudContainerEnvironment = %#v, want Production", payload["iCloudContainerEnvironment"])
	}
	if fmt.Sprint(payload["installerSigningCertificate"]) != "Mac Installer Distribution" {
		t.Fatalf("installerSigningCertificate = %#v, want Mac Installer Distribution", payload["installerSigningCertificate"])
	}
}

func writeMacExportOptionsTestArchive(t *testing.T) string {
	t.Helper()
	const teamID = "TEAM123"

	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	appPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app")
	if err := os.MkdirAll(filepath.Join(archivePath, "Products", "Applications", "AAA-Decoy.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "Contents", "PlugIns", "Share.appex", "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "Contents", "XPCServices", "Helper.xpc", "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "Contents", "Library", "LoginItems", "Login.app", "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "Contents", "Library", "SystemExtensions", "Network.systemextension", "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appPath, "Contents", "Library", "SystemExtensions", "Driver.dext", "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}

	archiveData, err := plist.Marshal(map[string]any{
		"ApplicationProperties": map[string]any{
			"ApplicationPath":    "Applications/Demo.app",
			"CFBundleIdentifier": "com.example.demo",
			"Team":               teamID,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "Info.plist"), archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	appData, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleExecutable":         "Demo",
		"DTPlatformName":             "macosx",
		"CFBundleSupportedPlatforms": []string{"macosx"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), appData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "MacOS", "Demo"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	extensionData, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo.share",
		"CFBundleExecutable": "Share",
		"DTPlatformName":     "macosx",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "PlugIns", "Share.appex", "Contents", "Info.plist"), extensionData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "PlugIns", "Share.appex", "Contents", "MacOS", "Share"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	helperData, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo.helper",
		"CFBundleExecutable": "Helper",
		"DTPlatformName":     "macosx",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "XPCServices", "Helper.xpc", "Contents", "Info.plist"), helperData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Contents", "XPCServices", "Helper.xpc", "Contents", "MacOS", "Helper"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, nested := range []struct {
		path       string
		bundleID   string
		executable string
	}{
		{path: filepath.Join("Contents", "Library", "LoginItems", "Login.app"), bundleID: "com.example.demo.login", executable: "Login"},
		{path: filepath.Join("Contents", "Library", "SystemExtensions", "Network.systemextension"), bundleID: "com.example.demo.network-extension", executable: "Network"},
		{path: filepath.Join("Contents", "Library", "SystemExtensions", "Driver.dext"), bundleID: "com.example.demo.driver-extension", executable: "Driver"},
		{path: filepath.Join("Contents", "PlugIns", "Support.bundle"), bundleID: "com.example.demo.bundle", executable: "Support"},
		{path: filepath.Join("Contents", "PlugIns", "Importer.plugin"), bundleID: "com.example.demo.plugin", executable: "Importer"},
		{path: filepath.Join("Contents", "Library", "QuickLook", "Preview.qlgenerator"), bundleID: "com.example.demo.quicklook", executable: "Preview"},
		{path: filepath.Join("Contents", "Library", "Spotlight", "Metadata.mdimporter"), bundleID: "com.example.demo.metadata-importer", executable: "Metadata"},
	} {
		data, err := plist.Marshal(map[string]any{
			"CFBundleIdentifier": nested.bundleID,
			"CFBundleExecutable": nested.executable,
			"DTPlatformName":     "macosx",
		}, plist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		bundlePath := filepath.Join(appPath, nested.path)
		if err := os.MkdirAll(filepath.Join(bundlePath, "Contents", "MacOS"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bundlePath, "Contents", "Info.plist"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bundlePath, "Contents", "MacOS", nested.executable), []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return archivePath
}

func signedMacProvisioningProfile(t *testing.T, uuid string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    now,
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        uuid,
		"Name":                        "Mac Profile " + uuid,
		"TeamName":                    "Example Team",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"Platform":                    []string{"osx"},
		"CreationDate":                now,
		"ExpirationDate":              now.Add(24 * time.Hour),
		"DeveloperCertificates":       [][]byte{der},
		"Entitlements": map[string]any{
			"application-identifier":              "TEAM123.com.example.demo",
			"com.apple.developer.team-identifier": "TEAM123",
			"get-task-allow":                      false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := pkcs7.NewSignedData(profilePlist)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSigner(certificate, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	data, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testInstalledIdentityCertificate(t *testing.T, commonName string) (certificateutil.CertificateInfoModel, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject: pkix.Name{
			CommonName:         commonName,
			Organization:       []string{"Example Team"},
			OrganizationalUnit: []string{"TEAM123"},
		},
		NotBefore: now,
		NotAfter:  now.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificateutil.NewCertificateInfo(*certificate, nil), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
