package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestNotifySlackValidationErrors(t *testing.T) {
	const webhook = "https://hooks.slack.com/services/test"
	const attachmentErr = "Error: --pretext and --success require --payload-json or --payload-file"

	tests := []struct {
		name      string
		webhook   string
		args      []string
		wantErr   string
		wantExact bool
	}{
		{
			name:    "missing webhook",
			args:    []string{"notify", "slack", "--message", "hello"},
			wantErr: "--webhook is required",
		},
		{
			name:    "missing message",
			args:    []string{"notify", "slack", "--webhook", webhook},
			wantErr: "--message is required",
		},
		{
			name:      "invalid thread ts",
			webhook:   webhook,
			args:      []string{"notify", "slack", "--message", "hello", "--thread-ts", "not-a-ts"},
			wantErr:   "Error: --thread-ts must be in Slack ts format (e.g. 1733977745.12345)",
			wantExact: true,
		},
		{
			name:      "pretext without payload",
			webhook:   webhook,
			args:      []string{"notify", "slack", "--message", "hello", "--pretext", "Release metadata"},
			wantErr:   attachmentErr,
			wantExact: true,
		},
		{
			name:      "success without payload",
			webhook:   webhook,
			args:      []string{"notify", "slack", "--message", "hello", "--success=false"},
			wantErr:   attachmentErr,
			wantExact: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_SLACK_WEBHOOK", test.webhook)
			t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "")

			stderr := assertExitStderr(t, test.args, rootcmd.ExitUsage, test.wantErr)
			if test.wantExact && stderr != test.wantErr+"\n" {
				t.Fatalf("stderr = %q, want only %q", stderr, test.wantErr+"\n")
			}
		})
	}
}

func TestNotifySlackSuccess(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("ASC_SLACK_WEBHOOK", server.URL)
	t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"notify", "slack", "--message", "Hello"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Message sent to Slack successfully") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if receivedPayload == nil {
		t.Fatal("expected payload to be sent")
	}
	if receivedPayload["text"] != "Hello" {
		t.Errorf("expected text 'Hello', got %v", receivedPayload["text"])
	}
}

func TestNotifySlackSuccessWithBlocks(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("ASC_SLACK_WEBHOOK", server.URL)
	t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"notify", "slack",
			"--message", "Hello",
			"--blocks-json", `[{"type":"section","text":{"type":"plain_text","text":"Hi"}}]`,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Message sent to Slack successfully") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	blocksValue, ok := receivedPayload["blocks"].([]any)
	if !ok {
		t.Fatalf("expected blocks array, got %T", receivedPayload["blocks"])
	}
	if len(blocksValue) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocksValue))
	}
}

func TestNotifySlackRejectsNullBlocksBeforeRequest(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	nullBlocksFile := filepath.Join(t.TempDir(), "null-blocks.json")
	if err := os.WriteFile(nullBlocksFile, []byte("null"), 0o600); err != nil {
		t.Fatalf("write null blocks file: %v", err)
	}

	tests := []struct {
		name           string
		blocksArgs     []string
		wantDiagnostic string
	}{
		{
			name:           "inline",
			blocksArgs:     []string{"--blocks-json", "null"},
			wantDiagnostic: "Error: --blocks-json must contain a JSON array\n",
		},
		{
			name:           "file",
			blocksArgs:     []string{"--blocks-file", nullBlocksFile},
			wantDiagnostic: "Error: --blocks-file must contain a JSON array\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_SLACK_WEBHOOK", "")
			t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "1")

			args := []string{"notify", "slack", "--webhook", server.URL, "--message", "Hello"}
			args = append(args, test.blocksArgs...)

			var exitCode int
			stdout, stderr := captureOutput(t, func() {
				exitCode = rootcmd.Run(args, "1.2.3")
			})

			if exitCode != rootcmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, exitCode)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != test.wantDiagnostic {
				t.Fatalf("expected stderr %q, got %q", test.wantDiagnostic, stderr)
			}
			if got := requestCount.Load(); got != 0 {
				t.Fatalf("expected no webhook requests, got %d", got)
			}
		})
	}
}

func TestNotifySlackExplicitEmptyBlocksArrayIsSent(t *testing.T) {
	var requestCount atomic.Int32
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("ASC_SLACK_WEBHOOK", "")
	t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "1")

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"notify", "slack",
			"--webhook", server.URL,
			"--message", "Hello",
			"--blocks-json", "[]",
		}, "1.2.3")
	})

	if exitCode != rootcmd.ExitSuccess {
		t.Fatalf("expected exit code %d, got %d with stderr %q", rootcmd.ExitSuccess, exitCode, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Message sent to Slack successfully") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected one webhook request, got %d", got)
	}
	blocksValue, ok := receivedPayload["blocks"].([]any)
	if !ok {
		t.Fatalf("expected blocks array, got %T", receivedPayload["blocks"])
	}
	if len(blocksValue) != 0 {
		t.Fatalf("expected empty blocks array, got %d blocks", len(blocksValue))
	}
}

func TestNotifySlackSuccessWithThreadAndPayload(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("ASC_SLACK_WEBHOOK", server.URL)
	t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"notify", "slack",
			"--message", "Release event",
			"--thread-ts", "1733977745.12345",
			"--payload-json", `{"app":"Example","version":"1.2.3"}`,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Message sent to Slack successfully") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if receivedPayload["thread_ts"] != "1733977745.12345" {
		t.Fatalf("expected thread_ts, got %v", receivedPayload["thread_ts"])
	}
	attachments, ok := receivedPayload["attachments"].([]any)
	if !ok {
		t.Fatalf("expected attachments array, got %T", receivedPayload["attachments"])
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
}
