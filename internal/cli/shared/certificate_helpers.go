package shared

import (
	"fmt"
	"strings"
)

// CertificateFieldsList returns supported certificate fields.
func CertificateFieldsList() []string {
	return []string{
		"name",
		"certificateType",
		"displayName",
		"serialNumber",
		"platform",
		"expirationDate",
		"certificateContent",
		"activated",
		"passTypeId",
	}
}

// CertificateIncludeList returns supported certificate includes.
func CertificateIncludeList() []string {
	return []string{"passTypeId"}
}

// CertificateSortValues returns supported certificate sort values.
func CertificateSortValues() []string {
	return []string{
		"displayName",
		"-displayName",
		"certificateType",
		"-certificateType",
		"serialNumber",
		"-serialNumber",
		"id",
		"-id",
	}
}

// NormalizeSelection validates CSV selections against allowed values.
func NormalizeSelection(value, flagName string, allowed []string) ([]string, error) {
	values := splitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	allowedSet := map[string]struct{}{}
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
	}
	for _, item := range values {
		if _, ok := allowedSet[item]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
		}
	}

	return values, nil
}

// NormalizeCertificateFields validates certificate field selections.
func NormalizeCertificateFields(value, flagName string) ([]string, error) {
	return NormalizeSelection(value, flagName, CertificateFieldsList())
}

// NormalizeCertificateInclude validates certificate include selections.
func NormalizeCertificateInclude(value, flagName string) ([]string, error) {
	return NormalizeSelection(value, flagName, CertificateIncludeList())
}
