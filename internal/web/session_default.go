package web

import (
	"errors"
	"fmt"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strings"
)

// ErrNoCachedSession reports that default Apple ID resolution found no cached
// web session to fall back to.
var ErrNoCachedSession = errors.New("no cached web session is available")

var readDefaultSessionFromFileFn = readSessionFromFile

// AmbiguousCachedSessionError reports that more than one cached web session
// could serve as the default, so the caller has to name one.
type AmbiguousCachedSessionError struct {
	AppleIDs []string
}

// CachedSessionSource identifies the backend that supplied a default account.
// Callers use it only to keep the subsequent session read aligned with the
// discovery result; explicit account lookups retain their configured backend
// precedence.
type CachedSessionSource uint8

const (
	CachedSessionSourceUnknown CachedSessionSource = iota
	CachedSessionSourceFile
	CachedSessionSourceKeychain
)

func (e *AmbiguousCachedSessionError) Error() string {
	return "multiple cached web sessions are available: " + strings.Join(e.AppleIDs, ", ")
}

// DefaultCachedAppleID returns the Apple ID of the only cached web session so
// commands can omit --apple-id when there is nothing to choose between. It
// reports ErrNoCachedSession when the cache is disabled or empty and an
// *AmbiguousCachedSessionError listing every cached Apple ID when there are
// several. Entries that predate stored Apple ID metadata cannot be selected by
// name and are ignored.
func DefaultCachedAppleID() (string, error) {
	appleID, _, err := DefaultCachedAppleIDWithSource()
	return appleID, err
}

// DefaultCachedAppleIDWithSource is DefaultCachedAppleID plus the backend that
// supplied the selected account. The source prevents a fallback keychain
// identity from being shadowed by an anonymous legacy file entry during the
// immediately following session lookup.
func DefaultCachedAppleIDWithSource() (string, CachedSessionSource, error) {
	selection := resolveBackendSelection()
	sessions, source, err := listSessionsBySelectionWithSource(selection)
	if err != nil {
		return "", CachedSessionSourceUnknown, err
	}
	appleIDs := cachedSessionAppleIDs(sessions)
	switch len(appleIDs) {
	case 0:
		return "", source, ErrNoCachedSession
	case 1:
		return appleIDs[0], source, nil
	default:
		return "", source, &AmbiguousCachedSessionError{AppleIDs: appleIDs}
	}
}

// CachedSessionAppleIDs lists the Apple IDs of every cached web session in the
// selected backend, sorted case-insensitively. Like the last-session lookup, an
// automatic backend consults the keychain when the file cache has no usable
// account identity.
func CachedSessionAppleIDs() ([]string, error) {
	selection := resolveBackendSelection()
	sessions, _, err := listSessionsBySelectionWithSource(selection)
	if err != nil {
		return nil, err
	}
	return cachedSessionAppleIDs(sessions), nil
}

func cachedSessionAppleIDs(sessions []persistedSession) []string {
	seen := map[string]struct{}{}
	appleIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		email := strings.TrimSpace(sess.UserEmail)
		if email == "" {
			continue
		}
		normalized := strings.ToLower(email)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		appleIDs = append(appleIDs, email)
	}
	sort.Slice(appleIDs, func(i, j int) bool {
		return strings.ToLower(appleIDs[i]) < strings.ToLower(appleIDs[j])
	})
	return appleIDs
}

func listSessionsBySelectionWithSource(selection backendSelection) ([]persistedSession, CachedSessionSource, error) {
	switch selection.backend {
	case sessionBackendOff:
		return nil, CachedSessionSourceUnknown, nil
	case sessionBackendKeychain:
		sessions, err := listSessionsFromKeychain()
		if err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				fallback, fallbackErr := listSessionsFromFile()
				return fallback, CachedSessionSourceFile, fallbackErr
			}
			return nil, CachedSessionSourceKeychain, err
		}
		if len(sessions) == 0 && selection.fallbackFile {
			fallback, fallbackErr := listSessionsFromFile()
			return fallback, CachedSessionSourceFile, fallbackErr
		}
		return sessions, CachedSessionSourceKeychain, nil
	case sessionBackendFile:
		sessions, err := listSessionsFromFile()
		if err != nil {
			if !selection.fallbackKeychain || !errors.Is(err, errUnsafeSessionCacheFile) {
				return nil, CachedSessionSourceFile, err
			}
			sessions = nil
		}
		if selection.fallbackKeychain {
			sessions = hydratableSessions(sessions)
		}
		if len(cachedSessionAppleIDs(sessions)) > 0 || !selection.fallbackKeychain {
			return sessions, CachedSessionSourceFile, nil
		}
		fallback, err := listSessionsFromKeychain()
		if err != nil {
			// Mirror the last-session lookup: a keychain that cannot be read
			// leaves the empty file result standing instead of failing.
			return nil, CachedSessionSourceFile, nil
		}
		return hydratableSessions(fallback), CachedSessionSourceKeychain, nil
	default:
		return nil, CachedSessionSourceUnknown, nil
	}
}

func hydratableSessions(sessions []persistedSession) []persistedSession {
	validationURL, err := url.Parse(olympusSessionURL)
	if err != nil {
		return nil
	}
	usable := make([]persistedSession, 0, len(sessions))
	for _, sess := range sessions {
		jar, err := cookiejar.New(nil)
		if err != nil {
			continue
		}
		hydrateCookieJar(jar, sess)
		if len(jar.Cookies(validationURL)) == 0 {
			continue
		}
		usable = append(usable, sess)
	}
	return usable
}

// listSessionsFromFile reads every well-formed, current-version session entry
// in the file cache. Malformed or stale entries are skipped: they cannot be
// resumed, so they must not block the default resolution either.
func listSessionsFromFile() ([]persistedSession, error) {
	dir, err := webSessionCacheDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []persistedSession
	var unsafeEntryErr error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, "session-"), ".json")
		sess, ok, err := readDefaultSessionFromFileFn(key)
		if err != nil {
			if errors.Is(err, errMalformedSessionFile) {
				continue
			}
			if errors.Is(err, errUnsafeSessionCacheFile) {
				if unsafeEntryErr == nil {
					unsafeEntryErr = fmt.Errorf("read cached web session %q: %w", name, err)
				}
				continue
			}
			return nil, fmt.Errorf("read cached web session %q: %w", name, err)
		}
		if !ok {
			continue
		}
		sessions = append(sessions, sess)
	}
	if len(sessions) == 0 && unsafeEntryErr != nil {
		return nil, unsafeEntryErr
	}
	return sessions, nil
}

func listSessionsFromKeychain() ([]persistedSession, error) {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return nil, err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil || !ok {
		return nil, err
	}
	sessions := make([]persistedSession, 0, len(store.Sessions))
	for _, sess := range store.Sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
