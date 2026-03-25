package web

import (
	"context"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestBindWebSessionFlagsIncludesDeprecatedTwoFactorAlias(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags := bindWebSessionFlags(fs)

	if flags.twoFactorCode == nil {
		t.Fatal("expected deprecated two-factor-code pointer to be populated")
	}

	twoFactorCodeFlag := fs.Lookup(deprecatedTwoFactorCodeFlagName)
	if twoFactorCodeFlag == nil {
		t.Fatalf("expected --%s to be registered", deprecatedTwoFactorCodeFlagName)
	}
	if !strings.Contains(twoFactorCodeFlag.Usage, "Deprecated:") {
		t.Fatalf("expected deprecated help text, got %q", twoFactorCodeFlag.Usage)
	}

	if fs.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command to remain registered")
	}
}

func TestResolveWebSessionForCommandPassesTwoFactorCodeCommand(t *testing.T) {
	restoreResolve := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		if appleID != "user@example.com" {
			t.Fatalf("appleID = %q, want %q", appleID, "user@example.com")
		}
		if twoFactorCode != "" {
			t.Fatalf("twoFactorCode = %q, want empty", twoFactorCode)
		}
		if twoFactorCodeCommand != "osascript /tmp/get-apple-2fa-code.scpt" {
			t.Fatalf("twoFactorCodeCommand = %q, want osascript helper", twoFactorCodeCommand)
		}
		return &webcore.AuthSession{}, "test", nil
	})
	t.Cleanup(restoreResolve)

	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCode:        ptrTo(""),
		twoFactorCodeCommand: ptrTo("osascript /tmp/get-apple-2fa-code.scpt"),
	}

	session, err := resolveWebSessionForCommand(context.Background(), flags)
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error = %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
}

func ptrTo(value string) *string {
	return &value
}
