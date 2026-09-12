package xcode

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

const (
	exportOptionsMethodAppStoreConnect = "app-store-connect"
	exportOptionsMethodReleaseTesting  = "release-testing"
	exportOptionsDestinationExport     = "export"
	exportOptionsDestinationUpload     = "upload"
	exportOptionsSigningStyleAutomatic = "automatic"
	exportOptionsSigningStyleManual    = "manual"
)

// ExportOptionsGenerateOptions describes an ExportOptions.plist generated from
// an existing Xcode archive.
type ExportOptionsGenerateOptions struct {
	ArchivePath  string
	OutputPath   string
	Method       string
	Destination  string
	SigningStyle string
	TeamID       string
	Overwrite    bool
}

// ExportOptionsGenerateResult describes the generated ExportOptions.plist.
type ExportOptionsGenerateResult struct {
	Path                        string            `json:"path"`
	ArchivePath                 string            `json:"archive_path"`
	Method                      string            `json:"method"`
	Destination                 string            `json:"destination"`
	SigningStyle                string            `json:"signing_style"`
	TeamID                      string            `json:"team_id,omitempty"`
	SigningCertificate          string            `json:"signing_certificate,omitempty"`
	InstallerSigningCertificate string            `json:"installer_signing_certificate,omitempty"`
	ProvisioningProfiles        map[string]string `json:"provisioning_profiles,omitempty"`
	Overwritten                 bool              `json:"overwritten"`
}

// manualExportOptions contains the signing material ASC owns after Bitrise has
// resolved installed certificates and profiles. It deliberately does not leak
// Bitrise's v1 ExportOptions interface into ASC's public API.
type manualExportOptions struct {
	TeamID                       string
	SigningCertificate           string
	InstallerSigningCertificate  string
	ProvisioningProfiles         map[string]string
	ICloudContainerEnvironment   string
	ProvisioningProfilesOptional bool
}

var (
	manualExportOptionsGeneratorFn    = generateManualExportOptions
	preflightExportOptionsParentFn    = preflightExportOptionsParent
	atomicWriteExportOptionsFileFn    = atomicWriteVersionFile
	atomicWriteNewExportOptionsFileFn = atomicWriteNewExportOptionsFile
)

// DefaultExportOptionsPathForArchive returns the archive-adjacent default path
// used when an explicit ExportOptions.plist was not supplied.
func DefaultExportOptionsPathForArchive(archivePath string) string {
	archivePath = normalizeDirectoryPath(archivePath)
	extension := filepath.Ext(archivePath)
	return strings.TrimSuffix(archivePath, extension) + "-ExportOptions.plist"
}

// UniqueExportOptionsPathForArchive returns a non-existing archive-adjacent
// path suitable for implicit generation. The random suffix prevents an
// implicit export from replacing a user-managed ExportOptions.plist.
func UniqueExportOptionsPathForArchive(archivePath string) (string, error) {
	archivePath = normalizeDirectoryPath(archivePath)
	extension := filepath.Ext(archivePath)
	base := strings.TrimSuffix(archivePath, extension) + "-ExportOptions-"
	for range 32 {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", fmt.Errorf("generate export options path suffix: %w", err)
		}
		candidate := base + hex.EncodeToString(random) + ".plist"
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("check export options path: %w", err)
		}
	}
	return "", fmt.Errorf("could not find an unused export options path for archive: %s", archivePath)
}

// GenerateExportOptions creates a validated ExportOptions.plist without
// changing signing assets. Automatic signing only reads archive metadata;
// manual signing may inspect locally installed profiles and identities.
func GenerateExportOptions(ctx context.Context, opts ExportOptionsGenerateOptions) (*ExportOptionsGenerateResult, error) {
	opts = normalizeExportOptionsGenerateOptions(opts)
	if err := validateExportOptionsGenerateOptions(opts); err != nil {
		return nil, err
	}
	if err := validateExistingPath(opts.ArchivePath, ".xcarchive", "--archive-path"); err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if _, _, err := prepareExportOptionsDestination(opts.OutputPath, opts.Overwrite); err != nil {
		return nil, err
	}

	archiveTeamID, err := readArchiveExportOptionsTeamID(opts.ArchivePath)
	if err != nil {
		return nil, err
	}
	teamID := opts.TeamID
	if teamID == "" {
		teamID = archiveTeamID
	}

	var manual manualExportOptions
	if opts.SigningStyle == exportOptionsSigningStyleManual {
		if err := preflightExportOptionsParentFn(opts.OutputPath); err != nil {
			return nil, err
		}
		manual, err = manualExportOptionsGeneratorFn(ctx, opts.ArchivePath, teamID, opts.Method)
		if err != nil {
			return nil, fmt.Errorf("generate manual export options: %w", err)
		}
		if err := validateManualExportOptions(manual); err != nil {
			return nil, err
		}
		if teamID != "" && manual.TeamID != teamID {
			return nil, fmt.Errorf("manual export options team ID %q does not match requested team ID %q", manual.TeamID, teamID)
		}
		teamID = manual.TeamID
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	payload := buildExportOptionsPayload(opts, teamID, manual)
	if err := validateExportOptionsPayload(payload, opts.Method, opts.SigningStyle, manual.ProvisioningProfilesOptional); err != nil {
		return nil, err
	}
	data, err := plist.MarshalIndent(payload, plist.XMLFormat, "\t")
	if err != nil {
		return nil, fmt.Errorf("serialize export options plist: %w", err)
	}
	var decoded map[string]any
	if _, err := plist.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode generated export options plist: %w", err)
	}
	if err := validateExportOptionsPayload(decoded, opts.Method, opts.SigningStyle, manual.ProvisioningProfilesOptional); err != nil {
		return nil, fmt.Errorf("validate generated export options plist: %w", err)
	}

	overwritten, mode, err := prepareExportOptionsDestination(opts.OutputPath, opts.Overwrite)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create export options parent directory: %w", err)
	}
	writeFile := atomicWriteNewExportOptionsFileFn
	if overwritten {
		writeFile = atomicWriteExportOptionsFileFn
	}
	if err := writeFile(opts.OutputPath, data, mode); err != nil {
		return nil, fmt.Errorf("write export options: %w", err)
	}

	result := &ExportOptionsGenerateResult{
		Path:         opts.OutputPath,
		ArchivePath:  opts.ArchivePath,
		Method:       opts.Method,
		Destination:  opts.Destination,
		SigningStyle: opts.SigningStyle,
		TeamID:       teamID,
		Overwritten:  overwritten,
	}
	if opts.SigningStyle == exportOptionsSigningStyleManual {
		result.SigningCertificate = manual.SigningCertificate
		result.InstallerSigningCertificate = manual.InstallerSigningCertificate
		result.ProvisioningProfiles = cloneProvisioningProfiles(manual.ProvisioningProfiles)
	}
	return result, nil
}

// NormalizeExportOptionsSigningStyle validates the public signing-style enum
// shared by export-options generation callers.
func NormalizeExportOptionsSigningStyle(value string) (string, error) {
	style := strings.TrimSpace(value)
	switch style {
	case exportOptionsSigningStyleAutomatic, exportOptionsSigningStyleManual:
		return style, nil
	default:
		return style, fmt.Errorf("--signing-style must be one of: automatic, manual")
	}
}

// NormalizeExportOptionsMethod validates the public method enum shared by
// export-options generation callers.
func NormalizeExportOptionsMethod(value string) (string, error) {
	method := strings.ToLower(strings.TrimSpace(value))
	switch method {
	case exportOptionsMethodAppStoreConnect, exportOptionsMethodReleaseTesting:
		return method, nil
	case "ad-hoc":
		return method, fmt.Errorf("--method must be one of: app-store-connect, release-testing; use release-testing instead of deprecated ad-hoc")
	default:
		return method, fmt.Errorf("--method must be one of: app-store-connect, release-testing")
	}
}

func normalizeExportOptionsGenerateOptions(opts ExportOptionsGenerateOptions) ExportOptionsGenerateOptions {
	opts.ArchivePath = normalizeDirectoryPath(opts.ArchivePath)
	opts.OutputPath = strings.TrimSpace(opts.OutputPath)
	if opts.OutputPath != "" {
		opts.OutputPath = filepath.Clean(opts.OutputPath)
	}
	opts.Method = strings.ToLower(strings.TrimSpace(opts.Method))
	if opts.Method == "" {
		opts.Method = exportOptionsMethodAppStoreConnect
	}
	opts.Destination = strings.ToLower(strings.TrimSpace(opts.Destination))
	if opts.Destination == "" {
		opts.Destination = exportOptionsDestinationExport
	}
	opts.SigningStyle = strings.ToLower(strings.TrimSpace(opts.SigningStyle))
	if opts.SigningStyle == "" {
		opts.SigningStyle = exportOptionsSigningStyleAutomatic
	}
	opts.TeamID = strings.TrimSpace(opts.TeamID)
	return opts
}

func validateExportOptionsGenerateOptions(opts ExportOptionsGenerateOptions) error {
	if opts.ArchivePath == "" {
		return fmt.Errorf("--archive-path is required")
	}
	if !strings.EqualFold(filepath.Ext(opts.ArchivePath), ".xcarchive") {
		return fmt.Errorf("--archive-path must end with .xcarchive")
	}
	if opts.OutputPath == "" {
		return fmt.Errorf("--output-path is required")
	}
	if _, err := NormalizeExportOptionsMethod(opts.Method); err != nil {
		return err
	}
	switch opts.Destination {
	case exportOptionsDestinationExport, exportOptionsDestinationUpload:
	default:
		return fmt.Errorf("--destination must be one of: export, upload")
	}
	if _, err := NormalizeExportOptionsSigningStyle(opts.SigningStyle); err != nil {
		return err
	}
	if opts.Method == exportOptionsMethodReleaseTesting && opts.Destination != exportOptionsDestinationExport {
		return fmt.Errorf("release-testing requires destination=export")
	}
	return nil
}

func readArchiveExportOptionsTeamID(archivePath string) (string, error) {
	archiveRoot, err := os.OpenRoot(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = archiveRoot.Close() }()
	data, err := readRegularFileFromRoot(archiveRoot, "Info.plist")
	if err != nil {
		return "", fmt.Errorf("read archive Info.plist: %w", err)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("decode archive Info.plist: %w", err)
	}
	applicationProperties, ok := payload["ApplicationProperties"].(map[string]any)
	if !ok || applicationProperties == nil {
		return "", fmt.Errorf("archive Info.plist missing ApplicationProperties")
	}
	applicationPath := strings.TrimSpace(coercePlistValueToString(applicationProperties["ApplicationPath"]))
	if applicationPath == "" {
		return "", fmt.Errorf("archive Info.plist missing ApplicationPath")
	}
	bundleID := strings.TrimSpace(coercePlistValueToString(applicationProperties["CFBundleIdentifier"]))
	if bundleID == "" {
		return "", fmt.Errorf("archive Info.plist missing CFBundleIdentifier")
	}
	embeddedAppInfo, err := readArchivedAppInfoPlistFromRoot(archiveRoot, applicationProperties)
	if err != nil {
		return "", fmt.Errorf("validate archived application: %w", err)
	}
	embeddedBundleID := strings.TrimSpace(coercePlistValueToString(embeddedAppInfo["CFBundleIdentifier"]))
	if embeddedBundleID == "" {
		return "", fmt.Errorf("archived app Info.plist missing CFBundleIdentifier")
	}
	if embeddedBundleID != bundleID {
		return "", fmt.Errorf("archived app bundle identifier %q does not match archive bundle identifier %q", embeddedBundleID, bundleID)
	}
	if platform := inferAppStorePlatformFromPlist(embeddedAppInfo); platform == "" {
		return "", fmt.Errorf("archived app Info.plist did not contain a supported platform marker")
	}
	return strings.TrimSpace(coercePlistValueToString(applicationProperties["Team"])), nil
}

func buildExportOptionsPayload(opts ExportOptionsGenerateOptions, teamID string, manual manualExportOptions) map[string]any {
	return buildPlatformExportOptionsPayload(opts, teamID, manual)
}

func validateExportOptionsPayload(payload map[string]any, method, signingStyle string, provisioningProfilesOptional bool) error {
	if got := exportOptionsString(payload["method"]); got != method {
		return fmt.Errorf("export options method must be %q", method)
	}
	destination := exportOptionsString(payload["destination"])
	if destination != exportOptionsDestinationExport && destination != exportOptionsDestinationUpload {
		return fmt.Errorf("export options destination must be export or upload")
	}
	style := exportOptionsString(payload["signingStyle"])
	if style != signingStyle {
		return fmt.Errorf("export options signing style must be %q", signingStyle)
	}
	if signingStyle == exportOptionsSigningStyleAutomatic {
		if _, found := payload["provisioningProfiles"]; found {
			return fmt.Errorf("automatic export options must not include provisioning profiles")
		}
		return nil
	}
	certificate := exportOptionsString(payload["signingCertificate"])
	if certificate == "" {
		return fmt.Errorf("manual export options require a signing certificate")
	}
	installerCertificate := exportOptionsString(payload["installerSigningCertificate"])
	profiles := map[string]string(nil)
	if profilesValue, found := payload["provisioningProfiles"]; found {
		var err error
		profiles, err = provisioningProfilesFromPayload(profilesValue)
		if err != nil {
			return err
		}
	}
	return validateManualExportOptions(manualExportOptions{
		SigningCertificate:           certificate,
		InstallerSigningCertificate:  installerCertificate,
		ProvisioningProfiles:         profiles,
		ProvisioningProfilesOptional: provisioningProfilesOptional,
	})
}

func exportOptionsString(value any) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

func validateManualExportOptions(options manualExportOptions) error {
	if strings.TrimSpace(options.SigningCertificate) == "" {
		return fmt.Errorf("manual export options require a signing certificate")
	}
	if options.ProvisioningProfilesOptional && strings.TrimSpace(options.InstallerSigningCertificate) == "" {
		return fmt.Errorf("manual macOS export options require an installer signing certificate")
	}
	if len(options.ProvisioningProfiles) == 0 && !options.ProvisioningProfilesOptional {
		return fmt.Errorf("manual export options require provisioning profile mappings")
	}
	for bundleID, profile := range options.ProvisioningProfiles {
		if strings.TrimSpace(bundleID) == "" {
			return fmt.Errorf("manual export options contain an empty bundle identifier")
		}
		if strings.TrimSpace(profile) == "" {
			return fmt.Errorf("manual export options contain an empty provisioning profile for bundle %q", bundleID)
		}
	}
	return nil
}

func provisioningProfilesFromPayload(value any) (map[string]string, error) {
	switch profiles := value.(type) {
	case map[string]string:
		return cloneProvisioningProfiles(profiles), nil
	case map[string]any:
		result := make(map[string]string, len(profiles))
		for bundleID, profile := range profiles {
			result[bundleID] = coercePlistValueToString(profile)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("manual export options require provisioning profile mappings")
	}
}

func prepareExportOptionsDestination(path string, overwrite bool) (bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0o644, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("lstat export options path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, 0, fmt.Errorf("refusing to replace symlink: %s", path)
	}
	if !info.Mode().IsRegular() {
		return false, 0, fmt.Errorf("export options path must be a regular file: %s", path)
	}
	if !overwrite {
		return false, 0, fmt.Errorf("export options path already exists; pass --overwrite to replace it: %s", path)
	}
	return true, info.Mode().Perm(), nil
}

func preflightExportOptionsParent(path string) error {
	return preflightWritableParent(path, "export options")
}

// atomicWriteNewExportOptionsFile publishes a fully written same-directory
// temporary file without replacing a destination created after validation.
// Hard-link creation is atomic and fails when path already exists.
func atomicWriteNewExportOptionsFile(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".asc-export-options-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err == nil {
		removeTemporary = false
	}
	if directoryHandle, err := os.Open(directory); err == nil {
		_ = directoryHandle.Sync()
		_ = directoryHandle.Close()
	}
	return nil
}

func cloneProvisioningProfiles(profiles map[string]string) map[string]string {
	if len(profiles) == 0 {
		return nil
	}
	clone := make(map[string]string, len(profiles))
	for bundleID, profile := range profiles {
		clone[bundleID] = profile
	}
	return clone
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
