package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	staleCookieTestAppleID  = "fixture@example.invalid"
	staleCookieTestPassword = "fixture-password-value"
	staleCookieTestTrust    = "DESfixture0123456789abcdef"
)

// cookieHeaderNames returns every cookie name exactly as it appears on the
// wire, including duplicates, so a test can see what IdMSA would receive.
func cookieHeaderNames(r *http.Request) []string {
	var names []string
	for _, header := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(header, ";") {
			name, _, _ := strings.Cut(strings.TrimSpace(part), "=")
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func TestSigninInitSendsEachCookieOnceWithQuotedTrustCookie(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	idmsa := &url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/"}
	jar.SetCookies(idmsa, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Path: "/"},
		{Name: "aasp", Value: "aasp-value", Path: "/"},
	})
	client := &http.Client{Jar: newSessionCookieTrackingJar(jar), Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		names := cookieHeaderNames(req)
		seen := map[string]int{}
		for _, name := range names {
			seen[name]++
		}
		if len(req.Header.Values("Cookie")) != 1 {
			t.Fatalf("expected one Cookie header, got %d", len(req.Header.Values("Cookie")))
		}
		if seen[staleCookieTestTrust] != 1 || seen["aasp"] != 1 || len(names) != 2 {
			t.Fatalf("expected each cookie exactly once, got names %v", names)
		}
		if header := req.Header.Get("Cookie"); !strings.Contains(header, staleCookieTestTrust+`="trust-value"`) {
			t.Fatal("expected the DES trust cookie value to keep fastlane's quoted form")
		}
		return jsonResponse(http.StatusOK, signinInitFixtureBody(t), nil), nil
	})}

	if _, err := signinInit(context.Background(), client, staleCookieTestAppleID, "A", "service-key"); err != nil {
		t.Fatalf("signinInit: %v", err)
	}
}

// TestExpiredDomainScopedTrustCookieIsNotReplayedToSignin reproduces #2806: Apple scopes its
// sign-in cookies with a Domain attribute, and the cache used to drop their
// deadlines, so a cookie Apple had expired was replayed to IdMSA forever.
func TestExpiredDomainScopedTrustCookieIsNotReplayedToSignin(t *testing.T) {
	t.Setenv(webSessionCacheDirEnv, t.TempDir())
	t.Setenv(webSessionBackendEnv, "file")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := newSessionCookieTrackingJar(jar)
	complete := &url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/appleauth/auth/2sv/trust"}
	tracked.SetCookies(complete, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 2592000, Secure: true, HttpOnly: true},
		{Name: "aasp", Value: "aasp-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 3600, Secure: true, HttpOnly: true},
		{Name: "myacinfo", Value: "session-value", Domain: "apple.com", Path: "/", Secure: true, HttpOnly: true},
	})
	session := &AuthSession{Client: &http.Client{Jar: tracked}, UserEmail: staleCookieTestAppleID}
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}

	// Let 31 days pass: both Apple deadlines are now in the past.
	shiftPersistedSessionClock(t, staleCookieTestAppleID, -31*24*time.Hour)

	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	var sawStaleCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appleauth/auth/signin/init" {
			serveSigninFixture(t, w, r)
			return
		}
		for _, name := range cookieHeaderNames(r) {
			if name == staleCookieTestTrust || name == "aasp" {
				sawStaleCookie = true
			}
		}
		if sawStaleCookie {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"serviceErrors":[{"code":"-20209","message":"Service unavailable."}]}`))
			return
		}
		_, _ = w.Write(signinInitFixtureBody(t))
	}))
	defer server.Close()
	loaded.Client.Transport = newTestServerRoutedClient(t, server).Transport

	_, err = LoginWithClient(context.Background(), loaded.Client, LoginCredentials{Username: staleCookieTestAppleID, Password: staleCookieTestPassword})
	if sawStaleCookie {
		t.Fatalf("expired cached trust cookies were replayed to signin/init: %v", err)
	}
	var tfa *TwoFactorRequiredError
	if !errors.As(err, &tfa) {
		t.Fatalf("expected sign-in to reach two-factor, got %v", err)
	}
}

// TestSignInFromCachedSessionBootstrapsWithTheIssuedCookie reproduces a code
// Apple accepted being followed by a 401 from a cached jar. Olympus here reads
// only the first myacinfo, as Apple does.
func TestSignInFromCachedSessionBootstrapsWithTheIssuedCookie(t *testing.T) {
	t.Setenv(webSessionCacheDirEnv, t.TempDir())
	t.Setenv(webSessionBackendEnv, "file")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := newSessionCookieTrackingJar(jar)
	tracked.SetCookies(&url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/appleauth/auth/2sv/trust"}, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 2592000, Secure: true, HttpOnly: true},
		{Name: "myacinfo", Value: "expired-session", Domain: "apple.com", Path: "/", Secure: true, HttpOnly: true},
	})
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: tracked}, UserEmail: staleCookieTestAppleID}); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	trustDeadline := persistedTrustCookieDeadline(t)

	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	var sawTrustCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/appleauth/auth/signin/init":
			sawTrustCookie = slices.Contains(cookieHeaderNames(r), staleCookieTestTrust)
			_, _ = w.Write(signinInitFixtureBody(t))
		case "/appleauth/auth":
			_, _ = w.Write([]byte(`{"trustedDevices":[{}]}`))
		case "/appleauth/auth/verify/trusteddevice/securitycode":
			w.WriteHeader(http.StatusNoContent)
		case "/appleauth/auth/2sv/trust":
			w.Header().Add("Set-Cookie", "myacinfo=issued-session; Domain=apple.com; Path=/; Secure; HttpOnly")
			w.WriteHeader(http.StatusNoContent)
		case "/olympus/v1/session":
			if cookie, err := r.Cookie("myacinfo"); err != nil || cookie.Value != "issued-session" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"user":{"emailAddress":"` + staleCookieTestAppleID + `"},"provider":{"providerId":1}}`))
		default:
			serveSigninFixture(t, w, r)
		}
	}))
	defer server.Close()
	loaded.Client.Transport = newTestServerRoutedClient(t, server).Transport

	session, err := LoginWithClient(context.Background(), loaded.Client, LoginCredentials{Username: staleCookieTestAppleID, Password: staleCookieTestPassword})
	var tfa *TwoFactorRequiredError
	if !errors.As(err, &tfa) {
		t.Fatalf("expected sign-in to reach two-factor, got %v", err)
	}
	if !sawTrustCookie {
		t.Fatal("expected the cached trust cookie to reach signin/init")
	}
	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode: %v", err)
	}

	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession after sign-in: %v", err)
	}
	if got := persistedTrustCookieDeadline(t); !got.Equal(trustDeadline) {
		t.Fatalf("trust cookie deadline after sign-in = %v, want %v", got, trustDeadline)
	}
	resumed, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession after sign-in ok=%v err=%v", ok, err)
	}
	cookies := resumed.Client.Jar.Cookies(&url.URL{Scheme: "https", Host: "appstoreconnect.apple.com", Path: "/"})
	if observedCookieNameCount(cookies, "myacinfo") != 1 || cookieValue(cookies, "myacinfo") != "issued-session" {
		t.Fatal("expected the cache to hold only the issued myacinfo")
	}
}

func TestDomainScopedCookieDeadlineSurvivesRepersist(t *testing.T) {
	t.Setenv(webSessionCacheDirEnv, t.TempDir())
	t.Setenv(webSessionBackendEnv, "file")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	tracked := newSessionCookieTrackingJar(jar)
	tracked.SetCookies(&url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/appleauth/auth/2sv/trust"}, []*http.Cookie{
		{Name: staleCookieTestTrust, Value: "trust-value", Domain: "idmsa.apple.com", Path: "/", MaxAge: 2592000},
		{Name: "myacinfo", Value: "session-value", Domain: "apple.com", Path: "/"},
	})
	if err := PersistSession(&AuthSession{Client: &http.Client{Jar: tracked}, UserEmail: staleCookieTestAppleID}); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	first := persistedTrustCookieDeadline(t)
	if first.IsZero() {
		t.Fatal("expected the Domain-scoped trust cookie to keep its Max-Age deadline")
	}

	// A resumed session that sees no new Set-Cookie must keep the deadline.
	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("re-persist: %v", err)
	}
	if again := persistedTrustCookieDeadline(t); !again.Equal(first) {
		t.Fatalf("trust cookie deadline after re-persist = %v, want %v", again, first)
	}
}

func persistedTrustCookieDeadline(t *testing.T) time.Time {
	t.Helper()
	path := filepath.Join(os.Getenv(webSessionCacheDirEnv), "session-"+webSessionCacheKey(staleCookieTestAppleID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached session: %v", err)
	}
	var sess persistedSession
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("decode cached session: %v", err)
	}
	for _, cookie := range sess.Cookies["https://idmsa.apple.com/"] {
		if cookie.Name == staleCookieTestTrust {
			return cookie.Expires
		}
	}
	t.Fatal("trust cookie missing from the cache")
	return time.Time{}
}

func TestSigninStageServiceErrorIsActionable(t *testing.T) {
	for _, stage := range []string{"signin_init", "signin_complete"} {
		t.Run(stage, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusServiceUnavailable, []byte(`{"serviceErrors":[{"code":"-20209","message":"Your request could not be completed. Try again later."}]}`), http.Header{"Retry-After": []string{"120"}}), nil
			})}
			var err error
			if stage == "signin_init" {
				_, err = signinInit(context.Background(), client, staleCookieTestAppleID, "A", "service-key")
			} else {
				err = signinComplete(context.Background(), client, staleCookieTestAppleID, "m1", "m2", json.RawMessage(`{}`), "service-key", "")
			}
			var serviceErr *SigninServiceError
			if !errors.As(err, &serviceErr) {
				t.Fatalf("expected *SigninServiceError, got %T %v", err, err)
			}
			if serviceErr.Stage != stage || serviceErr.HTTPStatusCode() != http.StatusServiceUnavailable || serviceErr.RetryAfter != "120" {
				t.Fatalf("unexpected service error fields: %+v", serviceErr)
			}
			message := err.Error()
			for _, want := range []string{stage, "503", "-20209", "Your request could not be completed. Try again later.", "Retry-After: 120"} {
				if !strings.Contains(message, want) {
					t.Fatalf("expected %q in %q", want, message)
				}
			}
		})
	}
}

func TestSigninStageDebugLogShowsRedactedBodyExcerpt(t *testing.T) {
	origLogger := webDebugLogger
	origDebugEnabled := webDebugEnabledFn
	t.Cleanup(func() {
		webDebugLogger = origLogger
		webDebugEnabledFn = origDebugEnabled
	})
	var logs bytes.Buffer
	webDebugLogger = slog.New(slog.NewTextHandler(&logs, nil))
	webDebugEnabledFn = func() bool { return true }

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(&url.URL{Scheme: "https", Host: "idmsa.apple.com", Path: "/"}, []*http.Cookie{
		{Name: "myacinfo", Value: "cookie-secret-value", Path: "/"},
	})
	body := `{"serviceErrors":[{"code":"-20209","message":"Try again later."}],` +
		`"accountName":"` + staleCookieTestAppleID + `","m1":"m1-proof-secret","m2":"m2-proof-secret",` +
		`"echo":"myacinfo=cookie-secret-value ` + staleCookieTestPassword + `"}`
	client := &http.Client{Jar: jar, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, []byte(body), nil), nil
	})}
	_ = signinComplete(context.Background(), client, staleCookieTestAppleID, "m1-proof-secret", "m2-proof-secret", json.RawMessage(`{}`), "service-key", "")

	output := logs.String()
	if !strings.Contains(output, "body=") || !strings.Contains(output, "-20209") || !strings.Contains(output, "Try again later.") {
		t.Fatalf("expected a service error body excerpt in debug output, got %q", output)
	}
	for _, secret := range []string{"cookie-secret-value", "m1-proof-secret", "m2-proof-secret", staleCookieTestPassword, staleCookieTestAppleID} {
		if strings.Contains(output, secret) {
			t.Fatalf("debug output leaked %q: %q", secret, output)
		}
	}

	logs.Reset()
	htmlClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<html><head><title>Service Unavailable</title></head><body>" + staleCookieTestAppleID + "</body></html>")),
		}, nil
	})}
	_, _ = signinInit(context.Background(), htmlClient, staleCookieTestAppleID, "A", "service-key")
	output = logs.String()
	if !strings.Contains(output, "Service Unavailable") {
		t.Fatalf("expected HTML title in debug output, got %q", output)
	}
	if strings.Contains(output, staleCookieTestAppleID) {
		t.Fatalf("debug output leaked the non-JSON body: %q", output)
	}
}

// serveSigninFixture answers the sign-in steps a test does not need to inspect.
func serveSigninFixture(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	switch r.URL.Path {
	case "/logout":
		w.Header().Set("Location", "https://idmsa.apple.com/appleauth/signout?widgetKey=fixture-widget-key")
		w.WriteHeader(http.StatusFound)
	case "/appleauth/auth/signin":
		w.WriteHeader(http.StatusOK)
	case "/appleauth/auth/signin/complete":
		w.Header().Set("X-Apple-ID-Session-Id", "fixture-session")
		w.Header().Set("scnt", "fixture-scnt")
		w.WriteHeader(http.StatusConflict)
	default:
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}
}

func signinInitFixtureBody(t *testing.T) []byte {
	t.Helper()
	salt := make([]byte, 16)
	serverB := make([]byte, 256)
	if _, err := rand.Read(salt); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(serverB); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"iteration": 1000,
		"salt":      base64.StdEncoding.EncodeToString(salt),
		"protocol":  "s2k",
		"b":         base64.StdEncoding.EncodeToString(serverB),
		"c":         "fixture-challenge",
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func jsonResponse(status int, body []byte, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(bytes.NewReader(body))}
}

// shiftPersistedSessionClock moves every absolute timestamp in a cached
// session by delta, which is equivalent to the wall clock moving by -delta.
func shiftPersistedSessionClock(t *testing.T, appleID string, delta time.Duration) {
	t.Helper()
	path := filepath.Join(os.Getenv(webSessionCacheDirEnv), "session-"+webSessionCacheKey(appleID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached session: %v", err)
	}
	var sess persistedSession
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("decode cached session: %v", err)
	}
	sess.UpdatedAt = sess.UpdatedAt.Add(delta)
	for origin, cookies := range sess.Cookies {
		for i := range cookies {
			if !cookies[i].Expires.IsZero() {
				cookies[i].Expires = cookies[i].Expires.Add(delta)
			}
		}
		sess.Cookies[origin] = cookies
	}
	data, err = json.Marshal(sess)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestResumedSessionReplacesHydratedCookieWithReissuedDomainCookie covers a
// Domain cookie Apple re-issues mid-session on a cached jar. The cache
// hydrates it host-only, so the re-issued cookie used to sit beside the old
// one, be sent second, and make persistence drop both.
func TestResumedSessionReplacesHydratedCookieWithReissuedDomainCookie(t *testing.T) {
	withFileSessionCache(t)
	expires := time.Now().UTC().Add(time.Hour)
	if err := writeSessionToFile(webSessionCacheKey(staleCookieTestAppleID), persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: staleCookieTestAppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "myacinfo", Value: "old", ScopeDomain: "apple.com", Path: "/", Expires: expires}},
			"https://idmsa.apple.com/":           {{Name: "myacinfo", Value: "old", ScopeDomain: "apple.com", Path: "/", Expires: expires}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{{Name: "myacinfo", Value: "new", Domain: "apple.com", Path: "/", Expires: expires.Add(time.Hour), Secure: true, HttpOnly: true}})

	for _, origin := range []string{olympusSessionURL, "https://idmsa.apple.com/"} {
		u, _ := url.Parse(origin)
		cookies := loaded.Client.Jar.Cookies(u)
		if observedCookieNameCount(cookies, "myacinfo") != 1 || cookieValue(cookies, "myacinfo") != "new" {
			t.Fatalf("%s sends %v, want only the re-issued myacinfo", origin, cookies)
		}
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	reloaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("reload ok=%v err=%v", ok, err)
	}
	if got := cookieValue(reloaded.Client.Jar.Cookies(endpoint), "myacinfo"); got != "new" {
		t.Fatalf("persisted myacinfo = %q, want the re-issued value", got)
	}
}

func TestDomainCookieKeepsGenuineHostOnlyCookieOfTheSameName(t *testing.T) {
	withFileSessionCache(t)
	if err := writeSessionToFile(webSessionCacheKey(staleCookieTestAppleID), persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: staleCookieTestAppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {{Name: "token", Value: "host-only", Path: "/", Expires: time.Now().UTC().Add(time.Hour)}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{{Name: "token", Value: "domain", Domain: "apple.com", Path: "/"}})
	if got := observedCookieNameCount(loaded.Client.Jar.Cookies(endpoint), "token"); got != 2 {
		t.Fatalf("token sent %d times, want the host-only cookie kept beside the Domain cookie", got)
	}
}

func TestResumedSessionDropsHydratedCookieWhenDomainCookieIsDeleted(t *testing.T) {
	withFileSessionCache(t)
	expires := time.Now().UTC().Add(time.Hour)
	if err := writeSessionToFile(webSessionCacheKey(staleCookieTestAppleID), persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now().UTC(),
		UserEmail: staleCookieTestAppleID,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com/": {
				{Name: "myacinfo", Value: "old", ScopeDomain: "apple.com", Path: "/", Expires: expires},
				{Name: "itctx", Value: "provider", Path: "/", Expires: expires},
			},
			"https://idmsa.apple.com/": {{Name: "myacinfo", Value: "old", ScopeDomain: "apple.com", Path: "/", Expires: expires}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("LoadCachedSession ok=%v err=%v", ok, err)
	}
	endpoint, _ := url.Parse(olympusSessionURL)
	loaded.Client.Jar.SetCookies(endpoint, []*http.Cookie{{Name: "myacinfo", Domain: "apple.com", Path: "/", MaxAge: -1}})

	for _, origin := range []string{olympusSessionURL, "https://idmsa.apple.com/"} {
		u, _ := url.Parse(origin)
		if got := observedCookieNameCount(loaded.Client.Jar.Cookies(u), "myacinfo"); got != 0 {
			t.Fatalf("%s still sends myacinfo %d times after Apple deleted it", origin, got)
		}
	}
	if err := PersistSession(loaded); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	reloaded, ok, err := LoadCachedSession(staleCookieTestAppleID)
	if err != nil || !ok {
		t.Fatalf("reload ok=%v err=%v", ok, err)
	}
	if got := observedCookieNameCount(reloaded.Client.Jar.Cookies(endpoint), "myacinfo"); got != 0 {
		t.Fatalf("persisted session resurrects deleted myacinfo %d times", got)
	}
}
