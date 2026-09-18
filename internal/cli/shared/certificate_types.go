package shared

import (
	"fmt"
	"os"
	"strings"
)

// certificateTypeValues mirrors the CertificateType enum in
// docs/openapi/latest.json. App Store Connect rejects any other value, so the
// CLI validates against this list before performing side effects such as
// generating a private key or issuing a create request.
var certificateTypeValues = []string{
	"APPLE_PAY",
	"APPLE_PAY_MERCHANT_IDENTITY",
	"APPLE_PAY_PSP_IDENTITY",
	"APPLE_PAY_RSA",
	"DEVELOPER_ID_APPLICATION",
	"DEVELOPER_ID_APPLICATION_G2",
	"DEVELOPER_ID_KEXT",
	"DEVELOPER_ID_KEXT_G2",
	"DEVELOPMENT",
	"DISTRIBUTION",
	"IDENTITY_ACCESS",
	"IOS_DEVELOPMENT",
	"IOS_DISTRIBUTION",
	"MAC_APP_DEVELOPMENT",
	"MAC_APP_DISTRIBUTION",
	"MAC_INSTALLER_DISTRIBUTION",
	"PASS_TYPE_ID",
	"PASS_TYPE_ID_WITH_NFC",
}

var certificateTypeSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(certificateTypeValues))
	for _, value := range certificateTypeValues {
		set[value] = struct{}{}
	}
	return set
}()

// CanonicalCertificateType normalizes value and returns the matching App Store
// Connect certificate type. Callers must use the returned value rather than the
// raw flag input: App Store Connect matches the enum exactly, so a normalized
// spelling such as "ios-distribution" has to reach the API as "IOS_DISTRIBUTION".
func CanonicalCertificateType(value string) (string, bool) {
	normalized := NormalizeEnumToken(value)
	if _, ok := certificateTypeSet[normalized]; !ok {
		return "", false
	}
	return normalized, true
}

// applePayCertificateTypePrefix marks the certificate types Apple issues against
// a merchant ID. Creating one requires the merchantId relationship in
// CertificateCreateRequest.
const applePayCertificateTypePrefix = "APPLE_PAY"

// IsApplePayCertificateType reports whether the canonical certificate type is one
// of the Apple Pay types.
func IsApplePayCertificateType(certificateType string) bool {
	return strings.HasPrefix(certificateType, applePayCertificateTypePrefix)
}

// ApplePayCertificateTypeList returns the certificate types that require a
// merchantId relationship.
func ApplePayCertificateTypeList() []string {
	values := make([]string, 0, 4)
	for _, value := range certificateTypeValues {
		if IsApplePayCertificateType(value) {
			values = append(values, value)
		}
	}
	return values
}

// CertificateCreateTypeList returns the certificate types asc certificates create
// can create. Use it for create help text and diagnostics so both discovery paths
// agree with what the command accepts.
func CertificateCreateTypeList() []string {
	values := make([]string, len(certificateTypeValues))
	copy(values, certificateTypeValues)
	return values
}

// ValidateCertificateCreateType returns the canonical certificate type for value,
// or a usage-class error when the value is not a CertificateType.
func ValidateCertificateCreateType(flagName, value string) (string, error) {
	canonical, ok := CanonicalCertificateType(value)
	if !ok {
		return "", UsageErrorf(
			"%s must be one of: %s (got %q)",
			flagName,
			strings.Join(CertificateCreateTypeList(), ", "),
			strings.TrimSpace(value),
		)
	}
	return canonical, nil
}

// ValidateCertificateCreateMerchantID enforces the merchantId relationship rules
// for POST /v1/certificates: the Apple Pay types require it, every other type
// must not send it. Callers must run this before side effects such as writing a
// private key or CSR.
func ValidateCertificateCreateMerchantID(certificateType, merchantID string) error {
	merchantID = strings.TrimSpace(merchantID)
	switch {
	case IsApplePayCertificateType(certificateType) && merchantID == "":
		fmt.Fprintf(os.Stderr, "Error: --merchant-id is required with --certificate-type %s\n", certificateType)
		return MissingRequiredUsageError("--merchant-id")
	case !IsApplePayCertificateType(certificateType) && merchantID != "":
		return UsageErrorf(
			"--merchant-id can only be used with --certificate-type %s",
			strings.Join(ApplePayCertificateTypeList(), ", "),
		)
	default:
		return nil
	}
}
