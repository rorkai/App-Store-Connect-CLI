package profiles

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newTestDeps() *shared.Deps {
	return &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}, Now: time.Now}
}

func TestProfilesGetCommand_MissingID(t *testing.T) {
	cmd := newProfilesGetCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingName(t *testing.T) {
	cmd := newProfilesCreateCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--profile-type", "IOS_APP_DEVELOPMENT", "--bundle", "BUNDLE_ID", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --name is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingProfileType(t *testing.T) {
	cmd := newProfilesCreateCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--bundle", "BUNDLE_ID", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --profile-type is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingBundle(t *testing.T) {
	cmd := newProfilesCreateCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--profile-type", "IOS_APP_DEVELOPMENT", "--certificate", "CERT_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --bundle is missing, got %v", err)
	}
}

func TestProfilesCreateCommand_MissingCertificate(t *testing.T) {
	cmd := newProfilesCreateCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--name", "Profile", "--profile-type", "IOS_APP_DEVELOPMENT", "--bundle", "BUNDLE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --certificate is missing, got %v", err)
	}
}

func TestProfilesDeleteCommand_MissingID(t *testing.T) {
	cmd := newProfilesDeleteCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := newProfilesDeleteCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--id", "PROFILE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestProfilesDownloadCommand_MissingID(t *testing.T) {
	cmd := newProfilesDownloadCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--output", "./profile.mobileprovision"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestProfilesDownloadCommand_MissingOutput(t *testing.T) {
	cmd := newProfilesDownloadCommand(newTestDeps())

	if err := cmd.FlagSet.Parse([]string{"--id", "PROFILE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --output is missing, got %v", err)
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
