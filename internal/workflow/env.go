package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

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
	if len(env) == 0 {
		return base
	}

	// Build an index once so each override is O(1) instead of scanning base.
	// Only track keys that we may override to avoid indexing unrelated env vars.
	indexByKey := make(map[string]int, len(env))
	remaining := len(env)
	for i, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
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
		entry := k + "=" + v
		if i, ok := indexByKey[k]; ok {
			base[i] = entry
			continue
		}
		indexByKey[k] = len(base)
		base = append(base, entry)
	}
	return base
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
	cmd.Env = buildEnvSlice(env)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
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
		_, _ = fmt.Fprintf(stderr, "[dry-run] hook: %s\n", command)
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
