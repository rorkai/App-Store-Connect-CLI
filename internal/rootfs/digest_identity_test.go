package rootfs

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRemoveFileIfSHA256SameLargeBoundedMemory(t *testing.T) {
	requireStrictIdentityPlatform(t)
	root := mustRoot(t, t.TempDir())
	defer root.Close()
	path := filepath.Join(root.Path(), "preview.mp4")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	const size int64 = 500_000_000
	if err := file.Truncate(size); err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	file.Close()
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := root.RemoveFileIfSHA256Same("preview.mp4", size, digest); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 16<<20 {
		t.Fatalf("removal allocated %d bytes for500MB file", allocated)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file remains: %v", err)
	}
	if len(root.selectedIdentity.retainedFiles) != 0 {
		t.Fatal("removal retained file descriptors")
	}
}

func TestRemoveFileIfSHA256SamePreservesMismatchesAndRaces(t *testing.T) {
	for _, scenario := range []string{"size", "digest", "capture-edit", "replacement", "quarantine-edit"} {
		t.Run(scenario, func(t *testing.T) {
			requireStrictIdentityPlatform(t)
			root := mustRoot(t, t.TempDir())
			defer root.Close()
			path := filepath.Join(root.Path(), "asset")
			original := []byte("original")
			digest := sha256.Sum256(original)
			size := int64(len(original))
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			quarantine := ""
			switch scenario {
			case "size":
				size++
			case "digest":
				digest[0]++
			case "capture-edit":
				root.afterIdentityCaptureReadForTest = func() {
					if err := os.WriteFile(path, []byte("modified"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case "replacement":
				root.beforeConditionalQuarantineForTest = func(parent *os.Root, name string) {
					if err := parent.Rename(name, "saved-original"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, original, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case "quarantine-edit":
				root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, name string) {
					quarantine = filepath.Join(root.Path(), name)
					info, err := parent.Stat(name)
					if err != nil {
						t.Fatal(err)
					}
					file, err := parent.OpenFile(name, os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := file.WriteAt([]byte("modified"), 0); err != nil {
						t.Fatal(err)
					}
					file.Close()
					// Restore mtime so the final digest, not only metadata, must detect this edit.
					if err := os.Chtimes(quarantine, info.ModTime(), info.ModTime()); err != nil {
						t.Fatal(err)
					}
				}
			}
			err := root.RemoveFileIfSHA256Same("asset", size, digest)
			if err == nil {
				t.Fatal("unsafe removal succeeded")
			}
			if scenario == "quarantine-edit" {
				if !errors.Is(err, ErrQuarantineCleanupUncertain) {
					t.Fatalf("want quarantine evidence, got %v", err)
				}
				if got := mustRead(t, quarantine); got != "modified" {
					t.Fatalf("quarantine content %q", got)
				}
			} else {
				if !errors.Is(err, ErrFileIdentityChanged) {
					t.Fatalf("want identity change, got %v", err)
				}
				want := string(original)
				if scenario == "capture-edit" {
					want = "modified"
				}
				if got := mustRead(t, path); got != want {
					t.Fatalf("preserved content %q, want %q", got, want)
				}
			}
			if len(root.selectedIdentity.retainedFiles) != 0 {
				t.Fatal("failed removal leaked retained file")
			}
		})
	}
}

func TestRemoveFileIfSHA256SameWindowsUnsupported(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows contract")
	}
	root := mustRoot(t, t.TempDir())
	defer root.Close()
	if err := root.WriteFile("asset", []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.RemoveFileIfSHA256Same("asset", 8, sha256.Sum256([]byte("original"))); !errors.Is(err, ErrFileIdentityMutationUnsupported) {
		t.Fatalf("got %v", err)
	}
	if got := mustRead(t, filepath.Join(root.Path(), "asset")); got != "original" {
		t.Fatalf("content %q", got)
	}
}

func TestReleaseFileIdentityLifecycle(t *testing.T) {
	requireStrictIdentityPlatform(t)
	root := mustRoot(t, t.TempDir())
	defer root.Close()
	other := mustRoot(t, t.TempDir())
	defer other.Close()
	if err := root.WriteFile("inventory.json", []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := root.CaptureFile("inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := other.ReleaseFileIdentity(previous); !errors.Is(err, ErrFileIdentityMismatch) {
		t.Fatalf("foreign release: %v", err)
	}
	current, err := root.ReplaceFileIfSame("inventory.json", previous, []byte("second"), 0o600, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.ReleaseFileIdentity(previous); err != nil {
		t.Fatal(err)
	}
	if err := root.ReleaseFileIdentity(previous); err != nil {
		t.Fatalf("double release: %v", err)
	}
	if len(root.selectedIdentity.retainedFiles) != 1 {
		t.Fatalf("retained descriptors=%d", len(root.selectedIdentity.retainedFiles))
	}
	if err := root.CheckFileIdentity("inventory.json", current); err != nil {
		t.Fatalf("current token lost: %v", err)
	}
	if _, err := root.ReplaceFileIfSame("inventory.json", previous, []byte("unsafe"), 0o600, true); err == nil {
		t.Fatal("released token reused")
	}
	if got := mustRead(t, filepath.Join(root.Path(), "inventory.json")); got != "second" {
		t.Fatalf("current bytes=%q", got)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.ReleaseFileIdentity(current); !errors.Is(err, ErrFileIdentityClosed) {
		t.Fatalf("release after root close: %v", err)
	}
}
