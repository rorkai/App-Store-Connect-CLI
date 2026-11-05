package distribution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestPrepareIPAWritesDeterministicPrivateBundle(t *testing.T) {
	ipaPath := validIPA(t, []string{"secret-device-b", "secret-device-a"}, time.Now().Add(24*time.Hour), false)
	root := t.TempDir()
	result := preparePath(t, ipaPath, PrepareOptions{
		Root: root, Title: "Preview", Channel: "pull-request-42", SourceRevision: "abcdef", SourceURL: "https://example.com/revision/abcdef",
	})
	if result.Reused {
		t.Fatal("new bundle reported as reused")
	}
	wantSuffix := filepath.Join(".asc", "distribution", "com.example.demo", "1.0-1-"+result.Descriptor.Artifact.SHA256[:12])
	if !strings.HasSuffix(result.BundlePath, wantSuffix) {
		t.Fatalf("bundle path = %q, want suffix %q", result.BundlePath, wantSuffix)
	}
	data, err := os.ReadFile(filepath.Join(result.BundlePath, "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("secret-device")) || bytes.Contains(data, []byte(ipaPath)) {
		t.Fatalf("descriptor leaked private or absolute input data: %s", data)
	}
	var descriptor Descriptor
	if err := json.Unmarshal(data, &descriptor); err != nil {
		t.Fatal(err)
	}
	if descriptor.App.Title != "Preview" || descriptor.Source == nil || descriptor.Source.Channel != "pull-request-42" {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	if descriptor.Artifact.RelativePath != "payload/app.ipa" {
		t.Fatalf("artifact path = %q", descriptor.Artifact.RelativePath)
	}
	if bytes.Contains(data, []byte("preparation")) {
		t.Fatalf("descriptor persisted transient eligibility: %s", data)
	}
	copied, err := os.ReadFile(filepath.Join(result.BundlePath, "payload", "app.ipa"))
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(ipaPath)
	if !bytes.Equal(copied, original) {
		t.Fatal("copied IPA differs")
	}
}

func TestPrepareIPAReusesExactBundleWithoutChangingFiles(t *testing.T) {
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(24*time.Hour), false)
	root := t.TempDir()
	first := preparePath(t, ipaPath, PrepareOptions{Root: root})
	descriptorPath := filepath.Join(first.BundlePath, "bundle.json")
	before, err := os.Stat(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second := preparePath(t, ipaPath, PrepareOptions{Root: root})
	if !second.Reused || second.BundlePath != first.BundlePath {
		t.Fatalf("unexpected reuse: %#v", second)
	}
	after, err := os.Stat(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("reuse rewrote descriptor")
	}
}

func TestPrepareIPARefusesConflictingExistingBundle(t *testing.T) {
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(24*time.Hour), false)
	root := t.TempDir()
	first := preparePath(t, ipaPath, PrepareOptions{Root: root})
	descriptorPath := filepath.Join(first.BundlePath, "bundle.json")
	if err := os.WriteFile(descriptorPath, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, _ := os.Open(ipaPath)
	defer file.Close()
	info, _ := file.Stat()
	_, err := PrepareIPAContext(context.Background(), file, info.Size(), PrepareOptions{Root: root})
	if !errors.Is(err, ErrBundleConflict) {
		t.Fatalf("error = %v, want ErrBundleConflict", err)
	}
	got, _ := os.ReadFile(descriptorPath)
	if string(got) != "sentinel" {
		t.Fatalf("conflict overwritten: %q", got)
	}
}

func TestPrepareIPAPublishesFromStableSnapshot(t *testing.T) {
	installVerifiedPreparationForTest(t)
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(24*time.Hour), false)
	original, err := os.ReadFile(ipaPath)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(ipaPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	afterIPASnapshotForTest = func() {
		if _, writeErr := file.WriteAt(bytes.Repeat([]byte{0}, len(original)), 0); writeErr != nil {
			t.Errorf("mutate source after snapshot: %v", writeErr)
		}
	}
	t.Cleanup(func() { afterIPASnapshotForTest = nil })

	result, err := PrepareIPAContext(context.Background(), file, int64(len(original)), PrepareOptions{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("PrepareIPAContext() error = %v", err)
	}
	copied, err := os.ReadFile(filepath.Join(result.BundlePath, "payload", "app.ipa"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(copied, original) {
		t.Fatal("prepared payload did not use the stable pre-mutation snapshot")
	}
}

func TestPrepareIPAPinsOutputRootBeforeInspection(t *testing.T) {
	installVerifiedPreparationForTest(t)
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(time.Hour), false)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "output-root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	afterIPASnapshotForTest = func() {
		if err := os.Rename(root, root+"-original"); err != nil {
			t.Fatalf("rename selected output root: %v", err)
		}
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatalf("replace selected output root: %v", err)
		}
	}
	t.Cleanup(func() { afterIPASnapshotForTest = nil })

	file, err := os.Open(ipaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIPAContext(context.Background(), file, info.Size(), PrepareOptions{Root: root}); !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("PrepareIPAContext() error = %v, want ErrSymlink", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("PrepareIPAContext() wrote to replacement root: %#v", entries)
	}
}

func TestPrepareIPARejectsOutputParentSwappedToSymlink(t *testing.T) {
	installVerifiedPreparationForTest(t)
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(time.Hour), false)
	root := t.TempDir()
	outside := t.TempDir()
	parent := filepath.Join(root, "output")
	afterOutputParentsCreatedForTest = func() {
		if err := os.Rename(parent, parent+"-original"); err != nil {
			t.Errorf("rename output parent: %v", err)
			return
		}
		if err := os.Symlink(outside, parent); err != nil {
			t.Errorf("replace output parent with symlink: %v", err)
		}
	}
	t.Cleanup(func() { afterOutputParentsCreatedForTest = nil })

	file, err := os.Open(ipaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIPAContext(context.Background(), file, info.Size(), PrepareOptions{Root: root, OutputDir: "output/bundle"}); err == nil {
		t.Fatal("expected swapped output parent rejection")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("wrote through swapped output symlink: %#v", entries)
	}
}

func TestPrepareIPACreatesEmptyOutputRootBeforeInspection(t *testing.T) {
	ipaPath := writeIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistBytes(t, map[string]any{
			"CFBundleIdentifier":         "com.example.demo",
			"CFBundleName":               "Demo",
			"CFBundleShortVersionString": "1.0",
			"CFBundleVersion":            "1",
			"DTPlatformName":             "xros",
			"CFBundleSupportedPlatforms": []string{"XROS"},
		}),
	})
	rootPath := filepath.Join(t.TempDir(), "missing")
	inspected := false
	afterIPASnapshotForTest = func() { inspected = true }
	t.Cleanup(func() { afterIPASnapshotForTest = nil })

	file, err := os.Open(ipaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIPAContext(context.Background(), file, info.Size(), PrepareOptions{Root: rootPath}); err == nil {
		t.Fatal("PrepareIPAContext() accepted unsupported archive platform")
	}
	if !inspected {
		t.Fatal("PrepareIPAContext() did not create and pin the output root before inspection")
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("PrepareIPAContext() published output after inspection failure: %#v", entries)
	}
}

func TestPrepareIPARejectsIneligibleAndCredentialURLBeforeWriting(t *testing.T) {
	unsupportedPlatformIPA := writeIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistBytes(t, map[string]any{
			"CFBundleIdentifier":         "com.example.demo",
			"CFBundleName":               "Demo",
			"CFBundleShortVersionString": "1.0",
			"CFBundleVersion":            "1",
			"DTPlatformName":             "xros",
			"CFBundleSupportedPlatforms": []string{"XROS"},
		}),
	})
	tests := []struct {
		name string
		path string
		opts PrepareOptions
	}{
		{name: "development", path: validIPA(t, []string{"one"}, time.Now().Add(time.Hour), true), opts: PrepareOptions{}},
		{name: "expired", path: validIPA(t, []string{"one"}, time.Now().Add(-time.Hour), false), opts: PrepareOptions{}},
		{name: "credential URL", path: validIPA(t, []string{"one"}, time.Now().Add(time.Hour), false), opts: PrepareOptions{SourceURL: "https://token@example.com/revision"}},
		{name: "unsupported archive platform", path: unsupportedPlatformIPA, opts: PrepareOptions{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.opts.Root = root
			file, _ := os.Open(test.path)
			defer file.Close()
			info, _ := file.Stat()
			if _, err := PrepareIPAContext(context.Background(), file, info.Size(), test.opts); err == nil {
				t.Fatal("expected error")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Fatalf("wrote before validation: %#v", entries)
			}
		})
	}
}

func TestPrepareIPARejectsUnverifiedProfileTrustAndCodeBeforeWriting(t *testing.T) {
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(time.Hour), false)
	root := t.TempDir()
	file, err := os.Open(ipaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIPAContext(context.Background(), file, info.Size(), PrepareOptions{Root: root}); !errors.Is(err, ErrNotEligible) {
		t.Fatalf("PrepareIPAContext() error = %v, want ErrNotEligible", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unverified IPA wrote output: %#v", entries)
	}
}

func TestPrepareIPARejectsSymlinkedDefaultOutputParent(t *testing.T) {
	ipaPath := validIPA(t, []string{"one"}, time.Now().Add(time.Hour), false)
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".asc")); err != nil {
		t.Fatal(err)
	}
	file, _ := os.Open(ipaPath)
	defer file.Close()
	info, _ := file.Stat()
	if _, err := PrepareIPAContext(context.Background(), file, info.Size(), PrepareOptions{Root: root}); err == nil {
		t.Fatal("expected symlinked .asc rejection")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("wrote through symlink: %#v", entries)
	}
}

func TestValidatePrepareOptionsRejectsControlMetadata(t *testing.T) {
	if err := ValidatePrepareOptions(PrepareOptions{Channel: "safe\nsecret"}); err == nil {
		t.Fatal("expected control character rejection")
	}
	if err := ValidatePrepareOptions(PrepareOptions{Channel: "safe\u202Esecret"}); err == nil {
		t.Fatal("expected bidi format control rejection")
	}
	if err := ValidatePrepareOptions(PrepareOptions{SourceURL: "https://example.com/safe\u202Esecret"}); err == nil {
		t.Fatal("expected source URL bidi format control rejection")
	}
	if err := ValidatePrepareOptions(PrepareOptions{SourceURL: "https://:443/path"}); err == nil {
		t.Fatal("expected empty source URL hostname rejection")
	}
}

func TestSafePathComponentIsCollisionSafeAndContained(t *testing.T) {
	values := []string{"..", "a/b", "a-b", "a\\b", "x\x00y", "é"}
	seen := map[string]bool{}
	for _, value := range values {
		got, err := safePathComponent(value)
		if err != nil {
			t.Fatalf("safePathComponent(%q): %v", value, err)
		}
		if got == "." || got == ".." || strings.ContainsAny(got, `/\\`) {
			t.Fatalf("unsafe result %q", got)
		}
		if seen[got] {
			t.Fatalf("collision for %q: %q", value, got)
		}
		seen[got] = true
	}
}

func TestPrepareIPAContextAlreadyCanceledHasNoFilesystemSideEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-created")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PrepareIPAContext(ctx, nil, 0, PrepareOptions{Root: root}); !errors.Is(err, context.Canceled) {
		t.Fatalf("PrepareIPAContext() error = %v, want context.Canceled", err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output root stat error = %v, want not found", err)
	}
}

func TestPrepareIPAContextCancellationDuringPublicationDoesNotPublish(t *testing.T) {
	installVerifiedPreparationForTest(t)
	path := validIPA(t, []string{"one"}, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), false)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	duringPublicationCopyForTest = cancel
	t.Cleanup(func() { duringPublicationCopyForTest = nil })
	if _, err := PrepareIPAContext(ctx, file, info.Size(), PrepareOptions{Root: root}); !errors.Is(err, context.Canceled) {
		t.Fatalf("PrepareIPAContext() error = %v, want context.Canceled", err)
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && (entry.Name() == "bundle.json" || entry.Name() == "app.ipa") {
			t.Errorf("canceled preparation left publication file %q", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func preparePath(t *testing.T, path string, options PrepareOptions) PrepareResult {
	t.Helper()
	installVerifiedPreparationForTest(t)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	result, err := PrepareIPAContext(context.Background(), file, info.Size(), options)
	if err != nil {
		t.Fatalf("PrepareIPAContext() error = %v", err)
	}
	return result
}

func installVerifiedPreparationForTest(t *testing.T) {
	t.Helper()
	verifyCompleteSigningForTest = func(inspection *Inspection) {
		certificate := strings.Repeat("a", 64)
		inspection.Signing.ProfileIntegrityVerification.Status = CodeSignatureVerified
		inspection.Signing.ProfileTrustVerification.Status = CodeSignatureVerified
		inspection.Signing.CodeSignatureVerification.Status = CodeSignatureVerified
		inspection.Signing.CodeSignatureVerification.Scope = mainCodeSignatureScope
		inspection.Signing.ProfileCertificateSHA256Fingerprints = []string{certificate}
		inspection.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints = []string{certificate}
	}
	t.Cleanup(func() { verifyCompleteSigningForTest = nil })
}
