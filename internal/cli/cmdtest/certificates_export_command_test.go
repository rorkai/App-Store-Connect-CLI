package cmdtest

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	certificatescli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/certificates"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestCertificatesExport_JSONOutputAndPKCS12RoundTrip(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportCommandArtifacts(t)
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	csrPath := filepath.Join(dir, "push.csr")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, artifacts.privateKeyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(csrPath, artifacts.csrPEM, 0o644); err != nil {
		t.Fatalf("write CSR: %v", err)
	}
	const password = "command-password"
	if err := os.WriteFile(passwordPath, []byte(password+"\n"), 0o600); err != nil {
		t.Fatalf("write password: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "export",
			"--certificate", certificatePath,
			"--private-key", privateKeyPath,
			"--csr", csrPath,
			"--password-file", passwordPath,
			"--p12-out", p12Path,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "BEGIN") || strings.Contains(stdout, password) {
		t.Fatalf("stdout contains secret material: %q", stdout)
	}

	var result struct {
		Operation         string `json:"operation"`
		CertificatePath   string `json:"certificatePath"`
		PrivateKeyPath    string `json:"privateKeyPath"`
		CSRPath           string `json:"csrPath"`
		P12Out            string `json:"p12Out"`
		CertificateSHA256 string `json:"certificateSha256"`
		KeyType           string `json:"keyType"`
		KeySize           int    `json:"keySize"`
		PrivateKeyMatched bool   `json:"privateKeyMatched"`
		CSRMatched        bool   `json:"csrMatched"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.Operation != "certificates export" || result.CertificatePath != certificatePath || result.PrivateKeyPath != privateKeyPath || result.CSRPath != csrPath || result.P12Out != p12Path {
		t.Fatalf("unexpected path/result metadata: %#v", result)
	}
	if result.CertificateSHA256 == "" || result.KeyType != "RSA" || result.KeySize != 2048 || !result.PrivateKeyMatched || !result.CSRMatched {
		t.Fatalf("unexpected key/match metadata: %#v", result)
	}

	p12Data, err := os.ReadFile(p12Path)
	if err != nil {
		t.Fatalf("read p12: %v", err)
	}
	privateKey, certificate, err := modernpkcs12.Decode(p12Data, password)
	if err != nil {
		t.Fatalf("decode p12: %v", err)
	}
	if certificate == nil || !certificate.Equal(artifacts.certificate) {
		t.Fatal("decoded certificate does not match fixture")
	}
	if !commandPublicKeysEqual(privateKey, artifacts.certificate.PublicKey) {
		t.Fatal("decoded private key does not match fixture")
	}
	legacyBagOID, err := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(p12Data, legacyBagOID) {
		t.Fatal("default p12 uses the legacy encoding")
	}

	tableRoot := RootCommand("1.2.3")
	tableRoot.FlagSet.SetOutput(io.Discard)
	tableStdout, tableStderr := captureOutput(t, func() {
		if err := tableRoot.Parse([]string{
			"certificates", "export",
			"--certificate", certificatePath,
			"--private-key", privateKeyPath,
			"--password-file", passwordPath,
			"--p12-out", p12Path,
			"--force",
			"--confirm",
			"--output", "table",
		}); err != nil {
			t.Fatalf("table parse error: %v", err)
		}
		if err := tableRoot.Run(context.Background()); err != nil {
			t.Fatalf("table run error: %v", err)
		}
	})
	if tableStderr != "" {
		t.Fatalf("expected empty table stderr, got %q", tableStderr)
	}
	for _, expected := range []string{"field", "certificate_path", "p12_out", "certificate_sha256"} {
		if !strings.Contains(tableStdout, expected) {
			t.Fatalf("table output missing %q: %q", expected, tableStdout)
		}
	}
	if strings.Contains(tableStdout, password) || strings.Contains(tableStdout, "BEGIN") {
		t.Fatalf("table output contains secret material: %q", tableStdout)
	}

	markdownRoot := RootCommand("1.2.3")
	markdownRoot.FlagSet.SetOutput(io.Discard)
	markdownStdout, markdownStderr := captureOutput(t, func() {
		if err := markdownRoot.Parse([]string{
			"certificates", "export",
			"--certificate", certificatePath,
			"--private-key", privateKeyPath,
			"--password-file", passwordPath,
			"--p12-out", p12Path,
			"--force",
			"--confirm",
			"--p12-encoding", "legacy",
			"--output", "markdown",
		}); err != nil {
			t.Fatalf("markdown parse error: %v", err)
		}
		if err := markdownRoot.Run(context.Background()); err != nil {
			t.Fatalf("markdown run error: %v", err)
		}
	})
	if markdownStderr != "" {
		t.Fatalf("expected empty markdown stderr, got %q", markdownStderr)
	}
	for _, expected := range []string{"| field", "| value", "| certificate_path", "| p12_out"} {
		if !strings.Contains(markdownStdout, expected) {
			t.Fatalf("markdown output missing %q: %q", expected, markdownStdout)
		}
	}
	if strings.Contains(markdownStdout, password) || strings.Contains(markdownStdout, "BEGIN") {
		t.Fatalf("markdown output contains secret material: %q", markdownStdout)
	}
	legacyData, err := os.ReadFile(p12Path)
	if err != nil {
		t.Fatalf("read legacy p12: %v", err)
	}
	if _, _, err := modernpkcs12.Decode(legacyData, password); err != nil {
		t.Fatalf("decode legacy p12: %v", err)
	}
	if !bytes.Contains(legacyData, legacyBagOID) {
		t.Fatal("--p12-encoding legacy did not write a legacy-encoded p12")
	}
}

func TestCertificatesExport_RejectsStdoutDestinationAsUsageError(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "export",
			"--certificate", "certificate.cer",
			"--private-key", "private.key",
			"--password-file", "password",
			"--p12-out", "-",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("run error = %v, want usage error", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--p12-out must be a file path, not stdout") {
		t.Fatalf("stderr = %q, want stdout-destination error", stderr)
	}
}

func TestCertificatesExport_RejectsInvalidOutputBeforeWriting(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unsupported output",
			args: []string{"--output", "yaml"},
			want: `--output must be one of: json, table, markdown (got "yaml")`,
		},
		{
			name: "pretty with table",
			args: []string{"--output", "table", "--pretty"},
			want: "--pretty is only valid with JSON output",
		},
		{
			name: "unsupported p12 encoding",
			args: []string{"--p12-encoding", "rc2"},
			want: `--p12-encoding must be one of: modern, legacy (got "rc2")`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			artifacts := newCertificateExportCommandArtifacts(t)
			certificatePath := filepath.Join(dir, "push.cer")
			privateKeyPath := filepath.Join(dir, "push.key")
			passwordPath := filepath.Join(dir, "password")
			p12Path := filepath.Join(dir, "push.p12")
			if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
				t.Fatalf("write certificate: %v", err)
			}
			if err := os.WriteFile(privateKeyPath, artifacts.privateKeyPEM, 0o600); err != nil {
				t.Fatalf("write private key: %v", err)
			}
			if err := os.WriteFile(passwordPath, []byte("command-password\n"), 0o600); err != nil {
				t.Fatalf("write password: %v", err)
			}

			command := certificatescli.CertificatesExportCommand()
			command.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := command.FlagSet.Parse(append([]string{
					"--certificate", certificatePath,
					"--private-key", privateKeyPath,
					"--password-file", passwordPath,
					"--p12-out", p12Path,
				}, test.args...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := command.Exec(context.Background(), nil)
				if !isUsageClassError(err) {
					t.Fatalf("run error = %v, want usage error", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if _, err := os.Stat(p12Path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("p12 output exists after invalid output flags, stat error = %v", err)
			}
		})
	}
}

func TestCertificatesExport_ForceRequiresConfirmBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportCommandArtifacts(t)
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, artifacts.privateKeyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(passwordPath, []byte("command-password\n"), 0o600); err != nil {
		t.Fatalf("write password: %v", err)
	}

	command := certificatescli.CertificatesExportCommand()
	command.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := command.FlagSet.Parse([]string{
			"--certificate", certificatePath,
			"--private-key", privateKeyPath,
			"--password-file", passwordPath,
			"--p12-out", p12Path,
			"--force",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := command.Exec(context.Background(), nil)
		if !isUsageClassError(err) {
			t.Fatalf("run error = %v, want usage error", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required with --force") {
		t.Fatalf("stderr = %q, want confirmation error", stderr)
	}
	if _, err := os.Stat(p12Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("p12 output exists without confirmation, stat error = %v", err)
	}
}

func TestCertificatesExport_ConfirmRequiresForceBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportCommandArtifacts(t)
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, artifacts.privateKeyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(passwordPath, []byte("command-password\n"), 0o600); err != nil {
		t.Fatalf("write password: %v", err)
	}

	command := certificatescli.CertificatesExportCommand()
	command.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := command.FlagSet.Parse([]string{
			"--certificate", certificatePath,
			"--private-key", privateKeyPath,
			"--password-file", passwordPath,
			"--p12-out", p12Path,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := command.Exec(context.Background(), nil)
		if !isUsageClassError(err) {
			t.Fatalf("run error = %v, want usage error", err)
		}
		if got, want := err.Error(), "--confirm requires --force"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if got, want := stderr, "Error: --confirm requires --force\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if _, err := os.Stat(p12Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("p12 output exists without --force, stat error = %v", err)
	}
}

type certificateExportCommandArtifacts struct {
	certificate    *x509.Certificate
	certificateDER []byte
	privateKeyPEM  []byte
	csrPEM         []byte
}

func newCertificateExportCommandArtifacts(t *testing.T) certificateExportCommandArtifacts {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "Command Fixture"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
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
	return certificateExportCommandArtifacts{
		certificate:    certificate,
		certificateDER: certificateDER,
		privateKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		csrPEM:         pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}),
	}
}

func commandPublicKeysEqual(privateKey, publicKey any) bool {
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return false
	}
	privateDER, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return false
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return false
	}
	return bytes.Equal(privateDER, publicDER)
}

func TestCertificatesExport_RejectsExplicitlyEmptyCSR(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportCommandArtifacts(t)
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, artifacts.privateKeyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(passwordPath, []byte("command-password\n"), 0o600); err != nil {
		t.Fatalf("write password: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "export",
			"--certificate", certificatePath,
			"--private-key", privateKeyPath,
			"--csr", "",
			"--password-file", passwordPath,
			"--p12-out", p12Path,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
		if got, want := err.Error(), "--csr must not be empty"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --csr must not be empty") {
		t.Fatalf("expected empty-CSR usage diagnostic, got %q", stderr)
	}
	if _, err := os.Lstat(p12Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("p12 output was created despite the usage error, stat error = %v", err)
	}
}
