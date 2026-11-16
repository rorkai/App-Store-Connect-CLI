package testflight

import (
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestFlightDistributionEditDoesNotRegisterExternalTesting(t *testing.T) {
	distribution := newTestFlightDistributionCommand(&shared.Deps{IO: &shared.IO{}, LookupEnv: os.LookupEnv})
	var editCommandFound bool
	for _, subcommand := range distribution.Subcommands {
		if subcommand.Name != "edit" {
			continue
		}
		editCommandFound = true

		if subcommand.FlagSet.Lookup("external-testing") != nil {
			t.Fatal("expected removed --external-testing flag to be unregistered")
		}
		if strings.Contains(subcommand.LongHelp, "external-testing") {
			t.Fatalf("expected help to omit --external-testing, got %q", subcommand.LongHelp)
		}
		if !strings.Contains(subcommand.LongHelp, `asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID" --submit --confirm`) {
			t.Fatalf("expected enable guidance in canonical help, got %q", subcommand.LongHelp)
		}
		if !strings.Contains(subcommand.LongHelp, `asc builds remove-groups --build-id "BUILD_ID" --group "GROUP_ID" --confirm`) {
			t.Fatalf("expected disable guidance in canonical help, got %q", subcommand.LongHelp)
		}
	}

	if !editCommandFound {
		t.Fatal("expected testflight distribution edit command")
	}
}
