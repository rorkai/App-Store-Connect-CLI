package cmdtest

import (
	"errors"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestEveryNextCommandRejectsLimit(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, errors.New("client factory must not run when --next conflicts with --limit")
	})
	defer restore()

	var paths [][]string
	var walk func(*ffcli.Command, []string)
	walk = func(cmd *ffcli.Command, path []string) {
		path = append(append([]string(nil), path...), cmd.Name)
		for _, sub := range cmd.Subcommands {
			walk(sub, path)
		}
		if cmd.Exec != nil && cmd.FlagSet != nil && cmd.FlagSet.Lookup("next") != nil && cmd.FlagSet.Lookup("limit") != nil {
			paths = append(paths, path)
		}
	}
	for _, sub := range RootCommand("test").Subcommands {
		walk(sub, nil)
	}
	if len(paths) < 250 {
		t.Fatalf("found %d commands with --next and --limit, want the full command tree", len(paths))
	}

	for _, path := range paths {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			args := append(path, "--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=AQ", "--limit", "5")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "--next cannot be combined with --limit") {
				t.Fatalf("stderr = %q, want --next/--limit conflict", stderr)
			}
		})
	}
}

func TestNextRejectsIgnoredQueryFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, errors.New("client factory must not run when --next conflicts with a query flag")
	})
	defer restore()

	tests := []struct {
		command string
		flag    string
		value   string
	}{
		{"android-ios-mapping list --app APP_ID", "fields", "packageName"},
		{"app-clips advanced-experiences list", "status", "ACTIVE"},
		{"app-clips default-experiences localizations list", "locale", "en-US"},
		{"app-clips list", "bundle-id", "com.example.clip"},
		{"app-tags list", "visible-in-app-store", "true"},
		{"app-tags territories", "fields", "currency"},
		{"apps app-encryption-declarations list", "build-id", "BUILD_ID"},
		{"apps info territory-age-ratings list", "fields", "ageRating"},
		{"apps search-keywords list", "platform", "IOS"},
		{"background-assets list", "archived", "true"},
		{"background-assets versions list", "locale", "en-US"},
		{"builds test-notes list", "locale", "en-US"},
		{"builds uploads list", "state", "COMPLETE"},
		{"devices list", "platform", "IOS"},
		{"encryption declarations list", "build-id", "BUILD_ID"},
		{"game-center details metrics classic-matchmaking", "granularity", "P1D"},
		{"game-center matchmaking metrics queue-sizes", "granularity", "P1D"},
		{"game-center matchmaking metrics rule-errors", "filter-queue", "QUEUE_ID"},
		{"iap pricing price-points list", "territory", "USA"},
		{"localizations list", "locale", "en-US"},
		{"merchant-ids certificates list", "display-name", "Cert"},
		{"nominations list", "status", "DRAFT"},
		{"nominations list", "has-in-app-events", "false"},
		{"pass-type-ids certificates list", "serial-number", "SERIAL"},
		{"performance diagnostics list --build-id BUILD_ID", "diagnostic-type", "HANGS"},
		{"pricing price-points", "territory", "USA"},
		{"product-pages experiments list", "state", "READY_FOR_REVIEW"},
		{"subscriptions offers win-back list", "include", "prices"},
		{"subscriptions offers win-back prices", "territory", "USA"},
		{"testflight agreements list", "fields", "agreementText"},
		{"testflight app-localizations list", "locale", "en-US"},
		{"testflight crashes list", "device-model", "iPhone15,3"},
		{"testflight feedback list", "os-version", "17.2"},
		{"testflight metrics app-testers", "period", "P7D"},
		{"testflight pre-release list", "platform", "IOS"},
		{"testflight recruitment options", "fields", "deviceFamilyOsVersions"},
		{"testflight testers list", "email", "tester@example.com"},
		{"testflight testers metrics", "period", "P7D"},
		{"versions list", "state", "READY_FOR_SALE"},
		{"webhooks deliveries", "created-after", "2026-01-01T00:00:00Z"},
		{"xcode-cloud build-runs list", "sort", "-number"},
	}

	for _, test := range tests {
		t.Run(test.command+" --"+test.flag, func(t *testing.T) {
			args := append(
				strings.Fields(test.command),
				"--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=AQ",
				"--"+test.flag, test.value,
			)
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if want := "--next cannot be combined with --" + test.flag; !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}
