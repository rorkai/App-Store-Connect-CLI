package signing

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var (
	keychainHostGOOS      = runtime.GOOS
	runKeychainSecurity   = defaultRunKeychainSecurity
	keychainListLimit     = 64
	keychainIdentityLimit = 128
)

func SigningKeychainListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc signing keychain list",
		ShortHelp:  "List signing keychains, lock state, and identities.",
		LongHelp: `List user search-list keychains and the code-signing identities in each.

The command is read-only. It does not unlock a keychain or change the search list.

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
		return applySigningKeychainPasswordCommand(ctx, options, "unlock-keychain")
	})
}

func SigningKeychainSetTimeoutCommand() *ffcli.Command {
	return signingKeychainPasswordCommand("set-timeout", "Set the lock timeout for a dedicated signing keychain.", func(ctx context.Context, options *signingKeychainPasswordOptions) error {
		if !options.TimeoutSet && !options.NoTimeout {
			return shared.UsageError("signing keychain set-timeout: --timeout or --no-timeout is required")
		}
		return applySigningKeychainPasswordCommand(ctx, options, "set-keychain-settings")
	})
}

func SigningKeychainSetPartitionListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set-partition-list", flag.ExitOnError)
	keychainPath := fs.String("keychain", "", "Dedicated keychain path")
	passwordFile := fs.String("keychain-password-file", "", "Protected file containing the keychain password")
	partition := fs.String("partition", "apple-tool:,apple:,codesign:", "Partition list applied to the signing identity")
	identitySHA := fs.String("identity-sha256", "", "Only apply when this certificate SHA-256 is present")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "set-partition-list",
		ShortUsage: "asc signing keychain set-partition-list --keychain PATH --keychain-password-file PATH [flags]",
		ShortHelp:  "Reapply the codesign partition list.",
		LongHelp: `Reapply the signing partition list without printing the keychain password.

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
			if err := requireKeychainHost("set-partition-list"); err != nil {
				return err
			}
			resolved, err := resolveExistingKeychainPath(options.KeychainPath, "set-partition-list")
			if err != nil {
				return err
			}
			password, err := readSigningKeychainPassword(options.PasswordPath, "set-partition-list")
			if err != nil {
				return err
			}
			defer clear(password)
			if options.IdentitySHA256 != "" {
				unlock, err := unlockKeychainCommand(resolved, password)
				if err != nil {
					return err
				}
				if _, _, err := runKeychainSecurity(ctx, []byte(unlock), "-i"); err != nil {
					return fmt.Errorf("signing keychain set-partition-list: %w", err)
				}
				if err := requireKeychainIdentity(ctx, resolved, options.IdentitySHA256); err != nil {
					return err
				}
			}
			command, err := partitionListCommand(resolved, password, options.Partition)
			if err != nil {
				return err
			}
			if _, _, err := runKeychainSecurity(ctx, []byte(command+"\n"), "-i"); err != nil {
				return fmt.Errorf("signing keychain set-partition-list: %w", err)
			}
			return shared.PrintOutput(&asc.SigningKeychainActionResult{Action: "set-partition-list", KeychainPath: resolved, Partition: options.Partition}, *output.Output, *output.Pretty)
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
			if _, _, err := runKeychainSecurity(ctx, nil, "lock-keychain", resolved); err != nil {
				return fmt.Errorf("signing keychain lock: %w", err)
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

There is no default path. login.keychain and System.keychain are refused. The command does not remove a keychain merely because it is on the search list.

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
			if _, _, err := runKeychainSecurity(ctx, nil, "delete-keychain", resolved); err != nil {
				return fmt.Errorf("signing keychain delete: %w", err)
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

Examples:
  asc signing keychain ` + name + ` --keychain .asc/keychains/release.keychain-db --keychain-password-file .asc/secrets/keychain-password`,
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
		partition = "apple-tool:,apple:,codesign:"
	}
	sha := strings.ToLower(strings.TrimSpace(identitySHA))
	if sha != "" && len(sha) != 64 {
		return signingKeychainPasswordOptions{}, shared.UsageError("signing keychain: --identity-sha256 must be 64 hexadecimal characters")
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
	script, err := keychainPasswordScript(action, resolved, password, *options)
	if err != nil {
		return err
	}
	if _, _, err := runKeychainSecurity(ctx, []byte(script), "-i"); err != nil {
		return fmt.Errorf("signing keychain %s: %w", action, err)
	}
	return nil
}

func keychainPasswordScript(action, path string, password []byte, options signingKeychainPasswordOptions) (string, error) {
	quotedPath, err := securityToken(path)
	if err != nil {
		return "", shared.UsageErrorf("signing keychain %s: %s", action, err.Error())
	}
	quotedPassword, err := securityToken(string(password))
	if err != nil {
		return "", shared.UsageErrorf("signing keychain %s: keychain password %s", action, err.Error())
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("unlock-keychain -p %s %s", quotedPassword, quotedPath))
	switch action {
	case "set-keychain-settings":
		if options.NoTimeout {
			lines = append(lines, fmt.Sprintf("set-keychain-settings -u %s", quotedPath))
		} else {
			lines = append(lines, fmt.Sprintf("set-keychain-settings -t %d %s", options.Timeout, quotedPath))
		}
	}
	if options.TimeoutSet || options.NoTimeout {
		if action == "unlock-keychain" {
			if options.NoTimeout {
				lines = append(lines, fmt.Sprintf("set-keychain-settings -u %s", quotedPath))
			} else if options.TimeoutSet {
				lines = append(lines, fmt.Sprintf("set-keychain-settings -t %d %s", options.Timeout, quotedPath))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func unlockKeychainCommand(path string, password []byte) (string, error) {
	quotedPath, err := securityToken(path)
	if err != nil {
		return "", shared.UsageErrorf("signing keychain set-partition-list: %s", err.Error())
	}
	quotedPassword, err := securityToken(string(password))
	if err != nil {
		return "", shared.UsageErrorf("signing keychain set-partition-list: keychain password %s", err.Error())
	}
	return fmt.Sprintf("unlock-keychain -p %s %s\n", quotedPassword, quotedPath), nil
}

func partitionListCommand(path string, password []byte, partition string) (string, error) {
	quotedPath, err := securityToken(path)
	if err != nil {
		return "", shared.UsageErrorf("signing keychain set-partition-list: %s", err.Error())
	}
	quotedPassword, err := securityToken(string(password))
	if err != nil {
		return "", shared.UsageErrorf("signing keychain set-partition-list: keychain password %s", err.Error())
	}
	quotedPartition, err := securityToken(partition)
	if err != nil {
		return "", shared.UsageErrorf("signing keychain set-partition-list: --partition %s", err.Error())
	}
	return fmt.Sprintf("unlock-keychain -p %s %s\nset-key-partition-list -S %s -k %s %s\n", quotedPassword, quotedPath, quotedPartition, quotedPassword, quotedPath), nil
}

func requireKeychainHost(action string) error {
	if keychainHostGOOS == "darwin" {
		return nil
	}
	return shared.NewValidationError(fmt.Errorf("signing keychain %s is supported only on macOS", action))
}

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
	return absolute, nil
}

func refusedKeychainPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "login.keychain", "login.keychain-db", "system.keychain", "system.keychain-db":
		return "refusing to operate on the login or System keychain; pass an explicit dedicated keychain path"
	}
	cleaned := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(cleaned, "/library/keychains/system.keychain") || strings.Contains(cleaned, "/system/library/keychains/") {
		return "refusing to operate on a system keychain"
	}
	return ""
}

func readSigningKeychainPassword(path, action string) ([]byte, error) {
	data, err := readBoundedSigningRunFile(path, signingRunPasswordLimit, true)
	if err != nil {
		return nil, fmt.Errorf("signing keychain %s: read keychain password: %w", action, err)
	}
	password := trimSigningKeychainSecret(data)
	if len(password) == 0 {
		return nil, shared.UsageErrorf("signing keychain %s: keychain password is empty", action)
	}
	if bytes.ContainsAny(password, "\r\n\x00") {
		return nil, shared.UsageErrorf("signing keychain %s: keychain password must not contain line breaks or NUL bytes", action)
	}
	return password, nil
}

func securityToken(value string) (string, error) {
	if strings.ContainsAny(value, "\"\\\r\n\x00") {
		return "", fmt.Errorf("value cannot contain quotes, backslashes, or line breaks")
	}
	return `"` + value + `"`, nil
}

func listSigningKeychains(ctx context.Context) (*asc.SigningKeychainListResult, error) {
	if err := requireKeychainHost("list"); err != nil {
		return nil, err
	}
	stdout, stderr, err := runKeychainSecurity(ctx, nil, "list-keychains", "-d", "user")
	if err != nil {
		return nil, fmt.Errorf("signing keychain list: %w: %s", err, sanitizeSecurityOutput(stderr))
	}
	paths := parseKeychainList(stdout)
	if len(paths) > keychainListLimit {
		return nil, fmt.Errorf("signing keychain list: search list has %d keychains; limit is %d", len(paths), keychainListLimit)
	}
	result := &asc.SigningKeychainListResult{Keychains: make([]asc.SigningKeychainInfo, 0, len(paths))}
	for _, path := range paths {
		info := asc.SigningKeychainInfo{Path: path, InSearchList: true}
		detail, detailErr, infoErr := runKeychainSecurity(ctx, nil, "show-keychain-info", path)
		text := string(append(detail, detailErr...))
		info.Locked = strings.Contains(strings.ToLower(text), "locked")
		if infoErr == nil && !info.Locked {
			identities, err := listKeychainIdentities(ctx, path, true)
			if err != nil {
				return nil, err
			}
			info.Identities = identities
		}
		result.Keychains = append(result.Keychains, info)
	}
	return result, nil
}

func listKeychainIdentities(ctx context.Context, path string, lockedIsEmpty bool) ([]asc.SigningKeychainIdentity, error) {
	stdout, stderr, err := runKeychainSecurity(ctx, nil, "find-certificate", "-a", "-p", "-Z", path)
	if err != nil {
		if lockedIsEmpty && strings.Contains(strings.ToLower(string(stderr)), "locked") {
			return nil, nil
		}
		return nil, fmt.Errorf("signing keychain list: identities for %s: %w", path, err)
	}
	identities := parseCertificateDump(stdout)
	if len(identities) > keychainIdentityLimit {
		return nil, fmt.Errorf("signing keychain list: %s has %d identities; limit is %d", path, len(identities), keychainIdentityLimit)
	}
	return identities, nil
}

func requireKeychainIdentity(ctx context.Context, path, sha256 string) error {
	stdout, stderr, err := runKeychainSecurity(ctx, nil, "find-certificate", "-a", "-p", "-Z", path)
	if err != nil {
		return fmt.Errorf("signing keychain set-partition-list: identities for %s: %w: %s", path, err, sanitizeSecurityOutput(stderr))
	}
	for _, identity := range parseCertificateDump(stdout) {
		if strings.EqualFold(identity.SHA256, sha256) {
			return nil
		}
	}
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SHA-256 hash:") && strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "SHA-256 hash:")), sha256) {
			return nil
		}
	}
	return fmt.Errorf("signing keychain set-partition-list: identity %s was not found", sha256)
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

func parseCertificateDump(output []byte) []asc.SigningKeychainIdentity {
	var identities []asc.SigningKeychainIdentity
	var currentSHA string
	for _, block := range bytes.Split(output, []byte("-----END CERTIFICATE-----")) {
		text := string(block)
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SHA-256 hash:") {
				currentSHA = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "SHA-256 hash:")))
			}
		}
		pemStart := bytes.Index(block, []byte("-----BEGIN CERTIFICATE-----"))
		if pemStart < 0 || currentSHA == "" {
			continue
		}
		pemBytes := append(block[pemStart:], []byte("\n-----END CERTIFICATE-----\n")...)
		decoded, _ := pem.Decode(pemBytes)
		if decoded == nil {
			continue
		}
		certificate, err := x509.ParseCertificate(decoded.Bytes)
		if err != nil {
			continue
		}
		identities = append(identities, asc.SigningKeychainIdentity{
			SHA256:     currentSHA,
			CommonName: certificate.Subject.CommonName,
			ExpiresAt:  certificate.NotAfter.UTC().Format(time.RFC3339),
		})
		currentSHA = ""
	}
	return identities
}

func sanitizeSecurityOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "security failed"
	}
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}
