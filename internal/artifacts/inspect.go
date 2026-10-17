package artifacts

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"

	"howett.net/plist"
)

const (
	maxZipEntries       = 20_000
	maxZipDeclaredBytes = 16 << 30
	maxPlistBytes       = 4 << 20
	maxXarTOCBytes      = 8 << 20
	maxXarFileBytes     = 4 << 20

	maxCompressedExecutableScanBytes = 1 << 30
	minCompressionRatioCheckBytes    = 16 << 20
	maxExecutableCompressionRatio    = 64
)

// IPAManifest is the offline manifest for an IPA.
type IPAManifest struct {
	BundleID         string
	Name             string
	Version          string
	BuildNumber      string
	MinimumOSVersion string
	Platforms        []string
	// DeviceFamilies lists the top-level app's UIDeviceFamily values (1 is
	// iPhone, 2 is iPad). It is nil when the key is absent or unreadable.
	DeviceFamilies   []int
	TeamID           string
	SignerCommonName string
	Status           string
	NestedBundles    []NestedBundle
	Entitlements     map[string]any
	Profile          *ProfileSummary
	// CodeSignature classifies the main executable's embedded signature. It is
	// empty when inspection stopped before the executable was read.
	CodeSignature      string
	CodeSignatureError string
	Signer             *SignerIdentity
	// Architectures lists every slice of the main executable in universal
	// header order; a thin executable has one. CodeSignature, CodeSignatureError,
	// and Signer describe the slice stored first. It is nil when no slice could
	// be identified.
	Architectures []ArchitectureSignature
	// SignerConsistent reports whether every slice has the same signature
	// classification and signer. It is nil when Architectures is nil.
	SignerConsistent *bool
	// SignatureVerification is set only when verification was requested. It
	// covers the primary slice, the one stored first.
	SignatureVerification *SignatureVerification
}

// NestedBundle is an extension or App Clip inside the IPA.
type NestedBundle struct {
	BundleID string
	Name     string
	Path     string
}

// ProfileSummary is the embedded provisioning profile summary.
type ProfileSummary struct {
	Name           string
	UUID           string
	ExpirationDate string
	ProfileType    string
}

// PKGManifest is the offline manifest for a flat component package or a
// Distribution-style product archive.
type PKGManifest struct {
	ProductID        string
	Version          string
	InstallLocation  string
	BundleIDs        []string
	SignerCommonName string
	TeamID           string
	Status           string
	// The fields below are set only for product archives. App fields come from
	// the primary component's app Info.plist when its payload can be read.
	BundleID          string
	BuildNumber       string
	MinimumOSVersion  string
	Platforms         []string
	HostArchitectures []string
	Components        []PKGComponent
	// Warnings describe component metadata that could not be read without
	// making the package unreadable.
	Warnings []string
	// PackageSignature classifies the xar signature in the table of contents.
	PackageSignature      string
	PackageSignatureError string
	Signer                *SignerIdentity
	// SignatureVerification is set only when verification was requested.
	SignatureVerification *SignatureVerification
}

type bundlePlist struct {
	BundleID         string   `plist:"CFBundleIdentifier"`
	Executable       string   `plist:"CFBundleExecutable"`
	DisplayName      string   `plist:"CFBundleDisplayName"`
	Name             string   `plist:"CFBundleName"`
	Version          string   `plist:"CFBundleShortVersionString"`
	BuildNumber      string   `plist:"CFBundleVersion"`
	MinimumOSVersion string   `plist:"MinimumOSVersion"`
	MinimumSystem    string   `plist:"LSMinimumSystemVersion"`
	Platforms        []string `plist:"CFBundleSupportedPlatforms"`
	Platform         string   `plist:"DTPlatformName"`
	// DeviceFamily is decoded loosely so an unusual UIDeviceFamily value never
	// makes the rest of the Info.plist unreadable.
	DeviceFamily any `plist:"UIDeviceFamily"`
}

// IPAOptions selects optional IPA inspection work.
type IPAOptions struct {
	IncludeEntitlements bool
	IncludeProfile      bool
	// VerifySignature verifies the main executable's code signature offline.
	VerifySignature bool
}

// InspectIPA reads a bounded IPA zip and returns a metadata manifest. A missing
// embedded profile is reported as unsigned. The main executable's signer
// identity is read from its code signature, which is not verified.
func InspectIPA(source io.ReaderAt, size int64, includeEntitlements, includeProfile bool) (IPAManifest, error) {
	return InspectIPAWithOptions(source, size, IPAOptions{IncludeEntitlements: includeEntitlements, IncludeProfile: includeProfile})
}

// InspectIPAWithOptions is InspectIPA with optional signature verification
// against the embedded Apple roots.
// Inspection failures caused by the IPA's own contents match ErrInvalidArchive.
func InspectIPAWithOptions(source io.ReaderAt, size int64, options IPAOptions) (IPAManifest, error) {
	if !options.VerifySignature {
		manifest, err := inspectIPA(source, size, options.IncludeEntitlements, options.IncludeProfile, nil)
		return manifest, markInvalidArchive(err)
	}
	policy, err := appleTrustPolicy()
	if err != nil {
		return IPAManifest{Status: "unreadable"}, fmt.Errorf("load Apple certificates: %w", err)
	}
	manifest, err := inspectIPAVerifying(source, size, options.IncludeEntitlements, options.IncludeProfile, policy)
	return manifest, markInvalidArchive(err)
}

// ErrInvalidArchive matches an inspection failure caused by the artifact's
// own contents, such as a malformed archive or unparsable metadata, as
// opposed to a failure of the inspector itself.
var ErrInvalidArchive = errors.New("invalid artifact archive")

// invalidArchiveError marks err as ErrInvalidArchive without changing its
// message or cause chain.
type invalidArchiveError struct{ err error }

func (e invalidArchiveError) Error() string        { return e.err.Error() }
func (e invalidArchiveError) Unwrap() error        { return e.err }
func (e invalidArchiveError) Is(target error) bool { return target == ErrInvalidArchive }

// markInvalidArchive marks err unless it carries an operational read failure
// from the source, such as EIO, which says nothing about the archive itself.
func markInvalidArchive(err error) error {
	if err == nil || isOperationalReadError(err) {
		return err
	}
	return invalidArchiveError{err: err}
}

func isOperationalReadError(err error) bool {
	var pathErr *fs.PathError
	var errno syscall.Errno
	return errors.As(err, &pathErr) || errors.As(err, &errno)
}

func inspectIPAVerifying(source io.ReaderAt, size int64, includeEntitlements, includeProfile bool, policy *trustPolicy) (IPAManifest, error) {
	manifest, err := inspectIPA(source, size, includeEntitlements, includeProfile, policy)
	if manifest.SignatureVerification == nil {
		detail := "IPA could not be inspected"
		if err != nil {
			detail += ": " + err.Error()
		}
		manifest.SignatureVerification = &SignatureVerification{Status: VerificationUnsupported, Detail: detail}
	}
	return manifest, err
}

// InspectIPAInfoPlist reads only the IPA's top-level app Info.plist, under
// the same archive bounds as InspectIPA, without reading the executable or the
// embedded profile. Unlike InspectIPA, it fails when UIDeviceFamily is present
// but malformed, so a caller that acts on the device families never mistakes
// an unreadable declaration for an iPhone-only app.
func InspectIPAInfoPlist(source io.ReaderAt, size int64) (IPAManifest, error) {
	scan, err := scanIPA(source, size)
	if err != nil {
		return IPAManifest{Status: "unreadable"}, err
	}
	mainPlist, err := readZipPlist(scan.main)
	if err != nil {
		return IPAManifest{Status: "unreadable"}, err
	}
	if _, err := deviceFamilies(mainPlist.DeviceFamily); err != nil {
		return IPAManifest{Status: "unreadable"}, err
	}
	manifest := manifestFromPlist(mainPlist)
	manifest.Status = "readable"
	return manifest, nil
}

// ipaScan holds the members an IPA inspection selects from the archive.
type ipaScan struct {
	reader *zip.Reader
	main   *zip.File
	nested []*zip.File
}

// scanIPA opens a bounded IPA zip and selects its single top-level app
// Info.plist and the nested bundle Info.plist members.
func scanIPA(source io.ReaderAt, size int64) (ipaScan, error) {
	if err := validateZIPDirectory(source, size); err != nil {
		return ipaScan{}, fmt.Errorf("open IPA: %w", err)
	}
	bounded := &zipDirectoryReader{ReaderAt: source, remaining: maxZIPDirectoryBytes + (512 << 10)}
	reader, err := zip.NewReader(bounded, size)
	bounded.remaining = -1
	if err != nil {
		return ipaScan{}, fmt.Errorf("open IPA: %w", err)
	}
	if len(reader.File) > maxZipEntries {
		return ipaScan{}, fmt.Errorf("IPA contains %d entries; limit is %d", len(reader.File), maxZipEntries)
	}
	scan := ipaScan{reader: reader}
	var declared uint64
	for _, file := range reader.File {
		if file.UncompressedSize64 > maxZipDeclaredBytes-declared {
			return ipaScan{}, fmt.Errorf("IPA declared expansion exceeds the limit")
		}
		declared += file.UncompressedSize64
		name := zipMemberName(file.Name)
		if file.FileInfo().IsDir() {
			continue
		}
		if isTopLevelAppInfoPlist(name) {
			if scan.main != nil {
				return ipaScan{}, fmt.Errorf("IPA has multiple top-level app Info.plist entries")
			}
			scan.main = file
			continue
		}
		if isNestedInfoPlist(name) {
			scan.nested = append(scan.nested, file)
		}
	}
	if scan.main == nil {
		return ipaScan{}, fmt.Errorf("IPA has no top-level app Info.plist")
	}
	return scan, nil
}

// inspectIPA verifies the code signature when policy is non-nil.
func inspectIPA(source io.ReaderAt, size int64, includeEntitlements, includeProfile bool, policy *trustPolicy) (IPAManifest, error) {
	scan, err := scanIPA(source, size)
	if err != nil {
		return IPAManifest{Status: "unreadable"}, err
	}
	reader, main, nested := scan.reader, scan.main, scan.nested
	var profile *zip.File
	appRoot := strings.TrimSuffix(zipMemberName(main.Name), "Info.plist")
	for _, file := range reader.File {
		if zipMemberName(file.Name) == appRoot+"embedded.mobileprovision" {
			if profile != nil {
				return IPAManifest{Status: "unreadable"}, fmt.Errorf("IPA has duplicate embedded profiles")
			}
			profile = file
		}
	}
	mainPlist, err := readZipPlist(main)
	if err != nil {
		return IPAManifest{Status: "unreadable"}, err
	}
	manifest := manifestFromPlist(mainPlist)
	executable, executableErr := mainExecutableMember(reader.File, appRoot, mainPlist.Executable)
	for _, file := range nested {
		path := zipMemberName(file.Name)
		if !strings.HasPrefix(path, appRoot) {
			continue
		}
		parsed, err := readZipPlist(file)
		if err != nil {
			manifest.Status = "unreadable"
			return manifest, fmt.Errorf("read %s: %w", path, err)
		}
		manifest.NestedBundles = append(manifest.NestedBundles, NestedBundle{
			BundleID: parsed.BundleID,
			Name:     firstNonEmpty(parsed.DisplayName, parsed.Name),
			Path:     path,
		})
	}
	var capture *signatureCapture
	if policy != nil {
		capture = &signatureCapture{}
	}
	if executableErr == nil {
		var primary int
		manifest.Architectures, primary, executableErr = readExecutableSignatures(source, executable, capture)
		if executableErr == nil {
			slice := manifest.Architectures[primary]
			manifest.CodeSignature, manifest.CodeSignatureError, manifest.Signer = slice.CodeSignature, slice.CodeSignatureError, slice.Signer
			consistent := signersConsistent(manifest.Architectures)
			manifest.SignerConsistent = &consistent
		}
	}
	if executableErr != nil {
		manifest.CodeSignature, manifest.Signer = SignatureUnreadable, nil
		manifest.CodeSignatureError = executableErr.Error()
	}
	if policy != nil {
		var readErr error
		if manifest.CodeSignatureError != "" {
			readErr = errors.New(manifest.CodeSignatureError)
		}
		verification := verifyIPACodeSignature(source, reader.File, appRoot, main, executable, manifest.CodeSignature, readErr, capture, policy)
		manifest.SignatureVerification = &verification
	}
	if manifest.Signer != nil {
		manifest.SignerCommonName = manifest.Signer.CommonName
		manifest.TeamID = manifest.Signer.TeamID
	}
	if profile != nil {
		summary, entitlements, err := readEmbeddedProfile(profile)
		if err != nil {
			manifest.Status = "unreadable"
			return manifest, fmt.Errorf("read embedded profile: %w", err)
		}
		manifest.Status = "readable"
		if includeProfile {
			manifest.Profile = summary
		}
		if includeEntitlements {
			manifest.Entitlements = entitlements
		}
		if manifest.TeamID == "" {
			if team, ok := entitlements["com.apple.developer.team-identifier"].(string); ok {
				manifest.TeamID = team
			}
		}
	}
	if manifest.Status == "" {
		manifest.Status = "unsigned"
	}
	return manifest, nil
}

// mainExecutableMember finds CFBundleExecutable inside the selected app. Xcode
// names the executable after the bundle when the key is absent.
func mainExecutableMember(files []*zip.File, appRoot, name string) (*zip.File, error) {
	if name == "" {
		name = strings.TrimSuffix(path.Base(strings.TrimSuffix(appRoot, "/")), ".app")
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("CFBundleExecutable %q is not a file name in the app bundle", name)
	}
	var found *zip.File
	for _, file := range files {
		if zipMemberName(file.Name) != appRoot+name || file.FileInfo().IsDir() {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("IPA has duplicate main executable entries")
		}
		found = file
	}
	if found == nil {
		return nil, fmt.Errorf("IPA has no main executable %q", name)
	}
	return found, nil
}

func readExecutableSignatures(source io.ReaderAt, file *zip.File, capture *signatureCapture) ([]ArchitectureSignature, int, error) {
	size := int64(file.UncompressedSize64)
	if file.Method == zip.Store && file.CompressedSize64 == file.UncompressedSize64 {
		// Stored members are addressable, so skipping code pages costs no reads.
		if offset, err := file.DataOffset(); err == nil {
			return readMachOSignaturesCapture(io.NewSectionReader(source, offset, size), size, capture)
		}
	}
	if err := compressedExecutableScanError(file.CompressedSize64, file.UncompressedSize64); err != nil {
		return nil, 0, err
	}
	reader, err := file.Open()
	if err != nil {
		return nil, 0, fmt.Errorf("open main executable: %w", err)
	}
	defer reader.Close()
	return readMachOSignaturesCapture(reader, size, capture)
}

// A compressed executable must be inflated up to its code signature near the
// end. Bound that work, and refuse ratios real Mach-O code does not reach.
func compressedExecutableScanError(compressed, uncompressed uint64) error {
	if uncompressed > maxCompressedExecutableScanBytes {
		return fmt.Errorf("compressed main executable exceeds the %d byte scan limit", uint64(maxCompressedExecutableScanBytes))
	}
	if uncompressed > minCompressionRatioCheckBytes && uncompressed/max(compressed, 1) > maxExecutableCompressionRatio {
		return fmt.Errorf("compressed main executable exceeds the scan limit's compression ratio")
	}
	return nil
}

func manifestFromPlist(parsed bundlePlist) IPAManifest {
	platforms := append([]string(nil), parsed.Platforms...)
	if len(platforms) == 0 && parsed.Platform != "" {
		platforms = []string{parsed.Platform}
	}
	families, _ := deviceFamilies(parsed.DeviceFamily)
	return IPAManifest{
		BundleID:         parsed.BundleID,
		Name:             firstNonEmpty(parsed.DisplayName, parsed.Name),
		Version:          parsed.Version,
		BuildNumber:      parsed.BuildNumber,
		MinimumOSVersion: firstNonEmpty(parsed.MinimumOSVersion, parsed.MinimumSystem),
		Platforms:        platforms,
		DeviceFamilies:   families,
	}
}

// deviceFamilies normalizes a decoded UIDeviceFamily value. Xcode writes an
// array of integers; a single integer and numeric strings are also accepted.
// An absent key returns nil: Apple's Information Property List Key Reference
// documents value 1 (iPhone and iPod touch) as the default, and Xcode always
// writes the key from the Targeted Device Family build setting, so a binary
// without it is iPhone-only. A present but empty array declares no device
// family and is malformed.
func deviceFamilies(value any) ([]int, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		items = []any{value}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("info.plist UIDeviceFamily is an empty array")
	}
	families := make([]int, 0, len(items))
	for _, item := range items {
		family, ok := deviceFamilyValue(item)
		if !ok {
			return nil, fmt.Errorf("info.plist UIDeviceFamily has unreadable value %v", item)
		}
		families = append(families, family)
	}
	return families, nil
}

func deviceFamilyValue(value any) (int, bool) {
	switch typed := value.(type) {
	case uint64:
		if typed > math.MaxInt32 {
			return 0, false
		}
		return int(typed), true
	case int64:
		if typed < 0 || typed > math.MaxInt32 {
			return 0, false
		}
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil || parsed < 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func zipMemberName(name string) string {
	return strings.TrimSuffix(strings.ReplaceAll(name, "\\", "/"), "/")
}

func isTopLevelAppInfoPlist(name string) bool {
	if !strings.HasPrefix(name, "Payload/") || !strings.HasSuffix(name, ".app/Info.plist") {
		return false
	}
	inner := strings.TrimPrefix(strings.TrimSuffix(name, "/Info.plist"), "Payload/")
	return !strings.Contains(inner, "/")
}

func isNestedInfoPlist(name string) bool {
	if !strings.HasSuffix(name, "/Info.plist") || !strings.HasPrefix(name, "Payload/") {
		return false
	}
	bundlePath := strings.TrimSuffix(name, "/Info.plist")
	for _, part := range strings.Split(bundlePath, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return strings.HasSuffix(bundlePath, ".appex") || (strings.Contains(bundlePath, ".app/AppClips/") && strings.HasSuffix(bundlePath, ".app"))
}

func readZipPlist(file *zip.File) (bundlePlist, error) {
	if err := infoPlistSize(file.UncompressedSize64); err != nil {
		return bundlePlist{}, err
	}
	reader, err := file.Open()
	if err != nil {
		return bundlePlist{}, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxPlistBytes+1))
	if err != nil {
		return bundlePlist{}, err
	}
	if len(data) > maxPlistBytes {
		return bundlePlist{}, fmt.Errorf("info.plist exceeds %d bytes", maxPlistBytes)
	}
	return decodeBundlePlist(data)
}

func decodeBundlePlist(data []byte) (bundlePlist, error) {
	if err := infoplist.ValidateStructure(data); err != nil {
		return bundlePlist{}, fmt.Errorf("validate Info.plist: %w", err)
	}
	var parsed bundlePlist
	if _, err := plist.Unmarshal(data, &parsed); err != nil {
		return bundlePlist{}, fmt.Errorf("decode Info.plist: %w", err)
	}
	return parsed, nil
}

func infoPlistSize(size uint64) error {
	if size > maxPlistBytes {
		return fmt.Errorf("declared Info.plist size %d exceeds %d bytes", size, maxPlistBytes)
	}
	return nil
}

func readEmbeddedProfile(file *zip.File) (*ProfileSummary, map[string]any, error) {
	if file.UncompressedSize64 > maxPlistBytes {
		return nil, nil, fmt.Errorf("embedded profile exceeds %d bytes", maxPlistBytes)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxPlistBytes+1))
	if err != nil {
		return nil, nil, err
	}
	plistData, err := cmsContent(data)
	if err != nil {
		return nil, nil, err
	}
	if err := infoplist.ValidateStructure(plistData); err != nil {
		return nil, nil, fmt.Errorf("validate profile plist: %w", err)
	}
	var payload map[string]any
	if _, err := plist.Unmarshal(plistData, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode profile plist: %w", err)
	}
	entitlements, ok := payload["Entitlements"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("profile Entitlements must be a dictionary")
	}
	expirationDate := stringValue(payload["ExpirationDate"])
	if date, ok := payload["ExpirationDate"].(time.Time); ok {
		expirationDate = date.UTC().Format(time.RFC3339)
	}
	summary := &ProfileSummary{
		Name:           stringValue(payload["Name"]),
		UUID:           stringValue(payload["UUID"]),
		ExpirationDate: expirationDate,
		ProfileType:    profileType(payload),
	}
	return summary, entitlements, nil
}

func profileType(payload map[string]any) string {
	if provisions, ok := payload["ProvisionsAllDevices"].(bool); ok && provisions {
		return "enterprise"
	}
	if devices, ok := payload["ProvisionedDevices"].([]any); ok && len(devices) > 0 {
		entitlements, _ := payload["Entitlements"].(map[string]any)
		if debug, ok := entitlements["get-task-allow"].(bool); ok && debug {
			return "development"
		}
		return "ad-hoc"
	}
	return "app-store"
}

func cmsContent(data []byte) ([]byte, error) {
	const begin = "<?xml"
	const plistStart = "<plist"
	index := bytes.Index(data, []byte(plistStart))
	if index < 0 {
		index = bytes.Index(data, []byte(begin))
	}
	if index < 0 {
		return nil, fmt.Errorf("profile is not a CMS or plist payload")
	}
	end := bytes.LastIndex(data, []byte("</plist>"))
	if end < index {
		return nil, fmt.Errorf("profile plist is truncated")
	}
	return data[index : end+len("</plist>")], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

// PKGOptions selects optional package inspection work.
type PKGOptions struct {
	// VerifySignature verifies the package signature offline.
	VerifySignature bool
}

// InspectPKG reads PackageInfo from a flat xar component package, or the
// Distribution and embedded component packages of a product archive. The
// signer identity is read from the package signature certificates, which are
// not verified.
func InspectPKG(source io.ReaderAt, size int64) (PKGManifest, error) {
	return inspectPKG(source, size, nil)
}

// InspectPKGWithOptions is InspectPKG with optional signature verification
// against the embedded Apple roots.
func InspectPKGWithOptions(source io.ReaderAt, size int64, options PKGOptions) (PKGManifest, error) {
	if !options.VerifySignature {
		return inspectPKG(source, size, nil)
	}
	policy, err := appleTrustPolicy()
	if err != nil {
		return PKGManifest{Status: "unreadable"}, fmt.Errorf("load Apple certificates: %w", err)
	}
	return inspectPKGVerifying(source, size, policy)
}

func inspectPKGVerifying(source io.ReaderAt, size int64, policy *trustPolicy) (PKGManifest, error) {
	manifest, err := inspectPKG(source, size, policy)
	if manifest.SignatureVerification == nil {
		detail := "package table of contents could not be read"
		if err != nil {
			detail += ": " + err.Error()
		}
		manifest.SignatureVerification = &SignatureVerification{Status: VerificationUnsupported, Detail: detail}
	}
	return manifest, err
}

// inspectPKG verifies the package signature when policy is non-nil.
func inspectPKG(source io.ReaderAt, size int64, policy *trustPolicy) (PKGManifest, error) {
	document, heap, files, err := readXarFiles(source, size)
	if document == nil {
		return PKGManifest{Status: "unreadable"}, err
	}
	// The signature lives in the table of contents, so report it even when the
	// package metadata cannot be read, such as for a product archive.
	signature := PKGManifest{}
	var signatureErr error
	signature.PackageSignature, signature.Signer, signatureErr = xarSigner(document.Signature, document.XSignature)
	if signatureErr != nil {
		signature.PackageSignatureError = signatureErr.Error()
	}
	if signature.Signer != nil {
		signature.SignerCommonName = signature.Signer.CommonName
		signature.TeamID = signature.Signer.TeamID
	}
	if policy != nil {
		verification := verifyXarSignature(source, size, document, signature.PackageSignature, signatureErr, policy)
		signature.SignatureVerification = &verification
	}
	withSignature := func(manifest PKGManifest, status string) PKGManifest {
		manifest.Status = status
		manifest.SignerCommonName = signature.SignerCommonName
		manifest.TeamID = signature.TeamID
		manifest.PackageSignature = signature.PackageSignature
		manifest.PackageSignatureError = signature.PackageSignatureError
		manifest.Signer = signature.Signer
		manifest.SignatureVerification = signature.SignatureVerification
		return manifest
	}
	if err != nil {
		return withSignature(PKGManifest{}, "unreadable"), err
	}
	info, ok := files["PackageInfo"]
	if !ok {
		if distribution := xarChild(document.Files, "Distribution", "file"); distribution != nil {
			manifest, err := inspectProductArchive(document.Files, *distribution, heap, heap.Size())
			if err != nil {
				return withSignature(manifest, "unreadable"), err
			}
			return withSignature(manifest, "readable"), nil
		}
		return withSignature(PKGManifest{}, "unreadable"), fmt.Errorf("flat pkg has no PackageInfo")
	}
	manifest, err := parsePackageInfo(info)
	if err != nil {
		return withSignature(manifest, "unreadable"), err
	}
	return withSignature(manifest, "readable"), nil
}

type packageInfoXML struct {
	XMLName         xml.Name        `xml:"pkg-info"`
	Version         string          `xml:"version,attr"`
	InstallLocation string          `xml:"install-location,attr"`
	Identifier      string          `xml:"identifier,attr"`
	Bundles         []packageBundle `xml:"bundle"`
}

type packageBundle struct {
	ID           string `xml:"id,attr"`
	Path         string `xml:"path,attr"`
	ShortVersion string `xml:"CFBundleShortVersionString,attr"`
	Version      string `xml:"CFBundleVersion,attr"`
}

func decodePackageInfo(data []byte) (packageInfoXML, error) {
	var info packageInfoXML
	if err := xml.Unmarshal(data, &info); err != nil {
		return packageInfoXML{}, fmt.Errorf("decode PackageInfo: %w", err)
	}
	return info, nil
}

func parsePackageInfo(data []byte) (PKGManifest, error) {
	info, err := decodePackageInfo(data)
	if err != nil {
		return PKGManifest{}, err
	}
	ids := make([]string, 0, len(info.Bundles))
	for _, bundle := range info.Bundles {
		if bundle.ID != "" {
			ids = append(ids, bundle.ID)
		}
	}
	return PKGManifest{
		ProductID:       info.Identifier,
		Version:         info.Version,
		InstallLocation: info.InstallLocation,
		BundleIDs:       ids,
	}, nil
}

// readXarFiles returns the decoded table of contents and the heap whenever the
// table of contents could be read, even if reading a member fails.
func readXarFiles(source io.ReaderAt, size int64) (*xarDocument, *io.SectionReader, map[string][]byte, error) {
	toc, heapStart, err := readXarTOC(source, size)
	if err != nil {
		return nil, nil, nil, err
	}
	document, err := decodeXarTOC(toc)
	if err != nil {
		return nil, nil, nil, err
	}
	heap := io.NewSectionReader(source, heapStart, size-heapStart)
	files, err := xarFilesFromDocument(document, heap, heap.Size())
	return &document, heap, files, err
}

func readXarTOC(source io.ReaderAt, size int64) ([]byte, int64, error) {
	data := make([]byte, 28)
	if size < 28 {
		return nil, 0, fmt.Errorf("not a flat xar package")
	}
	if _, err := source.ReadAt(data, 0); err != nil {
		return nil, 0, fmt.Errorf("read xar header: %w", err)
	}
	if string(data[:4]) != "xar!" {
		return nil, 0, fmt.Errorf("not a flat xar package")
	}
	headerSize := int64(binary.BigEndian.Uint16(data[4:6]))
	if headerSize < 28 || headerSize > size {
		return nil, 0, fmt.Errorf("invalid xar header size")
	}
	tocCompressed := binary.BigEndian.Uint64(data[8:16])
	tocUncompressed := binary.BigEndian.Uint64(data[16:24])
	if tocCompressed > maxXarTOCBytes || tocUncompressed > maxXarTOCBytes {
		return nil, 0, fmt.Errorf("xar table of contents exceeds the limit")
	}
	if int64(tocCompressed) > size-headerSize {
		return nil, 0, fmt.Errorf("xar table of contents is truncated")
	}
	tocReader, err := zlib.NewReader(io.NewSectionReader(source, headerSize, int64(tocCompressed)))
	if err != nil {
		return nil, 0, fmt.Errorf("open xar table of contents: %w", err)
	}
	defer tocReader.Close()
	toc, err := io.ReadAll(io.LimitReader(tocReader, int64(tocUncompressed)+1))
	if err != nil {
		return nil, 0, fmt.Errorf("read xar table of contents: %w", err)
	}
	if uint64(len(toc)) > tocUncompressed {
		return nil, 0, fmt.Errorf("xar table of contents exceeds the declared size")
	}
	return toc, headerSize + int64(tocCompressed), nil
}

type xarDocument struct {
	XMLName    xml.Name      `xml:"xar"`
	Files      []xarFile     `xml:"toc>file"`
	Signature  *xarSignature `xml:"toc>signature"`
	XSignature *xarSignature `xml:"toc>x-signature"`
	Checksum   *xarHeapRange `xml:"toc>checksum"`
}

// xarHeapRange locates a TOC checksum or signature in the heap.
type xarHeapRange struct {
	Style  string `xml:"style,attr"`
	Offset int64  `xml:"offset"`
	Size   int64  `xml:"size"`
}

type xarFile struct {
	Name string  `xml:"name"`
	Type string  `xml:"type"`
	Data xarData `xml:"data"`
	// Files are the members of a directory.
	Files []xarFile `xml:"file"`
}

type xarData struct {
	// XMLName is set only when the member has a data element.
	XMLName          xml.Name
	Length           int64            `xml:"length"`
	Size             int64            `xml:"size"`
	Offset           int64            `xml:"offset"`
	Encoding         xarEncoding      `xml:"encoding"`
	ArchivedChecksum *xarFileChecksum `xml:"archived-checksum"`
}

// xarFileChecksum is the hex digest the table of contents records for a
// member's bytes in the heap.
type xarFileChecksum struct {
	Style string `xml:"style,attr"`
	Value string `xml:",chardata"`
}

type xarEncoding struct {
	Style string `xml:"style,attr"`
}

func decodeXarTOC(toc []byte) (xarDocument, error) {
	var document xarDocument
	if err := xml.Unmarshal(toc, &document); err != nil {
		return xarDocument{}, fmt.Errorf("decode xar table of contents: %w", err)
	}
	return document, nil
}

func xarFilesFromTOC(toc []byte, heap io.ReaderAt, heapSize int64) (map[string][]byte, error) {
	document, err := decodeXarTOC(toc)
	if err != nil {
		return nil, err
	}
	return xarFilesFromDocument(document, heap, heapSize)
}

func xarFilesFromDocument(document xarDocument, heap io.ReaderAt, heapSize int64) (map[string][]byte, error) {
	files := make(map[string][]byte, 1)
	for _, file := range document.Files {
		if file.Type != "file" || file.Name != "PackageInfo" {
			continue
		}
		payload, err := readXarMember(file, heap, heapSize)
		if err != nil {
			return nil, err
		}
		if _, exists := files[file.Name]; exists {
			return nil, fmt.Errorf("xar has duplicate PackageInfo entries")
		}
		files[file.Name] = payload
	}
	return files, nil
}

// xarMemberSection validates a member's heap range without reading it.
func xarMemberSection(file xarFile, heap io.ReaderAt, heapSize int64) (*io.SectionReader, error) {
	if file.Data.Length < 0 || file.Data.Offset < 0 || file.Data.Size < 0 {
		return nil, fmt.Errorf("xar file %q has a negative size or offset", file.Name)
	}
	if file.Data.Offset > heapSize || file.Data.Length > heapSize-file.Data.Offset {
		return nil, fmt.Errorf("xar file %q is truncated", file.Name)
	}
	return io.NewSectionReader(heap, file.Data.Offset, file.Data.Length), nil
}

// readXarMember reads a small metadata member such as PackageInfo or
// Distribution into memory.
func readXarMember(file xarFile, heap io.ReaderAt, heapSize int64) ([]byte, error) {
	if file.Data.Length > maxXarFileBytes || file.Data.Size > maxXarFileBytes {
		return nil, fmt.Errorf("xar file %q is outside the read limit", file.Name)
	}
	section, err := xarMemberSection(file, heap, heapSize)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, int(file.Data.Length))
	if _, err := io.ReadFull(section, payload); err != nil {
		return nil, fmt.Errorf("read xar %s: %w", file.Name, err)
	}
	switch file.Data.Encoding.Style {
	case "", "application/octet-stream":
	case "application/x-gzip":
		reader, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("open xar %s: %w", file.Name, err)
		}
		payload, err = io.ReadAll(io.LimitReader(reader, maxXarFileBytes+1))
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("read xar %s: %w", file.Name, err)
		}
		if len(payload) > maxXarFileBytes {
			return nil, fmt.Errorf("xar %s exceeds the read limit", file.Name)
		}
	default:
		return nil, fmt.Errorf("xar file %q uses unsupported encoding %q", file.Name, file.Data.Encoding.Style)
	}
	if int64(len(payload)) != file.Data.Size {
		return nil, fmt.Errorf("xar %s size does not match its declaration", file.Name)
	}
	return payload, nil
}
