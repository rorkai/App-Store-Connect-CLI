package shared

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDepsWaitHelpersMatchProcessVariants(t *testing.T) {
	for _, name := range []string{"ASC_TIMEOUT", "ASC_TIMEOUT_SECONDS", "ASC_MAX_DELAY"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("ASC_MAX_RETRIES", "2")
	t.Setenv("ASC_BASE_DELAY", "1s")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	timedOut := func(context.Context) (int, error) { return 0, context.DeadlineExceeded }
	noMatch := func(context.Context) (int, bool, error) { return 0, false, nil }
	wantTimedOut := func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want the last attempt's deadline", err)
		}
	}

	// Each run uses the process form when d is nil and the Deps form otherwise.
	tests := []struct {
		name  string
		waits []time.Duration
		run   func(t *testing.T, d *Deps)
	}{
		{name: "After", waits: []time.Duration{3 * time.Second}, run: func(t *testing.T, d *Deps) {
			if d == nil {
				<-time.After(3 * time.Second)
			} else {
				<-d.After(3 * time.Second)
			}
		}},
		{name: "RetryReadWithFreshTimeout", waits: []time.Duration{time.Second, 2 * time.Second}, run: func(t *testing.T, d *Deps) {
			var err error
			if d == nil {
				_, err = RetryReadWithFreshTimeout(context.Background(), timedOut)
			} else {
				_, err = RetryReadWithFreshTimeoutWith(context.Background(), d, timedOut)
			}
			wantTimedOut(t, err)
		}},
		{name: "RunReconciledMutation", waits: []time.Duration{time.Second, 2 * time.Second}, run: func(t *testing.T, d *Deps) {
			var err error
			if d == nil {
				_, _, err = RunReconciledMutation(context.Background(), timedOut, noMatch)
			} else {
				_, _, err = RunReconciledMutationWith(context.Background(), d, timedOut, noMatch)
			}
			wantTimedOut(t, err)
		}},
		{name: "RunReconciledMutationNoReplay", waits: []time.Duration{time.Second}, run: func(t *testing.T, d *Deps) {
			var err error
			if d == nil {
				_, _, err = RunReconciledMutationNoReplay(context.Background(), timedOut, noMatch)
			} else {
				_, _, err = RunReconciledMutationNoReplayWith(context.Background(), d, timedOut, noMatch)
			}
			wantTimedOut(t, err)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var waits []time.Duration
				// injected fires at once, so a form that waits on any other
				// timer takes time.
				injected := &Deps{
					Timeout: ContextWithTimeout,
					After: func(delay time.Duration) <-chan time.Time {
						waits = append(waits, delay)
						fired := make(chan time.Time, 1)
						fired <- time.Now()
						return fired
					},
				}
				var total time.Duration
				for _, wait := range test.waits {
					total += wait
				}
				for _, variant := range []struct {
					name    string
					d       *Deps
					elapsed time.Duration
				}{
					{"process", nil, total},
					{"DefaultDeps", DefaultDeps(), total},
					{"injected After", injected, 0},
				} {
					start := time.Now()
					test.run(t, variant.d)
					if elapsed := time.Since(start); elapsed != variant.elapsed {
						t.Fatalf("%s: waited %v, want %v", variant.name, elapsed, variant.elapsed)
					}
				}
				if !reflect.DeepEqual(waits, test.waits) {
					t.Fatalf("injected After waits = %v, want %v", waits, test.waits)
				}
			})
		})
	}
}

func TestDepsWithSpinnerDelayedStartsOnDepsTimer(t *testing.T) {
	t.Setenv("ASC_SPINNER_DISABLED", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)
	synctest.Test(t, func(t *testing.T) {
		var stderr bytes.Buffer
		var delays []time.Duration
		fire := make(chan time.Time, 1)
		d := &Deps{
			IO: &IO{Out: &bytes.Buffer{}, ErrOut: &stderr, StdoutTTY: true, StderrTTY: true},
			After: func(delay time.Duration) <-chan time.Time {
				delays = append(delays, delay)
				return fire
			},
		}
		err := d.WithSpinnerDelayed("Working", time.Second, func() error {
			time.Sleep(time.Minute)
			synctest.Wait()
			if stderr.Len() != 0 {
				t.Errorf("stderr before d.After fired = %q, want empty", stderr.String())
			}
			fire <- time.Now()
			synctest.Wait()
			if !strings.Contains(stderr.String(), "Working") {
				t.Errorf("stderr after d.After fired = %q, want the spinner", stderr.String())
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WithSpinnerDelayed() error: %v", err)
		}
		if want := []time.Duration{time.Second}; !reflect.DeepEqual(delays, want) {
			t.Fatalf("d.After delays = %v, want %v", delays, want)
		}
	})
}
