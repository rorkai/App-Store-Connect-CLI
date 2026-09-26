package signing

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	defaultSigningKeychainPartition = "apple-tool:,apple:,codesign:"
	keychainDiagnosticLimit         = 1 << 10
)

var (
	keychainHostGOOS      = runtime.GOOS
	runKeychainSecurity   = defaultRunKeychainSecurity
	keychainLockState     = defaultKeychainLockState
	keychainListLimit     = 64
	keychainIdentityLimit = 128

	keychainIdentityLinePattern = regexp.MustCompile(`^\s*\d+\)\s+([0-9A-Fa-f]{40})\s`)
)

func SigningKeychainListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc signing keychain list",
		ShortHelp:  "List signing keychains, lock state, and identities.",
		LongHelp: `List user search-list keychains and the code-signing identities in each.

The command is read-only. It does not unlock a keychain, prompt for a password,
or change the search list. Identities are read from certificates, so a locked
keychain still reports them.

Examples:
  asc signing keychain list --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("signing keychain list does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			result, err := listSigningKeychains(ctx)
			if err != nil {
				return err
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func SigningKeychainUnlockCommand() *ffcli.Command {
	return signingKeychainPasswordCommand("unlock", "Unlock a dedicated signing keychain.", func(ctx context.Context, options *signingKeychainPasswordOptions) error {
		return applySigningKeychainPasswordCommand(ctx, options, "unlock")
	})
}

func SigningKeychainSetTimeoutCommand() *ffcli.Command {
	return signingKeychainPasswordCommand("set-timeout", "Set the lock timeout for a dedicated signing keychain.", func(ctx context.Context, options *signingKeychainPasswordOptions) error {
		if !options.TimeoutSet && !options.NoTimeout {
			return shared.UsageError("signing keychain set-timeout: --timeout or --no-timeout is required")
		}
		return applySigningKeychainPasswordCommand(ctx, options, "set-timeout")
	})
}

func SigningKeychainSetPartitionListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set-partition-list", flag.ExitOnError)
	keychainPath := fs.String("keychain", "", "Dedicated keychain path")
	passwordFile := fs.String("keychain-password-file", "", "Protected file containing the keychain password")
	partition := fs.String("partition", defaultSigningKeychainPartition, "Partition list applied to the signing private keys")
	identitySHA := fs.String("identity-sha256", "", "Only apply when this code-signing identity certificate SHA-256 is present")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "set-partition-list",
		ShortUsage: "asc signing keychain set-partition-list --keychain PATH --keychain-password-file PATH [flags]",
		ShortHelp:  "Reapply the codesign partition list.",
		LongHelp: `Unlock the keychain and reapply the partition list to its signing private keys.

The password is sent to security on stdin, never as a process argument. Repeat calls are safe.

Examples:
  asc signing keychain set-partition-list --keychain .asc/keychains/release.keychain-db --keychain-password-file .asc/secrets/keychain-password --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("signing keychain set-partition-list does not accept positional arguments")
			}
			options, err := parseSigningKeychainPasswordOptions(*keychainPath, *passwordFile, 0, false, false, *partition, *identitySHA)
			if err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			if err := applySigningKeychainPasswordCommand(ctx, &options, "set-partition-list"); err != nil {
				return err
			}
			return shared.PrintOutput(&asc.SigningKeychainActionResult{Action: "set-partition-list", KeychainPath: options.KeychainPath, Partition: options.Partition}, *output.Output, *output.Pretty)
		},
	}
}

func SigningKeychainLockCommand() *ffcli.Command {
	fs := flag.NewFlagSet("lock", flag.ExitOnError)
	keychainPath := fs.String("keychain", "", "Dedicated keychain path")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "lock",
		ShortUsage: "asc signing keychain lock --keychain PATH",
		ShortHelp:  "Lock a dedicated signing keychain.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("signing keychain lock does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			if err := requireKeychainHost("lock"); err != nil {
				return err
			}
			resolved, err := resolveExistingKeychainPath(*keychainPath, "lock")
			if err != nil {
				return err
			}
			if _, err := runKeychainStep(ctx, "lock", nil, nil, "lock-keychain", resolved); err != nil {
				return err
			}
			return shared.PrintOutput(&asc.SigningKeychainActionResult{Action: "lock", KeychainPath: resolved}, *output.Output, *output.Pretty)
		},
	}
}

func SigningKeychainDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	keychainPath := fs.String("keychain", "", "Dedicated keychain path")
	confirm := fs.Bool("confirm", false, "Confirm keychain deletion")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc signing keychain delete --keychain PATH --confirm",
		ShortHelp:  "Delete an explicitly selected signing keychain.",
		LongHelp: `Delete the keychain at the explicit --keychain path.

There is no default path. Symbolic links are resolved first. The login and
System keychains, and the user's current default keychain, are refused. Deleting
a keychain also removes it from the user search list.

Examples:
  asc signing keychain delete --keychain .asc/keychains/release.keychain-db --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("signing keychain delete does not accept positional arguments")
			}
			if !*confirm {
				return shared.UsageError("signing keychain delete: --confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			if err := requireKeychainHost("delete"); err != nil {
				return err
			}
			resolved, err := resolveExistingKeychainPath(*keychainPath, "delete")
			if err != nil {
				return err
			}
			if err := refuseDefaultKeychain(ctx, resolved); err != nil {
				return err
			}
			if _, err := runKeychainStep(ctx, "delete", nil, nil, "delete-keychain", resolved); err != nil {
				return err
			}
			return shared.PrintOutput(&asc.SigningKeychainActionResult{Action: "delete", KeychainPath: resolved}, *output.Output, *output.Pretty)
		},
	}
}

type signingKeychainPasswordOptions struct {
	KeychainPath   string
	PasswordPath   string
	Timeout        int
	TimeoutSet     bool
	NoTimeout      bool
	Partition      string
	IdentitySHA256 string
}

func signingKeychainPasswordCommand(name, shortHelp string, run func(context.Context, *signingKeychainPasswordOptions) error) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	keychainPath := fs.String("keychain", "", "Dedicated keychain path")
	passwordFile := fs.String("keychain-password-file", "", "Protected file containing the keychain password")
	timeout := fs.Int("timeout", 0, "Lock timeout in seconds")
	noTimeout := fs.Bool("no-timeout", false, "Do not automatically lock the keychain")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc signing keychain " + name + " --keychain PATH --keychain-password-file PATH [flags]",
		ShortHelp:  shortHelp,
		LongHelp: shortHelp + `

The password is read from --keychain-password-file and sent to security on stdin. It is never placed in the process argument list.
Changing the timeout keeps the keychain's existing lock-on-sleep setting.

Examples:
  asc signing keychain ` + name + ` --keychain .asc/keychains/release.keychain-db --keychain-password-file .asc/secrets/keychain-password --timeout 3600`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("signing keychain %s does not accept positional arguments", name)
			}
			timeoutSet := false
			fs.Visit(func(flag *flag.Flag) {
				if flag.Name == "timeout" {
					timeoutSet = true
				}
			})
			options, err := parseSigningKeychainPasswordOptions(*keychainPath, *passwordFile, *timeout, timeoutSet, *noTimeout, "", "")
			if err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			if err := run(ctx, &options); err != nil {
				return err
			}
			return shared.PrintOutput(&asc.SigningKeychainActionResult{Action: name, KeychainPath: options.KeychainPath}, *output.Output, *output.Pretty)
		},
	}
}

func parseSigningKeychainPasswordOptions(keychainPath, passwordPath string, timeout int, timeoutSet, noTimeout bool, partition, identitySHA string) (signingKeychainPasswordOptions, error) {
	if strings.TrimSpace(keychainPath) == "" {
		return signingKeychainPasswordOptions{}, shared.UsageError("signing keychain: --keychain is required")
	}
	if strings.TrimSpace(passwordPath) == "" {
		return signingKeychainPasswordOptions{}, shared.UsageError("signing keychain: --keychain-password-file is required")
	}
	if timeoutSet && noTimeout {
		return signingKeychainPasswordOptions{}, shared.UsageError("signing keychain: --timeout and --no-timeout are mutually exclusive")
	}
	if timeoutSet && timeout <= 0 {
		return signingKeychainPasswordOptions{}, shared.UsageError("signing keychain: --timeout must be greater than 0")
	}
	if strings.TrimSpace(partition) == "" {
		partition = defaultSigningKeychainPartition
	}
	sha := strings.ToLower(strings.TrimSpace(identitySHA))
	if sha != "" {
		if decoded, err := hex.DecodeString(sha); err != nil || len(decoded) != sha256.Size {
			return signingKeychainPasswordOptions{}, shared.UsageError("signing keychain: --identity-sha256 must be 64 hexadecimal characters")
		}
	}
	return signingKeychainPasswordOptions{
		KeychainPath:   strings.TrimSpace(keychainPath),
		PasswordPath:   strings.TrimSpace(passwordPath),
		Timeout:        timeout,
		TimeoutSet:     timeoutSet,
		NoTimeout:      noTimeout,
		Partition:      strings.TrimSpace(partition),
		IdentitySHA256: sha,
	}, nil
}

// applySigningKeychainPasswordCommand runs each security step as its own
// process and stops at the first failure, so a wrong password never lets a
// later step run against a locked keychain (which would prompt for the
// password in a GUI session).
func applySigningKeychainPasswordCommand(ctx context.Context, options *signingKeychainPasswordOptions, action string) error {
	if err := requireKeychainHost(action); err != nil {
		return err
	}
	resolved, err := resolveExistingKeychainPath(options.KeychainPath, action)
	if err != nil {
		return err
	}
	options.KeychainPath = resolved
	password, err := readSigningKeychainPassword(options.PasswordPath, action)
	if err != nil {
		return err
	}
	defer clear(password)
	stdin := make([]byte, len(password)+1)
	copy(stdin, password)
	stdin[len(password)] = '\n'
	defer clear(stdin)

	if _, err := runKeychainStep(ctx, action, stdin, password, "unlock-keychain", resolved); err != nil {
		return err
	}
	if options.TimeoutSet || options.NoTimeout {
		if err := setKeychainTimeout(ctx, action, resolved, *options); err != nil {
			return err
		}
	}
	if action != "set-partition-list" {
		return nil
	}
	if options.IdentitySHA256 != "" {
		identities, err := listKeychainIdentities(ctx, action, resolved)
		if err != nil {
			return err
		}
		found := false
		for _, identity := range identities {
			if identity.SHA256 == options.IdentitySHA256 {
				found = true
				break
			}
		}
		if !found {
			return shared.NewValidationError(fmt.Errorf("signing keychain set-partition-list: code-signing identity %s was not found in %s", options.IdentitySHA256, resolved))
		}
	}
	_, err = runKeychainStep(ctx, action, stdin, password, "set-key-partition-list", "-S", options.Partition, "-s", "-t", "private", resolved)
	return err
}

func setKeychainTimeout(ctx context.Context, action, path string, options signingKeychainPasswordOptions) error {
	stdout, stderr, err := runKeychainSecurity(ctx, nil, "show-keychain-info", path)
	if err != nil {
		return keychainStepError(action, stderr, err, nil)
	}
	args := []string{"set-keychain-settings"}
	if strings.Contains(string(stdout)+" "+string(stderr), "lock-on-sleep") {
		args = append(args, "-l")
	}
	if !options.NoTimeout {
		args = append(args, "-u", "-t", strconv.Itoa(options.Timeout))
	}
	args = append(args, path)
	_, err = runKeychainStep(ctx, action, nil, nil, args...)
	return err
}

func runKeychainStep(ctx context.Context, action string, stdin, secret []byte, args ...string) ([]byte, error) {
	stdout, stderr, err := runKeychainSecurity(ctx, stdin, args...)
	if err != nil {
		return nil, keychainStepError(action, stderr, err, secret)
	}
	return stdout, nil
}

func keychainStepError(action string, stderr []byte, err error, secret []byte) error {
	diagnostic := keychainSecurityDiagnostic(stderr, secret)
	if diagnostic == "" {
		return fmt.Errorf("signing keychain %s: %w", action, err)
	}
	return fmt.Errorf("signing keychain %s: %s: %w", action, diagnostic, err)
}

// keychainSecurityDiagnostic returns security's stderr with the password
// prompt, the password itself, and terminal control characters removed.
func keychainSecurityDiagnostic(stderr, secret []byte) string {
	lines := strings.Split(string(stderr), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "password to unlock ") {
			index := strings.Index(line, ": security: ")
			if index < 0 {
				continue
			}
			line = line[index+2:]
		}
		kept = append(kept, line)
	}
	diagnostic := strings.Join(kept, "\n")
	if len(secret) > 0 {
		for _, representation := range []string{string(secret), hex.EncodeToString(secret), strings.ToUpper(hex.EncodeToString(secret))} {
			diagnostic = strings.ReplaceAll(diagnostic, representation, "[REDACTED]")
		}
	}
	diagnostic = strings.TrimSpace(shared.SanitizeTerminal(diagnostic))
	if len(diagnostic) <= keychainDiagnosticLimit {
		return diagnostic
	}
	diagnostic = diagnostic[:keychainDiagnosticLimit]
	for !utf8.ValidString(diagnostic) {
		diagnostic = diagnostic[:len(diagnostic)-1]
	}
	return diagnostic + " [truncated]"
}

func requireKeychainHost(action string) error {
	if keychainHostGOOS == "darwin" {
		return nil
	}
	return shared.NewValidationError(fmt.Errorf("signing keychain %s is supported only on macOS", action))
}

// resolveExistingKeychainPath resolves symbolic links and refuses the login and
// System keychains by name, location, and file identity (hard links) before any
// security command runs.
func resolveExistingKeychainPath(path, action string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", shared.UsageErrorf("signing keychain %s: --keychain is required", action)
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", shared.UsageErrorf("signing keychain %s: %s", action, err.Error())
	}
	absolute = filepath.Clean(absolute)
	if reason := refusedKeychainPath(absolute); reason != "" {
		return "", shared.UsageErrorf("signing keychain %s: %s", action, reason)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", shared.NewValidationError(fmt.Errorf("signing keychain %s: keychain %s does not exist", action, absolute))
		}
		return "", fmt.Errorf("signing keychain %s: resolve keychain: %w", action, err)
	}
	if reason := refusedKeychainPath(physical); reason != "" {
		return "", shared.UsageErrorf("signing keychain %s: %s", action, reason)
	}
	info, err := os.Stat(physical)
	if err != nil {
		return "", fmt.Errorf("signing keychain %s: inspect keychain: %w", action, err)
	}
	if !info.Mode().IsRegular() {
		return "", shared.NewValidationError(fmt.Errorf("signing keychain %s: %s is not a keychain file", action, physical))
	}
	for _, protected := range protectedKeychainFiles() {
		protectedInfo, err := os.Stat(protected)
		if err == nil && os.SameFile(info, protectedInfo) {
			return "", shared.UsageErrorf("signing keychain %s: refusing to operate on the login or System keychain; pass an explicit dedicated keychain path", action)
		}
	}
	return physical, nil
}

func refusedKeychainPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "login.keychain", "login.keychain-db", "system.keychain", "system.keychain-db":
		return "refusing to operate on the login or System keychain; pass an explicit dedicated keychain path"
	}
	cleaned := strings.ToLower(filepath.ToSlash(path))
	if strings.HasPrefix(cleaned, "/library/keychains/") || strings.HasPrefix(cleaned, "/system/library/keychains/") {
		return "refusing to operate on a system keychain"
	}
	return ""
}

func protectedKeychainFiles() []string {
	paths := []string{"/Library/Keychains/System.keychain"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(
			paths,
			filepath.Join(home, "Library", "Keychains", "login.keychain-db"),
			filepath.Join(home, "Library", "Keychains", "login.keychain"),
		)
	}
	return paths
}

// refuseDefaultKeychain fails closed when the default keychain cannot be read
// or resolves to the keychain being deleted.
func refuseDefaultKeychain(ctx context.Context, resolved string) error {
	stdout, stderr, err := runKeychainSecurity(ctx, nil, "default-keychain", "-d", "user")
	if err != nil {
		return keychainStepError("delete: refusing because the default keychain could not be read", stderr, err, nil)
	}
	target, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("signing keychain delete: inspect keychain: %w", err)
	}
	for _, path := range parseKeychainList(stdout) {
		if physical, err := filepath.EvalSymlinks(path); err == nil {
			path = physical
		}
		if path == resolved {
			return shared.UsageError("signing keychain delete: refusing to delete the user's default keychain")
		}
		if info, err := os.Stat(path); err == nil && os.SameFile(info, target) {
			return shared.UsageError("signing keychain delete: refusing to delete the user's default keychain")
		}
	}
	return nil
}

func readSigningKeychainPassword(path, action string) ([]byte, error) {
	data, err := readBoundedSigningRunFile(path, signingRunPasswordLimit, true)
	if err != nil {
		return nil, fmt.Errorf("signing keychain %s: read keychain password: %w", action, err)
	}
	password := trimSigningKeychainSecret(data)
	if len(password) == 0 {
		clear(data)
		return nil, shared.UsageErrorf("signing keychain %s: keychain password is empty", action)
	}
	if bytes.ContainsAny(password, "\r\n\x00") {
		clear(data)
		return nil, shared.UsageErrorf("signing keychain %s: keychain password must not contain line breaks or NUL bytes", action)
	}
	return password, nil
}

func listSigningKeychains(ctx context.Context) (*asc.SigningKeychainListResult, error) {
	if err := requireKeychainHost("list"); err != nil {
		return nil, err
	}
	stdout, err := runKeychainStep(ctx, "list", nil, nil, "list-keychains", "-d", "user")
	if err != nil {
		return nil, err
	}
	paths := parseKeychainList(stdout)
	if len(paths) > keychainListLimit {
		return nil, fmt.Errorf("signing keychain list: search list has %d keychains; limit is %d", len(paths), keychainListLimit)
	}
	result := &asc.SigningKeychainListResult{Keychains: make([]asc.SigningKeychainInfo, 0, len(paths))}
	for _, path := range paths {
		info := asc.SigningKeychainInfo{Path: path, InSearchList: true}
		exists, locked, err := keychainLockState(path)
		if err != nil {
			return nil, fmt.Errorf("signing keychain list: lock state for %s: %w", path, err)
		}
		info.Exists = exists
		info.Locked = locked
		if exists {
			identities, err := listKeychainIdentities(ctx, "list", path)
			if err != nil {
				return nil, err
			}
			info.Identities = identities
		}
		result.Keychains = append(result.Keychains, info)
	}
	return result, nil
}

// listKeychainIdentities reads code-signing identities without unlocking the
// keychain: find-identity names the certificates that have a private key and
// find-certificate supplies the certificates themselves.
func listKeychainIdentities(ctx context.Context, action, path string) ([]asc.SigningKeychainIdentity, error) {
	identityOutput, err := runKeychainStep(ctx, action, nil, nil, "find-identity", "-p", "codesigning", path)
	if err != nil {
		return nil, err
	}
	identitySHA1 := parseKeychainIdentitySHA1(identityOutput)
	if len(identitySHA1) == 0 {
		return nil, nil
	}
	certificateOutput, err := runKeychainStep(ctx, action, nil, nil, "find-certificate", "-a", "-p", path)
	if err != nil {
		return nil, err
	}
	var identities []asc.SigningKeychainIdentity
	seen := map[string]bool{}
	for _, certificate := range parseKeychainCertificates(certificateOutput) {
		sha1Sum := sha1.Sum(certificate.Raw)
		sha1Hex := hex.EncodeToString(sha1Sum[:])
		if !identitySHA1[sha1Hex] || seen[sha1Hex] {
			continue
		}
		seen[sha1Hex] = true
		sha256Sum := sha256.Sum256(certificate.Raw)
		identities = append(identities, asc.SigningKeychainIdentity{
			SHA256:     hex.EncodeToString(sha256Sum[:]),
			SHA1:       sha1Hex,
			CommonName: certificate.Subject.CommonName,
			ExpiresAt:  certificate.NotAfter.UTC().Format(time.RFC3339),
		})
	}
	if len(identities) > keychainIdentityLimit {
		return nil, fmt.Errorf("signing keychain %s: %s has %d identities; limit is %d", action, path, len(identities), keychainIdentityLimit)
	}
	return identities, nil
}

func parseKeychainIdentitySHA1(output []byte) map[string]bool {
	hashes := map[string]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		if match := keychainIdentityLinePattern.FindStringSubmatch(line); match != nil {
			hashes[strings.ToLower(match[1])] = true
		}
	}
	return hashes
}

func parseKeychainCertificates(output []byte) []*x509.Certificate {
	var certificates []*x509.Certificate
	rest := output
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return certificates
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		certificates = append(certificates, certificate)
	}
}

func parseKeychainList(output []byte) []string {
	var paths []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.Trim(strings.TrimSpace(line), `"`)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths
}
