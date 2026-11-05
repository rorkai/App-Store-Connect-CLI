package distribute

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func TestReadDistributionConfigStrictPrivateS3(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	identityPath := filepath.Join(dir, "identity.p12")
	passwordPath := filepath.Join(dir, "identity.password")
	devicesPath := filepath.Join(dir, "devices.json")
	for _, path := range []string{identityPath, passwordPath, devicesPath} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	body := `{
  "schemaVersion": 1,
  "devicesFile": "` + devicesPath + `",
  "signing": {
    "identity": {"format":"pkcs12","path":"` + identityPath + `","passwordFile":"` + passwordPath + `","certificateSha256":"` + strings.Repeat("a", 64) + `"},
    "minimumValidityDays": 7,
    "maxMutations": 10
  },
  "publication": {
    "endpoint":"https://objects.example.com","downloadEndpoint":"https://downloads.example.com",
    "region":"us-east-1","bucket":"mobile-builds","prefix":"ios/pr-42","addressingStyle":"path",
    "urlTtl":"24h","downloadGrace":"1h","verifyTimeout":"30s"
  },
  "metadata":{"title":"Preview","channel":"pr-42","sourceRevision":"abc123","sourceUrl":"https://example.com/commit/abc123"}
}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, digest, err := readDistributionConfig(configPath)
	if err != nil {
		t.Fatalf("readDistributionConfig() error = %v", err)
	}
	if got.SchemaVersion != 1 || got.Signing.Identity.Format != "pkcs12" || got.Publication.AddressingStyle != "path" {
		t.Fatalf("config = %#v", got)
	}
	if got.Publication.URLTTLDuration != 24*time.Hour || got.Publication.DownloadGraceDuration != time.Hour || got.Publication.VerifyTimeoutDuration != 30*time.Second {
		t.Fatalf("parsed durations = %s %s %s", got.Publication.URLTTLDuration, got.Publication.DownloadGraceDuration, got.Publication.VerifyTimeoutDuration)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(digest) {
		t.Fatalf("digest = %q", digest)
	}

	for name, replacement := range map[string]string{
		"unknown":           strings.Replace(body, `"schemaVersion": 1`, `"schemaVersion": 1, "access":"public"`, 1),
		"duplicate":         strings.Replace(body, `"schemaVersion": 1`, `"schemaVersion": 1, "schemaVersion":1`, 1),
		"trailing":          body + `{}`,
		"inline secret":     strings.Replace(body, `"passwordFile":"`+passwordPath+`"`, `"passwordFile":"`+passwordPath+`","password":"secret"`, 1),
		"public config":     strings.Replace(body, `"prefix":"ios/pr-42"`, `"prefix":"ios/pr-42","publicBaseUrl":"https://public.example.com"`, 1),
		"origin source URL": strings.Replace(body, "https://example.com/commit/abc123", "https://example.com", 1),
		"bidi source URL":   strings.Replace(body, "https://example.com/commit/abc123", "https://example.com/commit/abc\u202e123", 1),
		"format source URL": strings.Replace(body, "https://example.com/commit/abc123", "https://example.com/commit/abc\u200b123", 1),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(replacement), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readDistributionConfig(path); err == nil {
				t.Fatalf("readDistributionConfig(%s) unexpectedly succeeded", name)
			}
		})
	}
}

func TestReadProtectedDistributionFileRejectsInPlaceMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected.json")
	if err := os.WriteFile(path, []byte(`{"state":"before"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	distributionAfterProtectedReadForTest = func() {
		if err := os.WriteFile(path, []byte(`{"state":"replacement-with-different-size"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { distributionAfterProtectedReadForTest = nil })

	if _, err := readProtectedDistributionFile(path, distributionStateMaxBytes); err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("readProtectedDistributionFile() error = %v, want in-place mutation rejection", err)
	}
}

func TestReadDistributionConfigAllowsEmptyPKCS12Password(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	body := `{"schemaVersion":1,"devicesFile":"devices.json","signing":{"identity":{"format":"pkcs12","path":"identity.p12"},"minimumValidityDays":0,"maxMutations":1},"publication":{"endpoint":"https://objects.example.com","region":"auto","bucket":"builds","prefix":"ios","addressingStyle":"path","urlTtl":"1h","downloadGrace":"0s","verifyTimeout":"30s"}}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	config, _, err := readDistributionConfig(configPath)
	if err != nil {
		t.Fatalf("readDistributionConfig() error = %v", err)
	}
	if config.Signing.Identity.PasswordFile != "" {
		t.Fatalf("passwordFile = %q", config.Signing.Identity.PasswordFile)
	}
}

func TestReadDistributionConfigRejectsUnprotectedFiles(t *testing.T) {
	dir := t.TempDir()
	valid := []byte(`{"schemaVersion":1,"devicesFile":"devices.json","signing":{"identity":{"format":"pkcs12","path":"identity.p12","passwordFile":"password"},"minimumValidityDays":7,"maxMutations":5},"publication":{"endpoint":"https://objects.example.com","region":"us-east-1","bucket":"builds","prefix":"ios","addressingStyle":"path","urlTtl":"1h","downloadGrace":"5m","verifyTimeout":"30s"}}`)

	worldReadable := filepath.Join(dir, "world-readable.json")
	if err := os.WriteFile(worldReadable, valid, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDistributionConfig(worldReadable); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("world-readable error = %v", err)
	}

	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(dir, "hardlink.json")
	if err := os.Link(target, hardlink); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if _, _, err := readDistributionConfig(hardlink); err == nil || !strings.Contains(err.Error(), "hard link") {
		t.Fatalf("hard-link error = %v", err)
	}

	symlink := filepath.Join(dir, "symlink.json")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDistributionConfig(symlink); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("symlink error = %v", err)
	}
}

func TestDistributionPlanHashExcludesTimestampAndHashButBindsEffects(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	first := plan.PlanHash
	plan.CreatedAt = "2099-01-01T00:00:00Z"
	plan.PlanHash = strings.Repeat("f", 64)
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan.PlanHash != first {
		t.Fatalf("timestamp/hash changed canonical hash: %s != %s", plan.PlanHash, first)
	}
	plan.Effects[0].Count++
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan.PlanHash == first {
		t.Fatal("effect change did not change plan hash")
	}
	if err := verifyDistributionPlanHash(plan); err != nil {
		t.Fatalf("verifyDistributionPlanHash() error = %v", err)
	}
}

func TestDistributionProtectedPersistenceAndIDs(t *testing.T) {
	syncCalls := 0
	previousSync := distributionSyncDirectoryForTest
	distributionSyncDirectoryForTest = func(*os.Root) error { syncCalls++; return nil }
	t.Cleanup(func() { distributionSyncDirectoryForTest = previousSync })
	for _, test := range []struct {
		name, pattern string
		generate      func() (string, error)
	}{
		{name: "plan", pattern: `^dplan_[0-9a-f]{32}$`, generate: newDistributionPlanID},
	} {
		t.Run(test.name, func(t *testing.T) {
			one, err := test.generate()
			if err != nil {
				t.Fatal(err)
			}
			two, err := test.generate()
			if err != nil {
				t.Fatal(err)
			}
			if one == two || !regexp.MustCompile(test.pattern).MatchString(one) {
				t.Fatalf("ids = %q %q", one, two)
			}
		})
	}

	root := filepath.Join(t.TempDir(), "state")
	planPath := filepath.Join(root, "plan.json")
	plan := distributionPersistedPlanFixture(root)
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := writePersistedDistributionPlan(planPath, plan); err != nil {
		t.Fatal(err)
	}
	if err := writePersistedDistributionPlan(planPath, plan); err == nil {
		t.Fatal("create-only plan was overwritten")
	}
	assertPrivateFile(t, planPath)
	loadedPlan, err := readPersistedDistributionPlan(planPath)
	if err != nil || loadedPlan.PlanHash != plan.PlanHash {
		t.Fatalf("read plan = %#v, %v", loadedPlan, err)
	}

	runID, err := newDistributionRunID()
	if err != nil {
		t.Fatal(err)
	}
	if err := createDistributionRunDirectory(root, runID); err != nil {
		t.Fatal(err)
	}
	run := persistedDistributionRunState{
		SchemaVersion: 1, RunID: runID, PlanID: plan.PlanID, PlanPath: planPath, PlanHash: plan.PlanHash, Status: "running", Stage: "identity_validate", UpdatedAt: time.Now().UTC().Format(time.RFC3339), Attempt: 1,
		Artifacts: distributionRunArtifacts{
			ArchiveSnapshot: &distributionArchiveSnapshot{RelativePath: "archive/App.xcarchive", TreeSHA256: plan.Archive.TreeSHA256, SizeBytes: plan.Archive.SizeBytes, EntryCount: plan.Archive.FileCount, App: distributionStateArchiveAppFixture()},
		},
	}
	if err := writeDistributionRunState(root, run); err != nil {
		t.Fatal(err)
	}
	run.Status, run.Stage, run.Attempt = "running", "fetch_verify", 2
	run.Recoverable = false
	run.Artifacts.ReconcileReceipt = &distributionFileArtifact{Path: "reconcile/receipt.json", SHA256: strings.Repeat("9", 64)}
	run.Artifacts.Profile = &distributionProfileArtifact{ResourceID: "profile-resource", UUID: "11111111-1111-1111-1111-111111111111", Path: "reconcile/profile.mobileprovision", SHA256: strings.Repeat("7", 64), BundleID: "com.example.app"}
	run.Artifacts.ExportOptions = &distributionFileArtifact{Path: "export/options.plist", SHA256: strings.Repeat("a", 64)}
	run.Artifacts.IPA = &distributionSizedFileArtifact{Path: "export/app.ipa", SHA256: strings.Repeat("b", 64), SizeBytes: 1234}
	run.Artifacts.SigningReceipt = &distributionFileArtifact{Path: "signing/receipt.json", SHA256: strings.Repeat("8", 64)}
	descriptorData, descriptorSHA256 := writeDistributionDescriptorFixture(t, root, runID, plan)
	publicationSHA256 := writeDistributionPublicationReceiptFixture(t, root, runID, plan)
	linkSHA256 := writeDistributionLinkFixture(t, root, runID, []byte("sensitive link fixture"))
	run.Artifacts.Bundle = &distributionBundleArtifact{Path: "prepared/bundle", DescriptorSHA256: descriptorSHA256}
	run.Artifacts.Publication = &distributionPublicationArtifact{ReceiptPath: "publish/receipt.json", ReceiptSHA256: publicationSHA256, LinkPath: "secrets/link.json", LinkSHA256: linkSHA256, ArtifactKey: "ios/artifact.ipa", ManifestKey: "ios/manifest.plist", PageKey: "ios/index.html", InstallURLRedacted: "https://objects.example.com/page?REDACTED"}
	if err := writeDistributionRunState(root, run); err != nil {
		t.Fatal(err)
	}
	assertPrivateDirectory(t, root)
	assertPrivateDirectory(t, filepath.Join(root, runID))
	assertPrivateFile(t, filepath.Join(root, runID, "state.json"))
	prematureComplete := run
	prematureComplete.Status, prematureComplete.Stage = "complete", "complete"
	if err := writeDistributionRunState(root, prematureComplete); err == nil || !strings.Contains(err.Error(), "immutable receipt") {
		t.Fatalf("premature complete error = %v", err)
	}

	receipt := persistedDistributionReceipt{
		SchemaVersion: 1, RunID: runID, PlanID: plan.PlanID, PlanHash: plan.PlanHash, Status: "published_and_fetch_verified", CompletedAt: "2098-01-01T00:00:01Z",
		PublicationReceiptPath: "publish/receipt.json", PublicationReceiptSHA256: publicationSHA256, LinkPath: "secrets/link.json", LinkSHA256: linkSHA256,
		ArtifactSHA256: strings.Repeat("b", 64), ArtifactSizeBytes: 1234, BundleDescriptorSHA256: descriptorSHA256, BundleDescriptorSizeBytes: int64(len(descriptorData)),
		ManifestSHA256: strings.Repeat("1", 64), ManifestSizeBytes: 200, PageSHA256: strings.Repeat("2", 64), PageSizeBytes: 300,
		ArtifactKey: "ios/artifact.ipa", ManifestKey: "ios/manifest.plist", PageKey: "ios/index.html", InstallURLRedacted: "https://objects.example.com/page?REDACTED",
		AppBundleID: "com.example.app", AppVersion: "1.0", AppBuildNumber: "1", TeamID: "TEAM123", ProfileResourceID: "profile-resource", ProfileClass: "ad-hoc", ProfileUUID: "11111111-1111-1111-1111-111111111111", ProfileExpiresAt: "2099-01-01T00:00:00Z", ProfileSHA256: strings.Repeat("7", 64), DeviceSetSHA256: strings.Repeat("3", 64), DeviceCount: 1, CertificateSHA256: strings.Repeat("4", 64), ProfileCertificateSHA256Fingerprints: []string{strings.Repeat("4", 64)}, SignerCertificateSHA256Fingerprints: []string{strings.Repeat("4", 64)}, ProfileIntegrityStatus: "verified", ProfileTrustStatus: "verified", CodeSignatureStatus: "verified", CodeSignatureScope: "complete-main-app-code-resources-entitlements-and-profile-certificate-binding", FetchVerified: true, FetchVerifiedAt: "2098-01-01T00:00:00Z",
	}
	mismatchedReceipt := receipt
	mismatchedReceipt.PlanHash = strings.Repeat("f", 64)
	if err := writeDistributionReceipt(root, mismatchedReceipt); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched receipt error = %v", err)
	}
	for name, mutate := range map[string]func(*persistedDistributionReceipt){
		"device set": func(value *persistedDistributionReceipt) { value.DeviceSetSHA256 = strings.Repeat("e", 64) },
		"team":       func(value *persistedDistributionReceipt) { value.TeamID = "OTHERTEAM" },
		"certificate": func(value *persistedDistributionReceipt) {
			value.CertificateSHA256 = strings.Repeat("e", 64)
			value.ProfileCertificateSHA256Fingerprints = []string{strings.Repeat("e", 64)}
			value.SignerCertificateSHA256Fingerprints = []string{strings.Repeat("e", 64)}
		},
	} {
		t.Run("receipt plan mismatch "+name, func(t *testing.T) {
			candidate := receipt
			candidate.ProfileCertificateSHA256Fingerprints = append([]string(nil), receipt.ProfileCertificateSHA256Fingerprints...)
			candidate.SignerCertificateSHA256Fingerprints = append([]string(nil), receipt.SignerCertificateSHA256Fingerprints...)
			mutate(&candidate)
			if err := writeDistributionReceipt(root, candidate); err == nil || !strings.Contains(err.Error(), "differs from plan") {
				t.Fatalf("%s mismatch error = %v", name, err)
			}
		})
	}
	t.Run("publication policy mismatch", func(t *testing.T) {
		root, run, receipt := writeDistributionFetchVerifyFixture(t)
		tamperDistributionPublicationReceiptFixture(t, root, run.RunID, func(value *core.PublishReceipt) { value.Prefix = "other-prefix" })
		rewriteDistributionPublicationDigest(t, root, &run, &receipt)
		if err := writeDistributionRunState(root, run); err != nil {
			t.Fatal(err)
		}
		if err := writeDistributionReceipt(root, receipt); err == nil || !strings.Contains(err.Error(), "destination or lifetime") {
			t.Fatalf("publication policy mismatch error = %v", err)
		}
	})
	t.Run("publication app title mismatch", func(t *testing.T) {
		root, run, receipt := writeDistributionFetchVerifyFixture(t)
		tamperDistributionPublicationReceiptFixture(t, root, run.RunID, func(value *core.PublishReceipt) { value.App.Title = "Another App" })
		rewriteDistributionPublicationDigest(t, root, &run, &receipt)
		if err := writeDistributionRunState(root, run); err != nil {
			t.Fatal(err)
		}
		if err := writeDistributionReceipt(root, receipt); err == nil || !strings.Contains(err.Error(), "app verification") {
			t.Fatalf("publication app mismatch error = %v", err)
		}
	})
	t.Run("sensitive link digest mismatch", func(t *testing.T) {
		root, run, receipt := writeDistributionFetchVerifyFixture(t)
		linkPath := filepath.Join(root, run.RunID, filepath.FromSlash(run.Artifacts.Publication.LinkPath))
		if err := os.WriteFile(linkPath, []byte("tampered sensitive link"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := writeDistributionRunState(root, run); err != nil {
			t.Fatal(err)
		}
		if err := writeDistributionReceipt(root, receipt); err == nil || !strings.Contains(err.Error(), "digest differs") {
			t.Fatalf("sensitive link mismatch error = %v", err)
		}
	})
	for name, mutate := range map[string]func(*persistedDistributionReceipt){
		"app version":        func(value *persistedDistributionReceipt) { value.AppVersion = "9.9" },
		"profile expiration": func(value *persistedDistributionReceipt) { value.ProfileExpiresAt = "2099-02-01T00:00:00Z" },
		"profile fingerprints": func(value *persistedDistributionReceipt) {
			value.ProfileCertificateSHA256Fingerprints = []string{strings.Repeat("3", 64), strings.Repeat("4", 64)}
		},
		"signer fingerprints": func(value *persistedDistributionReceipt) {
			value.SignerCertificateSHA256Fingerprints = []string{strings.Repeat("3", 64), strings.Repeat("4", 64)}
		},
		"descriptor size": func(value *persistedDistributionReceipt) { value.BundleDescriptorSizeBytes++ },
		"manifest digest": func(value *persistedDistributionReceipt) { value.ManifestSHA256 = strings.Repeat("e", 64) },
		"page size":       func(value *persistedDistributionReceipt) { value.PageSizeBytes++ },
	} {
		t.Run("receipt artifact mismatch "+name, func(t *testing.T) {
			candidate := receipt
			candidate.ProfileCertificateSHA256Fingerprints = append([]string(nil), receipt.ProfileCertificateSHA256Fingerprints...)
			candidate.SignerCertificateSHA256Fingerprints = append([]string(nil), receipt.SignerCertificateSHA256Fingerprints...)
			mutate(&candidate)
			if err := writeDistributionReceipt(root, candidate); err == nil || !strings.Contains(err.Error(), "evidence differs") {
				t.Fatalf("%s mismatch error = %v", name, err)
			}
		})
	}
	if err := writeDistributionReceipt(root, receipt); err != nil {
		t.Fatal(err)
	}
	if err := writeDistributionReceipt(root, receipt); err == nil {
		t.Fatal("create-only receipt was overwritten")
	}
	loadedReceipt, err := readDistributionReceipt(root, runID)
	if err != nil || loadedReceipt.InstallURLRedacted != receipt.InstallURLRedacted {
		t.Fatalf("read receipt = %#v, %v", loadedReceipt, err)
	}
	assertPrivateFile(t, filepath.Join(root, runID, "receipt.json"))
	run.Status, run.Stage = "complete", "complete"
	if err := writeDistributionRunState(root, run); err != nil {
		t.Fatal(err)
	}
	loadedRun, err := readDistributionRunState(root, runID)
	if err != nil || loadedRun.Status != "complete" || loadedRun.Attempt != 2 {
		t.Fatalf("read run = %#v, %v", loadedRun, err)
	}
	if syncCalls < 5 {
		t.Fatalf("parent directory sync calls = %d, want plan + run creation + two states + receipt", syncCalls)
	}

	encoded, err := json.Marshal(struct {
		Plan    persistedDistributionPlan     `json:"plan"`
		Run     persistedDistributionRunState `json:"run"`
		Receipt persistedDistributionReceipt  `json:"receipt"`
	}{plan, run, receipt})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET-UDID", "identity-password", "X-Amz-Signature"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("persisted models leaked %q: %s", secret, encoded)
		}
	}
}

func TestDistributionPlanRequiresExactDevicesFileDigest(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	plan.DeviceSet.FileSHA256 = ""
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err == nil || !strings.Contains(err.Error(), "fileSha256") {
		t.Fatalf("devices file digest error = %v", err)
	}
}

func TestDistributionPlanAllowsAccountReconcileProfileWriteEffect(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	plan.Effects = append(plan.Effects, distributionEffect{Stage: "account_reconcile", Kind: "write_profile", BundleID: plan.Archive.BundleID, Count: 1})
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err != nil {
		t.Fatalf("account_reconcile/write_profile effect rejected: %v", err)
	}

	plan.Effects[len(plan.Effects)-1].Stage = "export"
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err == nil {
		t.Fatal("write_profile effect outside account_reconcile accepted")
	}
}

func TestDistributionPlanUsesConvergentPublicationEffectKinds(t *testing.T) {
	for _, kind := range []string{"ensure_ipa", "ensure_manifest", "ensure_install_page"} {
		t.Run(kind, func(t *testing.T) {
			plan := distributionPersistedPlanFixture(t.TempDir())
			plan.Effects = append(plan.Effects, distributionEffect{Stage: "publish", Kind: kind, Count: 1})
			if err := sealDistributionPlan(&plan); err != nil {
				t.Fatal(err)
			}
			if err := validatePersistedDistributionPlan(plan); err != nil {
				t.Fatalf("publish/%s effect rejected: %v", kind, err)
			}
		})
	}

	plan := distributionPersistedPlanFixture(t.TempDir())
	plan.Effects = append(plan.Effects, distributionEffect{Stage: "publish", Kind: "put_artifact", Count: 1})
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err == nil {
		t.Fatal("legacy put_artifact effect accepted")
	}
}

func TestDistributionReadyPlanRequiresSingleMatchingValidIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*persistedDistributionPlan)
	}{
		{"multiple targets", func(plan *persistedDistributionPlan) { plan.Archive.TargetCount = 2 }},
		{"team mismatch", func(plan *persistedDistributionPlan) { plan.Archive.TeamID = "OTHERTEAM" }},
		{"missing app title", func(plan *persistedDistributionPlan) { plan.Archive.Title = "" }},
		{"missing published title", func(plan *persistedDistributionPlan) { plan.Archive.PublishedTitle = "" }},
		{"missing app version", func(plan *persistedDistributionPlan) { plan.Archive.Version = "" }},
		{"missing app build", func(plan *persistedDistributionPlan) { plan.Archive.BuildNumber = "" }},
		{"expired for operation", func(plan *persistedDistributionPlan) { plan.Identity.MinimumValidUntil = plan.Identity.ExpirationDate }},
		{"past validity threshold", func(plan *persistedDistributionPlan) {
			plan.CreatedAt = "2026-08-13T08:00:00Z"
			plan.Identity.MinimumValidUntil = "1970-01-01T00:00:00Z"
			plan.Identity.ExpirationDate = "2020-01-01T00:00:00Z"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := distributionPersistedPlanFixture(t.TempDir())
			test.mutate(&plan)
			if err := sealDistributionPlan(&plan); err != nil {
				t.Fatal(err)
			}
			if err := validatePersistedDistributionPlan(plan); err == nil {
				t.Fatalf("invalid ready plan accepted: %#v", plan)
			}
		})
	}
}

func TestDistributionPlanAllowsBoundedLongArchiveTitles(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	plan.Archive.Title = strings.Repeat("A", 512)
	plan.Archive.PublishedTitle = strings.Repeat("B", 512)
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err != nil {
		t.Fatalf("512-byte archive titles rejected: %v", err)
	}
	plan.Archive.PublishedTitle += "C"
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err == nil {
		t.Fatal("513-byte published title was accepted")
	}
}

func TestDistributionBlockedPlanPreservesMultiTargetEvidence(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	plan.Ready = false
	plan.Archive.TargetCount = 2
	plan.Blockers = []distributionBlocker{{Code: "embedded_targets_unsupported", Stage: "preflight", Message: "archive contains two signed targets"}}
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err != nil {
		t.Fatalf("truthful blocked plan rejected: %v", err)
	}
}

func TestDistributionReceiptRequiresCleanRunningFetchVerifyState(t *testing.T) {
	for _, status := range []string{"recoverable", "blocked"} {
		t.Run(status, func(t *testing.T) {
			root, run, receipt := writeDistributionFetchVerifyFixture(t)
			run.Status = status
			run.LastFailureCode = "fetch_failed"
			run.Recoverable = status == "recoverable"
			if err := writeDistributionRunState(root, run); err != nil {
				t.Fatal(err)
			}
			if err := writeDistributionReceipt(root, receipt); err == nil || !strings.Contains(err.Error(), "clean running fetch_verify") {
				t.Fatalf("%s receipt error = %v", status, err)
			}
		})
	}
}

func TestDistributionReceiptRejectsArchiveSnapshotIdentityDrift(t *testing.T) {
	root, run, receipt := writeDistributionFetchVerifyFixture(t)
	run.Artifacts.ArchiveSnapshot.App.Title = "Tampered Archive Title"
	if err := writeDistributionRunState(root, run); err != nil {
		t.Fatal(err)
	}
	if err := writeDistributionReceipt(root, receipt); err == nil || !strings.Contains(err.Error(), "archive snapshot evidence differs") {
		t.Fatalf("archive snapshot identity drift error = %v", err)
	}
}

func TestDistributionRunStateRequiresCanonicalAbsolutePlanPath(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"plan.json", "../plan.json", "/plans/../plan.json"} {
		state := persistedDistributionRunState{SchemaVersion: 1, RunID: "drun_0123456789abcdef0123456789abcdef", PlanID: plan.PlanID, PlanPath: path, PlanHash: plan.PlanHash, Status: "planned", Stage: "preflight", UpdatedAt: "2026-08-13T08:00:00Z", Attempt: 1}
		if err := validatePersistedDistributionRunState(state); err == nil {
			t.Fatalf("noncanonical PlanPath %q accepted", path)
		}
	}
}

func TestDistributionRunStateArtifactPrerequisitesAndContainment(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	state := persistedDistributionRunState{
		SchemaVersion: 1, RunID: "drun_0123456789abcdef0123456789abcdef", PlanID: plan.PlanID,
		PlanPath: "/plans/plan.json", PlanHash: plan.PlanHash, Status: "running", Stage: "publish",
		UpdatedAt: "2026-08-13T08:00:00Z", Attempt: 1,
		Artifacts: distributionRunArtifacts{
			ArchiveSnapshot:  &distributionArchiveSnapshot{RelativePath: "inputs/App.xcarchive", TreeSHA256: strings.Repeat("1", 64), SizeBytes: 1, EntryCount: 1, App: distributionStateArchiveAppFixture()},
			ReconcileReceipt: &distributionFileArtifact{Path: "reconcile/receipt.json", SHA256: strings.Repeat("2", 64)},
			Profile:          &distributionProfileArtifact{ResourceID: "profile", UUID: "uuid", Path: "reconcile/profile.mobileprovision", SHA256: strings.Repeat("3", 64), BundleID: "com.example"},
			ExportOptions:    &distributionFileArtifact{Path: "export/options.plist", SHA256: strings.Repeat("4", 64)},
			IPA:              &distributionSizedFileArtifact{Path: "export/app.ipa", SHA256: strings.Repeat("5", 64), SizeBytes: 1},
			SigningReceipt:   &distributionFileArtifact{Path: "signing/receipt.json", SHA256: strings.Repeat("6", 64)},
			Bundle:           &distributionBundleArtifact{Path: "prepared/bundle", DescriptorSHA256: strings.Repeat("7", 64)},
		},
	}
	if err := validatePersistedDistributionRunStateForRoot("/state/runs", state); err != nil {
		t.Fatalf("valid publish state rejected: %v", err)
	}
	missing := state
	missing.Artifacts.Bundle = nil
	if err := validatePersistedDistributionRunStateForRoot("/state/runs", missing); err == nil || !strings.Contains(err.Error(), "prepared bundle") {
		t.Fatalf("missing bundle error = %v", err)
	}
	escaped := state
	escaped.Artifacts.IPA = &distributionSizedFileArtifact{Path: "../app.ipa", SHA256: strings.Repeat("5", 64), SizeBytes: 1}
	if err := validatePersistedDistributionRunStateForRoot("/state/runs", escaped); err == nil {
		t.Fatalf("escaped IPA error = %v", err)
	}
	absolute := state
	absolute.Artifacts.Profile = &distributionProfileArtifact{ResourceID: "profile", UUID: "uuid", Path: "/tmp/profile.mobileprovision", SHA256: strings.Repeat("3", 64), BundleID: "com.example"}
	if err := validatePersistedDistributionRunStateForRoot("/state/runs", absolute); err == nil {
		t.Fatalf("absolute profile error = %v", err)
	}
}

func TestDistributionReceiptRequiresCompleteFetchVerifiedEvidence(t *testing.T) {
	receipt := validDistributionStateReceiptFixture()
	if err := validatePersistedDistributionReceipt(receipt); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*persistedDistributionReceipt)
	}{
		{"status", func(value *persistedDistributionReceipt) { value.Status = "complete" }},
		{"fetch", func(value *persistedDistributionReceipt) { value.FetchVerified = false }},
		{"trust", func(value *persistedDistributionReceipt) { value.ProfileTrustStatus = "not-verified" }},
		{"scope", func(value *persistedDistributionReceipt) { value.CodeSignatureScope = "main-app-only" }},
		{"manifest digest", func(value *persistedDistributionReceipt) { value.ManifestSHA256 = "" }},
		{"page size", func(value *persistedDistributionReceipt) { value.PageSizeBytes = 0 }},
		{"absolute receipt", func(value *persistedDistributionReceipt) { value.PublicationReceiptPath = "/tmp/receipt.json" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := receipt
			candidate.ProfileCertificateSHA256Fingerprints = append([]string(nil), receipt.ProfileCertificateSHA256Fingerprints...)
			candidate.SignerCertificateSHA256Fingerprints = append([]string(nil), receipt.SignerCertificateSHA256Fingerprints...)
			test.mutate(&candidate)
			if err := validatePersistedDistributionReceipt(candidate); err == nil {
				t.Fatalf("invalid receipt accepted: %#v", candidate)
			}
		})
	}
}

func TestDistributionPersistenceReportsDirectorySyncFailure(t *testing.T) {
	previousSync := distributionSyncDirectoryForTest
	distributionSyncDirectoryForTest = func(*os.Root) error { return errors.New("fsync canary") }
	t.Cleanup(func() { distributionSyncDirectoryForTest = previousSync })
	plan := distributionPersistedPlanFixture(t.TempDir())
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	err := writePersistedDistributionPlan(filepath.Join(t.TempDir(), "private", "plan.json"), plan)
	if err == nil || !strings.Contains(err.Error(), "fsync canary") {
		t.Fatalf("directory sync error = %v", err)
	}
}

func TestDistributionCreateOnlyLinkFallbackRollsBackPublishedFileWhenTemporaryCleanupFails(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "state.json")
	previousRename := distributionRenameNoReplaceForTest
	previousRemove := distributionRemoveTemporaryForTest
	distributionRenameNoReplaceForTest = func(*os.Root, string, string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}
	distributionRemoveTemporaryForTest = func(*os.Root, string) error {
		return errors.New("temporary cleanup canary")
	}
	t.Cleanup(func() {
		distributionRenameNoReplaceForTest = previousRename
		distributionRemoveTemporaryForTest = previousRemove
	})

	err := writeProtectedDistributionJSONCreateOnly(path, map[string]string{"state": "pending"})
	if err == nil || !strings.Contains(err.Error(), "temporary cleanup canary") {
		t.Fatalf("writeProtectedDistributionJSONCreateOnly() error = %v", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("published file remains after failed temporary cleanup: %v", statErr)
	}
}

func validDistributionStateReceiptFixture() persistedDistributionReceipt {
	certificate := strings.Repeat("5", 64)
	return persistedDistributionReceipt{
		SchemaVersion: 1, RunID: "drun_0123456789abcdef0123456789abcdef", PlanID: "dplan_0123456789abcdef0123456789abcdef", PlanHash: strings.Repeat("a", 64),
		Status: "published_and_fetch_verified", CompletedAt: "2098-01-01T00:00:01Z", PublicationReceiptPath: "publish/receipt.json", PublicationReceiptSHA256: strings.Repeat("b", 64), LinkPath: "secrets/link.json", LinkSHA256: strings.Repeat("0", 64),
		ArtifactSHA256: strings.Repeat("c", 64), ArtifactSizeBytes: 1234, BundleDescriptorSHA256: strings.Repeat("d", 64), BundleDescriptorSizeBytes: 500,
		ArtifactKey: "ios/artifact.ipa", ManifestKey: "ios/manifest.plist", PageKey: "ios/index.html", ManifestSHA256: strings.Repeat("e", 64), ManifestSizeBytes: 200, PageSHA256: strings.Repeat("f", 64), PageSizeBytes: 300,
		InstallURLRedacted: "https://objects.example.com/index.html?REDACTED", AppBundleID: "com.example", AppVersion: "1.0", AppBuildNumber: "1", TeamID: "TEAM", ProfileResourceID: "profile", ProfileClass: "ad-hoc", ProfileUUID: "uuid", ProfileExpiresAt: "2099-01-01T00:00:00Z", ProfileSHA256: strings.Repeat("1", 64), DeviceSetSHA256: strings.Repeat("2", 64), DeviceCount: 1, CertificateSHA256: certificate,
		ProfileCertificateSHA256Fingerprints: []string{certificate}, SignerCertificateSHA256Fingerprints: []string{certificate}, ProfileIntegrityStatus: "verified", ProfileTrustStatus: "verified", CodeSignatureStatus: "verified", CodeSignatureScope: "complete-main-app-code-resources-entitlements-and-profile-certificate-binding", FetchVerified: true, FetchVerifiedAt: "2098-01-01T00:00:00Z",
	}
}

func TestDistributionStateRejectsSecretsAndUnboundedIdentity(t *testing.T) {
	plan := distributionPersistedPlanFixture(t.TempDir())
	plan.Identity.ExpirationDate = "not-a-date"
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err == nil || !strings.Contains(err.Error(), "expirationDate") {
		t.Fatalf("expiration error = %v", err)
	}

	plan = distributionPersistedPlanFixture(t.TempDir())
	plan.Ready = false
	plan.Blockers = []distributionBlocker{{Code: "provider_error", Stage: "publish", Message: "fetch https://example.com/?token=secret"}}
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePersistedDistributionPlan(plan); err == nil || !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("blocker secret error = %v", err)
	}

	receipt := persistedDistributionReceipt{
		SchemaVersion: 1, RunID: "drun_0123456789abcdef0123456789abcdef", PlanID: "dplan_0123456789abcdef0123456789abcdef", PlanHash: strings.Repeat("a", 64), Status: "published_and_fetch_verified", CompletedAt: "2098-01-01T00:00:01Z",
		PublicationReceiptPath: "publish/receipt.json", PublicationReceiptSHA256: strings.Repeat("b", 64), LinkPath: "secrets/link.json", LinkSHA256: strings.Repeat("0", 64), ArtifactSHA256: strings.Repeat("c", 64), ArtifactSizeBytes: 1, BundleDescriptorSHA256: strings.Repeat("d", 64), BundleDescriptorSizeBytes: 1, ManifestSHA256: strings.Repeat("e", 64), ManifestSizeBytes: 1, PageSHA256: strings.Repeat("f", 64), PageSizeBytes: 1,
		AppBundleID: "com.example", AppVersion: "1", AppBuildNumber: "1", TeamID: "TEAM", ProfileResourceID: "profile", ProfileClass: "ad-hoc", ProfileUUID: "uuid", ProfileExpiresAt: "2099-01-01T00:00:00Z", ProfileSHA256: strings.Repeat("1", 64), DeviceSetSHA256: strings.Repeat("2", 64), DeviceCount: 1, CertificateSHA256: strings.Repeat("3", 64), ProfileCertificateSHA256Fingerprints: []string{strings.Repeat("3", 64)}, SignerCertificateSHA256Fingerprints: []string{strings.Repeat("3", 64)}, ProfileIntegrityStatus: "verified", ProfileTrustStatus: "verified", CodeSignatureStatus: "verified", CodeSignatureScope: "complete-main-app-code-resources-entitlements-and-profile-certificate-binding", FetchVerified: true, FetchVerifiedAt: "2098-01-01T00:00:00Z",
		ArtifactKey: "ios/artifact.ipa", ManifestKey: "ios/manifest.plist", PageKey: "ios/index.html", InstallURLRedacted: "https://objects.example.com/index.html?token=secret&REDACTED=true",
	}
	if err := validatePersistedDistributionReceipt(receipt); err == nil || !strings.Contains(err.Error(), "complete query redacted") {
		t.Fatalf("receipt URL secret error = %v", err)
	}
}

func TestDistributionStateRootAllowsOrdinaryOwnerAncestors(t *testing.T) {
	ancestor := filepath.Join(t.TempDir(), ".asc")
	if err := os.Mkdir(ancestor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ancestor, 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(ancestor, "distribution", "runs")
	if err := ensureDistributionStateRoot(stateDir); err != nil {
		t.Fatalf("ensureDistributionStateRoot() error = %v", err)
	}
	assertPrivateDirectory(t, stateDir)
	info, err := os.Stat(ancestor)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("ancestor mode changed to %o", info.Mode().Perm())
	}
}

func TestDistributionStateRootRejectsSymlinkAncestor(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, ".asc")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := ensureDistributionStateRoot(filepath.Join(link, "distribution", "runs")); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("symlink ancestor error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "distribution")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created state through symlink: %v", err)
	}
}

func TestDistributionStateWriteRetainsRunDirectoryAcrossSymlinkSwap(t *testing.T) {
	base := t.TempDir()
	stateDir := filepath.Join(base, "state")
	runID := "drun_0123456789abcdef0123456789abcdef"
	if err := createDistributionRunDirectory(stateDir, runID); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(stateDir, runID)
	movedDir := filepath.Join(stateDir, runID+"-original")
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "state.json")
	if err := os.WriteFile(sentinel, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	distributionAfterParentOpenForTest = func() {
		if err := os.Rename(runDir, movedDir); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, runDir); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { distributionAfterParentOpenForTest = nil })
	state := persistedDistributionRunState{
		SchemaVersion: 1, RunID: runID, PlanID: "dplan_0123456789abcdef0123456789abcdef",
		PlanPath: filepath.Join(base, "plan.json"), PlanHash: strings.Repeat("a", 64), Status: "running", Stage: "preflight",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339), Attempt: 1,
	}
	if err := writeDistributionRunState(stateDir, state); err != nil {
		t.Fatalf("writeDistributionRunState() error = %v", err)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "sentinel" {
		t.Fatalf("outside sentinel changed: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(movedDir, "state.json")); err != nil {
		t.Fatalf("pinned original did not receive state: %v", err)
	}
}

func distributionPersistedPlanFixture(stateDir string) persistedDistributionPlan {
	return persistedDistributionPlan{
		SchemaVersion: 1, PlanID: "dplan_0123456789abcdef0123456789abcdef", CreatedAt: time.Now().UTC().Format(time.RFC3339), Ready: true,
		ConfigPath: "/private/config.json", ConfigSHA256: strings.Repeat("1", 64),
		Archive: distributionArchiveBinding{
			Path: "/private/App.xcarchive", TreeSHA256: strings.Repeat("2", 64), SizeBytes: 1234, FileCount: 8,
			BundleID: "com.example.app", Title: "Preview", PublishedTitle: "Preview", Version: "1.0", BuildNumber: "1", TeamID: "TEAM123", TargetCount: 1,
		},
		DeviceSet:   distributionDeviceSetBinding{SHA256: strings.Repeat("3", 64), FileSHA256: strings.Repeat("6", 64), Count: 1},
		Identity:    distributionIdentityBinding{CertificateResourceID: "cert-resource-1", CertificateSHA256: strings.Repeat("4", 64), TeamID: "TEAM123", ExpirationDate: "2099-01-01T00:00:00Z", MinimumValidUntil: "2098-01-01T00:00:00Z"},
		Publication: distributionPublicationConfig{Endpoint: "https://objects.example.com", Region: "us-east-1", Bucket: "builds", Prefix: "ios/pr-42", AddressingStyle: "path", URLTTL: "1h", DownloadGrace: "5m", VerifyTimeout: "30s"},
		Reconcile:   distributionReconcileBinding{PlanPath: filepath.Join(stateDir, "reconcile-plan.json"), PlanHash: strings.Repeat("5", 64), ReceiptPath: filepath.Join(stateDir, "reconcile-receipt.json"), MinimumValidityDays: 1, MutationCount: 1, MaxMutations: 10},
		Effects:     []distributionEffect{{Stage: "account_reconcile", Kind: "register_device", Count: 1}},
		Paths:       distributionPlanPaths{StateDir: stateDir},
	}
}

func distributionStateArchiveAppFixture() archiveAppIdentity {
	return archiveAppIdentity{BundleID: "com.example.app", Title: "Preview", Version: "1.0", BuildNumber: "1"}
}

func writeDistributionFetchVerifyFixture(t *testing.T) (string, persistedDistributionRunState, persistedDistributionReceipt) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	planPath := filepath.Join(root, "plan.json")
	plan := distributionPersistedPlanFixture(root)
	plan.CreatedAt = "2097-01-01T00:00:00Z"
	if err := sealDistributionPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := writePersistedDistributionPlan(planPath, plan); err != nil {
		t.Fatal(err)
	}
	runID := "drun_0123456789abcdef0123456789abcdef"
	if err := createDistributionRunDirectory(root, runID); err != nil {
		t.Fatal(err)
	}
	descriptorData, descriptorSHA256 := writeDistributionDescriptorFixture(t, root, runID, plan)
	publicationSHA256 := writeDistributionPublicationReceiptFixture(t, root, runID, plan)
	linkSHA256 := writeDistributionLinkFixture(t, root, runID, []byte("sensitive link fixture"))
	run := persistedDistributionRunState{
		SchemaVersion: 1, RunID: runID, PlanID: plan.PlanID, PlanPath: planPath, PlanHash: plan.PlanHash,
		Status: "running", Stage: "fetch_verify", UpdatedAt: "2098-01-01T00:00:00Z", Attempt: 1,
		Artifacts: distributionRunArtifacts{
			ArchiveSnapshot:  &distributionArchiveSnapshot{RelativePath: "archive/App.xcarchive", TreeSHA256: plan.Archive.TreeSHA256, SizeBytes: plan.Archive.SizeBytes, EntryCount: plan.Archive.FileCount, App: distributionStateArchiveAppFixture()},
			ReconcileReceipt: &distributionFileArtifact{Path: "reconcile/receipt.json", SHA256: strings.Repeat("9", 64)},
			Profile:          &distributionProfileArtifact{ResourceID: "profile-resource", UUID: "11111111-1111-1111-1111-111111111111", Path: "reconcile/profile.mobileprovision", SHA256: strings.Repeat("7", 64), BundleID: plan.Archive.BundleID},
			ExportOptions:    &distributionFileArtifact{Path: "export/options.plist", SHA256: strings.Repeat("a", 64)},
			IPA:              &distributionSizedFileArtifact{Path: "export/app.ipa", SHA256: strings.Repeat("b", 64), SizeBytes: 1234},
			SigningReceipt:   &distributionFileArtifact{Path: "signing/receipt.json", SHA256: strings.Repeat("8", 64)},
			Bundle:           &distributionBundleArtifact{Path: "prepared/bundle", DescriptorSHA256: descriptorSHA256},
			Publication:      &distributionPublicationArtifact{ReceiptPath: "publish/receipt.json", ReceiptSHA256: publicationSHA256, LinkPath: "secrets/link.json", LinkSHA256: linkSHA256, ArtifactKey: "ios/artifact.ipa", ManifestKey: "ios/manifest.plist", PageKey: "ios/index.html", InstallURLRedacted: "https://objects.example.com/page?REDACTED"},
		},
	}
	receipt := persistedDistributionReceipt{
		SchemaVersion: 1, RunID: runID, PlanID: plan.PlanID, PlanHash: plan.PlanHash, Status: "published_and_fetch_verified", CompletedAt: "2098-01-01T00:00:01Z",
		PublicationReceiptPath: "publish/receipt.json", PublicationReceiptSHA256: publicationSHA256, LinkPath: "secrets/link.json", LinkSHA256: linkSHA256,
		ArtifactSHA256: strings.Repeat("b", 64), ArtifactSizeBytes: 1234, BundleDescriptorSHA256: descriptorSHA256, BundleDescriptorSizeBytes: int64(len(descriptorData)),
		ManifestSHA256: strings.Repeat("1", 64), ManifestSizeBytes: 200, PageSHA256: strings.Repeat("2", 64), PageSizeBytes: 300,
		ArtifactKey: "ios/artifact.ipa", ManifestKey: "ios/manifest.plist", PageKey: "ios/index.html", InstallURLRedacted: "https://objects.example.com/page?REDACTED",
		AppBundleID: plan.Archive.BundleID, AppVersion: "1.0", AppBuildNumber: "1", TeamID: plan.Identity.TeamID, ProfileResourceID: "profile-resource", ProfileClass: "ad-hoc", ProfileUUID: "11111111-1111-1111-1111-111111111111", ProfileExpiresAt: "2099-01-01T00:00:00Z", ProfileSHA256: strings.Repeat("7", 64), DeviceSetSHA256: plan.DeviceSet.SHA256, DeviceCount: plan.DeviceSet.Count, CertificateSHA256: plan.Identity.CertificateSHA256, ProfileCertificateSHA256Fingerprints: []string{plan.Identity.CertificateSHA256}, SignerCertificateSHA256Fingerprints: []string{plan.Identity.CertificateSHA256}, ProfileIntegrityStatus: "verified", ProfileTrustStatus: "verified", CodeSignatureStatus: "verified", CodeSignatureScope: core.CodeSignatureScopeCompleteMainApp, FetchVerified: true, FetchVerifiedAt: "2098-01-01T00:00:00Z",
	}
	return root, run, receipt
}

func writeDistributionDescriptorFixture(t *testing.T, stateDir, runID string, plan persistedDistributionPlan) ([]byte, string) {
	t.Helper()
	verified := core.CodeSignatureVerification{Status: core.CodeSignatureVerified}
	descriptor := core.Descriptor{
		SchemaVersion: "1", Platform: "IOS", DistributionMethod: "release-testing",
		App:      core.App{BundleID: plan.Archive.BundleID, Title: "Preview", Version: "1.0", BuildNumber: "1"},
		Artifact: core.Artifact{RelativePath: "payload/app.ipa", SHA256: strings.Repeat("b", 64), SizeBytes: 1234},
		Signing: core.Signing{
			ProfileClass: core.ProfileClassAdHoc, ProfileUUID: "11111111-1111-1111-1111-111111111111", TeamID: plan.Identity.TeamID, ExpiresAt: "2099-01-01T00:00:00Z",
			DeviceCount: plan.DeviceSet.Count, DeviceSetSHA256: plan.DeviceSet.SHA256, EmbeddedProfileSHA256: strings.Repeat("7", 64), ProfileCertificateSHA256Fingerprints: []string{plan.Identity.CertificateSHA256},
			ProfileIntegrityVerification: verified, ProfileTrustVerification: verified,
			CodeSignatureVerification: core.CodeSignatureVerification{Status: core.CodeSignatureVerified, Scope: core.CodeSignatureScopeCompleteMainApp, SignerCertificateSHA256Fingerprints: []string{plan.Identity.CertificateSHA256}},
		},
	}
	data, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join(stateDir, runID, "prepared", "bundle", "bundle.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return data, fmt.Sprintf("%x", digest[:])
}

func writeDistributionPublicationReceiptFixture(t *testing.T, stateDir, runID string, plan persistedDistributionPlan) string {
	t.Helper()
	verified := core.PreparedCodeSignatureVerification{Status: string(core.CodeSignatureVerified)}
	expiresAt := time.Date(2098, 1, 2, 0, 0, 0, 0, time.UTC)
	publication := core.PublishReceipt{
		SchemaVersion: "1", Endpoint: plan.Publication.Endpoint, DownloadEndpoint: plan.Publication.Endpoint, Region: plan.Publication.Region, AddressingStyle: plan.Publication.AddressingStyle,
		Access: core.AccessPrivate, Bucket: plan.Publication.Bucket, Prefix: plan.Publication.Prefix, URLTTL: plan.Publication.URLTTL, DownloadGrace: plan.Publication.DownloadGrace, Verified: true,
		Artifact:         core.StoredObject{Key: "ios/artifact.ipa", SHA256: strings.Repeat("b", 64), SizeBytes: 1234, ContentType: core.ContentTypeIPA, Status: "uploaded"},
		Manifest:         core.StoredObject{Key: "ios/manifest.plist", SHA256: strings.Repeat("1", 64), SizeBytes: 200, ContentType: core.ContentTypeManifest, Status: "reused"},
		Page:             core.StoredObject{Key: "ios/index.html", SHA256: strings.Repeat("2", 64), SizeBytes: 300, ContentType: core.ContentTypeHTML, Status: "uploaded"},
		InstallURL:       "https://objects.example.com/page?REDACTED",
		DirectInstallURL: "itms-services://?action=download-manifest&url=REDACTED",
		ExpiresAt:        &expiresAt,
		App:              core.PreparedApp{BundleID: plan.Archive.BundleID, Title: "Preview", Version: "1.0", BuildNumber: "1"},
		Signing: core.ReceiptSigning{
			ProfileClass: "ad-hoc", ProfileUUID: "11111111-1111-1111-1111-111111111111", EmbeddedProfileSHA256: strings.Repeat("7", 64), TeamID: plan.Identity.TeamID, ProfileExpiresAt: "2099-01-01T00:00:00Z",
			DeviceCount: plan.DeviceSet.Count, DeviceSetSHA256: plan.DeviceSet.SHA256, ProfileCertificateFingerprints: []string{plan.Identity.CertificateSHA256},
			ProfileIntegrityVerification: verified, ProfileTrustVerification: verified,
			CodeSignatureVerification: core.PreparedCodeSignatureVerification{Status: string(core.CodeSignatureVerified), Scope: core.CodeSignatureScopeCompleteMainApp, SignerCertificateSHA256Fingerprints: []string{plan.Identity.CertificateSHA256}},
		},
		ReceiptPath: filepath.Join(stateDir, runID, "publish", "receipt.json"), LinkPath: filepath.Join(stateDir, runID, "secrets", "link.json"),
	}
	data, err := json.MarshalIndent(publication, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join(stateDir, runID, "publish", "receipt.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest[:])
}

func writeDistributionLinkFixture(t *testing.T, stateDir, runID string, data []byte) string {
	t.Helper()
	linkPath := filepath.Join(stateDir, runID, "secrets", "link.json")
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(linkPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest[:])
}

func tamperDistributionPublicationReceiptFixture(t *testing.T, stateDir, runID string, mutate func(*core.PublishReceipt)) {
	t.Helper()
	receiptPath := filepath.Join(stateDir, runID, "publish", "receipt.json")
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt core.PublishReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	mutate(&receipt)
	data, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(receiptPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func rewriteDistributionPublicationDigest(t *testing.T, stateDir string, run *persistedDistributionRunState, receipt *persistedDistributionReceipt) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, run.RunID, "publish", "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	value := fmt.Sprintf("%x", digest[:])
	run.Artifacts.Publication.ReceiptSHA256 = value
	receipt.PublicationReceiptSHA256 = value
}

func assertPrivateFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("%s mode = %v", path, info.Mode())
	}
}

func assertPrivateDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("%s mode = %v", path, info.Mode())
	}
}

func newDistributionRunID() (string, error) { return newDistributionID("drun_") }

func ensureDistributionStateRoot(path string) error {
	rooted, err := openOrCreateDistributionDirectory(path)
	if rooted != nil {
		_ = rooted.Close()
	}
	return err
}
