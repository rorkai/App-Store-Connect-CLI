package shared

import (
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// shellSafeWordPattern matches values that are a single literal argument in
// every shell asc supports. Every metacharacter is excluded, including ones
// only one dialect treats specially, such as PowerShell's leading @ splat.
var shellSafeWordPattern = regexp.MustCompile(`^[A-Za-z0-9._/:=+-]+$`)

// windowsUnneutralCharacters are the characters that at least one Windows shell
// reads differently inside a double-quoted argument: $ and ` expand in
// PowerShell and Git Bash, % and ! expand in cmd.exe, and " and \ are escape
// characters in Git Bash but literals in cmd.exe, so a doubled or trailing
// backslash changes the argument or invalidates the command. PowerShell also
// treats the three curly double-quote runes as string delimiters.
const windowsUnneutralCharacters = "\"$`%!\\\u201c\u201d\u201e"

// ShellQuote renders value as one literal argument so a printed command can be
// copied into the user's shell unchanged. ok reports whether that is possible.
// asc cannot know which shell invoked it, so it only advertises a rendering
// that every plausible shell for the platform reads identically:
//
//   - On darwin and linux, single quotes suppress all expansion, and an
//     embedded apostrophe has one escaping that sh, bash, and zsh all accept.
//   - On Windows, double quotes group an argument in PowerShell, cmd.exe, and
//     Git Bash alike, but those shells disagree on single quotes, expansion,
//     and escaping, so a value containing any character one of them treats
//     specially has no neutral rendering.
//
// ok is false for those values, and for control characters, bidi marks, and
// invalid UTF-8, which cannot be both inert in a terminal and byte-exact.
// Callers must then omit the value instead of printing an approximation.
func ShellQuote(value string) (string, bool) {
	return ShellQuoteForOS(value, runtime.GOOS)
}

// ShellQuoteForOS applies ShellQuote's byte-exact rendering contract for the
// requested target platform. It exists for callers that already make an
// explicit platform decision and for deterministic cross-platform tests.
func ShellQuoteForOS(value, goos string) (string, bool) {
	if !utf8.ValidString(value) || asc.HasInterpretedTerminalSequence(value) {
		return "", false
	}
	// zsh expands a bare leading '=' as a command path (for example, =ls), so
	// route it through the platform-specific quoted rendering.
	if shellSafeWordPattern.MatchString(value) && !strings.HasPrefix(value, "=") {
		return value, true
	}
	if goos == "windows" {
		return windowsShellQuote(value)
	}
	return posixShellQuote(value), true
}

// posixShellQuote renders value for sh, bash, and zsh.
func posixShellQuote(value string) string {
	// A POSIX single-quoted string suppresses every expansion but cannot
	// contain an apostrophe, so each one closes the string, escapes itself,
	// and reopens it.
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// windowsShellQuote renders value for whichever shell Windows users run asc
// from, or reports that no neutral rendering exists.
func windowsShellQuote(value string) (string, bool) {
	if strings.ContainsAny(value, windowsUnneutralCharacters) {
		return "", false
	}
	return `"` + value + `"`, true
}
