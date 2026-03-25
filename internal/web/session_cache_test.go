package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

type countingKeyring struct {
	keyring.Keyring
	getCounts map[string]int
}

func (kr *countingKeyring) Get(key string) (keyring.Item, error) {
	kr.getCounts[key]++
	return kr.Keyring.Get(key)
}

func (kr *countingKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return kr.Keyring.GetMetadata(key)
}

func (kr *countingKeyring) Set(item keyring.Item) error {
	return kr.Keyring.Set(item)
}

func (kr *countingKeyring) Remove(key string) error {
	return kr.Keyring.Remove(key)
}

func (kr *countingKeyring) Keys() ([]string, error) {
	return kr.Keyring.Keys()
}

func (kr *countingKeyring) ResetCounts() {
	kr.getCounts = map[string]int{}
}

func (kr *countingKeyring) GetCount(key string) int {
	return kr.getCounts[key]
}

func withArraySessionKeyring(t *testing.T) *countingKeyring {
	t.Helper()
	prev := sessionKeyringOpen
	kr := &countingKeyring{
		Keyring:   keyring.NewArrayKeyring([]keyring.Item{}),
		getCounts: map[string]int{},
	}
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		sessionKeyringOpen = prev
	})
	return kr
}

func withUnavailableSessionKeyring(t *testing.T) {
	t.Helper()
	prev := sessionKeyringOpen
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		sessionKeyringOpen = prev
	})
}

func withSessionInfoStub(t *testing.T) {
	t.Helper()
	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		out := &sessionInfo{}
		out.Provider.ProviderID = 42
		out.User.EmailAddress = "user@example.com"
		return out, nil
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})
}

func TestResolveBackendSelectionDefaultsToFileWithKeychainFallback(t *testing.T) {
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")

	selection := resolveBackendSelection()
	if selection.backend != sessionBackendFile {
		t.Fatalf("expected default backend %v, got %v", sessionBackendFile, selection.backend)
	}
	if !selection.fallbackKeychain {
		t.Fatal("expected default backend to fall back to keychain")
	}
	if selection.fallbackFile {
		t.Fatal("did not expect default file backend to fall back to file")
	}
}

func TestResolveBackendSelectionKeychainFallsBackToFile(t *testing.T) {
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")

	selection := resolveBackendSelection()
	if selection.backend != sessionBackendKeychain {
		t.Fatalf("expected keychain backend %v, got %v", sessionBackendKeychain, selection.backend)
	}
	if !selection.fallbackFile {
		t.Fatal("expected keychain backend to fall back to file")
	}
	if selection.fallbackKeychain {
		t.Fatal("did not expect keychain backend to fall back to keychain")
	}
}

func TestPersistSessionDefaultBackendWritesFileWithoutKeychain(t *testing.T) {
	kr := withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "file-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	keys, err := kr.Keys()
	if err != nil {
		t.Fatalf("keyring keys error: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected default backend not to write keychain entries, got %#v", keys)
	}

	if _, ok, err := readSessionFromFile(webSessionCacheKey("user@example.com")); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if !ok {
		t.Fatal("expected default backend to persist a file-backed session")
	}
}

func TestTryResumeSessionDefaultBackendPrefersFileWithoutKeychain(t *testing.T) {
	kr := withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "file-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	kr.ResetCounts()
	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed session")
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("expected file-backed resume to avoid keychain reads, got %d store gets", got)
	}
}

func TestTryResumeSessionDefaultBackendFallsBackToKeychainAndPersistsFile(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "keychain-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed keychain-backed session")
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if !ok {
		t.Fatal("expected keychain fallback resume to repersist into file cache")
	}
}

func TestTryResumeSessionDefaultBackendFallsBackToKeychainWhenFileSessionIsCorrupt(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "keychain-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(sessionPath, []byte(`not-json`), 0o600); err != nil {
		t.Fatalf("write corrupt session file: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed keychain-backed session")
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if !ok {
		t.Fatal("expected corrupt-session fallback to repersist into file cache")
	}
}

func TestTryResumeLastSessionDefaultBackendFallsBackToKeychainWhenLastFileSessionMissing(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "keychain-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(lastPath), 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		t.Fatalf("marshal last-session marker: %v", err)
	}
	if err := os.WriteFile(lastPath, lastRaw, 0o600); err != nil {
		t.Fatalf("write last-session marker: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed keychain-backed last session")
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if !ok {
		t.Fatal("expected keychain fallback last-session resume to repersist into file cache")
	}
}

func TestTryResumeLastSessionDefaultBackendFallsBackToKeychainWhenLastFileSessionIsCorrupt(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "keychain-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(lastPath), 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		t.Fatalf("marshal last-session marker: %v", err)
	}
	if err := os.WriteFile(lastPath, lastRaw, 0o600); err != nil {
		t.Fatalf("write last-session marker: %v", err)
	}

	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(sessionPath, []byte(`not-json`), 0o600); err != nil {
		t.Fatalf("write corrupt session file: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed keychain-backed last session")
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if !ok {
		t.Fatal("expected corrupt last-session fallback to repersist into file cache")
	}
}

func TestTryResumeLastSessionDefaultBackendFallsBackToKeychainWhenLastFileMarkerIsCorrupt(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "keychain-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(lastPath), 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(lastPath, []byte(`not-json`), 0o600); err != nil {
		t.Fatalf("write corrupt last-session marker: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed keychain-backed last session")
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if !ok {
		t.Fatal("expected corrupt-marker keychain fallback to repersist into file cache")
	}
}

func TestTryResumeSessionKeychainBackendFallsBackToFileAndPersistsKeychain(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "file-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed file-backed session")
	}

	if _, ok, err := readSessionFromKeychain(key); err != nil {
		t.Fatalf("readSessionFromKeychain error: %v", err)
	} else if !ok {
		t.Fatal("expected file fallback resume to repersist into keychain")
	}
}

func TestTryResumeLastSessionKeychainBackendFallsBackToFileAndPersistsKeychain(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "file-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed last session from file fallback")
	}

	if _, ok, err := readSessionFromKeychain(key); err != nil {
		t.Fatalf("readSessionFromKeychain error: %v", err)
	} else if !ok {
		t.Fatal("expected last-session file fallback to repersist into keychain")
	}
}

func TestDeleteSessionDefaultBackendRemovesFileAndKeychain(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "shared-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	session := &AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected file-backed session to be removed")
	}
	if _, ok, err := readSessionFromKeychain(key); err != nil {
		t.Fatalf("readSessionFromKeychain error: %v", err)
	} else if ok {
		t.Fatal("expected keychain-backed session to be removed")
	}
}

func TestDeleteSessionFileBackendPreservesDifferentLastSessionMarker(t *testing.T) {
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	firstKey := webSessionCacheKey("first@example.com")
	secondKey := webSessionCacheKey("second@example.com")
	firstSession := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}
	secondSession := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}

	if err := writeSessionToFile(firstKey, firstSession); err != nil {
		t.Fatalf("writeSessionToFile first error: %v", err)
	}
	if err := writeSessionToFile(secondKey, secondSession); err != nil {
		t.Fatalf("writeSessionToFile second error: %v", err)
	}

	if err := DeleteSession("first@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}

	if _, ok, err := readSessionFromFile(firstKey); err != nil {
		t.Fatalf("readSessionFromFile first error: %v", err)
	} else if ok {
		t.Fatal("expected deleted session to be removed")
	}
	if _, ok, err := readSessionFromFile(secondKey); err != nil {
		t.Fatalf("readSessionFromFile second error: %v", err)
	} else if !ok {
		t.Fatal("expected unrelated session to remain")
	}
	if lastKey, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if !ok || lastKey != secondKey {
		t.Fatalf("expected last-session marker %q to remain, got %q (ok=%v)", secondKey, lastKey, ok)
	}
}

func TestDeleteSessionDefaultBackendIgnoresUnavailableKeychainFallback(t *testing.T) {
	withUnavailableSessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected file-backed session to be removed")
	}
	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected last-session marker to be removed")
	}
}

func TestDeleteSessionFileBackendIgnoresMalformedLastMarker(t *testing.T) {
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath error: %v", err)
	}
	if err := os.WriteFile(lastPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed last marker: %v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected deleted session to be removed")
	}
	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected malformed last-session marker to be cleared")
	}
}

func TestDeleteSessionKeychainBackendAlsoRemovesFileCache(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}
	if err := writeSessionToKeychain(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected mirrored file-backed session to be removed")
	}
	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected mirrored file-backed last marker to be removed")
	}
}

func TestDeleteSessionKeychainBackendSurfacesMirroredFileDeleteError(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")

	webCachePath := filepath.Join(t.TempDir(), "web-cache-file")
	if err := os.WriteFile(webCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write web cache file: %v", err)
	}
	t.Setenv(webSessionCacheDirEnv, webCachePath)

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	err := DeleteSession("user@example.com")
	if err == nil {
		t.Fatal("expected mirrored file delete error")
	}
	if !strings.Contains(err.Error(), webCachePath) {
		t.Fatalf("expected mirrored file delete error to mention %q, got %v", webCachePath, err)
	}
	if _, ok, readErr := readSessionFromKeychain(key); readErr != nil {
		t.Fatalf("readSessionFromKeychain error: %v", readErr)
	} else if ok {
		t.Fatal("expected keychain-backed session to still be removed")
	}
}

func TestDeleteSessionKeychainFallbackPreservesDifferentFileLastSessionMarker(t *testing.T) {
	withUnavailableSessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	firstKey := webSessionCacheKey("first@example.com")
	secondKey := webSessionCacheKey("second@example.com")
	firstSession := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}
	secondSession := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}

	if err := writeSessionToFile(firstKey, firstSession); err != nil {
		t.Fatalf("writeSessionToFile first error: %v", err)
	}
	if err := writeSessionToFile(secondKey, secondSession); err != nil {
		t.Fatalf("writeSessionToFile second error: %v", err)
	}

	if err := DeleteSession("first@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}

	if _, ok, err := readSessionFromFile(firstKey); err != nil {
		t.Fatalf("readSessionFromFile first error: %v", err)
	} else if ok {
		t.Fatal("expected deleted fallback file-backed session to be removed")
	}
	if _, ok, err := readSessionFromFile(secondKey); err != nil {
		t.Fatalf("readSessionFromFile second error: %v", err)
	} else if !ok {
		t.Fatal("expected unrelated fallback file-backed session to remain")
	}
	if lastKey, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if !ok || lastKey != secondKey {
		t.Fatalf("expected last-session marker %q to remain, got %q (ok=%v)", secondKey, lastKey, ok)
	}
}

func TestHydrateCookieJarSkipsExpiredCookies(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}

	now := time.Now().UTC()
	sess := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "expired", Value: "old", Expires: now.Add(-1 * time.Hour)},
				{Name: "valid", Value: "new", Expires: now.Add(1 * time.Hour)},
			},
		},
	}

	loaded := hydrateCookieJar(jar, sess)
	if loaded != 1 {
		t.Fatalf("expected 1 valid cookie loaded, got %d", loaded)
	}
	u, _ := url.Parse("https://appstoreconnect.apple.com/")
	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "valid" {
		t.Fatalf("expected only valid cookie, got %+v", cookies)
	}
}

func TestPersistSessionUsesSingleSharedKeychainStore(t *testing.T) {
	kr := withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")

	firstJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	firstJar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "token-one", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	secondJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	secondJar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "token-two", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: firstJar},
		UserEmail: "first@example.com",
	}); err != nil {
		t.Fatalf("PersistSession(first) error: %v", err)
	}
	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: secondJar},
		UserEmail: "second@example.com",
	}); err != nil {
		t.Fatalf("PersistSession(second) error: %v", err)
	}

	keys, err := kr.Keys()
	if err != nil {
		t.Fatalf("keyring keys error: %v", err)
	}
	if len(keys) != 1 || keys[0] != webSessionStoreItem {
		t.Fatalf("expected single shared keychain store item, got %#v", keys)
	}

	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		token := cookieValue(client.Jar.Cookies(targetURL), "myacinfo")
		out := &sessionInfo{}
		switch token {
		case "token-one":
			out.Provider.ProviderID = 1
			out.User.EmailAddress = "first@example.com"
		case "token-two":
			out.Provider.ProviderID = 2
			out.User.EmailAddress = "second@example.com"
		default:
			return nil, errors.New("unexpected cached session token")
		}
		return out, nil
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})

	kr.ResetCounts()
	last, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || last == nil {
		t.Fatal("expected last account session")
	}
	if last.UserEmail != "second@example.com" || last.ProviderID != 2 {
		t.Fatalf("unexpected last resumed session: %+v", last)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 2 {
		t.Fatalf("expected TryResumeLastSession to read shared store once and refresh it once, got %d store gets", got)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "first@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession(first) error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected first account session")
	}
	if resumed.UserEmail != "first@example.com" || resumed.ProviderID != 1 {
		t.Fatalf("unexpected first resumed session: %+v", resumed)
	}
}

func TestPersistAndResumeSessionFromKeychain(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	session := &AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed session")
	}
	if resumed.UserEmail != "user@example.com" {
		t.Fatalf("expected email user@example.com, got %q", resumed.UserEmail)
	}
	if resumed.ProviderID != 42 {
		t.Fatalf("expected provider id 42, got %d", resumed.ProviderID)
	}
}

func TestTryResumeSessionPersistsRefreshedCookies(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "stale-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	session := &AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		client.Jar.SetCookies(targetURL, []*http.Cookie{
			{Name: "myacinfo", Value: "refreshed-token", Path: "/", Expires: time.Now().Add(72 * time.Hour)},
		})
		out := &sessionInfo{}
		out.Provider.ProviderID = 42
		out.User.EmailAddress = "user@example.com"
		return out, nil
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed session")
	}

	selection := resolveBackendSelection()
	stored, ok, err := readSessionBySelection(selection, webSessionCacheKey("user@example.com"))
	if err != nil {
		t.Fatalf("readSessionBySelection error: %v", err)
	}
	if !ok {
		t.Fatal("expected refreshed session in cache")
	}

	if got := persistedCookieValue(stored, "https://appstoreconnect.apple.com/", "myacinfo"); got != "refreshed-token" {
		t.Fatalf("expected refreshed cookie value, got %q", got)
	}
}

func TestTryResumeLastSessionPersistsRefreshedCookies(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "old-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	session := &AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		client.Jar.SetCookies(targetURL, []*http.Cookie{
			{Name: "myacinfo", Value: "new-token", Path: "/", Expires: time.Now().Add(72 * time.Hour)},
		})
		out := &sessionInfo{}
		out.Provider.ProviderID = 99
		out.User.EmailAddress = "user@example.com"
		return out, nil
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed session")
	}

	selection := resolveBackendSelection()
	stored, ok, err := readSessionBySelection(selection, webSessionCacheKey("user@example.com"))
	if err != nil {
		t.Fatalf("readSessionBySelection error: %v", err)
	}
	if !ok {
		t.Fatal("expected refreshed session in cache")
	}
	if got := persistedCookieValue(stored, "https://appstoreconnect.apple.com/", "myacinfo"); got != "new-token" {
		t.Fatalf("expected refreshed cookie value, got %q", got)
	}
}

func TestReadSessionBySelectionKeychainBackendIgnoresBrokenFileMirror(t *testing.T) {
	withArraySessionKeyring(t)
	cacheDir := filepath.Join(t.TempDir(), "web-cache")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, cacheDir)

	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	if err := os.WriteFile(filepath.Join(cacheDir, "session-"+key+".json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed file mirror: %v", err)
	}

	_, ok, err := readSessionBySelection(resolveBackendSelection(), key)
	if err != nil {
		t.Fatalf("expected broken file mirror to be ignored in keychain mode, got %v", err)
	}
	if ok {
		t.Fatal("did not expect session when keychain misses and file mirror is broken")
	}
}

func TestReadLastSessionBySelectionKeychainBackendIgnoresBrokenFileMirror(t *testing.T) {
	withArraySessionKeyring(t)
	cacheDir := filepath.Join(t.TempDir(), "web-cache")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, cacheDir)

	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "last.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed last marker: %v", err)
	}

	_, ok, err := readLastSessionBySelection(resolveBackendSelection())
	if err != nil {
		t.Fatalf("expected broken last-session mirror to be ignored in keychain mode, got %v", err)
	}
	if ok {
		t.Fatal("did not expect session when keychain misses and last-session mirror is broken")
	}
}

func TestTryResumeSessionReturnsExpiredErrorForUnauthorizedCache(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "expired-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	session := &AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		return nil, &sessionInfoStatusError{Status: http.StatusUnauthorized}
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err == nil {
		t.Fatal("expected expired cached-session error")
	}
	if !errors.Is(err, ErrCachedSessionExpired) {
		t.Fatalf("expected ErrCachedSessionExpired, got %v", err)
	}
	if errors.Unwrap(err) != nil {
		t.Fatalf("expected bare ErrCachedSessionExpired sentinel, got wrapped error %v", err)
	}
	if ok {
		t.Fatal("did not expect cache resume success")
	}
	if resumed != nil {
		t.Fatal("did not expect resumed session")
	}
}

func TestLoadCachedSessionHydratesJarWithoutValidation(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "cached-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	session, ok, err := LoadCachedSession("user@example.com")
	if err != nil {
		t.Fatalf("LoadCachedSession error: %v", err)
	}
	if !ok || session == nil {
		t.Fatal("expected cached session to load")
	}
	if session.UserEmail != "user@example.com" {
		t.Fatalf("expected stored email user@example.com, got %q", session.UserEmail)
	}
	if session.Client == nil || session.Client.Jar == nil {
		t.Fatal("expected hydrated client jar")
	}
	if got := cookieValue(session.Client.Jar.Cookies(targetURL), "myacinfo"); got != "cached-token" {
		t.Fatalf("expected hydrated cookie value, got %q", got)
	}
}

func TestLoadLastCachedSessionHydratesJarWithoutValidation(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "cached-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	session, ok, err := LoadLastCachedSession()
	if err != nil {
		t.Fatalf("LoadLastCachedSession error: %v", err)
	}
	if !ok || session == nil {
		t.Fatal("expected last cached session to load")
	}
	if session.UserEmail != "user@example.com" {
		t.Fatalf("expected stored email user@example.com, got %q", session.UserEmail)
	}
	if session.Client == nil || session.Client.Jar == nil {
		t.Fatal("expected hydrated client jar")
	}
	if got := cookieValue(session.Client.Jar.Cookies(targetURL), "myacinfo"); got != "cached-token" {
		t.Fatalf("expected hydrated cookie value, got %q", got)
	}
}

func TestTryResumeLastSessionMigratesLegacyKeychainEntriesToSharedStore(t *testing.T) {
	kr := withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "legacy-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy session: %v", err)
	}
	if err := kr.Set(keyring.Item{Key: keyringSessionItem(key), Data: raw, Label: "ASC Web Session"}); err != nil {
		t.Fatalf("store legacy session: %v", err)
	}
	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		t.Fatalf("marshal legacy last-session marker: %v", err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionLastKeyItem, Data: lastRaw, Label: "ASC Web Session Last Key"}); err != nil {
		t.Fatalf("store legacy last-session marker: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed legacy session")
	}
	if resumed.UserEmail != "user@example.com" || resumed.ProviderID != 42 {
		t.Fatalf("unexpected resumed legacy session: %+v", resumed)
	}

	keys, err := kr.Keys()
	if err != nil {
		t.Fatalf("keyring keys error: %v", err)
	}
	if !containsString(keys, webSessionStoreItem) {
		t.Fatalf("expected shared keychain store after legacy migration, got %#v", keys)
	}
}

func TestTryResumeSessionMigratesLegacyIrisFileCache(t *testing.T) {
	withSessionInfoStub(t)
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "legacy-iris-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), raw, 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed migrated iris session")
	}
	if resumed.UserEmail != "user@example.com" || resumed.ProviderID != 42 {
		t.Fatalf("unexpected resumed migrated iris session: %+v", resumed)
	}

	stored, ok, err := readSessionBySelection(resolveBackendSelection(), key)
	if err != nil {
		t.Fatalf("readSessionBySelection error: %v", err)
	}
	if !ok {
		t.Fatal("expected migrated session in web cache")
	}
	if got := persistedCookieValue(stored, "https://appstoreconnect.apple.com/", "myacinfo"); got != "legacy-iris-token" {
		t.Fatalf("expected migrated legacy cookie value, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris session file to be removed after migration, stat err=%v", err)
	}
}

func TestTryResumeSessionMigratesLegacyIrisFileCacheKeepsResumedSessionWhenPersistFails(t *testing.T) {
	withSessionInfoStub(t)
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatalf("mkdir web dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(webDir, 0o700)
	})

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "legacy-iris-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), raw, 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}
	if err := os.Chmod(webDir, 0o500); err != nil {
		t.Fatalf("chmod web dir: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed migrated iris session")
	}
	if resumed.UserEmail != "user@example.com" || resumed.ProviderID != 42 {
		t.Fatalf("unexpected resumed migrated iris session: %+v", resumed)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris session file to be removed after migration, stat err=%v", err)
	}
	if err := os.Chmod(webDir, 0o700); err != nil {
		t.Fatalf("restore web dir perms: %v", err)
	}
}

func TestTryResumeSessionMigratesLegacyIrisFileCacheKeepsUnrelatedLastMarker(t *testing.T) {
	withSessionInfoStub(t)
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(legacyDir, 0o700)
	})

	firstKey := webSessionCacheKey("first@example.com")
	secondKey := webSessionCacheKey("second@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "legacy-iris-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+firstKey+".json"), raw, 0o600); err != nil {
		t.Fatalf("write first legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+secondKey+".json"), raw, 0o600); err != nil {
		t.Fatalf("write second legacy iris session: %v", err)
	}
	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: secondKey})
	if err != nil {
		t.Fatalf("marshal unrelated last marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "last.json"), lastRaw, 0o600); err != nil {
		t.Fatalf("write unrelated last marker: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "first@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed migrated iris session")
	}

	lastKey, ok, err := readLegacyIrisLastKeyFromFile()
	if err != nil {
		t.Fatalf("readLegacyIrisLastKeyFromFile error: %v", err)
	}
	if !ok || lastKey != secondKey {
		t.Fatalf("expected unrelated legacy last marker to remain %q, got %q (ok=%v)", secondKey, lastKey, ok)
	}
}

func TestTryResumeSessionTreatsMalformedLegacyIrisFileCacheAsMiss(t *testing.T) {
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	sessionPath := filepath.Join(legacyDir, "session-"+key+".json")
	if err := os.WriteFile(sessionPath, []byte(`not-json`), 0o600); err != nil {
		t.Fatalf("write malformed legacy iris session: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if ok || resumed != nil {
		t.Fatalf("expected malformed legacy iris session to behave like cache miss, got ok=%v resumed=%v", ok, resumed)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("expected malformed legacy iris session removed, stat err=%v", err)
	}
}

func TestTryResumeLastSessionMigratesLegacyIrisLastFileCache(t *testing.T) {
	withSessionInfoStub(t)
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "legacy-last-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), raw, 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}

	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		t.Fatalf("marshal legacy iris last marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "last.json"), lastRaw, 0o600); err != nil {
		t.Fatalf("write legacy iris last marker: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed migrated iris last session")
	}
	if resumed.UserEmail != "user@example.com" || resumed.ProviderID != 42 {
		t.Fatalf("unexpected resumed migrated iris last session: %+v", resumed)
	}

	lastKey, ok, err := readLastKeyFromFile()
	if err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	}
	if !ok || lastKey != key {
		t.Fatalf("expected migrated last key %q, got %q (ok=%v)", key, lastKey, ok)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "last.json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris last-session marker to be removed after migration, stat err=%v", err)
	}
}

func TestTryResumeLastSessionTreatsMalformedLegacyIrisLastMarkerAsMiss(t *testing.T) {
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	lastPath := filepath.Join(legacyDir, "last.json")
	if err := os.WriteFile(lastPath, []byte(`not-json`), 0o600); err != nil {
		t.Fatalf("write malformed legacy iris last marker: %v", err)
	}

	resumed, ok, err := TryResumeLastSession(context.Background())
	if err != nil {
		t.Fatalf("TryResumeLastSession error: %v", err)
	}
	if ok || resumed != nil {
		t.Fatalf("expected malformed legacy iris last marker to behave like cache miss, got ok=%v resumed=%v", ok, resumed)
	}
	if _, err := os.Stat(lastPath); !os.IsNotExist(err) {
		t.Fatalf("expected malformed legacy iris last marker removed, stat err=%v", err)
	}
}

func TestTryResumeSessionMigratesLegacyIrisFileCacheKeepsResumedSessionWhenCleanupFails(t *testing.T) {
	withSessionInfoStub(t)
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "legacy-iris-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy iris session: %v", err)
	}
	sessionPath := filepath.Join(legacyDir, "session-"+key+".json")
	if err := os.WriteFile(sessionPath, raw, 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}
	if err := os.Chmod(legacyDir, 0o500); err != nil {
		t.Fatalf("chmod legacy dir: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed migrated iris session")
	}
	if resumed.UserEmail != "user@example.com" || resumed.ProviderID != 42 {
		t.Fatalf("unexpected resumed migrated iris session: %+v", resumed)
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("expected cleanup failure to leave legacy session file behind, stat err=%v", err)
	}
	if err := os.Chmod(legacyDir, 0o700); err != nil {
		t.Fatalf("restore legacy dir perms: %v", err)
	}
}

func TestTryResumeSessionDoesNotPersistExpiredLegacyIrisCache(t *testing.T) {
	webDir := filepath.Join(t.TempDir(), "web-cache")
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "expired-legacy-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), raw, 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}

	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		return nil, &sessionInfoStatusError{Status: http.StatusUnauthorized}
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err == nil {
		t.Fatal("expected expired cached-session error")
	}
	if !errors.Is(err, ErrCachedSessionExpired) {
		t.Fatalf("expected ErrCachedSessionExpired, got %v", err)
	}
	if errors.Unwrap(err) != nil {
		t.Fatalf("expected bare ErrCachedSessionExpired sentinel, got wrapped error %v", err)
	}
	if ok || resumed != nil {
		t.Fatalf("did not expect resumed legacy session, got %+v ok=%v", resumed, ok)
	}

	if _, ok, err := readSessionBySelection(resolveBackendSelection(), key); err != nil {
		t.Fatalf("readSessionBySelection error: %v", err)
	} else if ok {
		t.Fatal("did not expect expired legacy session to be persisted into web cache")
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected expired legacy iris session file to be removed, stat err=%v", err)
	}
}

func TestDeleteSessionRemovesLegacyIrisCache(t *testing.T) {
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), []byte(`{"version":1,"updated_at":"2026-03-16T00:00:00Z","cookies":{}}`), 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}
	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		t.Fatalf("marshal legacy iris last marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "last.json"), lastRaw, 0o600); err != nil {
		t.Fatalf("write legacy iris last marker: %v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris session file removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "last.json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris last marker removed, stat err=%v", err)
	}
}

func TestDeleteSessionIgnoresMalformedLegacyLastMarker(t *testing.T) {
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "off")
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), []byte(`{"version":1,"updated_at":"2026-03-16T00:00:00Z","cookies":{}}`), 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "last.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed legacy iris last marker: %v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris session file removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "last.json")); !os.IsNotExist(err) {
		t.Fatalf("expected malformed legacy iris last marker removed, stat err=%v", err)
	}
}

func TestDeleteAllSessionsRemovesLegacyIrisCache(t *testing.T) {
	legacyDir := filepath.Join(t.TempDir(), "iris-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyDir)

	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	key := webSessionCacheKey("user@example.com")
	if err := os.WriteFile(filepath.Join(legacyDir, "session-"+key+".json"), []byte(`{"version":1,"updated_at":"2026-03-16T00:00:00Z","cookies":{}}`), 0o600); err != nil {
		t.Fatalf("write legacy iris session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "last.json"), []byte(`{"version":1,"key":"`+key+`"}`), 0o600); err != nil {
		t.Fatalf("write legacy iris last marker: %v", err)
	}

	if err := DeleteAllSessions(); err != nil {
		t.Fatalf("DeleteAllSessions error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris session file removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "last.json")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy iris last marker removed, stat err=%v", err)
	}
}

func TestDeleteAllSessionsDefaultBackendIgnoresUnavailableKeychainFallback(t *testing.T) {
	withUnavailableSessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	if err := DeleteAllSessions(); err != nil {
		t.Fatalf("DeleteAllSessions error: %v", err)
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected file-backed sessions to be removed")
	}
	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected last-session marker to be removed")
	}
}

func TestDeleteAllSessionsKeychainBackendAlsoRemovesFileCache(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}
	if err := writeSessionToKeychain(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	if err := DeleteAllSessions(); err != nil {
		t.Fatalf("DeleteAllSessions error: %v", err)
	}

	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected mirrored file-backed sessions to be removed")
	}
	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected mirrored file-backed last marker to be removed")
	}
}

func TestDeleteAllSessionsKeychainBackendSurfacesMirroredFileDeleteError(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")

	webCachePath := filepath.Join(t.TempDir(), "web-cache-file")
	if err := os.WriteFile(webCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write web cache file: %v", err)
	}
	t.Setenv(webSessionCacheDirEnv, webCachePath)

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToKeychain(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	err := DeleteAllSessions()
	if err == nil {
		t.Fatal("expected mirrored file delete error")
	}
	if !strings.Contains(err.Error(), webCachePath) {
		t.Fatalf("expected mirrored file delete error to mention %q, got %v", webCachePath, err)
	}
	if _, ok, readErr := readSessionFromKeychain(key); readErr != nil {
		t.Fatalf("readSessionFromKeychain error: %v", readErr)
	} else if ok {
		t.Fatal("expected keychain-backed session store to still be removed")
	}
}

func TestDeleteSessionJoinsLegacyCleanupError(t *testing.T) {
	webCachePath := filepath.Join(t.TempDir(), "web-cache-file")
	if err := os.WriteFile(webCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write web cache file: %v", err)
	}
	legacyCachePath := filepath.Join(t.TempDir(), "legacy-cache-file")
	if err := os.WriteFile(legacyCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write legacy cache file: %v", err)
	}

	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webCachePath)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyCachePath)

	err := DeleteSession("user@example.com")
	if err == nil {
		t.Fatal("expected delete session error")
	}
	if !strings.Contains(err.Error(), webCachePath) {
		t.Fatalf("expected primary error to mention %q, got %v", webCachePath, err)
	}
	if !strings.Contains(err.Error(), legacyCachePath) {
		t.Fatalf("expected joined legacy cleanup error to mention %q, got %v", legacyCachePath, err)
	}
}

func TestDeleteAllSessionsJoinsLegacyCleanupError(t *testing.T) {
	webCachePath := filepath.Join(t.TempDir(), "web-cache-file")
	if err := os.WriteFile(webCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write web cache file: %v", err)
	}
	legacyCachePath := filepath.Join(t.TempDir(), "legacy-cache-file")
	if err := os.WriteFile(legacyCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write legacy cache file: %v", err)
	}

	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webCachePath)
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "1")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyCachePath)

	err := DeleteAllSessions()
	if err == nil {
		t.Fatal("expected delete-all sessions error")
	}
	if !strings.Contains(err.Error(), webCachePath) {
		t.Fatalf("expected primary error to mention %q, got %v", webCachePath, err)
	}
	if !strings.Contains(err.Error(), legacyCachePath) {
		t.Fatalf("expected joined legacy cleanup error to mention %q, got %v", legacyCachePath, err)
	}
}

func TestDeleteSessionSkipsLegacyCleanupWhenDisabled(t *testing.T) {
	legacyCachePath := filepath.Join(t.TempDir(), "legacy-cache-file")
	if err := os.WriteFile(legacyCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write legacy cache file: %v", err)
	}

	t.Setenv(webSessionBackendEnv, "off")
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "0")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyCachePath)

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("expected disabled legacy cleanup to be skipped, got %v", err)
	}
}

func TestDeleteAllSessionsSkipsLegacyCleanupWhenDisabled(t *testing.T) {
	legacyCachePath := filepath.Join(t.TempDir(), "legacy-cache-file")
	if err := os.WriteFile(legacyCachePath, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write legacy cache file: %v", err)
	}

	t.Setenv(webSessionBackendEnv, "off")
	t.Setenv(legacyIrisSessionCacheEnabledEnv, "0")
	t.Setenv(legacyIrisSessionCacheDirEnv, legacyCachePath)

	if err := DeleteAllSessions(); err != nil {
		t.Fatalf("expected disabled legacy cleanup to be skipped, got %v", err)
	}
}

func TestClearLastSessionMarkerDefaultBackendIgnoresUnavailableKeychainFallback(t *testing.T) {
	withUnavailableSessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	if err := clearLastSessionMarker(); err != nil {
		t.Fatalf("clearLastSessionMarker error: %v", err)
	}

	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile error: %v", err)
	} else if ok {
		t.Fatal("expected last-session marker to be removed")
	}
}

func persistedCookieValue(sess persistedSession, baseURL, cookieName string) string {
	list := sess.Cookies[baseURL]
	for _, cookie := range list {
		if cookie.Name == cookieName {
			return cookie.Value
		}
	}
	return ""
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
