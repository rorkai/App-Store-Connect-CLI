package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestAnalyticsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "reports get missing report id",
			args:    []string{"analytics", "reports", "get"},
			wantErr: "--report-id is required",
		},
		{
			name:    "reports relationships missing report id",
			args:    []string{"analytics", "reports", "links"},
			wantErr: "--report-id is required",
		},
		{
			name:    "instances get missing instance id",
			args:    []string{"analytics", "instances", "get"},
			wantErr: "--instance-id is required",
		},
		{
			name:    "instances relationships missing instance id",
			args:    []string{"analytics", "instances", "links"},
			wantErr: "--instance-id is required",
		},
		{
			name:    "segments get missing segment id",
			args:    []string{"analytics", "segments", "get"},
			wantErr: "--segment-id is required",
		},
		{
			name:    "requests delete missing request id",
			args:    []string{"analytics", "requests", "delete", "--confirm"},
			wantErr: "--request-id is required",
		},
		{
			name:    "requests delete missing confirm",
			args:    []string{"analytics", "requests", "delete", "--request-id", "11111111-1111-1111-1111-111111111111"},
			wantErr: "--confirm is required",
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
