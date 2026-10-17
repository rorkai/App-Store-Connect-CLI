package signing

import (
	"bytes"
	"context"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/bitrise-io/go-pkcs12"
	"github.com/peterbourgon/ff/v3/ffcli"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
)

const (
	signingRunPurposeReleaseTesting = "release-testing"
	signingRunInputLimit            = 16 << 20
	signingRunPasswordLimit         = 64 << 10
)

// ErrEphemeralRecoveryJournalInvalid means the retained signing recovery
// journal is structurally unsafe and cannot be repaired by retrying.
var ErrEphemeralRecoveryJournalInvalid = errors.New("invalid ephemeral signing recovery journal")

type signingRunOptions struct {
	IdentityPath              string
	IdentityPasswordPath      string
	ProfilePath               string
	Purpose                   string
	ReceiptPath               string
	Child                     []string
	ExpectedCertificateSHA256 string
	ExpectedProfileSHA256     string
}

// EphemeralRunOptions identifies the private signing inputs used while an
// in-process operation runs. Release testing is the only supported purpose.
type EphemeralRunOptions struct {
	IdentityPath              string
	IdentityPasswordPath      string
	ProfilePath               string
	ReceiptPath               string
	ExpectedCertificateSHA256 string
	ExpectedProfileSHA256     string
}

// PKCS12IdentityOptions identifies a private PKCS#12 identity and its optional
// password file for read-only inspection.
type PKCS12IdentityOptions struct {
	IdentityPath         string
	IdentityPasswordPath string
}

// PKCS12IdentityInfo contains only public certificate metadata. It never
// exposes the source path, password, certificate bytes, or private key.
type PKCS12IdentityInfo struct {
	CertificateSHA256 string    `json:"certificateSha256"`
	CertificateSHA1   string    `json:"certificateSha1"`
	TeamID            string    `json:"teamId"`
	NotBefore         time.Time `json:"notBefore"`
	NotAfter          time.Time `json:"notAfter"`
}

type signingRunIdentity struct {
	Certificate       *x509.Certificate
	PrivateKey        crypto.PrivateKey
	CertificateSHA1   string
	CertificateSHA256 string
}

type signingRunInspection struct {
	ProfileUUID        string
	TeamID             string
	BundleID           string
	ProvisionedDevices []string
	CertificateSHA1    string
	CertificateSHA256  string
	ProfileSHA256      string
	Certificate        *x509.Certificate
	PrivateKey         crypto.PrivateKey
}

type signingRunProfileInstall struct {
	Path         string
	StagedPath   string
	Created      bool
	Digest       string
	StagedDigest string
	Device       uint64
	Inode        uint64
}

type signingRunDeps struct {
	GOOS                      string
	Stderr                    io.Writer
	RandomBytes               func(int) ([]byte, error)
	TempDir                   func() (string, error)
	RemoveTempDir             func(string) error
	AcquireLock               func(context.Context) (func() error, error)
	Recover                   func(context.Context) error
	WriteJournal              func(signingRunJournal, bool) error
	RemoveJournal             func() error
	KeychainSearchList        func(context.Context) ([]string, error)
	CreateKeychain            func(context.Context, string, []byte) error
	ImportIdentity            func(context.Context, string, []byte, []byte, []byte, string) error
	SetKeychainSearchList     func(context.Context, []string) error
	RemoveKeychainSearchEntry func(context.Context, string) error
	DeleteKeychain            func(context.Context, string) error
	InstallProfile            func(context.Context, string, []byte, string, func(signingRunProfileInstall) error) (signingRunProfileInstall, error)
	RemoveProfile             func(signingRunProfileInstall) error
	RunChild                  func(context.Context, []string) error
}

type signingRunJournal struct {
	SchemaVersion       int    `json:"schemaVersion"`
	TempDir             string `json:"tempDir"`
	KeychainPath        string `json:"keychainPath"`
	ProfilePath         string `json:"profilePath,omitempty"`
	StagedProfilePath   string `json:"stagedProfilePath,omitempty"`
	ProfileDigest       string `json:"profileDigest,omitempty"`
	StagedProfileDigest string `json:"stagedProfileDigest,omitempty"`
	ProfileDevice       uint64 `json:"profileDevice,omitempty"`
	ProfileInode        uint64 `json:"profileInode,omitempty"`
	ProfileCreated      bool   `json:"profileCreated,omitempty"`
}

type signingRunMobileProvision struct {
	UUID                        string         `plist:"UUID"`
	TeamIdentifier              []string       `plist:"TeamIdentifier"`
	ApplicationIdentifierPrefix []string       `plist:"ApplicationIdentifierPrefix"`
	Platform                    []string       `plist:"Platform"`
	ProvisionedDevices          []string       `plist:"ProvisionedDevices"`
	ProvisionsAllDevices        bool           `plist:"ProvisionsAllDevices"`
	CreationDate                time.Time      `plist:"CreationDate"`
	ExpirationDate              time.Time      `plist:"ExpirationDate"`
	Entitlements                map[string]any `plist:"Entitlements"`
	DeveloperCertificates       [][]byte       `plist:"DeveloperCertificates"`
}

type signingRunReceipt struct {
	SchemaVersion        int    `json:"schemaVersion"`
	Purpose              string `json:"purpose"`
	Outcome              string `json:"outcome"`
	ChildExitCode        int    `json:"childExitCode"`
	CertificateSHA256    string `json:"certificateSha256"`
	ProfileSHA256        string `json:"profileSha256"`
	ProfileUUID          string `json:"profileUuid"`
	TeamID               string `json:"teamId"`
	BundleID             string `json:"bundleId"`
	ProfileCleanupState  string `json:"profileCleanupState"`
	KeychainCleanupState string `json:"keychainCleanupState"`
}

var signingRunUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var (
	executeSigningRunFn       = executeSigningRun
	executeSigningOperationFn = executeSigningOperation
	signingRunSystemRootsFn   = systemSigningRunRoots
	signingRunProfileTrustFn  = verifySigningRunProfileTrust
	signingRunEnvironmentFn   = runSigningEnvironment
	signingRunNowFn           = time.Now
)

var sanitizedSigningChildEnvironmentNames = map[string]struct{}{
	"DEVELOPER_DIR": {},
	"HOME":          {},
	"LANG":          {},
	"LC_ALL":        {},
	"LC_CTYPE":      {},
	"PATH":          {},
	"SDKROOT":       {},
	"TEMP":          {},
	"TMP":           {},
	"TMPDIR":        {},
	"TOOLCHAINS":    {},
	"TZ":            {},
}

// SanitizedChildEnvironment returns the strict environment allowlist for
// subprocesses started inside an ephemeral signing environment. Authentication,
// cloud-provider, Git-helper, signing-password, loader, and unrecognized
// variables are intentionally excluded.
func SanitizedChildEnvironment(base []string) []string {
	filtered := make([]string, 0, len(sanitizedSigningChildEnvironmentNames))
	indexes := make(map[string]int, len(sanitizedSigningChildEnvironmentNames))
	for _, entry := range base {
		if strings.ContainsRune(entry, '\x00') {
			continue
		}
		name, _, found := strings.Cut(entry, "=")
		if !found || name == "" {
			continue
		}
		if _, allowed := sanitizedSigningChildEnvironmentNames[name]; allowed {
			cloned := strings.Clone(entry)
			if index, duplicate := indexes[name]; duplicate {
				filtered[index] = cloned
				continue
			}
			indexes[name] = len(filtered)
			filtered = append(filtered, cloned)
		}
	}
	return filtered
}

// RunEphemeral runs callback once inside the same audited ephemeral signing
// boundary used by `asc signing run`. The callback must be synchronous and
// context-aware; any subprocess it starts must use SanitizedChildEnvironment.
func RunEphemeral(ctx context.Context, options EphemeralRunOptions, callback func(context.Context) error) error {
	if strings.TrimSpace(options.IdentityPath) == "" {
		return shared.NewValidationError(fmt.Errorf("signing run: identity path is required"))
	}
	if strings.TrimSpace(options.ProfilePath) == "" {
		return shared.NewValidationError(fmt.Errorf("signing run: profile path is required"))
	}
	expectedCertificateSHA256, err := normalizeSigningRunExpectedSHA256(
		options.ExpectedCertificateSHA256, "expected certificate SHA-256",
	)
	if err != nil {
		return shared.NewValidationError(fmt.Errorf("signing run: %w", err))
	}
	expectedProfileSHA256, err := normalizeSigningRunExpectedSHA256(
		options.ExpectedProfileSHA256, "expected profile SHA-256",
	)
	if err != nil {
		return shared.NewValidationError(fmt.Errorf("signing run: %w", err))
	}
	if callback == nil {
		return shared.NewValidationError(fmt.Errorf("signing run: callback is required"))
	}
	return executeSigningOperationFn(ctx, signingRunOptions{
		IdentityPath:              options.IdentityPath,
		IdentityPasswordPath:      options.IdentityPasswordPath,
		ProfilePath:               options.ProfilePath,
		Purpose:                   signingRunPurposeReleaseTesting,
		ReceiptPath:               options.ReceiptPath,
		ExpectedCertificateSHA256: expectedCertificateSHA256,
		ExpectedProfileSHA256:     expectedProfileSHA256,
	}, callback)
}

// RecoverEphemeral serializes with every ephemeral signing run and performs
// only validated crash-journal recovery. It reads no identity or profile and
// creates no keychain or provisioning profile when no journal exists.
func RecoverEphemeral(ctx context.Context) error {
	if ctx == nil {
		return shared.NewValidationError(fmt.Errorf("recover signing environment: context is required"))
	}
	deps := platformSigningRunDeps()
	if deps.GOOS != "darwin" {
		return shared.NewValidationError(fmt.Errorf("recover signing environment is supported only on macOS"))
	}
	if !signingRunSecurityAvailable() {
		return shared.NewValidationError(fmt.Errorf("recover signing environment requires a cgo-enabled macOS build"))
	}
	return recoverEphemeralWith(ctx, deps)
}

func recoverEphemeralWith(ctx context.Context, deps signingRunDeps) (resultErr error) {
	if deps.AcquireLock == nil || deps.Recover == nil {
		return fmt.Errorf("recover signing environment: platform recovery is unavailable")
	}
	unlock, err := deps.AcquireLock(ctx)
	if err != nil {
		return fmt.Errorf("recover signing environment: acquire signing environment lock: %w", err)
	}
	if unlock == nil {
		return fmt.Errorf("recover signing environment: signing environment lock returned no release function")
	}
	defer func() {
		if err := unlock(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("recover signing environment: release signing environment lock: %w", err))
		}
	}()
	if err := deps.Recover(ctx); err != nil {
		return fmt.Errorf("recover signing environment: recover prior signing environment: %w", err)
	}
	return nil
}

func normalizeSigningRunExpectedSHA256(value, label string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("%s must be a 64-character hexadecimal digest", label)
	}
	return normalized, nil
}

// InspectPKCS12Identity securely reads and validates one PKCS#12 identity and
// returns only public leaf-certificate metadata. The certificate must be
// currently valid and its private key must cryptographically match.
func InspectPKCS12Identity(ctx context.Context, options PKCS12IdentityOptions) (PKCS12IdentityInfo, error) {
	if strings.TrimSpace(options.IdentityPath) == "" {
		return PKCS12IdentityInfo{}, shared.NewValidationError(fmt.Errorf("inspect PKCS#12 identity: identity path is required"))
	}
	if ctx == nil {
		return PKCS12IdentityInfo{}, shared.NewValidationError(fmt.Errorf("inspect PKCS#12 identity: context is required"))
	}
	if err := ctx.Err(); err != nil {
		return PKCS12IdentityInfo{}, err
	}
	if platformSigningRunDeps().GOOS != "darwin" {
		return PKCS12IdentityInfo{}, shared.NewValidationError(fmt.Errorf("inspect PKCS#12 identity is supported only on macOS"))
	}
	identityData, err := readBoundedSigningRunFile(options.IdentityPath, signingRunInputLimit, true)
	if err != nil {
		return PKCS12IdentityInfo{}, fmt.Errorf("inspect PKCS#12 identity: read identity: %w", err)
	}
	defer clear(identityData)
	var passwordData []byte
	if options.IdentityPasswordPath != "" {
		passwordData, err = readBoundedSigningRunFile(options.IdentityPasswordPath, signingRunPasswordLimit, true)
		if err != nil {
			return PKCS12IdentityInfo{}, fmt.Errorf("inspect PKCS#12 identity: read identity password: %w", err)
		}
	}
	defer clear(passwordData)
	if err := ctx.Err(); err != nil {
		return PKCS12IdentityInfo{}, err
	}
	identityPassword := bytes.TrimSuffix(passwordData, []byte("\n"))
	identityPassword = bytes.TrimSuffix(identityPassword, []byte("\r"))
	identity, err := inspectSigningRunIdentity(identityData, identityPassword, signingRunNowFn())
	if err != nil {
		return PKCS12IdentityInfo{}, fmt.Errorf("inspect PKCS#12 identity: %w", err)
	}
	teamID, err := signingRunCertificateTeamID(identity.Certificate)
	if err != nil {
		return PKCS12IdentityInfo{}, fmt.Errorf("inspect PKCS#12 identity: %w", err)
	}
	return PKCS12IdentityInfo{
		CertificateSHA256: identity.CertificateSHA256,
		CertificateSHA1:   identity.CertificateSHA1,
		TeamID:            teamID,
		NotBefore:         identity.Certificate.NotBefore,
		NotAfter:          identity.Certificate.NotAfter,
	}, nil
}

// SigningRunCommand returns the signing run subcommand.
func SigningRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("run", flag.ExitOnError)

	identityPath := fs.String("identity", "", "Path to a PKCS#12 distribution identity")
	identityPasswordPath := fs.String("identity-password-file", "", "Path to a file containing the PKCS#12 password")
	profilePath := fs.String("profile", "", "Path to the provisioning profile to install for the child")
	purpose := fs.String("purpose", signingRunPurposeReleaseTesting, "Signing purpose (release-testing)")
	receiptPath := fs.String("receipt", "", "Write a redacted JSON execution receipt to this path")

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc signing run --identity PATH --profile PATH [flags] -- <command> [args...]",
		ShortHelp:  "Run one command with an ephemeral signing identity.",
		LongHelp: `Run one command with an ephemeral signing identity and provisioning profile.

The identity is imported into a dedicated temporary keychain and is available
only while the child command runs. The child is executed directly without a
shell and inherits stdin, stdout, and stderr. Put the child command after --.

Examples:
  asc signing run --identity .asc/signing/App.p12 --profile .asc/signing/App.mobileprovision -- xcodebuild -exportArchive
  asc signing run --identity .asc/signing/App.p12 --identity-password-file .asc/secrets/p12-password --profile .asc/signing/App.mobileprovision --receipt .asc/distribution/signing-run.json -- xcodebuild -exportArchive`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			identity := strings.TrimSpace(*identityPath)
			if identity == "" {
				return shared.UsageError("--identity is required")
			}
			profile := strings.TrimSpace(*profilePath)
			if profile == "" {
				return shared.UsageError("--profile is required")
			}
			purposeValue := strings.TrimSpace(*purpose)
			if purposeValue != signingRunPurposeReleaseTesting {
				return shared.UsageError(`--purpose must be "release-testing"`)
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return shared.UsageError("a child command is required after --")
			}

			return classifySigningInputError(executeSigningRunFn(ctx, signingRunOptions{
				IdentityPath:         identity,
				IdentityPasswordPath: strings.TrimSpace(*identityPasswordPath),
				ProfilePath:          profile,
				Purpose:              purposeValue,
				ReceiptPath:          strings.TrimSpace(*receiptPath),
				Child:                append([]string(nil), args...),
			}))
		},
	}
}

func executeSigningRun(ctx context.Context, options signingRunOptions) error {
	return executeSigningOperation(ctx, options, func(runCtx context.Context) error {
		return runSigningRunChild(runCtx, options.Child)
	})
}

func executeSigningOperation(ctx context.Context, options signingRunOptions, operation func(context.Context) error) error {
	if ctx == nil {
		return shared.NewValidationError(fmt.Errorf("signing run: context is required"))
	}
	if operation == nil {
		return shared.NewValidationError(fmt.Errorf("signing run: callback is required"))
	}
	deps := platformSigningRunDeps()
	if deps.GOOS != "darwin" {
		return shared.NewValidationError(fmt.Errorf("signing run is supported only on macOS"))
	}
	if !signingRunSecurityAvailable() {
		return shared.NewValidationError(fmt.Errorf("signing run requires a cgo-enabled macOS build"))
	}
	return withSigningRunInputData(options, readBoundedSigningRunFile, func(identityData, identityPassword, profileData []byte) error {
		roots, err := signingRunSystemRootsFn()
		if err != nil {
			return fmt.Errorf("signing run: load system certificate roots: %w", err)
		}
		inspection, err := inspectSigningRunInputs(identityData, identityPassword, profileData, roots, signingRunNowFn())
		if err != nil {
			return fmt.Errorf("signing run: preflight: %w", err)
		}
		if err := validateSigningRunExpectedDigests(options, inspection); err != nil {
			return fmt.Errorf("signing run: preflight: %w", err)
		}

		runCtx, stopSignals := platformSigningRunContext(ctx)
		defer stopSignals()
		return withSigningRunReceipt(deps.Stderr, options.ReceiptPath, func() (signingRunReceipt, error) {
			return signingRunEnvironmentFn(runCtx, deps, options, profileData, inspection, operation)
		})
	})
}

func withSigningRunReceipt(
	stderr io.Writer,
	path string,
	run func() (signingRunReceipt, error),
) error {
	if err := preflightSigningRunReceipt(path); err != nil {
		return fmt.Errorf("signing run: preflight receipt: %w", err)
	}
	receipt, runErr := run()
	return finishSigningRunReceipt(stderr, path, receipt, runErr)
}

func preflightSigningRunReceipt(path string) error {
	if path == "" {
		return nil
	}
	preflightComplete := errors.New("receipt preflight complete")
	_, err := shared.SafeWriteFileNoSymlink(
		path,
		0o600,
		false,
		".asc-signing-run-receipt-*",
		".asc-signing-run-receipt-backup-*",
		func(*os.File) (int64, error) {
			return 0, preflightComplete
		},
	)
	if errors.Is(err, preflightComplete) {
		return nil
	}
	return err
}

func validateSigningRunExpectedDigests(options signingRunOptions, inspection *signingRunInspection) error {
	if inspection == nil {
		return fmt.Errorf("signing inspection is missing")
	}
	if options.ExpectedCertificateSHA256 != "" &&
		!strings.EqualFold(options.ExpectedCertificateSHA256, inspection.CertificateSHA256) {
		return fmt.Errorf("identity certificate changed after planning")
	}
	if options.ExpectedProfileSHA256 != "" &&
		!strings.EqualFold(options.ExpectedProfileSHA256, inspection.ProfileSHA256) {
		return fmt.Errorf("provisioning profile changed after reconciliation")
	}
	return nil
}

func withSigningRunInputData(
	options signingRunOptions,
	readFile func(string, int64, bool) ([]byte, error),
	operation func(identityData, identityPassword, profileData []byte) error,
) error {
	identityData, err := readFile(options.IdentityPath, signingRunInputLimit, true)
	if err != nil {
		return signingInputReadError("--identity", fmt.Errorf("signing run: read identity: %w", err))
	}
	defer clear(identityData)
	profileData, err := readFile(options.ProfilePath, signingRunInputLimit, false)
	if err != nil {
		return signingInputReadError("--profile", fmt.Errorf("signing run: read profile: %w", err))
	}
	defer clear(profileData)
	var passwordData []byte
	if options.IdentityPasswordPath != "" {
		passwordData, err = readFile(options.IdentityPasswordPath, signingRunPasswordLimit, true)
		if err != nil {
			return signingInputReadError("--identity-password-file", fmt.Errorf("signing run: read identity password: %w", err))
		}
	}
	defer clear(passwordData)
	identityPassword := bytes.TrimSuffix(passwordData, []byte("\n"))
	identityPassword = bytes.TrimSuffix(identityPassword, []byte("\r"))
	return operation(identityData, identityPassword, profileData)
}

func finishSigningRunReceipt(stderr io.Writer, path string, receipt signingRunReceipt, runErr error) error {
	if path == "" {
		return runErr
	}
	data, err := marshalSigningRunReceipt(receipt)
	if err == nil {
		_, err = shared.SafeWriteFileNoSymlink(
			path,
			0o600,
			false,
			".asc-signing-run-receipt-*",
			".asc-signing-run-receipt-backup-*",
			func(file *os.File) (int64, error) {
				written, writeErr := file.Write(data)
				return int64(written), writeErr
			},
		)
	}
	if err == nil {
		return runErr
	}
	return joinSigningRunCompanionError(stderr, runErr, fmt.Errorf("signing run: write receipt: %w", err))
}

func joinSigningRunCompanionError(stderr io.Writer, primary, companion error) error {
	if companion == nil {
		return primary
	}
	if _, ok := shared.ProcessExitCode(primary); !ok {
		// Neither error has been rendered. Leave both unreported so the root
		// command prints the complete joined diagnostic once.
		return errors.Join(primary, companion)
	}
	// The child owns stderr and its exact-exit marker tells the root command not
	// to duplicate it. Render every independent companion before joining it;
	// otherwise that marker suppresses the whole composite.
	if stderr == nil {
		stderr = io.Discard
	}
	fmt.Fprint(stderr, errfmt.FormatStderr(companion))
	return errors.Join(primary, shared.NewStderrReportedError(companion))
}

func readBoundedSigningRunFile(path string, limit int64, private bool) ([]byte, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, signingInputCheckErrorf(errSigningInputUnsupported, "%q is not a regular file", path)
	}
	if err := validateSigningRunInputPermissions(path, info, private); err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, signingInputCheckErrorf(errSigningInputUnsupported, "%q exceeds the %d-byte limit", path, limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, signingInputCheckErrorf(errSigningInputUnsupported, "%q exceeds the %d-byte limit", path, limit)
	}
	return data, nil
}

func runSigningEnvironment(
	ctx context.Context,
	deps signingRunDeps,
	options signingRunOptions,
	profileData []byte,
	inspection *signingRunInspection,
	operation func(context.Context) error,
) (receipt signingRunReceipt, resultErr error) {
	if ctx == nil {
		return signingRunReceipt{}, fmt.Errorf("signing run context is required")
	}
	if operation == nil && deps.RunChild != nil {
		operation = func(runCtx context.Context) error {
			return deps.RunChild(runCtx, options.Child)
		}
	}
	if operation == nil {
		return signingRunReceipt{}, fmt.Errorf("signing run operation is required")
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	receipt = signingRunReceipt{
		SchemaVersion:        1,
		Purpose:              options.Purpose,
		Outcome:              "failed",
		CertificateSHA256:    inspection.CertificateSHA256,
		ProfileSHA256:        inspection.ProfileSHA256,
		ProfileUUID:          inspection.ProfileUUID,
		TeamID:               inspection.TeamID,
		BundleID:             inspection.BundleID,
		ProfileCleanupState:  "not-installed",
		KeychainCleanupState: "not-created",
	}
	unlock, err := deps.AcquireLock(ctx)
	if err != nil {
		return receipt, fmt.Errorf("acquire signing environment lock: %w", err)
	}
	if unlock == nil {
		return receipt, fmt.Errorf("signing environment lock returned no release function")
	}
	defer func() {
		panicValue := recover()
		if err := unlock(); err != nil {
			unlockErr := fmt.Errorf("release signing environment lock: %w", err)
			if panicValue != nil {
				fmt.Fprint(deps.Stderr, errfmt.FormatStderr(unlockErr))
			} else {
				resultErr = joinSigningRunCompanionError(
					deps.Stderr,
					resultErr,
					unlockErr,
				)
			}
		}
		if panicValue != nil {
			panic(panicValue)
		}
	}()
	if err := deps.Recover(ctx); err != nil {
		return receipt, fmt.Errorf("recover prior signing environment: %w", err)
	}

	tempDir, err := deps.TempDir()
	if err != nil {
		return receipt, fmt.Errorf("create private signing directory: %w", err)
	}
	cleanupSetupFailure := func(primary error) error {
		if deps.RemoveTempDir == nil {
			return errors.Join(primary, fmt.Errorf("remove private signing directory: cleanup function is unavailable"))
		}
		if cleanupErr := deps.RemoveTempDir(tempDir); cleanupErr != nil {
			return errors.Join(primary, fmt.Errorf("remove private signing directory: %w", cleanupErr))
		}
		return primary
	}
	keychainPath := filepath.Join(tempDir, "signing.keychain-db")
	_, err = deps.KeychainSearchList(ctx)
	if err != nil {
		return receipt, cleanupSetupFailure(fmt.Errorf("read user keychain search list: %w", err))
	}
	keychainPassword, err := deps.RandomBytes(32)
	if err != nil {
		return receipt, cleanupSetupFailure(fmt.Errorf("generate keychain password: %w", err))
	}
	defer clear(keychainPassword)
	importPassword, err := deps.RandomBytes(32)
	if err != nil {
		return receipt, cleanupSetupFailure(fmt.Errorf("generate identity import password: %w", err))
	}
	importPasswordText := []byte(hex.EncodeToString(importPassword))
	clear(importPassword)
	normalizedIdentity, err := pkcs12.Encode(
		cryptorand.Reader,
		inspection.PrivateKey,
		inspection.Certificate,
		nil,
		string(importPasswordText),
	)
	if err != nil {
		clear(importPasswordText)
		return receipt, cleanupSetupFailure(fmt.Errorf("normalize identity for temporary import: %w", err))
	}
	defer clear(importPasswordText)
	defer clear(normalizedIdentity)
	journal := signingRunJournal{SchemaVersion: 1, TempDir: tempDir, KeychainPath: keychainPath}
	if err := deps.WriteJournal(journal, false); err != nil {
		return receipt, cleanupSetupFailure(fmt.Errorf("write signing environment recovery journal: %w", err))
	}

	keychainAttempted := false
	profileInstall := signingRunProfileInstall{}
	cleanup := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var cleanupErr error
		if profileInstall.Created || profileInstall.StagedPath != "" {
			if err := deps.RemoveProfile(profileInstall); err != nil {
				receipt.ProfileCleanupState = "failed"
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove installed provisioning profile: %w", err))
			} else {
				receipt.ProfileCleanupState = "removed"
			}
		}
		if keychainAttempted {
			if err := deps.RemoveKeychainSearchEntry(cleanupCtx, keychainPath); err != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove temporary keychain from search list: %w", err))
			}
			if err := deps.DeleteKeychain(cleanupCtx, keychainPath); err != nil {
				receipt.KeychainCleanupState = "failed"
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete temporary keychain: %w", err))
			} else {
				receipt.KeychainCleanupState = "deleted"
			}
		}
		if cleanupErr != nil {
			return cleanupErr
		}
		if err := deps.RemoveTempDir(tempDir); err != nil {
			return fmt.Errorf("remove private signing directory: %w", err)
		}
		if err := deps.RemoveJournal(); err != nil {
			return fmt.Errorf("remove signing environment recovery journal: %w", err)
		}
		return nil
	}
	finish := func(primary error) (signingRunReceipt, error) {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("signing run cleanup did not complete; the next run will retry recovery: %w", cleanupErr)
		}
		return receipt, joinSigningRunCompanionError(deps.Stderr, primary, cleanupErr)
	}
	defer func() {
		panicValue := recover()
		if panicValue == nil {
			return
		}
		if cleanupErr := cleanup(); cleanupErr != nil {
			fmt.Fprint(deps.Stderr, errfmt.FormatStderr(fmt.Errorf(
				"signing run cleanup did not complete after callback panic; the next run will retry recovery: %w",
				cleanupErr,
			)))
		}
		panic(panicValue)
	}()

	keychainAttempted = true
	receipt.KeychainCleanupState = "pending"
	if err := deps.CreateKeychain(ctx, keychainPath, keychainPassword); err != nil {
		return finish(fmt.Errorf("create temporary keychain: %w", err))
	}
	// Some macOS releases add newly created keychains to the user search list.
	// Remove it immediately; the command adds it back only for the child window.
	if err := deps.RemoveKeychainSearchEntry(ctx, keychainPath); err != nil {
		return finish(fmt.Errorf("isolate temporary keychain: %w", err))
	}
	if err := deps.ImportIdentity(ctx, keychainPath, keychainPassword, normalizedIdentity, importPasswordText, inspection.CertificateSHA1); err != nil {
		return finish(fmt.Errorf("import identity into temporary keychain: %w", err))
	}
	installedProfile, err := deps.InstallProfile(ctx, inspection.ProfileUUID, profileData, inspection.ProfileSHA256, func(planned signingRunProfileInstall) error {
		// Keep the ownership proof in memory even if the installer reports a
		// later error and returns a zero value. Cleanup must not depend on the
		// installer preserving its result on failure.
		profileInstall = planned
		journal.ProfilePath = planned.Path
		journal.StagedProfilePath = planned.StagedPath
		journal.ProfileDigest = planned.Digest
		journal.StagedProfileDigest = planned.StagedDigest
		journal.ProfileDevice = planned.Device
		journal.ProfileInode = planned.Inode
		journal.ProfileCreated = true
		if err := deps.WriteJournal(journal, true); err != nil {
			// The initial journal is already durable, but retry this overwrite
			// once so a cleanup failure retains the staged profile coordinates.
			retryErr := deps.WriteJournal(journal, true)
			return errors.Join(err, retryErr)
		}
		return nil
	})
	if err != nil {
		if installedProfile.Created || installedProfile.StagedPath != "" {
			profileInstall = installedProfile
		}
		return finish(fmt.Errorf("install provisioning profile: %w", err))
	}
	profileInstall = installedProfile
	if profileInstall.Created {
		receipt.ProfileCleanupState = "pending"
	} else {
		receipt.ProfileCleanupState = "reused"
	}

	currentSearchList, err := deps.KeychainSearchList(ctx)
	if err != nil {
		return finish(fmt.Errorf("refresh user keychain search list: %w", err))
	}
	expectedSearchList := []string{keychainPath}
	for _, path := range currentSearchList {
		if path != keychainPath {
			expectedSearchList = append(expectedSearchList, path)
		}
	}
	if err := deps.SetKeychainSearchList(ctx, expectedSearchList); err != nil {
		return finish(fmt.Errorf("activate temporary keychain: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return finish(err)
	}
	childErr := operation(ctx)
	if childErr != nil {
		receipt.ChildExitCode = childExitCode(childErr)
		return finish(childErr)
	}
	receipt.Outcome = "succeeded"
	return finish(nil)
}

func inspectSigningRunInputs(identityData, identityPassword, profileData []byte, roots *x509.CertPool, now time.Time) (*signingRunInspection, error) {
	identity, err := inspectSigningRunIdentity(identityData, identityPassword, now)
	if err != nil {
		return nil, err
	}
	certificate := identity.Certificate

	p7, err := pkcs7.Parse(profileData)
	if err != nil {
		return nil, signingInputDecodeError("--profile", fmt.Errorf("decode signed provisioning profile: %w", err))
	}
	if roots == nil {
		return nil, fmt.Errorf("load trusted roots for provisioning profile verification")
	}
	if err := signingRunProfileTrustFn(p7, roots, now); err != nil {
		return nil, fmt.Errorf("verify profile signature: %w", err)
	}
	if len(p7.Content) == 0 {
		return nil, signingInputDecodeError("--profile", fmt.Errorf("decode provisioning profile: signed content is empty"))
	}
	var profile signingRunMobileProvision
	if err := plist.NewDecoder(bytes.NewReader(p7.Content)).Decode(&profile); err != nil {
		return nil, signingInputDecodeError("--profile", fmt.Errorf("decode provisioning profile plist: %w", err))
	}

	profile.UUID = strings.TrimSpace(profile.UUID)
	if !signingRunUUIDPattern.MatchString(profile.UUID) {
		return nil, fmt.Errorf("provisioning profile UUID is missing or invalid")
	}
	if profile.ExpirationDate.IsZero() || !now.Before(profile.ExpirationDate) {
		return nil, fmt.Errorf("provisioning profile is expired")
	}
	if !profile.CreationDate.IsZero() && profile.CreationDate.After(now) {
		return nil, fmt.Errorf("provisioning profile creation date is in the future")
	}
	if !slices.ContainsFunc(profile.Platform, func(value string) bool { return strings.EqualFold(strings.TrimSpace(value), "iOS") }) {
		return nil, fmt.Errorf("provisioning profile does not target iOS")
	}
	if profile.ProvisionsAllDevices {
		return nil, fmt.Errorf("provisioning profile is an enterprise profile, not release-testing")
	}
	if value, exists := profile.Entitlements["get-task-allow"]; exists {
		allowed, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("provisioning profile get-task-allow entitlement must be a boolean")
		}
		if allowed {
			return nil, fmt.Errorf("provisioning profile is a development profile, not release-testing")
		}
	}
	devices := make([]string, 0, len(profile.ProvisionedDevices))
	for _, device := range profile.ProvisionedDevices {
		device = strings.TrimSpace(device)
		if device == "" {
			return nil, fmt.Errorf("provisioning profile contains an empty registered device identifier")
		}
		devices = append(devices, device)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("provisioning profile has no registered devices")
	}

	teamID, err := signingRunTeamID(profile.TeamIdentifier)
	if err != nil {
		return nil, err
	}
	entitlementTeam, ok := profile.Entitlements["com.apple.developer.team-identifier"].(string)
	if !ok || strings.TrimSpace(entitlementTeam) != teamID {
		return nil, fmt.Errorf("provisioning profile entitlement team identifier does not match TeamIdentifier")
	}
	if !slices.Contains(certificate.Subject.OrganizationalUnit, teamID) {
		return nil, fmt.Errorf("identity certificate organizational unit does not contain profile team %q", teamID)
	}

	bundleID, err := signingRunBundleID(profile, teamID)
	if err != nil {
		return nil, err
	}
	embedded := false
	for _, certificateDER := range profile.DeveloperCertificates {
		if _, err := x509.ParseCertificate(certificateDER); err != nil {
			return nil, fmt.Errorf("decode provisioning profile certificate: %w", err)
		}
		if bytes.Equal(certificate.Raw, certificateDER) {
			embedded = true
		}
	}
	if !embedded {
		return nil, fmt.Errorf("identity certificate is not embedded in provisioning profile")
	}

	profileDigest := sha256.Sum256(profileData)
	return &signingRunInspection{
		ProfileUUID:        profile.UUID,
		TeamID:             teamID,
		BundleID:           bundleID,
		ProvisionedDevices: devices,
		CertificateSHA1:    identity.CertificateSHA1,
		CertificateSHA256:  identity.CertificateSHA256,
		ProfileSHA256:      strings.ToUpper(hex.EncodeToString(profileDigest[:])),
		Certificate:        certificate,
		PrivateKey:         identity.PrivateKey,
	}, nil
}

// verifySigningRunProfileTrust reuses the same Apple signer and pinned-root
// checks used by the resign workflow. The caller-provided system pool is kept
// only for the test seam; production verification is pinned and does not
// delegate trust to the mutable system pool.
func verifySigningRunProfileTrust(profile *pkcs7.PKCS7, _ *x509.CertPool, now time.Time) error {
	return verifySigningResignProfileTrust(profile, now)
}

func inspectSigningRunIdentity(identityData, identityPassword []byte, now time.Time) (*signingRunIdentity, error) {
	privateKeys, certificates, err := pkcs12.DecodeAll(identityData, string(identityPassword))
	if err != nil {
		err = fmt.Errorf("decode identity: %w", err)
		if errors.Is(err, pkcs12.ErrIncorrectPassword) {
			return nil, signingIdentityPasswordError(err)
		}
		return nil, signingInputDecodeError("--identity", err)
	}
	if len(privateKeys) != 1 {
		return nil, signingInputDecodeError("--identity", fmt.Errorf("decode identity: expected exactly one private key, found %d", len(privateKeys)))
	}
	signer, ok := privateKeys[0].(crypto.Signer)
	if !ok {
		return nil, signingInputDecodeError("--identity", fmt.Errorf("decode identity: private key is not usable for signing"))
	}
	privatePublic, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, fmt.Errorf("inspect identity private key: %w", err)
	}
	var certificate *x509.Certificate
	for _, candidate := range certificates {
		if candidate == nil {
			continue
		}
		candidatePublic, err := x509.MarshalPKIXPublicKey(candidate.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("inspect identity certificate: %w", err)
		}
		if !bytes.Equal(privatePublic, candidatePublic) {
			continue
		}
		if certificate != nil {
			return nil, signingInputDecodeError("--identity", fmt.Errorf("decode identity: multiple certificates match the private key"))
		}
		certificate = candidate
	}
	if certificate == nil {
		return nil, fmt.Errorf("identity private key does not match its certificate")
	}
	if now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
		return nil, fmt.Errorf("identity certificate is not valid at the current time")
	}

	certificateDigestSHA1 := sha1.Sum(certificate.Raw)
	certificateDigest := sha256.Sum256(certificate.Raw)
	return &signingRunIdentity{
		CertificateSHA1:   strings.ToUpper(hex.EncodeToString(certificateDigestSHA1[:])),
		CertificateSHA256: strings.ToUpper(hex.EncodeToString(certificateDigest[:])),
		Certificate:       certificate,
		PrivateKey:        privateKeys[0],
	}, nil
}

func signingRunCertificateTeamID(certificate *x509.Certificate) (string, error) {
	if certificate == nil {
		return "", fmt.Errorf("identity certificate is missing")
	}
	var teamID string
	for _, organizationalUnit := range certificate.Subject.OrganizationalUnit {
		organizationalUnit = strings.TrimSpace(organizationalUnit)
		if organizationalUnit == "" {
			continue
		}
		if teamID == "" {
			teamID = organizationalUnit
			continue
		}
		if organizationalUnit != teamID {
			return "", fmt.Errorf("identity certificate contains conflicting team identifiers")
		}
	}
	if teamID == "" {
		return "", fmt.Errorf("identity certificate team identifier is missing")
	}
	return teamID, nil
}

func signingRunTeamID(values []string) (string, error) {
	var teamID string
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("provisioning profile contains an empty team identifier")
		}
		if _, exists := seen[value]; exists {
			return "", fmt.Errorf("provisioning profile contains duplicate team identifiers")
		}
		seen[value] = struct{}{}
		if teamID == "" {
			teamID = value
			continue
		}
		if value != teamID {
			return "", fmt.Errorf("provisioning profile contains conflicting team identifiers")
		}
	}
	if teamID == "" {
		return "", fmt.Errorf("provisioning profile team identifier is missing")
	}
	if err := validateSigningResignTeamID(teamID); err != nil {
		return "", fmt.Errorf("provisioning profile: %w", err)
	}
	return teamID, nil
}

func signingRunBundleID(profile signingRunMobileProvision, teamID string) (string, error) {
	value, ok := profile.Entitlements["application-identifier"].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("provisioning profile application identifier is missing")
	}
	applicationID := strings.TrimSpace(value)
	if alternate, exists := profile.Entitlements["com.apple.application-identifier"]; exists {
		alternateValue, ok := alternate.(string)
		if !ok || strings.TrimSpace(alternateValue) != applicationID {
			return "", fmt.Errorf("provisioning profile application identifiers are contradictory")
		}
	}
	prefixes, err := signingRunApplicationIdentifierPrefixes(profile.ApplicationIdentifierPrefix, teamID)
	if err != nil {
		return "", err
	}
	prefix := prefixes[0]
	if !strings.HasPrefix(applicationID, prefix+".") {
		return "", fmt.Errorf("provisioning profile application identifier does not match its declared prefix")
	}
	bundleID := strings.TrimPrefix(applicationID, prefix+".")
	if !validSigningRunBundlePattern(bundleID) {
		return "", fmt.Errorf("provisioning profile bundle identifier pattern %q is invalid", bundleID)
	}
	return bundleID, nil
}

func signingRunApplicationIdentifierPrefixes(values []string, _ string) ([]string, error) {
	if len(values) != 1 {
		return nil, fmt.Errorf("provisioning profile must declare exactly one application identifier prefix; duplicate or missing declarations are invalid")
	}
	prefixes := make([]string, 0, 1)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("provisioning profile contains an empty application identifier prefix")
		}
		if err := validateSigningResignTeamID(value); err != nil {
			return nil, fmt.Errorf("provisioning profile application identifier prefix: %w", err)
		}
		prefixes = append(prefixes, value)
	}
	return prefixes, nil
}

func validSigningRunBundlePattern(value string) bool {
	if value == "*" {
		return true
	}
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\x00\r\n\t") {
		return false
	}
	if strings.Count(value, "*") > 0 && !strings.HasSuffix(value, ".*") {
		return false
	}
	base := strings.TrimSuffix(value, ".*")
	if base == "" {
		return false
	}
	for _, component := range strings.Split(base, ".") {
		if component == "" || strings.Contains(component, "*") {
			return false
		}
	}
	return true
}

func marshalSigningRunReceipt(receipt signingRunReceipt) ([]byte, error) {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode signing run receipt: %w", err)
	}
	return append(data, '\n'), nil
}

func childExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	if code, ok := shared.ProcessExitCode(err); ok {
		return code
	}
	return 1
}
