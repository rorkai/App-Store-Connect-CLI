package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"testing"
)

func TestTestFlightMetricsBetaTesterUsagesValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing app",
			args: []string{"testflight", "metrics", "beta-tester-usages"},
		},
		{
			name: "invalid period",
			args: []string{"testflight", "metrics", "beta-tester-usages", "--app", "APP_ID", "--period", "P1D"},
		},
		{
			name: "limit out of range",
			args: []string{"testflight", "metrics", "beta-tester-usages", "--app", "APP_ID", "--limit", "500"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}
