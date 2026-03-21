package reviews

import (
	"strings"
	"testing"
)

func TestReviewDetailsCreateCommandClarifiesReviewerAccessGuidance(t *testing.T) {
	cmd := ReviewDetailsCreateCommand()

	if !strings.Contains(cmd.LongHelp, "Leave `--demo-account-required` false when `--notes` are enough") {
		t.Fatalf("expected create help to explain notes-only guidance, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "Use `--demo-account-required=true` only when App Review needs demo credentials") {
		t.Fatalf("expected create help to explain demo credential opt-in, got %q", cmd.LongHelp)
	}

	if got := cmd.FlagSet.Lookup("demo-account-required").Usage; !strings.Contains(got, "Set true only when App Review needs demo credentials") {
		t.Fatalf("expected --demo-account-required usage to clarify semantics, got %q", got)
	}
	if got := cmd.FlagSet.Lookup("notes").Usage; !strings.Contains(got, "reviewer instructions") {
		t.Fatalf("expected --notes usage to mention reviewer instructions, got %q", got)
	}
	if !strings.Contains(cmd.LongHelp, `--contact-first-name "Dev" --contact-last-name "Support" --contact-email "dev@example.com" --contact-phone "+1 555 0100" --notes "Reviewer can use the guest flow from the welcome screen."`) {
		t.Fatalf("expected notes-only create example to include the required contact fields, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, `--contact-first-name "Dev" --contact-last-name "Support" --contact-email "dev@example.com" --contact-phone "+1 555 0100" --demo-account-required=true --demo-account-name "reviewer@example.com" --demo-account-password "app-specific-password" --notes "2FA is disabled for this review account."`) {
		t.Fatalf("expected credentialed create example to include the required contact fields, got %q", cmd.LongHelp)
	}
}

func TestReviewDetailsUpdateCommandClarifiesReviewerAccessGuidance(t *testing.T) {
	cmd := ReviewDetailsUpdateCommand()

	if !strings.Contains(cmd.LongHelp, "Leave `--demo-account-required` false when `--notes` are enough") {
		t.Fatalf("expected update help to explain notes-only guidance, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "Do not use placeholder demo credentials") {
		t.Fatalf("expected update help to discourage placeholder credentials, got %q", cmd.LongHelp)
	}

	if got := cmd.FlagSet.Lookup("demo-account-name").Usage; !strings.Contains(got, "when demo credentials are required") {
		t.Fatalf("expected --demo-account-name usage to clarify when it is needed, got %q", got)
	}
	if got := cmd.FlagSet.Lookup("demo-account-password").Usage; !strings.Contains(got, "when demo credentials are required") {
		t.Fatalf("expected --demo-account-password usage to clarify when it is needed, got %q", got)
	}
}
