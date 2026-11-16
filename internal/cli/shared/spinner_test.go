package shared

import (
	"errors"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func withTTYStub(t *testing.T, stdoutTTY, stderrTTY bool) {
	t.Helper()

	// go test -json gives the test binary one fd for stdout and stderr, which
	// an fd-keyed stub cannot tell apart.
	if os.Stdout.Fd() == os.Stderr.Fd() {
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("open %s: %v", os.DevNull, err)
		}
		prevStderr := os.Stderr
		os.Stderr = devNull
		t.Cleanup(func() {
			os.Stderr = prevStderr
			_ = devNull.Close()
		})
	}

	prevIsTerminal := isTerminal
	stdoutFD := int(os.Stdout.Fd())
	stderrFD := int(os.Stderr.Fd())
	isTerminal = func(fd int) bool {
		switch fd {
		case stdoutFD:
			return stdoutTTY
		case stderrFD:
			return stderrTTY
		default:
			return true
		}
	}
	t.Cleanup(func() { isTerminal = prevIsTerminal })
}

func resetSpinnerTestState(t *testing.T) {
	t.Helper()

	// Ensure env/config-driven debug behavior is deterministic.
	t.Setenv("ASC_DEBUG", "")
	t.Setenv("ASC_RETRY_LOG", "")

	// Clear any asc-level overrides that could have been set by other tests.
	asc.SetDebugOverride(nil)
	asc.SetDebugHTTPOverride(nil)
	asc.SetRetryLogOverride(nil)

	prevNoProgress := noProgress
	prevDebug := debug
	prevAPIDebug := apiDebug
	prevRetryLog := retryLog
	t.Cleanup(func() {
		noProgress = prevNoProgress
		debug = prevDebug
		apiDebug = prevAPIDebug
		retryLog = prevRetryLog
	})

	noProgress = false
	debug = OptionalBool{}
	apiDebug = OptionalBool{}
	retryLog = OptionalBool{}
}

func TestSpinnerEnabled_InteractiveDefault(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, true)

	// Ensure ASC_SPINNER_DISABLED is unset.
	original, had := os.LookupEnv(spinnerDisabledEnvVar)
	_ = os.Unsetenv(spinnerDisabledEnvVar)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(spinnerDisabledEnvVar, original)
		} else {
			_ = os.Unsetenv(spinnerDisabledEnvVar)
		}
	})

	if !SpinnerEnabled() {
		t.Fatal("expected SpinnerEnabled() to be true on interactive stdout+stderr")
	}
}

func TestSpinnerEnabled_DisabledWhenStdoutNotTTY(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, false, true)

	if SpinnerEnabled() {
		t.Fatal("expected SpinnerEnabled() to be false when stdout is not a TTY")
	}
}

func TestSpinnerEnabled_DisabledWhenStderrNotTTY(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, false)

	if SpinnerEnabled() {
		t.Fatal("expected SpinnerEnabled() to be false when stderr is not a TTY")
	}
}

func TestSpinnerEnabled_ASCSpinnerDisabledEnvVar(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, true)

	t.Run("disables_on_truthy_and_invalid", func(t *testing.T) {
		for _, v := range []string{"1", "true", "yes", "garbage"} {
			t.Run(v, func(t *testing.T) {
				t.Setenv(spinnerDisabledEnvVar, v)
				if SpinnerEnabled() {
					t.Fatalf("expected SpinnerEnabled() to be false for %s=%q", spinnerDisabledEnvVar, v)
				}
			})
		}
	})

	t.Run("allows_on_falsey", func(t *testing.T) {
		for _, v := range []string{"0", "false", "no", ""} {
			t.Run(v, func(t *testing.T) {
				t.Setenv(spinnerDisabledEnvVar, v)
				if !SpinnerEnabled() {
					t.Fatalf("expected SpinnerEnabled() to be true for %s=%q", spinnerDisabledEnvVar, v)
				}
			})
		}
	})
}

func TestSpinnerEnabled_DisabledWhenDebugOrRetryNoisy(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, true)

	t.Run("debug_env", func(t *testing.T) {
		t.Setenv("ASC_DEBUG", "1")
		if SpinnerEnabled() {
			t.Fatal("expected SpinnerEnabled() to be false when ASC_DEBUG enables debug logs")
		}
	})

	t.Run("retry_log_env", func(t *testing.T) {
		t.Setenv("ASC_RETRY_LOG", "1")
		if SpinnerEnabled() {
			t.Fatal("expected SpinnerEnabled() to be false when ASC_RETRY_LOG enables retry logging")
		}
	})
}

func TestSpinnerEnabled_DebugFlagOverridesEnv(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, true)

	t.Setenv("ASC_DEBUG", "1")

	if err := debug.Set("false"); err != nil {
		t.Fatalf("debug.Set(false) error: %v", err)
	}

	// --debug=false should disable debug logs even if ASC_DEBUG=1.
	if !SpinnerEnabled() {
		t.Fatal("expected SpinnerEnabled() to be true when --debug=false overrides ASC_DEBUG")
	}
}

func TestSpinnerEnabled_FlagsDisableSpinner(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, true)

	t.Run("debug_flag_true", func(t *testing.T) {
		if err := debug.Set("true"); err != nil {
			t.Fatalf("debug.Set(true) error: %v", err)
		}
		if SpinnerEnabled() {
			t.Fatal("expected SpinnerEnabled() to be false when --debug enables noisy stderr logging")
		}
	})

	t.Run("api_debug_flag_true", func(t *testing.T) {
		resetSpinnerTestState(t)
		withTTYStub(t, true, true)
		if err := apiDebug.Set("true"); err != nil {
			t.Fatalf("apiDebug.Set(true) error: %v", err)
		}
		if SpinnerEnabled() {
			t.Fatal("expected SpinnerEnabled() to be false when --api-debug enables noisy stderr logging")
		}
	})

	t.Run("retry_log_flag_true", func(t *testing.T) {
		resetSpinnerTestState(t)
		withTTYStub(t, true, true)
		if err := retryLog.Set("true"); err != nil {
			t.Fatalf("retryLog.Set(true) error: %v", err)
		}
		if SpinnerEnabled() {
			t.Fatal("expected SpinnerEnabled() to be false when --retry-log enables noisy stderr logging")
		}
	})
}

func TestSpinnerEnabled_DisabledWhenNoProgress(t *testing.T) {
	resetSpinnerTestState(t)
	withTTYStub(t, true, true)

	noProgress = true
	if SpinnerEnabled() {
		t.Fatal("expected SpinnerEnabled() to be false when progress is globally suppressed")
	}
}

func TestWithSpinner_NoOpWhenDisabled(t *testing.T) {
	resetSpinnerTestState(t)

	stdout, stderr := captureOutput(t, func() {
		withTTYStub(t, false, true) // stdout not a TTY => SpinnerEnabled() false

		ran := false
		if err := DefaultDeps().WithSpinner("ignored", func() error {
			ran = true
			return nil
		}); err != nil {
			t.Fatalf("WithSpinner() error: %v", err)
		}
		if !ran {
			t.Fatal("expected WithSpinner() to run fn when spinner is disabled")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr when spinner is disabled, got %q", stderr)
	}
}

func TestWithSpinner_EnabledWritesAndClearsLine_NoANSI(t *testing.T) {
	resetSpinnerTestState(t)

	// Force spinner enabled in test environment even if the developer has
	// ASC_SPINNER_DISABLED set globally.
	t.Setenv(spinnerDisabledEnvVar, "0")

	stdout, stderr := captureOutput(t, func() {
		withTTYStub(t, true, true)

		if err := DefaultDeps().WithSpinner("Working", func() error { return nil }); err != nil {
			t.Fatalf("WithSpinner() error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "\r"+spinnerFrames[0]+" Working") {
		t.Fatalf("expected spinner to render initial frame + label, got %q", stderr)
	}
	if !strings.HasSuffix(stderr, "\r") {
		t.Fatalf("expected stderr to end with carriage return after clear, got %q", stderr)
	}
	if strings.Contains(stderr, "\x1b") {
		t.Fatalf("expected no ANSI escape codes in spinner output, got %q", stderr)
	}
}

func TestWithSpinner_ReturnsError(t *testing.T) {
	resetSpinnerTestState(t)
	t.Setenv(spinnerDisabledEnvVar, "0")

	_, _ = captureOutput(t, func() {
		withTTYStub(t, true, true)

		want := errors.New("boom")
		err := DefaultDeps().WithSpinner("", func() error { return want })
		if !errors.Is(err, want) {
			t.Fatalf("expected error %v, got %v", want, err)
		}
	})
}

func TestWithSpinner_RePanics(t *testing.T) {
	resetSpinnerTestState(t)
	t.Setenv(spinnerDisabledEnvVar, "0")

	_, _ = captureOutput(t, func() {
		withTTYStub(t, true, true)

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected WithSpinner() to re-panic")
			}
		}()
		_ = DefaultDeps().WithSpinner("", func() error { panic("kaboom") })
	})
}

func TestWithSpinner_NilFuncIsNoOp(t *testing.T) {
	resetSpinnerTestState(t)
	t.Setenv(spinnerDisabledEnvVar, "0")

	stdout, stderr := captureOutput(t, func() {
		withTTYStub(t, true, true)

		if err := DefaultDeps().WithSpinner("ignored", nil); err != nil {
			t.Fatalf("expected nil error for nil fn, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for nil fn, got %q", stderr)
	}
}

func TestWithSpinnerDelayed_NoOpWhenFast(t *testing.T) {
	resetSpinnerTestState(t)
	t.Setenv(spinnerDisabledEnvVar, "0")

	stdout, stderr := captureOutput(t, func() {
		withTTYStub(t, true, true)

		var err error
		synctest.Test(t, func(*testing.T) {
			err = WithSpinnerDelayed("Working", 200*time.Millisecond, func() error { return nil })
		})
		if err != nil {
			t.Fatalf("WithSpinnerDelayed() error: %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr when fn returns before delay, got %q", stderr)
	}
}

func TestWithSpinnerDelayed_StartsWhenSlow(t *testing.T) {
	resetSpinnerTestState(t)
	t.Setenv(spinnerDisabledEnvVar, "0")

	stdout, stderr := captureOutput(t, func() {
		withTTYStub(t, true, true)

		var err error
		synctest.Test(t, func(*testing.T) {
			err = WithSpinnerDelayed("Working", 10*time.Millisecond, func() error {
				time.Sleep(30 * time.Millisecond)
				return nil
			})
		})
		if err != nil {
			t.Fatalf("WithSpinnerDelayed() error: %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "\r"+spinnerFrames[0]+" Working") {
		t.Fatalf("expected delayed spinner to render initial frame + label, got %q", stderr)
	}
}
