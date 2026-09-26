package profiles

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestProfilesListQueryFlagsAreRegistered(t *testing.T) {
	cmd := ProfilesListCommand()
	for _, name := range []string{
		"name", "id", "sort", "fields", "bundle-id-fields", "device-fields", "certificate-fields", "include", "limit-devices", "limit-certificates",
	} {
		flagValue := cmd.FlagSet.Lookup(name)
		if flagValue == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
}

func TestProfilesGetCommand_MissingID(t *testing.T) {
	cmd := ProfilesGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_NameLongerThanLimitIsUsageError(t *testing.T) {
	cmd := ProfilesCreateCommand()
	name := strings.Repeat("n", 65)
	if err := cmd.FlagSet.Parse([]string{
		"--name", name,
		"--profile-type", "IOS_APP_DEVELOPMENT",
		"--bundle", "BUNDLE_ID",
		"--certificate", "CERT_ID",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want a usage error", err)
	}
	if !strings.Contains(err.Error(), "at most 64") {
		t.Fatalf("error = %v, want the length limit", err)
	}
}

func TestProfilesCreateCommand_MissingName(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--profile-type", "IOS_APP_DEVELOPMENT", "--bundle", "BUNDLE_ID", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --name is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingProfileType(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--bundle", "BUNDLE_ID", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --profile-type is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingBundle(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--profile-type", "IOS_APP_DEVELOPMENT", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --bundle is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingCertificate(t *testing.T) {
	cmd := ProfilesCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--profile-type", "IOS_APP_DEVELOPMENT", "--bundle", "BUNDLE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --certificate is missing, got %v", err)
	}
}

func TestProfilesDeleteCommand_MissingID(t *testing.T) {
	cmd := ProfilesDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := ProfilesDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "PROFILE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestProfilesDownloadCommand_MissingID(t *testing.T) {
	cmd := ProfilesDownloadCommand()

	if err := cmd.FlagSet.Parse([]string{"--output", "./profile.mobileprovision"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesDownloadCommand_MissingOutput(t *testing.T) {
	cmd := ProfilesDownloadCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "PROFILE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --output is missing, got %v", err)
	}
}

func TestProfilesRelationshipsBundleIDCommand_MissingID(t *testing.T) {
	cmd := ProfilesRelationshipsBundleIDCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesRelationshipsCertificatesCommand_MissingID(t *testing.T) {
	cmd := ProfilesRelationshipsCertificatesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesRelationshipsDevicesCommand_MissingID(t *testing.T) {
	cmd := ProfilesRelationshipsDevicesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestExtractProfileIDFromNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/profiles/profile-123/relationships/certificates?cursor=abc"
	got, err := extractProfileIDFromNextURL(next, "certificates")
	if err != nil {
		t.Fatalf("extractProfileIDFromNextURL() error: %v", err)
	}
	if got != "profile-123" {
		t.Fatalf("expected profile-123, got %q", got)
	}
}

func TestExtractProfileIDFromNextURL_Invalid(t *testing.T) {
	_, err := extractProfileIDFromNextURL("https://api.appstoreconnect.apple.com/v1/profiles", "certificates")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractProfileIDFromNextURL_RejectsMalformedHost(t *testing.T) {
	tests := []string{
		"http://localhost:80:80/v1/profiles/profile-123/relationships/certificates?cursor=abc",
		"http://::1/v1/profiles/profile-123/relationships/certificates?cursor=abc",
	}

	for _, next := range tests {
		t.Run(next, func(t *testing.T) {
			if _, err := extractProfileIDFromNextURL(next, "certificates"); err == nil {
				t.Fatalf("expected error for malformed URL %q", next)
			}
		})
	}
}
