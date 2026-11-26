package cmd

import (
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestNoTelemetryFlagOptsOutBeforeAnyEvent(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "root flag", args: []string{"--no-telemetry", "apps", "list", "--bogus"}, want: true},
		{name: "after the command", args: []string{"apps", "list", "--no-telemetry", "--bogus"}, want: true},
		{name: "explicit false", args: []string{"--no-telemetry=false", "apps", "list", "--bogus"}, want: false},
		{name: "after terminator", args: []string{"apps", "list", "--bogus", "--", "--no-telemetry"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetReportFlags(t)
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			emitted := false
			optedOut := false
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, _ telemetry.EventContext) {
				emitted = true
				optedOut = telemetry.ProcessOptedOut()
			}

			captureCommandOutput(t, func() {
				if code := Run(tt.args, "4.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if !emitted {
				t.Fatal("expected the usage failure to reach telemetry")
			}
			if optedOut != tt.want {
				t.Fatalf("ProcessOptedOut() at emit = %v, want %v", optedOut, tt.want)
			}
			if telemetry.ProcessOptedOut() {
				t.Fatal("process opt-out leaked past Run")
			}
		})
	}
}
