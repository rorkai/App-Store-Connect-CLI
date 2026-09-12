package signing

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

const signingProfileArtifactKind = "provisioning-profile"

func signingProfileArtifactMetadata(profile *asc.ProfileResponse, bundleID, profileType string) (signingpkg.EncryptedFileMetadata, error) {
	if profile == nil || strings.TrimSpace(profile.Data.ID) == "" {
		return signingpkg.EncryptedFileMetadata{}, fmt.Errorf("profile resource ID is missing")
	}
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return signingpkg.EncryptedFileMetadata{}, fmt.Errorf("profile bundle ID is missing")
	}
	normalizedType, err := normalizeSigningPullProfileType(profileType)
	if err != nil {
		return signingpkg.EncryptedFileMetadata{}, err
	}
	if apiType := strings.TrimSpace(profile.Data.Attributes.ProfileType); apiType != "" {
		normalizedAPIType, normalizeErr := normalizeSigningPullProfileType(apiType)
		if normalizeErr != nil || normalizedAPIType != normalizedType {
			return signingpkg.EncryptedFileMetadata{}, fmt.Errorf("profile type returned by App Store Connect does not match requested profile type")
		}
	}
	profileUUID := strings.TrimSpace(profile.Data.Attributes.UUID)
	if profileUUID != "" {
		profileUUID, err = normalizeIdentityProfileUUID(profileUUID)
		if err != nil {
			return signingpkg.EncryptedFileMetadata{}, fmt.Errorf("profile UUID returned by App Store Connect is invalid: %w", err)
		}
	}
	return signingpkg.EncryptedFileMetadata{
		Kind:              signingProfileArtifactKind,
		BundleID:          bundleID,
		ProfileType:       normalizedType,
		ProfileResourceID: strings.TrimSpace(profile.Data.ID),
		ProfileUUID:       profileUUID,
	}, nil
}

func validateSigningProfileArtifactMetadata(relPath string, metadata signingpkg.EncryptedFileMetadata) error {
	canonicalPath := canonicalSigningPullPath(relPath)
	if metadata.Version != 1 || metadata.Kind != signingProfileArtifactKind || metadata.Sensitive {
		return fmt.Errorf("provisioning profile must use a versioned non-sensitive envelope")
	}
	if !strings.HasPrefix(canonicalPath, "profiles/") || !shared.IsProvisioningProfilePath(canonicalPath) {
		return fmt.Errorf("provisioning profile metadata requires a profile repository path")
	}
	if strings.TrimSpace(metadata.BundleID) == "" || strings.TrimSpace(metadata.ProfileResourceID) == "" {
		return fmt.Errorf("provisioning profile metadata is missing its authenticated scope")
	}
	normalizedType, err := normalizeSigningPullProfileType(metadata.ProfileType)
	if err != nil || normalizedType != strings.TrimSpace(metadata.ProfileType) {
		return fmt.Errorf("provisioning profile metadata has an invalid profile type")
	}
	if metadata.ProfileUUID != "" {
		if _, err := normalizeIdentityProfileUUID(metadata.ProfileUUID); err != nil {
			return fmt.Errorf("provisioning profile metadata has an invalid profile UUID")
		}
	}
	if metadata.CertificateSHA256 != "" || metadata.TeamID != "" || metadata.ProfilePath != "" || metadata.ProfileSHA256 != "" {
		return fmt.Errorf("provisioning profile metadata contains unrelated identity fields")
	}
	return nil
}

func validateSigningProfileSelectionScope(file decryptedSigningFile, profile *identityMobileProvision, bundleID string) error {
	if file.Metadata.Kind != signingProfileArtifactKind {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(file.Metadata.BundleID), strings.TrimSpace(bundleID)) {
		return fmt.Errorf("stored profile %s authenticated bundle ID does not match signed profile", canonicalSigningPullPath(file.RelativePath))
	}
	if file.Metadata.ProfileUUID == "" {
		return nil
	}
	signedUUID, err := normalizeIdentityProfileUUID(profile.UUID)
	if err != nil || !strings.EqualFold(signedUUID, file.Metadata.ProfileUUID) {
		return fmt.Errorf("stored profile %s authenticated UUID does not match signed profile", canonicalSigningPullPath(file.RelativePath))
	}
	return nil
}

func sameSigningProfileArtifactMetadata(left, right signingpkg.EncryptedFileMetadata) bool {
	return left.Version == 1 && left.Kind == signingProfileArtifactKind && right.Kind == signingProfileArtifactKind &&
		canonicalSigningPullPath(left.RelativePath) == canonicalSigningPullPath(right.RelativePath) &&
		left.BundleID == right.BundleID && left.ProfileType == right.ProfileType &&
		left.ProfileResourceID == right.ProfileResourceID && left.ProfileUUID == right.ProfileUUID &&
		!left.Sensitive && !right.Sensitive
}

func sameSigningProfileArtifactScope(left, right signingpkg.EncryptedFileMetadata) bool {
	return left.Kind == signingProfileArtifactKind && right.Kind == signingProfileArtifactKind &&
		left.BundleID == right.BundleID && left.ProfileType == right.ProfileType
}

func preflightSigningProfileArtifact(store *signingpkg.GitStore, relPath string, plaintext []byte, password string, metadata signingpkg.EncryptedFileMetadata) (bool, error) {
	metadata.Version = 1
	metadata.RelativePath = canonicalSigningPullPath(relPath)
	if err := validateSigningProfileArtifactMetadata(relPath, metadata); err != nil {
		return false, fmt.Errorf("provisioning profile metadata is invalid: %w", err)
	}
	existing, existingMetadata, err := store.ReadEncryptedFileWithMetadata(relPath, password)
	if errors.Is(err, os.ErrNotExist) {
		return false, store.CheckWriteEncryptedFile(relPath)
	}
	if err != nil {
		return false, fmt.Errorf("existing provisioning profile cannot be authenticated")
	}
	if existingMetadata.Version != 0 {
		if err := validateSigningProfileArtifactMetadata(relPath, existingMetadata); err != nil {
			return false, fmt.Errorf("existing provisioning profile has incompatible authenticated metadata")
		}
		if !sameSigningProfileArtifactScope(existingMetadata, metadata) {
			return false, fmt.Errorf("existing provisioning profile has a different authenticated scope")
		}
		if bytes.Equal(existing, plaintext) && sameSigningProfileArtifactMetadata(existingMetadata, metadata) {
			return true, nil
		}
	}
	if err := store.CheckWriteEncryptedFile(relPath); err != nil {
		return false, err
	}
	return false, nil
}

func writeOrReuseSigningProfileArtifact(store *signingpkg.GitStore, relPath string, plaintext []byte, password string, metadata signingpkg.EncryptedFileMetadata) error {
	same, err := preflightSigningProfileArtifact(store, relPath, plaintext, password, metadata)
	if err != nil || same {
		return err
	}
	return store.ReplaceEncryptedFileWithMetadata(relPath, plaintext, password, metadata)
}

// resolveCompatibleSigningProfilePath keeps using the legacy repository path
// for a macOS profile when that is the only spelling already present. This
// prevents an extension upgrade from publishing a second copy of the same
// profile under .provisionprofile. Repositories that already contain both
// spellings are ambiguous and must be repaired explicitly.
func resolveCompatibleSigningProfilePath(store *signingpkg.GitStore, preferredPath string) (string, error) {
	if store == nil || !strings.EqualFold(filepath.Ext(preferredPath), ".provisionprofile") {
		return preferredPath, nil
	}
	legacyPath := strings.TrimSuffix(preferredPath, filepath.Ext(preferredPath)) + ".mobileprovision"
	existing, err := store.ListEncryptedFiles()
	if err != nil {
		return "", err
	}
	preferredCanonical := canonicalSigningPullPath(preferredPath)
	legacyCanonical := canonicalSigningPullPath(legacyPath)
	preferredExists := false
	legacyExists := false
	for _, path := range existing {
		switch canonicalSigningPullPath(path) {
		case preferredCanonical:
			preferredExists = true
		case legacyCanonical:
			legacyExists = true
		}
	}
	if preferredExists && legacyExists {
		return "", fmt.Errorf("macOS provisioning profile exists at both %s and %s", legacyPath, preferredPath)
	}
	if legacyExists {
		return legacyPath, nil
	}
	return preferredPath, nil
}
