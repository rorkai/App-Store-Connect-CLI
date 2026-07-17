package iap

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestIAPLegacy441CommandsAreDeprecated(t *testing.T) {
	tests := []struct {
		name        string
		command     func() *ffcli.Command
		oldCommand  string
		replacement string
	}{
		{"images list", IAPImagesListCommand, "asc iap images list", "asc iap versions images list --version-id \"IAP_VERSION_ID\""},
		{"images view", IAPImagesGetCommand, "asc iap images view", "asc iap versions images view --image-id \"IMAGE_ID\""},
		{"images create", IAPImagesCreateCommand, "asc iap images create", "asc iap versions images create --version-id \"IAP_VERSION_ID\" --file \"./image.png\""},
		{"images update", IAPImagesUpdateCommand, "asc iap images update", "asc iap versions images create --version-id \"IAP_VERSION_ID\" --file \"./image.png\""},
		{"images delete", IAPImagesDeleteCommand, "asc iap images delete", "asc iap versions images delete --image-id \"IMAGE_ID\" --confirm"},
		{"localizations list", IAPLocalizationsListCommand, "asc iap localizations list", "asc iap versions localizations list --version-id \"IAP_VERSION_ID\""},
		{"localizations create", IAPLocalizationsCreateCommand, "asc iap localizations create", "asc iap versions localizations create --version-id \"IAP_VERSION_ID\" --name \"NAME\" --locale \"LOCALE\""},
		{"localizations update", IAPLocalizationsUpdateCommand, "asc iap localizations update", "asc iap versions localizations update --localization-id \"LOCALIZATION_ID\" --name \"NAME\""},
		{"localizations delete", IAPLocalizationsDeleteCommand, "asc iap localizations delete", "asc iap versions localizations delete --localization-id \"LOCALIZATION_ID\" --confirm"},
		{"submit", IAPSubmitCommand, "asc iap submit", "asc review items add --submission \"SUBMISSION_ID\" --item-type inAppPurchaseVersions --item-id \"IAP_VERSION_ID\""},
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

			stderr := captureIAPLegacy441Stderr(t, func() {
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
	commands := []*ffcli.Command{
		IAPVersionImagesListCommand(),
		IAPVersionLocalizationsListCommand(),
		IAPVersionSubmitCommand(),
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
	tests := []struct {
		name        string
		command     *ffcli.Command
		legacy      string
		replacement string
	}{
		{"iap", IAPCommand(), "asc iap localizations list", "asc iap versions localizations list"},
		{"localizations", IAPLocalizationsCommand(), "asc iap localizations list", "asc iap versions localizations list"},
		{"images", IAPImagesCommand(), "asc iap images list", "asc iap versions images list"},
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

	setup := IAPSetupCommand()
	if strings.Contains(setup.LongHelp, " --locale ") {
		t.Fatalf("setup LongHelp still teaches deprecated localization flags: %q", setup.LongHelp)
	}
	if !strings.Contains(setup.LongHelp, "deprecated v1 localization resource") {
		t.Fatalf("setup LongHelp lacks localization migration guidance: %q", setup.LongHelp)
	}
}

func TestIAP441ParentHelpListsDeprecatedSubmitMigration(t *testing.T) {
	cmd := IAPCommand()
	usage := cmd.UsageFunc(cmd)
	const submitEntry = "\n  submit"
	const migration = "DEPRECATED: App Store Connect API 4.4.1 replaced this resource. Use `asc review items add --submission \"SUBMISSION_ID\" --item-type inAppPurchaseVersions --item-id \"IAP_VERSION_ID\"`."

	if got := strings.Count(usage, submitEntry); got != 1 {
		t.Fatalf("submit entry count = %d, want 1; usage = %q", got, usage)
	}
	if !strings.Contains(usage, migration) {
		t.Fatalf("usage = %q, want migration text %q", usage, migration)
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
			cmd := IAPSetupCommand()
			args := []string{"--app", "123", "--type", "NON_CONSUMABLE", "--reference-name", "Pro", "--product-id", "com.example.pro", "--tier", "-1"}
			args = append(args, tt.localizationArgs...)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			stderr := captureIAPLegacy441Stderr(t, func() {
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
