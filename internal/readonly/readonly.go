// Package readonly implements the global read-only mode that refuses every
// mutating request before it leaves the process.
//
// Read-only mode is enabled by the ASC_READ_ONLY environment variable or the
// root --read-only flag. It is enforced at the transport layer of each
// Apple-facing client (App Store Connect API, App Store Connect web session,
// Developer Portal, Apple Ads, StoreKit) rather than per command, so a session
// that sets it cannot write even through commands that were never taught about
// the mode. Either source only enables the mode; neither can disable the other.
package readonly

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

const (
	// EnvVar enables read-only mode when set to a truthy value (1/true/yes/on).
	EnvVar = "ASC_READ_ONLY"
	// FlagName is the root flag that enables read-only mode for one invocation.
	FlagName = "read-only"
)

// ErrRefused is the sentinel matched by errors.Is for every refusal raised by
// read-only mode.
var ErrRefused = errors.New("read-only mode refused a mutating request")

var flagEnabled atomic.Bool

// SetFlagEnabled records whether the root --read-only flag was passed. The root
// command resets it on every bind so one parse cannot leak into the next.
func SetFlagEnabled(enabled bool) {
	flagEnabled.Store(enabled)
}

// Enabled reports whether read-only mode is active from either source.
func Enabled() bool {
	return flagEnabled.Load() || envEnabled()
}

// Source names the mechanism that enabled read-only mode for diagnostics.
func Source() string {
	if flagEnabled.Load() {
		return "--" + FlagName
	}
	return EnvVar
}

func envEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvVar))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// RefusedError is returned when read-only mode blocks a mutating request.
type RefusedError struct {
	Source string
	Method string
	Target string
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("%s is set; refusing %s %s", e.Source, e.Method, e.Target)
}

// Is reports ErrRefused so callers can classify wrapped refusals.
func (e *RefusedError) Is(target error) bool {
	return target == ErrRefused
}

type readIntentKey struct{}

// WithReadIntent marks ctx as carrying a request that only reads data even
// though it is transported with a mutating method, such as Apple Ads selector
// queries or Developer Portal proxied GETs. Callers must only mark requests
// whose server-side effect is a read.
func WithReadIntent(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, readIntentKey{}, true)
}

// HasReadIntent reports whether ctx was marked by WithReadIntent.
func HasReadIntent(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	marked, _ := ctx.Value(readIntentKey{}).(bool)
	return marked
}

// IsMutatingMethod reports whether an HTTP method can change remote state.
// An empty method is treated as GET, matching net/http.
func IsMutatingMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// Check returns a *RefusedError when read-only mode is enabled and the request
// would mutate remote state. Requests marked with WithReadIntent pass.
func Check(ctx context.Context, method, target string) error {
	if !IsMutatingMethod(method) || HasReadIntent(ctx) || !Enabled() {
		return nil
	}
	return &RefusedError{
		Source: Source(),
		Method: strings.ToUpper(strings.TrimSpace(method)),
		Target: target,
	}
}

// Target renders a request URL for refusal diagnostics without its query
// string, so signed upload URLs and filters never reach stderr.
func Target(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return strings.SplitN(strings.TrimSpace(rawURL), "?", 2)[0]
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.Path
}
