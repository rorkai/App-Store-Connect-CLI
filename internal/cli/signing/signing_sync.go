package signing

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

const (
	signingSyncPasswordEnvVar = "ASC_SIGNING_SYNC_PASSWORD"
	maxEncryptedSigningFiles  = 256
	maxEncryptedSigningBytes  = 128 << 20
)

// SyncResult is the structured output for sync operations.
//
// The concrete output contract lives in internal/asc so the command's JSON
// and non-JSON renderers share one registered, exported result type.
type SyncResult = asc.SigningSyncResult

// SyncTargetResult describes one target in a multi-target signing sync push.
type SyncTargetResult = asc.SigningSyncTargetResult

type decryptedSigningFile struct {
	RelativePath string
	Plaintext    []byte
	Metadata     signingpkg.EncryptedFileMetadata
	Sensitive    bool
	Identity     bool
}

// SigningSyncCommand returns the signing sync command group.
func SigningSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc signing sync <subcommand> [flags]",
		ShortHelp:  "Sync signing assets with an encrypted git repo.",
		LongHelp: `Sync signing certificates and provisioning profiles with an encrypted git repository.

Fetches signing assets from App Store Connect, encrypts them, and stores them
in a shared git repository. Team members and CI workers can pull and decrypt
the same verified signing files.

Native macOS profile types use .provisionprofile paths; iOS and tvOS profile
types retain .mobileprovision. Existing legacy profile paths remain readable.

Examples:
  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

  asc signing sync push --targets-file ./signing-targets.json --profile-type IOS_APP_STORE \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

  asc signing sync pull --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password \
    --output-dir ./signing

  asc signing sync pull --repo git@github.com:team/certs.git --bundle-id com.example.app \
    --profile-type IOS_APP_STORE --password-file ~/.config/asc/signing-sync-password --output-dir ./signing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			syncPushCommand(),
			syncPullCommand(),
			syncRotatePasswordCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func resolveSyncPassword(passwordFile string) (string, error) {
	if passwordFile != "" && strings.TrimSpace(passwordFile) == "" {
		return "", shared.UsageError("--password-file must not be empty")
	}
	if strings.TrimSpace(passwordFile) != "" {
		data, readErr := readProtectedSecretFile(passwordFile, "signing sync password")
		if readErr != nil {
			return "", readErr
		}
		password := trimPasswordFileNewline(string(data))
		if password == "" {
			return "", shared.UsageError("signing sync password file is empty")
		}
		return password, nil
	}
	if password := os.Getenv(signingSyncPasswordEnvVar); password != "" {
		return password, nil
	}
	return "", shared.UsageError("--password-file is required (or set ASC_SIGNING_SYNC_PASSWORD)")
}

func trimPasswordFileNewline(password string) string {
	if strings.HasSuffix(password, "\r\n") {
		return strings.TrimSuffix(password, "\r\n")
	}
	return strings.TrimSuffix(password, "\n")
}

func onceAfterSuccess(operation func() error) func() error {
	done := false
	return func() error {
		if done {
			return nil
		}
		if err := operation(); err != nil {
			return err
		}
		done = true
		return nil
	}
}

// Kept as a narrow seam so command tests can verify that the command context is
// handed to the whole batch operation, including Git publication, and that the
// batch is not capped by a single outbound-request timeout.
var runSigningSyncBatchForCommand = runSigningSyncBatch

func syncPushCommand() *ffcli.Command {
	fs := flag.NewFlagSet("push", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Bundle identifier (required unless --targets-file is used)")
	targetsFile := fs.String("targets-file", "", "Command-root-relative JSON file containing 1-32 bundle targets (mutually exclusive with --bundle-id)")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, etc. (required)")
	repoURL := fs.String("repo", "", "Git repo URL for encrypted storage (required)")
	passwordFile := fs.String("password-file", "", "Protected file containing the repository encryption password (or set ASC_SIGNING_SYNC_PASSWORD)")
	branch := fs.String("branch", "main", "Git branch")
	certType := fs.String("certificate-type", "", "Certificate type filter (optional)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (requires --create-missing; required for development profiles)")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	identityPath := fs.String("identity", "", "Protected PKCS#12 signing identity file")
	privateKeyPath := fs.String("private-key", "", "Protected RSA or EC private key PEM file")
	identitySHA256 := fs.String("identity-sha256", "", "SHA-256 certificate fingerprint selecting a PKCS#12 identity or the ASC certificate for --private-key")
	identityPasswordFile := fs.String("identity-password-file", "", "Protected file containing the source PKCS#12 password")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "push",
		ShortUsage: "asc signing sync push (--bundle-id ID | --targets-file PATH) --profile-type TYPE --repo URL [--password-file PATH]",
		ShortHelp:  "Fetch signing assets from ASC, encrypt, and push to git.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			bundle := strings.TrimSpace(*bundleID)
			targetsPath := *targetsFile
			hasTargetsPath := strings.TrimSpace(targetsPath) != ""
			if targetsPath != "" && !hasTargetsPath {
				return shared.UsageError("--targets-file must not be empty")
			}
			if bundle != "" && hasTargetsPath {
				return shared.UsageError("--bundle-id and --targets-file are mutually exclusive")
			}
			var targetBundles []string
			if hasTargetsPath {
				var readErr error
				targetBundles, readErr = readSigningSyncTargetsFile(targetsPath)
				if readErr != nil {
					return shared.UsageError(readErr.Error())
				}
			} else if bundle == "" {
				return shared.UsageError("--bundle-id is required")
			} else {
				targetBundles = []string{bundle}
			}
			profType := strings.ToUpper(strings.TrimSpace(*profileType))
			if profType == "" {
				return shared.UsageError("--profile-type is required")
			}
			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			if err := rejectDeviceWithoutCreateMissing(*deviceIDs, *createMissing); err != nil {
				return err
			}
			if *createMissing && isDevelopmentProfile(profType) && strings.TrimSpace(*deviceIDs) == "" {
				return shared.UsageError("--device is required for development profiles with --create-missing")
			}
			identityInput := strings.TrimSpace(*identityPath)
			privateKeyInput := strings.TrimSpace(*privateKeyPath)
			if identityInput != "" && privateKeyInput != "" {
				return shared.UsageError("--identity and --private-key are mutually exclusive")
			}
			if strings.TrimSpace(*identitySHA256) != "" && identityInput == "" && privateKeyInput == "" {
				return shared.UsageError("--identity-sha256 requires --identity or --private-key")
			}
			if strings.TrimSpace(*identityPasswordFile) != "" && identityInput == "" {
				return shared.UsageError("--identity-password-file requires --identity")
			}
			if privateKeyInput != "" && strings.TrimSpace(*identitySHA256) == "" {
				return shared.UsageError("--identity-sha256 is required with --private-key to select one App Store Connect certificate")
			}
			requestedFingerprint, fingerprintErr := normalizeCertificateFingerprint(*identitySHA256)
			if fingerprintErr != nil {
				return shared.UsageError(fingerprintErr.Error())
			}
			pass, err := resolveSyncPassword(*passwordFile)
			if err != nil {
				return err
			}

			var identity *signingIdentity
			switch {
			case identityInput != "":
				identityPassword := ""
				if strings.TrimSpace(*identityPasswordFile) != "" {
					passwordBytes, readErr := readProtectedSecretFile(*identityPasswordFile, "identity password")
					if readErr != nil {
						return fmt.Errorf("signing sync push: identity password: %w", readErr)
					}
					identityPassword = trimPasswordFileNewline(string(passwordBytes))
				}
				identity, err = loadPKCS12Identity(identityInput, identityPassword, requestedFingerprint)
			case privateKeyInput != "":
				identity, err = loadPrivateSigningKey(privateKeyInput)
				if err == nil {
					identity.RequestedSHA256 = requestedFingerprint
				}
			}
			if err != nil {
				return fmt.Errorf("signing sync push: signing identity: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			if hasTargetsPath {
				// The batch spans one lookup, asset resolution, and optional
				// profile creation per target plus the Git clone and push, so it
				// receives the command context and applies its own per-request
				// timeouts. Capping the whole run with a single request budget
				// would fail valid multi-target runs and, with --create-missing,
				// could abandon created profiles before publication.
				result, batchErr := runSigningSyncBatchForCommand(ctx, client, signingSyncBatchOptions{
					RepoURL:         repo,
					Branch:          *branch,
					Password:        pass,
					ProfileType:     profType,
					CertificateType: *certType,
					DeviceIDs:       shared.SplitCSV(*deviceIDs),
					CreateMissing:   *createMissing,
					Identity:        identity,
					BundleIDs:       targetBundles,
				})
				if batchErr != nil {
					return fmt.Errorf("signing sync push: %w", batchErr)
				}
				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Fetch signing assets from ASC.
			fmt.Fprintln(os.Stderr, "Fetching signing assets from App Store Connect...")

			bundleIDResp, err := findBundleID(requestCtx, client, bundle)
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
			if err != nil {
				return fmt.Errorf("signing sync push: create temp dir: %w", err)
			}

			store := &signingpkg.GitStore{
				RepoURL:  repo,
				LocalDir: tmpDir,
				Branch:   *branch,
			}
			defer func() { _ = store.Cleanup() }()

			prepareRepository := onceAfterSuccess(func() error {
				fmt.Fprintln(os.Stderr, "Cloning signing repo...")
				if err := store.Clone(ctx, true); err != nil {
					return err
				}
				return nil
			})
			var identityArtifacts *signingIdentityArtifacts

			profile, certs, created, err := resolveSigningAssets(
				requestCtx,
				client,
				signingAssetsOptions{
					BundleIDResourceID: bundleIDResp.Data.ID,
					BundleIdentifier:   bundle,
					ProfileType:        profType,
					CertificateType:    *certType,
					DeviceIDs:          shared.SplitCSV(*deviceIDs),
					CreateMissing:      *createMissing,
					BeforeCreate: func(plan profileCreatePlan) error {
						if identity != nil {
							if err := preflightIdentityForProfileCreate(identity, plan, pass, time.Now()); err != nil {
								return err
							}
						}
						if err := prepareRepository(); err != nil {
							return err
						}
						if identity != nil {
							identityArtifacts, err = prepareSigningIdentityArtifacts(identity, pass, bundle, profType)
							if err != nil {
								return err
							}
						}
						profileExtension := shared.ProvisioningProfileExtension("", profType)
						preferredProfilePath := filepath.Join("profiles", profileDirectoryName(profType), safeFileName(plan.ProfileName, "profile")+profileExtension)
						profilePath, err := resolveCompatibleSigningProfilePath(store, preferredProfilePath)
						if err != nil {
							return err
						}
						plannedPaths := signingAssetRepositoryPathsForProfile(plan.Certificates, profType, profilePath, identityArtifacts)
						if err := store.CheckEncryptedRepositoryPaths(plannedPaths); err != nil {
							return err
						}
						if err := preflightSigningAssetDestinationsForProfile(store, plan, profType, profilePath); err != nil {
							return err
						}
						if identity != nil {
							if err := preflightSigningIdentityArtifactsForContextUpdate(store, identityArtifacts, pass); err != nil {
								return err
							}
						}
						return nil
					},
					CreateContext: func() (context.Context, context.CancelFunc) {
						return shared.ContextWithTimeout(ctx)
					},
					CertificateFilter: identityCertificateFilter(identity),
				},
			)
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}
			if created {
				fmt.Fprintln(os.Stderr, "Created new profile")
			}
			if identity != nil {
				if err := validateIdentityForResolvedAssets(identity, profile, certs, bundle, profType, time.Now()); err != nil {
					return fmt.Errorf("signing sync push: validate signing identity: %w", err)
				}
			}
			if err := prepareRepository(); err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}
			profileContent, err := base64.StdEncoding.DecodeString(strings.TrimSpace(profile.Data.Attributes.ProfileContent))
			if err != nil {
				return fmt.Errorf("signing sync push: decode profile: %w", err)
			}
			profileDir := profileDirectoryName(profType)
			profileExtension := shared.ProvisioningProfileExtension(string(profile.Data.Attributes.Platform), profType)
			profileRelPath := filepath.Join("profiles", profileDir, safeFileName(profile.Data.Attributes.Name, profile.Data.ID)+profileExtension)
			profileRelPath, err = resolveCompatibleSigningProfilePath(store, profileRelPath)
			if err != nil {
				return fmt.Errorf("signing sync push: resolve profile repository path: %w", err)
			}
			profileMetadata, err := signingProfileArtifactMetadata(profile, bundle, profType)
			if err != nil {
				return fmt.Errorf("signing sync push: prepare profile metadata: %w", err)
			}
			if identity != nil {
				if identityArtifacts == nil {
					identityArtifacts, err = prepareSigningIdentityArtifacts(identity, pass, bundle, profType)
					if err != nil {
						return fmt.Errorf("signing sync push: prepare signing identity: %w", err)
					}
				}
				if err := bindSigningIdentityProfile(identityArtifacts, profile, profileRelPath, profileContent); err != nil {
					return fmt.Errorf("signing sync push: bind signing identity profile: %w", err)
				}
			}
			plannedPaths := signingAssetRepositoryPathsForProfile(certs.Data, profType, profileRelPath, identityArtifacts)
			if err := store.CheckEncryptedRepositoryPaths(plannedPaths); err != nil {
				return fmt.Errorf("signing sync push: preflight repository paths: %w", err)
			}
			if identity != nil {
				if err := preflightSigningIdentityArtifactsForContextUpdate(store, identityArtifacts, pass); err != nil {
					return fmt.Errorf("signing sync push: preflight signing identity: %w", err)
				}
			}
			if _, err := preflightSigningProfileArtifact(store, profileRelPath, profileContent, pass, profileMetadata); err != nil {
				return fmt.Errorf("signing sync push: preflight profile: %w", err)
			}

			// Write encrypted files.
			var files []string

			certDir := certDirectoryName(profType)
			for _, cert := range certs.Data {
				certContent, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cert.Attributes.CertificateContent))
				if err != nil {
					return fmt.Errorf("signing sync push: decode cert: %w", err)
				}
				relPath := filepath.Join("certs", certDir, safeFileName(cert.Attributes.SerialNumber, cert.ID)+".cer")
				if err := store.WriteEncryptedFile(relPath, certContent, pass); err != nil {
					return fmt.Errorf("signing sync push: encrypt cert: %w", err)
				}
				files = append(files, relPath)
				fmt.Fprintf(os.Stderr, "  Encrypted %s\n", relPath)
			}

			if err := writeOrReuseSigningProfileArtifact(store, profileRelPath, profileContent, pass, profileMetadata); err != nil {
				return fmt.Errorf("signing sync push: encrypt profile: %w", err)
			}
			files = append(files, profileRelPath)
			fmt.Fprintf(os.Stderr, "  Encrypted %s\n", profileRelPath)

			var sensitiveFiles []string
			if identity != nil {
				if err := writeOrReuseSigningIdentityArtifacts(store, identityArtifacts, pass); err != nil {
					return fmt.Errorf("signing sync push: encrypt signing identity: %w", err)
				}
				files = append(files, identityArtifacts.IdentityPath, identityArtifacts.BindingPath)
				sensitiveFiles = append(sensitiveFiles, identityArtifacts.IdentityPath)
				fmt.Fprintf(os.Stderr, "  Encrypted %s\n", identityArtifacts.IdentityPath)
				fmt.Fprintf(os.Stderr, "  Encrypted %s\n", identityArtifacts.BindingPath)
			}

			// Commit and push.
			commitMsg := fmt.Sprintf("Update signing assets for %s (%s)", bundle, profType)
			fmt.Fprintln(os.Stderr, "Pushing to git...")
			if err := store.CommitAndPush(ctx, commitMsg); err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			fmt.Fprintln(os.Stderr, "Done")

			result := SyncResult{
				Operation:       "push",
				RepoURL:         sanitizeRepoURLForOutput(repo),
				BundleID:        bundle,
				ProfileType:     profType,
				Files:           files,
				IdentityPresent: identity != nil,
				SensitiveFiles:  sensitiveFiles,
			}
			if identity != nil {
				result.IdentitySHA256 = identity.CertificateSHA256
			}
			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

func syncPullCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)

	repoURL := fs.String("repo", "", "Git repo URL (required)")
	bundleID := fs.String("bundle-id", "", "Decrypt only one bundle target (requires --profile-type; mutually exclusive with --targets-file)")
	targetsFile := fs.String("targets-file", "", "Decrypt only the 1-32 bundle targets in a root-relative JSON file (requires --profile-type; mutually exclusive with --bundle-id)")
	profileType := fs.String("profile-type", "", "Profile type for --bundle-id or --targets-file")
	passwordFile := fs.String("password-file", "", "Protected file containing the repository encryption password (or set ASC_SIGNING_SYNC_PASSWORD)")
	branch := fs.String("branch", "main", "Git branch")
	outputDir := fs.String("output-dir", "./signing", "Output directory for decrypted files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc signing sync pull --repo URL [--bundle-id ID | --targets-file PATH] [--profile-type TYPE] [--password-file PATH] [--output-dir DIR]",
		ShortHelp:  "Pull and decrypt signing assets from git.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			provided := make(map[string]bool)
			fs.Visit(func(flag *flag.Flag) {
				provided[flag.Name] = true
			})
			bundle := strings.TrimSpace(*bundleID)
			profile := strings.ToUpper(strings.TrimSpace(*profileType))
			bundleProvided := provided["bundle-id"]
			targetsProvided := provided["targets-file"]
			profileProvided := provided["profile-type"]
			if bundleProvided && bundle == "" {
				return shared.UsageError("--bundle-id must not be empty")
			}
			if targetsProvided && strings.TrimSpace(*targetsFile) == "" {
				return shared.UsageError("--targets-file must not be empty")
			}
			if profileProvided && profile == "" {
				return shared.UsageError("--profile-type must not be empty")
			}
			if bundleProvided && targetsProvided {
				return shared.UsageError("--bundle-id and --targets-file are mutually exclusive")
			}
			selectionRequested := bundleProvided || targetsProvided
			if selectionRequested && profile == "" {
				return shared.UsageError("--profile-type is required with --bundle-id or --targets-file")
			}
			if !selectionRequested && profileProvided {
				return shared.UsageError("--profile-type requires --bundle-id or --targets-file")
			}
			if selectionRequested {
				var normalizeErr error
				profile, normalizeErr = normalizeSigningPullProfileType(profile)
				if normalizeErr != nil {
					return shared.UsageError(normalizeErr.Error())
				}
			}
			var selectedBundleIDs []string
			switch {
			case bundleProvided:
				selectedBundleIDs = []string{bundle}
			case targetsProvided:
				var readErr error
				selectedBundleIDs, readErr = readSigningSyncTargetsFile(*targetsFile)
				if readErr != nil {
					return shared.UsageError(readErr.Error())
				}
			}
			pass, err := resolveSyncPassword(*passwordFile)
			if err != nil {
				return err
			}

			outDir := strings.TrimSpace(*outputDir)
			if outDir == "" {
				outDir = "./signing"
			}

			// Clone git repo.
			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
			if err != nil {
				return fmt.Errorf("signing sync pull: create temp dir: %w", err)
			}

			store := &signingpkg.GitStore{
				RepoURL:  repo,
				LocalDir: tmpDir,
				Branch:   *branch,
			}
			defer func() { _ = store.Cleanup() }()

			fmt.Fprintln(os.Stderr, "Cloning signing repo...")
			if err := store.Clone(ctx, false); err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}

			// List and decrypt all files.
			encryptedFiles, err := store.ListEncryptedFiles()
			if err != nil {
				return fmt.Errorf("signing sync pull: list files: %w", err)
			}

			if len(encryptedFiles) == 0 {
				if selectionRequested {
					return fmt.Errorf("signing sync pull: no active %s profile found in encrypted repository for bundle ID(s): %s", profile, strings.Join(selectedBundleIDs, ", "))
				}
				fmt.Fprintln(os.Stderr, "No encrypted signing files found in repo")
				result := SyncResult{
					Operation: "pull",
					RepoURL:   sanitizeRepoURLForOutput(repo),
					Files:     []string{},
				}
				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			decrypted, err := decryptAndValidateSigningFiles(store, encryptedFiles, pass)
			if err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}
			var targets []SyncTargetResult
			if selectionRequested {
				decrypted, targets, err = selectSigningPullFiles(decrypted, selectedBundleIDs, profile)
				if err != nil {
					return fmt.Errorf("signing sync pull: %w", err)
				}
			}

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("signing sync pull: create output dir: %w", err)
			}
			outputRoot, err := rootfs.New(outDir)
			if err != nil {
				return fmt.Errorf("signing sync pull: create output root: %w", err)
			}
			defer outputRoot.Close()
			if err := preflightSigningPullFilesInRoot(outputRoot, decrypted); err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}

			var files []string
			var sensitiveFiles []string
			identityPresent := false
			for _, file := range decrypted {
				if err := writeDecryptedOutputFileInRoot(outputRoot, file.RelativePath, file.Plaintext, file.Sensitive); err != nil {
					return fmt.Errorf("signing sync pull: %w", err)
				}

				files = append(files, file.RelativePath)
				if file.Sensitive {
					sensitiveFiles = append(sensitiveFiles, file.RelativePath)
				}
				if file.Identity {
					identityPresent = true
				}
				fmt.Fprintf(os.Stderr, "  Decrypted %s\n", file.RelativePath)
			}

			fmt.Fprintf(os.Stderr, "Done — %d files written to %s\n", len(files), outDir)

			result := SyncResult{
				Operation:       "pull",
				RepoURL:         sanitizeRepoURLForOutput(repo),
				Files:           files,
				IdentityPresent: identityPresent,
				SensitiveFiles:  sensitiveFiles,
			}
			if selectionRequested {
				applySigningPullSelectionResult(&result, profile, selectedBundleIDs, targets, targetsProvided)
			}
			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

func applySigningPullSelectionResult(result *SyncResult, profileType string, bundleIDs []string, targets []SyncTargetResult, batch bool) {
	if result == nil {
		return
	}
	result.ProfileType = profileType
	if batch {
		result.BundleIDs = bundleIDs
		result.Targets = targets
		result.MarkBatch()
		return
	}
	if len(bundleIDs) == 1 {
		result.BundleID = bundleIDs[0]
	}
}

func prepareDecryptedSigningFiles(store *signingpkg.GitStore, encryptedFiles []string, password, outDir string) ([]decryptedSigningFile, error) {
	root, err := rootfs.New(outDir)
	if err != nil {
		return nil, fmt.Errorf("create output root: %w", err)
	}
	defer root.Close()
	return prepareDecryptedSigningFilesInRoot(store, encryptedFiles, password, root)
}

func prepareDecryptedSigningFilesInRoot(store *signingpkg.GitStore, encryptedFiles []string, password string, root rootfs.Root) ([]decryptedSigningFile, error) {
	decrypted, err := decryptAndValidateSigningFiles(store, encryptedFiles, password)
	if err != nil {
		return nil, err
	}
	if err := preflightSigningPullFilesInRoot(root, decrypted); err != nil {
		return nil, err
	}
	return decrypted, nil
}

func decryptAndValidateSigningFiles(store *signingpkg.GitStore, encryptedFiles []string, password string) ([]decryptedSigningFile, error) {
	decrypted, activeIdentityPaths, err := loadAndValidateSigningFiles(store, encryptedFiles, password)
	if err != nil {
		return nil, err
	}
	filtered := decrypted[:0]
	for _, file := range decrypted {
		if file.Metadata.Kind == "pkcs12-identity" {
			canonicalPath := strings.ReplaceAll(filepath.ToSlash(file.RelativePath), `\`, "/")
			if _, active := activeIdentityPaths[canonicalPath]; !active {
				continue
			}
		}
		filtered = append(filtered, file)
	}
	decrypted = filtered

	return decrypted, nil
}

func loadAndValidateSigningFiles(store *signingpkg.GitStore, encryptedFiles []string, password string) ([]decryptedSigningFile, map[string]struct{}, error) {
	if len(encryptedFiles) > maxEncryptedSigningFiles {
		return nil, nil, fmt.Errorf("encrypted signing repository contains %d files; limit is %d", len(encryptedFiles), maxEncryptedSigningFiles)
	}
	if err := signingpkg.ValidateEncryptedRepositoryPaths(encryptedFiles); err != nil {
		return nil, nil, err
	}
	var cumulativeSize int64
	for _, relPath := range encryptedFiles {
		size, err := store.EncryptedFileSize(relPath)
		if err != nil {
			return nil, nil, fmt.Errorf("inspect encrypted artifact %s: %w", relPath, err)
		}
		if size < 0 || size > maxEncryptedSigningBytes-cumulativeSize {
			return nil, nil, fmt.Errorf("encrypted signing repository exceeds the %d-byte cumulative size limit", maxEncryptedSigningBytes)
		}
		cumulativeSize += size
	}
	decrypted := make([]decryptedSigningFile, 0, len(encryptedFiles))
	for _, relPath := range encryptedFiles {
		plaintext, metadata, err := store.ReadEncryptedFileWithMetadata(relPath, password)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt %s: %w", relPath, err)
		}
		sensitive, identity, err := classifySigningFile(relPath, plaintext, metadata, password)
		if err != nil {
			return nil, nil, fmt.Errorf("validate %s: %w", relPath, err)
		}
		decrypted = append(decrypted, decryptedSigningFile{
			RelativePath: relPath,
			Plaintext:    plaintext,
			Metadata:     metadata,
			Sensitive:    sensitive,
			Identity:     identity,
		})
	}
	activeIdentityPaths, err := validateIdentityArtifactGraph(decrypted)
	if err != nil {
		return nil, nil, err
	}
	for _, file := range decrypted {
		if file.Metadata.Kind != "pkcs12-identity" {
			continue
		}
		canonicalPath := strings.ReplaceAll(filepath.ToSlash(file.RelativePath), `\`, "/")
		if _, active := activeIdentityPaths[canonicalPath]; !active {
			continue
		}
		_, certificate, err := modernpkcs12.Decode(file.Plaintext, password)
		if err != nil || certificate == nil {
			return nil, nil, fmt.Errorf("active identity core is not a decodable PKCS#12 identity")
		}
		if now := time.Now(); now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
			return nil, nil, fmt.Errorf("active identity certificate is not currently valid")
		}
	}
	return decrypted, activeIdentityPaths, nil
}

func validateIdentityArtifactGraph(files []decryptedSigningFile) (map[string]struct{}, error) {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.RelativePath)
	}
	if err := signingpkg.ValidateEncryptedRepositoryPaths(paths); err != nil {
		return nil, err
	}

	type identityCore struct {
		teamID            string
		certificateSHA256 string
		contextCount      int
	}
	cores := make(map[string]identityCore)
	activeCores := make(map[string]struct{})
	contexts := make(map[string]string)
	profiles := make(map[string][]byte)
	profileMetadata := make(map[string]signingpkg.EncryptedFileMetadata)
	contextProfileTypes := make(map[string]string)
	for _, file := range files {
		canonicalPath := strings.ReplaceAll(filepath.ToSlash(file.RelativePath), `\`, "/")
		profiles[canonicalPath] = file.Plaintext
		if file.Metadata.Kind == signingProfileArtifactKind {
			profileMetadata[canonicalPath] = file.Metadata
		}
		if file.Metadata.Kind == "pkcs12-identity" {
			if _, exists := cores[canonicalPath]; exists {
				return nil, fmt.Errorf("duplicate signing identity core %s", canonicalPath)
			}
			cores[canonicalPath] = identityCore{teamID: file.Metadata.TeamID, certificateSHA256: file.Metadata.CertificateSHA256}
		}
	}
	for _, file := range files {
		if file.Metadata.Kind != "identity-context" {
			continue
		}
		var binding identityContextBinding
		if err := json.Unmarshal(file.Plaintext, &binding); err != nil {
			return nil, fmt.Errorf("decode identity context graph: %w", err)
		}
		profilePath := canonicalSigningPullPath(binding.ProfilePath)
		profileType := strings.ToUpper(strings.TrimSpace(binding.ProfileType))
		if prior, exists := contextProfileTypes[profilePath]; exists && prior != profileType {
			return nil, fmt.Errorf("conflicting identity context profile-type provenance")
		}
		contextProfileTypes[profilePath] = profileType
		scope := strings.Join([]string{binding.TeamID, binding.BundleID, binding.ProfileType}, "\x00")
		if prior, exists := contexts[scope]; exists {
			if prior == binding.CertificateSHA256 {
				return nil, fmt.Errorf("duplicate identity context binding")
			}
			return nil, fmt.Errorf("conflicting identity context bindings")
		}
		contexts[scope] = binding.CertificateSHA256
	}
	for _, file := range files {
		if file.Metadata.Kind != "identity-context" {
			continue
		}
		var binding identityContextBinding
		if err := json.Unmarshal(file.Plaintext, &binding); err != nil {
			return nil, fmt.Errorf("decode identity context graph: %w", err)
		}
		corePath := filepath.ToSlash(filepath.Join("identities", certDirectoryName(binding.ProfileType), binding.CertificateSHA256+".p12"))
		core, exists := cores[corePath]
		if !exists {
			return nil, fmt.Errorf("identity context has no matching core identity")
		}
		if core.teamID != binding.TeamID {
			return nil, fmt.Errorf("identity context team does not match its core identity")
		}
		profilePath := canonicalSigningPullPath(binding.ProfilePath)
		profileContent, exists := profiles[profilePath]
		if !exists {
			return nil, fmt.Errorf("identity context has no matching profile artifact")
		}
		if metadata, authenticated := profileMetadata[profilePath]; authenticated &&
			(metadata.BundleID != binding.BundleID || metadata.ProfileType != binding.ProfileType ||
				metadata.ProfileResourceID != binding.ProfileResourceID || (metadata.ProfileUUID != "" && metadata.ProfileUUID != binding.ProfileUUID)) {
			return nil, fmt.Errorf("identity context does not match authenticated profile metadata")
		}
		profileDigest := sha256.Sum256(profileContent)
		if !strings.EqualFold(hex.EncodeToString(profileDigest[:]), binding.ProfileSHA256) {
			return nil, fmt.Errorf("identity context profile digest does not match profile artifact")
		}
		profile, err := parseIdentityMobileProvision(profileContent)
		if err != nil {
			return nil, fmt.Errorf("identity context profile is invalid: %w", err)
		}
		profileUUID, err := normalizeIdentityProfileUUID(profile.UUID)
		if err != nil {
			return nil, fmt.Errorf("identity context profile UUID is invalid: %w", err)
		}
		bindingUUID, err := normalizeIdentityProfileUUID(binding.ProfileUUID)
		if err != nil || profileUUID != bindingUUID {
			return nil, fmt.Errorf("identity context profile UUID does not match profile artifact")
		}
		if !profile.ExpirationDate.After(time.Now()) || !containsFold(profile.TeamIdentifier, binding.TeamID) {
			return nil, fmt.Errorf("identity context profile is expired or belongs to another team")
		}
		if !identityProfileTypeMatches(profile, binding.ProfileType) || !signingPullProfilePlatformMatches(profile, binding.ProfileType) {
			return nil, fmt.Errorf("identity context profile distribution type does not match authenticated scope")
		}
		applicationIdentifier, _ := profile.Entitlements["application-identifier"].(string)
		if applicationIdentifier == "" {
			applicationIdentifier, _ = profile.Entitlements["com.apple.application-identifier"].(string)
		}
		prefixMatches := false
		for _, prefix := range profile.ApplicationIdentifierPrefix {
			if applicationIdentifier == strings.TrimSpace(prefix)+"."+binding.BundleID {
				prefixMatches = true
				break
			}
		}
		if !prefixMatches {
			return nil, fmt.Errorf("identity context profile bundle does not match authenticated scope")
		}
		certificateMatches := false
		for _, der := range profile.DeveloperCertificates {
			parsed, err := x509.ParseCertificate(der)
			if err == nil && strings.EqualFold(signingCertificateSHA256(parsed), core.certificateSHA256) {
				certificateMatches = true
				break
			}
		}
		if !certificateMatches {
			return nil, fmt.Errorf("identity context profile does not embed its core certificate")
		}
		core.contextCount++
		cores[corePath] = core
		activeCores[corePath] = struct{}{}
	}
	return activeCores, nil
}

func identityProfileTypeMatches(profile *identityMobileProvision, profileType string) bool {
	if profile == nil {
		return false
	}
	getTaskAllow, _ := profile.Entitlements["get-task-allow"].(bool)
	normalized := strings.ToUpper(strings.TrimSpace(profileType))
	switch {
	case strings.Contains(normalized, "DEVELOPMENT"):
		return getTaskAllow && len(profile.ProvisionedDevices) > 0 && !profile.ProvisionsAllDevices
	case strings.Contains(normalized, "ADHOC"), strings.Contains(normalized, "AD_HOC"):
		return !getTaskAllow && len(profile.ProvisionedDevices) > 0 && !profile.ProvisionsAllDevices
	case isDirectDistributionProfile(normalized):
		// Direct profiles use the all-device claim. Exact native Mac versus Mac
		// Catalyst provenance still comes from the API profile type, and the
		// resolved Developer ID certificate is matched byte-for-byte.
		return !getTaskAllow && len(profile.ProvisionedDevices) == 0 && profile.ProvisionsAllDevices &&
			len(profile.Platform) == 1 && strings.EqualFold(strings.TrimSpace(profile.Platform[0]), "OSX")
	case strings.Contains(normalized, "INHOUSE"), strings.Contains(normalized, "IN_HOUSE"):
		return !getTaskAllow && profile.ProvisionsAllDevices
	case strings.Contains(normalized, "STORE"):
		return !getTaskAllow && len(profile.ProvisionedDevices) == 0 && !profile.ProvisionsAllDevices
	default:
		return false
	}
}

func isDirectDistributionProfile(profileType string) bool {
	switch strings.ToUpper(strings.TrimSpace(profileType)) {
	case "MAC_APP_DIRECT", "MAC_CATALYST_APP_DIRECT":
		return true
	default:
		return false
	}
}

func classifySigningFile(relPath string, plaintext []byte, metadata signingpkg.EncryptedFileMetadata, password string) (sensitive, identity bool, err error) {
	canonicalPath := strings.ReplaceAll(filepath.ToSlash(relPath), `\`, "/")
	if metadata.Kind == signingProfileArtifactKind {
		if err := validateSigningProfileArtifactMetadata(canonicalPath, metadata); err != nil {
			return false, false, err
		}
		return false, false, nil
	}
	parts := strings.Split(canonicalPath, "/")
	identityPath := len(parts) == 3 && parts[0] == "identities" &&
		(parts[1] == "distribution" || parts[1] == "development") && strings.HasSuffix(parts[2], ".p12")
	identityLikePath := strings.HasSuffix(strings.ToLower(canonicalPath), ".p12") ||
		strings.HasPrefix(strings.ToLower(canonicalPath), "identities/")
	if identityLikePath || metadata.Kind == "pkcs12-identity" {
		if !identityPath || metadata.Version != 1 || metadata.Kind != "pkcs12-identity" || !metadata.Sensitive {
			return false, false, fmt.Errorf("identity path requires a versioned sensitive pkcs12-identity envelope")
		}
		fingerprint := strings.TrimSuffix(parts[2], ".p12")
		if len(fingerprint) != 64 || !strings.EqualFold(fingerprint, metadata.CertificateSHA256) || metadata.TeamID == "" || metadata.BundleID != "" || metadata.ProfileType != "" {
			return false, false, fmt.Errorf("identity metadata does not match its team/certificate-scoped path")
		}
		privateKey, certificate, decodeErr := modernpkcs12.Decode(plaintext, password)
		if decodeErr != nil || certificate == nil || validateSigningPrivateKeyType(privateKey) != nil || !publicKeysEqual(privateKey, certificate.PublicKey) {
			return false, false, fmt.Errorf("identity artifact is not a usable password-protected PKCS#12 identity")
		}
		if !strings.EqualFold(signingCertificateSHA256(certificate), metadata.CertificateSHA256) || certificateTeamID(certificate) != metadata.TeamID {
			return false, false, fmt.Errorf("identity certificate does not match authenticated metadata")
		}
		return true, true, nil
	}
	if metadata.Kind == "identity-context" {
		if metadata.Version != 1 || metadata.Sensitive {
			return false, false, fmt.Errorf("identity context must be a versioned non-sensitive envelope")
		}
		var binding identityContextBinding
		if err := json.Unmarshal(plaintext, &binding); err != nil {
			return false, false, fmt.Errorf("decode identity context: %w", err)
		}
		if binding.CertificateSHA256 != metadata.CertificateSHA256 || binding.TeamID != metadata.TeamID || binding.BundleID != metadata.BundleID || binding.ProfileType != metadata.ProfileType ||
			binding.ProfileResourceID != metadata.ProfileResourceID || binding.ProfileUUID != metadata.ProfileUUID || binding.ProfilePath != metadata.ProfilePath || binding.ProfileSHA256 != metadata.ProfileSHA256 {
			return false, false, fmt.Errorf("identity context does not match authenticated metadata")
		}
		digest := sha256.Sum256([]byte(strings.Join([]string{binding.TeamID, binding.BundleID, binding.ProfileType}, "\x00")))
		wantPath := "identity-contexts/" + strings.ToUpper(hex.EncodeToString(digest[:])) + ".json"
		if canonicalPath != wantPath {
			return false, false, fmt.Errorf("identity context path does not match authenticated scope")
		}
	}
	return metadata.Sensitive, false, nil
}

func sanitizeRepoURLForOutput(raw string) string {
	return signingpkg.RedactRepoURL(raw)
}

func writeDecryptedOutputFile(outDir, relPath string, plaintext []byte, sensitive bool) error {
	root, err := rootfs.New(outDir)
	if err != nil {
		return fmt.Errorf("create output root: %w", err)
	}
	defer root.Close()
	return writeDecryptedOutputFileInRoot(root, relPath, plaintext, sensitive)
}

func writeDecryptedOutputFileInRoot(root rootfs.Root, relPath string, plaintext []byte, sensitive bool) error {
	var err error
	if sensitive {
		err = root.CreateNewFile(relPath, plaintext, 0o600)
	} else {
		err = root.WriteFilePreservingMode(relPath, plaintext, 0o600)
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", relPath, err)
	}
	return nil
}

func certDirectoryName(profileType string) string {
	normalized := strings.ToUpper(profileType)
	if strings.Contains(normalized, "DEVELOPMENT") {
		return "development"
	}
	return "distribution"
}

func profileDirectoryName(profileType string) string {
	normalized := strings.ToUpper(profileType)
	switch {
	case strings.Contains(normalized, "STORE"):
		return "appstore"
	case strings.Contains(normalized, "ADHOC"), strings.Contains(normalized, "AD_HOC"):
		return "adhoc"
	case strings.Contains(normalized, "DEVELOPMENT"):
		return "development"
	case strings.Contains(normalized, "INHOUSE"), strings.Contains(normalized, "IN_HOUSE"):
		return "enterprise"
	default:
		return "other"
	}
}
