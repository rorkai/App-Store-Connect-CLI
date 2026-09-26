package signing

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type signingKeychainInstallOptions struct {
	IdentityPath              string
	IdentityPasswordPath      string
	KeychainPath              string
	KeychainPasswordPath      string
	ExpectedCertificateSHA256 string
	AddToSearchList           bool
}

type signingKeychainInstallDeps struct {
	GOOS                      string
	SecurityAvailable         bool
	Now                       func() time.Time
	AcquireLock               func(context.Context) (func() error, error)
	CreateKeychain            func(context.Context, string, []byte) (bool, error)
	ImportIdentity            func(context.Context, string, []byte, []byte, []byte, string) error
	KeychainSearchList        func(context.Context) ([]string, error)
	SetKeychainSearchList     func(context.Context, []string) error
	RemoveKeychainSearchEntry func(context.Context, string) error
	DeleteKeychain            func(context.Context, string) error
}

var (
	installSigningKeychainFn      = executeSigningKeychainInstall
	signingKeychainInstallContext = platformSigningRunContext
)

// SigningKeychainCommand returns the signing keychain command group.
func SigningKeychainCommand() *ffcli.Command {
	fs := flag.NewFlagSet("keychain", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "keychain",
		ShortUsage: "asc signing keychain <subcommand> [flags]",
		ShortHelp:  "Manage dedicated local signing keychains.",
		LongHelp:   "Manage dedicated local signing keychains without changing the default keychain.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SigningKeychainListCommand(),
			SigningKeychainInstallCommand(),
			SigningKeychainUnlockCommand(),
			SigningKeychainLockCommand(),
			SigningKeychainSetTimeoutCommand(),
			SigningKeychainSetPartitionListCommand(),
			SigningKeychainDeleteCommand(),
		},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}
}

// SigningKeychainInstallCommand returns the persistent keychain installer.
func SigningKeychainInstallCommand() *ffcli.Command {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	identityPath := fs.String("identity", "", "Path to one PKCS#12 code-signing identity (required)")
	identityPasswordPath := fs.String("identity-password-file", "", "Protected file containing the PKCS#12 password (required)")
	keychainPath := fs.String("keychain", "", "New dedicated keychain path (required)")
	keychainPasswordPath := fs.String("keychain-password-file", "", "Protected file containing the new keychain password (required)")
	expectedCertificateSHA256 := fs.String("expected-certificate-sha256", "", "Expected identity certificate SHA-256")
	addToSearchList := fs.Bool("add-to-search-list", false, "Append the new keychain to the user search list")
	confirm := fs.Bool("confirm", false, "Confirm persistent keychain creation")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "install",
		ShortUsage: "asc signing keychain install --identity PATH --identity-password-file PATH --keychain PATH --keychain-password-file PATH --confirm [flags]",
		ShortHelp:  "Install one identity in a new persistent keychain.",
		LongHelp: `Create a dedicated persistent keychain and import one code-signing identity.

The destination must not exist. Identity and keychain passwords are read from
protected files and are never placed in command arguments. The command rolls
back the new keychain if import, verification, or search-list activation fails.
Provisioning profiles remain a separate operation under profiles local install.

Examples:
  asc signing keychain install --identity .asc/signing/App.p12 --identity-password-file .asc/secrets/p12-password --keychain .asc/keychains/release.keychain-db --keychain-password-file .asc/secrets/keychain-password --confirm
  asc signing keychain install --identity .asc/signing/App.p12 --identity-password-file .asc/secrets/p12-password --keychain .asc/keychains/release.keychain-db --keychain-password-file .asc/secrets/keychain-password --expected-certificate-sha256 CERTIFICATE_SHA256 --add-to-search-list --confirm --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			identity := strings.TrimSpace(*identityPath)
			if identity == "" {
				return shared.UsageError("--identity is required")
			}
			identityPassword := strings.TrimSpace(*identityPasswordPath)
			if identityPassword == "" {
				return shared.UsageError("--identity-password-file is required")
			}
			keychain := strings.TrimSpace(*keychainPath)
			if keychain == "" {
				return shared.UsageError("--keychain is required")
			}
			keychainPassword := strings.TrimSpace(*keychainPasswordPath)
			if keychainPassword == "" {
				return shared.UsageError("--keychain-password-file is required")
			}
			expectedDigest, err := normalizeOptionalSigningKeychainSHA256(*expectedCertificateSHA256)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				return shared.UsageError("--confirm is required to create a persistent signing keychain")
			}
			installCtx, stopSignals := signingKeychainInstallContext(ctx)
			defer stopSignals()
			result, err := installSigningKeychainFn(installCtx, signingKeychainInstallOptions{
				IdentityPath:              identity,
				IdentityPasswordPath:      identityPassword,
				KeychainPath:              keychain,
				KeychainPasswordPath:      keychainPassword,
				ExpectedCertificateSHA256: expectedDigest,
				AddToSearchList:           *addToSearchList,
			})
			if err != nil {
				return err
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeOptionalSigningKeychainSHA256(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return normalizeSigningRunExpectedSHA256(value, "expected certificate SHA-256")
}

func executeSigningKeychainInstall(ctx context.Context, options signingKeychainInstallOptions) (*asc.SigningKeychainInstallResult, error) {
	return executeSigningKeychainInstallWith(ctx, options, platformSigningKeychainInstallDeps())
}

func executeSigningKeychainInstallWith(ctx context.Context, options signingKeychainInstallOptions, deps signingKeychainInstallDeps) (result *asc.SigningKeychainInstallResult, resultErr error) {
	if deps.GOOS != "darwin" {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install is supported only on macOS"))
	}
	if !deps.SecurityAvailable {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install requires a cgo-enabled macOS build"))
	}
	if ctx == nil {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install: context is required"))
	}
	resolvedKeychainPath, err := preflightSigningKeychainPath(options.KeychainPath)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: preflight destination: %w", err)
	}
	if err := requireSigningKeychainInstallDeps(deps, options.AddToSearchList); err != nil {
		return nil, err
	}
	identityData, err := readBoundedSigningRunFile(options.IdentityPath, signingRunInputLimit, true)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: read identity: %w", err)
	}
	defer clear(identityData)
	identityPasswordData, err := readBoundedSigningRunFile(options.IdentityPasswordPath, signingRunPasswordLimit, true)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: read identity password: %w", err)
	}
	defer clear(identityPasswordData)
	keychainPasswordData, err := readBoundedSigningRunFile(options.KeychainPasswordPath, signingRunPasswordLimit, true)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: read keychain password: %w", err)
	}
	defer clear(keychainPasswordData)

	identityPassword := trimSigningKeychainSecret(identityPasswordData)
	keychainPassword := trimSigningKeychainSecret(keychainPasswordData)
	if len(identityPassword) == 0 {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install: identity password is empty"))
	}
	if len(keychainPassword) == 0 {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install: keychain password is empty"))
	}
	if bytes.ContainsAny(keychainPassword, "\r\n\x00") {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install: keychain password must not contain line breaks or NUL bytes"))
	}

	now := time.Now
	if deps.Now != nil {
		now = deps.Now
	}
	identity, err := inspectSigningRunIdentity(identityData, identityPassword, now())
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: inspect identity: %w", err)
	}
	if err := validateSigningKeychainCertificate(identity.Certificate); err != nil {
		return nil, fmt.Errorf("signing keychain install: inspect identity: %w", err)
	}
	if options.ExpectedCertificateSHA256 != "" && !strings.EqualFold(options.ExpectedCertificateSHA256, identity.CertificateSHA256) {
		return nil, shared.NewValidationError(fmt.Errorf("signing keychain install: identity certificate does not match --expected-certificate-sha256"))
	}
	teamID, err := signingRunCertificateTeamID(identity.Certificate)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: inspect identity: %w", err)
	}
	unlock, err := deps.AcquireLock(ctx)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: acquire signing environment lock: %w", err)
	}
	defer func() {
		if unlock == nil {
			return
		}
		if err := unlock(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("signing keychain install: release signing environment lock: %w", err))
		}
	}()
	originalSearchList, err := deps.KeychainSearchList(ctx)
	if err != nil {
		return nil, fmt.Errorf("signing keychain install: read keychain search list: %w", err)
	}
	originalSearchList = append([]string(nil), originalSearchList...)
	searchListHadPath := slices.Contains(originalSearchList, resolvedKeychainPath)

	created := false
	rollback := func(primary error) error {
		if !created {
			return primary
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var cleanupErr error
		if err := deps.DeleteKeychain(cleanupCtx, resolvedKeychainPath); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete keychain: %w", err))
		}
		if err := deps.SetKeychainSearchList(cleanupCtx, originalSearchList); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("restore keychain search list: %w", err))
		}
		if cleanupErr != nil {
			return errors.Join(primary, fmt.Errorf("signing keychain install: rollback new keychain: %w", cleanupErr))
		}
		return primary
	}
	created, err = deps.CreateKeychain(ctx, resolvedKeychainPath, keychainPassword)
	if err != nil {
		return nil, rollback(fmt.Errorf("signing keychain install: create keychain: %w", err))
	}
	removedSearchEntry := false
	if !options.AddToSearchList && searchListHadPath {
		if err := deps.RemoveKeychainSearchEntry(ctx, resolvedKeychainPath); err != nil {
			return nil, rollback(fmt.Errorf("signing keychain install: isolate keychain: %w", err))
		}
		removedSearchEntry = true
	}
	if !searchListHadPath || removedSearchEntry {
		paths := make([]string, 0, len(originalSearchList)+1)
		staged := false
		for _, path := range originalSearchList {
			if path == resolvedKeychainPath {
				if !staged {
					paths = append(paths, resolvedKeychainPath)
					staged = true
				}
				continue
			}
			paths = append(paths, path)
		}
		if !staged {
			paths = append(paths, resolvedKeychainPath)
		}
		if err := deps.SetKeychainSearchList(ctx, paths); err != nil {
			return nil, rollback(fmt.Errorf("signing keychain install: stage keychain search list: %w", err))
		}
	}
	if err := deps.ImportIdentity(ctx, resolvedKeychainPath, keychainPassword, identityData, identityPassword, identity.CertificateSHA1); err != nil {
		return nil, rollback(fmt.Errorf("signing keychain install: import identity: %w", err))
	}

	searchListUpdated := options.AddToSearchList && !searchListHadPath
	if !options.AddToSearchList {
		if err := deps.RemoveKeychainSearchEntry(ctx, resolvedKeychainPath); err != nil {
			return nil, rollback(fmt.Errorf("signing keychain install: isolate keychain: %w", err))
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, rollback(fmt.Errorf("signing keychain install: %w", err))
	}

	return &asc.SigningKeychainInstallResult{
		Action:            "installed",
		KeychainPath:      resolvedKeychainPath,
		CertificateSHA256: identity.CertificateSHA256,
		CertificateSHA1:   identity.CertificateSHA1,
		TeamID:            teamID,
		SearchListUpdated: searchListUpdated,
	}, nil
}

func requireSigningKeychainInstallDeps(deps signingKeychainInstallDeps, addToSearchList bool) error {
	if deps.AcquireLock == nil || deps.CreateKeychain == nil || deps.ImportIdentity == nil || deps.DeleteKeychain == nil {
		return fmt.Errorf("signing keychain install: platform keychain operations are unavailable")
	}
	if deps.KeychainSearchList == nil || deps.SetKeychainSearchList == nil {
		return fmt.Errorf("signing keychain install: platform search-list operations are unavailable")
	}
	if !addToSearchList && deps.RemoveKeychainSearchEntry == nil {
		return fmt.Errorf("signing keychain install: platform search-list isolation is unavailable")
	}
	return nil
}

func preflightSigningKeychainPath(path string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	parent := filepath.Dir(absolute)
	physicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("resolve keychain parent: %w", err)
	}
	absolute = filepath.Join(physicalParent, filepath.Base(absolute))
	root, err := rootfs.New(physicalParent)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := root.CheckCreateNewFile(filepath.Base(absolute)); err != nil {
		return "", err
	}
	return absolute, nil
}

func trimSigningKeychainSecret(data []byte) []byte {
	trimmed := bytes.TrimSuffix(data, []byte("\n"))
	return bytes.TrimSuffix(trimmed, []byte("\r"))
}

func validateSigningKeychainCertificate(certificate *x509.Certificate) error {
	if certificate == nil {
		return fmt.Errorf("identity certificate is missing")
	}
	if certificate.KeyUsage != 0 && certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return fmt.Errorf("identity certificate does not permit digital signatures")
	}
	if len(certificate.ExtKeyUsage) == 0 {
		return nil
	}
	for _, usage := range certificate.ExtKeyUsage {
		if usage == x509.ExtKeyUsageCodeSigning || usage == x509.ExtKeyUsageAny {
			return nil
		}
	}
	return fmt.Errorf("identity certificate is not valid for code signing")
}
