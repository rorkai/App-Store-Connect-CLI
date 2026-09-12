package signing

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

func TestSigningProfileArtifactUpgradesLegacyAndPreservesExactScope(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	const password = "repository-password"
	path := filepath.Join("profiles", "appstore", "profile.mobileprovision")
	plaintext := []byte("signed-profile-content")
	if err := store.WriteEncryptedFile(path, plaintext, password); err != nil {
		t.Fatal(err)
	}
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{ID: "profile-1"}}
	metadata, err := signingProfileArtifactMetadata(profile, "com.example.mac", "MAC_CATALYST_APP_STORE")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningProfileArtifact(store, path, plaintext, password, metadata); err != nil {
		t.Fatal(err)
	}
	got, gotMetadata, err := store.ReadEncryptedFileWithMetadata(path, password)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) || gotMetadata.Version != 1 || gotMetadata.Kind != signingProfileArtifactKind ||
		gotMetadata.BundleID != "com.example.mac" || gotMetadata.ProfileType != "MAC_CATALYST_APP_STORE" || gotMetadata.ProfileResourceID != "profile-1" {
		t.Fatalf("artifact metadata = %+v plaintext=%q", gotMetadata, got)
	}
	repositoryRoot, err := rootfs.New(store.LocalDir)
	if err != nil {
		t.Fatal(err)
	}
	defer repositoryRoot.Close()
	encryptedPath := path + ".enc"
	before, err := repositoryRoot.ReadFileLimited(encryptedPath, maxEncryptedSigningBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningProfileArtifact(store, path, plaintext, password, metadata); err != nil {
		t.Fatal(err)
	}
	after, err := repositoryRoot.ReadFileLimited(encryptedPath, maxEncryptedSigningBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("semantic no-op rewrote authenticated profile ciphertext")
	}

	wrongScope := metadata
	wrongScope.ProfileType = "MAC_APP_STORE"
	if _, err := preflightSigningProfileArtifact(store, path, plaintext, password, wrongScope); err == nil || !strings.Contains(err.Error(), "different authenticated scope") {
		t.Fatalf("wrong-scope error = %v", err)
	}
}

func TestClassifySigningProfileArtifactRejectsIncompleteMetadata(t *testing.T) {
	_, _, err := classifySigningFile("profiles/appstore/profile.mobileprovision", []byte("profile"), signingpkg.EncryptedFileMetadata{
		Version:     1,
		Kind:        signingProfileArtifactKind,
		BundleID:    "com.example.app",
		ProfileType: "IOS_APP_STORE",
	}, "password")
	if err == nil || !strings.Contains(err.Error(), "missing its authenticated scope") {
		t.Fatalf("error = %v, want incomplete metadata refusal", err)
	}
}

func TestValidateSigningProfileArtifactMetadataAcceptsMacOSExtension(t *testing.T) {
	for _, path := range []string{
		"profiles/appstore/profile.provisionprofile",
		"profiles/appstore/profile.mobileprovision",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			err := validateSigningProfileArtifactMetadata(path, signingpkg.EncryptedFileMetadata{
				Version:           1,
				Kind:              signingProfileArtifactKind,
				BundleID:          "com.example.mac",
				ProfileType:       "MAC_APP_STORE",
				ProfileResourceID: "profile-1",
			})
			if err != nil {
				t.Fatalf("validateSigningProfileArtifactMetadata(%q): %v", path, err)
			}
		})
	}
}

func TestResolveCompatibleSigningProfilePathPreservesLegacyMacOSArtifact(t *testing.T) {
	const password = "repository-password"
	preferred := filepath.Join("profiles", "appstore", "profile.provisionprofile")
	legacy := filepath.Join("profiles", "appstore", "profile.mobileprovision")

	t.Run("legacy only", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFile(legacy, []byte("profile"), password); err != nil {
			t.Fatal(err)
		}
		got, err := resolveCompatibleSigningProfilePath(store, preferred)
		if err != nil || got != legacy {
			t.Fatalf("path = %q, error = %v, want legacy path %q", got, err, legacy)
		}
	})

	t.Run("preferred only", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFile(preferred, []byte("profile"), password); err != nil {
			t.Fatal(err)
		}
		got, err := resolveCompatibleSigningProfilePath(store, preferred)
		if err != nil || got != preferred {
			t.Fatalf("path = %q, error = %v, want preferred path %q", got, err, preferred)
		}
	})

	t.Run("both spellings", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		for _, path := range []string{legacy, preferred} {
			if err := store.WriteEncryptedFile(path, []byte("profile"), password); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := resolveCompatibleSigningProfilePath(store, preferred); err == nil || !strings.Contains(err.Error(), "exists at both") {
			t.Fatalf("error = %v, want ambiguous repository refusal", err)
		}
	})

	t.Run("iOS path is unchanged", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		got, err := resolveCompatibleSigningProfilePath(store, legacy)
		if err != nil || got != legacy {
			t.Fatalf("path = %q, error = %v, want %q", got, err, legacy)
		}
	})
}
