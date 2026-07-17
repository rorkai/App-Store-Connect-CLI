package subscriptions

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestSubscriptionsLegacy441CommandsAreDeprecated(t *testing.T) {
	tests := []struct {
		name        string
		command     func() *ffcli.Command
		oldCommand  string
		replacement string
	}{
		{"images list", SubscriptionsImagesListCommand, "asc subscriptions images list", "asc subscriptions versions images list --version-id \"SUBSCRIPTION_VERSION_ID\""},
		{"images view", SubscriptionsImagesGetCommand, "asc subscriptions images view", "asc subscriptions versions images view --id \"IMAGE_ID\""},
		{"images create", SubscriptionsImagesCreateCommand, "asc subscriptions images create", "asc subscriptions versions images upload --version-id \"SUBSCRIPTION_VERSION_ID\" --file \"./image.png\""},
		{"images update", SubscriptionsImagesUpdateCommand, "asc subscriptions images update", "asc subscriptions versions images upload --version-id \"SUBSCRIPTION_VERSION_ID\" --file \"./image.png\""},
		{"images delete", SubscriptionsImagesDeleteCommand, "asc subscriptions images delete", "asc subscriptions versions images delete --id \"IMAGE_ID\" --confirm"},
		{"localizations list", SubscriptionsLocalizationsListCommand, "asc subscriptions localizations list", "asc subscriptions versions localizations list --version-id \"SUBSCRIPTION_VERSION_ID\""},
		{"localizations view", SubscriptionsLocalizationsGetCommand, "asc subscriptions localizations view", "asc subscriptions versions localizations view --id \"LOCALIZATION_ID\""},
		{"localizations create", SubscriptionsLocalizationsCreateCommand, "asc subscriptions localizations create", "asc subscriptions versions localizations create --version-id \"SUBSCRIPTION_VERSION_ID\" --name \"NAME\" --locale \"LOCALE\""},
		{"localizations update", SubscriptionsLocalizationsUpdateCommand, "asc subscriptions localizations update", "asc subscriptions versions localizations update --id \"LOCALIZATION_ID\" --name \"NAME\""},
		{"localizations delete", SubscriptionsLocalizationsDeleteCommand, "asc subscriptions localizations delete", "asc subscriptions versions localizations delete --id \"LOCALIZATION_ID\" --confirm"},
		{"localizations sync", SubscriptionsLocalizationsSyncCommand, "asc subscriptions localizations sync", "asc subscriptions versions localizations"},
		{"group localizations list", SubscriptionsGroupsLocalizationsListCommand, "asc subscriptions groups localizations list", "asc subscriptions groups versions localizations list --version-id \"GROUP_VERSION_ID\""},
		{"group localizations view", SubscriptionsGroupsLocalizationsGetCommand, "asc subscriptions groups localizations view", "asc subscriptions groups versions localizations view --id \"LOCALIZATION_ID\""},
		{"group localizations create", SubscriptionsGroupsLocalizationsCreateCommand, "asc subscriptions groups localizations create", "asc subscriptions groups versions localizations create --version-id \"GROUP_VERSION_ID\" --name \"NAME\" --locale \"LOCALE\""},
		{"group localizations update", SubscriptionsGroupsLocalizationsUpdateCommand, "asc subscriptions groups localizations update", "asc subscriptions groups versions localizations update --id \"LOCALIZATION_ID\" --name \"NAME\""},
		{"group localizations delete", SubscriptionsGroupsLocalizationsDeleteCommand, "asc subscriptions groups localizations delete", "asc subscriptions groups versions localizations delete --id \"LOCALIZATION_ID\" --confirm"},
		{"group localizations sync", SubscriptionsGroupsLocalizationsSyncCommand, "asc subscriptions groups localizations sync", "asc subscriptions groups versions localizations"},
		{"submit", subscriptionsReviewSubmitCommandForTest, "asc subscriptions review submit", "asc review items add --submission \"SUBMISSION_ID\" --item-type subscriptionVersions --item-id \"SUBSCRIPTION_VERSION_ID\""},
		{"group submit", subscriptionsReviewSubmitGroupCommandForTest, "asc subscriptions review submit-group", "asc review items add --submission \"SUBMISSION_ID\" --item-type subscriptionGroupVersions --item-id \"GROUP_VERSION_ID\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.command()
			if !strings.HasPrefix(cmd.ShortHelp, "DEPRECATED:") {
				t.Fatalf("ShortHelp = %q, want DEPRECATED prefix", cmd.ShortHelp)
			}
			if !strings.Contains(cmd.LongHelp, tt.replacement) {
				t.Fatalf("LongHelp = %q, want replacement %q", cmd.LongHelp, tt.replacement)
			}

			stderr := captureSubscriptionsLegacy441Stderr(t, func() {
				_ = cmd.Exec(context.Background(), nil)
			})
			if strings.Count(stderr, "Warning:") != 1 {
				t.Fatalf("stderr = %q, want exactly one warning", stderr)
			}
			if !strings.Contains(stderr, "`"+tt.oldCommand+"`") || !strings.Contains(stderr, "`"+tt.replacement+"`") {
				t.Fatalf("stderr = %q, want old command %q and replacement %q", stderr, tt.oldCommand, tt.replacement)
			}
		})
	}
}

func subscriptionsReviewSubmitCommandForTest() *ffcli.Command {
	return SubscriptionsReviewCommand().Subcommands[2]
}

func subscriptionsReviewSubmitGroupCommandForTest() *ffcli.Command {
	return SubscriptionsReviewCommand().Subcommands[3]
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
	commands := []*ffcli.Command{
		SubscriptionsVersionImagesListCommand(),
		SubscriptionsVersionLocalizationsListCommand(),
		SubscriptionsGroupsVersionLocalizationsCommand(),
		SubscriptionsGroupsVersionLocalizationsListCommand(),
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
	tests := []struct {
		name        string
		command     *ffcli.Command
		legacy      string
		replacement string
	}{
		{"subscriptions", SubscriptionsCommand(), "asc subscriptions review submit", "asc review items add"},
		{"localizations", SubscriptionsLocalizationsCommand(), "asc subscriptions localizations list", "asc subscriptions versions localizations list"},
		{"images", SubscriptionsImagesCommand(), "asc subscriptions images list", "asc subscriptions versions images list"},
		{"group localizations", SubscriptionsGroupsLocalizationsCommand(), "asc subscriptions groups localizations list", "asc subscriptions groups versions localizations list"},
		{"review", SubscriptionsReviewCommand(), "asc subscriptions review submit", "asc review items add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.command.LongHelp, tt.legacy) {
				t.Fatalf("LongHelp still teaches deprecated command %q: %q", tt.legacy, tt.command.LongHelp)
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

func TestSubscriptions441ExperimentalSyncHelpPreservesBothLifecycleLabels(t *testing.T) {
	commands := []*ffcli.Command{
		SubscriptionsLocalizationsSyncCommand(),
		SubscriptionsGroupsLocalizationsSyncCommand(),
	}
	for _, cmd := range commands {
		if !strings.HasPrefix(cmd.ShortHelp, "DEPRECATED: [experimental] ") {
			t.Fatalf("%s ShortHelp = %q, want deprecation and experimental labels", cmd.ShortUsage, cmd.ShortHelp)
		}
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
