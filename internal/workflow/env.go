package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

var shellWaitDelay = 5 * time.Second

var (
	lookPathFn       = exec.LookPath
	commandContextFn = exec.CommandContext

	cachedShellMu    sync.RWMutex
	cachedShellName  string
	cachedShellFlags []string
)

// mergeEnv merges environment maps in order. Later values override earlier.
func mergeEnv(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// isTruthy returns true if a value is explicitly truthy.
// Truthy: "1", "true", "yes", "y", "on" (case-insensitive).
// Everything else (empty, "0", "false", "no", "n", "off", unknown) is falsy.
func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// buildEnvSlice creates a []string for exec.Cmd.Env by overlaying the
// tracks env map onto os.Environ().
func buildEnvSlice(env map[string]string) []string {
	base := os.Environ()

	// bash evaluates BASH_ENV for non-interactive shells; never inherit or allow
	// callers to provide it because workflow params and env are untrusted input.
	const bashEnvKey = "BASH_ENV"
	sanitized := make([]string, 0, len(base))
	for _, entry := range base {
		if strings.HasPrefix(entry, bashEnvKey+"=") {
			continue
		}
		sanitized = append(sanitized, entry)
	}
	base = sanitized

	if len(env) == 0 {
		return forceReadOnlyEnv(base)
	}

	// Build an index once so each override is O(1) instead of scanning base.
	// Only track keys that we may override to avoid indexing unrelated env vars.
	indexByKey := make(map[string]int, len(env))
	remaining := len(env)
	if _, hasBashEnvOverride := env[bashEnvKey]; hasBashEnvOverride {
		remaining--
	}
	for i, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if key == bashEnvKey {
			continue
		}
		if _, needsOverride := env[key]; !needsOverride {
			continue
		}
		if _, exists := indexByKey[key]; !exists {
			indexByKey[key] = i
			remaining--
			if remaining == 0 {
				break
			}
		}
	}

	for k, v := range env {
		if k == bashEnvKey {
			continue
		}
		entry := k + "=" + v
		if i, ok := indexByKey[k]; ok {
			base[i] = entry
			continue
		}
		indexByKey[k] = len(base)
		base = append(base, entry)
	}
	return forceReadOnlyEnv(base)
}

// forceReadOnlyEnv propagates read-only mode into workflow steps. A step runs
// a shell command that may invoke asc again, and the root --read-only flag is
// process-local, so the child needs the environment variable to inherit the
// policy. It is applied after the step's declared env so a workflow file cannot
// override it declaratively. This is inheritance rather than containment: a
// step's shell command can still clear the variable for a process it starts,
// and a step that runs a different tool never reaches these guards.
func forceReadOnlyEnv(env []string) []string {
	if !readonly.Enabled() {
		return env
	}
	forced := make([]string, 0, forcedEnvCapacity(len(env)))
	for _, entry := range env {
		if key, _, ok := strings.Cut(entry, "="); ok && key == readonly.EnvVar {
			continue
		}
		forced = append(forced, entry)
	}
	return append(forced, readonly.EnvVar+"=1")
}

// maxForcedEnvEntries bounds the capacity hint computed by forcedEnvCapacity.
// A process environment is orders of magnitude smaller than this, so the clamp
// never changes the allocation for a real environment; it only keeps the
// arithmetic provably free of integer overflow.
const maxForcedEnvEntries = 1 << 16

// forcedEnvCapacity returns the capacity hint for the slice forceReadOnlyEnv
// builds: one slot per inherited entry plus one for the forced entry. The
// addition is clamped so no entry count can overflow it. The result is only a
// hint, so append still grows the slice correctly beyond the clamp.
func forcedEnvCapacity(entries int) int {
	if entries > maxForcedEnvEntries {
		return maxForcedEnvEntries + 1
	}
	return entries + 1
}

// runShellCommand executes a command string via bash -o pipefail -c when bash
// is available. It falls back to sh -c when bash is unavailable.
// Bash preserves pipeline failures (e.g., "false | cat") for CI correctness.
func runShellCommand(ctx context.Context, command string, env map[string]string, stdout, stderr io.Writer) error {
	shell, flags, err := resolveShell()
	if err != nil {
		return err
	}
	args := append(append([]string{}, flags...), command)

	cmd := commandContextFn(ctx, shell, args...)
	configureProcessTree(cmd)
	cmd.WaitDelay = shellWaitDelay
	cmd.Env = buildEnvSlice(env)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) && ctx.Err() == nil {
		// The shell exited successfully, but a background descendant retained a
		// captured output pipe. WaitDelay closed the pipe to bound cleanup; do not
		// turn that completed command into a retryable failure.
		return nil
	}
	return err
}

func resolveShell() (string, []string, error) {
	cachedShellMu.RLock()
	if cachedShellName != "" {
		name := cachedShellName
		flags := append([]string(nil), cachedShellFlags...)
		cachedShellMu.RUnlock()
		return name, flags, nil
	}
	cachedShellMu.RUnlock()

	cachedShellMu.Lock()
	defer cachedShellMu.Unlock()

	if cachedShellName != "" {
		return cachedShellName, append([]string(nil), cachedShellFlags...), nil
	}

	if _, err := lookPathFn("bash"); err == nil {
		cachedShellName = "bash"
		cachedShellFlags = []string{"-o", "pipefail", "-c"}
		return cachedShellName, append([]string(nil), cachedShellFlags...), nil
	}
	if _, err := lookPathFn("sh"); err == nil {
		cachedShellName = "sh"
		cachedShellFlags = []string{"-c"}
		return cachedShellName, append([]string(nil), cachedShellFlags...), nil
	}
	return "", nil, fmt.Errorf("workflow: no supported shell found (need bash or sh)")
}

// runHook executes a hook command. No-op if command is empty or whitespace-only.
func runHook(ctx context.Context, command string, env map[string]string, dryRun bool, stdout, stderr io.Writer) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	if dryRun {
		fmt.Fprintf(stderr, "[dry-run] hook: %s\n", command)
		return nil
	}
	return runShellCommand(ctx, command, env, stdout, stderr)
}

func resetShellCacheForTest() {
	cachedShellMu.Lock()
	defer cachedShellMu.Unlock()
	cachedShellName = ""
	cachedShellFlags = nil
}

// ParseParams converts CLI arguments in KEY:VALUE or KEY=VALUE format to a map.
func ParseParams(args []string) (map[string]string, error) {
	params := make(map[string]string, len(args))
	for _, arg := range args {
		colonIdx := strings.Index(arg, ":")
		equalsIdx := strings.Index(arg, "=")

		var idx int
		switch {
		case colonIdx > 0 && equalsIdx > 0:
			// Use whichever comes first
			if colonIdx < equalsIdx {
				idx = colonIdx
			} else {
				idx = equalsIdx
			}
		case colonIdx > 0:
			idx = colonIdx
		case equalsIdx > 0:
			idx = equalsIdx
		default:
			return nil, fmt.Errorf("invalid parameter %q (expected KEY:VALUE or KEY=VALUE)", arg)
		}

		key := arg[:idx]
		value := arg[idx+1:]
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid parameter %q (key must not be empty or whitespace)", arg)
		}
		params[key] = value
	}
	return params, nil
}
