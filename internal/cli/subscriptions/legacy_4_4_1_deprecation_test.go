package subscriptions

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// The App Store Connect API 4.4.1 product-scoped command families were removed
// in 5.0.0. Their parents must no longer register them anywhere in the tree.
func TestSubscriptionsLegacy441CommandsAreNotRegistered(t *testing.T) {
	testDeps, _ := newSubscriptionsTestDeps()
	tests := []struct {
		name    string
		parent  *ffcli.Command
		removed []string
	}{
		{"subscriptions", NewSubscriptionsCommand(testDeps), []string{"localizations", "images", "submit"}},
		{"groups", newSubscriptionsGroupsCommand(testDeps), []string{"localizations", "submit"}},
		{"review", SubscriptionsReviewCommand(), []string{"submit", "submit-group"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, sub := range tt.parent.Subcommands {
				if sub == nil {
					continue
				}
				for _, removed := range tt.removed {
					if sub.Name == removed {
						t.Fatalf("%s still registers removed subcommand %q", tt.name, removed)
					}
				}
			}
		})
	}
}

func captureSubscriptionsLegacy441Stderr(t *testing.T, run func()) string {
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

func TestSubscriptionVersionScoped441CommandsAreNotDeprecated(t *testing.T) {
	testDeps, _ := newSubscriptionsTestDeps()
	commands := []*ffcli.Command{
		SubscriptionsVersionImagesListCommand(),
		SubscriptionsVersionLocalizationsListCommand(),
		newSubscriptionsGroupsVersionLocalizationsCommand(testDeps),
		newSubscriptionsGroupsVersionLocalizationsListCommand(testDeps),
	}
	for _, cmd := range commands {
		if strings.Contains(cmd.ShortHelp, "DEPRECATED:") || strings.Contains(cmd.LongHelp, "DEPRECATED:") {
			t.Fatalf("%s unexpectedly deprecated: %q", cmd.ShortUsage, cmd.ShortHelp)
		}
		if strings.Contains(cmd.LongHelp, "remain unchanged") {
			t.Fatalf("%s LongHelp presents deprecated product-scoped behavior as unchanged: %q", cmd.ShortUsage, cmd.LongHelp)
		}
	}
}

func TestSubscriptions441ParentHelpPromotesVersionScopedResources(t *testing.T) {
	testDeps, _ := newSubscriptionsTestDeps()
	tests := []struct {
		name        string
		command     *ffcli.Command
		legacy      string
		replacement string
	}{
		{"subscriptions", NewSubscriptionsCommand(testDeps), "asc subscriptions review submit", "asc review items add"},
		{"review", SubscriptionsReviewCommand(), "asc subscriptions review submit", "asc review items add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.command.LongHelp, tt.legacy) {
				t.Fatalf("LongHelp still teaches removed command %q: %q", tt.legacy, tt.command.LongHelp)
			}
			if !strings.Contains(tt.command.LongHelp, tt.replacement) {
				t.Fatalf("LongHelp = %q, want replacement %q", tt.command.LongHelp, tt.replacement)
			}
		})
	}

	setup := SubscriptionsSetupCommand()
	if strings.Contains(setup.LongHelp, " --locale ") || strings.Contains(setup.LongHelp, " --group-locale ") {
		t.Fatalf("setup LongHelp still teaches deprecated localization flags: %q", setup.LongHelp)
	}
	if !strings.Contains(setup.LongHelp, "deprecated v1") {
		t.Fatalf("setup LongHelp lacks localization migration guidance: %q", setup.LongHelp)
	}
}

func TestSubscriptionsSetupWarnsOnceOnlyForLegacyLocalizations(t *testing.T) {
	tests := []struct {
		name             string
		localizationArgs []string
		wantWarning      bool
	}{
		{"without localizations", nil, false},
		{"subscription localization", []string{"--locale", "en-US", "--display-name", "Pro"}, true},
		{"group localization", []string{"--group-locale", "en-US", "--group-display-name", "Premium"}, true},
		{"both localization families", []string{"--locale", "en-US", "--display-name", "Pro", "--group-locale", "en-US", "--group-display-name", "Premium"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := SubscriptionsSetupCommand()
			args := []string{"--group-id", "group-1", "--reference-name", "Pro", "--product-id", "com.example.pro", "--subscription-period", "ONE_MONTH", "--tier", "-1"}
			args = append(args, tt.localizationArgs...)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			stderr := captureSubscriptionsLegacy441Stderr(t, func() {
				_ = cmd.Exec(context.Background(), cmd.FlagSet.Args())
			})
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
