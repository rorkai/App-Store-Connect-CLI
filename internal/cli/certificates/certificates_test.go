package certificates

import (
	"context"
	"flag"
	"testing"
)

func TestCertificatesCreateCommand_MissingType(t *testing.T) {
	cmd := CertificatesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--csr", "./cert.csr"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --certificate-type is missing, got %v", err)
	}
}

func TestCertificatesCreateCommand_MissingCSR(t *testing.T) {
	cmd := CertificatesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--certificate-type", "IOS_DISTRIBUTION"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --csr is missing, got %v", err)
	}
}

func TestCertificatesRevokeCommand_MissingID(t *testing.T) {
	cmd := CertificatesRevokeCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestCertificatesRevokeCommand_MissingConfirm(t *testing.T) {
	cmd := CertificatesRevokeCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestCertificatesGetCommand_MissingID(t *testing.T) {
	cmd := CertificatesGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestCertificatesRelationshipsPassTypeIDCommand_MissingID(t *testing.T) {
	cmd := CertificatesRelationshipsPassTypeIDCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}
