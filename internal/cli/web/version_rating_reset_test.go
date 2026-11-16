package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubVersionRatingResetSession(t *testing.T) {
	t.Helper()
	originalResolver := resolveSessionFn
	originalClient := newWebClientFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolver
		newWebClientFn = originalClient
	})
	resolveSessionFn = func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
}

func requireVersionRatingResetDeadline(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected operation to receive the command timeout context")
	}
}

func TestVersionRatingResetCreate(t *testing.T) {
	stubVersionRatingResetSession(t)
	originalCreate := createVersionRatingResetFn
	t.Cleanup(func() { createVersionRatingResetFn = originalCreate })

	var receivedVersion string
	createVersionRatingResetFn = func(ctx context.Context, _ *webcore.Client, versionID string) (*webcore.RatingResetRequestResponse, error) {
		requireVersionRatingResetDeadline(t, ctx)
		receivedVersion = versionID
		return &webcore.RatingResetRequestResponse{
			Data: &webcore.RatingResetRequest{Type: "resetRatingsRequests", ID: "reset-1"},
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newVersionRatingResetCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--version-id", " version-1 ", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if receivedVersion != "version-1" {
		t.Fatalf("version = %q, want trimmed version-1", receivedVersion)
	}

	var result asc.AppStoreVersionRatingResetCreateResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v; output=%q", err, stdout)
	}
	if result.RatingResetRequestID != "reset-1" || result.VersionID != "version-1" || !result.Scheduled {
		t.Fatalf("result = %#v, want scheduled reset receipt", result)
	}
}

func TestVersionRatingResetViewTable(t *testing.T) {
	stubVersionRatingResetSession(t)
	originalGet := getVersionRatingResetFn
	t.Cleanup(func() { getVersionRatingResetFn = originalGet })

	resetDate := "2026-09-01T08:00:00Z"
	getVersionRatingResetFn = func(ctx context.Context, _ *webcore.Client, versionID string) (*webcore.RatingResetRequestResponse, error) {
		requireVersionRatingResetDeadline(t, ctx)
		if versionID != "version-1" {
			t.Fatalf("version = %q, want version-1", versionID)
		}
		return &webcore.RatingResetRequestResponse{
			Data: &webcore.RatingResetRequest{
				Type:       "resetRatingsRequests",
				ID:         "reset-1",
				Attributes: webcore.RatingResetRequestAttributes{ResetDate: &resetDate},
			},
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newVersionRatingResetViewCommand(d)
	if err := command.FlagSet.Parse([]string{"--version-id", "version-1", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"Rating Reset Request ID", "Reset Date", "reset-1", resetDate} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
}

func TestVersionRatingResetViewReportsUnscheduledRelationship(t *testing.T) {
	stubVersionRatingResetSession(t)
	originalGet := getVersionRatingResetFn
	t.Cleanup(func() { getVersionRatingResetFn = originalGet })
	getVersionRatingResetFn = func(context.Context, *webcore.Client, string) (*webcore.RatingResetRequestResponse, error) {
		return &webcore.RatingResetRequestResponse{Data: nil}, nil
	}

	t.Run("json preserves null data", func(t *testing.T) {
		d, depsStdout, depsStderr := newWebTestDeps()
		command := newVersionRatingResetViewCommand(d)
		if err := command.FlagSet.Parse([]string{"--version-id", "version-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
			if err := command.Exec(context.Background(), nil); err != nil {
				t.Fatalf("Exec() error = %v", err)
			}
		})
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
		if strings.TrimSpace(stdout) != `{"data":null}` {
			t.Fatalf("stdout = %q, want null JSON:API relationship", stdout)
		}
	})

	for _, format := range []string{"table", "markdown"} {
		t.Run(format+" reports unscheduled", func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			command := newVersionRatingResetViewCommand(d)
			if err := command.FlagSet.Parse([]string{"--version-id", "version-1", "--output", format}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("Exec() error = %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			for _, want := range []string{"Scheduled", "false"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want %q", stdout, want)
				}
			}
		})
	}
}

func TestVersionRatingResetDelete(t *testing.T) {
	stubVersionRatingResetSession(t)
	originalDelete := deleteVersionRatingResetFn
	t.Cleanup(func() { deleteVersionRatingResetFn = originalDelete })

	var receivedID string
	deleteVersionRatingResetFn = func(ctx context.Context, _ *webcore.Client, requestID string) error {
		requireVersionRatingResetDeadline(t, ctx)
		receivedID = requestID
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newVersionRatingResetDeleteCommand(d)
	if err := command.FlagSet.Parse([]string{"--id", " reset-1 ", "--confirm", "--output", "markdown"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if receivedID != "reset-1" {
		t.Fatalf("id = %q, want trimmed reset-1", receivedID)
	}
	for _, want := range []string{"| Rating Reset Request ID | Cancelled |", "reset-1", "true"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
}

func TestVersionRatingResetValidationRunsBeforeSessionResolution(t *testing.T) {
	originalResolver := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolver })
	resolverCalls := 0
	resolveSessionFn = func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		resolverCalls++
		return nil, "", errors.New("session should not be resolved")
	}

	cases := []struct {
		name    string
		command func(*shared.Deps) *ffcli.Command
		args    []string
		want    string
	}{
		{name: "view missing version", command: newVersionRatingResetViewCommand, want: "--version-id is required"},
		{name: "create missing version", command: newVersionRatingResetCreateCommand, args: []string{"--confirm"}, want: "--version-id is required"},
		{name: "create missing confirmation", command: newVersionRatingResetCreateCommand, args: []string{"--version-id", "version-1"}, want: "--confirm is required"},
		{name: "delete missing id", command: newVersionRatingResetDeleteCommand, args: []string{"--confirm"}, want: "--id is required"},
		{name: "delete missing confirmation", command: newVersionRatingResetDeleteCommand, args: []string{"--id", "reset-1"}, want: "--confirm is required"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			command := test.command(d)
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := command.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want flag.ErrHelp", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "Error: "+test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
	if resolverCalls != 0 {
		t.Fatalf("session resolver calls = %d, want 0", resolverCalls)
	}
}

func TestVersionRatingResetSessionResolutionDoesNotUseRequestTimeout(t *testing.T) {
	originalResolver := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolver })
	resolveErr := errors.New("stop after context inspection")

	cases := []struct {
		name    string
		command func(*shared.Deps) *ffcli.Command
		args    []string
	}{
		{name: "view", command: newVersionRatingResetViewCommand, args: []string{"--version-id", "version-1"}},
		{name: "create", command: newVersionRatingResetCreateCommand, args: []string{"--version-id", "version-1", "--confirm"}},
		{name: "delete", command: newVersionRatingResetDeleteCommand, args: []string{"--id", "reset-1", "--confirm"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			hadDeadline := false
			resolveSessionFn = func(ctx context.Context, _, _, _, _ string) (*webcore.AuthSession, string, error) {
				_, hadDeadline = ctx.Deadline()
				return nil, "", resolveErr
			}

			d, _, _ := newWebTestDeps()
			command := test.command(d)
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := command.Exec(context.Background(), nil); !errors.Is(err, resolveErr) {
				t.Fatalf("error = %v, want resolve error", err)
			}
			if hadDeadline {
				t.Fatal("authentication must not receive the request timeout context")
			}
		})
	}
}

func TestVersionRatingResetWrapsUnauthorizedSessionError(t *testing.T) {
	stubVersionRatingResetSession(t)
	originalGet := getVersionRatingResetFn
	t.Cleanup(func() { getVersionRatingResetFn = originalGet })
	getVersionRatingResetFn = func(context.Context, *webcore.Client, string) (*webcore.RatingResetRequestResponse, error) {
		return nil, &webcore.APIError{Status: 401}
	}

	d, _, _ := newWebTestDeps()
	command := newVersionRatingResetViewCommand(d)
	if err := command.FlagSet.Parse([]string{"--version-id", "version-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "web session is unauthorized or expired") || !strings.Contains(err.Error(), "asc web auth login") {
		t.Fatalf("error = %v, want web auth recovery hint", err)
	}
}

func TestVersionRatingResetRejectsSuccessfulResponseWithoutID(t *testing.T) {
	stubVersionRatingResetSession(t)
	originalCreate := createVersionRatingResetFn
	t.Cleanup(func() { createVersionRatingResetFn = originalCreate })
	createVersionRatingResetFn = func(context.Context, *webcore.Client, string) (*webcore.RatingResetRequestResponse, error) {
		return &webcore.RatingResetRequestResponse{}, nil
	}

	d, _, _ := newWebTestDeps()
	command := newVersionRatingResetCreateCommand(d)
	if err := command.FlagSet.Parse([]string{"--version-id", "version-1", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "rating reset request ID missing from response") {
		t.Fatalf("error = %v, want missing response ID", err)
	}
}

func TestVersionRatingResetSubcommandsAcceptWebSessionFlags(t *testing.T) {
	d, _, _ := newWebTestDeps()
	for _, subcommand := range []*ffcli.Command{
		newVersionRatingResetViewCommand(d),
		newVersionRatingResetCreateCommand(d),
		newVersionRatingResetDeleteCommand(d),
	} {
		for _, flagName := range []string{"apple-id", "provider-id", "public-provider-id", "two-factor-code-command"} {
			if subcommand.FlagSet.Lookup(flagName) == nil {
				t.Fatalf("%s missing --%s", subcommand.Name, flagName)
			}
		}
	}
}
