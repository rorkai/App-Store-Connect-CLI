package web

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// This first contract test is intentionally added before the implementation.
// It establishes the command's destructive-operation gate for the manager's
// bounded RED/GREEN loop.
func TestWebServiceIDsCreateRequiresConfirm(t *testing.T) {
	command := WebServiceIDsCreateCommand()
	if err := command.FlagSet.Parse([]string{
		"--identifier", "com.example.service",
		"--name", "Example Service",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("stderr %q does not contain confirmation error", stderr)
	}
}

func TestWebServiceIDsCommandHierarchy(t *testing.T) {
	command := WebServiceIDsCommand()
	want := []string{"list", "view", "create", "rename", "delete", "domains"}
	if len(command.Subcommands) != len(want) {
		t.Fatalf("subcommands = %d, want %d", len(command.Subcommands), len(want))
	}
	for index, name := range want {
		if command.Subcommands[index].Name != name || command.Subcommands[index].UsageFunc == nil {
			t.Fatalf("subcommand %d = %+v, want %q with usage", index, command.Subcommands[index], name)
		}
	}
}

func TestWebServiceIDsDomainsSetRefusesBeforeAnyRequest(t *testing.T) {
	command := WebServiceIDsDomainsSetCommand()
	if err := command.FlagSet.Parse([]string{
		"--service-id", "service-1",
		"--domain", "example.com",
		"--return-url", "https://example.com/callback",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected a non-usage refusal, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "no accepted write request has been captured") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestWebServiceIDsCommandsAreRegistered(t *testing.T) {
	commands := []*ffcli.Command{
		WebServiceIDsCommand(),
		WebServiceIDsListCommand(),
		WebServiceIDsViewCommand(),
		WebServiceIDsCreateCommand(),
		WebServiceIDsRenameCommand(),
		WebServiceIDsDeleteCommand(),
	}
	for _, command := range commands {

		usage := command.UsageFunc(command)
		if !strings.Contains(usage, command.ShortHelp) {
			t.Fatalf("%s help = %q, want ShortHelp %q", command.Name, usage, command.ShortHelp)
		}
	}
}

func TestWebServiceIDMutationsRejectPrettyTableBeforeSession(t *testing.T) {
	originalResolve := resolveSessionFn
	originalNewClient := newWebClientFn
	originalCreate := createDeveloperServiceIDFn
	originalRename := renameDeveloperServiceIDFn
	originalDelete := deleteDeveloperServiceIDFn
	originalPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalNewClient
		createDeveloperServiceIDFn = originalCreate
		renameDeveloperServiceIDFn = originalRename
		deleteDeveloperServiceIDFn = originalDelete
		persistWebSessionFn = originalPersist
	})

	var resolveCalls, mutationCalls, persistCalls int
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	createDeveloperServiceIDFn = func(context.Context, *webcore.Client, webcore.DeveloperServiceIDCreateRequest) (*asc.WebServiceIDMutationResult, error) {
		mutationCalls++
		return &asc.WebServiceIDMutationResult{}, nil
	}
	renameDeveloperServiceIDFn = func(context.Context, *webcore.Client, webcore.DeveloperServiceIDRenameRequest) (*asc.WebServiceIDMutationResult, error) {
		mutationCalls++
		return &asc.WebServiceIDMutationResult{}, nil
	}
	deleteDeveloperServiceIDFn = func(context.Context, *webcore.Client, webcore.DeveloperServiceIDDeleteRequest) (*asc.WebServiceIDMutationResult, error) {
		mutationCalls++
		return &asc.WebServiceIDMutationResult{}, nil
	}
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}

	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{
			name:    "create",
			command: WebServiceIDsCreateCommand,
			args:    []string{"--identifier", "com.example.service", "--name", "Example Service", "--confirm", "--output", "table", "--pretty"},
		},
		{
			name:    "rename",
			command: WebServiceIDsRenameCommand,
			args:    []string{"--service-id", "service-1", "--name", "New Name", "--confirm", "--output", "table", "--pretty"},
		},
		{
			name:    "delete",
			command: WebServiceIDsDeleteCommand,
			args:    []string{"--service-id", "service-1", "--confirm", "--output", "table", "--pretty"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			command := tc.command()
			if err := command.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var gotErr error
			stdout, stderr := captureWebCommandOutput(t, func() {
				gotErr = command.Exec(context.Background(), command.FlagSet.Args())
			})
			if gotErr == nil || !errors.Is(gotErr, flag.ErrHelp) || !strings.Contains(gotErr.Error(), "--pretty is only valid with JSON output") {
				t.Fatalf("error = %v, want invalid-pretty usage error", gotErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
				t.Fatalf("stderr = %q, want invalid-pretty diagnostic", stderr)
			}
		})
	}

	if resolveCalls != 0 || mutationCalls != 0 || persistCalls != 0 {
		t.Fatalf("invalid pretty reached side effects: resolve=%d mutation=%d persist=%d", resolveCalls, mutationCalls, persistCalls)
	}
}

func TestWebServiceIDsCreatePrintsVerifiedReceipt(t *testing.T) {
	originalResolve := resolveSessionFn
	originalClient := newWebClientFn
	originalCreate := createDeveloperServiceIDFn
	originalPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalClient
		createDeveloperServiceIDFn = originalCreate
		persistWebSessionFn = originalPersist
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	var got webcore.DeveloperServiceIDCreateRequest
	createDeveloperServiceIDFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperServiceIDCreateRequest) (*asc.WebServiceIDMutationResult, error) {
		got = request
		return &asc.WebServiceIDMutationResult{
			Operation: "create", ServiceID: "service-1", Identifier: request.Identifier,
			Name: request.Name, Changed: true, Verified: true, Status: "created",
		}, nil
	}

	command := WebServiceIDsCreateCommand()
	if err := command.FlagSet.Parse([]string{
		"--identifier", "com.example.service",
		"--name", "Example Service",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if got.Identifier != "com.example.service" || got.Name != "Example Service" {
		t.Fatalf("request = %+v", got)
	}
	for _, want := range []string{`"serviceId":"service-1"`, `"verified":true`, `"status":"created"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout %q does not contain %q", stdout, want)
		}
	}
}
