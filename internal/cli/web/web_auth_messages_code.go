package web

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	messagesCodeDefaultSince = time.Minute
	// Apple verification codes expire within minutes, and a short window keeps
	// every candidate inside the newest-rows query limit.
	messagesCodeMaxSince = 10 * time.Minute
	// Allow for clock skew between asc's request and Messages' receipt time.
	messagesCodeRequestSkew = 2 * time.Second
	// The login flow kills its 2FA code command after webTwoFactorCommandTimeout,
	// so the default poll budget stays below it to surface this command's own,
	// clearer timeout error instead of a generic interruption.
	messagesCodeDefaultTimeout = 50 * time.Second
	messagesCodeRowLimit       = 50
	// Modern Messages stores message.date as nanoseconds since 2001-01-01 UTC;
	// older databases stored seconds. Second-based values stay below this.
	messagesSecondsThreshold = int64(100_000_000_000)
	messagesSystemSQLite     = "/usr/bin/sqlite3"
)

var (
	messagesAppleEpoch = time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)

	messagesCodeGOOSFn                     = func() string { return runtime.GOOS }
	messagesCodeQueryFn                    = runMessagesSQLiteQuery
	messagesCodePollInterval time.Duration = 2 * time.Second

	appleDomainBoundCodePattern   = regexp.MustCompile(`(?i)@apple\.com\s+#(\d{6})\b`)
	appleVerificationCodePattern  = regexp.MustCompile(`(?i)\bapple\s+(?:account|id)\s+code\s+is:?\s*(\d{6})\b`)
	foreignDomainBoundCodePattern = regexp.MustCompile(`@[A-Za-z0-9.-]+\s+#\d{6}\b`)
	typedstreamNSStringMarker     = []byte("NSString")
)

// WebAuthMessagesCodeCommand reads a fresh Apple verification code from the
// local macOS Messages database so it can serve as a 2FA code command.
func WebAuthMessagesCodeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth messages-code", flag.ExitOnError)

	dbPath := fs.String("db", "", "Path to the Messages database (default ~/Library/Messages/chat.db)")
	since := fs.Duration("since", messagesCodeDefaultSince, "Only accept messages received within this lookback from when the command starts (at most 10m)")
	timeout := fs.Duration("timeout", messagesCodeDefaultTimeout, "How long to wait for a matching message before failing")

	return &ffcli.Command{
		Name:       "messages-code",
		ShortUsage: "asc web auth messages-code [--since DURATION] [--timeout DURATION] [--db PATH]",
		ShortHelp:  "Read a fresh Apple 2FA code from macOS Messages.",
		LongHelp: `WEB SESSION WORKFLOWS

Read the newest Apple verification code that reached this Mac through the
Messages app and print it, so it can be used as the 2FA code command for
"asc web auth login".

Output contract:
  - stdout contains ONLY the 6-digit code followed by a newline. This plain-text
    output is intentional: it matches the --two-factor-code-command contract,
    so --output is not supported.
  - the command never prints message contents, and never writes the code to
    stderr or logs.

How it works:
  - macOS only.
  - opens ~/Library/Messages/chat.db (or --db) strictly read-only through the
    system /usr/bin/sqlite3.
  - accepts only incoming Apple verification messages (for example
    "Your Apple Account code is: 123456 ... @apple.com #123456") received within
    --since (at most 10m) of when the command starts, so stale codes are never
    returned. When "asc web auth login" runs it, only codes received after asc
    requested the current code are accepted (from ASC_WEB_2FA_CODE_REQUESTED_AT);
    an explicit --since overrides that.
  - polls until a matching message arrives or --timeout elapses. The login
    flow stops its 2FA code command after 60s, so keep --timeout below that.

Setup:
  - grant Full Disk Access to the app that runs asc (Terminal, iTerm, your
    agent host) in System Settings > Privacy & Security > Full Disk Access.
  - SMS codes reach the Mac only when the iPhone forwards them: enable
    Settings > Messages > Text Message Forwarding on the iPhone.
  - accounts with trusted devices get an SMS only when the login asks for one:
    pass --two-factor-method sms to "asc web auth login".

Examples:
  asc web auth login --apple-id "user@example.com" --two-factor-method sms --two-factor-code-command "asc web auth messages-code"
  asc web auth messages-code --since 2m --timeout 45s`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *since <= 0 {
				return shared.UsageError("--since must be a positive duration")
			}
			if *since > messagesCodeMaxSince {
				return shared.UsageErrorf("--since must be at most %s", messagesCodeMaxSince)
			}
			if *timeout <= 0 {
				return shared.UsageError("--timeout must be a positive duration")
			}
			dbFlagSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "db" {
					dbFlagSet = true
				}
			})
			if dbFlagSet && strings.TrimSpace(*dbPath) == "" {
				return shared.UsageError("--db must not be empty")
			}

			if goos := messagesCodeGOOSFn(); goos != "darwin" {
				return fmt.Errorf("web auth messages-code: only supported on macOS (current OS: %s)", goos)
			}

			resolvedDB, err := resolveMessagesDBPath(*dbPath)
			if err != nil {
				return fmt.Errorf("web auth messages-code: %w", err)
			}

			sinceSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "since" {
					sinceSet = true
				}
			})
			start := time.Now()
			cutoff := start.Add(-*since)
			window := fmt.Sprintf("received up to %s before start", *since)
			// Run by "asc web auth login", accept only codes sent after the
			// current request so a code from an earlier attempt is never reused.
			if raw := strings.TrimSpace(os.Getenv(webTwoFactorCodeRequestedAtEnv)); raw != "" && !sinceSet {
				requestedAt, err := time.Parse(time.RFC3339Nano, raw)
				if err != nil {
					return fmt.Errorf("web auth messages-code: %s must be an RFC 3339 timestamp", webTwoFactorCodeRequestedAtEnv)
				}
				cutoff = requestedAt.Add(-messagesCodeRequestSkew)
				if oldest := start.Add(-messagesCodeMaxSince); cutoff.Before(oldest) {
					cutoff = oldest
				}
				window = "received since asc requested the code"
			}
			code, err := waitForAppleMessagesCode(ctx, resolvedDB, cutoff, window, *timeout)
			if err != nil {
				return fmt.Errorf("web auth messages-code: %w", err)
			}
			_, _ = fmt.Fprintln(os.Stdout, code)
			return nil
		},
	}
}

func resolveMessagesDBPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory for the Messages database: %w", err)
		}
		path = filepath.Join(home, "Library", "Messages", "chat.db")
	} else if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory for --db: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve Messages database path %q: %w", path, err)
	}

	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("no Messages database found at %s; use --db to point at chat.db, and make sure Messages is set up on this Mac", abs)
	case errors.Is(err, fs.ErrPermission):
		return "", messagesPrivacyError(abs, err.Error())
	case err != nil:
		return "", fmt.Errorf("inspect Messages database %s: %w", abs, err)
	case info.IsDir():
		return "", fmt.Errorf("the Messages database path %s is a directory; --db must point at chat.db", abs)
	}
	return abs, nil
}

func waitForAppleMessagesCode(ctx context.Context, dbPath string, cutoff time.Time, window string, timeout time.Duration) (string, error) {
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	query := messagesCodeQuery(cutoff)
	timeoutErr := fmt.Errorf("no Apple verification code found in Messages within %s (accepting messages %s); check that the code was sent by SMS and that iPhone Text Message Forwarding is enabled for this Mac", timeout, window)

	for {
		out, err := messagesCodeQueryFn(pollCtx, dbPath, query)
		if err != nil {
			if pollCtx.Err() != nil && ctx.Err() == nil {
				return "", timeoutErr
			}
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if isMessagesPrivacyError(err.Error()) {
				return "", messagesPrivacyError(dbPath, err.Error())
			}
			return "", fmt.Errorf("read Messages database %s: %w", dbPath, err)
		}
		rows, err := parseMessagesRows(out)
		if err != nil {
			return "", err
		}
		if code, ok := selectAppleCode(rows, cutoff); ok {
			return code, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-pollCtx.Done():
			return "", timeoutErr
		case <-time.After(messagesCodePollInterval):
		}
	}
}

func isMessagesPrivacyError(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range []string{
		"unable to open database file",
		"authorization denied",
		"operation not permitted",
		"not authorized",
		"permission denied",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func messagesPrivacyError(dbPath, detail string) error {
	return fmt.Errorf(
		"cannot read Messages database %s (%s). macOS privacy protection likely blocked access: grant Full Disk Access to the app that runs asc (Terminal, iTerm, or your agent host) in System Settings > Privacy & Security > Full Disk Access, then restart that app. SMS codes reach this Mac only when iPhone Text Message Forwarding is enabled (iPhone Settings > Messages > Text Message Forwarding)",
		dbPath,
		strings.TrimSpace(detail),
	)
}

// messagesCodeQuery builds the read-only query. Only integer cutoffs are
// interpolated; text columns are hex-encoded so the list output stays
// delimiter-safe regardless of message contents or sqlite3 version.
func messagesCodeQuery(cutoff time.Time) string {
	nanos := cutoff.Sub(messagesAppleEpoch).Nanoseconds()
	seconds := nanos / int64(time.Second)
	return fmt.Sprintf(
		"SELECT ROWID, date, hex(text), hex(attributedBody) FROM message "+
			"WHERE is_from_me = 0 AND (date >= %d OR (date < %d AND date >= %d)) "+
			// Filter for Apple before the row limit, so busy chats cannot
			// push a code out of the newest rows.
			"AND (text LIKE '%%apple%%' OR instr(attributedBody, CAST('Apple' AS BLOB)) > 0 OR instr(attributedBody, CAST('apple' AS BLOB)) > 0) "+
			"ORDER BY date DESC, ROWID DESC LIMIT %d;",
		nanos, messagesSecondsThreshold, seconds, messagesCodeRowLimit,
	)
}

func runMessagesSQLiteQuery(ctx context.Context, dbPath, query string) ([]byte, error) {
	sqlitePath, err := lookupMessagesSQLite()
	if err != nil {
		return nil, err
	}
	uri := (&url.URL{Scheme: "file", Path: dbPath, RawQuery: "mode=ro"}).String()
	cmd := exec.CommandContext(ctx, sqlitePath,
		"-readonly",
		"-batch",
		"-noheader",
		"-list",
		"-separator", "|",
		"-cmd", ".timeout 2000",
		uri,
		query,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return nil, fmt.Errorf("sqlite3 failed: %s", detail)
		}
		return nil, fmt.Errorf("sqlite3 failed: %w", err)
	}
	return out, nil
}

func lookupMessagesSQLite() (string, error) {
	if info, err := os.Stat(messagesSystemSQLite); err == nil && !info.IsDir() {
		return messagesSystemSQLite, nil
	}
	path, err := exec.LookPath("sqlite3")
	if err != nil {
		return "", fmt.Errorf("sqlite3 not found (expected %s): %w", messagesSystemSQLite, err)
	}
	return path, nil
}

type messagesRow struct {
	RowID          int64
	Date           int64
	Text           string
	AttributedBody []byte
}

// parseMessagesRows parses "ROWID|date|hex(text)|hex(attributedBody)" lines.
// Errors never echo database output because it contains message contents.
func parseMessagesRows(out []byte) ([]messagesRow, error) {
	var rows []messagesRow
	for i, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 4 {
			return nil, fmt.Errorf("unexpected sqlite3 output on line %d: expected 4 columns, got %d", i+1, len(fields))
		}
		rowID, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected sqlite3 output on line %d: invalid ROWID", i+1)
		}
		date, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected sqlite3 output on line %d: invalid date", i+1)
		}
		text, err := hex.DecodeString(fields[2])
		if err != nil {
			return nil, fmt.Errorf("unexpected sqlite3 output on line %d: invalid text encoding", i+1)
		}
		var attributed []byte
		if fields[3] != "" {
			attributed, err = hex.DecodeString(fields[3])
			if err != nil {
				return nil, fmt.Errorf("unexpected sqlite3 output on line %d: invalid attributedBody encoding", i+1)
			}
		}
		rows = append(rows, messagesRow{RowID: rowID, Date: date, Text: string(text), AttributedBody: attributed})
	}
	return rows, nil
}

// messagesDateToTime converts a Messages date value (nanoseconds since
// 2001-01-01 UTC, or seconds in older databases) to a time.
func messagesDateToTime(value int64) time.Time {
	if value > -messagesSecondsThreshold && value < messagesSecondsThreshold {
		return messagesAppleEpoch.Add(time.Duration(value) * time.Second)
	}
	return messagesAppleEpoch.Add(time.Duration(value))
}

// selectAppleCode returns the code from the newest message received at or
// after cutoff that is an Apple verification message.
func selectAppleCode(rows []messagesRow, cutoff time.Time) (string, bool) {
	sorted := append([]messagesRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Date != sorted[j].Date {
			return messagesDateToTime(sorted[i].Date).After(messagesDateToTime(sorted[j].Date))
		}
		return sorted[i].RowID > sorted[j].RowID
	})
	for _, row := range sorted {
		if messagesDateToTime(row.Date).Before(cutoff) {
			continue
		}
		if code, ok := appleCodeFromRow(row); ok {
			return code, true
		}
	}
	return "", false
}

func appleCodeFromRow(row messagesRow) (string, bool) {
	if code, ok := extractAppleVerificationCode(row.Text); ok {
		return code, true
	}
	if len(row.AttributedBody) == 0 {
		return "", false
	}
	if decoded, ok := decodeTypedstreamString(row.AttributedBody); ok {
		if code, ok := extractAppleVerificationCode(decoded); ok {
			return code, true
		}
	}
	return extractAppleVerificationCode(strings.ToValidUTF8(string(row.AttributedBody), " "))
}

// extractAppleVerificationCode returns the 6-digit code from an Apple
// verification message. The domain-bound "@apple.com #123456" form wins;
// otherwise the message must use Apple's "Apple Account/ID Code is" wording,
// so other Apple messages that carry numbers are never mistaken for codes.
func extractAppleVerificationCode(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	if match := appleDomainBoundCodePattern.FindStringSubmatch(text); match != nil {
		return match[1], true
	}
	// A domain-bound code for another origin means the message belongs to
	// that origin, even if it also mentions Apple.
	if foreignDomainBoundCodePattern.MatchString(text) {
		return "", false
	}
	if match := appleVerificationCodePattern.FindStringSubmatch(text); match != nil {
		return match[1], true
	}
	return "", false
}

// decodeTypedstreamString extracts the NSString payload from an NSArchiver
// "streamtyped" attributedBody blob.
func decodeTypedstreamString(blob []byte) (string, bool) {
	idx := bytes.Index(blob, typedstreamNSStringMarker)
	if idx < 0 {
		return "", false
	}
	rest := blob[idx+len(typedstreamNSStringMarker):]
	plus := bytes.IndexByte(rest, '+')
	if plus < 0 {
		return "", false
	}
	rest = rest[plus+1:]
	if len(rest) == 0 {
		return "", false
	}

	var length uint64
	switch marker := rest[0]; {
	case marker < 0x80:
		length = uint64(marker)
		rest = rest[1:]
	case marker == 0x81:
		if len(rest) < 3 {
			return "", false
		}
		length = uint64(binary.LittleEndian.Uint16(rest[1:3]))
		rest = rest[3:]
	case marker == 0x82:
		if len(rest) < 5 {
			return "", false
		}
		length = uint64(binary.LittleEndian.Uint32(rest[1:5]))
		rest = rest[5:]
	default:
		return "", false
	}
	if length > uint64(len(rest)) {
		return "", false
	}
	payload := rest[:length]
	if !utf8.Valid(payload) {
		return "", false
	}
	return string(payload), true
}
