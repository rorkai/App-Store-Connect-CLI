package certificates

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type csrGenerateSubject struct {
	CommonName         string `json:"commonName"`
	Email              string `json:"email,omitempty"`
	Organization       string `json:"organization,omitempty"`
	OrganizationalUnit string `json:"organizationalUnit,omitempty"`
	Country            string `json:"country,omitempty"`
}

type csrGenerateResult struct {
	KeyOut  string             `json:"keyOut"`
	CSROut  string             `json:"csrOut"`
	KeyType string             `json:"keyType"`
	KeySize int                `json:"keySize"`
	Subject csrGenerateSubject `json:"subject"`
}

type csrGenerateOptions struct {
	KeyOut             string
	CSROut             string
	CommonName         string
	Email              string
	Organization       string
	OrganizationalUnit string
	Country            string
	KeyType            string
	KeySize            int
	Force              bool
}

// CertificatesCSRCommand returns the certificates csr command group.
func CertificatesCSRCommand() *ffcli.Command {
	fs := flag.NewFlagSet("csr", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "csr",
		ShortUsage: "asc certificates csr <subcommand> [flags]",
		ShortHelp:  "Generate certificate signing requests (CSR).",
		LongHelp: `Generate certificate signing requests (CSR).

Examples:
  asc certificates csr generate --key-out "./signing/cert.key" --csr-out "./signing/cert.csr"
  asc certificates csr generate --common-name "ASC Signing" --key-out "./signing/cert.key" --csr-out "./signing/cert.csr"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CertificatesCSRGenerateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CertificatesCSRGenerateCommand returns the certificates csr generate subcommand.
func CertificatesCSRGenerateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	keyOut := fs.String("key-out", "", "Private key output path (PEM)")
	csrOut := fs.String("csr-out", "", "CSR output path (PEM)")
	commonName := fs.String("common-name", "asc", "Subject Common Name (CN)")
	email := fs.String("email", "", "Subject email address")
	organization := fs.String("organization", "", "Subject organization (O)")
	orgUnit := fs.String("organizational-unit", "", "Subject organizational unit (OU)")
	country := fs.String("country", "", "Subject country (C)")
	keyType := fs.String("key-type", "rsa", "Key type: rsa")
	keySize := fs.Int("key-size", 2048, "RSA key size in bits (e.g., 2048)")
	force := fs.Bool("force", false, "Overwrite existing output files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "asc certificates csr generate --key-out \"./signing/cert.key\" --csr-out \"./signing/cert.csr\"",
		ShortHelp:  "Generate a private key and CSR.",
		LongHelp: `Generate a private key and certificate signing request (CSR).

This command is non-interactive and does not print key material to stdout/stderr.

Examples:
  asc certificates csr generate --key-out "./signing/cert.key" --csr-out "./signing/cert.csr"
  asc certificates csr generate --common-name "ASC Signing" --key-out "./signing/cert.key" --csr-out "./signing/cert.csr"
  asc certificates csr generate --key-out "./signing/cert.key" --csr-out "./signing/cert.csr" --force`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			keyOutValue := strings.TrimSpace(*keyOut)
			if keyOutValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --key-out is required")
				return flag.ErrHelp
			}
			csrOutValue := strings.TrimSpace(*csrOut)
			if csrOutValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --csr-out is required")
				return flag.ErrHelp
			}
			if filepath.Clean(keyOutValue) == filepath.Clean(csrOutValue) {
				return shared.UsageError("--key-out and --csr-out must be different paths")
			}

			result, _, err := generateCSRFiles(csrGenerateOptions{
				KeyOut:             keyOutValue,
				CSROut:             csrOutValue,
				CommonName:         *commonName,
				Email:              *email,
				Organization:       *organization,
				OrganizationalUnit: *orgUnit,
				Country:            *country,
				KeyType:            *keyType,
				KeySize:            *keySize,
				Force:              *force,
			})
			if err != nil {
				return fmt.Errorf("certificates csr generate: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderCSRGenerateResult(result, false) },
				func() error { return renderCSRGenerateResult(result, true) },
			)
		},
	}
}

func generateCSRFiles(opts csrGenerateOptions) (*csrGenerateResult, []byte, error) {
	keyOutValue := strings.TrimSpace(opts.KeyOut)
	if keyOutValue == "" {
		return nil, nil, fmt.Errorf("--key-out is required")
	}
	csrOutValue := strings.TrimSpace(opts.CSROut)
	if csrOutValue == "" {
		return nil, nil, fmt.Errorf("--csr-out is required")
	}
	if filepath.Clean(keyOutValue) == filepath.Clean(csrOutValue) {
		return nil, nil, shared.UsageError("--key-out and --csr-out must be different paths")
	}

	normalizedKeyType := strings.ToLower(strings.TrimSpace(opts.KeyType))
	if normalizedKeyType == "" {
		normalizedKeyType = "rsa"
	}
	if normalizedKeyType != "rsa" {
		return nil, nil, shared.UsageError("--key-type must be one of: rsa")
	}
	if opts.KeySize < 2048 {
		return nil, nil, shared.UsageError("--key-size must be at least 2048")
	}

	subject := csrGenerateSubject{
		CommonName:         strings.TrimSpace(opts.CommonName),
		Email:              strings.TrimSpace(opts.Email),
		Organization:       strings.TrimSpace(opts.Organization),
		OrganizationalUnit: strings.TrimSpace(opts.OrganizationalUnit),
		Country:            strings.TrimSpace(opts.Country),
	}
	if subject.CommonName == "" {
		subject.CommonName = "asc"
	}

	// Pre-check output paths to avoid leaving an orphaned key when CSR write fails.
	// This is not atomic (TOCTOU), but it prevents the common confusing case.
	if !opts.Force {
		if _, err := os.Lstat(keyOutValue); err == nil {
			return nil, nil, fmt.Errorf("write --key-out: output file already exists: %w", os.ErrExist)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("write --key-out: %w", err)
		}
		if _, err := os.Lstat(csrOutValue); err == nil {
			return nil, nil, fmt.Errorf("write --csr-out: output file already exists: %w", os.ErrExist)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("write --csr-out: %w", err)
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, opts.KeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if keyPEM == nil {
		return nil, nil, fmt.Errorf("encode key PEM failed")
	}

	req := &x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject: pkix.Name{
			CommonName: subject.CommonName,
		},
	}
	if subject.Organization != "" {
		req.Subject.Organization = []string{subject.Organization}
	}
	if subject.OrganizationalUnit != "" {
		req.Subject.OrganizationalUnit = []string{subject.OrganizationalUnit}
	}
	if subject.Country != "" {
		req.Subject.Country = []string{subject.Country}
	}
	if subject.Email != "" {
		req.EmailAddresses = []string{subject.Email}
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, req, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create csr: %w", err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if csrPEM == nil {
		return nil, nil, fmt.Errorf("encode csr PEM failed")
	}

	// Write key first: if anything fails, do not leave a CSR without its key.
	if err := writeFileBytesNoSymlink(keyOutValue, keyPEM, 0o600, opts.Force); err != nil {
		return nil, nil, fmt.Errorf("write --key-out: %w", err)
	}
	if err := writeFileBytesNoSymlink(csrOutValue, csrPEM, 0o644, opts.Force); err != nil {
		return nil, nil, fmt.Errorf("write --csr-out: %w", err)
	}

	result := &csrGenerateResult{
		KeyOut:  keyOutValue,
		CSROut:  csrOutValue,
		KeyType: normalizedKeyType,
		KeySize: opts.KeySize,
		Subject: subject,
	}
	return result, csrPEM, nil
}

func renderCSRGenerateResult(result *csrGenerateResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Key Out", "CSR Out", "Key Type", "Key Size"},
		[][]string{{
			result.KeyOut,
			result.CSROut,
			result.KeyType,
			fmt.Sprintf("%d", result.KeySize),
		}},
	)
	render(
		[]string{"Common Name", "Email", "Organization", "Org Unit", "Country"},
		[][]string{{
			result.Subject.CommonName,
			result.Subject.Email,
			result.Subject.Organization,
			result.Subject.OrganizationalUnit,
			result.Subject.Country,
		}},
	)
	return nil
}

func writeFileBytesNoSymlink(path string, data []byte, perm os.FileMode, force bool) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("output path is required")
	}
	if strings.HasSuffix(trimmed, string(filepath.Separator)) {
		return fmt.Errorf("output path must be a file")
	}

	_, err := shared.SafeWriteFileNoSymlink(
		trimmed,
		perm,
		force,
		".asc-csr-*",
		".asc-csr-backup-*",
		func(f *os.File) (int64, error) {
			n, err := f.Write(data)
			return int64(n), err
		},
	)
	return err
}
