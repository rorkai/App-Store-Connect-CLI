package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/99designs/keyring"
)

const (
	webSessionCacheEnabledEnv = "ASC_WEB_SESSION_CACHE"
	webSessionCacheDirEnv     = "ASC_WEB_SESSION_CACHE_DIR"
	webSessionBackendEnv      = "ASC_WEB_SESSION_CACHE_BACKEND"

	webSessionCacheVersion = 1

	webSessionKeyringService = "asc-web-session"
	webSessionStoreItem      = "asc:web-session:store"
	webSessionKeyPrefix      = "asc:web-session:"
	webSessionLastKeyItem    = "asc:web-session:last"
)

var ErrCachedSessionExpired = errors.New("cached web session expired")

type sessionBackend int

const (
	sessionBackendOff sessionBackend = iota
	sessionBackendKeychain
	sessionBackendFile
)

type backendSelection struct {
	backend      sessionBackend
	fallbackFile bool
}

type persistedSession struct {
	Version   int                  `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
	Cookies   map[string][]pCookie `json:"cookies"`
}

type persistedSessionStore struct {
	Version  int                         `json:"version"`
	LastKey  string                      `json:"last_key,omitempty"`
	Sessions map[string]persistedSession `json:"sessions,omitempty"`
}

type pCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	MaxAge   int       `json:"max_age,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HttpOnly bool      `json:"http_only,omitempty"`
	SameSite int       `json:"same_site,omitempty"`
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
	sessionInfoFetcher = getSessionInfo
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
		return backendSelection{backend: sessionBackendKeychain}
	case "", "auto":
		return backendSelection{backend: sessionBackendKeychain, fallbackFile: true}
	default:
		return backendSelection{backend: sessionBackendKeychain, fallbackFile: true}
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
		{Scheme: "https", Host: "idmsa.apple.com", Path: "/"},
		{Scheme: "https", Host: "gsa.apple.com", Path: "/"},
	}
}

func isExpiredCookie(c pCookie, now time.Time) bool {
	if c.MaxAge < 0 {
		return true
	}
	if !c.Expires.IsZero() && c.Expires.Before(now) {
		return true
	}
	return false
}

func serializeCookieJar(jar http.CookieJar) persistedSession {
	now := time.Now().UTC()
	out := persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: now,
		Cookies:   map[string][]pCookie{},
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
			if isExpiredCookie(pc, now) {
				continue
			}
			list = append(list, pc)
		}
		if len(list) > 0 {
			out.Cookies[u.String()] = list
		}
	}
	return out
}

func hydrateCookieJar(jar http.CookieJar, sess persistedSession) int {
	now := time.Now().UTC()
	loaded := 0
	for base, list := range sess.Cookies {
		u, err := url.Parse(base)
		if err != nil {
			continue
		}
		cookies := make([]*http.Cookie, 0, len(list))
		for _, pc := range list {
			if pc.Name == "" || isExpiredCookie(pc, now) {
				continue
			}
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
		return persistedSessionStore{}, false, fmt.Errorf("failed to decode keychain session store: %w", err)
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
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		return err
	}
	tmpSessionPath := sessionPath + ".tmp"
	if err := os.WriteFile(tmpSessionPath, raw, 0o600); err != nil {
		return fmt.Errorf("failed to write session cache: %w", err)
	}
	if err := os.Rename(tmpSessionPath, sessionPath); err != nil {
		return fmt.Errorf("failed to finalize session cache: %w", err)
	}

	lastPath, err := webSessionLastFilePath()
	if err != nil {
		return nil
	}
	lastRaw, err := json.Marshal(persistedLastSession{Version: webSessionCacheVersion, Key: key})
	if err != nil {
		return nil
	}
	tmpLastPath := lastPath + ".tmp"
	if err := os.WriteFile(tmpLastPath, lastRaw, 0o600); err == nil {
		_ = os.Rename(tmpLastPath, lastPath)
	}
	return nil
}

func readSessionFromFile(key string) (persistedSession, bool, error) {
	path, err := webSessionFilePath(key)
	if err != nil {
		return persistedSession{}, false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedSession{}, false, nil
		}
		return persistedSession{}, false, err
	}
	var sess persistedSession
	if err := json.Unmarshal(raw, &sess); err != nil {
		return persistedSession{}, false, fmt.Errorf("failed to decode session cache: %w", err)
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
	raw, err := os.ReadFile(path)
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

func persistSessionBySelection(selection backendSelection, key string, sess persistedSession) error {
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

func readSessionBySelection(selection backendSelection, key string) (persistedSession, bool, error) {
	switch selection.backend {
	case sessionBackendOff:
		return persistedSession{}, false, nil
	case sessionBackendKeychain:
		sess, ok, err := readSessionFromKeychain(key)
		if err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				return readSessionFromFile(key)
			}
			return persistedSession{}, false, err
		}
		return sess, ok, nil
	case sessionBackendFile:
		return readSessionFromFile(key)
	default:
		return persistedSession{}, false, nil
	}
}

func readLastSessionFromKeychain() (persistedSession, bool, error) {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return persistedSession{}, false, err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil || !ok {
		return persistedSession{}, false, err
	}
	lastKey, ok := resolvePersistedSessionStoreLastKey(store)
	if !ok {
		return persistedSession{}, false, nil
	}
	sess, ok := store.Sessions[lastKey]
	if !ok {
		return persistedSession{}, false, nil
	}
	return sess, true, nil
}

func readLastSessionBySelection(selection backendSelection) (persistedSession, bool, error) {
	switch selection.backend {
	case sessionBackendOff:
		return persistedSession{}, false, nil
	case sessionBackendKeychain:
		sess, ok, err := readLastSessionFromKeychain()
		if err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				key, ok, err := readLastKeyFromFile()
				if err != nil || !ok {
					return persistedSession{}, false, err
				}
				return readSessionFromFile(key)
			}
			return persistedSession{}, false, err
		}
		return sess, ok, nil
	case sessionBackendFile:
		key, ok, err := readLastKeyFromFile()
		if err != nil || !ok {
			return persistedSession{}, false, err
		}
		return readSessionFromFile(key)
	default:
		return persistedSession{}, false, nil
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
	kr, err := sessionKeyringOpen()
	if err != nil {
		return err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil {
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

func clearLastKeyInKeychain() error {
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
		if strings.HasPrefix(name, "session-") && strings.HasSuffix(name, ".json") || name == "last.json" {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func deleteAllFromKeychain() error {
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
	client := newWebHTTPClient(jar)
	info, err := sessionInfoFetcher(ctx, client)
	if err != nil {
		if isSessionInfoAuthExpired(err) {
			return nil, false, fmt.Errorf("%w: %w", ErrCachedSessionExpired, err)
		}
		return nil, false, nil
	}
	return &AuthSession{
		Client:           client,
		ProviderID:       info.Provider.ProviderID,
		PublicProviderID: strings.TrimSpace(info.Provider.PublicProviderID),
		TeamID:           fmt.Sprintf("%d", info.Provider.ProviderID),
		UserEmail:        strings.TrimSpace(info.User.EmailAddress),
	}, true, nil
}

// PersistSession stores web-session cookies for later reuse.
func PersistSession(session *AuthSession) error {
	if session == nil || session.Client == nil || session.Client.Jar == nil {
		return nil
	}
	username := strings.TrimSpace(session.UserEmail)
	if username == "" {
		return nil
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil
	}

	key := webSessionCacheKey(username)
	serialized := serializeCookieJar(session.Client.Jar)
	return persistSessionBySelection(selection, key, serialized)
}

// TryResumeSession attempts to resume a session for a specific Apple ID.
func TryResumeSession(ctx context.Context, username string) (*AuthSession, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, false, nil
	}

	selection := resolveBackendSelection()
	if selection.backend == sessionBackendOff {
		return nil, false, nil
	}

	key := webSessionCacheKey(username)
	sess, ok, err := readSessionBySelection(selection, key)
	if err != nil || !ok {
		return nil, false, err
	}
	resumed, ok, err := resumeFromPersistedSession(ctx, sess)
	if err != nil || !ok || resumed == nil {
		return resumed, ok, err
	}
	// Best effort: persist refreshed cookies after successful session validation.
	_ = PersistSession(resumed)
	return resumed, true, nil
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

	sess, ok, err := readLastSessionBySelection(selection)
	if err != nil || !ok {
		return nil, false, err
	}
	resumed, ok, err := resumeFromPersistedSession(ctx, sess)
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
	switch selection.backend {
	case sessionBackendOff:
		return nil
	case sessionBackendKeychain:
		if err := deleteSessionFromKeychain(key); err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				if err := deleteSessionFromFile(key); err != nil {
					return err
				}
				return clearLastSessionMarker()
			}
			return err
		}
		return nil
	case sessionBackendFile:
		if err := deleteSessionFromFile(key); err != nil {
			return err
		}
		return clearLastSessionMarker()
	default:
		return nil
	}
}

// DeleteAllSessions removes all cached web sessions.
func DeleteAllSessions() error {
	selection := resolveBackendSelection()
	switch selection.backend {
	case sessionBackendOff:
		return nil
	case sessionBackendKeychain:
		if err := deleteAllFromKeychain(); err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				if err := deleteAllFromFile(); err != nil {
					return err
				}
				return clearLastSessionMarker()
			}
			return err
		}
		return nil
	case sessionBackendFile:
		if err := deleteAllFromFile(); err != nil {
			return err
		}
		return clearLastSessionMarker()
	default:
		return nil
	}
}

// clearLastSessionMarker clears the "last used session" pointer.
func clearLastSessionMarker() error {
	selection := resolveBackendSelection()
	switch selection.backend {
	case sessionBackendOff:
		return nil
	case sessionBackendKeychain:
		if err := clearLastKeyInKeychain(); err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				return clearLastKeyInFile()
			}
			return err
		}
		return nil
	case sessionBackendFile:
		return clearLastKeyInFile()
	default:
		return nil
	}
}
