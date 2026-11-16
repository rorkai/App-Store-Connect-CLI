package cmdtest

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestGivenButEmptyAppFlagIsRejected(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	commands := []struct {
		name string
		args []string
	}{
		{name: "read", args: []string{"testflight", "review", "view"}},
		{name: "write", args: []string{"accessibility", "create", "--device-family", "IPHONE"}},
		{name: "delete", args: []string{"testflight", "testers", "remove", "--email", "tester@example.com", "--confirm"}},
	}
	inputs := []struct {
		name  string
		flags []string
		given bool
	}{
		{name: "empty", flags: []string{"--app", ""}, given: true},
		{name: "spaces", flags: []string{"--app", "  \t "}, given: true},
		{name: "equals empty", flags: []string{"--app="}, given: true},
		{name: "omitted"},
	}

	for _, command := range commands {
		for _, input := range inputs {
			for _, envApp := range []string{"", "app-1"} {
				name := command.name + "/" + input.name + "/ASC_APP_ID=" + envApp
				t.Run(name, func(t *testing.T) {
					t.Setenv("ASC_APP_ID", envApp)
					clientCalls := 0
					client := func() (*asc.Client, error) {
						clientCalls++
						return nil, errors.New("client requested")
					}
					args := append(append([]string{}, command.args...), input.flags...)
					stdout, stderr, code := runWithDeps(t, client, args...)

					switch {
					case input.given:
						if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: --app must not be empty\n") || !strings.Contains(stderr, "USAGE") || clientCalls != 0 {
							t.Fatalf("code=%d clientCalls=%d stdout=%q stderr=%q, want exit 2, no client and the --app must not be empty usage error", code, clientCalls, stdout, stderr)
						}
						_, _, err := runRootWithDeps(t, client, args...)
						diagnostic, ok := shared.DiagnosticFromError(err)
						if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--app" {
							t.Fatalf("diagnostic = %+v (ok=%v), want invalid_input on --app", diagnostic, ok)
						}
					case envApp == "":
						if code != 2 || !strings.HasPrefix(stderr, "Error: --app is required (or set ASC_APP_ID)") || clientCalls != 0 {
							t.Fatalf("code=%d clientCalls=%d stderr=%q, want the --app is required usage error", code, clientCalls, stderr)
						}
					default:
						if clientCalls == 0 || !strings.Contains(stderr, "client requested") {
							t.Fatalf("code=%d clientCalls=%d stderr=%q, want ASC_APP_ID to be used and the client requested", code, clientCalls, stderr)
						}
					}
				})
			}
		}
	}
}
