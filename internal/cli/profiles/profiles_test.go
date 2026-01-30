package profiles

import (
	"context"
	"flag"
	"testing"
)

func TestProfilesGetCommand_MissingID(t *testing.T) {
	cmd := ProfilesGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingName(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--profile-type", "IOS_APP_DEVELOPMENT", "--bundle", "BUNDLE_ID", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --name is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingProfileType(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--bundle", "BUNDLE_ID", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --profile-type is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingBundle(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--profile-type", "IOS_APP_DEVELOPMENT", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --bundle is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingCertificate(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--profile-type", "IOS_APP_DEVELOPMENT", "--bundle", "BUNDLE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --certificate is missing, got %v", err)
	}
}

func TestProfilesDeleteCommand_MissingID(t *testing.T) {
	cmd := ProfilesDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := ProfilesDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "PROFILE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestProfilesDownloadCommand_MissingID(t *testing.T) {
	cmd := ProfilesDownloadCommand()

	if err := cmd.FlagSet.Parse([]string{"--output", "./profile.mobileprovision"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesDownloadCommand_MissingOutput(t *testing.T) {
	cmd := ProfilesDownloadCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "PROFILE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --output is missing, got %v", err)
	}
}

func TestProfilesRelationshipsBundleIDCommand_MissingID(t *testing.T) {
	cmd := ProfilesRelationshipsBundleIDCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesRelationshipsCertificatesCommand_MissingID(t *testing.T) {
	cmd := ProfilesRelationshipsCertificatesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesRelationshipsDevicesCommand_MissingID(t *testing.T) {
	cmd := ProfilesRelationshipsDevicesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}
