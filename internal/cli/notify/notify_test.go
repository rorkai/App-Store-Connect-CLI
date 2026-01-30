package notify

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNotifySlackValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		envVar  string
		wantErr error
	}{
		{
			name:    "notify slack missing webhook via env",
			args:    []string{"--message", "hello"},
			envVar:  "",
			wantErr: flag.ErrHelp,
		},
		{
			name:    "notify slack missing message",
			args:    []string{"--webhook", "https://hooks.slack.com/test"},
			envVar:  "",
			wantErr: flag.ErrHelp,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.envVar != "" {
				t.Setenv(slackWebhookEnvVar, test.envVar)
			} else {
				os.Unsetenv(slackWebhookEnvVar)
			}
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			root := SlackCommand()
			root.FlagSet.SetOutput(io.Discard)

			err := root.Parse(test.args)
			if err != nil && !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("parse error: %v", err)
			}
			runErr := root.Run(context.Background())
			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(runErr, test.wantErr) {
				t.Fatalf("expected %v, got %v", test.wantErr, runErr)
			}
		})
	}
}

func TestResolveWebhook(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		flagValue string
		want      string
	}{
		{
			name:      "prefers flag over env",
			envValue:  "https://hooks.slack.com/env",
			flagValue: "https://hooks.slack.com/flag",
			want:      "https://hooks.slack.com/flag",
		},
		{
			name:      "uses env when flag empty",
			envValue:  "https://hooks.slack.com/env",
			flagValue: "",
			want:      "https://hooks.slack.com/env",
		},
		{
			name:      "empty when both empty",
			envValue:  "",
			flagValue: "",
			want:      "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.envValue != "" {
				t.Setenv(slackWebhookEnvVar, test.envValue)
			} else {
				os.Unsetenv(slackWebhookEnvVar)
			}
			got := resolveWebhook(test.flagValue)
			if got != test.want {
				t.Errorf("resolveWebhook(%q) = %q, want %q", test.flagValue, got, test.want)
			}
		})
	}
}

func TestNotifyCommandHasSubcommands(t *testing.T) {
	cmd := NotifyCommand()
	if len(cmd.Subcommands) == 0 {
		t.Fatal("expected subcommands, got none")
	}
	found := false
	for _, sub := range cmd.Subcommands {
		if sub.Name == "slack" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'slack' subcommand")
	}
}

func TestSlackCommandName(t *testing.T) {
	cmd := SlackCommand()
	if cmd.Name != "slack" {
		t.Errorf("expected name 'slack', got %q", cmd.Name)
	}
}
