package artifacts

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"go.mozilla.org/pkcs7"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
)

func signedIPA(t *testing.T, executable []byte, extra map[string][]byte) []byte {
	t.Helper()
	files := map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo", "CFBundleExecutable": "Demo"}),
		"Payload/Demo.app/Demo":       executable,
	}
	for name, data := range extra {
		files[name] = data
	}
	return zipArtifact(t, files)
}

func TestInspectIPAReportsSignerIdentityFromMainExecutable(t *testing.T) {
	chain := artifactstest.NewChain(t)
	cms, leaf := chain.CMS(t), chain.Leaf
	profile := []byte("<plist version=\"1.0\"><dict><key>Entitlements</key><dict><key>com.apple.developer.team-identifier</key><string>PROFILE01</string></dict></dict></plist>")
	ipa := signedIPA(t, artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms))), map[string][]byte{
		"Payload/Demo.app/embedded.mobileprovision": profile,
	})
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Status != "readable" || manifest.CodeSignature != "signed" || manifest.Signer == nil {
		t.Fatalf("manifest=%+v", manifest)
	}
	signer := manifest.Signer
	if signer.CommonName != artifactstest.SignerCommonName || signer.TeamID != artifactstest.TeamID || signer.Organization != "Example Corp" {
		t.Fatalf("signer=%+v", signer)
	}
	if signer.IssuerCommonName != artifactstest.IssuerCommonName || signer.SerialNumber != "1234ABCD" {
		t.Fatalf("signer=%+v", signer)
	}
	if signer.NotBefore != "2026-01-02T03:04:05Z" || signer.NotAfter != "2027-01-02T03:04:05Z" {
		t.Fatalf("validity=%s..%s", signer.NotBefore, signer.NotAfter)
	}
	if signer.SHA1Fingerprint != fmt.Sprintf("%X", sha1.Sum(leaf.Raw)) || len(signer.SHA256Fingerprint) != 64 {
		t.Fatalf("fingerprints=%s %s", signer.SHA1Fingerprint, signer.SHA256Fingerprint)
	}
	if manifest.SignerCommonName != signer.CommonName || manifest.TeamID != artifactstest.TeamID {
		t.Fatalf("signerCommonName=%q teamId=%q", manifest.SignerCommonName, manifest.TeamID)
	}
}

func TestInspectIPAFallsBackToAppDirectoryExecutableName(t *testing.T) {
	cms := artifactstest.NewChain(t).CMS(t)
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo"}),
		"Payload/Demo.app/Demo":       artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms))),
	})
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || manifest.CodeSignature != "signed" || manifest.TeamID != artifactstest.TeamID {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectIPAReadsLowestOffsetFatSlice(t *testing.T) {
	cms := artifactstest.NewChain(t).CMS(t)
	fat := artifactstest.FatMachO(artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms))), artifactstest.MachO(nil))
	ipa := signedIPA(t, fat, nil)
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || manifest.CodeSignature != "signed" || manifest.Signer == nil || manifest.Signer.TeamID != artifactstest.TeamID {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectIPAClassifiesUnsignedAndAdHocExecutables(t *testing.T) {
	tests := map[string]struct {
		executable []byte
		want       string
	}{
		"no signature command":  {artifactstest.MachO(nil), "unsigned"},
		"code directory only":   {artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot())), "ad-hoc"},
		"empty CMS wrapper":     {artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(nil))), "ad-hoc"},
		"fat without signature": {artifactstest.FatMachO(artifactstest.MachO(nil), artifactstest.MachO(nil)), "unsigned"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ipa := signedIPA(t, test.executable, nil)
			manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
			if err != nil || manifest.CodeSignature != test.want || manifest.Signer != nil || manifest.SignerCommonName != "" || manifest.CodeSignatureError != "" {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
			if manifest.BundleID != "com.example.demo" || manifest.Status != "unsigned" {
				t.Fatalf("metadata changed: %+v", manifest)
			}
		})
	}
}

func TestInspectIPAReportsMalformedSignaturesAsUnreadable(t *testing.T) {
	cms := artifactstest.NewChain(t).CMS(t)
	valid := artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms))
	hugeLength := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(hugeLength[4:8], 0xffffffff)
	badSlotOffset := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(badSlotOffset[24:28], 0x7fffffff)
	hugeCount := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(hugeCount[8:12], 0x7fffffff)
	signatureBeyondFile := artifactstest.MachO(valid)
	binary.LittleEndian.PutUint32(signatureBeyondFile[40:44], 0x7fffffff)
	tests := map[string][]byte{
		"not mach-o":             []byte("#!/bin/sh\necho hi\n"),
		"truncated header":       artifactstest.MachO(valid)[:20],
		"bad superblob magic":    artifactstest.MachO(append([]byte{0, 0, 0, 0}, valid[4:]...)),
		"superblob too long":     artifactstest.MachO(hugeLength),
		"slot outside superblob": artifactstest.MachO(badSlotOffset),
		"too many slots":         artifactstest.MachO(hugeCount),
		"signature beyond file":  signatureBeyondFile,
		"garbage CMS":            artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot([]byte{0x30, 0x80, 0x01, 0x02}))),
		"CMS without certs":      artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(testCMSWithoutCertificates(t)))),
		"no code directory":      artifactstest.MachO(artifactstest.Superblob(artifactstest.Slot{Kind: 2, Blob: artifactstest.Blob(0xfade0c01, nil)})),
	}
	for name, executable := range tests {
		t.Run(name, func(t *testing.T) {
			ipa := signedIPA(t, executable, nil)
			manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
			if err != nil {
				t.Fatalf("signature problems must not fail metadata inspection: %v", err)
			}
			if manifest.CodeSignature != "unreadable" || manifest.Signer != nil || manifest.CodeSignatureError == "" || manifest.BundleID != "com.example.demo" {
				t.Fatalf("manifest=%+v", manifest)
			}
		})
	}
}

func TestInspectIPAMissingExecutableIsUnreadableSignature(t *testing.T) {
	ipa := zipArtifact(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo", "CFBundleExecutable": "Demo"}),
	})
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || manifest.CodeSignature != "unreadable" || !strings.Contains(manifest.CodeSignatureError, "main executable") {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectIPARejectsUnsafeExecutableNames(t *testing.T) {
	for _, name := range []string{"../Other", "Sub/Demo", ".", ".."} {
		t.Run(name, func(t *testing.T) {
			ipa := zipArtifact(t, map[string][]byte{
				"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo", "CFBundleExecutable": name}),
				"Payload/Other":               artifactstest.MachO(nil),
			})
			manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
			if err != nil || manifest.CodeSignature != "unreadable" || !strings.Contains(manifest.CodeSignatureError, "CFBundleExecutable") {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
		})
	}
}

func TestInspectIPAStoredExecutableSkipsPayloadWithoutReadingIt(t *testing.T) {
	cms := artifactstest.NewChain(t).CMS(t)
	executable := artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms)))
	// Move the signature behind 8 MiB of code pages.
	const padding = 8 << 20
	signature := executable[4096:]
	executable = append(append(executable[:4096:4096], make([]byte, padding)...), signature...)
	binary.LittleEndian.PutUint32(executable[40:44], 4096+padding)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range map[string][]byte{
		"Payload/Demo.app/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo", "CFBundleExecutable": "Demo"}),
		"Payload/Demo.app/Demo":       executable,
	} {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
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
	reader := &artifactReadBudget{ReaderAt: bytes.NewReader(buffer.Bytes()), remaining: 1 << 20}
	manifest, err := InspectIPAWithOptions(reader, int64(buffer.Len()), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || manifest.CodeSignature != "signed" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestReadMachOSignatureSurvivesTruncation(t *testing.T) {
	cms := artifactstest.NewChain(t).CMS(t)
	executable := artifactstest.FatMachO(artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(cms))), artifactstest.MachO(nil))
	for length := 0; length < len(executable); length += 97 {
		data := executable[:length]
		slices, primary, err := readMachOSignaturesCapture(bytes.NewReader(data), int64(len(data)), nil)
		if err == nil && slices[primary].CodeSignature == "signed" && length < 4096 {
			t.Fatalf("length %d reported signed", length)
		}
	}
}

func TestInspectPKGReportsXarSigner(t *testing.T) {
	chain := artifactstest.NewChain(t)
	leaf := chain.Leaf
	info := []byte(`<pkg-info version="1.0" identifier="com.example.pkg"/>`)
	pkg := writeXarWithTOCExtra(t, map[string][]byte{"PackageInfo": info}, chain.XarSignature("signature"))
	manifest, err := inspectPKG(bytes.NewReader(pkg), int64(len(pkg)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.PackageSignature != "signed" || manifest.Signer == nil || manifest.Signer.TeamID != artifactstest.TeamID || manifest.SignerCommonName != leaf.Subject.CommonName || manifest.TeamID != artifactstest.TeamID {
		t.Fatalf("manifest=%+v signer=%+v", manifest, manifest.Signer)
	}
}

func TestInspectPKGReadsCMSSignatureCertificates(t *testing.T) {
	leaf := artifactstest.NewChain(t).Leaf
	signature := `<x-signature style="CMS"><offset>0</offset><size>256</size><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>` +
		base64.StdEncoding.EncodeToString(leaf.Raw) + `</X509Certificate></X509Data></KeyInfo></x-signature>`
	pkg := writeXarWithTOCExtra(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.pkg"/>`)}, signature)
	manifest, err := inspectPKG(bytes.NewReader(pkg), int64(len(pkg)), nil)
	if err != nil || manifest.PackageSignature != "signed" || manifest.Signer == nil || manifest.Signer.CommonName != leaf.Subject.CommonName {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestInspectPKGClassifiesUnsignedAndMalformedSignatures(t *testing.T) {
	leaf := artifactstest.NewChain(t).Leaf
	info := map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.pkg"/>`)}
	tests := map[string]struct {
		extra string
		want  string
	}{
		"unsigned":         {"", "unsigned"},
		"bad base64":       {`<signature style="RSA"><KeyInfo><X509Data><X509Certificate>!!!</X509Certificate></X509Data></KeyInfo></signature>`, "unreadable"},
		"bad certificate":  {`<signature style="RSA"><KeyInfo><X509Data><X509Certificate>AAAA</X509Certificate></X509Data></KeyInfo></signature>`, "unreadable"},
		"no certificates":  {`<signature style="RSA"><offset>0</offset><size>256</size></signature>`, "unreadable"},
		"too many signers": {`<signature style="RSA"><KeyInfo><X509Data>` + strings.Repeat(`<X509Certificate>`+base64.StdEncoding.EncodeToString(leaf.Raw)+`</X509Certificate>`, 33) + `</X509Data></KeyInfo></signature>`, "unreadable"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			pkg := writeXarWithTOCExtra(t, info, test.extra)
			manifest, err := inspectPKG(bytes.NewReader(pkg), int64(len(pkg)), nil)
			if err != nil || manifest.Status != "readable" {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
			if manifest.PackageSignature != test.want || manifest.Signer != nil || manifest.SignerCommonName != "" {
				t.Fatalf("manifest=%+v", manifest)
			}
			if (test.want == "unreadable") != (manifest.PackageSignatureError != "") {
				t.Fatalf("error=%q", manifest.PackageSignatureError)
			}
		})
	}
}

func testCMSWithoutCertificates(t *testing.T) []byte {
	t.Helper()
	signed, err := pkcs7.NewSignedData([]byte("content"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInspectPKGReportsSignerWhenPackageInfoIsMissing(t *testing.T) {
	chain := artifactstest.NewChain(t)
	pkg := writeXarWithTOCExtra(t, map[string][]byte{"Distribution": []byte(`<installer-gui-script/>`)}, chain.XarSignature("signature"))
	manifest, err := inspectPKG(bytes.NewReader(pkg), int64(len(pkg)), nil)
	if err == nil || manifest.Status != "unreadable" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	if manifest.PackageSignature != "signed" || manifest.Signer == nil || manifest.TeamID != artifactstest.TeamID {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestInspectIPABoundsCompressedExecutableScan(t *testing.T) {
	// 32 MiB of zeros deflates to a few KiB: a small archive that would
	// otherwise force a large inflate before the signature is reached.
	executable := append(artifactstest.MachO(nil), make([]byte, 32<<20)...)
	ipa := signedIPA(t, executable, nil)
	if len(ipa) > 1<<20 {
		t.Fatalf("fixture is not highly compressed: %d bytes", len(ipa))
	}
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || manifest.CodeSignature != "unreadable" || !strings.Contains(manifest.CodeSignatureError, "scan limit") {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

func TestCompressedExecutableScanError(t *testing.T) {
	tests := []struct {
		compressed, uncompressed uint64
		wantErr                  bool
	}{
		{compressed: 100, uncompressed: 64 << 10},
		{compressed: 100 << 20, uncompressed: 400 << 20},
		{compressed: 1 << 20, uncompressed: 32 << 20},
		{compressed: 100 << 10, uncompressed: 32 << 20, wantErr: true},
		{compressed: 1 << 30, uncompressed: (1 << 30) + 1, wantErr: true},
		{compressed: 0, uncompressed: 17 << 20, wantErr: true},
	}
	for _, test := range tests {
		if err := compressedExecutableScanError(test.compressed, test.uncompressed); (err != nil) != test.wantErr {
			t.Fatalf("compressed=%d uncompressed=%d err=%v", test.compressed, test.uncompressed, err)
		}
	}
}
