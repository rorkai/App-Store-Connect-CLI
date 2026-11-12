package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestTestFlightInputValidationReturnsUsageExitCode locks the usage-error contract
// for testflight pre-request flag validation: every check must print
// "Error: <message>" to stderr, naming the command path the operator invoked,
// and exit with code 2 rather than the generic runtime failure code.
//
// Part of #518.
func TestTestFlightInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-localizations list app 123 limit 201",
			args:    []string{"testflight", "app-localizations", "list", "--app", "123", "--limit", "201"},
			wantErr: "testflight app-localizations list: --limit must be between 1 and 200",
		},
		{
			name:    "app-localizations list app 123 next non apple url",
			args:    []string{"testflight", "app-localizations", "list", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "testflight app-localizations list: --next must be an App Store Connect URL",
		},
		{
			name:    "groups list limit 201",
			args:    []string{"testflight", "groups", "list", "--limit", "201"},
			wantErr: "groups list: --limit must be between 1 and 200",
		},
		{
			name:    "groups list next non apple url",
			args:    []string{"testflight", "groups", "list", "--next", "http://example.com/x"},
			wantErr: "groups list: --next must be an App Store Connect URL",
		},
		{
			name:    "groups links view group-id g type betaTesters limit 201",
			args:    []string{"testflight", "groups", "links", "view", "--group-id", "g", "--type", "betaTesters", "--limit", "201"},
			wantErr: "testflight groups links view: --limit must be between 1 and 200",
		},
		{
			name:    "groups links view group-id g type betaTesters next non apple url",
			args:    []string{"testflight", "groups", "links", "view", "--group-id", "g", "--type", "betaTesters", "--next", "http://example.com/x"},
			wantErr: "testflight groups links view: --next must be an App Store Connect URL",
		},
		{
			name:    "testers list limit 201",
			args:    []string{"testflight", "testers", "list", "--limit", "201"},
			wantErr: "testers list: --limit must be between 1 and 200",
		},
		{
			name:    "testers list next non apple url",
			args:    []string{"testflight", "testers", "list", "--next", "http://example.com/x"},
			wantErr: "testers list: --next must be an App Store Connect URL",
		},
		{
			name:    "testers links view tester-id t type apps limit 201",
			args:    []string{"testflight", "testers", "links", "view", "--tester-id", "t", "--type", "apps", "--limit", "201"},
			wantErr: "testflight testers links view: --limit must be between 1 and 200",
		},
		{
			name:    "testers links view tester-id t type apps next non apple url",
			args:    []string{"testflight", "testers", "links", "view", "--tester-id", "t", "--type", "apps", "--next", "http://example.com/x"},
			wantErr: "testflight testers links view: --next must be an App Store Connect URL",
		},
		{
			name:    "testers apps list tester-id t limit 201",
			args:    []string{"testflight", "testers", "apps", "list", "--tester-id", "t", "--limit", "201"},
			wantErr: "testflight testers apps list: --limit must be between 1 and 200",
		},
		{
			name:    "testers apps list tester-id t next non apple url",
			args:    []string{"testflight", "testers", "apps", "list", "--tester-id", "t", "--next", "http://example.com/x"},
			wantErr: "testflight testers apps list: --next must be an App Store Connect URL",
		},
		{
			name:    "testers groups list tester-id t limit 201",
			args:    []string{"testflight", "testers", "groups", "list", "--tester-id", "t", "--limit", "201"},
			wantErr: "testflight testers groups list: --limit must be between 1 and 200",
		},
		{
			name:    "testers groups list tester-id t next non apple url",
			args:    []string{"testflight", "testers", "groups", "list", "--tester-id", "t", "--next", "http://example.com/x"},
			wantErr: "testflight testers groups list: --next must be an App Store Connect URL",
		},
		{
			name:    "testers builds list tester-id t limit 201",
			args:    []string{"testflight", "testers", "builds", "list", "--tester-id", "t", "--limit", "201"},
			wantErr: "testflight testers builds list: --limit must be between 1 and 200",
		},
		{
			name:    "testers builds list tester-id t next non apple url",
			args:    []string{"testflight", "testers", "builds", "list", "--tester-id", "t", "--next", "http://example.com/x"},
			wantErr: "testflight testers builds list: --next must be an App Store Connect URL",
		},
		{
			name:    "testers metrics tester-id t app 123 next non apple url",
			args:    []string{"testflight", "testers", "metrics", "--tester-id", "t", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "testflight testers metrics: --next must be an App Store Connect URL",
		},
		{
			name:    "testers metrics tester-id t app 123 period BAD",
			args:    []string{"testflight", "testers", "metrics", "--tester-id", "t", "--app", "123", "--period", "BAD"},
			wantErr: "--period must be one of: P7D, P30D, P90D, P365D",
		},
		{
			name:    "agreements list limit 201",
			args:    []string{"testflight", "agreements", "list", "--limit", "201"},
			wantErr: "agreements list: --limit must be between 1 and 200",
		},
		{
			name:    "agreements list next non apple url",
			args:    []string{"testflight", "agreements", "list", "--next", "http://example.com/x"},
			wantErr: "agreements list: --next must be an App Store Connect URL",
		},
		{
			name:    "review view app 123 limit 201",
			args:    []string{"testflight", "review", "view", "--app", "123", "--limit", "201"},
			wantErr: "testflight review view: --limit must be between 1 and 200",
		},
		{
			name:    "review view app 123 next non apple url",
			args:    []string{"testflight", "review", "view", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "testflight review view: --next must be an App Store Connect URL",
		},
		{
			name:    "review submissions list build-id b limit 201",
			args:    []string{"testflight", "review", "submissions", "list", "--build-id", "b", "--limit", "201"},
			wantErr: "testflight review submissions list: --limit must be between 1 and 200",
		},
		{
			name:    "review submissions list build-id b next non apple url",
			args:    []string{"testflight", "review", "submissions", "list", "--build-id", "b", "--next", "http://example.com/x"},
			wantErr: "testflight review submissions list: --next must be an App Store Connect URL",
		},
		{
			name:    "distribution view build-id b limit 201",
			args:    []string{"testflight", "distribution", "view", "--build-id", "b", "--limit", "201"},
			wantErr: "testflight distribution view: --limit must be between 1 and 200",
		},
		{
			name:    "distribution view build-id b next non apple url",
			args:    []string{"testflight", "distribution", "view", "--build-id", "b", "--next", "http://example.com/x"},
			wantErr: "testflight distribution view: --next must be an App Store Connect URL",
		},
		{
			name:    "pre-release list app 123 limit 201",
			args:    []string{"testflight", "pre-release", "list", "--app", "123", "--limit", "201"},
			wantErr: "testflight pre-release list: --limit must be between 1 and 200",
		},
		{
			name:    "pre-release list app 123 next non apple url",
			args:    []string{"testflight", "pre-release", "list", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "testflight pre-release list: --next must be an App Store Connect URL",
		},
		{
			name:    "pre-release builds list id p limit 201",
			args:    []string{"testflight", "pre-release", "builds", "list", "--id", "p", "--limit", "201"},
			wantErr: "testflight pre-release builds list: --limit must be between 1 and 200",
		},
		{
			name:    "pre-release builds list id p next non apple url",
			args:    []string{"testflight", "pre-release", "builds", "list", "--id", "p", "--next", "http://example.com/x"},
			wantErr: "testflight pre-release builds list: --next must be an App Store Connect URL",
		},
		{
			name:    "pre-release links view id p type builds limit 201",
			args:    []string{"testflight", "pre-release", "links", "view", "--id", "p", "--type", "builds", "--limit", "201"},
			wantErr: "testflight pre-release links view: --limit must be between 1 and 200",
		},
		{
			name:    "pre-release links view id p type builds next non apple url",
			args:    []string{"testflight", "pre-release", "links", "view", "--id", "p", "--type", "builds", "--next", "http://example.com/x"},
			wantErr: "testflight pre-release links view: --next must be an App Store Connect URL",
		},
		{
			name:    "recruitment options limit 201",
			args:    []string{"testflight", "recruitment", "options", "--limit", "201"},
			wantErr: "testflight recruitment options: --limit must be between 1 and 200",
		},
		{
			name:    "recruitment options next non apple url",
			args:    []string{"testflight", "recruitment", "options", "--next", "http://example.com/x"},
			wantErr: "testflight recruitment options: --next must be an App Store Connect URL",
		},
		{
			name:    "recruitment options fields bogus",
			args:    []string{"testflight", "recruitment", "options", "--fields", "bogus"},
			wantErr: "testflight recruitment options: --fields must be one of: deviceFamilyOsVersions",
		},
		{
			name:    "recruitment set group g os-version-filter BAD",
			args:    []string{"testflight", "recruitment", "set", "--group", "g", "--os-version-filter", "BAD"},
			wantErr: "testflight recruitment set: --os-version-filter must use DEVICE_FAMILY=MIN_OS (e.g., IPHONE=26)",
		},
		{
			name:    "recruitment set group g os-version-filter BOGUSFAMILY=26",
			args:    []string{"testflight", "recruitment", "set", "--group", "g", "--os-version-filter", "BOGUSFAMILY=26"},
			wantErr: "testflight recruitment set: --os-version-filter device family must be one of: IPHONE, IPAD, MAC, VISION, APPLE_TV, APPLE_WATCH",
		},
		{
			name:    "metrics app-testers app 123 next non apple url",
			args:    []string{"testflight", "metrics", "app-testers", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "testflight metrics app-testers: --next must be an App Store Connect URL",
		},
		{
			name:    "feedback view without submission-id",
			args:    []string{"testflight", "feedback", "view"},
			wantErr: "--submission-id is required",
		},
		{
			name:    "review app view without id",
			args:    []string{"testflight", "review", "app", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "review submissions view without id",
			args:    []string{"testflight", "review", "submissions", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "distribution build view without id",
			args:    []string{"testflight", "distribution", "build", "view"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageErrorStderr(t, stderr, test.wantErr)
		})
	}
}
