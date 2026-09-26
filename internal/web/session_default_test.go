package web

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func writeDefaultTestSessionFile(t *testing.T, dir, email string, version int) {
	writeDefaultTestSessionFileForKey(t, dir, email, email, version)
}

func writeDefaultTestSessionFileForKey(t *testing.T, dir, keyEmail, sessionEmail string, version int) {
	t.Helper()
	sess := persistedSession{
		Version:   version,
		UpdatedAt: time.Now(),
		UserEmail: sessionEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "cookie-" + sessionEmail}},
		},
	}
	writeDefaultTestSessionRecord(t, dir, keyEmail, sess)
}

func writeDefaultTestSessionRecord(t *testing.T, dir, keyEmail string, sess persistedSession) {
	t.Helper()
	raw, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "session-"+webSessionCacheKey(keyEmail)+".json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write session: %v", err)
	}
}

func TestDefaultCachedAppleIDFileBackend(t *testing.T) {
	t.Run("no sessions", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)

		appleID, err := DefaultCachedAppleID()
		if !errors.Is(err, ErrNoCachedSession) {
			t.Fatalf("err = %v, want ErrNoCachedSession", err)
		}
		if appleID != "" {
			t.Fatalf("appleID = %q, want empty", appleID)
		}
	})

	t.Run("missing cache dir", func(t *testing.T) {
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "missing"))

		if _, err := DefaultCachedAppleID(); !errors.Is(err, ErrNoCachedSession) {
			t.Fatalf("err = %v, want ErrNoCachedSession", err)
		}
	})

	t.Run("exactly one session", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "Only@Example.com", webSessionCacheVersion)
		// Malformed, wrong-version, and anonymous entries never count.
		if err := os.WriteFile(filepath.Join(dir, "session-broken.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeDefaultTestSessionFile(t, dir, "old@example.com", webSessionCacheVersion+1)
		writeDefaultTestSessionFile(t, dir, "", webSessionCacheVersion)

		appleID, err := DefaultCachedAppleID()
		if err != nil {
			t.Fatalf("DefaultCachedAppleID() error = %v", err)
		}
		if appleID != "Only@Example.com" {
			t.Fatalf("appleID = %q, want Only@Example.com", appleID)
		}
	})

	t.Run("two sessions", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "zed@example.com", webSessionCacheVersion)
		writeDefaultTestSessionFile(t, dir, "amy@example.com", webSessionCacheVersion)

		appleID, err := DefaultCachedAppleID()
		var ambiguous *AmbiguousCachedSessionError
		if !errors.As(err, &ambiguous) {
			t.Fatalf("err = %v, want *AmbiguousCachedSessionError", err)
		}
		if appleID != "" {
			t.Fatalf("appleID = %q, want empty", appleID)
		}
		if want := []string{"amy@example.com", "zed@example.com"}; !reflect.DeepEqual(ambiguous.AppleIDs, want) {
			t.Fatalf("AppleIDs = %v, want %v", ambiguous.AppleIDs, want)
		}
		if got, want := err.Error(), "multiple cached web sessions are available: amy@example.com, zed@example.com"; got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("unreadable session prevents an unsafe default", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "only@example.com", webSessionCacheVersion)
		if err := os.WriteFile(filepath.Join(dir, "session-unreadable.json"), []byte("unreadable"), 0o600); err != nil {
			t.Fatal(err)
		}
		originalRead := readDefaultSessionFromFileFn
		readDefaultSessionFromFileFn = func(key string) (persistedSession, bool, error) {
			if key == "unreadable" {
				return persistedSession{}, false, errors.New("permission denied")
			}
			return originalRead(key)
		}
		t.Cleanup(func() { readDefaultSessionFromFileFn = originalRead })

		appleID, err := DefaultCachedAppleID()
		if err == nil {
			t.Fatalf("DefaultCachedAppleID() = %q, nil; want unreadable-cache error", appleID)
		}
		if appleID != "" {
			t.Fatalf("appleID = %q, want empty", appleID)
		}
		if !strings.Contains(err.Error(), "session-unreadable.json") {
			t.Fatalf("error = %q, want unreadable entry name", err)
		}
	})

	t.Run("cache disabled", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionCacheEnabledEnv, "0")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "only@example.com", webSessionCacheVersion)

		if _, err := DefaultCachedAppleID(); !errors.Is(err, ErrNoCachedSession) {
			t.Fatalf("err = %v, want ErrNoCachedSession", err)
		}
	})
}

func TestDefaultCachedAppleIDRejectsSymlinkedSessionFile(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, dir)

	keyEmail := "outside@example.com"
	writeDefaultTestSessionFile(t, outside, keyEmail, webSessionCacheVersion)
	key := webSessionCacheKey(keyEmail)
	cachePath := filepath.Join(dir, "session-"+key+".json")
	targetPath := filepath.Join(outside, "session-"+key+".json")
	if err := os.Symlink(targetPath, cachePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	appleID, err := DefaultCachedAppleID()
	if err == nil {
		t.Fatalf("DefaultCachedAppleID() = %q, nil; want symlink rejection", appleID)
	}
	if appleID != "" {
		t.Fatalf("appleID = %q, want empty", appleID)
	}
	if !strings.Contains(err.Error(), cachePath) {
		t.Fatalf("error = %q, want cache path", err)
	}
}

func TestDefaultCachedAppleIDAutoBackendSkipsSymlinkedSessionFile(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)

	writeDefaultTestSessionFile(t, outside, "outside@example.com", webSessionCacheVersion)
	key := webSessionCacheKey("outside@example.com")
	cachePath := filepath.Join(dir, "session-"+key+".json")
	targetPath := filepath.Join(outside, "session-"+key+".json")
	if err := os.Symlink(targetPath, cachePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	kr := withArraySessionKeyring(t)
	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "keychain-cookie"}},
		},
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}
}

func TestDefaultCachedAppleIDAutoBackendKeepsRegularSessionAlongsideSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	kr := withArraySessionKeyring(t)

	writeDefaultTestSessionFile(t, dir, "file@example.com", webSessionCacheVersion)
	writeDefaultTestSessionFile(t, outside, "outside@example.com", webSessionCacheVersion)
	key := webSessionCacheKey("outside@example.com")
	if err := os.Symlink(
		filepath.Join(outside, "session-"+key+".json"),
		filepath.Join(dir, "session-"+key+".json"),
	); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	kr.ResetCounts()

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "file@example.com" {
		t.Fatalf("appleID = %q, want file@example.com", appleID)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("keychain store read %d times, want 0", got)
	}
}

func TestDefaultCachedAppleIDKeychainBackend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, dir)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}
}

func TestDefaultCachedAppleIDAutoBackendFallsBackToKeychainWhenFileCacheEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "keychain-cookie"}},
		},
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}

	// A populated file cache wins without consulting the keychain again.
	writeDefaultTestSessionFile(t, dir, "file@example.com", webSessionCacheVersion)
	kr.ResetCounts()
	appleID, err = DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "file@example.com" {
		t.Fatalf("appleID = %q, want file@example.com", appleID)
	}
	if kr.GetCount(webSessionStoreItem) != 0 {
		t.Fatalf("keychain store read %d times, want 0", kr.GetCount(webSessionStoreItem))
	}
}

func TestDefaultCachedAppleIDAutoBackendFallsBackToKeychainWhenFileCacheHasNoUsableIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	writeDefaultTestSessionFile(t, dir, "", webSessionCacheVersion)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "keychain-cookie"}},
		},
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}
	kr.ResetCounts()

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 1 {
		t.Fatalf("keychain store read %d times, want 1", got)
	}
}

func TestDefaultCachedAppleIDAutoBackendFallsBackToKeychainWhenFileSessionCannotHydrate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cookies map[string][]pCookie
	}{
		{name: "no cookies"},
		{
			name: "expired cookies",
			cookies: map[string][]pCookie{
				"https://appstoreconnect.apple.com": {{
					Name:    "myacinfo",
					Value:   "expired-file-cookie",
					Expires: time.Now().Add(-time.Hour),
				}},
			},
		},
		{
			name: "unrelated cookies",
			cookies: map[string][]pCookie{
				"https://example.com": {{Name: "session", Value: "unrelated-cookie"}},
			},
		},
		{
			name: "non-session path cookies",
			cookies: map[string][]pCookie{
				"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "iris-only-cookie", Path: "/iris"}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(webSessionBackendEnv, "auto")
			t.Setenv(webSessionCacheDirEnv, dir)
			writeDefaultTestSessionRecord(t, dir, "file@example.com", persistedSession{
				Version:   webSessionCacheVersion,
				UpdatedAt: time.Now(),
				UserEmail: "file@example.com",
				Cookies:   tc.cookies,
			})

			kr := withArraySessionKeyring(t)
			store := newPersistedSessionStore()
			store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
				Version:   webSessionCacheVersion,
				UpdatedAt: time.Now(),
				UserEmail: "kc@example.com",
				Cookies: map[string][]pCookie{
					"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "keychain-cookie"}},
				},
			}
			raw, err := json.Marshal(store)
			if err != nil {
				t.Fatal(err)
			}
			if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
				t.Fatal(err)
			}

			appleID, source, err := DefaultCachedAppleIDWithSource()
			if err != nil {
				t.Fatalf("DefaultCachedAppleIDWithSource() error = %v", err)
			}
			if appleID != "kc@example.com" {
				t.Fatalf("appleID = %q, want kc@example.com", appleID)
			}
			if source != CachedSessionSourceKeychain {
				t.Fatalf("source = %v, want keychain", source)
			}
		})
	}
}

func TestDefaultCachedAppleIDAutoBackendKeepsSessionEndpointScopedFileCookie(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	writeDefaultTestSessionRecord(t, dir, "file@example.com", persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "file@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "file-cookie", Path: "/olympus"}},
		},
	})
	kr := withArraySessionKeyring(t)
	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "keychain-cookie"}},
		},
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}
	kr.ResetCounts()

	appleID, source, err := DefaultCachedAppleIDWithSource()
	if err != nil {
		t.Fatalf("DefaultCachedAppleIDWithSource() error = %v", err)
	}
	if appleID != "file@example.com" {
		t.Fatalf("appleID = %q, want file@example.com", appleID)
	}
	if source != CachedSessionSourceFile {
		t.Fatalf("source = %v, want file", source)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("keychain store read %d times, want 0", got)
	}
}

func TestDefaultCachedAppleIDAutoBackendIgnoresUnusableFileIdentityWhenUsableFileSessionExists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	writeDefaultTestSessionRecord(t, dir, "unusable@example.com", persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "unusable@example.com",
	})
	writeDefaultTestSessionFile(t, dir, "usable@example.com", webSessionCacheVersion)
	kr := withArraySessionKeyring(t)
	kr.ResetCounts()

	appleID, source, err := DefaultCachedAppleIDWithSource()
	if err != nil {
		t.Fatalf("DefaultCachedAppleIDWithSource() error = %v", err)
	}
	if appleID != "usable@example.com" {
		t.Fatalf("appleID = %q, want usable@example.com", appleID)
	}
	if source != CachedSessionSourceFile {
		t.Fatalf("source = %v, want file", source)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("keychain store read %d times, want 0", got)
	}
}

func TestDefaultCachedAppleIDAutoBackendDoesNotLetAnonymousFileEntryShadowKeychainSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	writeDefaultTestSessionFileForKey(t, dir, "kc@example.com", "", webSessionCacheVersion)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "cookie-keychain"}},
		},
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}

	appleID, source, err := DefaultCachedAppleIDWithSource()
	if err != nil {
		t.Fatalf("DefaultCachedAppleIDWithSource() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}
	if source != CachedSessionSourceKeychain {
		t.Fatalf("source = %v, want keychain", source)
	}
	loaded, ok, err := LoadCachedSessionFromSource(appleID, source)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSessionFromSource() = (%v, %t, %v), want keychain session", loaded, ok, err)
	}
	if loaded.UserEmail != "kc@example.com" {
		t.Fatalf("loaded UserEmail = %q, want keychain identity", loaded.UserEmail)
	}
	loaded, ok, err = LoadCachedSession(appleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession() = (%v, %t, %v), want legacy file session", loaded, ok, err)
	}
	if loaded.UserEmail != "" {
		t.Fatalf("generic loaded UserEmail = %q, want anonymous file precedence", loaded.UserEmail)
	}
}

func TestDefaultCachedAppleIDExplicitFileBackendDoesNotFallbackForAnonymousCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, dir)
	writeDefaultTestSessionFile(t, dir, "", webSessionCacheVersion)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}
	kr.ResetCounts()

	appleID, err := DefaultCachedAppleID()
	if !errors.Is(err, ErrNoCachedSession) {
		t.Fatalf("DefaultCachedAppleID() = %q, %v; want ErrNoCachedSession", appleID, err)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("keychain store read %d times, want 0", got)
	}
}

func TestDefaultCachedAppleIDExplicitFileBackendKeepsCookielessIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, dir)
	writeDefaultTestSessionRecord(t, dir, "file@example.com", persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "file@example.com",
	})
	kr := withArraySessionKeyring(t)
	kr.ResetCounts()

	appleID, source, err := DefaultCachedAppleIDWithSource()
	if err != nil {
		t.Fatalf("DefaultCachedAppleIDWithSource() error = %v", err)
	}
	if appleID != "file@example.com" {
		t.Fatalf("appleID = %q, want file@example.com", appleID)
	}
	if source != CachedSessionSourceFile {
		t.Fatalf("source = %v, want file", source)
	}
	if got := kr.GetCount(webSessionStoreItem); got != 0 {
		t.Fatalf("keychain store read %d times, want 0", got)
	}
}
