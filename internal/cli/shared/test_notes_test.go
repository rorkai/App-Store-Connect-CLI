package shared

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNormalizeTestNotesComposesNFCAndDropsRejectedRunes(t *testing.T) {
	// "q" has no precomposed form with U+0301, so NFC leaves the mark behind.
	normalization, err := NormalizeTestNotes("  Cafe\u0301 <b q\u0301  ")
	if err != nil {
		t.Fatalf("NormalizeTestNotes() error = %v", err)
	}
	if normalization.Notes != "Café b q" {
		t.Fatalf("normalized notes = %q, want %q", normalization.Notes, "Café b q")
	}
	if !normalization.Changed || !normalization.RemovedAngleBrackets || !normalization.RemovedCombiningMarks {
		t.Fatalf("normalization flags = %#v, want every flag set", normalization)
	}
	notice := normalization.Notice()
	if !strings.Contains(notice, "normalized") || strings.Count(notice, "\n") != 0 {
		t.Fatalf("notice = %q, want a single-line normalization notice", notice)
	}
	if strings.Contains(notice, "Caf") {
		t.Fatalf("notice echoed the notes: %q", notice)
	}
}

func TestNormalizeTestNotesLeavesAcceptedTextUnchanged(t *testing.T) {
	normalization, err := NormalizeTestNotes("Test the new export flow")
	if err != nil {
		t.Fatalf("NormalizeTestNotes() error = %v", err)
	}
	if normalization.Notes != "Test the new export flow" {
		t.Fatalf("normalized notes = %q, want the input unchanged", normalization.Notes)
	}
	if normalization.Changed || normalization.Notice() != "" {
		t.Fatalf("accepted text reported a change: %#v notice=%q", normalization, normalization.Notice())
	}
}

func TestNormalizeTestNotesIsIdempotent(t *testing.T) {
	first, err := NormalizeTestNotes("Cafe\u0301 <b q\u0301")
	if err != nil {
		t.Fatalf("NormalizeTestNotes() error = %v", err)
	}
	second, err := NormalizeTestNotes(first.Notes)
	if err != nil {
		t.Fatalf("NormalizeTestNotes(normalized) error = %v", err)
	}
	if second.Notes != first.Notes || second.Changed {
		t.Fatalf("second pass = %#v, want %q unchanged", second, first.Notes)
	}
}

func TestNormalizeTestNotesRejectsTextWithNothingLeft(t *testing.T) {
	for _, notes := range []string{"<", " \u0301 ", "<<\u0301\u0301"} {
		normalization, err := NormalizeTestNotes(notes)
		if err == nil {
			t.Fatalf("NormalizeTestNotes(%q) error = nil, want usage failure", notes)
		}
		if normalization.Notes != "" {
			t.Fatalf("NormalizeTestNotes(%q) notes = %q, want empty", notes, normalization.Notes)
		}
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("NormalizeTestNotes(%q) error = %v, want usage-class error", notes, err)
		}
		if got := ClassifyUsageError(err); got != UsageErrorInvalidValue {
			t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorInvalidValue)
		}
	}
}

func TestNormalizeTestNotesForCommandWritesNoticeOnlyWhenTextChanged(t *testing.T) {
	var changedOut bytes.Buffer
	notes, err := NormalizeTestNotesForCommand(&changedOut, "Cafe\u0301 <b q\u0301")
	if err != nil {
		t.Fatalf("NormalizeTestNotesForCommand() error = %v", err)
	}
	if notes != "Café b q" {
		t.Fatalf("notes = %q, want %q", notes, "Café b q")
	}
	if !strings.Contains(changedOut.String(), "normalized") {
		t.Fatalf("diagnostics = %q, want a normalization notice", changedOut.String())
	}

	var unchangedOut bytes.Buffer
	notes, err = NormalizeTestNotesForCommand(&unchangedOut, "Test the new export flow")
	if err != nil {
		t.Fatalf("NormalizeTestNotesForCommand() error = %v", err)
	}
	if notes != "Test the new export flow" {
		t.Fatalf("notes = %q, want the input unchanged", notes)
	}
	if unchangedOut.String() != "" {
		t.Fatalf("diagnostics = %q, want empty output for accepted text", unchangedOut.String())
	}
}

func TestNewTestNotesRecoveryErrorSeparatesHumanAndMachineRecovery(t *testing.T) {
	cause := errors.New("server rejected notes\x1b[31m\nforged line")
	buildID := `build 'quoted' $(touch build)`
	locale := `en-US; touch locale`
	notes := "First line; $(touch notes)\nIt's still quoted\x1b[0m"

	err := NewTestNotesRecoveryError(buildID, locale, notes, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("expected recovery error to wrap cause, got %v", err)
	}
	if asc.HasInterpretedTerminalSequence(err.Error()) {
		t.Fatalf("human recovery error contains terminal controls: %q", err)
	}
	if strings.Contains(err.Error(), "First line") {
		t.Fatalf("human recovery error must not embed notes: %q", err)
	}
	wantHumanParts := []string{
		"retry without uploading the build again",
		"reuse the original notes",
		"asc builds test-notes create --build-id BUILD_ID --locale LOCALE --whats-new NOTES",
	}
	for _, want := range wantHumanParts {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("human recovery error missing %q: %q", want, err)
		}
	}

	recovery := err.Recovery()
	if recovery.BuildID != buildID || recovery.Locale != locale || recovery.SubmittedNotes != notes {
		t.Fatalf("recovery fields lost exact values: %#v", recovery)
	}
	if recovery.Command != "asc" {
		t.Fatalf("recovery command = %q, want asc", recovery.Command)
	}
	want := []string{
		"builds", "test-notes", "create",
		"--build-id", buildID,
		"--locale", locale,
		"--whats-new", notes,
	}
	if len(recovery.Arguments) != len(want) {
		t.Fatalf("retry args = %#v, want %#v", recovery.Arguments, want)
	}
	for i := range want {
		if recovery.Arguments[i] != want[i] {
			t.Fatalf("retry arg %d = %q, want %q; all args=%#v", i, recovery.Arguments[i], want[i], recovery.Arguments)
		}
	}
}

func TestNewTestNotesRecoveryErrorDoesNotEchoNotesFromServerDetail(t *testing.T) {
	notes := "First line\nSecond line\x1b[31m"
	cause := errors.New("server rejected value: " + notes)

	err := NewTestNotesRecoveryError("build-1", "en-US", notes, cause)
	human := err.Error()
	if !strings.Contains(human, "server rejected value: (original notes omitted)") {
		t.Fatalf("human recovery error lost the non-sensitive server diagnostic: %q", human)
	}
	if strings.Contains(human, "First line") || strings.Contains(human, "Second line") || strings.Contains(human, "[31m") {
		t.Fatalf("human recovery error must not echo submitted notes from server detail: %q", human)
	}
}

func TestNewTestNotesRecoveryErrorRedactsExactEchoesWithoutCorruptingDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		notes  string
		cause  string
		redact bool
	}{
		{
			name:   "short exact echo",
			notes:  "invalid",
			cause:  "invalid",
			redact: true,
		},
		{
			name:   "short quoted echo",
			notes:  "invalid",
			cause:  `value "invalid" is not accepted`,
			redact: true,
		},
		{
			name:  "short diagnostic word",
			notes: "invalid",
			cause: "invalid attribute",
		},
		{
			name:  "single-letter substring collision",
			notes: "a",
			cause: "request failed while validating an attribute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTestNotesRecoveryError("build-1", "en-US", tt.notes, errors.New(tt.cause))
			human := err.Error()
			gotRedaction := strings.Contains(human, "(original notes omitted)")
			if gotRedaction != tt.redact {
				t.Fatalf("redaction = %t, want %t: %q", gotRedaction, tt.redact, human)
			}
			if !tt.redact && !strings.Contains(human, tt.cause) {
				t.Fatalf("normal diagnostic was rewritten: %q", human)
			}
		})
	}
}
