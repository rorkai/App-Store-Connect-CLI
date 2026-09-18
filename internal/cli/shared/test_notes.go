package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// UpsertBetaBuildLocalizationOptions carries caller context for a What to Test
// write.
type UpsertBetaBuildLocalizationOptions struct {
	// AppID identifies the build's app. When empty, the app is resolved from
	// the build so the locale's TestFlight app localization can be ensured.
	AppID string
}

// TestNotesNormalization reports the What to Test text that will be stored and
// which characters App Store Connect rejects were dropped to reach it.
type TestNotesNormalization struct {
	Notes                 string
	Changed               bool
	RemovedAngleBrackets  bool
	RemovedCombiningMarks bool
}

// Notice returns a single-line operator notice for a normalization that
// changed the stored text, or an empty string when the text was kept as
// submitted. It never includes the notes themselves.
func (n TestNotesNormalization) Notice() string {
	if !n.Changed {
		return ""
	}
	notice := "Notice: What to Test notes were normalized (Unicode NFC) before sending"
	switch {
	case n.RemovedAngleBrackets && n.RemovedCombiningMarks:
		notice += `; removed "<" characters and combining marks`
	case n.RemovedAngleBrackets:
		notice += `; removed "<" characters`
	case n.RemovedCombiningMarks:
		notice += "; removed combining marks"
	}
	return notice + "."
}

// NormalizeTestNotes rewrites What to Test notes into the form App Store
// Connect accepts: NFC-composed text without "<" characters or leftover
// nonspacing marks. Notes that keep no usable character are a usage failure so
// the caller never sends a request Apple is certain to reject.
func NormalizeTestNotes(notes string) (TestNotesNormalization, error) {
	trimmed := strings.TrimSpace(notes)

	var builder strings.Builder
	builder.Grow(len(trimmed))
	var normalization TestNotesNormalization
	for _, r := range norm.NFC.String(trimmed) {
		switch {
		case r == '<':
			normalization.RemovedAngleBrackets = true
		case unicode.Is(unicode.Mn, r):
			normalization.RemovedCombiningMarks = true
		default:
			builder.WriteRune(r)
		}
	}

	normalized := strings.TrimSpace(builder.String())
	if normalized == "" {
		return TestNotesNormalization{}, classifiedUsageError{
			kind:    UsageErrorInvalidValue,
			message: `What to Test notes are invalid: no text remains after removing "<" characters and combining marks`,
		}
	}

	normalization.Notes = normalized
	normalization.Changed = normalized != trimmed
	return normalization, nil
}

// NormalizeTestNotesForCommand normalizes What to Test notes for a command,
// writing the one-line notice or the usage diagnostic to w. It returns the text
// the caller must submit and store in any retry payload.
func NormalizeTestNotesForCommand(w io.Writer, notes string) (string, error) {
	normalization, err := NormalizeTestNotes(notes)
	if err != nil {
		fmt.Fprintf(w, "Error: %s\n", err.Error())
		return "", err
	}
	if notice := normalization.Notice(); notice != "" {
		fmt.Fprintln(w, notice)
	}
	return normalization.Notes, nil
}

// UpsertBetaBuildLocalization creates or updates a beta build localization,
// first ensuring the app has a TestFlight app localization for the locale.
func UpsertBetaBuildLocalization(ctx context.Context, client *asc.Client, buildID, locale, notes string, opts UpsertBetaBuildLocalizationOptions) (*asc.BetaBuildLocalizationResponse, error) {
	localeValue := strings.TrimSpace(locale)
	notesValue := strings.TrimSpace(notes)
	if localeValue == "" || notesValue == "" {
		return nil, fmt.Errorf("locale and notes are required")
	}

	normalization, err := NormalizeTestNotes(notesValue)
	if err != nil {
		return nil, err
	}
	notesValue = normalization.Notes

	if err := ensureBetaAppLocalization(ctx, client, buildID, opts.AppID, localeValue); err != nil {
		return nil, err
	}

	resp, err := client.GetBetaBuildLocalizations(
		ctx, buildID,
		asc.WithBetaBuildLocalizationsLimit(200),
	)
	if err != nil {
		return nil, err
	}

	localizationID := ""
	foundLocale := false
	if resp != nil {
		for _, localization := range resp.Data {
			if !strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), localeValue) {
				continue
			}
			foundLocale = true
			localizationID = strings.TrimSpace(localization.ID)
			break
		}
	}
	if foundLocale {
		if localizationID == "" {
			return nil, fmt.Errorf("missing localization ID for locale %q", localeValue)
		}
		attrs := asc.BetaBuildLocalizationAttributes{
			WhatsNew: notesValue,
		}
		return client.UpdateBetaBuildLocalization(ctx, localizationID, attrs)
	}

	attrs := asc.BetaBuildLocalizationAttributes{
		Locale:   localeValue,
		WhatsNew: notesValue,
	}
	return client.CreateBetaBuildLocalization(ctx, buildID, attrs)
}

// ensureBetaAppLocalization makes the locale available for TestFlight notes by
// creating the app's beta app localization when it is missing. Existing
// records are left untouched, and a concurrent creator that wins the race
// satisfies the same requirement.
func ensureBetaAppLocalization(ctx context.Context, client *asc.Client, buildID, appID, locale string) error {
	resolvedAppID, err := resolveBetaAppLocalizationAppID(ctx, client, buildID, appID)
	if err != nil {
		return err
	}

	exists, err := hasBetaAppLocalization(ctx, client, resolvedAppID, locale)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Apple requires only the locale and the app relationship here; every
	// other attribute belongs to the operator's App Store Connect content.
	attrs := asc.BetaAppLocalizationAttributes{Locale: locale}
	if _, err := client.CreateBetaAppLocalization(ctx, resolvedAppID, attrs); err != nil {
		if errors.Is(err, asc.ErrConflict) {
			return nil
		}
		return fmt.Errorf("failed to create TestFlight app localization for locale %q: %w", locale, err)
	}
	return nil
}

func resolveBetaAppLocalizationAppID(ctx context.Context, client *asc.Client, buildID, appID string) (string, error) {
	if resolved := strings.TrimSpace(appID); resolved != "" {
		return resolved, nil
	}

	buildIDValue := strings.TrimSpace(buildID)
	appResp, err := client.GetBuildApp(ctx, buildIDValue)
	if err != nil {
		return "", fmt.Errorf("failed to resolve app for build %q: %w", buildIDValue, err)
	}
	resolved := ""
	if appResp != nil {
		resolved = strings.TrimSpace(appResp.Data.ID)
	}
	if resolved == "" {
		return "", fmt.Errorf("failed to resolve app for build %q: empty app response", buildIDValue)
	}
	return resolved, nil
}

func hasBetaAppLocalization(ctx context.Context, client *asc.Client, appID, locale string) (bool, error) {
	firstPage, err := client.GetBetaAppLocalizations(
		ctx,
		asc.WithBetaAppLocalizationAppIDs([]string{appID}),
		asc.WithBetaAppLocalizationsLimit(200),
	)
	if err != nil {
		return false, fmt.Errorf("failed to list TestFlight app localizations for app %q: %w", appID, err)
	}
	if firstPage == nil {
		return false, fmt.Errorf("empty TestFlight app localization response for app %q", appID)
	}
	if containsBetaAppLocalizationLocale(firstPage.Data, locale) {
		return true, nil
	}
	if strings.TrimSpace(firstPage.Links.Next) == "" {
		return false, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextPage, err := client.GetBetaAppLocalizations(pageCtx, asc.WithBetaAppLocalizationsNextURL(nextURL))
		if err != nil {
			return nil, err
		}
		if nextPage == nil {
			return nil, fmt.Errorf("empty TestFlight app localization response for app %q", appID)
		}
		return nextPage, nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to list TestFlight app localizations for app %q: %w", appID, err)
	}
	allPages, ok := paginated.(*asc.BetaAppLocalizationsResponse)
	if !ok {
		return false, fmt.Errorf("unexpected TestFlight app localization pagination response type")
	}
	return containsBetaAppLocalizationLocale(allPages.Data, locale), nil
}

func containsBetaAppLocalizationLocale(localizations []asc.Resource[asc.BetaAppLocalizationAttributes], locale string) bool {
	for _, localization := range localizations {
		if strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), locale) {
			return true
		}
	}
	return false
}

// TestNotesRecoveryError preserves a discovered build and the exact retry
// arguments while keeping its human-facing diagnostic terminal-safe.
type TestNotesRecoveryError struct {
	buildID string
	locale  string
	notes   string
	cause   error
}

// NewTestNotesRecoveryError returns recovery context for a failed post-upload
// What to Test request.
func NewTestNotesRecoveryError(buildID, locale, notes string, cause error) *TestNotesRecoveryError {
	return &TestNotesRecoveryError{
		buildID: buildID,
		locale:  locale,
		notes:   notes,
		cause:   cause,
	}
}

func (e *TestNotesRecoveryError) Error() string {
	buildID := asc.SanitizeTerminalText(e.buildID)
	locale := asc.SanitizeTerminalText(e.locale)
	cause := "unknown error"
	if e.cause != nil {
		cause = redactTestNotes(asc.SanitizeTerminalText(e.cause.Error()), e.notes)
	}
	return fmt.Sprintf(
		"build %q is available, but setting What to Test notes for locale %q failed: %s; retry without uploading the build again and reuse the original notes: asc builds test-notes create --build-id BUILD_ID --locale LOCALE --whats-new NOTES",
		buildID,
		locale,
		cause,
	)
}

func redactTestNotes(message, notes string) string {
	const redacted = "(original notes omitted)"

	sanitizedNotes := asc.SanitizeTerminalText(notes)
	candidate := strings.TrimSpace(sanitizedNotes)
	if candidate == "" {
		return message
	}

	// A cause that consists of the submitted value is unambiguous, even for
	// short notes such as "a" or "invalid".
	if strings.TrimSpace(message) == candidate {
		return redacted
	}

	// Quoted values are also unambiguous. Keep the surrounding quotes so the
	// shape of Apple's diagnostic remains useful to the operator.
	for _, quote := range []string{`"`, `'`, "`"} {
		if strings.Contains(candidate, quote) {
			continue
		}
		message = strings.ReplaceAll(message, quote+candidate+quote, quote+redacted+quote)
	}

	// Terminal-sensitive notes can be echoed after sanitization turns their
	// separators into spaces. Redact only a whole phrase in that case; plain
	// short notes are intentionally not treated as secrets inside arbitrary
	// diagnostic text because there is no reliable way to distinguish an echo
	// from normal prose.
	if asc.HasInterpretedTerminalSequence(notes) && hasTestNotesBoundary(candidate) {
		message = replaceWholeTestNotes(message, candidate, redacted)
	}
	return message
}

func hasTestNotesBoundary(candidate string) bool {
	for _, r := range candidate {
		if unicode.IsSpace(r) || !isTestNotesWordRune(r) {
			return true
		}
	}
	return false
}

func isTestNotesWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_'
}

func replaceWholeTestNotes(message, candidate, replacement string) string {
	for offset := 0; offset < len(message); {
		relativeStart := strings.Index(message[offset:], candidate)
		if relativeStart < 0 {
			break
		}
		start := offset + relativeStart
		end := start + len(candidate)
		if testNotesBoundaryAt(message, start, end) {
			message = message[:start] + replacement + message[end:]
			offset = start + len(replacement)
			continue
		}
		offset = end
	}
	return message
}

func testNotesBoundaryAt(message string, start, end int) bool {
	if start > 0 {
		before, _ := utf8.DecodeLastRuneInString(message[:start])
		if isTestNotesWordRune(before) {
			return false
		}
	}
	if end < len(message) {
		after, _ := utf8.DecodeRuneInString(message[end:])
		if isTestNotesWordRune(after) {
			return false
		}
	}
	return true
}

// Unwrap preserves API error status and exit classification.
func (e *TestNotesRecoveryError) Unwrap() error {
	return e.cause
}

// Recovery returns exact, shell-neutral retry data for structured output.
func (e *TestNotesRecoveryError) Recovery() *asc.TestNotesRecovery {
	return &asc.TestNotesRecovery{
		BuildID:        e.buildID,
		Locale:         e.locale,
		SubmittedNotes: e.notes,
		Command:        "asc",
		Arguments: []string{
			"builds", "test-notes", "create",
			"--build-id", e.buildID,
			"--locale", e.locale,
			"--whats-new", e.notes,
		},
	}
}
