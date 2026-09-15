package workflow

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestBuildEnvSliceForcesReadOnlyIntoSteps(t *testing.T) {
	t.Setenv("ASC_WORKFLOW_ENV_PROBE", "probe")
	t.Setenv(readonly.EnvVar, "")

	if got := buildEnvSlice(nil); slices.Contains(got, readonly.EnvVar+"=1") {
		t.Fatalf("step environment carries %s=1 while read-only mode is off", readonly.EnvVar)
	}

	t.Setenv(readonly.EnvVar, "1")
	for _, env := range []map[string]string{
		nil,
		{"STEP_KEY": "step-value"},
		// A step's declared env must not override the session policy.
		{readonly.EnvVar: "0"},
	} {
		got := buildEnvSlice(env)
		if count := countEnvKey(got, readonly.EnvVar); count != 1 {
			t.Fatalf("step environment has %d %s entries, want 1: %v", count, readonly.EnvVar, got)
		}
		if got[len(got)-1] != readonly.EnvVar+"=1" {
			t.Fatalf("last step environment entry = %q, want %s=1", got[len(got)-1], readonly.EnvVar)
		}
		if !slices.Contains(got, "ASC_WORKFLOW_ENV_PROBE=probe") {
			t.Fatal("step environment dropped an inherited variable")
		}
	}
}

func countEnvKey(entries []string, key string) int {
	count := 0
	for _, entry := range entries {
		if len(entry) > len(key) && entry[:len(key)+1] == key+"=" {
			count++
		}
	}
	return count
}

// The capacity hint for the forced-entry slice adds one to an entry count, so
// it must stay bounded no matter how large that count is.
func TestForcedEnvCapacityStaysBounded(t *testing.T) {
	for _, entries := range []int{0, 1, maxForcedEnvEntries - 1, maxForcedEnvEntries, maxForcedEnvEntries + 1, math.MaxInt - 1, math.MaxInt} {
		t.Run(strconv.Itoa(entries), func(t *testing.T) {
			got := forcedEnvCapacity(entries)
			if got <= 0 {
				t.Fatalf("forcedEnvCapacity(%d) = %d, want a positive capacity (the addition overflowed)", entries, got)
			}
			if got > maxForcedEnvEntries+1 {
				t.Fatalf("forcedEnvCapacity(%d) = %d, want at most %d", entries, got, maxForcedEnvEntries+1)
			}
			if entries <= maxForcedEnvEntries && got != entries+1 {
				t.Fatalf("forcedEnvCapacity(%d) = %d, want %d", entries, got, entries+1)
			}
		})
	}
}

// The clamped hint is only a preallocation, so an environment larger than the
// clamp must still keep every entry and the forced entry last.
func TestForceReadOnlyEnvKeepsEveryEntryBeyondCapacityHint(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	env := make([]string, 0, maxForcedEnvEntries+2)
	env = append(env, readonly.EnvVar+"=0")
	for i := range maxForcedEnvEntries + 1 {
		env = append(env, fmt.Sprintf("ASC_WORKFLOW_ENV_PROBE_%d=probe", i))
	}

	got := forceReadOnlyEnv(env)
	if want := maxForcedEnvEntries + 2; len(got) != want {
		t.Fatalf("forced environment has %d entries, want %d", len(got), want)
	}
	if count := countEnvKey(got, readonly.EnvVar); count != 1 {
		t.Fatalf("forced environment has %d %s entries, want 1", count, readonly.EnvVar)
	}
	if got[len(got)-1] != readonly.EnvVar+"=1" {
		t.Fatalf("last forced environment entry = %q, want %s=1", got[len(got)-1], readonly.EnvVar)
	}
	if got[0] != "ASC_WORKFLOW_ENV_PROBE_0=probe" || got[len(got)-2] != fmt.Sprintf("ASC_WORKFLOW_ENV_PROBE_%d=probe", maxForcedEnvEntries) {
		t.Fatal("forced environment dropped or reordered an inherited variable")
	}
}
