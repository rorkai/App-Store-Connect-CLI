package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestSubscriptionPricePointsFilterValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "price and min-price mutually exclusive",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--price", "4.99", "--min-price", "1.00"},
			wantErr: "--price and --min-price/--max-price are mutually exclusive",
		},
		{
			name:    "price and max-price mutually exclusive",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--price", "4.99", "--max-price", "9.99"},
			wantErr: "--price and --min-price/--max-price are mutually exclusive",
		},
		{
			name:    "invalid price value",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--price", "abc"},
			wantErr: "--price must be a number",
		},
		{
			name:    "non-finite price value",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--price", "NaN"},
			wantErr: "--price must be a finite number",
		},
		{
			name:    "invalid min-price",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--min-price", "abc"},
			wantErr: "--min-price must be a number",
		},
		{
			name:    "min exceeds max",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--min-price", "10.00", "--max-price", "5.00"},
			wantErr: "--min-price (10.00) cannot exceed --max-price (5.00)",
		},
		{
			name:    "price filter with stream is unsupported",
			args:    []string{"subscriptions", "price-points", "list", "--subscription-id", "sub-1", "--price", "4.99", "--stream", "--paginate"},
			wantErr: "price filtering is not supported with --stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected stderr to contain %q, got: %q", tt.wantErr, stderr)
			}
		})
	}
}

func TestIAPPricePointsFilterValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "price and min-price mutually exclusive",
			args:    []string{"iap", "price-points", "list", "--iap-id", "iap-1", "--price", "4.99", "--min-price", "1.00"},
			wantErr: "--price and --min-price/--max-price are mutually exclusive",
		},
		{
			name:    "invalid price value",
			args:    []string{"iap", "price-points", "list", "--iap-id", "iap-1", "--price", "abc"},
			wantErr: "--price must be a number",
		},
		{
			name:    "non-finite price value",
			args:    []string{"iap", "price-points", "list", "--iap-id", "iap-1", "--price", "NaN"},
			wantErr: "--price must be a finite number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected stderr to contain %q, got: %q", tt.wantErr, stderr)
			}
		})
	}
}
