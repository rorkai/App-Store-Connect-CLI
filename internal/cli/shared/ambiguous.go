package shared

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// AmbiguousCandidateLimit bounds how many candidates an ambiguity error lists.
// Remaining candidates are summarized as "... and N more".
const AmbiguousCandidateLimit = 10

// AmbiguousDiagnosticTextLimit bounds each provider-supplied text field in a
// terminal ambiguity diagnostic. The limit is measured in bytes, with the
// truncation boundary adjusted to preserve valid UTF-8.
const AmbiguousDiagnosticTextLimit = 256

const ambiguousDiagnosticTextTruncationMarker = "..."

// AmbiguousCandidate is one resource that matched a selector which should
// have matched exactly one. ID is the value the disambiguating flag accepts;
// Label and Extra are optional human context (name, platform, state).
type AmbiguousCandidate struct {
	ID    string
	Label string
	Extra string
}

// AmbiguousSelectionError reports that a selector matched several resources.
// The rendered message names every candidate (bounded by
// AmbiguousCandidateLimit) with its ID and the flag that disambiguates, so an
// agent can recover by re-running with one of the listed IDs.
type AmbiguousSelectionError struct {
	// Kind is the singular resource noun, for example "app" or "beta group".
	Kind string
	// Description says what was matched, already quoted or qualified, for
	// example `"Outslept"` or `version "1.2.3" on platform "IOS"`.
	Description string
	// Flag is the flag that accepts one candidate ID, for example "--app".
	// Leave empty when no single flag disambiguates.
	Flag string
	// Candidates lists the matches in the order they should be shown.
	Candidates []AmbiguousCandidate
	// CandidatesAreSample reports that the candidates prove ambiguity but do
	// not establish the total match count. This keeps early-stopping resolvers
	// from presenting the retained sample as a complete result set.
	CandidatesAreSample bool
	// Hint is an optional final line with additional guidance.
	Hint string
	// DisplayTextLimit bounds each rendered field after terminal sanitization.
	// Zero preserves the historical unbounded rendering. Structured fields
	// always retain their complete values.
	DisplayTextLimit int
}

// AmbiguousError builds an AmbiguousSelectionError for a selector value that
// matched several resources of kind; selector is quoted in the message.
func AmbiguousError(kind, flag, selector string, candidates []AmbiguousCandidate) error {
	return &AmbiguousSelectionError{
		Kind:        kind,
		Description: fmt.Sprintf("%q", selector),
		Flag:        flag,
		Candidates:  candidates,
	}
}

// ValidationFailure reports ambiguity as a validation outcome: the request
// was understood but needs a disambiguating selector, which is not a defect in
// asc. The message and exit code are unchanged.
func (e *AmbiguousSelectionError) ValidationFailure() bool { return true }

// Error renders the header, the bounded candidate table, and the hint.
func (e *AmbiguousSelectionError) Error() string {
	var b strings.Builder
	if e.CandidatesAreSample {
		fmt.Fprintf(&b, "multiple %s match %s", pluralizeKind(e.Kind), sanitizeAmbiguousText(e.Description, e.DisplayTextLimit))
	} else {
		fmt.Fprintf(&b, "%d %s match %s", len(e.Candidates), pluralizeKind(e.Kind), sanitizeAmbiguousText(e.Description, e.DisplayTextLimit))
	}
	if flag := strings.TrimSpace(e.Flag); flag != "" {
		if e.CandidatesAreSample {
			fmt.Fprintf(&b, "; pass %s with one of these sample matches:", flag)
		} else {
			fmt.Fprintf(&b, "; pass %s with one of:", flag)
		}
	} else {
		if e.CandidatesAreSample {
			b.WriteString("; these are sample matches:")
		} else {
			b.WriteString(":")
		}
	}

	shown := e.Candidates
	if len(shown) > AmbiguousCandidateLimit {
		shown = shown[:AmbiguousCandidateLimit]
	}
	rows := make([][3]string, 0, len(shown))
	idWidth, labelWidth := 0, 0
	for _, candidate := range shown {
		row := [3]string{
			sanitizeAmbiguousText(candidate.ID, e.DisplayTextLimit),
			sanitizeAmbiguousText(candidate.Label, e.DisplayTextLimit),
			sanitizeAmbiguousText(candidate.Extra, e.DisplayTextLimit),
		}
		if row[0] == "" {
			row[0] = "<no id>"
		}
		idWidth = max(idWidth, len(row[0]))
		labelWidth = max(labelWidth, len(row[1]))
		rows = append(rows, row)
	}
	for _, row := range rows {
		b.WriteString("\n  ")
		switch {
		case row[2] != "":
			fmt.Fprintf(&b, "%-*s  %-*s  %s", idWidth, row[0], labelWidth, row[1], row[2])
		case row[1] != "":
			fmt.Fprintf(&b, "%-*s  %s", idWidth, row[0], row[1])
		default:
			b.WriteString(row[0])
		}
	}
	if remaining := len(e.Candidates) - len(shown); remaining > 0 {
		fmt.Fprintf(&b, "\n  ... and %d more", remaining)
	}
	if hint := sanitizeAmbiguousText(e.Hint, e.DisplayTextLimit); hint != "" {
		b.WriteString("\n")
		b.WriteString(hint)
	}
	return b.String()
}

// IsAmbiguousSelection reports whether err is, or wraps, an ambiguous
// selection error. Callers that classify local selector-resolution failures
// should use this instead of matching on message text.
func IsAmbiguousSelection(err error) bool {
	var ambiguous *AmbiguousSelectionError
	return errors.As(err, &ambiguous)
}

// AmbiguousUsageError prints an ambiguity error to stderr line by line and
// returns a usage-class error (exit code 2). UsageError cannot be used for
// this because its terminal sanitizer flattens the candidate table onto one
// line.
func AmbiguousUsageError(err error) error {
	return AmbiguousUsageErrorWithKind(err, UsageErrorOther)
}

// AmbiguousUsageErrorWithKind prints an ambiguity error to stderr line by
// line and returns a usage-class error (exit code 2) with the requested
// telemetry classification. The default AmbiguousUsageError intentionally
// remains UsageErrorOther for callers whose ambiguity is not a missing or
// invalid flag.
func AmbiguousUsageErrorWithKind(err error, kind UsageErrorKind) error {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	if message != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	}
	// The full table stays on the error so wrapped callers and tests can read
	// the candidates; cmd does not reprint usage-class errors, so this cannot
	// duplicate the stderr output above.
	return NewErrorWithCause(
		classifiedUsageError{kind: kind, message: message},
		err,
	)
}

// MarkAmbiguousSelectionSample marks an ambiguity whose candidates came from
// a response that advertises another page. The returned error keeps its
// concrete type and wrapping chain so callers can still inspect the
// candidates and classify the failure.
func MarkAmbiguousSelectionSample(err error) error {
	if err == nil {
		return nil
	}
	var ambiguous *AmbiguousSelectionError
	if errors.As(err, &ambiguous) {
		ambiguous.CandidatesAreSample = true
	}
	return err
}

func sanitizeAmbiguousText(value string, limit ...int) string {
	value = strings.TrimSpace(SanitizeTerminal(strings.TrimSpace(value)))
	if len(limit) == 0 || limit[0] <= 0 || len(value) <= limit[0] {
		return value
	}
	return truncateAmbiguousDiagnosticText(value, limit[0])
}

func truncateAmbiguousDiagnosticText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.ToValidUTF8(value, "\uFFFD")
	if len(value) <= limit {
		return value
	}
	marker := ambiguousDiagnosticTextTruncationMarker
	if len(marker) >= limit {
		return utf8SafePrefix(value, limit)
	}
	prefixLimit := limit - len(marker)
	for prefixLimit > 0 && !utf8.ValidString(value[:prefixLimit]) {
		prefixLimit--
	}
	return value[:prefixLimit] + marker
}

func utf8SafePrefix(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit]
}

func pluralizeKind(kind string) string {
	kind = strings.TrimSpace(kind)
	switch {
	case kind == "":
		return "resources"
	case strings.HasSuffix(kind, "s"), strings.HasSuffix(kind, "x"), strings.HasSuffix(kind, "ch"), strings.HasSuffix(kind, "sh"):
		return kind + "es"
	case strings.HasSuffix(kind, "y") && len(kind) > 1 && !strings.ContainsRune("aeiou", rune(kind[len(kind)-2])):
		return kind[:len(kind)-1] + "ies"
	default:
		return kind + "s"
	}
}
