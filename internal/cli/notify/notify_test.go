package notify

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNotifySlackValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		envVar     string
		wantErrMsg string
	}{
		{
			name:       "notify slack missing webhook via env",
			args:       []string{"--message", "hello"},
			envVar:     "",
			wantErrMsg: "--webhook is required or set ASC_SLACK_WEBHOOK env var",
		},
		{
			name:       "notify slack missing message",
			args:       []string{"--webhook", "https://hooks.slack.com/test"},
			envVar:     "",
			wantErrMsg: "--message is required",
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
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
		})
	}
}

func TestNotifySlackSuccess(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv(slackWebhookEnvVar, server.URL)
	defer os.Unsetenv(slackWebhookEnvVar)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"--message", "Hello, Slack!"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	if receivedPayload == nil {
		t.Fatal("expected payload to be sent")
	}
	if receivedPayload["text"] != "Hello, Slack!" {
		t.Errorf("expected text 'Hello, Slack!', got %v", receivedPayload["text"])
	}
}

func TestNotifySlackWithChannel(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv(slackWebhookEnvVar, server.URL)
	defer os.Unsetenv(slackWebhookEnvVar)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"--message", "Test", "--channel", "#deploy"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	if receivedPayload["channel"] != "#deploy" {
		t.Errorf("expected channel '#deploy', got %v", receivedPayload["channel"])
	}
	if receivedPayload["text"] != "Test" {
		t.Errorf("expected text 'Test', got %v", receivedPayload["text"])
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

func TestSlackCommandHasUsageFunc(t *testing.T) {
	cmd := SlackCommand()
	if cmd.UsageFunc == nil {
		t.Error("expected UsageFunc to be set")
	}
}

func TestNotifyCommandHasUsageFunc(t *testing.T) {
	cmd := NotifyCommand()
	if cmd.UsageFunc == nil {
		t.Error("expected UsageFunc to be set")
	}
}
