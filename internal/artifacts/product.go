package artifacts

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"compress/zlib"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const (
	maxPayloadEntries = 200_000
	maxCPIONameBytes  = 4096
	cpioODCHeaderSize = 76
)

// PKGComponent is a component package embedded in a product archive.
type PKGComponent struct {
	// Path is the component's directory name inside the product archive.
	Path            string
	Identifier      string
	Version         string
	InstallLocation string
	// InstallKBytes is the size the Distribution declares, when it declares one.
	InstallKBytes *int64
	BundleIDs     []string
	// Primary marks the component that supplies the product's app metadata.
	Primary bool
	// App is read from the app bundle's Info.plist in the component payload.
	App *PKGComponentApp
}

// PKGComponentApp is the app bundle metadata read from a component payload.
type PKGComponentApp struct {
	Path             string
	BundleID         string
	Name             string
	Version          string
	BuildNumber      string
	MinimumOSVersion string
	Platforms        []string
}

type distributionXML struct {
	XMLName          xml.Name
	Product          *distributionProduct    `xml:"product"`
	Options          []distributionOptions   `xml:"options"`
	PkgRefs          []distributionPkgRef    `xml:"pkg-ref"`
	VolumeOSVersions []distributionOSVersion `xml:"volume-check>allowed-os-versions>os-version"`
	OSVersions       []distributionOSVersion `xml:"allowed-os-versions>os-version"`
}

type distributionProduct struct {
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr"`
}

type distributionOptions struct {
	HostArchitectures string `xml:"hostArchitectures,attr"`
}

type distributionPkgRef struct {
	ID            string `xml:"id,attr"`
	Version       string `xml:"version,attr"`
	InstallKBytes string `xml:"installKBytes,attr"`
	Location      string `xml:",chardata"`
}

type distributionOSVersion struct {
	Min string `xml:"min,attr"`
}

// productRef merges the pkg-ref elements that share an identifier.
type productRef struct {
	id            string
	location      string
	installKBytes *int64
}

// inspectProductArchive reads a product archive's Distribution and the
// PackageInfo of each embedded component package. The archive is readable when
// the primary component's PackageInfo parses; problems with other components
// or with app payloads are reported as warnings.
func inspectProductArchive(files []xarFile, distributionFile xarFile, heap io.ReaderAt, heapSize int64) (PKGManifest, error) {
	data, err := readXarMember(distributionFile, heap, heapSize)
	if err != nil {
		return PKGManifest{}, err
	}
	var distribution distributionXML
	if err := xml.Unmarshal(data, &distribution); err != nil {
		return PKGManifest{}, fmt.Errorf("decode Distribution: %w", err)
	}
	if name := distribution.XMLName.Local; name != "installer-gui-script" && name != "installer-script" {
		return PKGManifest{}, fmt.Errorf("distribution root element %q is not an installer script", name)
	}
	refs, err := embeddedProductRefs(distribution, files)
	if err != nil {
		return PKGManifest{}, err
	}
	var manifest PKGManifest
	if distribution.Product != nil {
		manifest.ProductID = strings.TrimSpace(distribution.Product.ID)
		manifest.Version = strings.TrimSpace(distribution.Product.Version)
	}
	for _, options := range distribution.Options {
		for _, arch := range strings.Split(options.HostArchitectures, ",") {
			if arch = strings.TrimSpace(arch); arch != "" {
				manifest.HostArchitectures = append(manifest.HostArchitectures, arch)
			}
		}
		if len(manifest.HostArchitectures) > 0 {
			break
		}
	}
	for _, version := range append(distribution.VolumeOSVersions, distribution.OSVersions...) {
		if min := strings.TrimSpace(version.Min); min != "" {
			manifest.MinimumOSVersion = min
			break
		}
	}
	seenBundleIDs := map[string]bool{}
	for index, ref := range refs {
		primary := index == 0
		component, appBundle, app, warning, err := readProductComponent(files, ref, heap, heapSize)
		component.Primary = primary
		if err != nil {
			if primary {
				return PKGManifest{}, fmt.Errorf("read primary component %s: %w", ref.location, err)
			}
			manifest.Warnings = append(manifest.Warnings, fmt.Sprintf("component %s is unreadable: %v", ref.location, err))
		}
		if warning != "" {
			manifest.Warnings = append(manifest.Warnings, warning)
		}
		manifest.Components = append(manifest.Components, component)
		for _, id := range component.BundleIDs {
			if !seenBundleIDs[id] {
				seenBundleIDs[id] = true
				manifest.BundleIDs = append(manifest.BundleIDs, id)
			}
		}
		if !primary {
			continue
		}
		manifest.ProductID = firstNonEmpty(manifest.ProductID, component.Identifier)
		manifest.Version = firstNonEmpty(manifest.Version, component.Version)
		manifest.InstallLocation = component.InstallLocation
		if appBundle != nil {
			manifest.BundleID = appBundle.ID
			manifest.BuildNumber = appBundle.Version
		}
		if app != nil {
			manifest.BundleID = firstNonEmpty(app.BundleID, manifest.BundleID)
			manifest.BuildNumber = firstNonEmpty(app.BuildNumber, manifest.BuildNumber)
			manifest.MinimumOSVersion = firstNonEmpty(manifest.MinimumOSVersion, app.MinimumOSVersion)
			manifest.Platforms = app.Platforms
		}
	}
	return manifest, nil
}

// embeddedProductRefs returns the pkg-refs that name a component package inside
// the archive, primary first. The primary is the pkg-ref whose identifier
// matches the product identifier, or else the first embedded pkg-ref.
func embeddedProductRefs(distribution distributionXML, files []xarFile) ([]productRef, error) {
	var refs []*productRef
	byID := map[string]*productRef{}
	for _, element := range distribution.PkgRefs {
		id := strings.TrimSpace(element.ID)
		if id == "" {
			continue
		}
		ref := byID[id]
		if ref == nil {
			ref = &productRef{id: id}
			byID[id] = ref
			refs = append(refs, ref)
		}
		if ref.installKBytes == nil {
			if size, err := strconv.ParseInt(strings.TrimSpace(element.InstallKBytes), 10, 64); err == nil && size >= 0 {
				ref.installKBytes = &size
			}
		}
		if location := strings.TrimSpace(element.Location); ref.location == "" && strings.HasPrefix(location, "#") {
			name := strings.TrimPrefix(location, "#")
			if unescaped, err := url.PathUnescape(name); err == nil {
				name = unescaped
			}
			ref.location = name
		}
	}
	productID := ""
	if distribution.Product != nil {
		productID = strings.TrimSpace(distribution.Product.ID)
	}
	var ordered []productRef
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.location == "" || seen[ref.location] {
			continue
		}
		seen[ref.location] = true
		if ref.id == productID {
			ordered = append([]productRef{*ref}, ordered...)
		} else {
			ordered = append(ordered, *ref)
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("product archive Distribution references no embedded component package")
	}
	for _, ref := range ordered {
		if err := uniqueXarChild(files, ref.location); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func uniqueXarChild(files []xarFile, name string) error {
	count := 0
	for _, file := range files {
		if file.Name == name {
			count++
		}
	}
	if count > 1 {
		return fmt.Errorf("xar has duplicate %q entries", name)
	}
	return nil
}

// xarChild returns the member with the given name and type, if any.
func xarChild(files []xarFile, name, fileType string) *xarFile {
	for index := range files {
		if files[index].Name == name && files[index].Type == fileType {
			return &files[index]
		}
	}
	return nil
}

// readProductComponent reads one component's PackageInfo and, when it lists an
// app bundle and the component has a payload, that app's Info.plist. A payload
// problem is returned as a warning, not an error.
func readProductComponent(files []xarFile, ref productRef, heap io.ReaderAt, heapSize int64) (PKGComponent, *packageBundle, *PKGComponentApp, string, error) {
	component := PKGComponent{Path: ref.location, InstallKBytes: ref.installKBytes}
	directory := xarChild(files, ref.location, "directory")
	if directory == nil {
		return component, nil, nil, "", fmt.Errorf("component package is missing from the archive")
	}
	infoFile := xarChild(directory.Files, "PackageInfo", "file")
	if infoFile == nil {
		return component, nil, nil, "", fmt.Errorf("component package has no PackageInfo")
	}
	if err := uniqueXarChild(directory.Files, "PackageInfo"); err != nil {
		return component, nil, nil, "", err
	}
	data, err := readXarMember(*infoFile, heap, heapSize)
	if err != nil {
		return component, nil, nil, "", err
	}
	info, err := decodePackageInfo(data)
	if err != nil {
		return component, nil, nil, "", err
	}
	component.Identifier = info.Identifier
	component.Version = info.Version
	component.InstallLocation = info.InstallLocation
	for _, bundle := range info.Bundles {
		if bundle.ID != "" {
			component.BundleIDs = append(component.BundleIDs, bundle.ID)
		}
	}
	appBundle, appPath := componentAppBundle(info.Bundles)
	if appBundle == nil {
		return component, nil, nil, "", nil
	}
	payload := xarChild(directory.Files, "Payload", "file")
	if payload == nil {
		return component, appBundle, nil, "", nil
	}
	parsed, err := readPayloadInfoPlist(*payload, heap, heapSize, appPath)
	if err != nil {
		return component, appBundle, nil, fmt.Sprintf("component %s: app Info.plist is unreadable: %v", ref.location, err), nil
	}
	fields := manifestFromPlist(parsed)
	component.App = &PKGComponentApp{
		Path:             appPath,
		BundleID:         fields.BundleID,
		Name:             fields.Name,
		Version:          fields.Version,
		BuildNumber:      fields.BuildNumber,
		MinimumOSVersion: fields.MinimumOSVersion,
		Platforms:        fields.Platforms,
	}
	return component, appBundle, component.App, "", nil
}

// componentAppBundle returns the first top-level .app bundle PackageInfo lists,
// with its payload-relative path.
func componentAppBundle(bundles []packageBundle) (*packageBundle, string) {
	for index := range bundles {
		cleaned := path.Clean(strings.TrimSpace(bundles[index].Path))
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
			continue
		}
		if strings.HasSuffix(cleaned, ".app") {
			return &bundles[index], cleaned
		}
	}
	return nil, ""
}

// readPayloadInfoPlist streams a component payload, a compressed odc cpio
// archive, until it finds the app's Info.plist. Expansion is bounded by the
// same scan and compression-ratio limits as compressed IPA executables.
func readPayloadInfoPlist(file xarFile, heap io.ReaderAt, heapSize int64, appPath string) (bundlePlist, error) {
	section, err := xarMemberSection(file, heap, heapSize)
	if err != nil {
		return bundlePlist{}, err
	}
	var stream io.Reader = section
	switch file.Data.Encoding.Style {
	case "", "application/octet-stream":
	case "application/x-gzip":
		decoder, err := zlib.NewReader(section)
		if err != nil {
			return bundlePlist{}, fmt.Errorf("open payload: %w", err)
		}
		defer decoder.Close()
		stream = decoder
	default:
		return bundlePlist{}, fmt.Errorf("payload uses unsupported xar encoding %q", file.Data.Encoding.Style)
	}
	buffered := bufio.NewReader(stream)
	magic, _ := buffered.Peek(6)
	var archive io.Reader
	switch {
	case bytes.HasPrefix(magic, []byte{0x1f, 0x8b}):
		decoder, err := gzip.NewReader(buffered)
		if err != nil {
			return bundlePlist{}, fmt.Errorf("open payload: %w", err)
		}
		defer decoder.Close()
		archive = decoder
	case bytes.HasPrefix(magic, []byte("BZh")):
		archive = bzip2.NewReader(buffered)
	case bytes.HasPrefix(magic, []byte("pbzx")):
		return bundlePlist{}, fmt.Errorf("pbzx payloads are not supported")
	case bytes.HasPrefix(magic, []byte("070707")):
		archive = buffered
	default:
		return bundlePlist{}, fmt.Errorf("payload is not a gzip, bzip2, or odc cpio archive")
	}
	limited := &payloadScanReader{reader: archive, archived: uint64(file.Data.Length)}
	wanted := map[string]bool{appPath + "/Contents/Info.plist": true, appPath + "/Info.plist": true}
	data, err := findCPIOFile(limited, wanted)
	if err != nil {
		return bundlePlist{}, err
	}
	return decodeBundlePlist(data)
}

// payloadScanReader stops a payload scan that expands past the scan limit or
// the compression ratio real app payloads reach.
type payloadScanReader struct {
	reader   io.Reader
	archived uint64
	expanded uint64
}

func (r *payloadScanReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.expanded += uint64(n)
	if r.expanded > maxCompressedExecutableScanBytes {
		return n, fmt.Errorf("payload exceeds the %d byte scan limit", uint64(maxCompressedExecutableScanBytes))
	}
	if r.expanded > minCompressionRatioCheckBytes && r.expanded/max(r.archived, 1) > maxExecutableCompressionRatio {
		return n, fmt.Errorf("payload exceeds the scan limit's compression ratio")
	}
	return n, err
}

// findCPIOFile returns the first regular file in an odc cpio stream whose
// name, without a leading "./", is wanted.
func findCPIOFile(reader io.Reader, wanted map[string]bool) ([]byte, error) {
	header := make([]byte, cpioODCHeaderSize)
	for entries := 0; entries < maxPayloadEntries; entries++ {
		if _, err := io.ReadFull(reader, header); err != nil {
			return nil, cpioReadError(err)
		}
		if string(header[:6]) != "070707" {
			return nil, fmt.Errorf("payload is not an odc cpio archive")
		}
		mode, modeErr := strconv.ParseUint(string(header[18:24]), 8, 32)
		nameSize, nameErr := strconv.ParseUint(string(header[59:65]), 8, 32)
		fileSize, sizeErr := strconv.ParseUint(string(header[65:76]), 8, 64)
		if modeErr != nil || nameErr != nil || sizeErr != nil {
			return nil, fmt.Errorf("payload has a malformed cpio header")
		}
		if nameSize < 2 || nameSize > maxCPIONameBytes {
			return nil, fmt.Errorf("payload has a cpio name of %d bytes", nameSize)
		}
		name := make([]byte, nameSize)
		if _, err := io.ReadFull(reader, name); err != nil {
			return nil, cpioReadError(err)
		}
		if name[len(name)-1] != 0 {
			return nil, fmt.Errorf("payload has an unterminated cpio name")
		}
		member := strings.TrimPrefix(string(name[:len(name)-1]), "./")
		if member == "TRAILER!!!" {
			return nil, fmt.Errorf("payload has no app Info.plist")
		}
		if wanted[member] {
			if mode&0o170000 != 0o100000 {
				return nil, fmt.Errorf("%s is not a regular file", member)
			}
			if err := infoPlistSize(fileSize); err != nil {
				return nil, err
			}
			data := make([]byte, fileSize)
			if _, err := io.ReadFull(reader, data); err != nil {
				return nil, cpioReadError(err)
			}
			return data, nil
		}
		if fileSize > math.MaxInt64 {
			return nil, fmt.Errorf("payload has a cpio entry of %d bytes", fileSize)
		}
		if _, err := io.CopyN(io.Discard, reader, int64(fileSize)); err != nil {
			return nil, cpioReadError(err)
		}
	}
	return nil, fmt.Errorf("payload exceeds %d entries", maxPayloadEntries)
}

func cpioReadError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("payload is truncated")
	}
	return fmt.Errorf("read payload: %w", err)
}
