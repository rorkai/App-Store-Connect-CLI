package shared

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

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
	if !strings.Contains(notice, `removed "<" characters and uncomposable combining diacritics, which App Store Connect rejects`) {
		t.Fatalf("notice = %q, want it to name the removed characters and why", notice)
	}
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

func TestNormalizeTestNotesPreservesScriptMarks(t *testing.T) {
	// App Store Connect accepts these marks (verified live on a TestFlight
	// localization text field); removing them corrupts the text.
	tests := []struct {
		name  string
		notes string
	}{
		{name: "hindi", notes: "हिंदी में परीक्षण करें"},
		{name: "thai", notes: "ที่นี่ ทดสอบ"},
		{name: "hebrew niqqud", notes: "שָׁלוֹם"},
		{name: "arabic harakat", notes: "مَرْحَبًا"},
		{name: "cyrillic titlo", notes: "а҃ ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalization, err := NormalizeTestNotes(tt.notes)
			if err != nil {
				t.Fatalf("NormalizeTestNotes(%q) error = %v", tt.notes, err)
			}
			want := norm.NFC.String(tt.notes)
			if normalization.Notes != want {
				t.Fatalf("NormalizeTestNotes(%q) = %q, want %q", tt.notes, normalization.Notes, want)
			}
			if normalization.RemovedCombiningMarks {
				t.Fatalf("NormalizeTestNotes(%q) reported removed combining marks", tt.notes)
			}
		})
	}
}

func TestNormalizeTestNotesDropsGenericCombiningDiacritics(t *testing.T) {
	// Marks from the generic combining-diacritic blocks that survive NFC are
	// rejected by App Store Connect ("Text contains invalid characters/formats").
	tests := []struct {
		notes string
		want  string
	}{
		{notes: "q́ test", want: "q test"},
		{notes: "a᪰ b", want: "a b"},
		{notes: "x⃗ vec", want: "x vec"},
		{notes: "ok᷀ go", want: "ok go"},
		{notes: "ok︠ go", want: "ok go"},
	}
	for _, tt := range tests {
		normalization, err := NormalizeTestNotes(tt.notes)
		if err != nil {
			t.Fatalf("NormalizeTestNotes(%q) error = %v", tt.notes, err)
		}
		if normalization.Notes != tt.want || !normalization.RemovedCombiningMarks {
			t.Fatalf("NormalizeTestNotes(%q) = %#v, want %q with RemovedCombiningMarks", tt.notes, normalization, tt.want)
		}
	}
}

func TestNormalizeTestNotesRejectsCharactersAppStoreConnectRefuses(t *testing.T) {
	// Each input was rejected live by App Store Connect in whatsNew with "Text
	// for whatsNew contains invalid characters" (see docs/API_NOTES.md).
	tests := []struct {
		name  string
		notes string
		want  []string
	}{
		{name: "heart", notes: "Love \u2764", want: []string{"U+2764 \"\u2764\""}},
		{name: "heart with VS16", notes: "Love \u2764\ufe0f", want: []string{"U+2764 \"\u2764\"", "U+FE0F"}},
		{name: "text VS15 after a letter", notes: "a\ufe0e", want: []string{"U+FE0E"}},
		{name: "grinning face", notes: "Smile \U0001F600", want: []string{"U+1F600"}},
		{name: "check mark button", notes: "Done \u2705", want: []string{"U+2705"}},
		{name: "check mark dingbat", notes: "Done \u2713", want: []string{"U+2713"}},
		{name: "black star", notes: "Rate \u2605", want: []string{"U+2605"}},
		{name: "box drawing", notes: "a \u2500 b", want: []string{"U+2500"}},
		{name: "block element", notes: "a \u2588 b", want: []string{"U+2588"}},
		{name: "misc symbols and arrows", notes: "Up \u2b06", want: []string{"U+2B06"}},
		{name: "braille", notes: "a \u2801", want: []string{"U+2801"}},
		{name: "keycap", notes: "Press 1\ufe0f\u20e3", want: []string{"U+FE0F", "U+20E3"}},
		{name: "zwj family", notes: "\U0001F468\u200d\U0001F469\u200d\U0001F467 ok", want: []string{"U+1F468", "U+1F469", "U+1F467"}},
		{name: "flag", notes: "\U0001F1FA\U0001F1F8 ok", want: []string{"U+1F1FA", "U+1F1F8"}},
		{name: "supplementary letter", notes: "Bold \U0001D400", want: []string{"U+1D400"}},
		{name: "private use", notes: "Logo \uf8ff", want: []string{"U+F8FF"}},
		{name: "replacement character", notes: "Broken \ufffd", want: []string{"U+FFFD"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalization, err := NormalizeTestNotes(tt.notes)
			if err == nil {
				t.Fatalf("NormalizeTestNotes(%q) = %#v, want usage failure", tt.notes, normalization)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("NormalizeTestNotes(%q) error = %v, want usage-class error", tt.notes, err)
			}
			if got := ClassifyUsageError(err); got != UsageErrorInvalidValue {
				t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorInvalidValue)
			}
			message := err.Error()
			if !strings.Contains(message, "What to Test notes") {
				t.Fatalf("error = %q, want it to name the notes", message)
			}
			for _, want := range tt.want {
				if !strings.Contains(message, want) {
					t.Fatalf("error = %q, want it to contain %q", message, want)
				}
			}
			if strings.Count(message, "U+") != len(tt.want) {
				t.Fatalf("error = %q, want exactly the distinct offending characters %v", message, tt.want)
			}
			if strings.Contains(message, "\n") {
				t.Fatalf("error = %q, want a single line", message)
			}
		})
	}
}

func TestNormalizeTestNotesRejectedCharacterListIsBounded(t *testing.T) {
	var notes strings.Builder
	for r := rune(0x1F600); r < 0x1F600+20; r++ {
		notes.WriteRune(r)
		notes.WriteRune(r)
	}
	_, err := NormalizeTestNotes(notes.String())
	if err == nil {
		t.Fatal("NormalizeTestNotes() error = nil, want usage failure")
	}
	message := err.Error()
	if got := strings.Count(message, "U+"); got != maxReportedTestNotesRunes {
		t.Fatalf("error = %q, want %d listed characters, got %d", message, maxReportedTestNotesRunes, got)
	}
	if !strings.Contains(message, "and 12 more") {
		t.Fatalf("error = %q, want it to count the unlisted characters", message)
	}
}

func TestNormalizeTestNotesAcceptsCharactersAppStoreConnectAllows(t *testing.T) {
	// Each input was accepted live by App Store Connect in whatsNew, including
	// symbols that have emoji presentations in other blocks.
	for _, notes := range []string{
		"Next \u2192 back \u2194 up \u21ff",
		"Brand\u2122 \u00a9 \u00ae \u2139 \u2116",
		"Keys \u2318 \u231a \u231b \u23e9 \u23ff",
		"Items \u2022 \u2026 \u2014 \u20ac \u2264 \u22ff",
		"Shapes \u25a0 \u25aa \u25b6 \u25cf \u25ff \u2460 \u24c2 \u24ff",
		"Punct \u203c \u2049 \u3030 \u303d \u3297 \u3299 \u2934 \u27f6",
		"Line one\nLine two\r\nTab\there",
		"Invisible a\u00a0b a\u00adb a\u200bb a\u200eb",
		"\u0915\u094d\u200d\u0937 \u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645",
		"\u65e5\u672c\u8a9e \ud55c\uad6d\uc5b4 \uff01 \uff76 \uf900",
		"Roll the die and check the oracle. \u0939\u093f\u0902\u0926\u0940 \u092e\u0947\u0902 \u092a\u0930\u0940\u0915\u094d\u0937\u0923 \u0915\u0930\u0947\u0902",
		"a > b & c #1",
	} {
		normalization, err := NormalizeTestNotes(notes)
		if err != nil {
			t.Fatalf("NormalizeTestNotes(%q) error = %v, want accepted", notes, err)
		}
		if want := norm.NFC.String(strings.TrimSpace(notes)); normalization.Notes != want {
			t.Fatalf("NormalizeTestNotes(%q) = %q, want %q", notes, normalization.Notes, want)
		}
	}
}
