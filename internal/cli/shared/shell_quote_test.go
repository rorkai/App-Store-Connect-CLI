package shared

import (
	"flag"
	"io"
	"os/exec"
	"runtime"
	"slices"
	"testing"
)

func TestShellQuoteSafeWordsStayBare(t *testing.T) {
	for _, value := range []string{"release-2026.1", "/tmp/AuthKey_ABC123.p8", "json", "MyKey", "1.2.3"} {
		got, ok := ShellQuote(value)
		if !ok || got != value {
			t.Fatalf("ShellQuote(%q) = (%q, %t), want (%q, true)", value, got, ok, value)
		}
	}
}

func TestShellQuoteQuotesMetacharacters(t *testing.T) {
	// Values that are not safe words must never be printed bare, whichever
	// platform rendering applies.
	for _, value := range []string{"", "My Key", "=ls", "$(whoami)", "`id`", "~/keys", "@args", "%PATH%", "a;b", "a&b", "it's"} {
		got, ok := ShellQuote(value)
		if !ok {
			continue
		}
		if got == value {
			t.Fatalf("ShellQuote(%q) = %q, want it quoted", value, got)
		}
		if quote := got[:1]; quote != "'" && quote != `"` {
			t.Fatalf("ShellQuote(%q) = %q, want a quoted argument", value, got)
		}
	}
}

func TestPosixShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty value", value: "", want: "''"},
		{name: "spaces", value: "My Key", want: "'My Key'"},
		{name: "command substitution stays inert", value: "$(whoami)", want: "'$(whoami)'"},
		{name: "backticks stay inert", value: "`id`", want: "'`id`'"},
		{name: "tilde is not expanded", value: "~/keys", want: "'~/keys'"},
		{name: "embedded apostrophe", value: "O'Brien key", want: `'O'\''Brien key'`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := posixShellQuote(test.value); got != test.want {
				t.Fatalf("posixShellQuote(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestShellQuoteKeepsLeadingEqualsLiteralInZsh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("zsh quoting is only relevant on POSIX platforms")
	}
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not installed")
	}

	rendered, ok := ShellQuote("=ls")
	if !ok {
		t.Fatal("ShellQuote(\"=ls\") reported the value as unquotable")
	}
	output, err := exec.Command("zsh", "-f", "-c", `printf '%s' `+rendered).CombinedOutput()
	if err != nil {
		t.Fatalf("zsh round trip failed: %v (%s)", err, output)
	}
	if got := string(output); got != "=ls" {
		t.Fatalf("zsh round trip = %q, want literal %q", got, "=ls")
	}
}

func TestWindowsShellQuote(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   string
		wantOK bool
	}{
		{name: "empty value", value: "", want: `""`, wantOK: true},
		{name: "spaces", value: "My Key", want: `"My Key"`, wantOK: true},
		{name: "apostrophe is literal in double quotes", value: "O'Brien key", want: `"O'Brien key"`, wantOK: true},
		{name: "splat", value: "@args", want: `"@args"`, wantOK: true},
		{name: "semicolon", value: "a;b", want: `"a;b"`, wantOK: true},
		{name: "powershell variable", value: "$env:PATH", wantOK: false},
		{name: "powershell escape", value: "a`b", wantOK: false},
		{name: "cmd variable", value: "%PATH%", wantOK: false},
		{name: "cmd delayed expansion", value: "a!b!", wantOK: false},
		{name: "embedded double quote", value: `a"b`, wantOK: false},
		{name: "powershell left curly double quote", value: "a\u201cb", wantOK: false},
		{name: "powershell right curly double quote", value: "a\u201db", wantOK: false},
		{name: "powershell low curly double quote", value: "a\u201eb", wantOK: false},
		{name: "unc path", value: `\\server\share path`, wantOK: false},
		{name: "trailing backslash", value: `C:\keys\`, wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := windowsShellQuote(test.value)
			if ok != test.wantOK {
				t.Fatalf("windowsShellQuote(%q) = (%q, %t), want ok %t", test.value, got, ok, test.wantOK)
			}
			if got != test.want {
				t.Fatalf("windowsShellQuote(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestShellQuoteUsesPlatformRendering(t *testing.T) {
	const value = "My Key"
	want, wantOK := posixShellQuote(value), true
	if runtime.GOOS == "windows" {
		want, wantOK = windowsShellQuote(value)
	}
	got, ok := ShellQuote(value)
	if got != want || ok != wantOK {
		t.Fatalf("ShellQuote(%q) = (%q, %t), want (%q, %t)", value, got, ok, want, wantOK)
	}
}

func TestShellQuoteForOSUsesRequestedPlatformRendering(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "'My Key'"},
		{goos: "linux", want: "'My Key'"},
		{goos: "windows", want: `"My Key"`},
	}

	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			got, ok := ShellQuoteForOS("My Key", test.goos)
			if !ok || got != test.want {
				t.Fatalf("ShellQuoteForOS(%q, %q) = (%q, %t), want (%q, true)", "My Key", test.goos, got, ok, test.want)
			}
		})
	}
}

func TestShellQuoteForOSRejectsValuesItCannotRenderExactly(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, value := range []string{"line\nnext", "a\x1b[31mred", "a\u202eb", "a\xffb"} {
			if got, ok := ShellQuoteForOS(value, goos); ok || got != "" {
				t.Fatalf("ShellQuoteForOS(%q, %q) = (%q, %t), want (\"\", false)", value, goos, got, ok)
			}
		}
	}
}

func TestShellQuoteRejectsValuesItCannotRenderExactly(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "ansi escape", value: "a\x1b[31mred"},
		{name: "newline", value: "line\nnext"},
		{name: "tab", value: "a\tb"},
		{name: "bell", value: "bell\a"},
		{name: "bidi override", value: "a\u202eb"},
		{name: "invalid utf-8", value: "a\xffb"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ShellQuote(test.value)
			if ok {
				t.Fatalf("ShellQuote(%q) = %q, want it reported as unquotable", test.value, got)
			}
			if got != "" {
				t.Fatalf("ShellQuote(%q) returned %q alongside ok=false", test.value, got)
			}
		})
	}
}

func TestRootFlagsForReinvocation(t *testing.T) {
	quoted := func(t *testing.T, value string) string {
		t.Helper()
		rendered, ok := ShellQuote(value)
		if !ok {
			t.Fatalf("ShellQuote(%q) reported the value as unquotable", value)
		}
		return rendered
	}

	tests := []struct {
		name   string
		args   []string
		want   func(*testing.T) []string
		wantOK bool
	}{
		{
			name:   "no root flags",
			args:   []string{},
			want:   func(*testing.T) []string { return []string{} },
			wantOK: true,
		},
		{
			name:   "profile",
			args:   []string{"--profile", "release"},
			want:   func(*testing.T) []string { return []string{"--profile", "release"} },
			wantOK: true,
		},
		{
			name: "profile is quoted",
			args: []string{"--profile", "my key"},
			want: func(t *testing.T) []string {
				return []string{"--profile", quoted(t, "my key")}
			},
			wantOK: true,
		},
		{
			name: "leading equals profile is quoted",
			args: []string{"--profile", "=ls"},
			want: func(t *testing.T) []string {
				return []string{"--profile", quoted(t, "=ls")}
			},
			wantOK: true,
		},
		{
			name:   "strict auth",
			args:   []string{"--strict-auth"},
			want:   func(*testing.T) []string { return []string{"--strict-auth"} },
			wantOK: true,
		},
		{
			name: "report flags",
			args: []string{"--profile", "release", "--report", "junit", "--report-file", "out dir/report.xml"},
			want: func(t *testing.T) []string {
				return []string{"--profile", "release", "--report", "junit", "--report-file", quoted(t, "out dir/report.xml")}
			},
			wantOK: true,
		},
		{
			name:   "unrenderable profile",
			args:   []string{"--profile", "a\x1bb"},
			want:   func(*testing.T) []string { return nil },
			wantOK: false,
		},
		{
			name:   "unrenderable report path",
			args:   []string{"--report", "junit", "--report-file", "out\ndir/report.xml"},
			want:   func(*testing.T) []string { return nil },
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// BindRootFlags rebinds the package-level root flag state, so each
			// case starts from the flag defaults.
			fs := flag.NewFlagSet("asc", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			BindRootFlags(fs)
			t.Cleanup(func() {
				restore := flag.NewFlagSet("asc", flag.ContinueOnError)
				restore.SetOutput(io.Discard)
				BindRootFlags(restore)
			})
			if err := fs.Parse(test.args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			got, ok := RootFlagsForReinvocation()
			if ok != test.wantOK {
				t.Fatalf("RootFlagsForReinvocation() ok = %t, want %t (args %q)", ok, test.wantOK, got)
			}
			if want := test.want(t); !slices.Equal(got, want) {
				t.Fatalf("RootFlagsForReinvocation() = %q, want %q", got, want)
			}
		})
	}
}
