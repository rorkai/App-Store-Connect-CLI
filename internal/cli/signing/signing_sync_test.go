package signing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

func TestSigningSyncCommandLongHelpUsesOutputDirExample(t *testing.T) {
	cmd := SigningSyncCommand()
	if !strings.Contains(cmd.LongHelp, "--output-dir ./signing") {
		t.Fatalf("expected long help to document --output-dir, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "--output ./signing") {
		t.Fatalf("expected long help to avoid --output path example, got %q", cmd.LongHelp)
	}
}

func TestSigningSyncPushHelpDocumentsDeviceTransition(t *testing.T) {
	deviceFlag := syncPushCommand().FlagSet.Lookup("device")
	if deviceFlag == nil {
		t.Fatal("expected --device flag")
	}
	if !strings.Contains(deviceFlag.Usage, "--create-missing") ||
		!strings.Contains(deviceFlag.Usage, "deprecated") ||
		!strings.Contains(deviceFlag.Usage, "5.0.0") {
		t.Fatalf("--device usage = %q, want the transition and rejection release", deviceFlag.Usage)
	}
}

func TestSigningSyncPreparesRepositoryOnceInAssetOrder(t *testing.T) {
	tests := []struct {
		name       string
		hasProfile bool
		wantEvents []string
	}{
		{
			name:       "existing profile",
			hasProfile: true,
			wantEvents: []string{
				"GET /v1/bundleIds/bundle-main/profiles",
				"GET /v1/profiles/profile-main/certificates",
				"clone repository",
			},
		},
		{
			name: "missing profile",
			wantEvents: []string{
				"GET /v1/bundleIds/bundle-main/profiles",
				"GET /v1/certificates",
				"clone repository",
				"POST /v1/profiles",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
				events = append(events, req.Method+" "+req.URL.Path)
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
					if tt.hasProfile {
						return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-main","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`)
					}
					return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles/profile-main/certificates":
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION"}}]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
					return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
				}
			})

			cloneCount := 0
			prepareRepository := onceAfterSuccess(func() error {
				cloneCount++
				events = append(events, "clone repository")
				return nil
			})
			_, _, _, err := resolveSigningAssets(
				context.Background(),
				client,
				signingAssetsOptions{
					BundleIDResourceID: "bundle-main",
					BundleIdentifier:   "com.example.signing.profile",
					ProfileType:        "IOS_APP_STORE",
					CreateMissing:      !tt.hasProfile,
					BeforeCreate: func(profileCreatePlan) error {
						return prepareRepository()
					},
				},
			)
			if err != nil {
				t.Fatalf("resolveSigningAssets() error: %v", err)
			}
			if err := prepareRepository(); err != nil {
				t.Fatalf("prepareRepository() error: %v", err)
			}
			if cloneCount != 1 {
				t.Fatalf("repository clone count = %d, want 1", cloneCount)
			}
			if strings.Join(events, ",") != strings.Join(tt.wantEvents, ",") {
				t.Fatalf("unexpected operation order: got %v, want %v", events, tt.wantEvents)
			}
		})
	}
}

func TestSigningSyncIdentityConflictFailsBeforeProfileCreatePOST(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 19)
	active := true
	certificateJSON := fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":%t,"expirationDate":%q,"certificateContent":%q}}]}`,
		active, time.Now().Add(time.Hour).Format(time.RFC3339), base64.StdEncoding.EncodeToString(certificate.Raw))
	postCalled := false
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, certificateJSON)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			postCalled = true
			return signingFetchJSONResponse(http.StatusCreated, `{}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	identity := &signingIdentity{PrivateKey: key}
	_, _, _, err := resolveSigningAssets(context.Background(), client, signingAssetsOptions{
		BundleIDResourceID: "bundle-main",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_ADHOC",
		CreateMissing:      true,
		CertificateFilter:  identityCertificateFilter(identity),
		BeforeCreate: func(plan profileCreatePlan) error {
			if err := preflightIdentityForProfileCreate(identity, plan, "repository-password", time.Now()); err != nil {
				return err
			}
			artifacts, err := prepareSigningIdentityArtifacts(identity, "repository-password", "com.example.app", "IOS_APP_ADHOC")
			if err != nil {
				return err
			}
			path := filepath.Join(store.LocalDir, artifacts.IdentityPath+".enc")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("conflicting-artifact"), 0o600); err != nil {
				return err
			}
			return preflightSigningIdentityArtifacts(store, artifacts, "repository-password")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "before creating profile") {
		t.Fatalf("resolveSigningAssets() error = %v", err)
	}
	if postCalled {
		t.Fatal("profile POST occurred after local identity conflict")
	}
}

func TestSigningSyncLegacyDestinationConflictFailsBeforeProfileCreatePOST(t *testing.T) {
	active := true
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"serialNumber":"SERIAL","certificateType":"IOS_DISTRIBUTION","activated":%t,"expirationDate":%q}}]}`,
				active, time.Now().Add(time.Hour).Format(time.RFC3339)))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			t.Fatal("profile POST occurred after local signing destination conflict")
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
	})
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	outside := t.TempDir()
	_, _, _, err := resolveSigningAssets(context.Background(), client, signingAssetsOptions{
		BundleIDResourceID: "bundle-main", BundleIdentifier: "com.example.app", ProfileType: "IOS_APP_ADHOC", CreateMissing: true,
		BeforeCreate: func(plan profileCreatePlan) error {
			conflict := filepath.Join(store.LocalDir, "profiles", "adhoc", safeFileName(plan.ProfileName, "profile")+".mobileprovision.enc")
			if err := os.MkdirAll(filepath.Dir(conflict), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(filepath.Join(outside, "profile.enc"), conflict); err != nil {
				return err
			}
			return preflightSigningAssetDestinations(store, plan, "IOS_APP_ADHOC")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "preflight profile destination") {
		t.Fatalf("legacy destination preflight error = %v", err)
	}
}

func TestSigningSyncPushWarnsForDeviceWithoutCreateMissing(t *testing.T) {
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client reached after validation")
	}))

	cmd := syncPushCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Parse([]string{
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--repo", "git@github.com:team/certs.git",
			"--password", "secret",
			"--device", "DEVICE1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})

	if runErr == nil || runErr.Error() != "signing sync push: client reached after validation" {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("deprecated input must not return a usage error: %v", runErr)
	}
	if clientCalls != 1 {
		t.Fatalf("client factory calls = %d, want 1", clientCalls)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	wantWarning := "Warning: --device without --create-missing is deprecated and ignored because device IDs are only applied when creating a profile. Add --create-missing so they can be applied if a profile must be created. This combination will be rejected in 5.0.0.\n" +
		"Warning: --password and ASC_MATCH_PASSWORD are deprecated for signing sync; use --password-file or ASC_SIGNING_SYNC_PASSWORD. The legacy sources will be removed in 5.0.0.\n"
	if stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
	}
}

func TestSanitizeRepoURLForOutputRedactsCredentials(t *testing.T) {
	raw := "https://token:secret@example.com/org/repo.git?access_token=abc123"
	got := sanitizeRepoURLForOutput(raw)

	if strings.Contains(got, "token:secret@") || strings.Contains(got, "secret") || strings.Contains(got, "abc123") {
		t.Fatalf("expected credentials to be redacted, got %q", got)
	}
	if !strings.Contains(got, "%5BREDACTED%5D") {
		t.Fatalf("expected sanitized marker, got %q", got)
	}
}

func TestSigningCommandLongHelpUsesOutputDirForSyncPull(t *testing.T) {
	cmd := SigningCommand()
	if !strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output-dir ./signing") {
		t.Fatalf("expected top-level help to use --output-dir for sync pull, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output ./signing") {
		t.Fatalf("expected top-level help to avoid --output for sync pull, got %q", cmd.LongHelp)
	}
}

func TestSigningSyncCommandLongHelpPullExampleOmitsUnsupportedFlags(t *testing.T) {
	cmd := SigningSyncCommand()
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --bundle-id") {
		t.Fatalf("expected pull example to omit --bundle-id, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --profile-type") {
		t.Fatalf("expected pull example to omit --profile-type, got %q", cmd.LongHelp)
	}
}

func TestWriteDecryptedOutputFileWritesPlaintext(t *testing.T) {
	outDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	plaintext := []byte("profile-data")

	if err := writeDecryptedOutputFile(outDir, relPath, plaintext, false); err != nil {
		t.Fatalf("writeDecryptedOutputFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, relPath))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("output mismatch: got %q, want %q", got, plaintext)
	}
	info, err := os.Stat(filepath.Join(outDir, relPath))
	if err != nil {
		t.Fatalf("stat output file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %o, want 600", info.Mode().Perm())
	}
}

func TestWriteDecryptedOutputFilePreservesExistingNonSensitiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows mode bits do not model Unix file permissions")
	}
	outDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	destination := filepath.Join(outDir, relPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeDecryptedOutputFile(outDir, relPath, []byte("new"), false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

func TestResolveSyncPasswordPrefersProtectedFileThenEnvironment(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "environment-password")
	t.Setenv(matchPasswordEnvVar, "legacy-environment-password")
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("file-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	password, legacy, err := resolveSyncPassword(path, "legacy-flag-password")
	if err != nil {
		t.Fatalf("resolveSyncPassword(file) error = %v", err)
	}
	if password != "file-password" || legacy {
		t.Fatalf("file result = %q, legacy=%v", password, legacy)
	}

	password, legacy, err = resolveSyncPassword("", "")
	if err != nil {
		t.Fatalf("resolveSyncPassword(env) error = %v", err)
	}
	if password != "environment-password" || legacy {
		t.Fatalf("environment result = %q, legacy=%v", password, legacy)
	}

	t.Setenv(signingSyncPasswordEnvVar, "")
	password, legacy, err = resolveSyncPassword("", "legacy-flag-password")
	if err != nil {
		t.Fatalf("resolveSyncPassword(legacy) error = %v", err)
	}
	if password != "legacy-flag-password" || !legacy {
		t.Fatalf("legacy result = %q, legacy=%v", password, legacy)
	}
}

func TestResolveSyncPasswordExplicitLegacyFlagWinsAmbientNewEnvironment(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "ambient-new-password")
	password, legacy, err := resolveSyncPassword("", " explicit-legacy-password ")
	if err != nil {
		t.Fatal(err)
	}
	if password != "explicit-legacy-password" || !legacy {
		t.Fatalf("password=%q legacy=%v", password, legacy)
	}
}

func TestResolveSyncPasswordPreservesEnvironmentWhitespace(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "  environment password  ")
	password, legacy, err := resolveSyncPassword("", "")
	if err != nil {
		t.Fatal(err)
	}
	if password != "  environment password  " || legacy {
		t.Fatalf("password = %q, legacy=%v", password, legacy)
	}

	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("file password \n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	password, legacy, err = resolveSyncPassword(path, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if password != "file password \n" || legacy {
		t.Fatalf("file password = %q, legacy=%v", password, legacy)
	}
}

func TestResolveSyncPasswordPreservesLegacyTrimCompatibility(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "")
	t.Setenv(matchPasswordEnvVar, "  legacy environment password  ")
	password, legacy, err := resolveSyncPassword("", "")
	if err != nil {
		t.Fatal(err)
	}
	if password != "legacy environment password" || !legacy {
		t.Fatalf("legacy environment password = %q, legacy=%v", password, legacy)
	}

	password, legacy, err = resolveSyncPassword("", "  legacy flag password  ")
	if err != nil {
		t.Fatal(err)
	}
	if password != "legacy flag password" || !legacy {
		t.Fatalf("legacy flag password = %q, legacy=%v", password, legacy)
	}
}

func TestClassifySigningFileKeepsIdentitySeparateFromOtherSensitiveFiles(t *testing.T) {
	sensitive, identity, err := classifySigningFile("future/secret.bin", nil, signingpkg.EncryptedFileMetadata{Kind: "future-secret", Sensitive: true}, "password")
	if err != nil || !sensitive || identity {
		t.Fatalf("future secret classified as sensitive=%v identity=%v", sensitive, identity)
	}
	_, _, err = classifySigningFile("certs/distribution/cert.cer", nil, signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", Sensitive: true}, "password")
	if err == nil {
		t.Fatal("PKCS#12 identity metadata outside identity path was accepted")
	}
}

func TestPrepareDecryptedSigningFilesRejectsLegacyCiphertextAtIdentityPathWithoutWriting(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	identityPath := filepath.Join("identities", "distribution", strings.Repeat("A", 64)+".p12")
	if err := store.WriteEncryptedFile(identityPath, []byte("not-an-identity"), "password"); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	destination := filepath.Join(outDir, identityPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(store, []string{identityPath}, "password", outDir); err == nil || !strings.Contains(err.Error(), "versioned sensitive") {
		t.Fatalf("legacy identity substitution error = %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Fatalf("existing identity changed to %q", got)
	}
}

func TestPrepareDecryptedSigningFilesRejectsLegacyCiphertextAtArbitraryP12Path(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	identityPath := filepath.Join("nested", "Renamed.P12")
	if err := store.WriteEncryptedFile(identityPath, []byte("not-an-identity"), "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(store, []string{identityPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "versioned sensitive") {
		t.Fatalf("arbitrary P12 substitution error = %v", err)
	}
}

func TestPrepareDecryptedSigningFilesRejectsIdentityMetadataMismatchWithoutWriting(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 20)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	normalized, err := normalizeSigningIdentity(identity, "password")
	if err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join("identities", "distribution", identity.CertificateSHA256+".p12")
	if err := store.WriteEncryptedFileWithMetadata(identityPath, normalized, "password", signingpkg.EncryptedFileMetadata{
		Kind: "pkcs12-identity", Sensitive: true, CertificateSHA256: identity.CertificateSHA256, TeamID: "WRONGTEAM",
	}); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	if _, err := prepareDecryptedSigningFiles(store, []string{identityPath}, "password", outDir); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("identity metadata mismatch error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, identityPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity output exists after mismatch: %v", err)
	}
}

func TestPrepareDecryptedSigningFilesRequiresIdentityCoreContextIntegrity(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 24)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "profile-context")

	t.Run("core without context", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFileWithMetadata(artifacts.IdentityPath, artifacts.IdentityData, "password", artifacts.IdentityMetadata); err != nil {
			t.Fatal(err)
		}
		decrypted, err := prepareDecryptedSigningFiles(store, []string{artifacts.IdentityPath}, "password", t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(decrypted) != 0 {
			t.Fatalf("unreferenced core remained active: %#v", decrypted)
		}
	})

	t.Run("orphan context", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFileWithMetadata(artifacts.BindingPath, artifacts.BindingData, "password", artifacts.BindingMetadata); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareDecryptedSigningFiles(store, []string{artifacts.BindingPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "no matching core") {
			t.Fatalf("orphan context error = %v", err)
		}
	})

	t.Run("matched graph", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
			t.Fatal(err)
		}
		if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, "password"); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareDecryptedSigningFiles(store, []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}, "password", t.TempDir()); err != nil {
			t.Fatalf("matched identity graph rejected: %v", err)
		}
	})
}

func TestIdentityContextCurrentPointerReplacesOldProfileBinding(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 27)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}

	oldArtifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	oldProfilePath, oldProfileContent := bindTestSigningIdentityArtifacts(t, oldArtifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "old-profile")
	if err := store.WriteEncryptedFile(oldProfilePath, oldProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, oldArtifacts, "password"); err != nil {
		t.Fatal(err)
	}

	newArtifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	newProfilePath, newProfileContent := bindTestSigningIdentityArtifacts(t, newArtifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "new-profile")
	if oldArtifacts.BindingPath != newArtifacts.BindingPath {
		t.Fatalf("current context paths differ: old=%q new=%q", oldArtifacts.BindingPath, newArtifacts.BindingPath)
	}
	if err := preflightSigningIdentityArtifactsForContextUpdate(store, newArtifacts, "password"); err != nil {
		t.Fatalf("pre-create renewal preflight: %v", err)
	}
	if err := store.WriteEncryptedFile(newProfilePath, newProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := preflightSigningIdentityArtifactsForContextUpdate(store, newArtifacts, "password"); err != nil {
		t.Fatalf("post-create renewal preflight: %v", err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, newArtifacts, "password"); err != nil {
		t.Fatalf("replace current context: %v", err)
	}
	files := []string{oldProfilePath, newProfilePath, newArtifacts.IdentityPath, newArtifacts.BindingPath}
	if _, err := prepareDecryptedSigningFiles(store, files, "password", t.TempDir()); err != nil {
		t.Fatalf("current context graph with retained old profile rejected: %v", err)
	}
}

func TestIdentityContextCurrentPointerSupportsCertificateRotationWithRetainedOldCore(t *testing.T) {
	oldKey := mustECKey(t)
	oldCertificate := mustSigningCertificate(t, oldKey, 29)
	oldIdentity := &signingIdentity{PrivateKey: oldKey, Certificate: oldCertificate, CertificateSHA256: certificateSHA256(oldCertificate)}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	oldArtifacts, err := prepareSigningIdentityArtifacts(oldIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	oldProfilePath, oldProfileContent := bindTestSigningIdentityArtifacts(t, oldArtifacts, oldCertificate, oldKey, "com.example.app", "IOS_APP_ADHOC", "old-cert-profile")
	if err := store.WriteEncryptedFile(oldProfilePath, oldProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, oldArtifacts, "password"); err != nil {
		t.Fatal(err)
	}

	newKey := mustECKey(t)
	newCertificate := mustSigningCertificate(t, newKey, 30)
	newIdentity := &signingIdentity{PrivateKey: newKey, Certificate: newCertificate, CertificateSHA256: certificateSHA256(newCertificate)}
	newArtifacts, err := prepareSigningIdentityArtifacts(newIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	if oldArtifacts.BindingPath != newArtifacts.BindingPath {
		t.Fatalf("rotation context paths differ: old=%q new=%q", oldArtifacts.BindingPath, newArtifacts.BindingPath)
	}
	if err := preflightSigningIdentityArtifactsForContextUpdate(store, newArtifacts, "password"); err != nil {
		t.Fatalf("rotation preflight: %v", err)
	}
	newProfilePath, newProfileContent := bindTestSigningIdentityArtifacts(t, newArtifacts, newCertificate, newKey, "com.example.app", "IOS_APP_ADHOC", "new-cert-profile")
	if err := store.WriteEncryptedFile(newProfilePath, newProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, newArtifacts, "password"); err != nil {
		t.Fatalf("rotation write: %v", err)
	}
	files := []string{oldProfilePath, newProfilePath, oldArtifacts.IdentityPath, newArtifacts.IdentityPath, newArtifacts.BindingPath}
	decrypted, err := prepareDecryptedSigningFiles(store, files, "password", t.TempDir())
	if err != nil {
		t.Fatalf("rotation graph rejected: %v", err)
	}
	for _, file := range decrypted {
		if file.RelativePath == oldArtifacts.IdentityPath {
			t.Fatal("retained old identity core was selected for output")
		}
	}
}

func TestPrepareDecryptedSigningFilesIgnoresRetainedExpiredUnreferencedCore(t *testing.T) {
	expiredKey := mustECKey(t)
	expiredCertificate := mustSigningCertificateWithValidity(t, expiredKey, 31, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	expiredIdentity := &signingIdentity{PrivateKey: expiredKey, Certificate: expiredCertificate, CertificateSHA256: certificateSHA256(expiredCertificate)}
	expiredArtifacts, err := prepareSigningIdentityArtifacts(expiredIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if err := store.WriteEncryptedFileWithMetadata(expiredArtifacts.IdentityPath, expiredArtifacts.IdentityData, "password", expiredArtifacts.IdentityMetadata); err != nil {
		t.Fatal(err)
	}

	activeKey := mustECKey(t)
	activeCertificate := mustSigningCertificate(t, activeKey, 32)
	activeIdentity := &signingIdentity{PrivateKey: activeKey, Certificate: activeCertificate, CertificateSHA256: certificateSHA256(activeCertificate)}
	activeArtifacts, err := prepareSigningIdentityArtifacts(activeIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, activeArtifacts, activeCertificate, activeKey, "com.example.app", "IOS_APP_ADHOC", "active-profile")
	if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, activeArtifacts, "password"); err != nil {
		t.Fatal(err)
	}
	files := []string{profilePath, expiredArtifacts.IdentityPath, activeArtifacts.IdentityPath, activeArtifacts.BindingPath}
	decrypted, err := prepareDecryptedSigningFiles(store, files, "password", t.TempDir())
	if err != nil {
		t.Fatalf("retained expired unreferenced core rejected: %v", err)
	}
	for _, file := range decrypted {
		if file.RelativePath == expiredArtifacts.IdentityPath {
			t.Fatal("retained expired identity core was selected for output")
		}
	}
}

func TestPrepareDecryptedSigningFilesRejectsInvalidActiveIdentityCoreValidity(t *testing.T) {
	tests := []struct {
		name      string
		notBefore time.Time
		notAfter  time.Time
	}{
		{name: "expired", notBefore: time.Now().Add(-48 * time.Hour), notAfter: time.Now().Add(-24 * time.Hour)},
		{name: "not yet valid", notBefore: time.Now().Add(24 * time.Hour), notAfter: time.Now().Add(48 * time.Hour)},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := mustECKey(t)
			certificate := mustSigningCertificateWithValidity(t, key, int64(33+index), test.notBefore, test.notAfter)
			identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
			artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
			if err != nil {
				t.Fatal(err)
			}
			signerKey := mustECKey(t)
			signerCertificate := mustSigningCertificate(t, signerKey, int64(35+index))
			profilePath, profileContent := bindTestSigningIdentityArtifactsWithSigner(t, artifacts, certificate, signerCertificate, signerKey, "com.example.app", "IOS_APP_ADHOC", "invalid-core-profile")
			store := &signingpkg.GitStore{LocalDir: t.TempDir()}
			if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
				t.Fatal(err)
			}
			if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, "password"); err != nil {
				t.Fatal(err)
			}
			if _, err := prepareDecryptedSigningFiles(store, []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "not currently valid") {
				t.Fatalf("invalid active identity error = %v", err)
			}
		})
	}
}

func TestIdentityContextRejectsProfileDistributionTypeMismatch(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 28)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_STORE")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, "com.example.app", "IOS_APP_STORE", "store-profile")
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(store, []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "distribution type") {
		t.Fatalf("profile type mismatch error = %v", err)
	}
}

func TestValidateIdentityArtifactGraphRejectsConflictingContextScope(t *testing.T) {
	binding := func(certificate string) []byte {
		data, err := json.Marshal(identityContextBinding{CertificateSHA256: certificate, TeamID: "TEAM123", BundleID: "com.example.app", ProfileType: "IOS_APP_ADHOC"})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	files := []decryptedSigningFile{
		{RelativePath: "identities/distribution/AAA.p12", Metadata: signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", TeamID: "TEAM123"}},
		{RelativePath: "identities/distribution/BBB.p12", Metadata: signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", TeamID: "TEAM123"}},
		{Plaintext: binding("AAA"), Metadata: signingpkg.EncryptedFileMetadata{Kind: "identity-context"}},
		{Plaintext: binding("BBB"), Metadata: signingpkg.EncryptedFileMetadata{Kind: "identity-context"}},
	}
	if _, err := validateIdentityArtifactGraph(files); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting graph error = %v", err)
	}
}

func TestValidateIdentityArtifactGraphRejectsCrossTeamContext(t *testing.T) {
	binding, err := json.Marshal(identityContextBinding{CertificateSHA256: "AAA", TeamID: "TEAM-B", BundleID: "com.example.app", ProfileType: "IOS_APP_ADHOC"})
	if err != nil {
		t.Fatal(err)
	}
	files := []decryptedSigningFile{
		{RelativePath: "identities/distribution/AAA.p12", Metadata: signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", TeamID: "TEAM-A"}},
		{Plaintext: binding, Metadata: signingpkg.EncryptedFileMetadata{Kind: "identity-context"}},
	}
	if _, err := validateIdentityArtifactGraph(files); err == nil || !strings.Contains(err.Error(), "team") {
		t.Fatalf("cross-team graph error = %v", err)
	}
}

func TestPrepareDecryptedSigningFilesBoundsRepositoryBeforeDecrypt(t *testing.T) {
	paths := make([]string, maxEncryptedSigningFiles+1)
	for i := range paths {
		paths[i] = fmt.Sprintf("certs/distribution/%03d.cer", i)
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if _, err := prepareDecryptedSigningFiles(store, paths, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "files; limit") {
		t.Fatalf("file count error = %v", err)
	}

	paths = nil
	for i := 0; i < 4; i++ {
		relPath := fmt.Sprintf("certs/distribution/large-%d.cer", i)
		path := filepath.Join(store.LocalDir, relPath+".enc")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(33 << 20); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, relPath)
	}
	if _, err := prepareDecryptedSigningFiles(store, paths, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "cumulative size limit") {
		t.Fatalf("cumulative size error = %v", err)
	}
}

func TestSigningSyncPushRejectsIdentityFlagConflictsBeforeSecretsOrClient(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "identity and private key",
			args: []string{"--identity", "identity.p12", "--private-key", "key.pem"},
			want: "--identity and --private-key are mutually exclusive",
		},
		{
			name: "fingerprint without identity",
			args: []string{"--identity-sha256", strings.Repeat("A", 64)},
			want: "--identity-sha256 requires --identity or --private-key",
		},
		{
			name: "identity password without identity",
			args: []string{"--identity-password-file", "password"},
			want: "--identity-password-file requires --identity",
		},
		{
			name: "private key without fingerprint",
			args: []string{"--private-key", "key.pem"},
			want: "--identity-sha256 is required with --private-key to select one App Store Connect certificate",
		},
		{
			name: "password file and legacy password",
			args: []string{"--password-file", "password", "--password", "legacy-password"},
			want: "--password-file and --password are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := syncPushCommand()
			base := []string{
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
			}
			if err := cmd.Parse(append(base, tt.args...)); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestSigningSyncRejectsBlankPasswordFile(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ffcli.Command
		args []string
	}{
		{
			name: "push",
			cmd:  syncPushCommand(),
			args: []string{
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
				"--password-file", " \t ",
			},
		},
		{
			name: "pull",
			cmd:  syncPullCommand(),
			args: []string{
				"--repo", "git@example.com:team/signing.git",
				"--password-file", " \t ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cmd.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			err := tt.cmd.Run(context.Background())
			if err == nil || err.Error() != "--password-file must not be empty" {
				t.Fatalf("error = %v, want blank password-file usage error", err)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestSigningSyncPushRejectsDirectDistributionIdentityBeforeSecretReads(t *testing.T) {
	for _, profileType := range []string{"MAC_APP_DIRECT", "MAC_CATALYST_APP_DIRECT"} {
		t.Run(profileType, func(t *testing.T) {
			cmd := syncPushCommand()
			if err := cmd.Parse([]string{
				"--bundle-id", "com.example.app",
				"--profile-type", profileType,
				"--repo", "git@example.com:team/signing.git",
				"--identity", filepath.Join(t.TempDir(), "missing.p12"),
			}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			want := "private identity sync does not support --profile-type " + profileType + " yet; omit --identity/--private-key"
			if err == nil || err.Error() != want || !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error %q", err, want)
			}
		})
	}
}

func TestSigningSyncPushIdentityLoadFailureIsOperational(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	cmd := syncPushCommand()
	if err := cmd.Parse([]string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--repo", "git@example.com:team/signing.git",
		"--identity", filepath.Join(t.TempDir(), "missing.p12"),
	}); err != nil {
		t.Fatal(err)
	}

	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "signing sync push: signing identity") {
		t.Fatalf("error = %v, want signing identity load failure", err)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want operational error", err)
	}
}

func TestWriteDecryptedSensitiveOutputRefusesExistingFile(t *testing.T) {
	outDir := t.TempDir()
	relPath := filepath.Join("identities", "distribution", "ABC.p12")
	destination := filepath.Join(outDir, relPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeDecryptedOutputFile(outDir, relPath, []byte("identity"), true); err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("error = %v, want existing-file refusal", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("existing output mode = %o, want unchanged 644", info.Mode().Perm())
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("existing output = %q, want unchanged", data)
	}
}

func TestPrepareDecryptedSigningFilesPreflightsAllDestinationsBeforeWriting(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	password := "repository-password"
	certificatePath := filepath.Join("certs", "distribution", "certificate.cer")
	identityPath := filepath.Join("future-secrets", "credential.bin")
	if err := store.WriteEncryptedFile(certificatePath, []byte("new-certificate"), password); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteEncryptedFileWithMetadata(identityPath, []byte("identity"), password, signingpkg.EncryptedFileMetadata{
		Kind: "future-secret", Sensitive: true,
	}); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	certificateDestination := filepath.Join(outDir, certificatePath)
	identityDestination := filepath.Join(outDir, identityPath)
	if err := os.MkdirAll(filepath.Dir(certificateDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certificateDestination, []byte("existing-certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(identityDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityDestination, []byte("existing-identity"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := prepareDecryptedSigningFiles(store, []string{certificatePath, identityPath}, password, outDir)
	if err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("prepareDecryptedSigningFiles() error = %v, want identity collision", err)
	}
	certificate, readErr := os.ReadFile(certificateDestination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(certificate) != "existing-certificate" {
		t.Fatalf("earlier destination changed to %q", certificate)
	}
}

func TestPrepareDecryptedSigningFilesDecryptsAllBeforeDestinationPreflight(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	password := "repository-password"
	certificatePath := filepath.Join("certs", "distribution", "certificate.cer")
	corruptPath := filepath.Join("profiles", "adhoc", "app.mobileprovision")
	if err := store.WriteEncryptedFile(certificatePath, []byte("new-certificate"), password); err != nil {
		t.Fatal(err)
	}
	corruptDestination := filepath.Join(store.LocalDir, corruptPath+".enc")
	if err := os.MkdirAll(filepath.Dir(corruptDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corruptDestination, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	certificateDestination := filepath.Join(outDir, certificatePath)
	if err := os.MkdirAll(filepath.Dir(certificateDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certificateDestination, []byte("existing-certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareDecryptedSigningFiles(store, []string{certificatePath, corruptPath}, password, outDir); err == nil {
		t.Fatal("corrupt later artifact was accepted")
	}
	certificate, readErr := os.ReadFile(certificateDestination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(certificate) != "existing-certificate" {
		t.Fatalf("earlier destination changed to %q", certificate)
	}
}

func TestWriteDecryptedOutputFileRejectsSymlinkTarget(t *testing.T) {
	outDir := t.TempDir()
	targetDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	destPath := filepath.Join(outDir, relPath)
	targetPath := filepath.Join(targetDir, "app.mobileprovision")

	if err := os.WriteFile(targetPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		t.Fatalf("mkdir output parent: %v", err)
	}
	if err := os.Symlink(targetPath, destPath); err != nil {
		t.Fatalf("create destination symlink: %v", err)
	}

	err := writeDecryptedOutputFile(outDir, relPath, []byte("updated"), false)
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}

	got, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target file: %v", readErr)
	}
	if string(got) != "original" {
		t.Fatalf("did not expect write through symlink target, got %q", got)
	}
}
