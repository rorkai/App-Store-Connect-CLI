package signing

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type signingPullProfile struct {
	file        decryptedSigningFile
	certificate []string
}

func normalizeSigningPullProfileType(raw string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	switch normalized {
	case "IOS_APP_DEVELOPMENT",
		"IOS_APP_STORE",
		"IOS_APP_ADHOC",
		"IOS_APP_INHOUSE",
		"MAC_APP_DEVELOPMENT",
		"MAC_APP_STORE",
		"MAC_APP_DIRECT",
		"TVOS_APP_DEVELOPMENT",
		"TVOS_APP_STORE",
		"TVOS_APP_ADHOC",
		"TVOS_APP_INHOUSE",
		"MAC_CATALYST_APP_DEVELOPMENT",
		"MAC_CATALYST_APP_STORE",
		"MAC_CATALYST_APP_DIRECT":
		return normalized, nil
	default:
		return "", fmt.Errorf("--profile-type must be a supported App Store Connect profile type")
	}
}

func selectSigningPullFiles(files []decryptedSigningFile, bundleIDs []string, profileType string) ([]decryptedSigningFile, []SyncTargetResult, error) {
	requested := uniqueSortedSigningSyncStrings(bundleIDs)
	if len(requested) == 0 {
		return nil, nil, fmt.Errorf("signing pull selection contains no bundle IDs")
	}
	profileType = strings.ToUpper(strings.TrimSpace(profileType))
	if profileType == "" {
		return nil, nil, fmt.Errorf("signing pull selection has no profile type")
	}

	filesByPath := make(map[string]decryptedSigningFile, len(files))
	certificatesByFingerprint := make(map[string]decryptedSigningFile)
	profilesByBundle := make(map[string][]signingPullProfile)
	contextsByBundle := make(map[string][]decryptedSigningFile)
	profileTypesByPath := make(map[string]map[string]struct{})
	for _, file := range files {
		path := canonicalSigningPullPath(file.RelativePath)
		if file.Metadata.Kind == signingProfileArtifactKind {
			addSigningPullProfileTypeProvenance(profileTypesByPath, path, file.Metadata.ProfileType)
		}
		if file.Metadata.Kind != "identity-context" {
			continue
		}
		var binding identityContextBinding
		if err := json.Unmarshal(file.Plaintext, &binding); err != nil {
			return nil, nil, fmt.Errorf("decode identity context for pull selection: %w", err)
		}
		addSigningPullProfileTypeProvenance(profileTypesByPath, binding.ProfilePath, binding.ProfileType)
		if strings.EqualFold(strings.TrimSpace(binding.ProfileType), profileType) {
			contextsByBundle[binding.BundleID] = append(contextsByBundle[binding.BundleID], file)
		}
	}
	for _, file := range files {
		path := canonicalSigningPullPath(file.RelativePath)
		filesByPath[path] = file
		switch {
		case strings.HasSuffix(strings.ToLower(path), ".cer"):
			certificate, err := x509.ParseCertificate(file.Plaintext)
			if err != nil {
				return nil, nil, fmt.Errorf("stored certificate %s is invalid", path)
			}
			fingerprint := signingCertificateSHA256(certificate)
			if prior, exists := certificatesByFingerprint[fingerprint]; exists && canonicalSigningPullPath(prior.RelativePath) != path {
				return nil, nil, fmt.Errorf("stored certificate fingerprint appears at multiple repository paths")
			}
			certificatesByFingerprint[fingerprint] = file
		case shared.IsProvisioningProfilePath(path):
			profile, err := parseIdentityMobileProvision(file.Plaintext)
			if err != nil {
				return nil, nil, fmt.Errorf("stored profile %s is invalid: %w", path, err)
			}
			bundleID, err := signingPullProfileBundleID(profile)
			if err != nil {
				return nil, nil, fmt.Errorf("stored profile %s: %w", path, err)
			}
			if err := validateSigningProfileSelectionScope(file, profile, bundleID); err != nil {
				return nil, nil, err
			}
			if !profile.ExpirationDate.After(time.Now()) || !identityProfileTypeMatches(profile, profileType) || !signingPullProfilePlatformMatches(profile, profileType) ||
				!signingPullProfileHasExactTypeProvenance(profileTypesByPath[path], profileType) {
				continue
			}
			fingerprints := make([]string, 0, len(profile.DeveloperCertificates))
			for _, der := range profile.DeveloperCertificates {
				certificate, parseErr := x509.ParseCertificate(der)
				if parseErr != nil {
					return nil, nil, fmt.Errorf("stored profile %s contains an invalid developer certificate", path)
				}
				fingerprints = append(fingerprints, signingCertificateSHA256(certificate))
			}
			if len(fingerprints) == 0 {
				return nil, nil, fmt.Errorf("stored profile %s contains no developer certificates", path)
			}
			profilesByBundle[bundleID] = append(profilesByBundle[bundleID], signingPullProfile{
				file:        file,
				certificate: uniqueSortedSigningSyncStrings(fingerprints),
			})
		}
	}

	selectedByPath := make(map[string]decryptedSigningFile)
	targets := make([]SyncTargetResult, 0, len(requested))
	missing := make([]string, 0)
	for _, bundleID := range requested {
		profiles := profilesByBundle[bundleID]
		if len(profiles) == 0 {
			missing = append(missing, bundleID)
			continue
		}
		sort.Slice(profiles, func(i, j int) bool {
			return canonicalSigningPullPath(profiles[i].file.RelativePath) < canonicalSigningPullPath(profiles[j].file.RelativePath)
		})

		targetPaths := make(map[string]decryptedSigningFile)
		profilePaths := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			path := canonicalSigningPullPath(profile.file.RelativePath)
			targetPaths[path] = profile.file
			profilePaths = append(profilePaths, path)
			storedCertificateFound := false
			for _, fingerprint := range profile.certificate {
				certificate, exists := certificatesByFingerprint[fingerprint]
				if !exists {
					continue
				}
				storedCertificateFound = true
				targetPaths[canonicalSigningPullPath(certificate.RelativePath)] = certificate
			}
			if !storedCertificateFound {
				return nil, nil, fmt.Errorf("selected profile %s has no matching stored public certificate", path)
			}
		}

		for _, contextFile := range contextsByBundle[bundleID] {
			var binding identityContextBinding
			if err := json.Unmarshal(contextFile.Plaintext, &binding); err != nil {
				return nil, nil, fmt.Errorf("decode selected identity context: %w", err)
			}
			profilePath := canonicalSigningPullPath(binding.ProfilePath)
			if _, selected := targetPaths[profilePath]; !selected {
				return nil, nil, fmt.Errorf("selected identity context for %s does not reference a selected profile", bundleID)
			}
			contextPath := canonicalSigningPullPath(contextFile.RelativePath)
			targetPaths[contextPath] = contextFile
			corePath := canonicalSigningPullPath(filepath.Join("identities", certDirectoryName(binding.ProfileType), binding.CertificateSHA256+".p12"))
			core, exists := filesByPath[corePath]
			if !exists || core.Metadata.Kind != "pkcs12-identity" {
				return nil, nil, fmt.Errorf("selected identity context for %s has no usable core identity", bundleID)
			}
			targetPaths[corePath] = core
			certificate, exists := certificatesByFingerprint[binding.CertificateSHA256]
			if !exists {
				return nil, nil, fmt.Errorf("selected identity context for %s has no matching stored public certificate", bundleID)
			}
			targetPaths[canonicalSigningPullPath(certificate.RelativePath)] = certificate
		}

		targetFiles := signingPullSortedFiles(targetPaths)
		for _, file := range targetFiles {
			selectedByPath[canonicalSigningPullPath(file.RelativePath)] = file
		}
		targets = append(targets, SyncTargetResult{
			BundleID:       bundleID,
			ProfileType:    profileType,
			ProfilePath:    profilePaths[0],
			ProfilePaths:   profilePaths,
			ProfileCreated: false,
			Files:          signingPullRelativePathsFromFiles(targetFiles),
		})
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("no active %s profile found in encrypted repository for bundle ID(s): %s", profileType, strings.Join(missing, ", "))
	}
	return signingPullSortedFiles(selectedByPath), targets, nil
}

func addSigningPullProfileTypeProvenance(index map[string]map[string]struct{}, path, profileType string) {
	path = canonicalSigningPullPath(path)
	profileType = strings.ToUpper(strings.TrimSpace(profileType))
	if path == "" || profileType == "" {
		return
	}
	if index[path] == nil {
		index[path] = make(map[string]struct{})
	}
	index[path][profileType] = struct{}{}
}

func signingPullProfileHasExactTypeProvenance(profileTypes map[string]struct{}, profileType string) bool {
	profileType = strings.ToUpper(strings.TrimSpace(profileType))
	if len(profileTypes) > 0 {
		_, exists := profileTypes[profileType]
		return exists
	}
	return !strings.HasPrefix(profileType, "MAC_APP_") && !strings.HasPrefix(profileType, "MAC_CATALYST_APP_")
}

func signingPullProfileBundleID(profile *identityMobileProvision) (string, error) {
	if profile == nil {
		return "", fmt.Errorf("profile is missing")
	}
	applicationIdentifier, _ := profile.Entitlements["application-identifier"].(string)
	if applicationIdentifier == "" {
		applicationIdentifier, _ = profile.Entitlements["com.apple.application-identifier"].(string)
	}
	applicationIdentifier = strings.TrimSpace(applicationIdentifier)
	if applicationIdentifier == "" {
		return "", fmt.Errorf("profile has no application identifier entitlement")
	}
	for _, prefix := range profile.ApplicationIdentifierPrefix {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		bundleID, found := strings.CutPrefix(applicationIdentifier, prefix+".")
		if found && strings.TrimSpace(bundleID) != "" && !strings.Contains(bundleID, "*") {
			return bundleID, nil
		}
	}
	return "", fmt.Errorf("profile application identifier does not match its declared prefix")
}

func signingPullProfilePlatformMatches(profile *identityMobileProvision, profileType string) bool {
	if profile == nil || len(profile.Platform) == 0 {
		return false
	}
	wanted := ""
	switch {
	case strings.HasPrefix(profileType, "IOS_"):
		wanted = "ios"
	case strings.HasPrefix(profileType, "TVOS_"):
		wanted = "tvos"
	case strings.HasPrefix(profileType, "MAC_"), strings.HasPrefix(profileType, "MAC_CATALYST_"):
		wanted = "osx"
	}
	if wanted == "" {
		return false
	}
	for _, platform := range profile.Platform {
		normalized := strings.ToLower(strings.TrimSpace(platform))
		if normalized == wanted || (wanted == "osx" && normalized == "macos") {
			return true
		}
	}
	return false
}

func signingPullSortedFiles(files map[string]decryptedSigningFile) []decryptedSigningFile {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]decryptedSigningFile, 0, len(paths))
	for _, path := range paths {
		result = append(result, files[path])
	}
	return result
}

func signingPullRelativePathsFromFiles(files []decryptedSigningFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, canonicalSigningPullPath(file.RelativePath))
	}
	return uniqueSortedSigningSyncStrings(paths)
}

func canonicalSigningPullPath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), `\`, "/")
}

func preflightSigningPullFiles(outputDir string, files []decryptedSigningFile) error {
	root, err := rootfs.New(outputDir)
	if err != nil {
		return fmt.Errorf("create output root: %w", err)
	}
	defer root.Close()
	return preflightSigningPullFilesInRoot(root, files)
}

func preflightSigningPullFilesInRoot(root rootfs.Root, files []decryptedSigningFile) error {
	for _, file := range files {
		var err error
		if file.Sensitive {
			err = root.CheckCreateNewFile(file.RelativePath)
		} else {
			err = root.CheckWriteFilePreservingMode(file.RelativePath)
		}
		if err != nil {
			return fmt.Errorf("preflight output %s: %w", file.RelativePath, err)
		}
	}
	return nil
}
