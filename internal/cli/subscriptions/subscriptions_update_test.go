package subscriptions

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func isolateSubscriptionsAuthEnv(t *testing.T) {
	t.Helper()

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
}

func TestSubscriptionsUpdateCommand_GroupLevelHelpOmitsSentinelDefault(t *testing.T) {
	cmd := SubscriptionsUpdateCommand()

	groupLevelFlag := cmd.FlagSet.Lookup("group-level")
	if groupLevelFlag == nil {
		t.Fatal("expected --group-level flag to be defined")
	}
	if groupLevelFlag.DefValue != "" {
		t.Fatalf("expected --group-level to have no displayed default, got %q", groupLevelFlag.DefValue)
	}

	help := shared.DefaultUsageFunc(cmd)
	if !strings.Contains(help, "--group-level") {
		t.Fatalf("expected help output to mention --group-level, got %q", help)
	}
	if strings.Contains(help, "(default: -1)") {
		t.Fatalf("expected help output to omit invalid sentinel default, got %q", help)
	}

	withFlag := cmd.FlagSet.Lookup("with")
	if withFlag == nil {
		t.Fatal("expected --with flag to be defined")
	}
	if !strings.Contains(help, "--with") {
		t.Fatalf("expected help output to mention --with, got %q", help)
	}
}

func TestSubscriptionsUpdateCommand_GroupLevelWorksAsOnlyUpdateFlag(t *testing.T) {
	isolateSubscriptionsAuthEnv(t)

	cmd := SubscriptionsUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--id", "sub-1", "--group-level", "3"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected --group-level to satisfy update validation, got %v", err)
	}
	if err == nil {
		t.Fatal("expected execution to continue to auth/client setup in test environment")
	}
}

func TestSubscriptionsUpdateCommand_WithWorksAsOnlyUpdateFlag(t *testing.T) {
	isolateSubscriptionsAuthEnv(t)

	cmd := SubscriptionsUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--id", "sub-1", "--with", "sub-2"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected --with to satisfy update validation, got %v", err)
	}
	if err == nil {
		t.Fatal("expected execution to continue to auth/client setup in test environment")
	}
}
