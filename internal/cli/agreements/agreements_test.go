package agreements

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestAgreementsCommandShape(t *testing.T) {
	cmd := AgreementsCommand()
	if cmd == nil {
		t.Fatal("expected agreements command")
		return
	}
	if cmd.Name != "agreements" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 1 {
		t.Fatalf("expected 1 subcommand, got %d", len(cmd.Subcommands))
	}
}

func TestAgreementsTerritoriesListValidation(t *testing.T) {
	t.Run("missing id and next", func(t *testing.T) {
		cmd := AgreementsTerritoriesListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("invalid limit", func(t *testing.T) {
		cmd := AgreementsTerritoriesListCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "EULA_ID", "--limit", "300"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-ErrHelp limit error, got %v", err)
		}
	})
}

func TestExtractEULATerritoryIDFromNextURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{
			name:   "resource path",
			input:  "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements/123/territories?limit=50",
			wantID: "123",
		},
		{
			name:   "relationship path",
			input:  "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements/abc/relationships/territories?limit=50",
			wantID: "abc",
		},
		{
			name:    "invalid path",
			input:   "https://api.appstoreconnect.apple.com/v1/apps",
			wantErr: true,
		},
		{
			name:    "malformed host localhost double port",
			input:   "http://localhost:80:80/v1/endUserLicenseAgreements/123/territories?limit=50",
			wantErr: true,
		},
		{
			name:    "malformed host ipv6 without brackets",
			input:   "http://::1/v1/endUserLicenseAgreements/123/territories?limit=50",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractEULATerritoryIDFromNextURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantID {
				t.Fatalf("got id=%q, want %q", got, tc.wantID)
			}
		})
	}
}
