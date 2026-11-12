package certificates

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const maxCertificateExportFileSize int64 = 32 << 20

// Deterministic race hooks for tests. They run at the two moments a local
// attacker would race the export: after the advisory preflight and after the
// destination parent has been pinned. Both are nil outside tests.
var (
	certificateExportTestHookAfterPreflight    func()
	certificateExportTestHookAfterParentPinned func()
)

type certificateExportOptions struct {
	CertificatePath string
	PrivateKeyPath  string
	CSRPath         string
	PasswordPath    string
	P12Out          string
	P12Encoding     shared.PKCS12Encoding
	Force           bool
}

type certificateExportInput struct {
	Data []byte
}

// CertificatesExportCommand returns the local certificate packaging command.
func CertificatesExportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("export", flag.ExitOnError)

	certificatePath := fs.String("certificate", "", "Apple-issued X.509 certificate path (DER .cer or PEM)")
	privateKeyPath := fs.String("private-key", "", "Matching unencrypted RSA or EC private key path (PEM)")
	csrPath := fs.String("csr", "", "Optional CSR path to verify against the certificate and private key")
	passwordPath := fs.String("password-file", "", "Protected file containing the PKCS#12 password")
	p12Out := fs.String("p12-out", "", "Destination path for the password-protected PKCS#12 identity")
	p12Encoding := fs.String("p12-encoding", string(shared.PKCS12EncodingModern), shared.PKCS12EncodingFlagUsage)
	force := fs.Bool("force", false, "Replace an existing PKCS#12 identity")
	confirm := fs.Bool("confirm", false, "Confirm replacement when --force is set")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc certificates export --certificate ./push/push.cer --private-key ./push/push.key --password-file ./push/password --p12-out ./push/push.p12 [--csr ./push/push.csr] [--p12-encoding legacy] [--force --confirm]",
		ShortHelp:  "Package a certificate and private key as a protected PKCS#12 identity.",
		LongHelp: "Package an Apple-issued certificate and its matching private key as a\n" +
			"password-protected PKCS#12 identity. This command is local-only: obtain the\n" +
			"certificate through Apple's Developer website after uploading the CSR.\n\n" +
			"The command accepts DER or PEM certificates, validates the private-key match,\n" +
			"and optionally verifies the original CSR. It never prints key material or\n" +
			"writes binary PKCS#12 data to stdout. Replacing an existing output requires\n" +
			"both --force and --confirm.\n\n" +
			"The identity uses the modern encoding (PBES2 with AES-256-CBC and an\n" +
			"HMAC-SHA-256 MAC) by default. Some importers reject it with an error that\n" +
			"looks like a wrong password; --p12-encoding legacy writes 3DES with a SHA-1\n" +
			"MAC instead. The legacy algorithms are weak, so protect that file by other\n" +
			"means.\n\n" +
			"Examples:\n" +
			"  asc certificates export --certificate \"./push/push.cer\" --private-key \"./push/push.key\" --password-file \"./secrets/push.p12.password\" --p12-out \"./push/push.p12\"\n" +
			"  asc certificates export --certificate \"./push/push.cer\" --private-key \"./push/push.key\" --csr \"./push/push.csr\" --password-file \"./secrets/push.p12.password\" --p12-out \"./push/push.p12\" --output json\n" +
			"  asc certificates export --certificate \"./push/push.cer\" --private-key \"./push/push.key\" --password-file \"./secrets/push.p12.password\" --p12-out \"./push/push.p12\" --force --confirm\n" +
			"  asc certificates export --certificate \"./push/push.cer\" --private-key \"./push/push.key\" --password-file \"./secrets/push.p12.password\" --p12-out \"./push/push.p12\" --p12-encoding legacy",
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("certificates export does not accept positional arguments")
			}

			certificateValue := *certificatePath
			if strings.TrimSpace(certificateValue) == "" {
				fmt.Fprintln(os.Stderr, "Error: --certificate is required")
				return shared.MissingRequiredUsageError("--certificate")
			}
			privateKeyValue := *privateKeyPath
			if strings.TrimSpace(privateKeyValue) == "" {
				fmt.Fprintln(os.Stderr, "Error: --private-key is required")
				return shared.MissingRequiredUsageError("--private-key")
			}
			passwordValue := *passwordPath
			if strings.TrimSpace(passwordValue) == "" {
				fmt.Fprintln(os.Stderr, "Error: --password-file is required")
				return shared.MissingRequiredUsageError("--password-file")
			}
			p12Value := *p12Out
			if strings.TrimSpace(p12Value) == "" {
				fmt.Fprintln(os.Stderr, "Error: --p12-out is required")
				return shared.MissingRequiredUsageError("--p12-out")
			}
			csrSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "csr" {
					csrSet = true
				}
			})
			if csrSet && strings.TrimSpace(*csrPath) == "" {
				// An explicitly empty --csr must not silently disable the
				// requested CSR verification; omit the flag to skip it.
				return shared.UsageError("--csr must not be empty")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			encoding, err := shared.ParsePKCS12Encoding(*p12Encoding)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *confirm && !*force {
				return shared.UsageError("--confirm requires --force")
			}
			if *force && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required with --force")
				return shared.MissingRequiredUsageError("--confirm")
			}

			result, err := runCertificateExport(ctx, certificateExportOptions{
				CertificatePath: certificateValue,
				PrivateKeyPath:  privateKeyValue,
				CSRPath:         *csrPath,
				PasswordPath:    passwordValue,
				P12Out:          p12Value,
				P12Encoding:     encoding,
				Force:           *force,
			})
			if err != nil {
				return fmt.Errorf("certificates export: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func runCertificateExport(_ context.Context, opts certificateExportOptions) (*asc.CertificateExportResult, error) {
	certificatePath, err := validateCertificateExportInputPath(opts.CertificatePath, "--certificate")
	if err != nil {
		return nil, err
	}
	privateKeyPath, err := validateCertificateExportInputPath(opts.PrivateKeyPath, "--private-key")
	if err != nil {
		return nil, err
	}
	passwordPath, err := validateCertificateExportInputPath(opts.PasswordPath, "--password-file")
	if err != nil {
		return nil, err
	}
	p12Out, err := validateCertificateExportOutputPath(opts.P12Out)
	if err != nil {
		return nil, err
	}
	csrPath := opts.CSRPath
	if csrPath != "" {
		csrPath, err = validateCertificateExportInputPath(csrPath, "--csr")
		if err != nil {
			return nil, err
		}
	}

	inputPaths := []string{certificatePath, privateKeyPath, passwordPath}
	if csrPath != "" {
		inputPaths = append(inputPaths, csrPath)
	}
	if err := preflightCertificateExportDestination(p12Out, opts.Force, inputPaths...); err != nil {
		return nil, err
	}
	if certificateExportTestHookAfterPreflight != nil {
		certificateExportTestHookAfterPreflight()
	}

	// Pin the destination parent before any input is read so the validated
	// directory chain cannot be swapped for a symlink while inputs are parsed
	// or the PKCS#12 is encoded. Publication happens through this handle.
	outputParent, outputBase, err := pinCertificateExportDestination(p12Out)
	if err != nil {
		return nil, err
	}
	defer func() { _ = outputParent.Close() }()
	if certificateExportTestHookAfterParentPinned != nil {
		certificateExportTestHookAfterParentPinned()
	}

	certificateInput, err := readCertificateExportInput(certificatePath, "certificate", false)
	if err != nil {
		return nil, err
	}
	defer clear(certificateInput.Data)
	certificate, err := parseCertificateExportCertificate(certificateInput.Data)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
		return nil, fmt.Errorf("certificate is not currently valid")
	}
	if certificate.IsCA {
		return nil, fmt.Errorf("certificate must be a leaf certificate")
	}

	privateKeyInput, err := readCertificateExportInput(privateKeyPath, "private key", true)
	if err != nil {
		return nil, err
	}
	defer clear(privateKeyInput.Data)
	privateKey, err := parseCertificateExportPrivateKey(privateKeyInput.Data)
	if err != nil {
		return nil, err
	}
	if !certificateExportPublicKeysEqual(privateKey, certificate.PublicKey) {
		return nil, fmt.Errorf("private key does not match certificate")
	}

	var csrMatched *bool
	if csrPath != "" {
		csrInput, readErr := readCertificateExportInput(csrPath, "CSR", false)
		if readErr != nil {
			return nil, readErr
		}
		defer clear(csrInput.Data)
		csr, parseErr := parseCertificateExportCSR(csrInput.Data)
		if parseErr != nil {
			return nil, parseErr
		}
		matched := certificateExportPublicKeysEqual(csr.PublicKey, certificate.PublicKey) && certificateExportPublicKeysEqual(privateKey, csr.PublicKey)
		if !matched {
			return nil, fmt.Errorf("CSR public key does not match certificate and private key")
		}
		csrMatched = &matched
	}

	passwordInput, err := readCertificateExportInput(passwordPath, "password", true)
	if err != nil {
		return nil, err
	}
	defer clear(passwordInput.Data)
	password := trimCertificateExportPassword(passwordInput.Data)
	if len(password) == 0 {
		return nil, fmt.Errorf("password file contains an empty password")
	}

	p12Data, err := shared.EncodePKCS12Identity(opts.P12Encoding, privateKey, certificate, string(password))
	if err != nil {
		return nil, fmt.Errorf("encode PKCS#12 identity: %w", err)
	}
	defer clear(p12Data)

	if _, err := shared.SafeWriteFileNoSymlinkWithPreparationAndCreatorInRoot(
		outputParent,
		p12Out,
		outputBase,
		0o600,
		opts.Force,
		".asc-cert-export-*",
		".asc-cert-export-backup-*",
		prepareCertificateExportOutput,
		createCertificateExportStagingFile,
		func(file *os.File) (int64, error) {
			n, writeErr := file.Write(p12Data)
			return int64(n), writeErr
		},
	); err != nil {
		return nil, fmt.Errorf("write --p12-out: %w", err)
	}

	keyType, keySize := certificateExportKeyDetails(privateKey)
	result := &asc.CertificateExportResult{
		Operation:         "certificates export",
		CertificatePath:   certificatePath,
		PrivateKeyPath:    privateKeyPath,
		CSRPath:           csrPath,
		P12Out:            p12Out,
		CertificateSHA256: certificateExportCertificateSHA256(certificate),
		NotBefore:         certificate.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:          certificate.NotAfter.UTC().Format(time.RFC3339),
		KeyType:           keyType,
		KeySize:           keySize,
		PrivateKeyMatched: true,
		CSRMatched:        csrMatched,
	}
	return result, nil
}

func validateCertificateExportInputPath(path, flagName string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", shared.UsageErrorf("%s is required", flagName)
	}
	if isCertificateExportDirectoryPath(path) {
		return "", shared.UsageErrorf("%s must be a file path", flagName)
	}
	return path, nil
}

func validateCertificateExportOutputPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", shared.UsageError("--p12-out is required")
	}
	if path == "-" {
		return "", shared.UsageError("--p12-out must be a file path, not stdout")
	}
	if isCertificateExportDirectoryPath(path) {
		return "", shared.UsageError("--p12-out must be a file path")
	}
	return path, nil
}

func isCertificateExportDirectoryPath(path string) bool {
	if path == "" {
		return false
	}
	last := path[len(path)-1]
	return os.IsPathSeparator(last)
}

func preflightCertificateExportDestination(output string, force bool, inputs ...string) error {
	outputAbs, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("resolve --p12-out: %w", err)
	}
	outputAbs = filepath.Clean(outputAbs)
	if err := rejectCertificateExportSymlinkedParent(outputAbs); err != nil {
		return err
	}
	for _, input := range inputs {
		inputAbs, absErr := filepath.Abs(input)
		if absErr != nil {
			return fmt.Errorf("resolve input path: %w", absErr)
		}
		if outputAbs == filepath.Clean(inputAbs) {
			return fmt.Errorf("--p12-out must differ from input path %q", input)
		}
	}

	info, err := os.Lstat(output)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect --p12-out: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink %q", output)
	}
	if info.IsDir() {
		return fmt.Errorf("--p12-out %q is a directory", output)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("--p12-out %q is not a regular file", output)
	}
	if !force {
		return fmt.Errorf("output file already exists: %w", os.ErrExist)
	}

	for _, input := range inputs {
		inputInfo, statErr := os.Stat(input)
		if statErr != nil {
			continue
		}
		if os.SameFile(info, inputInfo) {
			return fmt.Errorf("--p12-out must not resolve to input path %q", input)
		}
	}
	return nil
}

// rejectCertificateExportSymlinkedParent checks the existing destination
// parents through a selected, anchored no-follow traversal before any input is
// read. Components below the first missing parent do not exist yet; the
// pinned write-time walk creates them.
func rejectCertificateExportSymlinkedParent(output string) error {
	parent, err := openCertificateExportDestinationParent(output, false)
	if err != nil {
		return err
	}
	if parent != nil {
		_ = parent.Close()
	}
	return nil
}

// pinCertificateExportDestination resolves --p12-out and opens its parent
// directory through the anchored, no-follow traversal, creating missing
// components through the pinned roots. The returned root is held from before
// the first input read until publication so a concurrent swap of a checked
// parent cannot redirect where the identity is written.
func pinCertificateExportDestination(output string) (*os.Root, string, error) {
	outputAbs, err := filepath.Abs(output)
	if err != nil {
		return nil, "", fmt.Errorf("resolve --p12-out: %w", err)
	}
	outputAbs = filepath.Clean(outputAbs)
	parent, err := openCertificateExportDestinationParent(outputAbs, true)
	if err != nil {
		return nil, "", err
	}
	return parent, filepath.Base(outputAbs), nil
}

// openCertificateExportDestinationParent walks the destination's parent chain
// through a selected, anchored no-follow traversal and returns the pinned
// parent directory root. The working directory and temporary-directory anchors
// preserve normal platform layouts such as macOS's /var and /tmp aliases while still
// rejecting symlinks introduced below the operator's likely output root. When
// createMissing is false the walk stops at the first missing component and
// returns a nil root; when true, missing components are created through the
// pinned traversal so no path-based MkdirAll can be redirected by a concurrent
// symlink swap. The final output entry is checked separately by
// preflightCertificateExportDestination and the rooted writer.
func openCertificateExportDestinationParent(output string, createMissing bool) (*os.Root, error) {
	output = certificateExportRootPath(output)
	volumeRoot := filepath.VolumeName(output) + string(filepath.Separator)
	rootPath := volumeRoot
	for _, candidate := range []string{certificateExportWorkingDirectory(), os.TempDir()} {
		candidateAbs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		candidateAbs = filepath.Clean(candidateAbs)
		if certificateExportPathWithinRoot(candidateAbs, output) && len(candidateAbs) > len(rootPath) {
			rootPath = candidateAbs
		}
	}

	trustedRoot, err := rootfs.New(rootPath)
	if err != nil {
		return nil, fmt.Errorf("inspect --p12-out parent: %w", err)
	}
	defer trustedRoot.Close()

	current, err := trustedRoot.OpenRoot()
	if err != nil {
		return nil, fmt.Errorf("inspect --p12-out parent: %w", err)
	}

	relative, err := filepath.Rel(rootPath, filepath.Dir(output))
	if err != nil {
		_ = current.Close()
		return nil, fmt.Errorf("inspect --p12-out parent: %w", err)
	}
	if relative == "." {
		return current, nil
	}

	path := rootPath
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		componentPath := filepath.Join(path, component)
		next, found, descendErr := descendCertificateExportParent(current, component, componentPath, createMissing)
		_ = current.Close()
		if descendErr != nil {
			return nil, descendErr
		}
		if !found {
			// Check-only mode: components below the first missing parent do
			// not exist yet and are created by the pinned write-time walk.
			return nil, nil
		}
		current = next
		path = componentPath
	}
	return current, nil
}

// descendCertificateExportParent moves the pinned traversal into component.
// os.Root.OpenRoot follows symlinks that stay inside the root, so the entry is
// checked with Lstat before the descent and its identity is verified after,
// keeping the walk no-follow even against concurrent replacement.
func descendCertificateExportParent(current *os.Root, component, componentPath string, createMissing bool) (*os.Root, bool, error) {
	info, statErr := current.Lstat(component)
	if errors.Is(statErr, os.ErrNotExist) {
		if !createMissing {
			return nil, false, nil
		}
		if mkdirErr := current.Mkdir(component, 0o755); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
			return nil, false, fmt.Errorf("create --p12-out parent %q: %w", componentPath, mkdirErr)
		}
		info, statErr = current.Lstat(component)
	}
	if statErr != nil {
		return nil, false, fmt.Errorf("inspect --p12-out parent %q: %w", componentPath, statErr)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("refusing to write --p12-out through symlinked parent %q", componentPath)
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("--p12-out parent %q is not a directory", componentPath)
	}
	next, openErr := current.OpenRoot(component)
	if openErr != nil {
		return nil, false, fmt.Errorf("inspect --p12-out parent %q: %w", componentPath, openErr)
	}
	if err := verifyCertificateExportDescent(current, component, next, componentPath); err != nil {
		_ = next.Close()
		return nil, false, err
	}
	return next, true, nil
}

// verifyCertificateExportDescent confirms the opened directory is exactly the
// non-symlink entry Lstat approved, so an entry swapped between Lstat and
// OpenRoot cannot smuggle a followed symlink into the pinned traversal.
func verifyCertificateExportDescent(parent *os.Root, component string, opened *os.Root, componentPath string) error {
	after, err := parent.Lstat(component)
	if err != nil {
		return fmt.Errorf("inspect --p12-out parent %q: %w", componentPath, err)
	}
	if after.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write --p12-out through symlinked parent %q", componentPath)
	}
	directory, err := opened.Open(".")
	if err != nil {
		return fmt.Errorf("inspect --p12-out parent %q: %w", componentPath, err)
	}
	defer directory.Close()
	openedInfo, err := directory.Stat()
	if err != nil {
		return fmt.Errorf("inspect --p12-out parent %q: %w", componentPath, err)
	}
	if !os.SameFile(after, openedInfo) {
		return fmt.Errorf("--p12-out parent %q changed during inspection", componentPath)
	}
	return nil
}

func certificateExportWorkingDirectory() string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return ""
	}
	return workingDirectory
}

func certificateExportPathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func readCertificateExportInput(path, label string, protected bool) (certificateExportInput, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return certificateExportInput{}, fmt.Errorf("%s file does not exist", label)
		}
		return certificateExportInput{}, fmt.Errorf("open %s without following symlinks: %w", label, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return certificateExportInput{}, fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return certificateExportInput{}, fmt.Errorf("%s must be a regular file", label)
	}
	if protected {
		if err := validateCertificateExportProtectedFile(file, info, label); err != nil {
			return certificateExportInput{}, err
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCertificateExportFileSize+1))
	if err != nil {
		return certificateExportInput{}, fmt.Errorf("read %s: %w", label, err)
	}
	if len(data) == 0 {
		return certificateExportInput{}, fmt.Errorf("%s is empty", label)
	}
	if int64(len(data)) > maxCertificateExportFileSize {
		return certificateExportInput{}, fmt.Errorf("%s exceeds the 32 MiB size limit", label)
	}
	return certificateExportInput{Data: data}, nil
}

func parseCertificateExportCertificate(data []byte) (*x509.Certificate, error) {
	der, err := parseCertificateExportObject(data, "certificate", map[string]bool{"CERTIFICATE": true})
	if err != nil {
		return nil, err
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("certificate is not a valid X.509 certificate: %w", err)
	}
	return certificate, nil
}

func parseCertificateExportCSR(data []byte) (*x509.CertificateRequest, error) {
	der, err := parseCertificateExportObject(data, "CSR", map[string]bool{
		"CERTIFICATE REQUEST":     true,
		"NEW CERTIFICATE REQUEST": true,
	})
	if err != nil {
		return nil, err
	}
	csr, err := x509.ParseCertificateRequest(der)
	if err != nil {
		return nil, fmt.Errorf("CSR is not a valid PKCS#10 request: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("CSR signature is invalid: %w", err)
	}
	return csr, nil
}

func parseCertificateExportObject(data []byte, label string, pemTypes map[string]bool) ([]byte, error) {
	trimmed := bytes.TrimLeftFunc(data, unicode.IsSpace)
	if block, rest := pem.Decode(trimmed); block != nil {
		if !isSingleCertificateExportPEM(trimmed) {
			return nil, fmt.Errorf("%s must contain exactly one object", label)
		}
		if !pemTypes[block.Type] {
			return nil, fmt.Errorf("%s PEM block type %q is unsupported", label, block.Type)
		}
		if next, trailing := pem.Decode(rest); next != nil || len(bytes.TrimSpace(trailing)) != 0 {
			return nil, fmt.Errorf("%s must contain exactly one object", label)
		}
		return parseCertificateExportDER(block.Bytes, label)
	}
	return parseCertificateExportDER(data, label)
}

func parseCertificateExportDER(data []byte, label string) ([]byte, error) {
	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(data, &raw)
	if err != nil || len(rest) != 0 || len(raw.FullBytes) == 0 {
		return nil, fmt.Errorf("%s must contain exactly one DER object", label)
	}
	return raw.FullBytes, nil
}

func parseCertificateExportPrivateKey(data []byte) (any, error) {
	trimmed := bytes.TrimLeftFunc(data, unicode.IsSpace)
	block, rest := pem.Decode(trimmed)
	if block == nil || !isSingleCertificateExportPEM(trimmed) || len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf("private key must contain exactly one PEM object")
	}
	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("private key must be an unencrypted RSA or EC private key")
		}
		if err := validateCertificateExportPrivateKeyType(key); err != nil {
			return nil, err
		}
		return key, nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("private key must be an unencrypted RSA or EC private key")
		}
		return key, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("private key must be an unencrypted RSA or EC private key")
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q for private key", block.Type)
	}
}

func isSingleCertificateExportPEM(data []byte) bool {
	return bytes.HasPrefix(data, []byte("-----BEGIN ")) &&
		bytes.Count(data, []byte("-----BEGIN ")) == 1 &&
		bytes.Count(data, []byte("-----END ")) == 1
}

func validateCertificateExportPrivateKeyType(key any) error {
	switch key.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey:
		return nil
	default:
		return fmt.Errorf("private key must be an RSA or EC private key")
	}
}

func certificateExportPublicKeysEqual(privateOrPublic, public any) bool {
	var derived any
	if signer, ok := privateOrPublic.(crypto.Signer); ok {
		derived = signer.Public()
	} else {
		derived = privateOrPublic
	}
	derivedDER, err := x509.MarshalPKIXPublicKey(derived)
	if err != nil {
		return false
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return false
	}
	return bytes.Equal(derivedDER, publicDER)
}

func certificateExportCertificateSHA256(certificate *x509.Certificate) string {
	if certificate == nil {
		return ""
	}
	sum := sha256.Sum256(certificate.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func certificateExportKeyDetails(key any) (string, int) {
	switch typed := key.(type) {
	case *rsa.PrivateKey:
		return "RSA", typed.N.BitLen()
	case *ecdsa.PrivateKey:
		if typed.Curve != nil && typed.Params() != nil {
			return "EC", typed.Params().BitSize
		}
		return "EC", 0
	default:
		return "", 0
	}
}

func trimCertificateExportPassword(data []byte) []byte {
	if bytes.HasSuffix(data, []byte("\r\n")) {
		return data[:len(data)-2]
	}
	if bytes.HasSuffix(data, []byte("\n")) {
		return data[:len(data)-1]
	}
	return data
}
