package telemetry

import (
	"errors"
	"testing"
	"time"
)

func TestEmitSwallowsSenderErrors(t *testing.T) {
	clearContextEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	called := false
	original := sendHTTP
	sendHTTP = func(ev Event) error {
		called = true
		if ev.CommandPath != "asc builds list" {
			t.Fatalf("CommandPath = %q", ev.CommandPath)
		}
		return errors.New("network down")
	}
	t.Cleanup(func() { sendHTTP = original })

	Emit([]string{"builds", "list"}, "asc builds list", "1.2.3", time.Millisecond, 0)
	if !called {
		t.Fatal("expected sender to be called")
	}
}

func TestEmitHonorsDisabledEnv(t *testing.T) {
	clearContextEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")

	original := sendHTTP
	sendHTTP = func(ev Event) error {
		t.Fatal("sender should not be called when disabled")
		return nil
	}
	t.Cleanup(func() { sendHTTP = original })

	Emit([]string{"builds", "list"}, "asc builds list", "1.2.3", time.Millisecond, 0)
}
