package prerelease

import "testing"

func TestPreReleaseCommandConstructors(t *testing.T) {
	top := PreReleaseVersionsCommand()
	if top == nil {
		t.Fatal("expected pre-release command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := PreReleaseVersionsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := PreReleaseVersionsRelationshipsCommand(); got == nil {
		t.Fatal("expected relationships command")
	}
	if got := PreReleaseVersionsAppCommand(); got == nil {
		t.Fatal("expected related app command")
	}
}
