package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestOfferCodesValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "offer codes view missing offer code id",
			args:    []string{"subscriptions", "offers", "offer-codes", "view"},
			wantErr: "Error: --offer-code-id is required",
		},
		{
			name:    "offer codes create missing subscription id",
			args:    []string{"subscriptions", "offers", "offer-codes", "create"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "offer codes create missing name",
			args:    []string{"subscriptions", "offers", "offer-codes", "create", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --name is required",
		},
		{
			name:    "offer codes create missing customer eligibilities",
			args:    []string{"subscriptions", "offers", "offer-codes", "create", "--subscription-id", "SUB_ID", "--name", "SPRING"},
			wantErr: "Error: --offer-eligibility is required",
		},
		{
			name:    "offer codes update missing active",
			args:    []string{"subscriptions", "offers", "offer-codes", "update", "--offer-code-id", "OFFER_ID"},
			wantErr: "Error: --active is required",
		},
		{
			name:    "custom codes list missing offer code id",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "list"},
			wantErr: "Error: --offer-code-id is required",
		},
		{
			name:    "custom codes view missing custom code id",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "view"},
			wantErr: "Error: --custom-code-id is required",
		},
		{
			name:    "custom codes create missing offer code id",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "create", "--code", "SPRING", "--quantity", "10"},
			wantErr: "Error: --offer-code-id is required",
		},
		{
			name:    "custom codes create missing code",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "create", "--offer-code-id", "OFFER_ID", "--quantity", "10"},
			wantErr: "Error: --code is required",
		},
		{
			name:    "custom codes create missing quantity",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "create", "--offer-code-id", "OFFER_ID", "--code", "SPRING"},
			wantErr: "Error: --quantity is required",
		},
		{
			name:    "custom codes update missing active",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "update", "--custom-code-id", "CUSTOM_ID"},
			wantErr: "Error: --active is required",
		},
		{
			name:    "prices list missing offer code id",
			args:    []string{"subscriptions", "offers", "offer-codes", "prices"},
			wantErr: "Error: --offer-code-id is required",
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

func TestOfferCodesValuesWritesCSVFile(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodeOneTimeUseCodes/batch-1/values" {
			t.Fatalf("expected values path, got %s", req.URL.Path)
		}
		if req.Header.Get("Accept") != "text/csv" {
			t.Fatalf("expected Accept=text/csv, got %q", req.Header.Get("Accept"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("code\nABC123\nDEF456\n")),
			Header:     http.Header{"Content-Type": []string{"text/csv"}},
		}, nil
	})

	outputPath := filepath.Join(t.TempDir(), "codes.csv")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "values",
			"--batch-id", "batch-1",
			"--output", outputPath,
			"--format", "csv",
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
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if got := string(data); got != "code\nABC123\nDEF456\n" {
		t.Fatalf("unexpected CSV output %q", got)
	}
}

func TestOfferCodesValuesWritesCSVFile_WriteFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodeOneTimeUseCodes/batch-1/values" {
			t.Fatalf("expected values path, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("code\nABC123\n")),
			Header:     http.Header{"Content-Type": []string{"text/csv"}},
		}, nil
	})

	outputPath := filepath.Join(t.TempDir(), "codes.csv")
	if err := os.Mkdir(outputPath, 0o755); err != nil {
		t.Fatalf("mkdir output path: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "values",
			"--batch-id", "batch-1",
			"--output", outputPath,
			"--format", "csv",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected write failure")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "offer-codes values:") {
		t.Fatalf("expected offer-codes values error, got %v", runErr)
	}
	if info, err := os.Stat(outputPath); err != nil || !info.IsDir() {
		t.Fatalf("expected output path to remain directory, info=%v err=%v", info, err)
	}
}

func TestOfferCodesValuesRejectsInvalidFormat(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "invalid format",
			args: []string{
				"subscriptions", "offers", "offer-codes", "values",
				"--batch-id", "batch-1",
				"--format", "yaml",
			},
		},
		{
			name: "root flags before subcommands",
			args: []string{
				"--profile", "ci",
				"subscriptions", "offers", "offer-codes", "values",
				"--batch-id", "batch-1",
				"--format", "yaml",
			},
		},
		{
			name: "mixed flag order",
			args: []string{
				"subscriptions", "offers", "offer-codes", "values",
				"--format", "yaml",
				"--batch-id", "batch-1",
			},
		},
		{
			name: "format value matches subcommand",
			args: []string{
				"subscriptions", "offers", "offer-codes", "values",
				"--batch-id", "batch-1",
				"--format", "values",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "1.2.3")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --format must be text or csv") {
				t.Fatalf("expected format error, got %q", stderr)
			}
		})
	}
}
