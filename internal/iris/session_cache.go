package iris

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	irisSessionCacheEnvEnabled = "ASC_IRIS_SESSION_CACHE"
	irisSessionCacheEnvDir     = "ASC_IRIS_SESSION_CACHE_DIR"
)

type persistedSession struct {
	Version   int                  `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
	Cookies   map[string][]pCookie `json:"cookies"` // keyed by base URL string
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
	Key     string `json:"key"` // hash key for the last used username
}

func irisSessionCacheEnabled() bool {
	v, ok := os.LookupEnv(irisSessionCacheEnvEnabled)
	if !ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func irisSessionCacheDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(irisSessionCacheEnvDir)); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".asc", "iris"), nil
}

func irisSessionCacheKey(username string) string {
	normalized := strings.ToLower(strings.TrimSpace(username))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func irisSessionCachePathForKey(key string) (string, error) {
	dir, err := irisSessionCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session-"+key+".json"), nil
}

func irisLastSessionPath() (string, error) {
	dir, err := irisSessionCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last.json"), nil
}

func sessionCookieURLs() []*url.URL {
	// These are the domains we touch during SRP/2FA and IRIS calls.
	return []*url.URL{
		{Scheme: "https", Host: "appstoreconnect.apple.com", Path: "/"},
		{Scheme: "https", Host: "idmsa.apple.com", Path: "/"},
		{Scheme: "https", Host: "gsa.apple.com", Path: "/"},
	}
}

func persistCookieJar(jar http.CookieJar, username string) error {
	if jar == nil {
		return fmt.Errorf("cookie jar is required")
	}
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username is required")
	}
	if !irisSessionCacheEnabled() {
		return nil
	}

	dir, err := irisSessionCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create session cache dir: %w", err)
	}

	out := persistedSession{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
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
			list = append(list, pCookie{
				Name:     c.Name,
				Value:    c.Value,
				Path:     c.Path,
				Domain:   c.Domain,
				Expires:  c.Expires,
				MaxAge:   c.MaxAge,
				Secure:   c.Secure,
				HttpOnly: c.HttpOnly,
				SameSite: int(c.SameSite),
			})
		}
		if len(list) > 0 {
			out.Cookies[u.String()] = list
		}
	}

	key := irisSessionCacheKey(username)
	cachePath, err := irisSessionCachePathForKey(key)
	if err != nil {
		return err
	}
	tmpPath := cachePath + ".tmp"
	raw, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return fmt.Errorf("failed to write session cache: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return fmt.Errorf("failed to finalize session cache: %w", err)
	}

	// Track last-used username (hashed) so users don't need to re-enter email.
	lastPath, err := irisLastSessionPath()
	if err == nil {
		_ = os.WriteFile(lastPath, mustJSON(persistedLastSession{Version: 1, Key: key}), 0o600)
	}

	return nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func loadCookieJarInto(jar http.CookieJar, username string) (bool, error) {
	if jar == nil {
		return false, fmt.Errorf("cookie jar is required")
	}
	if strings.TrimSpace(username) == "" {
		return false, nil
	}
	if !irisSessionCacheEnabled() {
		return false, nil
	}

	key := irisSessionCacheKey(username)
	cachePath, err := irisSessionCachePathForKey(key)
	if err != nil {
		return false, err
	}
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read session cache: %w", err)
	}

	var sess persistedSession
	if err := json.Unmarshal(raw, &sess); err != nil {
		return false, fmt.Errorf("failed to decode session cache: %w", err)
	}
	if sess.Version != 1 {
		return false, fmt.Errorf("unsupported session cache version: %d", sess.Version)
	}

	loaded := 0
	for base, list := range sess.Cookies {
		u, err := url.Parse(base)
		if err != nil {
			continue
		}
		cookies := make([]*http.Cookie, 0, len(list))
		for _, pc := range list {
			if pc.Name == "" {
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

	return loaded > 0, nil
}

func loadLastSessionKey() (string, bool, error) {
	if !irisSessionCacheEnabled() {
		return "", false, nil
	}
	path, err := irisLastSessionPath()
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
	if last.Version != 1 || strings.TrimSpace(last.Key) == "" {
		return "", false, nil
	}
	return last.Key, true, nil
}

// TryResumeSession attempts to reuse a previously authenticated session (cookies)
// for the provided Apple ID. It validates the session via /olympus/v1/session.
//
// If ok=false, callers should fall back to SRP login.
func TryResumeSession(username string) (*AuthSession, bool, error) {
	if strings.TrimSpace(username) == "" {
		return nil, false, nil
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, false, err
	}
	ok, err := loadCookieJarInto(jar, username)
	if err != nil || !ok {
		return nil, false, err
	}
	client := newIrisHTTPClient(jar)

	info, err := getSessionInfo(client)
	if err != nil {
		return nil, false, nil
	}
	return &AuthSession{
		Client:     client,
		ProviderID: info.Provider.ProviderID,
		TeamID:     fmt.Sprintf("%d", info.Provider.ProviderID),
		UserEmail:  info.User.EmailAddress,
	}, true, nil
}

// TryResumeLastSession attempts to resume the last cached session (if any),
// without requiring the caller to provide an Apple ID.
func TryResumeLastSession() (*AuthSession, bool, error) {
	key, ok, err := loadLastSessionKey()
	if err != nil || !ok {
		return nil, false, err
	}
	cachePath, err := irisSessionCachePathForKey(key)
	if err != nil {
		return nil, false, err
	}
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false, nil
	}
	var sess persistedSession
	if err := json.Unmarshal(raw, &sess); err != nil {
		return nil, false, nil
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, false, err
	}
	loaded := 0
	for base, list := range sess.Cookies {
		u, err := url.Parse(base)
		if err != nil {
			continue
		}
		cookies := make([]*http.Cookie, 0, len(list))
		for _, pc := range list {
			if pc.Name == "" {
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
	if loaded == 0 {
		return nil, false, nil
	}

	client := newIrisHTTPClient(jar)
	info, err := getSessionInfo(client)
	if err != nil {
		return nil, false, nil
	}
	return &AuthSession{
		Client:     client,
		ProviderID: info.Provider.ProviderID,
		TeamID:     fmt.Sprintf("%d", info.Provider.ProviderID),
		UserEmail:  info.User.EmailAddress,
	}, true, nil
}

// PersistSession caches cookies for future runs.
// This effectively acts like a "remember me" token; treat the cache file as sensitive.
func PersistSession(session *AuthSession) {
	if session == nil || session.Client == nil || session.Client.Jar == nil {
		return
	}
	username := strings.TrimSpace(session.UserEmail)
	if username == "" {
		return
	}
	_ = persistCookieJar(session.Client.Jar, username)
}
