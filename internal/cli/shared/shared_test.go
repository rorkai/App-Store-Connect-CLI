package shared

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, stderr
}

func resetDefaultOutput(t *testing.T) {
	t.Helper()
	ResetDefaultOutputFormat()
	t.Cleanup(func() {
		ResetDefaultOutputFormat()
	})
}

func TestDefaultOutputFormat_ReturnsJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "")
	if got := DefaultOutputFormat(); got != "json" {
		t.Fatalf("expected json, got %q", got)
	}
}

func TestDefaultOutputFormat_UnsetReturnsJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "")
	os.Unsetenv("ASC_DEFAULT_OUTPUT")
	if got := DefaultOutputFormat(); got != "json" {
		t.Fatalf("expected json, got %q", got)
	}
}

func TestDefaultOutputFormat_Table(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")
	if got := DefaultOutputFormat(); got != "table" {
		t.Fatalf("expected table, got %q", got)
	}
}

func TestDefaultOutputFormat_Markdown(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "markdown")
	if got := DefaultOutputFormat(); got != "markdown" {
		t.Fatalf("expected markdown, got %q", got)
	}
}

func TestDefaultOutputFormat_MD(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "md")
	if got := DefaultOutputFormat(); got != "md" {
		t.Fatalf("expected md, got %q", got)
	}
}

func TestDefaultOutputFormat_JSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")
	if got := DefaultOutputFormat(); got != "json" {
		t.Fatalf("expected json, got %q", got)
	}
}

func TestDefaultOutputFormat_CaseInsensitive(t *testing.T) {
	for _, value := range []string{"TABLE", "Table", "tAbLe", "MARKDOWN", "JSON"} {
		t.Run(value, func(t *testing.T) {
			resetDefaultOutput(t)
			t.Setenv("ASC_DEFAULT_OUTPUT", value)
			got := DefaultOutputFormat()
			expected := strings.ToLower(value)
			if got != expected {
				t.Fatalf("expected %q, got %q", expected, got)
			}
		})
	}
}

func TestDefaultOutputFormat_WhitespaceHandled(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "  table  ")
	if got := DefaultOutputFormat(); got != "table" {
		t.Fatalf("expected table, got %q", got)
	}
}

func TestDefaultOutputFormat_InvalidFallsBackToJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "xml")
	stdout, stderr := captureOutput(t, func() {
		got := DefaultOutputFormat()
		if got != "json" {
			t.Fatalf("expected json fallback, got %q", got)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "invalid ASC_DEFAULT_OUTPUT value") {
		t.Fatalf("expected warning on stderr, got %q", stderr)
	}
}

func TestBindOutputFlagsUsesDefaultOutputFormat(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := BindOutputFlags(fs)
	if output.Output == nil || output.Pretty == nil {
		t.Fatal("expected output flag pointers to be set")
	}
	if *output.Output != "table" {
		t.Fatalf("expected output default table, got %q", *output.Output)
	}
	if *output.Pretty {
		t.Fatal("expected pretty default false")
	}
}

func TestBindOutputFlagsParsesValues(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := BindOutputFlags(fs)
	if err := fs.Parse([]string{"--output", "markdown", "--pretty"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if *output.Output != "markdown" {
		t.Fatalf("expected output markdown, got %q", *output.Output)
	}
	if !*output.Pretty {
		t.Fatal("expected pretty true after parse")
	}
}

func TestBindOutputFlagsWithParsesCustomFlagName(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := BindOutputFlagsWith(fs, "format", "json", "Output format: json (default), table, markdown")
	if err := fs.Parse([]string{"--format", "markdown", "--pretty"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if *output.Output != "markdown" {
		t.Fatalf("expected format markdown, got %q", *output.Output)
	}
	if !*output.Pretty {
		t.Fatal("expected pretty true after parse")
	}
}

func TestBindOutputFlagsWithDefaultsFlagNameToOutput(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := BindOutputFlagsWith(fs, "", "json", "Output format: json (default), table, markdown")
	if err := fs.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if *output.Output != "table" {
		t.Fatalf("expected output table, got %q", *output.Output)
	}
}

func TestBindPrettyJSONFlagDefaultsFalseAndParses(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	pretty := BindPrettyJSONFlag(fs)
	if pretty == nil {
		t.Fatal("expected pretty flag pointer to be set")
	}
	if *pretty {
		t.Fatal("expected pretty default false")
	}

	if err := fs.Parse([]string{"--pretty"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !*pretty {
		t.Fatal("expected pretty true after parse")
	}
}

func TestNormalizeOutputFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{name: "json unchanged", input: "json", output: "json"},
		{name: "uppercase lowered", input: "TABLE", output: "table"},
		{name: "md alias canonicalized", input: "md", output: "markdown"},
		{name: "md alias canonicalized uppercase", input: "MD", output: "markdown"},
		{name: "trimmed and lowered", input: "  TABLE  ", output: "table"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeOutputFormat(tc.input); got != tc.output {
				t.Fatalf("NormalizeOutputFormat(%q) = %q, want %q", tc.input, got, tc.output)
			}
		})
	}
}

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		pretty     bool
		wantFormat string
		wantErr    string
	}{
		{name: "empty defaults json", input: "", pretty: false, wantFormat: "json"},
		{name: "json allows pretty", input: "json", pretty: true, wantFormat: "json"},
		{name: "md alias", input: "md", pretty: false, wantFormat: "markdown"},
		{name: "table pretty rejected", input: "table", pretty: true, wantErr: "--pretty is only valid with JSON output"},
		{name: "unsupported rejected", input: "yaml", pretty: false, wantErr: "unsupported format: yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateOutputFormat(tc.input, tc.pretty)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantFormat {
				t.Fatalf("expected format %q, got %q", tc.wantFormat, got)
			}
		})
	}
}

func TestValidateOutputFormatAllowed(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		pretty     bool
		allowed    []string
		wantFormat string
		wantErr    string
	}{
		{name: "text allowed", input: "text", pretty: false, allowed: []string{"text", "json"}, wantFormat: "text"},
		{name: "json default allowed", input: "", pretty: false, allowed: []string{"text", "json"}, wantFormat: "json"},
		{name: "md unsupported when not allowed", input: "md", pretty: false, allowed: []string{"text", "json"}, wantErr: "unsupported format: markdown"},
		{name: "alias allowed when markdown allowed", input: "md", pretty: false, allowed: []string{"markdown", "json"}, wantFormat: "markdown"},
		{name: "pretty rejected for text", input: "text", pretty: true, allowed: []string{"text", "json"}, wantErr: "--pretty is only valid with JSON output"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateOutputFormatAllowed(tc.input, tc.pretty, tc.allowed...)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantFormat {
				t.Fatalf("expected format %q, got %q", tc.wantFormat, got)
			}
		})
	}
}

func TestValidateOutputFormatAllowed_EmptyAllowedFallsBackToDefaultSet(t *testing.T) {
	got, err := ValidateOutputFormatAllowed("table", false)
	if err != nil {
		t.Fatalf("unexpected error for default allowed set: %v", err)
	}
	if got != "table" {
		t.Fatalf("expected table, got %q", got)
	}

	_, err = ValidateOutputFormatAllowed("yaml", false)
	if err == nil || !strings.Contains(err.Error(), "unsupported format: yaml") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestPrintOutputWithRenderers_JSONPath(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		if err := PrintOutputWithRenderers(
			map[string]string{"status": "ok"},
			"json",
			false,
			func() error { t.Fatal("table renderer should not run"); return nil },
			func() error { t.Fatal("markdown renderer should not run"); return nil },
		); err != nil {
			t.Fatalf("PrintOutputWithRenderers() error = %v", err)
		}
	})
	if !strings.Contains(stdout, `"status":"ok"`) {
		t.Fatalf("expected JSON output, got %q", stdout)
	}
}

func TestPrintOutputWithRenderers_JSONPrettyPath(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		if err := PrintOutputWithRenderers(
			map[string]string{"status": "ok"},
			"json",
			true,
			func() error { t.Fatal("table renderer should not run"); return nil },
			func() error { t.Fatal("markdown renderer should not run"); return nil },
		); err != nil {
			t.Fatalf("PrintOutputWithRenderers() error = %v", err)
		}
	})
	if !strings.Contains(stdout, `"status": "ok"`) {
		t.Fatalf("expected pretty JSON output, got %q", stdout)
	}
}

func TestPrintOutputWithRenderers_EmptyFormatDefaultsJSON(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		if err := PrintOutputWithRenderers(
			map[string]string{"status": "ok"},
			"",
			false,
			func() error { t.Fatal("table renderer should not run"); return nil },
			func() error { t.Fatal("markdown renderer should not run"); return nil },
		); err != nil {
			t.Fatalf("PrintOutputWithRenderers() error = %v", err)
		}
	})
	if !strings.Contains(stdout, `"status":"ok"`) {
		t.Fatalf("expected JSON output for empty format, got %q", stdout)
	}
}

func TestPrintOutputWithRenderers_TableAndMarkdownPaths(t *testing.T) {
	tableCalls := 0
	markdownCalls := 0

	if err := PrintOutputWithRenderers(
		struct{}{},
		"table",
		false,
		func() error { tableCalls++; return nil },
		func() error { markdownCalls++; return nil },
	); err != nil {
		t.Fatalf("table output error = %v", err)
	}
	if tableCalls != 1 || markdownCalls != 0 {
		t.Fatalf("expected table=1 markdown=0, got table=%d markdown=%d", tableCalls, markdownCalls)
	}

	if err := PrintOutputWithRenderers(
		struct{}{},
		"md",
		false,
		func() error { tableCalls++; return nil },
		func() error { markdownCalls++; return nil },
	); err != nil {
		t.Fatalf("markdown output error = %v", err)
	}
	if tableCalls != 1 || markdownCalls != 1 {
		t.Fatalf("expected table=1 markdown=1, got table=%d markdown=%d", tableCalls, markdownCalls)
	}
}

func TestPrintOutputWithRenderers_RejectsPrettyForNonJSON(t *testing.T) {
	err := PrintOutputWithRenderers(struct{}{}, "table", true, func() error { return nil }, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty validation error, got %v", err)
	}
}

func TestPrintOutputWithRenderers_RequiresTableRenderer(t *testing.T) {
	err := PrintOutputWithRenderers(struct{}{}, "table", false, nil, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "table renderer is required") {
		t.Fatalf("expected table renderer required error, got %v", err)
	}
}

func TestPrintOutputWithRenderers_RequiresMarkdownRenderer(t *testing.T) {
	err := PrintOutputWithRenderers(struct{}{}, "markdown", false, func() error { return nil }, nil)
	if err == nil || !strings.Contains(err.Error(), "markdown renderer is required") {
		t.Fatalf("expected markdown renderer required error, got %v", err)
	}
}

func TestBindMetadataOutputFlagsUsesJSONDefault(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := BindMetadataOutputFlags(fs)
	if output.OutputFormat == nil || output.Pretty == nil {
		t.Fatal("expected metadata output flag pointers to be set")
	}
	if *output.OutputFormat != "json" {
		t.Fatalf("expected output-format default json, got %q", *output.OutputFormat)
	}
	if *output.Pretty {
		t.Fatal("expected pretty default false")
	}
}

func TestBindMetadataOutputFlagsParsesValues(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := BindMetadataOutputFlags(fs)
	if err := fs.Parse([]string{"--output-format", "markdown", "--pretty"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if *output.OutputFormat != "markdown" {
		t.Fatalf("expected output-format markdown, got %q", *output.OutputFormat)
	}
	if !*output.Pretty {
		t.Fatal("expected pretty true after parse")
	}
}

func TestValidateNextURL_ValidAppStoreConnectURL(t *testing.T) {
	err := validateNextURL("https://api.appstoreconnect.apple.com/v1/apps?cursor=abc")
	if err != nil {
		t.Fatalf("validateNextURL() error = %v", err)
	}
}

func TestValidateNextURL_RejectsMalformedHost(t *testing.T) {
	tests := []string{
		"http://localhost:80:80/v1/apps?cursor=abc",
		"http://::1/v1/apps?cursor=abc",
	}

	for _, next := range tests {
		t.Run(next, func(t *testing.T) {
			err := validateNextURL(next)
			if err == nil {
				t.Fatalf("expected error for malformed URL %q", next)
			}
			if !strings.Contains(err.Error(), "--next must be a valid URL") {
				t.Fatalf("expected parse validation error, got %v", err)
			}
		})
	}
}

func TestProgressEnabled_RespectsNoProgressFlag(t *testing.T) {
	prevNoProgress := noProgress
	prevIsTerminal := isTerminal
	t.Cleanup(func() {
		SetNoProgress(prevNoProgress)
		isTerminal = prevIsTerminal
	})

	isTerminal = func(int) bool { return true }
	SetNoProgress(true)

	if ProgressEnabled() {
		t.Fatal("expected progress to be disabled when --no-progress is set")
	}
}

func TestProgressEnabled_DisabledWhenStderrNotTTY(t *testing.T) {
	prevNoProgress := noProgress
	prevIsTerminal := isTerminal
	t.Cleanup(func() {
		SetNoProgress(prevNoProgress)
		isTerminal = prevIsTerminal
	})

	isTerminal = func(int) bool { return false }
	SetNoProgress(false)

	if ProgressEnabled() {
		t.Fatal("expected progress to be disabled when stderr is not a TTY")
	}
}

func TestProgressEnabled_EnabledWhenTTYAndNotDisabled(t *testing.T) {
	prevNoProgress := noProgress
	prevIsTerminal := isTerminal
	t.Cleanup(func() {
		SetNoProgress(prevNoProgress)
		isTerminal = prevIsTerminal
	})

	isTerminal = func(int) bool { return true }
	SetNoProgress(false)

	if !ProgressEnabled() {
		t.Fatal("expected progress to be enabled when stderr is a TTY and --no-progress is not set")
	}
}

func TestResolvePrivateKeyPathPrefersPath(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")
	t.Setenv("ASC_PRIVATE_KEY_B64", base64.StdEncoding.EncodeToString([]byte("ignored")))
	t.Setenv("ASC_PRIVATE_KEY", "ignored")

	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	if path != "/tmp/AuthKey.p8" {
		t.Fatalf("expected path /tmp/AuthKey.p8, got %q", path)
	}
}

func TestResolvePrivateKeyPathFromBase64(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")

	encoded := base64.StdEncoding.EncodeToString([]byte("key-data"))
	t.Setenv("ASC_PRIVATE_KEY_B64", encoded)

	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "key-data" {
		t.Fatalf("expected key data %q, got %q", "key-data", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected 0600 permissions, got %v", info.Mode().Perm())
	}
}

func TestResolvePrivateKeyPathFromRawValue(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	t.Setenv("ASC_PRIVATE_KEY", "line1\\nline2")
	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "line1\nline2" {
		t.Fatalf("expected newline expansion, got %q", string(data))
	}
}

func TestCleanupTempPrivateKeysRemovesFile(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")

	encoded := base64.StdEncoding.EncodeToString([]byte("key-data"))
	t.Setenv("ASC_PRIVATE_KEY_B64", encoded)

	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected temp key file to exist, got %v", err)
	}

	CleanupTempPrivateKeys()

	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected temp key file to be removed, got %v", err)
	}
	if privateKeyTempPath != "" {
		t.Fatalf("expected temp key path to be cleared, got %q", privateKeyTempPath)
	}
}

func TestResolvePrivateKeyPathInvalidBase64(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "not-base64")

	if _, err := resolvePrivateKeyPath(); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestCheckMixedCredentialSourcesWarns(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "")

	stdout, stderr := captureOutput(t, func() {
		if err := checkMixedCredentialSources(credentialSource{
			keyID:    "keychain",
			issuerID: "env",
			keyPath:  "env",
		}); err != nil {
			t.Fatalf("expected warning only, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Warning: credentials loaded from multiple sources") {
		t.Fatalf("expected mixed-source warning, got %q", stderr)
	}
}

func TestCheckMixedCredentialSourcesStrictErrors(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = true
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "")

	stdout, stderr := captureOutput(t, func() {
		if err := checkMixedCredentialSources(credentialSource{
			keyID:    "keychain",
			issuerID: "env",
			keyPath:  "env",
		}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestCheckMixedCredentialSourcesStrictAuthEnvErrors(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "yes")

	stdout, stderr := captureOutput(t, func() {
		if err := checkMixedCredentialSources(credentialSource{
			keyID:    "keychain",
			issuerID: "env",
			keyPath:  "env",
		}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestStrictAuthEnabled_EnvTruthyValues(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})

	values := []string{"1", "true", "TRUE", "yes", "y", "on", "On"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			t.Setenv(strictAuthEnvVar, value)
			stdout, stderr := captureOutput(t, func() {
				if !strictAuthEnabled() {
					t.Fatalf("expected strict auth enabled for %q", value)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestStrictAuthEnabled_EnvFalseyValues(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})

	values := []string{"0", "false", "FALSE", "no", "n", "off", "Off"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			t.Setenv(strictAuthEnvVar, value)
			stdout, stderr := captureOutput(t, func() {
				if strictAuthEnabled() {
					t.Fatalf("expected strict auth disabled for %q", value)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestStrictAuthEnabled_InvalidValueWarnsAndDisables(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "maybe")

	stdout, stderr := captureOutput(t, func() {
		if strictAuthEnabled() {
			t.Fatal("expected strict auth to be disabled for invalid value")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "invalid ASC_STRICT_AUTH value \"maybe\"") {
		t.Fatalf("expected invalid value warning, got %q", stderr)
	}
}

func TestGetASCClient_ProfileMissingSkipsEnvFallback(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "personal",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "missing")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	previousProfile := selectedProfile
	selectedProfile = ""
	t.Cleanup(func() {
		selectedProfile = previousProfile
	})

	_, err := getASCClient()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetASCClient_BypassKeychainPrefersEnvOverConfig(t *testing.T) {
	resetPrivateKeyTemp(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	envKeyPath := filepath.Join(tempDir, "AuthKey-Env.p8")
	writeECDSAPEM(t, envKeyPath)

	cfg := &config.Config{
		DefaultKeyName: "config",
		Keys: []config.Credential{
			{
				Name:           "config",
				KeyID:          "CFGKEY",
				IssuerID:       "CFGISS",
				PrivateKeyPath: filepath.Join(tempDir, "missing.p8"),
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", envKeyPath)

	previousProfile := selectedProfile
	selectedProfile = ""
	t.Cleanup(func() {
		selectedProfile = previousProfile
	})

	if _, err := getASCClient(); err != nil {
		t.Fatalf("expected env credentials to override config, got %v", err)
	}
}

func resetPrivateKeyTemp(t *testing.T) {
	t.Helper()
	CleanupTempPrivateKeys()
	t.Cleanup(func() {
		CleanupTempPrivateKeys()
	})
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
}

func writeECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if data == nil {
		t.Fatal("failed to encode PEM")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write key file error: %v", err)
	}
}

func TestProgressEnabled_DisabledByFlag(t *testing.T) {
	previousNoProgress := noProgress
	t.Cleanup(func() {
		noProgress = previousNoProgress
	})

	SetNoProgress(true)
	if ProgressEnabled() {
		t.Fatal("expected ProgressEnabled() to return false when noProgress is true")
	}

	SetNoProgress(false)
	// Progress should still be disabled in tests because stderr is piped (not a TTY)
	if ProgressEnabled() {
		t.Fatal("expected ProgressEnabled() to return false in test environment (stderr not a TTY)")
	}
}

func TestProgressEnabled_DisabledInNonTTY(t *testing.T) {
	previousNoProgress := noProgress
	noProgress = false
	t.Cleanup(func() {
		noProgress = previousNoProgress
	})

	// In test environment, stderr is piped (not a TTY)
	// So ProgressEnabled should return false regardless of flag
	if ProgressEnabled() {
		t.Fatal("expected ProgressEnabled() to return false when stderr is not a TTY")
	}
}

func TestSetNoProgress(t *testing.T) {
	previousNoProgress := noProgress
	t.Cleanup(func() {
		noProgress = previousNoProgress
	})

	SetNoProgress(true)
	if !noProgress {
		t.Fatal("expected noProgress to be true after SetNoProgress(true)")
	}

	SetNoProgress(false)
	if noProgress {
		t.Fatal("expected noProgress to be false after SetNoProgress(false)")
	}
}
