package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
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
