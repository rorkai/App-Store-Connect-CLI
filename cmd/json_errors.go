package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const errorFormatFlagName = "error-format"

func requestedErrorFormat(d *shared.Deps, args []string) string {
	format := "text"
	if !slices.ContainsFunc(args, func(token string) bool {
		name, _ := splitFlagToken(strings.TrimLeft(token, "-"))
		return hasValidFlagPrefix(token) && name == errorFormatFlagName
	}) {
		return format
	}
	flags := newRootCommand("", d, nil).FlagSet
	for i := 0; i < len(args); {
		token := args[i]
		if token == "--" || !hasValidFlagPrefix(token) {
			break
		}
		next, consumed := consumeFlagToken(flags, token, args, i)
		if !consumed {
			i++
			continue
		}
		name, value, inline := strings.Cut(strings.TrimLeft(token, "-"), "=")
		if name == errorFormatFlagName {
			format = value
			if !inline && next == i+2 {
				format = args[i+1]
			}
		} else if !inline && isBoolFlag(flags.Lookup(name)) && next < len(args) {
			if _, err := strconv.ParseBool(strings.TrimSpace(args[next])); err == nil {
				next++
			}
		}
		i = next
	}
	return format
}

func withQuietErrorStream(d *shared.Deps) *shared.Deps {
	deps := *d
	deps.IO = d.IO.WithQuietDiagnostics()
	return &deps
}

type jsonError struct {
	Type              string `json:"type"`
	ExitCode          int    `json:"exit_code"`
	Message           string `json:"message"`
	Hint              string `json:"hint"`
	HTTPStatus        int    `json:"http_status,omitempty"`
	RetryAfterSeconds int64  `json:"retry_after_seconds,omitempty"`
}

func writeJSONError(w io.Writer, exitCode int, failureType string, err error, hint string) {
	failure := jsonError{ExitCode: exitCode, HTTPStatus: httpStatusFromError(err)}
	failure.Type, failure.Message, failure.Hint = safeJSONErrorDetails(exitCode, failureType, failure.HTTPStatus, err)
	if exitCode == ExitUsage && hint != "" {
		failure.Hint = hint
	}
	if delay := asc.GetRetryAfter(err); delay > 0 {
		failure.RetryAfterSeconds = 1 + int64((delay-1)/time.Second)
	}
	// A child can exit after writing a line without a newline.
	_, _ = fmt.Fprintln(w)
	_ = json.NewEncoder(w).Encode(struct {
		Version int       `json:"schema_version"`
		Error   jsonError `json:"error"`
	}{Version: 1, Error: failure})
}

func safeJSONErrorDetails(exitCode int, failureType string, status int, err error) (kind, message, hint string) {
	if _, ok := shared.ProcessExitCode(err); ok {
		return "child_process_failed", "The child process exited unsuccessfully.", "Check the child process logs for its failure."
	}
	if exitCode == ExitUsage {
		switch failureType {
		case "unknown_command":
			return failureType, "The command is not recognized.", "Run asc --help or use asc search to find the command."
		case "invalid_flag":
			return failureType, "A flag or its value is invalid.", "Check the command's --help for supported flags and values."
		}
		if errors.Is(err, shared.ErrMissingWebSession) {
			return "web_session_required", "An Apple web session is required.", "Run asc web auth status to check the selected account."
		}
	}
	if status == http.StatusTooManyRequests {
		return "rate_limited", "The provider rate-limited the request.", "Wait for the reported delay if present and verify the outcome of any write before repeating it."
	}
	switch exitCode {
	case ExitUsage:
		kind, message, hint = "usage_error", "The command input is incomplete or invalid.", "Check the command's --help for required inputs."
	case ExitAuth:
		kind, message, hint = "authentication_required", "Authentication or authorization failed.", "Check the active credentials with asc auth status and verify access to the selected resource."
		if errors.Is(err, appleads.ErrMissingCredentials) || errors.Is(err, appleads.ErrOAuthCredentialsRejected) {
			hint = "Check Apple Ads credentials with asc ads auth status. Verify the ASC_ADS_* settings or configure a profile with asc ads auth login."
		}
	case ExitNotFound:
		kind, message, hint = "resource_not_found", "The requested resource was not found.", "Verify the app and resource selectors."
	case ExitConflict:
		kind, message, hint = "resource_conflict", "The request conflicts with the current resource state.", "Read the current state and review the change before repeating it."
	case ExitReadOnly:
		kind, message, hint = "read_only", "Read-only mode refused a write.", "Use a read command or review and explicitly authorize the write outside read-only mode."
	case ExitPending:
		kind, message, hint = "pending", "The requested state is still pending.", "Read the resource status before starting another operation."
	default:
		if status != 0 {
			kind, message, hint = "request_failed", "The provider rejected the request.", "Check the HTTP status and verify the outcome before repeating any write."
		} else {
			kind, message, hint = "command_failed", "The command could not finish.", "Check the command inputs and verify any reported changes before retrying."
		}
	}
	if diagnostic, ok := shared.DiagnosticFromError(err); ok {
		kind = string(diagnostic.Code)
		switch {
		case diagnostic.Code == shared.DiagnosticFilePermissionsInsecure && diagnostic.Parameter == "--private-key":
			hint = "Set the private key file to owner-only access with chmod 600, then retry."
		case exitCode == ExitNotFound && status == 0 && diagnostic.Code == shared.DiagnosticResourceNotFound && diagnostic.Parameter == "--country":
			hint = "Choose a storefront where the app is released with --country, or use --all with a numeric App Store app ID."
		}
	}
	return kind, message, hint
}
