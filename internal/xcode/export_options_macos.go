//go:build darwin

package xcode

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bitrise-io/go-xcode/certificateutil"
	legacyexportoptions "github.com/bitrise-io/go-xcode/exportoptions"
	"github.com/bitrise-io/go-xcode/v2/exportoptionsgenerator"
	"github.com/bitrise-io/go-xcode/v2/plistutil"
	"github.com/bitrise-io/go-xcode/v2/profileutil"
	"github.com/fullsailor/pkcs7"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
	"github.com/ryanuber/go-glob"
	"howett.net/plist"
)

const (
	macEmbeddedProfileMaxBytes     int64 = 16 << 20
	macExecutableMaxBytes          int64 = 4 << 30
	macProfileInventoryMaxEntries        = 100_000
	macArchiveMaxEntries                 = 100_000
	macArchiveMaxDepth                   = 128
	macCodesignOutputMaxBytes            = infoplist.MaxBytes + 64<<10
	macSecurityOutputMaxBytes            = 1 << 20
	macSecurityCertificateMaxBytes       = 16 << 20
)

type macArchiveExportInfo struct {
	exportoptionsgenerator.ArchiveInfo
	EmbeddedProfiles map[string]profileutil.ProvisioningProfileInfoModel
	SigningIdentity  string
}

var (
	readMacArchiveExportInfoFn             = readMacArchiveExportInfo
	installedMacProvisioningProfileInfosFn = installedMacProvisioningProfileInfos
	installedCodesignIdentityInfosFn       = installedCodesignIdentityInfos
	installedInstallerIdentityInfosFn      = installedInstallerIdentityInfos
	macCodesignCommandContextFn            = exec.CommandContext
	macSecurityCommandContextFn            = exec.CommandContext
	macSecurityRunCommandFn                = func(command *exec.Cmd) error { return command.Run() }
	macUserHomeDirFn                       = os.UserHomeDir
	afterMacArchiveInitialLstatFn          func()
)

func generateMacManualExportOptions(ctx context.Context, archivePath, teamID string) (manualExportOptions, error) {
	if err := contextError(ctx); err != nil {
		return manualExportOptions{}, err
	}

	var manual manualExportOptions
	if _, err := captureBitriseStdout(func() error {
		archiveInfo, err := readMacArchiveExportInfoFn(ctx, archivePath)
		if err != nil {
			return err
		}
		profiles, err := installedMacProvisioningProfileInfosFn(ctx)
		if err != nil {
			return fmt.Errorf("read installed macOS provisioning profiles: %w", err)
		}
		certificates, err := installedCodesignIdentityInfosFn(ctx)
		if err != nil {
			return fmt.Errorf("read installed code-signing identities: %w", err)
		}
		installerCertificates, err := installedInstallerIdentityInfosFn(ctx)
		if err != nil {
			return fmt.Errorf("read installed installer-signing identities: %w", err)
		}
		manual, err = resolveMacManualExportOptions(archiveInfo, profiles, certificates, installerCertificates, teamID)
		return err
	}); err != nil {
		return manualExportOptions{}, err
	}
	return manual, nil
}

func readMacArchiveExportInfo(ctx context.Context, archivePath string) (macArchiveExportInfo, error) {
	if err := contextError(ctx); err != nil {
		return macArchiveExportInfo{}, err
	}
	archivePath = filepath.Clean(strings.TrimSpace(archivePath))
	info, err := os.Lstat(archivePath)
	if err != nil {
		return macArchiveExportInfo{}, fmt.Errorf("inspect macOS archive: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return macArchiveExportInfo{}, fmt.Errorf("macOS archive path must be a non-symlinked directory")
	}
	if afterMacArchiveInitialLstatFn != nil {
		afterMacArchiveInitialLstatFn()
	}
	archiveRoot, err := rootfs.New(archivePath)
	if err != nil {
		return macArchiveExportInfo{}, fmt.Errorf("open macOS archive: %w", err)
	}
	defer archiveRoot.Close()
	openedArchiveRoot, err := openNonSymlinkedSelectedRoot(archiveRoot, archivePath, info)
	if err != nil {
		return macArchiveExportInfo{}, fmt.Errorf("verify macOS archive root: %w", err)
	}
	if err := openedArchiveRoot.Close(); err != nil {
		return macArchiveExportInfo{}, fmt.Errorf("close verified macOS archive root: %w", err)
	}

	applicationProperties, applicationPath, err := readMacArchiveApplicationProperties(archiveRoot)
	if err != nil {
		return macArchiveExportInfo{}, err
	}
	application, ok, err := readMacBundleExportInfo(ctx, archiveRoot, applicationPath, true)
	if err != nil {
		return macArchiveExportInfo{}, fmt.Errorf("read macOS archived application: %w", err)
	}
	if !ok {
		return macArchiveExportInfo{}, fmt.Errorf("macOS archived application is missing executable bundle metadata")
	}
	expectedBundleID := strings.TrimSpace(coercePlistValueToString(applicationProperties["CFBundleIdentifier"]))
	if application.BundleID == "" || expectedBundleID == "" || application.BundleID != expectedBundleID {
		return macArchiveExportInfo{}, fmt.Errorf("macOS archived application bundle identifier %q does not match archive bundle identifier %q", application.BundleID, expectedBundleID)
	}

	result := macArchiveExportInfo{
		ArchiveInfo: exportoptionsgenerator.ArchiveInfo{
			AppBundleID: application.BundleID,
			EntitlementsByBundleID: map[string]plistutil.PlistData{
				application.BundleID: application.Entitlements,
			},
		},
		EmbeddedProfiles: make(map[string]profileutil.ProvisioningProfileInfoModel),
		SigningIdentity:  strings.TrimSpace(coercePlistValueToString(applicationProperties["SigningIdentity"])),
	}
	if application.Profile != nil {
		result.EmbeddedProfiles[application.BundleID] = *application.Profile
	}
	nestedPaths, err := discoverNestedMacBundlePaths(ctx, archiveRoot, applicationPath)
	if err != nil {
		return macArchiveExportInfo{}, err
	}
	for _, nestedPath := range nestedPaths {
		bundle, executable, err := readMacBundleExportInfo(ctx, archiveRoot, nestedPath, false)
		if err != nil {
			return macArchiveExportInfo{}, fmt.Errorf("read nested macOS executable bundle %q: %w", nestedPath, err)
		}
		if !executable {
			continue
		}
		if _, duplicate := result.EntitlementsByBundleID[bundle.BundleID]; duplicate {
			return macArchiveExportInfo{}, fmt.Errorf("duplicate macOS executable bundle identifier %q", bundle.BundleID)
		}
		result.EntitlementsByBundleID[bundle.BundleID] = bundle.Entitlements
		if bundle.Profile != nil {
			result.EmbeddedProfiles[bundle.BundleID] = *bundle.Profile
		}
	}
	return result, nil
}

type macBundleExportInfo struct {
	BundleID     string
	Entitlements plistutil.PlistData
	Profile      *profileutil.ProvisioningProfileInfoModel
}

func readMacArchiveApplicationProperties(archiveRoot rootfs.Root) (map[string]any, string, error) {
	payload, err := readMacArchivePlist(archiveRoot, "Info.plist")
	if err != nil {
		return nil, "", fmt.Errorf("read macOS archive Info.plist: %w", err)
	}
	applicationProperties, ok := payload["ApplicationProperties"].(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("macOS archive Info.plist missing ApplicationProperties")
	}
	applicationRelativePath := strings.TrimSpace(coercePlistValueToString(applicationProperties["ApplicationPath"]))
	cleanPath := filepath.Clean(filepath.FromSlash(applicationRelativePath))
	if applicationRelativePath == "" || strings.ContainsRune(applicationRelativePath, 0) || filepath.IsAbs(cleanPath) || filepath.VolumeName(cleanPath) != "" || cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("macOS archive Info.plist contains unsafe ApplicationPath %q", applicationRelativePath)
	}
	applicationPath := filepath.ToSlash(filepath.Join("Products", cleanPath))
	handle, err := archiveRoot.OpenDir(filepath.FromSlash(applicationPath))
	if err != nil {
		return nil, "", fmt.Errorf("open macOS archived application: %w", err)
	}
	if err := handle.Close(); err != nil {
		return nil, "", fmt.Errorf("close macOS archived application: %w", err)
	}
	return applicationProperties, applicationPath, nil
}

func readMacBundleExportInfo(ctx context.Context, archiveRoot rootfs.Root, bundlePath string, required bool) (macBundleExportInfo, bool, error) {
	infoPath := filepath.ToSlash(filepath.Join(bundlePath, "Contents", "Info.plist"))
	payload, err := readMacArchivePlist(archiveRoot, infoPath)
	if errors.Is(err, os.ErrNotExist) && !required {
		return macBundleExportInfo{}, false, nil
	}
	if err != nil {
		return macBundleExportInfo{}, false, err
	}
	bundleID := strings.TrimSpace(coercePlistValueToString(payload["CFBundleIdentifier"]))
	if bundleID == "" {
		return macBundleExportInfo{}, false, fmt.Errorf("info.plist is missing CFBundleIdentifier")
	}
	executableName := strings.TrimSpace(coercePlistValueToString(payload["CFBundleExecutable"]))
	if executableName == "" && !required {
		return macBundleExportInfo{}, false, nil
	}
	if executableName == "" || executableName == "." || executableName == ".." || executableName != filepath.Base(executableName) || strings.ContainsAny(executableName, `/\\`) || strings.ContainsRune(executableName, 0) {
		return macBundleExportInfo{}, false, fmt.Errorf("info.plist contains unsafe CFBundleExecutable %q", executableName)
	}
	executablePath := filepath.ToSlash(filepath.Join(bundlePath, "Contents", "MacOS", executableName))
	executable, err := archiveRoot.OpenFile(filepath.FromSlash(executablePath))
	if errors.Is(err, os.ErrNotExist) && !required {
		return macBundleExportInfo{}, false, nil
	}
	if err != nil {
		return macBundleExportInfo{}, false, fmt.Errorf("open executable: %w", err)
	}
	defer executable.Close()
	entitlements, err := readMacExecutableEntitlements(ctx, executable)
	if err != nil {
		return macBundleExportInfo{}, false, fmt.Errorf("read signed entitlements: %w", err)
	}
	profile, err := readEmbeddedMacProfile(archiveRoot, filepath.ToSlash(filepath.Join(bundlePath, "Contents", "embedded.provisionprofile")))
	if err != nil {
		return macBundleExportInfo{}, false, err
	}
	return macBundleExportInfo{BundleID: bundleID, Entitlements: entitlements, Profile: profile}, true, nil
}

func readMacArchivePlist(archiveRoot rootfs.Root, relativePath string) (map[string]any, error) {
	data, err := archiveRoot.ReadFileLimited(filepath.FromSlash(relativePath), infoplist.MaxBytes)
	if err != nil {
		return nil, err
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return nil, fmt.Errorf("validate %s: %w", relativePath, err)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", relativePath, err)
	}
	if payload == nil {
		return nil, fmt.Errorf("decode %s: expected a plist dictionary", relativePath)
	}
	return payload, nil
}

func readEmbeddedMacProfile(archiveRoot rootfs.Root, relativePath string) (*profileutil.ProvisioningProfileInfoModel, error) {
	data, err := archiveRoot.ReadFileLimited(filepath.FromSlash(relativePath), macEmbeddedProfileMaxBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read embedded macOS provisioning profile: %w", err)
	}
	profilePKCS7, err := pkcs7.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse embedded macOS provisioning profile: %w", err)
	}
	if err := infoplist.ValidateStructure(profilePKCS7.Content); err != nil {
		return nil, fmt.Errorf("validate embedded macOS provisioning profile: %w", err)
	}
	profile, err := profileutil.NewProvisioningProfileInfo(*profilePKCS7)
	if err != nil {
		return nil, fmt.Errorf("decode embedded macOS provisioning profile: %w", err)
	}
	return &profile, nil
}

func discoverNestedMacBundlePaths(ctx context.Context, archiveRoot rootfs.Root, applicationPath string) ([]string, error) {
	type directory struct {
		path  string
		depth int
	}
	queue := []directory{{path: applicationPath}}
	var bundlePaths []string
	entryCount := 0
	for len(queue) > 0 {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		current := queue[0]
		queue = queue[1:]
		if current.depth > macArchiveMaxDepth {
			return nil, fmt.Errorf("macOS archive directory depth exceeds %d", macArchiveMaxDepth)
		}
		directoryHandle, err := archiveRoot.OpenDir(filepath.FromSlash(current.path))
		if err != nil {
			return nil, fmt.Errorf("open macOS archive directory %q: %w", current.path, err)
		}
		for {
			if err := contextError(ctx); err != nil {
				_ = directoryHandle.Close()
				return nil, err
			}
			entries, readErr := directoryHandle.ReadDir(256)
			if err := contextError(ctx); err != nil {
				_ = directoryHandle.Close()
				return nil, err
			}
			for _, entry := range entries {
				if err := contextError(ctx); err != nil {
					_ = directoryHandle.Close()
					return nil, err
				}
				entryCount++
				if entryCount > macArchiveMaxEntries {
					_ = directoryHandle.Close()
					return nil, fmt.Errorf("macOS archive contains more than %d entries", macArchiveMaxEntries)
				}
				entryPath := filepath.ToSlash(filepath.Join(current.path, entry.Name()))
				if entry.Type()&os.ModeSymlink != 0 {
					if isMacBundleLikePath(entryPath) {
						_ = directoryHandle.Close()
						return nil, fmt.Errorf("refusing symlinked macOS executable bundle %q", entryPath)
					}
					continue
				}
				if !entry.IsDir() {
					continue
				}
				// Bundle extensions are extensible and custom executable bundles are
				// valid. Treat every directory as a candidate, then require a safe
				// Contents/Info.plist and matching Contents/MacOS executable before
				// including it in the export inventory.
				bundlePaths = append(bundlePaths, entryPath)
				queue = append(queue, directory{path: entryPath, depth: current.depth + 1})
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				_ = directoryHandle.Close()
				return nil, fmt.Errorf("read macOS archive directory %q: %w", current.path, readErr)
			}
		}
		if err := directoryHandle.Close(); err != nil {
			return nil, fmt.Errorf("close macOS archive directory %q: %w", current.path, err)
		}
	}
	sort.Strings(bundlePaths)
	return bundlePaths, nil
}

func isMacBundleLikePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".app", ".appex", ".bundle", ".dext", ".mdimporter", ".plugin", ".qlgenerator", ".systemextension", ".xpc":
		return true
	default:
		return false
	}
}

func readMacExecutableEntitlements(ctx context.Context, executable *os.File) (plistutil.PlistData, error) {
	snapshotPath, cleanup, err := snapshotMacExecutable(ctx, executable)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if ctx == nil {
		ctx = context.Background()
	}
	command := macCodesignCommandContextFn(ctx, "/usr/bin/codesign", "-d", "--entitlements", ":-", snapshotPath)
	stdout := &macBoundedCapture{limit: macCodesignOutputMaxBytes}
	stderr := &macBoundedCapture{limit: macCodesignOutputMaxBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, fmt.Errorf("codesign failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("codesign output exceeded the %d-byte limit", macCodesignOutputMaxBytes)
	}
	data := bytes.TrimSpace(stdout.Bytes())
	if len(data) == 0 {
		data = extractMacPlistDocument(stderr.Bytes())
	}
	if len(data) == 0 {
		return plistutil.PlistData{}, nil
	}
	if len(data) > infoplist.MaxBytes {
		return nil, fmt.Errorf("signed entitlements exceed %d bytes", infoplist.MaxBytes)
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return nil, fmt.Errorf("validate signed entitlements: %w", err)
	}
	var entitlements plistutil.PlistData
	if _, err := plist.Unmarshal(data, &entitlements); err != nil {
		return nil, fmt.Errorf("decode signed entitlements: %w", err)
	}
	if entitlements == nil {
		entitlements = plistutil.PlistData{}
	}
	return entitlements, nil
}

func snapshotMacExecutable(ctx context.Context, executable *os.File) (string, func(), error) {
	if err := contextError(ctx); err != nil {
		return "", nil, err
	}
	if executable == nil {
		return "", nil, fmt.Errorf("macOS executable is nil")
	}
	before, err := executable.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("inspect macOS executable: %w", err)
	}
	if !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > macExecutableMaxBytes {
		return "", nil, fmt.Errorf("macOS executable size must be between 1 and %d bytes", macExecutableMaxBytes)
	}
	if _, err := executable.Seek(0, io.SeekStart); err != nil {
		return "", nil, fmt.Errorf("seek macOS executable: %w", err)
	}
	directory, err := os.MkdirTemp("", ".asc-mac-executable-*")
	if err != nil {
		return "", nil, fmt.Errorf("create private macOS executable directory: %w", err)
	}
	cleanupDirectory := func() { _ = os.Remove(directory) }
	temporaryRoot, err := os.OpenRoot(directory)
	if err != nil {
		cleanupDirectory()
		return "", nil, fmt.Errorf("open private macOS executable directory: %w", err)
	}
	cleanup := func() {
		_ = temporaryRoot.Remove("Executable")
		_ = temporaryRoot.Close()
		cleanupDirectory()
	}
	temporary, err := secureopen.OpenNewPrivateFileNoFollowInRoot(temporaryRoot, "Executable", 0o700)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create private macOS executable snapshot: %w", err)
	}
	if err := secureopen.PreparePrivateFile(temporary, 0o700); err != nil {
		_ = temporary.Close()
		cleanup()
		return "", nil, fmt.Errorf("secure private macOS executable snapshot: %w", err)
	}
	written, copyErr := copyMacExecutableContext(ctx, temporary, io.LimitReader(executable, before.Size()+1))
	if copyErr == nil && written != before.Size() {
		copyErr = fmt.Errorf("copied %d of %d bytes", written, before.Size())
	}
	if copyErr == nil {
		after, statErr := executable.Stat()
		if statErr != nil {
			copyErr = statErr
		} else if before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
			copyErr = fmt.Errorf("macOS executable changed during snapshot")
		}
	}
	if copyErr == nil {
		copyErr = temporary.Sync()
	}
	closeErr := temporary.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("snapshot macOS executable: %w", copyErr)
	}
	return filepath.Join(directory, "Executable"), cleanup, nil
}

func copyMacExecutableContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	buffer := make([]byte, 64<<10)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if err := ctx.Err(); err != nil {
				return written, err
			}
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

type macBoundedCapture struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (capture *macBoundedCapture) Write(data []byte) (int, error) {
	original := len(data)
	remaining := capture.limit - capture.Len()
	if remaining <= 0 {
		capture.overflow = true
		return original, nil
	}
	if len(data) > remaining {
		data = data[:remaining]
		capture.overflow = true
	}
	_, _ = capture.Buffer.Write(data)
	return original, nil
}

func extractMacPlistDocument(data []byte) []byte {
	start := bytes.Index(data, []byte("<?xml"))
	if start < 0 {
		start = bytes.Index(data, []byte("<plist"))
	}
	endMarker := []byte("</plist>")
	end := bytes.LastIndex(data, endMarker)
	if start < 0 || end < start {
		return nil
	}
	return bytes.TrimSpace(data[start : end+len(endMarker)])
}

func installedMacProvisioningProfileInfos(ctx context.Context) ([]profileutil.ProvisioningProfileInfoModel, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	home, err := macUserHomeDirFn()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	directories := []string{
		filepath.Join(home, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"),
		filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles"),
	}
	var profiles []profileutil.ProvisioningProfileInfoModel
	for _, directory := range directories {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		found, err := readInstalledMacProvisioningProfiles(ctx, directory)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, found...)
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].UUID != profiles[j].UUID {
			return profiles[i].UUID < profiles[j].UUID
		}
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

func readInstalledMacProvisioningProfiles(ctx context.Context, directory string) ([]profileutil.ProvisioningProfileInfoModel, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect macOS provisioning profile directory %q: %w", directory, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("macOS provisioning profile directory %q must be a non-symlinked directory", directory)
	}
	profileRoot, err := rootfs.New(directory)
	if err != nil {
		return nil, fmt.Errorf("open macOS provisioning profile directory %q: %w", directory, err)
	}
	defer profileRoot.Close()
	openedRoot, err := openNonSymlinkedSelectedRoot(profileRoot, directory, info)
	if err != nil {
		return nil, fmt.Errorf("verify macOS provisioning profile directory %q: %w", directory, err)
	}
	defer openedRoot.Close()
	directoryHandle, err := openedRoot.Open(".")
	if err != nil {
		return nil, fmt.Errorf("read macOS provisioning profile directory %q: %w", directory, err)
	}
	var entries []os.DirEntry
	entryCount := 0
	for {
		if err := contextError(ctx); err != nil {
			_ = directoryHandle.Close()
			return nil, err
		}
		batch, readErr := directoryHandle.ReadDir(256)
		if err := contextError(ctx); err != nil {
			_ = directoryHandle.Close()
			return nil, err
		}
		entryCount += len(batch)
		if entryCount > macProfileInventoryMaxEntries {
			_ = directoryHandle.Close()
			return nil, fmt.Errorf("macOS provisioning profile directory %q contains more than %d entries", directory, macProfileInventoryMaxEntries)
		}
		entries = append(entries, batch...)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			_ = directoryHandle.Close()
			return nil, fmt.Errorf("read macOS provisioning profile directory %q: %w", directory, readErr)
		}
	}
	if err := directoryHandle.Close(); err != nil {
		return nil, fmt.Errorf("close macOS provisioning profile directory %q: %w", directory, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	profiles := make([]profileutil.ProvisioningProfileInfoModel, 0, len(entries))
	for _, entry := range entries {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != profileutil.MacExtension && extension != profileutil.IOSExtension {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing symlinked macOS provisioning profile %q", path)
		}
		file, err := secureopen.OpenExistingNoFollowInRoot(openedRoot, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("open macOS provisioning profile %q: %w", path, err)
		}
		openedInfo, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			return nil, fmt.Errorf("inspect macOS provisioning profile %q: %w", path, statErr)
		}
		if !openedInfo.Mode().IsRegular() {
			_ = file.Close()
			return nil, fmt.Errorf("macOS provisioning profile %q must be a regular file", path)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, macEmbeddedProfileMaxBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read macOS provisioning profile %q: %w", path, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close macOS provisioning profile %q: %w", path, closeErr)
		}
		if int64(len(data)) > macEmbeddedProfileMaxBytes {
			return nil, fmt.Errorf("macOS provisioning profile %q exceeds the %d-byte size limit", path, macEmbeddedProfileMaxBytes)
		}
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		profilePKCS7, err := pkcs7.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parse macOS provisioning profile %q: %w", path, err)
		}
		if err := infoplist.ValidateStructure(profilePKCS7.Content); err != nil {
			return nil, fmt.Errorf("validate macOS provisioning profile %q: %w", path, err)
		}
		profile, err := profileutil.NewProvisioningProfileInfo(*profilePKCS7)
		if err != nil {
			return nil, fmt.Errorf("decode macOS provisioning profile %q: %w", path, err)
		}
		if profile.Type != profileutil.ProfileTypeMacOs {
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func openNonSymlinkedSelectedRoot(root rootfs.Root, path string, expected os.FileInfo) (*os.Root, error) {
	opened, err := root.OpenRoot()
	if err != nil {
		return nil, err
	}
	openedInfo, err := opened.Stat(".")
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	selectedInfo, err := os.Lstat(path)
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	if expected == nil || expected.Mode()&os.ModeSymlink != 0 || !expected.IsDir() || selectedInfo.Mode()&os.ModeSymlink != 0 || !selectedInfo.IsDir() || !os.SameFile(expected, openedInfo) || !os.SameFile(expected, selectedInfo) {
		_ = opened.Close()
		return nil, fmt.Errorf("selected path is no longer the same non-symlinked directory")
	}
	return opened, nil
}

func installedCodesignIdentityInfos(ctx context.Context) ([]certificateutil.CertificateInfoModel, error) {
	return installedIdentityInfos(ctx, "codesigning", installedCertificateInfos)
}

func installedInstallerIdentityInfos(ctx context.Context) ([]certificateutil.CertificateInfoModel, error) {
	return installedIdentityInfos(ctx, "macappstore", installedCertificateInfos)
}

func installedIdentityInfos(
	ctx context.Context,
	policy string,
	readCertificates func(context.Context) ([]certificateutil.CertificateInfoModel, error),
) ([]certificateutil.CertificateInfoModel, error) {
	fingerprints, err := installedIdentityFingerprints(ctx, policy)
	if err != nil {
		return nil, err
	}
	if len(fingerprints) == 0 {
		return []certificateutil.CertificateInfoModel{}, nil
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	certificates, err := readCertificates(ctx)
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	byFingerprint := make(map[string]certificateutil.CertificateInfoModel, len(certificates))
	for _, certificate := range certificates {
		fingerprint := strings.ToUpper(strings.TrimSpace(certificate.SHA1Fingerprint))
		if sha1IdentitySelectorPattern.MatchString(fingerprint) {
			byFingerprint[fingerprint] = certificate
		}
	}
	identities := make([]certificateutil.CertificateInfoModel, 0, len(fingerprints))
	for _, fingerprint := range fingerprints {
		certificate, ok := byFingerprint[fingerprint]
		if !ok {
			return nil, fmt.Errorf("installed %s identity %s did not match an accessible certificate", policy, fingerprint)
		}
		identities = append(identities, certificate)
	}
	return identities, nil
}

func installedCertificateInfos(ctx context.Context) ([]certificateutil.CertificateInfoModel, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	command := macSecurityCommandContextFn(ctx, "/usr/bin/security", "find-certificate", "-a", "-p")
	stdout := &macBoundedCapture{limit: macSecurityCertificateMaxBytes}
	stderr := &macBoundedCapture{limit: macSecurityOutputMaxBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := macSecurityRunCommandFn(command); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, fmt.Errorf("security find-certificate failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("security find-certificate output exceeded its size limit")
	}
	return parseInstalledCertificateInfos(ctx, stdout.Bytes())
}

func parseInstalledCertificateInfos(ctx context.Context, data []byte) ([]certificateutil.CertificateInfoModel, error) {
	remaining := bytes.TrimSpace(data)
	certificates := make([]certificateutil.CertificateInfoModel, 0)
	for len(remaining) > 0 {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		block, rest := pem.Decode(remaining)
		if block == nil {
			return nil, fmt.Errorf("security returned malformed certificate data")
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("security returned unexpected PEM block %q", block.Type)
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse installed certificate: %w", err)
		}
		certificates = append(certificates, certificateutil.NewCertificateInfo(*certificate, nil))
		remaining = bytes.TrimSpace(rest)
	}
	return certificates, nil
}

func installedIdentityFingerprints(ctx context.Context, policy string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	switch policy {
	case "codesigning", "macappstore":
	default:
		return nil, fmt.Errorf("unsupported security identity policy %q", policy)
	}
	command := macSecurityCommandContextFn(ctx, "/usr/bin/security", "find-identity", "-v", "-p", policy)
	stdout := &macBoundedCapture{limit: macSecurityOutputMaxBytes}
	stderr := &macBoundedCapture{limit: macSecurityOutputMaxBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := macSecurityRunCommandFn(command); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, fmt.Errorf("security find-identity -p %s failed: %w: %s", policy, err, strings.TrimSpace(stderr.String()))
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("security find-identity -p %s output exceeded the %d-byte limit", policy, macSecurityOutputMaxBytes)
	}
	return parseInstalledIdentityFingerprints(stdout.Bytes())
}

func parseInstalledIdentityFingerprints(data []byte) ([]string, error) {
	lines := strings.Split(string(data), "\n")
	seen := make(map[string]struct{})
	fingerprints := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		closeParen := strings.IndexByte(line, ')')
		if closeParen <= 0 {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(line[:closeParen])); err != nil {
			continue
		}
		fields := strings.Fields(line[closeParen+1:])
		if len(fields) == 0 || !sha1IdentitySelectorPattern.MatchString(fields[0]) {
			return nil, fmt.Errorf("security returned a malformed identity fingerprint")
		}
		fingerprint := strings.ToUpper(fields[0])
		if _, ok := seen[fingerprint]; ok {
			continue
		}
		seen[fingerprint] = struct{}{}
		fingerprints = append(fingerprints, fingerprint)
	}
	return fingerprints, nil
}

func resolveMacManualExportOptions(
	archiveInfo macArchiveExportInfo,
	profiles []profileutil.ProvisioningProfileInfoModel,
	certificates []certificateutil.CertificateInfoModel,
	installerCertificates []certificateutil.CertificateInfoModel,
	teamID string,
) (manualExportOptions, error) {
	if strings.TrimSpace(archiveInfo.AppBundleID) == "" {
		return manualExportOptions{}, fmt.Errorf("macOS archive is missing the application bundle identifier")
	}
	if len(archiveInfo.EntitlementsByBundleID) == 0 {
		return manualExportOptions{}, fmt.Errorf("macOS archive is missing application entitlements")
	}

	bundleIDs := make([]string, 0, len(archiveInfo.EntitlementsByBundleID))
	for bundleID := range archiveInfo.EntitlementsByBundleID {
		if strings.TrimSpace(bundleID) != "" {
			bundleIDs = append(bundleIDs, bundleID)
		}
	}
	sort.Strings(bundleIDs)
	if len(bundleIDs) == 0 {
		return manualExportOptions{}, fmt.Errorf("macOS archive is missing executable bundle identifiers")
	}

	sort.SliceStable(certificates, func(i, j int) bool {
		if certificates[i].CommonName != certificates[j].CommonName {
			return certificates[i].CommonName < certificates[j].CommonName
		}
		return certificates[i].Serial < certificates[j].Serial
	})
	matchedApplicationIdentity := false
	matchedInstallerIdentity := false
	for _, certificate := range certificates {
		if !isMacAppDistributionCertificate(certificate.CommonName) {
			continue
		}
		effectiveTeamID := strings.TrimSpace(teamID)
		if effectiveTeamID == "" {
			effectiveTeamID = strings.TrimSpace(certificate.TeamID)
		}
		if effectiveTeamID == "" || certificate.TeamID != effectiveTeamID {
			continue
		}
		matchedApplicationIdentity = true
		installerCertificate, ok := selectMacInstallerCertificate(installerCertificates, effectiveTeamID)
		if !ok {
			continue
		}
		matchedInstallerIdentity = true
		mapping := make(map[string]string, len(bundleIDs))
		allMatched := true
		for _, bundleID := range bundleIDs {
			targetEntitlements := archiveInfo.EntitlementsByBundleID[bundleID]
			_, hasEmbeddedProfile := archiveInfo.EmbeddedProfiles[bundleID]
			if !macBundleRequiresProvisioningProfile(targetEntitlements, hasEmbeddedProfile) {
				continue
			}
			profile, ok := selectMacProvisioningProfile(bundleID, archiveInfo.EmbeddedProfiles, profiles, certificate.Serial, effectiveTeamID, targetEntitlements)
			if !ok {
				allMatched = false
				break
			}
			value := strings.TrimSpace(profile.UUID)
			if value == "" {
				value = strings.TrimSpace(profile.Name)
			}
			if value == "" {
				allMatched = false
				break
			}
			mapping[bundleID] = value
		}
		if allMatched {
			return manualExportOptions{
				TeamID:                       effectiveTeamID,
				SigningCertificate:           macCertificateSpecifier(certificate),
				InstallerSigningCertificate:  macCertificateSpecifier(installerCertificate),
				ProvisioningProfiles:         mapping,
				ICloudContainerEnvironment:   macICloudContainerEnvironment(archiveInfo.EntitlementsByBundleID),
				ProvisioningProfilesOptional: len(mapping) == 0,
			}, nil
		}
	}

	if matchedApplicationIdentity && !matchedInstallerIdentity {
		return manualExportOptions{}, fmt.Errorf("could not find an installed Mac App Store installer certificate matching an installed macOS distribution identity for team %q", teamID)
	}
	return manualExportOptions{}, fmt.Errorf("could not find one installed macOS distribution certificate and required App Store provisioning profile set for team %q covering bundle identifiers %s", teamID, strings.Join(bundleIDs, ", "))
}

func macCertificateSpecifier(certificate certificateutil.CertificateInfoModel) string {
	if fingerprint := strings.TrimSpace(certificate.SHA1Fingerprint); fingerprint != "" {
		return fingerprint
	}
	return strings.TrimSpace(certificate.CommonName)
}

func macBundleRequiresProvisioningProfile(entitlements plistutil.PlistData, hasEmbeddedProfile bool) bool {
	if hasEmbeddedProfile {
		return true
	}
	for key := range entitlements {
		key = strings.TrimSpace(key)
		if key == "" || macEntitlementDoesNotRequireProfile(key) {
			continue
		}
		return true
	}
	return false
}

// macEntitlementDoesNotRequireProfile intentionally recognizes only identity
// and ordinary sandbox/hardened-runtime claims. Unknown capability entitlements
// fail closed unless the archive carried an embedded profile.
func macEntitlementDoesNotRequireProfile(key string) bool {
	if key == "com.apple.developer.team-identifier" || key == "com.apple.security.app-sandbox" || key == "com.apple.security.inherit" || key == "com.apple.security.get-task-allow" || key == "com.apple.security.print" || key == "com.apple.security.scripting-targets" {
		return true
	}
	for _, prefix := range []string{
		"com.apple.security.assets.",
		"com.apple.security.automation.",
		"com.apple.security.cs.",
		"com.apple.security.device.",
		"com.apple.security.files.",
		"com.apple.security.network.",
		"com.apple.security.personal-information.",
		"com.apple.security.temporary-exception.",
	} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func isMacAppDistributionCertificate(commonName string) bool {
	name := strings.TrimSpace(commonName)
	return strings.HasPrefix(name, "Apple Distribution:") ||
		strings.HasPrefix(name, "Mac App Distribution:") ||
		strings.HasPrefix(name, "3rd Party Mac Developer Application:")
}

func selectMacInstallerCertificate(certificates []certificateutil.CertificateInfoModel, teamID string) (certificateutil.CertificateInfoModel, bool) {
	sort.SliceStable(certificates, func(i, j int) bool {
		if certificates[i].CommonName != certificates[j].CommonName {
			return certificates[i].CommonName < certificates[j].CommonName
		}
		return certificates[i].Serial < certificates[j].Serial
	})
	for _, certificate := range certificates {
		if teamID != "" && certificate.TeamID != teamID {
			continue
		}
		name := strings.TrimSpace(certificate.CommonName)
		if strings.HasPrefix(name, "Mac Installer Distribution:") || strings.HasPrefix(name, "3rd Party Mac Developer Installer:") {
			return certificate, true
		}
	}
	return certificateutil.CertificateInfoModel{}, false
}

func selectMacProvisioningProfile(
	bundleID string,
	embeddedProfiles map[string]profileutil.ProvisioningProfileInfoModel,
	profiles []profileutil.ProvisioningProfileInfoModel,
	certificateSerial string,
	teamID string,
	targetEntitlements plistutil.PlistData,
) (profileutil.ProvisioningProfileInfoModel, bool) {
	candidates := make([]profileutil.ProvisioningProfileInfoModel, 0)
	now := time.Now()
	for _, profile := range profiles {
		if profile.Type != profileutil.ProfileTypeMacOs || !macProfileIsAppStore(profile.ExportType) {
			continue
		}
		if teamID != "" && profile.TeamID != teamID {
			continue
		}
		if !profile.ExpirationDate.IsZero() && !now.Before(profile.ExpirationDate) {
			continue
		}
		if !glob.Glob(profile.BundleID, bundleID) || !macProfileContainsCertificate(profile, certificateSerial) {
			continue
		}
		if len(profileutil.MatchTargetAndProfileEntitlements(targetEntitlements, profile.Entitlements, profile.Type)) > 0 {
			continue
		}
		if !macProfileEntitlementsPermitTarget(profile.Entitlements, targetEntitlements) {
			continue
		}
		candidates = append(candidates, profile)
	}
	if len(candidates) == 0 {
		return profileutil.ProvisioningProfileInfoModel{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		iExact := candidates[i].BundleID == bundleID
		jExact := candidates[j].BundleID == bundleID
		if iExact != jExact {
			return iExact
		}
		iEmbedded := embeddedProfiles[bundleID].UUID != "" && embeddedProfiles[bundleID].UUID == candidates[i].UUID
		jEmbedded := embeddedProfiles[bundleID].UUID != "" && embeddedProfiles[bundleID].UUID == candidates[j].UUID
		if iEmbedded != jEmbedded {
			return iEmbedded
		}
		if len(candidates[i].BundleID) != len(candidates[j].BundleID) {
			return len(candidates[i].BundleID) > len(candidates[j].BundleID)
		}
		if !candidates[i].ExpirationDate.Equal(candidates[j].ExpirationDate) {
			return candidates[i].ExpirationDate.After(candidates[j].ExpirationDate)
		}
		if candidates[i].UUID != candidates[j].UUID {
			return candidates[i].UUID < candidates[j].UUID
		}
		return candidates[i].Name < candidates[j].Name
	})
	return candidates[0], true
}

func macProfileEntitlementsPermitTarget(profileEntitlements, targetEntitlements plistutil.PlistData) bool {
	for key, targetValue := range targetEntitlements {
		key = strings.TrimSpace(key)
		if key == "" || macEntitlementDoesNotRequireProfile(key) {
			continue
		}
		profileValue, ok := profileEntitlements[key]
		if !ok || !macProfileEntitlementValuePermitsForExport(key, profileValue, targetValue) {
			return false
		}
	}
	return true
}

func macProfileEntitlementValuePermitsForExport(key string, profileValue, targetValue any) bool {
	if macProfileEntitlementValuePermits(profileValue, targetValue) {
		return true
	}

	profileString, profileIsString := profileValue.(string)
	targetString, targetIsString := targetValue.(string)
	if !profileIsString || !targetIsString {
		return false
	}

	switch key {
	case "com.apple.developer.icloud-container-environment":
		return profileString == "Production" && targetString == "Development"
	case "com.apple.developer.aps-environment",
		"com.apple.developer.devicecheck.appattest-environment":
		return profileString == "production" && targetString == "development"
	default:
		return false
	}
}

func macProfileEntitlementValuePermits(profileValue, targetValue any) bool {
	profile := reflect.ValueOf(profileValue)
	target := reflect.ValueOf(targetValue)
	if !profile.IsValid() || !target.IsValid() {
		return !profile.IsValid() && !target.IsValid()
	}

	if profile.Kind() != target.Kind() {
		return false
	}
	switch profile.Kind() {
	case reflect.String:
		return macProfileStringEntitlementPermits(profile.String(), target.String())
	case reflect.Slice, reflect.Array:
		for profileIndex := 0; profileIndex < profile.Len(); profileIndex++ {
			value, ok := profile.Index(profileIndex).Interface().(string)
			if !ok || !macProfileEntitlementPatternIsSafe(value) {
				return false
			}
		}
		for targetIndex := 0; targetIndex < target.Len(); targetIndex++ {
			value, ok := target.Index(targetIndex).Interface().(string)
			if !ok || strings.Contains(value, "*") {
				return false
			}
			permitted := false
			for profileIndex := 0; profileIndex < profile.Len(); profileIndex++ {
				if macProfileEntitlementValuePermits(profile.Index(profileIndex).Interface(), target.Index(targetIndex).Interface()) {
					permitted = true
					break
				}
			}
			if !permitted {
				return false
			}
		}
		return true
	case reflect.Map:
		if profile.Type().Key().Kind() != reflect.String || target.Type().Key().Kind() != reflect.String {
			return false
		}
		for _, targetKey := range target.MapKeys() {
			var profileEntry reflect.Value
			for _, profileKey := range profile.MapKeys() {
				if profileKey.String() == targetKey.String() {
					profileEntry = profile.MapIndex(profileKey)
					break
				}
			}
			if !profileEntry.IsValid() || !macProfileEntitlementValuePermits(profileEntry.Interface(), target.MapIndex(targetKey).Interface()) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(profileValue, targetValue)
	}
}

func macProfileStringEntitlementPermits(profileValue, targetValue string) bool {
	if strings.Contains(targetValue, "*") || !macProfileEntitlementPatternIsSafe(profileValue) {
		return false
	}
	return glob.Glob(profileValue, targetValue)
}

func macProfileEntitlementPatternIsSafe(value string) bool {
	switch strings.Count(value, "*") {
	case 0:
		return true
	case 1:
		return strings.HasSuffix(value, ".*") && strings.TrimSuffix(value, ".*") != ""
	default:
		return false
	}
}

func macProfileIsAppStore(method legacyexportoptions.Method) bool {
	return method == legacyexportoptions.MethodAppStore || method == legacyexportoptions.MethodAppStoreConnect
}

func macProfileContainsCertificate(profile profileutil.ProvisioningProfileInfoModel, serial string) bool {
	if strings.TrimSpace(serial) == "" {
		return false
	}
	for _, certificate := range profile.DeveloperCertificates {
		if certificate.Serial == serial {
			return true
		}
	}
	return false
}

func macICloudContainerEnvironment(entitlements map[string]plistutil.PlistData) string {
	for _, values := range entitlements {
		services, ok := values.GetStringArray("com.apple.developer.icloud-services")
		if !ok {
			continue
		}
		for _, service := range services {
			if service == "CloudKit" || service == "CloudDocuments" {
				return "Production"
			}
		}
	}
	return ""
}
