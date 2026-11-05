package distribute

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestValidateStoredPrivateStateRejectsLifetimeOverflow(t *testing.T) {
	receipt := privatePublishTestReceipt()
	receipt.Endpoint = "https://objects.example.com"
	receipt.DownloadEndpoint = "https://objects.example.com"
	receipt.Region = "auto"
	receipt.AddressingStyle = "path"
	receipt.URLTTL = time.Duration(1 << 62).String()
	receipt.DownloadGrace = time.Duration(1 << 62).String()
	receipt.ReceiptPath = "/state/receipt.json"
	receipt.LinkPath = "/state/link.json"
	bundle := &distribution.PreparedBundle{
		IPASHA256: "sha",
		IPASize:   3,
		Descriptor: distribution.PreparedDescriptor{
			App: distribution.PreparedApp{BundleID: "com.example", Version: "1", BuildNumber: "2"},
		},
	}

	err := validateStoredPrivateState(
		publishState{
			SchemaVersion: "1",
			Receipt:       receipt,
			Links:         distribution.SensitiveLinks{SchemaVersion: "1", InstallURL: "https://objects.example.com/install?secret"},
		},
		bundle,
		receipt.ReceiptPath,
		receipt.LinkPath,
	)
	if err == nil || !strings.Contains(err.Error(), "invalid saved download grace") {
		t.Fatalf("overflowing saved publication lifetime error = %v", err)
	}
}

func TestLoadPublishStateRejectsInPlaceMutation(t *testing.T) {
	originalHook := publishAfterProtectedReadForTest
	t.Cleanup(func() { publishAfterProtectedReadForTest = originalHook })

	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "receipt.json")
	linkPath := filepath.Join(stateDir, "link.json")
	writePrivatePublishTestState(
		t,
		receiptPath,
		linkPath,
		privatePublishTestReceipt(),
		distribution.SensitiveLinks{SchemaVersion: "1", InstallURL: "https://objects.example.com/install?secret"},
	)
	artifacts, err := openExistingArtifactPaths(receiptPath, linkPath)
	if err != nil {
		t.Fatal(err)
	}
	defer artifacts.close()
	publishAfterProtectedReadForTest = func(label string) {
		if label != "sensitive link artifact" {
			return
		}
		file, openErr := os.OpenFile(linkPath, os.O_APPEND|os.O_WRONLY, 0)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, writeErr := file.Write([]byte(" ")); writeErr != nil {
			_ = file.Close()
			t.Fatal(writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}

	_, _, err = artifacts.loadState()
	if err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("mutated sensitive state error = %v", err)
	}
}

func TestOpenExistingArtifactPathsSupportsDistinctTopLevelParents(t *testing.T) {
	receiptDir, err := os.MkdirTemp(t.TempDir(), "asc-existing-receipt-*")
	if err != nil {
		t.Fatal(err)
	}
	linkDir, err := os.MkdirTemp(t.TempDir(), "asc-existing-link-*")
	if err != nil {
		t.Fatal(err)
	}

	receiptPath := filepath.Join(receiptDir, "receipt.json")
	linkPath := filepath.Join(linkDir, "link.json")
	writePrivatePublishTestState(
		t,
		receiptPath,
		linkPath,
		privatePublishTestReceipt(),
		distribution.SensitiveLinks{SchemaVersion: "1", InstallURL: "https://objects.example.com/install?secret"},
	)
	paths, err := openExistingArtifactPaths(receiptPath, linkPath)
	if err != nil {
		t.Fatal(err)
	}
	defer paths.close()
	if !paths.receiptExists || !paths.linkExists {
		t.Fatalf("artifact existence = receipt:%t link:%t, want both true", paths.receiptExists, paths.linkExists)
	}
}

func TestReverifyPrivatePublishIsReadOnly(t *testing.T) {
	originalLoad, originalStore, originalPublish, originalReverify := loadPreparedBundle, newObjectStore, runPublish, reverifyPublication
	t.Cleanup(func() {
		loadPreparedBundle, newObjectStore, runPublish, reverifyPublication = originalLoad, originalStore, originalPublish, originalReverify
	})

	bundleDir, bundle := privatePublishTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
	newObjectStore = func(context.Context, distribution.S3StoreConfig) (distribution.ObjectStore, time.Time, error) {
		t.Fatal("read-only verification constructed an object store")
		return nil, time.Time{}, nil
	}
	runPublish = func(context.Context, io.ReadSeeker, distribution.PreparedDescriptor, distribution.PublishOptions) (distribution.PublishReceipt, distribution.SensitiveLinks, error) {
		t.Fatal("read-only verification attempted publication")
		return distribution.PublishReceipt{}, distribution.SensitiveLinks{}, nil
	}
	reverifyCalls := 0
	reverifyPublication = func(_ context.Context, _ distribution.Verifier, _ distribution.PublishReceipt, _ distribution.SensitiveLinks, _ time.Time) error {
		reverifyCalls++
		return nil
	}

	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "receipt.json")
	linkPath := filepath.Join(stateDir, "link.json")
	receipt := privatePublishTestReceipt()
	receipt.Endpoint = "https://objects.example.com"
	receipt.DownloadEndpoint = "https://objects.example.com"
	receipt.Region = "auto"
	receipt.AddressingStyle = "path"
	receipt.URLTTL = "24h0m0s"
	receipt.DownloadGrace = "1h0m0s"
	receipt.ReceiptPath = receiptPath
	receipt.LinkPath = linkPath
	links := distribution.SensitiveLinks{
		SchemaVersion: "1",
		InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=exact-secret-canary",
	}
	writePrivatePublishTestState(t, receiptPath, linkPath, receipt, links)
	beforeReceipt, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLink, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeReceiptInfo, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLinkInfo, err := os.Stat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeEntries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}

	result, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir:     bundleDir,
		ReceiptPath:   receiptPath,
		LinkPath:      linkPath,
		VerifyTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("reverifyPrivatePublish() error = %v", err)
	}
	if !result.Recovered || reverifyCalls != 1 {
		t.Fatalf("result=%+v reverifyCalls=%d", result, reverifyCalls)
	}
	afterReceipt, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	afterLink, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterReceipt) != string(beforeReceipt) || string(afterLink) != string(beforeLink) {
		t.Fatal("read-only verification changed publication artifacts")
	}
	afterReceiptInfo, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	afterLinkInfo, err := os.Stat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	afterEntries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if beforeReceiptInfo.Mode() != afterReceiptInfo.Mode() || !beforeReceiptInfo.ModTime().Equal(afterReceiptInfo.ModTime()) ||
		beforeLinkInfo.Mode() != afterLinkInfo.Mode() || !beforeLinkInfo.ModTime().Equal(afterLinkInfo.ModTime()) ||
		!reflect.DeepEqual(directoryEntryNames(beforeEntries), directoryEntryNames(afterEntries)) {
		t.Fatal("read-only verification changed artifact metadata or directory entries")
	}
}

func TestReverifyPrivatePublishReadsCaseDistinctArtifactsFromReadOnlyDirectory(t *testing.T) {
	originalAliasProbe := probeConfiguredArtifactAliasForPreflight
	t.Cleanup(func() { probeConfiguredArtifactAliasForPreflight = originalAliasProbe })
	probeConfiguredArtifactAliasForPreflight = func(artifactPaths) error {
		t.Fatal("read-only verification attempted an artifact alias probe")
		return nil
	}

	stateDir := t.TempDir()
	probe := filepath.Join(stateDir, "CaseSensitiveProbe")
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(stateDir, "casesensitiveprobe"))
	if err == nil {
		t.Skip("test volume is case-insensitive")
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}

	originalLoad, originalReverify := loadPreparedBundle, reverifyPublication
	t.Cleanup(func() { loadPreparedBundle, reverifyPublication = originalLoad, originalReverify })
	bundleDir, bundle := privatePublishTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
	reverifyPublication = func(context.Context, distribution.Verifier, distribution.PublishReceipt, distribution.SensitiveLinks, time.Time) error {
		return nil
	}

	receiptPath := filepath.Join(stateDir, "Publish.JSON")
	linkPath := filepath.Join(stateDir, "publish.json")
	receipt := privatePublishTestReceipt()
	receipt.Endpoint = "https://objects.example.com"
	receipt.DownloadEndpoint = "https://objects.example.com"
	receipt.Region = "auto"
	receipt.AddressingStyle = "path"
	receipt.URLTTL = "24h0m0s"
	receipt.DownloadGrace = "1h0m0s"
	receipt.ReceiptPath = receiptPath
	receipt.LinkPath = linkPath
	writePrivatePublishTestState(t, receiptPath, linkPath, receipt, distribution.SensitiveLinks{
		SchemaVersion: "1",
		InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=read-only-secret-canary",
	})
	if err := os.Chmod(stateDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stateDir, 0o700) })

	result, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir: bundleDir, ReceiptPath: receiptPath, LinkPath: linkPath, VerifyTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("reverifyPrivatePublish() error = %v", err)
	}
	if !result.Recovered {
		t.Fatal("case-distinct artifacts were not reported as recovered")
	}
}

func TestOpenExistingArtifactPathsChecksExistingFilesBeforeAliasProbe(t *testing.T) {
	originalAliasProbe := probeConfiguredArtifactAliasForPreflight
	t.Cleanup(func() { probeConfiguredArtifactAliasForPreflight = originalAliasProbe })
	probeConfiguredArtifactAliasForPreflight = func(artifactPaths) error {
		t.Fatal("existing-artifact verification attempted an artifact alias probe")
		return nil
	}

	stateDir := t.TempDir()
	_, err := openExistingArtifactPaths(
		filepath.Join(stateDir, "Publish.JSON"),
		filepath.Join(stateDir, "publish.json"),
	)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("openExistingArtifactPaths() error = %v, want missing-artifact error", err)
	}
}

func TestLoadPublishStateMissingDoesNotCreateArtifactParent(t *testing.T) {
	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "missing", "receipt.json")
	linkPath := filepath.Join(stateDir, "missing", "link.json")
	artifacts, err := inspectArtifactPaths(receiptPath, linkPath)
	if err != nil {
		t.Fatal(err)
	}
	defer artifacts.close()
	if _, found, err := artifacts.loadState(); err != nil || found {
		t.Fatalf("loadState() = found:%t err:%v, want missing state without error", found, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "missing")); !os.IsNotExist(err) {
		t.Fatalf("loadState() created missing artifact parent: %v", err)
	}
}

func TestReverifyPrivatePublishTrimsArtifactPathsBeforeContainmentCheck(t *testing.T) {
	originalLoad, originalReverify := loadPreparedBundle, reverifyPublication
	t.Cleanup(func() { loadPreparedBundle, reverifyPublication = originalLoad, originalReverify })
	bundleDir, bundle := privatePublishTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
	reverifyPublication = func(context.Context, distribution.Verifier, distribution.PublishReceipt, distribution.SensitiveLinks, time.Time) error {
		return nil
	}

	stateDir := filepath.Join(bundleDir, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(stateDir, "receipt.json")
	linkPath := filepath.Join(stateDir, "link.json")
	receipt := privatePublishTestReceipt()
	receipt.Endpoint = "https://objects.example.com"
	receipt.DownloadEndpoint = "https://objects.example.com"
	receipt.Region = "auto"
	receipt.AddressingStyle = "path"
	receipt.URLTTL = "24h0m0s"
	receipt.DownloadGrace = "1h0m0s"
	receipt.ReceiptPath = receiptPath
	receipt.LinkPath = linkPath
	writePrivatePublishTestState(t, receiptPath, linkPath, receipt, distribution.SensitiveLinks{
		SchemaVersion: "1",
		InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=containment-canary",
	})

	_, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir: bundleDir, ReceiptPath: " " + receiptPath + " ", LinkPath: " " + linkPath + " ", VerifyTimeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "outside the immutable prepared bundle") {
		t.Fatalf("reverifyPrivatePublish() error = %v, want bundle-containment rejection", err)
	}
}

func TestReverifyPrivatePublishDoesNotCreateOrRepairArtifacts(t *testing.T) {
	bundleDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "missing")
	_, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir:     bundleDir,
		ReceiptPath:   filepath.Join(missingDir, "receipt.json"),
		LinkPath:      filepath.Join(missingDir, "link.json"),
		VerifyTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected missing artifact error")
	}
	if _, statErr := os.Stat(missingDir); !os.IsNotExist(statErr) {
		t.Fatalf("read-only verification created state directory: %v", statErr)
	}

	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "receipt.json")
	linkPath := filepath.Join(stateDir, "link.json")
	receipt := privatePublishTestReceipt()
	receipt.ReceiptPath = receiptPath
	receipt.LinkPath = linkPath
	stateData, marshalErr := encodeJSON(publishState{SchemaVersion: "1", Receipt: receipt})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if writeErr := os.WriteFile(linkPath, stateData, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir: bundleDir, ReceiptPath: receiptPath, LinkPath: linkPath, VerifyTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected missing receipt error")
	}
	if _, statErr := os.Stat(receiptPath); !os.IsNotExist(statErr) {
		t.Fatalf("read-only verification repaired receipt: %v", statErr)
	}
}

func TestReverifyPrivatePublishRejectsPublicStateWithoutNetwork(t *testing.T) {
	originalLoad, originalReverify := loadPreparedBundle, reverifyPublication
	t.Cleanup(func() { loadPreparedBundle, reverifyPublication = originalLoad, originalReverify })
	bundleDir, bundle := privatePublishTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
	networkCalled := false
	reverifyPublication = func(context.Context, distribution.Verifier, distribution.PublishReceipt, distribution.SensitiveLinks, time.Time) error {
		networkCalled = true
		return nil
	}
	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "receipt.json")
	linkPath := filepath.Join(stateDir, "link.json")
	receipt := privatePublishTestReceipt()
	receipt.Access = distribution.AccessPublic
	receipt.PublicBaseURL = "https://downloads.example.com"
	receipt.ReceiptPath = receiptPath
	receipt.LinkPath = linkPath
	writePrivatePublishTestState(t, receiptPath, linkPath, receipt, distribution.SensitiveLinks{
		SchemaVersion: "1",
		InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=expired-secret-canary",
	})

	_, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir: bundleDir, ReceiptPath: receiptPath, LinkPath: linkPath, VerifyTimeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "not private") {
		t.Fatalf("error = %v, want private-state rejection", err)
	}
	if networkCalled {
		t.Fatal("public state reached network re-verification")
	}
}

func TestReverifyPrivatePublishDoesNotRefreshExpiredLinks(t *testing.T) {
	originalLoad, originalStore, originalPublish, originalReverify := loadPreparedBundle, newObjectStore, runPublish, reverifyPublication
	t.Cleanup(func() {
		loadPreparedBundle, newObjectStore, runPublish, reverifyPublication = originalLoad, originalStore, originalPublish, originalReverify
	})
	bundleDir, bundle := privatePublishTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
	newObjectStore = func(context.Context, distribution.S3StoreConfig) (distribution.ObjectStore, time.Time, error) {
		t.Fatal("expired-link verification constructed an object store")
		return nil, time.Time{}, nil
	}
	runPublish = func(context.Context, io.ReadSeeker, distribution.PreparedDescriptor, distribution.PublishOptions) (distribution.PublishReceipt, distribution.SensitiveLinks, error) {
		t.Fatal("expired-link verification refreshed publication")
		return distribution.PublishReceipt{}, distribution.SensitiveLinks{}, nil
	}
	reverifyPublication = func(context.Context, distribution.Verifier, distribution.PublishReceipt, distribution.SensitiveLinks, time.Time) error {
		return errors.New("saved install link is expired")
	}
	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "receipt.json")
	linkPath := filepath.Join(stateDir, "link.json")
	receipt := privatePublishTestReceipt()
	receipt.Endpoint = "https://objects.example.com"
	receipt.DownloadEndpoint = "https://objects.example.com"
	receipt.Region = "auto"
	receipt.AddressingStyle = "path"
	receipt.URLTTL = "24h0m0s"
	receipt.DownloadGrace = "1h0m0s"
	receipt.ReceiptPath = receiptPath
	receipt.LinkPath = linkPath
	writePrivatePublishTestState(t, receiptPath, linkPath, receipt, distribution.SensitiveLinks{
		SchemaVersion: "1",
		InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=expired-secret-canary",
	})
	beforeReceipt, _ := os.ReadFile(receiptPath)
	beforeLink, _ := os.ReadFile(linkPath)

	_, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
		BundleDir: bundleDir, ReceiptPath: receiptPath, LinkPath: linkPath, VerifyTimeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("error = %v, want expired-link failure", err)
	}
	afterReceipt, _ := os.ReadFile(receiptPath)
	afterLink, _ := os.ReadFile(linkPath)
	if string(afterReceipt) != string(beforeReceipt) || string(afterLink) != string(beforeLink) {
		t.Fatal("expired-link verification changed publication artifacts")
	}
}

func TestPublishCommandRecoveryRejectsHardLinkedArtifacts(t *testing.T) {
	for _, artifact := range []string{"receipt", "link"} {
		t.Run(artifact, func(t *testing.T) {
			originalLoad, originalStore, originalPublish, originalReverify := loadPreparedBundle, newObjectStore, runPublish, reverifyPublication
			t.Cleanup(func() {
				loadPreparedBundle, newObjectStore, runPublish, reverifyPublication = originalLoad, originalStore, originalPublish, originalReverify
			})
			bundleDir, bundle := privatePublishTestBundle(t)
			loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
			storeCalls := 0
			newObjectStore = func(context.Context, distribution.S3StoreConfig) (distribution.ObjectStore, time.Time, error) {
				storeCalls++
				return noOpStore{}, time.Time{}, nil
			}
			runPublish = func(context.Context, io.ReadSeeker, distribution.PreparedDescriptor, distribution.PublishOptions) (distribution.PublishReceipt, distribution.SensitiveLinks, error) {
				return privatePublishTestReceipt(), distribution.SensitiveLinks{
					SchemaVersion: "1",
					InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=hard-link-secret-canary",
				}, nil
			}
			reverifyPublication = func(context.Context, distribution.Verifier, distribution.PublishReceipt, distribution.SensitiveLinks, time.Time) error {
				return nil
			}
			stateDir := t.TempDir()
			receiptPath := filepath.Join(stateDir, "receipt.json")
			linkPath := filepath.Join(stateDir, "link.json")
			args := []string{
				"--bundle-dir", bundleDir, "--endpoint", "https://objects.example.com", "--region", "auto", "--bucket", "bucket", "--prefix", "app",
				"--verify-timeout", "1s", "--receipt", receiptPath, "--link-path", linkPath, "--output", "json",
			}
			if err := PublishCommand().ParseAndRun(context.Background(), args); err != nil {
				t.Fatalf("initial publish: %v", err)
			}
			target := receiptPath
			if artifact == "link" {
				target = linkPath
			}
			if err := os.Link(target, filepath.Join(t.TempDir(), "exposed-copy")); err != nil {
				t.Skipf("hard links unavailable: %v", err)
			}
			err := PublishCommand().ParseAndRun(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "multiple hard links") {
				t.Fatalf("recovery error = %v, want hard-link rejection", err)
			}
			if storeCalls != 1 {
				t.Fatalf("object store calls = %d, want no recovery store call", storeCalls)
			}
		})
	}
}

func TestReverifyPrivatePublishRejectsHardLinkedArtifacts(t *testing.T) {
	for _, artifact := range []string{"receipt", "link"} {
		t.Run(artifact, func(t *testing.T) {
			originalLoad, originalReverify := loadPreparedBundle, reverifyPublication
			t.Cleanup(func() { loadPreparedBundle, reverifyPublication = originalLoad, originalReverify })
			bundleDir, bundle := privatePublishTestBundle(t)
			loadPreparedBundle = func(context.Context, rootfs.Root) (*distribution.PreparedBundle, error) { return bundle(), nil }
			networkCalled := false
			reverifyPublication = func(context.Context, distribution.Verifier, distribution.PublishReceipt, distribution.SensitiveLinks, time.Time) error {
				networkCalled = true
				return nil
			}
			stateDir := t.TempDir()
			receiptPath := filepath.Join(stateDir, "receipt.json")
			linkPath := filepath.Join(stateDir, "link.json")
			receipt := privatePublishTestReceipt()
			receipt.Endpoint = "https://objects.example.com"
			receipt.DownloadEndpoint = "https://objects.example.com"
			receipt.Region = "auto"
			receipt.AddressingStyle = "path"
			receipt.URLTTL = "24h0m0s"
			receipt.DownloadGrace = "1h0m0s"
			receipt.ReceiptPath = receiptPath
			receipt.LinkPath = linkPath
			writePrivatePublishTestState(t, receiptPath, linkPath, receipt, distribution.SensitiveLinks{
				SchemaVersion: "1",
				InstallURL:    "https://objects.example.com/bucket/app/page?X-Amz-Signature=hard-link-secret-canary",
			})
			target := receiptPath
			if artifact == "link" {
				target = linkPath
			}
			if err := os.Link(target, filepath.Join(t.TempDir(), "exposed-copy")); err != nil {
				t.Skipf("hard links unavailable: %v", err)
			}
			_, err := reverifyPrivatePublish(context.Background(), privatePublishVerificationRequest{
				BundleDir: bundleDir, ReceiptPath: receiptPath, LinkPath: linkPath, VerifyTimeout: time.Second,
			})
			if err == nil || !strings.Contains(err.Error(), "multiple hard links") {
				t.Fatalf("verification error = %v, want hard-link rejection", err)
			}
			if networkCalled {
				t.Fatal("hard-linked artifact reached network verification")
			}
		})
	}
}

func privatePublishTestBundle(t *testing.T) (string, func() *distribution.PreparedBundle) {
	t.Helper()
	dir := t.TempDir()
	ipaPath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("ipa"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, func() *distribution.PreparedBundle {
		file, err := os.Open(ipaPath)
		if err != nil {
			t.Fatal(err)
		}
		return &distribution.PreparedBundle{
			IPA: file, IPASHA256: "sha", IPASize: 3,
			Descriptor: distribution.PreparedDescriptor{App: distribution.PreparedApp{BundleID: "com.example", Version: "1", BuildNumber: "2"}},
		}
	}
}

func privatePublishTestReceipt() distribution.PublishReceipt {
	return distribution.PublishReceipt{
		SchemaVersion: "1", Access: distribution.AccessPrivate, Bucket: "bucket", Prefix: "app",
		Artifact:   distribution.StoredObject{SHA256: "sha", SizeBytes: 3},
		App:        distribution.PreparedApp{BundleID: "com.example", Version: "1", BuildNumber: "2"},
		InstallURL: "https://downloads.example.com/install?X-Amz-Signature=REDACTED", Verified: true,
	}
}

func writePrivatePublishTestState(t *testing.T, receiptPath, linkPath string, receipt distribution.PublishReceipt, links distribution.SensitiveLinks) {
	t.Helper()
	receiptData, err := encodeJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	stateData, err := encodeJSON(publishState{SchemaVersion: "1", Receipt: receipt, Links: links})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, receiptData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(linkPath, stateData, 0o600); err != nil {
		t.Fatal(err)
	}
}

func directoryEntryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
