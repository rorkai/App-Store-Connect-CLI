package iap

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestIAPPriceSchedulePriceCommands_HaveResolvedFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  func(*shared.Deps) *ffcli.Command
	}{
		{name: "manual", cmd: newIAPPriceSchedulesManualPricesCommand},
		{name: "automatic", cmd: newIAPPriceSchedulesAutomaticPricesCommand},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _, _ := newTestDeps(nil)
			cmd := tc.cmd(d)
			if cmd.FlagSet.Lookup("resolved") == nil {
				t.Fatal("expected --resolved flag")
			}
			if !strings.Contains(cmd.LongHelp, "--resolved") {
				t.Fatalf("expected long help to mention --resolved, got %q", cmd.LongHelp)
			}
		})
	}
}
