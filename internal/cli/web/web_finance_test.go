package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebFinanceTransactionTaxCommandRegistrationAndHelp(t *testing.T) {
	d, _, _ := newWebTestDeps()
	finance := newWebFinanceCommand(d)
	if finance.Name != "finance" || len(finance.Subcommands) != 1 {
		t.Fatalf("finance command = %#v, want one transaction-tax subcommand", finance)
	}
	transactionTax := finance.Subcommands[0]
	if transactionTax.Name != "transaction-tax" || len(transactionTax.Subcommands) != 1 {
		t.Fatalf("transaction-tax command = %#v, want one download subcommand", transactionTax)
	}
	download := transactionTax.Subcommands[0]

	for _, name := range []string{"date", "output-path", "output", "pretty", "apple-id", "provider-id", "public-provider-id"} {
		if download.FlagSet.Lookup(name) == nil {
			t.Fatalf("download command missing --%s", name)
		}
	}
}

func TestWebFinanceTransactionTaxValidatesFlagsBeforeSession(t *testing.T) {
	called := false
	previous := resolveSessionFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		called = true
		return &webcore.AuthSession{}, "cache", nil
	}
	t.Cleanup(func() { resolveSessionFn = previous })

	for name, args := range map[string][]string{
		"missing date":   {"--output-path", filepath.Join(t.TempDir(), "tax.zip")},
		"invalid date":   {"--date", "2026-7", "--output-path", filepath.Join(t.TempDir(), "tax.zip")},
		"missing output": {"--date", "2026-07"},
		"invalid output": {"--date", "2026-07", "--output-path", filepath.Join(t.TempDir(), "tax.zip"), "--output", "yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			d, _, _ := newWebTestDeps()
			cmd := newWebTransactionTaxDownloadCommand(d)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if called {
				t.Fatal("session resolver was called before flag validation")
			}
		})
	}
}

func TestWebFinanceTransactionTaxExistingOutputPathMakesNoNetworkRequest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.zip")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}
	resolverCalls := 0
	previousResolver := resolveSessionFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		resolverCalls++
		return &webcore.AuthSession{ProviderID: 1234}, "cache", nil
	}
	previousDownload := downloadTransactionTaxReportFn
	downloadTransactionTaxReportFn = func(context.Context, *webcore.Client, webcore.TransactionTaxReportRequest) (*webcore.TransactionTaxReportDownload, error) {
		t.Fatal("download was called despite an existing destination")
		return nil, nil
	}
	t.Cleanup(func() {
		resolveSessionFn = previousResolver
		downloadTransactionTaxReportFn = previousDownload
	})

	d, _, _ := newWebTestDeps()
	cmd := newWebTransactionTaxDownloadCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--date", "2026-07", "--output-path", path}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "choose a different --output-path") {
		t.Fatalf("error = %v, want existing-destination error", err)
	}
	if resolverCalls != 0 {
		t.Fatalf("resolver calls = %d, want 0", resolverCalls)
	}
}

func TestWebFinanceTransactionTaxHappyPathWritesSecretSafeReceipt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transaction-tax.zip")
	previousResolver := resolveSessionFn
	previousClient := newWebClientFn
	previousDownload := downloadTransactionTaxReportFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{ProviderID: 1234}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	downloadTransactionTaxReportFn = func(requestContext context.Context, _ *webcore.Client, request webcore.TransactionTaxReportRequest) (*webcore.TransactionTaxReportDownload, error) {
		if request.ProviderID != 1234 || request.Date != "2026-07" {
			t.Fatalf("request = %+v, want provider 1234 and July 2026", request)
		}
		deadline, ok := requestContext.Deadline()
		if !ok || time.Until(deadline) < 10*time.Minute {
			t.Fatalf("request context deadline = %v, want at least ten minutes", deadline)
		}
		return &webcore.TransactionTaxReportDownload{
			Body:                      io.NopCloser(strings.NewReader("PK\x03\x04safe-archive")),
			PollStatus:                "readyForDownload",
			ContentType:               "application/octet-stream",
			ContentDispositionPresent: true,
		}, nil
	}
	t.Cleanup(func() {
		resolveSessionFn = previousResolver
		newWebClientFn = previousClient
		downloadTransactionTaxReportFn = previousDownload
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebTransactionTaxDownloadCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--date", "2026-07", "--output-path", path, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var receipt map[string]any
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt["date"] != "2026-07" || receipt["pollStatus"] != "readyForDownload" || receipt["verified"] != true {
		t.Fatalf("receipt = %#v, want date/status/verified", receipt)
	}
	for _, secret := range []string{"job-123", "signature=SECRET", "providerId", "sapVendorNumber"} {
		if strings.Contains(stdout, secret) {
			t.Fatalf("receipt contains %q: %s", secret, stdout)
		}
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(contents) != "PK\x03\x04safe-archive" {
		t.Fatalf("archive contents = %q, want staged ZIP bytes", contents)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("archive mode = %o, want 600", info.Mode().Perm())
	}
}

func TestWebFinanceTransactionTaxTableOutputUsesRegisteredRenderer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transaction-tax.zip")
	previousResolver := resolveSessionFn
	previousClient := newWebClientFn
	previousDownload := downloadTransactionTaxReportFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{ProviderID: 1234}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	downloadTransactionTaxReportFn = func(context.Context, *webcore.Client, webcore.TransactionTaxReportRequest) (*webcore.TransactionTaxReportDownload, error) {
		return &webcore.TransactionTaxReportDownload{
			Body:        io.NopCloser(strings.NewReader("PK\x03\x04table-archive")),
			PollStatus:  "readyForDownload",
			ContentType: "application/octet-stream",
		}, nil
	}
	t.Cleanup(func() {
		resolveSessionFn = previousResolver
		newWebClientFn = previousClient
		downloadTransactionTaxReportFn = previousDownload
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebTransactionTaxDownloadCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--date", "2026-07", "--output-path", path, "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, value := range []string{"Date", "Path", "Poll Status", "2026-07", "readyForDownload"} {
		if !strings.Contains(stdout, value) {
			t.Fatalf("table output = %q, want %q", stdout, value)
		}
	}
}

func TestWebFinanceTransactionTaxPartialDownloadLeavesNoDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transaction-tax.zip")
	previousResolver := resolveSessionFn
	previousClient := newWebClientFn
	previousDownload := downloadTransactionTaxReportFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{ProviderID: 1234}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	downloadTransactionTaxReportFn = func(context.Context, *webcore.Client, webcore.TransactionTaxReportRequest) (*webcore.TransactionTaxReportDownload, error) {
		return &webcore.TransactionTaxReportDownload{Body: io.NopCloser(strings.NewReader("not-a-zip")), PollStatus: "readyForDownload"}, nil
	}
	t.Cleanup(func() {
		resolveSessionFn = previousResolver
		newWebClientFn = previousClient
		downloadTransactionTaxReportFn = previousDownload
	})

	d, _, _ := newWebTestDeps()
	cmd := newWebTransactionTaxDownloadCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--date", "2026-07", "--output-path", path}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "not a ZIP file") {
		t.Fatalf("error = %v, want ZIP validation error", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want absent destination", statErr)
	}
}
