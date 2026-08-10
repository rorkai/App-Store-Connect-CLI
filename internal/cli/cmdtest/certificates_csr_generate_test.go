package cmdtest

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCertificatesCSRGenerate_MissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"certificates", "csr", "generate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --key-out is required") {
		t.Fatalf("expected missing key-out error, got %q", stderr)
	}
}

func TestCertificatesCSRGenerate_GeneratesKeyAndCSR(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	csrOut := filepath.Join(dir, "cert.csr")

	type subject struct {
		CommonName         string `json:"commonName"`
		Email              string `json:"email"`
		Organization       string `json:"organization"`
		OrganizationalUnit string `json:"organizationalUnit"`
		Country            string `json:"country"`
	}
	type result struct {
		KeyOut  string  `json:"keyOut"`
		CSROut  string  `json:"csrOut"`
		KeyType string  `json:"keyType"`
		KeySize int     `json:"keySize"`
		Subject subject `json:"subject"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "csr", "generate",
			"--key-out", keyOut,
			"--csr-out", csrOut,
			"--common-name", "ASC Signing",
			"--email", "ci@example.com",
			"--organization", "Example Co",
			"--organizational-unit", "Dev",
			"--country", "US",
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
	if strings.Contains(stdout, "BEGIN") {
		t.Fatalf("stdout must not contain PEM material, got %q", stdout)
	}

	var got result
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if filepath.Clean(got.KeyOut) != filepath.Clean(keyOut) {
		t.Fatalf("expected keyOut=%q, got %q", keyOut, got.KeyOut)
	}
	if filepath.Clean(got.CSROut) != filepath.Clean(csrOut) {
		t.Fatalf("expected csrOut=%q, got %q", csrOut, got.CSROut)
	}
	if got.KeyType != "rsa" {
		t.Fatalf("expected keyType=rsa, got %q", got.KeyType)
	}
	if got.KeySize != 2048 {
		t.Fatalf("expected keySize=2048, got %d", got.KeySize)
	}
	if got.Subject.CommonName != "ASC Signing" {
		t.Fatalf("expected commonName ASC Signing, got %q", got.Subject.CommonName)
	}
	if got.Subject.Email != "ci@example.com" {
		t.Fatalf("expected email ci@example.com, got %q", got.Subject.Email)
	}
	if got.Subject.Organization != "Example Co" {
		t.Fatalf("expected organization Example Co, got %q", got.Subject.Organization)
	}
	if got.Subject.OrganizationalUnit != "Dev" {
		t.Fatalf("expected organizationalUnit Dev, got %q", got.Subject.OrganizationalUnit)
	}
	if got.Subject.Country != "US" {
		t.Fatalf("expected country US, got %q", got.Subject.Country)
	}

	keyPEM, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) error: %v", err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		t.Fatalf("failed to decode private key PEM")
		return
	}
	privAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey() error: %v", err)
	}
	if _, ok := privAny.(*rsa.PrivateKey); !ok {
		t.Fatalf("expected RSA private key, got %T", privAny)
	}

	csrPEM, err := os.ReadFile(csrOut)
	if err != nil {
		t.Fatalf("ReadFile(csrOut) error: %v", err)
	}
	csrBlock, _ := pem.Decode(csrPEM)
	if csrBlock == nil {
		t.Fatalf("failed to decode CSR PEM")
		return
	}
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificateRequest() error: %v", err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatalf("CSR signature invalid: %v", err)
	}
	if csr.Subject.CommonName != "ASC Signing" {
		t.Fatalf("expected CSR CN ASC Signing, got %q", csr.Subject.CommonName)
	}
}

func TestCertificatesCSRGenerate_RefusesOverwriteWithoutForce(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	csrOut := filepath.Join(dir, "cert.csr")

	if err := os.WriteFile(keyOut, []byte("OLD-KEY"), 0o600); err != nil {
		t.Fatalf("WriteFile(keyOut) error: %v", err)
	}
	if err := os.WriteFile(csrOut, []byte("OLD-CSR"), 0o600); err != nil {
		t.Fatalf("WriteFile(csrOut) error: %v", err)
	}

	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "csr", "generate",
			"--key-out", keyOut,
			"--csr-out", csrOut,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(runErr.Error()), "exists") {
		t.Fatalf("expected exists error, got %v", runErr)
	}

	keyData, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) error: %v", err)
	}
	if string(keyData) != "OLD-KEY" {
		t.Fatalf("expected key file unchanged, got %q", string(keyData))
	}

	csrData, err := os.ReadFile(csrOut)
	if err != nil {
		t.Fatalf("ReadFile(csrOut) error: %v", err)
	}
	if string(csrData) != "OLD-CSR" {
		t.Fatalf("expected csr file unchanged, got %q", string(csrData))
	}
}

func TestCertificatesCSRGenerate_DoesNotOrphanKeyWhenCSROutExistsWithoutForce(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	csrOut := filepath.Join(dir, "cert.csr")

	if err := os.WriteFile(csrOut, []byte("OLD-CSR"), 0o600); err != nil {
		t.Fatalf("WriteFile(csrOut) error: %v", err)
	}

	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "csr", "generate",
			"--key-out", keyOut,
			"--csr-out", csrOut,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(runErr.Error()), "exists") {
		t.Fatalf("expected exists error, got %v", runErr)
	}

	if _, err := os.Stat(keyOut); err == nil {
		t.Fatalf("expected key file to not be created when csr-out exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(keyOut) unexpected error: %v", err)
	}

	csrData, err := os.ReadFile(csrOut)
	if err != nil {
		t.Fatalf("ReadFile(csrOut) error: %v", err)
	}
	if string(csrData) != "OLD-CSR" {
		t.Fatalf("expected csr file unchanged, got %q", string(csrData))
	}
}

func TestCertificatesCSRGenerate_ForcePreservesExistingKeyWhenCSROutCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	writeECDSAPEM(t, keyOut)
	originalKey, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) error: %v", err)
	}

	blockedParent := filepath.Join(dir, "blocked-parent")
	if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(blockedParent) error: %v", err)
	}
	failedCSROut := filepath.Join(blockedParent, "cert.csr")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "csr", "generate",
			"--key-out", keyOut,
			"--csr-out", failedCSROut,
			"--force",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse force command: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected CSR destination failure")
	}
	if !strings.Contains(runErr.Error(), blockedParent) {
		t.Fatalf("expected error to identify blocked CSR parent %q, got %v", blockedParent, runErr)
	}
	currentKey, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) after failure: %v", err)
	}
	if string(currentKey) != string(originalKey) {
		t.Fatal("existing private key changed before the CSR destination failure")
	}
	if _, err := os.Lstat(failedCSROut); err == nil {
		t.Fatalf("unexpected CSR output at %q", failedCSROut)
	}
}

func TestCertificatesCSRGenerate_RejectsNestedOutputPathsBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		keyOut func(string) string
		csrOut func(string) string
	}{
		{
			name:   "key contains csr",
			keyOut: func(dir string) string { return filepath.Join(dir, "pair") },
			csrOut: func(dir string) string { return filepath.Join(dir, "pair", "cert.csr") },
		},
		{
			name:   "csr contains key",
			keyOut: func(dir string) string { return filepath.Join(dir, "pair", "cert.key") },
			csrOut: func(dir string) string { return filepath.Join(dir, "pair") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			keyOut := test.keyOut(dir)
			csrOut := test.csrOut(dir)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			_, _ = captureOutput(t, func() {
				if err := root.Parse([]string{
					"certificates", "csr", "generate",
					"--key-out", keyOut,
					"--csr-out", csrOut,
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse command: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected nested output path error")
			}
			if _, err := os.Lstat(keyOut); err == nil {
				t.Errorf("key output was created before nested paths were rejected: %q", keyOut)
			}
			if _, err := os.Lstat(csrOut); err == nil {
				t.Errorf("CSR output was created before nested paths were rejected: %q", csrOut)
			}
			if !strings.Contains(runErr.Error(), "must not contain one another") {
				t.Errorf("expected nested output path error, got %v", runErr)
			}
		})
	}
}

func TestCertificatesCSRGenerate_RejectsAliasEquivalentOutputsBeforeWriting(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "without force"
		if force {
			name = "with force"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			realDir := filepath.Join(dir, "real")
			if err := os.Mkdir(realDir, 0o755); err != nil {
				t.Fatalf("Mkdir(realDir) error: %v", err)
			}
			aliasDir := filepath.Join(dir, "alias")
			if err := os.Symlink(realDir, aliasDir); err != nil {
				t.Fatalf("Symlink() error: %v", err)
			}

			keyOut := filepath.Join(realDir, "pair")
			csrOut := filepath.Join(aliasDir, "pair")
			args := []string{
				"certificates", "csr", "generate",
				"--key-out", keyOut,
				"--csr-out", csrOut,
				"--output", "json",
			}
			if force {
				args = append(args, "--force")
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse command: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Error("expected alias-equivalent output path error")
			} else {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Errorf("expected usage error, got %v", runErr)
				}
				const expected = "--key-out and --csr-out must be different paths"
				if runErr.Error() != expected {
					t.Errorf("error = %q, want %q", runErr, expected)
				}
			}
			if stdout != "" {
				t.Errorf("expected empty stdout, got %q", stdout)
			}
			for _, output := range []string{keyOut, csrOut} {
				if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("output was created before alias-equivalent paths were rejected: %q (stat error: %v)", output, err)
				}
			}
		})
	}
}

func TestCertificatesCSRGenerate_RejectsCaseEquivalentOutputsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "case-probe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatalf("Mkdir(probe) error: %v", err)
	}
	probeInfo, err := os.Stat(probe)
	if err != nil {
		t.Fatalf("Stat(probe) error: %v", err)
	}
	caseVariantInfo, err := os.Stat(filepath.Join(dir, "CASE-PROBE"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("temporary volume is demonstrably case-sensitive")
	}
	if err != nil {
		t.Fatalf("Stat(case variant) error: %v", err)
	}
	if !os.SameFile(probeInfo, caseVariantInfo) {
		t.Skip("temporary volume resolves case variants to distinct entries")
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("Remove(probe) error: %v", err)
	}

	for _, force := range []bool{false, true} {
		name := "without force"
		if force {
			name = "with force"
		}
		t.Run(name, func(t *testing.T) {
			outputDir := filepath.Join(dir, strings.ReplaceAll(name, " ", "-"))
			if err := os.Mkdir(outputDir, 0o755); err != nil {
				t.Fatalf("Mkdir(outputDir) error: %v", err)
			}
			keyOut := filepath.Join(outputDir, "cert.key")
			csrOut := filepath.Join(outputDir, "CERT.KEY")
			args := []string{
				"certificates", "csr", "generate",
				"--key-out", keyOut,
				"--csr-out", csrOut,
				"--output", "json",
			}
			if force {
				args = append(args, "--force")
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse command: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Errorf("expected usage error, got %v", runErr)
			}
			const expected = "--key-out and --csr-out must be different paths"
			if runErr == nil || runErr.Error() != expected {
				t.Errorf("error = %v, want %q", runErr, expected)
			}
			if stdout != "" {
				t.Errorf("expected empty stdout, got %q", stdout)
			}
			entries, err := os.ReadDir(outputDir)
			if err != nil {
				t.Fatalf("ReadDir(outputDir) error: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("outputs were created before case-equivalent paths were rejected: %v", entries)
			}
		})
	}
}

func TestCertificatesCSRGenerate_RejectsCaseVariantNestedOutputsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "case-probe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatalf("Mkdir(probe) error: %v", err)
	}
	probeInfo, err := os.Stat(probe)
	if err != nil {
		t.Fatalf("Stat(probe) error: %v", err)
	}
	caseVariantInfo, err := os.Stat(filepath.Join(dir, "CASE-PROBE"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("temporary volume is demonstrably case-sensitive")
	}
	if err != nil {
		t.Fatalf("Stat(case variant) error: %v", err)
	}
	if !os.SameFile(probeInfo, caseVariantInfo) {
		t.Skip("temporary volume resolves case variants to distinct entries")
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("Remove(probe) error: %v", err)
	}

	for _, relation := range []struct {
		name   string
		keyOut func(string) string
		csrOut func(string) string
	}{
		{
			name: "key is case-variant ancestor",
			keyOut: func(outputDir string) string {
				return filepath.Join(outputDir, "pair")
			},
			csrOut: func(outputDir string) string {
				return filepath.Join(outputDir, "PAIR", "cert.csr")
			},
		},
		{
			name: "CSR is case-variant ancestor",
			keyOut: func(outputDir string) string {
				return filepath.Join(outputDir, "PAIR", "cert.key")
			},
			csrOut: func(outputDir string) string {
				return filepath.Join(outputDir, "pair")
			},
		},
	} {
		t.Run(relation.name, func(t *testing.T) {
			for _, force := range []bool{false, true} {
				name := "without force"
				if force {
					name = "with force"
				}
				t.Run(name, func(t *testing.T) {
					outputDir := filepath.Join(dir, strings.ReplaceAll(relation.name+"-"+name, " ", "-"))
					if err := os.Mkdir(outputDir, 0o755); err != nil {
						t.Fatalf("Mkdir(outputDir) error: %v", err)
					}
					keyOut := relation.keyOut(outputDir)
					csrOut := relation.csrOut(outputDir)
					args := []string{
						"certificates", "csr", "generate",
						"--key-out", keyOut,
						"--csr-out", csrOut,
						"--output", "json",
					}
					if force {
						args = append(args, "--force")
					}

					root := RootCommand("1.2.3")
					root.FlagSet.SetOutput(io.Discard)
					var runErr error
					stdout, _ := captureOutput(t, func() {
						if err := root.Parse(args); err != nil {
							t.Fatalf("parse command: %v", err)
						}
						runErr = root.Run(context.Background())
					})

					if !errors.Is(runErr, flag.ErrHelp) {
						t.Errorf("expected usage error, got %v", runErr)
					}
					const expected = "--key-out and --csr-out must not contain one another"
					if runErr == nil || runErr.Error() != expected {
						t.Errorf("error = %v, want %q", runErr, expected)
					}
					if stdout != "" {
						t.Errorf("expected empty stdout, got %q", stdout)
					}
					entries, err := os.ReadDir(outputDir)
					if err != nil {
						t.Fatalf("ReadDir(outputDir) error: %v", err)
					}
					if len(entries) != 0 {
						t.Errorf("outputs were created before case-variant nested paths were rejected: %v", entries)
					}
				})
			}
		})
	}
}

func TestCertificatesCSRGenerate_AllowsCaseDistinctOutputsOnCaseSensitiveVolume(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "case-probe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatalf("Mkdir(probe) error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "CASE-PROBE")); err == nil {
		t.Skip("temporary volume is case-insensitive")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(case variant) error: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("Remove(probe) error: %v", err)
	}

	keyOut := filepath.Join(dir, "cert.key")
	csrOut := filepath.Join(dir, "CERT.KEY")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"certificates", "csr", "generate",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse command: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if _, err := os.Stat(keyOut); err != nil {
		t.Fatalf("expected case-distinct key output: %v", err)
	}
	if _, err := os.Stat(csrOut); err != nil {
		t.Fatalf("expected case-distinct CSR output: %v", err)
	}
}

func TestCertificatesCSRGenerate_AllowsCaseVariantNonNestedOutputsOnCaseSensitiveVolume(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "case-probe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatalf("Mkdir(probe) error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "CASE-PROBE")); err == nil {
		t.Skip("temporary volume is case-insensitive")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(case variant) error: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("Remove(probe) error: %v", err)
	}

	keyOut := filepath.Join(dir, "pair")
	csrOut := filepath.Join(dir, "PAIR", "cert.csr")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"certificates", "csr", "generate",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse command: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if _, err := os.Stat(keyOut); err != nil {
		t.Fatalf("expected key output: %v", err)
	}
	if _, err := os.Stat(csrOut); err != nil {
		t.Fatalf("expected case-distinct nested-looking CSR output: %v", err)
	}
}

func TestCertificatesCSRGenerate_ForcePreservesExistingKeyWhenCSRParentIsNotWritable(t *testing.T) {
	dir := t.TempDir()
	keyDir := filepath.Join(dir, "key")
	csrDir := filepath.Join(dir, "csr")
	if err := os.Mkdir(keyDir, 0o755); err != nil {
		t.Fatalf("Mkdir(keyDir) error: %v", err)
	}
	if err := os.Mkdir(csrDir, 0o755); err != nil {
		t.Fatalf("Mkdir(csrDir) error: %v", err)
	}
	if err := os.Chmod(csrDir, 0o555); err != nil {
		t.Fatalf("Chmod(csrDir) error: %v", err)
	}
	defer func() {
		if err := os.Chmod(csrDir, 0o755); err != nil {
			t.Errorf("restore csrDir permissions: %v", err)
		}
	}()
	permissionProbe, err := os.CreateTemp(csrDir, "permission-probe-")
	if err == nil {
		probePath := permissionProbe.Name()
		_ = permissionProbe.Close()
		_ = os.Remove(probePath)
		t.Skip("temporary volume does not enforce directory write permissions")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("CreateTemp permission probe error = %v, want permission error", err)
	}

	keyOut := filepath.Join(keyDir, "existing.key")
	csrOut := filepath.Join(csrDir, "request.csr")
	originalKey := []byte("existing-private-key")
	if err := os.WriteFile(keyOut, originalKey, 0o600); err != nil {
		t.Fatalf("WriteFile(keyOut) error: %v", err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"certificates", "csr", "generate",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--force",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse command: %v", err)
	}

	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected unwritable CSR parent error")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	keyContents, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) error: %v", err)
	}
	if string(keyContents) != string(originalKey) {
		t.Errorf("existing key was replaced before CSR parent failure: got %q", keyContents)
	}
	entries, err := os.ReadDir(csrDir)
	if err != nil {
		t.Fatalf("ReadDir(csrDir) error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("CSR parent contains preflight artifacts: %v", entries)
	}
}

func TestCertificatesCSRGenerate_AllowsDistinctOutputsBelowSymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir(realDir) error: %v", err)
	}
	aliasDir := filepath.Join(dir, "alias")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	keyOut := filepath.Join(aliasDir, "cert.key")
	csrOut := filepath.Join(aliasDir, "cert.csr")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"certificates", "csr", "generate",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse command: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if _, err := os.Stat(keyOut); err != nil {
		t.Fatalf("expected key output below symlinked directory: %v", err)
	}
	if _, err := os.Stat(csrOut); err != nil {
		t.Fatalf("expected CSR output below symlinked directory: %v", err)
	}
}

func TestCertificatesCSRGenerate_RefusesSymlinkOutputs(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	dir := t.TempDir()
	target := filepath.Join(dir, "target.key")
	if err := os.WriteFile(target, []byte("TARGET"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error: %v", err)
	}

	keyOut := filepath.Join(dir, "cert.key")
	if err := os.Symlink(target, keyOut); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}
	csrOut := filepath.Join(dir, "cert.csr")

	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "csr", "generate",
			"--key-out", keyOut,
			"--csr-out", csrOut,
			"--force",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(runErr.Error()), "symlink") {
		t.Fatalf("expected symlink error, got %v", runErr)
	}

	targetData, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(target) error: %v", err)
	}
	if string(targetData) != "TARGET" {
		t.Fatalf("expected symlink target unchanged, got %q", string(targetData))
	}
	if _, err := os.Stat(csrOut); err == nil {
		t.Fatalf("expected csr file to not be created when key-out is a symlink")
	}
}
