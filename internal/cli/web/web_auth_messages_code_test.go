package web

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	testAppleCurrentSMS = "Your Apple Account code is: 482913. Don't share it with anyone.\n\n@apple.com #482913 %apple.com"
	testAppleOldSMS     = "Your Apple ID Code is: 771204. Don't share it with anyone."
)

func typedstreamBlob(text string) []byte {
	prefix := "\x04\x0bstreamtyped\x81\xe8\x03\x84\x01@\x84\x84\x84\x12NSAttributedString\x00\x84\x84\x08NSObject\x00\x85\x92\x84\x84\x84\x08NSString\x01\x94\x84\x01+"
	n := len(text)
	var length string
	switch {
	case n < 0x80:
		length = string([]byte{byte(n)})
	case n <= 0xFFFF:
		length = string([]byte{0x81, byte(n), byte(n >> 8)})
	default:
		length = string([]byte{0x82, byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
	}
	return []byte(prefix + length + text + "\x86\x84\x02iI\x01")
}

func TestExtractAppleVerificationCode(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
		ok   bool
	}{
		{name: "current apple account format", text: testAppleCurrentSMS, want: "482913", ok: true},
		{name: "old apple id format", text: testAppleOldSMS, want: "771204", ok: true},
		{name: "domain bound form wins over other numbers", text: "Apple order 123456 shipped\n@apple.com #654321 %apple.com", want: "654321", ok: true},
		{name: "domain bound form without apple wording", text: "@apple.com #246810", want: "246810", ok: true},
		{name: "non apple code ignored", text: "Your Acme verification code is 135790", ok: false},
		{name: "pineapple is not apple", text: "Pineapple delivery code 112233", ok: false},
		{name: "other domain bound form ignored", text: "Your code is 445566\n\n@example.com #445566", ok: false},
		{name: "apple message without code", text: "Your Apple Account was used to sign in.", ok: false},
		{name: "longer numbers are not codes", text: "Apple reference 12345678", ok: false},
		{name: "live macOS 26 wording", text: "Your Apple Account Code is: 908172. Don't share it with anyone.", want: "908172", ok: true},
		{name: "apple mention with unrelated number ignored", text: "Apple order 123456 has shipped", ok: false},
		{name: "apple store pickup number ignored", text: "Your Apple Store pickup code is ready: 246813", ok: false},
		{name: "empty", text: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractAppleVerificationCode(tt.text)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("extractAppleVerificationCode() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestDecodeTypedstreamString(t *testing.T) {
	t.Run("short text uses single byte length", func(t *testing.T) {
		got, ok := decodeTypedstreamString(typedstreamBlob(testAppleCurrentSMS))
		if !ok || got != testAppleCurrentSMS {
			t.Fatalf("decodeTypedstreamString() = (%q, %v), want (%q, true)", got, ok, testAppleCurrentSMS)
		}
	})

	t.Run("long text uses 0x81 two byte length", func(t *testing.T) {
		long := testAppleCurrentSMS + " " + strings.Repeat("padding ", 20)
		if len(long) < 0x80 {
			t.Fatalf("test text must exceed 127 bytes, got %d", len(long))
		}
		blob := typedstreamBlob(long)
		if !strings.Contains(string(blob), "+\x81") {
			t.Fatal("expected 0x81 length marker in synthetic blob")
		}
		got, ok := decodeTypedstreamString(blob)
		if !ok || got != long {
			t.Fatalf("decodeTypedstreamString() = (%q, %v), want long text", got, ok)
		}
	})

	t.Run("0x82 four byte length", func(t *testing.T) {
		long := strings.Repeat("x", 0x10005)
		got, ok := decodeTypedstreamString(typedstreamBlob(long))
		if !ok || got != long {
			t.Fatalf("decodeTypedstreamString() ok=%v len=%d, want len %d", ok, len(got), len(long))
		}
	})

	t.Run("missing NSString marker", func(t *testing.T) {
		if got, ok := decodeTypedstreamString([]byte("\x04\x0bstreamtyped+\x05hello")); ok {
			t.Fatalf("decodeTypedstreamString() = %q, want failure", got)
		}
	})

	t.Run("truncated length", func(t *testing.T) {
		blob := typedstreamBlob("hello")
		blob = blob[:len(blob)-len("hello\x86\x84\x02iI\x01")+2]
		if got, ok := decodeTypedstreamString(blob); ok {
			t.Fatalf("decodeTypedstreamString() = %q, want failure for truncated blob", got)
		}
	})
}

func TestMessagesDateToTime(t *testing.T) {
	want := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	seconds := want.Unix() - messagesAppleEpoch.Unix()
	nanos := seconds * int64(time.Second)

	if got := messagesDateToTime(nanos); !got.Equal(want) {
		t.Fatalf("messagesDateToTime(ns) = %v, want %v", got, want)
	}
	if got := messagesDateToTime(seconds); !got.Equal(want) {
		t.Fatalf("messagesDateToTime(seconds) = %v, want %v", got, want)
	}
	if got := messagesDateToTime(0); !got.Equal(messagesAppleEpoch) {
		t.Fatalf("messagesDateToTime(0) = %v, want Apple epoch", got)
	}
}

func messagesNanos(t time.Time) int64 {
	return t.Sub(messagesAppleEpoch).Nanoseconds()
}

func messagesRowLine(rowID int64, date int64, text string, attributed []byte) string {
	return fmt.Sprintf("%d|%d|%s|%s", rowID, date, strings.ToUpper(hex.EncodeToString([]byte(text))), strings.ToUpper(hex.EncodeToString(attributed)))
}

func TestParseMessagesRows(t *testing.T) {
	blob := typedstreamBlob(testAppleCurrentSMS)
	out := messagesRowLine(7, 42, "", blob) + "\n" + messagesRowLine(6, 41, "hi|there", nil) + "\n"
	rows, err := parseMessagesRows([]byte(out))
	if err != nil {
		t.Fatalf("parseMessagesRows() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].RowID != 7 || rows[0].Date != 42 || rows[0].Text != "" || string(rows[0].AttributedBody) != string(blob) {
		t.Fatalf("rows[0] = %+v", rows[0])
	}
	if rows[1].Text != "hi|there" || rows[1].AttributedBody != nil {
		t.Fatalf("rows[1] = %+v", rows[1])
	}

	if rows, err := parseMessagesRows(nil); err != nil || len(rows) != 0 {
		t.Fatalf("parseMessagesRows(empty) = (%v, %v), want no rows", rows, err)
	}

	_, err = parseMessagesRows([]byte("1|2|ZZ|\n"))
	if err == nil {
		t.Fatal("parseMessagesRows() error = nil, want malformed hex error")
	}
	if strings.Contains(err.Error(), "ZZ") {
		t.Fatalf("parse error must not echo database output: %v", err)
	}
	if _, err := parseMessagesRows([]byte("1|2|AB\n")); err == nil {
		t.Fatal("parseMessagesRows() error = nil, want wrong column count error")
	}
}

func TestSelectAppleCode(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-time.Minute)

	t.Run("newest matching message wins", func(t *testing.T) {
		rows := []messagesRow{
			{RowID: 9, Date: messagesNanos(now.Add(-5 * time.Second)), Text: "Your Acme code is 999999"},
			{RowID: 8, Date: messagesNanos(now.Add(-10 * time.Second)), AttributedBody: typedstreamBlob("Your Apple Account code is: 111111.\n\n@apple.com #111111 %apple.com")},
			{RowID: 7, Date: messagesNanos(now.Add(-20 * time.Second)), Text: "Your Apple Account code is: 222222.\n\n@apple.com #222222 %apple.com"},
		}
		got, ok := selectAppleCode(rows, cutoff)
		if !ok || got != "111111" {
			t.Fatalf("selectAppleCode() = (%q, %v), want (111111, true)", got, ok)
		}
	})

	t.Run("orders by date even when input is unordered", func(t *testing.T) {
		rows := []messagesRow{
			{RowID: 1, Date: messagesNanos(now.Add(-30 * time.Second)), Text: "Your Apple ID Code is: 333333"},
			{RowID: 2, Date: messagesNanos(now.Add(-2 * time.Second)), Text: "Your Apple ID Code is: 444444"},
		}
		got, ok := selectAppleCode(rows, cutoff)
		if !ok || got != "444444" {
			t.Fatalf("selectAppleCode() = (%q, %v), want (444444, true)", got, ok)
		}
	})

	t.Run("stale messages are excluded", func(t *testing.T) {
		rows := []messagesRow{
			{RowID: 1, Date: messagesNanos(now.Add(-2 * time.Minute)), Text: testAppleCurrentSMS},
		}
		if got, ok := selectAppleCode(rows, cutoff); ok {
			t.Fatalf("selectAppleCode() = %q, want no code for stale message", got)
		}
	})

	t.Run("raw blob fallback when typedstream decoding fails", func(t *testing.T) {
		rows := []messagesRow{
			{RowID: 1, Date: messagesNanos(now), AttributedBody: []byte("\x00garbage " + testAppleCurrentSMS + "\x00")},
		}
		got, ok := selectAppleCode(rows, cutoff)
		if !ok || got != "482913" {
			t.Fatalf("selectAppleCode() = (%q, %v), want raw fallback code", got, ok)
		}
	})
}

func TestMessagesCodeQueryUsesOnlyIntegerCutoffs(t *testing.T) {
	cutoff := time.Date(2026, 10, 17, 11, 59, 0, 0, time.UTC)
	query := messagesCodeQuery(cutoff)
	nanos := messagesNanos(cutoff)
	seconds := nanos / int64(time.Second)
	for _, want := range []string{
		"is_from_me = 0",
		fmt.Sprintf("date >= %d", nanos),
		fmt.Sprintf("date >= %d", seconds),
		"hex(attributedBody)",
		"text LIKE '%apple%'",
		"instr(attributedBody, CAST('Apple' AS BLOB)) > 0",
		"ORDER BY date DESC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q missing %q", query, want)
		}
	}
}

// messagesCodeTestHooks installs deterministic hooks for command tests.
func messagesCodeTestHooks(t *testing.T, goos string, query func(ctx context.Context, dbPath, query string) ([]byte, error)) {
	t.Helper()
	prevGOOS := messagesCodeGOOSFn
	prevQuery := messagesCodeQueryFn
	prevInterval := messagesCodePollInterval
	t.Cleanup(func() {
		messagesCodeGOOSFn = prevGOOS
		messagesCodeQueryFn = prevQuery
		messagesCodePollInterval = prevInterval
	})
	messagesCodeGOOSFn = func() string { return goos }
	messagesCodeQueryFn = query
	messagesCodePollInterval = 5 * time.Millisecond
}

func runMessagesCodeCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := WebAuthMessagesCodeCommand()
	cmd.FlagSet.SetOutput(os.Stderr)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.FlagSet.Parse(args); err != nil {
			runErr = err
			return
		}
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})
	return stdout, stderr, runErr
}

func tempMessagesDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chat.db")
	if err := os.WriteFile(path, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestWebAuthMessagesCodePrintsOnlyCode(t *testing.T) {
	dbPath := tempMessagesDB(t)
	var gotDB string
	calls := 0
	messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
		calls++
		gotDB = path
		if calls == 1 {
			return nil, nil
		}
		now := time.Now()
		out := messagesRowLine(12, messagesNanos(now), "", typedstreamBlob(testAppleCurrentSMS)) + "\n" +
			messagesRowLine(11, messagesNanos(now.Add(-time.Second)), "Your Acme code is 999999", nil) + "\n"
		return []byte(out), nil
	})

	stdout, stderr, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "2s")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if stdout != "482913\n" {
		t.Fatalf("stdout = %q, want only the code", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty (must never print message contents)", stderr)
	}
	if calls < 2 {
		t.Fatalf("expected polling until the message arrived, calls = %d", calls)
	}
	if gotDB != dbPath {
		t.Fatalf("db path = %q, want %q", gotDB, dbPath)
	}
}

func TestWebAuthMessagesCodeHonorsCodeRequestTime(t *testing.T) {
	t.Run("ignores codes from before the request", func(t *testing.T) {
		dbPath := tempMessagesDB(t)
		requestedAt := time.Now().Add(-5 * time.Second)
		t.Setenv(webTwoFactorCodeRequestedAtEnv, requestedAt.UTC().Format(time.RFC3339Nano))
		calls := 0
		messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
			calls++
			out := messagesRowLine(1, messagesNanos(requestedAt.Add(-15*time.Second)), "Your Apple Account Code is: 111111.", nil) + "\n"
			if calls > 1 {
				out = messagesRowLine(2, messagesNanos(time.Now()), "Your Apple Account Code is: 222222.", nil) + "\n" + out
			}
			return []byte(out), nil
		})

		stdout, _, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "2s")
		if err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
		if stdout != "222222\n" || calls < 2 {
			t.Fatalf("stdout = %q after %d polls, want the code sent after the request", stdout, calls)
		}
	})

	t.Run("explicit since wins", func(t *testing.T) {
		dbPath := tempMessagesDB(t)
		requestedAt := time.Now()
		t.Setenv(webTwoFactorCodeRequestedAtEnv, requestedAt.UTC().Format(time.RFC3339Nano))
		messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
			return []byte(messagesRowLine(1, messagesNanos(requestedAt.Add(-20*time.Second)), "Your Apple Account Code is: 333333.", nil) + "\n"), nil
		})

		stdout, _, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "2s", "--since", "1m")
		if err != nil || stdout != "333333\n" {
			t.Fatalf("stdout = %q, err = %v, want the code inside the explicit lookback", stdout, err)
		}
	})

	t.Run("invalid request time fails", func(t *testing.T) {
		dbPath := tempMessagesDB(t)
		t.Setenv(webTwoFactorCodeRequestedAtEnv, "yesterday")
		messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
			t.Fatal("expected no database read with an invalid request time")
			return nil, nil
		})

		_, _, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "2s")
		if err == nil || !strings.Contains(err.Error(), webTwoFactorCodeRequestedAtEnv+" must be an RFC 3339 timestamp") {
			t.Fatalf("error = %v, want invalid request time error", err)
		}
	})
}

func TestWebAuthMessagesCodeTimesOut(t *testing.T) {
	dbPath := tempMessagesDB(t)
	messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
		stale := messagesNanos(time.Now().Add(-10 * time.Minute))
		return []byte(messagesRowLine(1, stale, testAppleCurrentSMS, nil) + "\n"), nil
	})

	stdout, stderr, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "50ms", "--since", "30s")
	if err == nil {
		t.Fatal("Exec() error = nil, want timeout")
	}
	if !strings.Contains(err.Error(), "no Apple verification code") || !strings.Contains(err.Error(), "50ms") {
		t.Fatalf("error = %v, want timeout guidance", err)
	}
	if strings.Contains(err.Error(), "482913") {
		t.Fatalf("error leaked message code: %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout, stderr)
	}
}

func TestWebAuthMessagesCodePrivacyErrorGuidance(t *testing.T) {
	dbPath := tempMessagesDB(t)
	for _, sqliteErr := range []string{
		`Error: unable to open database "file:///x/chat.db?mode=ro": unable to open database file`,
		"Error: authorization denied",
		"Error: operation not permitted",
	} {
		t.Run(sqliteErr, func(t *testing.T) {
			messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
				return nil, errors.New(sqliteErr)
			})
			stdout, _, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "1s")
			if err == nil {
				t.Fatal("Exec() error = nil, want privacy guidance")
			}
			for _, want := range []string{"Full Disk Access", "System Settings > Privacy & Security > Full Disk Access", "Text Message Forwarding"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %v, want %q", err, want)
				}
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestWebAuthMessagesCodeRejectsNonDarwin(t *testing.T) {
	called := false
	messagesCodeTestHooks(t, "linux", func(ctx context.Context, path, query string) ([]byte, error) {
		called = true
		return nil, nil
	})
	_, _, err := runMessagesCodeCommand(t)
	if err == nil || !strings.Contains(err.Error(), "only supported on macOS") {
		t.Fatalf("error = %v, want macOS-only error", err)
	}
	if called {
		t.Fatal("did not expect sqlite query on non-darwin")
	}
}

func TestWebAuthMessagesCodeMissingDatabase(t *testing.T) {
	messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
		t.Fatal("did not expect sqlite query for missing database")
		return nil, nil
	})
	missing := filepath.Join(t.TempDir(), "nope.db")
	_, _, err := runMessagesCodeCommand(t, "--db", missing)
	if err == nil || !strings.Contains(err.Error(), "no Messages database found") || !strings.Contains(err.Error(), missing) {
		t.Fatalf("error = %v, want missing database error", err)
	}
}

func TestWebAuthMessagesCodeExpandsHomeInDBPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dbPath := filepath.Join(home, "custom.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var gotDB string
	messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
		gotDB = path
		return []byte(messagesRowLine(1, messagesNanos(time.Now()), testAppleOldSMS, nil) + "\n"), nil
	})
	stdout, _, err := runMessagesCodeCommand(t, "--db", "~/custom.db")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if stdout != "771204\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if gotDB != dbPath {
		t.Fatalf("db path = %q, want %q", gotDB, dbPath)
	}
}

func TestWebAuthMessagesCodeFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "zero since", args: []string{"--since", "0s"}, wantErr: "--since must be a positive duration"},
		{name: "since too long", args: []string{"--since", "24h"}, wantErr: "--since must be at most 10m0s"},
		{name: "negative timeout", args: []string{"--timeout", "-1s"}, wantErr: "--timeout must be a positive duration"},
		{name: "blank db", args: []string{"--db", "  "}, wantErr: "--db must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messagesCodeTestHooks(t, "darwin", func(ctx context.Context, path, query string) ([]byte, error) {
				t.Fatal("did not expect sqlite query for invalid flags")
				return nil, nil
			})
			stdout, stderr, err := runMessagesCodeCommand(t, tt.args...)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestWebAuthMessagesCodeReadsRealSQLiteDatabase(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Messages code reader is macOS-only")
	}
	sqlitePath, err := exec.LookPath("/usr/bin/sqlite3")
	if err != nil {
		t.Skip("/usr/bin/sqlite3 is not available")
	}
	messagesCodeTestHooks(t, "darwin", runMessagesSQLiteQuery)

	dbPath := filepath.Join(t.TempDir(), "chat db's ?.db")
	now := time.Now()
	staleBlob := typedstreamBlob("Your Apple Account code is: 000111.\n\n@apple.com #000111 %apple.com")
	freshBlob := typedstreamBlob(testAppleCurrentSMS)
	script := strings.Join([]string{
		"CREATE TABLE message (ROWID INTEGER PRIMARY KEY AUTOINCREMENT, text TEXT, attributedBody BLOB, date INTEGER, is_from_me INTEGER DEFAULT 0);",
		fmt.Sprintf("INSERT INTO message (text, attributedBody, date, is_from_me) VALUES (NULL, X'%s', %d, 0);", hex.EncodeToString(staleBlob), messagesNanos(now.Add(-10*time.Minute))),
		fmt.Sprintf("INSERT INTO message (text, attributedBody, date, is_from_me) VALUES (NULL, X'%s', %d, 0);", hex.EncodeToString(freshBlob), messagesNanos(now.Add(-2*time.Second))),
		fmt.Sprintf("INSERT INTO message (text, attributedBody, date, is_from_me) VALUES ('Your Apple ID Code is: 999000', NULL, %d, 1);", messagesNanos(now)),
		fmt.Sprintf("INSERT INTO message (text, attributedBody, date, is_from_me) VALUES ('Your Acme code is 123123', NULL, %d, 0);", messagesNanos(now)),
	}, "\n")
	// Busy chats newer than the code must not push it out of the row limit.
	for i := 0; i < messagesCodeRowLimit+10; i++ {
		script += fmt.Sprintf("\nINSERT INTO message (text, attributedBody, date, is_from_me) VALUES ('group chat %d', NULL, %d, 0);", i, messagesNanos(now.Add(-time.Second)))
	}
	create := exec.Command(sqlitePath, dbPath)
	create.Stdin = strings.NewReader(script)
	if out, err := create.CombinedOutput(); err != nil {
		t.Fatalf("creating test database failed: %v: %s", err, out)
	}

	stdout, stderr, err := runMessagesCodeCommand(t, "--db", dbPath, "--timeout", "5s")
	if err != nil {
		t.Fatalf("Exec() error = %v (stderr=%q)", err, stderr)
	}
	if stdout != "482913\n" {
		t.Fatalf("stdout = %q, want fresh Apple code", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}
