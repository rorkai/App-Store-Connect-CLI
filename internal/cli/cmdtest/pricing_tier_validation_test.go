package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPricingRejectsExplicitZeroTierBeforeRequests(t *testing.T) {
	commands := []struct {
		name      string
		args      []string
		selectors [][]string
	}{
		{"app_schedule", []string{"pricing", "schedule", "create", "--app", "123456789", "--base-territory", "USA"}, [][]string{nil, {"--price-point", "PP"}, {"--price", "0.99"}, {"--free"}}},
		{"app_setup", []string{"app-setup", "pricing", "set", "--app", "123456789", "--base-territory", "USA"}, [][]string{nil, {"--price-point", "PP"}, {"--price", "0.99"}, {"--free"}}},
		{"subscription", []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000003", "--territory", "USA"}, [][]string{nil, {"--price-point", "PP"}, {"--price", "0.99"}}},
		{"iap", []string{"iap", "pricing", "schedules", "create", "--iap-id", "9000000003", "--base-territory", "USA"}, [][]string{nil, {"--prices", "PP"}, {"--price", "0.99"}}},
	}
	for _, command := range commands {
		for _, selector := range command.selectors {
			t.Run(command.name+"/"+strings.Join(selector, "_"), func(t *testing.T) {
				setupAuth(t)
				reg := asctest.NewRegistry(t)
				reg.Install()
				args := append(append([]string(nil), command.args...), "--tier", "0")
				args = append(args, selector...)
				var code int
				stdout, stderr := captureOutput(t, func() { code = rootcmd.Run(args, "1.2.3") })
				if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: --tier must be a positive integer\n") || len(reg.Requests()) != 0 {
					t.Fatalf("want tier usage error before requests, got code=%d requests=%d stdout=%q stderr=%q", code, len(reg.Requests()), stdout, stderr)
				}
			})
		}
	}
}
