package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestPersistSessionDoesNotEraseOtherAccountsWhenKeychainStoreIsMalformed(t *testing.T) {
	kr := withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: []byte("{")}); err != nil {
		t.Fatalf("seed malformed keychain store: %v", err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{{Name: "myacinfo", Value: "token", Path: "/"}})
	err = PersistSession(&AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	})
	if !errors.Is(err, errMalformedSessionStore) {
		t.Fatalf("PersistSession() error = %v, want malformed-store error", err)
	}
	item, err := kr.Get(webSessionStoreItem)
	if err != nil {
		t.Fatalf("read malformed keychain store: %v", err)
	}
	if string(item.Data) != "{" {
		t.Fatalf("malformed keychain store changed to %q", item.Data)
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

func TestNormalizePersistedCookieDeadlineHonorsMaxAgePrecedence(t *testing.T) {
	now := time.Date(2026, time.September, 17, 3, 0, 0, 0, time.UTC)
	observedAt := now.Add(-30 * time.Second)
	tests := []struct {
		name        string
		cookie      pCookie
		observedAt  time.Time
		wantOK      bool
		wantExpiry  time.Time
		wantMaxAge  int
		wantExpired bool
	}{
		{
			name: "positive max age overrides stale expires",
			cookie: pCookie{
				MaxAge:  60,
				Expires: now.Add(-time.Hour),
			},
			observedAt:  observedAt,
			wantOK:      true,
			wantExpiry:  observedAt.Add(time.Minute),
			wantMaxAge:  0,
			wantExpired: false,
		},
		{
			name: "negative max age overrides future expires",
			cookie: pCookie{
				MaxAge:  -1,
				Expires: now.Add(time.Hour),
			},
			observedAt:  observedAt,
			wantOK:      true,
			wantExpiry:  now.Add(time.Hour),
			wantMaxAge:  -1,
			wantExpired: true,
		},
		{
			name: "expires at current time",
			cookie: pCookie{
				Expires: now,
			},
			observedAt:  observedAt,
			wantOK:      true,
			wantExpiry:  now,
			wantExpired: true,
		},
		{
			name:   "positive max age without observation time",
			cookie: pCookie{MaxAge: 60},
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := normalizePersistedCookieDeadline(test.cookie, test.observedAt)
			if ok != test.wantOK {
				t.Fatalf("normalizePersistedCookieDeadline() ok = %t, want %t", ok, test.wantOK)
			}
			if !ok {
				return
			}
			if !got.Expires.Equal(test.wantExpiry) {
				t.Fatalf("Expires = %v, want %v", got.Expires, test.wantExpiry)
			}
			if got.MaxAge != test.wantMaxAge {
				t.Fatalf("MaxAge = %d, want %d", got.MaxAge, test.wantMaxAge)
			}
			if expired := isExpiredCookie(got, now); expired != test.wantExpired {
				t.Fatalf("isExpiredCookie() = %t, want %t", expired, test.wantExpired)
			}
		})
	}
}

func TestHydrateCookieJarDoesNotResetPersistedMaxAge(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name       string
		updatedAt  time.Time
		wantLoaded int
	}{
		{name: "remaining lifetime", updatedAt: now.Add(-30 * time.Second), wantLoaded: 1},
		{name: "elapsed lifetime", updatedAt: now.Add(-60 * time.Second), wantLoaded: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatalf("cookiejar.New() error: %v", err)
			}
			sess := persistedSession{
				Version:   webSessionCacheVersion,
				UpdatedAt: test.updatedAt,
				Cookies: map[string][]pCookie{
					"https://appstoreconnect.apple.com/": {{
						Name: "myacinfo", Value: "token", MaxAge: 60, Expires: now.Add(-time.Hour),
					}},
				},
			}
			if loaded := hydrateCookieJar(jar, sess); loaded != test.wantLoaded {
				t.Fatalf("hydrateCookieJar() = %d, want %d", loaded, test.wantLoaded)
			}
		})
	}
}

func TestHydrateCookieJarNarrowsLegacyParentDomainCookieToItsOrigin(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	sess := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "token", Value: "legacy", Path: "/olympus", Domain: ".apple.com",
			}},
		},
	}
	if loaded := hydrateCookieJar(jar, sess); loaded != 1 {
		t.Fatalf("hydrateCookieJar() = %d, want one cookie", loaded)
	}
	appEndpoint, _ := url.Parse(olympusSessionURL)
	developerRoot, _ := url.Parse("https://developer.apple.com/")
	if got := cookieValue(jar.Cookies(appEndpoint), "token"); got != "legacy" {
		t.Fatalf("app-store endpoint cookie = %q, want legacy", got)
	}
	if got := cookieValue(jar.Cookies(developerRoot), "token"); got != "" {
		t.Fatalf("developer endpoint received legacy parent-domain cookie %q", got)
	}
}

func TestHydrateCookieJarDropsConflictingHostParentAliases(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	now := time.Now().UTC()
	sess := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "host", Path: "/olympus", Expires: now.Add(time.Hour)},
				{Name: "token", Value: "parent", Path: "/olympus", Domain: ".apple.com", Expires: now.Add(2 * time.Hour)},
			},
		},
	}

	if loaded := hydrateCookieJar(jar, sess); loaded != 0 {
		t.Fatalf("hydrateCookieJar() = %d, want conflicting host/parent aliases rejected", loaded)
	}
}

func TestHydrateCookieJarRejectsEquivalentHostParentAliases(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	now := time.Now().UTC()
	cookie := pCookie{Name: "token", Value: "same", Path: "/olympus", Expires: now.Add(time.Hour)}
	parent := cookie
	parent.Domain = ".apple.com"
	sess := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {cookie, parent},
		},
	}

	if loaded := hydrateCookieJar(jar, sess); loaded != 0 {
		t.Fatalf("hydrateCookieJar() = %d, want aliases with different RFC scopes rejected", loaded)
	}
}

func TestSessionCookieTrackingJarScopesUpdatesByOriginAndPath(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	appURL, _ := url.Parse("https://appstoreconnect.apple.com/olympus/v1/session")
	appRoot, _ := url.Parse("https://appstoreconnect.apple.com/")
	developerRoot, _ := url.Parse("https://developer.apple.com/")
	oldExpiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	appExpiry := oldExpiry.Add(time.Hour)
	pathExpiry := oldExpiry.Add(2 * time.Hour)
	for _, target := range []*url.URL{appRoot, developerRoot} {
		jar.SetCookies(target, []*http.Cookie{{
			Name: "token", Value: "same", Path: "/", Expires: oldExpiry,
		}})
	}

	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(appURL, []*http.Cookie{{
		Name: "token", Value: "same", Path: "/", Expires: appExpiry,
	}})
	_, updates, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	if got, ok := updates[trackedCookieKey{origin: appRoot.String(), name: "token", value: "same"}]; !ok || !got.cookie.Expires.Equal(appExpiry) {
		t.Fatalf("app-store update = (%+v, %t), want expiry %v", got, ok, appExpiry)
	}
	if _, ok := updates[trackedCookieKey{origin: developerRoot.String(), name: "token", value: "same"}]; ok {
		t.Fatal("host-only app-store update was attributed to developer.apple.com")
	}

	// The source URL's default path is /olympus/v1, so this update does not
	// apply to the cached root cookie even though its name and value match.
	tracker.SetCookies(appURL, []*http.Cookie{{
		Name: "token", Value: "same", Expires: pathExpiry,
	}})
	_, updates, err = tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() after path update error: %v", err)
	}
	if got := updates[trackedCookieKey{origin: appRoot.String(), name: "token", value: "same"}].cookie.Expires; !got.Equal(appExpiry) {
		t.Fatalf("root-cookie expiry changed to %v after path-scoped update, want %v", got, appExpiry)
	}
}

func TestPreserveCachedCookieDeadlineDropsSessionOnlySameValueUpdate(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	oldExpiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	jar.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}})
	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/"}})
	serialized, updates, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	cached := persistedSession{
		UpdatedAt: oldExpiry.Add(-24 * time.Hour),
		Cookies:   map[string][]pCookie{target.String(): {{Name: "token", Value: "same", Expires: oldExpiry}}},
	}
	preserveCachedCookieDeadlines(&serialized, &cached, updates, oldExpiry.Add(-time.Hour))
	if got := serialized.Cookies[target.String()]; len(got) != 0 {
		t.Fatalf("session-only replacement persisted as %#v, want none", got)
	}
}

func TestPreserveCachedCookieDeadlineUsesLatestPersistentUpdate(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	oldExpiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	firstExpiry := oldExpiry.Add(time.Hour)
	latestExpiry := oldExpiry.Add(2 * time.Hour)
	jar.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}})
	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(target, []*http.Cookie{
		{Name: "token", Value: "same", Path: "/", Expires: firstExpiry},
		{Name: "token", Value: "same", Path: "/", Expires: latestExpiry},
	})
	serialized, updates, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	cached := persistedSession{
		UpdatedAt: oldExpiry.Add(-24 * time.Hour),
		Cookies:   map[string][]pCookie{target.String(): {{Name: "token", Value: "same", Expires: oldExpiry}}},
	}
	preserveCachedCookieDeadlines(&serialized, &cached, updates, oldExpiry.Add(-time.Hour))
	got := serialized.Cookies[target.String()][0]
	if !got.Expires.Equal(latestExpiry) {
		t.Fatalf("latest persistent update expiry = %v, want %v", got.Expires, latestExpiry)
	}
}

func TestPreserveCachedCookieDeadlineDoesNotCrossCookiePaths(t *testing.T) {
	now := time.Date(2026, time.September, 17, 3, 0, 0, 0, time.UTC)
	origin := "https://appstoreconnect.apple.com/"
	current := persistedSession{
		Cookies: map[string][]pCookie{
			origin: {{Name: "token", Value: "same"}},
		},
	}
	cached := persistedSession{
		UpdatedAt: now.Add(-time.Hour),
		Cookies: map[string][]pCookie{
			origin: {{Name: "token", Value: "same", Path: "/olympus/v1", Expires: now.Add(time.Hour)}},
		},
	}
	preserveCachedCookieDeadlines(&current, &cached, nil, now)
	if got := current.Cookies[origin][0].Expires; !got.IsZero() {
		t.Fatalf("root cookie inherited path-scoped expiry %v", got)
	}
}

func TestSerializeCookieJarDropsHostOnlyDomainScopeAmbiguity(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	oldExpiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	jar.SetCookies(target, []*http.Cookie{
		{Name: "token", Value: "host", Path: "/", Expires: oldExpiry},
		{Name: "token", Value: "domain", Domain: ".apple.com", Path: "/", Expires: oldExpiry},
	})
	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(target, []*http.Cookie{{
		Name: "token", Value: "host", Path: "/", Expires: oldExpiry.Add(time.Hour),
	}})
	serialized, _, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	if got := serialized.Cookies[target.String()]; len(got) != 0 {
		t.Fatalf("ambiguous host-only/domain cookies persisted as %#v, want none", got)
	}
}

func TestPersistSessionKeepsSessionOnlyUpdateNonPersistable(t *testing.T) {
	withFileSessionCache(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	oldExpiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	firstExpiry := oldExpiry.Add(time.Hour)
	jar.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}})
	tracker := newSessionCookieTrackingJar(jar)
	cached := persistedSession{
		UpdatedAt: oldExpiry.Add(-24 * time.Hour),
		Cookies:   map[string][]pCookie{target.String(): {{Name: "token", Value: "same", Expires: oldExpiry}}},
	}
	session := &AuthSession{
		Client:        &http.Client{Jar: tracker},
		UserEmail:     "user@example.com",
		cachedSession: &cached,
	}

	tracker.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: firstExpiry}})
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession(first) error: %v", err)
	}
	tracker.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/"}})
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession(second) error: %v", err)
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession(third) error: %v", err)
	}

	stored, ok, err := readSessionFromFile(webSessionCacheKey(session.UserEmail))
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	if got := stored.Cookies[target.String()]; len(got) != 0 {
		t.Fatalf("session-only replacement persisted after repeated saves as %#v, want none", got)
	}
	if session.cachedSession == nil || len(session.cachedSession.Cookies[target.String()]) != 0 {
		t.Fatal("successful persistence did not advance the cached baseline")
	}
}

func TestPersistSessionDoesNotAdvanceBaselineWhenPersistenceFails(t *testing.T) {
	withFileSessionCache(t)
	previousWrite := sessionFileWrite
	sessionFileWrite = func(string, *os.File, []byte, os.FileMode) error {
		return errors.New("injected session-cache write failure")
	}
	t.Cleanup(func() { sessionFileWrite = previousWrite })

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	oldExpiry := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	newExpiry := oldExpiry.Add(time.Hour)
	jar.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}})
	tracker := newSessionCookieTrackingJar(jar)
	cached := persistedSession{
		UpdatedAt: oldExpiry.Add(-24 * time.Hour),
		Cookies:   map[string][]pCookie{target.String(): {{Name: "token", Value: "same", Expires: oldExpiry}}},
	}
	session := &AuthSession{
		Client:        &http.Client{Jar: tracker},
		UserEmail:     "user@example.com",
		cachedSession: &cached,
	}
	tracker.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: newExpiry}})
	if err := PersistSession(session); err == nil {
		t.Fatal("PersistSession() unexpectedly succeeded")
	}
	if session.cachedSession == nil || !session.cachedSession.Cookies[target.String()][0].Expires.Equal(oldExpiry) {
		t.Fatal("failed persistence advanced the cached baseline")
	}
	_, updates, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("failed persistence cleared the pending tracker update")
	}
}

type blockingSessionCookieJar struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	cookie  *http.Cookie
}

func (j *blockingSessionCookieJar) Cookies(*url.URL) []*http.Cookie {
	if j.cookie == nil {
		return nil
	}
	return []*http.Cookie{{Name: j.cookie.Name, Value: j.cookie.Value}}
}

func (j *blockingSessionCookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.once.Do(func() { close(j.entered) })
	<-j.release
	if len(cookies) > 0 && cookies[len(cookies)-1] != nil {
		copy := *cookies[len(cookies)-1]
		j.cookie = &copy
	}
}

func TestSessionCookieTrackingJarSerializesWithSetCookies(t *testing.T) {
	underlying := &blockingSessionCookieJar{entered: make(chan struct{}), release: make(chan struct{})}
	tracker := newSessionCookieTrackingJar(underlying)
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	setDone := make(chan struct{})
	go func() {
		tracker.SetCookies(target, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: time.Now().Add(time.Hour)}})
		close(setDone)
	}()
	<-underlying.entered

	serializeDone := make(chan struct{})
	var serialized persistedSession
	var updates map[trackedCookieKey]trackedCookieUpdate
	var serializeErr error
	go func() {
		serialized, updates, serializeErr = tracker.serializeWithUpdates("user@example.com")
		close(serializeDone)
	}()
	select {
	case <-serializeDone:
		t.Fatal("serializeWithUpdates completed while SetCookies was still in progress")
	case <-time.After(50 * time.Millisecond):
	}
	close(underlying.release)
	<-setDone
	<-serializeDone
	if serializeErr != nil {
		t.Fatalf("serializeWithUpdates() error: %v", serializeErr)
	}
	if serialized.UserEmail != "user@example.com" {
		t.Fatalf("serialized user email = %q, want user@example.com", serialized.UserEmail)
	}
	if len(updates) == 0 {
		t.Fatal("serialized update snapshot lost the completed SetCookies update")
	}
}

func TestLoginWithClientTracksFreshMaxAgeForFirstPersistence(t *testing.T) {
	withFileSessionCache(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	var responseAt time.Time
	client := &http.Client{
		Jar: jar,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			responseAt = time.Now().UTC()
			header := make(http.Header)
			header.Set("Set-Cookie", "myacinfo=fresh-token; Max-Age=60; Expires=Wed, 01 Jan 2020 00:00:00 GMT; Path=/")
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader("unavailable")),
				Request:    req,
			}, nil
		}),
	}
	if _, err := LoginWithClient(context.Background(), client, LoginCredentials{
		Username: "user@example.com", Password: "fixture-password",
	}); err == nil {
		t.Fatal("LoginWithClient() unexpectedly succeeded")
	}
	if _, ok := client.Jar.(*sessionCookieTrackingJar); !ok {
		t.Fatalf("LoginWithClient() jar type = %T, want tracked jar", client.Jar)
	}

	if err := PersistSession(&AuthSession{Client: client, UserEmail: "user@example.com"}); err != nil {
		t.Fatalf("PersistSession() error: %v", err)
	}
	stored, ok, err := readSessionFromFile(webSessionCacheKey("user@example.com"))
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 {
		t.Fatalf("stored cookies = %#v, want one cookie", cookies)
	}
	if !cookies[0].Expires.After(responseAt.Add(50*time.Second)) || !cookies[0].Expires.Before(responseAt.Add(70*time.Second)) {
		t.Fatalf("fresh Max-Age deadline = %v, want approximately %v", cookies[0].Expires, responseAt.Add(60*time.Second))
	}
}

func TestSerializeCookieJarIncludesDeveloperPortalOrigin(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	developerURL, err := url.Parse("https://developer.apple.com/")
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}
	jar.SetCookies(developerURL, []*http.Cookie{
		{Name: "myacinfo", Value: "developer-session", Secure: true},
	})

	serialized := serializeCookieJar(jar, "user@example.com")
	if got := persistedMyacinfoCookieValue(serialized, "https://developer.apple.com/"); got != "developer-session" {
		t.Fatalf("developer portal cookie = %q, want persisted session", got)
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

func TestPersistSessionRoundTripsDeveloperTeamID(t *testing.T) {
	withSessionInfoStub(t)
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, t.TempDir())

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: "token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})
	if err := PersistSession(&AuthSession{
		Client:          &http.Client{Jar: jar},
		UserEmail:       "user@example.com",
		DeveloperTeamID: "TEAMTWO456",
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	resumed, ok, err := TryResumeSession(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed session")
	}
	if resumed.DeveloperTeamID != "TEAMTWO456" {
		t.Fatalf("DeveloperTeamID = %q, want TEAMTWO456", resumed.DeveloperTeamID)
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

	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "refreshed-token" {
		t.Fatalf("expected refreshed cookie value, got %q", got)
	}
}

func TestTryResumeSessionPersistsRefreshedOlympusPathCookieWithoutBroadeningScope(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	oldExpiry := time.Now().UTC().Add(24 * time.Hour)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "stale-token", Path: "/olympus",
				Domain: "appstoreconnect.apple.com", Expires: oldExpiry,
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		endpoint, err := url.Parse(olympusSessionURL)
		if err != nil {
			return nil, err
		}
		if got := cookieValue(client.Jar.Cookies(endpoint), "myacinfo"); got != "stale-token" {
			return nil, fmt.Errorf("session endpoint received cookie %q, want stale-token", got)
		}
		client.Jar.SetCookies(endpoint, []*http.Cookie{{
			Name: "myacinfo", Value: "refreshed-token", Path: "/olympus",
			Domain: "appstoreconnect.apple.com", Expires: time.Now().UTC().Add(72 * time.Hour),
			Secure: true, HttpOnly: true,
		}})
		out := &sessionInfo{}
		out.Provider.ProviderID = 42
		out.User.EmailAddress = webTestSessionEmail
		return out, nil
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	resumed, ok, err := TryResumeSession(context.Background(), webTestSessionEmail)
	if err != nil {
		t.Fatalf("TryResumeSession error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatal("expected resumed session")
	}

	root, _ := url.Parse("https://appstoreconnect.apple.com/")
	if got := resumed.Client.Jar.Cookies(root); len(got) != 0 {
		t.Fatalf("path-scoped cookie leaked to app root: %#v", got)
	}

	stored, ok, err := readSessionFromFile(key)
	if err != nil {
		t.Fatalf("readSessionFromFile error: %v", err)
	}
	if !ok {
		t.Fatal("expected resumed session in cache")
	}
	var got pCookie
	var found int
	for _, cookies := range stored.Cookies {
		for _, cookie := range cookies {
			if cookie.Name == "myacinfo" {
				got = cookie
				found++
			}
		}
	}
	if found != 1 {
		t.Fatalf("persisted myacinfo count = %d, want one cookie: %#v", found, stored.Cookies)
	}
	if got.Value != "refreshed-token" || got.Path != "/olympus" || got.Domain != "" {
		t.Fatalf("persisted cookie scope/value = %+v, want refreshed-token at /olympus with a host-only domain", got)
	}
}

func TestPersistSessionPreservesUntouchedOlympusPathCookieScope(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "path-token", Path: "/olympus",
				Domain: "appstoreconnect.apple.com", Expires: time.Now().UTC().Add(24 * time.Hour),
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want cached session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Name != "myacinfo" || cookies[0].Value != "path-token" ||
		cookies[0].Path != "/olympus" || cookies[0].Domain != "" {
		t.Fatalf("persisted path cookie = %#v, want its original value and narrowed host-only scope", cookies)
	}
	root, _ := url.Parse("https://appstoreconnect.apple.com/")
	if got := loaded.Client.Jar.Cookies(root); len(got) != 0 {
		t.Fatalf("path-scoped cookie leaked to app root: %#v", got)
	}
}

func TestPersistSessionKeepsFreshSessionOnlyOlympusPathCookie(t *testing.T) {
	withFileSessionCache(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(endpoint, []*http.Cookie{{Name: "myacinfo", Value: "fresh-path", Path: "/olympus"}})
	session := &AuthSession{
		Client:    &http.Client{Jar: tracker},
		UserEmail: webTestSessionEmail,
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession(first) error: %v", err)
	}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession(second) error: %v", err)
	}

	stored, ok, err := readSessionFromFile(webSessionCacheKey(webTestSessionEmail))
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Name != "myacinfo" || cookies[0].Value != "fresh-path" || cookies[0].Path != "/olympus" {
		t.Fatalf("persisted fresh session-only path cookie = %#v, want one /olympus cookie after repeated saves", cookies)
	}
}

func TestPersistSessionDropsSessionOnlyOlympusReplacement(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "cached-path", Path: "/olympus", Expires: time.Now().UTC().Add(time.Hour),
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}
	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{{Name: "myacinfo", Value: "rotated-path", Path: "/olympus"}})
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession(first) error: %v", err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession(second) error: %v", err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	if got := stored.Cookies["https://appstoreconnect.apple.com/"]; len(got) != 0 {
		t.Fatalf("session-only Olympus replacement persisted as %#v, want none", got)
	}
}

func TestPersistSessionDoesNotResurrectDeletedOlympusCookieWhenRootValueMatches(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	cached := persistedSession{
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "same", Path: "/", Expires: time.Now().UTC().Add(time.Hour)},
				{Name: "token", Value: "same", Path: "/olympus", Expires: time.Now().UTC().Add(time.Hour)},
			},
		},
	}
	hydrateCookieJar(jar, cached)
	tracker := newSessionCookieTrackingJarWithCached(jar, &cached)
	endpoint, _ := url.Parse(olympusSessionURL)
	tracker.SetCookies(endpoint, []*http.Cookie{{Name: "token", Path: "/olympus", MaxAge: -1}})

	serialized, updates, err := tracker.serializeWithUpdates(webTestSessionEmail)
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	preserveCachedCookieDeadlines(&serialized, &cached, updates, time.Now().UTC())
	for _, cookie := range serialized.Cookies["https://appstoreconnect.apple.com/"] {
		if cookie.Name == "token" && persistedCookiePath(cookie.Path) == "/olympus" {
			t.Fatalf("deleted Olympus cookie was resurrected: %#v", serialized.Cookies)
		}
	}
}

func TestPersistSessionDoesNotRetainRotatedOlympusCookieWhenRootValueMatches(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	cached := persistedSession{
		UpdatedAt: time.Now().UTC(),
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "token-a", Path: "/", Expires: time.Now().UTC().Add(time.Hour)},
				{Name: "token", Value: "token-a", Path: "/olympus", Expires: time.Now().UTC().Add(time.Hour)},
			},
		},
	}
	hydrateCookieJar(jar, cached)
	tracker := newSessionCookieTrackingJarWithCached(jar, &cached)
	endpoint, _ := url.Parse(olympusSessionURL)
	tracker.SetCookies(endpoint, []*http.Cookie{
		{Name: "token", Value: "token-b", Path: "/olympus", Expires: time.Now().UTC().Add(2 * time.Hour)},
	})

	serialized, updates, err := tracker.serializeWithUpdates(webTestSessionEmail)
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	preserveCachedCookieDeadlines(&serialized, &cached, updates, time.Now().UTC())
	var got []pCookie
	for _, cookie := range serialized.Cookies["https://appstoreconnect.apple.com/"] {
		if cookie.Name == "token" && persistedCookiePath(cookie.Path) == "/olympus" {
			got = append(got, cookie)
		}
	}
	if len(got) != 1 || got[0].Value != "token-b" {
		t.Fatalf("rotated Olympus cookies = %#v, want only token-b", got)
	}
}

func TestSessionCookieTrackingJarKeepsRootAndOlympusUpdatesSeparate(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	root, _ := url.Parse("https://appstoreconnect.apple.com/")
	endpoint, _ := url.Parse(olympusSessionURL)
	rootExpiry := time.Now().UTC().Add(time.Hour)
	pathExpiry := rootExpiry.Add(time.Hour)
	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(root, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: rootExpiry}})
	tracker.SetCookies(endpoint, []*http.Cookie{{Name: "token", Value: "same", Path: "/olympus", Expires: pathExpiry}})
	serialized, updates, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	preserveCachedCookieDeadlines(&serialized, nil, updates, time.Now().UTC())
	cookies := serialized.Cookies[root.String()]
	if len(cookies) != 2 {
		t.Fatalf("persisted cookies = %#v, want root and Olympus scopes", cookies)
	}
	for _, cookie := range cookies {
		switch persistedCookiePath(cookie.Path) {
		case "/":
			if !cookie.Expires.Equal(rootExpiry) {
				t.Fatalf("root expiry = %v, want %v", cookie.Expires, rootExpiry)
			}
		case "/olympus":
			if !cookie.Expires.Equal(pathExpiry) {
				t.Fatalf("Olympus expiry = %v, want %v", cookie.Expires, pathExpiry)
			}
		default:
			t.Fatalf("unexpected persisted path %q", cookie.Path)
		}
	}
}

func TestPersistSessionKeepsUntouchedOlympusScopeWhenRootRenews(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	oldExpiry := time.Now().UTC().Add(time.Hour)
	rootExpiry := oldExpiry.Add(time.Hour)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "same", Path: "/", Expires: oldExpiry},
				{Name: "token", Value: "same", Path: "/olympus", Expires: oldExpiry},
			},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}
	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	root, _ := url.Parse("https://appstoreconnect.apple.com/")
	loaded.Client.Jar.SetCookies(root, []*http.Cookie{{Name: "token", Value: "same", Path: "/", Expires: rootExpiry}})
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession() error: %v", err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	cookies := stored.Cookies[root.String()]
	if len(cookies) != 2 {
		t.Fatalf("persisted cookies = %#v, want root and untouched Olympus scopes", cookies)
	}
	for _, cookie := range cookies {
		switch persistedCookiePath(cookie.Path) {
		case "/":
			if !cookie.Expires.Equal(rootExpiry) {
				t.Fatalf("root expiry = %v, want %v", cookie.Expires, rootExpiry)
			}
		case "/olympus":
			if !cookie.Expires.Equal(oldExpiry) {
				t.Fatalf("untouched Olympus expiry = %v, want %v", cookie.Expires, oldExpiry)
			}
		default:
			t.Fatalf("unexpected persisted path %q", cookie.Path)
		}
	}
}

func TestPersistSessionPreservesUntouchedOlympusParentDomainCookie(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "parent-token", Path: "/olympus",
				Domain: ".apple.com", Expires: time.Now().UTC().Add(24 * time.Hour),
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want cached session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Value != "parent-token" || cookies[0].Path != "/olympus" ||
		cookies[0].Domain != "" || cookies[0].ScopeDomain != "apple.com" {
		t.Fatalf("persisted parent-domain cookie = %#v, want a narrowed host-only Domain with apple.com scope provenance", cookies)
	}
	bundle, ok, err := ExportSessionBundle(webTestSessionEmail)
	if err != nil || !ok || bundle == nil {
		t.Fatalf("ExportSessionBundle() = (%+v, %t, %v), want the narrowed cached session", bundle, ok, err)
	}
	if err := bundle.Validate(); err != nil {
		t.Fatalf("narrowed exported bundle failed validation: %v", err)
	}
}

func TestPersistSessionDoesNotResurrectParentDomainCookieDeletedAfterNarrowing(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "parent-token", Path: "/olympus",
				Domain: ".apple.com", Expires: time.Now().UTC().Add(24 * time.Hour),
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}

	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession(first) error: %v", err)
	}

	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{{
		Name: "myacinfo", Path: "/olympus", Domain: ".apple.com", MaxAge: -1,
	}})
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession(second) error: %v", err)
	}

	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	for _, cookie := range stored.Cookies["https://appstoreconnect.apple.com/"] {
		if cookie.Name == "myacinfo" && persistedCookiePath(cookie.Path) == "/olympus" {
			t.Fatalf("deleted parent-domain cookie was resurrected after narrowing: %#v", stored.Cookies)
		}
	}
}

func TestPersistSessionDropsSessionOnlyParentDomainReplacementAfterNarrowing(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "parent-token", Path: "/olympus",
				Domain: ".apple.com", Expires: time.Now().UTC().Add(24 * time.Hour),
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}

	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession(first) error: %v", err)
	}

	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{{
		Name: "myacinfo", Value: "rotated-token", Path: "/olympus", Domain: ".apple.com",
	}})
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession(second) error: %v", err)
	}

	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	for _, cookie := range stored.Cookies["https://appstoreconnect.apple.com/"] {
		if cookie.Name == "myacinfo" && persistedCookiePath(cookie.Path) == "/olympus" {
			t.Fatalf("session-only parent-domain replacement persisted after narrowing: %#v", stored.Cookies)
		}
	}
}

func TestPersistSessionAppliesParentDomainRenewalAfterNarrowing(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	oldExpiry := time.Now().UTC().Add(time.Hour)
	newExpiry := oldExpiry.Add(time.Hour)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: webTestSessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "parent-token", Path: "/olympus", Domain: ".apple.com", Expires: oldExpiry,
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}
	loaded, ok, err := LoadCachedSession(webTestSessionEmail)
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%+v, %t, %v), want cached session", loaded, ok, err)
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{
		{Name: "myacinfo", Value: "parent-token", Path: "/olympus", Domain: ".apple.com", Expires: newExpiry},
	})
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession() error: %v", err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%t, %v), want stored session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Domain != "" || cookies[0].ScopeDomain != "apple.com" || !cookies[0].Expires.Equal(newExpiry) {
		t.Fatalf("renewed parent-domain cookie = %#v, want narrowed cookie with apple.com scope provenance and expiry %v", cookies, newExpiry)
	}
}

func TestPersistSessionPreservesHostOnlyCookieWhenParentDomainTombstoneArrives(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	now := time.Now().UTC()
	cached := persistedSession{
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "token", Value: "host-only", Path: "/olympus", Expires: now.Add(time.Hour),
			}},
		},
	}
	if loaded := hydrateCookieJar(jar, cached); loaded != 1 {
		t.Fatalf("hydrateCookieJar() = %d, want one host-only cookie", loaded)
	}

	tracker := newSessionCookieTrackingJarWithCached(jar, &cached)
	endpoint, _ := url.Parse(olympusSessionURL)
	tracker.SetCookies(endpoint, []*http.Cookie{{
		Name: "token", Path: "/olympus", Domain: ".apple.com", MaxAge: -1,
	}})
	serialized, updates, err := tracker.serializeWithUpdates(webTestSessionEmail)
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	preserveCachedCookieDeadlines(&serialized, &cached, updates, now)
	cookies := serialized.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Name != "token" || cookies[0].Value != "host-only" ||
		cookies[0].Path != "/olympus" || cookies[0].Domain != "" {
		t.Fatalf("persisted cookies after parent-domain tombstone = %#v, want untouched host-only cookie", cookies)
	}
}

func TestSessionCookieTrackingJarPersistsDefaultOlympusCookiePath(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	tracker := newSessionCookieTrackingJar(jar)
	endpoint, _ := url.Parse(olympusSessionURL)
	tracker.SetCookies(endpoint, []*http.Cookie{{Name: "myacinfo", Value: "default-path", Expires: time.Now().UTC().Add(time.Hour)}})
	serialized, _, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	cookies := serialized.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Path != "/olympus/v1" {
		t.Fatalf("default-path cookie = %#v, want /olympus/v1 scope", cookies)
	}
}

func TestPreserveCachedCookieDeadlineDoesNotCrossAppleHosts(t *testing.T) {
	appRoot := "https://appstoreconnect.apple.com/"
	developerRoot := "https://developer.apple.com/"
	oldExpiry := time.Date(2026, time.September, 18, 3, 0, 0, 0, time.UTC)
	newExpiry := oldExpiry.Add(time.Hour)
	current := persistedSession{Cookies: map[string][]pCookie{
		appRoot: {{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}},
	}}
	updates := map[trackedCookieKey]trackedCookieUpdate{
		{origin: developerRoot, name: "token", value: "same"}: {
			cookie: pCookie{Name: "token", Value: "same", Path: "/", Expires: newExpiry},
		},
	}
	preserveCachedCookieDeadlines(&current, nil, updates, oldExpiry.Add(-time.Hour))
	if got := current.Cookies[appRoot][0].Expires; !got.Equal(oldExpiry) {
		t.Fatalf("app-store cookie expiry = %v, want unrelated developer update ignored", got)
	}
}

func TestPreserveCachedCookieDeadlineDoesNotApplyOlympusUpdateToOtherAppleHost(t *testing.T) {
	developerRoot := "https://developer.apple.com/"
	oldExpiry := time.Date(2026, time.September, 18, 3, 0, 0, 0, time.UTC)
	newExpiry := oldExpiry.Add(time.Hour)
	current := persistedSession{Cookies: map[string][]pCookie{
		developerRoot: {{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}},
	}}
	updates := map[trackedCookieKey]trackedCookieUpdate{
		{origin: olympusSessionURL, name: "token", value: "same"}: {
			cookie: pCookie{Name: "token", Value: "same", Path: "/", Expires: newExpiry},
		},
	}
	preserveCachedCookieDeadlines(&current, &persistedSession{Cookies: map[string][]pCookie{
		developerRoot: {{Name: "token", Value: "same", Path: "/", Expires: oldExpiry}},
	}}, updates, oldExpiry.Add(-time.Hour))
	if got := current.Cookies[developerRoot][0].Expires; !got.Equal(oldExpiry) {
		t.Fatalf("developer cookie expiry = %v, want unrelated Olympus update ignored", got)
	}
}

func TestPreserveTrackedCookieScopesKeepsDistinctRootAndOlympusCookies(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	root, _ := url.Parse("https://appstoreconnect.apple.com/")
	endpoint, _ := url.Parse(olympusSessionURL)
	rootExpiry := time.Now().UTC().Add(time.Hour)
	pathExpiry := rootExpiry.Add(time.Hour)
	jar.SetCookies(root, []*http.Cookie{{Name: "token", Value: "root", Path: "/", Expires: rootExpiry}})
	tracker := newSessionCookieTrackingJar(jar)
	tracker.SetCookies(endpoint, []*http.Cookie{{Name: "token", Value: "path", Path: "/olympus", Expires: pathExpiry}})
	serialized, updates, err := tracker.serializeWithUpdates("user@example.com")
	if err != nil {
		t.Fatalf("serializeWithUpdates() error: %v", err)
	}
	preserveCachedCookieDeadlines(&serialized, nil, updates, time.Now().UTC())
	cookies := serialized.Cookies[root.String()]
	if len(cookies) != 2 {
		t.Fatalf("persisted cookies = %#v, want distinct root and Olympus cookies", cookies)
	}
	paths := map[string]bool{}
	for _, cookie := range cookies {
		paths[persistedCookiePath(cookie.Path)] = true
	}
	if !paths["/"] || !paths["/olympus"] {
		t.Fatalf("persisted paths = %#v, want / and /olympus", cookies)
	}
}

func TestPersistedCookieScopeNormalizesLeadingDotDomain(t *testing.T) {
	root := pCookie{Name: "token", Value: "same", Path: "/olympus", Domain: "appstoreconnect.apple.com"}
	leadingDot := root
	leadingDot.Domain = ".appstoreconnect.apple.com"
	if !persistedCookieListHasScope([]pCookie{root}, leadingDot) {
		t.Fatal("equivalent leading-dot domain was treated as a distinct cookie scope")
	}
}

func TestCachedCookieForProbeRejectsConflictingDeadlinesForSameScope(t *testing.T) {
	now := time.Now().UTC()
	cached := persistedSession{
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "same", Path: "/olympus", Domain: "appstoreconnect.apple.com", Expires: now.Add(time.Hour)},
				{Name: "token", Value: "same", Path: "/olympus", Domain: ".appstoreconnect.apple.com", Expires: now.Add(2 * time.Hour)},
			},
		},
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	if _, ok := cachedCookieForProbe(&cached, endpoint, "token", "same"); ok {
		t.Fatal("same-scope cookies with conflicting deadlines were accepted")
	}
}

func TestCachedCookieForProbeRejectsConflictingHostParentAliases(t *testing.T) {
	now := time.Now().UTC()
	cached := persistedSession{
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "same", Path: "/olympus", Expires: now.Add(time.Hour)},
				{Name: "token", Value: "same", Path: "/olympus", Domain: ".apple.com", Expires: now.Add(2 * time.Hour)},
			},
		},
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	if _, ok := cachedCookieForProbe(&cached, endpoint, "token", "same"); ok {
		t.Fatal("host-only and parent-domain aliases with conflicting deadlines were accepted")
	}
}

func TestCachedCookieForProbeRejectsEquivalentHostParentAliases(t *testing.T) {
	now := time.Now().UTC()
	expires := now.Add(time.Hour)
	cached := persistedSession{
		UpdatedAt: now,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "token", Value: "same", Path: "/olympus", Expires: expires},
				{Name: "token", Value: "same", Path: "/olympus", Domain: ".apple.com", Expires: expires},
			},
		},
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	if _, ok := cachedCookieForProbe(&cached, endpoint, "token", "same"); ok {
		t.Fatal("equivalent host-only and parent-domain aliases were resolved by input order")
	}
}

func TestCookieScopeMatchingKeepsHostOnlyAndParentDomainDistinct(t *testing.T) {
	endpoint, _ := url.Parse(olympusSessionURL)
	hostOnly := pCookie{Name: "token", Value: "same", Path: "/olympus"}
	parent := hostOnly
	parent.Domain = ".apple.com"
	if cookieScopesMatchForOrigin(endpoint.String(), hostOnly, parent) {
		t.Fatal("host-only and parent-domain cookies were treated as one RFC scope")
	}
	updates := map[trackedCookieKey]trackedCookieUpdate{
		trackedCookieKeyForCookie(endpoint.String(), parent): {cookie: parent},
	}
	if trackedCookieScopeUpdatedForOrigin(updates, endpoint.String(), hostOnly) {
		t.Fatal("parent-domain renewal was treated as a host-only update")
	}
	if _, ok := trackedCookieUpdateForPersistedCookie(updates, endpoint.String(), hostOnly); ok {
		t.Fatal("parent-domain renewal matched a host-only persisted cookie")
	}
}

func TestDropAmbiguousPersistedCookiesRejectsEquivalentRecordsWithDifferentScopeProvenance(t *testing.T) {
	origin, _ := url.Parse("https://appstoreconnect.apple.com/")
	now := time.Now().UTC()
	hostOnly := pCookie{Name: "token", Value: "same", Path: "/olympus", Expires: now.Add(time.Hour)}
	parent := hostOnly
	parent.Domain = ".apple.com"
	got := dropAmbiguousPersistedCookies([]pCookie{
		hostOnly,
		narrowCookieDomainForOrigin(origin, parent),
	})
	if len(got) != 0 {
		t.Fatalf("equivalent narrowed aliases = %#v, want differing RFC scopes rejected", got)
	}
}

func TestDropAmbiguousPersistedCookiesDropsConflictingRecords(t *testing.T) {
	origin, _ := url.Parse("https://appstoreconnect.apple.com/")
	now := time.Now().UTC()
	hostOnly := pCookie{Name: "token", Value: "host", Path: "/olympus", Expires: now.Add(time.Hour)}
	parent := hostOnly
	parent.Value = "parent"
	parent.Domain = ".apple.com"
	got := dropAmbiguousPersistedCookies([]pCookie{
		hostOnly,
		narrowCookieDomainForOrigin(origin, parent),
	})
	if len(got) != 0 {
		t.Fatalf("conflicting narrowed aliases = %#v, want all records dropped", got)
	}
}

func TestTryResumeSessionFailedRefreshPreservesNewerReplacementOnCleanup(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey(webTestSessionEmail)
	stale := webTestPersistedSession(t, "stale-token", time.Now().UTC().Add(-time.Minute))
	if err := writeSessionToFile(key, stale); err != nil {
		t.Fatalf("write stale session: %v", err)
	}

	previousWrite := sessionFileWrite
	sessionFileWrite = func(string, *os.File, []byte, os.FileMode) error {
		return errors.New("injected refresh persistence failure")
	}
	resumed, ok, err := TryResumeSession(context.Background(), webTestSessionEmail)
	sessionFileWrite = previousWrite
	if err != nil || !ok || resumed == nil {
		t.Fatalf("TryResumeSession() = (%v, %t, %v), want resumed session", resumed, ok, err)
	}

	fresh := webTestPersistedSession(t, "fresh-token", stale.UpdatedAt.Add(2*time.Minute))
	if err := writeSessionToFile(key, fresh); err != nil {
		t.Fatalf("write replacement session: %v", err)
	}
	deleted, err := DeleteSessionIfMatches(webTestSessionEmail, resumed)
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches() error: %v", err)
	}
	if deleted {
		t.Fatal("failed refresh cleanup deleted a newer replacement")
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("read replacement session = (%t, %v), want stored session", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "fresh-token" {
		t.Fatalf("replacement cookie = %q, want fresh-token", got)
	}
}

func TestTryResumeSessionPreservesNormalizedCookieDeadline(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	observedAt := time.Now().UTC().Add(-30 * time.Second)
	wantExpiry := observedAt.Add(time.Minute)
	sess := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: observedAt,
		UserEmail: "user@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "token", MaxAge: 60, Expires: observedAt.Add(-time.Hour),
			}},
		},
	}
	key := webSessionCacheKey(sess.UserEmail)
	if err := writeSessionToKeychain(key, sess); err != nil {
		t.Fatalf("writeSessionToKeychain() error: %v", err)
	}

	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
		info := &sessionInfo{}
		info.User.EmailAddress = sess.UserEmail
		return info, nil
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	if resumed, ok, err := TryResumeSession(context.Background(), sess.UserEmail); err != nil || !ok || resumed == nil {
		t.Fatalf("TryResumeSession() = (%v, %t, %v), want resumed session", resumed, ok, err)
	}
	stored, ok, err := readSessionBySelection(resolveBackendSelection(), key)
	if err != nil || !ok {
		t.Fatalf("readSessionBySelection() = (%t, %v), want stored session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 {
		t.Fatalf("stored cookies = %v, want one cookie", cookies)
	}
	if cookies[0].MaxAge != 0 || !cookies[0].Expires.Equal(wantExpiry) {
		t.Fatalf("stored deadline = Expires %v MaxAge %d, want Expires %v MaxAge 0", cookies[0].Expires, cookies[0].MaxAge, wantExpiry)
	}
}

func TestTryResumeSessionPersistsSameValueCookieRenewalDeadline(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	observedAt := time.Now().UTC().Add(-30 * time.Second)
	refreshedExpiry := observedAt.Add(24 * time.Hour)
	sess := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: observedAt,
		UserEmail: "user@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "token", MaxAge: 60, Expires: observedAt.Add(-time.Hour),
			}},
		},
	}
	key := webSessionCacheKey(sess.UserEmail)
	if err := writeSessionToKeychain(key, sess); err != nil {
		t.Fatalf("writeSessionToKeychain() error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(_ context.Context, client *http.Client) (*sessionInfo, error) {
		client.Jar.SetCookies(targetURL, []*http.Cookie{{
			Name: "myacinfo", Value: "token", Path: "/", Expires: refreshedExpiry,
		}})
		info := &sessionInfo{}
		info.User.EmailAddress = sess.UserEmail
		return info, nil
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	if resumed, ok, err := TryResumeSession(context.Background(), sess.UserEmail); err != nil || !ok || resumed == nil {
		t.Fatalf("TryResumeSession() = (%v, %t, %v), want resumed session", resumed, ok, err)
	}
	stored, ok, err := readSessionBySelection(resolveBackendSelection(), key)
	if err != nil || !ok {
		t.Fatalf("readSessionBySelection() = (%t, %v), want stored session", ok, err)
	}
	cookies := stored.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 {
		t.Fatalf("stored cookies = %v, want one cookie", cookies)
	}
	if cookies[0].MaxAge != 0 || !cookies[0].Expires.Equal(refreshedExpiry) {
		t.Fatalf("stored deadline = Expires %v MaxAge %d, want refreshed Expires %v MaxAge 0", cookies[0].Expires, cookies[0].MaxAge, refreshedExpiry)
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
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "new-token" {
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

func TestReadLastKeyFromFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	t.Setenv(webSessionCacheDirEnv, dir)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	last := persistedLastSession{Version: webSessionCacheVersion, Key: webSessionCacheKey("outside@example.com")}
	raw, err := json.Marshal(last)
	if err != nil {
		t.Fatalf("marshal last marker: %v", err)
	}
	targetPath := filepath.Join(outside, "last.json")
	if err := os.WriteFile(targetPath, raw, 0o600); err != nil {
		t.Fatalf("write target last marker: %v", err)
	}
	cachePath := filepath.Join(dir, "last.json")
	if err := os.Symlink(targetPath, cachePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	key, ok, err := readLastKeyFromFile()
	if err == nil {
		t.Fatalf("readLastKeyFromFile() = (%q, %t, nil); want symlink rejection", key, ok)
	}
	if key != "" || ok {
		t.Fatalf("readLastKeyFromFile() = (%q, %t, %v), want no key", key, ok, err)
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

func TestTryResumeSessionFromSourceFallsBackToKeychainWhenFileSessionIsRejectedByServer(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, webTestPersistedSession(t, "file-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}
	if err := writeSessionToKeychain(key, webTestPersistedSession(t, "keychain-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToKeychain() error: %v", err)
	}

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if cookie, err := r.Cookie("myacinfo"); err == nil {
			token = cookie.Value
		}
		requests = append(requests, token)
		switch token {
		case "file-token":
			w.WriteHeader(http.StatusUnauthorized)
		case "keychain-token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"provider":{"providerId":42},"user":{"emailAddress":"user@example.com"}}`)
		default:
			t.Fatalf("unexpected session cookie %q", token)
		}
	}))
	t.Cleanup(server.Close)

	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		if err != nil {
			return nil, err
		}
		validationURL, err := url.Parse(olympusSessionURL)
		if err != nil {
			return nil, err
		}
		for _, cookie := range client.Jar.Cookies(validationURL) {
			req.AddCookie(cookie)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, &sessionInfoStatusError{Status: resp.StatusCode}
		}
		var info sessionInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return nil, err
		}
		return &info, nil
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	appleID, source, err := DefaultCachedAppleIDWithSource()
	if err != nil {
		t.Fatalf("DefaultCachedAppleIDWithSource() error: %v", err)
	}
	if appleID != webTestSessionEmail || source != CachedSessionSourceFile {
		t.Fatalf("default cache selection = (%q, %v), want (%q, file)", appleID, source, webTestSessionEmail)
	}

	resumed, ok, err := TryResumeSessionFromSource(context.Background(), appleID, source)
	if err != nil {
		t.Fatalf("TryResumeSessionFromSource() error: %v", err)
	}
	if !ok || resumed == nil {
		t.Fatalf("TryResumeSessionFromSource() = (%v, %t, %v), want keychain-backed session", resumed, ok, err)
	}
	if resumed.UserEmail != webTestSessionEmail || resumed.ProviderID != 42 {
		t.Fatalf("resumed session = %+v, want user %q and provider 42", resumed, webTestSessionEmail)
	}
	if got, want := strings.Join(requests, ","), "file-token,keychain-token"; got != want {
		t.Fatalf("server validation sequence = %q, want %q", got, want)
	}

	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %t, %v), want refreshed file mirror", stored, ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "keychain-token" {
		t.Fatalf("refreshed file cookie = %q, want keychain-token", got)
	}
}

func TestTryResumeSessionFromSourceHonorsExplicitFileBackend(t *testing.T) {
	kr := withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, webTestPersistedSession(t, "file-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}
	if err := writeSessionToKeychain(key, webTestPersistedSession(t, "keychain-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToKeychain() error: %v", err)
	}
	kr.ResetCounts()

	previousFetcher := sessionInfoFetcher
	var validationCalls int
	sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
		validationCalls++
		return nil, &sessionInfoStatusError{Status: http.StatusUnauthorized}
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	resumed, ok, err := TryResumeSessionFromSource(context.Background(), webTestSessionEmail, CachedSessionSourceFile)
	if !errors.Is(err, ErrCachedSessionExpired) || ok || resumed != nil {
		t.Fatalf("TryResumeSessionFromSource() = (%v, %t, %v), want explicit file expiry", resumed, ok, err)
	}
	if validationCalls != 1 {
		t.Fatalf("validation calls = %d, want 1", validationCalls)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("explicit file source read keychain %d times, want 0", got)
	}
}

func TestTryResumeSessionFromSourceFallsBackToKeychainWhenSelectedFileDisappears(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToFile(key, webTestPersistedSession(t, "file-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToFile() error: %v", err)
	}
	if err := writeSessionToKeychain(key, webTestPersistedSession(t, "keychain-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToKeychain() error: %v", err)
	}

	appleID, source, err := DefaultCachedAppleIDWithSource()
	if err != nil {
		t.Fatalf("DefaultCachedAppleIDWithSource() error: %v", err)
	}
	if appleID != webTestSessionEmail || source != CachedSessionSourceFile {
		t.Fatalf("default cache selection = (%q, %v), want (%q, file)", appleID, source, webTestSessionEmail)
	}
	path, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove selected file session: %v", err)
	}

	resumed, ok, err := TryResumeSessionFromSource(context.Background(), appleID, source)
	if err != nil || !ok || resumed == nil {
		t.Fatalf("TryResumeSessionFromSource() = (%v, %t, %v), want keychain fallback", resumed, ok, err)
	}
	if resumed.cachedSource != CachedSessionSourceKeychain {
		t.Fatalf("cached source = %v, want keychain", resumed.cachedSource)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %t, %v), want refreshed file mirror", stored, ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "keychain-token" {
		t.Fatalf("refreshed file cookie = %q, want keychain-token", got)
	}
}

func TestTryResumeLastSessionFallsBackToKeychainWhenFileSessionIsRejectedByServer(t *testing.T) {
	for _, tc := range []struct {
		name          string
		fileUserEmail string
	}{
		{name: "matching metadata", fileUserEmail: webTestSessionEmail},
		{name: "legacy empty metadata", fileUserEmail: ""},
		{name: "marker key differs from metadata", fileUserEmail: "other@example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withArraySessionKeyring(t)
			t.Setenv(webSessionCacheEnabledEnv, "1")
			t.Setenv(webSessionBackendEnv, "auto")
			t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

			key := webSessionCacheKey(webTestSessionEmail)
			fileSession := webTestPersistedSession(t, "file-token", time.Now().UTC())
			fileSession.UserEmail = tc.fileUserEmail
			if err := writeSessionToFile(key, fileSession); err != nil {
				t.Fatalf("writeSessionToFile() error: %v", err)
			}
			if err := writeSessionToKeychain(key, webTestPersistedSession(t, "keychain-token", time.Now().UTC())); err != nil {
				t.Fatalf("writeSessionToKeychain() error: %v", err)
			}

			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token := ""
				if cookie, err := r.Cookie("myacinfo"); err == nil {
					token = cookie.Value
				}
				requests = append(requests, token)
				switch token {
				case "file-token":
					w.WriteHeader(http.StatusForbidden)
				case "keychain-token":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"provider":{"providerId":42},"user":{"emailAddress":"user@example.com"}}`)
				default:
					t.Fatalf("unexpected session cookie %q", token)
				}
			}))
			t.Cleanup(server.Close)

			previousFetcher := sessionInfoFetcher
			sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
				if err != nil {
					return nil, err
				}
				validationURL, err := url.Parse(olympusSessionURL)
				if err != nil {
					return nil, err
				}
				for _, cookie := range client.Jar.Cookies(validationURL) {
					req.AddCookie(cookie)
				}
				resp, err := client.Do(req)
				if err != nil {
					return nil, err
				}
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					return nil, &sessionInfoStatusError{Status: resp.StatusCode}
				}
				var info sessionInfo
				if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
					return nil, err
				}
				return &info, nil
			}
			t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

			resumed, ok, err := TryResumeLastSession(context.Background())
			if err != nil || !ok || resumed == nil {
				t.Fatalf("TryResumeLastSession() = (%v, %t, %v), want keychain fallback", resumed, ok, err)
			}
			if resumed.cachedSource != CachedSessionSourceKeychain {
				t.Fatalf("cached source = %v, want keychain", resumed.cachedSource)
			}
			if got, want := strings.Join(requests, ","), "file-token,keychain-token"; got != want {
				t.Fatalf("server validation sequence = %q, want %q", got, want)
			}
			stored, ok, err := readSessionFromFile(key)
			if err != nil || !ok {
				t.Fatalf("readSessionFromFile() = (%v, %t, %v), want refreshed file mirror", stored, ok, err)
			}
			if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "keychain-token" {
				t.Fatalf("refreshed file cookie = %q, want keychain-token", got)
			}
		})
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

func TestDeleteSessionWithCachingOffCreatesNoState(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "web-cache")
	sharedRoot := filepath.Join(root, "shared-lock-root")
	withStubbedSessionSharedLockRoot(t, sharedRoot)

	t.Setenv(webSessionBackendEnv, "off")
	t.Setenv(webSessionCacheDirEnv, cacheDir)

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("expected disabled session caching to delete nothing, got %v", err)
	}
	for _, path := range []string{cacheDir, sharedRoot} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected disabled session caching not to create %q, stat error: %v", path, err)
		}
	}
}

// The legacy ~/.asc/iris file cache and its ASC_IRIS_SESSION_CACHE* environment
// variables are no longer read: a session that exists only there is a cache
// miss, is never migrated, and is left untouched by delete operations.
func TestSessionCacheIgnoresLegacyIrisFileCacheAndEnvironment(t *testing.T) {
	withSessionInfoStub(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	webDir := filepath.Join(t.TempDir(), "web-cache")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, webDir)

	key := webSessionCacheKey("user@example.com")
	legacy := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: "user@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "legacy-iris-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)}},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy session: %v", err)
	}
	last, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		t.Fatalf("marshal legacy last marker: %v", err)
	}

	legacyDirs := []string{
		filepath.Join(home, ".asc", "iris"),
		filepath.Join(t.TempDir(), "custom-iris-cache"),
	}
	t.Setenv("ASC_IRIS_SESSION_CACHE", "1")
	t.Setenv("ASC_IRIS_SESSION_CACHE_DIR", legacyDirs[1])
	for _, dir := range legacyDirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir legacy dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "session-"+key+".json"), raw, 0o600); err != nil {
			t.Fatalf("write legacy session: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "last.json"), last, 0o600); err != nil {
			t.Fatalf("write legacy last marker: %v", err)
		}
	}

	if resumed, ok, err := TryResumeSession(context.Background(), "user@example.com"); err != nil || ok || resumed != nil {
		t.Fatalf("TryResumeSession = (%+v, %t, %v), want cache miss", resumed, ok, err)
	}
	if resumed, ok, err := TryResumeLastSession(context.Background()); err != nil || ok || resumed != nil {
		t.Fatalf("TryResumeLastSession = (%+v, %t, %v), want cache miss", resumed, ok, err)
	}
	if resumed, ok, err := ResumeCachedSessionWithoutPersist(context.Background(), "user@example.com"); err != nil || ok || resumed != nil {
		t.Fatalf("ResumeCachedSessionWithoutPersist = (%+v, %t, %v), want cache miss", resumed, ok, err)
	}
	if resumed, ok, err := ResumeLastCachedSessionWithoutPersist(context.Background()); err != nil || ok || resumed != nil {
		t.Fatalf("ResumeLastCachedSessionWithoutPersist = (%+v, %t, %v), want cache miss", resumed, ok, err)
	}
	if _, err := os.Stat(filepath.Join(webDir, "session-"+key+".json")); !os.IsNotExist(err) {
		t.Fatalf("legacy session was migrated into the web cache, stat err=%v", err)
	}

	if err := DeleteSession("user@example.com"); err != nil {
		t.Fatalf("DeleteSession error: %v", err)
	}
	if err := DeleteAllSessions(); err != nil {
		t.Fatalf("DeleteAllSessions error: %v", err)
	}
	for _, dir := range legacyDirs {
		for _, name := range []string{"session-" + key + ".json", "last.json"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Fatalf("legacy file %s in %s was removed or unreadable: %v", name, dir, err)
			}
		}
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

func TestResumeCachedSessionWithoutPersistNeverOpensKeychain(t *testing.T) {
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	opened := 0
	previousOpen := sessionKeyringOpen
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		opened++
		return nil, errors.New("keychain should not be opened")
	}
	t.Cleanup(func() { sessionKeyringOpen = previousOpen })

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{{Name: "myacinfo", Value: "file-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)}})
	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	resumed, ok, err := ResumeCachedSessionWithoutPersist(context.Background(), "user@example.com")
	if err != nil || !ok || resumed == nil {
		t.Fatalf("resume = (%+v, %t, %v), want cached file session", resumed, ok, err)
	}
	if opened != 0 {
		t.Fatalf("keychain opened %d times", opened)
	}
}

func TestResumeCachedSessionWithoutPersistPreservesExpiredCache(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Setenv(webSessionCacheEnabledEnv, "1")
			t.Setenv(webSessionBackendEnv, "file")
			t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

			previousFetcher := sessionInfoFetcher
			sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
				return nil, &sessionInfoStatusError{Status: status}
			}
			t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatalf("cookiejar.New error: %v", err)
			}
			targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
			jar.SetCookies(targetURL, []*http.Cookie{{Name: "myacinfo", Value: "expired-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)}})
			key := webSessionCacheKey("user@example.com")
			if err := writeSessionToFile(key, serializeCookieJar(jar, "user@example.com")); err != nil {
				t.Fatalf("writeSessionToFile error: %v", err)
			}
			path, err := webSessionFilePath(key)
			if err != nil {
				t.Fatalf("session path: %v", err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read cache before resume: %v", err)
			}

			resumed, ok, err := ResumeCachedSessionWithoutPersist(context.Background(), "user@example.com")
			if !errors.Is(err, ErrCachedSessionExpired) || ok || resumed == nil {
				t.Fatalf("resume = (%+v, %t, %v), want identity plus expired sentinel", resumed, ok, err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read cache after resume: %v", err)
			}
			if string(after) != string(before) {
				t.Fatal("read-only expired-session validation changed the cache")
			}
		})
	}
}

func TestResumeCachedSessionWithoutPersistClassifiesTransientValidationFailure(t *testing.T) {
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
		return nil, errors.New("temporary network failure")
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{{Name: "myacinfo", Value: "file-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)}})
	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, serializeCookieJar(jar, "user@example.com")); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	resumed, ok, err := ResumeCachedSessionWithoutPersist(context.Background(), "user@example.com")
	if !errors.Is(err, ErrCachedSessionValidationFailed) || ok || resumed == nil || resumed.UserEmail != "user@example.com" {
		t.Fatalf("resume = (%+v, %t, %v), want identity plus validation sentinel", resumed, ok, err)
	}
}

func persistedMyacinfoCookieValue(sess persistedSession, baseURL string) string {
	list := sess.Cookies[baseURL]
	for _, cookie := range list {
		if cookie.Name == "myacinfo" {
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

// A process that proves its loaded jar unusable must not delete by Apple ID
// alone: another process can persist a valid replacement while the first is
// still working through 2FA, and deleting that leaves no cached session at all.
func TestDeleteSessionIfMatchesPreservesAConcurrentlyRefreshedSession(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	persist := func(token string) {
		t.Helper()
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("cookiejar.New error: %v", err)
		}
		targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
		jar.SetCookies(targetURL, []*http.Cookie{
			{Name: "myacinfo", Value: token, Path: "/", Expires: time.Now().Add(24 * time.Hour)},
		})
		if err := PersistSession(&AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}); err != nil {
			t.Fatalf("PersistSession error: %v", err)
		}
	}

	persist("stale-token")
	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v error=%v", ok, err)
	}
	if loaded.cachedUpdatedAt.IsZero() {
		t.Fatal("expected the loaded session to carry the cached entry stamp")
	}

	// A concurrent process replaces the entry while this one is busy.
	time.Sleep(2 * time.Millisecond)
	persist("fresh-token")

	deleted, err := DeleteSessionIfMatches("user@example.com", loaded)
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}
	if deleted {
		t.Fatal("expected the newer cached session to be preserved")
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil || !ok {
		t.Fatalf("expected the replacement session to survive, ok=%v error=%v", ok, err)
	}
}

// The proven-stale entry itself must still go, or the next invocation reloads
// it and burns another 2FA code against the same failure.
func TestDeleteSessionIfMatchesRemovesTheEntryItLoaded(t *testing.T) {
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
		{Name: "myacinfo", Value: "stale-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v error=%v", ok, err)
	}

	deleted, err := DeleteSessionIfMatches("user@example.com", loaded)
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}
	if !deleted {
		t.Fatal("expected the unchanged stale entry to be deleted")
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil {
		t.Fatalf("LoadCachedSession error: %v", err)
	} else if ok {
		t.Fatal("expected the stale cached session to be gone")
	}
}

// A source-specific read failure must not make cleanup fall back to the
// configured primary backend. That backend can contain an unrelated session
// for the same account while the default identity came from the keychain.
func TestDeleteSessionIfMatchesReadFailureDoesNotDeleteOtherBackend(t *testing.T) {
	kr := withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey(webTestSessionEmail)
	fileSession := webTestPersistedSession(t, "file-token", time.Now().UTC())
	if err := writeSessionToFile(key, fileSession); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: []byte("{")}); err != nil {
		t.Fatalf("seed malformed keychain store: %v", err)
	}

	loaded := &AuthSession{
		UserEmail:        webTestSessionEmail,
		cachedUpdatedAt:  time.Now().UTC(),
		cachedGeneration: "keychain-generation",
		cachedSource:     CachedSessionSourceKeychain,
	}
	deleted, err := DeleteSessionIfMatches(webTestSessionEmail, loaded)
	if !errors.Is(err, errMalformedSessionStore) {
		t.Fatalf("DeleteSessionIfMatches error = %v, want malformed keychain store", err)
	}
	if !deleted {
		t.Fatal("expected cleanup to be attempted for the unreadable source entry")
	}

	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("expected unrelated file session to survive, ok=%v error=%v", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "file-token" {
		t.Fatalf("expected unrelated file cookie to survive, got %q", got)
	}
}

func TestSourceSpecificSessionReadsHonorDisabledCache(t *testing.T) {
	kr := withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	key := webSessionCacheKey(webTestSessionEmail)
	if err := writeSessionToKeychain(key, webTestPersistedSession(t, "keychain-token", time.Now().UTC())); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}
	kr.ResetCounts()
	t.Setenv(webSessionCacheEnabledEnv, "0")

	if loaded, ok, err := LoadCachedSessionFromSource(webTestSessionEmail, CachedSessionSourceKeychain); err != nil || ok || loaded != nil {
		t.Fatalf("LoadCachedSessionFromSource = (%v, %v, %v), want disabled cache miss", loaded, ok, err)
	}
	if resumed, ok, err := TryResumeSessionFromSource(context.Background(), webTestSessionEmail, CachedSessionSourceKeychain); err != nil || ok || resumed != nil {
		t.Fatalf("TryResumeSessionFromSource = (%v, %v, %v), want disabled cache miss", resumed, ok, err)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("disabled source-specific reads touched the keychain %d times", got)
	}
}

// A caller with no stamp to compare (a freshly logged-in session) keeps the
// unconditional delete rather than silently skipping it.
func TestDeleteSessionIfMatchesFallsBackWhenNoStampIsAvailable(t *testing.T) {
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
		{Name: "myacinfo", Value: "stale-token", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}

	deleted, err := DeleteSessionIfMatches("user@example.com", &AuthSession{UserEmail: "user@example.com"})
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}
	if !deleted {
		t.Fatal("expected the unconditional delete without a stamp to compare")
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil {
		t.Fatalf("LoadCachedSession error: %v", err)
	} else if ok {
		t.Fatal("expected the cached session to be gone")
	}
}

func webTestSessionJar(t *testing.T, token string) http.CookieJar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	targetURL, _ := url.Parse("https://appstoreconnect.apple.com/")
	jar.SetCookies(targetURL, []*http.Cookie{
		{Name: "myacinfo", Value: token, Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	})
	return jar
}

const webTestSessionEmail = "user@example.com"

func webTestPersistedSession(t *testing.T, token string, updatedAt time.Time) persistedSession {
	t.Helper()
	sess := serializeCookieJar(webTestSessionJar(t, token), webTestSessionEmail)
	sess.UpdatedAt = updatedAt
	return sess
}

// The compare and the delete must happen under one lock: a replacement
// persisted between them would otherwise be the entry that gets removed.
func TestDeleteSessionIfMatchesSerializesWithSameSessionPersistenceState(t *testing.T) {
	t.Setenv(webSessionCacheEnabledEnv, "0")
	loaded := &AuthSession{cachedGeneration: "generation"}
	loaded.persistMu.Lock()
	done := make(chan struct{})
	go func() {
		_, _ = DeleteSessionIfMatches(webTestSessionEmail, loaded)
		close(done)
	}()
	select {
	case <-done:
		loaded.persistMu.Unlock()
		t.Fatal("DeleteSessionIfMatches read persistence state without the session lock")
	case <-time.After(100 * time.Millisecond):
	}
	loaded.persistMu.Unlock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DeleteSessionIfMatches did not continue after the session lock was released")
	}
}

func TestDeleteSessionIfMatchesSerializesWithAConcurrentPersist(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: webTestSessionJar(t, "stale-token")},
		UserEmail: webTestSessionEmail,
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}
	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v error=%v", ok, err)
	}

	persistDone := make(chan struct{})
	persistErr := make(chan error, 1)
	prev := sessionCompareDeleteBarrier
	t.Cleanup(func() { sessionCompareDeleteBarrier = prev })
	sessionCompareDeleteBarrier = func() {
		go func() {
			defer close(persistDone)
			persistErr <- PersistSession(&AuthSession{
				Client:    &http.Client{Jar: webTestSessionJar(t, "fresh-token")},
				UserEmail: "user@example.com",
			})
		}()
		// Give the concurrent persist every chance to land inside the window.
		select {
		case <-persistDone:
		case <-time.After(250 * time.Millisecond):
		}
	}

	if _, err := DeleteSessionIfMatches("user@example.com", loaded); err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}

	select {
	case <-persistDone:
	case <-time.After(10 * time.Second):
		t.Fatal("the concurrent persist never completed")
	}
	if err := <-persistErr; err != nil {
		t.Fatalf("concurrent PersistSession error: %v", err)
	}

	stored, ok, err := readSessionFromFile(webSessionCacheKey("user@example.com"))
	if err != nil || !ok {
		t.Fatalf("expected the replacement persisted during the compare-and-delete window to survive, ok=%v error=%v", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "fresh-token" {
		t.Fatalf("expected the replacement cookie to survive, got %q", got)
	}
}

// Two processes configured with different backends need no race at all: a
// matching file entry must not take out a newer keychain entry.
func TestDeleteSessionIfMatchesKeepsAFresherKeychainEntry(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: webTestSessionJar(t, "stale-token")},
		UserEmail: webTestSessionEmail,
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}
	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v error=%v", ok, err)
	}

	key := webSessionCacheKey("user@example.com")
	fresh := webTestPersistedSession(t, "fresh-token", loaded.cachedUpdatedAt.Add(time.Minute))
	if err := writeSessionToKeychain(key, fresh); err != nil {
		t.Fatalf("writeSessionToKeychain error: %v", err)
	}

	deleted, err := DeleteSessionIfMatches("user@example.com", loaded)
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}
	if !deleted {
		t.Fatal("expected the matched file entry to be deleted")
	}
	if _, ok, err := readSessionFromFile(key); err != nil || ok {
		t.Fatalf("expected the matched file entry to be gone, ok=%v error=%v", ok, err)
	}
	stored, ok, err := readSessionFromKeychain(key)
	if err != nil || !ok {
		t.Fatalf("expected the newer keychain entry to survive, ok=%v error=%v", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "fresh-token" {
		t.Fatalf("expected the newer keychain cookie to survive, got %q", got)
	}
}

// The mirror direction is symmetric: a matching keychain entry must not take
// out a newer file entry another process persisted.
func TestDeleteSessionIfMatchesKeepsAFresherFileEntry(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: webTestSessionJar(t, "stale-token")},
		UserEmail: webTestSessionEmail,
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}
	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v error=%v", ok, err)
	}

	key := webSessionCacheKey("user@example.com")
	fresh := webTestPersistedSession(t, "fresh-token", loaded.cachedUpdatedAt.Add(time.Minute))
	if err := writeSessionToFile(key, fresh); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	deleted, err := DeleteSessionIfMatches("user@example.com", loaded)
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}
	if !deleted {
		t.Fatal("expected the matched keychain entry to be deleted")
	}
	if _, ok, err := readSessionFromKeychain(key); err != nil || ok {
		t.Fatalf("expected the matched keychain entry to be gone, ok=%v error=%v", ok, err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("expected the newer file entry to survive, ok=%v error=%v", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "fresh-token" {
		t.Fatalf("expected the newer file cookie to survive, got %q", got)
	}
}

// A mirror carrying the same stamp is the same proven-stale session, so it
// still goes: leaving it behind reloads it on the next invocation.
func TestDeleteSessionIfMatchesRemovesAMirrorCarryingTheSameStamp(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	if err := PersistSession(&AuthSession{
		Client:    &http.Client{Jar: webTestSessionJar(t, "stale-token")},
		UserEmail: webTestSessionEmail,
	}); err != nil {
		t.Fatalf("PersistSession error: %v", err)
	}
	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v error=%v", ok, err)
	}

	key := webSessionCacheKey("user@example.com")
	mirror := webTestPersistedSession(t, "stale-token", loaded.cachedUpdatedAt)
	mirror.Generation = loaded.cachedGeneration
	if err := writeSessionToFile(key, mirror); err != nil {
		t.Fatalf("writeSessionToFile error: %v", err)
	}

	deleted, err := DeleteSessionIfMatches("user@example.com", loaded)
	if err != nil {
		t.Fatalf("DeleteSessionIfMatches error: %v", err)
	}
	if !deleted {
		t.Fatal("expected the matched keychain entry to be deleted")
	}
	if _, ok, err := readSessionFromKeychain(key); err != nil || ok {
		t.Fatalf("expected the matched keychain entry to be gone, ok=%v error=%v", ok, err)
	}
	if _, ok, err := readSessionFromFile(key); err != nil || ok {
		t.Fatalf("expected the identically stamped file mirror to be gone, ok=%v error=%v", ok, err)
	}
}

func TestSerializeCookieJarAssignsUniqueGeneration(t *testing.T) {
	jar := webTestSessionJar(t, "generation-token")
	a := serializeCookieJar(jar, "user@example.com")
	b := serializeCookieJar(jar, "other@example.com")
	if a.Generation == "" || b.Generation == "" {
		t.Fatal("expected non-empty session generations")
	}
	if a.Generation == b.Generation {
		t.Fatalf("expected unique generations, got %q", a.Generation)
	}
}

func TestPersistSessionConcurrentDifferentAppleIDsPreservesBothKeychainEntries(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, tc := range []struct{ email, token string }{
		{email: "first@example.com", token: "first-token"},
		{email: "second@example.com", token: "second-token"},
	} {
		tc := tc
		go func() {
			<-start
			errs <- PersistSession(&AuthSession{Client: &http.Client{Jar: webTestSessionJar(t, tc.token)}, UserEmail: tc.email})
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent PersistSession error: %v", err)
		}
	}
	for _, email := range []string{"first@example.com", "second@example.com"} {
		stored, ok, err := readSessionFromKeychain(webSessionCacheKey(email))
		if err != nil || !ok {
			t.Fatalf("expected %s to survive, ok=%v error=%v", email, ok, err)
		}
		if stored.UserEmail != email {
			t.Fatalf("stored wrong identity %q for %s", stored.UserEmail, email)
		}
	}
}

func TestSerializeCookieJarPropagatesGenerationFailure(t *testing.T) {
	previous := sessionGenerationReader
	sessionGenerationReader = func([]byte) (int, error) { return 0, errors.New("rng unavailable") }
	t.Cleanup(func() { sessionGenerationReader = previous })
	if _, err := serializeCookieJarWithError(webTestSessionJar(t, "token"), "user@example.com"); err == nil {
		t.Fatal("expected generation failure")
	}
}

func TestSamePersistedSessionIdentityRejectsDifferentGenerationSameTimestamp(t *testing.T) {
	now := time.Now().UTC()
	loaded := &AuthSession{cachedUpdatedAt: now, cachedGeneration: "old"}
	if samePersistedSessionIdentity(persistedSession{UpdatedAt: now, Generation: "new"}, loaded) {
		t.Fatal("different generations must not match even with equal timestamps")
	}
}

func TestSamePersistedSessionIdentityRejectsGeneratedLegacyPair(t *testing.T) {
	now := time.Now().UTC()
	if samePersistedSessionIdentity(persistedSession{UpdatedAt: now}, &AuthSession{cachedUpdatedAt: now, cachedGeneration: "generated"}) {
		t.Fatal("generated and legacy sessions must not match")
	}
}
