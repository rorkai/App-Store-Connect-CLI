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
)

const batchImportConfirmError = "--confirm is required unless --dry-run is set"

func writeBatchImportFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestFileDrivenMutationsRequireConfirm proves that every file-driven mutation
// command refuses to mutate until the caller either previews with --dry-run or
// accepts the change with --confirm, and that it fails before any network call.
func TestFileDrivenMutationsRequireConfirm(t *testing.T) {
	pricesCSV := writeBatchImportFile(t, "prices.csv", "territory,price\nUSA,0.99\n")
	offersCSV := writeBatchImportFile(t, "offers.csv", "territory\nUSA\n")
	testersCSV := writeBatchImportFile(t, "testers.csv", "email,first_name,last_name,groups\nrita@example.com,Rita,Tester,\n")
	repliesJSON := writeBatchImportFile(t, "replies.json", `{"replies":[{"response":"Thanks for the feedback.","reviewIds":["REVIEW_1"]}]}`)
	iapJSON := writeBatchImportFile(t, "iap.json", `{"products":[{"type":"CONSUMABLE","referenceName":"Pro","productId":"com.example.pro"}]}`)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "subscription prices import",
			args: []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "6000000001", "--input", pricesCSV},
		},
		{
			name: "introductory offers import",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "6000000001",
				"--input", offersCSV,
				"--offer-duration", "ONE_WEEK",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
			},
		},
		{
			name: "testflight testers import",
			args: []string{"testflight", "testers", "import", "--app", "123456789", "--input", testersCSV},
		},
		{
			name: "reviews respond-batch",
			args: []string{"reviews", "respond-batch", "--app", "123456789", "--file", repliesJSON},
		},
		{
			name: "migrate import",
			args: []string{"migrate", "import", "--app", "123456789", "--version-id", "VERSION_1", "--fastlane-dir", t.TempDir()},
		},
		{
			name: "iap import",
			args: []string{"iap", "import", "--app", "123456789", "--file", iapJSON},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected HTTP request before the apply decision: %s %s", req.Method, req.URL.String())
				return nil, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, batchImportConfirmError) {
				t.Fatalf("expected %q on stderr, got %q", batchImportConfirmError, stderr)
			}
		})
	}
}

// TestFileDrivenMutationsGateBeforeReadingInput proves the apply decision is
// uniform across every file-driven mutation command: it is evaluated before the
// input is opened, so even a missing input path reports the absent --confirm
// first and no request leaves the process.
func TestFileDrivenMutationsGateBeforeReadingInput(t *testing.T) {
	missingInput := filepath.Join(t.TempDir(), "does-not-exist.csv")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "subscription prices import",
			args: []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "6000000001", "--input", missingInput},
		},
		{
			name: "introductory offers import",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "6000000001",
				"--input", missingInput,
				"--offer-duration", "ONE_WEEK",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
			},
		},
		{
			name: "testflight testers import",
			args: []string{"testflight", "testers", "import", "--app", "123456789", "--input", missingInput},
		},
		{
			name: "reviews respond-batch",
			args: []string{"reviews", "respond-batch", "--app", "123456789", "--file", missingInput},
		},
		{
			name: "migrate import",
			args: []string{"migrate", "import", "--app", "123456789", "--version-id", "VERSION_1", "--fastlane-dir", missingInput},
		},
		{
			name: "iap import",
			args: []string{"iap", "import", "--app", "123456789", "--file", missingInput},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected HTTP request before the apply decision: %s %s", req.Method, req.URL.String())
				return nil, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, batchImportConfirmError) {
				t.Fatalf("expected %q on stderr, got %q", batchImportConfirmError, stderr)
			}
		})
	}
}

// TestFileDrivenMutationsValidateFlagsBeforeConfirmGate keeps flag-value
// validation ahead of the apply decision: a bad flag value reports its
// specific error even when --confirm is absent, and no request is issued.
func TestFileDrivenMutationsValidateFlagsBeforeConfirmGate(t *testing.T) {
	setupAuth(t)

	pricesCSV := writeBatchImportFile(t, "prices.csv", "territory,price\nUSA,0.99\n")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request during flag validation: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "import",
			"--subscription-id", "6000000001",
			"--input", pricesCSV,
			"--start-date", "not-a-date",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--start-date") {
		t.Fatalf("expected the start-date error, got %q", stderr)
	}
	if strings.Contains(stderr, batchImportConfirmError) {
		t.Fatalf("expected the specific flag error before the confirm gate, got %q", stderr)
	}
}

// TestFileDrivenMutationsAcceptDryRunWithConfirm keeps the shared convention:
// --dry-run wins over --confirm so contradictory input never mutates.
func TestFileDrivenMutationsAcceptDryRunWithConfirm(t *testing.T) {
	setupAuth(t)

	offersCSV := writeBatchImportFile(t, "offers.csv", "territory\nUSA\n")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request during dry run: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "6000000001",
			"--input", offersCSV,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--dry-run",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"dryRun":true`) {
		t.Fatalf("expected dry-run plan on stdout, got %q", stdout)
	}
}
