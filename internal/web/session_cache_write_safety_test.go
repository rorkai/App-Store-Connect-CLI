package web

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupSessionFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.json")
	cachePath := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(targetPath, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Symlink(targetPath, cachePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := backupSessionFile(cachePath)
	if !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("backupSessionFile() error = %v, want errUnsafeSessionCacheFile", err)
	}
}

func TestBackupSessionFilePreservesRegularFileDataAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	want := []byte("cached session")
	if err := os.WriteFile(path, want, 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	backup, err := backupSessionFile(path)
	if err != nil {
		t.Fatalf("backupSessionFile() error = %v", err)
	}
	if !backup.exists {
		t.Fatal("backupSessionFile() exists = false, want true")
	}
	if string(backup.data) != string(want) {
		t.Fatalf("backupSessionFile() data = %q, want %q", backup.data, want)
	}
	if backup.mode != 0o640 {
		t.Fatalf("backupSessionFile() mode = %#o, want %#o", backup.mode, 0o640)
	}
}

func TestBackupSessionFileRejectsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache-dir")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	_, err := backupSessionFile(path)
	if !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("backupSessionFile() error = %v, want errUnsafeSessionCacheFile", err)
	}
}

func TestBackupSessionFileRejectsOversizedRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := file.Truncate(webSessionCacheMaxBytes + 1); err != nil {
		_ = file.Close()
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = backupSessionFile(path)
	if !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("backupSessionFile() error = %v, want errUnsafeSessionCacheFile", err)
	}
}

func TestReadSessionCacheFileRejectsOversizedRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := file.Truncate(webSessionCacheMaxBytes + 1); err != nil {
		_ = file.Close()
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = readSessionCacheFile(path)
	if !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("readSessionCacheFile() error = %v, want errUnsafeSessionCacheFile", err)
	}
}

func TestRestoreSessionFileDoesNotReuseFailedPersistenceWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	previousWrite := sessionFileWrite
	sessionFileWrite = func(string, *os.File, []byte, os.FileMode) error {
		return errors.New("injected persistence failure")
	}
	t.Cleanup(func() { sessionFileWrite = previousWrite })

	backup := sessionFileBackup{exists: true, data: []byte("original"), mode: 0o640}
	if err := restoreSessionFile(path, backup); err != nil {
		t.Fatalf("restoreSessionFile() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("restored data = %q, want %q", got, "original")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o640 {
		t.Fatalf("restored mode = %#o, want captured mode %#o", gotMode, os.FileMode(0o640))
	}
}

func TestWriteSessionToFileIfAbsentCreatesPrivateFiles(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFileIfAbsent(key, webTestPersistedSession(t, "private-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToFileIfAbsent() error = %v", err)
	}
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	assertSessionCacheMode(t, sessionPath)
	assertSessionCacheMode(t, lastPath)
}

func TestPersistSessionRejectsSymlinkedCacheEntry(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "outside.json")
	sentinel := []byte("outside session must not be replaced")
	if err := os.WriteFile(targetPath, sentinel, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
	if err := os.Symlink(targetPath, sessionPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	target, err := url.Parse("https://appstoreconnect.apple.com/")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	jar.SetCookies(target, []*http.Cookie{{Name: "myacinfo", Value: "token", Path: "/", Expires: time.Now().Add(time.Hour)}})
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}); !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("PersistSession() error = %v, want errUnsafeSessionCacheFile", err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(target) error = %v", err)
	}
	if string(got) != string(sentinel) {
		t.Fatalf("target data = %q, want unchanged sentinel", got)
	}
}

func TestImportSessionBundleOverwriteRejectsSymlinkedCacheEntry(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "outside.json")
	sentinel := []byte("outside session must not be replaced")
	if err := os.WriteFile(targetPath, sentinel, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
	if err := os.Symlink(targetPath, sessionPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := ImportSessionBundleWithOptions(bundle, true); !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want errUnsafeSessionCacheFile", err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(target) error = %v", err)
	}
	if string(got) != string(sentinel) {
		t.Fatalf("target data = %q, want unchanged sentinel", got)
	}
}

func TestPersistSessionIgnoresSymlinkedSessionTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	targetPath := seedSessionCacheSymlink(t, sessionPath+".tmp")

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want pre-existing temp symlink ignored", err)
	}
	assertSessionCacheSymlinkUnchanged(t, sessionPath+".tmp", targetPath)
}

func TestPersistSessionIgnoresSymlinkedLastTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	targetPath := seedSessionCacheSymlink(t, lastPath+".tmp")

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want pre-existing last-marker temp symlink ignored", err)
	}
	assertSessionCacheSymlinkUnchanged(t, lastPath+".tmp", targetPath)
}

func TestImportSessionBundleOverwriteIgnoresSymlinkedSessionTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	targetPath := seedSessionCacheSymlink(t, sessionPath+".tmp")

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want pre-existing temp symlink ignored", err)
	}
	assertSessionCacheSymlinkUnchanged(t, sessionPath+".tmp", targetPath)
}

func TestImportSessionBundleOverwriteIgnoresSymlinkedLastTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	targetPath := seedSessionCacheSymlink(t, lastPath+".tmp")

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want pre-existing last-marker temp symlink ignored", err)
	}
	assertSessionCacheSymlinkUnchanged(t, lastPath+".tmp", targetPath)
}

func TestPersistSessionIgnoresStaleRegularSessionTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	stalePath := sessionPath + ".tmp"
	seedStaleSessionCacheFile(t, stalePath)

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want stale regular temp ignored", err)
	}
	assertSessionCacheTempPreserved(t, stalePath)
	assertSessionCacheMode(t, sessionPath)
	if _, ok, err := readSessionFromFile(key); err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want persisted session", ok, err)
	}
}

func TestPersistSessionIgnoresStaleRegularLastTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	stalePath := lastPath + ".tmp"
	seedStaleSessionCacheFile(t, stalePath)

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want stale regular temp ignored", err)
	}
	assertSessionCacheTempPreserved(t, stalePath)
	assertSessionCacheMode(t, lastPath)
	key, ok, err := readLastKeyFromFile()
	if err != nil || !ok || key != webSessionCacheKey("user@example.com") {
		t.Fatalf("readLastKeyFromFile() = (%q, %t, %v), want persisted session key", key, ok, err)
	}
}

func TestImportSessionBundleOverwriteIgnoresStaleRegularSessionTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	stalePath := sessionPath + ".tmp"
	seedStaleSessionCacheFile(t, stalePath)

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want stale regular temp ignored", err)
	}
	assertSessionCacheTempPreserved(t, stalePath)
	assertSessionCacheMode(t, sessionPath)
	if _, ok, err := readSessionFromFile(key); err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want imported session", ok, err)
	}
}

func TestImportSessionBundleOverwriteIgnoresStaleRegularLastTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	stalePath := lastPath + ".tmp"
	seedStaleSessionCacheFile(t, stalePath)

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want stale regular temp ignored", err)
	}
	assertSessionCacheTempPreserved(t, stalePath)
	assertSessionCacheMode(t, lastPath)
	key, ok, err := readLastKeyFromFile()
	if err != nil || !ok || key != webSessionCacheKey(bundle.AppleID) {
		t.Fatalf("readLastKeyFromFile() = (%q, %t, %v), want imported session key", key, ok, err)
	}
}

func TestPersistSessionPreservesHardLinkedSessionTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	tmpPath := sessionPath + ".tmp"
	targetPath := seedSessionCacheHardlink(t, tmpPath)

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want pre-existing temp hard link ignored", err)
	}
	assertSessionCacheHardlinkUnchanged(t, tmpPath, targetPath)
}

func TestPersistSessionPreservesHardLinkedLastTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	tmpPath := lastPath + ".tmp"
	targetPath := seedSessionCacheHardlink(t, tmpPath)

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want pre-existing temp hard link ignored", err)
	}
	assertSessionCacheHardlinkUnchanged(t, tmpPath, targetPath)
}

func TestImportSessionBundleOverwritePreservesHardLinkedSessionTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	tmpPath := sessionPath + ".tmp"
	targetPath := seedSessionCacheHardlink(t, tmpPath)

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want pre-existing temp hard link ignored", err)
	}
	assertSessionCacheHardlinkUnchanged(t, tmpPath, targetPath)
}

func TestImportSessionBundleOverwritePreservesHardLinkedLastTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	tmpPath := lastPath + ".tmp"
	targetPath := seedSessionCacheHardlink(t, tmpPath)

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want pre-existing temp hard link ignored", err)
	}
	assertSessionCacheHardlinkUnchanged(t, tmpPath, targetPath)
}

func TestPersistSessionIgnoresDirectoryTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	tmpPath := sessionPath + ".tmp"
	if err := os.MkdirAll(tmpPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(temp directory) error = %v", err)
	}

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want pre-existing temp directory ignored", err)
	}
	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v, want temp directory preserved", tmpPath, err)
	}
	if !info.IsDir() {
		t.Fatalf("temp path mode = %v, want directory preserved", info.Mode())
	}
}

func seedSessionCacheSymlink(t *testing.T, cachePath string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(targetPath, []byte("outside temp must not be replaced"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Symlink(targetPath, cachePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	return targetPath
}

func seedStaleSessionCacheFile(t *testing.T, cachePath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("stale temporary contents"), 0o640); err != nil {
		t.Fatalf("WriteFile(stale temp) error = %v", err)
	}
}

func assertSessionCacheTempPreserved(t *testing.T, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want stale temp preserved", path, err)
	}
	if string(got) != "stale temporary contents" {
		t.Fatalf("stale temp data = %q, want unchanged contents", got)
	}
}

func assertSessionCacheMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode for %q = %#o, want %#o", path, got, os.FileMode(0o600))
	}
}

func assertSessionCacheTargetUnchanged(t *testing.T, targetPath string) {
	t.Helper()
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(target) error = %v", err)
	}
	if string(got) != "outside temp must not be replaced" {
		t.Fatalf("target data = %q, want unchanged sentinel", got)
	}
}

func assertSessionCacheSymlinkUnchanged(t *testing.T, cachePath, targetPath string) {
	t.Helper()
	info, err := os.Lstat(cachePath)
	if err != nil {
		t.Fatalf("Lstat(%q) error = %v, want symlink preserved", cachePath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("cache temp mode = %v, want symlink preserved", info.Mode())
	}
	if got, err := os.Readlink(cachePath); err != nil || got != targetPath {
		t.Fatalf("Readlink(%q) = (%q, %v), want %q", cachePath, got, err, targetPath)
	}
	assertSessionCacheTargetUnchanged(t, targetPath)
}

func seedSessionCacheHardlink(t *testing.T, cachePath string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
	targetPath := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(targetPath, []byte("outside temp must not be replaced"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Link(targetPath, cachePath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	return targetPath
}

func assertSessionCacheHardlinkUnchanged(t *testing.T, cachePath, targetPath string) {
	t.Helper()
	assertSessionCacheTargetUnchanged(t, targetPath)
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v, want hard link preserved", cachePath, err)
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v, want hard link preserved", targetPath, err)
	}
	if !os.SameFile(cacheInfo, targetInfo) {
		t.Fatalf("cache temp and outside target are no longer the same file")
	}
}

func newTestAuthSession(t *testing.T) *AuthSession {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	target, err := url.Parse("https://appstoreconnect.apple.com/")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	jar.SetCookies(target, []*http.Cookie{{Name: "myacinfo", Value: "token", Path: "/", Expires: time.Now().Add(time.Hour)}})
	return &AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}
}
