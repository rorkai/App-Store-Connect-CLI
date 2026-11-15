package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestMerchantIDsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "view missing merchant id",
			args:    []string{"merchant-ids", "view"},
			wantErr: "--merchant-id is required",
		},
		{
			name:    "create missing identifier",
			args:    []string{"merchant-ids", "create", "--name", "Example"},
			wantErr: "--identifier is required",
		},
		{
			name:    "create missing name",
			args:    []string{"merchant-ids", "create", "--identifier", "merchant.com.example"},
			wantErr: "--name is required",
		},
		{
			name:    "update missing merchant id",
			args:    []string{"merchant-ids", "update", "--name", "New Name"},
			wantErr: "--merchant-id is required",
		},
		{
			name:    "update missing name",
			args:    []string{"merchant-ids", "update", "--merchant-id", "m1"},
			wantErr: "--name is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"merchant-ids", "delete", "--merchant-id", "m1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "delete missing merchant id",
			args:    []string{"merchant-ids", "delete", "--confirm"},
			wantErr: "--merchant-id is required",
		},
		{
			name:    "certificates list missing merchant id",
			args:    []string{"merchant-ids", "certificates", "list"},
			wantErr: "--merchant-id is required",
		},
		{
			name:    "certificates view missing merchant id",
			args:    []string{"merchant-ids", "certificates", "view"},
			wantErr: "--merchant-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runRootWithDeps(t, nil, test.args...)
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
