package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotifySlackValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing webhook",
			args:    []string{"notify", "slack", "--message", "hello"},
			wantErr: "--webhook is required",
		},
		{
			name:    "missing message",
			args:    []string{"notify", "slack", "--webhook", "https://hooks.slack.com/services/test"},
			wantErr: "--message is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_SLACK_WEBHOOK", "")
			t.Setenv("ASC_SLACK_WEBHOOK_ALLOW_LOCALHOST", "")

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestNotifySlackSuccess(t *testing.T) {
	var receivedPayload map[string]interface{}
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
