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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
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
				return shared.MissingRequiredUsageError()
			}
			csrOutValue := strings.TrimSpace(*csrOut)
			if csrOutValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --csr-out is required")
				return shared.MissingRequiredUsageError()
			}
			if err := validateCSRPairOutputPaths(keyOutValue, csrOutValue); err != nil {
				return err
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
	if err := validateCSRPairOutputPaths(keyOutValue, csrOutValue); err != nil {
		return nil, nil, err
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

	// Preflight both outputs before generating or replacing either file. This is
	// not a cross-file transaction, but locally knowable path failures must not
	// split the generated key and CSR pair.
	for _, output := range []struct {
		flag string
		path string
	}{
		{flag: "--key-out", path: keyOutValue},
		{flag: "--csr-out", path: csrOutValue},
	} {
		if err := preflightCSRFileWrite(output.path, opts.Force); err != nil {
			return nil, nil, fmt.Errorf("write %s: %w", output.flag, err)
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

func preflightCSRFileWrite(path string, force bool) error {
	trimmed, err := validateCSRFileOutputPath(path)
	if err != nil {
		return err
	}
	existingParent, err := preflightCSRParentDirectory(trimmed)
	if err != nil {
		return err
	}

	info, err := os.Lstat(trimmed)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if !force {
			return fmt.Errorf("output file already exists: %w", os.ErrExist)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink %q", trimmed)
		}
		if info.IsDir() {
			return fmt.Errorf("output path %q is a directory", trimmed)
		}
	}
	return probeCSRParentWrite(existingParent, force)
}

func validateCSRPairOutputPaths(keyOut, csrOut string) error {
	keyPath, err := resolveCSRDestinationPath(keyOut)
	if err != nil {
		return fmt.Errorf("resolve --key-out: %w", err)
	}
	csrPath, err := resolveCSRDestinationPath(csrOut)
	if err != nil {
		return fmt.Errorf("resolve --csr-out: %w", err)
	}

	keyToCSR, keyRelativeErr := filepath.Rel(keyPath, csrPath)
	csrToKey, csrRelativeErr := filepath.Rel(csrPath, keyPath)
	if (keyRelativeErr == nil && keyToCSR == ".") || (csrRelativeErr == nil && csrToKey == ".") {
		return shared.UsageError("--key-out and --csr-out must be different paths")
	}
	caseEquivalent, caseNested, err := classifyCSRCaseFoldRelation(keyPath, csrPath)
	if err != nil {
		return fmt.Errorf("compare --key-out and --csr-out: %w", err)
	}
	if caseEquivalent {
		return shared.UsageError("--key-out and --csr-out must be different paths")
	}
	if caseNested || isCSRDescendantPath(keyToCSR, keyRelativeErr) || isCSRDescendantPath(csrToKey, csrRelativeErr) {
		return shared.UsageError("--key-out and --csr-out must not contain one another")
	}
	return nil
}

func classifyCSRCaseFoldRelation(keyPath, csrPath string) (same bool, nested bool, err error) {
	keyDepth := csrPathDepth(keyPath)
	csrDepth := csrPathDepth(csrPath)
	if keyDepth == csrDepth {
		same, err = areCSRPathsFilesystemEquivalent(keyPath, csrPath)
		return same, false, err
	}

	shorter := keyPath
	longer := csrPath
	shorterDepth := keyDepth
	longerDepth := csrDepth
	if keyDepth > csrDepth {
		shorter = csrPath
		longer = keyPath
		shorterDepth = csrDepth
		longerDepth = keyDepth
	}
	longerPrefix := longer
	for range longerDepth - shorterDepth {
		longerPrefix = filepath.Dir(longerPrefix)
	}
	if !strings.EqualFold(shorter, longerPrefix) {
		return false, false, nil
	}

	nested, err = areCSRPathsFilesystemEquivalent(shorter, longerPrefix)
	return false, nested, err
}

func csrPathDepth(path string) int {
	depth := 0
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		parent := filepath.Dir(current)
		if parent == current {
			return depth
		}
		depth++
	}
}

func areCSRPathsFilesystemEquivalent(leftPath, rightPath string) (bool, error) {
	if leftPath == rightPath {
		return true, nil
	}
	if !strings.EqualFold(leftPath, rightPath) {
		return false, nil
	}

	leftAncestor, leftInfo, leftMissing, err := deepestExistingCSRPath(leftPath)
	if err != nil {
		return false, err
	}
	_, rightInfo, rightMissing, err := deepestExistingCSRPath(rightPath)
	if err != nil {
		return false, err
	}
	if !os.SameFile(leftInfo, rightInfo) || leftMissing != rightMissing {
		return false, nil
	}
	if leftMissing == 0 || haveSameCSRMissingSuffix(leftPath, rightPath, leftMissing) {
		return true, nil
	}
	if !leftInfo.IsDir() {
		return false, nil
	}
	return isCSRDirectoryCaseInsensitive(leftAncestor)
}

func haveSameCSRMissingSuffix(leftPath, rightPath string, missing int) bool {
	left := leftPath
	right := rightPath
	for range missing {
		if filepath.Base(left) != filepath.Base(right) {
			return false
		}
		left = filepath.Dir(left)
		right = filepath.Dir(right)
	}
	return true
}

func deepestExistingCSRPath(path string) (string, os.FileInfo, int, error) {
	current := path
	missing := 0
	for {
		info, err := os.Stat(current)
		if err == nil {
			return current, info, missing, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", nil, 0, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil, 0, err
		}
		current = parent
		missing++
	}
}

func isCSRDirectoryCaseInsensitive(path string) (insensitive bool, err error) {
	probePath, err := os.MkdirTemp(path, ".asc-csr-case-*")
	if err != nil {
		return false, err
	}
	defer func() {
		if removeErr := os.Remove(probePath); removeErr != nil {
			cleanupErr := fmt.Errorf("remove case-sensitivity probe: %w", removeErr)
			if err == nil {
				err = cleanupErr
			} else {
				err = errors.Join(err, cleanupErr)
			}
		}
	}()

	probeInfo, err := os.Lstat(probePath)
	if err != nil {
		return false, err
	}
	caseVariantInfo, err := os.Lstat(csrCaseVariantPath(probePath))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(probeInfo, caseVariantInfo), nil
}

func csrCaseVariantPath(path string) string {
	name := []byte(filepath.Base(path))
	for i, character := range name {
		switch {
		case character >= 'a' && character <= 'z':
			name[i] = character - ('a' - 'A')
			return filepath.Join(filepath.Dir(path), string(name))
		case character >= 'A' && character <= 'Z':
			name[i] = character + ('a' - 'A')
			return filepath.Join(filepath.Dir(path), string(name))
		}
	}
	return path
}

func resolveCSRDestinationPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolute
	missing := []string{}
	// Resolve the deepest existing ancestor so absent output names keep their
	// intended spelling while aliases in the directory chain are canonicalized.
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(absolute), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func isCSRDescendantPath(relative string, err error) bool {
	return err == nil && relative != "" && relative != "." && relative != ".." &&
		!filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func preflightCSRParentDirectory(path string) (string, error) {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if errors.Is(err, os.ErrNotExist) {
			if next := filepath.Dir(parent); next != parent {
				continue
			}
			return "", err
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(parent)
			if err != nil {
				return "", err
			}
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%q is not a directory", parent)
		}
		return parent, nil
	}
}

func probeCSRParentWrite(parent string, requireRename bool) (err error) {
	rooted, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := rooted.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	probe, probeName, err := secureopen.CreateTempNoFollowInRoot(rooted, ".", ".asc-csr-preflight-*", 0o600)
	if err != nil {
		return err
	}
	if closeErr := probe.Close(); closeErr != nil {
		return errors.Join(closeErr, rooted.Remove(probeName))
	}
	currentName := probeName
	defer func() {
		if removeErr := rooted.Remove(currentName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()

	if !requireRename {
		return nil
	}

	spare, spareName, err := secureopen.CreateTempNoFollowInRoot(rooted, ".", ".asc-csr-preflight-*", 0o600)
	if err != nil {
		return err
	}
	spareNeedsCleanup := true
	defer func() {
		if spareNeedsCleanup {
			if removeErr := rooted.Remove(spareName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
		}
	}()
	if closeErr := spare.Close(); closeErr != nil {
		return closeErr
	}
	if removeErr := rooted.Remove(spareName); removeErr != nil {
		return removeErr
	}
	spareNeedsCleanup = false
	if renameErr := rooted.Rename(probeName, spareName); renameErr != nil {
		return renameErr
	}
	currentName = spareName
	return nil
}

func validateCSRFileOutputPath(path string) (string, error) {
	return validateCSRFileOutputPathWithSeparator(path, os.IsPathSeparator)
}

func validateCSRFileOutputPathWithSeparator(path string, isPathSeparator func(uint8) bool) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("output path is required")
	}
	if isPathSeparator(trimmed[len(trimmed)-1]) {
		return "", fmt.Errorf("output path must be a file")
	}
	return trimmed, nil
}

func writeFileBytesNoSymlink(path string, data []byte, perm os.FileMode, force bool) error {
	trimmed, err := validateCSRFileOutputPath(path)
	if err != nil {
		return err
	}

	_, err = shared.SafeWriteFileNoSymlink(
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
