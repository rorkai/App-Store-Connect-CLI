package shared

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// IfExistsMode selects what a create-style command does when App Store
// Connect reports that the resource already exists.
type IfExistsMode string

const (
	// IfExistsFail returns the conflict unchanged. It is the default and
	// preserves the historical behavior of every create command.
	IfExistsFail IfExistsMode = "fail"
	// IfExistsSkip treats the existing resource as success and leaves it
	// unchanged.
	IfExistsSkip IfExistsMode = "skip"
	// IfExistsUpdate routes the same inputs to the resource's update call.
	IfExistsUpdate IfExistsMode = "update"
)

const ifExistsFlagName = "if-exists"

// BindIfExistsFlag registers --if-exists with the supported modes listed in
// the usage text. fail is always supported and is the default.
func BindIfExistsFlag(fs *flag.FlagSet, supported ...IfExistsMode) *string {
	modes := ifExistsModeNames(supported)
	return fs.String(ifExistsFlagName, string(IfExistsFail), fmt.Sprintf("Behavior when the resource already exists: %s", strings.Join(modes, ", ")))
}

// ParseIfExistsMode validates the raw --if-exists value against the modes the
// command supports. It returns a usage-class error (exit code 2) before any
// HTTP request so an unsupported value is never silently ignored.
//
// BindIfExistsFlag defaults the flag to fail, so an empty or all-whitespace raw
// value can only come from an explicitly supplied --if-exists "" and is
// rejected rather than silently read as fail.
func ParseIfExistsMode(raw string, supported ...IfExistsMode) (IfExistsMode, error) {
	value := IfExistsMode(strings.ToLower(strings.TrimSpace(raw)))
	modes := ifExistsModeNames(supported)
	for _, mode := range modes {
		if string(value) == mode {
			return value, nil
		}
	}
	return "", UsageErrorf("--%s must be one of %s (got %q)", ifExistsFlagName, strings.Join(modes, ", "), strings.TrimSpace(raw))
}

// ParseOptionalIfExistsMode is ParseIfExistsMode for a value that may legitimately
// be absent: a PushExecutionOptions-style field an in-process caller left unset,
// or a mode read back from an artifact written before --if-exists existed. An
// absent value means fail, the historical behavior. A non-empty unsupported
// value is still a usage error.
//
// Do not use this for a bound --if-exists flag value. BindIfExistsFlag defaults
// the flag to fail, so an empty value there was supplied explicitly and
// ParseIfExistsMode rejects it.
func ParseOptionalIfExistsMode(raw string, supported ...IfExistsMode) (IfExistsMode, error) {
	if strings.TrimSpace(raw) == "" {
		return IfExistsFail, nil
	}
	return ParseIfExistsMode(raw, supported...)
}

func ifExistsModeNames(supported []IfExistsMode) []string {
	modes := []string{string(IfExistsFail)}
	for _, mode := range []IfExistsMode{IfExistsSkip, IfExistsUpdate} {
		for _, candidate := range supported {
			if candidate == mode {
				modes = append(modes, string(mode))
				break
			}
		}
	}
	return modes
}

// IsIfExistsConflict reports whether err is an HTTP 409 any of whose Apple
// error codes is one the calling command recorded as "already exists". Every
// other 409 (for example an unlisted STATE_ERROR.* or a relationship rejection
// on a command that keys on a duplicate attribute) is not an existence
// conflict.
//
// Apple can report several causes for one 409, and the existence cause is not
// always the first: a duplicate versionString on POST /v1/appStoreVersions
// arrives as ENTITY_ERROR.RELATIONSHIP.INVALID ("You cannot create a new
// version of the App in the current state.") followed by
// ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE ("The version number has been
// previously used."), verified live against app 6759231657 on 2026-09-15. Every
// code in the response is therefore matched, and the command's read-back stays
// the decisive existence check.
func IsIfExistsConflict(err error, existsCodes []string) bool {
	if err == nil || !errors.Is(err, asc.ErrConflict) {
		return false
	}
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok || apiErr == nil {
		return false
	}
	codes := apiErr.AllCodes
	if len(codes) == 0 {
		codes = []string{apiErr.Code}
	}
	for _, code := range codes {
		code = strings.TrimSpace(code)
		for _, candidate := range existsCodes {
			if strings.EqualFold(code, strings.TrimSpace(candidate)) {
				return true
			}
		}
	}
	return false
}

// ResolveIfExistsConflict applies the existence rule for a failed create:
//
//  1. The failure must be an HTTP 409 whose Apple error code is one of
//     existsCodes (see IsIfExistsConflict).
//  2. lookup must read the resource back by its natural key.
//
// Only when both hold is the conflict treated as "already exists" and the
// existing resource returned with handled=true. Every other outcome returns
// createErr unchanged so state conflicts, unrelated 409s, and conflicts whose
// read-back finds nothing keep failing exactly as they do with --if-exists
// fail. A read-back that fails for a reason other than not-found is appended
// to the original conflict so the caller sees both.
func ResolveIfExistsConflict[T any](mode IfExistsMode, createErr error, existsCodes []string, lookup func() (T, bool, error)) (T, bool, error) {
	var zero T
	if createErr == nil {
		return zero, false, nil
	}
	if mode == IfExistsFail || mode == "" || !IsIfExistsConflict(createErr, existsCodes) {
		return zero, false, createErr
	}
	existing, found, err := lookup()
	if err != nil {
		if asc.IsNotFound(err) {
			return zero, false, createErr
		}
		return zero, false, fmt.Errorf("%w (read-back after conflict failed: %w)", createErr, err)
	}
	if !found {
		return zero, false, createErr
	}
	return existing, true, nil
}
