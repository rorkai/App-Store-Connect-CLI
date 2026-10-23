package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	pkcs12 "github.com/bitrise-io/go-pkcs12"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

const maxOpenSSLImportFileSize = 4 << 20

type openSSLImportCertificate struct {
	sourcePath string
	sourceType string
	category   string
	cert       *x509.Certificate
	privateKey any
	sha256     string
}

type openSSLImportProfile struct {
	sourcePath  string
	content     []byte
	profile     *identityMobileProvision
	uuid        string
	bundleID    string
	profileType string
}

type openSSLImportWrite struct {
	relPath   string
	plaintext []byte
	metadata  *signingpkg.EncryptedFileMetadata
	identity  *openSSLImportCertificate
}

func syncImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("import", flag.ExitOnError)

	source := fs.String("source", "", "Local checkout directory or Git URL of the OpenSSL-encrypted signing repository (required)")
	sourceBranch := fs.String("source-branch", "master", "Git branch of --source when it is a Git URL")
	sourcePassword := fs.String("source-password", "", "Password of the OpenSSL-encrypted signing repository; pass @env:NAME or @file:PATH to keep it off the command line (required)")
	repoURL := fs.String("repo", "", "Git repo URL of the destination signing sync store (required with --storage git)")
	branch := fs.String("branch", "main", "Git branch of the destination store")
	passwordFile := fs.String("password-file", "", "Protected file containing the destination store encryption password (or set ASC_SIGNING_SYNC_PASSWORD)")
	storageFlags := bindSigningSyncStorageFlags(fs)
	confirm := fs.Bool("confirm", false, "Confirm writing the imported artifacts to the destination store")
	dryRun := fs.Bool("dry-run", false, "Decrypt and verify the source and list what would be imported without writing anything")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc signing sync import --source PATH|URL --source-password VALUE [--storage STORAGE] [storage options] [--password-file PATH] (--confirm | --dry-run)",
		ShortHelp:  "Import an OpenSSL-encrypted signing repository into a signing sync store.",
		LongHelp: `Import certificates, private keys, and provisioning profiles from an
OpenSSL-encrypted signing repository (v1 and v2 file formats) into a signing
sync store.

The source uses a certs/<type>/ and profiles/<type>/ layout: each certificate
is <id>.cer with its private key in <id>.p12 (a PEM private key or a PKCS#12
file without a password), and each profile is a .mobileprovision or
.provisionprofile file. Every file is decrypted with --source-password. Each
certificate must pair with its private key and each profile must carry a
valid signature, or nothing is imported. Wildcard profiles are skipped with a
warning because signing sync stores only explicit bundle ID profiles.

Imported assets are re-encrypted in the signing sync format with the
destination password and published through the selected storage, with the
same conflict checks as push. An artifact already in the store is reused; a
different artifact at the same path stops the import. A private key is linked
to a profile only while the certificate and profile are both valid; a macOS
profile without a linked key is returned only by a pull without
--profile-type.

Import never contacts App Store Connect, never revokes or creates anything,
and never touches a keychain. --dry-run lists what would be imported without
reading or writing the destination store.

Examples:
  asc signing sync import --source ./certificates --source-password @env:SOURCE_PASSWORD \
    --repo git@github.com:team/signing.git --dry-run

  asc signing sync import --source git@github.com:team/certificates.git --source-password @env:SOURCE_PASSWORD \
    --repo git@github.com:team/signing.git --password-file ~/.config/asc/signing-sync-password --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			sourceLocation := strings.TrimSpace(*source)
			if sourceLocation == "" {
				return shared.UsageError("--source is required")
			}
			if *sourcePassword == "" {
				return shared.UsageError("--source-password is required")
			}
			if *confirm && *dryRun {
				return shared.UsageError("--confirm and --dry-run are mutually exclusive")
			}
			if !*confirm && !*dryRun {
				return shared.UsageError("--confirm is required to write the destination store (or pass --dry-run to preview)")
			}
			storageSelection, err := parseSigningSyncStorage(fs, storageFlags, *repoURL, *branch)
			if err != nil {
				return err
			}
			var destinationPassword string
			if *confirm {
				destinationPassword, err = resolveSyncPassword(*passwordFile)
				if err != nil {
					return err
				}
			}

			sourceRoot, sourceLabel, cleanup, err := openOpenSSLImportSource(ctx, sourceLocation, strings.TrimSpace(*sourceBranch))
			if err != nil {
				return fmt.Errorf("signing sync import: %w", err)
			}
			defer cleanup()
			certificates, profiles, err := loadOpenSSLImportSource(sourceRoot, *sourcePassword)
			if err != nil {
				return fmt.Errorf("signing sync import: %w", err)
			}

			result := asc.SigningSyncImportResult{
				Operation:        "import",
				Source:           sourceLabel,
				DryRun:           *dryRun,
				Certificates:     openSSLImportCertificateResults(certificates),
				Profiles:         openSSLImportProfileResults(profiles),
				PublicationState: "skipped",
			}
			now := time.Now()
			if *dryRun {
				writes, err := planOpenSSLImportWrites(certificates, profiles, nil, now)
				if err != nil {
					return fmt.Errorf("signing sync import: %w", err)
				}
				result.Files = openSSLImportWritePaths(writes)
				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			}

			transport, err := storageSelection.transport(ctx)
			if err != nil {
				return err
			}
			result.RepoURL = transport.Locator()
			result.Storage = transport.Storage()
			tmpDir, err := os.MkdirTemp("", "asc-signing-sync-import-*")
			if err != nil {
				return fmt.Errorf("signing sync import: create temp dir: %w", err)
			}
			store := newSigningSyncStore(transport, tmpDir, storageSelection.repoURL, *branch)
			defer func() { _ = store.Cleanup() }()
			if err := transport.Fetch(ctx, store, true); err != nil {
				return fmt.Errorf("signing sync import: %w", err)
			}
			existingPaths, err := store.ListEncryptedFiles()
			if err != nil {
				return fmt.Errorf("signing sync import: list destination: %w", err)
			}
			existing, _, err := loadAndValidateSigningFiles(store, existingPaths, destinationPassword)
			if err != nil {
				return fmt.Errorf("signing sync import: destination store: %w", err)
			}
			writes, err := planOpenSSLImportWrites(certificates, profiles, existing, now)
			if err != nil {
				return fmt.Errorf("signing sync import: %w", err)
			}
			written, err := applyOpenSSLImportWrites(store, writes, destinationPassword)
			if err != nil {
				return fmt.Errorf("signing sync import: %w", err)
			}
			result.Files = written
			if len(written) == 0 {
				result.PublicationState = "not-needed"
				fmt.Fprintln(os.Stderr, "Destination store already contains every imported artifact")
				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			}
			allPaths, err := store.ListEncryptedFiles()
			if err != nil {
				return fmt.Errorf("signing sync import: list destination: %w", err)
			}
			if _, _, err := loadAndValidateSigningFiles(store, allPaths, destinationPassword); err != nil {
				return fmt.Errorf("signing sync import: validate imported store: %w", err)
			}
			if err := transport.Publish(ctx, store, "Import signing assets from OpenSSL-encrypted repository"); err != nil {
				return fmt.Errorf("signing sync import: %w", err)
			}
			result.PublicationState = "succeeded"
			fmt.Fprintf(os.Stderr, "Done — %d encrypted signing files imported\n", len(written))
			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

func openOpenSSLImportSource(ctx context.Context, location, branch string) (rootfs.Root, string, func(), error) {
	if info, err := os.Stat(location); err == nil && info.IsDir() {
		root, err := rootfs.New(location)
		if err != nil {
			return rootfs.Root{}, "", nil, fmt.Errorf("open source: %w", err)
		}
		return root, location, func() { _ = root.Close() }, nil
	}
	if branch == "" {
		return rootfs.Root{}, "", nil, shared.UsageError("--source-branch must not be empty")
	}
	tmpDir, err := os.MkdirTemp("", "asc-signing-sync-import-source-*")
	if err != nil {
		return rootfs.Root{}, "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	clone := &signingpkg.GitStore{RepoURL: location, LocalDir: filepath.Join(tmpDir, "source"), Branch: branch}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	fmt.Fprintln(os.Stderr, "Cloning source repository...")
	if err := clone.Clone(ctx, false); err != nil {
		cleanup()
		return rootfs.Root{}, "", nil, fmt.Errorf("clone source: %w", err)
	}
	root, err := rootfs.New(clone.LocalDir)
	if err != nil {
		cleanup()
		return rootfs.Root{}, "", nil, fmt.Errorf("open source: %w", err)
	}
	return root, sanitizeRepoURLForOutput(location), func() { _ = root.Close(); cleanup() }, nil
}

func listOpenSSLImportFiles(root rootfs.Root, top string) ([]string, error) {
	topDir, err := root.OpenDir(top)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open source %s: %w", top, err)
	}
	types, err := topDir.ReadDir(-1)
	_ = topDir.Close()
	if err != nil {
		return nil, fmt.Errorf("read source %s: %w", top, err)
	}
	var files []string
	for _, entry := range types {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		typeDir, err := root.OpenDir(path.Join(top, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("open source %s/%s: %w", top, entry.Name(), err)
		}
		children, err := typeDir.ReadDir(-1)
		_ = typeDir.Close()
		if err != nil {
			return nil, fmt.Errorf("read source %s/%s: %w", top, entry.Name(), err)
		}
		for _, child := range children {
			if strings.HasPrefix(child.Name(), ".") {
				continue
			}
			files = append(files, path.Join(top, entry.Name(), child.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func loadOpenSSLImportSource(root rootfs.Root, password string) ([]openSSLImportCertificate, []openSSLImportProfile, error) {
	certFiles, err := listOpenSSLImportFiles(root, "certs")
	if err != nil {
		return nil, nil, err
	}
	profileFiles, err := listOpenSSLImportFiles(root, "profiles")
	if err != nil {
		return nil, nil, err
	}
	if len(certFiles)+len(profileFiles) == 0 {
		return nil, nil, fmt.Errorf("source contains no files under certs/ or profiles/")
	}
	if len(certFiles)+len(profileFiles) > maxEncryptedSigningFiles {
		return nil, nil, fmt.Errorf("source contains more than %d signing files", maxEncryptedSigningFiles)
	}
	read := func(relPath string, accept func([]byte) bool) error {
		data, err := root.ReadFileLimited(relPath, maxOpenSSLImportFileSize)
		if err != nil {
			return fmt.Errorf("read %s: %w", relPath, err)
		}
		if _, err := signingpkg.DecryptOpenSSLRepositoryFile(data, password, accept); err != nil {
			return fmt.Errorf("decrypt %s: %w", relPath, err)
		}
		return nil
	}

	keys := make(map[string][]any)
	var certificates []openSSLImportCertificate
	for _, relPath := range certFiles {
		switch strings.ToLower(path.Ext(relPath)) {
		case ".p12":
			var parsed []any
			if err := read(relPath, func(data []byte) bool { parsed, err = parseOpenSSLImportPrivateKeys(data); return err == nil }); err != nil {
				return nil, nil, err
			}
			keys[strings.TrimSuffix(relPath, path.Ext(relPath))] = parsed
		case ".cer":
			var cert *x509.Certificate
			if err := read(relPath, func(data []byte) bool { cert, err = x509.ParseCertificate(data); return err == nil }); err != nil {
				return nil, nil, err
			}
			sourceType := path.Base(path.Dir(relPath))
			category := "distribution"
			if sourceType == "development" {
				category = "development"
			}
			certificates = append(certificates, openSSLImportCertificate{sourcePath: relPath, sourceType: sourceType, category: category, cert: cert, sha256: signingCertificateSHA256(cert)})
		default:
			return nil, nil, fmt.Errorf("unsupported source file %s; expected .cer or .p12", relPath)
		}
	}
	seenCertificates := make(map[string]string)
	for index := range certificates {
		certificate := &certificates[index]
		if prior, exists := seenCertificates[certificate.sha256]; exists {
			return nil, nil, fmt.Errorf("certificate %s duplicates %s", certificate.sourcePath, prior)
		}
		seenCertificates[certificate.sha256] = certificate.sourcePath
		base := strings.TrimSuffix(certificate.sourcePath, path.Ext(certificate.sourcePath))
		candidates, exists := keys[base]
		if !exists {
			return nil, nil, fmt.Errorf("certificate %s has no private key at %s.p12", certificate.sourcePath, base)
		}
		delete(keys, base)
		for _, key := range candidates {
			if publicKeysEqual(key, certificate.cert.PublicKey) {
				certificate.privateKey = key
				break
			}
		}
		if certificate.privateKey == nil {
			return nil, nil, fmt.Errorf("private key %s.p12 does not match certificate %s", base, certificate.sourcePath)
		}
		if certificateTeamID(certificate.cert) == "" {
			return nil, nil, fmt.Errorf("certificate %s has no team identifier", certificate.sourcePath)
		}
	}
	for base := range keys {
		return nil, nil, fmt.Errorf("private key %s.p12 has no certificate at %s.cer", base, base)
	}

	var profiles []openSSLImportProfile
	for _, relPath := range profileFiles {
		if !shared.IsProvisioningProfilePath(relPath) {
			return nil, nil, fmt.Errorf("unsupported source file %s; expected .mobileprovision or .provisionprofile", relPath)
		}
		var plaintext []byte
		var profile *identityMobileProvision
		if err := read(relPath, func(data []byte) bool {
			plaintext = data
			profile, err = parseIdentityMobileProvision(data)
			return err == nil
		}); err != nil {
			return nil, nil, err
		}
		identifier, _ := profile.Entitlements["application-identifier"].(string)
		macIdentifier, _ := profile.Entitlements["com.apple.application-identifier"].(string)
		if strings.HasSuffix(identifier, "*") || (identifier == "" && strings.HasSuffix(macIdentifier, "*")) {
			fmt.Fprintf(os.Stderr, "Warning: skipping wildcard profile %s; signing sync stores only explicit bundle ID profiles\n", relPath)
			continue
		}
		imported, err := newOpenSSLImportProfile(relPath, plaintext, profile)
		if err != nil {
			return nil, nil, fmt.Errorf("profile %s: %w", relPath, err)
		}
		profiles = append(profiles, imported)
	}
	return certificates, profiles, nil
}

func parseOpenSSLImportPrivateKeys(data []byte) ([]any, error) {
	if block, _ := pem.Decode(data); block != nil {
		key, err := parseSigningPrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return []any{key}, nil
	}
	keys, _, err := pkcs12.DecodeAll(data, "")
	if err != nil || len(keys) == 0 {
		return nil, fmt.Errorf("private key must be PEM or a PKCS#12 file without a password")
	}
	return keys, nil
}

func newOpenSSLImportProfile(relPath string, content []byte, profile *identityMobileProvision) (openSSLImportProfile, error) {
	uuid, err := normalizeIdentityProfileUUID(profile.UUID)
	if err != nil {
		return openSSLImportProfile{}, err
	}
	bundleID, err := signingPullProfileBundleID(profile)
	if err != nil {
		return openSSLImportProfile{}, err
	}
	if len(profile.TeamIdentifier) == 0 {
		return openSSLImportProfile{}, fmt.Errorf("profile has no team identifier")
	}
	distribution := map[string]string{
		"appstore":     "APP_STORE",
		"adhoc":        "APP_ADHOC",
		"development":  "APP_DEVELOPMENT",
		"enterprise":   "APP_INHOUSE",
		"developer_id": "APP_DIRECT",
	}[path.Base(path.Dir(relPath))]
	if distribution == "" {
		return openSSLImportProfile{}, fmt.Errorf("unsupported profile directory %s", path.Dir(relPath))
	}
	platform := "IOS_"
	switch {
	case signingPullProfilePlatformMatches(profile, "MAC_") && strings.HasSuffix(strings.TrimSuffix(path.Base(relPath), path.Ext(relPath)), "_catalyst"):
		platform = "MAC_CATALYST_"
	case signingPullProfilePlatformMatches(profile, "MAC_"):
		platform = "MAC_"
	case signingPullProfilePlatformMatches(profile, "TVOS_") && !signingPullProfilePlatformMatches(profile, "IOS_"):
		platform = "TVOS_"
	}
	profileType, err := normalizeSigningPullProfileType(platform + distribution)
	if err != nil || !identityProfileTypeMatches(profile, profileType) || !signingPullProfilePlatformMatches(profile, profileType) {
		return openSSLImportProfile{}, fmt.Errorf("profile contents do not match a supported %s profile type", path.Base(path.Dir(relPath)))
	}
	return openSSLImportProfile{
		sourcePath:  relPath,
		content:     content,
		profile:     profile,
		uuid:        uuid,
		bundleID:    bundleID,
		profileType: profileType,
	}, nil
}

// planOpenSSLImportWrites maps the verified source onto destination
// artifacts. Certificates and profiles already stored under any path are
// reused; existing may be nil for a dry run.
func planOpenSSLImportWrites(certificates []openSSLImportCertificate, profiles []openSSLImportProfile, existing []decryptedSigningFile, now time.Time) ([]openSSLImportWrite, error) {
	storedCertificates := make(map[string]string)
	storedProfiles := make(map[string]decryptedSigningFile)
	for _, file := range existing {
		relPath := canonicalSigningPullPath(file.RelativePath)
		switch {
		case strings.HasPrefix(relPath, "certs/") && strings.HasSuffix(strings.ToLower(relPath), ".cer"):
			if cert, err := x509.ParseCertificate(file.Plaintext); err == nil {
				storedCertificates[signingCertificateSHA256(cert)] = relPath
			}
		case strings.HasPrefix(relPath, "profiles/") && shared.IsProvisioningProfilePath(relPath):
			if profile, err := parseIdentityMobileProvision(file.Plaintext); err == nil {
				if uuid, err := normalizeIdentityProfileUUID(profile.UUID); err == nil {
					storedProfiles[uuid] = file
				}
			}
		}
	}

	var writes []openSSLImportWrite
	activeCores := make(map[string]*openSSLImportCertificate)
	for index := range certificates {
		certificate := &certificates[index]
		if _, stored := storedCertificates[certificate.sha256]; !stored {
			serial := fmt.Sprintf("%X", certificate.cert.SerialNumber)
			writes = append(writes, openSSLImportWrite{
				relPath:   path.Join("certs", certificate.category, safeFileName(serial, certificate.sha256)+".cer"),
				plaintext: certificate.cert.Raw,
			})
		}
		writes = append(writes, openSSLImportWrite{
			relPath:  path.Join("identities", certificate.category, certificate.sha256+".p12"),
			identity: certificate,
			metadata: &signingpkg.EncryptedFileMetadata{
				Kind:              "pkcs12-identity",
				Sensitive:         true,
				CertificateSHA256: certificate.sha256,
				TeamID:            certificateTeamID(certificate.cert),
			},
		})
		if !now.Before(certificate.cert.NotBefore) && now.Before(certificate.cert.NotAfter) {
			activeCores[certificate.sha256] = certificate
		}
	}

	contextScopes := make(map[string]string)
	plannedProfilePaths := make(map[string]struct{})
	for _, profile := range profiles {
		storedProfile, stored := storedProfiles[profile.uuid]
		profilePath := canonicalSigningPullPath(storedProfile.RelativePath)
		if !stored {
			profileDir := path.Join("profiles", profileDirectoryName(profile.profileType))
			name, extension := safeFileName(profile.profile.Name, profile.uuid), strings.ToLower(path.Ext(profile.sourcePath))
			profilePath = path.Join(profileDir, name+extension)
			if _, used := plannedProfilePaths[strings.ToLower(profilePath)]; used {
				profilePath = path.Join(profileDir, name+"--"+profile.uuid+extension)
			}
			plannedProfilePaths[strings.ToLower(profilePath)] = struct{}{}
			writes = append(writes, openSSLImportWrite{relPath: profilePath, plaintext: profile.content})
		}
		if !profile.profile.ExpirationDate.After(now) {
			continue
		}
		var bound *openSSLImportCertificate
		for _, der := range profile.profile.DeveloperCertificates {
			parsed, err := x509.ParseCertificate(der)
			if err != nil {
				continue
			}
			candidate, active := activeCores[signingCertificateSHA256(parsed)]
			if active && candidate.category == certDirectoryName(profile.profileType) && containsFold(profile.profile.TeamIdentifier, certificateTeamID(candidate.cert)) &&
				(bound == nil || candidate.sha256 < bound.sha256) {
				bound = candidate
			}
		}
		if bound == nil {
			if !signingPullProfileHasExactTypeProvenance(nil, profile.profileType) {
				fmt.Fprintf(os.Stderr, "Warning: %s has no currently valid imported private key; only a full pull returns this %s profile\n", profile.sourcePath, profile.profileType)
			}
			continue
		}
		teamID := certificateTeamID(bound.cert)
		scope := strings.Join([]string{teamID, profile.bundleID, profile.profileType}, "\x00")
		if prior, exists := contextScopes[scope]; exists {
			return nil, fmt.Errorf("profiles %s and %s are both active %s profiles for %s; remove one from the source", prior, profile.sourcePath, profile.profileType, profile.bundleID)
		}
		contextScopes[scope] = profile.sourcePath
		profileDigest := sha256.Sum256(profile.content)
		binding := identityContextBinding{
			CertificateSHA256: bound.sha256,
			TeamID:            teamID,
			BundleID:          profile.bundleID,
			ProfileType:       profile.profileType,
			ProfileResourceID: storedProfile.Metadata.ProfileResourceID,
			ProfileUUID:       profile.uuid,
			ProfilePath:       profilePath,
			ProfileSHA256:     strings.ToUpper(hex.EncodeToString(profileDigest[:])),
		}
		data, err := json.Marshal(binding)
		if err != nil {
			return nil, fmt.Errorf("encode identity context: %w", err)
		}
		scopeDigest := sha256.Sum256([]byte(scope))
		contextPath := path.Join("identity-contexts", strings.ToUpper(hex.EncodeToString(scopeDigest[:]))+".json")
		writes = append(writes, openSSLImportWrite{
			relPath:   contextPath,
			plaintext: data,
			metadata: &signingpkg.EncryptedFileMetadata{
				Kind:              "identity-context",
				CertificateSHA256: binding.CertificateSHA256,
				TeamID:            binding.TeamID,
				BundleID:          binding.BundleID,
				ProfileType:       binding.ProfileType,
				ProfileResourceID: binding.ProfileResourceID,
				ProfileUUID:       binding.ProfileUUID,
				ProfilePath:       binding.ProfilePath,
				ProfileSHA256:     binding.ProfileSHA256,
			},
		})
	}
	paths := openSSLImportWritePaths(writes)
	if err := signingpkg.ValidateEncryptedRepositoryPaths(paths); err != nil {
		return nil, err
	}
	return writes, nil
}

// applyOpenSSLImportWrites writes the planned artifacts into the local store
// and returns the paths that changed. Matching artifacts are reused; any
// different artifact at a planned path stops the import before publication.
func applyOpenSSLImportWrites(store *signingpkg.GitStore, writes []openSSLImportWrite, password string) ([]string, error) {
	if err := store.CheckEncryptedRepositoryPaths(openSSLImportWritePaths(writes)); err != nil {
		return nil, err
	}
	var written []string
	for _, write := range writes {
		plaintext := write.plaintext
		same := bytes.Equal
		if write.identity != nil {
			normalized, err := normalizeSigningIdentity(&signingIdentity{
				PrivateKey:        write.identity.privateKey,
				Certificate:       write.identity.cert,
				CertificateSHA256: write.identity.sha256,
			}, password)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", write.identity.sourcePath, err)
			}
			plaintext = normalized
			same = func(existing, wanted []byte) bool { return samePKCS12Identity(existing, wanted, password) }
		}
		if write.metadata != nil {
			exists, err := preflightSigningArtifact(store, write.relPath, plaintext, password, *write.metadata, same)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", write.relPath, err)
			}
			if exists {
				continue
			}
			if err := store.WriteEncryptedFileWithMetadata(write.relPath, plaintext, password, *write.metadata); err != nil {
				return nil, fmt.Errorf("write %s: %w", write.relPath, err)
			}
		} else {
			existing, err := store.ReadEncryptedFile(write.relPath, password)
			if err == nil {
				if bytes.Equal(existing, plaintext) {
					continue
				}
				return nil, fmt.Errorf("destination already has a different artifact at %s", write.relPath)
			}
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("destination artifact %s cannot be verified: %w", write.relPath, err)
			}
			if err := store.WriteEncryptedFile(write.relPath, plaintext, password); err != nil {
				return nil, fmt.Errorf("write %s: %w", write.relPath, err)
			}
		}
		written = append(written, write.relPath)
	}
	return written, nil
}

func openSSLImportWritePaths(writes []openSSLImportWrite) []string {
	paths := make([]string, 0, len(writes))
	for _, write := range writes {
		paths = append(paths, write.relPath)
	}
	return paths
}

func openSSLImportCertificateResults(certificates []openSSLImportCertificate) []asc.SigningSyncImportCertificate {
	results := make([]asc.SigningSyncImportCertificate, 0, len(certificates))
	for _, certificate := range certificates {
		results = append(results, asc.SigningSyncImportCertificate{
			SourcePath:     certificate.sourcePath,
			Type:           certificate.sourceType,
			Name:           certificate.cert.Subject.CommonName,
			TeamID:         certificateTeamID(certificate.cert),
			SerialNumber:   fmt.Sprintf("%X", certificate.cert.SerialNumber),
			SHA256:         certificate.sha256,
			ExpirationDate: certificate.cert.NotAfter.UTC().Format(time.RFC3339),
		})
	}
	return results
}

func openSSLImportProfileResults(profiles []openSSLImportProfile) []asc.SigningSyncImportProfile {
	results := make([]asc.SigningSyncImportProfile, 0, len(profiles))
	for _, profile := range profiles {
		results = append(results, asc.SigningSyncImportProfile{
			SourcePath:     profile.sourcePath,
			Name:           profile.profile.Name,
			UUID:           profile.uuid,
			BundleID:       profile.bundleID,
			ProfileType:    profile.profileType,
			TeamID:         strings.Join(profile.profile.TeamIdentifier, ","),
			ExpirationDate: profile.profile.ExpirationDate.UTC().Format(time.RFC3339),
		})
	}
	return results
}
