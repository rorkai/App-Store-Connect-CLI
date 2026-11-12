package shared

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"fmt"
	"strings"

	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

// PKCS12Encoding selects the algorithms used to protect a written .p12 identity.
type PKCS12Encoding string

const (
	PKCS12EncodingModern PKCS12Encoding = "modern"
	PKCS12EncodingLegacy PKCS12Encoding = "legacy"
)

// PKCS12EncodingFlagUsage is the help text for --p12-encoding.
const PKCS12EncodingFlagUsage = "PKCS#12 encoding: modern (AES-256-CBC, HMAC-SHA-256) or legacy (3DES, SHA-1 MAC) for importers that reject modern files"

// ParsePKCS12Encoding validates a --p12-encoding value.
func ParsePKCS12Encoding(value string) (PKCS12Encoding, error) {
	switch encoding := PKCS12Encoding(strings.ToLower(strings.TrimSpace(value))); encoding {
	case PKCS12EncodingModern, PKCS12EncodingLegacy:
		return encoding, nil
	default:
		return "", fmt.Errorf("--p12-encoding must be one of: modern, legacy (got %q)", value)
	}
}

// EncodePKCS12Identity writes a password-protected PKCS#12 identity. The zero
// value selects the modern encoding.
func EncodePKCS12Identity(encoding PKCS12Encoding, privateKey any, certificate *x509.Certificate, password string) ([]byte, error) {
	var encoder *modernpkcs12.Encoder
	switch encoding {
	case "", PKCS12EncodingModern:
		encoder = modernpkcs12.Modern2023
	case PKCS12EncodingLegacy:
		encoder = modernpkcs12.LegacyDES
	default:
		return nil, fmt.Errorf("unsupported PKCS#12 encoding %q", encoding)
	}
	return encoder.WithRand(cryptorand.Reader).Encode(privateKey, certificate, nil, password)
}
