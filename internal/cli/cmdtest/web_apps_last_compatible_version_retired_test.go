package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// The retired web-session read. `asc web apps last-compatible-version view`
// was never part of a tagged release, so it is removed outright rather than
// deprecated. Callers land on the standard unknown-command error with the
// group help pointer, and no web session is resolved.
func TestWebAppsLastCompatibleVersionIsRetired(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stubTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("retired command must not send a request: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"web", "apps", "last-compatible-version", "view", "--app", "6759231657"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Error: unknown command `asc web apps last-compatible-version`") {
		t.Fatalf("stderr = %q, want unknown-command error", stderr)
	}
	if !strings.Contains(stderr, "asc web apps --help") {
		t.Fatalf("stderr = %q, want the group help pointer", stderr)
	}
}
