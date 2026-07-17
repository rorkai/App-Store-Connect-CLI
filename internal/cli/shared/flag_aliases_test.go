package shared

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestDeprecatedStringFlagAliasApply(t *testing.T) {
	tests := []struct {
		name           string
		canonicalArg   string
		aliasArg       string
		want           string
		wantErr        string
		wantWarning    bool
		parseCanonical bool
		parseAlias     bool
	}{
		{name: "canonical only", canonicalArg: "canonical", want: "canonical", parseCanonical: true},
		{name: "alias only", aliasArg: "alias", want: "alias", wantWarning: true, parseAlias: true},
		{name: "matching values", canonicalArg: "same", aliasArg: "same", want: "same", wantWarning: true, parseCanonical: true, parseAlias: true},
		{name: "conflicting values", canonicalArg: "canonical", aliasArg: "alias", want: "canonical", wantErr: "--legacy conflicts with --canonical", wantWarning: true, parseCanonical: true, parseAlias: true},
		{name: "empty canonical conflicts with alias", aliasArg: "alias", wantErr: "--legacy conflicts with --canonical", wantWarning: true, parseCanonical: true, parseAlias: true},
		{name: "empty alias conflicts with canonical", canonicalArg: "canonical", want: "canonical", wantErr: "--legacy conflicts with --canonical", wantWarning: true, parseCanonical: true, parseAlias: true},
		{name: "matching empty values", wantWarning: true, parseCanonical: true, parseAlias: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			canonical := fs.String("canonical", "", "Canonical value")
			alias := BindDeprecatedStringFlagAlias(fs, "legacy", "canonical")

			args := []string{}
			if test.parseCanonical {
				args = append(args, "--canonical", test.canonicalArg)
			}
			if test.parseAlias {
				args = append(args, "--legacy", test.aliasArg)
			}
			if err := fs.Parse(args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			_, stderr := captureOutput(t, func() {
				err := alias.Apply(canonical)
				if test.wantErr == "" && err != nil {
					t.Fatalf("Apply() error: %v", err)
				}
				if test.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), test.wantErr) {
						t.Fatalf("Apply() error = %v, want containing %q", err, test.wantErr)
					}
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("Apply() error = %v, want usage error", err)
					}
				}
			})

			if *canonical != test.want {
				t.Fatalf("canonical value = %q, want %q", *canonical, test.want)
			}
			if test.wantErr != "" && !strings.Contains(stderr, "Error: "+test.wantErr) {
				t.Fatalf("stderr = %q, want usage error", stderr)
			}
			if test.wantWarning && !strings.Contains(stderr, "Warning: `--legacy` is deprecated. Use `--canonical`.") {
				t.Fatalf("stderr = %q, want deprecation warning", stderr)
			}
			if test.wantErr == "" && !test.wantWarning && stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestBindDeprecatedStringFlagAliasHidesCompatibilityFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.String("canonical", "", "Canonical value")
	BindDeprecatedStringFlagAlias(fs, "legacy", "canonical")

	if fs.Lookup("legacy") == nil {
		t.Fatal("compatibility flag was not registered")
	}
	for _, visible := range VisibleHelpFlags(fs) {
		if visible.Name == "legacy" {
			t.Fatal("compatibility flag should be hidden from canonical help")
		}
	}
}

func TestDeprecatedIntFlagAliasApply(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		want         int
		wantErr      string
		warningCount int
		aliasWasSet  bool
	}{
		{name: "canonical only", args: []string{"--canonical", "7"}, want: 7},
		{name: "alias only", args: []string{"--legacy", "8"}, want: 8, warningCount: 1, aliasWasSet: true},
		{name: "both spellings conflict", args: []string{"--canonical", "9", "--legacy", "9"}, want: 9, wantErr: "--legacy conflicts with --canonical", warningCount: 1, aliasWasSet: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			canonical := fs.Int("canonical", 0, "Canonical value")
			alias := BindDeprecatedIntFlagAlias(fs, "legacy", "canonical")
			if err := fs.Parse(test.args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			_, stderr := captureOutput(t, func() {
				err := alias.Apply(canonical)
				if test.wantErr == "" && err != nil {
					t.Fatalf("Apply() error: %v", err)
				}
				if test.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), test.wantErr) {
						t.Fatalf("Apply() error = %v, want containing %q", err, test.wantErr)
					}
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("Apply() error = %v, want usage error", err)
					}
				}
			})

			if *canonical != test.want {
				t.Fatalf("canonical value = %d, want %d", *canonical, test.want)
			}
			if got := strings.Count(stderr, "Warning: `--legacy` is deprecated. Use `--canonical`."); got != test.warningCount {
				t.Fatalf("warning count = %d, want %d; stderr=%q", got, test.warningCount, stderr)
			}
			if got := alias.WasProvided(); got != test.aliasWasSet {
				t.Fatalf("WasProvided() = %t, want %t", got, test.aliasWasSet)
			}
		})
	}
}

func TestDeprecatedIntFlagAliasParsingMatchesCanonical(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      int
		wantError bool
	}{
		{name: "decimal", value: "42", want: 42},
		{name: "octal", value: "0664", want: 436},
		{name: "hexadecimal", value: "0x10", want: 16},
		{name: "negative hexadecimal", value: "-0x10", want: -16},
		{name: "invalid octal", value: "08", wantError: true},
		{name: "invalid text", value: "invalid", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			canonical := fs.Int("canonical", 0, "Canonical value")
			alias := BindDeprecatedIntFlagAlias(fs, "legacy", "canonical")

			err := fs.Parse([]string{"--legacy", test.value})
			if test.wantError {
				if err == nil {
					t.Fatal("Parse() error = nil, want invalid integer error")
				}
				if alias.WasProvided() {
					t.Fatal("WasProvided() = true after invalid alias value")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			captureOutput(t, func() {
				if err := alias.Apply(canonical); err != nil {
					t.Fatalf("Apply() error: %v", err)
				}
			})
			if *canonical != test.want {
				t.Fatalf("canonical value = %d, want %d", *canonical, test.want)
			}
			if !alias.WasProvided() {
				t.Fatal("WasProvided() = false after valid alias value")
			}
		})
	}
}

func TestBindDeprecatedIntFlagAliasHidesCompatibilityFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.Int("canonical", 0, "Canonical value")
	BindDeprecatedIntFlagAlias(fs, "legacy", "canonical")

	if fs.Lookup("legacy") == nil {
		t.Fatal("compatibility flag was not registered")
	}
	for _, visible := range VisibleHelpFlags(fs) {
		if visible.Name == "legacy" {
			t.Fatal("compatibility flag should be hidden from canonical help")
		}
	}
}
