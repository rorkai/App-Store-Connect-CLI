package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

var shellWaitDelay = 5 * time.Second

// bash evaluates BASH_ENV for non-interactive shells; never inherit or allow
// callers to provide it because workflow params and env are untrusted input.
const bashEnvKey = "BASH_ENV"

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
	names := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			setEnvironmentValue(result, names, k, v)
		}
	}
	return result
}

func environmentKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func setEnvironmentValue(values, names map[string]string, key, value string) {
	identity := environmentKey(key)
	if previous, exists := names[identity]; exists && previous != key {
		delete(values, previous)
	}
	values[key] = value
	names[identity] = key
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

var (
	errUnsupportedStepIf = errors.New("use NAME, $NAME, NAME == VALUE, or NAME != VALUE")
	stepIfName           = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// evalStepIf reports whether a step's `if` passes. NAME or $NAME tests the
// variable for a truthy value; NAME == VALUE and NAME != VALUE compare its
// value, or "" when unset, with a literal. Step env wins over process env.
func evalStepIf(expr string, env map[string]string) (bool, error) {
	values := env
	if runtime.GOOS == "windows" {
		values = make(map[string]string, len(env))
		for key, value := range env {
			values[environmentKey(key)] = value
		}
	}
	lookup := func(operand string) (string, error) {
		name := strings.TrimSpace(operand)
		if val, ok := values[environmentKey(name)]; ok {
			return val, nil
		}
		if val, ok := os.LookupEnv(name); ok {
			return val, nil
		}
		name = strings.TrimPrefix(name, "$")
		if !stepIfName.MatchString(name) {
			return "", errUnsupportedStepIf
		}
		if val, ok := values[environmentKey(name)]; ok {
			return val, nil
		}
		return os.Getenv(name), nil
	}

	op := strings.Index(expr, "==")
	if neq := strings.Index(expr, "!="); neq >= 0 && (op < 0 || neq < op) {
		op = neq
	}
	if op < 0 {
		val, err := lookup(expr)
		return isTruthy(val), err
	}
	val, err := lookup(expr[:op])
	if err != nil {
		return false, err
	}
	literal := strings.TrimSpace(expr[op+2:])
	if n := len(literal); n >= 2 && (literal[0] == '"' || literal[0] == '\'') && literal[n-1] == literal[0] {
		quote := literal[:1]
		literal = literal[1 : n-1]
		if strings.Contains(literal, quote) {
			return false, errUnsupportedStepIf
		}
	} else if strings.ContainsAny(literal, " \t\n\r=!<>&|") {
		return false, errUnsupportedStepIf
	}
	return (val == literal) == (expr[op] == '='), nil
}

// buildEnvSlice creates a []string for exec.Cmd.Env by overlaying the
// tracks env map onto os.Environ().
func buildEnvSlice(env map[string]string) []string {
	overrides := make(map[string]struct{}, len(env))
	for key := range env {
		overrides[environmentKey(key)] = struct{}{}
	}
	inherited := os.Environ()
	result := make([]string, 0, len(inherited)+len(env))
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		key = environmentKey(key)
		if key == bashEnvKey {
			continue
		}
		if _, replaced := overrides[key]; !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range env {
		if environmentKey(key) != bashEnvKey {
			result = append(result, key+"="+value)
		}
	}
	return forceReadOnlyEnv(result)
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

// LoadEnvFiles reads KEY=VALUE files in order; a later file overrides an
// earlier one. Errors name the file and line but never include line content,
// because values are often secrets.
func LoadEnvFiles(paths []string) (map[string]string, error) {
	env := make(map[string]string)
	names := make(map[string]string)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read env file: %w", err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if rest, ok := strings.CutPrefix(line, "export"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
				line = strings.TrimSpace(rest)
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("env file %s:%d: expected KEY=VALUE", path, i+1)
			}
			key = strings.TrimSpace(key)
			if !stepIfName.MatchString(key) {
				return nil, fmt.Errorf("env file %s:%d: invalid variable name", path, i+1)
			}
			if environmentKey(key) == bashEnvKey {
				return nil, fmt.Errorf("env file %s:%d: %s cannot be set from an env file", path, i+1, bashEnvKey)
			}
			value = strings.TrimSpace(value)
			if n := len(value); n >= 2 && (value[0] == '"' || value[0] == '\'') && value[n-1] == value[0] {
				value = value[1 : n-1]
			}
			setEnvironmentValue(env, names, key, value)
		}
	}
	return env, nil
}

// ParseParams converts CLI arguments in KEY:VALUE or KEY=VALUE format to a map.
func ParseParams(args []string) (map[string]string, error) {
	params := make(map[string]string, len(args))
	names := make(map[string]string)
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
		setEnvironmentValue(params, names, key, value)
	}
	return params, nil
}
