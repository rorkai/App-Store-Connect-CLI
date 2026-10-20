package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultSigninBackoff is how long automatic sign-in pauses after Apple's
	// sign-in service answered with a 5xx and sent no usable Retry-After.
	DefaultSigninBackoff = 15 * time.Minute
	// MaxSigninBackoff caps any cooldown, including one Apple asked for, so a
	// huge Retry-After or a tampered record cannot block sign-in indefinitely.
	MaxSigninBackoff = time.Hour

	signinBackoffVersion = 1
)

// SigninBackoff records that Apple's IdMSA sign-in service answered an
// automatic or explicit sign-in with a 5xx, so later automatic sign-ins for the
// same Apple Account can wait instead of adding to Apple's throttling count.
type SigninBackoff struct {
	Version    int       `json:"version"`
	FailedAt   time.Time `json:"failed_at"`
	NotBefore  time.Time `json:"not_before"`
	Stage      string    `json:"stage,omitempty"`
	Status     int       `json:"status,omitempty"`
	RetryAfter string    `json:"retry_after,omitempty"`
	// Locked records that Apple reported the account locked for API sign-in
	// (ErrAppleAccountLocked), which only the account owner can clear.
	Locked bool `json:"locked,omitempty"`
	// SucceededAt is when Apple last accepted a sign-in for the account. It
	// outlives the cooldown it cleared so that a slower process cannot record
	// a failure it observed before that success.
	SucceededAt time.Time `json:"succeeded_at,omitzero"`
}

// NewSigninBackoff builds the cooldown for a sign-in 5xx observed at now. A
// Retry-After from Apple, in seconds or as an HTTP date, sets the cooldown, and
// one allowing an immediate retry leaves none; without a valid one
// DefaultSigninBackoff applies. Either is capped at MaxSigninBackoff. A locked
// account pauses for the full cap: no retry can unlock it, and the explicit
// `asc web auth login` that follows the unlock clears the pause.
func NewSigninBackoff(serviceErr *SigninServiceError, now time.Time) SigninBackoff {
	backoff := SigninBackoff{Version: signinBackoffVersion, FailedAt: now.UTC()}
	cooldown := DefaultSigninBackoff
	if serviceErr != nil {
		backoff.Stage = serviceErr.Stage
		backoff.Status = serviceErr.Status
		backoff.RetryAfter = serviceErr.RetryAfter
		backoff.Locked = serviceErr.AccountLocked()
		if requested, ok := parseSigninRetryAfter(serviceErr.RetryAfter, now); ok {
			cooldown = requested
		}
		if backoff.Locked {
			cooldown = MaxSigninBackoff
		}
	}
	backoff.NotBefore = backoff.FailedAt.Add(min(cooldown, MaxSigninBackoff))
	return backoff
}

// Active reports whether the cooldown pauses sign-in at all.
func (b SigninBackoff) Active() bool {
	return b.NotBefore.After(b.FailedAt)
}

// ActiveUntil reports when automatic sign-in may resume and whether that is
// still in the future at now. The deadline never lies more than
// MaxSigninBackoff past the recorded failure, and a failure recorded in the
// future (a clock moved backward, or an edited record) is ignored, so neither
// can extend the pause beyond the cap.
func (b SigninBackoff) ActiveUntil(now time.Time) (time.Time, bool) {
	if b.FailedAt.IsZero() || b.FailedAt.After(now) {
		return time.Time{}, false
	}
	notBefore := b.NotBefore
	if limit := b.FailedAt.Add(MaxSigninBackoff); notBefore.After(limit) {
		notBefore = limit
	}
	return notBefore, now.Before(notBefore)
}

func parseSigninRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 {
			return 0, false
		}
		if seconds > int64(MaxSigninBackoff/time.Second) {
			return MaxSigninBackoff, true
		}
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}

func signinBackoffFilePath(appleID string) (string, error) {
	dir, err := webSessionCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "signin-backoff-"+webSessionCacheKey(appleID)+".json"), nil
}

// LoadSigninBackoff returns the recorded sign-in cooldown for appleID, if any.
func LoadSigninBackoff(appleID string) (SigninBackoff, bool, error) {
	path, err := signinBackoffFilePath(appleID)
	if err != nil {
		return SigninBackoff{}, false, err
	}
	raw, err := readSessionCacheFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SigninBackoff{}, false, nil
		}
		return SigninBackoff{}, false, err
	}
	var backoff SigninBackoff
	if err := json.Unmarshal(raw, &backoff); err != nil {
		return SigninBackoff{}, false, fmt.Errorf("sign-in backoff record %q is malformed: %w", path, err)
	}
	if backoff.Version != signinBackoffVersion {
		return SigninBackoff{}, false, nil
	}
	return backoff, true, nil
}

// RecordSigninBackoff stores the sign-in cooldown for appleID next to the web
// session cache, readable only by the current user, and reports whether it did.
// Concurrent asc processes update the record under a per-account lock, and a
// failure observed before the recorded outcome is dropped, so an attempt that
// finishes late cannot reinstate a cooldown a newer sign-in already cleared or
// replace a newer failure's cooldown.
func RecordSigninBackoff(appleID string, backoff SigninBackoff, now time.Time) (bool, error) {
	recorded := false
	err := withSigninBackoffLock(appleID, func(existing SigninBackoff, found bool) (*SigninBackoff, error) {
		if found && (outcomeNewer(existing.SucceededAt, backoff.FailedAt, now) || outcomeNewer(existing.FailedAt, backoff.FailedAt, now)) {
			return nil, nil
		}
		backoff.SucceededAt = time.Time{}
		if found {
			backoff.SucceededAt = existing.SucceededAt
		}
		recorded = true
		return &backoff, nil
	})
	return recorded && err == nil, err
}

// ClearSigninBackoff ends any cooldown for appleID observed before Apple
// accepted a sign-in at succeededAt. A failure observed later is kept.
func ClearSigninBackoff(appleID string, succeededAt, now time.Time) error {
	return withSigninBackoffLock(appleID, func(existing SigninBackoff, found bool) (*SigninBackoff, error) {
		if found && (outcomeNewer(existing.FailedAt, succeededAt, now) || outcomeNewer(existing.SucceededAt, succeededAt, now)) {
			return nil, nil
		}
		return &SigninBackoff{SucceededAt: succeededAt.UTC()}, nil
	})
}

// outcomeNewer reports whether a stored outcome time supersedes one observed at
// observed. A stored time later than now can only come from a clock that has
// since moved backward, or an edited record, so it is stale and never wins.
func outcomeNewer(stored, observed, now time.Time) bool {
	return stored.After(observed) && !stored.After(now)
}

// withSigninBackoffLock runs update on appleID's current record under the
// account's advisory lock and writes the record it returns, if any.
func withSigninBackoffLock(appleID string, update func(existing SigninBackoff, found bool) (*SigninBackoff, error)) error {
	path, err := signinBackoffFilePath(appleID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create session cache dir: %w", err)
	}
	release := acquireSessionEntryLock("signin-backoff-" + webSessionCacheKey(appleID))
	defer release()

	existing, found, err := LoadSigninBackoff(appleID)
	if err != nil {
		// A malformed record carries no ordering worth preserving.
		existing, found = SigninBackoff{}, false
	}
	next, err := update(existing, found)
	if err != nil || next == nil {
		return err
	}
	next.Version = signinBackoffVersion
	raw, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("failed to marshal sign-in backoff: %w", err)
	}
	temp, err := createAndWriteSessionTempFile(path, raw)
	if err != nil {
		return fmt.Errorf("failed to write sign-in backoff: %w", err)
	}
	defer temp.cleanup()
	if err := publishSessionTempFile(temp, path); err != nil {
		return fmt.Errorf("failed to finalize sign-in backoff: %w", err)
	}
	return nil
}
