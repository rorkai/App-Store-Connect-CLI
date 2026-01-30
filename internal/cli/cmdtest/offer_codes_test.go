package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"testing"
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
			name:    "offer codes get missing offer code id",
			args:    []string{"offer-codes", "get"},
			wantErr: "Error: --offer-code-id is required",
		},
		{
			name:    "offer codes create missing subscription id",
			args:    []string{"offer-codes", "create"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "offer codes create missing name",
			args:    []string{"offer-codes", "create", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --name is required",
		},
		{
			name:    "offer codes create missing customer eligibilities",
			args:    []string{"offer-codes", "create", "--subscription-id", "SUB_ID", "--name", "SPRING"},
			wantErr: "Error: --customer-eligibilities is required",
		},
		{
			name:    "offer codes update missing active",
			args:    []string{"offer-codes", "update", "--offer-code-id", "OFFER_ID"},
			wantErr: "Error: --active is required",
		},
		{
			name:    "custom codes list missing offer code id",
			args:    []string{"offer-codes", "custom-codes", "list"},
			wantErr: "Error: --offer-code-id is required",
		},
		{
			name:    "custom codes get missing custom code id",
			args:    []string{"offer-codes", "custom-codes", "get"},
			wantErr: "Error: --custom-code-id is required",
		},
		{
			name:    "custom codes create missing offer code id",
			args:    []string{"offer-codes", "custom-codes", "create", "--code", "SPRING", "--quantity", "10"},
			wantErr: "Error: --offer-code-id is required",
		},
		{
			name:    "custom codes create missing code",
			args:    []string{"offer-codes", "custom-codes", "create", "--offer-code-id", "OFFER_ID", "--quantity", "10"},
			wantErr: "Error: --code is required",
		},
		{
			name:    "custom codes create missing quantity",
			args:    []string{"offer-codes", "custom-codes", "create", "--offer-code-id", "OFFER_ID", "--code", "SPRING"},
			wantErr: "Error: --quantity is required",
		},
		{
			name:    "custom codes update missing active",
			args:    []string{"offer-codes", "custom-codes", "update", "--custom-code-id", "CUSTOM_ID"},
			wantErr: "Error: --active is required",
		},
		{
			name:    "prices list missing offer code id",
			args:    []string{"offer-codes", "prices", "list"},
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
