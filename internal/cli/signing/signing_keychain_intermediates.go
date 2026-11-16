package signing

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// selectSigningKeychainIntermediates returns the receipt entries for embedded
// intermediates that issued leaf and the unexpired ones to import. Only the
// issuer is imported: codesign needs no other Apple intermediate for leaf.
func selectSigningKeychainIntermediates(stderr io.Writer, leaf *x509.Certificate, embedded []artifacts.AppleIntermediateCertificate, now time.Time) ([]asc.SigningKeychainIntermediate, []artifacts.AppleIntermediateCertificate) {
	var receipts []asc.SigningKeychainIntermediate
	var pending []artifacts.AppleIntermediateCertificate
	for _, candidate := range embedded {
		certificate := candidate.Certificate
		if !bytes.Equal(certificate.RawSubject, leaf.RawIssuer) ||
			(len(leaf.AuthorityKeyId) > 0 && len(certificate.SubjectKeyId) > 0 && !bytes.Equal(leaf.AuthorityKeyId, certificate.SubjectKeyId)) {
			continue
		}
		receipt := asc.SigningKeychainIntermediate{Name: candidate.Name, SHA256: signingCertificateSHA256(certificate), Status: "added"}
		if now.After(certificate.NotAfter) {
			receipt.Status = "skipped-expired"
			fmt.Fprintf(stderr, "Warning: skipping expired Apple intermediate certificate %s (expired %s)\n", candidate.Name, certificate.NotAfter.UTC().Format(time.RFC3339))
		} else {
			pending = append(pending, candidate)
		}
		receipts = append(receipts, receipt)
	}
	if len(receipts) == 0 {
		fmt.Fprintf(stderr, "Warning: no embedded Apple intermediate certificate matches the identity issuer %q\n", leaf.Issuer.CommonName)
	}
	return receipts, pending
}
