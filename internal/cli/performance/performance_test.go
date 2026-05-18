package performance

import "testing"

func TestPerformanceCommandConstructors(t *testing.T) {
	top := PerformanceCommand()
	if top == nil {
		t.Fatal("expected performance command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := PerformanceCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	if got := PerformanceMetricsCommand(); got == nil {
		t.Fatal("expected metrics command")
	}
	if got := PerformanceDiagnosticsCommand(); got == nil {
		t.Fatal("expected diagnostics command")
	}
	if got := PerformanceDownloadCommand(); got == nil {
		t.Fatal("expected download command")
	}
}
