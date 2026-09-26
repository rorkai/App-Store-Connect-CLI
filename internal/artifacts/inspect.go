package artifacts

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
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
)

// IPAManifest is the offline manifest for an IPA.
type IPAManifest struct {
	BundleID         string
	Name             string
	Version          string
	BuildNumber      string
	MinimumOSVersion string
	Platforms        []string
	TeamID           string
	SignerCommonName string
	Status           string
	NestedBundles    []NestedBundle
	Entitlements     map[string]any
	Profile          *ProfileSummary
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

// PKGManifest is the offline manifest for a flat component package.
type PKGManifest struct {
	ProductID        string
	Version          string
	InstallLocation  string
	BundleIDs        []string
	SignerCommonName string
	Status           string
}

type bundlePlist struct {
	BundleID         string   `plist:"CFBundleIdentifier"`
	DisplayName      string   `plist:"CFBundleDisplayName"`
	Name             string   `plist:"CFBundleName"`
	Version          string   `plist:"CFBundleShortVersionString"`
	BuildNumber      string   `plist:"CFBundleVersion"`
	MinimumOSVersion string   `plist:"MinimumOSVersion"`
	MinimumSystem    string   `plist:"LSMinimumSystemVersion"`
	Platforms        []string `plist:"CFBundleSupportedPlatforms"`
	Platform         string   `plist:"DTPlatformName"`
}

// InspectIPA reads a bounded IPA zip and returns a metadata manifest. A missing
// embedded profile is reported as unsigned; code signatures are not verified.
func InspectIPA(source io.ReaderAt, size int64, includeEntitlements, includeProfile bool) (IPAManifest, error) {
	if err := validateZIPDirectory(source, size); err != nil {
		return IPAManifest{Status: "unreadable"}, fmt.Errorf("open IPA: %w", err)
	}
	bounded := &zipDirectoryReader{ReaderAt: source, remaining: maxZIPDirectoryBytes + (512 << 10)}
	reader, err := zip.NewReader(bounded, size)
	bounded.remaining = -1
	if err != nil {
		return IPAManifest{Status: "unreadable"}, fmt.Errorf("open IPA: %w", err)
	}
	if len(reader.File) > maxZipEntries {
		return IPAManifest{Status: "unreadable"}, fmt.Errorf("IPA contains %d entries; limit is %d", len(reader.File), maxZipEntries)
	}
	var declared uint64
	var main *zip.File
	var nested []*zip.File
	var profile *zip.File
	for _, file := range reader.File {
		if file.UncompressedSize64 > maxZipDeclaredBytes-declared {
			return IPAManifest{Status: "unreadable"}, fmt.Errorf("IPA declared expansion exceeds the limit")
		}
		declared += file.UncompressedSize64
		name := zipMemberName(file.Name)
		if file.FileInfo().IsDir() {
			continue
		}
		if isTopLevelAppInfoPlist(name) {
			if main != nil {
				return IPAManifest{Status: "unreadable"}, fmt.Errorf("IPA has multiple top-level app Info.plist entries")
			}
			main = file
			continue
		}
		if isNestedInfoPlist(name) {
			nested = append(nested, file)
		}
	}
	if main == nil {
		return IPAManifest{Status: "unreadable"}, fmt.Errorf("IPA has no top-level app Info.plist")
	}
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

func manifestFromPlist(parsed bundlePlist) IPAManifest {
	platforms := append([]string(nil), parsed.Platforms...)
	if len(platforms) == 0 && parsed.Platform != "" {
		platforms = []string{parsed.Platform}
	}
	return IPAManifest{
		BundleID:         parsed.BundleID,
		Name:             firstNonEmpty(parsed.DisplayName, parsed.Name),
		Version:          parsed.Version,
		BuildNumber:      parsed.BuildNumber,
		MinimumOSVersion: firstNonEmpty(parsed.MinimumOSVersion, parsed.MinimumSystem),
		Platforms:        platforms,
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

// InspectPKG reads PackageInfo from a flat xar component package.
func InspectPKG(source io.ReaderAt, size int64) (PKGManifest, error) {
	files, err := readXarFiles(source, size)
	if err != nil {
		return PKGManifest{Status: "unreadable"}, err
	}
	info, ok := files["PackageInfo"]
	if !ok {
		return PKGManifest{Status: "unreadable"}, fmt.Errorf("flat pkg has no PackageInfo")
	}
	manifest, err := parsePackageInfo(info)
	if err != nil {
		manifest.Status = "unreadable"
		return manifest, err
	}
	manifest.Status = "readable"
	return manifest, nil
}

type packageInfoXML struct {
	XMLName         xml.Name        `xml:"pkg-info"`
	Version         string          `xml:"version,attr"`
	InstallLocation string          `xml:"install-location,attr"`
	Identifier      string          `xml:"identifier,attr"`
	Bundles         []packageBundle `xml:"bundle"`
}

type packageBundle struct {
	ID string `xml:"id,attr"`
}

func parsePackageInfo(data []byte) (PKGManifest, error) {
	var info packageInfoXML
	if err := xml.Unmarshal(data, &info); err != nil {
		return PKGManifest{}, fmt.Errorf("decode PackageInfo: %w", err)
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

func readXarFiles(source io.ReaderAt, size int64) (map[string][]byte, error) {
	data := make([]byte, 28)
	if size < 28 {
		return nil, fmt.Errorf("not a flat xar package")
	}
	if _, err := source.ReadAt(data, 0); err != nil {
		return nil, fmt.Errorf("read xar header: %w", err)
	}
	if string(data[:4]) != "xar!" {
		return nil, fmt.Errorf("not a flat xar package")
	}
	headerSize := int64(binary.BigEndian.Uint16(data[4:6]))
	if headerSize < 28 || headerSize > size {
		return nil, fmt.Errorf("invalid xar header size")
	}
	tocCompressed := binary.BigEndian.Uint64(data[8:16])
	tocUncompressed := binary.BigEndian.Uint64(data[16:24])
	if tocCompressed > maxXarTOCBytes || tocUncompressed > maxXarTOCBytes {
		return nil, fmt.Errorf("xar table of contents exceeds the limit")
	}
	if int64(tocCompressed) > size-headerSize {
		return nil, fmt.Errorf("xar table of contents is truncated")
	}
	tocReader, err := zlib.NewReader(io.NewSectionReader(source, headerSize, int64(tocCompressed)))
	if err != nil {
		return nil, fmt.Errorf("open xar table of contents: %w", err)
	}
	defer tocReader.Close()
	toc, err := io.ReadAll(io.LimitReader(tocReader, int64(tocUncompressed)+1))
	if err != nil {
		return nil, fmt.Errorf("read xar table of contents: %w", err)
	}
	if uint64(len(toc)) > tocUncompressed {
		return nil, fmt.Errorf("xar table of contents exceeds the declared size")
	}
	tocEnd := headerSize + int64(tocCompressed)
	return xarFilesFromTOC(toc, io.NewSectionReader(source, tocEnd, size-tocEnd), size-tocEnd)
}

type xarDocument struct {
	XMLName xml.Name  `xml:"xar"`
	Files   []xarFile `xml:"toc>file"`
}

type xarFile struct {
	Name string  `xml:"name"`
	Type string  `xml:"type"`
	Data xarData `xml:"data"`
}

type xarData struct {
	Length   int64       `xml:"length"`
	Size     int64       `xml:"size"`
	Offset   int64       `xml:"offset"`
	Encoding xarEncoding `xml:"encoding"`
}

type xarEncoding struct {
	Style string `xml:"style,attr"`
}

func xarFilesFromTOC(toc []byte, heap io.ReaderAt, heapSize int64) (map[string][]byte, error) {
	var document xarDocument
	if err := xml.Unmarshal(toc, &document); err != nil {
		return nil, fmt.Errorf("decode xar table of contents: %w", err)
	}
	files := make(map[string][]byte, 1)
	for _, file := range document.Files {
		if file.Type != "file" || file.Name != "PackageInfo" {
			continue
		}
		if file.Data.Length < 0 || file.Data.Offset < 0 || file.Data.Size < 0 || file.Data.Length > maxXarFileBytes || file.Data.Size > maxXarFileBytes {
			return nil, fmt.Errorf("xar file %q is outside the read limit", file.Name)
		}
		if file.Data.Offset > heapSize || file.Data.Length > heapSize-file.Data.Offset {
			return nil, fmt.Errorf("xar file %q is truncated", file.Name)
		}
		payload := make([]byte, int(file.Data.Length))
		if _, err := heap.ReadAt(payload, file.Data.Offset); err != nil {
			return nil, fmt.Errorf("read xar PackageInfo: %w", err)
		}
		switch file.Data.Encoding.Style {
		case "", "application/octet-stream":
		case "application/x-gzip":
			reader, err := zlib.NewReader(bytes.NewReader(payload))
			if err != nil {
				return nil, fmt.Errorf("open xar PackageInfo: %w", err)
			}
			payload, err = io.ReadAll(io.LimitReader(reader, maxXarFileBytes+1))
			reader.Close()
			if err != nil {
				return nil, fmt.Errorf("read xar PackageInfo: %w", err)
			}
			if len(payload) > maxXarFileBytes {
				return nil, fmt.Errorf("xar PackageInfo exceeds the read limit")
			}
		default:
			return nil, fmt.Errorf("xar file %q uses unsupported encoding %q", file.Name, file.Data.Encoding.Style)
		}
		if int64(len(payload)) != file.Data.Size {
			return nil, fmt.Errorf("xar PackageInfo size does not match its declaration")
		}
		if _, exists := files[file.Name]; exists {
			return nil, fmt.Errorf("xar has duplicate PackageInfo entries")
		}
		files[file.Name] = append([]byte(nil), payload...)
	}
	return files, nil
}
