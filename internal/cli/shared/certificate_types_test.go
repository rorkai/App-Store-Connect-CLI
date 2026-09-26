package shared

import (
	"errors"
	"flag"
	"slices"
	"strings"
	"testing"
)

func TestCanonicalCertificateTypeNormalizesSeparatorsAndCase(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "canonical", value: "IOS_DISTRIBUTION", want: "IOS_DISTRIBUTION"},
		{name: "lowercase", value: "ios_distribution", want: "IOS_DISTRIBUTION"},
		{name: "hyphenated", value: "ios-distribution", want: "IOS_DISTRIBUTION"},
		{name: "spaced", value: " mac installer distribution ", want: "MAC_INSTALLER_DISTRIBUTION"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CanonicalCertificateType(tt.value)
			if !ok {
				t.Fatalf("CanonicalCertificateType(%q) reported an unsupported type", tt.value)
			}
			if got != tt.want {
				t.Fatalf("CanonicalCertificateType(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestCanonicalCertificateTypeRejectsUnknownValues(t *testing.T) {
	for _, value := range []string{"", "DEVELOPER_ID_INSTALLER", "TVOS_DISTRIBUTION", "ios distribution 2"} {
		if got, ok := CanonicalCertificateType(value); ok {
			t.Fatalf("CanonicalCertificateType(%q) = %q, want rejection", value, got)
		}
	}
}

func TestCertificateCreateTypeListCoversEveryCertificateType(t *testing.T) {
	values := CertificateCreateTypeList()
	if len(values) != len(certificateTypeValues) {
		t.Fatalf("expected every certificate type to be creatable, got %d of %d", len(values), len(certificateTypeValues))
	}
	for _, value := range []string{"APPLE_PAY", "APPLE_PAY_MERCHANT_IDENTITY", "APPLE_PAY_PSP_IDENTITY", "APPLE_PAY_RSA", "MAC_INSTALLER_DISTRIBUTION"} {
		if !slices.Contains(values, value) {
			t.Fatalf("CertificateCreateTypeList() dropped %q", value)
		}
	}
}

func TestValidateCertificateCreateTypeReturnsCanonicalValue(t *testing.T) {
	got, err := ValidateCertificateCreateType("--certificate-type", "developer-id-application-g2")
	if err != nil {
		t.Fatalf("ValidateCertificateCreateType() error: %v", err)
	}
	if got != "DEVELOPER_ID_APPLICATION_G2" {
		t.Fatalf("ValidateCertificateCreateType() = %q, want %q", got, "DEVELOPER_ID_APPLICATION_G2")
	}
}

func TestValidateCertificateCreateTypeAcceptsApplePayTypes(t *testing.T) {
	for _, value := range []string{"APPLE_PAY", "apple-pay-merchant-identity", "APPLE_PAY_PSP_IDENTITY", "APPLE_PAY_RSA"} {
		got, err := ValidateCertificateCreateType("--certificate-type", value)
		if err != nil {
			t.Fatalf("ValidateCertificateCreateType(%q) error: %v", value, err)
		}
		if !IsApplePayCertificateType(got) {
			t.Fatalf("expected %q to canonicalize to an Apple Pay type, got %q", value, got)
		}
	}
}

func TestValidateCertificateCreateTypeListsSupportedValuesForUnknownInput(t *testing.T) {
	_, err := ValidateCertificateCreateType("--certificate-type", "DEVELOPER_ID_INSTALLER")
	if err == nil {
		t.Fatal("expected an error for an unsupported certificate type")
	}
	if !strings.Contains(err.Error(), "MAC_INSTALLER_DISTRIBUTION") {
		t.Fatalf("expected the creatable types in the diagnostic, got %v", err)
	}
}

func TestValidateCertificateCreateMerchantIDRequiresMerchantForApplePay(t *testing.T) {
	for _, value := range []string{"APPLE_PAY", "APPLE_PAY_MERCHANT_IDENTITY", "APPLE_PAY_PSP_IDENTITY", "APPLE_PAY_RSA"} {
		err := ValidateCertificateCreateMerchantID(value, "")
		if err == nil {
			t.Fatalf("expected %q without a merchant ID to be rejected", value)
		}
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected a usage-class error for %q, got %v", value, err)
		}
	}
}

func TestValidateCertificateCreateMerchantIDRejectsMerchantOnOtherTypes(t *testing.T) {
	err := ValidateCertificateCreateMerchantID("IOS_DISTRIBUTION", "merchant-1")
	if err == nil {
		t.Fatal("expected --merchant-id to be rejected for IOS_DISTRIBUTION")
	}
	if !strings.Contains(err.Error(), "--merchant-id can only be used with") {
		t.Fatalf("expected the Apple Pay pairing diagnostic, got %v", err)
	}
	if !strings.Contains(err.Error(), "APPLE_PAY_MERCHANT_IDENTITY") {
		t.Fatalf("expected the Apple Pay types in the diagnostic, got %v", err)
	}
}

func TestValidateCertificateCreateMerchantIDAcceptsValidPairings(t *testing.T) {
	if err := ValidateCertificateCreateMerchantID("APPLE_PAY_RSA", " merchant-1 "); err != nil {
		t.Fatalf("expected APPLE_PAY_RSA with a merchant ID to be accepted, got %v", err)
	}
	if err := ValidateCertificateCreateMerchantID("IOS_DISTRIBUTION", ""); err != nil {
		t.Fatalf("expected IOS_DISTRIBUTION without a merchant ID to be accepted, got %v", err)
	}
}
