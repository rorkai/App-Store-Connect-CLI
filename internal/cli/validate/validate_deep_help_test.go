package validate

import (
	"strings"
	"testing"
)

func TestValidateHelpDocumentsDeepCachedSessionContract(t *testing.T) {
	d, _, _ := validateTestDeps()
	cmd := NewValidateCommand(d)
	for _, want := range []string{"--deep", "--apple-id", "Deep validation", "cached Apple web session", "App Privacy", "agreements", "subscription"} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("validate help missing %q:\n%s", want, cmd.LongHelp)
		}
	}
	for _, name := range []string{"deep", "apple-id"} {
		flagDef := cmd.FlagSet.Lookup(name)
		if flagDef == nil {
			t.Fatalf("--%s flag is not registered", name)
		}
	}
}
