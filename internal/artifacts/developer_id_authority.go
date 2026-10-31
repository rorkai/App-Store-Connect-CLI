package artifacts

import (
	"crypto/x509/pkix"
	"slices"
	"time"
)

// Both Developer ID Sub-CAs use the same common name. Only the organizational
// unit in a certificate's issuer name tells them apart.
const (
	DeveloperIDAuthorityCommonName   = "Developer ID Certification Authority"
	DeveloperIDPreviousAuthorityUnit = "Apple Certification Authority"
	DeveloperIDCurrentAuthorityUnit  = "G2"
	DeveloperIDReplacementAuthority  = DeveloperIDAuthorityCommonName + " (G2)"
)

// Developer ID authority statuses reported for a certificate.
const (
	DeveloperIDAffected    = "affected"
	DeveloperIDNotAffected = "not-affected"
)

// DeveloperIDPreviousAuthorityExpiry is the date Apple gives for certificates
// from the previous Sub-CA to stop working.
var DeveloperIDPreviousAuthorityExpiry = time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC)

// DeveloperIDAuthorityStatus classifies a certificate by its issuer name. It is
// empty for an issuer that is not a Developer ID Sub-CA this package knows.
func DeveloperIDAuthorityStatus(issuer pkix.Name) string {
	if issuer.CommonName != DeveloperIDAuthorityCommonName {
		return ""
	}
	switch {
	case slices.Contains(issuer.OrganizationalUnit, DeveloperIDPreviousAuthorityUnit):
		return DeveloperIDAffected
	case slices.Contains(issuer.OrganizationalUnit, DeveloperIDCurrentAuthorityUnit):
		return DeveloperIDNotAffected
	default:
		return ""
	}
}
