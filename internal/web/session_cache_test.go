package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func withArraySessionKeyring(t *testing.T) {
	t.Helper()
	prev := sessionKeyringOpen
	kr := keyring.NewArrayKeyring([]keyring.Item{})
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		sessionKeyringOpen = prev
	})
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

func persistedCookieValue(sess persistedSession, baseURL, cookieName string) string {
	list := sess.Cookies[baseURL]
	for _, cookie := range list {
		if cookie.Name == cookieName {
			return cookie.Value
		}
	}
	return ""
}
