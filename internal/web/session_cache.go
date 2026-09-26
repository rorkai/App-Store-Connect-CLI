package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	webSessionCacheEnabledEnv = "ASC_WEB_SESSION_CACHE"
	webSessionCacheDirEnv     = "ASC_WEB_SESSION_CACHE_DIR"
	webSessionBackendEnv      = "ASC_WEB_SESSION_CACHE_BACKEND"

	webSessionCacheVersion  = 1
	webSessionCacheMaxBytes = 16 << 20

	webSessionKeyringService = "asc-web-session"
	webSessionStoreItem      = "asc:web-session:store"
	webSessionKeyPrefix      = "asc:web-session:"
	webSessionLastKeyItem    = "asc:web-session:last"
)

var (
	ErrCachedSessionExpired          = errors.New("cached web session expired")
	ErrCachedSessionValidationFailed = errors.New("cached web session could not be validated")
	errMalformedSessionFile          = errors.New("web session cache is malformed")
	errUnsafeSessionCacheFile        = errors.New("web session cache file is unsafe")
	// errMalformedSessionStore identifies malformed aggregate keychain data.
	// It is separate from the file-cache sentinel so an explicit keychain
	// recovery cannot be triggered by an unrelated file-read error.
	errMalformedSessionStore = errors.New("web session store is malformed")
)

type sessionBackend int

const (
	sessionBackendOff sessionBackend = iota
	sessionBackendKeychain
	sessionBackendFile
)

// sessionEntryOrigin records which backend a cached entry was actually read
// from. A conditional delete needs it: the entry whose stamp matched is the
// only one proven stale, and the other backend may hold a newer session.
type sessionEntryOrigin int

const (
	sessionEntryOriginNone sessionEntryOrigin = iota
	sessionEntryOriginKeychain
	sessionEntryOriginFile
)

type backendSelection struct {
	backend          sessionBackend
	fallbackFile     bool
	fallbackKeychain bool
}

type persistedSession struct {
	Version         int                  `json:"version"`
	UpdatedAt       time.Time            `json:"updated_at"`
	Generation      string               `json:"generation,omitempty"`
	UserEmail       string               `json:"user_email,omitempty"`
	DeveloperTeamID string               `json:"developer_team_id,omitempty"`
	Cookies         map[string][]pCookie `json:"cookies"`
}

type persistedSessionStore struct {
	Version  int                         `json:"version"`
	LastKey  string                      `json:"last_key,omitempty"`
	Sessions map[string]persistedSession `json:"sessions,omitempty"`
}

type pCookie struct {
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Path        string    `json:"path,omitempty"`
	Domain      string    `json:"domain,omitempty"`
	ScopeDomain string    `json:"scope_domain,omitempty"`
	Expires     time.Time `json:"expires,omitempty"`
	MaxAge      int       `json:"max_age,omitempty"`
	Secure      bool      `json:"secure,omitempty"`
	HttpOnly    bool      `json:"http_only,omitempty"`
	SameSite    int       `json:"same_site,omitempty"`
}

type persistedLastSession struct {
	Version int    `json:"version"`
	Key     string `json:"key"`
}

var (
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		return keyring.Open(keyring.Config{
			ServiceName:                    webSessionKeyringService,
			KeychainTrustApplication:       true,
			KeychainSynchronizable:         false,
			KeychainAccessibleWhenUnlocked: true,
			AllowedBackends: []keyring.BackendType{
				keyring.KeychainBackend,
				keyring.WinCredBackend,
				keyring.SecretServiceBackend,
				keyring.KWalletBackend,
				keyring.KeyCtlBackend,
			},
		})
	}
	sessionFileWrite   = writeSessionFileNoFollow
	sessionInfoFetcher = getSessionInfo

	// sessionCompareDeleteBarrier runs between the stamp comparison and the
	// delete in DeleteSessionIfMatches. Tests set it to schedule a concurrent
	// persist inside that window; it is nil in production.
	sessionCompareDeleteBarrier func()
	sessionGenerationReader     = func(b []byte) (int, error) { return rand.Read(b) }
)

func webSessionCacheEnabled() bool {
	raw := strings.TrimSpace(os.Getenv(webSessionCacheEnabledEnv))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func resolveBackendSelection() backendSelection {
	if !webSessionCacheEnabled() {
		return backendSelection{backend: sessionBackendOff}
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(webSessionBackendEnv))) {
	case "off", "none", "disabled":
		return backendSelection{backend: sessionBackendOff}
	case "file":
		return backendSelection{backend: sessionBackendFile}
	case "keychain":
		// Allow explicit keychain mode to import sessions from the file cache
		// so users can switch back after running on the default file-backed mode.
		return backendSelection{backend: sessionBackendKeychain, fallbackFile: true}
	case "", "auto":
		// Default to file-backed web sessions so successful logins can be reused
		// without recurring per-binary keychain approval prompts.
		return backendSelection{backend: sessionBackendFile, fallbackKeychain: true}
	default:
		return backendSelection{backend: sessionBackendFile, fallbackKeychain: true}
	}
}

func webSessionCacheDir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(webSessionCacheDirEnv)); custom != "" {
		return custom, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".asc", "web"), nil
}

func webSessionCacheKey(username string) string {
	normalized := strings.ToLower(strings.TrimSpace(username))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func webSessionFilePath(key string) (string, error) {
	dir, err := webSessionCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session-"+key+".json"), nil
}

func webSessionLastFilePath() (string, error) {
	dir, err := webSessionCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last.json"), nil
}

func sessionCookieURLs() []*url.URL {
	return []*url.URL{
		{Scheme: "https", Host: "appstoreconnect.apple.com", Path: "/"},
		{Scheme: "https", Host: "developer.apple.com", Path: "/"},
		{Scheme: "https", Host: "idmsa.apple.com", Path: "/"},
		{Scheme: "https", Host: "gsa.apple.com", Path: "/"},
	}
}

// sessionCookieProbeURLs includes request paths that can carry cookies which
// are intentionally absent from the root-origin probes above. Keep the root
// origins as the persisted cache keys; the extra endpoint is only a safe
// observation point for path-scoped cookies.
func sessionCookieProbeURLs() []*url.URL {
	urls := sessionCookieURLs()
	endpoint, _ := url.Parse(olympusSessionURL)
	return append(urls, endpoint)
}

type trackedCookieKey struct {
	origin string
	name   string
	value  string
	path   string
	domain string
}

type trackedCookieUpdate struct {
	cookie                 pCookie
	deleted                bool
	sessionOnlyReplacement bool
}

// sessionCookieTrackingJar records cookie deadlines supplied by responses
// after a cached jar has been hydrated. net/http/cookiejar intentionally omits
// expiry metadata from Cookies, so persistence otherwise cannot distinguish an
// untouched cached cookie from a same-value renewal.
type sessionCookieTrackingJar struct {
	http.CookieJar
	mu      sync.Mutex
	updates map[trackedCookieKey]trackedCookieUpdate
	cached  *persistedSession
}

func newSessionCookieTrackingJar(jar http.CookieJar) *sessionCookieTrackingJar {
	return newSessionCookieTrackingJarWithCached(jar, nil)
}

func newSessionCookieTrackingJarWithCached(jar http.CookieJar, cached *persistedSession) *sessionCookieTrackingJar {
	return &sessionCookieTrackingJar{
		CookieJar: jar,
		updates:   make(map[trackedCookieKey]trackedCookieUpdate),
		cached:    cached,
	}
}

func (j *sessionCookieTrackingJar) Cookies(u *url.URL) []*http.Cookie {
	if j == nil || j.CookieJar == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.CookieJar.Cookies(u)
}

func (j *sessionCookieTrackingJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if j == nil || j.CookieJar == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()

	j.CookieJar.SetCookies(u, cookies)
	now := time.Now().UTC()
	for _, supplied := range cookies {
		if supplied == nil || supplied.Name == "" {
			continue
		}
		updated := pCookie{
			Name:     supplied.Name,
			Value:    supplied.Value,
			Path:     effectiveCookiePath(u, supplied),
			Domain:   supplied.Domain,
			Expires:  supplied.Expires,
			MaxAge:   supplied.MaxAge,
			Secure:   supplied.Secure,
			HttpOnly: supplied.HttpOnly,
			SameSite: int(supplied.SameSite),
		}
		deleting := isCookieDeletion(updated, now)
		for _, origin := range sessionCookieUpdateOrigins(u, supplied, deleting) {
			deadline, usable := normalizePersistedCookieDeadline(updated, now)
			if deleting {
				j.recordUpdate(trackedCookieKeyForCookie(origin, deadline), deadline, true)
				continue
			}
			if !usable || isExpiredCookie(deadline, now) {
				continue
			}
			j.recordUpdate(trackedCookieKeyForCookie(origin, deadline), deadline, false)
		}
	}
}

func sessionCookieUpdateOrigins(source *url.URL, supplied *http.Cookie, deleting bool) []string {
	if source == nil || supplied == nil || supplied.Name == "" {
		return nil
	}
	probe, err := cookiejar.New(nil)
	if err != nil {
		return nil
	}
	// Use a fresh jar for each response cookie. This applies the actual
	// source URL's host and path defaults and avoids attributing a path- or
	// host-only cookie to unrelated cached origins. A deletion Set-Cookie is
	// not returned by Cookies, so probe its scope with a temporary live value.
	probeCookie := *supplied
	probeValue := supplied.Value
	if deleting {
		probeValue = "__asc_cookie_deletion_probe__"
		probeCookie.Value = probeValue
		probeCookie.Expires = time.Time{}
		probeCookie.MaxAge = 0
	}
	probe.SetCookies(source, []*http.Cookie{&probeCookie})
	origins := make([]string, 0, len(sessionCookieProbeURLs()))
	for _, origin := range sessionCookieProbeURLs() {
		if cookieListContains(probe.Cookies(origin), supplied.Name, probeValue) {
			origins = append(origins, origin.String())
		}
	}
	return origins
}

func cookieListContains(cookies []*http.Cookie, name, value string) bool {
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name == name && cookie.Value == value {
			return true
		}
	}
	return false
}

func (j *sessionCookieTrackingJar) recordUpdate(key trackedCookieKey, cookie pCookie, deleted bool) {
	if j.updates == nil {
		j.updates = make(map[trackedCookieKey]trackedCookieUpdate)
	}
	update := trackedCookieUpdate{cookie: cookie, deleted: deleted}
	if isSessionOnlyCookie(cookie) && cachedCookieHasActiveScope(j.cached, key.origin, cookie, time.Now().UTC()) {
		update.sessionOnlyReplacement = true
	}
	j.updates[key] = update
}

func trackedCookieKeyForCookie(origin string, cookie pCookie) trackedCookieKey {
	key := trackedCookieKey{origin: origin, name: cookie.Name, value: cookie.Value}
	if persistedCookiePath(cookie.Path) != "/" {
		key.path = persistedCookiePath(cookie.Path)
	}
	if parsed, err := url.Parse(origin); err == nil && parsed != nil {
		domain := normalizedCookieDomain(cookie.Domain)
		// Keep an explicit host Domain distinct from a host-only cookie. Both
		// are sent to this origin, but RFC 6265 gives them different storage
		// scopes and a parent-domain tombstone must not delete either one.
		if domain != "" {
			key.domain = domain
		}
	}
	return key
}

func sameTrackedCookieUpdate(a, b trackedCookieUpdate) bool {
	return a.cookie.Name == b.cookie.Name &&
		a.cookie.Value == b.cookie.Value &&
		a.deleted == b.deleted &&
		persistedCookiePath(a.cookie.Path) == persistedCookiePath(b.cookie.Path) &&
		normalizedCookieDomain(a.cookie.Domain) == normalizedCookieDomain(b.cookie.Domain) &&
		a.cookie.Expires.Equal(b.cookie.Expires) &&
		a.cookie.MaxAge == b.cookie.MaxAge &&
		a.cookie.Secure == b.cookie.Secure &&
		a.cookie.HttpOnly == b.cookie.HttpOnly &&
		a.cookie.SameSite == b.cookie.SameSite &&
		a.sessionOnlyReplacement == b.sessionOnlyReplacement
}

// trackedCookieUpdateForPersistedCookie also considers a path-scoped update
// observed at the Olympus endpoint. The persisted cache keeps the app root as
// its origin key, while the response was observed at a deeper request path.
func trackedCookieUpdateForPersistedCookie(updates map[trackedCookieKey]trackedCookieUpdate, origin string, cookie pCookie) (trackedCookieUpdate, bool) {
	var (
		matched trackedCookieUpdate
		found   bool
	)
	for key, update := range updates {
		if key.name != cookie.Name || key.value != cookie.Value {
			continue
		}
		if key.origin != origin && (key.origin != olympusSessionURL || !isAppStoreConnectRootOrigin(origin)) {
			continue
		}
		if !cookieScopesMatchForOrigin(origin, update.cookie, cookie) {
			continue
		}
		if found && !sameTrackedCookieUpdate(matched, update) {
			return trackedCookieUpdate{}, false
		}
		matched = update
		found = true
	}
	return matched, found
}

func trackedCookieUpdatesForOrigin(updates map[trackedCookieKey]trackedCookieUpdate, origin, name, value string) []trackedCookieUpdate {
	matched := make([]trackedCookieUpdate, 0, len(updates))
	for key, update := range updates {
		if key.origin == origin && key.name == name && key.value == value {
			matched = append(matched, update)
		}
	}
	return matched
}

// trackedCookieScopeUpdatedForOrigin reports any response update for a cookie
// scope, regardless of value. A root-origin probe can still expose the old
// value after a deeper cookie was deleted or rotated; that observation must
// not resurrect the cached cookie from the deeper scope.
func trackedCookieScopeUpdatedForOrigin(updates map[trackedCookieKey]trackedCookieUpdate, origin string, cookie pCookie) bool {
	for key, update := range updates {
		if key.name != cookie.Name {
			continue
		}
		if key.origin != origin && (key.origin != olympusSessionURL || !isAppStoreConnectRootOrigin(origin)) {
			continue
		}
		if cookieScopesMatchForOrigin(origin, update.cookie, cookie) {
			return true
		}
	}
	return false
}

func isSessionOnlyCookie(c pCookie) bool {
	return c.MaxAge == 0 && c.Expires.IsZero()
}

func isCookieDeletion(c pCookie, now time.Time) bool {
	if c.MaxAge != 0 {
		return c.MaxAge < 0
	}
	return !c.Expires.IsZero() && !c.Expires.After(now)
}

// markPersisted clears only the update snapshot that was actually written.
// A response racing with the backend write remains in updates for the next
// persistence attempt instead of being lost.
func (j *sessionCookieTrackingJar) markPersisted(updates map[trackedCookieKey]trackedCookieUpdate, persisted *persistedSession) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for key, applied := range updates {
		current, ok := j.updates[key]
		if ok && sameTrackedCookieUpdate(current, applied) &&
			(!isSessionOnlyCookie(current.cookie) || persistedSessionContainsCookieScope(persisted, key.origin, current.cookie)) {
			delete(j.updates, key)
		}
	}
}

func persistedSessionContainsCookieScope(persisted *persistedSession, origin string, cookie pCookie) bool {
	if persisted == nil {
		return false
	}
	base, err := url.Parse(origin)
	if err != nil || base == nil {
		return false
	}
	cookie = narrowCookieDomainForOrigin(base, cookie)
	for cachedOrigin, list := range persisted.Cookies {
		cachedBase, err := url.Parse(cachedOrigin)
		if err != nil || cachedBase == nil ||
			!strings.EqualFold(cachedBase.Scheme, base.Scheme) ||
			!strings.EqualFold(cachedBase.Host, base.Host) {
			continue
		}
		for _, candidate := range list {
			if candidate.Name == cookie.Name && candidate.Value == cookie.Value &&
				cookieScopesMatchForOrigin(origin, candidate, cookie) {
				return true
			}
		}
	}
	return false
}

func (j *sessionCookieTrackingJar) serializeWithUpdates(userEmail string) (persistedSession, map[trackedCookieKey]trackedCookieUpdate, error) {
	if j == nil || j.CookieJar == nil {
		return persistedSession{}, nil, errors.New("web session cookie jar is unavailable")
	}
	j.mu.Lock()
	defer j.mu.Unlock()

	serialized, err := serializeCookieJarWithError(j.CookieJar, userEmail)
	if err != nil {
		return persistedSession{}, nil, err
	}
	updates := make(map[trackedCookieKey]trackedCookieUpdate, len(j.updates))
	for key, update := range j.updates {
		updates[key] = update
	}
	preserveTrackedCookieScopes(&serialized, j.CookieJar, j.cached, updates)
	return serialized, updates, nil
}

// preserveTrackedCookieScopes adds cookies visible only at a known request
// path to the root-origin cache record. It never invents scope metadata: a
// path cookie must come from a tracked Set-Cookie response or an existing
// persisted record, otherwise it is dropped rather than broadened.
func preserveTrackedCookieScopes(serialized *persistedSession, jar http.CookieJar, cached *persistedSession, updates map[trackedCookieKey]trackedCookieUpdate) {
	if serialized == nil || jar == nil {
		return
	}
	rootOrigin := sessionCookieURLs()[0].String()
	rootURL := sessionCookieURLs()[0]
	rootCookies := serialized.Cookies[rootOrigin]
	rootCookies = patchTrackedCookieScopes(rootOrigin, rootURL, rootCookies, cached, updates)
	serialized.Cookies[rootOrigin] = rootCookies
	now := time.Now().UTC()
	addCookie := func(cookie pCookie) {
		if isExpiredCookie(cookie, now) {
			return
		}
		if !cookieDomainStorableForOrigin(rootURL, cookie) {
			return
		}
		// A root-origin entry with the same name and value but unknown scope
		// is ambiguous with this path-scoped cookie. Fail closed instead of
		// persisting a duplicate that could be hydrated too broadly.
		if persistedCookieListHasUnknownIdentity(rootCookies, cookie) {
			return
		}
		rootCookies = append(rootCookies, cookie)
	}

	for _, probe := range sessionCookieProbeURLs()[len(sessionCookieURLs()):] {
		observedCookies := jar.Cookies(probe)
		for _, observed := range observedCookies {
			if observed == nil || observed.Name == "" {
				continue
			}
			trackedUpdates := trackedCookieUpdatesForOrigin(updates, probe.String(), observed.Name, observed.Value)
			cachedCookies := cachedCookieScopesForProbe(cached, probe, observed.Name, observed.Value)
			for _, update := range trackedUpdates {
				if persistedCookiePath(update.cookie.Path) == "/" {
					continue
				}
				if update.deleted {
					continue
				}
				if isSessionOnlyCookie(update.cookie) && (update.sessionOnlyReplacement || cachedCookieHasActiveScope(cached, probe.String(), update.cookie, now)) {
					continue
				}
				addCookie(update.cookie)
			}
			for _, cachedCookie := range cachedCookies {
				if persistedCookiePath(cachedCookie.Path) == "/" {
					continue
				}
				updated := trackedCookieScopeUpdatedForOrigin(updates, probe.String(), cachedCookie)
				for _, update := range trackedUpdates {
					if cookieScopesMatchForOrigin(probe.String(), update.cookie, cachedCookie) {
						updated = true
						break
					}
				}
				if !updated {
					addCookie(cachedCookie)
				}
			}
			if len(trackedUpdates) > 0 || len(cachedCookies) > 0 {
				continue
			}
			// An untracked cookie with duplicate visibility has no trustworthy
			// path/domain metadata and must not be persisted.
			if observedCookieNameCount(observedCookies, observed.Name) > 1 {
				continue
			}
		}
	}
	if rootCookies = dropAmbiguousPersistedCookies(rootCookies); len(rootCookies) > 0 {
		serialized.Cookies[rootOrigin] = rootCookies
	} else {
		delete(serialized.Cookies, rootOrigin)
	}
}

func observedCookieNameCount(cookies []*http.Cookie, name string) int {
	count := 0
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name == name {
			count++
		}
	}
	return count
}

func patchTrackedCookieScopes(origin string, probe *url.URL, cookies []pCookie, cached *persistedSession, updates map[trackedCookieKey]trackedCookieUpdate) []pCookie {
	patched := make([]pCookie, 0, len(cookies))
	now := time.Now().UTC()
	for _, cookie := range cookies {
		if update, ok := trackedCookieUpdateForPersistedCookie(updates, origin, cookie); ok {
			if isSessionOnlyCookie(update.cookie) && (update.sessionOnlyReplacement || cachedCookieHasActiveScope(cached, origin, update.cookie, now)) {
				continue
			}
			cookie = update.cookie
		} else if scope, ok := cachedCookieForProbe(cached, probe, cookie.Name, cookie.Value); ok {
			cookie = scope
		}
		if !isExpiredCookie(cookie, now) && cookieDomainStorableForOrigin(probe, cookie) {
			patched = append(patched, cookie)
		}
	}
	return patched
}

func cookieScopesMatchForOrigin(origin string, a, b pCookie) bool {
	if persistedCookiePath(a.Path) != persistedCookiePath(b.Path) {
		return false
	}
	base, err := url.Parse(origin)
	if err != nil || base == nil {
		return persistedCookieScopeDomain(nil, a) == persistedCookieScopeDomain(nil, b)
	}
	return persistedCookieScopeDomain(base, a) == persistedCookieScopeDomain(base, b)
}

func persistedCookieScopeDomain(origin *url.URL, cookie pCookie) string {
	domain := normalizedCookieDomain(cookie.Domain)
	if domain != "" {
		return domain
	}
	scopeDomain := normalizedCookieDomain(cookie.ScopeDomain)
	if scopeDomain == "" || origin == nil {
		return scopeDomain
	}
	if cookieDomainMatchesHost(scopeDomain, origin.Hostname()) {
		return scopeDomain
	}
	return ""
}

func cachedCookieHasActiveScope(cached *persistedSession, origin string, cookie pCookie, now time.Time) bool {
	if cached == nil {
		return false
	}
	base, err := url.Parse(origin)
	if err != nil || base == nil {
		return false
	}
	for cachedOrigin, list := range cached.Cookies {
		cachedBase, err := url.Parse(cachedOrigin)
		if err != nil || cachedBase == nil ||
			!strings.EqualFold(cachedBase.Scheme, base.Scheme) ||
			!strings.EqualFold(cachedBase.Host, base.Host) {
			continue
		}
		for _, candidate := range list {
			candidate, usable := normalizePersistedCookieDeadline(candidate, cached.UpdatedAt)
			if !usable || isExpiredCookie(candidate, now) ||
				candidate.Name != cookie.Name ||
				!cookieScopesMatchForOrigin(origin, candidate, cookie) {
				continue
			}
			return true
		}
	}
	return false
}

func cachedCookieForProbe(cached *persistedSession, probe *url.URL, name, value string) (pCookie, bool) {
	matches := cachedCookieScopesForProbe(cached, probe, name, value)
	if len(matches) != 1 {
		return pCookie{}, false
	}
	return matches[0], true
}

func cachedCookieScopesForProbe(cached *persistedSession, probe *url.URL, name, value string) []pCookie {
	if cached == nil || probe == nil {
		return nil
	}
	matches := make([]pCookie, 0)
	ambiguous := make([]bool, 0)
	for origin, list := range cached.Cookies {
		base, err := url.Parse(origin)
		if err != nil || base == nil || !strings.EqualFold(base.Scheme, probe.Scheme) || !strings.EqualFold(base.Host, probe.Host) {
			continue
		}
		for _, candidate := range list {
			if candidate.Name != name || candidate.Value != value ||
				!persistedCookiePathMatches(candidate.Path, probe.Path) ||
				!cookieDomainMatchesHost(cookieDomainForOrigin(base, candidate.Domain), probe.Hostname()) {
				continue
			}
			candidate, usable := normalizePersistedCookieDeadline(candidate, cached.UpdatedAt)
			if !usable || isExpiredCookie(candidate, time.Now().UTC()) {
				continue
			}
			duplicate := false
			for index := range matches {
				if !samePersistedCookieStorageScope(base, matches[index], candidate) {
					continue
				}
				duplicate = true
				if !cookieScopesMatchForOrigin(probe.String(), matches[index], candidate) ||
					!matches[index].Expires.Equal(candidate.Expires) || matches[index].MaxAge != candidate.MaxAge {
					ambiguous[index] = true
				}
				break
			}
			if !duplicate {
				matches = append(matches, candidate)
				ambiguous = append(ambiguous, false)
			}
		}
	}
	filtered := matches[:0]
	for index, candidate := range matches {
		if !ambiguous[index] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func samePersistedCookieScope(a, b pCookie) bool {
	return a.Name == b.Name && a.Value == b.Value &&
		persistedCookiePath(a.Path) == persistedCookiePath(b.Path) &&
		normalizedCookieDomain(a.Domain) == normalizedCookieDomain(b.Domain) &&
		a.Secure == b.Secure && a.HttpOnly == b.HttpOnly && a.SameSite == b.SameSite
}

func samePersistedCookieStorageScope(origin *url.URL, a, b pCookie) bool {
	if origin != nil {
		a = narrowCookieDomainForOrigin(origin, a)
		b = narrowCookieDomainForOrigin(origin, b)
	}
	return samePersistedCookieScope(a, b)
}

func persistedCookieListHasScope(cookies []pCookie, cookie pCookie) bool {
	for _, existing := range cookies {
		if samePersistedCookieScope(existing, cookie) {
			return true
		}
	}
	return false
}

func persistedCookieListHasUnknownIdentity(cookies []pCookie, cookie pCookie) bool {
	for _, existing := range cookies {
		if existing.Name == cookie.Name && existing.Value == cookie.Value &&
			strings.TrimSpace(existing.Path) == "" && strings.TrimSpace(existing.Domain) == "" {
			return true
		}
	}
	return false
}

func isExpiredCookie(c pCookie, now time.Time) bool {
	if c.MaxAge < 0 {
		return true
	}
	if c.MaxAge > 0 {
		return false
	}
	return !c.Expires.IsZero() && !c.Expires.After(now)
}

func normalizePersistedCookieDeadline(c pCookie, observedAt time.Time) (pCookie, bool) {
	if c.MaxAge <= 0 {
		return c, true
	}
	if observedAt.IsZero() {
		return c, false
	}
	c.Expires, c.MaxAge = absoluteCookieDeadline(c.Expires, c.MaxAge, observedAt)
	return c, true
}

func persistedCookiePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '/' {
		return "/"
	}
	return path
}

// effectiveCookiePath applies the same default-path rule as net/http/cookiejar
// before a response cookie is recorded. The jar does not expose that inferred
// path through Cookies, but dropping it would turn a path-scoped cookie into a
// root cookie on the next cache hydration.
func effectiveCookiePath(source *url.URL, cookie *http.Cookie) string {
	if cookie != nil && strings.HasPrefix(cookie.Path, "/") {
		return cookie.Path
	}
	if source == nil || source.Path == "" || !strings.HasPrefix(source.Path, "/") {
		return "/"
	}
	path := source.Path
	if index := strings.LastIndex(path, "/"); index > 0 {
		return path[:index]
	}
	return "/"
}

func cookieDomainForOrigin(origin *url.URL, domain string) string {
	domain = normalizedCookieDomain(domain)
	if domain != "" {
		return domain
	}
	if origin == nil {
		return ""
	}
	return normalizedCookieDomain(origin.Hostname())
}

func normalizedCookieDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, ".")
	return strings.TrimSuffix(domain, ".")
}

func cookieDomainMatchesHost(domain, host string) bool {
	domain = normalizedCookieDomain(domain)
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	return domain != "" && (host == domain || strings.HasSuffix(host, "."+domain))
}

func isAppStoreConnectRootOrigin(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return false
	}
	root := sessionCookieURLs()[0]
	return strings.EqualFold(parsed.Scheme, root.Scheme) &&
		strings.EqualFold(parsed.Host, root.Host) &&
		persistedCookiePath(parsed.Path) == "/"
}

func narrowCookieDomainForOrigin(origin *url.URL, cookie pCookie) pCookie {
	if origin == nil {
		return cookie
	}
	if normalizedCookieDomain(cookie.Domain) == "" {
		if scopeDomain := normalizedCookieDomain(cookie.ScopeDomain); scopeDomain != "" {
			if cookieDomainMatchesHost(scopeDomain, origin.Hostname()) {
				cookie.ScopeDomain = scopeDomain
			} else {
				cookie.ScopeDomain = ""
			}
		}
		return cookie
	}
	if cookieDomainMatchesHost(cookie.Domain, origin.Hostname()) {
		// The old cache representation narrowed domain cookies to the host that
		// was being serialized. Preserve the response's RFC storage scope as
		// internal provenance so a later renewal or tombstone can still match it
		// without exporting or hydrating the broader Domain.
		cookie.ScopeDomain = normalizedCookieDomain(cookie.Domain)
		cookie.Domain = ""
	}
	return cookie
}

func cookieDomainStorableForOrigin(origin *url.URL, cookie pCookie) bool {
	if origin == nil {
		return false
	}
	domain := normalizedCookieDomain(cookie.Domain)
	return domain == "" || cookieDomainMatchesHost(domain, origin.Hostname())
}

func persistedCookiePathMatches(path, requestPath string) bool {
	path = persistedCookiePath(path)
	if requestPath == "" || requestPath[0] != '/' {
		requestPath = "/"
	}
	if path == "/" || requestPath == path {
		return true
	}
	return strings.HasPrefix(requestPath, path) && (strings.HasSuffix(path, "/") || requestPath[len(path)] == '/')
}

// dropAmbiguousPersistedCookies resolves duplicate storage identities. The
// cookie jar intentionally omits the domain and path scope from Cookies, so a
// tracked update cannot prove which of two aliases it renewed. Equivalent
// records can be collapsed only when their original RFC storage scope also
// matches; conflicting records are dropped fail-closed so a resume cannot
// extend or send the wrong credential.
func dropAmbiguousPersistedCookies(cookies []pCookie) []pCookie {
	type cookieGroup struct {
		first      pCookie
		consistent bool
	}
	groups := make(map[string]*cookieGroup, len(cookies))
	order := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		identity := strings.Join([]string{cookie.Name, normalizedCookieDomain(cookie.Domain), persistedCookiePath(cookie.Path)}, "\x00")
		group, ok := groups[identity]
		if !ok {
			groups[identity] = &cookieGroup{first: cookie, consistent: true}
			order = append(order, identity)
			continue
		}
		if !samePersistedCookieRecord(group.first, cookie) {
			group.consistent = false
		}
	}

	// A narrowed parent-domain cookie can alias a host-only record in the
	// persisted representation. A disagreement in original scope, value,
	// flags, or deadline is unsafe to resolve by input order, so drop the whole
	// identity and require a fresh login.
	filtered := make([]pCookie, 0, len(groups))
	for _, identity := range order {
		group := groups[identity]
		if group.consistent {
			filtered = append(filtered, group.first)
		}
	}
	return filtered
}

func samePersistedCookieRecord(a, b pCookie) bool {
	return samePersistedCookieScope(a, b) &&
		persistedCookieScopeDomain(nil, a) == persistedCookieScopeDomain(nil, b) &&
		a.Expires.Equal(b.Expires) &&
		a.MaxAge == b.MaxAge
}

func preserveCachedCookieDeadlines(current *persistedSession, cached *persistedSession, updates map[trackedCookieKey]trackedCookieUpdate, now time.Time) {
	if current == nil {
		return
	}
	for origin, cookies := range current.Cookies {
		persistable := make([]pCookie, 0, len(cookies))
		for i := range cookies {
			if refreshed, ok := trackedCookieUpdateForPersistedCookie(updates, origin, cookies[i]); ok {
				if isSessionOnlyCookie(refreshed.cookie) && (refreshed.sessionOnlyReplacement || cachedCookieHasActiveScope(cached, origin, refreshed.cookie, now)) {
					continue
				}
				cookies[i].Expires = refreshed.cookie.Expires
				cookies[i].MaxAge = refreshed.cookie.MaxAge
				persistable = append(persistable, cookies[i])
				continue
			}
			if cached == nil {
				persistable = append(persistable, cookies[i])
				continue
			}
			previous := cached.Cookies[origin]
			var matched *pCookie
			ambiguous := false
			for _, candidate := range previous {
				candidate, usable := normalizePersistedCookieDeadline(candidate, cached.UpdatedAt)
				if !usable ||
					isExpiredCookie(candidate, now) ||
					candidate.Name != cookies[i].Name ||
					candidate.Value != cookies[i].Value ||
					!cookieScopesMatchForOrigin(origin, candidate, cookies[i]) {
					continue
				}
				if matched != nil && (!matched.Expires.Equal(candidate.Expires) || matched.MaxAge != candidate.MaxAge) {
					ambiguous = true
					break
				}
				copy := candidate
				matched = &copy
			}
			if matched != nil && !ambiguous && !matched.Expires.IsZero() {
				cookies[i].Expires = matched.Expires
				cookies[i].MaxAge = 0
			}
			persistable = append(persistable, cookies[i])
		}
		if len(persistable) == 0 {
			delete(current.Cookies, origin)
			continue
		}
		current.Cookies[origin] = persistable
	}
}

func narrowPersistedCookieDomains(current *persistedSession) {
	if current == nil {
		return
	}
	for origin, cookies := range current.Cookies {
		base, err := url.Parse(origin)
		if err != nil || base == nil {
			delete(current.Cookies, origin)
			continue
		}
		narrowed := make([]pCookie, 0, len(cookies))
		for _, cookie := range cookies {
			if !cookieDomainStorableForOrigin(base, cookie) {
				continue
			}
			narrowed = append(narrowed, narrowCookieDomainForOrigin(base, cookie))
		}
		if narrowed = dropAmbiguousPersistedCookies(narrowed); len(narrowed) > 0 {
			current.Cookies[origin] = narrowed
		} else {
			delete(current.Cookies, origin)
		}
	}
}

func serializeCookieJar(jar http.CookieJar, userEmail string) persistedSession {
	sess, _ := serializeCookieJarWithError(jar, userEmail)
	return sess
}

func serializeCookieJarWithError(jar http.CookieJar, userEmail string) (persistedSession, error) {
	now := time.Now().UTC()
	var generation [16]byte
	if _, err := sessionGenerationReader(generation[:]); err != nil {
		return persistedSession{}, fmt.Errorf("generate session cache identity: %w", err)
	}
	out := persistedSession{
		Version:    webSessionCacheVersion,
		UpdatedAt:  now,
		Generation: fmt.Sprintf("%x", generation[:]),
		UserEmail:  strings.TrimSpace(userEmail),
		Cookies:    map[string][]pCookie{},
	}
	for _, u := range sessionCookieURLs() {
		cookies := jar.Cookies(u)
		if len(cookies) == 0 {
			continue
		}
		list := make([]pCookie, 0, len(cookies))
		for _, c := range cookies {
			if c == nil || c.Name == "" {
				continue
			}
			pc := pCookie{
				Name:     c.Name,
				Value:    c.Value,
				Path:     c.Path,
				Domain:   c.Domain,
				Expires:  c.Expires,
				MaxAge:   c.MaxAge,
				Secure:   c.Secure,
				HttpOnly: c.HttpOnly,
				SameSite: int(c.SameSite),
			}
			if !cookieDomainStorableForOrigin(u, pc) {
				continue
			}
			pc = narrowCookieDomainForOrigin(u, pc)
			pc, _ = normalizePersistedCookieDeadline(pc, now)
			if isExpiredCookie(pc, now) {
				continue
			}
			list = append(list, pc)
		}
		if list = dropAmbiguousPersistedCookies(list); len(list) > 0 {
			out.Cookies[u.String()] = list
		}
	}
	return out, nil
}

func hydrateCookieJar(jar http.CookieJar, sess persistedSession) int {
	now := time.Now().UTC()
	loaded := 0
	for base, list := range sess.Cookies {
		u, err := url.Parse(base)
		if err != nil {
			continue
		}
		persisted := make([]pCookie, 0, len(list))
		for _, pc := range list {
			var usable bool
			pc, usable = normalizePersistedCookieDeadline(pc, sess.UpdatedAt)
			if !usable {
				continue
			}
			if !cookieDomainStorableForOrigin(u, pc) {
				continue
			}
			pc = narrowCookieDomainForOrigin(u, pc)
			if pc.Name == "" || isExpiredCookie(pc, now) {
				continue
			}
			persisted = append(persisted, pc)
		}
		persisted = dropAmbiguousPersistedCookies(persisted)
		cookies := make([]*http.Cookie, 0, len(persisted))
		for _, pc := range persisted {
			cookies = append(cookies, &http.Cookie{
				Name:     pc.Name,
				Value:    pc.Value,
				Path:     pc.Path,
				Domain:   pc.Domain,
				Expires:  pc.Expires,
				MaxAge:   pc.MaxAge,
				Secure:   pc.Secure,
				HttpOnly: pc.HttpOnly,
				SameSite: http.SameSite(pc.SameSite),
			})
		}
		if len(cookies) > 0 {
			jar.SetCookies(u, cookies)
			loaded += len(cookies)
		}
	}
	return loaded
}

func keyringSessionItem(key string) string {
	return webSessionKeyPrefix + key
}

func isKeyringUnavailable(err error) bool {
	return errors.Is(err, keyring.ErrNoAvailImpl)
}

func newPersistedSessionStore() persistedSessionStore {
	return persistedSessionStore{
		Version:  webSessionCacheVersion,
		Sessions: map[string]persistedSession{},
	}
}

func normalizePersistedSessionStore(store persistedSessionStore) persistedSessionStore {
	if store.Version == 0 {
		store.Version = webSessionCacheVersion
	}
	if store.Sessions == nil {
		store.Sessions = map[string]persistedSession{}
	}
	return store
}

func resolvePersistedSessionStoreLastKey(store persistedSessionStore) (string, bool) {
	store = normalizePersistedSessionStore(store)
	if key := strings.TrimSpace(store.LastKey); key != "" {
		if _, ok := store.Sessions[key]; ok {
			return key, true
		}
	}
	if len(store.Sessions) == 0 {
		return "", false
	}
	keys := make([]string, 0, len(store.Sessions))
	for key := range store.Sessions {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return "", false
	}
	sort.Slice(keys, func(i, j int) bool {
		left := store.Sessions[keys[i]].UpdatedAt
		right := store.Sessions[keys[j]].UpdatedAt
		if left.Equal(right) {
			return keys[i] < keys[j]
		}
		return left.After(right)
	})
	return keys[0], true
}

func readLegacySessionFromKeyring(kr keyring.Keyring, key string) (persistedSession, bool, error) {
	item, err := kr.Get(keyringSessionItem(key))
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return persistedSession{}, false, nil
		}
		return persistedSession{}, false, err
	}
	var sess persistedSession
	if err := json.Unmarshal(item.Data, &sess); err != nil {
		return persistedSession{}, false, fmt.Errorf("failed to decode keychain session: %w", err)
	}
	if sess.Version != webSessionCacheVersion {
		return persistedSession{}, false, nil
	}
	return sess, true, nil
}

func readLegacyLastKeyFromKeyring(kr keyring.Keyring) (string, bool, error) {
	item, err := kr.Get(webSessionLastKeyItem)
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	var last persistedLastSession
	if err := json.Unmarshal(item.Data, &last); err != nil {
		return "", false, err
	}
	if last.Version != webSessionCacheVersion || strings.TrimSpace(last.Key) == "" {
		return "", false, nil
	}
	return strings.TrimSpace(last.Key), true, nil
}

func readLegacySessionStoreFromKeyring(kr keyring.Keyring) (persistedSessionStore, bool, error) {
	keys, err := kr.Keys()
	if err != nil {
		return persistedSessionStore{}, false, err
	}
	store := newPersistedSessionStore()
	for _, itemKey := range keys {
		if !strings.HasPrefix(itemKey, webSessionKeyPrefix) || itemKey == webSessionLastKeyItem || itemKey == webSessionStoreItem {
			continue
		}
		key := strings.TrimPrefix(itemKey, webSessionKeyPrefix)
		sess, ok, err := readLegacySessionFromKeyring(kr, key)
		if err != nil {
			return persistedSessionStore{}, false, err
		}
		if ok {
			store.Sessions[key] = sess
		}
	}
	if len(store.Sessions) == 0 {
		return persistedSessionStore{}, false, nil
	}
	if lastKey, ok, err := readLegacyLastKeyFromKeyring(kr); err != nil {
		return persistedSessionStore{}, false, err
	} else if ok {
		store.LastKey = lastKey
	}
	if resolved, ok := resolvePersistedSessionStoreLastKey(store); ok {
		store.LastKey = resolved
	}
	return store, true, nil
}

func readSessionStoreFromKeyring(kr keyring.Keyring) (persistedSessionStore, bool, error) {
	item, err := kr.Get(webSessionStoreItem)
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return readLegacySessionStoreFromKeyring(kr)
		}
		return persistedSessionStore{}, false, err
	}
	var store persistedSessionStore
	if err := json.Unmarshal(item.Data, &store); err != nil {
		return persistedSessionStore{}, false, fmt.Errorf("%w: failed to decode keychain session store: %w", errMalformedSessionStore, err)
	}
	if store.Version != webSessionCacheVersion {
		return persistedSessionStore{}, false, nil
	}
	store = normalizePersistedSessionStore(store)
	if resolved, ok := resolvePersistedSessionStoreLastKey(store); ok {
		store.LastKey = resolved
	}
	return store, true, nil
}

func writeSessionStoreToKeyring(kr keyring.Keyring, store persistedSessionStore) error {
	store = normalizePersistedSessionStore(store)
	raw, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("failed to marshal session store: %w", err)
	}
	return kr.Set(keyring.Item{
		Key:   webSessionStoreItem,
		Data:  raw,
		Label: "ASC Web Session Store",
	})
}

func removeSessionStoreFromKeyring(kr keyring.Keyring) error {
	err := kr.Remove(webSessionStoreItem)
	if err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return err
	}
	return nil
}

func removeLegacySessionFromKeyring(kr keyring.Keyring, key string) error {
	err := kr.Remove(keyringSessionItem(key))
	if err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return err
	}
	return nil
}

func removeLegacyLastKeyFromKeyring(kr keyring.Keyring) error {
	err := kr.Remove(webSessionLastKeyItem)
	if err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return err
	}
	return nil
}

func writeSessionToKeychain(key string, sess persistedSession) error {
	return withSessionStoreLock(func() error { return writeSessionToKeychainUnlocked(key, sess) })
}

func writeSessionToKeychainUnlocked(key string, sess persistedSession) error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
		return err
	}
	if !ok {
		store = newPersistedSessionStore()
	}
	store = normalizePersistedSessionStore(store)
	store.Sessions[key] = sess
	store.LastKey = key
	return writeSessionStoreToKeyring(kr, store)
}

func writeSessionToKeychainIfAbsentUnlocked(key string, sess persistedSession) error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
		// Never treat malformed or unreadable state as absent for a create-only
		// import. Doing so could destroy another account's credentials.
		return err
	}
	if ok {
		if _, exists := store.Sessions[key]; exists {
			return cachedSessionAlreadyExistsError(key)
		}
	} else {
		store = newPersistedSessionStore()
	}
	store = normalizePersistedSessionStore(store)
	store.Sessions[key] = sess
	store.LastKey = key
	return writeSessionStoreToKeyring(kr, store)
}

func writeSessionToKeychainWithRecoveryUnlocked(key string, sess persistedSession, recoverMalformed bool) error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
		if !recoverMalformed || !errors.Is(err, errMalformedSessionStore) {
			return err
		}
		store = newPersistedSessionStore()
		ok = true
	}
	if !ok {
		store = newPersistedSessionStore()
	}
	store = normalizePersistedSessionStore(store)
	store.Sessions[key] = sess
	store.LastKey = key
	return writeSessionStoreToKeyring(kr, store)
}

func keychainSessionEntryCollisionUnlocked(key string) error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
		return err
	}
	if ok {
		if _, exists := store.Sessions[key]; exists {
			return cachedSessionAlreadyExistsError(key)
		}
	}
	return nil
}

func readSessionFromKeychain(key string) (persistedSession, bool, error) {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return persistedSession{}, false, err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil || !ok {
		return persistedSession{}, false, err
	}
	sess, ok := store.Sessions[key]
	if !ok {
		return persistedSession{}, false, nil
	}
	return sess, true, nil
}

type sessionFileBackup struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

type sessionFileState struct {
	sessionPath string
	lastPath    string
	session     sessionFileBackup
	last        sessionFileBackup
}

// writeSessionFileNoFollow writes through the already-open staging descriptor.
// The descriptor is created exclusively by createSessionTempFile, so this
// helper never reopens or mutates a caller-controlled pathname.
func writeSessionFileNoFollow(_ string, file *os.File, data []byte, perm os.FileMode) error {
	if file == nil {
		return errors.New("web session cache temporary file is nil")
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat web session cache temporary file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: temporary path is not a regular file", errUnsafeSessionCacheFile)
	}
	if err := file.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set web session cache temporary file permissions: %w", err)
	}
	if err := file.Truncate(0); err != nil {
		return err
	}

	n, err := file.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

type sessionTempFile struct {
	root *os.Root
	file *os.File
	name string
	path string
	mode os.FileMode
}

// createSessionTempFile creates a fresh staging inode beneath the destination
// directory. The rooted helper provides exclusive creation and no-follow
// semantics; callers own the returned root until cleanup.
func createSessionTempFile(destination string, perm os.FileMode) (*sessionTempFile, error) {
	parent, err := os.OpenRoot(filepath.Dir(destination))
	if err != nil {
		return nil, err
	}
	privatePerm := perm.Perm() & 0o700
	if privatePerm == 0 {
		privatePerm = 0o600
	}
	pattern := fmt.Sprintf(".asc-web-session-%s-*.tmp", filepath.Base(destination))
	file, name, err := secureopen.CreateTempNoFollowInRootWithCreator(
		parent,
		".",
		pattern,
		privatePerm,
		secureopen.OpenNewPrivateFileNoFollowInRoot,
	)
	if err != nil {
		_ = parent.Close()
		return nil, err
	}
	if err := secureopen.PreparePrivateFile(file, privatePerm); err != nil {
		_ = file.Close()
		_ = parent.Remove(name)
		_ = parent.Close()
		return nil, fmt.Errorf("failed to secure web session cache temporary file: %w", err)
	}
	return &sessionTempFile{
		root: parent,
		file: file,
		name: name,
		path: filepath.Join(filepath.Dir(destination), name),
		mode: privatePerm,
	}, nil
}

func (temp *sessionTempFile) close() error {
	if temp == nil || temp.file == nil {
		return nil
	}
	err := temp.file.Close()
	temp.file = nil
	return err
}

func (temp *sessionTempFile) cleanup() {
	if temp == nil {
		return
	}
	_ = temp.close()
	if temp.root != nil {
		if temp.name != "" {
			_ = temp.root.Remove(temp.name)
		}
		_ = temp.root.Close()
		temp.root = nil
	}
}

func createAndWriteSessionTempFile(destination string, data []byte, perm os.FileMode) (*sessionTempFile, error) {
	temp, err := createSessionTempFile(destination, perm)
	if err != nil {
		return nil, err
	}
	if err := sessionFileWrite(temp.path, temp.file, data, temp.mode); err != nil {
		temp.cleanup()
		return nil, err
	}
	if err := temp.close(); err != nil {
		temp.cleanup()
		return nil, err
	}
	return temp, nil
}

func publishSessionTempFile(temp *sessionTempFile, destination string) error {
	if temp == nil || temp.root == nil {
		return errors.New("web session cache temporary file is unavailable")
	}
	if err := temp.root.Rename(temp.name, filepath.Base(destination)); err != nil {
		return err
	}
	temp.name = ""
	return nil
}

func writeNewPrivateSessionFile(path string, data []byte) error {
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()

	name := filepath.Base(path)
	file, err := secureopen.OpenNewPrivateFileNoFollowInRoot(parent, name, 0o600)
	if err != nil {
		return err
	}
	cleanup := func() {
		_ = file.Close()
		_ = parent.Remove(name)
	}
	if err := secureopen.PreparePrivateFile(file, 0o600); err != nil {
		cleanup()
		return fmt.Errorf("failed to secure web session cache file: %w", err)
	}
	if n, writeErr := file.Write(data); writeErr != nil {
		cleanup()
		return writeErr
	} else if n != len(data) {
		cleanup()
		return io.ErrShortWrite
	}
	if err := file.Close(); err != nil {
		_ = parent.Remove(name)
		return err
	}
	return nil
}

func classifySessionCachePathError(path string, err error) error {
	info, statErr := os.Lstat(path)
	if statErr != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing symlink %q", errUnsafeSessionCacheFile, path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: path is not a regular file: %q", errUnsafeSessionCacheFile, path)
	}
	return err
}

func readBoundedSessionCacheFile(file *os.File, path string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, webSessionCacheMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > webSessionCacheMaxBytes {
		return nil, fmt.Errorf("%w: %q exceeds the %d-byte size limit", errUnsafeSessionCacheFile, path, webSessionCacheMaxBytes)
	}
	return data, nil
}

func backupSessionFile(path string) (sessionFileBackup, error) {
	file, err := secureopen.OpenExistingNoFollow(path)
	if err != nil {
		err = classifySessionCachePathError(path, err)
		if os.IsNotExist(err) {
			return sessionFileBackup{}, nil
		}
		return sessionFileBackup{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return sessionFileBackup{}, fmt.Errorf("failed to stat web session cache backup: %w", err)
	}
	if !info.Mode().IsRegular() {
		return sessionFileBackup{}, fmt.Errorf("%w: path is not a regular file: %q", errUnsafeSessionCacheFile, path)
	}

	if info.Size() > webSessionCacheMaxBytes {
		return sessionFileBackup{}, fmt.Errorf("%w: %q exceeds the %d-byte size limit", errUnsafeSessionCacheFile, path, webSessionCacheMaxBytes)
	}
	data, err := readBoundedSessionCacheFile(file, path)
	if err != nil {
		return sessionFileBackup{}, err
	}
	return sessionFileBackup{exists: true, data: data, mode: info.Mode().Perm()}, nil
}

func restoreSessionFile(path string, backup sessionFileBackup) error {
	if !backup.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	tmp, err := createSessionTempFile(path, backup.mode)
	if err != nil {
		return err
	}
	// Rollback must not reuse the injectable persistence writer: the writer may
	// be the failure being recovered from. The private staging descriptor still
	// preserves the same exclusive no-follow write boundary.
	if err := writeSessionFileNoFollow(tmp.path, tmp.file, backup.data, tmp.mode); err != nil {
		tmp.cleanup()
		return err
	}
	// The staging inode stays private while secret bytes are written. Restore
	// the captured mode only after the write so rollback remains an exact state
	// restoration without exposing a broader temporary file.
	if err := tmp.file.Chmod(backup.mode.Perm()); err != nil {
		tmp.cleanup()
		return err
	}
	if err := tmp.close(); err != nil {
		tmp.cleanup()
		return err
	}
	if err := publishSessionTempFile(tmp, path); err != nil {
		tmp.cleanup()
		return err
	}
	tmp.cleanup()
	return nil
}

// captureFileSessionState snapshots both files that make up a file-backed
// session. The last-session pointer is part of the cache state: restoring only
// the account file can leave a failed overwrite selecting a different session.
func captureFileSessionState(key string) (sessionFileState, error) {
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		return sessionFileState{}, err
	}
	lastPath, err := webSessionLastFilePath()
	if err != nil {
		return sessionFileState{}, err
	}
	sessionBackup, err := backupSessionFile(sessionPath)
	if err != nil {
		return sessionFileState{}, fmt.Errorf("failed to back up session cache: %w", err)
	}
	lastBackup, err := backupSessionFile(lastPath)
	if err != nil {
		return sessionFileState{}, fmt.Errorf("failed to back up last session pointer: %w", err)
	}
	return sessionFileState{
		sessionPath: sessionPath,
		lastPath:    lastPath,
		session:     sessionBackup,
		last:        lastBackup,
	}, nil
}

func (state sessionFileState) restore() error {
	return errors.Join(
		restoreSessionFile(state.sessionPath, state.session),
		restoreSessionFile(state.lastPath, state.last),
	)
}

func writeSessionToFile(key string, sess persistedSession) error {
	dir, err := webSessionCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create session cache dir: %w", err)
	}

	raw, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	state, err := captureFileSessionState(key)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		if rollbackErr := state.restore(); rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("failed to roll back session cache: %w", rollbackErr))
		}
		return cause
	}

	tmpSession, err := createAndWriteSessionTempFile(state.sessionPath, raw, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write session cache: %w", err)
	}
	if err := publishSessionTempFile(tmpSession, state.sessionPath); err != nil {
		tmpSession.cleanup()
		return fmt.Errorf("failed to finalize session cache: %w", err)
	}
	tmpSession.cleanup()

	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		return rollback(fmt.Errorf("failed to marshal last session pointer: %w", err))
	}
	tmpLast, err := createAndWriteSessionTempFile(state.lastPath, lastRaw, 0o600)
	if err != nil {
		return rollback(fmt.Errorf("failed to write last session pointer: %w", err))
	}
	if err := publishSessionTempFile(tmpLast, state.lastPath); err != nil {
		tmpLast.cleanup()
		return rollback(fmt.Errorf("failed to finalize last session pointer: %w", err))
	}
	tmpLast.cleanup()
	return nil
}

// writeSessionToFileIfAbsent creates a file-backed session without replacing
// an entry that appeared after import validation. O_EXCL is the persistence
// boundary: a preceding existence check alone would leave a TOCTOU window.
func writeSessionToFileIfAbsent(key string, sess persistedSession) error {
	dir, err := webSessionCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create session cache dir: %w", err)
	}

	raw, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	state, err := captureFileSessionState(key)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		if rollbackErr := state.restore(); rollbackErr != nil {
			return errors.Join(cause, fmt.Errorf("failed to roll back session cache: %w", rollbackErr))
		}
		return cause
	}

	if err := writeNewPrivateSessionFile(state.sessionPath, raw); err != nil {
		if os.IsExist(err) {
			return cachedSessionAlreadyExistsError(key)
		}
		return rollback(fmt.Errorf("failed to create session cache: %w", err))
	}

	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		return rollback(fmt.Errorf("failed to marshal last session pointer: %w", err))
	}
	tmpLast, err := createAndWriteSessionTempFile(state.lastPath, lastRaw, 0o600)
	if err != nil {
		return rollback(fmt.Errorf("failed to write last session pointer: %w", err))
	}
	if err := publishSessionTempFile(tmpLast, state.lastPath); err != nil {
		tmpLast.cleanup()
		return rollback(fmt.Errorf("failed to finalize last session pointer: %w", err))
	}
	tmpLast.cleanup()
	return nil
}

func cachedSessionAlreadyExistsError(key string) error {
	return fmt.Errorf("cached web session already exists for %s: %w", key, os.ErrExist)
}

// fileSessionEntryCollision reports whether any file artifact already
// occupies the target path. Lstat intentionally counts a malformed file or a
// symlink as occupied: no-overwrite must not guess that either is absent.
func fileSessionEntryCollision(key string) error {
	path, err := webSessionFilePath(key)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect session cache: %w", err)
	}
	return cachedSessionAlreadyExistsError(key)
}

func readSessionFromFile(key string) (persistedSession, bool, error) {
	path, err := webSessionFilePath(key)
	if err != nil {
		return persistedSession{}, false, err
	}
	raw, err := readSessionCacheFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedSession{}, false, nil
		}
		return persistedSession{}, false, err
	}
	var sess persistedSession
	if err := json.Unmarshal(raw, &sess); err != nil {
		return persistedSession{}, false, fmt.Errorf("%w: %w", errMalformedSessionFile, err)
	}
	if sess.Version != webSessionCacheVersion {
		return persistedSession{}, false, nil
	}
	return sess, true, nil
}

func readLastKeyFromFile() (string, bool, error) {
	path, err := webSessionLastFilePath()
	if err != nil {
		return "", false, err
	}
	raw, err := readSessionCacheFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	var last persistedLastSession
	if err := json.Unmarshal(raw, &last); err != nil {
		return "", false, err
	}
	if last.Version != webSessionCacheVersion || strings.TrimSpace(last.Key) == "" {
		return "", false, nil
	}
	return strings.TrimSpace(last.Key), true, nil
}

// readSessionCacheFile reads one cache entry without following a symlink in
// the cache pathname. Empty regular files are still returned to the JSON
// decoder so they retain the existing malformed-entry behavior.
func readSessionCacheFile(path string) ([]byte, error) {
	file, err := secureopen.OpenExistingNoFollow(path)
	if err != nil {
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: refusing symlink %q", errUnsafeSessionCacheFile, path)
		}
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat web session cache file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: path is not a regular file: %q", errUnsafeSessionCacheFile, path)
	}
	if info.Size() > webSessionCacheMaxBytes {
		return nil, fmt.Errorf("%w: %q exceeds the %d-byte size limit", errUnsafeSessionCacheFile, path, webSessionCacheMaxBytes)
	}

	return readBoundedSessionCacheFile(file, path)
}

func persistSessionBySelection(selection backendSelection, key string, sess persistedSession) error {
	if selection.backend == sessionBackendOff {
		return nil
	}
	// Hold the entry lock so a concurrent conditional delete cannot compare a
	// stamp before this write and delete the entry it produces.
	return withSessionEntryLock(key, func() error {
		return persistSessionBySelectionLocked(selection, key, sess)
	})
}

func persistSessionBySelectionLocked(selection backendSelection, key string, sess persistedSession) error {
	switch selection.backend {
	case sessionBackendOff:
		return nil
	case sessionBackendKeychain:
		if err := writeSessionToKeychain(key, sess); err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				return writeSessionToFile(key, sess)
			}
			return err
		}
		return nil
	case sessionBackendFile:
		return writeSessionToFile(key, sess)
	default:
		return nil
	}
}

type keychainItemState struct {
	key    string
	item   keyring.Item
	exists bool
}

type keychainSessionState struct {
	items    []keychainItemState
	captured bool
}

// captureKeychainSessionState snapshots the aggregate and legacy items that
// can change while replacing one account. Raw item bytes preserve the prior
// last-session choice and malformed-store bytes when a later write fails.
// Callers hold the shared store lock when the keychain is part of a mutation.
func captureKeychainSessionState(key string) (keychainSessionState, error) {
	kr, err := sessionKeyringOpen()
	if err != nil {
		if isKeyringUnavailable(err) {
			return keychainSessionState{}, nil
		}
		return keychainSessionState{}, fmt.Errorf("failed to back up keychain session: %w", err)
	}
	keys := []string{webSessionStoreItem, keyringSessionItem(key), webSessionLastKeyItem}
	state := keychainSessionState{items: make([]keychainItemState, 0, len(keys)), captured: true}
	for _, itemKey := range keys {
		item, err := kr.Get(itemKey)
		if err != nil {
			if errors.Is(err, keyring.ErrKeyNotFound) {
				state.items = append(state.items, keychainItemState{key: itemKey})
				continue
			}
			return keychainSessionState{}, fmt.Errorf("failed to back up keychain item: %w", err)
		}
		state.items = append(state.items, keychainItemState{key: itemKey, item: item, exists: true})
	}
	return state, nil
}

func (state keychainSessionState) restore() error {
	if !state.captured {
		return nil
	}
	kr, err := sessionKeyringOpen()
	if err != nil {
		return fmt.Errorf("failed to restore keychain session: %w", err)
	}
	var restoreErr error
	for _, itemState := range state.items {
		if itemState.exists {
			restoreErr = errors.Join(restoreErr, kr.Set(itemState.item))
			continue
		}
		removeErr := kr.Remove(itemState.key)
		if errors.Is(removeErr, keyring.ErrKeyNotFound) {
			removeErr = nil
		}
		restoreErr = errors.Join(restoreErr, removeErr)
	}
	return restoreErr
}

type importedSessionState struct {
	file     *sessionFileState
	keychain *keychainSessionState
}

func captureImportedSessionState(selection backendSelection, key string) (importedSessionState, error) {
	state := importedSessionState{}
	if selection.backend == sessionBackendFile || selection.fallbackFile {
		fileState, err := captureFileSessionState(key)
		if err != nil {
			return importedSessionState{}, err
		}
		state.file = &fileState
	}
	if selection.backend == sessionBackendKeychain || selection.fallbackKeychain {
		keychainState, err := captureKeychainSessionState(key)
		if err != nil {
			return importedSessionState{}, err
		}
		state.keychain = &keychainState
	}
	return state, nil
}

func (state importedSessionState) restore() error {
	var restoreErr error
	if state.file != nil {
		restoreErr = errors.Join(restoreErr, state.file.restore())
	}
	if state.keychain != nil {
		restoreErr = errors.Join(restoreErr, state.keychain.restore())
	}
	return restoreErr
}

// persistImportedSessionBySelection stores an imported session at the final
// persistence boundary. Explicit overwrite imports snapshot both backends
// before cleanup so a failure after one backend changes restores the previous
// session, mirror, and last-session pointer exactly.
func persistImportedSessionBySelection(selection backendSelection, key string, sess persistedSession, overwrite bool) error {
	if selection.backend == sessionBackendOff {
		return nil
	}
	return withSessionEntryLock(key, func() error {
		// Any selection that can inspect or mutate the aggregate keychain must
		// hold the fail-closed shared lock. This also serializes a keychain
		// collision check with the file O_EXCL create in fallback mode.
		if selection.backend == sessionBackendKeychain || selection.fallbackKeychain {
			return withSessionStoreLock(func() error {
				return persistImportedSessionBySelectionLocked(selection, key, sess, overwrite)
			})
		}
		return persistImportedSessionBySelectionLocked(selection, key, sess, overwrite)
	})
}

func persistImportedSessionBySelectionLocked(selection backendSelection, key string, sess persistedSession, overwrite bool) error {
	switch selection.backend {
	case sessionBackendOff:
		return nil
	case sessionBackendKeychain:
		if !overwrite {
			if selection.fallbackFile {
				if err := fileSessionEntryCollision(key); err != nil {
					return err
				}
			}
			if err := writeSessionToKeychainIfAbsentUnlocked(key, sess); err != nil {
				if selection.fallbackFile && isKeyringUnavailable(err) {
					return writeSessionToFileIfAbsent(key, sess)
				}
				return err
			}
			return nil
		}

		state, err := captureImportedSessionState(selection, key)
		if err != nil {
			return err
		}
		if selection.fallbackFile {
			if err := deleteMirroredSessionFromFile(key); err != nil {
				return errors.Join(err, state.restore())
			}
		}
		if err := writeSessionToKeychainWithRecoveryUnlocked(key, sess, true); err != nil {
			// Fail closed: a stale keychain entry must not remain ahead of a
			// replacement written only to the file fallback.
			return errors.Join(err, state.restore())
		}
		return nil

	case sessionBackendFile:
		if !overwrite {
			if selection.fallbackKeychain {
				err := keychainSessionEntryCollisionUnlocked(key)
				if !isKeyringUnavailable(err) && err != nil {
					return fmt.Errorf("failed to inspect keychain session: %w", err)
				}
			}
			return writeSessionToFileIfAbsent(key, sess)
		}

		state, err := captureImportedSessionState(selection, key)
		if err != nil {
			return err
		}
		if selection.fallbackKeychain {
			if err := deleteSessionFromKeychainWithRecoveryUnlocked(key, true); err != nil && !isKeyringUnavailable(err) {
				return errors.Join(err, state.restore())
			}
		}
		if err := writeSessionToFile(key, sess); err != nil {
			return errors.Join(err, state.restore())
		}
		return nil
	default:
		return nil
	}
}

func readSessionFromFileWithKeychainFallbackOrigin(key string, fallbackKeychain bool) (persistedSession, sessionEntryOrigin, bool, error) {
	sess, ok, err := readSessionFromFile(key)
	if err == nil && (ok || !fallbackKeychain) {
		return sess, sessionEntryOriginWhenFound(sessionEntryOriginFile, ok), ok, nil
	}
	if err != nil && !fallbackKeychain {
		return persistedSession{}, sessionEntryOriginNone, false, err
	}

	sess, ok, keychainErr := readSessionFromKeychain(key)
	if keychainErr != nil {
		if err != nil {
			return persistedSession{}, sessionEntryOriginNone, false, err
		}
		return persistedSession{}, sessionEntryOriginNone, false, nil
	}
	if err != nil && !ok {
		return persistedSession{}, sessionEntryOriginNone, false, err
	}
	return sess, sessionEntryOriginWhenFound(sessionEntryOriginKeychain, ok), ok, nil
}

func sessionEntryOriginWhenFound(origin sessionEntryOrigin, found bool) sessionEntryOrigin {
	if !found {
		return sessionEntryOriginNone
	}
	return origin
}

func readSessionFromFileIgnoringErrors(key string) (persistedSession, bool) {
	sess, ok, err := readSessionFromFile(key)
	if err != nil {
		return persistedSession{}, false
	}
	return sess, ok
}

func readLastSessionFromFileIgnoringErrorsWithKey() (persistedSession, string, bool) {
	key, ok, err := readLastKeyFromFile()
	if err != nil || !ok {
		return persistedSession{}, "", false
	}
	sess, ok := readSessionFromFileIgnoringErrors(key)
	return sess, key, ok
}

func readSessionBySelection(selection backendSelection, key string) (persistedSession, bool, error) {
	sess, _, ok, err := readSessionBySelectionWithOrigin(selection, key)
	return sess, ok, err
}

func cachedSessionSourceForOrigin(origin sessionEntryOrigin) CachedSessionSource {
	switch origin {
	case sessionEntryOriginFile:
		return CachedSessionSourceFile
	case sessionEntryOriginKeychain:
		return CachedSessionSourceKeychain
	default:
		return CachedSessionSourceUnknown
	}
}

func selectionForCachedSessionSource(source CachedSessionSource) (backendSelection, bool) {
	switch source {
	case CachedSessionSourceFile:
		return backendSelection{backend: sessionBackendFile}, true
	case CachedSessionSourceKeychain:
		return backendSelection{backend: sessionBackendKeychain}, true
	default:
		return backendSelection{}, false
	}
}

func readSessionBySelectionWithOrigin(selection backendSelection, key string) (persistedSession, sessionEntryOrigin, bool, error) {
	switch selection.backend {
	case sessionBackendOff:
		return persistedSession{}, sessionEntryOriginNone, false, nil
	case sessionBackendKeychain:
		sess, ok, err := readSessionFromKeychain(key)
		if err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				return readSessionFromFileIgnoringErrorsWithOrigin(key)
			}
			return persistedSession{}, sessionEntryOriginNone, false, err
		}
		if !ok && selection.fallbackFile {
			return readSessionFromFileIgnoringErrorsWithOrigin(key)
		}
		return sess, sessionEntryOriginWhenFound(sessionEntryOriginKeychain, ok), ok, nil
	case sessionBackendFile:
		return readSessionFromFileWithKeychainFallbackOrigin(key, selection.fallbackKeychain)
	default:
		return persistedSession{}, sessionEntryOriginNone, false, nil
	}
}

func readSessionFromFileIgnoringErrorsWithOrigin(key string) (persistedSession, sessionEntryOrigin, bool, error) {
	sess, ok := readSessionFromFileIgnoringErrors(key)
	return sess, sessionEntryOriginWhenFound(sessionEntryOriginFile, ok), ok, nil
}

func readLastSessionFromKeychain() (persistedSession, bool, error) {
	sess, _, ok, err := readLastSessionFromKeychainWithKey()
	return sess, ok, err
}

func readLastSessionFromKeychainWithKey() (persistedSession, string, bool, error) {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return persistedSession{}, "", false, err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil || !ok {
		return persistedSession{}, "", false, err
	}
	lastKey, ok := resolvePersistedSessionStoreLastKey(store)
	if !ok {
		return persistedSession{}, "", false, nil
	}
	sess, ok := store.Sessions[lastKey]
	if !ok {
		return persistedSession{}, "", false, nil
	}
	return sess, lastKey, true, nil
}

func readLastSessionBySelection(selection backendSelection) (persistedSession, bool, error) {
	sess, _, _, ok, err := readLastSessionBySelectionWithOrigin(selection)
	return sess, ok, err
}

func readLastSessionBySelectionWithOrigin(selection backendSelection) (persistedSession, sessionEntryOrigin, string, bool, error) {
	switch selection.backend {
	case sessionBackendOff:
		return persistedSession{}, sessionEntryOriginNone, "", false, nil
	case sessionBackendKeychain:
		sess, key, ok, err := readLastSessionFromKeychainWithKey()
		if err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				fallback, fallbackKey, fallbackOK := readLastSessionFromFileIgnoringErrorsWithKey()
				return fallback, sessionEntryOriginWhenFound(sessionEntryOriginFile, fallbackOK), fallbackKey, fallbackOK, nil
			}
			return persistedSession{}, sessionEntryOriginNone, "", false, err
		}
		if !ok && selection.fallbackFile {
			fallback, fallbackKey, fallbackOK := readLastSessionFromFileIgnoringErrorsWithKey()
			return fallback, sessionEntryOriginWhenFound(sessionEntryOriginFile, fallbackOK), fallbackKey, fallbackOK, nil
		}
		return sess, sessionEntryOriginWhenFound(sessionEntryOriginKeychain, ok), key, ok, nil
	case sessionBackendFile:
		key, ok, err := readLastKeyFromFile()
		if err == nil && ok {
			sess, origin, found, readErr := readSessionFromFileWithKeychainFallbackOrigin(key, selection.fallbackKeychain)
			return sess, origin, key, found, readErr
		}
		if err != nil {
			if !selection.fallbackKeychain {
				return persistedSession{}, sessionEntryOriginNone, "", false, err
			}
			sess, key, ok, keychainErr := readLastSessionFromKeychainWithKey()
			if keychainErr == nil && ok {
				return sess, sessionEntryOriginKeychain, key, true, nil
			}
			return persistedSession{}, sessionEntryOriginNone, "", false, err
		}
		if !selection.fallbackKeychain {
			return persistedSession{}, sessionEntryOriginNone, "", false, nil
		}
		sess, key, ok, err := readLastSessionFromKeychainWithKey()
		if err != nil {
			return persistedSession{}, sessionEntryOriginNone, "", false, nil
		}
		return sess, sessionEntryOriginWhenFound(sessionEntryOriginKeychain, ok), key, ok, nil
	default:
		return persistedSession{}, sessionEntryOriginNone, "", false, nil
	}
}

func deleteSessionFromFile(key string) error {
	path, err := webSessionFilePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func deleteSessionFromKeychain(key string) error {
	return withSessionStoreLock(func() error { return deleteSessionFromKeychainUnlocked(key) })
}

func deleteSessionFromKeychainUnlocked(key string) error {
	return deleteSessionFromKeychainWithRecoveryUnlocked(key, false)
}

func deleteSessionFromKeychainWithRecoveryUnlocked(key string, recoverMalformed bool) error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
		if recoverMalformed && errors.Is(err, errMalformedSessionStore) {
			if err := removeSessionStoreFromKeyring(kr); err != nil {
				return err
			}
			if err := removeLegacySessionFromKeyring(kr, key); err != nil {
				return err
			}
			return removeLegacyLastKeyFromKeyring(kr)
		}
		return err
	}
	if ok {
		delete(store.Sessions, key)
		if len(store.Sessions) == 0 {
			if err := removeSessionStoreFromKeyring(kr); err != nil {
				return err
			}
		} else {
			if resolved, ok := resolvePersistedSessionStoreLastKey(store); ok {
				store.LastKey = resolved
			} else {
				store.LastKey = ""
			}
			if err := writeSessionStoreToKeyring(kr, store); err != nil {
				return err
			}
		}
	}
	if err := removeLegacySessionFromKeyring(kr, key); err != nil {
		return err
	}
	return removeLegacyLastKeyFromKeyring(kr)
}

func clearLastKeyInFile() error {
	path, err := webSessionLastFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func clearLastKeyInKeychainUnlocked() error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
		return err
	}
	if ok {
		store.LastKey = ""
		if len(store.Sessions) == 0 {
			if err := removeSessionStoreFromKeyring(kr); err != nil {
				return err
			}
		} else if err := writeSessionStoreToKeyring(kr, store); err != nil {
			return err
		}
	}
	return removeLegacyLastKeyFromKeyring(kr)
}

func deleteAllFromFile() error {
	dir, err := webSessionCacheDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if (strings.HasPrefix(name, "session-") && strings.HasSuffix(name, ".json")) || name == "last.json" {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func deleteAllFromKeychainUnlocked() error {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	if err := removeSessionStoreFromKeyring(kr); err != nil {
		return err
	}
	keys, err := kr.Keys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if key == webSessionStoreItem || key == webSessionLastKeyItem || strings.HasPrefix(key, webSessionKeyPrefix) {
			if err := kr.Remove(key); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
				return err
			}
		}
	}
	return nil
}

func resumeFromPersistedSession(ctx context.Context, sess persistedSession) (*AuthSession, bool, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, false, err
	}
	loaded := hydrateCookieJar(jar, sess)
	if loaded == 0 {
		return nil, false, nil
	}
	client := newWebHTTPClient(newSessionCookieTrackingJarWithCached(jar, &sess))
	info, err := sessionInfoFetcher(ctx, client)
	if err != nil {
		if isSessionInfoAuthExpired(err) {
			// Callers treat expiration as a soft re-auth path, so return the sentinel
			// directly instead of burying it inside transport-specific context.
			return nil, false, ErrCachedSessionExpired
		}
		return nil, false, nil
	}
	cached := sess
	session := &AuthSession{
		Client:           client,
		cachedUpdatedAt:  sess.UpdatedAt,
		cachedGeneration: sess.Generation,
		cachedSession:    &cached,
	}
	applySessionInfo(session, info)
	session.DeveloperTeamID = strings.TrimSpace(sess.DeveloperTeamID)
	return session, true, nil
}

func validatePersistedSessionReadOnly(ctx context.Context, sess persistedSession) (*http.Client, *sessionInfo, bool, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, false, err
	}
	loaded := hydrateCookieJar(jar, sess)
	if loaded == 0 {
		return nil, nil, false, nil
	}
	client := newWebHTTPClient(jar)
	info, err := sessionInfoFetcher(ctx, client)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, false, ctxErr
		}
		if isSessionInfoAuthExpired(err) {
			return nil, nil, false, ErrCachedSessionExpired
		}
		return nil, nil, false, ErrCachedSessionValidationFailed
	}
	return client, info, true, nil
}

func resumeFromPersistedSessionReadOnly(ctx context.Context, sess persistedSession) (*AuthSession, bool, error) {
	identity := &AuthSession{UserEmail: strings.TrimSpace(sess.UserEmail)}
	client, info, ok, err := validatePersistedSessionReadOnly(ctx, sess)
	if err != nil || !ok {
		return identity, ok, err
	}
	session := &AuthSession{Client: client, UserEmail: strings.TrimSpace(sess.UserEmail)}
	applySessionInfo(session, info)
	session.DeveloperTeamID = strings.TrimSpace(sess.DeveloperTeamID)
	return session, true, nil
}

func readOnlyFileBackendSelection() backendSelection {
	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return selection
	}
	// Deep validation is non-interactive. Read only file-backed caches so an
	// automatic or explicitly selected Keychain backend cannot open a native
	// authorization prompt.
	return backendSelection{backend: sessionBackendFile}
}

func loadSessionFromPersistedSession(sess persistedSession) (*AuthSession, bool, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, false, err
	}
	loaded := hydrateCookieJar(jar, sess)
	if loaded == 0 {
		return nil, false, nil
	}
	return &AuthSession{
		Client:           newWebHTTPClient(newSessionCookieTrackingJarWithCached(jar, &sess)),
		UserEmail:        strings.TrimSpace(sess.UserEmail),
		DeveloperTeamID:  strings.TrimSpace(sess.DeveloperTeamID),
		cachedUpdatedAt:  sess.UpdatedAt,
		cachedGeneration: sess.Generation,
		cachedSession:    &sess,
	}, true, nil
}

// PersistSession stores web-session cookies for later reuse.
func PersistSession(session *AuthSession) error {
	if session == nil || session.Client == nil || session.Client.Jar == nil {
		return nil
	}
	session.persistMu.Lock()
	defer session.persistMu.Unlock()
	username := strings.TrimSpace(session.UserEmail)
	if username == "" {
		return nil
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil
	}

	key := webSessionCacheKey(username)
	var (
		serialized persistedSession
		updates    map[trackedCookieKey]trackedCookieUpdate
		tracker    *sessionCookieTrackingJar
		err        error
	)
	if tracked, ok := session.Client.Jar.(*sessionCookieTrackingJar); ok {
		tracker = tracked
		serialized, updates, err = tracker.serializeWithUpdates(username)
	} else {
		serialized, err = serializeCookieJarWithError(session.Client.Jar, username)
	}
	if err != nil {
		return err
	}
	preserveCachedCookieDeadlines(&serialized, session.cachedSession, updates, serialized.UpdatedAt)
	// CookieJar.Cookies omits Domain, so scope-aware patching above may retain
	// the source domain temporarily to match its exact renewal/tombstone scope.
	// Persist only the safe host-narrowed representation and resolve aliases
	// after narrowing, before the cache is made visible to the next load.
	narrowPersistedCookieDomains(&serialized)
	serialized.DeveloperTeamID = strings.TrimSpace(session.DeveloperTeamID)
	if err := persistSessionBySelection(selection, key, serialized); err != nil {
		return err
	}
	if tracker != nil {
		tracker.markPersisted(updates, &serialized)
	}
	cached := serialized
	session.cachedSession = &cached
	session.cachedUpdatedAt = serialized.UpdatedAt
	session.cachedGeneration = serialized.Generation
	if tracker != nil {
		tracker.mu.Lock()
		tracker.cached = &cached
		tracker.mu.Unlock()
	}
	return nil
}

// LoadCachedSession loads a cached web session cookie jar without validating it
// against the live App Store Connect session endpoint. This is used for
// best-effort relogin attempts that want to preserve Apple trust cookies.
func LoadCachedSession(username string) (*AuthSession, bool, error) {
	return loadCachedSessionWithSource(username, CachedSessionSourceUnknown)
}

// LoadCachedSessionFromSource loads an account from the backend that supplied
// a default identity. Explicit account lookups should use LoadCachedSession so
// configured backend precedence remains unchanged.
func LoadCachedSessionFromSource(username string, source CachedSessionSource) (*AuthSession, bool, error) {
	return loadCachedSessionWithSource(username, source)
}

func loadCachedSessionWithSource(username string, source CachedSessionSource) (*AuthSession, bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, false, nil
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}
	if selected, ok := selectionForCachedSessionSource(source); ok {
		selection = selected
	}

	key := webSessionCacheKey(username)
	sess, origin, ok, err := readSessionBySelectionWithOrigin(selection, key)
	if err != nil || !ok {
		return nil, false, err
	}
	loaded, ok, err := loadSessionFromPersistedSession(sess)
	if loaded != nil {
		loaded.cachedSource = cachedSessionSourceForOrigin(origin)
	}
	return loaded, ok, err
}

// ResumeCachedSessionWithoutPersist validates a cached session for one Apple
// ID without prompting or writing/migrating cached state.
func ResumeCachedSessionWithoutPersist(ctx context.Context, username string) (*AuthSession, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, false, nil
	}

	selection := readOnlyFileBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}

	key := webSessionCacheKey(username)
	sess, ok, err := readSessionBySelection(selection, key)
	if err != nil || !ok {
		return nil, false, err
	}
	if strings.TrimSpace(sess.UserEmail) == "" {
		sess.UserEmail = username
	}
	return resumeFromPersistedSessionReadOnly(ctx, sess)
}

// TryResumeSession attempts to resume a session for a specific Apple ID.
func TryResumeSession(ctx context.Context, username string) (*AuthSession, bool, error) {
	return tryResumeSessionWithSource(ctx, username, CachedSessionSourceUnknown)
}

// TryResumeSessionFromSource resumes an account from the backend that supplied
// a default identity. Explicit account lookups should use TryResumeSession so
// configured backend precedence remains unchanged.
func TryResumeSessionFromSource(ctx context.Context, username string, source CachedSessionSource) (*AuthSession, bool, error) {
	return tryResumeSessionWithSource(ctx, username, source)
}

func tryResumeSessionWithSource(ctx context.Context, username string, source CachedSessionSource) (*AuthSession, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, false, nil
	}

	configuredSelection := resolveBackendSelection()
	selection := configuredSelection
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}
	preserveAutoFileFallback := source == CachedSessionSourceFile &&
		configuredSelection.backend == sessionBackendFile && configuredSelection.fallbackKeychain
	if selected, ok := selectionForCachedSessionSource(source); ok && !preserveAutoFileFallback {
		selection = selected
	}

	key := webSessionCacheKey(username)
	sess, origin, ok, err := readSessionBySelectionWithOrigin(selection, key)
	if err != nil || !ok {
		return nil, false, err
	}
	resumed, origin, ok, err := resumePersistedSessionWithKeychainFallback(
		ctx,
		sess,
		origin,
		key,
		configuredSelection.fallbackKeychain,
	)
	if resumed != nil {
		resumed.cachedSource = cachedSessionSourceForOrigin(origin)
	}
	if err != nil || !ok || resumed == nil {
		return resumed, ok, err
	}
	// Best effort: persist refreshed cookies after successful session validation.
	_ = PersistSession(resumed)
	return resumed, true, nil
}

func resumePersistedSessionWithKeychainFallback(
	ctx context.Context,
	sess persistedSession,
	origin sessionEntryOrigin,
	key string,
	allowKeychainFallback bool,
) (*AuthSession, sessionEntryOrigin, bool, error) {
	resumed, ok, err := resumeFromPersistedSession(ctx, sess)
	if origin != sessionEntryOriginFile || !allowKeychainFallback || strings.TrimSpace(key) == "" || !errors.Is(err, ErrCachedSessionExpired) {
		return resumed, origin, ok, err
	}

	// Automatic discovery can select a locally hydratable file session whose
	// cookie has since been rejected by Apple. Retry the same account from the
	// keychain mirror before making the caller reauthenticate. The account key
	// stays fixed, so this cannot silently switch to another cached identity.
	fallback, fallbackOK, fallbackErr := readSessionFromKeychain(key)
	if fallbackErr != nil || !fallbackOK {
		return resumed, origin, ok, err
	}
	fallbackResumed, fallbackResumedOK, fallbackResumeErr := resumeFromPersistedSession(ctx, fallback)
	if fallbackResumeErr != nil || !fallbackResumedOK || fallbackResumed == nil {
		return resumed, origin, ok, err
	}
	return fallbackResumed, sessionEntryOriginKeychain, true, nil
}

// LoadLastCachedSession loads the last cached web session cookie jar without
// validating it against the live App Store Connect session endpoint.
func LoadLastCachedSession() (*AuthSession, bool, error) {
	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}

	sess, ok, err := readLastSessionBySelection(selection)
	if err != nil || !ok {
		return nil, false, err
	}
	return loadSessionFromPersistedSession(sess)
}

// ResumeLastCachedSessionWithoutPersist validates the last cached session
// without prompting or writing/migrating cached state.
func ResumeLastCachedSessionWithoutPersist(ctx context.Context) (*AuthSession, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	selection := readOnlyFileBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}

	sess, ok, err := readLastSessionBySelection(selection)
	if err != nil || !ok {
		return nil, false, err
	}
	return resumeFromPersistedSessionReadOnly(ctx, sess)
}

// TryResumeLastSession attempts to resume the last successful web session.
func TryResumeLastSession(ctx context.Context) (*AuthSession, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}

	sess, origin, key, ok, err := readLastSessionBySelectionWithOrigin(selection)
	if err != nil || !ok {
		return nil, false, err
	}
	resumed, origin, ok, err := resumePersistedSessionWithKeychainFallback(
		ctx,
		sess,
		origin,
		key,
		selection.fallbackKeychain,
	)
	if resumed != nil {
		resumed.cachedSource = cachedSessionSourceForOrigin(origin)
	}
	if err != nil || !ok || resumed == nil {
		return resumed, ok, err
	}
	// Best effort: persist refreshed cookies after successful session validation.
	_ = PersistSession(resumed)
	return resumed, true, nil
}

// DeleteSession removes the cached session for a specific Apple ID.
func DeleteSession(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	key := webSessionCacheKey(username)
	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return deleteSessionEntryLocked(selection, key)
	}
	return withSessionEntryLock(key, func() error {
		return deleteSessionEntryLocked(selection, key)
	})
}

// deleteSessionEntryLocked removes every cached entry for key. Callers already
// holding the entry lock use it directly so a nested acquisition cannot stall
// behind the lock they hold.
func deleteSessionEntryLocked(selection backendSelection, key string) error {
	var err error
	switch selection.backend {
	case sessionBackendOff:
		err = nil
	case sessionBackendKeychain:
		if deleteErr := deleteSessionFromKeychain(key); deleteErr != nil {
			if selection.fallbackFile && isKeyringUnavailable(deleteErr) {
				err = deleteMirroredSessionFromFile(key)
			} else {
				err = deleteErr
			}
		} else if selection.fallbackFile {
			err = deleteMirroredSessionFromFile(key)
		}
	case sessionBackendFile:
		if deleteErr := deleteSessionFromFile(key); deleteErr != nil {
			err = deleteErr
		} else {
			err = clearLastKeyInFileIfMatches(key)
		}
		if selection.fallbackKeychain {
			err = joinDeleteErrors(err, ignoreUnavailableKeyringError(deleteSessionFromKeychain(key)))
		}
	default:
		err = nil
	}
	return err
}

// DeleteSessionIfMatches removes the cached web session for username only while
// the stored entry is still the one loaded carries. A caller that proves its
// loaded cookie jar unusable would otherwise delete by Apple ID alone and take
// out a valid replacement that a concurrent process persisted while it was
// working through 2FA, leaving no cached session at all. Reporting whether the
// delete happened lets the caller stay quiet when a newer entry was preserved.
//
// The comparison and the delete it authorizes run under the entry lock that
// persistence also takes, so a replacement written between them is no longer
// deleted by a decision made before it existed. Only the entry whose stamp
// matched is removed: the other backend can hold a newer session persisted by
// a process configured with a different ASC_WEB_SESSION_CACHE_BACKEND, and it
// is removed only when it carries the same stamp.
//
// When the current entry cannot be read, or the caller has no stamp to
// compare, the unconditional delete stands: a proven-stale jar left on disk is
// reloaded by the next invocation and burns another 2FA code against the same
// failure.
func DeleteSessionIfMatches(username string, loaded *AuthSession) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return false, nil
	}
	if loaded != nil {
		loaded.persistMu.Lock()
		defer loaded.persistMu.Unlock()
	}
	if loaded == nil || (loaded.cachedUpdatedAt.IsZero() && loaded.cachedGeneration == "") {
		return true, DeleteSession(username)
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return false, nil
	}
	key := webSessionCacheKey(username)
	deleted := false
	err := withSessionEntryLock(key, func() error {
		readSelection := selection
		if selected, selectedOK := selectionForCachedSessionSource(loaded.cachedSource); selectedOK {
			readSelection = selected
		}
		current, origin, ok, err := readSessionBySelectionWithOrigin(readSelection, key)
		if err != nil {
			deleted = true
			return deleteSessionEntryLocked(readSelection, key)
		}
		if !ok {
			return nil
		}
		if !samePersistedSessionIdentity(current, loaded) {
			return nil
		}
		if sessionCompareDeleteBarrier != nil {
			sessionCompareDeleteBarrier()
		}
		deleted = true
		return deleteMatchedSessionEntryLocked(selection, key, origin, current.UpdatedAt, current.Generation)
	})
	return deleted, err
}

func samePersistedSessionIdentity(current persistedSession, loaded *AuthSession) bool {
	if loaded.cachedGeneration != "" || current.Generation != "" {
		return loaded.cachedGeneration != "" && current.Generation != "" && current.Generation == loaded.cachedGeneration
	}
	return !loaded.cachedUpdatedAt.IsZero() && current.UpdatedAt.Equal(loaded.cachedUpdatedAt)
}

// deleteMatchedSessionEntryLocked removes the cache entry whose stamp matched
// the loaded session, plus the mirrored entry in the other backend only when
// that one carries the same stamp and is therefore the same proven-stale
// session. Legacy artifacts are always cleared: nothing writes them any more,
// so they can only hold a session at least as stale as the matched one.
func deleteMatchedSessionEntryLocked(selection backendSelection, key string, origin sessionEntryOrigin, stamp time.Time, generation string) error {
	var err error
	switch origin {
	case sessionEntryOriginFile:
		if deleteErr := deleteSessionFromFile(key); deleteErr != nil {
			err = deleteErr
		} else {
			err = clearLastKeyInFileIfMatches(key)
		}
		if sessionMirrorEnabled(selection) && keychainSessionCarriesIdentity(key, stamp, generation) {
			err = joinDeleteErrors(err, ignoreUnavailableKeyringError(deleteSessionFromKeychain(key)))
		}
	case sessionEntryOriginKeychain:
		if deleteErr := deleteSessionFromKeychain(key); deleteErr != nil && (!selection.fallbackFile || !isKeyringUnavailable(deleteErr)) {
			err = deleteErr
		}
		if sessionMirrorEnabled(selection) && fileSessionCarriesIdentity(key, stamp, generation) {
			err = joinDeleteErrors(err, deleteMirroredSessionFromFile(key))
		}
	default:
		return nil
	}
	return err
}

// sessionMirrorEnabled reports whether the selection keeps entries in both
// backends, which is what makes a mirrored entry possible at all.
func sessionMirrorEnabled(selection backendSelection) bool {
	switch selection.backend {
	case sessionBackendFile:
		return selection.fallbackKeychain
	case sessionBackendKeychain:
		return selection.fallbackFile
	default:
		return false
	}
}

// fileSessionCarriesStamp reports whether the file entry is the same session as
// the matched one. An unreadable entry counts: it cannot be the valid
// replacement this guard exists to protect, and leaving a corrupt file behind
// only makes the next invocation fall back to a staler backend.
func fileSessionCarriesIdentity(key string, stamp time.Time, generation string) bool {
	sess, ok, err := readSessionFromFile(key)
	if err != nil {
		// A keychain entry already proven stale may safely clean up a corrupt
		// mirrored file; leaving it causes repeated fallback failures.
		return errors.Is(err, errMalformedSessionFile)
	}
	return ok && persistedSessionIdentityMatches(sess, stamp, generation)
}

// keychainSessionCarriesStamp reports whether the keychain entry is the same
// session as the matched one. A keychain that cannot be read is left alone
// rather than cleared blindly: unavailability says nothing about the entry.
func keychainSessionCarriesIdentity(key string, stamp time.Time, generation string) bool {
	sess, ok, err := readSessionFromKeychain(key)
	if err != nil {
		return false
	}
	return ok && persistedSessionIdentityMatches(sess, stamp, generation)
}

func persistedSessionIdentityMatches(sess persistedSession, stamp time.Time, generation string) bool {
	if generation != "" || sess.Generation != "" {
		return generation != "" && sess.Generation != "" && generation == sess.Generation
	}
	return sess.UpdatedAt.Equal(stamp)
}

// DeleteAllSessions removes all cached web sessions.
func DeleteAllSessions() error {
	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return deleteAllSessionsLocked(selection)
	}
	return withSessionDeleteAllLock(selection, func() error {
		return deleteAllSessionsLocked(selection)
	})
}

// deleteAllSessionsLocked removes all cached web sessions while its caller
// holds the cache-global lock and, when a keychain backend is selected, the
// stable aggregate-store lock. Keep keychain calls on their unlocked helpers:
// the outer transaction already owns that lock.
func deleteAllSessionsLocked(selection backendSelection) error {
	var err error
	switch selection.backend {
	case sessionBackendOff:
		err = nil
	case sessionBackendKeychain:
		if deleteErr := deleteAllFromKeychainUnlocked(); deleteErr != nil {
			if selection.fallbackFile && isKeyringUnavailable(deleteErr) {
				err = deleteAllFromFile()
			} else {
				err = deleteErr
			}
		} else if selection.fallbackFile {
			err = deleteAllFromFile()
		}
	case sessionBackendFile:
		if deleteErr := deleteAllFromFile(); deleteErr != nil {
			err = deleteErr
		} else {
			err = clearLastSessionMarkerUnlocked()
		}
		if selection.fallbackKeychain {
			err = joinDeleteErrors(err, ignoreUnavailableKeyringError(deleteAllFromKeychainUnlocked()))
		}
	default:
		err = nil
	}
	return err
}

func joinDeleteErrors(primaryErr, secondaryErr error) error {
	if primaryErr == nil {
		return secondaryErr
	}
	if secondaryErr == nil {
		return primaryErr
	}
	return errors.Join(primaryErr, secondaryErr)
}

func ignoreUnavailableKeyringError(err error) error {
	if isKeyringUnavailable(err) {
		return nil
	}
	return err
}

func deleteMirroredSessionFromFile(key string) error {
	return joinDeleteErrors(deleteSessionFromFile(key), clearLastKeyInFileIfMatches(key))
}

// clearLastSessionMarker clears the "last used session" pointer.
func clearLastSessionMarker() error {
	selection := resolveBackendSelection()
	if selection.backend == sessionBackendKeychain || selection.fallbackKeychain {
		return withSessionStoreLock(clearLastSessionMarkerUnlocked)
	}
	return clearLastSessionMarkerUnlocked()
}

func clearLastSessionMarkerUnlocked() error {
	selection := resolveBackendSelection()
	switch selection.backend {
	case sessionBackendOff:
		return nil
	case sessionBackendKeychain:
		if err := clearLastKeyInKeychainUnlocked(); err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				return clearLastKeyInFile()
			}
			return err
		}
		return nil
	case sessionBackendFile:
		err := clearLastKeyInFile()
		if selection.fallbackKeychain {
			err = joinDeleteErrors(err, ignoreUnavailableKeyringError(clearLastKeyInKeychainUnlocked()))
		}
		return err
	default:
		return nil
	}
}

func clearLastKeyInFileIfMatches(key string) error {
	lastKey, ok, err := readLastKeyFromFile()
	if err != nil {
		// Session deletion already succeeded. If the marker is malformed/unreadable,
		// clear it best-effort instead of turning logout into a false-negative.
		_ = clearLastKeyInFile()
		return nil
	}
	if !ok || lastKey != key {
		return nil
	}
	return clearLastKeyInFile()
}
