package artifacts

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"strconv"
	"testing"

	"howett.net/plist"
)

func TestInspectIPAReadsExtensionAndProfile(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist":                      plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo", "CFBundleDisplayName": "Demo", "CFBundleShortVersionString": "1.2.3", "CFBundleVersion": "9", "MinimumOSVersion": "17.0", "CFBundleSupportedPlatforms": []any{"iPhoneOS"}}),
		"Payload/Demo.app/PlugIns/Widget.appex/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo.widget", "CFBundleDisplayName": "Widget"}),
		"Payload/Demo.app/embedded.mobileprovision":        []byte("ignore<?xml version=\"1.0\"?><plist version=\"1.0\"><dict><key>Name</key><string>Demo Profile</string><key>UUID</key><string>PROFILE-UUID</string><key>ExpirationDate</key><string>2030-01-01T00:00:00Z</string><key>ProvisionedDevices</key><array><string>device</string></array><key>Entitlements</key><dict><key>com.apple.developer.team-identifier</key><string>TEAM1</string><key>get-task-allow</key><false/></dict></dict></plist>tail"),
	})
	manifest, err := InspectIPA(ipa, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BundleID != "com.example.demo" || manifest.Version != "1.2.3" || manifest.Status != "readable" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if len(manifest.NestedBundles) != 1 || manifest.NestedBundles[0].BundleID != "com.example.demo.widget" {
		t.Fatalf("nested = %+v", manifest.NestedBundles)
	}
	if manifest.TeamID != "TEAM1" || manifest.Profile == nil || manifest.Profile.ProfileType != "ad-hoc" {
		t.Fatalf("profile = %+v team=%s", manifest.Profile, manifest.TeamID)
	}
	if manifest.Entitlements["com.apple.developer.team-identifier"] != "TEAM1" {
		t.Fatalf("entitlements = %#v", manifest.Entitlements)
	}
}

func TestInspectIPAReadsNestedBundleWithBackslashSeparators(t *testing.T) {
	ipa := storedZip(t, map[string][]byte{
		`Payload\Demo.app\Info.plist`:                      plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
		`Payload\Demo.app\PlugIns\Widget.appex\Info.plist`: plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo.widget", "CFBundleDisplayName": "Widget"}),
	})
	manifest, err := InspectIPA(ipa, false, false)
	if err != nil && manifest.Status == "unreadable" {
		t.Fatal(err)
	}
	if len(manifest.NestedBundles) != 1 || manifest.NestedBundles[0].BundleID != "com.example.demo.widget" {
		t.Fatalf("nested = %+v err=%v", manifest.NestedBundles, err)
	}
	if manifest.NestedBundles[0].Path != "Payload/Demo.app/PlugIns/Widget.appex/Info.plist" {
		t.Fatalf("path = %q", manifest.NestedBundles[0].Path)
	}
}

func TestInspectIPAUnsignedReturnsMetadata(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
	})
	manifest, err := InspectIPA(ipa, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Status != "unsigned" || manifest.BundleID != "com.example.demo" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestInspectPKGReadsPackageInfo(t *testing.T) {
	info := []byte(`<pkg-info version="1.4.0" install-location="/Applications" identifier="com.example.pkg"><bundle id="com.example.demo" path="./Demo.app"/><bundle id="com.example.demo.widget" path="./Widget.appex"/></pkg-info>`)
	pkg := writeXar(t, map[string][]byte{"PackageInfo": info})
	manifest, err := InspectPKG(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProductID != "com.example.pkg" || manifest.Version != "1.4.0" || manifest.InstallLocation != "/Applications" || manifest.Status != "readable" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if len(manifest.BundleIDs) != 2 || manifest.BundleIDs[0] != "com.example.demo" {
		t.Fatalf("bundle IDs = %#v", manifest.BundleIDs)
	}
}

func storedZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	type entry struct {
		name string
		data []byte
	}
	ordered := make([]entry, 0, len(files))
	for name, data := range files {
		ordered = append(ordered, entry{name: name, data: data})
	}
	var body, directory bytes.Buffer
	for _, item := range ordered {
		crc := crc32.ChecksumIEEE(item.data)
		local := make([]byte, 30)
		binary.LittleEndian.PutUint32(local[0:4], 0x04034b50)
		binary.LittleEndian.PutUint16(local[4:6], 20)
		binary.LittleEndian.PutUint32(local[14:18], crc)
		binary.LittleEndian.PutUint32(local[18:22], uint32(len(item.data)))
		binary.LittleEndian.PutUint32(local[22:26], uint32(len(item.data)))
		binary.LittleEndian.PutUint16(local[26:28], uint16(len(item.name)))
		offset := body.Len()
		body.Write(local)
		body.WriteString(item.name)
		body.Write(item.data)

		central := make([]byte, 46)
		binary.LittleEndian.PutUint32(central[0:4], 0x02014b50)
		binary.LittleEndian.PutUint16(central[4:6], 20)
		binary.LittleEndian.PutUint16(central[6:8], 20)
		binary.LittleEndian.PutUint32(central[16:20], crc)
		binary.LittleEndian.PutUint32(central[20:24], uint32(len(item.data)))
		binary.LittleEndian.PutUint32(central[24:28], uint32(len(item.data)))
		binary.LittleEndian.PutUint16(central[28:30], uint16(len(item.name)))
		binary.LittleEndian.PutUint32(central[42:46], uint32(offset))
		directory.Write(central)
		directory.WriteString(item.name)
	}
	var end bytes.Buffer
	endHeader := make([]byte, 22)
	binary.LittleEndian.PutUint32(endHeader[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(endHeader[8:10], uint16(len(ordered)))
	binary.LittleEndian.PutUint16(endHeader[10:12], uint16(len(ordered)))
	binary.LittleEndian.PutUint32(endHeader[12:16], uint32(directory.Len()))
	binary.LittleEndian.PutUint32(endHeader[16:20], uint32(body.Len()))
	end.Write(endHeader)
	return append(append(body.Bytes(), directory.Bytes()...), end.Bytes()...)
}

func zipArtifact(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func plistXML(t *testing.T, value map[string]any) []byte {
	t.Helper()
	data, err := plist.Marshal(value, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeXar(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var heap bytes.Buffer
	var filesXML bytes.Buffer
	id := 1
	for name, data := range files {
		offset := heap.Len()
		heap.Write(data)
		filesXML.WriteString(`<file id="` + strconv.Itoa(id) + `"><name>` + name + `</name><type>file</type><data><length>` + strconv.Itoa(len(data)) + `</length><offset>` + strconv.Itoa(offset) + `</offset><size>` + strconv.Itoa(len(data)) + `</size><encoding style="application/octet-stream"/></data></file>`)
		id++
	}
	toc := []byte(`<xar><toc>` + filesXML.String() + `</toc></xar>`)
	var compressed bytes.Buffer
	encoder := zlib.NewWriter(&compressed)
	if _, err := encoder.Write(toc); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 28)
	copy(header[:4], "xar!")
	binary.BigEndian.PutUint16(header[4:6], 28)
	binary.BigEndian.PutUint16(header[6:8], 1)
	binary.BigEndian.PutUint64(header[8:16], uint64(compressed.Len()))
	binary.BigEndian.PutUint64(header[16:24], uint64(len(toc)))
	binary.BigEndian.PutUint32(header[24:28], 0)
	return append(append(header, compressed.Bytes()...), heap.Bytes()...)
}
