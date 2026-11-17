package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestWebAnalyticsExperimentsUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_APP_ID", "")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing app", []string{"list"}, "--app is required"},
		{"invalid app", []string{"list", "--app", "../other"}, "--app must be a numeric App ID"},
		{"missing experiment", []string{"results"}, "--experiment is required"},
		{"invalid experiment", []string{"results", "--experiment", "../settings"}, "--experiment must be a valid UUID"},
		{"invalid baseline", []string{"results", "--experiment", "11111111-1111-1111-1111-111111111111", "--baseline", "unknown"}, "--baseline must be default or a treatment UUID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"web", "analytics", "product-pages", "experiments"}, tc.args...)
			stdout, stderr, err := runRootWithDeps(t, nil, args...)
			if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage || stdout != "" {
				t.Fatalf("expected usage exit without data: err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
			assertUsageErrorStderr(t, stderr, tc.want)
		})
	}
}
