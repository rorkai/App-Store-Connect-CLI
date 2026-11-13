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
	privateKey, ok := privAny.(*rsa.PrivateKey)
	if !ok {
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
	csrPublicKey, ok := csr.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("expected RSA CSR public key, got %T", csr.PublicKey)
	}
	if privateKey.E != csrPublicKey.E || privateKey.N.Cmp(csrPublicKey.N) != 0 {
		t.Fatal("generated private key does not match CSR public key")
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

func TestCertificatesCSRGenerate_ValidatesLexicallyNestedOutputPaths(t *testing.T) {
	tests := []struct {
		name       string
		keyParts   []string
		csrParts   []string
		force      bool
		wantNested bool
	}{
		{
			name:       "key output contains CSR output",
			keyParts:   []string{"output"},
			csrParts:   []string{"output", "request.csr"},
			wantNested: true,
		},
		{
			name:       "CSR output contains key output with force",
			keyParts:   []string{"output", "private.key"},
			csrParts:   []string{"output"},
			force:      true,
			wantNested: true,
		},
		{
			name:     "sibling prefix remains valid",
			keyParts: []string{"output"},
			csrParts: []string{"output-extra", "request.csr"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			keyOut := filepath.Join(append([]string{dir}, test.keyParts...)...)
			csrOut := filepath.Join(append([]string{dir}, test.csrParts...)...)
			args := []string{
				"certificates", "csr", "generate",
				"--key-out", keyOut,
				"--csr-out", csrOut,
				"--output", "json",
			}
			if test.force {
				args = append(args, "--force")
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if test.wantNested {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Errorf("run error = %v, want flag.ErrHelp", runErr)
				}
				if runErr == nil || runErr.Error() != "--key-out and --csr-out must not be nested paths" {
					t.Errorf("run error = %v, want nested-path error", runErr)
				}
				if stdout != "" {
					t.Errorf("stdout = %q, want empty output", stdout)
				}
				for _, path := range []string{keyOut, csrOut} {
					if _, err := os.Lstat(path); err == nil {
						t.Errorf("output artifact %q was created", path)
					} else if !errors.Is(err, os.ErrNotExist) {
						t.Errorf("Lstat(%q) error = %v, want not exist", path, err)
					}
				}
				return
			}

			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stdout == "" {
				t.Fatal("expected generated output")
			}
			for _, path := range []string{keyOut, csrOut} {
				if info, err := os.Lstat(path); err != nil {
					t.Fatalf("Lstat(%q) error: %v", path, err)
				} else if !info.Mode().IsRegular() {
					t.Fatalf("output %q mode = %v, want regular file", path, info.Mode())
				}
			}
		})
	}
}

func TestCertificatesCSRGenerate_RejectsEquivalentSpellingsOfSameOutputPath(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	if err := os.WriteFile(keyOut, []byte("original-private-key"), 0o600); err != nil {
		t.Fatalf("WriteFile(keyOut) error: %v", err)
	}
	t.Chdir(dir)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "csr", "generate",
			"--key-out", "cert.key",
			"--csr-out", keyOut,
			"--force",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("run error = %v, want usage error", runErr)
	}
	if got, want := runErr.Error(), "--key-out and --csr-out must be different paths"; got != want {
		t.Fatalf("run error = %q, want %q", got, want)
	}
	if !strings.Contains(stderr, "--key-out and --csr-out must be different paths") {
		t.Fatalf("stderr = %q, want same-path usage error", stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output", stdout)
	}
	keyContents, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) error: %v", err)
	}
	if string(keyContents) != "original-private-key" {
		t.Errorf("key contents = %q, want original key", keyContents)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(dir) error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("unexpected output artifacts: %v", entries)
	}
}

func TestCertificatesCSRGenerate_ForcePreflightsCSRDestination(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string) string
		want  string
	}{
		{
			name: "parent is a file",
			setup: func(t *testing.T, dir string) string {
				parent := filepath.Join(dir, "blocked-parent")
				if err := os.WriteFile(parent, []byte("not-a-directory"), 0o600); err != nil {
					t.Fatalf("WriteFile(parent) error: %v", err)
				}
				return filepath.Join(parent, "cert.csr")
			},
			want: "not a directory",
		},
		{
			name: "destination is a directory",
			setup: func(t *testing.T, dir string) string {
				destination := filepath.Join(dir, "cert.csr")
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatalf("Mkdir(destination) error: %v", err)
				}
				return destination
			},
			want: "directory",
		},
		{
			name: "destination is a symlink",
			setup: func(t *testing.T, dir string) string {
				target := filepath.Join(dir, "target.csr")
				if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
					t.Fatalf("WriteFile(target) error: %v", err)
				}
				destination := filepath.Join(dir, "cert.csr")
				if err := os.Symlink(target, destination); err != nil {
					t.Skipf("symlink not supported: %v", err)
				}
				return destination
			},
			want: "symlink",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			keyOut := filepath.Join(dir, "cert.key")
			if err := os.WriteFile(keyOut, []byte("original-private-key"), 0o600); err != nil {
				t.Fatalf("WriteFile(keyOut) error: %v", err)
			}
			originalKeyInfo, err := os.Stat(keyOut)
			if err != nil {
				t.Fatalf("Stat(keyOut) error: %v", err)
			}
			csrOut := test.setup(t, dir)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
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
				t.Fatal("expected CSR destination failure")
			}
			if !strings.Contains(runErr.Error(), "write --csr-out") || !strings.Contains(strings.ToLower(runErr.Error()), test.want) {
				t.Fatalf("run error = %q, want CSR %q failure", runErr, test.want)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty output", stdout)
			}
			keyContents, err := os.ReadFile(keyOut)
			if err != nil {
				t.Fatalf("ReadFile(keyOut) error: %v", err)
			}
			if string(keyContents) != "original-private-key" {
				t.Errorf("key contents = %q, want original key", keyContents)
			}
			currentKeyInfo, err := os.Stat(keyOut)
			if err != nil {
				t.Fatalf("Stat(keyOut) after failure error: %v", err)
			}
			if !os.SameFile(originalKeyInfo, currentKeyInfo) {
				t.Error("key inode changed before deterministic CSR destination failure")
			}
		})
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
