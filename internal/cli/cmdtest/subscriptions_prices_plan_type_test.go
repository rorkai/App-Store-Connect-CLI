package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

func TestSubscriptionsPricingPricesListSendsPlanTypeFilter(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("filter[planType]"); got != "MONTHLY" {
			t.Fatalf("expected filter[planType]=MONTHLY, got %q", got)
		}
		body := `{"data":[{"type":"subscriptionPrices","id":"price-monthly","attributes":{"planType":"MONTHLY","startDate":"2026-01-01","preserved":false}}]}`
		return jsonResponse(http.StatusOK, body)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
			"--plan-type", "MONTHLY",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}

	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "price-monthly" {
		t.Fatalf("unexpected data: %#v", got.Data)
	}
}

func TestSubscriptionsPricingPricesListPaginatePreservesPlanTypeFilter(t *testing.T) {
	setupAuth(t)

	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("filter[planType]"); got != "MONTHLY" {
			t.Fatalf("request %d: expected filter[planType]=MONTHLY, got %q", requests, got)
		}
		switch requests {
		case 1:
			body := `{"data":[{"type":"subscriptionPrices","id":"price-monthly-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=next"}}`
			return jsonResponse(http.StatusOK, body)
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-monthly-2"}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request count: %d", requests)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
			"--plan-type", "MONTHLY",
			"--paginate",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if requests != 2 {
		t.Fatalf("expected two paginated requests, got %d", requests)
	}

	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
	}
	if len(got.Data) != 2 || got.Data[0].ID != "price-monthly-1" || got.Data[1].ID != "price-monthly-2" {
		t.Fatalf("unexpected data: %#v", got.Data)
	}
}

func TestSubscriptionsPricingPricesListPaginatePreservesPlanTypeFilterOnRelativeNextURL(t *testing.T) {
	setupAuth(t)

	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("filter[planType]"); got != "MONTHLY" {
			t.Fatalf("request %d: expected filter[planType]=MONTHLY, got %q", requests, got)
		}
		switch requests {
		case 1:
			body := `{"data":[{"type":"subscriptionPrices","id":"price-monthly-1"}],"links":{"next":"/v1/subscriptions/8000000001/prices?cursor=next"}}`
			return jsonResponse(http.StatusOK, body)
		case 2:
			if got := req.URL.Query().Get("cursor"); got != "next" {
				t.Fatalf("expected cursor=next, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-monthly-2"}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request count: %d", requests)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
			"--plan-type", "MONTHLY",
			"--paginate",
			"--output", "json",
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
	if requests != 2 {
		t.Fatalf("expected two paginated requests, got %d", requests)
	}
}

func TestSubscriptionsPricingPricesListNextPreservesPlanTypeFilter(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=next"
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("cursor"); got != "next" {
			t.Fatalf("expected cursor=next, got %q", got)
		}
		if got := req.URL.Query().Get("filter[planType]"); got != "UPFRONT" {
			t.Fatalf("expected filter[planType]=UPFRONT, got %q", got)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-upfront"}],"links":{"next":""}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--next", nextURL,
			"--plan-type", "UPFRONT",
			"--output", "json",
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
}

func TestSubscriptionsPricingPricesListPlanTypeValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid plan type",
			args: []string{
				"subscriptions", "pricing", "prices", "list",
				"--subscription-id", "8000000001",
				"--plan-type", "annual",
			},
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name: "empty plan type",
			args: []string{
				"subscriptions", "pricing", "prices", "list",
				"--subscription-id", "8000000001",
				"--plan-type", "",
			},
			wantErr: "invalid value for --plan-type: cannot be empty",
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

func TestSubscriptionsPricingPricesListPlanTypeUsageExitCodes(t *testing.T) {
	binaryPath := buildASCBlackBoxBinary(t)

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{
			name:    "invalid plan type",
			value:   "annual",
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "empty plan type",
			value:   "",
			wantErr: "invalid value for --plan-type: cannot be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(
				binaryPath,
				"subscriptions", "pricing", "prices", "list",
				"--subscription-id", "8000000001",
				"--plan-type", test.value,
			)

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
