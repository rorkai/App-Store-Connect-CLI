package cmdtest

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

func TestSubscriptionsPricingMonthlyCommitmentHelp(t *testing.T) {
	root := RootCommand("1.2.3")

	cmd := findSubcommand(root, "subscriptions", "pricing")
	if cmd == nil {
		t.Fatal("expected subscriptions pricing command")
		return
	}
	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "monthly-commitment") {
		t.Fatalf("expected pricing help to mention monthly-commitment, got %q", usage)
	}

	monthlyCmd := findSubcommand(root, "subscriptions", "pricing", "monthly-commitment")
	if monthlyCmd == nil {
		t.Fatal("expected monthly-commitment command")
		return
	}
	monthlyUsage := monthlyCmd.UsageFunc(monthlyCmd)
	if !strings.Contains(monthlyUsage, "April 27, 2026") {
		t.Fatalf("expected monthly-commitment help to mention Apple announcement date, got %q", monthlyUsage)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "enable missing subscription",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--price", "9.99", "--price-territory", "Norway", "--territories", "Norway"},
			wantErr: "--subscription-id is required",
		},
		{
			name:    "enable rejects excluded price territory",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--subscription-id", "sub-1", "--price", "9.99", "--price-territory", "United States", "--territories", "Norway"},
			wantErr: "--price-territory cannot be USA or Singapore",
		},
		{
			name:    "enable rejects excluded price territory with mixed flag order",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--territories", "Norway", "--price-territory", "USA", "--subscription-id", "sub-1", "--price", "9.99"},
			wantErr: "--price-territory cannot be USA or Singapore",
		},
		{
			name:    "disable missing territories",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "disable", "--subscription-id", "sub-1"},
			wantErr: "--territories is required",
		},
		{
			name:    "disable treats subcommand name as flag value",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "disable", "--subscription-id", "sub-1", "--territories", "list"},
			wantErr: "territory \"list\" could not be mapped",
		},
		{
			name:    "list missing subscription",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "list"},
			wantErr: "--subscription-id is required",
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

func TestSubscriptionsPricingMonthlyCommitmentUsageExitCodes(t *testing.T) {
	binaryPath := buildASCBlackBoxBinary(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "subcommand flag before subcommand returns usage",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "--subscription-id", "sub-1", "enable", "--price", "9.99", "--price-territory", "Norway", "--territories", "Norway"},
			wantErr: "flag provided but not defined: -subscription-id",
		},
		{
			name:    "mixed flag order invalid price territory returns usage",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--territories", "Norway", "--price-territory", "USA", "--subscription-id", "sub-1", "--price", "9.99"},
			wantErr: "--price-territory cannot be USA or Singapore",
		},
		{
			name:    "flag value matching subcommand returns usage when invalid",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "disable", "--subscription-id", "sub-1", "--territories", "list"},
			wantErr: "territory \"list\" could not be mapped",
		},
		{
			name:    "availability edit invalid billing mode returns usage",
			args:    []string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "sub-1", "--territories", "Norway", "--billing-mode", "list"},
			wantErr: "--billing-mode must be one of: upfront, monthly-commitment",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, test.args...)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected process exit error, got %v", err)
			}
			if exitErr.ExitCode() != 2 {
				t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
			}
			if stdout.String() != "" {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr.String())
			}
		})
	}
}

func TestSubscriptionsPricingMonthlyCommitmentDisableFiltersExcludedTerritories(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "disable",
			"--subscription-id", "sub-1",
			"--territories", "United States,Norway,Singapore",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "not yet supported by Apple's public App Store Connect API") {
			t.Fatalf("expected upstream API unsupported error, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Warning: monthly-commitment billing is unavailable in USA,SGP") {
		t.Fatalf("expected excluded territory warning, got %q", stderr)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentEnableRejectsPriceOutsideRange(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001" && req.Method == http.MethodGet:
			body := `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"name":"Yearly","productId":"com.example.yearly","subscriptionPeriod":"ONE_YEAR","state":"APPROVED"}}}`
			return jsonResponse(http.StatusOK, body)
		case req.URL.Path == "/v1/subscriptions/8000000001/prices" && req.Method == http.MethodGet:
			if got := req.URL.Query().Get("filter[territory]"); got != "NOR" {
				t.Fatalf("expected price territory NOR, got %q", got)
			}
			body := `{
				"data":[{
					"type":"subscriptionPrices","id":"price-1",
					"attributes":{"startDate":"2024-01-01"},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"NOR"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-1"}}
					}
				}],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-1","attributes":{"customerPrice":"120.00"}},
					{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
				],
				"links":{"next":""}
			}`
			return jsonResponse(http.StatusOK, body)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "enable",
			"--subscription-id", "8000000001",
			"--price", "15.01",
			"--price-territory", "Norway",
			"--territories", "Norway,Germany",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "monthly commitment total 180.12 is outside the allowed range [120.00, 180.00]") {
			t.Fatalf("expected pricing range rejection, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
