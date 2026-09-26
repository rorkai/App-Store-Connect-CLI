package signing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

type signingSyncBatchOptions struct {
	Transport                signingSyncTransport
	RepoURL                  string
	Branch                   string
	Password                 string
	ProfileType              string
	CertificateType          string
	DeviceIDs                []string
	CreateMissing            bool
	CreateMissingCertificate bool
	IdentityPassword         []byte
	Identity                 *signingIdentity
	BundleIDs                []string
	ContextWithTimeout       func(context.Context) (context.Context, context.CancelFunc)
}

type signingSyncBatchTarget struct {
	BundleID                 string
	BundleIDResourceID       string
	ProfileType              string
	Profile                  *asc.ProfileResponse
	Certificates             []asc.Resource[asc.CertificateAttributes]
	ProfileCreated           bool
	ProfileContent           []byte
	ProfilePath              string
	IdentityArtifacts        *signingIdentityArtifacts
	Files                    []string
	CertificateCreationState string
	ProfileCreationState     string
	CertificateAttempted     bool
	ProfileCreateAttempted   bool
}

type signingSyncBatchLegacyFile struct {
	RelativePath string
	Plaintext    []byte
	Profile      *signingpkg.EncryptedFileMetadata
}

func runSigningSyncBatch(ctx context.Context, client *asc.Client, options signingSyncBatchOptions) (SyncResult, error) {
	if client == nil {
		return SyncResult{}, fmt.Errorf("signing sync client is nil")
	}
	if len(options.BundleIDs) == 0 {
		return SyncResult{}, fmt.Errorf("targets manifest contains no bundle IDs")
	}
	if options.CreateMissingCertificate && len(options.IdentityPassword) == 0 {
		return SyncResult{}, fmt.Errorf("identity password is empty")
	}
	contextWithTimeout := options.ContextWithTimeout
	if contextWithTimeout == nil {
		contextWithTimeout = shared.ContextWithTimeout
	}
	bundleIDs := append([]string(nil), options.BundleIDs...)
	sort.Slice(bundleIDs, func(i, j int) bool {
		left, right := strings.ToLower(bundleIDs[i]), strings.ToLower(bundleIDs[j])
		if left == right {
			return bundleIDs[i] < bundleIDs[j]
		}
		return left < right
	})

	tmpDir, err := os.MkdirTemp("", "asc-signing-sync-*")
	if err != nil {
		return SyncResult{}, fmt.Errorf("create temp dir: %w", err)
	}
	transport := options.Transport
	if transport == nil {
		transport = signingSyncGitTransport{repoURL: options.RepoURL, branch: options.Branch}
	}
	store := newSigningSyncStore(transport, tmpDir, options.RepoURL, options.Branch)
	defer func() { _ = store.Cleanup() }()
	identity := options.Identity
	var createdCertificate asc.Resource[asc.CertificateAttributes]
	certificateOutputs := &signingCertificateOutputs{BasePath: tmpDir}
	defer func() { _ = certificateOutputs.Close() }()
	certificateRequest := signingCertificateCreateRequest{}
	if options.CreateMissingCertificate {
		certificateRequest = signingCertificateCreateRequest{
			Outputs:  certificateOutputs,
			KeyPath:  filepath.Join(tmpDir, "created.key"),
			CSRPath:  filepath.Join(tmpDir, "created.csr"),
			P12Path:  filepath.Join(tmpDir, "created.p12"),
			Password: options.IdentityPassword,
		}
	}

	prepareRepository := onceAfterSuccess(func() error {
		return transport.Fetch(ctx, store, true)
	})

	fmt.Fprintln(os.Stderr, "Fetching signing assets from App Store Connect...")
	targets := make([]signingSyncBatchTarget, 0, len(options.BundleIDs))
	partial := func(err error, current []signingSyncBatchTarget) (SyncResult, error) {
		return signingSyncBatchPartialResult(options, bundleIDs, current, identity), signingSyncBatchPublicationError(err, current)
	}
	for _, bundleID := range bundleIDs {
		requestCtx, cancelRequest := contextWithTimeout(ctx)
		bundleIDResponse, err := findBundleID(requestCtx, client, bundleID)
		if err != nil {
			cancelRequest()
			return partial(
				fmt.Errorf("resolve bundle ID %s: %w", bundleID, err),
				targets,
			)
		}
		cancelRequest()

		assetCtx, cancelAssets := contextWithTimeout(ctx)
		var createdIdentity createdSigningIdentity
		progress := &signingAssetsProgress{}
		profile, certificates, created, err := resolveSigningAssets(
			assetCtx,
			client,
			signingAssetsOptions{
				BundleIDResourceID:       bundleIDResponse.Data.ID,
				BundleIdentifier:         bundleID,
				ProfileType:              options.ProfileType,
				ProfileName:              profileCreateNameForTarget(options.ProfileType, bundleID, time.Now()),
				CertificateType:          options.CertificateType,
				DeviceIDs:                options.DeviceIDs,
				CreateMissing:            options.CreateMissing,
				CreateMissingCertificate: options.CreateMissingCertificate,
				CertificateCreate:        certificateRequest,
				CreatedIdentity:          &createdIdentity,
				CertificateFallback:      &createdCertificate,
				CreatedCertificate:       &createdCertificate,
				Progress:                 progress,
				AfterCertificateCreate: func(created createdSigningIdentity) error {
					if identity != nil {
						return fmt.Errorf("--create-missing-certificate cannot replace an existing signing identity")
					}
					loaded, loadErr := loadPKCS12Identity(created.P12Path, string(options.IdentityPassword), "")
					if loadErr != nil {
						return loadErr
					}
					identity = loaded
					return nil
				},
				BeforeCertificateCreate: func(plan profileCreatePlan) error {
					if err := certificateOutputs.Prepare([]string{certificateRequest.KeyPath, certificateRequest.CSRPath, certificateRequest.P12Path}, false); err != nil {
						return err
					}
					if err := prepareRepository(); err != nil {
						return err
					}
					profileExtension := shared.ProvisioningProfileExtension("", options.ProfileType)
					profilePath := filepath.Join("profiles", profileDirectoryName(options.ProfileType), safeFileName(plan.ProfileName, "profile")+profileExtension)
					return preflightSigningAssetDestinationsForProfile(store, plan, options.ProfileType, profilePath)
				},
				BeforeCreate: func(plan profileCreatePlan) error {
					if identity != nil {
						if err := preflightIdentityForProfileCreate(identity, plan, options.Password, time.Now()); err != nil {
							return err
						}
					}
					if err := prepareRepository(); err != nil {
						return err
					}
					for _, certificate := range plan.Certificates {
						certificateContent, err := decodeBase64Content("certificate", certificate.Attributes.CertificateContent)
						if err != nil {
							return err
						}
						relPath := filepath.Join("certs", certDirectoryName(options.ProfileType), safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer")
						if _, err := preflightSigningSyncLegacyArtifact(store, relPath, certificateContent, options.Password); err != nil {
							return fmt.Errorf("preflight certificate destination: %w", err)
						}
					}
					if identity != nil {
						artifacts, err := prepareSigningIdentityArtifacts(identity, options.Password, bundleID, options.ProfileType)
						if err != nil {
							return err
						}
						if _, err := preflightSigningArtifact(store, artifacts.IdentityPath, artifacts.IdentityData, options.Password, artifacts.IdentityMetadata, func(existing, wanted []byte) bool {
							return samePKCS12Identity(existing, wanted, options.Password)
						}); err != nil {
							return fmt.Errorf("preflight signing identity: %w", err)
						}
						if err := preflightSigningIdentityArtifactsForContextUpdate(store, artifacts, options.Password); err != nil {
							return fmt.Errorf("preflight signing identity context: %w", err)
						}
					}
					return preflightSigningSyncBatchProfileCreate(store, plan, options.ProfileType)
				},
				CreateContext: func() (context.Context, context.CancelFunc) {
					return contextWithTimeout(ctx)
				},
				CertificateFilter: identityCertificateFilter(identity),
			},
		)
		cancelAssets()
		if err != nil {
			candidateTargets := append([]signingSyncBatchTarget(nil), targets...)
			if createdIdentity.CertificateAttempted || progress.ProfileCreateAttempted {
				candidate := signingSyncBatchTarget{
					BundleID:               bundleID,
					BundleIDResourceID:     bundleIDResponse.Data.ID,
					ProfileType:            options.ProfileType,
					Certificates:           append([]asc.Resource[asc.CertificateAttributes](nil), progress.Certificates...),
					CertificateAttempted:   createdIdentity.CertificateAttempted,
					ProfileCreateAttempted: progress.ProfileCreateAttempted,
				}
				switch {
				case createdIdentity.CertificateID != "" && createdIdentity.P12Path != "":
					candidate.CertificateCreationState = "created"
				case createdIdentity.CertificateAttempted:
					candidate.CertificateCreationState = "unknown"
				case options.CreateMissingCertificate:
					candidate.CertificateCreationState = "reused"
				}
				if progress.ProfileCreateAttempted {
					candidate.ProfileCreationState = "unknown"
				}
				candidateTargets = append(candidateTargets, candidate)
			}
			return partial(
				fmt.Errorf("resolve signing assets for %s: %w", bundleID, err),
				candidateTargets,
			)
		}
		if created {
			fmt.Fprintf(os.Stderr, "Created new profile for %s\n", bundleID)
		}

		target := signingSyncBatchTarget{
			BundleID:             bundleID,
			BundleIDResourceID:   bundleIDResponse.Data.ID,
			ProfileType:          options.ProfileType,
			Profile:              profile,
			Certificates:         certificates.Data,
			ProfileCreated:       created,
			CertificateAttempted: createdIdentity.CertificateAttempted,
		}
		if createdIdentity.CertificateID != "" && createdIdentity.P12Path != "" {
			target.CertificateCreationState = "created"
		} else if options.CreateMissingCertificate {
			target.CertificateCreationState = "reused"
		}
		if created {
			target.ProfileCreationState = "created"
		} else if options.CreateMissing {
			target.ProfileCreationState = "reused"
		}
		if identity != nil {
			if err := validateIdentityForResolvedAssets(identity, profile, certificates, bundleID, options.ProfileType, time.Now()); err != nil {
				candidateTargets := append(append([]signingSyncBatchTarget(nil), targets...), target)
				return partial(fmt.Errorf("validate signing identity for %s: %w", bundleID, err), candidateTargets)
			}
		}
		targets = append(targets, target)
	}

	if err := prepareRepository(); err != nil {
		return partial(err, targets)
	}

	legacyFiles, identityCore, err := prepareSigningSyncBatchFiles(store, targets, identity, options.Password)
	if err != nil {
		return partial(err, targets)
	}

	plannedPaths := make([]string, 0, len(legacyFiles)+len(targets)*2)
	for _, file := range legacyFiles {
		plannedPaths = append(plannedPaths, file.RelativePath)
	}
	for _, target := range targets {
		if target.IdentityArtifacts == nil {
			continue
		}
		plannedPaths = append(plannedPaths, target.IdentityArtifacts.IdentityPath, target.IdentityArtifacts.BindingPath)
	}
	if err := store.CheckEncryptedRepositoryPaths(plannedPaths); err != nil {
		return partial(fmt.Errorf("preflight repository paths: %w", err), targets)
	}

	for _, file := range legacyFiles {
		var err error
		if file.Profile != nil {
			_, err = preflightSigningProfileArtifact(store, file.RelativePath, file.Plaintext, options.Password, *file.Profile)
		} else {
			_, err = preflightSigningSyncLegacyArtifact(store, file.RelativePath, file.Plaintext, options.Password)
		}
		if err != nil {
			return partial(fmt.Errorf("preflight %s: %w", file.RelativePath, err), targets)
		}
	}
	for _, target := range targets {
		if target.IdentityArtifacts == nil {
			continue
		}
		if err := preflightSigningIdentityArtifactsForContextUpdate(store, target.IdentityArtifacts, options.Password); err != nil {
			return partial(fmt.Errorf("preflight signing identity for %s: %w", target.BundleID, err), targets)
		}
	}

	for _, file := range legacyFiles {
		var err error
		if file.Profile != nil {
			err = writeOrReuseSigningProfileArtifact(store, file.RelativePath, file.Plaintext, options.Password, *file.Profile)
		} else {
			err = writeOrReuseSigningSyncLegacyArtifact(store, file.RelativePath, file.Plaintext, options.Password)
		}
		if err != nil {
			return partial(fmt.Errorf("encrypt %s: %w", file.RelativePath, err), targets)
		}
		fmt.Fprintf(os.Stderr, "  Encrypted %s\n", file.RelativePath)
	}
	for _, target := range targets {
		if target.IdentityArtifacts == nil {
			continue
		}
		if err := writeOrReuseSigningIdentityArtifacts(store, target.IdentityArtifacts, options.Password); err != nil {
			return partial(fmt.Errorf("encrypt signing identity for %s: %w", target.BundleID, err), targets)
		}
		fmt.Fprintf(os.Stderr, "  Encrypted %s\n", target.IdentityArtifacts.IdentityPath)
		fmt.Fprintf(os.Stderr, "  Encrypted %s\n", target.IdentityArtifacts.BindingPath)
	}

	commitMessage := fmt.Sprintf("Update signing assets for %s (%d targets)", options.ProfileType, len(targets))
	if err := transport.Publish(ctx, store, commitMessage); err != nil {
		return partial(err, targets)
	}
	fmt.Fprintln(os.Stderr, "Done")

	result := SyncResult{
		Operation:       "push",
		RepoURL:         transport.Locator(),
		ProfileType:     options.ProfileType,
		Files:           make([]string, 0),
		IdentityPresent: identity != nil,
		Targets:         make([]SyncTargetResult, 0, len(targets)),
		BundleIDs:       bundleIDs,
	}
	result.MarkBatch()
	if identity != nil {
		result.IdentitySHA256 = identity.CertificateSHA256
		if identityCore != "" {
			result.SensitiveFiles = []string{identityCore}
		}
	}
	for _, target := range targets {
		result.CertificateIDs = append(result.CertificateIDs, extractIDs(target.Certificates)...)
		result.CertificateCreationState = mergeSigningCreationState(result.CertificateCreationState, target.CertificateCreationState)
		result.ProfileCreationState = mergeSigningCreationState(result.ProfileCreationState, target.ProfileCreationState)
	}
	result.CertificateIDs = uniqueSortedSigningSyncStrings(result.CertificateIDs)

	for _, target := range targets {
		files := uniqueSortedSigningSyncStrings(target.Files)
		result.Targets = append(result.Targets, SyncTargetResult{
			BundleID:                 target.BundleID,
			ProfileType:              target.ProfileType,
			ProfilePath:              target.ProfilePath,
			ProfileCreated:           target.ProfileCreated,
			CertificateCreationState: target.CertificateCreationState,
			ProfileCreationState:     target.ProfileCreationState,
			Files:                    files,
		})
		result.Files = append(result.Files, files...)
	}
	result.Files = uniqueSortedSigningSyncStrings(result.Files)
	return result, nil
}

func signingSyncBatchPartialResult(options signingSyncBatchOptions, bundleIDs []string, targets []signingSyncBatchTarget, identity *signingIdentity) SyncResult {
	locator := sanitizeRepoURLForOutput(options.RepoURL)
	if options.Transport != nil {
		locator = options.Transport.Locator()
	}
	result := SyncResult{
		Operation:       "push",
		RepoURL:         locator,
		ProfileType:     options.ProfileType,
		Files:           []string{},
		IdentityPresent: identity != nil,
		BundleIDs:       append([]string(nil), bundleIDs...),
		Targets:         make([]SyncTargetResult, 0, len(targets)),
		Partial:         len(targets) > 0,
	}
	result.MarkBatch()
	if identity != nil {
		result.IdentitySHA256 = identity.CertificateSHA256
	}
	for _, target := range targets {
		files := uniqueSortedSigningSyncStrings(target.Files)
		result.Targets = append(result.Targets, SyncTargetResult{
			BundleID:                 target.BundleID,
			ProfileType:              target.ProfileType,
			ProfilePath:              target.ProfilePath,
			ProfileCreated:           target.ProfileCreated,
			CertificateCreationState: target.CertificateCreationState,
			ProfileCreationState:     target.ProfileCreationState,
			Files:                    files,
		})
		result.Files = append(result.Files, files...)
		result.CertificateIDs = append(result.CertificateIDs, extractIDs(target.Certificates)...)
		result.CertificateCreationState = mergeSigningCreationState(result.CertificateCreationState, target.CertificateCreationState)
		result.ProfileCreationState = mergeSigningCreationState(result.ProfileCreationState, target.ProfileCreationState)
	}
	result.Files = uniqueSortedSigningSyncStrings(result.Files)
	result.CertificateIDs = uniqueSortedSigningSyncStrings(result.CertificateIDs)
	if result.Partial {
		result.PublicationState = "unknown"
		if result.ProfileCreationState == "" {
			result.ProfileCreationState = "unknown"
		}
		if options.CreateMissingCertificate && result.CertificateCreationState == "" {
			result.CertificateCreationState = "unknown"
		}
	}
	return result
}

func mergeSigningCreationState(current, next string) string {
	if current == "unknown" || next == "unknown" {
		return "unknown"
	}
	if current == "created" || next == "created" {
		return "created"
	}
	if current == "reused" || next == "reused" {
		return "reused"
	}
	return current
}

func signingSyncBatchPublicationError(err error, targets []signingSyncBatchTarget) error {
	if err == nil {
		return nil
	}
	for _, target := range targets {
		if target.ProfileCreated {
			return fmt.Errorf("%w; repository publication did not complete; earlier App Store Connect profile creations may remain", err)
		}
	}
	return err
}

func prepareSigningSyncBatchFiles(store *signingpkg.GitStore, targets []signingSyncBatchTarget, identity *signingIdentity, password string) ([]signingSyncBatchLegacyFile, string, error) {
	legacyByPath := make(map[string][]byte)
	profileMetadataByPath := make(map[string]signingpkg.EncryptedFileMetadata)
	legacyOrder := make([]string, 0)
	certificateContentByID := make(map[string][]byte)
	certificatePathByID := make(map[string]string)
	certificateIDByPath := make(map[string]string)
	var sharedIdentityArtifacts *signingIdentityArtifacts
	identityCore := ""

	for index := range targets {
		target := &targets[index]
		if target.Profile == nil {
			return nil, "", fmt.Errorf("profile for %s is missing", target.BundleID)
		}
		profileContent, err := decodeBase64Content("profile", target.Profile.Data.Attributes.ProfileContent)
		if err != nil {
			return nil, "", fmt.Errorf("decode profile for %s: %w", target.BundleID, err)
		}
		if strings.TrimSpace(target.Profile.Data.ID) == "" {
			return nil, "", fmt.Errorf("profile for %s has no resource ID", target.BundleID)
		}
		target.ProfileContent = profileContent
		target.ProfilePath = signingSyncBatchProfilePath(target.BundleID, target.ProfileType, target.Profile.Data.ID)
		target.ProfilePath, err = resolveCompatibleSigningProfilePath(store, target.ProfilePath)
		if err != nil {
			return nil, "", fmt.Errorf("resolve profile repository path for %s: %w", target.BundleID, err)
		}
		profileMetadata, err := signingProfileArtifactMetadata(target.Profile, target.BundleID, target.ProfileType)
		if err != nil {
			return nil, "", fmt.Errorf("profile for %s metadata: %w", target.BundleID, err)
		}
		if existing, exists := profileMetadataByPath[target.ProfilePath]; exists && !sameSigningProfileArtifactScope(existing, profileMetadata) {
			return nil, "", fmt.Errorf("profile for %s maps to a conflicting authenticated scope", target.BundleID)
		}
		profileMetadataByPath[target.ProfilePath] = profileMetadata
		if err := addSigningSyncBatchLegacyFile(legacyByPath, &legacyOrder, target.ProfilePath, profileContent); err != nil {
			return nil, "", fmt.Errorf("profile for %s: %w", target.BundleID, err)
		}
		target.Files = append(target.Files, target.ProfilePath)

		if identity != nil {
			if err := validateIdentityForResolvedAssets(identity, target.Profile, &asc.CertificatesResponse{Data: target.Certificates}, target.BundleID, target.ProfileType, time.Now()); err != nil {
				return nil, "", fmt.Errorf("validate signing identity for %s: %w", target.BundleID, err)
			}
			artifacts, err := prepareSigningIdentityArtifacts(identity, password, target.BundleID, target.ProfileType)
			if err != nil {
				return nil, "", fmt.Errorf("prepare signing identity for %s: %w", target.BundleID, err)
			}
			if sharedIdentityArtifacts == nil {
				sharedIdentityArtifacts = artifacts
				identityCore = artifacts.IdentityPath
			} else {
				artifacts.IdentityPath = sharedIdentityArtifacts.IdentityPath
				artifacts.IdentityData = sharedIdentityArtifacts.IdentityData
				artifacts.IdentityMetadata = sharedIdentityArtifacts.IdentityMetadata
			}
			if err := bindSigningIdentityProfile(artifacts, target.Profile, target.ProfilePath, profileContent); err != nil {
				return nil, "", fmt.Errorf("bind signing identity for %s: %w", target.BundleID, err)
			}
			target.IdentityArtifacts = artifacts
			target.Files = append(target.Files, artifacts.IdentityPath, artifacts.BindingPath)
		}

		for _, certificate := range target.Certificates {
			certificateContent, err := decodeBase64Content("certificate", certificate.Attributes.CertificateContent)
			if err != nil {
				return nil, "", fmt.Errorf("decode certificate %s for %s: %w", certificate.ID, target.BundleID, err)
			}
			relPath := filepath.Join("certs", certDirectoryName(target.ProfileType), safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer")
			certificateID := strings.TrimSpace(certificate.ID)
			if certificateID != "" {
				if existing, ok := certificateContentByID[certificateID]; ok && !bytes.Equal(existing, certificateContent) {
					return nil, "", fmt.Errorf("certificate %s returned conflicting content", certificateID)
				}
				if existingPath, ok := certificatePathByID[certificateID]; ok && existingPath != relPath {
					return nil, "", fmt.Errorf("certificate %s maps to conflicting repository paths", certificateID)
				}
				certificateContentByID[certificateID] = append([]byte(nil), certificateContent...)
				certificatePathByID[certificateID] = relPath
			}
			if existingID, ok := certificateIDByPath[relPath]; !ok || certificateID < existingID {
				certificateIDByPath[relPath] = certificateID
			}
			if err := addSigningSyncBatchLegacyFile(legacyByPath, &legacyOrder, relPath, certificateContent); err != nil {
				return nil, "", fmt.Errorf("certificate %s for %s: %w", certificate.ID, target.BundleID, err)
			}
			target.Files = append(target.Files, relPath)
		}
	}

	if err := store.CheckEncryptedRepositoryPaths(append([]string(nil), legacyOrder...)); err != nil {
		return nil, "", err
	}
	sort.Slice(legacyOrder, func(i, j int) bool {
		left, right := legacyOrder[i], legacyOrder[j]
		leftCertificate := strings.HasPrefix(filepath.ToSlash(left), "certs/")
		rightCertificate := strings.HasPrefix(filepath.ToSlash(right), "certs/")
		if leftCertificate != rightCertificate {
			return leftCertificate
		}
		if leftCertificate {
			leftID, rightID := certificateIDByPath[left], certificateIDByPath[right]
			if leftID != rightID {
				return leftID < rightID
			}
		}
		return left < right
	})
	legacyFiles := make([]signingSyncBatchLegacyFile, 0, len(legacyOrder))
	for _, path := range legacyOrder {
		file := signingSyncBatchLegacyFile{RelativePath: path, Plaintext: legacyByPath[path]}
		if metadata, exists := profileMetadataByPath[path]; exists {
			metadataCopy := metadata
			file.Profile = &metadataCopy
		}
		legacyFiles = append(legacyFiles, file)
	}
	return legacyFiles, identityCore, nil
}

func addSigningSyncBatchLegacyFile(files map[string][]byte, order *[]string, relPath string, plaintext []byte) error {
	if existing, ok := files[relPath]; ok {
		if !bytes.Equal(existing, plaintext) {
			return fmt.Errorf("repository path %s maps to conflicting certificate or profile content", relPath)
		}
		return nil
	}
	files[relPath] = append([]byte(nil), plaintext...)
	*order = append(*order, relPath)
	return nil
}

func preflightSigningSyncBatchProfileCreate(store *signingpkg.GitStore, plan profileCreatePlan, profileType string) error {
	certDir := certDirectoryName(profileType)
	for _, certificate := range plan.Certificates {
		relPath := filepath.Join("certs", certDir, safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer")
		if err := store.CheckWriteEncryptedFile(relPath); err != nil {
			return fmt.Errorf("preflight certificate destination: %w", err)
		}
	}
	profileExtension := shared.ProvisioningProfileExtension("", profileType)
	placeholder := filepath.Join("profiles", profileDirectoryName(profileType), "target-placeholder"+profileExtension)
	if err := store.CheckEncryptedFileParent(placeholder); err != nil {
		return fmt.Errorf("preflight profile destination: %w", err)
	}
	return nil
}

func preflightSigningSyncLegacyArtifact(store *signingpkg.GitStore, relPath string, wanted []byte, password string) (bool, error) {
	existing, metadata, err := store.ReadEncryptedFileWithMetadata(relPath, password)
	if errors.Is(err, os.ErrNotExist) {
		if err := store.CheckNewEncryptedFile(relPath); err != nil {
			return false, err
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("existing signing artifact cannot be authenticated")
	}
	if metadata.Version != 0 {
		return false, fmt.Errorf("existing signing artifact has incompatible authenticated metadata")
	}
	if bytes.Equal(existing, wanted) {
		return true, nil
	}
	if err := store.CheckWriteEncryptedFile(relPath); err != nil {
		return false, err
	}
	return false, nil
}

func writeOrReuseSigningSyncLegacyArtifact(store *signingpkg.GitStore, relPath string, plaintext []byte, password string) error {
	same, err := preflightSigningSyncLegacyArtifact(store, relPath, plaintext, password)
	if err != nil || same {
		return err
	}
	return store.WriteEncryptedFile(relPath, plaintext, password)
}

func signingSyncBatchProfilePath(bundleID, profileType, profileID string) string {
	profileExtension := shared.ProvisioningProfileExtension("", profileType)
	return filepath.Join(
		"profiles",
		profileDirectoryName(profileType),
		safeFileName(bundleID, "bundle")+"--"+safeFileName(profileID, "profile")+profileExtension,
	)
}

func uniqueSortedSigningSyncStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	unique := sorted[:0]
	for _, value := range sorted {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}
