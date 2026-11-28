package artifacts

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"slices"
	"strings"
)

// signingPurpose is what an artifact's signature must be issued for.
type signingPurpose int

const (
	purposeCodeSigning signingPurpose = iota
	purposeInstaller
)

func (purpose signingPurpose) String() string {
	if purpose == purposeInstaller {
		return "signing installer packages"
	}
	return "code signing"
}

func appleOID(arc ...int) asn1.ObjectIdentifier {
	return append(asn1.ObjectIdentifier{1, 2, 840, 113635, 100}, arc...)
}

// leafType is an Apple leaf certificate type, identified by the Apple
// extensions it carries. A leaf of the type must also carry its extended key
// usage: id-kp-codeSigning for code, or an Apple usage for installers.
//
// Each marker and usage below was read from real certificates on a macOS host
// (keychain and provisioning-profile certificates, signed apps, IPAs, and
// packages) and, where Apple publishes it, from Apple's "Allowing and denying
// apps and binaries" device-management documentation. docs/API_NOTES.md lists
// the evidence per type.
type leafType struct {
	name     string
	markers  []asn1.ObjectIdentifier
	purpose  signingPurpose
	appleEKU asn1.ObjectIdentifier // nil means id-kp-codeSigning
}

// leafTypes is ordered most specific first: a leaf takes the first type whose
// markers it carries all of.
var leafTypes = []leafType{
	// Developer ID Application: 1.2.840.113635.100.6.1.13, EKU codeSigning.
	{name: "Developer ID Application", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 13)}, purpose: purposeCodeSigning},
	// Developer ID Installer: 1.2.840.113635.100.6.1.14, EKU 1.2.840.113635.100.4.13.
	{name: "Developer ID Installer", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 14)}, purpose: purposeInstaller, appleEKU: appleOID(4, 13)},
	// Mac Installer Distribution ("3rd Party Mac Developer Installer"):
	// 1.2.840.113635.100.6.1.8, EKU 1.2.840.113635.100.4.9.
	{name: "Mac Installer Distribution", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 8)}, purpose: purposeInstaller, appleEKU: appleOID(4, 9)},
	// Apple's own package signer ("Software Update", Xcode packages):
	// 1.2.840.113635.100.6.1.29.2, EKU 1.2.840.113635.100.4.1.
	{name: "Apple Software Update signing", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 29, 2)}, purpose: purposeInstaller, appleEKU: appleOID(4, 1)},
	// Mac App Store re-signing ("Apple Mac OS Application Signing"):
	// 1.2.840.113635.100.6.1.9, EKU codeSigning.
	{name: "Mac App Store application signing", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 9)}, purpose: purposeCodeSigning},
	// Apple platform binaries ("macOS Software Signing"): 1.2.840.113635.100.6.22, EKU codeSigning.
	{name: "Apple software signing", markers: []asn1.ObjectIdentifier{appleOID(6, 22)}, purpose: purposeCodeSigning},
	// Apple Distribution carries both the iOS (6.1.4) and Mac (6.1.7) markers.
	{name: "Apple Distribution", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 4), appleOID(6, 1, 7)}, purpose: purposeCodeSigning},
	// Apple Development carries both the iOS (6.1.2) and Mac (6.1.12) markers.
	{name: "Apple Development", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 2), appleOID(6, 1, 12)}, purpose: purposeCodeSigning},
	// Legacy single-platform types.
	{name: "iPhone Distribution", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 4)}, purpose: purposeCodeSigning},
	{name: "iPhone Developer", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 2)}, purpose: purposeCodeSigning},
	{name: "Mac App Distribution", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 7)}, purpose: purposeCodeSigning},
	{name: "Mac Development", markers: []asn1.ObjectIdentifier{appleOID(6, 1, 12)}, purpose: purposeCodeSigning},
}

func (kind leafType) matches(leaf *x509.Certificate) bool {
	for _, marker := range kind.markers {
		if !slices.ContainsFunc(leaf.Extensions, func(extension pkix.Extension) bool { return extension.Id.Equal(marker) }) {
			return false
		}
	}
	return true
}

func (kind leafType) hasUsage(leaf *x509.Certificate) bool {
	if kind.appleEKU == nil {
		return slices.Contains(leaf.ExtKeyUsage, x509.ExtKeyUsageCodeSigning)
	}
	return slices.ContainsFunc(leaf.UnknownExtKeyUsage, kind.appleEKU.Equal)
}

// checkLeafType reports why leaf may not sign for purpose, or returns its
// type name when it may.
func checkLeafType(leaf *x509.Certificate, purpose signingPurpose) (string, error) {
	var matched *leafType
	purposes := map[signingPurpose]bool{}
	for index := range leafTypes {
		if leafTypes[index].matches(leaf) {
			purposes[leafTypes[index].purpose] = true
			if matched == nil {
				matched = &leafTypes[index]
			}
		}
	}
	switch {
	case matched == nil:
		return "", fmt.Errorf("leaf is not a recognized Apple certificate type; not valid for %s", purpose)
	case len(purposes) > 1:
		return "", fmt.Errorf("leaf carries markers for both code signing and installer packages; not valid for %s", purpose)
	case matched.purpose != purpose:
		return "", fmt.Errorf("leaf is %s certificate; not valid for %s", withArticle(matched.name), purpose)
	case !matched.hasUsage(leaf):
		return "", fmt.Errorf("leaf is %s certificate without its extended key usage; not valid for %s", withArticle(matched.name), purpose)
	}
	return matched.name, nil
}

// InstallerCertificateType checks certificate markers and key usage only.
// It does not verify the issuer chain, expiration, or revocation.
func InstallerCertificateType(leaf *x509.Certificate) (string, error) {
	if leaf == nil || leaf.IsCA || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return "", fmt.Errorf("installer identity requires a leaf certificate permitting digital signatures")
	}
	return checkLeafType(leaf, purposeInstaller)
}

func withArticle(name string) string {
	if name != "" && strings.ContainsRune("AEIOUaeiou", rune(name[0])) {
		return "an " + name
	}
	return "a " + name
}
