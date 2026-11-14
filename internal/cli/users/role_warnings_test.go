package users

import (
	"bytes"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestWarnDeprecatedUserRolesEmitsWarning(t *testing.T) {
	var stderr bytes.Buffer
	warnDeprecatedUserRoles(&shared.Deps{IO: &shared.IO{ErrOut: &stderr}}, []string{"DEVELOPER", "access_to_reports"})
	if stderr.String() != deprecatedAccessToReportsWarning+"\n" {
		t.Fatalf("expected deprecation warning, got %q", stderr.String())
	}
}

func TestWarnDeprecatedUserRolesNoWarningForOtherRoles(t *testing.T) {
	var stderr bytes.Buffer
	warnDeprecatedUserRoles(&shared.Deps{IO: &shared.IO{ErrOut: &stderr}}, []string{"ADMIN", "DEVELOPER"})
	if stderr.String() != "" {
		t.Fatalf("expected no warning, got %q", stderr.String())
	}
}
