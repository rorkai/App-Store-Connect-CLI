package errfmt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

type ClassifiedError struct {
	Message string
	Hint    string
}

const (
	requestTimeoutHint = "Increase the request timeout (e.g. set `ASC_TIMEOUT=90s`)."
	uploadTimeoutHint  = "Increase the upload timeout (e.g. set `ASC_UPLOAD_TIMEOUT=45m`)."
	systemStatusHint   = "Check Apple's service health with `asc system-status --service \"App Store Connect\"`."
	bearerTokenHint    = "Set ASC_BEARER_TOKEN to a new token from the service that holds your private key, set ASC_BEARER_TOKEN_COMMAND to a command that prints one, or unset both to use your configured credentials."
	missingAuthHint    = "Run `asc auth status` to see configured credentials. To add an API key, create one at " + auth.APIKeysURL +
		" and run `" + auth.LoginCommandExample + "` (or set ASC_KEY_ID/ASC_ISSUER_ID/ASC_PRIVATE_KEY_PATH)."
	networkHint         = "Check your network connection and proxy settings (HTTPS_PROXY), then retry."
	symlinkedConfigHint = "Replace the symlink with the real file, or set ASC_CONFIG_PATH to the absolute path of a config file whose path contains no symlinks."
)

func Classify(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{}
	}

	// A read-only refusal is a policy decision, not a command failure: render
	// the refusal itself so the line stays stable regardless of which command
	// wrapped it, and add no hint because the message already names the cause.
	if refused, ok := errors.AsType[*readonly.RefusedError](err); ok {
		return ClassifiedError{Message: refused.Error()}
	}

	// A missing Apple web session carries its own next step; the App Store
	// Connect API credential hint below would send the caller to the wrong
	// sign-in.
	if missing, ok := errors.AsType[*shared.MissingWebSessionError](err); ok {
		return ClassifiedError{Message: err.Error(), Hint: missing.Hint}
	}

	// The stored-key hint below does not help a caller whose token was signed
	// elsewhere; only the service that holds the key can mint a new one.
	if errors.Is(err, asc.ErrBearerTokenExpired) || errors.Is(err, asc.ErrInvalidBearerToken) || errors.Is(err, asc.ErrBearerTokenRenewal) {
		return ClassifiedError{Message: err.Error(), Hint: bearerTokenHint}
	}

	if errors.Is(err, config.ErrSymlinkedConfig) {
		return ClassifiedError{Message: err.Error(), Hint: symlinkedConfigHint}
	}

	if errors.Is(err, shared.ErrMissingAuth) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    missingAuthHint,
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		// A wait that already names its --timeout flag needs no second,
		// conflicting knob.
		if strings.Contains(err.Error(), "--timeout") {
			return ClassifiedError{Message: err.Error()}
		}
		hint := requestTimeoutHint
		if isUploadTimeoutError(err) {
			hint = uploadTimeoutHint
		}
		return ClassifiedError{
			Message: err.Error(),
			Hint:    hint,
		}
	}

	if IsNetworkFailure(err) {
		return ClassifiedError{Message: err.Error(), Hint: networkHint}
	}

	var apiErr *asc.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatusCode() >= 500 {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    systemStatusHint,
		}
	}

	if bundleID := appNotFoundBundleID(apiErr); bundleID != "" {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    fmt.Sprintf("App IDs are numeric. Find this app's ID with `asc apps list --bundle-id %q` and pass that instead.", bundleID),
		}
	}

	if containsPrivacyError(err) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "App privacy declarations (data usages) are not available via the public API. Use `asc web privacy pull|plan|apply|publish` or complete App Privacy in the App Store Connect web UI: https://appstoreconnect.apple.com",
		}
	}

	// API-level remediation is already part of the rendered error. Do not add
	// the generic permission hint as well: agreement-blocked 403s are account
	// state, not an API-key role problem, and the two messages conflict.
	var remediationErr *asc.APIError
	if errors.As(err, &remediationErr) && strings.TrimSpace(remediationErr.Remediation) != "" {
		return ClassifiedError{Message: err.Error()}
	}

	if errors.Is(err, asc.ErrForbidden) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "Check that your API key has the right role/permissions for this operation in App Store Connect.",
		}
	}

	if errors.Is(err, asc.ErrUnauthorized) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "Your credentials may be invalid or expired. Try `asc auth status` and re-login if needed.",
		}
	}

	return ClassifiedError{
		Message: err.Error(),
		Hint:    "",
	}
}

// IsNetworkFailure reports whether err is a failure to reach a server (dial,
// DNS, TLS, proxy, or a dropped connection) rather than a server response or
// local work. It matches the transport error types, never a bare io.EOF, so a
// truncated local file is not mistaken for a network problem.
func IsNetworkFailure(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	_, isURLError := errors.AsType[*url.Error](err)
	_, isTransportError := errors.AsType[*urlsanitize.TransportError](err)
	_, isOpError := errors.AsType[*net.OpError](err)
	return isURLError || isTransportError || isOpError
}

// appNotFoundIDPattern matches Apple's 404 detail for an unknown app, such as
// "There is no resource of type 'apps' with id 'com.example.app'". Many
// commands put --app straight into /v1/apps/{id}, so a bundle ID there ends
// in this error instead of a lookup.
var (
	appNotFoundIDPattern = regexp.MustCompile(`no resource of type 'apps' with id '([^']*)'`)
	bundleIDPattern      = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
)

func appNotFoundBundleID(apiErr *asc.APIError) string {
	if apiErr == nil || apiErr.HTTPStatusCode() != http.StatusNotFound {
		return ""
	}
	match := appNotFoundIDPattern.FindStringSubmatch(apiErr.Detail)
	if match == nil || !bundleIDPattern.MatchString(match[1]) {
		return ""
	}
	return match[1]
}

func isUploadTimeoutError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "upload failed") ||
		strings.Contains(msg, "upload operation") ||
		strings.Contains(msg, "multipart upload") ||
		strings.Contains(msg, "s3 upload")
}

// containsPrivacyError checks whether the error references app data usage /
// privacy declaration resources that are not manageable via the API.
func containsPrivacyError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "appdatausages") || strings.Contains(msg, "appdatausagespublications")
}

const submissionStateRead = "`asc review submissions-get --id SUBMISSION_ID --include items`"

// conflictNextSteps names, per command path, the read command that shows the
// state an App Store Connect 409 conflicted with, so a caller inspects that
// state instead of repeating the same request.
var conflictNextSteps = map[string]string{
	"asc review items add":          "Check the submission's state and items with " + submissionStateRead + ". If the item is already on it, `--if-exists skip` accepts it.",
	"asc review items-add":          "Check the submission's state and items with " + submissionStateRead + ". If the item is already on it, `--if-exists skip` accepts it.",
	"asc review submissions-submit": "Check the submission's state and items with " + submissionStateRead + ", and what blocks review with `asc review doctor --app APP_ID`.",
	"asc review submit":             "Check the app's review state and blockers with `asc review status --app APP_ID` and `asc review doctor --app APP_ID`.",
	"asc localizations update":      "Check the version's state with `asc versions view --version-id VERSION_ID`, or for app-info localizations `asc localizations list --app APP_ID --type app-info --app-info-fields state`.",
	"asc testflight testers add":    "Check the tester's current state with `asc testflight testers list --app APP_ID --email EMAIL`.",
	"asc testflight testers invite": "Check the tester's current state with `asc testflight testers list --app APP_ID --email EMAIL`.",
}

func FormatStderr(err error) string {
	return FormatCommandStderr("", err)
}

// FormatCommandStderr is FormatStderr for an error returned by the command at
// path, such as "asc review items add". An HTTP 409 that gets no other hint
// gets that command's conflictNextSteps entry.
func FormatCommandStderr(path string, err error) string {
	ce := Classify(err)
	if ce.Hint == "" && errors.Is(err, asc.ErrConflict) {
		ce.Hint = conflictNextSteps[path]
	}
	if ce.Message == "" {
		return ""
	}
	if ce.Hint == "" {
		return fmt.Sprintf("Error: %s\n", ce.Message)
	}
	return fmt.Sprintf("Error: %s\nHint: %s\n", ce.Message, ce.Hint)
}
