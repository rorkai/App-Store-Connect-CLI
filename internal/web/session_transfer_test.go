package web

import (
	"context"
	"errors"
	"fmt"
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

func withFileSessionCache(t *testing.T) {
	t.Helper()
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "file")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
}

func persistTestSession(t *testing.T, appleID string, cookies ...*http.Cookie) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	target, err := url.Parse("https://appstoreconnect.apple.com/")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	jar.SetCookies(target, cookies)
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: jar}, UserEmail: appleID}); err != nil {
		t.Fatalf("PersistSession() error = %v", err)
	}
}

func TestExportSessionBundleRoundTripsThroughImport(t *testing.T) {
	withFileSessionCache(t)
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	persistTestSession(
		t, "user@example.com",
		&http.Cookie{Name: "myacinfo", Value: "token-value", Path: "/", Expires: expires},
		&http.Cookie{Name: "itctx", Value: "ctx-value", Path: "/", Expires: expires.Add(time.Hour)},
	)

	bundle, ok, err := ExportSessionBundle("user@example.com")
	if err != nil {
		t.Fatalf("ExportSessionBundle() error = %v", err)
	}
	if !ok || bundle == nil {
		t.Fatal("ExportSessionBundle() ok = false, want a cached session")
	}
	if bundle.Kind != SessionBundleKind || bundle.Version != SessionBundleVersion {
		t.Fatalf("unexpected bundle envelope: kind=%q version=%d", bundle.Kind, bundle.Version)
	}
	if bundle.AppleID != "user@example.com" {
		t.Fatalf("AppleID = %q, want user@example.com", bundle.AppleID)
	}
	if len(bundle.Cookies) != 2 {
		t.Fatalf("len(Cookies) = %d, want 2", len(bundle.Cookies))
	}
	// Cookies are ordered deterministically so exports are reproducible.
	if bundle.Cookies[0].Name != "itctx" || bundle.Cookies[1].Name != "myacinfo" {
		t.Fatalf("unexpected cookie order: %q, %q", bundle.Cookies[0].Name, bundle.Cookies[1].Name)
	}
	// net/http/cookiejar returns only name and value, so the session cache never
	// records cookie expiry and an export from the cache cannot report one.
	if bundle.ExpiresAt != nil {
		t.Fatalf("ExpiresAt = %v, want nil for a cache-backed export", bundle.ExpiresAt)
	}
	for _, cookie := range bundle.Cookies {
		if cookie.Expires != nil {
			t.Fatalf("cookie %q carries an expiry the cookie jar cannot preserve", cookie.Name)
		}
	}

	// Import into a second, empty cache and confirm the session resumes.
	withFileSessionCache(t)
	if _, ok, err := ExportSessionBundle("user@example.com"); err != nil || ok {
		t.Fatalf("ExportSessionBundle() on empty cache = (%v, %v), want (false, nil)", ok, err)
	}

	summary, err := ImportSessionBundle(bundle)
	if err != nil {
		t.Fatalf("ImportSessionBundle() error = %v", err)
	}
	if summary.AppleID != "user@example.com" || summary.CookieCount != 2 || summary.SkippedExpired != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil {
		t.Fatalf("LoadCachedSession() error = %v", err)
	}
	if !ok || loaded == nil {
		t.Fatal("LoadCachedSession() ok = false, want the imported session")
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	values := map[string]string{}
	for _, cookie := range loaded.Client.Jar.Cookies(target) {
		values[cookie.Name] = cookie.Value
	}
	if values["myacinfo"] != "token-value" || values["itctx"] != "ctx-value" {
		t.Fatalf("imported cookie jar = %#v, want the exported values", values)
	}

	// The imported session is also the last cached session.
	last, ok, err := LoadLastCachedSession()
	if err != nil || !ok || last == nil {
		t.Fatalf("LoadLastCachedSession() = (%v, %v, %v), want the imported session", last, ok, err)
	}
	if last.UserEmail != "user@example.com" {
		t.Fatalf("last session UserEmail = %q, want user@example.com", last.UserEmail)
	}
}

func TestExportSessionBundleUsesLastCachedSessionWithoutAppleID(t *testing.T) {
	withFileSessionCache(t)
	persistTestSession(
		t, "last@example.com",
		&http.Cookie{Name: "myacinfo", Value: "token", Path: "/", Expires: time.Now().Add(time.Hour)},
	)

	bundle, ok, err := ExportSessionBundle("")
	if err != nil || !ok || bundle == nil {
		t.Fatalf("ExportSessionBundle(\"\") = (%v, %v, %v), want the last cached session", bundle, ok, err)
	}
	if bundle.AppleID != "last@example.com" {
		t.Fatalf("AppleID = %q, want last@example.com", bundle.AppleID)
	}
}

func TestExportSessionBundleNarrowsLegacyParentDomainCookie(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: "user@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "legacy-token", Path: "/olympus", Domain: ".apple.com",
				Expires: time.Now().UTC().Add(time.Hour),
			}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}

	bundle, ok, err := ExportSessionBundle("user@example.com")
	if err != nil || !ok || bundle == nil {
		t.Fatalf("ExportSessionBundle() = (%+v, %t, %v), want exported legacy session", bundle, ok, err)
	}
	if len(bundle.Cookies) != 1 || bundle.Cookies[0].Domain != "" {
		t.Fatalf("exported legacy cookie = %#v, want narrowed host-only domain", bundle.Cookies)
	}
	if err := bundle.Validate(); err != nil {
		t.Fatalf("narrowed legacy bundle failed validation: %v", err)
	}
}

func TestExportSessionBundleRejectsDuplicateLegacyCookieAliases(t *testing.T) {
	withFileSessionCache(t)
	now := time.Now().UTC()
	key := webSessionCacheKey("user@example.com")
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: now,
		UserEmail: "user@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "same", Path: "/olympus", Expires: now.Add(time.Hour)},
				{Name: "myacinfo", Value: "same", Path: "/olympus", Domain: ".apple.com", Expires: now.Add(time.Hour)},
			},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}

	bundle, ok, err := ExportSessionBundle("user@example.com")
	if ok || bundle != nil || !errors.Is(err, ErrSessionCookieDuplicate) {
		t.Fatalf("ExportSessionBundle() = (%+v, %t, %v), want duplicate-cookie rejection", bundle, ok, err)
	}
}

func TestExportBundleCookiesDoesNotResetPersistedMaxAge(t *testing.T) {
	now := time.Date(2026, time.September, 17, 3, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-30 * time.Second)
	sess := persistedSession{
		UpdatedAt: updatedAt,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{
				Name: "myacinfo", Value: "token", MaxAge: 60, Expires: now.Add(-time.Hour),
			}},
		},
	}

	cookies := exportBundleCookies(sess, now)
	if len(cookies) != 1 {
		t.Fatalf("exportBundleCookies() returned %d cookies, want 1", len(cookies))
	}
	wantExpiry := updatedAt.Add(time.Minute)
	if cookies[0].Expires == nil || !cookies[0].Expires.Equal(wantExpiry) {
		t.Fatalf("exported expiry = %v, want %v", cookies[0].Expires, wantExpiry)
	}
	if cookies[0].MaxAge != 0 {
		t.Fatalf("exported MaxAge = %d, want 0", cookies[0].MaxAge)
	}

	sess.UpdatedAt = now.Add(-time.Minute)
	if cookies := exportBundleCookies(sess, now); len(cookies) != 0 {
		t.Fatalf("elapsed MaxAge exported %d cookies, want 0", len(cookies))
	}
}

func TestExportSessionBundleReportsDisabledCache(t *testing.T) {
	withFileSessionCache(t)
	t.Setenv(webSessionCacheEnabledEnv, "0")

	if _, _, err := ExportSessionBundle("user@example.com"); !errors.Is(err, ErrSessionCacheDisabled) {
		t.Fatalf("ExportSessionBundle() error = %v, want ErrSessionCacheDisabled", err)
	}
}

func TestImportSessionBundleReportsDisabledCache(t *testing.T) {
	withFileSessionCache(t)
	t.Setenv(webSessionBackendEnv, "off")

	bundle := validTestBundle(time.Now().Add(time.Hour))
	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionCacheDisabled) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionCacheDisabled", err)
	}
}

func validTestBundle(expires time.Time) *SessionBundle {
	expiry := expires.UTC()
	return &SessionBundle{
		Kind:       SessionBundleKind,
		Version:    SessionBundleVersion,
		ExportedAt: time.Now().UTC(),
		AppleID:    "user@example.com",
		Cookies: []SessionBundleCookie{{
			URL:     "https://appstoreconnect.apple.com/",
			Name:    "myacinfo",
			Value:   "token",
			Path:    "/",
			Expires: &expiry,
		}},
	}
}

func TestImportSessionBundleIsLocalOnly(t *testing.T) {
	withFileSessionCache(t)
	var calls int
	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
		calls++
		return nil, errors.New("live session validation must not run during import")
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	summary, err := ImportSessionBundleWithContext(context.Background(), validTestBundle(time.Now().Add(time.Hour)), false)
	if err != nil {
		t.Fatalf("ImportSessionBundleWithContext() error = %v", err)
	}
	if summary.AppleID != "user@example.com" || summary.CookieCount != 1 {
		t.Fatalf("unexpected import summary: %+v", summary)
	}
	if calls != 0 {
		t.Fatalf("live session validation calls = %d, want 0", calls)
	}
}

func TestValidateSessionBundleUsesTemporarySessionWithoutPersisting(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	cacheDir := os.Getenv(webSessionCacheDirEnv)
	var calls int
	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(_ context.Context, client *http.Client) (*sessionInfo, error) {
		calls++
		target, err := url.Parse("https://appstoreconnect.apple.com/")
		if err != nil {
			return nil, err
		}
		var token string
		for _, cookie := range client.Jar.Cookies(target) {
			if cookie != nil && cookie.Name == "myacinfo" {
				token = cookie.Value
				break
			}
		}
		if token != "token" {
			return nil, fmt.Errorf("temporary session cookie = %q, want token", token)
		}
		info := &sessionInfo{}
		info.User.EmailAddress = "USER@example.com"
		return info, nil
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	if err := ValidateSessionBundle(context.Background(), bundle); err != nil {
		t.Fatalf("ValidateSessionBundle() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("session validation calls = %d, want 1", calls)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("successful live validation wrote to the cache: %v", entries)
	}
}

func TestValidateSessionBundleRejectsNilBundle(t *testing.T) {
	if err := ValidateSessionBundle(context.Background(), nil); !errors.Is(err, ErrSessionBundleValidationFailed) {
		t.Fatalf("ValidateSessionBundle(nil) error = %v, want ErrSessionBundleValidationFailed", err)
	}
}

func TestValidateSessionBundleRejectsLiveFailuresWithoutPersisting(t *testing.T) {
	cases := []struct {
		name      string
		fetch     func() (*sessionInfo, error)
		wantCause error
		wantText  string
	}{
		{
			name: "account mismatch",
			fetch: func() (*sessionInfo, error) {
				info := &sessionInfo{}
				info.User.EmailAddress = "other@example.com"
				return info, nil
			},
			wantText: "does not match",
		},
		{
			name: "missing account identity",
			fetch: func() (*sessionInfo, error) {
				return &sessionInfo{}, nil
			},
			wantText: "no Apple Account identity",
		},
		{
			name: "expired unauthorized session",
			fetch: func() (*sessionInfo, error) {
				return nil, &sessionInfoStatusError{Status: http.StatusUnauthorized}
			},
			wantCause: ErrCachedSessionExpired,
		},
		{
			name: "expired forbidden session",
			fetch: func() (*sessionInfo, error) {
				return nil, &sessionInfoStatusError{Status: http.StatusForbidden}
			},
			wantCause: ErrCachedSessionExpired,
		},
		{
			name: "transient validation failure",
			fetch: func() (*sessionInfo, error) {
				return nil, errors.New("temporary network failure")
			},
			wantCause: ErrCachedSessionValidationFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withFileSessionCache(t)
			previousFetcher := sessionInfoFetcher
			sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
				return tc.fetch()
			}
			t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

			if err := ValidateSessionBundle(context.Background(), validTestBundle(time.Now().Add(time.Hour))); err == nil {
				t.Fatal("ValidateSessionBundle() error = nil, want a validation failure")
			} else {
				if !errors.Is(err, ErrSessionBundleValidationFailed) {
					t.Fatalf("ValidateSessionBundle() error = %v, want ErrSessionBundleValidationFailed", err)
				}
				if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
					t.Fatalf("ValidateSessionBundle() error = %v, want cause %v", err, tc.wantCause)
				}
				if tc.wantText != "" && !strings.Contains(err.Error(), tc.wantText) {
					t.Fatalf("ValidateSessionBundle() error = %v, want text %q", err, tc.wantText)
				}
				if strings.Contains(err.Error(), "token") {
					t.Fatalf("validation error leaked a cookie value: %v", err)
				}
			}

			cacheDir := os.Getenv(webSessionCacheDirEnv)
			entries, err := os.ReadDir(cacheDir)
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("ReadDir(%q) error = %v", cacheDir, err)
			}
			if len(entries) != 0 {
				t.Fatalf("failed live validation wrote to the cache: %v", entries)
			}
		})
	}
}

func TestImportSessionBundleSkipsExpiredCookies(t *testing.T) {
	withFileSessionCache(t)
	live := time.Now().Add(time.Hour).UTC()
	stale := time.Now().Add(-time.Hour).UTC()
	bundle := validTestBundle(live)
	bundle.Cookies = append(bundle.Cookies, SessionBundleCookie{
		URL:     "https://idmsa.apple.com/",
		Name:    "expired",
		Value:   "gone",
		Expires: &stale,
	})

	summary, err := ImportSessionBundle(bundle)
	if err != nil {
		t.Fatalf("ImportSessionBundle() error = %v", err)
	}
	if summary.CookieCount != 1 || summary.SkippedExpired != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestImportSessionBundleRejectsDuplicateCanonicalCookieIdentity(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies = append(bundle.Cookies, bundle.Cookies[0])

	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionCookieDuplicate) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionCookieDuplicate", err)
	}
	if _, ok, err := LoadCachedSession(bundle.AppleID); err != nil || ok {
		t.Fatalf("LoadCachedSession() = (%v, %v), want no cached session after a refused import", ok, err)
	}
}

func TestImportSessionBundleRefusesFullyExpiredBundle(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(-time.Hour))

	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionBundleUnusable) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionBundleUnusable", err)
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil || ok {
		t.Fatalf("LoadCachedSession() = (%v, %v), want no cached session after a refused import", ok, err)
	}
}

func TestDecodeSessionBundleRejectsInvalidDocuments(t *testing.T) {
	live := time.Now().Add(time.Hour).UTC()

	cases := []struct {
		name    string
		mutate  func(*SessionBundle)
		wantErr string
	}{
		{
			name:    "wrong kind",
			mutate:  func(b *SessionBundle) { b.Kind = "cookies.txt" },
			wantErr: "kind",
		},
		{
			name:    "unsupported version",
			mutate:  func(b *SessionBundle) { b.Version = 99 },
			wantErr: "version",
		},
		{
			name:    "missing apple id",
			mutate:  func(b *SessionBundle) { b.AppleID = "  " },
			wantErr: "appleId",
		},
		{
			name:    "no cookies",
			mutate:  func(b *SessionBundle) { b.Cookies = nil },
			wantErr: "no cookies",
		},
		{
			name:    "missing cookie name",
			mutate:  func(b *SessionBundle) { b.Cookies[0].Name = "" },
			wantErr: "missing name",
		},
		{
			name:    "unsupported origin",
			mutate:  func(b *SessionBundle) { b.Cookies[0].URL = "https://evil.example.com/" },
			wantErr: "unsupported url",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := validTestBundle(live)
			tc.mutate(bundle)
			if err := bundle.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want a validation failure")
			} else if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestDecodeSessionBundleParsesExportedDocument(t *testing.T) {
	document := []byte(`{
  "kind": "asc-web-session",
  "version": 1,
  "exportedAt": "2026-09-02T10:00:00Z",
  "appleId": "user@example.com",
  "cookies": [
    {"url": "https://appstoreconnect.apple.com/", "name": "myacinfo", "value": "token", "path": "/", "secure": true, "httpOnly": true}
  ]
}`)

	bundle, err := DecodeSessionBundle(document)
	if err != nil {
		t.Fatalf("DecodeSessionBundle() error = %v", err)
	}
	if bundle.AppleID != "user@example.com" || len(bundle.Cookies) != 1 {
		t.Fatalf("unexpected bundle: %+v", bundle)
	}
	if !bundle.Cookies[0].Secure || !bundle.Cookies[0].HTTPOnly {
		t.Fatalf("cookie attributes were dropped: %+v", bundle.Cookies[0])
	}
}

func TestDecodeSessionBundleRejectsMalformedJSON(t *testing.T) {
	if _, err := DecodeSessionBundle([]byte("not json")); err == nil {
		t.Fatal("DecodeSessionBundle() error = nil, want a decode failure")
	}
	if _, err := DecodeSessionBundle(nil); err == nil {
		t.Fatal("DecodeSessionBundle(nil) error = nil, want a decode failure")
	}
}

func TestDecodeSessionBundleRejectsUnknownFields(t *testing.T) {
	document := []byte(`{
  "kind": "asc-web-session",
  "version": 1,
  "appleId": "user@example.com",
  "cookies": [{
    "url": "https://appstoreconnect.apple.com/",
    "name": "myacinfo",
    "value": "token",
    "expire": "2099-01-01T00:00:00Z"
  }]
}`)

	if _, err := DecodeSessionBundle(document); err == nil {
		t.Fatal("DecodeSessionBundle() error = nil, want an unknown-field failure")
	} else if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeSessionBundle() error = %v, want unknown-field context", err)
	}
}

func TestImportSessionBundleRejectsInvalidCookieSyntax(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Value = "token;injected"

	if err := bundle.Validate(); !errors.Is(err, ErrSessionCookieInvalid) {
		t.Fatalf("Validate() error = %v, want ErrSessionCookieInvalid", err)
	}
	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionCookieInvalid) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionCookieInvalid", err)
	}
	if err := bundle.Validate(); err != nil && strings.Contains(err.Error(), "token;injected") {
		t.Fatalf("validation error leaked a cookie value: %v", err)
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil || ok {
		t.Fatalf("LoadCachedSession() = (%v, %v), want no cached session after a refused import", ok, err)
	}
}

func TestImportSessionBundleRejectsCookieDomainTheJarCannotStore(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Domain = "evil.example"

	if err := bundle.Validate(); !errors.Is(err, ErrSessionCookieNotStorable) {
		t.Fatalf("Validate() error = %v, want ErrSessionCookieNotStorable", err)
	}
	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionCookieNotStorable) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionCookieNotStorable", err)
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil || ok {
		t.Fatalf("LoadCachedSession() = (%v, %v), want no cached session after a refused import", ok, err)
	}
}

func TestImportSessionBundleRejectsPublicSuffixCookieDomain(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Domain = "com"

	if err := bundle.Validate(); !errors.Is(err, ErrSessionCookieNotStorable) {
		t.Fatalf("Validate() error = %v, want ErrSessionCookieNotStorable", err)
	}
	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionCookieNotStorable) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionCookieNotStorable", err)
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil || ok {
		t.Fatalf("LoadCachedSession() = (%v, %v), want no cached session after a refused import", ok, err)
	}
}

func TestImportSessionBundleRejectsCookieDomainBroaderThanOrigin(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Domain = "apple.com"

	if err := bundle.Validate(); !errors.Is(err, ErrSessionCookieNotStorable) {
		t.Fatalf("Validate() error = %v, want ErrSessionCookieNotStorable", err)
	}
}

func TestImportSessionBundleAcceptsOriginHostCookieDomain(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Domain = "appstoreconnect.apple.com"

	if err := bundle.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil for the origin host", err)
	}
	if _, err := ImportSessionBundle(bundle); err != nil {
		t.Fatalf("ImportSessionBundle() error = %v", err)
	}
}

func TestImportSessionBundleTreatsExplicitZeroExpiryAsExpired(t *testing.T) {
	withFileSessionCache(t)
	zero := time.Time{}
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Expires = &zero

	if _, err := ImportSessionBundle(bundle); !errors.Is(err, ErrSessionBundleUnusable) {
		t.Fatalf("ImportSessionBundle() error = %v, want ErrSessionBundleUnusable", err)
	}
	if _, ok, err := LoadCachedSession("user@example.com"); err != nil || ok {
		t.Fatalf("LoadCachedSession() = (%v, %v), want no cached session after a refused import", ok, err)
	}
}

func TestImportSessionBundleFailsWhenLastSessionPointerCannotBeUpdated(t *testing.T) {
	withFileSessionCache(t)
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	if err := os.MkdirAll(lastPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", lastPath, err)
	}

	bundle := validTestBundle(time.Now().Add(time.Hour))
	if _, err := ImportSessionBundle(bundle); err == nil {
		t.Fatal("ImportSessionBundle() error = nil, want a last-session pointer failure")
	}
	key := webSessionCacheKey(bundle.AppleID)
	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile() error = %v", err)
	} else if ok {
		t.Fatal("failed import left a new session cache entry behind")
	}
}

func TestImportSessionBundleRestoresExistingSessionWhenLastSessionPointerCannotBeUpdated(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	old := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-token", Path: "/"}},
		},
	}
	if err := writeSessionToFile(key, old); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	if err := os.Remove(lastPath); err != nil {
		t.Fatalf("Remove(%q) error = %v", lastPath, err)
	}
	if err := os.MkdirAll(lastPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", lastPath, err)
	}

	if _, err := ImportSessionBundle(bundle); err == nil {
		t.Fatal("ImportSessionBundle() error = nil, want a last-session pointer failure")
	}
	restored, ok, err := readSessionFromFile(key)
	if err != nil {
		t.Fatalf("readSessionFromFile() error = %v", err)
	}
	if !ok {
		t.Fatal("failed import removed the previous session cache entry")
	}
	if got := restored.Cookies["https://appstoreconnect.apple.com/"][0].Value; got != "old-token" {
		t.Fatalf("restored cookie value = %q, want old-token", got)
	}
}

func TestImportSessionBundleOverwritesMalformedKeychainStore(t *testing.T) {
	kr := withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "unused-file-cache"))
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: []byte("{")}); err != nil {
		t.Fatalf("seed malformed keychain store: %v", err)
	}

	bundle := validTestBundle(time.Now().Add(time.Hour))
	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundle() error = %v, want overwrite of a malformed keychain store", err)
	}
	loaded, ok, err := LoadCachedSession("user@example.com")
	if err != nil || !ok || loaded == nil {
		t.Fatalf("LoadCachedSession() = (%v, %v, %v), want the imported session", loaded, ok, err)
	}
}

func TestImportSessionBundleOverwriteRemovesDefaultKeychainFallback(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	if err := writeSessionToKeychain(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToKeychain() error = %v", err)
	}

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v", err)
	}
	if _, ok, err := readSessionFromKeychain(key); err != nil {
		t.Fatalf("readSessionFromKeychain() error = %v", err)
	} else if ok {
		t.Fatal("overwrite left the stale keychain fallback entry behind")
	}
	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile() error = %v", err)
	} else if !ok {
		t.Fatal("overwrite did not persist the imported file-backed session")
	}
}

func TestImportSessionBundleConvertsPositiveMaxAgeToAbsoluteExpiry(t *testing.T) {
	withFileSessionCache(t)
	bundle := validTestBundle(time.Now().Add(time.Hour))
	bundle.Cookies[0].Expires = nil
	bundle.Cookies[0].MaxAge = 60

	before := time.Now().UTC()
	summary, err := ImportSessionBundle(bundle)
	if err != nil {
		t.Fatalf("ImportSessionBundle() error = %v", err)
	}
	after := time.Now().UTC()
	if summary.CookieCount != 1 || summary.SkippedExpired != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	key := webSessionCacheKey("user@example.com")
	sess, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %v), want the imported session", ok, err)
	}
	got := sess.Cookies["https://appstoreconnect.apple.com/"]
	if len(got) != 1 {
		t.Fatalf("imported cookies = %#v, want 1 cookie", sess.Cookies)
	}
	if got[0].MaxAge != 0 {
		t.Fatalf("MaxAge = %d, want 0 after converting to an absolute expiry", got[0].MaxAge)
	}
	if got[0].Expires.IsZero() {
		t.Fatal("Expires is zero, want an absolute deadline derived from MaxAge")
	}
	wantEarliest := before.Add(60 * time.Second)
	wantLatest := after.Add(60 * time.Second)
	if got[0].Expires.Before(wantEarliest) || got[0].Expires.After(wantLatest) {
		t.Fatalf("Expires = %v, want between %v and %v", got[0].Expires, wantEarliest, wantLatest)
	}
}

func TestImportSessionBundleDoesNotMergeDifferentAppleIDs(t *testing.T) {
	withFileSessionCache(t)
	firstExpires := time.Now().Add(time.Hour)
	persistTestSession(
		t, "first@example.com",
		&http.Cookie{Name: "myacinfo", Value: "first-token", Path: "/", Expires: firstExpires},
	)

	bundle := validTestBundle(time.Now().Add(2 * time.Hour))
	bundle.AppleID = "second@example.com"
	bundle.Cookies[0].Name = "myacinfo"
	bundle.Cookies[0].Value = "second-token"
	previousFetcher := sessionInfoFetcher
	sessionInfoFetcher = func(context.Context, *http.Client) (*sessionInfo, error) {
		out := &sessionInfo{}
		out.User.EmailAddress = "second@example.com"
		return out, nil
	}
	t.Cleanup(func() { sessionInfoFetcher = previousFetcher })

	if _, err := ImportSessionBundle(bundle); err != nil {
		t.Fatalf("ImportSessionBundle() error = %v", err)
	}

	first, ok, err := LoadCachedSession("first@example.com")
	if err != nil || !ok || first == nil {
		t.Fatalf("LoadCachedSession(first) = (%v, %v, %v), want the original session", first, ok, err)
	}
	target, _ := url.Parse("https://appstoreconnect.apple.com/")
	firstValues := map[string]string{}
	for _, cookie := range first.Client.Jar.Cookies(target) {
		firstValues[cookie.Name] = cookie.Value
	}
	if firstValues["myacinfo"] != "first-token" {
		t.Fatalf("first account cookies = %#v, want the original value", firstValues)
	}

	second, ok, err := LoadCachedSession("second@example.com")
	if err != nil || !ok || second == nil {
		t.Fatalf("LoadCachedSession(second) = (%v, %v, %v), want the imported session", second, ok, err)
	}

	last, ok, err := LoadLastCachedSession()
	if err != nil || !ok || last == nil {
		t.Fatalf("LoadLastCachedSession() = (%v, %v, %v), want the imported session", last, ok, err)
	}
	if last.UserEmail != "second@example.com" {
		t.Fatalf("last session UserEmail = %q, want second@example.com", last.UserEmail)
	}
}

func TestImportSessionBundleNoOverwriteDoesNotReplaceKeychainSession(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	old := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-keychain-token", Path: "/"}},
		},
	}
	if err := writeSessionToKeychain(key, old); err != nil {
		t.Fatalf("writeSessionToKeychain() error = %v", err)
	}

	if _, err := ImportSessionBundleWithOptions(bundle, false); !errors.Is(err, os.ErrExist) {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want an atomic no-overwrite conflict", err)
	}
	loaded, ok, err := readSessionFromKeychain(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromKeychain() = (%v, %v), want the original session", ok, err)
	}
	if got := loaded.Cookies["https://appstoreconnect.apple.com/"][0].Value; got != "old-keychain-token" {
		t.Fatalf("cached keychain cookie = %q, want old-keychain-token", got)
	}
}

// The command-layer preflight can observe an empty file cache while another
// process writes the same account to the keychain before import reaches its
// final persistence boundary. The file backend's keychain fallback must treat
// that writer as an occupied entry and preserve its last-session choice.
func TestImportSessionBundleNoOverwritePreservesSessionWrittenDuringKeychainFallbackPreflight(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
	withStubbedSessionSharedLockRoot(t, t.TempDir())

	key := webSessionCacheKey(webTestSessionEmail)
	importer := backendSelection{backend: sessionBackendFile, fallbackKeychain: true}
	writer := backendSelection{backend: sessionBackendKeychain}

	if _, ok, err := readSessionBySelection(importer, key); err != nil || ok {
		t.Fatalf("import preflight = (%v, %v), want an empty cache", ok, err)
	}
	if err := persistSessionBySelection(writer, key, webTestPersistedSession(t, "keychain-token", time.Now().UTC())); err != nil {
		t.Fatalf("concurrent keychain writer error: %v", err)
	}

	err := persistImportedSessionBySelection(importer, key, webTestPersistedSession(t, "import-token", time.Now().UTC()), false)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-overwrite import error = %v, want an atomic keychain-fallback conflict", err)
	}
	stored, ok, err := readSessionFromKeychain(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromKeychain() = (%v, %v), want the concurrent writer", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "keychain-token" {
		t.Fatalf("keychain cookie = %q, want keychain-token", got)
	}
	last, ok, err := readLastSessionFromKeychain()
	if err != nil || !ok || last.UserEmail != webTestSessionEmail {
		t.Fatalf("readLastSessionFromKeychain() = (%+v, %v, %v), want the concurrent writer's marker", last, ok, err)
	}
}

// The selected file backend must enforce the same guarantee at its final
// O_EXCL create, even when both the preflight and the writer use that backend.
func TestImportSessionBundleNoOverwritePreservesSessionWrittenDuringFilePreflight(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
	withStubbedSessionSharedLockRoot(t, t.TempDir())

	key := webSessionCacheKey(webTestSessionEmail)
	selection := backendSelection{backend: sessionBackendFile}

	if _, ok, err := readSessionBySelection(selection, key); err != nil || ok {
		t.Fatalf("import preflight = (%v, %v), want an empty cache", ok, err)
	}
	if err := persistSessionBySelection(selection, key, webTestPersistedSession(t, "file-token", time.Now().UTC())); err != nil {
		t.Fatalf("concurrent file writer error: %v", err)
	}

	err := persistImportedSessionBySelection(selection, key, webTestPersistedSession(t, "import-token", time.Now().UTC()), false)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-overwrite import error = %v, want an atomic file conflict", err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %v), want the concurrent writer", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "file-token" {
		t.Fatalf("file cookie = %q, want file-token", got)
	}
	lastKey, ok, err := readLastKeyFromFile()
	if err != nil || !ok || lastKey != key {
		t.Fatalf("readLastKeyFromFile() = (%q, %v, %v), want the concurrent writer's marker", lastKey, ok, err)
	}
}

// The reverse configuration has the same final-write guarantee: a keychain
// selected import must not replace a file entry that appeared after its
// command-layer preflight.
func TestImportSessionBundleNoOverwritePreservesSessionWrittenDuringFileFallbackPreflight(t *testing.T) {
	withArraySessionKeyring(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))
	withStubbedSessionSharedLockRoot(t, t.TempDir())

	key := webSessionCacheKey(webTestSessionEmail)
	importer := backendSelection{backend: sessionBackendKeychain, fallbackFile: true}
	writer := backendSelection{backend: sessionBackendFile}

	if _, ok, err := readSessionBySelection(importer, key); err != nil || ok {
		t.Fatalf("import preflight = (%v, %v), want an empty cache", ok, err)
	}
	if err := persistSessionBySelection(writer, key, webTestPersistedSession(t, "file-token", time.Now().UTC())); err != nil {
		t.Fatalf("concurrent file writer error: %v", err)
	}

	err := persistImportedSessionBySelection(importer, key, webTestPersistedSession(t, "import-token", time.Now().UTC()), false)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-overwrite import error = %v, want an atomic file-fallback conflict", err)
	}
	stored, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %v), want the concurrent writer", ok, err)
	}
	if got := persistedMyacinfoCookieValue(stored, "https://appstoreconnect.apple.com/"); got != "file-token" {
		t.Fatalf("file cookie = %q, want file-token", got)
	}
	lastKey, ok, err := readLastKeyFromFile()
	if err != nil || !ok || lastKey != key {
		t.Fatalf("readLastKeyFromFile() = (%q, %v, %v), want the concurrent writer's marker", lastKey, ok, err)
	}
}

func TestImportSessionBundleOverwriteRemovesFileFallbackOnKeychainBackend(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}

	if _, err := ImportSessionBundleWithOptions(bundle, true); err != nil {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v", err)
	}
	if _, ok, err := readSessionFromKeychain(key); err != nil {
		t.Fatalf("readSessionFromKeychain() error = %v", err)
	} else if !ok {
		t.Fatal("keychain overwrite did not persist the imported session")
	}
	if _, ok, err := readSessionFromFile(key); err != nil {
		t.Fatalf("readSessionFromFile() error = %v", err)
	} else if ok {
		t.Fatal("overwrite left the stale file fallback credential behind")
	}
	if _, ok, err := readLastKeyFromFile(); err != nil {
		t.Fatalf("readLastKeyFromFile() error = %v", err)
	} else if ok {
		t.Fatal("overwrite left the stale file last-session pointer behind")
	}
}

func TestImportSessionBundleLeavesFileCacheIntactWhenKeychainCleanupFails(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}

	// A refused keychain unlock is not an unavailable keyring, so the mirrored
	// cleanup fails for a reason the import must report.
	previousOpen := sessionKeyringOpen
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		return nil, errors.New("keychain unlock refused")
	}
	t.Cleanup(func() { sessionKeyringOpen = previousOpen })

	if _, err := ImportSessionBundleWithOptions(bundle, true); err == nil {
		t.Fatal("ImportSessionBundleWithOptions() error = nil, want the mirrored keychain cleanup failure")
	}

	sess, ok, err := readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %v), want the untouched cached session", ok, err)
	}
	cookies := sess.Cookies["https://appstoreconnect.apple.com/"]
	if len(cookies) != 1 || cookies[0].Value != "old-token" {
		t.Fatalf("cached cookies = %#v, want the import to report failure without replacing them", cookies)
	}
}

func TestImportSessionBundleRestoresFileMirrorAfterKeychainPersistenceFails(t *testing.T) {
	withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	lastRaw := []byte(`{"key":"` + key + `","version":1}` + "\n")
	if err := os.WriteFile(lastPath, lastRaw, 0o640); err != nil {
		t.Fatalf("write last-session pointer: %v", err)
	}
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	previousSessionRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read previous session cache: %v", err)
	}

	previousOpen := sessionKeyringOpen
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		return nil, errors.New("keychain unlock refused")
	}
	t.Cleanup(func() { sessionKeyringOpen = previousOpen })

	if _, err := ImportSessionBundleWithOptions(bundle, true); err == nil {
		t.Fatal("ImportSessionBundleWithOptions() error = nil, want the keychain persistence failure")
	}

	gotSessionRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read restored session cache: %v", err)
	}
	if string(gotSessionRaw) != string(previousSessionRaw) {
		t.Fatalf("restored session cache changed: got %q, want %q", gotSessionRaw, previousSessionRaw)
	}
	gotLastRaw, err := os.ReadFile(lastPath)
	if err != nil {
		t.Fatalf("read restored last-session pointer: %v", err)
	}
	if string(gotLastRaw) != string(lastRaw) {
		t.Fatalf("restored last-session pointer changed: got %q, want %q", gotLastRaw, lastRaw)
	}
}

func TestImportSessionBundleKeychainOverwriteDoesNotFallbackAfterCapture(t *testing.T) {
	kr := withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	old := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-token", Path: "/"}},
		},
	}
	if err := writeSessionToKeychain(key, old); err != nil {
		t.Fatalf("writeSessionToKeychain() error = %v", err)
	}
	if err := writeSessionToFile(key, old); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}

	previousOpen := sessionKeyringOpen
	openCalls := 0
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		openCalls++
		if openCalls == 2 {
			return nil, keyring.ErrNoAvailImpl
		}
		return kr, nil
	}
	t.Cleanup(func() { sessionKeyringOpen = previousOpen })

	if _, err := ImportSessionBundleWithOptions(bundle, true); !errors.Is(err, keyring.ErrNoAvailImpl) {
		t.Fatalf("ImportSessionBundleWithOptions() error = %v, want transient keychain failure", err)
	}
	if openCalls < 3 {
		t.Fatalf("session keychain opened %d times, want capture, failed write, and restore", openCalls)
	}
	stored, ok, err := readSessionFromKeychain(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromKeychain() = (%v, %v), want the original session", ok, err)
	}
	if got := stored.Cookies["https://appstoreconnect.apple.com/"][0].Value; got != "old-token" {
		t.Fatalf("keychain cookie = %q, want old-token", got)
	}
	stored, ok, err = readSessionFromFile(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromFile() = (%v, %v), want the original mirror", ok, err)
	}
	if got := stored.Cookies["https://appstoreconnect.apple.com/"][0].Value; got != "old-token" {
		t.Fatalf("file cookie = %q, want old-token", got)
	}
}

func TestImportSessionBundleRestoresKeychainMirrorAfterFilePersistenceFails(t *testing.T) {
	testKeyring := withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "web-cache"))

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	if err := writeSessionToFile(key, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-file-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	lastRaw := []byte(`{"key":"` + key + `","version":1}` + "\n")
	if err := os.WriteFile(lastPath, lastRaw, 0o640); err != nil {
		t.Fatalf("write last-session pointer: %v", err)
	}
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	previousSessionRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read previous session cache: %v", err)
	}

	keychainSession := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
		UserEmail: bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-keychain-token", Path: "/"}},
		},
	}
	if err := writeSessionToKeychain(key, keychainSession); err != nil {
		t.Fatalf("writeSessionToKeychain() error = %v", err)
	}
	otherKey := webSessionCacheKey("other@example.com")
	if err := writeSessionToKeychain(otherKey, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-2 * time.Hour),
		UserEmail: "other@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "other-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToKeychain(other) error = %v", err)
	}
	keychainItem, err := testKeyring.Get(webSessionStoreItem)
	if err != nil {
		t.Fatalf("read previous keychain store: %v", err)
	}
	previousKeychainRaw := append([]byte(nil), keychainItem.Data...)

	previousWrite := sessionFileWrite
	sessionFileWrite = func(path string, file *os.File, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, ".tmp") && strings.Contains(filepath.Base(path), "session-") {
			return errors.New("file replacement refused")
		}
		return previousWrite(path, file, data, perm)
	}
	t.Cleanup(func() { sessionFileWrite = previousWrite })

	if _, err := ImportSessionBundleWithOptions(bundle, true); err == nil {
		t.Fatal("ImportSessionBundleWithOptions() error = nil, want the file persistence failure")
	}

	gotSessionRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read restored session cache: %v", err)
	}
	if string(gotSessionRaw) != string(previousSessionRaw) {
		t.Fatalf("restored session cache changed: got %q, want %q", gotSessionRaw, previousSessionRaw)
	}
	gotLastRaw, err := os.ReadFile(lastPath)
	if err != nil {
		t.Fatalf("read restored last-session pointer: %v", err)
	}
	if string(gotLastRaw) != string(lastRaw) {
		t.Fatalf("restored last-session pointer changed: got %q, want %q", gotLastRaw, lastRaw)
	}
	restored, ok, err := readSessionFromKeychain(key)
	if err != nil || !ok {
		t.Fatalf("readSessionFromKeychain() = (%v, %v), want the restored mirror", ok, err)
	}
	if got := restored.Cookies["https://appstoreconnect.apple.com/"][0].Value; got != "old-keychain-token" {
		t.Fatalf("restored keychain cookie = %q, want old-keychain-token", got)
	}
	other, ok, err := readSessionFromKeychain(otherKey)
	if err != nil || !ok {
		t.Fatalf("readSessionFromKeychain(other) = (%v, %v), want unrelated state preserved", ok, err)
	}
	if got := other.Cookies["https://appstoreconnect.apple.com/"][0].Value; got != "other-token" {
		t.Fatalf("restored unrelated keychain cookie = %q, want other-token", got)
	}
	gotKeychainItem, err := testKeyring.Get(webSessionStoreItem)
	if err != nil {
		t.Fatalf("read restored keychain store: %v", err)
	}
	if string(gotKeychainItem.Data) != string(previousKeychainRaw) {
		t.Fatalf("restored keychain store changed: got %q, want %q", gotKeychainItem.Data, previousKeychainRaw)
	}
}

func TestImportSessionBundleRestoresPriorStateWhenLastPointerWriteFailsAfterSessionRename(t *testing.T) {
	testKeyring := withArraySessionKeyring(t)
	withSessionInfoStub(t)
	t.Setenv(webSessionCacheEnabledEnv, "1")
	t.Setenv(webSessionBackendEnv, "")
	cacheDir := filepath.Join(t.TempDir(), "web-cache")
	t.Setenv(webSessionCacheDirEnv, cacheDir)

	bundle := validTestBundle(time.Now().Add(time.Hour))
	key := webSessionCacheKey(bundle.AppleID)
	old := persistedSession{
		Version:    webSessionCacheVersion,
		UpdatedAt:  time.Now().UTC().Add(-time.Hour),
		Generation: "prior-generation",
		UserEmail:  bundle.AppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-file-token", Path: "/"}},
		},
	}
	if err := writeSessionToFile(key, old); err != nil {
		t.Fatalf("writeSessionToFile() error = %v", err)
	}

	// Make the previous last-session pointer deliberately select another
	// account. The rollback must restore its bytes, rather than merely point
	// back at the account being replaced.
	otherKey := webSessionCacheKey("other@example.com")
	if err := writeSessionToFile(otherKey, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-2 * time.Hour),
		UserEmail: "other@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "other-file-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToFile(other) error = %v", err)
	}

	// Keep a keychain mirror for the selected account and an unrelated account;
	// the raw aggregate proves that rollback restored both entries and its
	// previous last-session choice after the file rename already happened.
	if err := writeSessionToKeychain(key, persistedSession{
		Version:    webSessionCacheVersion,
		UpdatedAt:  old.UpdatedAt,
		Generation: old.Generation,
		UserEmail:  old.UserEmail,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old-keychain-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToKeychain() error = %v", err)
	}
	if err := writeSessionToKeychain(otherKey, persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC().Add(-2 * time.Hour),
		UserEmail: "other@example.com",
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "other-keychain-token", Path: "/"}},
		},
	}); err != nil {
		t.Fatalf("writeSessionToKeychain(other) error = %v", err)
	}

	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	priorSessionRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read prior session cache: %v", err)
	}
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		t.Fatalf("webSessionLastFilePath() error = %v", err)
	}
	priorLastRaw, err := os.ReadFile(lastPath)
	if err != nil {
		t.Fatalf("read prior last-session pointer: %v", err)
	}
	priorStore, err := testKeyring.Get(webSessionStoreItem)
	if err != nil {
		t.Fatalf("read prior keychain store: %v", err)
	}
	priorStoreRaw := append([]byte(nil), priorStore.Data...)

	previousWrite := sessionFileWrite
	sessionWriteSucceeded := false
	sessionFileWrite = func(path string, file *os.File, data []byte, perm os.FileMode) error {
		base := filepath.Base(path)
		if strings.Contains(base, "last.json-") {
			if !sessionWriteSucceeded {
				return errors.New("injected before session rename")
			}
			return errors.New("injected last-session pointer write failure")
		}
		err := previousWrite(path, file, data, perm)
		if err == nil && strings.Contains(base, "session-"+key+".json-") {
			sessionWriteSucceeded = true
		}
		return err
	}
	t.Cleanup(func() { sessionFileWrite = previousWrite })

	if _, err := ImportSessionBundleWithOptions(bundle, true); err == nil {
		t.Fatal("ImportSessionBundleWithOptions() error = nil, want the injected last-pointer failure")
	}
	if !sessionWriteSucceeded {
		t.Fatal("fault injection did not reach the last-pointer write after the session temp write")
	}

	gotSessionRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read restored session cache: %v", err)
	}
	if string(gotSessionRaw) != string(priorSessionRaw) {
		t.Fatalf("restored session cache changed: got %q, want %q", gotSessionRaw, priorSessionRaw)
	}
	gotLastRaw, err := os.ReadFile(lastPath)
	if err != nil {
		t.Fatalf("read restored last-session pointer: %v", err)
	}
	if string(gotLastRaw) != string(priorLastRaw) {
		t.Fatalf("restored last-session pointer changed: got %q, want %q", gotLastRaw, priorLastRaw)
	}
	gotStore, err := testKeyring.Get(webSessionStoreItem)
	if err != nil {
		t.Fatalf("read restored keychain store: %v", err)
	}
	if string(gotStore.Data) != string(priorStoreRaw) {
		t.Fatalf("restored keychain store changed: got %q, want %q", gotStore.Data, priorStoreRaw)
	}
}
