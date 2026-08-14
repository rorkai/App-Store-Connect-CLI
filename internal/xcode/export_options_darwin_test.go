//go:build darwin

package xcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	legacyexportoptions "github.com/bitrise-io/go-xcode/exportoptions"
	"github.com/bitrise-io/go-xcode/v2/exportoptionsgenerator"
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

func TestGenerateManualExportOptionsRejectsMacOSArchiveClearly(t *testing.T) {
	archivePath := writeExportOptionsTestArchive(t, "TEAM123")
	appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
	data, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo",
		"DTPlatformName":     "macosx",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appInfoPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = generateManualExportOptions(t.Context(), archivePath, "TEAM123", exportOptionsMethodAppStoreConnect)
	if err == nil || !strings.Contains(err.Error(), "only supports iOS and tvOS archives") || !strings.Contains(err.Error(), "MAC_OS") {
		t.Fatalf("expected clear macOS archive rejection, got %v", err)
	}
}

func TestManualExportOptionsFromHashPreservesCloudKitEnvironment(t *testing.T) {
	manual, err := manualExportOptionsFromHash(map[string]interface{}{
		"teamID":                     "TEAM123",
		"signingCertificate":         "Apple Distribution: Example (TEAM123)",
		"provisioningProfiles":       map[string]string{"com.example.demo": "profile-uuid"},
		"iCloudContainerEnvironment": "Production",
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
}
