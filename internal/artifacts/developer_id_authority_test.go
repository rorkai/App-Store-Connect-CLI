package artifacts

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
)

func TestDeveloperIDAuthorityStatusMatchesAppleIntermediates(t *testing.T) {
	for _, test := range []struct {
		file string
		want string
	}{
		{file: "applecerts/intermediate-developer-id.pem", want: DeveloperIDAffected},
		{file: "applecerts/intermediate-developer-id-g2.pem", want: DeveloperIDNotAffected},
		{file: "applecerts/intermediate-apple-wwdr-g6.pem", want: ""},
	} {
		t.Run(test.file, func(t *testing.T) {
			data, err := appleCertificates.ReadFile(test.file)
			if err != nil {
				t.Fatal(err)
			}
			block, _ := pem.Decode(data)
			if block == nil {
				t.Fatal("no PEM block")
			}
			authority, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Fatal(err)
			}
			// A leaf's issuer name is its authority's subject name.
			if got := DeveloperIDAuthorityStatus(authority.Subject); got != test.want {
				t.Fatalf("DeveloperIDAuthorityStatus(%v) = %q, want %q", authority.Subject, got, test.want)
			}
		})
	}
}

func TestDeveloperIDAuthorityStatusRequiresTheAuthorityName(t *testing.T) {
	issuer := pkix.Name{CommonName: "Example Certification Authority", OrganizationalUnit: []string{DeveloperIDPreviousAuthorityUnit}}
	if got := DeveloperIDAuthorityStatus(issuer); got != "" {
		t.Fatalf("DeveloperIDAuthorityStatus(%v) = %q, want empty", issuer, got)
	}
}
