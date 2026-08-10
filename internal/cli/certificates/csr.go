package certificates

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/text/unicode/norm"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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

type preparedCSRWrite struct {
	flag         string
	path         string
	data         []byte
	mode         os.FileMode
	force        bool
	original     []byte
	originalMode os.FileMode
	existed      bool
	root         rootfs.Root
	name         string
	backupName   string
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

	keyWrite, err := prepareCSRWrite("--key-out", keyOutValue, keyPEM, 0o600, opts.Force)
	if err != nil {
		return nil, nil, fmt.Errorf("write --key-out: %w", err)
	}
	csrWrite, err := prepareCSRWrite("--csr-out", csrOutValue, csrPEM, 0o644, opts.Force)
	if err != nil {
		return nil, nil, fmt.Errorf("write --csr-out: %w", err)
	}
	writes := []preparedCSRWrite{keyWrite, csrWrite}

	// Write key first so a successful command never exposes a CSR before its key.
	// If the CSR commit fails, restore the key's pre-command state.
	if err := commitCSRWrites(writes, writeFileBytesNoSymlink); err != nil {
		return nil, nil, err
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
	sameOutput, nestedOutput, err := classifyCSRFilesystemRelation(keyPath, csrPath)
	if err != nil {
		return fmt.Errorf("compare --key-out and --csr-out: %w", err)
	}
	if sameOutput {
		return shared.UsageError("--key-out and --csr-out must be different paths")
	}
	if nestedOutput || isCSRDescendantPath(keyToCSR, keyRelativeErr) || isCSRDescendantPath(csrToKey, csrRelativeErr) {
		return shared.UsageError("--key-out and --csr-out must not contain one another")
	}
	return nil
}

func classifyCSRFilesystemRelation(keyPath, csrPath string) (same bool, nested bool, err error) {
	return classifyCSRFilesystemRelationWithStat(keyPath, csrPath, os.Stat)
}

func classifyCSRFilesystemRelationWithStat(keyPath, csrPath string, stat csrStatFunc) (same bool, nested bool, err error) {
	if keyPath == csrPath {
		return true, false, nil
	}
	// Bind mounts and other filesystem aliases can have unrelated spellings and
	// lexical depths. Anchor the comparison on the deepest existing inode, then
	// compare only the unresolved destination components beneath it.
	keyAncestor, keyInfo, keyMissing, err := deepestExistingCSRPath(keyPath, stat)
	if err != nil {
		return false, false, err
	}
	_, csrInfo, csrMissing, err := deepestExistingCSRPath(csrPath, stat)
	if err != nil {
		return false, false, err
	}
	if !os.SameFile(keyInfo, csrInfo) {
		return false, false, nil
	}

	keyComponents := csrMissingPathComponents(keyPath, keyMissing)
	csrComponents := csrMissingPathComponents(csrPath, csrMissing)
	if len(keyComponents) == len(csrComponents) {
		same, err = areCSRPathComponentsEquivalent(keyAncestor, keyInfo, keyComponents, csrComponents)
		return same, false, err
	}
	shorter := keyComponents
	longer := csrComponents
	if len(csrComponents) < len(keyComponents) {
		shorter = csrComponents
		longer = keyComponents
	}
	nested, err = areCSRPathComponentsEquivalent(keyAncestor, keyInfo, shorter, longer[:len(shorter)])
	return false, nested, err
}

type csrStatFunc func(string) (os.FileInfo, error)

func normalizedFoldEqual(left, right string) bool {
	return strings.EqualFold(norm.NFC.String(left), norm.NFC.String(right))
}

func csrMissingPathComponents(path string, missing int) []string {
	components := make([]string, missing)
	current := path
	for i := missing - 1; i >= 0; i-- {
		components[i] = filepath.Base(current)
		current = filepath.Dir(current)
	}
	return components
}

func sameCSRPathComponents(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func areCSRPathComponentsEquivalent(parent string, parentInfo os.FileInfo, left, right []string) (bool, error) {
	if sameCSRPathComponents(left, right) {
		return true, nil
	}
	if !parentInfo.IsDir() {
		return false, nil
	}
	return probeCSRPathComponentsEquivalent(parent, left, right)
}

func deepestExistingCSRPath(path string, stat csrStatFunc) (string, os.FileInfo, int, error) {
	current := path
	missing := 0
	for {
		info, err := stat(current)
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

func probeCSRPathComponentsEquivalent(parent string, left, right []string) (equivalent bool, err error) {
	if len(left) != len(right) {
		return false, nil
	}
	for i := range left {
		if !normalizedFoldEqual(left[i], right[i]) {
			return false, nil
		}
	}

	// Replay unresolved components beneath a private directory on the target
	// filesystem. SameFile, rather than normalization alone, decides whether
	// the two spellings would address the same destination.
	probePath, err := os.MkdirTemp(parent, ".asc-csr-equivalence-*")
	if err != nil {
		return false, err
	}
	rooted, err := os.OpenRoot(probePath)
	if err != nil {
		return false, errors.Join(err, os.Remove(probePath))
	}
	created := []string{}
	defer func() {
		for i := len(created) - 1; i >= 0; i-- {
			if removeErr := rooted.Remove(created[i]); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove equivalence probe component: %w", removeErr))
			}
		}
		if closeErr := rooted.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(probePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove equivalence probe: %w", removeErr))
		}
	}()

	leftPath := "."
	rightPath := "."
	for i := range left {
		leftPath = filepath.Join(leftPath, left[i])
		rightPath = filepath.Join(rightPath, right[i])
		if err := rooted.Mkdir(leftPath, 0o700); err != nil {
			return false, err
		}
		created = append(created, leftPath)

		leftInfo, err := rooted.Stat(leftPath)
		if err != nil {
			return false, err
		}
		rightInfo, err := rooted.Stat(rightPath)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !os.SameFile(leftInfo, rightInfo) {
			return false, nil
		}
	}
	return true, nil
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

func prepareCSRWrite(flagName, path string, data []byte, mode os.FileMode, force bool) (preparedCSRWrite, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return preparedCSRWrite{}, err
	}
	existingParent, err := preflightCSRParentDirectory(absolute)
	if err != nil {
		return preparedCSRWrite{}, err
	}
	root, err := rootfs.New(existingParent)
	if err != nil {
		return preparedCSRWrite{}, err
	}
	name, err := filepath.Rel(root.Path(), absolute)
	if err != nil {
		return preparedCSRWrite{}, err
	}

	write := preparedCSRWrite{
		flag:  flagName,
		path:  path,
		data:  data,
		mode:  mode,
		force: force,
		root:  root,
		name:  name,
	}
	rooted, err := os.OpenRoot(root.Path())
	if err != nil {
		return preparedCSRWrite{}, err
	}
	defer rooted.Close()
	// Only record existence here. Backing up the destination happens at commit
	// time and does not require read access to the existing file.
	if _, err := rooted.Lstat(name); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return write, nil
		}
		return preparedCSRWrite{}, err
	}
	write.existed = true
	return write, nil
}

type csrWriteFileFunc func(path string, data []byte, mode os.FileMode, force bool) error

func commitCSRWrites(writes []preparedCSRWrite, writeFile csrWriteFileFunc) error {
	committed := make([]*preparedCSRWrite, 0, len(writes))
	rollback := func(writeErr error) error {
		var rollbackErrors []error
		for index := len(committed) - 1; index >= 0; index-- {
			if rollbackErr := restoreCSRWrite(committed[index], writeFile); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", committed[index].flag, rollbackErr))
			}
		}
		if len(rollbackErrors) == 0 {
			return writeErr
		}
		return errors.Join(writeErr, fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrors...)))
	}

	for index := range writes {
		write := &writes[index]
		if err := backupCSRDestination(write); err != nil {
			return rollback(fmt.Errorf("write %s: %w", write.flag, err))
		}
		if err := writeFile(write.path, write.data, write.mode, write.force); err != nil {
			writeErr := fmt.Errorf("write %s: %w", write.flag, err)
			// The overwrite path preserves the destination on failure, in which
			// case restoring from the backup link is a no-op that drops the
			// extra directory entry.
			if restoreErr := restoreCSRBackup(write); restoreErr != nil {
				writeErr = errors.Join(writeErr, fmt.Errorf("restore %s: %w", write.flag, restoreErr))
			}
			return rollback(writeErr)
		}
		committed = append(committed, write)
	}

	var cleanupErrors []error
	for _, write := range committed {
		if err := discardCSRBackup(write); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}

// backupCSRDestination preserves an existing destination before a forced
// replacement. It prefers a hard link to the original directory entry, which
// requires no read access and lets rollback restore the original file with its
// full metadata and inode identity. Filesystems without hard links fall back
// to an in-memory snapshot of the file contents and permissions.
func backupCSRDestination(write *preparedCSRWrite) error {
	if !write.existed || !write.force {
		return nil
	}
	linkErr := linkCSRBackup(write)
	if linkErr == nil {
		return nil
	}
	if write.original != nil {
		return nil
	}
	original, originalMode, err := snapshotCSRDestination(write.root, write.name)
	if err != nil {
		return errors.Join(fmt.Errorf("back up existing %s before replacement: %w", write.flag, linkErr), err)
	}
	write.original = original
	write.originalMode = originalMode
	return nil
}

func linkCSRBackup(write *preparedCSRWrite) error {
	rooted, err := os.OpenRoot(write.root.Path())
	if err != nil {
		return err
	}
	defer rooted.Close()

	dir := filepath.Dir(write.name)
	var randBytes [12]byte
	const maxAttempts = 10_000
	for range maxAttempts {
		if _, err := rand.Read(randBytes[:]); err != nil {
			return err
		}
		backupName := filepath.Join(dir, ".asc-csr-keybackup-"+hex.EncodeToString(randBytes[:]))
		err := rooted.Link(write.name, backupName)
		if err == nil {
			write.backupName = backupName
			return nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return err
	}
	return fmt.Errorf("create backup link for %q", write.path)
}

func snapshotCSRDestination(root rootfs.Root, name string) ([]byte, os.FileMode, error) {
	file, err := root.OpenFile(name)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	original, err := io.ReadAll(file)
	if err != nil {
		return nil, 0, err
	}
	return original, info.Mode().Perm(), nil
}

// restoreCSRBackup puts the backed-up original directory entry back at the
// destination. Rename atomically replaces a failed or partial replacement with
// the original file. When the destination still names the original file, POSIX
// defines the same-file rename as a no-op, so the leftover backup entry is
// removed afterwards.
func restoreCSRBackup(write *preparedCSRWrite) error {
	if write.backupName == "" {
		return nil
	}
	rooted, err := os.OpenRoot(write.root.Path())
	if err != nil {
		return err
	}
	defer rooted.Close()
	if err := rooted.Rename(write.backupName, write.name); err != nil {
		return err
	}
	if err := rooted.Remove(write.backupName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	write.backupName = ""
	return nil
}

func discardCSRBackup(write *preparedCSRWrite) error {
	if write.backupName == "" {
		return nil
	}
	rooted, err := os.OpenRoot(write.root.Path())
	if err != nil {
		return fmt.Errorf("remove backup of replaced %s at %q: %w", write.flag, filepath.Join(write.root.Path(), write.backupName), err)
	}
	defer rooted.Close()
	if err := rooted.Remove(write.backupName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove backup of replaced %s at %q: %w", write.flag, filepath.Join(write.root.Path(), write.backupName), err)
	}
	write.backupName = ""
	return nil
}

func restoreCSRWrite(write *preparedCSRWrite, writeFile csrWriteFileFunc) error {
	if write.backupName != "" {
		return restoreCSRBackup(write)
	}
	if write.existed {
		return writeFile(write.path, write.original, write.originalMode, true)
	}
	rooted, err := os.OpenRoot(write.root.Path())
	if err != nil {
		return err
	}
	defer rooted.Close()
	if err := rooted.Remove(write.name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
