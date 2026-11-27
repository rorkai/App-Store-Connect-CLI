package shared

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

type IPABundleInfo struct {
	BundleID    string
	Version     string
	BuildNumber string
	Platform    asc.Platform
}

// ValidateIPAPath ensures an IPA path points to a non-empty regular file and
// rejects symlinks so upload commands don't dereference unexpected files.
func ValidateIPAPath(ipaPath string) (os.FileInfo, error) {
	file, fileInfo, err := OpenValidatedIPAPath(ipaPath)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	return fileInfo, nil
}

// ValidatePKGPath ensures a PKG path points to a non-empty regular file and
// rejects symlinks before an upload reservation is created.
func ValidatePKGPath(pkgPath string) (os.FileInfo, error) {
	file, fileInfo, err := OpenValidatedPKGPath(pkgPath)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	return fileInfo, nil
}

// OpenValidatedIPAPath opens and validates an IPA without following a symlink.
// The returned handle pins the validated file across the upload lifecycle.
func OpenValidatedIPAPath(ipaPath string) (*os.File, os.FileInfo, error) {
	return openValidatedArtifactPath(ipaPath, "IPA", "--ipa")
}

// OpenValidatedPKGPath opens and validates a PKG without following a symlink.
// The returned handle pins the validated file across the upload lifecycle.
func OpenValidatedPKGPath(pkgPath string) (*os.File, os.FileInfo, error) {
	return openValidatedArtifactPath(pkgPath, "PKG", "--pkg")
}

func openValidatedArtifactPath(artifactPath, artifactName, flagName string) (*os.File, os.FileInfo, error) {
	file, fileInfo, err := secureopen.OpenExistingRegularFileNoFollow(artifactPath, artifactName, flagName)
	switch {
	case err == nil:
		return file, fileInfo, nil
	case errors.Is(err, os.ErrNotExist):
		notFound := NewErrorWithCause(fmt.Errorf("%s file not found: %q", flagName, artifactPath), err)
		return nil, nil, WithDiagnostic(NewValidationError(notFound), DiagnosticFileNotFound, flagName)
	case errors.Is(err, os.ErrPermission):
		return nil, nil, WithDiagnostic(NewValidationError(err), DiagnosticFilePermissionDenied, flagName)
	default:
		return nil, nil, WithDiagnostic(NewValidationError(err), DiagnosticInvalidInput, flagName)
	}
}

// ExtractBundleInfoFromIPA reads the top-level app's bundle identifier and
// version metadata from an IPA.
func ExtractBundleInfoFromIPA(ipaPath string) (IPABundleInfo, error) {
	reader, err := zip.OpenReader(ipaPath)
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("open IPA: %w", err)
	}
	defer reader.Close()
	return extractBundleInfoFromIPAFiles(reader.File)
}

// ExtractBundleInfoFromIPAFile reads top-level app metadata from an opened IPA.
func ExtractBundleInfoFromIPAFile(file *os.File) (IPABundleInfo, error) {
	if file == nil {
		return IPABundleInfo{}, fmt.Errorf("open IPA: file is required")
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("stat IPA: %w", err)
	}
	reader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("open IPA: %w", err)
	}
	return extractBundleInfoFromIPAFiles(reader.File)
}

func extractBundleInfoFromIPAFiles(files []*zip.File) (IPABundleInfo, error) {
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		if !isTopLevelAppInfoPlist(file.Name) {
			continue
		}
		return readBundleInfoFromInfoPlist(file)
	}

	// Keep the canonical casing for the filename, but follow Go error string style.
	return IPABundleInfo{}, fmt.Errorf("missing Info.plist in IPA")
}

func isTopLevelAppInfoPlist(name string) bool {
	cleaned := path.Clean(name)
	if !strings.HasPrefix(cleaned, "Payload/") || !strings.HasSuffix(cleaned, "/Info.plist") {
		return false
	}
	dir := path.Dir(cleaned)
	if !strings.HasSuffix(dir, ".app") {
		return false
	}
	return path.Dir(dir) == "Payload"
}

func readBundleInfoFromInfoPlist(file *zip.File) (IPABundleInfo, error) {
	if err := infoplist.CheckDeclaredSize(file.UncompressedSize64); err != nil {
		return IPABundleInfo{}, fmt.Errorf("read Info.plist: %w", err)
	}

	reader, err := file.Open()
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("open Info.plist: %w", err)
	}
	defer reader.Close()

	data, err := infoplist.ReadBounded(reader)
	if err != nil {
		return IPABundleInfo{}, fmt.Errorf("read Info.plist: %w", err)
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return IPABundleInfo{}, fmt.Errorf("decode Info.plist: %w", err)
	}

	var info map[string]any
	decoder := plist.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&info); err != nil {
		return IPABundleInfo{}, fmt.Errorf("decode Info.plist: %w", err)
	}

	platform, err := detectIPAPlatform(info)
	if err != nil {
		return IPABundleInfo{}, err
	}

	return IPABundleInfo{
		BundleID:    coercePlistValueToString(info["CFBundleIdentifier"]),
		Version:     coercePlistValueToString(info["CFBundleShortVersionString"]),
		BuildNumber: coercePlistValueToString(info["CFBundleVersion"]),
		Platform:    platform,
	}, nil
}

func detectIPAPlatform(info map[string]any) (asc.Platform, error) {
	type platformMarker struct {
		key   string
		value string
	}

	markers := make([]platformMarker, 0, 3)
	if value := coercePlistValueToString(info["DTPlatformName"]); value != "" {
		markers = append(markers, platformMarker{key: "DTPlatformName", value: value})
	}
	for _, value := range plistStringValues(info["CFBundleSupportedPlatforms"]) {
		markers = append(markers, platformMarker{key: "CFBundleSupportedPlatforms", value: value})
	}

	var detected asc.Platform
	for _, marker := range markers {
		platform, ok := appStorePlatformForIPA(marker.value)
		if !ok {
			return "", fmt.Errorf("unsupported IPA platform metadata %s=%q", marker.key, marker.value)
		}
		if detected != "" && detected != platform {
			return "", fmt.Errorf("conflicting IPA platform metadata: %s and %s", detected, platform)
		}
		detected = platform
	}
	return detected, nil
}

func plistStringValues(value any) []string {
	values := make([]string, 0, 1)
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if text := coercePlistValueToString(item); text != "" {
				values = append(values, text)
			}
		}
	case []string:
		for _, item := range typed {
			if text := strings.TrimSpace(item); text != "" {
				values = append(values, text)
			}
		}
	}
	return values
}

func appStorePlatformForIPA(value string) (asc.Platform, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "iphoneos", "watchos":
		return asc.PlatformIOS, true
	case "appletvos":
		return asc.PlatformTVOS, true
	case "xros":
		return asc.PlatformVisionOS, true
	case "macosx":
		return asc.PlatformMacOS, true
	default:
		return "", false
	}
}

// ResolveBundleInfoForIPA reads IPA metadata and fills missing version/build-number
// values while preserving explicit overrides and CLI-facing error messages.
func ResolveBundleInfoForIPA(ipaPath, version, buildNumber string) (IPABundleInfo, error) {
	versionValue := strings.TrimSpace(version)
	buildNumberValue := strings.TrimSpace(buildNumber)
	info, err := ExtractBundleInfoFromIPA(ipaPath)
	if err != nil {
		missingFlags := make([]string, 0, 2)
		if versionValue == "" {
			missingFlags = append(missingFlags, "--version")
		}
		if buildNumberValue == "" {
			missingFlags = append(missingFlags, "--build-number")
		}
		if len(missingFlags) > 0 {
			return IPABundleInfo{}, fmt.Errorf("%s required (failed to extract from IPA: %w)", strings.Join(missingFlags, " and "), err)
		}
		return IPABundleInfo{Version: versionValue, BuildNumber: buildNumberValue}, nil
	}
	if versionValue == "" {
		versionValue = info.Version
	}
	if buildNumberValue == "" {
		buildNumberValue = info.BuildNumber
	}
	if versionValue == "" || buildNumberValue == "" {
		missingFields := make([]string, 0, 2)
		missingFlags := make([]string, 0, 2)
		if versionValue == "" {
			missingFields = append(missingFields, "CFBundleShortVersionString")
			missingFlags = append(missingFlags, "--version")
		}
		if buildNumberValue == "" {
			missingFields = append(missingFields, "CFBundleVersion")
			missingFlags = append(missingFlags, "--build-number")
		}
		return IPABundleInfo{}, fmt.Errorf("missing Info.plist keys %s; provide %s", strings.Join(missingFields, " and "), strings.Join(missingFlags, " and "))
	}
	info.Version, info.BuildNumber = versionValue, buildNumberValue
	return info, nil
}

func coercePlistValueToString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case int, int8, int16, int32, int64:
		return fmt.Sprint(v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v)
	case float32, float64:
		return strings.TrimSpace(fmt.Sprint(v))
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}
