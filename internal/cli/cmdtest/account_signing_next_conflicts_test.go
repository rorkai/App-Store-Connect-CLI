package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAccountSigningListsRejectNextQueryFlagsBeforeAuth(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/resources?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "certificate type before next", args: []string{"certificates", "list", "--certificate-type", "IOS_DISTRIBUTION", "--next", nextURL}, wantErr: "certificates list: --next cannot be combined with --certificate-type"},
		{name: "certificate explicit zero limit after next", args: []string{"certificates", "list", "--next", nextURL, "--limit", "0"}, wantErr: "certificates list: --next cannot be combined with --limit"},
		{name: "user explicit empty email before next", args: []string{"users", "list", "--email", "", "--next", nextURL}, wantErr: "users list: --next cannot be combined with --email"},
		{name: "user role after next", args: []string{"users", "list", "--next", nextURL, "--role", "ADMIN"}, wantErr: "users list: --next cannot be combined with --role"},
		{name: "user limit", args: []string{"users", "list", "--next", nextURL, "--limit", "25"}, wantErr: "users list: --next cannot be combined with --limit"},
		{name: "actor id", args: []string{"actors", "list", "--next", nextURL, "--id", "actor-1"}, wantErr: "actors list: --next cannot be combined with --id"},
		{name: "actor fields", args: []string{"actors", "list", "--next", nextURL, "--fields", "actorType"}, wantErr: "actors list: --next cannot be combined with --fields"},
		{name: "actor limit", args: []string{"actors", "list", "--next", nextURL, "--limit", "25"}, wantErr: "actors list: --next cannot be combined with --limit"},
		{name: "pass type id", args: []string{"pass-type-ids", "list", "--next", nextURL, "--id", "pass-1"}, wantErr: "pass-type-ids list: --next cannot be combined with --id"},
		{name: "pass type identifier", args: []string{"pass-type-ids", "list", "--next", nextURL, "--identifier", "pass.example"}, wantErr: "pass-type-ids list: --next cannot be combined with --identifier"},
		{name: "pass type name", args: []string{"pass-type-ids", "list", "--next", nextURL, "--name", "Example"}, wantErr: "pass-type-ids list: --next cannot be combined with --name"},
		{name: "pass type sort", args: []string{"pass-type-ids", "list", "--next", nextURL, "--sort", "name"}, wantErr: "pass-type-ids list: --next cannot be combined with --sort"},
		{name: "pass type fields", args: []string{"pass-type-ids", "list", "--next", nextURL, "--fields", "name"}, wantErr: "pass-type-ids list: --next cannot be combined with --fields"},
		{name: "pass type certificate fields", args: []string{"pass-type-ids", "list", "--next", nextURL, "--certificate-fields", "name"}, wantErr: "pass-type-ids list: --next cannot be combined with --certificate-fields"},
		{name: "pass type include", args: []string{"pass-type-ids", "list", "--next", nextURL, "--include", "certificates"}, wantErr: "pass-type-ids list: --next cannot be combined with --include"},
		{name: "pass type explicit zero certificate limit", args: []string{"pass-type-ids", "list", "--next", nextURL, "--limit-certificates", "0"}, wantErr: "pass-type-ids list: --next cannot be combined with --limit-certificates"},
		{name: "pass type limit", args: []string{"pass-type-ids", "list", "--next", nextURL, "--limit", "25"}, wantErr: "pass-type-ids list: --next cannot be combined with --limit"},
		{name: "merchant identifier", args: []string{"merchant-ids", "list", "--next", nextURL, "--identifier", "merchant.example"}, wantErr: "merchant-ids list: --next cannot be combined with --identifier"},
		{name: "merchant name", args: []string{"merchant-ids", "list", "--next", nextURL, "--name", "Example"}, wantErr: "merchant-ids list: --next cannot be combined with --name"},
		{name: "merchant sort", args: []string{"merchant-ids", "list", "--next", nextURL, "--sort", "name"}, wantErr: "merchant-ids list: --next cannot be combined with --sort"},
		{name: "merchant fields", args: []string{"merchant-ids", "list", "--next", nextURL, "--fields", "name"}, wantErr: "merchant-ids list: --next cannot be combined with --fields"},
		{name: "merchant certificate fields", args: []string{"merchant-ids", "list", "--next", nextURL, "--certificate-fields", "name"}, wantErr: "merchant-ids list: --next cannot be combined with --certificate-fields"},
		{name: "merchant include", args: []string{"merchant-ids", "list", "--next", nextURL, "--include", "certificates"}, wantErr: "merchant-ids list: --next cannot be combined with --include"},
		{name: "merchant certificate limit", args: []string{"merchant-ids", "list", "--next", nextURL, "--certificates-limit", "25"}, wantErr: "merchant-ids list: --next cannot be combined with --certificates-limit"},
		{name: "merchant limit", args: []string{"merchant-ids", "list", "--next", nextURL, "--limit", "25"}, wantErr: "merchant-ids list: --next cannot be combined with --limit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			assertUsageExit(t, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestAccountSigningListsValidateNextBeforeQueryConflicts(t *testing.T) {
	const invalidNext = "http://api.appstoreconnect.apple.com/v1/resources?cursor=next"
	// Hand-written lists report --next first; builder lists such as actors
	// list check --limit first.
	tests := []struct {
		name           string
		args           []string
		wantDiagnostic string
	}{
		{name: "certificates", args: []string{"certificates", "list", "--next", invalidNext, "--limit", "201"}, wantDiagnostic: "Error: certificates list: --next must be an App Store Connect URL"},
		{name: "users", args: []string{"users", "list", "--next", invalidNext, "--limit", "201"}, wantDiagnostic: "Error: users list: --next must be an App Store Connect URL"},
		{name: "actors", args: []string{"actors", "list", "--next", invalidNext, "--limit", "201"}, wantDiagnostic: "Error: actors list: --limit must be between 1 and 200"},
		{name: "pass type ids", args: []string{"pass-type-ids", "list", "--next", invalidNext, "--limit", "201"}, wantDiagnostic: "Error: pass-type-ids list: --next must be an App Store Connect URL"},
		{name: "merchant ids", args: []string{"merchant-ids", "list", "--next", invalidNext, "--limit", "201"}, wantDiagnostic: "Error: merchant-ids list: --next must be an App Store Connect URL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			// A usage error renders the command usage page, which lists every
			// flag, so the precedence assertion is scoped to the diagnostic line.
			if diagnostic, _, _ := strings.Cut(stderr, "\n"); diagnostic != test.wantDiagnostic {
				t.Fatalf("stderr diagnostic = %q, want %q", diagnostic, test.wantDiagnostic)
			}
		})
	}
}
