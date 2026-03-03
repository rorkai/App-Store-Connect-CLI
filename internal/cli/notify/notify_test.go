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

			stderr := captureOutput(t, func() {
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
	var receivedPayload map[string]any
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
	defer func() { server.Close() }()

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
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

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

func TestNotifySlackWithThreadTS(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"--message", "Threaded release update", "--thread-ts", "1733977745.12345"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	if receivedPayload["thread_ts"] != "1733977745.12345" {
		t.Errorf("expected thread_ts to be set, got %v", receivedPayload["thread_ts"])
	}
}

func TestNotifySlackWithInvalidThreadTS(t *testing.T) {
	t.Setenv(slackWebhookEnvVar, "https://hooks.slack.com/services/test")
	t.Setenv(slackWebhookAllowLocalEnv, "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	stderr := captureOutput(t, func() {
		err := root.Parse([]string{"--message", "Threaded release update", "--thread-ts", "not-a-ts"})
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

	if !strings.Contains(stderr, "--thread-ts must be in Slack ts format") {
		t.Fatalf("expected thread-ts validation error, got %q", stderr)
	}
}

func TestNotifySlackWithPayloadJSON(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	payload := `{"app":"ExampleApp","version":"1.2.3","build":42,"release_id":123456789012345678901234567890}`
	err := root.Parse([]string{"--message", "Release submitted", "--payload-json", payload})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	attachments, ok := receivedPayload["attachments"].([]any)
	if !ok {
		t.Fatalf("expected attachments array, got %T", receivedPayload["attachments"])
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}

	attachment, ok := attachments[0].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment object, got %T", attachments[0])
	}
	if attachment["color"] != "good" {
		t.Fatalf("expected attachment color 'good', got %v", attachment["color"])
	}
	if receivedPayload["text"] != "Release submitted" {
		t.Fatalf("expected top-level text 'Release submitted', got %v", receivedPayload["text"])
	}
	if attachment["fallback"] != "Release submitted" {
		t.Fatalf("expected attachment fallback 'Release submitted', got %v", attachment["fallback"])
	}
	if _, exists := attachment["text"]; exists {
		t.Fatalf("expected attachment text to be omitted when top-level text is set, got %v", attachment["text"])
	}

	fields, ok := attachment["fields"].([]any)
	if !ok {
		t.Fatalf("expected attachment fields array, got %T", attachment["fields"])
	}

	gotFields := make(map[string]string, len(fields))
	for _, rawField := range fields {
		field, ok := rawField.(map[string]any)
		if !ok {
			t.Fatalf("expected field object, got %T", rawField)
		}
		title, _ := field["title"].(string)
		value, _ := field["value"].(string)
		gotFields[title] = value
	}

	if gotFields["app"] != "ExampleApp" {
		t.Fatalf("expected app field, got %q", gotFields["app"])
	}
	if gotFields["version"] != "1.2.3" {
		t.Fatalf("expected version field, got %q", gotFields["version"])
	}
	if gotFields["build"] != "42" {
		t.Fatalf("expected build field, got %q", gotFields["build"])
	}
	if gotFields["release_id"] != "123456789012345678901234567890" {
		t.Fatalf("expected release_id field to preserve precision, got %q", gotFields["release_id"])
	}
}

func TestNotifySlackWithPayloadFile(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	payloadPath := filepath.Join(t.TempDir(), "release-payload.json")
	if err := os.WriteFile(payloadPath, []byte(`{"status":"submitted","platform":"iOS"}`), 0o600); err != nil {
		t.Fatalf("write payload file: %v", err)
	}

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"--message", "Release submitted", "--payload-file", payloadPath})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	attachments, ok := receivedPayload["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("expected single attachment, got %v", receivedPayload["attachments"])
	}
}

func TestNotifySlackWithBlocksJSON(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	blocks := `[{"type":"section","text":{"type":"mrkdwn","text":"*Release* ready"}}]`
	err := root.Parse([]string{"--message", "Release ready", "--blocks-json", blocks})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	blocksValue, ok := receivedPayload["blocks"].([]any)
	if !ok {
		t.Fatalf("expected blocks array, got %T", receivedPayload["blocks"])
	}
	if len(blocksValue) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocksValue))
	}
	if receivedPayload["text"] != "Release ready" {
		t.Errorf("expected text 'Release ready', got %v", receivedPayload["text"])
	}
}

func TestNotifySlackWithBlocksFile(t *testing.T) {
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

	t.Setenv(slackWebhookEnvVar, server.URL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	blocksPath := filepath.Join(t.TempDir(), "blocks.json")
	if err := os.WriteFile(blocksPath, []byte(`[{"type":"divider"}]`), 0o600); err != nil {
		t.Fatalf("write blocks file: %v", err)
	}

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"--message", "Release ready", "--blocks-file", blocksPath})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	blocksValue, ok := receivedPayload["blocks"].([]any)
	if !ok {
		t.Fatalf("expected blocks array, got %T", receivedPayload["blocks"])
	}
	if len(blocksValue) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocksValue))
	}
}

func TestNotifySlackBlocksValidationErrors(t *testing.T) {
	blocksPath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(blocksPath, []byte(`{"type":"section"}`), 0o600); err != nil {
		t.Fatalf("write blocks file: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{
			name:       "blocks json invalid",
			args:       []string{"--message", "hello", "--blocks-json", "{invalid"},
			wantErrMsg: "--blocks-json must contain a JSON array",
		},
		{
			name:       "blocks file invalid",
			args:       []string{"--message", "hello", "--blocks-file", blocksPath},
			wantErrMsg: "--blocks-file must contain a JSON array",
		},
		{
			name:       "blocks file missing",
			args:       []string{"--message", "hello", "--blocks-file", filepath.Join(t.TempDir(), "missing.json")},
			wantErrMsg: "--blocks-file must be readable",
		},
		{
			name:       "blocks both set",
			args:       []string{"--message", "hello", "--blocks-json", "[]", "--blocks-file", blocksPath},
			wantErrMsg: "only one of --blocks-json or --blocks-file may be set",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(slackWebhookEnvVar, "https://hooks.slack.com/services/test")
			t.Setenv(slackWebhookAllowLocalEnv, "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			root := SlackCommand()
			root.FlagSet.SetOutput(io.Discard)

			stderr := captureOutput(t, func() {
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

func TestNotifySlackPayloadValidationErrors(t *testing.T) {
	payloadPath := filepath.Join(t.TempDir(), "invalid-payload.json")
	if err := os.WriteFile(payloadPath, []byte(`["not-an-object"]`), 0o600); err != nil {
		t.Fatalf("write payload file: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantErrMsg string
	}{
		{
			name:       "payload json invalid",
			args:       []string{"--message", "hello", "--payload-json", "{invalid"},
			wantErrMsg: "--payload-json must contain a JSON object",
		},
		{
			name:       "payload file invalid",
			args:       []string{"--message", "hello", "--payload-file", payloadPath},
			wantErrMsg: "--payload-file must contain a JSON object",
		},
		{
			name:       "payload file missing",
			args:       []string{"--message", "hello", "--payload-file", filepath.Join(t.TempDir(), "missing-payload.json")},
			wantErrMsg: "--payload-file must be readable",
		},
		{
			name:       "payload both set",
			args:       []string{"--message", "hello", "--payload-json", "{}", "--payload-file", payloadPath},
			wantErrMsg: "only one of --payload-json or --payload-file may be set",
		},
		{
			name:       "pretext without payload",
			args:       []string{"--message", "hello", "--pretext", "Release metadata"},
			wantErrMsg: "--pretext and --success require --payload-json or --payload-file",
		},
		{
			name:       "success without payload",
			args:       []string{"--message", "hello", "--success=false"},
			wantErrMsg: "--pretext and --success require --payload-json or --payload-file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(slackWebhookEnvVar, "https://hooks.slack.com/services/test")
			t.Setenv(slackWebhookAllowLocalEnv, "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			root := SlackCommand()
			root.FlagSet.SetOutput(io.Discard)

			stderr := captureOutput(t, func() {
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

func TestParseSlackPayloadUsesJSONNumber(t *testing.T) {
	payload, err := parseSlackPayload(`{"release_id":123456789012345678901234567890}`, "")
	if err != nil {
		t.Fatalf("parseSlackPayload returned error: %v", err)
	}
	value, ok := payload["release_id"].(json.Number)
	if !ok {
		t.Fatalf("expected json.Number, got %T", payload["release_id"])
	}
	if value.String() != "123456789012345678901234567890" {
		t.Fatalf("expected exact number lexeme, got %q", value.String())
	}
}

func TestNotifySlackNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer func() { server.Close() }()

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

	stderr := captureOutput(t, func() {
		if err := cmd.Parse([]string{"--webhook", "https://example.com/services/test", "--message", "hi"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr := cmd.Run(context.Background())
		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", runErr)
		}
	})

	if !strings.Contains(stderr, "hooks.slack.com") || !strings.Contains(stderr, "hooks.slack-gov.com") {
		t.Fatalf("expected host validation error, got %q", stderr)
	}
}

func TestValidateSlackWebhookURLAllowsGovHost(t *testing.T) {
	err := validateSlackWebhookURL("https://hooks.slack-gov.com/services/test")
	if err != nil {
		t.Fatalf("expected GovSlack host to be allowed, got %v", err)
	}
}

func TestNotifySlackRejectsInsecureScheme(t *testing.T) {
	t.Setenv(slackWebhookEnvVar, "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	cmd := SlackCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	stderr := captureOutput(t, func() {
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

func TestNotifySlackRejectsMalformedWebhookURL(t *testing.T) {
	t.Setenv(slackWebhookEnvVar, "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	cmd := SlackCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	stderr := captureOutput(t, func() {
		if err := cmd.Parse([]string{"--webhook", "http://localhost:80:80/services/test", "--message", "hi"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr := cmd.Run(context.Background())
		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", runErr)
		}
	})

	if !strings.Contains(stderr, "valid Slack webhook URL") {
		t.Fatalf("expected malformed URL validation error, got %q", stderr)
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

func captureOutput(t *testing.T, fn func()) string {
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

	<-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stderr
}
