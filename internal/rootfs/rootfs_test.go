package rootfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestValidateRelativeRejectsEscapes(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"unix absolute", "/etc/passwd"},
		{"windows absolute backslash", `\Windows\System32`},
		{"windows drive absolute", `C:\Windows\System32`},
		{"windows drive relative", "C:evil.txt"},
		{"unc share", `\\attacker\share\evil.txt`},
		{"parent traversal", "../evil.txt"},
		{"nested parent traversal", "a/../../evil.txt"},
		{"bare parent", ".."},
		{"backslash parent traversal", `..\evil.txt`},
		{"mixed separator traversal", `a\..\..\evil.txt`},
		{"nul byte", "a\x00b"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := ValidateRelative(testCase.path); err == nil {
				t.Fatalf("ValidateRelative(%q) = nil, want error", testCase.path)
			}
		})
	}
}

func TestValidateRelativeAcceptsOrdinaryPaths(t *testing.T) {
	cases := []string{
		"file.txt",
		"metadata/en-US/description.txt",
		"./file.txt",
		"a..b/c.txt",
		"...hidden",
	}

	for _, path := range cases {
		if err := ValidateRelative(path); err != nil {
			t.Fatalf("ValidateRelative(%q) error = %v, want nil", path, err)
		}
	}
}

func TestResolveRejectsEscapingAbsolutePath(t *testing.T) {
	root := mustRoot(t, t.TempDir())
	outside := filepath.Join(t.TempDir(), "sentinel.txt")

	if _, err := root.Resolve(outside); !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("Resolve(%q) error = %v, want ErrEscapesRoot", outside, err)
	}
}

func TestResolveAcceptsAbsolutePathInsideRoot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	inside := filepath.Join(dir, "nested", "file.txt")

	resolved, err := root.Resolve(inside)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", inside, err)
	}
	if resolved != filepath.Clean(inside) {
		t.Fatalf("Resolve(%q) = %q, want %q", inside, resolved, filepath.Clean(inside))
	}
}

func TestRootPreservesWhitespaceInValidPathNames(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "trusted ")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	root := mustRoot(t, dir)
	if root.Path() != dir {
		t.Fatalf("Path() = %q, want exact path %q", root.Path(), dir)
	}
	if err := root.WriteFile("report ", []byte("exact"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "report ")); got != "exact" {
		t.Fatalf("content = %q, want exact whitespace-bearing destination", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "report")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("trimmed sibling exists or returned unexpected error: %v", err)
	}
	if err := root.WriteFile("   ", []byte("all spaces"), 0o600); err != nil {
		t.Fatalf("WriteFile(all-spaces) error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "   ")); got != "all spaces" {
		t.Fatalf("all-space filename content = %q", got)
	}

	allSpaceDir := filepath.Join(parent, "   ")
	if err := os.Mkdir(allSpaceDir, 0o755); err != nil {
		t.Fatalf("Mkdir(all-space root) error = %v", err)
	}
	allSpaceRoot := mustRoot(t, allSpaceDir)
	if allSpaceRoot.Path() != allSpaceDir {
		t.Fatalf("all-space Path() = %q, want %q", allSpaceRoot.Path(), allSpaceDir)
	}
}

func TestReadFileRefusesSymlinkedFinalComponent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	mustWrite(t, secretPath, "top-secret")

	linkPath := filepath.Join(dir, "description.txt")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if _, err := root.ReadFile("description.txt"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ReadFile() error = %v, want ErrSymlink", err)
	}
}

func TestResolveContainedFinalSymlinkAcceptsOnlyInRootTarget(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "actual.md"), "inside")
	if err := os.Symlink("actual.md", filepath.Join(dir, "ASC.md")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	root := mustRoot(t, dir)
	resolved, err := root.ResolveContainedFinalSymlink("ASC.md")
	if err != nil {
		t.Fatalf("ResolveContainedFinalSymlink() error = %v", err)
	}
	if resolved != "actual.md" {
		t.Fatalf("ResolveContainedFinalSymlink() = %q, want actual.md", resolved)
	}

	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(dir, "external.md")); err != nil {
		t.Fatalf("Symlink(external) error = %v", err)
	}
	if _, err := root.ResolveContainedFinalSymlink("external.md"); !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("external ResolveContainedFinalSymlink() error = %v, want ErrEscapesRoot", err)
	}
}

func TestReadFileRefusesSymlinkedParentComponent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	secretDir := t.TempDir()
	mustWrite(t, filepath.Join(secretDir, "secret.txt"), "top-secret")

	if err := os.Symlink(secretDir, filepath.Join(dir, "en-US")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if _, err := root.ReadFile(filepath.Join("en-US", "secret.txt")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ReadFile() error = %v, want ErrSymlink", err)
	}
}

func TestOpenFileRefusesSymlinkedParentComponent(t *testing.T) {
	requireSymlinks(t)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(temp dir) error = %v", err)
	}
	realDir := filepath.Join(dir, "real")
	mustWrite(t, filepath.Join(realDir, "devices.json"), `{"schemaVersion":1}`)
	if err := os.Symlink(realDir, filepath.Join(dir, "linked")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := OpenFile(filepath.Join(dir, "linked", "devices.json")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenFile() error = %v, want ErrSymlink", err)
	}
}

func TestReadFileReadsOrdinaryFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "keywords.txt"), "one,two")

	root := mustRoot(t, dir)
	data, err := root.ReadFile("keywords.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "one,two" {
		t.Fatalf("ReadFile() = %q, want %q", data, "one,two")
	}
}

func TestReadFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	mustWrite(t, filepath.Join(nested, "report.txt"), "trusted")
	external := t.TempDir()
	mustWrite(t, filepath.Join(external, "report.txt"), "external-secret")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	data, err := root.ReadFile(filepath.Join("nested", "report.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "trusted" {
		t.Fatalf("ReadFile() = %q, want the file anchored before the parent swap", data)
	}
}

func TestReadFileRejectsFinalSymlinkSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	report := filepath.Join(dir, "report.txt")
	mustWrite(t, report, "trusted")
	mustWrite(t, filepath.Join(dir, "secret.txt"), "in-root-secret")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Remove(report); err != nil {
			t.Fatalf("remove validated file: %v", err)
		}
		if err := os.Symlink("secret.txt", report); err != nil {
			t.Fatalf("replace validated file with symlink: %v", err)
		}
	}

	data, err := root.ReadFile("report.txt")
	if err == nil {
		t.Fatalf("ReadFile() returned %q through a swapped final symlink, want an error", data)
	}
	if string(data) == "in-root-secret" {
		t.Fatal("ReadFile() disclosed the swapped symlink target")
	}
}

func TestReadFileOptionalReportsMissingWithoutError(t *testing.T) {
	root := mustRoot(t, t.TempDir())

	data, found, err := root.ReadFileOptional("missing.txt")
	if err != nil {
		t.Fatalf("ReadFileOptional() error = %v", err)
	}
	if found {
		t.Fatal("ReadFileOptional() found = true, want false")
	}
	if len(data) != 0 {
		t.Fatalf("ReadFileOptional() data = %q, want empty", data)
	}
}

func TestWriteFileRefusesFinalSymlink(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")

	if err := os.Symlink(sentinelPath, filepath.Join(dir, "out.json")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.WriteFile("out.json", []byte("attacker"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestWriteFileRefusesSymlinkedParent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")

	if err := os.Symlink(sentinelDir, filepath.Join(dir, "nested")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	err := root.WriteFile(filepath.Join("nested", "sentinel.txt"), []byte("attacker"), 0o600)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestWriteFileCreatesAndReplacesInRoot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	target := filepath.Join("nested", "deep", "out.json")
	if err := root.WriteFile(target, []byte("first"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, target)); got != "first" {
		t.Fatalf("content = %q, want %q", got, "first")
	}
	if err := root.WriteFile(target, []byte("second"), 0o600); err != nil {
		t.Fatalf("WriteFile() replace error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, target)); got != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}

	info, err := os.Lstat(filepath.Join(dir, target))
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	if leftovers := temporaryLeftovers(t, filepath.Join(dir, "nested", "deep")); len(leftovers) > 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}

func TestWriteFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	external := t.TempDir()

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	err := root.WriteFile(filepath.Join("nested", "out.json"), []byte("attacker"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(external, "out.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("WriteFile() created a file outside the trusted root: %v", statErr)
	}
	if got := mustRead(t, filepath.Join(dir, "nested-original", "out.json")); got != "attacker" {
		t.Fatalf("WriteFile() content in anchored parent = %q", got)
	}
}

func TestWriteFileDoesNotUsePredictableTemporaryPath(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")

	// A predictable "<target>.tmp" staging path lets a lower-trust checkout
	// pre-create a symlink that redirects the staged write.
	if err := os.Symlink(sentinelPath, filepath.Join(dir, "checkpoint.json.tmp")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.WriteFile("checkpoint.json", []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
	if got := mustRead(t, filepath.Join(dir, "checkpoint.json")); got != `{"ok":true}` {
		t.Fatalf("checkpoint content = %q", got)
	}
}

func TestWriteFromWritesReaderContents(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	written, err := root.WriteFrom("payload.bin", bytes.NewReader([]byte("payload")), 0o600)
	if err != nil {
		t.Fatalf("WriteFrom() error = %v", err)
	}
	if written != int64(len("payload")) {
		t.Fatalf("WriteFrom() = %d, want %d", written, len("payload"))
	}
	if got := mustRead(t, filepath.Join(dir, "payload.bin")); got != "payload" {
		t.Fatalf("content = %q", got)
	}
}

func TestWriteFilePreservingModeKeepsExistingModeAndDefaultsForNew(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	existingPath := filepath.Join(dir, "existing.txt")
	mustWrite(t, existingPath, "old")
	if err := os.Chmod(existingPath, 0o600); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if err := root.WriteFilePreservingMode("existing.txt", []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFilePreservingMode() error = %v", err)
	}
	info, err := os.Lstat(existingPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("existing mode = %v, want preserved 0600", info.Mode().Perm())
	}
	if got := mustRead(t, existingPath); got != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}

	if err := root.WriteFilePreservingMode("fresh.txt", []byte("data"), 0o640); err != nil {
		t.Fatalf("WriteFilePreservingMode() new file error = %v", err)
	}
	freshInfo, err := os.Lstat(filepath.Join(dir, "fresh.txt"))
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && freshInfo.Mode().Perm() != 0o640 {
		t.Fatalf("new mode = %v, want default 0640", freshInfo.Mode().Perm())
	}
}

func TestWriteFilePreservingModeRefusesReadOnlyExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not authoritative on Windows")
	}

	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "read-only.txt")
	mustWrite(t, path, "original")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	if err := root.WriteFilePreservingMode("read-only.txt", []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFilePreservingMode() error = nil, want read-only destination refusal")
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("content = %q, want original", got)
	}
}

func TestWriteFilePreservingModeRefusesMultiplyLinkedFile(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "existing.txt")
	linkPath := filepath.Join(dir, "linked.txt")
	mustWrite(t, path, "original")
	if err := os.Link(path, linkPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if err := root.WriteFilePreservingMode("existing.txt", []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFilePreservingMode() error = nil, want multiply-linked file refusal")
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("destination content = %q, want original", got)
	}
	if got := mustRead(t, linkPath); got != "original" {
		t.Fatalf("hard-link content = %q, want original", got)
	}
}

func TestWriteFilePreservingModeDoesNotMutateHardLinkAddedAfterValidation(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "existing.txt")
	externalLink := filepath.Join(t.TempDir(), "external.txt")
	mustWrite(t, path, "original")
	root.afterValidationForTest = func() {
		if err := os.Link(path, externalLink); err != nil {
			t.Skipf("hard links unavailable: %v", err)
		}
	}

	if err := root.WriteFilePreservingMode("existing.txt", []byte("replacement"), 0o644); err != nil {
		t.Fatalf("WriteFilePreservingMode() error = %v", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("destination content = %q, want replacement", got)
	}
	if got := mustRead(t, externalLink); got != "original" {
		t.Fatalf("late hard link content = %q, want original", got)
	}
}

func TestWriteFilePreservingModeCreatesMissingIntermediateDirectories(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	name := filepath.Join("nested", "deep", "fresh.txt")

	if err := root.WriteFilePreservingMode(name, []byte("data"), 0o640); err != nil {
		t.Fatalf("WriteFilePreservingMode() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, name)); got != "data" {
		t.Fatalf("content = %q, want %q", got, "data")
	}
}

func TestCreateNewFileRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AuthKey.p8"), "existing")

	root := mustRoot(t, dir)
	if err := root.CreateNewFile("AuthKey.p8", []byte("new"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("CreateNewFile() error = %v, want os.ErrExist", err)
	}
	if got := mustRead(t, filepath.Join(dir, "AuthKey.p8")); got != "existing" {
		t.Fatalf("content = %q, want %q", got, "existing")
	}
}

func TestCreateNewFileAtomicRefusesRacingDestination(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		mustWrite(t, filepath.Join(dir, "plan.json"), "racing")
	}

	err := root.CreateNewFileAtomic("plan.json", []byte("planned"), 0o600)
	if err == nil {
		t.Fatal("CreateNewFileAtomic() replaced a racing destination")
	}
	if got := mustRead(t, filepath.Join(dir, "plan.json")); got != "racing" {
		t.Fatalf("content = %q, want racing destination preserved", got)
	}
}

func TestCreateNewFileAtomicWritesExactMode(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := root.CreateNewFileAtomic("plan.json", []byte("planned"), 0o600); err != nil {
		t.Fatalf("CreateNewFileAtomic() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestCreateNewFileFallsBackWhenAtomicRenameIsUnsupported(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}

	if err := root.CreateNewFile("AuthKey.p8", []byte("complete"), 0o600); err != nil {
		t.Fatalf("CreateNewFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "AuthKey.p8")); got != "complete" {
		t.Fatalf("content = %q, want complete", got)
	}
	if err := root.CreateNewFile("AuthKey.p8", []byte("replacement"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite error = %v, want os.ErrExist", err)
	}
	if got := mustRead(t, filepath.Join(dir, "AuthKey.p8")); got != "complete" {
		t.Fatalf("existing content = %q, want complete", got)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after fallback: %v", matches)
	}
}

func TestCreateNewFileAtomicRejectsUnsupportedRenameWithoutOutput(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}

	err := root.CreateNewFileAtomic("identity.p12.enc", []byte("complete"), 0o600)
	if !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		t.Fatalf("CreateNewFileAtomic() error = %v, want unsupported rename", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "identity.p12.enc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial destination exists: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after strict failure: %v", matches)
	}

	mustWrite(t, filepath.Join(dir, "identity.p12.enc"), "existing")
	if err := root.CreateNewFileAtomic("identity.p12.enc", []byte("replacement"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite error = %v, want os.ErrExist", err)
	}
	if got := mustRead(t, filepath.Join(dir, "identity.p12.enc")); got != "existing" {
		t.Fatalf("existing content = %q, want existing", got)
	}
}

func TestCreateNewFromWriteFailureLeavesNoDestination(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	wantErr := errors.New("simulated reader failure")
	reader := io.MultiReader(strings.NewReader("partial"), errorReader{err: wantErr})

	if _, err := root.CreateNewFrom("identity.p12", reader, 0o600); !errors.Is(err, wantErr) {
		t.Fatalf("CreateNewFrom() error = %v, want %v", err, wantErr)
	}
	if _, err := os.Lstat(filepath.Join(dir, "identity.p12")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial destination exists after failure: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after failure: %v", matches)
	}
}

func TestReadFileLimitedRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "artifact.enc"), "12345")
	root := mustRoot(t, dir)

	if _, err := root.ReadFileLimited("artifact.enc", 4); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("ReadFileLimited() error = %v, want size limit", err)
	}
}

func TestReadFileLimitedMaxIntDoesNotOverflow(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "artifact.enc"), "complete")
	root := mustRoot(t, dir)
	data, err := root.ReadFileLimited("artifact.enc", math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "complete" {
		t.Fatalf("data = %q", data)
	}
}

func TestCreateNewFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	external := t.TempDir()

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	err := root.CreateNewFile(filepath.Join("nested", "AuthKey.p8"), []byte("private"), 0o600)
	if err != nil {
		t.Fatalf("CreateNewFile() error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(external, "AuthKey.p8")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("CreateNewFile() created a file outside the trusted root: %v", statErr)
	}
	if got := mustRead(t, filepath.Join(dir, "nested-original", "AuthKey.p8")); got != "private" {
		t.Fatalf("CreateNewFile() content in anchored parent = %q", got)
	}
}

func TestMkdirAllRefusesSymlinkedComponent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "metadata")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	err := root.MkdirAll(filepath.Join("metadata", "en-US"), 0o755)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("MkdirAll() error = %v, want ErrSymlink", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "en-US")); statErr == nil {
		t.Fatal("MkdirAll() created a directory outside the root")
	}
}

func TestMkdirAllCreatesNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	if err := root.MkdirAll(filepath.Join("metadata", "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "metadata", "en-US"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("MkdirAll() did not create a directory")
	}
}

func TestMkdirAllCreatesMissingRoot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "created-root")
	root := mustRoot(t, dir)

	if err := root.MkdirAll(".", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("Stat(root) = %v, %v", info, err)
	}
}

func TestMkdirAllCreatesNestedMissingRootBelowSymlinkedAncestor(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	physical := filepath.Join(parent, "physical")
	if err := os.Mkdir(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(parent, "selected")
	if err := os.Symlink(physical, selected); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(selected, "missing", "root")
	root := mustRoot(t, directory)

	if err := root.WriteFile("sentinel", []byte("selected"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(physical, "missing", "root", "sentinel")); got != "selected" {
		t.Fatalf("nested missing root content = %q", got)
	}
}

func TestRootedWriteRejectsMissingRootAncestorReplacedBeforeCreation(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(parent, "physical")
	if err := os.Mkdir(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(parent, "selected")
	if err := os.Symlink(physical, selected); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(selected, "missing", "root")
	root := mustRoot(t, directory)
	root.beforeCreateRootForTest = func() {
		if err := os.Rename(physical, physical+"-original"); err != nil {
			t.Fatalf("rename selected ancestor: %v", err)
		}
		if err := os.Mkdir(physical, 0o755); err != nil {
			t.Fatalf("replace selected ancestor: %v", err)
		}
	}

	if err := root.WriteFile("escaped", []byte("unsafe"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if _, err := os.Stat(filepath.Join(physical, "missing", "root", "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached replacement ancestor: %v", err)
	}
}

func TestAppendFileRefusesSymlinkAndLeavesModeIntact(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")
	if err := os.Chmod(sentinelPath, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	if err := os.Symlink(sentinelPath, filepath.Join(dir, "snitch.log")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.AppendFile("snitch.log", []byte("entry\n"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("AppendFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
	info, err := os.Lstat(sentinelPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("sentinel mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestAppendFileAppendsInRoot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	if err := root.AppendFile("snitch.log", []byte("one\n"), 0o600); err != nil {
		t.Fatalf("AppendFile() error = %v", err)
	}
	if err := root.AppendFile("snitch.log", []byte("two\n"), 0o600); err != nil {
		t.Fatalf("AppendFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "snitch.log")); got != "one\ntwo\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestAppendFileRefusesMultiplyLinkedFile(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	externalPath := filepath.Join(t.TempDir(), "external.log")
	mustWrite(t, externalPath, "external\n")
	if err := os.Chmod(externalPath, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	linkedPath := filepath.Join(dir, "snitch.log")
	if err := os.Link(externalPath, linkedPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	if err := root.AppendFile("snitch.log", []byte("entry\n"), 0o600); err == nil {
		t.Fatal("AppendFile() error = nil, want multiply-linked file refusal")
	}
	if got := mustRead(t, externalPath); got != "external\n" {
		t.Fatalf("external hard-link content = %q, want unchanged", got)
	}
	if got := mustRead(t, linkedPath); got != "external\n" {
		t.Fatalf("rooted hard-link content = %q, want unchanged", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Lstat(externalPath)
		if err != nil {
			t.Fatalf("Lstat() error = %v", err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("external hard-link mode = %v, want unchanged 0644", info.Mode().Perm())
		}
	}
}

func TestAppendFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	mustWrite(t, filepath.Join(nested, "snitch.log"), "trusted\n")
	external := t.TempDir()
	externalLog := filepath.Join(external, "snitch.log")
	mustWrite(t, externalLog, "external\n")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	err := root.AppendFile(filepath.Join("nested", "snitch.log"), []byte("attacker\n"), 0o600)
	if err != nil {
		t.Fatalf("AppendFile() error = %v", err)
	}
	if got := mustRead(t, externalLog); got != "external\n" {
		t.Fatalf("AppendFile() modified a file outside the trusted root: %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "nested-original", "snitch.log")); got != "trusted\nattacker\n" {
		t.Fatalf("AppendFile() content in anchored parent = %q", got)
	}
}

func TestAppendFileRejectsFinalSymlinkSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "snitch.log")
	mustWrite(t, logPath, "trusted\n")
	siblingPath := filepath.Join(dir, "sibling.log")
	mustWrite(t, siblingPath, "sibling\n")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Remove(logPath); err != nil {
			t.Fatalf("remove validated file: %v", err)
		}
		if err := os.Symlink("sibling.log", logPath); err != nil {
			t.Fatalf("replace validated file with symlink: %v", err)
		}
	}

	err := root.AppendFile("snitch.log", []byte("attacker\n"), 0o600)
	if err == nil {
		t.Fatal("AppendFile() succeeded through a swapped final symlink, want an error")
	}
	if got := mustRead(t, siblingPath); got != "sibling\n" {
		t.Fatalf("AppendFile() modified the swapped symlink target: %q", got)
	}
}

func TestAllowingInternalSymlinksAcceptsContainedSymlinkedParent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	realDir := filepath.Join(dir, "SharedGenerated")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir).AllowingInternalSymlinks()
	if err := root.WriteFile(filepath.Join("Generated", "Info.plist"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(realDir, "Info.plist")); got != "payload" {
		t.Fatalf("content = %q", got)
	}
}

func TestAllowingInternalSymlinksRejectsEscapingSymlinkedParent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir).AllowingInternalSymlinks()
	err := root.WriteFile(filepath.Join("Generated", "Info.plist"), []byte("payload"), 0o644)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if _, statErr := os.Stat(filepath.Join(external, "Info.plist")); statErr == nil {
		t.Fatal("WriteFile() escaped through a symlinked parent")
	}
}

func TestAllowingInternalSymlinksStillRejectsFinalSymlink(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	mustWrite(t, sentinelPath, "original")
	if err := os.Symlink(sentinelPath, filepath.Join(dir, "Info.plist")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir).AllowingInternalSymlinks()
	if err := root.WriteFile("Info.plist", []byte("payload"), 0o644); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestCheckContainedRejectsSymlinkedParentForExternalCandidate(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "Configs")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	mustWrite(t, filepath.Join(outside, "Shared.xcconfig"), "MARKETING_VERSION = 1.0.0\n")

	root := mustRoot(t, dir)
	err := root.CheckContained(filepath.Join(dir, "Configs", "Shared.xcconfig"))
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("CheckContained() error = %v, want ErrSymlink", err)
	}
}

func TestErrorMessagesIdentifyRejectedPath(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	_, err := root.Resolve("../escape.txt")
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "../escape.txt") {
		t.Fatalf("error %q does not identify the rejected path", err)
	}
}

func TestOpenAbsoluteRootNoFollowUsesCurrentDirectoryAnchor(t *testing.T) {
	workingDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(workingDir, "selected")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}

	openWorkingDir := func() (*os.Root, error) {
		return os.OpenRoot(workingDir)
	}
	denyVolumeRoot := func(string) (*os.Root, error) {
		return nil, os.ErrPermission
	}

	opened, err := openAbsoluteRootNoFollowFrom(
		selected,
		workingDir,
		openWorkingDir,
		denyVolumeRoot,
	)
	if err != nil {
		t.Fatalf("openAbsoluteRootNoFollowFrom() error = %v", err)
	}
	defer opened.Close()
	if _, err := opened.Stat("."); err != nil {
		t.Fatalf("opened selected root is unusable: %v", err)
	}
}

func TestOpenAbsoluteRootNoFollowFailsClosedOutsideCurrentDirectory(t *testing.T) {
	workingDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	openWorkingDir := func() (*os.Root, error) {
		return os.OpenRoot(workingDir)
	}
	denyVolumeRoot := func(string) (*os.Root, error) {
		return nil, os.ErrPermission
	}

	opened, err := openAbsoluteRootNoFollowFrom(
		outside,
		workingDir,
		openWorkingDir,
		denyVolumeRoot,
	)
	if opened != nil {
		_ = opened.Close()
		t.Fatal("openAbsoluteRootNoFollowFrom() opened a root outside the current directory")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("openAbsoluteRootNoFollowFrom() error = %v, want permission denied", err)
	}
}

func TestOpenAbsoluteRootNoFollowRejectsReplacedCurrentDirectoryPath(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workingDir := filepath.Join(parent, "working")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	openedWorkingDir, err := os.OpenRoot(workingDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(workingDir, workingDir+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	opened, err := openAbsoluteRootNoFollowFrom(
		workingDir,
		workingDir,
		func() (*os.Root, error) { return openedWorkingDir, nil },
		os.OpenRoot,
	)
	if opened != nil {
		_ = opened.Close()
		t.Fatal("openAbsoluteRootNoFollowFrom() accepted a replaced working directory path")
	}
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("openAbsoluteRootNoFollowFrom() error = %v, want ErrSymlink", err)
	}
}

func TestOpenRootPinsOriginalDirectoryAcrossPathReplacement(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "root")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, directory)
	opened, err := root.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := os.Rename(directory, directory+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), directory); err != nil {
		t.Fatal(err)
	}
	if err := opened.WriteFile("sentinel", []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory+"-original", "sentinel")); err != nil {
		t.Fatalf("pinned root did not address original directory: %v", err)
	}
}

func TestContainsPathUsesPinnedRootIdentity(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	selected := filepath.Join(parent, "selected")
	inside := filepath.Join(selected, "state", "receipt.json")
	outside := filepath.Join(parent, "outside", "receipt.json")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, selected)

	for path, want := range map[string]bool{inside: true, outside: false} {
		got, err := root.ContainsPath(path)
		if err != nil {
			t.Fatalf("ContainsPath(%q) error = %v", path, err)
		}
		if got != want {
			t.Fatalf("ContainsPath(%q) = %t, want %t", path, got, want)
		}
	}

	if err := os.Rename(selected, selected+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := root.ContainsPath(filepath.Join(selected, "receipt.json")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ContainsPath() after root replacement error = %v, want ErrSymlink", err)
	}
}

func TestContainsAnchoredPathRejectsPathSubstitution(t *testing.T) {
	parent := t.TempDir()
	bundle := filepath.Join(parent, "bundle")
	artifact := filepath.Join(bundle, "state")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	bundleRoot := mustRoot(t, bundle)
	anchored, err := os.OpenRoot(artifact)
	if err != nil {
		t.Fatal(err)
	}
	defer anchored.Close()
	contained, err := bundleRoot.ContainsAnchoredPath(artifact, anchored)
	if err != nil || !contained {
		t.Fatalf("ContainsAnchoredPath() = %t, %v, want true", contained, err)
	}
	if err := os.Rename(artifact, artifact+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := bundleRoot.ContainsAnchoredPath(artifact, anchored); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ContainsAnchoredPath() replacement error = %v, want ErrSymlink", err)
	}
}

func TestContainmentUsesIdentityOnCaseInsensitiveVolume(t *testing.T) {
	parent := t.TempDir()
	selected := filepath.Join(parent, "PreparedBundle")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	caseAlias := filepath.Join(parent, "preparedbundle")
	selectedInfo, err := os.Stat(selected)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(caseAlias)
	if err != nil || !os.SameFile(selectedInfo, aliasInfo) {
		t.Skip("test volume is case-sensitive")
	}
	root := mustRoot(t, selected)
	contained, err := root.ContainsPath(filepath.Join(caseAlias, "state", "receipt.json"))
	if err != nil || !contained {
		t.Fatalf("ContainsPath() = %t, %v, want identity-based containment", contained, err)
	}
	anchored, err := os.OpenRoot(caseAlias)
	if err != nil {
		t.Fatal(err)
	}
	defer anchored.Close()
	contained, err = root.ContainsAnchoredPath(caseAlias, anchored)
	if err != nil || !contained {
		t.Fatalf("ContainsAnchoredPath() = %t, %v, want identity-based containment", contained, err)
	}
}

func TestRootCloseReleasesSharedPinnedIdentity(t *testing.T) {
	root := mustRoot(t, t.TempDir())
	copy := root
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	opened, err := root.OpenRoot()
	if opened != nil {
		_ = opened.Close()
		t.Fatal("OpenRoot() succeeded after a copied Root was closed")
	}
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenRoot() error = %v, want ErrSymlink", err)
	}
}

func TestOpenRootRejectsSelectedPathSwappedBeforeOpen(t *testing.T) {
	requireSymlinks(t)
	for _, test := range []struct {
		name         string
		rootRelative string
		swapRelative string
		outsideRoot  string
	}{
		{name: "final root", rootRelative: "root", swapRelative: "root"},
		{name: "intermediate directory", rootRelative: filepath.Join("selected", "root"), swapRelative: "selected", outsideRoot: "root"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(parent, test.rootRelative)
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			swapPath := filepath.Join(parent, test.swapRelative)
			outside, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if test.outsideRoot != "" {
				if err := os.Mkdir(filepath.Join(outside, test.outsideRoot), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			root := mustRoot(t, directory)
			root.beforeOpenRootForTest = func() {
				if err := os.Rename(swapPath, swapPath+"-original"); err != nil {
					t.Fatalf("rename selected path: %v", err)
				}
				if err := os.Symlink(outside, swapPath); err != nil {
					t.Fatalf("replace selected path with symlink: %v", err)
				}
			}

			opened, openErr := root.OpenRoot()
			if opened != nil {
				if openErr == nil {
					_ = opened.WriteFile("escaped", []byte("unsafe"), 0o600)
				}
				_ = opened.Close()
			}
			if !errors.Is(openErr, ErrSymlink) {
				t.Fatalf("OpenRoot() error = %v, want ErrSymlink", openErr)
			}
			if _, err := os.Lstat(filepath.Join(outside, test.outsideRoot, "escaped")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("OpenRoot() wrote through substituted path: %v", err)
			}
		})
	}
}

func TestOpenRootRejectsSelectedDirectoryReplacedBeforeOpen(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "root")
	replacement := filepath.Join(parent, "replacement")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacement, 0o755); err != nil {
		t.Fatal(err)
	}

	root := mustRoot(t, directory)
	root.beforeOpenRootForTest = func() {
		if err := os.Rename(directory, directory+"-original"); err != nil {
			t.Fatalf("rename selected root: %v", err)
		}
		if err := os.Rename(replacement, directory); err != nil {
			t.Fatalf("replace selected root: %v", err)
		}
	}

	writeErr := root.WriteFile("escaped", []byte("unsafe"), 0o600)
	if !errors.Is(writeErr, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", writeErr)
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached replacement directory: %v", err)
	}
}

func TestRootedWriteRejectsSelectedDirectoryRecreatedBeforeOpen(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "root")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, directory)
	root.beforeOpenRootForTest = func() {
		if err := os.Remove(directory); err != nil {
			t.Fatalf("remove selected root: %v", err)
		}
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatalf("recreate selected root: %v", err)
		}
	}
	if err := root.WriteFile("escaped", []byte("unsafe"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached recreated directory: %v", err)
	}
}

func TestOpenRootDoesNotAdoptDirectoryCreatedAfterNew(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "not-yet-selected")
	root, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	opened, err := root.OpenRoot()
	if opened != nil {
		_ = opened.Close()
		t.Fatal("OpenRoot() adopted a directory created after New")
	}
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenRoot() error = %v, want ErrSymlink", err)
	}
}

func TestRootedWriteRejectsIntermediateDirectoryReplacedBeforeOpen(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	selectedParent := filepath.Join(parent, "selected")
	directory := filepath.Join(selectedParent, "root")
	replacementParent := filepath.Join(parent, "replacement")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacementParent, 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, directory)
	root.beforeOpenRootForTest = func() {
		if err := os.Rename(selectedParent, selectedParent+"-original"); err != nil {
			t.Fatalf("rename selected parent: %v", err)
		}
		if err := os.Rename(replacementParent, selectedParent); err != nil {
			t.Fatalf("replace selected parent: %v", err)
		}
	}
	if err := root.WriteFile("escaped", []byte("unsafe"), 0o600); err == nil {
		t.Fatal("WriteFile() succeeded through replacement ancestor")
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached replacement ancestor: %v", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() created the selected root in replacement ancestor: %v", err)
	}
}

func TestOpenRootPreservesSelectedSymlinkedParentLayout(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physicalParent := filepath.Join(parent, "physical")
	directory := filepath.Join(physicalParent, "root")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkedParent := filepath.Join(parent, "selected")
	if err := os.Symlink(physicalParent, symlinkedParent); err != nil {
		t.Fatal(err)
	}

	root := mustRoot(t, filepath.Join(symlinkedParent, "root"))
	opened, err := root.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := opened.WriteFile("sentinel", []byte("selected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(directory, "sentinel")); got != "selected" {
		t.Fatalf("pinned symlinked parent content = %q", got)
	}
}

func TestMissingNestedRootResolvesNearestSymlinkedAncestor(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(parent, "physical")
	if err := os.Mkdir(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(physical, link); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, filepath.Join(link, "one", "two", "root"))
	defer root.Close()
	if err := root.MkdirAll(".", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := root.WriteFile("sentinel", []byte("selected"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(physical, "one", "two", "root", "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "selected" {
		t.Fatalf("sentinel = %q", data)
	}
}

func TestMissingRootResolvesNestedSymlinkPrefix(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(parent, "first")
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(first, filepath.Join(parent, "outer")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(first, "inner")); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, filepath.Join(parent, "outer", "inner", "new", "root"))
	defer root.Close()
	if err := root.MkdirAll(".", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(target, "new", "root")); err != nil || !info.IsDir() {
		t.Fatalf("physical nested root stat = %#v, %v", info, err)
	}
}

func TestNewRejectsInvalidExistingAncestorWithoutCreating(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	t.Run("dangling symlink", func(t *testing.T) {
		link := filepath.Join(parent, "dangling")
		if err := os.Symlink(filepath.Join(parent, "absent"), link); err != nil {
			t.Fatal(err)
		}
		if _, err := New(filepath.Join(link, "new", "root")); err == nil {
			t.Fatal("New() accepted a dangling symlink ancestor")
		}
		if _, err := os.Stat(filepath.Join(parent, "absent")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("New() created through dangling symlink: %v", err)
		}
	})
	t.Run("symlink loop", func(t *testing.T) {
		loop := filepath.Join(parent, "loop")
		if err := os.Symlink("loop", loop); err != nil {
			t.Fatal(err)
		}
		if _, err := New(filepath.Join(loop, "new", "root")); err == nil {
			t.Fatal("New() accepted a symlink loop ancestor")
		}
	})
	t.Run("regular file", func(t *testing.T) {
		file := filepath.Join(parent, "file")
		if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := New(filepath.Join(file, "new", "root")); err == nil {
			t.Fatal("New() accepted a non-directory ancestor")
		}
		if data, err := os.ReadFile(file); err != nil || string(data) != "file" {
			t.Fatalf("ancestor file changed: %q, %v", data, err)
		}
	})
}

func TestMissingRootRejectsAncestorReplacementBeforeCreation(t *testing.T) {
	requireSymlinks(t)
	for _, test := range []struct {
		name    string
		symlink bool
	}{
		{name: "ordinary ancestor"},
		{name: "symlinked ancestor", symlink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(parent, "original")
			replacement := filepath.Join(parent, "replacement")
			if err := os.Mkdir(original, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(replacement, 0o755); err != nil {
				t.Fatal(err)
			}
			selectedBase := original
			if test.symlink {
				selectedBase = filepath.Join(parent, "selected")
				if err := os.Symlink(original, selectedBase); err != nil {
					t.Fatal(err)
				}
			}
			root := mustRoot(t, filepath.Join(selectedBase, "new", "root"))
			defer root.Close()
			root.beforeCreateRootForTest = func() {
				if test.symlink {
					if err := os.Rename(selectedBase, selectedBase+"-original"); err != nil {
						t.Fatalf("rename selected symlink: %v", err)
					}
					if err := os.Symlink(replacement, selectedBase); err != nil {
						t.Fatalf("replace selected symlink: %v", err)
					}
					return
				}
				if err := os.Rename(original, original+"-original"); err != nil {
					t.Fatalf("rename selected ancestor: %v", err)
				}
				if err := os.Rename(replacement, original); err != nil {
					t.Fatalf("replace selected ancestor: %v", err)
				}
			}
			if err := root.MkdirAll(".", 0o755); !errors.Is(err, ErrSymlink) {
				t.Fatalf("MkdirAll() error = %v, want ErrSymlink", err)
			}
			for _, directory := range []string{original, original + "-original", replacement} {
				if _, err := os.Stat(filepath.Join(directory, "new")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("partial root created beneath %q: %v", directory, err)
				}
			}
		})
	}
}

func TestValidateMissingRootComponent(t *testing.T) {
	for _, component := range []string{"", ".", "..", string([]byte{'b', 'a', 'd', 0})} {
		if err := validateMissingRootComponent(component); !errors.Is(err, ErrEscapesRoot) {
			t.Fatalf("validateMissingRootComponent(%q) error = %v, want ErrEscapesRoot", component, err)
		}
	}
	for _, component := range []string{"root", "legacy-name", "a.b"} {
		if err := validateMissingRootComponent(component); err != nil {
			t.Fatalf("validateMissingRootComponent(%q) error = %v", component, err)
		}
	}
}

func TestCloseReleasesPendingRootAncestorDescriptors(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before := openDescriptorCount(t)
	const rootCount = 64
	roots := make([]Root, 0, rootCount)
	for index := 0; index < rootCount; index++ {
		roots = append(roots, mustRoot(t, filepath.Join(parent, fmt.Sprintf("missing-%d", index), "root")))
	}
	for _, root := range roots {
		if err := root.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
	after := openDescriptorCount(t)
	if after > before+4 {
		t.Fatalf("open descriptors after pending-root Close = %d, baseline %d", after, before)
	}
}

func TestCloseReleasesPinnedDescriptorAcrossRootCopies(t *testing.T) {
	root := mustRoot(t, t.TempDir())
	copyOfRoot := root

	if err := root.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if opened, err := copyOfRoot.OpenRoot(); err == nil {
		_ = opened.Close()
		t.Fatal("copied Root remained usable after shared Close")
	}
	if err := copyOfRoot.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCloseReleasesPinnedDescriptorsDeterministically(t *testing.T) {
	before := openDescriptorCount(t)
	const rootCount = 128
	roots := make([]Root, 0, rootCount)
	for range rootCount {
		roots = append(roots, mustRoot(t, t.TempDir()))
	}
	for _, root := range roots {
		if err := root.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
	after := openDescriptorCount(t)
	if after > before+4 {
		t.Fatalf("open descriptors after Close = %d, baseline %d", after, before)
	}
}

func openDescriptorCount(t *testing.T) int {
	t.Helper()
	for _, directory := range []string{"/dev/fd", "/proc/self/fd"} {
		entries, err := os.ReadDir(directory)
		if err == nil {
			return len(entries)
		}
	}
	t.Skip("open descriptor enumeration is unavailable")
	return 0
}

func mustRoot(t *testing.T, path string) Root {
	t.Helper()
	root, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) error = %v", path, err)
	}
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

func temporaryLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	var leftovers []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".asc-tmp-") {
			leftovers = append(leftovers, entry.Name())
		}
	}
	return leftovers
}

func requireSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Skip("symlink creation is not permitted on this host")
	}
}
