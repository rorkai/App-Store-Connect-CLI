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

	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

const (
	matchPasswordEnvVar       = "ASC_MATCH_PASSWORD"
	signingSyncPasswordEnvVar = "ASC_SIGNING_SYNC_PASSWORD"
	maxEncryptedSigningFiles  = 256
	maxEncryptedSigningBytes  = 128 << 20
)

// SyncResult is the structured output for sync operations.
type SyncResult struct {
	Operation       string   `json:"operation"`
	RepoURL         string   `json:"repoUrl"`
	BundleID        string   `json:"bundleId"`
	ProfileType     string   `json:"profileType"`
	Files           []string `json:"files"`
	IdentityPresent bool     `json:"identityPresent"`
	IdentitySHA256  string   `json:"identitySha256,omitempty"`
	SensitiveFiles  []string `json:"sensitiveFiles,omitempty"`
}

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

Lightweight alternative to fastlane match. Fetches signing assets from
App Store Connect, encrypts them, and stores in a shared git repo.
Team members pull and decrypt to get signing files.

Examples:
  asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
    --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

  asc signing sync pull --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password \
    --output-dir ./signing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			syncPushCommand(),
			syncPullCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func resolvePassword(flagValue string) (string, error) {
	password := strings.TrimSpace(flagValue)
	if password != "" {
		return password, nil
	}
	password = strings.TrimSpace(os.Getenv(matchPasswordEnvVar))
	if password != "" {
		return password, nil
	}
	return "", shared.UsageError("--password is required (or set ASC_MATCH_PASSWORD)")
}

func resolveSyncPassword(passwordFile, legacyFlagValue string) (password string, legacy bool, err error) {
	if passwordFile != "" && strings.TrimSpace(passwordFile) == "" {
		return "", false, shared.UsageError("--password-file must not be empty")
	}
	if strings.TrimSpace(passwordFile) != "" {
		data, readErr := readProtectedSecretFile(passwordFile, "signing sync password")
		if readErr != nil {
			return "", false, readErr
		}
		password = trimPasswordFileNewline(string(data))
		if password == "" {
			return "", false, shared.UsageError("signing sync password file is empty")
		}
		return password, false, nil
	}
	if strings.TrimSpace(legacyFlagValue) != "" {
		password, err = resolvePassword(legacyFlagValue)
		return password, true, err
	}
	if password = os.Getenv(signingSyncPasswordEnvVar); password != "" {
		return password, false, nil
	}
	password, err = resolvePassword(legacyFlagValue)
	if err != nil {
		return "", false, shared.UsageError("--password-file is required (or set ASC_SIGNING_SYNC_PASSWORD; legacy --password and ASC_MATCH_PASSWORD remain available through 4.x)")
	}
	return password, err == nil, err
}

func warnLegacySyncPassword() {
	fmt.Fprintln(os.Stderr, "Warning: --password and ASC_MATCH_PASSWORD are deprecated for signing sync; use --password-file or ASC_SIGNING_SYNC_PASSWORD. The legacy sources will be removed in 5.0.0.")
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

func syncPushCommand() *ffcli.Command {
	fs := flag.NewFlagSet("push", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Bundle identifier (required)")
	profileType := fs.String("profile-type", "", "Profile type: IOS_APP_STORE, IOS_APP_DEVELOPMENT, etc. (required)")
	repoURL := fs.String("repo", "", "Git repo URL for encrypted storage (required)")
	password := fs.String("password", "", "Deprecated: encryption password (or ASC_MATCH_PASSWORD env); use --password-file")
	passwordFile := fs.String("password-file", "", "[experimental] Protected file containing the repository encryption password")
	branch := fs.String("branch", "main", "Git branch")
	certType := fs.String("certificate-type", "", "Certificate type filter (optional)")
	deviceIDs := fs.String("device", "", "Device ID(s), comma-separated (required with --create-missing for development profiles; deprecated and ignored without it until 5.0.0)")
	createMissing := fs.Bool("create-missing", false, "Create missing profiles")
	identityPath := fs.String("identity", "", "[experimental] Protected PKCS#12 signing identity file")
	privateKeyPath := fs.String("private-key", "", "[experimental] Protected RSA or EC private key PEM file")
	identitySHA256 := fs.String("identity-sha256", "", "[experimental] SHA-256 certificate fingerprint selecting a PKCS#12 identity or the ASC certificate for --private-key")
	identityPasswordFile := fs.String("identity-password-file", "", "[experimental] Protected file containing the source PKCS#12 password")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "push",
		ShortUsage: "asc signing sync push --bundle-id ID --profile-type TYPE --repo URL [--password-file PATH]",
		ShortHelp:  "Fetch signing assets from ASC, encrypt, and push to git.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			bundle := strings.TrimSpace(*bundleID)
			if bundle == "" {
				return shared.UsageError("--bundle-id is required")
			}
			profType := strings.ToUpper(strings.TrimSpace(*profileType))
			if profType == "" {
				return shared.UsageError("--profile-type is required")
			}
			repo := strings.TrimSpace(*repoURL)
			if repo == "" {
				return shared.UsageError("--repo is required")
			}
			warnDeviceWithoutCreateMissing(*deviceIDs, *createMissing)
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
			if (identityInput != "" || privateKeyInput != "") && isDirectDistributionProfile(profType) {
				return shared.UsageErrorf("private identity sync does not support --profile-type %s yet; omit --identity/--private-key", profType)
			}
			requestedFingerprint, fingerprintErr := normalizeCertificateFingerprint(*identitySHA256)
			if fingerprintErr != nil {
				return shared.UsageError(fingerprintErr.Error())
			}
			if strings.TrimSpace(*passwordFile) != "" && *password != "" {
				return shared.UsageError("--password-file and --password are mutually exclusive")
			}

			pass, legacyPassword, err := resolveSyncPassword(*passwordFile, *password)
			if err != nil {
				return err
			}
			if legacyPassword {
				warnLegacySyncPassword()
			}

			var identity *signingIdentity
			switch {
			case identityInput != "":
				identityPassword := ""
				if strings.TrimSpace(*identityPasswordFile) != "" {
					passwordBytes, readErr := readProtectedSecretFile(*identityPasswordFile, "identity password")
					if readErr != nil {
						return shared.UsageErrorf("--identity-password-file: %v", readErr)
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
						if err := preflightSigningAssetDestinations(store, plan, profType); err != nil {
							return err
						}
						if identity != nil {
							identityArtifacts, err = prepareSigningIdentityArtifacts(identity, pass, bundle, profType)
							if err != nil {
								return err
							}
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
			profileRelPath := filepath.Join("profiles", profileDir, safeFileName(profile.Data.Attributes.Name, profile.Data.ID)+".mobileprovision")
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
				if err := preflightSigningIdentityArtifactsForContextUpdate(store, identityArtifacts, pass); err != nil {
					return fmt.Errorf("signing sync push: preflight signing identity: %w", err)
				}
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

			if err := store.WriteEncryptedFile(profileRelPath, profileContent, pass); err != nil {
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
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func syncPullCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)

	repoURL := fs.String("repo", "", "Git repo URL (required)")
	password := fs.String("password", "", "Deprecated: decryption password (or ASC_MATCH_PASSWORD env); use --password-file")
	passwordFile := fs.String("password-file", "", "[experimental] Protected file containing the repository encryption password")
	branch := fs.String("branch", "main", "Git branch")
	outputDir := fs.String("output-dir", "./signing", "Output directory for decrypted files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc signing sync pull --repo URL [--password-file PATH] [--output-dir DIR]",
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
			if strings.TrimSpace(*passwordFile) != "" && *password != "" {
				return shared.UsageError("--password-file and --password are mutually exclusive")
			}
			pass, legacyPassword, err := resolveSyncPassword(*passwordFile, *password)
			if err != nil {
				return err
			}
			if legacyPassword {
				warnLegacySyncPassword()
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
				fmt.Fprintln(os.Stderr, "No encrypted signing files found in repo")
				result := SyncResult{
					Operation: "pull",
					RepoURL:   sanitizeRepoURLForOutput(repo),
					Files:     []string{},
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("signing sync pull: create output dir: %w", err)
			}

			decrypted, err := prepareDecryptedSigningFiles(store, encryptedFiles, pass, outDir)
			if err != nil {
				return fmt.Errorf("signing sync pull: %w", err)
			}

			var files []string
			var sensitiveFiles []string
			identityPresent := false
			for _, file := range decrypted {
				if err := writeDecryptedOutputFile(outDir, file.RelativePath, file.Plaintext, file.Sensitive); err != nil {
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
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func prepareDecryptedSigningFiles(store *signingpkg.GitStore, encryptedFiles []string, password, outDir string) ([]decryptedSigningFile, error) {
	if len(encryptedFiles) > maxEncryptedSigningFiles {
		return nil, fmt.Errorf("encrypted signing repository contains %d files; limit is %d", len(encryptedFiles), maxEncryptedSigningFiles)
	}
	var cumulativeSize int64
	for _, relPath := range encryptedFiles {
		size, err := store.EncryptedFileSize(relPath)
		if err != nil {
			return nil, fmt.Errorf("inspect encrypted artifact %s: %w", relPath, err)
		}
		if size < 0 || size > maxEncryptedSigningBytes-cumulativeSize {
			return nil, fmt.Errorf("encrypted signing repository exceeds the %d-byte cumulative size limit", maxEncryptedSigningBytes)
		}
		cumulativeSize += size
	}
	decrypted := make([]decryptedSigningFile, 0, len(encryptedFiles))
	for _, relPath := range encryptedFiles {
		plaintext, metadata, err := store.ReadEncryptedFileWithMetadata(relPath, password)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s: %w", relPath, err)
		}
		sensitive, identity, err := classifySigningFile(relPath, plaintext, metadata, password)
		if err != nil {
			return nil, fmt.Errorf("validate %s: %w", relPath, err)
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
		return nil, err
	}
	filtered := decrypted[:0]
	for _, file := range decrypted {
		if file.Metadata.Kind == "pkcs12-identity" {
			canonicalPath := strings.ReplaceAll(filepath.ToSlash(file.RelativePath), `\`, "/")
			if _, active := activeIdentityPaths[canonicalPath]; !active {
				continue
			}
			_, certificate, err := modernpkcs12.Decode(file.Plaintext, password)
			if err != nil || certificate == nil {
				return nil, fmt.Errorf("active identity core is not a decodable PKCS#12 identity")
			}
			if now := time.Now(); now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
				return nil, fmt.Errorf("active identity certificate is not currently valid")
			}
		}
		filtered = append(filtered, file)
	}
	decrypted = filtered

	root, err := rootfs.New(outDir)
	if err != nil {
		return nil, fmt.Errorf("create output root: %w", err)
	}
	for _, file := range decrypted {
		if file.Sensitive {
			err = root.CheckCreateNewFile(file.RelativePath)
		} else {
			err = root.CheckWriteFilePreservingMode(file.RelativePath)
		}
		if err != nil {
			return nil, fmt.Errorf("preflight output %s: %w", file.RelativePath, err)
		}
	}
	return decrypted, nil
}

func validateIdentityArtifactGraph(files []decryptedSigningFile) (map[string]struct{}, error) {
	type identityCore struct {
		teamID            string
		certificateSHA256 string
		contextCount      int
	}
	cores := make(map[string]identityCore)
	activeCores := make(map[string]struct{})
	contexts := make(map[string]string)
	profiles := make(map[string][]byte)
	for _, file := range files {
		canonicalPath := strings.ReplaceAll(filepath.ToSlash(file.RelativePath), `\`, "/")
		profiles[canonicalPath] = file.Plaintext
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
		profileContent, exists := profiles[binding.ProfilePath]
		if !exists {
			return nil, fmt.Errorf("identity context has no matching profile artifact")
		}
		profileDigest := sha256.Sum256(profileContent)
		if !strings.EqualFold(hex.EncodeToString(profileDigest[:]), binding.ProfileSHA256) {
			return nil, fmt.Errorf("identity context profile digest does not match profile artifact")
		}
		profile, err := parseIdentityMobileProvision(profileContent)
		if err != nil {
			return nil, fmt.Errorf("identity context profile is invalid: %w", err)
		}
		if strings.TrimSpace(profile.UUID) == "" || profile.UUID != binding.ProfileUUID {
			return nil, fmt.Errorf("identity context profile UUID does not match profile artifact")
		}
		if !profile.ExpirationDate.After(time.Now()) || !containsFold(profile.TeamIdentifier, binding.TeamID) {
			return nil, fmt.Errorf("identity context profile is expired or belongs to another team")
		}
		if !identityProfileTypeMatches(profile, binding.ProfileType) {
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
