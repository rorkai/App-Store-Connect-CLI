//go:build darwin

package xcode

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	legacyexportoptions "github.com/bitrise-io/go-xcode/exportoptions"
	"github.com/bitrise-io/go-xcode/v2/exportoptions"
	"github.com/bitrise-io/go-xcode/v2/exportoptionsgenerator"
	"github.com/bitrise-io/go-xcode/v2/xcarchive"
	"github.com/bitrise-io/go-xcode/v2/xcodeversion"
)

var bitriseStdoutCaptureMu sync.Mutex

var readArchiveExportInfoFn = readArchiveExportInfo

// buildPlatformExportOptionsPayload uses Bitrise's current typed v2 model on
// macOS, where xcodebuild and local signing asset resolution are available.
func buildPlatformExportOptionsPayload(opts ExportOptionsGenerateOptions, teamID string, manual manualExportOptions) map[string]any {
	if opts.Method == exportOptionsMethodReleaseTesting {
		model := exportoptions.NewNonAppStoreOptions(exportoptions.MethodReleaseTesting)
		model.TeamID = teamID
		model.Destination = exportoptions.Destination(opts.Destination)
		model.SigningStyle = exportoptions.SigningStyle(opts.SigningStyle)
		if opts.SigningStyle == exportOptionsSigningStyleManual {
			model.SigningCertificate = manual.SigningCertificate
			model.BundleIDProvisioningProfileMapping = cloneProvisioningProfiles(manual.ProvisioningProfiles)
			model.ICloudContainerEnvironment = exportoptions.ICloudContainerEnvironment(manual.ICloudContainerEnvironment)
		}
		return model.Hash()
	}
	model := exportoptions.NewAppStoreConnectOptions(exportoptions.MethodAppStoreConnect)
	model.TeamID = teamID
	model.Destination = exportoptions.Destination(opts.Destination)
	model.SigningStyle = exportoptions.SigningStyle(opts.SigningStyle)
	if opts.SigningStyle == exportOptionsSigningStyleManual {
		model.SigningCertificate = manual.SigningCertificate
		model.BundleIDProvisioningProfileMapping = cloneProvisioningProfiles(manual.ProvisioningProfiles)
		model.ICloudContainerEnvironment = exportoptions.ICloudContainerEnvironment(manual.ICloudContainerEnvironment)
	}
	return model.Hash()
}

func generateManualExportOptions(ctx context.Context, archivePath, teamID, method string) (manualExportOptions, error) {
	if err := contextError(ctx); err != nil {
		return manualExportOptions{}, err
	}
	platform, err := InferArchivePlatform(archivePath)
	if err != nil {
		return manualExportOptions{}, fmt.Errorf("infer archive platform: %w", err)
	}
	if platform != "IOS" && platform != "TV_OS" {
		return manualExportOptions{}, fmt.Errorf("manual signing export options generation only supports iOS and tvOS archives; archive platform is %s", platform)
	}
	var generated legacyexportoptions.ExportOptions
	if _, err := captureBitriseStdout(func() error {
		archiveInfo, err := readArchiveExportInfoFn(archivePath)
		if err != nil {
			return err
		}
		generator := exportoptionsgenerator.New(
			xcodeversion.NewXcodeVersionProvider(command.NewFactory(env.NewRepository())),
			log.NewLogger(),
		)
		var generateErr error
		exportMethod := manualExportOptionsResolverMethod(method)
		generated, generateErr = generator.GenerateApplicationExportOptions(
			exportoptionsgenerator.ExportProductApp,
			archiveInfo,
			// Bitrise v2's generator currently exposes these v1 argument types.
			exportMethod,
			legacyexportoptions.SigningStyleManual,
			manualExportOptionsResolverOptions(teamID, method),
		)
		return generateErr
	}); err != nil {
		return manualExportOptions{}, err
	}
	return manualExportOptionsFromHash(generated.Hash())
}

func readArchiveExportInfo(archivePath string) (exportoptionsgenerator.ArchiveInfo, error) {
	archive, err := xcarchive.NewIosArchive(archivePath)
	if err != nil {
		return exportoptionsgenerator.ArchiveInfo{}, fmt.Errorf("read iOS archive: %w", err)
	}
	archiveInfo, err := exportoptionsgenerator.ReadArchiveExportInfo(archive)
	if err != nil {
		return exportoptionsgenerator.ArchiveInfo{}, fmt.Errorf("read archive export information: %w", err)
	}
	return archiveInfo, nil
}

func manualExportOptionsResolverOptions(teamID, method string) exportoptionsgenerator.Opts {
	opts := exportoptionsgenerator.Opts{TeamID: teamID}
	if method == exportOptionsMethodReleaseTesting {
		// Distribution profiles carry production entitlements. Bitrise requires
		// this value for non-App-Store CloudKit archives instead of inferring it.
		opts.ContainerEnvironment = "Production"
	}
	return opts
}

// manualExportOptionsResolverMethod adapts ASC's current public Xcode method
// name to the pinned resolver's profile classification. Bitrise still labels
// installed ad hoc profiles as MethodAdHoc and filters by exact equality, even
// though the final ExportOptions.plist must use MethodReleaseTesting.
func manualExportOptionsResolverMethod(method string) legacyexportoptions.Method {
	if method == exportOptionsMethodReleaseTesting {
		return legacyexportoptions.MethodAdHoc
	}
	return legacyexportoptions.MethodAppStoreConnect
}

// captureBitriseStdout contains upstream status prints so structured CLI
// output remains valid. Bitrise does not currently expose a writer for these
// messages, and os.Stdout is process-global, so captures are serialized.
func captureBitriseStdout(run func() error) (string, error) {
	bitriseStdoutCaptureMu.Lock()
	defer bitriseStdoutCaptureMu.Unlock()

	reader, writer, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("capture Bitrise stdout: %w", err)
	}
	originalStdout := os.Stdout
	defer func() {
		os.Stdout = originalStdout
		_ = writer.Close()
		_ = reader.Close()
	}()

	type readResult struct {
		data []byte
		err  error
	}
	readDone := make(chan readResult, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		readDone <- readResult{data: data, err: readErr}
	}()

	os.Stdout = writer
	runErr := run()
	os.Stdout = originalStdout
	closeErr := writer.Close()
	result := <-readDone

	if runErr != nil {
		return string(result.data), runErr
	}
	if closeErr != nil {
		return string(result.data), fmt.Errorf("close Bitrise stdout capture: %w", closeErr)
	}
	if result.err != nil {
		return string(result.data), fmt.Errorf("read Bitrise stdout capture: %w", result.err)
	}
	return string(result.data), nil
}

func manualExportOptionsFromHash(payload map[string]interface{}) (manualExportOptions, error) {
	profiles, err := provisioningProfilesFromPayload(payload["provisioningProfiles"])
	if err != nil {
		return manualExportOptions{}, err
	}
	cloudEnvironment := ""
	if value, ok := payload["iCloudContainerEnvironment"]; ok {
		cloudEnvironment = strings.TrimSpace(fmt.Sprint(value))
	}
	return manualExportOptions{
		TeamID:                     strings.TrimSpace(coercePlistValueToString(payload["teamID"])),
		SigningCertificate:         strings.TrimSpace(coercePlistValueToString(payload["signingCertificate"])),
		ProvisioningProfiles:       profiles,
		ICloudContainerEnvironment: cloudEnvironment,
	}, nil
}
