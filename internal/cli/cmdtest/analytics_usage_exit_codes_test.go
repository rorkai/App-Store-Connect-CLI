package cmdtest

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

const analyticsUsageExitRequestID = "11111111-1111-1111-1111-111111111111"

// TestAnalyticsInputValidationReturnsUsageExitCode locks the usage-error
// contract for analytics flag validation: every pre-request flag check must
// print "Error: <message>" to stderr and exit with code 2, not the generic
// runtime failure code.
func TestAnalyticsInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
		appEnv  string
	}{
		{
			name:    "requests limit above maximum",
			args:    []string{"analytics", "requests", "--app", "123456789", "--limit", "201"},
			wantErr: "analytics requests: --limit must be between 1 and 200",
		},
		{
			name:    "requests limit below minimum",
			args:    []string{"analytics", "requests", "--app", "123456789", "--limit", "-1"},
			wantErr: "analytics requests: --limit must be between 1 and 200",
		},
		{
			name:    "requests invalid next",
			args:    []string{"analytics", "requests", "--next", "http://api.appstoreconnect.apple.com/v1/analyticsReportRequests"},
			wantErr: "analytics requests: --next must be an App Store Connect URL",
		},
		{
			name:    "requests next uses unsupported top-level collection",
			args:    []string{"analytics", "requests", "--next", "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests?limit=1"},
			wantErr: "analytics requests: --next must point to an app's analytics report requests",
		},
		{
			name:    "requests next points at unrelated resource",
			args:    []string{"analytics", "requests", "--next", "https://api.appstoreconnect.apple.com/v1/apps?limit=1"},
			wantErr: "analytics requests: --next must point to an app's analytics report requests",
		},
		{
			name:    "requests app differs from cursor",
			args:    []string{"analytics", "requests", "--app", "123456789", "--next", "https://api.appstoreconnect.apple.com/v1/apps/6759231657/analyticsReportRequests?limit=1"},
			wantErr: "analytics requests: the selected app must match the app in --next",
		},
		{
			name:    "requests environment app differs from cursor",
			args:    []string{"analytics", "requests", "--next", "https://api.appstoreconnect.apple.com/v1/apps/6759231657/analyticsReportRequests?limit=1"},
			wantErr: "analytics requests: the selected app must match the app in --next",
			appEnv:  "123456789",
		},
		{
			name:    "requests invalid request id",
			args:    []string{"analytics", "requests", "--request-id", "not-a-uuid"},
			wantErr: "analytics requests: --request-id must be a valid UUID",
		},
		{
			name:    "requests delete invalid request id",
			args:    []string{"analytics", "requests", "delete", "--request-id", "not-a-uuid", "--confirm"},
			wantErr: "analytics requests delete: --request-id must be a valid UUID",
		},
		{
			name:    "view invalid request id",
			args:    []string{"analytics", "view", "--request-id", "not-a-uuid"},
			wantErr: "analytics view: --request-id must be a valid UUID",
		},
		{
			name:    "view invalid instance id",
			args:    []string{"analytics", "view", "--request-id", analyticsUsageExitRequestID, "--instance-id", "bad/instance"},
			wantErr: analyticsReservedSegmentUsageError("analytics view", "--instance-id", "bad/instance"),
		},
		{
			name:    "view limit above maximum",
			args:    []string{"analytics", "view", "--request-id", analyticsUsageExitRequestID, "--limit", "201"},
			wantErr: "analytics view: --limit must be between 1 and 200",
		},
		{
			name:    "view invalid next",
			args:    []string{"analytics", "view", "--next", "http://api.appstoreconnect.apple.com/v1/analyticsReportRequests/x/reports"},
			wantErr: "analytics view: --next must be an App Store Connect URL",
		},
		{
			name:    "view request id differs from cursor",
			args:    []string{"analytics", "view", "--request-id", analyticsUsageExitRequestID, "--next", "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/22222222-2222-2222-2222-222222222222/reports?cursor=AQ"},
			wantErr: "analytics view: --request-id must match the report request in --next",
		},
		{
			name:    "view next points at request list",
			args:    []string{"analytics", "view", "--next", "https://api.appstoreconnect.apple.com/v1/apps/6759231657/analyticsReportRequests?limit=1"},
			wantErr: "analytics view: --next must point to an analytics report request's reports",
		},
		{
			name:    "download invalid request id",
			args:    []string{"analytics", "download", "--request-id", "not-a-uuid", "--instance-id", "instance-1"},
			wantErr: "analytics download: --request-id must be a valid UUID",
		},
		{
			name:    "download invalid instance id",
			args:    []string{"analytics", "download", "--request-id", analyticsUsageExitRequestID, "--instance-id", "bad/instance"},
			wantErr: analyticsReservedSegmentUsageError("analytics download", "--instance-id", "bad/instance"),
		},
		{
			name: "download invalid segment id",
			args: []string{
				"analytics", "download",
				"--request-id", analyticsUsageExitRequestID,
				"--instance-id", "instance-1",
				"--segment-id", "bad/segment",
			},
			wantErr: analyticsReservedSegmentUsageError("analytics download", "--segment-id", "bad/segment"),
		},
		{
			name:    "reports links limit above maximum",
			args:    []string{"analytics", "reports", "links", "--report-id", "report-1", "--limit", "201"},
			wantErr: "analytics reports links: --limit must be between 1 and 200",
		},
		{
			name:    "reports links invalid next",
			args:    []string{"analytics", "reports", "links", "--next", "http://api.appstoreconnect.apple.com/v1/analyticsReports/x/instances"},
			wantErr: "analytics reports links: --next must be an App Store Connect URL",
		},
		{
			name:    "instances view invalid instance id",
			args:    []string{"analytics", "instances", "view", "--instance-id", "bad/instance"},
			wantErr: analyticsReservedSegmentUsageError("analytics instances view", "--instance-id", "bad/instance"),
		},
		{
			name:    "instances links limit above maximum",
			args:    []string{"analytics", "instances", "links", "--instance-id", "instance-1", "--limit", "201"},
			wantErr: "analytics instances links: --limit must be between 1 and 200",
		},
		{
			name:    "instances links invalid next",
			args:    []string{"analytics", "instances", "links", "--next", "http://api.appstoreconnect.apple.com/v1/analyticsReportInstances/x/segments"},
			wantErr: "analytics instances links: --next must be an App Store Connect URL",
		},
		{
			name:    "instances links invalid instance id",
			args:    []string{"analytics", "instances", "links", "--instance-id", "bad/instance"},
			wantErr: analyticsReservedSegmentUsageError("analytics instances links", "--instance-id", "bad/instance"),
		},
		{
			name:    "segments view invalid segment id",
			args:    []string{"analytics", "segments", "view", "--segment-id", "bad/segment"},
			wantErr: analyticsReservedSegmentUsageError("analytics segments view", "--segment-id", "bad/segment"),
		},
		{
			name:    "reports get missing report id",
			args:    []string{"analytics", "reports", "view"},
			wantErr: "--report-id is required",
		},
		{
			name:    "reports relationships missing report id",
			args:    []string{"analytics", "reports", "links"},
			wantErr: "--report-id is required",
		},
		{
			name:    "instances get missing instance id",
			args:    []string{"analytics", "instances", "view"},
			wantErr: "--instance-id is required",
		},
		{
			name:    "instances relationships missing instance id",
			args:    []string{"analytics", "instances", "links"},
			wantErr: "--instance-id is required",
		},
		{
			name:    "segments get missing segment id",
			args:    []string{"analytics", "segments", "view"},
			wantErr: "--segment-id is required",
		},
		{
			name:    "requests delete missing request id",
			args:    []string{"analytics", "requests", "delete", "--confirm"},
			wantErr: "--request-id is required",
		},
		{
			name:    "requests delete missing confirm",
			args:    []string{"analytics", "requests", "delete", "--request-id", "11111111-1111-1111-1111-111111111111"},
			wantErr: "--confirm is required",
		},
		{
			name:    "request reuse existing missing app",
			args:    []string{"analytics", "request", "--access-type", "ONGOING", "--reuse-existing"},
			wantErr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:    "request reuse existing missing access type",
			args:    []string{"analytics", "request", "--app", "APP_ID", "--reuse-existing"},
			wantErr: "--access-type is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.appEnv != "" {
				t.Setenv("ASC_APP_ID", test.appEnv)
			}
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

// analyticsReservedSegmentUsageError builds the complete diagnostic a command
// prints when a resource identifier carries a reserved path delimiter. The
// trailing quoted set mirrors asc.ValidateResourcePathSegment, which renders
// the reserved characters with %q.
func analyticsReservedSegmentUsageError(commandPath, flagName, id string) string {
	return commandPath + ": " + flagName + ": resource identifier " + strconv.Quote(id) +
		` must be a single path segment without any of "/?#%\\"`
}

// assertUsageErrorStderr locks the usage-error stderr contract: the diagnostic
// is the complete first line, formatted exactly as "Error: <message>\n", and it
// appears exactly once.
//
// The whole buffer is deliberately not compared. shared.UsageError returns an
// error wrapping flag.ErrHelp, so ffcli's Run renders the command's full usage
// page after the diagnostic; asserting that too would turn every usage test
// into a help-text golden test that `make generate-command-docs` already
// covers. Pinning the exact diagnostic line and its occurrence count still
// rejects extra diagnostics, duplicated errors, and changed formatting.
func assertUsageErrorStderr(t *testing.T, stderr, wantMessage string) {
	t.Helper()

	wantLine := "Error: " + wantMessage + "\n"
	if !strings.HasPrefix(stderr, wantLine) {
		t.Fatalf("stderr = %q, want it to start with %q", stderr, wantLine)
	}
	if got := strings.Count(stderr, "Error: "); got != 1 {
		t.Fatalf("stderr = %q, want exactly one %q diagnostic, got %d", stderr, "Error: ", got)
	}
}
