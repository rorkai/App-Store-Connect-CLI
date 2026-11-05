package validate

import "testing"

func TestValidateCommandAcceptsOptInURLChecks(t *testing.T) {
	cmd := ValidateCommand()
	if err := cmd.Parse([]string{"--app", "app-1", "--version-id", "version-1", "--check-urls"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	flagDef := cmd.FlagSet.Lookup("check-urls")
	if flagDef == nil {
		t.Fatal("--check-urls flag is not registered")
	}
}
