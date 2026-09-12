package signing

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	"howett.net/plist"
)

func TestSigningSyncPullSelectorFlagsFailBeforeSecretsOrRepository(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "bundle requires profile type",
			args: []string{"--repo", "git@example.com:team/signing.git", "--bundle-id", "com.example.app"},
			want: "--profile-type is required with --bundle-id or --targets-file",
		},
		{
			name: "targets require profile type",
			args: []string{"--repo", "git@example.com:team/signing.git", "--targets-file", "targets.json"},
			want: "--profile-type is required with --bundle-id or --targets-file",
		},
		{
			name: "profile type requires selector",
			args: []string{"--repo", "git@example.com:team/signing.git", "--profile-type", "IOS_APP_STORE"},
			want: "--profile-type requires --bundle-id or --targets-file",
		},
		{
			name: "selectors conflict",
			args: []string{"--repo", "git@example.com:team/signing.git", "--bundle-id", "com.example.app", "--targets-file", "targets.json", "--profile-type", "IOS_APP_STORE"},
			want: "--bundle-id and --targets-file are mutually exclusive",
		},
		{
			name: "profile type must use exact supported value",
			args: []string{"--repo", "git@example.com:team/signing.git", "--bundle-id", "com.example.app", "--profile-type", "IOS_APP_STORE_BOGUS"},
			want: "--profile-type must be a supported App Store Connect profile type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, "must-not-be-used")
			cmd := syncPullCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || err.Error() != test.want || !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error %q", err, test.want)
			}
		})
	}
}

func TestSelectSigningPullFilesChoosesDirectDistributionTarget(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 807)
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"ProvisionsAllDevices":        true,
		"Platform":                    []string{"OSX"},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123.com.example.direct",
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profile := mustSignedCMS(t, profilePlist, certificate, key)
	files := []decryptedSigningFile{
		{RelativePath: "certs/distribution/direct.cer", Plaintext: certificate.Raw},
		{
			RelativePath: "profiles/direct/direct.provisionprofile",
			Plaintext:    profile,
			Metadata: signingpkg.EncryptedFileMetadata{
				Version:           1,
				Kind:              signingProfileArtifactKind,
				BundleID:          "com.example.direct",
				ProfileType:       "MAC_APP_DIRECT",
				ProfileResourceID: "profile-direct",
			},
		},
	}

	selected, targets, err := selectSigningPullFiles(files, []string{"com.example.direct"}, "MAC_APP_DIRECT")
	if err != nil {
		t.Fatal(err)
	}
	if got := signingPullRelativePaths(selected); !slices.Equal(got, []string{"certs/distribution/direct.cer", "profiles/direct/direct.provisionprofile"}) {
		t.Fatalf("selected paths = %v", got)
	}
	if len(targets) != 1 || targets[0].ProfileType != "MAC_APP_DIRECT" {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestSelectSigningPullFilesRequiresExactMacProfileProvenance(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 814)
	profile := mustSignedMacStoreProfile(t, certificate, key, "com.example.mac")
	certificateFile := decryptedSigningFile{RelativePath: "certs/distribution/mac.cer", Plaintext: certificate.Raw}

	for _, profileType := range []string{"MAC_APP_STORE", "MAC_CATALYST_APP_STORE"} {
		t.Run("legacy "+profileType, func(t *testing.T) {
			files := []decryptedSigningFile{
				certificateFile,
				{RelativePath: "profiles/appstore/mac.mobileprovision", Plaintext: profile},
			}
			_, _, err := selectSigningPullFiles(files, []string{"com.example.mac"}, profileType)
			if err == nil || !strings.Contains(err.Error(), "no active "+profileType+" profile") {
				t.Fatalf("error = %v, want ambiguous legacy Mac profile refusal", err)
			}
		})

		t.Run("authenticated "+profileType, func(t *testing.T) {
			files := []decryptedSigningFile{
				certificateFile,
				{
					RelativePath: "profiles/appstore/mac.mobileprovision",
					Plaintext:    profile,
					Metadata: signingpkg.EncryptedFileMetadata{
						Version:           1,
						Kind:              "provisioning-profile",
						BundleID:          "com.example.mac",
						ProfileType:       profileType,
						ProfileResourceID: "profile-mac",
					},
				},
			}
			selected, _, err := selectSigningPullFiles(files, []string{"com.example.mac"}, profileType)
			if err != nil {
				t.Fatal(err)
			}
			if got := signingPullRelativePaths(selected); !slices.Equal(got, []string{"certs/distribution/mac.cer", "profiles/appstore/mac.mobileprovision"}) {
				t.Fatalf("selected paths = %v", got)
			}
			otherType := "MAC_APP_STORE"
			if profileType == otherType {
				otherType = "MAC_CATALYST_APP_STORE"
			}
			if _, _, err := selectSigningPullFiles(files, []string{"com.example.mac"}, otherType); err == nil {
				t.Fatalf("authenticated %s profile selected as %s", profileType, otherType)
			}
		})
	}
}

func TestSelectSigningPullFilesRejectsProfileWithoutPlatform(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 813)
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123.com.example.missing-platform",
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profile := mustSignedCMS(t, profilePlist, certificate, key)
	files := []decryptedSigningFile{
		{RelativePath: "certs/distribution/certificate.cer", Plaintext: certificate.Raw},
		{RelativePath: "profiles/appstore/profile.mobileprovision", Plaintext: profile},
	}

	_, _, err = selectSigningPullFiles(files, []string{"com.example.missing-platform"}, "IOS_APP_STORE")
	if err == nil || !strings.Contains(err.Error(), "com.example.missing-platform") {
		t.Fatalf("error = %v, want exact-platform selection failure", err)
	}
}

func TestSigningSyncPullOneTargetManifestUsesBatchOutputShape(t *testing.T) {
	result := SyncResult{Operation: "pull", Files: []string{"profile.mobileprovision"}}
	targets := []SyncTargetResult{{
		BundleID:     "com.example.app",
		ProfileType:  "IOS_APP_STORE",
		ProfilePath:  "profile.mobileprovision",
		ProfilePaths: []string{"profile.mobileprovision"},
		Files:        []string{"profile.mobileprovision"},
	}}
	applySigningPullSelectionResult(&result, "IOS_APP_STORE", []string{"com.example.app"}, targets, true)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatal(err)
	}
	if _, exists := shape["bundleId"]; exists {
		t.Fatalf("one-target manifest emitted singular bundleId: %s", encoded)
	}
	for _, key := range []string{"bundleIds", "targets"} {
		if _, exists := shape[key]; !exists {
			t.Fatalf("one-target manifest omitted %s: %s", key, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"profilePaths":["profile.mobileprovision"]`) {
		t.Fatalf("one-target manifest omitted profilePaths: %s", encoded)
	}
}

func TestSigningSyncPullReadsTargetsManifestBeforePassword(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "must-not-be-used")
	cmd := syncPullCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{
		"--repo", "git@example.com:team/signing.git",
		"--targets-file", "missing-targets.json",
		"--profile-type", "IOS_APP_STORE",
	}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "targets manifest") || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want manifest usage error", err)
	}
}

func TestSelectSigningPullFilesChoosesCertificateOnlyTarget(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 801)
	selectedProfile := mustSignedProfile(t, certificate, key, "TEAM123", "TEAM123.com.example.selected", time.Now().Add(time.Hour))
	otherProfile := mustSignedProfile(t, certificate, key, "TEAM123", "TEAM123.com.example.other", time.Now().Add(time.Hour))

	selectedPath := "profiles/adhoc/selected.mobileprovision"
	secondSelectedPath := "profiles/adhoc/selected-second.mobileprovision"
	otherPath := "profiles/adhoc/other.mobileprovision"
	certificatePath := "certs/distribution/shared.cer"
	files := []decryptedSigningFile{
		{RelativePath: certificatePath, Plaintext: certificate.Raw},
		{RelativePath: otherPath, Plaintext: otherProfile},
		{RelativePath: selectedPath, Plaintext: selectedProfile},
		{RelativePath: secondSelectedPath, Plaintext: selectedProfile},
	}

	selected, targets, err := selectSigningPullFiles(files, []string{"com.example.selected"}, "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	if got := signingPullRelativePaths(selected); !slices.Equal(got, []string{certificatePath, secondSelectedPath, selectedPath}) {
		t.Fatalf("selected paths = %v", got)
	}
	if len(targets) != 1 || targets[0].BundleID != "com.example.selected" || targets[0].ProfilePath != secondSelectedPath {
		t.Fatalf("targets = %#v", targets)
	}
	if !slices.Equal(targets[0].ProfilePaths, []string{secondSelectedPath, selectedPath}) {
		t.Fatalf("target profile paths = %v", targets[0].ProfilePaths)
	}
	if !slices.Equal(targets[0].Files, []string{certificatePath, secondSelectedPath, selectedPath}) {
		t.Fatalf("target files = %v", targets[0].Files)
	}
}

func TestSelectSigningPullFilesAcceptsStoredSubsetOfEmbeddedCertificates(t *testing.T) {
	storedKey := mustECKey(t)
	storedCertificate := mustSigningCertificate(t, storedKey, 808)
	additionalKey := mustECKey(t)
	additionalCertificate := mustSigningCertificate(t, additionalKey, 809)
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"Platform":                    []string{"iOS"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{storedCertificate.Raw, additionalCertificate.Raw},
		"ProvisionedDevices":          []string{"DEVICE1"},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123.com.example.selected",
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profile := mustSignedCMS(t, profilePlist, storedCertificate, storedKey)
	files := []decryptedSigningFile{
		{RelativePath: "certs/distribution/stored.cer", Plaintext: storedCertificate.Raw},
		{RelativePath: "profiles/adhoc/selected.mobileprovision", Plaintext: profile},
	}

	selected, _, err := selectSigningPullFiles(files, []string{"com.example.selected"}, "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"certs/distribution/stored.cer", "profiles/adhoc/selected.mobileprovision"}
	if got := signingPullRelativePaths(selected); !slices.Equal(got, want) {
		t.Fatalf("selected paths = %v, want %v", got, want)
	}
}

func TestSelectSigningPullFilesRequiresExactIdentityPublicCertificate(t *testing.T) {
	const password = "repository-password"
	identityKey := mustECKey(t)
	identityCertificate := mustSigningCertificate(t, identityKey, 811)
	otherKey := mustECKey(t)
	otherCertificate := mustSigningCertificate(t, otherKey, 812)
	identity := &signingIdentity{
		PrivateKey:        identityKey,
		Certificate:       identityCertificate,
		CertificateSHA256: certificateSHA256(identityCertificate),
	}
	artifacts, err := prepareSigningIdentityArtifacts(identity, password, "com.example.selected", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"Platform":                    []string{"iOS"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{identityCertificate.Raw, otherCertificate.Raw},
		"ProvisionedDevices":          []string{"DEVICE1"},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123.com.example.selected",
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profileContent := mustSignedCMS(t, profilePlist, identityCertificate, identityKey)
	profilePath := "profiles/adhoc/selected.mobileprovision"
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{
		ID: "profile-selected",
		Attributes: asc.ProfileAttributes{
			Name: "profile-selected",
			UUID: "01234567-89ab-cdef-0123-456789abcdef",
		},
	}}
	if err := bindSigningIdentityProfile(artifacts, profile, profilePath, profileContent); err != nil {
		t.Fatal(err)
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	otherCertificatePath := "certs/distribution/other.cer"
	if err := store.WriteEncryptedFile(otherCertificatePath, otherCertificate.Raw, password); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteEncryptedFile(profilePath, profileContent, password); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, password); err != nil {
		t.Fatal(err)
	}
	paths := []string{otherCertificatePath, profilePath, artifacts.IdentityPath, artifacts.BindingPath}
	decrypted, err := prepareDecryptedSigningFiles(store, paths, password, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = selectSigningPullFiles(decrypted, []string{"com.example.selected"}, "IOS_APP_ADHOC")
	if err == nil || !strings.Contains(err.Error(), "identity context") || !strings.Contains(err.Error(), "stored public certificate") {
		t.Fatalf("error = %v, want exact identity certificate requirement", err)
	}
}

func TestSigningIdentityGraphRejectsDirectProfileWithIOSPlatform(t *testing.T) {
	const password = "repository-password"
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 810)
	identity := &signingIdentity{
		PrivateKey:        key,
		Certificate:       certificate,
		CertificateSHA256: certificateSHA256(certificate),
	}
	artifacts, err := prepareSigningIdentityArtifacts(identity, password, "com.example.direct", "MAC_APP_DIRECT")
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"ProvisionsAllDevices":        true,
		"Platform":                    []string{"iOS"},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123.com.example.direct",
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profileContent := mustSignedCMS(t, profilePlist, certificate, key)
	profilePath := "profiles/direct/direct.mobileprovision"
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{
		ID: "profile-direct",
		Attributes: asc.ProfileAttributes{
			Name: "profile-direct",
			UUID: "01234567-89ab-cdef-0123-456789abcdef",
		},
	}}
	if err := bindSigningIdentityProfile(artifacts, profile, profilePath, profileContent); err != nil {
		t.Fatal(err)
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if err := store.WriteEncryptedFile(profilePath, profileContent, password); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, password); err != nil {
		t.Fatal(err)
	}
	paths := []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}
	if _, err := prepareDecryptedSigningFiles(store, paths, password, t.TempDir()); err == nil || !strings.Contains(err.Error(), "distribution type") {
		t.Fatalf("error = %v, want direct-profile platform mismatch", err)
	}
}

func TestSelectSigningPullFilesRejectsCorruptUnselectedProfile(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 804)
	profile := mustSignedProfile(t, certificate, key, "TEAM123", "TEAM123.com.example.selected", time.Now().Add(time.Hour))
	files := []decryptedSigningFile{
		{RelativePath: "certs/distribution/shared.cer", Plaintext: certificate.Raw},
		{RelativePath: "profiles/adhoc/selected.mobileprovision", Plaintext: profile},
		{RelativePath: "profiles/adhoc/unselected.mobileprovision", Plaintext: []byte("not CMS")},
	}

	_, _, err := selectSigningPullFiles(files, []string{"com.example.selected"}, "IOS_APP_ADHOC")
	if err == nil || !strings.Contains(err.Error(), "unselected.mobileprovision") {
		t.Fatalf("error = %v, want corrupt unselected profile refusal", err)
	}
}

func TestSelectSigningPullFilesRequiresStoredProfileCertificate(t *testing.T) {
	profileKey := mustECKey(t)
	profileCertificate := mustSigningCertificate(t, profileKey, 805)
	storedKey := mustECKey(t)
	storedCertificate := mustSigningCertificate(t, storedKey, 806)
	profile := mustSignedProfile(t, profileCertificate, profileKey, "TEAM123", "TEAM123.com.example.selected", time.Now().Add(time.Hour))
	files := []decryptedSigningFile{
		{RelativePath: "certs/distribution/other.cer", Plaintext: storedCertificate.Raw},
		{RelativePath: "profiles/adhoc/selected.mobileprovision", Plaintext: profile},
	}

	_, _, err := selectSigningPullFiles(files, []string{"com.example.selected"}, "IOS_APP_ADHOC")
	if err == nil || !strings.Contains(err.Error(), "no matching stored public certificate") {
		t.Fatalf("error = %v, want missing stored certificate refusal", err)
	}
}

func TestSelectSigningPullFilesKeepsOnlyRequestedIdentityContext(t *testing.T) {
	password := "repository-password"
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 802)
	identity := &signingIdentity{
		PrivateKey:        key,
		Certificate:       certificate,
		CertificateSHA256: certificateSHA256(certificate),
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	certificatePath := "certs/distribution/shared.cer"
	if err := store.WriteEncryptedFile(certificatePath, certificate.Raw, password); err != nil {
		t.Fatal(err)
	}

	allPaths := []string{certificatePath}
	contexts := make(map[string]*signingIdentityArtifacts)
	profiles := make(map[string]string)
	for _, bundleID := range []string{"com.example.selected", "com.example.other"} {
		artifacts, err := prepareSigningIdentityArtifacts(identity, password, bundleID, "IOS_APP_ADHOC")
		if err != nil {
			t.Fatal(err)
		}
		profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, bundleID, "IOS_APP_ADHOC", strings.TrimPrefix(bundleID, "com.example."))
		if err := store.WriteEncryptedFile(profilePath, profileContent, password); err != nil {
			t.Fatal(err)
		}
		if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, password); err != nil {
			t.Fatal(err)
		}
		contexts[bundleID] = artifacts
		profiles[bundleID] = profilePath
		allPaths = append(allPaths, profilePath, artifacts.IdentityPath, artifacts.BindingPath)
	}
	allPaths = uniqueSortedSigningSyncStrings(allPaths)
	decrypted, err := prepareDecryptedSigningFiles(store, allPaths, password, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	selected, targets, err := selectSigningPullFiles(decrypted, []string{"com.example.selected"}, "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	paths := signingPullRelativePaths(selected)
	want := uniqueSortedSigningSyncStrings([]string{
		certificatePath,
		profiles["com.example.selected"],
		contexts["com.example.selected"].IdentityPath,
		contexts["com.example.selected"].BindingPath,
	})
	if !slices.Equal(paths, want) {
		t.Fatalf("selected paths = %v, want %v", paths, want)
	}
	if slices.Contains(paths, profiles["com.example.other"]) || slices.Contains(paths, contexts["com.example.other"].BindingPath) {
		t.Fatalf("selected paths contain other target: %v", paths)
	}
	if len(targets) != 1 || !slices.Equal(targets[0].Files, want) {
		t.Fatalf("targets = %#v, want files %v", targets, want)
	}
}

func TestSelectSigningPullFilesRequiresEveryRequestedTarget(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 803)
	profile := mustSignedProfile(t, certificate, key, "TEAM123", "TEAM123.com.example.present", time.Now().Add(time.Hour))
	files := []decryptedSigningFile{
		{RelativePath: "certs/distribution/shared.cer", Plaintext: certificate.Raw},
		{RelativePath: "profiles/adhoc/present.mobileprovision", Plaintext: profile},
	}

	_, _, err := selectSigningPullFiles(files, []string{"com.example.missing", "com.example.present"}, "IOS_APP_ADHOC")
	if err == nil || !strings.Contains(err.Error(), "com.example.missing") {
		t.Fatalf("error = %v, want missing target", err)
	}
}

func TestPreflightSigningPullFilesIgnoresUnselectedDestinationCollision(t *testing.T) {
	rootDir := t.TempDir()
	unselectedPath := filepath.Join("profiles", "adhoc", "other.mobileprovision")
	destination := filepath.Join(rootDir, unselectedPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	selected := []decryptedSigningFile{{RelativePath: filepath.Join("profiles", "adhoc", "selected.mobileprovision")}}
	if err := preflightSigningPullFiles(rootDir, selected); err != nil {
		t.Fatalf("selected preflight was blocked by unrelated destination: %v", err)
	}
}

func signingPullRelativePaths(files []decryptedSigningFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, filepath.ToSlash(file.RelativePath))
	}
	return uniqueSortedSigningSyncStrings(paths)
}

func mustSignedMacStoreProfile(t *testing.T, certificate *x509.Certificate, privateKey any, bundleID string) []byte {
	t.Helper()
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":                        "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"Platform":                    []string{"OSX"},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123." + bundleID,
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return mustSignedCMS(t, profilePlist, certificate, privateKey)
}
