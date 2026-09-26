package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DEVELOPER_ID_INSTALLER is not part of Apple's CertificateType enum, so the
// create command must reject it locally instead of writing a private key and
// posting an invalid type to App Store Connect.
func TestCertificatesCreate_RejectsUnknownCertificateType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	dir := t.TempDir()
	keyOut := filepath.Join(dir, "installer.key")
	csrOut := filepath.Join(dir, "installer.csr")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "create",
			"--certificate-type", "DEVELOPER_ID_INSTALLER",
			"--generate-csr",
			"--key-out", keyOut,
			"--csr-out", csrOut,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp for an unknown certificate type, got %v", err)
		}
		if !isUsageClassError(err) {
			t.Fatalf("expected usage-class error, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--certificate-type must be one of:") {
		t.Fatalf("expected allowed certificate type list on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "MAC_INSTALLER_DISTRIBUTION") {
		t.Fatalf("expected supported types on stderr, got %q", stderr)
	}
	if _, err := os.Stat(keyOut); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private key should not be generated, stat error: %v", err)
	}
	if _, err := os.Stat(csrOut); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CSR should not be generated, stat error: %v", err)
	}
}

func TestCertificatesCreate_AcceptsSupportedCertificateTypeCasing(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "create",
			"--certificate-type", "mac_installer_distribution",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp for the missing CSR, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "--certificate-type must be one of:") {
		t.Fatalf("supported certificate type should not be rejected, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error: --csr is required") {
		t.Fatalf("expected missing CSR error, got %q", stderr)
	}
}

// The four APPLE_PAY types require a merchantId relationship, so the command
// must reject them without --merchant-id before --generate-csr writes key
// material to disk.
func TestCertificatesCreate_RejectsApplePayTypesWithoutMerchantIDBeforeGeneratingFiles(t *testing.T) {
	for _, certificateType := range []string{
		"APPLE_PAY",
		"APPLE_PAY_MERCHANT_IDENTITY",
		"APPLE_PAY_PSP_IDENTITY",
		"APPLE_PAY_RSA",
	} {
		t.Run(certificateType, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			dir := t.TempDir()
			keyOut := filepath.Join(dir, "applepay.key")
			csrOut := filepath.Join(dir, "applepay.csr")

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"certificates", "create",
					"--certificate-type", certificateType,
					"--generate-csr",
					"--key-out", keyOut,
					"--csr-out", csrOut,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp for an Apple Pay certificate type, got %v", err)
				}
				if !isUsageClassError(err) {
					t.Fatalf("expected usage-class error, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --merchant-id is required with --certificate-type "+certificateType) {
				t.Fatalf("expected the missing merchant ID error on stderr, got %q", stderr)
			}
			if _, err := os.Stat(keyOut); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("private key should not be generated, stat error: %v", err)
			}
			if _, err := os.Stat(csrOut); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("CSR should not be generated, stat error: %v", err)
			}
		})
	}
}

// Non-Apple-Pay types must keep working: the guard is create-specific and must
// not narrow the certificate types the command already supports.
func TestCertificatesCreate_ApplePayGuardLeavesOtherTypesAlone(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"certificates", "create",
			"--certificate-type", "DEVELOPER_ID_APPLICATION_G2",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp for the missing CSR, got %v", err)
		}
	})

	if strings.Contains(stderr, "Error: --merchant-id") {
		t.Fatalf("Apple Pay guard should not reject DEVELOPER_ID_APPLICATION_G2, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error: --csr is required") {
		t.Fatalf("expected missing CSR error, got %q", stderr)
	}
}
