package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
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

func withSessionInfoStub(t *testing.T, email string, providerID int64) {
	t.Helper()
	prev := sessionInfoFetcher
	sessionInfoFetcher = func(ctx context.Context, client *http.Client) (*sessionInfo, error) {
		out := &sessionInfo{}
		out.Provider.ProviderID = providerID
		out.User.EmailAddress = email
		return out, nil
	}
	t.Cleanup(func() {
		sessionInfoFetcher = prev
	})
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
	withSessionInfoStub(t, "user@example.com", 42)
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
	if ok {
		t.Fatal("did not expect cache resume success")
	}
	if resumed != nil {
		t.Fatal("did not expect resumed session")
	}
}

func TestTryResumeLastSessionMigratesLegacyKeychainEntriesToSharedStore(t *testing.T) {
	kr := withArraySessionKeyring(t)
	withSessionInfoStub(t, "user@example.com", 42)
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
