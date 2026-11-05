package artifacts

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
)

const productDistribution = `<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
    <pkg-ref id="com.example.demo">
        <bundle-version><bundle CFBundleShortVersionString="2.3.4" CFBundleVersion="56" id="com.example.demo" path="Demo.app"/></bundle-version>
    </pkg-ref>
    <product id="com.example.demo" version="2.3.4"/>
    <title>Demo</title>
    <options customize="never" require-scripts="false" hostArchitectures="x86_64,arm64"/>
    <volume-check><allowed-os-versions><os-version min="13.0"/></allowed-os-versions></volume-check>
    <choices-outline><line choice="default"><line choice="com.example.demo"/></line></choices-outline>
    <choice id="com.example.demo" visible="false"><pkg-ref id="com.example.demo"/></choice>
    <pkg-ref id="com.example.helper" version="1.0" installKBytes="12">#Helper.pkg</pkg-ref>
    <pkg-ref id="com.example.demo" version="2.3.4" onConclusion="none" installKBytes="2048">#Demo.pkg</pkg-ref>
</installer-gui-script>`

const productPackageInfo = `<pkg-info identifier="com.example.demo" version="2.3.4" install-location="/Applications"><payload numberOfFiles="5" installKBytes="2048"/><bundle path="./Demo.app" id="com.example.demo" CFBundleShortVersionString="2.3.4" CFBundleVersion="56"/><bundle-version><bundle id="com.example.demo"/></bundle-version></pkg-info>`

func productAppPlist(t *testing.T) []byte {
	t.Helper()
	return plistXML(t, map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleName":               "Demo",
		"CFBundleShortVersionString": "2.3.4",
		"CFBundleVersion":            "57",
		"LSMinimumSystemVersion":     "12.0",
		"CFBundleSupportedPlatforms": []any{"MacOSX"},
	})
}

func productArchive(t *testing.T, payload []byte, overrides map[string][]byte) []byte {
	t.Helper()
	files := map[string][]byte{
		"Distribution":           []byte(productDistribution),
		"Demo.pkg/PackageInfo":   []byte(productPackageInfo),
		"Demo.pkg/Payload":       payload,
		"Demo.pkg/Bom":           []byte("bom"),
		"Helper.pkg/PackageInfo": []byte(`<pkg-info identifier="com.example.helper" version="1.0" install-location="/Library/Helper"><bundle path="./Helper.app" id="com.example.helper"/></pkg-info>`),
	}
	for name, data := range overrides {
		if data == nil {
			delete(files, name)
			continue
		}
		files[name] = data
	}
	return artifactstest.XarTree(t, files, "")
}

func demoPayload(t *testing.T, plist []byte, before ...artifactstest.CPIOEntry) []byte {
	t.Helper()
	entries := []artifactstest.CPIOEntry{
		{Name: ".", Mode: artifactstest.ModeDirectory},
		{Name: "./Demo.app", Mode: artifactstest.ModeDirectory},
		{Name: "./Demo.app/Contents", Mode: artifactstest.ModeDirectory},
	}
	entries = append(entries, before...)
	entries = append(
		entries,
		artifactstest.CPIOEntry{Name: "./Demo.app/Contents/Info.plist", Data: plist},
		artifactstest.CPIOEntry{Name: "./Demo.app/Contents/MacOS/Demo", Data: []byte("#!/bin/sh\n")},
	)
	return artifactstest.GzipCPIO(t, entries)
}

func TestInspectPKGReadsProductArchive(t *testing.T) {
	pkg := productArchive(t, demoPayload(t, productAppPlist(t)), nil)
	manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Status != "readable" || manifest.ProductID != "com.example.demo" || manifest.Version != "2.3.4" || manifest.InstallLocation != "/Applications" {
		t.Fatalf("manifest=%+v", manifest)
	}
	if manifest.BundleID != "com.example.demo" || manifest.BuildNumber != "57" || manifest.MinimumOSVersion != "13.0" {
		t.Fatalf("app fields=%+v", manifest)
	}
	if !reflect.DeepEqual(manifest.Platforms, []string{"MacOSX"}) || !reflect.DeepEqual(manifest.HostArchitectures, []string{"x86_64", "arm64"}) {
		t.Fatalf("platforms=%v architectures=%v", manifest.Platforms, manifest.HostArchitectures)
	}
	if !reflect.DeepEqual(manifest.BundleIDs, []string{"com.example.demo", "com.example.helper"}) || len(manifest.Warnings) != 0 {
		t.Fatalf("bundleIDs=%v warnings=%v", manifest.BundleIDs, manifest.Warnings)
	}
	demoKB, helperKB := int64(2048), int64(12)
	want := []PKGComponent{
		{
			Path: "Demo.pkg", Identifier: "com.example.demo", Version: "2.3.4", InstallLocation: "/Applications", InstallKBytes: &demoKB,
			BundleIDs: []string{"com.example.demo"}, Primary: true,
			App: &PKGComponentApp{Path: "Demo.app", BundleID: "com.example.demo", Name: "Demo", Version: "2.3.4", BuildNumber: "57", MinimumOSVersion: "12.0", Platforms: []string{"MacOSX"}},
		},
		{Path: "Helper.pkg", Identifier: "com.example.helper", Version: "1.0", InstallLocation: "/Library/Helper", InstallKBytes: &helperKB, BundleIDs: []string{"com.example.helper"}},
	}
	if !reflect.DeepEqual(manifest.Components, want) {
		t.Fatalf("components=%+v", manifest.Components)
	}
}

func TestInspectPKGProductArchiveFallsBackToPackageInfoWhenPayloadIsUnreadable(t *testing.T) {
	for name, payload := range map[string][]byte{
		"pbzx":      append([]byte("pbzx"), make([]byte, 32)...),
		"truncated": demoPayload(t, productAppPlist(t))[:40],
		"garbage":   []byte("not an archive"),
	} {
		t.Run(name, func(t *testing.T) {
			pkg := productArchive(t, payload, nil)
			manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
			if err != nil || manifest.Status != "readable" {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
			if manifest.BundleID != "com.example.demo" || manifest.BuildNumber != "56" || manifest.Platforms != nil || manifest.Components[0].App != nil {
				t.Fatalf("manifest=%+v", manifest)
			}
			if len(manifest.Warnings) != 1 || !strings.Contains(manifest.Warnings[0], "Demo.pkg") {
				t.Fatalf("warnings=%v", manifest.Warnings)
			}
		})
	}
}

func TestInspectPKGProductArchiveWithoutPayloadSkipsAppMetadata(t *testing.T) {
	pkg := productArchive(t, nil, map[string][]byte{"Demo.pkg/Payload": nil})
	manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil || manifest.Status != "readable" || manifest.BuildNumber != "56" || len(manifest.Warnings) != 0 {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectPKGProductArchiveBoundsPayloadExpansion(t *testing.T) {
	// 32 MiB of zeros ahead of Info.plist compresses far beyond the ratio limit.
	payload := demoPayload(t, productAppPlist(t), artifactstest.CPIOEntry{Name: "./Demo.app/Contents/Frameworks/Zero", Data: make([]byte, 32<<20)})
	if len(payload) > 1<<20 {
		t.Fatalf("fixture is not highly compressed: %d bytes", len(payload))
	}
	pkg := productArchive(t, payload, nil)
	manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil || manifest.Status != "readable" || manifest.Components[0].App != nil {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	if len(manifest.Warnings) != 1 || !strings.Contains(manifest.Warnings[0], "compression ratio") {
		t.Fatalf("warnings=%v", manifest.Warnings)
	}
}

func TestInspectPKGProductArchiveRejectsUnsafeInfoPlistEntries(t *testing.T) {
	tests := map[string]artifactstest.CPIOEntry{
		"symlink":   {Name: "./Demo.app/Contents/Info.plist", Mode: artifactstest.ModeSymlink, Data: []byte("/etc/passwd")},
		"oversized": {Name: "./Demo.app/Contents/Info.plist", Data: make([]byte, maxPlistBytes+1)},
	}
	for name, entry := range tests {
		t.Run(name, func(t *testing.T) {
			payload := artifactstest.GzipCPIO(t, []artifactstest.CPIOEntry{entry})
			pkg := productArchive(t, payload, nil)
			manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
			if err != nil || manifest.Status != "readable" || manifest.Components[0].App != nil || len(manifest.Warnings) != 1 {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
		})
	}
}

func TestInspectPKGProductArchiveRequiresPrimaryComponent(t *testing.T) {
	tests := map[string]map[string][]byte{
		"missing component":      {"Demo.pkg/PackageInfo": nil, "Demo.pkg/Payload": nil, "Demo.pkg/Bom": nil},
		"malformed component":    {"Demo.pkg/PackageInfo": []byte("<pkg-info")},
		"malformed distribution": {"Distribution": []byte("<installer-gui-script>")},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			pkg := productArchive(t, demoPayload(t, productAppPlist(t)), overrides)
			manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
			if err == nil || manifest.Status != "unreadable" {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
		})
	}
}

func TestInspectPKGProductArchiveToleratesBrokenSecondaryComponent(t *testing.T) {
	pkg := productArchive(t, demoPayload(t, productAppPlist(t)), map[string][]byte{"Helper.pkg/PackageInfo": []byte("<pkg-info")})
	manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil || manifest.Status != "readable" || len(manifest.Components) != 2 || manifest.Components[1].Identifier != "" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	if len(manifest.Warnings) != 1 || !strings.Contains(manifest.Warnings[0], "Helper.pkg") {
		t.Fatalf("warnings=%v", manifest.Warnings)
	}
}

func TestInspectPKGFlatPackageHasNoComponents(t *testing.T) {
	pkg := writeXar(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.pkg" version="1.0"><bundle id="com.example.demo" path="./Demo.app" CFBundleVersion="9"/></pkg-info>`)})
	manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil || manifest.Components != nil || manifest.BundleID != "" || manifest.BuildNumber != "" || manifest.Warnings != nil {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

// TestFindCPIOFileSkipsLargestDeclaredEntryAsTruncated covers the skip path
// with the largest size an odc header can declare (11 octal digits): the
// entry is skipped without overflow and the short stream reports truncation.
func TestFindCPIOFileSkipsLargestDeclaredEntryAsTruncated(t *testing.T) {
	name := "./other"
	var archive bytes.Buffer
	fmt.Fprintf(&archive, "070707%06o%06o%06o%06o%06o%06o%06o%011o%06o%011o", 0, 1, 0o100644, 0, 0, 1, 0, 0, len(name)+1, uint64(0o77777777777))
	archive.WriteString(name + "\x00")
	archive.WriteString("short")

	_, err := findCPIOFile(&archive, map[string]bool{"Demo.app/Contents/Info.plist": true})
	if err == nil || err.Error() != "payload is truncated" {
		t.Fatalf("findCPIOFile() error = %v, want payload is truncated", err)
	}
}
