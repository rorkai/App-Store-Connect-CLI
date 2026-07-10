package analytics

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func parseAnalyticsArgs(args []string) []string {
	if len(args) > 0 && args[0] == "analytics" {
		return args[1:]
	}
	return args
}

func runAnalyticsCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	cmd := AnalyticsCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Parse(parseAnalyticsArgs(args)); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})

	return stdout, stderr, runErr
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

func TestAnalyticsSalesValidationErrors(t *testing.T) {
	t.Setenv("ASC_VENDOR_NUMBER", "")
	t.Setenv("ASC_ANALYTICS_VENDOR_NUMBER", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing vendor",
			args:    []string{"analytics", "sales", "--type", "SALES", "--subtype", "SUMMARY", "--frequency", "DAILY", "--date", "2024-01-20"},
			wantErr: "--vendor is required",
		},
		{
			name:    "missing type",
			args:    []string{"analytics", "sales", "--vendor", "12345678", "--subtype", "SUMMARY", "--frequency", "DAILY", "--date", "2024-01-20"},
			wantErr: "--type is required",
		},
		{
			name:    "missing subtype",
			args:    []string{"analytics", "sales", "--vendor", "12345678", "--type", "SALES", "--frequency", "DAILY", "--date", "2024-01-20"},
			wantErr: "--subtype is required",
		},
		{
			name:    "missing frequency",
			args:    []string{"analytics", "sales", "--vendor", "12345678", "--type", "SALES", "--subtype", "SUMMARY", "--date", "2024-01-20"},
			wantErr: "--frequency is required",
		},
		{
			name:    "missing date",
			args:    []string{"analytics", "sales", "--vendor", "12345678", "--type", "SALES", "--subtype", "SUMMARY", "--frequency", "DAILY"},
			wantErr: "--date is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runAnalyticsCommand(t, test.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAnalyticsRequestValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"analytics", "request", "--access-type", "ONGOING"},
			wantErr: "--app is required",
		},
		{
			name:    "missing access type",
			args:    []string{"analytics", "request", "--app", "APP_ID"},
			wantErr: "--access-type is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runAnalyticsCommand(t, test.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAnalyticsRequestsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "requests"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestAnalyticsGetValidationErrors(t *testing.T) {
	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "view"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--request-id is required") {
		t.Fatalf("expected missing request-id error, got %q", stderr)
	}
}

func TestAnalyticsReportsGetValidationErrors(t *testing.T) {
	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "reports", "view"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--report-id is required") {
		t.Fatalf("expected missing report-id error, got %q", stderr)
	}
}

func TestAnalyticsReportsRelationshipsValidationErrors(t *testing.T) {
	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "reports", "links"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--report-id is required") {
		t.Fatalf("expected missing report-id error, got %q", stderr)
	}
}

func TestAnalyticsInstancesGetValidationErrors(t *testing.T) {
	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "instances", "view"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--instance-id is required") {
		t.Fatalf("expected missing instance-id error, got %q", stderr)
	}
}

func TestAnalyticsInstancesRelationshipsValidationErrors(t *testing.T) {
	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "instances", "links"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--instance-id is required") {
		t.Fatalf("expected missing instance-id error, got %q", stderr)
	}
}

func TestAnalyticsSegmentsGetValidationErrors(t *testing.T) {
	stdout, stderr, err := runAnalyticsCommand(t, []string{"analytics", "segments", "view"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--segment-id is required") {
		t.Fatalf("expected missing segment-id error, got %q", stderr)
	}
}

func TestAnalyticsDownloadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing request id",
			args:    []string{"analytics", "download"},
			wantErr: "--request-id is required",
		},
		{
			name:    "missing instance id",
			args:    []string{"analytics", "download", "--request-id", "11111111-1111-1111-1111-111111111111"},
			wantErr: "--instance-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runAnalyticsCommand(t, test.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
