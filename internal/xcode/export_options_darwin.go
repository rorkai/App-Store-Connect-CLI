//go:build darwin

package xcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
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

var (
	readArchiveExportInfoFn                   = readArchiveExportInfo
	generateBitriseApplicationExportOptionsFn = generateBitriseApplicationExportOptions
)

// bitriseLogLevelPattern matches the color prefixes Bitrise's logger uses for
// warning (yellow) and error (red) lines; plain progress lines are skipped.
var bitriseLogLevelPattern = regexp.MustCompile(`^\x1b\[3[13];1m`)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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
		model.InstallerSigningCertificate = manual.InstallerSigningCertificate
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
	if platform == "MAC_OS" {
		if method != exportOptionsMethodAppStoreConnect {
			return manualExportOptions{}, fmt.Errorf("manual signing export options generation for macOS archives only supports method %q; got %q", exportOptionsMethodAppStoreConnect, method)
		}
		return generateMacManualExportOptions(ctx, archivePath, teamID)
	}
	if platform != "IOS" && platform != "TV_OS" {
		return manualExportOptions{}, fmt.Errorf("manual signing export options generation only supports iOS, tvOS, and App Store macOS archives; archive platform is %s", platform)
	}
	var generated legacyexportoptions.ExportOptions
	var bundleIDs []string
	resolverLog, err := captureBitriseStdout(func() error {
		archiveInfo, err := readArchiveExportInfoFn(archivePath)
		if err != nil {
			return err
		}
		for bundleID := range archiveInfo.EntitlementsByBundleID {
			// Bitrise drops the App Clip target from non-App-Store exports,
			// so it never needs a matching profile for release-testing.
			if method == exportOptionsMethodReleaseTesting && bundleID == archiveInfo.AppClipBundleID {
				continue
			}
			bundleIDs = append(bundleIDs, bundleID)
		}
		var generateErr error
		generated, generateErr = generateBitriseApplicationExportOptionsFn(
			archiveInfo,
			manualExportOptionsResolverMethod(method),
			manualExportOptionsResolverOptions(teamID, method),
		)
		return generateErr
	})
	if err != nil {
		return manualExportOptions{}, err
	}
	payload := generated.Hash()
	if _, found := payload["provisioningProfiles"]; !found {
		return manualExportOptions{}, manualExportOptionsResolutionError(bundleIDs, method, teamID, resolverLog)
	}
	return manualExportOptionsFromHash(payload)
}

func generateBitriseApplicationExportOptions(archiveInfo exportoptionsgenerator.ArchiveInfo, method legacyexportoptions.Method, opts exportoptionsgenerator.Opts) (legacyexportoptions.ExportOptions, error) {
	generator := exportoptionsgenerator.New(
		xcodeversion.NewXcodeVersionProvider(command.NewFactory(env.NewRepository())),
		log.NewLogger(),
	)
	return generator.GenerateApplicationExportOptions(
		exportoptionsgenerator.ExportProductApp,
		archiveInfo,
		// Bitrise v2's generator currently exposes these v1 argument types.
		method,
		legacyexportoptions.SigningStyleManual,
		opts,
	)
}

// manualExportOptionsResolutionError explains why no code signing group was
// found, keeping only the resolver's warning and error lines. Those lines name
// bundle IDs and targets, never certificate or profile contents.
func manualExportOptionsResolutionError(bundleIDs []string, method, teamID, resolverLog string) error {
	sort.Strings(bundleIDs)
	var reasons []string
	for _, line := range strings.Split(resolverLog, "\n") {
		if !bitriseLogLevelPattern.MatchString(line) {
			continue
		}
		if reason := strings.TrimSpace(ansiEscapePattern.ReplaceAllString(line, "")); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	team := ""
	if strings.TrimSpace(teamID) != "" {
		team = fmt.Sprintf(" and team %q", teamID)
	}
	message := fmt.Sprintf(
		"manual export options require provisioning profile mappings: no installed signing certificate and provisioning profile matched bundle IDs %s for method %q%s",
		strings.Join(bundleIDs, ", "), method, team,
	)
	if len(reasons) > 0 {
		message += " (resolver: " + strings.Join(reasons, "; ") + ")"
	}
	message += "; install the distribution certificate with its private key and a matching provisioning profile for every bundle ID, or write an ExportOptions.plist with explicit provisioningProfiles and pass it to asc xcode export --export-options"
	return errors.New(message)
}

func readArchiveExportInfo(archivePath string) (exportoptionsgenerator.ArchiveInfo, error) {
	platform, err := InferArchivePlatform(archivePath)
	if err == nil && platform == "MAC_OS" {
		macArchive, macErr := readMacArchiveExportInfo(context.Background(), archivePath)
		if macErr != nil {
			return exportoptionsgenerator.ArchiveInfo{}, macErr
		}
		return macArchive.ArchiveInfo, nil
	}
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
// names to the pinned resolver's profile classification. Bitrise still labels
// installed iOS and tvOS profiles with the pre-Xcode 15.3 names (app-store and
// ad-hoc) and filters code signing groups by exact equality, even though the
// final ExportOptions.plist must use app-store-connect or release-testing.
func manualExportOptionsResolverMethod(method string) legacyexportoptions.Method {
	if method == exportOptionsMethodReleaseTesting {
		return legacyexportoptions.MethodAdHoc
	}
	return legacyexportoptions.MethodAppStore
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
		TeamID:                      strings.TrimSpace(coercePlistValueToString(payload["teamID"])),
		SigningCertificate:          strings.TrimSpace(coercePlistValueToString(payload["signingCertificate"])),
		InstallerSigningCertificate: strings.TrimSpace(coercePlistValueToString(payload["installerSigningCertificate"])),
		ProvisioningProfiles:        profiles,
		ICloudContainerEnvironment:  cloudEnvironment,
	}, nil
}
