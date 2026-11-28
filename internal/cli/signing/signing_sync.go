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

func newSigningSyncCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("sync", flag.ExitOnError)

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

Push and pull can keep the same encrypted artifacts in GitLab Secure Files,
AWS Secrets Manager, or S3-compatible object storage instead of git.
Encryption, path checks, and size limits are unchanged; only
the transport differs, and the remote stores hold ciphertext only.

Object storage (--storage object) uses the git tree's layout, so a bucket
prefix and a repository can be mirrored byte for byte. Credentials and, unless
--object-region is set, the region come from the standard AWS configuration:
environment variables, shared config or SSO profiles, web identity, or
container and instance metadata. No flag accepts a credential. Writes are
conditional: a push never overwrites an object that changed after it was
fetched and fails instead, so pull and retry. Endpoints that ignore
If-Match and If-None-Match lose this protection.

Password rotation supports git and object storage. import re-encrypts an
existing OpenSSL-encrypted signing repository into any of these stores
without contacting App Store Connect.

Push --renew-expired replaces an expired profile with a same-name profile, and
--force-for-new-devices recreates a development or ad hoc profile when its
devices differ from the enabled device list. nuke deletes every profile of one
type, revokes the matching certificates, and removes their encrypted
artifacts. These lifecycle operations are git-only.

Examples:
  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

  asc signing sync push --targets-file ./signing-targets.json --profile-type IOS_APP_STORE \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

  asc signing sync pull --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password \
    --output-dir ./signing

  asc signing sync pull --repo git@github.com:team/certs.git --bundle-id com.example.app \
    --profile-type IOS_APP_STORE --password-file ~/.config/asc/signing-sync-password --output-dir ./signing

  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
    --storage gitlab-secure-files --gitlab-project 1234 --prefix asc-signing \
    --gitlab-token-file ~/.config/asc/gitlab-token --password-file ~/.config/asc/signing-sync-password

  asc signing sync pull --storage aws-secrets-manager --region us-east-1 --prefix asc-signing \
    --password-file ~/.config/asc/signing-sync-password --output-dir ./signing

  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
    --storage object --object-bucket team-certs --object-prefix asc/ \
    --password-file ~/.config/asc/signing-sync-password

  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_ADHOC --force-for-new-devices \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

  asc signing sync nuke --profile-type IOS_APP_DEVELOPMENT --repo git@github.com:team/certs.git \
    --password-file ~/.config/asc/signing-sync-password --dry-run`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			syncPushCommand(d),
			syncPullCommand(d),
			syncRotatePasswordCommand(d),
			syncNukeCommand(d),
			syncImportCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func resolveSyncPassword(d *shared.Deps, passwordFile string) (string, error) {
	if passwordFile != "" && strings.TrimSpace(passwordFile) == "" {
		return "", d.UsageError("--password-file must not be empty")
	}
	if strings.TrimSpace(passwordFile) != "" {
		data, readErr := readProtectedSecretFile(passwordFile, "signing sync password")
		if readErr != nil {
			return "", readErr
		}
		password := trimPasswordFileNewline(string(data))
		if password == "" {
			return "", d.UsageError("signing sync password file is empty")
		}
		return password, nil
	}
	if password := os.Getenv(signingSyncPasswordEnvVar); password != "" {
		return password, nil
	}
	return "", d.UsageError("--password-file is required (or set ASC_SIGNING_SYNC_PASSWORD)")
}

func trimPasswordFileNewline(password string) string {
	if strings.HasSuffix(password, "\r\n") {
		return strings.TrimSuffix(password, "\r\n")
	}
	return strings.TrimSuffix(password, "\n")
}

func trimPasswordFileNewlineBytes(password []byte) []byte {
	switch length := len(password); {
	case length >= 2 && password[length-2] == '\r' && password[length-1] == '\n':
		return password[:length-2]
	case length >= 1 && password[length-1] == '\n':
		return password[:length-1]
	default:
		return password
	}
}

func readNonEmptyIdentityPasswordFile(d *shared.Deps, path string) ([]byte, error) {
	password, err := readProtectedSecretFile(path, "identity password")
	if err != nil {
		if err.Error() == "identity password file is empty" {
			return nil, d.UsageError(err.Error())
		}
		return nil, err
	}
	password = trimPasswordFileNewlineBytes(password)
	if len(password) == 0 {
		return nil, d.UsageError("identity password file is empty")
	}
	return password, nil
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

func syncPushCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("push", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Bundle identifier (required unless --targets-file is used)")
	matchExtensions := fs.Bool("match-extensions", false, "Also include registered bundle IDs that extend --bundle-id, such as <id>.widget (not with --targets-file; at most 32 targets)")
	strictMatch := fs.Bool("strict-match-identifier", false, "Include only the exact --bundle-id")
	targetsFile := fs.String("targets-file", "", "Command-root-relative JSON file containing 1-32 bundle targets (mutually exclusive with --bundle-id)")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, etc. (required)")
	repoURL := fs.String("repo", "", "Git repo URL for encrypted storage (required with --storage git)")
	passwordFile := fs.String("password-file", "", "Protected file containing the repository encryption password (or set ASC_SIGNING_SYNC_PASSWORD)")
	branch := fs.String("branch", "main", "Git branch")
	certType := fs.String("certificate-type", "", "Certificate types to match, comma-separated; --create-missing-certificate creates the first (default: inferred from --profile-type, such as DISTRIBUTION,IOS_DISTRIBUTION)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (requires --create-missing, --renew-expired, or --force-for-new-devices; required for development profiles with --create-missing unless --force-for-new-devices is set)")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	renewExpired := fs.Bool("renew-expired", false, "When no active profile exists, replace the most recently expired profile of --profile-type with a same-name profile that reuses active certificates (git storage only)")
	forceForNewDevices := fs.Bool("force-for-new-devices", false, "Recreate a development or ad hoc profile with the same name when its devices differ from --device or the enabled device list (git storage only)")
	includeMacInProfiles := fs.Bool("include-mac-in-profiles", false, "With --force-for-new-devices, also include enabled Apple silicon Macs in IOS_APP_DEVELOPMENT or IOS_APP_ADHOC profiles")
	createMissingCertificate := fs.Bool("create-missing-certificate", false, "Create a certificate when none are active, then create the profile")
	identityPath := fs.String("identity", "", "Protected PKCS#12 signing identity file")
	privateKeyPath := fs.String("private-key", "", "Protected RSA or EC private key PEM file")
	identitySHA256 := fs.String("identity-sha256", "", "SHA-256 certificate fingerprint selecting a PKCS#12 identity or the ASC certificate for --private-key")
	identityPasswordFile := fs.String("identity-password-file", "", "Protected file containing the source PKCS#12 password")
	storageFlags := bindSigningSyncStorageFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "push",
		ShortUsage: "asc signing sync push (--bundle-id ID | --targets-file PATH) --profile-type TYPE [--storage STORAGE] [storage options] [--password-file PATH]",
		ShortHelp:  "Fetch signing assets from ASC, encrypt, and publish to the selected storage.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			bundle := strings.TrimSpace(*bundleID)
			targetsPath := *targetsFile
			hasTargetsPath := strings.TrimSpace(targetsPath) != ""
			if targetsPath != "" && !hasTargetsPath {
				return d.UsageError("--targets-file must not be empty")
			}
			if bundle != "" && hasTargetsPath {
				return d.UsageError("--bundle-id and --targets-file are mutually exclusive")
			}
			if *matchExtensions && *strictMatch {
				return d.UsageError("--match-extensions and --strict-match-identifier are mutually exclusive")
			}
			if *matchExtensions && hasTargetsPath {
				return d.UsageError("--match-extensions requires --bundle-id; list extension targets in --targets-file instead")
			}
			var targetBundles []string
			if hasTargetsPath {
				var readErr error
				targetBundles, readErr = readSigningSyncTargetsFile(targetsPath)
				if readErr != nil {
					return d.UsageError(readErr.Error())
				}
			} else if bundle == "" {
				return d.UsageError("--bundle-id is required")
			} else {
				targetBundles = []string{bundle}
			}
			profType := strings.ToUpper(strings.TrimSpace(*profileType))
			if profType == "" {
				return d.UsageError("--profile-type is required")
			}
			repo := strings.TrimSpace(*repoURL)
			storageSelection, err := parseSigningSyncStorage(d, fs, storageFlags, repo, *branch)
			if err != nil {
				return err
			}
			if err := validateSigningSyncLifecycleFlags(d, profType, storageSelection.kind, *deviceIDs, *renewExpired, *forceForNewDevices, *includeMacInProfiles); err != nil {
				return err
			}
			if err := rejectDeviceWithoutCreateMissing(d, *deviceIDs, *createMissing || *renewExpired || *forceForNewDevices); err != nil {
				return err
			}
			if *createMissing && !*forceForNewDevices && isDevelopmentProfile(profType) && strings.TrimSpace(*deviceIDs) == "" {
				return d.UsageError("--device is required for development profiles with --create-missing")
			}
			identityInput := strings.TrimSpace(*identityPath)
			privateKeyInput := strings.TrimSpace(*privateKeyPath)
			if identityInput != "" && privateKeyInput != "" {
				return d.UsageError("--identity and --private-key are mutually exclusive")
			}
			if strings.TrimSpace(*identitySHA256) != "" && identityInput == "" && privateKeyInput == "" {
				return d.UsageError("--identity-sha256 requires --identity or --private-key")
			}
			if strings.TrimSpace(*identityPasswordFile) != "" && identityInput == "" && !*createMissingCertificate {
				return d.UsageError("--identity-password-file requires --identity")
			}
			if *createMissingCertificate && !*createMissing {
				return d.UsageError("--create-missing-certificate requires --create-missing")
			}
			if *createMissingCertificate && (identityInput != "" || privateKeyInput != "" || strings.TrimSpace(*identitySHA256) != "") {
				return d.UsageError("--create-missing-certificate cannot be combined with --identity, --private-key, or --identity-sha256")
			}
			if *createMissingCertificate && strings.TrimSpace(*identityPasswordFile) == "" {
				return shared.MissingRequiredUsageError("--identity-password-file")
			}
			if privateKeyInput != "" && strings.TrimSpace(*identitySHA256) == "" {
				return d.UsageError("--identity-sha256 is required with --private-key to select one App Store Connect certificate")
			}
			requestedFingerprint, fingerprintErr := normalizeCertificateFingerprint(*identitySHA256)
			if fingerprintErr != nil {
				return d.UsageError(fingerprintErr.Error())
			}
			pass, err := resolveSyncPassword(d, *passwordFile)
			if err != nil {
				return err
			}
			var certificatePassword []byte
			if *createMissingCertificate {
				certificatePassword, err = readNonEmptyIdentityPasswordFile(d, *identityPasswordFile)
				if err != nil {
					return fmt.Errorf("signing sync push: identity password: %w", err)
				}
				defer clear(certificatePassword)
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

			transport, err := storageSelection.transport(ctx, d)
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}
			if *matchExtensions && !hasTargetsPath {
				requestCtx, cancel := d.Timeout(ctx)
				expanded, expandErr := matchedSyncTargetBundles(requestCtx, client, bundle)
				cancel()
				if expandErr != nil {
					return fmt.Errorf("signing sync push: %w", expandErr)
				}
				targetBundles = expanded
			}
			partialResult := SyncResult{
				Operation:       "push",
				RepoURL:         transport.Locator(),
				Storage:         transport.Storage(),
				BundleID:        bundle,
				ProfileType:     profType,
				Files:           []string{},
				IdentityPresent: identity != nil,
			}
			if identity != nil {
				partialResult.IdentitySHA256 = identity.CertificateSHA256
			}

			if hasTargetsPath || len(targetBundles) > 1 {
				// The batch spans one lookup, asset resolution, and optional
				// profile creation per target plus the Git clone and push, so it
				// receives the command context and applies its own per-request
				// timeouts. Capping the whole run with a single request budget
				// would fail valid multi-target runs and, with --create-missing,
				// could abandon created profiles before publication.
				result, batchErr := runSigningSyncBatchForCommand(ctx, d, client, signingSyncBatchOptions{
					Transport:                transport,
					RepoURL:                  repo,
					Branch:                   *branch,
					Password:                 pass,
					ProfileType:              profType,
					CertificateType:          *certType,
					DeviceIDs:                shared.SplitCSV(*deviceIDs),
					CreateMissing:            *createMissing,
					CreateMissingCertificate: *createMissingCertificate,
					IdentityPassword:         certificatePassword,
					Identity:                 identity,
					BundleIDs:                targetBundles,
					RenewExpired:             *renewExpired,
					ForceForNewDevices:       *forceForNewDevices,
					IncludeMacDevices:        *includeMacInProfiles,
				})
				if batchErr != nil {
					if result.Partial {
						_ = d.PrintOutput(&result, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("signing sync push: %w", batchErr)
				}
				return d.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			// Fetch signing assets from ASC.
			fmt.Fprintln(d.IO.ErrOut, "Fetching signing assets from App Store Connect...")

			bundleIDResp, err := shared.FindBundleID(requestCtx, client, bundle)
			if err != nil {
				return fmt.Errorf("signing sync push: %w", err)
			}

			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
			if err != nil {
				return fmt.Errorf("signing sync push: create temp dir: %w", err)
			}

			store := newSigningSyncStore(transport, tmpDir, repo, *branch)
			defer func() { _ = store.Cleanup() }()

			prepareRepository := onceAfterSuccess(func() error {
				return transport.Fetch(ctx, store, true)
			})
			var identityArtifacts *signingIdentityArtifacts

			var createdIdentity createdSigningIdentity
			progress := &signingAssetsProgress{}
			profileCreated := false
			reportPartial := func(primary error) error {
				if !createdIdentity.CertificateAttempted && createdIdentity.CertificateID == "" && !progress.ProfileCreateAttempted && !progress.ReplacementAttempted && len(partialResult.Files) == 0 {
					return fmt.Errorf("signing sync push: %w", primary)
				}
				partialResult.Partial = true
				if partialResult.PublicationState == "" {
					partialResult.PublicationState = "unknown"
				}
				partialResult.IdentityPresent = identity != nil
				if identity != nil {
					partialResult.IdentitySHA256 = identity.CertificateSHA256
				}
				partialResult.CertificateIDs = extractIDs(progress.Certificates)
				applyCreatedIdentityToSyncResult(&partialResult, createdIdentity, createdIdentity.CertificateID != "" && createdIdentity.P12Path != "")
				if profileCreated {
					partialResult.ProfileCreationState = "created"
				} else if progress.ProfileCreateAttempted {
					partialResult.ProfileCreationState = "unknown"
				}
				_ = d.PrintOutput(&partialResult, *output.Output, *output.Pretty)
				return fmt.Errorf("signing sync push: %w", primary)
			}
			var certificateRequest signingCertificateCreateRequest
			certificateOutputs := &signingCertificateOutputs{BasePath: tmpDir}
			defer func() { _ = certificateOutputs.Close() }()
			if *createMissingCertificate {
				certificateRequest = signingCertificateCreateRequest{
					Outputs:  certificateOutputs,
					KeyPath:  filepath.Join(tmpDir, "created.key"),
					CSRPath:  filepath.Join(tmpDir, "created.csr"),
					P12Path:  filepath.Join(tmpDir, "created.p12"),
					Password: certificatePassword,
				}
			}
			profile, certs, created, err := resolveSigningAssets(
				requestCtx,
				client,
				signingAssetsOptions{
					Now:                      d.Now,
					BundleIDResourceID:       bundleIDResp.Data.ID,
					BundleIdentifier:         bundle,
					ProfileType:              profType,
					CertificateType:          *certType,
					DeviceIDs:                shared.SplitCSV(*deviceIDs),
					CreateMissing:            *createMissing,
					CreateMissingCertificate: *createMissingCertificate,
					CertificateCreate:        certificateRequest,
					CreatedIdentity:          &createdIdentity,
					Progress:                 progress,
					BeforeCertificateCreate: func(plan profileCreatePlan) error {
						if err := certificateOutputs.Prepare([]string{certificateRequest.KeyPath, certificateRequest.CSRPath, certificateRequest.P12Path}, false); err != nil {
							return err
						}
						if err := prepareRepository(); err != nil {
							return err
						}
						profileExtension := shared.ProvisioningProfileExtension("", profType)
						profilePath := filepath.Join("profiles", profileDirectoryName(profType), safeFileName(plan.ProfileName, "profile")+profileExtension)
						return preflightSigningAssetDestinationsForProfile(store, plan, profType, profilePath)
					},
					AfterCertificateCreate: func(created createdSigningIdentity) error {
						if identity != nil {
							return nil
						}
						loaded, loadErr := loadPKCS12Identity(created.P12Path, string(certificateRequest.Password), requestedFingerprint)
						if loadErr != nil {
							return loadErr
						}
						identity = loaded
						return nil
					},
					BeforeCreate: func(plan profileCreatePlan) error {
						if identity != nil {
							if err := preflightIdentityForProfileCreate(identity, plan, pass, d.Now()); err != nil {
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
						return d.Timeout(ctx)
					},
					CertificateFilter:  identityCertificateFilter(identity, d.Now),
					RenewExpired:       *renewExpired,
					ForceForNewDevices: *forceForNewDevices,
					IncludeMacDevices:  *includeMacInProfiles,
				},
			)
			partialResult.ProfileLifecycle = progress.Lifecycle
			if err != nil {
				return reportPartial(err)
			}
			partialResult.CertificateIDs = extractIDs(certs.Data)
			profileCreated = created
			if *createMissingCertificate && createdIdentity.CertificateID == "" {
				partialResult.CertificateCreationState = "reused"
			}
			if created {
				partialResult.ProfileCreationState = "created"
			} else if *createMissing {
				partialResult.ProfileCreationState = "reused"
			}
			applyCreatedIdentityToSyncResult(&partialResult, createdIdentity, createdIdentity.CertificateID != "" && createdIdentity.P12Path != "")
			if created {
				fmt.Fprintln(d.IO.ErrOut, "Created new profile")
			}
			reportSigningProfileLifecycle(d, bundle, progress.Lifecycle)
			if identity != nil {
				if err := validateIdentityForResolvedAssets(identity, profile, certs, bundle, profType, d.Now()); err != nil {
					return reportPartial(fmt.Errorf("validate signing identity: %w", err))
				}
			}
			if err := prepareRepository(); err != nil {
				return reportPartial(err)
			}
			profileContent, err := base64.StdEncoding.DecodeString(strings.TrimSpace(profile.Data.Attributes.ProfileContent))
			if err != nil {
				return reportPartial(fmt.Errorf("decode profile: %w", err))
			}
			profileDir := profileDirectoryName(profType)
			profileExtension := shared.ProvisioningProfileExtension(string(profile.Data.Attributes.Platform), profType)
			profileRelPath := filepath.Join("profiles", profileDir, safeFileName(profile.Data.Attributes.Name, profile.Data.ID)+profileExtension)
			profileRelPath, err = resolveCompatibleSigningProfilePath(store, profileRelPath)
			if err != nil {
				return reportPartial(fmt.Errorf("resolve profile repository path: %w", err))
			}
			profileMetadata, err := signingProfileArtifactMetadata(profile, bundle, profType)
			if err != nil {
				return reportPartial(fmt.Errorf("prepare profile metadata: %w", err))
			}
			if identity != nil {
				if identityArtifacts == nil {
					identityArtifacts, err = prepareSigningIdentityArtifacts(identity, pass, bundle, profType)
					if err != nil {
						return reportPartial(fmt.Errorf("prepare signing identity: %w", err))
					}
				}
				if err := bindSigningIdentityProfile(identityArtifacts, profile, profileRelPath, profileContent); err != nil {
					return reportPartial(fmt.Errorf("bind signing identity profile: %w", err))
				}
			}
			plannedPaths := signingAssetRepositoryPathsForProfile(certs.Data, profType, profileRelPath, identityArtifacts)
			if err := store.CheckEncryptedRepositoryPaths(plannedPaths); err != nil {
				return reportPartial(fmt.Errorf("preflight repository paths: %w", err))
			}
			if identity != nil {
				if err := preflightSigningIdentityArtifactsForContextUpdate(store, identityArtifacts, pass); err != nil {
					return reportPartial(fmt.Errorf("preflight signing identity: %w", err))
				}
			}
			if _, err := preflightSigningProfileArtifact(store, profileRelPath, profileContent, pass, profileMetadata); err != nil {
				return reportPartial(fmt.Errorf("preflight profile: %w", err))
			}

			// Write encrypted files.
			var files []string

			certDir := certDirectoryName(profType)
			for _, cert := range certs.Data {
				certContent, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cert.Attributes.CertificateContent))
				if err != nil {
					return reportPartial(fmt.Errorf("decode cert: %w", err))
				}
				relPath := filepath.Join("certs", certDir, safeFileName(cert.Attributes.SerialNumber, cert.ID)+".cer")
				if err := store.WriteEncryptedFile(relPath, certContent, pass); err != nil {
					return reportPartial(fmt.Errorf("encrypt cert: %w", err))
				}
				files = append(files, relPath)
				partialResult.Files = append(partialResult.Files, relPath)
				fmt.Fprintf(d.IO.ErrOut, "  Encrypted %s\n", relPath)
			}

			if err := writeOrReuseSigningProfileArtifact(store, profileRelPath, profileContent, pass, profileMetadata); err != nil {
				return reportPartial(fmt.Errorf("encrypt profile: %w", err))
			}
			files = append(files, profileRelPath)
			partialResult.Files = append(partialResult.Files, profileRelPath)
			fmt.Fprintf(d.IO.ErrOut, "  Encrypted %s\n", profileRelPath)

			var sensitiveFiles []string
			if identity != nil {
				if err := writeOrReuseSigningIdentityArtifacts(store, identityArtifacts, pass); err != nil {
					return reportPartial(fmt.Errorf("encrypt signing identity: %w", err))
				}
				files = append(files, identityArtifacts.IdentityPath, identityArtifacts.BindingPath)
				partialResult.Files = append(partialResult.Files, identityArtifacts.IdentityPath, identityArtifacts.BindingPath)
				sensitiveFiles = append(sensitiveFiles, identityArtifacts.IdentityPath)
				fmt.Fprintf(d.IO.ErrOut, "  Encrypted %s\n", identityArtifacts.IdentityPath)
				fmt.Fprintf(d.IO.ErrOut, "  Encrypted %s\n", identityArtifacts.BindingPath)
			}

			if replacedID := signingLifecycleReplacedProfileID(progress.Lifecycle); replacedID != "" {
				removed, removeErr := removeSupersededProfileArtifacts(store, pass, bundle, profType, replacedID, profileRelPath)
				if removeErr != nil {
					return reportPartial(fmt.Errorf("remove replaced profile artifacts: %w", removeErr))
				}
				for _, path := range removed {
					fmt.Fprintf(d.IO.ErrOut, "  Removed %s\n", path)
				}
			}

			// Publish every encrypted artifact through the selected storage.
			commitMsg := fmt.Sprintf("Update signing assets for %s (%s)", bundle, profType)
			if err := transport.Publish(ctx, store, commitMsg); err != nil {
				partialResult.PublicationState = "unknown"
				return reportPartial(err)
			}

			fmt.Fprintln(d.IO.ErrOut, "Done")

			result := partialResult
			result.Files = files
			result.SensitiveFiles = sensitiveFiles
			result.IdentityPresent = identity != nil
			result.PublicationState = "succeeded"
			if identity != nil {
				result.IdentitySHA256 = identity.CertificateSHA256
			}
			return d.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

func syncPullCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pull", flag.ExitOnError)

	repoURL := fs.String("repo", "", "Git repo URL (required with --storage git)")
	bundleID := fs.String("bundle-id", "", "Decrypt only one bundle target (requires --profile-type; mutually exclusive with --targets-file)")
	targetsFile := fs.String("targets-file", "", "Decrypt only the 1-32 bundle targets in a root-relative JSON file (requires --profile-type; mutually exclusive with --bundle-id)")
	profileType := fs.String("profile-type", "", "Profile type for --bundle-id or --targets-file")
	passwordFile := fs.String("password-file", "", "Protected file containing the repository encryption password (or set ASC_SIGNING_SYNC_PASSWORD)")
	branch := fs.String("branch", "main", "Git branch")
	outputDir := fs.String("output-dir", "./signing", "Output directory for decrypted files")
	p12Encoding := fs.String("p12-encoding", string(shared.PKCS12EncodingLegacy), "Installer output only: "+shared.PKCS12EncodingFlagUsage+"; requires a full pull containing an active installer identity")
	sourceFormat := fs.String("source-format", "", "Set to openssl to read an OpenSSL-encrypted signing repository laid out as certs/<type>/ and profiles/<type>/ from --repo (a Git URL or local checkout; --branch then defaults to master) instead of a signing sync store; read-only, Git only")
	storageFlags := bindSigningSyncStorageFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc signing sync pull [--storage STORAGE] [storage options] [--source-format openssl] [--bundle-id ID | --targets-file PATH] [--profile-type TYPE] [--password-file PATH] [--output-dir DIR] [--p12-encoding legacy|modern]",
		ShortHelp:  "Pull and decrypt signing assets from the selected storage.",
		LongHelp: `A full pull restores currently valid standalone installer identities as well as profile-bound app signing assets.
Bundle-specific pulls return only that bundle's profile-bound assets.

Installer output defaults to --p12-encoding legacy (3DES and a SHA-1 MAC) for native macOS SecItemImport compatibility. Use --p12-encoding modern for AES-256-CBC and HMAC-SHA-256 when your importer supports it, such as SecPKCS12Import. Explicit --p12-encoding requires a full pull containing an active installer identity; it cannot be used with --bundle-id or --targets-file.

Conversion happens in memory; no unencrypted private key is written. Stored identities and app identity output keep their existing encryption.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			repo := strings.TrimSpace(*repoURL)
			provided := providedSigningSyncFlags(fs)
			var installerEncoding shared.PKCS12Encoding
			if provided["p12-encoding"] {
				var err error
				installerEncoding, err = shared.ParsePKCS12Encoding(*p12Encoding)
				if err != nil {
					return d.UsageError(err.Error())
				}
				if provided["bundle-id"] || provided["targets-file"] {
					return d.UsageError("--p12-encoding requires a full pull without --bundle-id or --targets-file")
				}
			}
			openSSLSource, sourceBranch, err := parseSigningPullSourceFormat(d, provided, *sourceFormat, *storageFlags.storage, repo, *branch)
			if err != nil {
				return err
			}
			storageSelection, err := parseSigningSyncStorage(d, fs, storageFlags, repo, *branch)
			if err != nil {
				return err
			}
			bundle := strings.TrimSpace(*bundleID)
			profile := strings.ToUpper(strings.TrimSpace(*profileType))
			bundleProvided := provided["bundle-id"]
			targetsProvided := provided["targets-file"]
			profileProvided := provided["profile-type"]
			if bundleProvided && bundle == "" {
				return d.UsageError("--bundle-id must not be empty")
			}
			if targetsProvided && strings.TrimSpace(*targetsFile) == "" {
				return d.UsageError("--targets-file must not be empty")
			}
			if profileProvided && profile == "" {
				return d.UsageError("--profile-type must not be empty")
			}
			if bundleProvided && targetsProvided {
				return d.UsageError("--bundle-id and --targets-file are mutually exclusive")
			}
			selectionRequested := bundleProvided || targetsProvided
			if selectionRequested && profile == "" {
				return d.UsageError("--profile-type is required with --bundle-id or --targets-file")
			}
			if !selectionRequested && profileProvided {
				return d.UsageError("--profile-type requires --bundle-id or --targets-file")
			}
			if selectionRequested {
				var normalizeErr error
				profile, normalizeErr = normalizeSigningPullProfileType(profile)
				if normalizeErr != nil {
					return d.UsageError(normalizeErr.Error())
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
					return d.UsageError(readErr.Error())
				}
			}
			pass, err := resolveSyncPassword(d, *passwordFile)
			if err != nil {
				return err
			}

			outDir := strings.TrimSpace(*outputDir)
			if outDir == "" {
				outDir = "./signing"
			}
			if openSSLSource {
				result, err := pullOpenSSLSigningSource(ctx, d, repo, sourceBranch, pass, outDir, selectedBundleIDs, profile, targetsProvided, installerEncoding)
				if err != nil {
					return err
				}
				return d.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			transport, err := storageSelection.transport(ctx, d)
			if err != nil {
				return err
			}

			// Fetch the encrypted artifacts into an isolated working tree.
			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
			if err != nil {
				return fmt.Errorf("signing sync pull: create temp dir: %w", err)
			}

			store := newSigningSyncStore(transport, tmpDir, repo, *branch)
			defer func() { _ = store.Cleanup() }()

			if err := transport.Fetch(ctx, store, false); err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}

			// List and decrypt all files.
			encryptedFiles, err := store.ListEncryptedFiles()
			if err != nil {
				return fmt.Errorf("signing sync pull: list files: %w", err)
			}

			if len(encryptedFiles) == 0 && installerEncoding == "" {
				if selectionRequested {
					return fmt.Errorf("signing sync pull: no active %s profile found in encrypted repository for bundle ID(s): %s", profile, strings.Join(selectedBundleIDs, ", "))
				}
				fmt.Fprintln(d.IO.ErrOut, "No encrypted signing files found in repo")
				result := SyncResult{
					Operation: "pull",
					RepoURL:   transport.Locator(),
					Storage:   transport.Storage(),
					Files:     []string{},
				}
				return d.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			decrypted, err := decryptAndValidateSigningFiles(d, store, encryptedFiles, pass)
			if err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}
			result, err := writeSigningPullFiles(d, SyncResult{Operation: "pull", RepoURL: transport.Locator(), Storage: transport.Storage()},
				decrypted, pass, outDir, selectedBundleIDs, profile, targetsProvided, installerEncoding)
			if err != nil {
				return err
			}
			return d.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// writeSigningPullFiles applies the bundle selection, checks every
// destination, and then writes the decrypted files under outDir.
func writeSigningPullFiles(d *shared.Deps, result SyncResult, decrypted []decryptedSigningFile, password, outDir string, bundleIDs []string, profileType string, batch bool, installerEncoding shared.PKCS12Encoding) (SyncResult, error) {
	var targets []SyncTargetResult
	if len(bundleIDs) > 0 {
		var err error
		decrypted, targets, err = selectSigningPullFiles(d, decrypted, bundleIDs, profileType)
		if err != nil {
			return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
		}
	}
	installerPresent := false
	encoding := installerEncoding
	if encoding == "" {
		encoding = shared.PKCS12EncodingLegacy
	}
	for index := range decrypted {
		file := &decrypted[index]
		if file.Metadata.Kind != installerIdentityKind {
			continue
		}
		installerPresent = true
		key, certificate, err := modernpkcs12.Decode(file.Plaintext, password)
		if err != nil {
			return SyncResult{}, fmt.Errorf("signing sync pull: decode installer identity: %w", err)
		}
		file.Plaintext, err = shared.EncodePKCS12Identity(encoding, key, certificate, password)
		if err != nil {
			return SyncResult{}, fmt.Errorf("signing sync pull: encode installer identity: %w", err)
		}
	}
	if installerEncoding != "" && !installerPresent {
		return SyncResult{}, fmt.Errorf("signing sync pull: --p12-encoding requires at least one active installer identity")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: create output dir: %w", err)
	}
	outputRoot, err := rootfs.New(outDir)
	if err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: create output root: %w", err)
	}
	defer outputRoot.Close()
	if err := preflightSigningPullFilesInRoot(outputRoot, decrypted); err != nil {
		return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
	}
	for _, file := range decrypted {
		if err := writeDecryptedOutputFileInRoot(outputRoot, file.RelativePath, file.Plaintext, file.Sensitive); err != nil {
			return SyncResult{}, fmt.Errorf("signing sync pull: %w", err)
		}
		result.Files = append(result.Files, file.RelativePath)
		if file.Sensitive {
			result.SensitiveFiles = append(result.SensitiveFiles, file.RelativePath)
		}
		if file.Identity {
			result.IdentityPresent = true
		}
		fmt.Fprintf(d.IO.ErrOut, "  Decrypted %s\n", file.RelativePath)
	}
	fmt.Fprintf(d.IO.ErrOut, "Done — %d files written to %s\n", len(result.Files), outDir)
	if len(bundleIDs) > 0 {
		applySigningPullSelectionResult(&result, profileType, bundleIDs, targets, batch)
	}
	return result, nil
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

func prepareDecryptedSigningFiles(d *shared.Deps, store *signingpkg.GitStore, encryptedFiles []string, password, outDir string) ([]decryptedSigningFile, error) {
	root, err := rootfs.New(outDir)
	if err != nil {
		return nil, fmt.Errorf("create output root: %w", err)
	}
	defer root.Close()
	return prepareDecryptedSigningFilesInRoot(d, store, encryptedFiles, password, root)
}

func prepareDecryptedSigningFilesInRoot(d *shared.Deps, store *signingpkg.GitStore, encryptedFiles []string, password string, root rootfs.Root) ([]decryptedSigningFile, error) {
	decrypted, err := decryptAndValidateSigningFiles(d, store, encryptedFiles, password)
	if err != nil {
		return nil, err
	}
	if err := preflightSigningPullFilesInRoot(root, decrypted); err != nil {
		return nil, err
	}
	return decrypted, nil
}

func decryptAndValidateSigningFiles(d *shared.Deps, store *signingpkg.GitStore, encryptedFiles []string, password string) ([]decryptedSigningFile, error) {
	decrypted, activeIdentityPaths, err := loadAndValidateSigningFiles(d, store, encryptedFiles, password)
	if err != nil {
		return nil, err
	}
	filtered := decrypted[:0]
	for _, file := range decrypted {
		if file.Identity {
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

func loadAndValidateSigningFiles(d *shared.Deps, store *signingpkg.GitStore, encryptedFiles []string, password string) ([]decryptedSigningFile, map[string]struct{}, error) {
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
	activeIdentityPaths, err := validateDecryptedSigningFiles(d, decrypted, password)
	if err != nil {
		return nil, nil, err
	}
	return decrypted, activeIdentityPaths, nil
}

// validateDecryptedSigningFiles checks the identity links between decrypted
// files and returns the active identity paths, each currently valid.
func validateDecryptedSigningFiles(d *shared.Deps, decrypted []decryptedSigningFile, password string) (map[string]struct{}, error) {
	activeIdentityPaths, err := validateIdentityArtifactGraph(d, decrypted)
	if err != nil {
		return nil, err
	}
	for _, file := range decrypted {
		if !file.Identity {
			continue
		}
		canonicalPath := strings.ReplaceAll(filepath.ToSlash(file.RelativePath), `\`, "/")
		if _, active := activeIdentityPaths[canonicalPath]; !active && file.Metadata.Kind != installerIdentityKind {
			continue
		}
		_, certificate, err := modernpkcs12.Decode(file.Plaintext, password)
		if err != nil || certificate == nil {
			return nil, fmt.Errorf("active identity core is not a decodable PKCS#12 identity")
		}
		if now := d.Now(); now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
			if file.Metadata.Kind == installerIdentityKind {
				continue
			}
			return nil, fmt.Errorf("active identity certificate is not currently valid")
		}
		if file.Metadata.Kind == installerIdentityKind {
			activeIdentityPaths[canonicalPath] = struct{}{}
		}
	}
	return activeIdentityPaths, nil
}

func validateIdentityArtifactGraph(d *shared.Deps, files []decryptedSigningFile) (map[string]struct{}, error) {
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
		if !profile.ExpirationDate.After(d.Now()) || !containsFold(profile.TeamIdentifier, binding.TeamID) {
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
	installerPath := len(parts) == 3 && parts[0] == "identities" && isInstallerIdentityCategory(parts[1]) && strings.HasSuffix(parts[2], ".p12")
	identityKind := "pkcs12-identity"
	if installerPath {
		identityKind = installerIdentityKind
	}
	identityPath := installerPath || len(parts) == 3 && parts[0] == "identities" &&
		(parts[1] == "distribution" || parts[1] == "development") && strings.HasSuffix(parts[2], ".p12")
	identityLikePath := strings.HasSuffix(strings.ToLower(canonicalPath), ".p12") ||
		strings.HasPrefix(strings.ToLower(canonicalPath), "identities/")
	if identityLikePath || metadata.Kind == "pkcs12-identity" || metadata.Kind == installerIdentityKind {
		if !identityPath || metadata.Version != 1 || metadata.Kind != identityKind || !metadata.Sensitive {
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
		if installerPath {
			if err := validateInstallerIdentityPurpose(certificate, parts[1]); err != nil {
				return false, false, err
			}
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

// matchedSyncTargetBundles expands --match-extensions into the parent bundle
// ID and its registered extensions, within the batch target limit.
func matchedSyncTargetBundles(ctx context.Context, client *asc.Client, bundle string) ([]string, error) {
	expanded, err := listSigningBundleIDs(ctx, client, bundle, true)
	if err != nil {
		return nil, err
	}
	if len(expanded) > maxSigningSyncTargets {
		return nil, fmt.Errorf("--match-extensions matched %d bundle IDs, more than the %d targets one run supports; split them across --targets-file runs", len(expanded), maxSigningSyncTargets)
	}
	targets := make([]string, 0, len(expanded))
	for _, item := range expanded {
		targets = append(targets, strings.TrimSpace(item.Attributes.Identifier))
	}
	return targets, nil
}
