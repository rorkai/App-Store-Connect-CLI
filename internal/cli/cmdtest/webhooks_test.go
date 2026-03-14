package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestWebhooksValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing app",
			args:    []string{"webhooks", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "get missing webhook id",
			args:    []string{"webhooks", "get"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "create missing app",
			args:    []string{"webhooks", "create", "--name", "Build Updates", "--url", "https://example.com/webhook", "--secret", "secret", "--events", "BUILD_UPLOAD_STATE_UPDATED", "--enabled", "true"},
			wantErr: "--app is required",
		},
		{
			name:    "create missing name",
			args:    []string{"webhooks", "create", "--app", "APP_ID", "--url", "https://example.com/webhook", "--secret", "secret", "--events", "BUILD_UPLOAD_STATE_UPDATED", "--enabled", "true"},
			wantErr: "--name is required",
		},
		{
			name:    "create missing url",
			args:    []string{"webhooks", "create", "--app", "APP_ID", "--name", "Build Updates", "--secret", "secret", "--events", "BUILD_UPLOAD_STATE_UPDATED", "--enabled", "true"},
			wantErr: "--url is required",
		},
		{
			name:    "create missing secret",
			args:    []string{"webhooks", "create", "--app", "APP_ID", "--name", "Build Updates", "--url", "https://example.com/webhook", "--events", "BUILD_UPLOAD_STATE_UPDATED", "--enabled", "true"},
			wantErr: "--secret is required",
		},
		{
			name:    "create missing events",
			args:    []string{"webhooks", "create", "--app", "APP_ID", "--name", "Build Updates", "--url", "https://example.com/webhook", "--secret", "secret", "--enabled", "true"},
			wantErr: "--events is required",
		},
		{
			name:    "create missing enabled",
			args:    []string{"webhooks", "create", "--app", "APP_ID", "--name", "Build Updates", "--url", "https://example.com/webhook", "--secret", "secret", "--events", "BUILD_UPLOAD_STATE_UPDATED"},
			wantErr: "--enabled is required",
		},
		{
			name:    "update missing webhook id",
			args:    []string{"webhooks", "update", "--url", "https://example.com/webhook"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "update missing fields",
			args:    []string{"webhooks", "update", "--webhook-id", "wh-1"},
			wantErr: "--name, --url, --secret, --events, or --enabled is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"webhooks", "delete", "--webhook-id", "wh-1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "delete missing webhook id",
			args:    []string{"webhooks", "delete", "--confirm"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "deliveries missing webhook id",
			args:    []string{"webhooks", "deliveries"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "deliveries missing filter",
			args:    []string{"webhooks", "deliveries", "--webhook-id", "wh-1"},
			wantErr: "--created-after or --created-before is required",
		},
		{
			name:    "deliveries multiple filters",
			args:    []string{"webhooks", "deliveries", "--webhook-id", "wh-1", "--created-after", "2026-01-01", "--created-before", "2026-01-02"},
			wantErr: "only one of --created-after or --created-before can be used",
		},
		{
			name:    "deliveries redeliver missing delivery id",
			args:    []string{"webhooks", "deliveries", "redeliver"},
			wantErr: "--delivery-id is required",
		},
		{
			name:    "deliveries relationships missing webhook id",
			args:    []string{"webhooks", "deliveries", "links"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "deliveries relationships paginate missing webhook id",
			args:    []string{"webhooks", "deliveries", "links", "--paginate"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "ping missing webhook id",
			args:    []string{"webhooks", "ping"},
			wantErr: "--webhook-id is required",
		},
		{
			name:    "serve invalid port high",
			args:    []string{"webhooks", "serve", "--port", "70000"},
			wantErr: "--port must be between 0 and 65535",
		},
		{
			name:    "serve invalid output",
			args:    []string{"webhooks", "serve", "--output", "table"},
			wantErr: "--output must be one of: text, json",
		},
		{
			name:    "serve positional args",
			args:    []string{"webhooks", "serve", "extra"},
			wantErr: "webhooks serve does not accept positional arguments",
		},
		{
			name:    "serve empty host",
			args:    []string{"webhooks", "serve", "--host", ""},
			wantErr: "--host is required",
		},
		{
			name:    "serve invalid max body bytes",
			args:    []string{"webhooks", "serve", "--max-body-bytes", "0"},
			wantErr: "--max-body-bytes must be greater than 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
