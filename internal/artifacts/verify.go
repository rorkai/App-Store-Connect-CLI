package artifacts

import (
	"bytes"
	"crypto"
	"crypto/subtle"
	"crypto/x509"
	"embed"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
	"time"

	"go.mozilla.org/pkcs7"
)

// Signature verification results. They are reported only when verification is
// requested; otherwise receipts say not-verified.
const (
	VerificationValid          = "valid"
	VerificationInvalid        = "invalid"
	VerificationUntrustedChain = "untrusted-chain"
	VerificationExpired        = "expired"
	VerificationUnsupported    = "unsupported"
)

// SignatureVerification is the outcome of an offline signature check. Detail
// says what was checked, or why the check failed.
type SignatureVerification struct {
	Status string
	Detail string
}

// appleCertificates holds Apple's code-signing roots and intermediates as PEM
// files. Files named root-*.pem are trust anchors; intermediate-*.pem files
// only help build chains that signatures omit and grant no trust themselves.
//
//go:embed applecerts/*.pem
var appleCertificates embed.FS

// trustPolicy anchors chain evaluation. Tests replace it with throwaway roots.
type trustPolicy struct {
	roots         *x509.CertPool
	rootNames     map[string]string
	intermediates []*x509.Certificate
	now           func() time.Time
}

var appleTrustPolicy = sync.OnceValues(func() (*trustPolicy, error) {
	return loadTrustPolicy(appleCertificates, "applecerts")
})

func loadTrustPolicy(files fs.FS, dir string) (*trustPolicy, error) {
	entries, err := fs.ReadDir(files, dir)
	if err != nil {
		return nil, err
	}
	policy := &trustPolicy{roots: x509.NewCertPool(), rootNames: map[string]string{}, now: time.Now}
	for _, entry := range entries {
		data, err := fs.ReadFile(files, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
			return nil, fmt.Errorf("%s must hold exactly one PEM certificate", entry.Name())
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		switch {
		case strings.HasPrefix(entry.Name(), "root-"):
			policy.addRoot(certificate)
		case strings.HasPrefix(entry.Name(), "intermediate-"):
			policy.intermediates = append(policy.intermediates, withAppleExtensionsHandled(certificate))
		default:
			return nil, fmt.Errorf("%s is neither a root nor an intermediate", entry.Name())
		}
	}
	return policy, nil
}

// AppleIntermediateCertificate is one embedded Apple intermediate certificate.
// Name is its file name without the intermediate- prefix and .pem suffix.
type AppleIntermediateCertificate struct {
	Name        string
	Certificate *x509.Certificate
}

// AppleIntermediateCertificates returns the embedded Apple intermediate
// certificates. Nothing is downloaded.
func AppleIntermediateCertificates() ([]AppleIntermediateCertificate, error) {
	names, err := fs.Glob(appleCertificates, "applecerts/intermediate-*.pem")
	if err != nil {
		return nil, err
	}
	intermediates := make([]AppleIntermediateCertificate, 0, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(appleCertificates, name)
		if err != nil {
			return nil, err
		}
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("%s must hold a PEM certificate", name)
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		base := strings.TrimSuffix(strings.TrimPrefix(path.Base(name), "intermediate-"), ".pem")
		intermediates = append(intermediates, AppleIntermediateCertificate{Name: base, Certificate: certificate})
	}
	return intermediates, nil
}

func (policy *trustPolicy) addRoot(certificate *x509.Certificate) {
	policy.roots.AddCert(certificate)
	policy.rootNames[string(certificate.Raw)] = certificate.Subject.CommonName
}

// oidAppleExtensionArc prefixes Apple's certificate extensions. Apple marks
// several certificate-type markers critical; Go rejects any critical
// extension it does not process, so these are treated as handled here and
// the leaf's markers are evaluated by checkLeafType.
var oidAppleExtensionArc = asn1.ObjectIdentifier{1, 2, 840, 113635, 100}

func withAppleExtensionsHandled(certificate *x509.Certificate) *x509.Certificate {
	kept := certificate.UnhandledCriticalExtensions[:0]
	for _, oid := range certificate.UnhandledCriticalExtensions {
		if len(oid) > len(oidAppleExtensionArc) && oid[:len(oidAppleExtensionArc)].Equal(oidAppleExtensionArc) {
			continue
		}
		kept = append(kept, oid)
	}
	certificate.UnhandledCriticalExtensions = kept
	return certificate
}

// evaluateChain checks that leaf chains to a trusted root at the current time,
// using the policy's intermediates and any certificates the signature carries,
// and that the leaf's Apple certificate type may sign for purpose. Signing
// times inside a signature are asserted by the signer and could be backdated,
// so they are not used; trusted timestamps are not evaluated.
func (policy *trustPolicy) evaluateChain(leaf *x509.Certificate, carried []*x509.Certificate, purpose signingPurpose) SignatureVerification {
	intermediates := x509.NewCertPool()
	for _, certificate := range policy.intermediates {
		intermediates.AddCert(certificate)
	}
	for _, certificate := range carried {
		if certificate != leaf {
			intermediates.AddCert(withAppleExtensionsHandled(certificate))
		}
	}
	leaf = withAppleExtensionsHandled(leaf)
	verify := func(at time.Time) ([][]*x509.Certificate, error) {
		return leaf.Verify(x509.VerifyOptions{
			Roots:         policy.roots,
			Intermediates: intermediates,
			CurrentTime:   at,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		})
	}
	now := policy.now()
	at := now.UTC().Format(time.RFC3339)
	chains, err := verify(now)
	if err == nil {
		kind, typeErr := checkLeafType(leaf, purpose)
		if typeErr != nil {
			return SignatureVerification{Status: VerificationUntrustedChain, Detail: typeErr.Error()}
		}
		root := chains[0][len(chains[0])-1]
		return SignatureVerification{Status: VerificationValid, Detail: fmt.Sprintf("leaf is %s certificate; chain to %s verified at current time %s", withArticle(kind), policy.rootNames[string(root.Raw)], at)}
	}
	// A chain that verifies inside the leaf's validity window is trusted but
	// was used outside it. A leaf of the wrong type is untrusted either way.
	midpoint := leaf.NotBefore.Add(leaf.NotAfter.Sub(leaf.NotBefore) / 2)
	if _, midErr := verify(midpoint); midErr == nil {
		if _, typeErr := checkLeafType(leaf, purpose); typeErr != nil {
			return SignatureVerification{Status: VerificationUntrustedChain, Detail: typeErr.Error()}
		}
		return SignatureVerification{Status: VerificationExpired, Detail: fmt.Sprintf("certificate chain is not valid at current time %s (leaf valid %s to %s)", at, leaf.NotBefore.UTC().Format(time.RFC3339), leaf.NotAfter.UTC().Format(time.RFC3339))}
	} else if isExpiryError(err) {
		// Report why the chain is untrusted rather than the expiry that masked it.
		err = midErr
	}
	return SignatureVerification{Status: VerificationUntrustedChain, Detail: "signer does not chain to an embedded Apple root: " + err.Error()}
}

func isExpiryError(err error) bool {
	var invalid x509.CertificateInvalidError
	return errors.As(err, &invalid) && invalid.Reason == x509.Expired
}

// verifiedCMS is a CMS signer whose signature over detached content checked out.
type verifiedCMS struct {
	parsed *pkcs7.PKCS7
	leaf   *x509.Certificate
}

type cmsAttribute struct {
	Type  asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"set"`
}

// verifyDetachedCMS checks the single signer's signature over content. It does
// not evaluate the certificate chain or the signer's validity period.
func verifyDetachedCMS(data, content []byte) (result verifiedCMS, err error) {
	defer func() {
		// The CMS parser indexes untrusted BER.
		if recovered := recover(); recovered != nil {
			result, err = verifiedCMS{}, fmt.Errorf("CMS signature is malformed")
		}
	}()
	parsed, err := pkcs7.Parse(data)
	if err != nil {
		return verifiedCMS{}, fmt.Errorf("parse CMS signature: %w", err)
	}
	if len(parsed.Signers) != 1 {
		return verifiedCMS{}, fmt.Errorf("CMS signature has %d signers; expected 1", len(parsed.Signers))
	}
	leaf := parsed.GetOnlySigner()
	if leaf == nil {
		return verifiedCMS{}, fmt.Errorf("CMS signature does not carry its signer's certificate")
	}
	signer := parsed.Signers[0]
	digestHash, err := cmsDigestHash(signer.DigestAlgorithm.Algorithm)
	if err != nil {
		return verifiedCMS{}, err
	}
	signed := content
	result = verifiedCMS{parsed: parsed, leaf: leaf}
	if len(signer.AuthenticatedAttributes) > 0 {
		var digest []byte
		if err := parsed.UnmarshalSignedAttribute(pkcs7.OIDAttributeMessageDigest, &digest); err != nil {
			return verifiedCMS{}, fmt.Errorf("CMS signature has no message digest: %w", err)
		}
		hasher := digestHash.New()
		hasher.Write(content)
		if subtle.ConstantTimeCompare(digest, hasher.Sum(nil)) != 1 {
			return verifiedCMS{}, fmt.Errorf("CMS message digest does not match the signed content")
		}
		attributes := make([]cmsAttribute, 0, len(signer.AuthenticatedAttributes))
		for _, attribute := range signer.AuthenticatedAttributes {
			attributes = append(attributes, cmsAttribute{Type: attribute.Type, Value: attribute.Value})
		}
		encoded, err := asn1.Marshal(struct {
			A []cmsAttribute `asn1:"set"`
		}{A: attributes})
		if err != nil {
			return verifiedCMS{}, fmt.Errorf("encode CMS signed attributes: %w", err)
		}
		var outer asn1.RawValue
		if _, err := asn1.Unmarshal(encoded, &outer); err != nil {
			return verifiedCMS{}, fmt.Errorf("encode CMS signed attributes: %w", err)
		}
		signed = outer.Bytes
	}
	algorithm, err := cmsSignatureAlgorithm(signer.DigestEncryptionAlgorithm.Algorithm, digestHash)
	if err != nil {
		return verifiedCMS{}, err
	}
	if err := leaf.CheckSignature(algorithm, signed, signer.EncryptedDigest); err != nil {
		return verifiedCMS{}, fmt.Errorf("CMS signature does not verify: %w", err)
	}
	return result, nil
}

func cmsDigestHash(oid asn1.ObjectIdentifier) (crypto.Hash, error) {
	switch {
	case oid.Equal(pkcs7.OIDDigestAlgorithmSHA1):
		return crypto.SHA1, nil
	case oid.Equal(pkcs7.OIDDigestAlgorithmSHA256):
		return crypto.SHA256, nil
	case oid.Equal(pkcs7.OIDDigestAlgorithmSHA384):
		return crypto.SHA384, nil
	case oid.Equal(pkcs7.OIDDigestAlgorithmSHA512):
		return crypto.SHA512, nil
	}
	return 0, errUnsupportedf("CMS digest algorithm %s is not supported", oid)
}

var oidECPublicKey = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}

func cmsSignatureAlgorithm(oid asn1.ObjectIdentifier, digest crypto.Hash) (x509.SignatureAlgorithm, error) {
	rsa := map[crypto.Hash]x509.SignatureAlgorithm{crypto.SHA1: x509.SHA1WithRSA, crypto.SHA256: x509.SHA256WithRSA, crypto.SHA384: x509.SHA384WithRSA, crypto.SHA512: x509.SHA512WithRSA}
	ecdsa := map[crypto.Hash]x509.SignatureAlgorithm{crypto.SHA1: x509.ECDSAWithSHA1, crypto.SHA256: x509.ECDSAWithSHA256, crypto.SHA384: x509.ECDSAWithSHA384, crypto.SHA512: x509.ECDSAWithSHA512}
	switch {
	case oid.Equal(pkcs7.OIDEncryptionAlgorithmRSA), oid.Equal(pkcs7.OIDEncryptionAlgorithmRSASHA1),
		oid.Equal(pkcs7.OIDEncryptionAlgorithmRSASHA256), oid.Equal(pkcs7.OIDEncryptionAlgorithmRSASHA384),
		oid.Equal(pkcs7.OIDEncryptionAlgorithmRSASHA512):
		return rsa[digest], nil
	case oid.Equal(oidECPublicKey), oid.Equal(pkcs7.OIDDigestAlgorithmECDSASHA1),
		oid.Equal(pkcs7.OIDDigestAlgorithmECDSASHA256), oid.Equal(pkcs7.OIDDigestAlgorithmECDSASHA384),
		oid.Equal(pkcs7.OIDDigestAlgorithmECDSASHA512):
		return ecdsa[digest], nil
	}
	return 0, errUnsupportedf("CMS signature algorithm %s is not supported", oid)
}

// unsupportedError marks a signature that uses a format this verifier does not
// implement, as opposed to one that fails verification.
type unsupportedError struct{ message string }

func (err unsupportedError) Error() string { return err.message }

func errUnsupportedf(format string, args ...any) error {
	return unsupportedError{message: fmt.Sprintf(format, args...)}
}

// failedVerification maps an integrity error to invalid, or to unsupported when
// the signature uses a format the verifier does not implement.
func failedVerification(err error) SignatureVerification {
	var unsupported unsupportedError
	if errors.As(err, &unsupported) {
		return SignatureVerification{Status: VerificationUnsupported, Detail: err.Error()}
	}
	return SignatureVerification{Status: VerificationInvalid, Detail: err.Error()}
}
