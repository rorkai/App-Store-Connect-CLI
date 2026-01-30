package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
			args:       []string{"--webhook", "https://hooks.slack.com/services/test"},
			envVar:     "",
			wantErrMsg: "--message is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.envVar != "" {
				t.Setenv(slackWebhookEnvVar, test.envVar)
			} else {
				t.Setenv(slackWebhookEnvVar, "")
			}
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			root := SlackCommand()
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
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

			if !strings.Contains(stderr, test.wantErrMsg) {
				t.Fatalf("expected error %q, got %q", test.wantErrMsg, stderr)
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
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
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
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
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

func TestNotifySlackNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"--message", "Test failure"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "unexpected response 500") {
		t.Fatalf("expected status error, got %v", runErr)
	}
}

func TestNotifySlackRejectsInvalidWebhookHost(t *testing.T) {
	t.Setenv(slackWebhookEnvVar, "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	cmd := SlackCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := cmd.Parse([]string{"--webhook", "https://example.com/services/test", "--message", "hi"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr := cmd.Run(context.Background())
		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", runErr)
		}
	})

	if !strings.Contains(stderr, "hooks.slack.com") {
		t.Fatalf("expected host validation error, got %q", stderr)
	}
}

func TestNotifySlackRejectsInsecureScheme(t *testing.T) {
	t.Setenv(slackWebhookEnvVar, "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	cmd := SlackCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := cmd.Parse([]string{"--webhook", "http://hooks.slack.com/services/test", "--message", "hi"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr := cmd.Run(context.Background())
		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", runErr)
		}
	})

	if !strings.Contains(stderr, "https") {
		t.Fatalf("expected https validation error, got %q", stderr)
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
				t.Setenv(slackWebhookEnvVar, "")
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

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, stderr
}
