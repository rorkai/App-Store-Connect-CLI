package iap

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// TestIAPProductScoped441SurfacesAreRemoved locks the 5.0.0 removal of the
// App Store Connect API 4.4.1 product-scoped surfaces. The former
// `asc iap images`, `asc iap localizations`, and `asc iap submit` commands are
// no longer registered, so the CLI reports them as unknown commands instead of
// warning and continuing. The version-scoped commands are the replacements.
func TestIAPProductScoped441SurfacesAreRemoved(t *testing.T) {
	d, _, _ := newTestDeps(nil)
	cmd := NewIAPCommand(d)
	removed := map[string]bool{"images": true, "localizations": true, "submit": true}
	for _, sub := range cmd.Subcommands {
		if sub != nil && removed[sub.Name] {
			t.Fatalf("asc iap still registers removed product-scoped subcommand %q", sub.Name)
		}
	}

	usage := cmd.UsageFunc(cmd)
	for _, entry := range []string{"\n  images", "\n  localizations", "\n  submit", "DEPRECATED"} {
		if strings.Contains(usage, entry) {
			t.Fatalf("usage still lists removed surface %q: %q", entry, usage)
		}
	}
	for _, legacy := range []string{"asc iap images", "asc iap localizations", "asc iap submit"} {
		if strings.Contains(cmd.LongHelp, legacy) {
			t.Fatalf("LongHelp still teaches removed command %q: %q", legacy, cmd.LongHelp)
		}
	}
}

func captureIAPLegacy441Stderr(t *testing.T, run func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = writer
	t.Cleanup(func() { os.Stderr = oldStderr })

	run()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stderr = oldStderr
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(data)
}

func TestIAPVersionScoped441CommandsAreNotDeprecated(t *testing.T) {
	d, _, _ := newTestDeps(nil)
	commands := []*ffcli.Command{
		newIAPVersionImagesListCommand(d),
		newIAPVersionLocalizationsListCommand(d),
		newIAPVersionSubmitCommand(d),
	}
	for _, cmd := range commands {
		if strings.Contains(cmd.ShortHelp, "DEPRECATED:") || strings.Contains(cmd.LongHelp, "DEPRECATED:") {
			t.Fatalf("%s unexpectedly deprecated: %q", cmd.ShortUsage, cmd.ShortHelp)
		}
		if strings.Contains(cmd.LongHelp, "asc iap submit") {
			t.Fatalf("%s LongHelp still teaches deprecated product-scoped submission: %q", cmd.ShortUsage, cmd.LongHelp)
		}
	}
}

func TestIAP441ParentHelpPromotesVersionScopedResources(t *testing.T) {
	d, _, _ := newTestDeps(nil)
	parent := NewIAPCommand(d)
	for _, replacement := range []string{"asc iap versions localizations list", "asc iap versions images create"} {
		if !strings.Contains(parent.LongHelp, replacement) {
			t.Fatalf("LongHelp = %q, want replacement %q", parent.LongHelp, replacement)
		}
	}

	setup := newIAPSetupCommand(d)
	if strings.Contains(setup.LongHelp, " --locale ") {
		t.Fatalf("setup LongHelp still teaches deprecated localization flags: %q", setup.LongHelp)
	}
	if !strings.Contains(setup.LongHelp, "deprecated v1 localization resource") {
		t.Fatalf("setup LongHelp lacks localization migration guidance: %q", setup.LongHelp)
	}
}

func TestIAPSetupWarnsOnlyForLegacyLocalization(t *testing.T) {
	tests := []struct {
		name             string
		localizationArgs []string
		wantWarning      bool
	}{
		{"without localization", nil, false},
		{"with localization", []string{"--locale", "en-US", "--display-name", "Pro"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, stdout, stderrBuf := newTestDeps(nil)
			cmd := newIAPSetupCommand(d)
			args := []string{"--app", "123", "--type", "NON_CONSUMABLE", "--reference-name", "Pro", "--product-id", "com.example.pro", "--tier", "-1"}
			args = append(args, tt.localizationArgs...)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_ = cmd.Exec(context.Background(), cmd.FlagSet.Args())
			stderr := stderrBuf.String()
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			gotWarning := strings.Contains(stderr, "Warning:")
			if gotWarning != tt.wantWarning {
				t.Fatalf("stderr = %q, warning = %v, want %v", stderr, gotWarning, tt.wantWarning)
			}
			if tt.wantWarning && strings.Count(stderr, "Warning:") != 1 {
				t.Fatalf("stderr = %q, want exactly one warning", stderr)
			}
		})
	}
}
