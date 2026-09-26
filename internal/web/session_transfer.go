package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	// SessionBundleKind identifies an exported web-session document.
	SessionBundleKind = "asc-web-session"

	// SessionBundleVersion is the schema version written by the current
	// exporter. Importing a different version is refused instead of guessed.
	SessionBundleVersion = 1

	// MaxSessionBundleSize bounds how many bytes an imported bundle may hold.
	MaxSessionBundleSize = 1 << 20
)

var (
	// ErrSessionCacheDisabled reports that web-session caching is turned off,
	// so a session can neither be read from nor written to the cache.
	ErrSessionCacheDisabled = errors.New("web session cache is disabled")

	// ErrSessionBundleValidationFailed reports that an explicitly requested
	// live validation did not accept an imported session bundle.
	ErrSessionBundleValidationFailed = errors.New("web session bundle could not be validated")

	// ErrSessionBundleUnusable reports that a bundle carries no unexpired
	// cookie for a supported Apple origin.
	ErrSessionBundleUnusable = errors.New("web session bundle has no unexpired cookies")

	// ErrSessionCookieNotStorable reports that a cookie names a supported
	// origin but a Domain the session jar will not store for that origin.
	ErrSessionCookieNotStorable = errors.New("web session bundle cookie cannot be stored for its origin")

	// ErrSessionCookieInvalid reports that a cookie name, value, path, or
	// domain is not a valid HTTP cookie field.
	ErrSessionCookieInvalid = errors.New("web session bundle cookie is invalid")

	// ErrSessionCookieDuplicate reports that a bundle repeats a cookie identity
	// after its origin, domain, and path are canonicalized.
	ErrSessionCookieDuplicate = errors.New("web session bundle cookie identity is duplicated")
)

// SessionBundle is the portable representation of a cached Apple web session.
// It is written by `asc web auth export` and read by `asc web auth import` so
// an already-authenticated session can move to another machine or to CI
// without repeating two-factor verification.
//
// The document holds live session credentials. Treat an exported file exactly
// like a password.
type SessionBundle struct {
	Kind       string                `json:"kind"`
	Version    int                   `json:"version"`
	ExportedAt time.Time             `json:"exportedAt"`
	AppleID    string                `json:"appleId"`
	ExpiresAt  *time.Time            `json:"expiresAt,omitempty"`
	Cookies    []SessionBundleCookie `json:"cookies"`
}

// SessionBundleCookie is one cookie in an exported session bundle. URL is the
// canonical Apple origin the cookie belongs to.
type SessionBundleCookie struct {
	URL      string     `json:"url"`
	Name     string     `json:"name"`
	Value    string     `json:"value"`
	Path     string     `json:"path,omitempty"`
	Domain   string     `json:"domain,omitempty"`
	Expires  *time.Time `json:"expires,omitempty"`
	MaxAge   int        `json:"maxAge,omitempty"`
	Secure   bool       `json:"secure,omitempty"`
	HTTPOnly bool       `json:"httpOnly,omitempty"`
	SameSite int        `json:"sameSite,omitempty"`
}

// SessionImportSummary reports what an import stored in the session cache.
type SessionImportSummary struct {
	AppleID        string
	CookieCount    int
	SkippedExpired int
	ExpiresAt      *time.Time
}

// SupportedSessionBundleOrigins lists the Apple origins a bundle may carry
// cookies for. It matches the origins the session cache itself persists.
func SupportedSessionBundleOrigins() []string {
	urls := sessionCookieURLs()
	origins := make([]string, 0, len(urls))
	for _, u := range urls {
		origins = append(origins, u.String())
	}
	return origins
}

// canonicalSessionCookieURL maps an origin to the exact cache key used for it,
// so imports cannot inject cookies for hosts the session cache never serves.
func canonicalSessionCookieURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	for _, candidate := range sessionCookieURLs() {
		if strings.EqualFold(parsed.Scheme, candidate.Scheme) && strings.EqualFold(parsed.Host, candidate.Host) {
			return candidate.String(), true
		}
	}
	return "", false
}

// cookieStorableForOrigin reports whether the session cookie jar will keep
// this cookie for the canonical Apple origin. A matching URL is not enough:
// cookiejar.SetCookies drops Domain values that do not belong to that host.
func cookieStorableForOrigin(origin string, cookie SessionBundleCookie) bool {
	if strings.TrimSpace(cookie.Name) == "" {
		return false
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u == nil {
		return false
	}
	path := strings.TrimSpace(cookie.Path)
	if path == "" {
		path = "/"
	}
	u.Path = path
	jar.SetCookies(u, []*http.Cookie{{
		Name:    cookie.Name,
		Value:   "1",
		Path:    cookie.Path,
		Domain:  cookie.Domain,
		Secure:  cookie.Secure,
		Expires: time.Now().Add(time.Hour),
	}})
	for _, got := range jar.Cookies(u) {
		if got.Name == cookie.Name && got.Value == "1" {
			return true
		}
	}
	return false
}

// cookieDomainMatchesOrigin requires an empty Domain (host-only) or a Domain
// equal to the origin host. Broader values such as apple.com or the public
// suffix com would otherwise be accepted by cookiejar.New(nil) and sent to
// other hosts the web client later contacts.
func cookieDomainMatchesOrigin(origin, domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed == nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), ".")
	domain = strings.TrimPrefix(strings.ToLower(domain), ".")
	return domain != "" && domain == host
}

// sessionBundleCookieIdentity matches the identity used by net/http/cookiejar
// when it stores a cookie. Empty domains and paths receive the same defaults
// as the jar, and domain spelling is case-insensitive.
func sessionBundleCookieIdentity(origin string, cookie SessionBundleCookie) string {
	parsed, _ := url.Parse(origin)
	domain := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(cookie.Domain), "."))
	if domain == "" && parsed != nil {
		domain = strings.ToLower(parsed.Hostname())
	}
	path := strings.TrimSpace(cookie.Path)
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/"
	}
	return strings.Join([]string{origin, cookie.Name, domain, path}, "\x00")
}

func cookieExpiry(expires *time.Time) time.Time {
	if expires == nil {
		return time.Time{}
	}
	return *expires
}

func earliestBundleExpiry(cookies []SessionBundleCookie) *time.Time {
	var earliest time.Time
	for _, cookie := range cookies {
		expires := cookieExpiry(cookie.Expires)
		if expires.IsZero() {
			continue
		}
		if earliest.IsZero() || expires.Before(earliest) {
			earliest = expires
		}
	}
	if earliest.IsZero() {
		return nil
	}
	utc := earliest.UTC()
	return &utc
}

// ExportSessionBundle reads a cached web session and returns it as a portable
// bundle. An empty username exports the last cached session. It reports
// ok=false when no session is cached.
func ExportSessionBundle(username string) (*SessionBundle, bool, error) {
	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, ErrSessionCacheDisabled
	}

	username = strings.TrimSpace(username)
	var (
		sess persistedSession
		ok   bool
		err  error
	)
	if username == "" {
		sess, ok, err = readLastSessionBySelection(selection)
	} else {
		sess, ok, err = readSessionBySelection(selection, webSessionCacheKey(username))
	}
	if err != nil || !ok {
		return nil, false, err
	}

	appleID := strings.TrimSpace(sess.UserEmail)
	if appleID == "" {
		appleID = username
	}
	if appleID == "" {
		return nil, false, errors.New("cached web session does not record an Apple Account email; run \"asc web auth login\" again")
	}

	now := time.Now().UTC()
	bundle := &SessionBundle{
		Kind:       SessionBundleKind,
		Version:    SessionBundleVersion,
		ExportedAt: now.Truncate(time.Second),
		AppleID:    appleID,
		Cookies:    exportBundleCookies(sess, now),
	}
	if len(bundle.Cookies) == 0 {
		return nil, false, ErrSessionBundleUnusable
	}
	if err := bundle.Validate(); err != nil {
		return nil, false, fmt.Errorf("exported web session bundle is invalid: %w", err)
	}
	bundle.ExpiresAt = earliestBundleExpiry(bundle.Cookies)
	return bundle, true, nil
}

// exportBundleCookies flattens the cached cookie map into a deterministically
// ordered list. Cached sessions only ever hold the canonical Apple origins, so
// any other origin is left out rather than exported into a document the
// importer would reject.
func exportBundleCookies(sess persistedSession, now time.Time) []SessionBundleCookie {
	cookies := make([]SessionBundleCookie, 0, len(sess.Cookies))
	for origin, list := range sess.Cookies {
		canonical, ok := canonicalSessionCookieURL(origin)
		if !ok {
			continue
		}
		for _, cookie := range list {
			var usable bool
			cookie, usable = normalizePersistedCookieDeadline(cookie, sess.UpdatedAt)
			if !usable {
				continue
			}
			base, err := url.Parse(canonical)
			if err != nil || !cookieDomainStorableForOrigin(base, cookie) {
				continue
			}
			cookie = narrowCookieDomainForOrigin(base, cookie)
			if strings.TrimSpace(cookie.Name) == "" || isExpiredCookie(cookie, now) {
				continue
			}
			exported := SessionBundleCookie{
				URL:      canonical,
				Name:     cookie.Name,
				Value:    cookie.Value,
				Path:     cookie.Path,
				Domain:   cookie.Domain,
				Secure:   cookie.Secure,
				HTTPOnly: cookie.HttpOnly,
				SameSite: cookie.SameSite,
			}
			expires, maxAge := absoluteCookieDeadline(cookie.Expires, cookie.MaxAge, now)
			exported.MaxAge = maxAge
			if !expires.IsZero() {
				exported.Expires = &expires
			}
			cookies = append(cookies, exported)
		}
	}
	sort.Slice(cookies, func(i, j int) bool {
		if cookies[i].URL != cookies[j].URL {
			return cookies[i].URL < cookies[j].URL
		}
		if cookies[i].Name != cookies[j].Name {
			return cookies[i].Name < cookies[j].Name
		}
		if cookies[i].Path != cookies[j].Path {
			return cookies[i].Path < cookies[j].Path
		}
		if normalizedCookieDomain(cookies[i].Domain) != normalizedCookieDomain(cookies[j].Domain) {
			return normalizedCookieDomain(cookies[i].Domain) < normalizedCookieDomain(cookies[j].Domain)
		}
		if cookies[i].Value != cookies[j].Value {
			return cookies[i].Value < cookies[j].Value
		}
		return cookieExpiry(cookies[i].Expires).Before(cookieExpiry(cookies[j].Expires))
	})
	return cookies
}

// DecodeSessionBundle parses and validates a bundle document.
func DecodeSessionBundle(data []byte) (*SessionBundle, error) {
	if len(data) == 0 {
		return nil, errors.New("web session bundle is empty")
	}
	var bundle SessionBundle
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return nil, fmt.Errorf("decode web session bundle: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, errors.New("decode web session bundle: multiple JSON values")
		}
		return nil, fmt.Errorf("decode web session bundle: trailing data: %w", err)
	}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	return &bundle, nil
}

// Validate checks the document shape without inspecting cookie expiry.
func (b *SessionBundle) Validate() error {
	if b == nil {
		return errors.New("web session bundle is empty")
	}
	if strings.TrimSpace(b.Kind) != SessionBundleKind {
		return fmt.Errorf("web session bundle kind %q is not %q", b.Kind, SessionBundleKind)
	}
	if b.Version != SessionBundleVersion {
		return fmt.Errorf("web session bundle version %d is not supported (expected %d)", b.Version, SessionBundleVersion)
	}
	if strings.TrimSpace(b.AppleID) == "" {
		return errors.New("web session bundle is missing appleId")
	}
	if len(b.Cookies) == 0 {
		return errors.New("web session bundle contains no cookies")
	}
	seen := make(map[string]struct{}, len(b.Cookies))
	for index, cookie := range b.Cookies {
		if strings.TrimSpace(cookie.Name) == "" {
			return fmt.Errorf("web session bundle cookie %d is missing name", index)
		}
		canonical, ok := canonicalSessionCookieURL(cookie.URL)
		if !ok {
			return fmt.Errorf(
				"web session bundle cookie %q has unsupported url %q (supported origins: %s)",
				cookie.Name,
				cookie.URL,
				strings.Join(SupportedSessionBundleOrigins(), ", "),
			)
		}
		if !cookieDomainMatchesOrigin(canonical, cookie.Domain) {
			return fmt.Errorf(
				"%w: cookie %q has domain %q that is not host-only for %q",
				ErrSessionCookieNotStorable,
				cookie.Name,
				cookie.Domain,
				canonical,
			)
		}
		if !cookieStorableForOrigin(canonical, cookie) {
			return fmt.Errorf(
				"%w: cookie %q has domain %q that the session jar cannot store for %q",
				ErrSessionCookieNotStorable,
				cookie.Name,
				cookie.Domain,
				canonical,
			)
		}
		if err := sessionBundleCookieSyntaxValid(cookie); err != nil {
			return err
		}
		identity := sessionBundleCookieIdentity(canonical, cookie)
		if _, exists := seen[identity]; exists {
			return fmt.Errorf("%w: cookie %q at %q", ErrSessionCookieDuplicate, cookie.Name, canonical)
		}
		seen[identity] = struct{}{}
	}
	return nil
}

func sessionBundleCookieSyntaxValid(cookie SessionBundleCookie) error {
	parsed := http.Cookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Path:     cookie.Path,
		Domain:   cookie.Domain,
		Secure:   cookie.Secure,
		HttpOnly: cookie.HTTPOnly,
		SameSite: http.SameSite(cookie.SameSite),
	}
	if cookie.Expires != nil {
		parsed.Expires = *cookie.Expires
	}
	if err := parsed.Valid(); err != nil {
		return fmt.Errorf("%w: cookie %q has an invalid name, value, path, or domain", ErrSessionCookieInvalid, cookie.Name)
	}
	return nil
}

// absoluteCookieDeadline converts a positive MaxAge into an absolute expiry
// so later cache loads cannot resurrect the cookie by applying the same
// relative lifetime again.
func absoluteCookieDeadline(expires time.Time, maxAge int, now time.Time) (time.Time, int) {
	if maxAge > 0 {
		return now.Add(time.Duration(maxAge) * time.Second).UTC(), 0
	}
	if expires.IsZero() {
		return time.Time{}, maxAge
	}
	return expires.UTC(), maxAge
}

// importedCookieDeadline converts positive MaxAge to an absolute expiry and
// treats an explicit zero timestamp as already expired so it cannot be stored
// as a session cookie.
func importedCookieDeadline(expires *time.Time, maxAge int, now time.Time) (time.Time, int, bool) {
	if maxAge > 0 {
		return now.Add(time.Duration(maxAge) * time.Second).UTC(), 0, false
	}
	if maxAge < 0 {
		return time.Time{}, maxAge, true
	}
	if expires == nil {
		return time.Time{}, 0, false
	}
	utc := expires.UTC()
	if utc.IsZero() || utc.Before(now) {
		return utc, 0, true
	}
	return utc, 0, false
}

// normalize converts a validated bundle into the cache representation, leaving
// out cookies that already expired.
func (b *SessionBundle) normalize(now time.Time) (persistedSession, SessionImportSummary, error) {
	if err := b.Validate(); err != nil {
		return persistedSession{}, SessionImportSummary{}, err
	}

	appleID := strings.TrimSpace(b.AppleID)
	out := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: now,
		UserEmail: appleID,
		Cookies:   map[string][]pCookie{},
	}
	summary := SessionImportSummary{AppleID: appleID}
	kept := make([]SessionBundleCookie, 0, len(b.Cookies))
	for _, cookie := range b.Cookies {
		canonical, ok := canonicalSessionCookieURL(cookie.URL)
		if !ok {
			continue
		}
		expires, maxAge, expired := importedCookieDeadline(cookie.Expires, cookie.MaxAge, now)
		persisted := pCookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Path:     cookie.Path,
			Domain:   cookie.Domain,
			Expires:  expires,
			MaxAge:   maxAge,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HTTPOnly,
			SameSite: cookie.SameSite,
		}
		if expired || isExpiredCookie(persisted, now) {
			summary.SkippedExpired++
			continue
		}
		normalizedCookie := cookie
		normalizedCookie.MaxAge = maxAge
		if !expires.IsZero() {
			expiresCopy := expires
			normalizedCookie.Expires = &expiresCopy
		} else {
			normalizedCookie.Expires = nil
		}
		out.Cookies[canonical] = append(out.Cookies[canonical], persisted)
		kept = append(kept, normalizedCookie)
	}
	if len(kept) == 0 {
		return persistedSession{}, SessionImportSummary{}, ErrSessionBundleUnusable
	}
	summary.CookieCount = len(kept)
	summary.ExpiresAt = earliestBundleExpiry(kept)
	return out, summary, nil
}

// ValidateSessionBundle checks an imported bundle against Apple's live web
// session endpoint without reading or writing the local session cache. The
// caller can persist the bundle afterwards with ImportSessionBundleWithOptions
// when this explicit preflight succeeds.
func ValidateSessionBundle(ctx context.Context, bundle *SessionBundle) error {
	_, err := OpenValidatedSessionBundle(ctx, bundle)
	return err
}

// OpenValidatedSessionBundle validates a bundle with Apple and returns its
// in-memory session. It never reads or writes the cache, logs in, or switches
// providers. The returned provider identity comes from Apple's session response.
func OpenValidatedSessionBundle(ctx context.Context, bundle *SessionBundle) (*AuthSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if bundle == nil {
		return nil, fmt.Errorf("%w: web session bundle is empty", ErrSessionBundleValidationFailed)
	}

	sess, _, err := bundle.normalize(time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSessionBundleValidationFailed, err)
	}

	client, info, ok, err := validatePersistedSessionReadOnly(ctx, sess)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSessionBundleValidationFailed, err)
	}
	if !ok || info == nil {
		return nil, fmt.Errorf("%w: %w", ErrSessionBundleValidationFailed, ErrSessionBundleUnusable)
	}

	expected := strings.TrimSpace(bundle.AppleID)
	actual := strings.TrimSpace(info.User.EmailAddress)
	if actual == "" {
		return nil, fmt.Errorf("%w: Apple session returned no Apple Account identity", ErrSessionBundleValidationFailed)
	}
	if !strings.EqualFold(expected, actual) {
		return nil, fmt.Errorf("%w: Apple session account does not match the bundle", ErrSessionBundleValidationFailed)
	}
	session := &AuthSession{Client: client}
	applySessionInfo(session, info)
	return session, nil
}

// ImportSessionBundle stores a bundle in the same cache `asc web auth login`
// writes, so later `asc web` commands resume it. Import performs local bundle
// validation only; use `asc web auth status` when live Apple validation is
// needed. The imported session also becomes the last cached session.
func ImportSessionBundle(bundle *SessionBundle) (SessionImportSummary, error) {
	return importSessionBundleLocally(bundle, false)
}

// ImportSessionBundleWithOptions imports a bundle and optionally permits
// replacing an existing cache entry. The overwrite bit is also used to scope
// recovery from a malformed keychain aggregate to the explicit replacement
// path; ordinary login and refresh writes must not erase other accounts. The
// import itself performs local validation only; it does not contact Apple.
func ImportSessionBundleWithOptions(bundle *SessionBundle, overwrite bool) (SessionImportSummary, error) {
	return importSessionBundleLocally(bundle, overwrite)
}

// ImportSessionBundleWithContext retains the context-aware API for callers
// compiled against the original transfer surface. Import is local-only, so
// ctx is intentionally ignored; callers that need live validation should run
// the status or resume workflow separately.
func ImportSessionBundleWithContext(_ context.Context, bundle *SessionBundle, overwrite bool) (SessionImportSummary, error) {
	return importSessionBundleLocally(bundle, overwrite)
}

func importSessionBundleLocally(bundle *SessionBundle, overwrite bool) (SessionImportSummary, error) {
	if bundle == nil {
		return SessionImportSummary{}, errors.New("web session bundle is empty")
	}

	sess, summary, err := bundle.normalize(time.Now().UTC())
	if err != nil {
		return SessionImportSummary{}, err
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return SessionImportSummary{}, ErrSessionCacheDisabled
	}
	if err := persistImportedSessionBySelection(selection, webSessionCacheKey(summary.AppleID), sess, overwrite); err != nil {
		return SessionImportSummary{}, err
	}
	return summary, nil
}
