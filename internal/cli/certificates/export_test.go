package certificates

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

type certificateExportTestArtifacts struct {
	certificate    *x509.Certificate
	privateKey     *rsa.PrivateKey
	csr            []byte
	certificateDER []byte
	keyPEM         []byte
}

func TestRunCertificateExportCreatesPKCS12Identity(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	csrPath := filepath.Join(dir, "push.csr")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, csrPath, passwordPath, artifacts, []byte("correct-password\r\n"))

	result, err := runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		CSRPath:         csrPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err != nil {
		t.Fatalf("runCertificateExport() error = %v", err)
	}

	if result.Operation != "certificates export" {
		t.Fatalf("operation = %q, want certificates export", result.Operation)
	}
	if result.CertificateSHA256 != certificateExportCertificateSHA256(artifacts.certificate) {
		t.Fatalf("certificateSha256 = %q, want %q", result.CertificateSHA256, certificateExportCertificateSHA256(artifacts.certificate))
	}
	if result.KeyType != "RSA" || result.KeySize != 2048 {
		t.Fatalf("key metadata = %s/%d, want RSA/2048", result.KeyType, result.KeySize)
	}
	if !result.PrivateKeyMatched || result.CSRMatched == nil || !*result.CSRMatched {
		t.Fatalf("unexpected match metadata: %#v", result)
	}

	info, err := os.Stat(p12Path)
	if err != nil {
		t.Fatalf("stat p12: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("p12 permissions = %o, want 600", got)
	}
	p12Data, err := os.ReadFile(p12Path)
	if err != nil {
		t.Fatalf("read p12: %v", err)
	}
	privateKey, certificate, err := modernpkcs12.Decode(p12Data, "correct-password")
	if err != nil {
		t.Fatalf("decode p12: %v", err)
	}
	if certificate == nil || !certificate.Equal(artifacts.certificate) {
		t.Fatal("decoded certificate does not match source certificate")
	}
	if !certificateExportPublicKeysEqual(privateKey, artifacts.certificate.PublicKey) {
		t.Fatal("decoded private key does not match source certificate")
	}
	if _, _, err := modernpkcs12.Decode(p12Data, "wrong-password"); err == nil {
		t.Fatal("p12 decoded with the wrong password")
	}
}

func TestRunCertificateExportAcceptsPEMCertificateWithoutCSR(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.pem")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: artifacts.certificateDER})
	if err := os.WriteFile(certificatePath, certificatePEM, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, artifacts.keyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(passwordPath, []byte("password\n"), 0o600); err != nil {
		t.Fatalf("write password: %v", err)
	}

	result, err := runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err != nil {
		t.Fatalf("runCertificateExport() error = %v", err)
	}
	if result.CSRPath != "" || result.CSRMatched != nil {
		t.Fatalf("CSR metadata should be omitted: %#v", result)
	}
}

func TestRunCertificateExportPreservesWhitespaceInPaths(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, " certificate.cer ")
	privateKeyPath := filepath.Join(dir, " private.key ")
	passwordPath := filepath.Join(dir, " password ")
	p12Path := filepath.Join(dir, " identity.p12 ")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password\n"))

	result, err := runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err != nil {
		t.Fatalf("runCertificateExport() error = %v", err)
	}
	if result.CertificatePath != certificatePath || result.PrivateKeyPath != privateKeyPath || result.P12Out != p12Path {
		t.Fatalf("paths were not preserved: certificate=%q privateKey=%q p12=%q", result.CertificatePath, result.PrivateKeyPath, result.P12Out)
	}
	if _, err := os.Stat(p12Path); err != nil {
		t.Fatalf("stat whitespace-preserving output: %v", err)
	}
}

func TestRunCertificateExportRejectsInvalidInputsBeforeWriting(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*certificateExportTestArtifacts)
		password []byte
		want     string
	}{
		{
			name: "expired certificate",
			mutate: func(artifacts *certificateExportTestArtifacts) {
				artifacts.certificate.NotAfter = time.Now().Add(-time.Minute)
				var err error
				artifacts.certificateDER, err = x509.CreateCertificate(rand.Reader, artifacts.certificate, artifacts.certificate, &artifacts.privateKey.PublicKey, artifacts.privateKey)
				if err != nil {
					panic(err)
				}
			},
			password: []byte("password"),
			want:     "certificate is not currently valid",
		},
		{
			name: "mismatched private key",
			mutate: func(artifacts *certificateExportTestArtifacts) {
				other, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					panic(err)
				}
				der, err := x509.MarshalPKCS8PrivateKey(other)
				if err != nil {
					panic(err)
				}
				artifacts.keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
			},
			password: []byte("password"),
			want:     "private key does not match certificate",
		},
		{
			name:     "empty password",
			password: []byte("\r\n"),
			want:     "password file contains an empty password",
		},
		{
			name: "mismatched CSR",
			mutate: func(artifacts *certificateExportTestArtifacts) {
				other, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					panic(err)
				}
				requestDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "other"}}, other)
				if err != nil {
					panic(err)
				}
				artifacts.csr = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: requestDER})
			},
			password: []byte("password"),
			want:     "CSR public key does not match certificate and private key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
			if test.mutate != nil {
				test.mutate(artifacts)
			}
			certificatePath := filepath.Join(dir, "push.cer")
			privateKeyPath := filepath.Join(dir, "push.key")
			csrPath := filepath.Join(dir, "push.csr")
			passwordPath := filepath.Join(dir, "password")
			p12Path := filepath.Join(dir, "push.p12")
			writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, csrPath, passwordPath, artifacts, test.password)
			_, err := runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
				CertificatePath: certificatePath,
				PrivateKeyPath:  privateKeyPath,
				CSRPath:         csrPath,
				PasswordPath:    passwordPath,
				P12Out:          p12Path,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runCertificateExport() error = %v, want substring %q", err, test.want)
			}
			if _, statErr := os.Stat(p12Path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("p12 output exists after rejected input, stat error = %v", statErr)
			}
		})
	}
}

func TestRunCertificateExportRefusesExistingDestinationAndPreservesIt(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))
	original := []byte("old identity")
	if err := os.WriteFile(p12Path, original, 0o600); err != nil {
		t.Fatalf("write existing p12: %v", err)
	}

	_, err := runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("runCertificateExport() error = %v, want existing-file error", err)
	}
	got, readErr := os.ReadFile(p12Path)
	if readErr != nil {
		t.Fatalf("read existing p12: %v", readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("existing p12 changed: %q", got)
	}

	if _, err := runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
		Force:           true,
	}); err != nil {
		t.Fatalf("force export error = %v", err)
	}
	if got, err := os.ReadFile(p12Path); err != nil {
		t.Fatalf("read replaced p12: %v", err)
	} else if bytes.Equal(got, original) {
		t.Fatal("force export did not replace existing p12")
	}
}

func TestParseCertificateExportObjectRejectsMultipleObjects(t *testing.T) {
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	multiple := append(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: artifacts.certificateDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: artifacts.certificateDER})...,
	)
	if _, err := parseCertificateExportCertificate(multiple); err == nil || !strings.Contains(err.Error(), "exactly one object") {
		t.Fatalf("parseCertificateExportCertificate() error = %v, want multiple-object error", err)
	}
}

func TestParseCertificateExportObjectRejectsNonWhitespacePEMPrefix(t *testing.T) {
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: artifacts.certificateDER})
	withPrefix := append([]byte("unexpected prefix\n"), certificatePEM...)

	if _, err := parseCertificateExportCertificate(withPrefix); err == nil || !strings.Contains(err.Error(), "exactly one object") {
		t.Fatalf("parseCertificateExportCertificate() error = %v, want non-whitespace-prefix error", err)
	}
}

func TestParseCertificateExportPrivateKeyRejectsUnsupportedPEMType(t *testing.T) {
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	keyDER, err := x509.MarshalPKCS8PrivateKey(artifacts.privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}

	if _, err := parseCertificateExportPrivateKey(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: keyDER})); err == nil || !strings.Contains(err.Error(), "unsupported PEM block type") {
		t.Fatalf("parseCertificateExportPrivateKey() error = %v, want unsupported-type error", err)
	}
}

func TestRunCertificateExportRejectsCACertificate(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	artifacts.certificate.IsCA = true
	artifacts.certificate.KeyUsage |= x509.KeyUsageCertSign
	certificateDER, err := x509.CreateCertificate(rand.Reader, artifacts.certificate, artifacts.certificate, &artifacts.privateKey.PublicKey, artifacts.privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error: %v", err)
	}
	artifacts.certificateDER = certificateDER
	artifacts.certificate, err = x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("ParseCertificate() error: %v", err)
	}

	certificatePath := filepath.Join(dir, "ca.cer")
	privateKeyPath := filepath.Join(dir, "ca.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "ca.p12")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	_, err = runCertificateExport(context.Background(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "certificate must be a leaf certificate") {
		t.Fatalf("runCertificateExport() error = %v, want CA-certificate error", err)
	}
	if _, statErr := os.Stat(p12Path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("p12 output exists after CA certificate rejection, stat error = %v", statErr)
	}
}

func TestTrimCertificateExportPasswordRemovesOnlyOneLineEnding(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " secret \n", want: " secret "},
		{input: "secret\r\n", want: "secret"},
		{input: "secret\n\n", want: "secret\n"},
		{input: " secret ", want: " secret "},
	}
	for _, test := range tests {
		if got := string(trimCertificateExportPassword([]byte(test.input))); got != test.want {
			t.Errorf("trimCertificateExportPassword(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func newCertificateExportTestArtifacts(t *testing.T, notBefore, notAfter time.Time) *certificateExportTestArtifacts {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Synthetic Push Certificate"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error: %v", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("ParseCertificate() error: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: template.Subject}, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificateRequest() error: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	return &certificateExportTestArtifacts{
		certificate:    certificate,
		privateKey:     privateKey,
		csr:            pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}),
		certificateDER: certificateDER,
		keyPEM:         pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}
}

func writeCertificateExportTestFiles(t *testing.T, certificatePath, privateKeyPath, csrPath, passwordPath string, artifacts *certificateExportTestArtifacts, password []byte) {
	t.Helper()
	if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, artifacts.keyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if csrPath != "" {
		if err := os.WriteFile(csrPath, artifacts.csr, 0o644); err != nil {
			t.Fatalf("write CSR: %v", err)
		}
	}
	if err := os.WriteFile(passwordPath, password, 0o600); err != nil {
		t.Fatalf("write password: %v", err)
	}
}
