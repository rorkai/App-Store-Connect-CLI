package artifacts

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestInspectIPARejectsMalformedProfileEntitlements(t *testing.T) {
	for _, value := range []any{nil, "not a dictionary"} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			ipa := zipArtifact(t, map[string][]byte{
				"Payload/Demo.app/Info.plist":               plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
				"Payload/Demo.app/embedded.mobileprovision": malformedProfile(t, value),
			})
			manifest, err := InspectIPA(bytes.NewReader(ipa), int64(len(ipa)), true, true)
			if err == nil || manifest.Status != "unreadable" {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
		})
	}
}

func TestInspectIPAUsesOnlySelectedAppMetadata(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist":                                plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
		"Other.app/embedded.mobileprovision":                         plistXML(t, map[string]any{"Entitlements": map[string]any{"com.apple.developer.team-identifier": "OTHER"}}),
		"Payload/Other.app/PlugIns/Other.appex/Info.plist":           plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.other"}),
		"Payload/Demo.app/PlugIns/Widget.appex/Info.plist":           plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo.widget"}),
		"Payload/Demo.app/PlugIns/Widget.appex/Resources/Info.plist": []byte("not a bundle plist"),
	})
	manifest, err := InspectIPA(bytes.NewReader(ipa), int64(len(ipa)), true, true)
	if err != nil || manifest.Status != "unsigned" || manifest.TeamID != "" || len(manifest.NestedBundles) != 1 {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectIPAProfileExpirationRFC3339(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist":               plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
		"Payload/Demo.app/embedded.mobileprovision": plistXML(t, map[string]any{"ExpirationDate": time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), "Entitlements": map[string]any{}}),
	})
	manifest, err := InspectIPA(bytes.NewReader(ipa), int64(len(ipa)), false, true)
	if err != nil || manifest.Profile == nil || manifest.Profile.ExpirationDate != "2030-01-01T00:00:00Z" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectIPARejectsAmbiguousMainApp(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist":  plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
		"Payload/Other.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.other"}),
	})
	manifest, err := InspectIPA(bytes.NewReader(ipa), int64(len(ipa)), false, false)
	if err == nil || manifest.Status != "unreadable" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectIPARejectsExcessivePlistNesting(t *testing.T) {
	data := []byte(`<plist>` + strings.Repeat(`<array>`, 130) + strings.Repeat(`</array>`, 130) + `</plist>`)
	ipa := zipArtifact(t, map[string][]byte{"Payload/Demo.app/Info.plist": data})
	_, err := InspectIPA(bytes.NewReader(ipa), int64(len(ipa)), false, false)
	if err == nil || !strings.Contains(err.Error(), "nesting depth") {
		t.Fatalf("err=%v", err)
	}
}

func TestXarRejectsOverflowedPackageInfoOffset(t *testing.T) {
	toc := []byte(`<xar><toc><file><name>PackageInfo</name><type>file</type><data><offset>9223372036854775807</offset><length>1</length><encoding style="application/octet-stream"/></data></file></toc></xar>`)
	_, err := xarFilesFromTOC(toc, strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("expected invalid offset error")
	}
}

func TestInspectPKGDoesNotReadPayload(t *testing.T) {
	pkg := writeXar(t, map[string][]byte{
		"PackageInfo": []byte(`<pkg-info identifier="com.example.demo" version="1.0"/>`),
		"Payload":     make([]byte, maxXarFileBytes+1),
	})
	manifest, err := InspectPKG(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil || manifest.ProductID != "com.example.demo" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func malformedProfile(t *testing.T, value any) []byte {
	t.Helper()
	payload := map[string]any{"ProvisionedDevices": []any{"device"}}
	if value != nil {
		payload["Entitlements"] = value
	}
	return plistXML(t, payload)
}

func TestXarReadsCompressedPackageInfo(t *testing.T) {
	info := []byte(`<pkg-info identifier="com.example.demo" version="1.0"/>`)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(info); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	toc := []byte(fmt.Sprintf(`<xar><toc><file><name>PackageInfo</name><type>file</type><data><offset>0</offset><length>%d</length><size>%d</size><encoding style="application/x-gzip"/></data></file></toc></xar>`, compressed.Len(), len(info)))
	files, err := xarFilesFromTOC(toc, bytes.NewReader(compressed.Bytes()), int64(compressed.Len()))
	if err != nil || !bytes.Equal(files["PackageInfo"], info) {
		t.Fatalf("files=%q err=%v", files, err)
	}
}

func TestXarRejectsIncorrectExtractedSize(t *testing.T) {
	for _, size := range []string{"", "<size>0</size>", "<size>2</size>"} {
		toc := []byte(`<xar><toc><file><name>PackageInfo</name><type>file</type><data><offset>0</offset><length>1</length>` + size + `<encoding style="application/octet-stream"/></data></file></toc></xar>`)
		if _, err := xarFilesFromTOC(toc, strings.NewReader("x"), 1); err == nil {
			t.Fatalf("expected rejection for size %q", size)
		}
	}
}

func TestXarBoundsCompressedMetadata(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("x"), maxXarFileBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	toc := []byte(fmt.Sprintf(`<xar><toc><file><name>PackageInfo</name><type>file</type><data><offset>0</offset><length>%d</length><size>1</size><encoding style="application/x-gzip"/></data></file></toc></xar>`, compressed.Len()))
	if _, err := xarFilesFromTOC(toc, bytes.NewReader(compressed.Bytes()), int64(compressed.Len())); err == nil || !strings.Contains(err.Error(), "read limit") {
		t.Fatalf("err=%v", err)
	}
}
