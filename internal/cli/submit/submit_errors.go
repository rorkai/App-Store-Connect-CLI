package submit

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// alreadyAddedPattern matches Apple's error message when a version is already
// in another review submission. The capture group extracts the submission ID.
// Uses \S+ rather than a strict UUID pattern because the API spec defines
// ReviewSubmission.id as a generic string.
var alreadyAddedPattern = regexp.MustCompile(
	`(?i)already added to another reviewSubmission with id\s+(\S+)`,
)

var stillInProgressPattern = regexp.MustCompile(
	`(?i)reviewSubmission[s]?\s+with id\s+(\S+)\s+still in progress`,
)

type submissionConflictKind string

const (
	submissionConflictNone            submissionConflictKind = ""
	submissionConflictAlreadyAttached submissionConflictKind = "already_attached"
	submissionConflictStillInProgress submissionConflictKind = "still_in_progress"
)

type submissionConflict struct {
	Kind         submissionConflictKind
	SubmissionID string
}

type submissionErrorHintContext struct {
	AppID         string
	Platform      string
	VersionID     string
	VersionString string
}

type submissionErrorSignals struct {
	existingSubmissionID string
	activeSubmissionID   string
	versionNotReady      bool
	ageRating            bool
	contentRights        bool
	usesNonExempt        bool
	appDataUsage         bool
	primaryCategory      bool
}

func collectSubmissionErrorSignals(err error) submissionErrorSignals {
	var signals submissionErrorSignals
	if err == nil {
		return signals
	}

	type errorFrame struct {
		err      error
		expanded bool
	}

	stack := []errorFrame{{err: err}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		current := frame.err
		if current == nil {
			continue
		}

		if frame.expanded {
			signals.ingest("", "", current.Error())
			continue
		}

		if apiErr, ok := any(current).(*asc.APIError); ok {
			signals.ingest("", apiErr.Code, apiErr.Detail)
			for path, entries := range apiErr.AssociatedErrors {
				signals.ingest(path, "", "")
				for _, entry := range entries {
					signals.ingest(path, entry.Code, entry.Detail)
				}
			}
		}

		// Process descendants before the wrapper text so structured API detail wins.
		stack = append(stack, errorFrame{err: current, expanded: true})

		type unwrapMany interface {
			Unwrap() []error
		}
		if multi, ok := current.(unwrapMany); ok {
			children := multi.Unwrap()
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, errorFrame{err: children[i]})
			}
			continue
		}
		if next := errors.Unwrap(current); next != nil {
			stack = append(stack, errorFrame{err: next})
		}
	}

	return signals
}

func (signals *submissionErrorSignals) ingest(path, code, detail string) {
	combined := strings.ToLower(strings.Join([]string{path, code, detail}, " "))
	pathLower := strings.ToLower(path)

	if signals.existingSubmissionID == "" {
		if m := alreadyAddedPattern.FindStringSubmatch(detail); len(m) == 2 {
			signals.existingSubmissionID = m[1]
		}
	}
	if signals.activeSubmissionID == "" {
		if m := stillInProgressPattern.FindStringSubmatch(detail); len(m) == 2 {
			signals.activeSubmissionID = m[1]
		}
	}

	if strings.Contains(combined, "ageratingdeclaration") || strings.Contains(pathLower, "ageratingdeclaration") {
		signals.ageRating = true
	}
	if strings.Contains(combined, "contentrightsdeclaration") || strings.Contains(pathLower, "contentrightsdeclaration") {
		signals.contentRights = true
	}
	if strings.Contains(combined, "usesnonexemptencryption") {
		signals.usesNonExempt = true
	}
	if strings.Contains(combined, "appdatausage") || strings.Contains(pathLower, "appdatausages") || strings.Contains(pathLower, "appdatausage") {
		signals.appDataUsage = true
	}
	if strings.Contains(combined, "primarycategory") || strings.Contains(pathLower, "primarycategory") {
		signals.primaryCategory = true
	}
	if strings.Contains(combined, "not ready to be submitted") || strings.Contains(combined, "not in valid state") {
		signals.versionNotReady = true
	}
}

func extractSubmissionConflict(err error) submissionConflict {
	signals := collectSubmissionErrorSignals(err)
	switch {
	case signals.existingSubmissionID != "":
		return submissionConflict{
			Kind:         submissionConflictAlreadyAttached,
			SubmissionID: signals.existingSubmissionID,
		}
	case signals.activeSubmissionID != "":
		return submissionConflict{
			Kind:         submissionConflictStillInProgress,
			SubmissionID: signals.activeSubmissionID,
		}
	default:
		return submissionConflict{}
	}
}

// extractExistingSubmissionID inspects an error returned by AddReviewSubmissionItem
// to see if it indicates the version is already in another review submission.
// If so, it returns the existing submission's ID; otherwise it returns "".
func extractExistingSubmissionID(err error) string {
	conflict := extractSubmissionConflict(err)
	if conflict.Kind != submissionConflictAlreadyAttached {
		return ""
	}
	return conflict.SubmissionID
}

// printSubmissionErrorHints inspects an error returned by App Store Connect
// during submission and prints actionable fix suggestions to stderr.
func printSubmissionErrorHints(err error, ctx submissionErrorHintContext) {
	if err == nil {
		return
	}

	signals := collectSubmissionErrorSignals(err)
	var hints []string
	if signals.ageRating && strings.TrimSpace(ctx.AppID) != "" {
		hints = append(
			hints,
			fmt.Sprintf("Review current age rating: asc age-rating view --app %s", ctx.AppID),
			"Review age-rating update flags: asc age-rating edit --help",
		)
	}
	if signals.contentRights && strings.TrimSpace(ctx.AppID) != "" {
		hints = append(
			hints,
			fmt.Sprintf("If your app does not use third-party content: asc apps update --id %s --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT", ctx.AppID),
			fmt.Sprintf("If your app uses third-party content: asc apps update --id %s --content-rights USES_THIRD_PARTY_CONTENT", ctx.AppID),
		)
	}
	if signals.usesNonExempt {
		hints = append(
			hints,
			"Set Uses Non-Exempt Encryption for the attached build in App Store Connect, then retry submission.",
		)
	}
	if signals.appDataUsage && strings.TrimSpace(ctx.AppID) != "" {
		hints = append(hints, fmt.Sprintf("Complete App Privacy at: https://appstoreconnect.apple.com/apps/%s/appPrivacy", ctx.AppID))
	}
	if signals.primaryCategory {
		hints = append(
			hints,
			"List available categories: asc categories list",
			"Review category update flags: asc app-setup categories set --help",
		)
	}
	if activeID := strings.TrimSpace(signals.activeSubmissionID); activeID != "" {
		hints = appendUniqueHints(
			hints,
			fmt.Sprintf("Check the active submission: asc submit status --id %s", activeID),
			fmt.Sprintf("Inspect the active submission payload: asc review submissions-get --id %s", activeID),
		)
	}
	if strings.TrimSpace(signals.existingSubmissionID) != "" && strings.TrimSpace(signals.activeSubmissionID) == "" {
		hints = appendUniqueHints(
			hints,
			fmt.Sprintf("Inspect the existing submission: asc submit status --id %s", signals.existingSubmissionID),
			fmt.Sprintf("Inspect the existing submission payload: asc review submissions-get --id %s", signals.existingSubmissionID),
		)
	}
	if signals.versionNotReady {
		if strings.TrimSpace(ctx.AppID) != "" && strings.TrimSpace(ctx.VersionID) != "" {
			hints = appendUniqueHints(hints, fmt.Sprintf("Re-run readiness validation: asc validate --app %s --version-id %s", ctx.AppID, ctx.VersionID))
		} else if strings.TrimSpace(ctx.AppID) != "" && strings.TrimSpace(ctx.VersionString) != "" {
			preflightHint := fmt.Sprintf("Re-run readiness validation: asc validate --app %s --version %s", ctx.AppID, ctx.VersionString)
			if strings.TrimSpace(ctx.Platform) != "" {
				preflightHint = fmt.Sprintf("%s --platform %s", preflightHint, strings.TrimSpace(ctx.Platform))
			}
			hints = appendUniqueHints(hints, preflightHint)
		}
		if strings.TrimSpace(ctx.AppID) != "" {
			hints = appendUniqueHints(hints, fmt.Sprintf("Review the release dashboard: asc status --app %s --include submission,appstore,review", ctx.AppID))
		}
	}

	if len(hints) > 0 {
		fmt.Fprintln(os.Stderr, "")
		for _, hint := range hints {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", hint)
		}
	}
}

func appendUniqueHints(hints []string, values ...string) []string {
	seen := make(map[string]struct{}, len(hints))
	for _, hint := range hints {
		seen[hint] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		hints = append(hints, value)
		seen[value] = struct{}{}
	}
	return hints
}

func isExpectedNonCancellableReviewSubmissionError(err error) bool {
	return isResourceStateInvalid(err)
}

// isResourceStateInvalid returns true if the error message indicates the
// resource is not in a cancellable state — an expected condition when racing
// with App Store Connect state transitions.
func isResourceStateInvalid(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Resource is not in cancellable state") ||
		strings.Contains(msg, "Resource state is invalid")
}
