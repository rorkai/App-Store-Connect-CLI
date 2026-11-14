package artifacts

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
)

var (
	validFrom  = time.Now().Add(-24 * time.Hour)
	validUntil = time.Now().AddDate(1, 0, 0)
)

func testPolicy(chain artifactstest.TrustChain) *trustPolicy {
	policy := &trustPolicy{roots: x509.NewCertPool(), rootNames: map[string]string{}, now: time.Now}
	policy.addRoot(chain.Root)
	return policy
}

var testInfoPlist = []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleExecutable</key><string>Demo</string></dict></plist>`)

var testCodeResources = []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>files</key><dict/></dict></plist>`)

// verificationIPA packages executable with the sealed bundle files.
func verificationIPA(t *testing.T, executable []byte, method uint16, overrides map[string][]byte) []byte {
	t.Helper()
	files := map[string][]byte{
		"Payload/Demo.app/Info.plist":                   testInfoPlist,
		"Payload/Demo.app/Demo":                         executable,
		"Payload/Demo.app/_CodeSignature/CodeResources": testCodeResources,
		"Payload/Demo.app/embedded.mobileprovision":     []byte("<plist version=\"1.0\"><dict><key>Entitlements</key><dict/></dict></plist>"),
		"Payload/Demo.app/PlugIns/Ext.appex/Info.plist": plistXML(t, map[string]any{"CFBundleIdentifier": "com.example.demo.ext"}),
	}
	for name, data := range overrides {
		if data == nil {
			delete(files, name)
			continue
		}
		files[name] = data
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: method})
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
	return buffer.Bytes()
}

func verifyIPA(t *testing.T, ipa []byte, policy *trustPolicy) SignatureVerification {
	t.Helper()
	manifest, _ := inspectIPAVerifying(bytes.NewReader(ipa), int64(len(ipa)), false, false, policy)
	if manifest.SignatureVerification == nil {
		t.Fatal("verification result is missing")
	}
	return *manifest.SignatureVerification
}

func sealedOptions(chain *artifactstest.TrustChain) artifactstest.CodeSignatureOptions {
	return artifactstest.CodeSignatureOptions{
		Chain:         chain,
		InfoPlist:     testInfoPlist,
		CodeResources: testCodeResources,
		Entitlements:  []byte(`<plist version="1.0"><dict/></plist>`),
	}
}

func expectVerification(t *testing.T, got SignatureVerification, status, detail string) {
	t.Helper()
	if got.Status != status || !strings.Contains(got.Detail, detail) {
		t.Fatalf("verification=%+v; want %s containing %q", got, status, detail)
	}
}

func TestVerifyIPAAcceptsValidSignature(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	for name, method := range map[string]uint16{"stored": zip.Store, "deflated": zip.Deflate} {
		t.Run(name, func(t *testing.T) {
			ipa := verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&chain)), method, nil)
			expectVerification(t, verifyIPA(t, ipa, testPolicy(chain)), VerificationValid, "chain to Test Root CA verified at current time")
		})
	}
}

func TestVerifyIPAAcceptsSealedLaunchConstraint(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	options := sealedOptions(&chain)
	options.LaunchConstraint = []byte{0x70, 0x00}
	ipa := verificationIPA(t, artifactstest.SignedMachO(t, options), zip.Deflate, nil)
	expectVerification(t, verifyIPA(t, ipa, testPolicy(chain)), VerificationValid, "chain to Test Root CA verified at current time")
}

func TestVerifyIPAAcceptsBoundAlternateCodeDirectories(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	options := sealedOptions(&chain)
	options.HashTypes = []uint8{1, 2}
	ipa := verificationIPA(t, artifactstest.SignedMachO(t, options), zip.Deflate, nil)
	expectVerification(t, verifyIPA(t, ipa, testPolicy(chain)), VerificationValid, "2 code directories and 4 code pages verified")

	options.OmitCDHashes = true
	ipa = verificationIPA(t, artifactstest.SignedMachO(t, options), zip.Deflate, nil)
	expectVerification(t, verifyIPA(t, ipa, testPolicy(chain)), VerificationInvalid, "not bound to the CMS signature")
}

func TestVerifyIPARejectsTampering(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	policy := testPolicy(chain)
	signed := artifactstest.SignedMachO(t, sealedOptions(&chain))
	directoryAt := bytes.Index(signed, []byte("com.example.demo\x00"))
	entitlementsAt := bytes.Index(signed, []byte("<plist version=\"1.0\"><dict/></plist>"))
	if directoryAt < 0 || entitlementsAt < 0 {
		t.Fatal("fixture layout changed")
	}
	flip := func(offset int) []byte {
		tampered := append([]byte(nil), signed...)
		tampered[offset] ^= 0x01
		return tampered
	}
	withOptions := func(change func(*artifactstest.CodeSignatureOptions)) []byte {
		options := sealedOptions(&chain)
		change(&options)
		return artifactstest.SignedMachO(t, options)
	}
	tests := map[string]struct {
		executable []byte
		overrides  map[string][]byte
		detail     string
	}{
		"executable page":        {executable: flip(2 * 4096), detail: "code page 2 does not match"},
		"final partial page":     {executable: flip(artifactstest.CodeSize - 1), detail: "code page 3 does not match"},
		"code directory":         {executable: flip(directoryAt), detail: "CMS message digest does not match"},
		"entitlements blob":      {executable: flip(entitlementsAt + 10), detail: "entitlements blob does not match"},
		"Info.plist":             {executable: signed, overrides: map[string][]byte{"Payload/Demo.app/Info.plist": bytes.Replace(testInfoPlist, []byte("com.example.demo"), []byte("com.example.evil"), 1)}, detail: "Info.plist does not match"},
		"CodeResources":          {executable: signed, overrides: map[string][]byte{"Payload/Demo.app/_CodeSignature/CodeResources": append(append([]byte(nil), testCodeResources...), ' ')}, detail: "CodeResources does not match"},
		"unsealed Info.plist":    {executable: artifactstest.SignedMachO(t, artifactstest.CodeSignatureOptions{Chain: &chain, CodeResources: testCodeResources}), detail: "bundle Info.plist is not sealed"},
		"unsealed CodeResources": {executable: artifactstest.SignedMachO(t, artifactstest.CodeSignatureOptions{Chain: &chain, InfoPlist: testInfoPlist}), detail: "bundle CodeResources is not sealed"},
		"missing sealed file":    {executable: signed, overrides: map[string][]byte{"Payload/Demo.app/_CodeSignature/CodeResources": nil}, detail: "seals CodeResources, but the bundle has none"},
		"stripped requirements":  {executable: withOptions(func(options *artifactstest.CodeSignatureOptions) { options.OmitBlobs = []uint32{2} }), detail: "code directory seals requirements, but the signature has no requirements blob"},
		"stripped entitlements":  {executable: withOptions(func(options *artifactstest.CodeSignatureOptions) { options.OmitBlobs = []uint32{5} }), detail: "code directory seals entitlements, but the signature has no entitlements blob"},
		"stripped launch constraint": {executable: withOptions(func(options *artifactstest.CodeSignatureOptions) {
			options.LaunchConstraint = []byte{0x70, 0x00}
			options.OmitBlobs = []uint32{8}
		}), detail: "code directory seals self launch constraint, but the signature has no self launch constraint blob"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ipa := verificationIPA(t, test.executable, zip.Deflate, test.overrides)
			expectVerification(t, verifyIPA(t, ipa, policy), VerificationInvalid, test.detail)
		})
	}
}

func TestVerifyIPAClassifiesChainProblems(t *testing.T) {
	// Legacy platform chains must remain untrusted even when the signer,
	// integrity, root, and validity window are otherwise acceptable.
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for name, algorithms := range map[string][2]x509.SignatureAlgorithm{
		"SHA-1 leaf":         {x509.ECDSAWithSHA1, x509.ECDSAWithSHA256},
		"SHA-1 intermediate": {x509.ECDSAWithSHA256, x509.ECDSAWithSHA1},
		"modern chain":       {x509.ECDSAWithSHA256, x509.ECDSAWithSHA256},
	} {
		t.Run(name, func(t *testing.T) {
			chain := artifactstest.NewTrustChainWithAlgorithms(t, artifactstest.AppleSoftwareSigningLeaf, at.Add(-time.Hour), at.Add(time.Hour), algorithms[0], algorithms[1])
			policy := testPolicy(chain)
			policy.now = func() time.Time { return at }
			ipa := verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&chain)), zip.Deflate, nil)
			want := VerificationUntrustedChain
			if name == "modern chain" {
				want = VerificationValid
			}
			expectVerification(t, verifyIPA(t, ipa, policy), want, "CMS signature and 1 code directory and 4 code pages verified")
		})
	}

	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	ipa := verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&chain)), zip.Deflate, nil)
	apple := appleTrustPolicyForAnyOS(t)
	expectVerification(t, verifyIPA(t, ipa, apple), VerificationUntrustedChain, "does not chain to an embedded Apple root")

	stranger := artifactstest.NewTrustChain(t, validFrom, validUntil)
	expectVerification(t, verifyIPA(t, ipa, testPolicy(stranger)), VerificationUntrustedChain, "CMS signature and 1 code directory")

	expired := artifactstest.NewTrustChain(t, time.Now().AddDate(-2, 0, 0), time.Now().AddDate(-1, 0, 0))
	ipa = verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&expired)), zip.Deflate, nil)
	expectVerification(t, verifyIPA(t, ipa, testPolicy(expired)), VerificationExpired, "not valid at current time")

	// Expired and untrusted reports the missing trust, not the expiry.
	expectVerification(t, verifyIPA(t, ipa, apple), VerificationUntrustedChain, "unknown authority")
}

func TestVerifyIPAClassifiesUnsignedAdHocAndUnreadable(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	policy := testPolicy(chain)
	adHoc := artifactstest.SignedMachO(t, sealedOptions(nil))
	expectVerification(t, verifyIPA(t, verificationIPA(t, adHoc, zip.Deflate, nil), policy), VerificationUntrustedChain, "ad-hoc signature has no signing certificate")
	tamperedAdHoc := append([]byte(nil), adHoc...)
	tamperedAdHoc[5000] ^= 0x01
	expectVerification(t, verifyIPA(t, verificationIPA(t, tamperedAdHoc, zip.Deflate, nil), policy), VerificationInvalid, "code page 1")
	expectVerification(t, verifyIPA(t, verificationIPA(t, artifactstest.MachO(nil), zip.Deflate, nil), policy), VerificationInvalid, "no code signature")
	expectVerification(t, verifyIPA(t, verificationIPA(t, []byte("#!/bin/sh\n"), zip.Deflate, nil), policy), VerificationUnsupported, "code signature could not be read")
	expectVerification(t, verifyIPA(t, []byte("not a zip"), policy), VerificationUnsupported, "IPA could not be inspected")
}

func TestVerifyIPAHandlesMalformedSignaturesWithoutPanicking(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	policy := testPolicy(chain)
	signed := artifactstest.SignedMachO(t, sealedOptions(&chain))
	superblobEnd := artifactstest.CodeSize + int(binary.BigEndian.Uint32(signed[artifactstest.CodeSize+4:]))
	for offset := artifactstest.CodeSize; offset < superblobEnd; offset += 5 {
		for _, value := range []byte{0x00, 0xff} {
			mutated := append([]byte(nil), signed...)
			mutated[offset] = value
			if result := verifyMachOBytes(mutated, policy); result.Status == "" || result.Detail == "" {
				t.Fatalf("offset %d: empty result %+v", offset, result)
			}
		}
	}
	for length := 0; length < len(signed); length += 509 {
		if result := verifyMachOBytes(signed[:length], policy); result.Status == VerificationValid {
			t.Fatalf("truncated to %d bytes verified as valid", length)
		}
	}
}

func TestVerifyPKG(t *testing.T) {
	chain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, validFrom, validUntil)
	policy := testPolicy(chain)
	info := map[string][]byte{"PackageInfo": []byte(`<pkg-info version="1.0" identifier="com.example.pkg"/>`)}
	valid := artifactstest.SignedXar(t, info, chain, time.Now())
	verify := func(data []byte, policy *trustPolicy) SignatureVerification {
		t.Helper()
		manifest, _ := inspectPKGVerifying(bytes.NewReader(data), int64(len(data)), policy)
		if manifest.SignatureVerification == nil {
			t.Fatal("verification result is missing")
		}
		return *manifest.SignatureVerification
	}
	expectVerification(t, verify(valid, policy), VerificationValid, "chain to Test Root CA verified at current time")

	// Older productsign releases signed the SHA-1 digest of the checksum;
	// pkgutil accepts that form, so the verifier does too.
	legacy := artifactstest.SignedXarHashingChecksum(t, info, chain, time.Now())
	expectVerification(t, verify(legacy, policy), VerificationValid, "chain to Test Root CA verified at current time")
	legacyDocument, _, _, err := readXarFiles(bytes.NewReader(legacy), int64(len(legacy)))
	if err != nil {
		t.Fatal(err)
	}
	legacyChecksum, err := verifiedXarChecksum(bytes.NewReader(legacy), int64(len(legacy)), legacyDocument)
	if err != nil {
		t.Fatal(err)
	}
	legacyTampered := append([]byte(nil), legacy...)
	legacyTampered[legacyChecksum.heap+30] ^= 0x01
	expectVerification(t, verify(legacyTampered, policy), VerificationInvalid, "RSA signature does not verify")

	document, _, _, err := readXarFiles(bytes.NewReader(valid), int64(len(valid)))
	if err != nil {
		t.Fatal(err)
	}
	checksum, err := verifiedXarChecksum(bytes.NewReader(valid), int64(len(valid)), document)
	if err != nil {
		t.Fatal(err)
	}
	flip := func(offset int64) []byte {
		tampered := append([]byte(nil), valid...)
		tampered[offset] ^= 0x01
		return tampered
	}
	expectVerification(t, verify(flip(checksum.heap+5), policy), VerificationInvalid, "checksum does not match the table of contents")
	expectVerification(t, verify(flip(checksum.heap+30), policy), VerificationInvalid, "RSA signature does not verify")

	// A table of contents rewritten after signing no longer matches the checksum.
	resigned := artifactstest.SignedXar(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info version="10.0" identifier="com.example.pkg"/>`)}, chain, time.Now())
	resignedDocument, _, _, _ := readXarFiles(bytes.NewReader(resigned), int64(len(resigned)))
	resignedChecksum, _ := verifiedXarChecksum(bytes.NewReader(resigned), int64(len(resigned)), resignedDocument)
	swapped := append(append([]byte(nil), resigned[:resignedChecksum.heap]...), valid[checksum.heap:]...)
	expectVerification(t, verify(swapped, policy), VerificationInvalid, "checksum does not match")

	expectVerification(t, verify(valid, appleTrustPolicyForAnyOS(t)), VerificationUntrustedChain, "does not chain to an embedded Apple root")
	expired := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, time.Now().AddDate(-2, 0, 0), time.Now().AddDate(-1, 0, 0))
	expectVerification(t, verify(artifactstest.SignedXar(t, info, expired, time.Now()), testPolicy(expired)), VerificationExpired, "not valid at current time")
	// A backdated creation time does not rescue an expired certificate.
	expectVerification(t, verify(artifactstest.SignedXar(t, info, expired, time.Now().AddDate(-1, -6, 0)), testPolicy(expired)), VerificationExpired, "not valid at current time")

	expectVerification(t, verify(writeXar(t, info), policy), VerificationInvalid, "package is unsigned")
	expectVerification(t, verify([]byte("xar!garbage"), policy), VerificationUnsupported, "table of contents could not be read")
	xSignatureOnly := writeXarWithTOCExtra(t, info, artifactstest.NewChain(t).XarSignature("x-signature"))
	expectVerification(t, verify(xSignatureOnly, policy), VerificationUnsupported, "only a CMS x-signature")
	unreadable := writeXarWithTOCExtra(t, info, `<signature style="RSA"><KeyInfo><X509Data><X509Certificate>AAAA</X509Certificate></X509Data></KeyInfo></signature>`)
	expectVerification(t, verify(unreadable, policy), VerificationUnsupported, "package signature could not be read")
	// A signature element over a header without a checksum cannot verify.
	unchecksummed := writeXarWithTOCExtra(t, info, artifactstest.NewChain(t).XarSignature("signature"))
	expectVerification(t, verify(unchecksummed, policy), VerificationInvalid, "has no checksum")

	for length := 0; length < len(valid); length += 97 {
		if result := verify(valid[:length], policy); result.Status == VerificationValid {
			t.Fatalf("truncated to %d bytes verified as valid", length)
		}
	}
}

func TestInspectWithoutVerificationLeavesResultUnset(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	ipa := verificationIPA(t, artifactstest.SignedMachO(t, sealedOptions(&chain)), zip.Deflate, nil)
	manifest, err := InspectIPAWithOptions(bytes.NewReader(ipa), int64(len(ipa)), IPAOptions{IncludeEntitlements: false, IncludeProfile: false})
	if err != nil || manifest.SignatureVerification != nil || manifest.CodeSignature != SignatureSigned {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	pkg := artifactstest.SignedXar(t, map[string][]byte{"PackageInfo": []byte(`<pkg-info identifier="com.example.pkg"/>`)}, chain, time.Now())
	pkgManifest, err := inspectPKG(bytes.NewReader(pkg), int64(len(pkg)), nil)
	if err != nil || pkgManifest.SignatureVerification != nil || pkgManifest.PackageSignature != SignatureSigned {
		t.Fatalf("manifest=%+v err=%v", pkgManifest, err)
	}
}

func appleTrustPolicyForAnyOS(t *testing.T) *trustPolicy {
	t.Helper()
	policy, err := appleTrustPolicy()
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

// The embedded certificates are pinned by SHA-256 so a changed PEM is a
// reviewable test change. Sources: macOS SystemRootCertificates.keychain for
// the roots, and Apple's certificate authority files for the intermediates.
func TestEmbeddedAppleCertificates(t *testing.T) {
	want := map[string]string{
		"root-apple-root-ca.pem":           "b0b1730ecbc7ff4505142c49f1295e6eda6bcaed7e2c68c5be91b5a11001f024",
		"root-apple-root-ca-g3.pem":        "63343abfb89a6a03ebb57e9b3f5fa7be7c4f5c756f3017b3a8c488c3653e9179",
		"intermediate-apple-wwdr-g3.pem":   "dcf21878c77f4198e4b4614f03d696d89c66c66008d4244e1b99161aac91601f",
		"intermediate-apple-wwdr-g5.pem":   "53fd008278e5a595fe1e908ae9c5e5675f26243264a5a6438c023e3ce2870760",
		"intermediate-apple-wwdr-g6.pem":   "bdd4ed6e74691f0c2bfd01be0296197af1379e0418e2d300efa9c3bef642ca30",
		"intermediate-developer-id.pem":    "7afc9d01a62f03a2de9637936d4afe68090d2de18d03f29c88cfb0b1ba63587f",
		"intermediate-developer-id-g2.pem": "f16cd3c54c7f83cea4bf1a3e6a0819c8aaa8e4a1528fd144715f350643d2df3a",
	}
	entries, err := fs.ReadDir(appleCertificates, "applecerts")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Fatalf("embedded %d certificates; want %d", len(entries), len(want))
	}
	policy := appleTrustPolicyForAnyOS(t)
	for _, entry := range entries {
		data, err := fs.ReadFile(appleCertificates, "applecerts/"+entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		block, _ := pem.Decode(data)
		if block == nil {
			t.Fatalf("%s is not PEM", entry.Name())
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(certificate.Raw)
		if hex.EncodeToString(digest[:]) != want[entry.Name()] {
			t.Fatalf("%s fingerprint = %x", entry.Name(), digest)
		}
		if strings.HasPrefix(entry.Name(), "intermediate-") {
			if _, err := certificate.Verify(x509.VerifyOptions{Roots: policy.roots, CurrentTime: certificate.NotBefore.Add(time.Hour), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
				t.Fatalf("%s does not chain to an embedded root: %v", entry.Name(), err)
			}
		} else if !bytes.Equal(certificate.RawSubject, certificate.RawIssuer) {
			t.Fatalf("%s is not self-signed", entry.Name())
		}
	}
	if len(policy.intermediates) != 5 || len(policy.rootNames) != 2 {
		t.Fatalf("policy has %d intermediates and %d roots", len(policy.intermediates), len(policy.rootNames))
	}
}

func TestXarChecksumHash(t *testing.T) {
	tests := []struct {
		algorithm uint32
		name      string
		want      string
		status    string
	}{
		{1, "", "sha1", ""},
		{3, "", "sha256", ""},
		{3, "sha512", "sha512", ""},
		{4, "", "sha512", ""},
		{0, "", "", VerificationInvalid},
		{2, "", "", VerificationUnsupported},
		{3, "whirlpool", "", VerificationUnsupported},
		{9, "", "", VerificationUnsupported},
	}
	for _, test := range tests {
		_, style, err := xarChecksumHash(test.algorithm, test.name)
		if test.status == "" {
			if err != nil || style != test.want {
				t.Fatalf("%d %q: style=%q err=%v", test.algorithm, test.name, style, err)
			}
			continue
		}
		if err == nil || failedVerification(err).Status != test.status {
			t.Fatalf("%d %q: err=%v", test.algorithm, test.name, err)
		}
	}
}

func TestVerifyIPARejectsDuplicateCodeResources(t *testing.T) {
	chain := artifactstest.NewTrustChain(t, validFrom, validUntil)
	signed := artifactstest.SignedMachO(t, sealedOptions(&chain))
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, member := range []struct {
		name string
		data []byte
	}{
		{"Payload/Demo.app/Info.plist", testInfoPlist},
		{"Payload/Demo.app/Demo", signed},
		{"Payload/Demo.app/_CodeSignature/CodeResources", testCodeResources},
		{"Payload/Demo.app/_CodeSignature/CodeResources", append(append([]byte(nil), testCodeResources...), ' ')},
		{"Payload/Demo.app/embedded.mobileprovision", []byte(`<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`)},
	} {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: member.name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(member.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	expectVerification(t, verifyIPA(t, buffer.Bytes(), testPolicy(chain)), VerificationInvalid, "duplicate CodeResources entries")
}

// xarMemberOffset returns the absolute file offset of the member at
// memberPath, whose parts are separated by "/".
func xarMemberOffset(t *testing.T, data []byte, memberPath string) int64 {
	t.Helper()
	document, heap, _, _ := readXarFiles(bytes.NewReader(data), int64(len(data)))
	if document == nil {
		t.Fatal("table of contents is unreadable")
	}
	files := document.Files
	parts := strings.Split(memberPath, "/")
	for index, part := range parts {
		fileType := "directory"
		if index == len(parts)-1 {
			fileType = "file"
		}
		member := xarChild(files, part, fileType)
		if member == nil {
			t.Fatalf("member %s is missing", memberPath)
		}
		if index == len(parts)-1 {
			return int64(len(data)) - heap.Size() + member.Data.Offset
		}
		files = member.Files
	}
	return 0
}

func TestVerifyPKGRecomputesFilePayloadChecksums(t *testing.T) {
	chain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, validFrom, validUntil)
	policy := testPolicy(chain)
	verify := func(data []byte) SignatureVerification {
		t.Helper()
		manifest, _ := inspectPKGVerifying(bytes.NewReader(data), int64(len(data)), policy)
		if manifest.SignatureVerification == nil {
			t.Fatal("verification result is missing")
		}
		return *manifest.SignatureVerification
	}
	files := map[string][]byte{
		"PackageInfo": []byte(`<pkg-info version="1.0" identifier="com.example.pkg"/>`),
		"Payload":     bytes.Repeat([]byte("payload bytes "), 64),
	}
	valid := artifactstest.SignedXar(t, files, chain, time.Now())
	expectVerification(t, verify(valid), VerificationValid, "2 file payload checksums recomputed")

	for _, style := range []string{"sha256", "sha512", "md5", "SHA1"} {
		pkg := artifactstest.SignedXarWithChecksumStyle(t, files, chain, time.Now(), style)
		expectVerification(t, verify(pkg), VerificationValid, "2 file payload checksums recomputed")
	}

	// Payload bytes changed after signing leave the signed table of contents
	// and its checksum intact.
	tampered := append([]byte(nil), valid...)
	tampered[xarMemberOffset(t, valid, "Payload")+100] ^= 0x01
	expectVerification(t, verify(tampered), VerificationInvalid, `file "Payload" does not match its archived sha1 checksum`)

	truncated := valid[:len(valid)-1]
	expectVerification(t, verify(truncated), VerificationInvalid, `file "Payload" data is outside the package`)

	unknown := artifactstest.SignedXarWithChecksumStyle(t, files, chain, time.Now(), "whirlpool")
	expectVerification(t, verify(unknown), VerificationUnsupported, `file "PackageInfo" archived checksum style "whirlpool" is not supported`)

	// A payload whose checksum style is not supported cannot be vouched for,
	// so tampering with it is not reported as valid.
	unknownTampered := append([]byte(nil), unknown...)
	unknownTampered[xarMemberOffset(t, unknown, "Payload")] ^= 0x01
	expectVerification(t, verify(unknownTampered), VerificationUnsupported, "is not supported")
}

func TestVerifyPKGRecomputesProductArchiveComponentChecksums(t *testing.T) {
	chain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, validFrom, validUntil)
	policy := testPolicy(chain)
	files := map[string][]byte{
		"Distribution":         []byte(productDistribution),
		"Demo.pkg/PackageInfo": []byte(productPackageInfo),
		"Demo.pkg/Payload":     demoPayload(t, productAppPlist(t)),
		"Demo.pkg/Bom":         []byte("bom"),
	}
	product := artifactstest.SignedXar(t, files, chain, time.Now())
	manifest, err := inspectPKGVerifying(bytes.NewReader(product), int64(len(product)), policy)
	if err != nil || manifest.Status != "readable" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	expectVerification(t, *manifest.SignatureVerification, VerificationValid, "4 file payload checksums recomputed")

	tampered := append([]byte(nil), product...)
	tampered[xarMemberOffset(t, product, "Demo.pkg/Bom")] ^= 0x01
	manifest, _ = inspectPKGVerifying(bytes.NewReader(tampered), int64(len(tampered)), policy)
	expectVerification(t, *manifest.SignatureVerification, VerificationInvalid, `file "Demo.pkg/Bom" does not match its archived sha1 checksum`)
}

func TestVerifyXarFileChecksumsRejectsMalformedEntries(t *testing.T) {
	heap := []byte("0123456789abcdef")
	digest := func(data []byte) string {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	member := func(name string, offset, length int64, checksum *xarFileChecksum) xarFile {
		return xarFile{Name: name, Type: "file", Data: xarData{XMLName: xml.Name{Local: "data"}, Offset: offset, Length: length, Size: length, ArchivedChecksum: checksum}}
	}
	sha := func(value string) *xarFileChecksum { return &xarFileChecksum{Style: "sha256", Value: value} }
	cases := []struct {
		name    string
		files   []xarFile
		status  string
		detail  string
		checked int
	}{
		{"valid", []xarFile{member("a", 0, 4, sha(digest(heap[:4]))), member("b", 4, 12, sha(" "+digest(heap[4:])+"\n"))}, VerificationValid, "", 2},
		{"shared range", []xarFile{member("a", 0, 16, sha(digest(heap))), member("b", 0, 16, sha(digest(heap)))}, VerificationValid, "", 2},
		{"no data", []xarFile{{Name: "dir", Type: "directory", Files: []xarFile{member("a", 0, 4, sha(digest(heap[:4])))}}}, VerificationValid, "", 1},
		{"missing checksum", []xarFile{member("a", 0, 4, nil)}, VerificationInvalid, `file "a" has no archived checksum`, 0},
		{"malformed checksum", []xarFile{member("a", 0, 4, sha("zz"))}, VerificationInvalid, `file "a" archived checksum is malformed`, 0},
		{"short checksum", []xarFile{member("a", 0, 4, sha("abcd"))}, VerificationInvalid, `file "a" archived checksum is malformed`, 0},
		{"negative offset", []xarFile{member("a", -1, 4, sha(digest(heap[:4])))}, VerificationInvalid, `file "a" data is outside the package`, 0},
		{"past heap", []xarFile{member("a", 10, 7, sha(digest(heap[10:])))}, VerificationInvalid, `file "a" data is outside the package`, 0},
		{"overlapping ranges", []xarFile{member("a", 0, 16, sha(digest(heap))), member("b", 1, 15, sha(digest(heap[1:])))}, VerificationInvalid, "overlap", 0},
		{"mismatch after unsupported style", []xarFile{member("a", 0, 4, &xarFileChecksum{Style: "whirlpool", Value: "00"}), member("b", 4, 4, sha(digest(heap[:4])))}, VerificationInvalid, `file "b" does not match its archived sha256 checksum`, 0},
		{"nested mismatch", []xarFile{{Name: "C.pkg", Type: "directory", Files: []xarFile{member("Payload", 0, 4, sha(digest(heap[1:5])))}}}, VerificationInvalid, `file "C.pkg/Payload" does not match`, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			source := append([]byte("header"), heap...)
			checked, err := verifyXarFileChecksums(bytes.NewReader(source), int64(len(source)), 6, test.files)
			if test.status == VerificationValid {
				if err != nil || checked != test.checked {
					t.Fatalf("checked=%d err=%v", checked, err)
				}
				return
			}
			expectVerification(t, failedVerification(err), test.status, test.detail)
		})
	}
}
