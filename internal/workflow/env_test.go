package workflow

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestMergeEnv(t *testing.T) {
	a := map[string]string{"A": "1", "B": "2"}
	b := map[string]string{"B": "3", "C": "4"}

	result := mergeEnv(a, b)
	if result["A"] != "1" {
		t.Fatalf("expected A=1, got %q", result["A"])
	}
	if result["B"] != "3" {
		t.Fatalf("expected B=3 (overridden), got %q", result["B"])
	}
	if result["C"] != "4" {
		t.Fatalf("expected C=4, got %q", result["C"])
	}
}

func TestMergeEnv_NilMaps(t *testing.T) {
	result := mergeEnv(nil, map[string]string{"A": "1"}, nil)
	if result["A"] != "1" {
		t.Fatalf("expected A=1, got %q", result["A"])
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"no", false},
		{"No", false},
		{"n", false},
		{"off", false},
		{"OFF", false},
		{"yep", false},   // unknown = falsy
		{"nope", false},  // unknown = falsy
		{"maybe", false}, // unknown = falsy
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"ON", true},
		{"  true  ", true}, // trimmed
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got := isTruthy(test.value)
			if got != test.want {
				t.Fatalf("isTruthy(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestBuildEnvSlice_AddsNew(t *testing.T) {
	env := map[string]string{"WORKFLOW_TEST_NEW": "value"}
	slice := buildEnvSlice(env)

	found := false
	for _, entry := range slice {
		if entry == "WORKFLOW_TEST_NEW=value" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected WORKFLOW_TEST_NEW=value in env slice")
	}
}

func TestBuildEnvSlice_OverridesExisting(t *testing.T) {
	t.Setenv("WORKFLOW_TEST_EXIST", "original")
	env := map[string]string{"WORKFLOW_TEST_EXIST": "overridden"}
	slice := buildEnvSlice(env)

	count := 0
	for _, entry := range slice {
		if strings.HasPrefix(entry, "WORKFLOW_TEST_EXIST=") {
			count++
			if entry != "WORKFLOW_TEST_EXIST=overridden" {
				t.Fatalf("expected overridden value, got %q", entry)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 entry, got %d", count)
	}
}

func TestBuildEnvSlice_StripsInheritedBashEnv(t *testing.T) {
	t.Setenv("BASH_ENV", "/tmp/workflow-test-bashenv")

	slice := buildEnvSlice(nil)
	for _, entry := range slice {
		if strings.HasPrefix(entry, "BASH_ENV=") {
			t.Fatalf("expected BASH_ENV to be stripped, got %q", entry)
		}
	}
}

func TestBuildEnvSlice_IgnoresProvidedBashEnvOverride(t *testing.T) {
	t.Setenv("BASH_ENV", "/tmp/workflow-test-bashenv")

	slice := buildEnvSlice(map[string]string{"BASH_ENV": "/tmp/workflow-param-bashenv"})
	for _, entry := range slice {
		if strings.HasPrefix(entry, "BASH_ENV=") {
			t.Fatalf("expected BASH_ENV to be ignored, got %q", entry)
		}
	}
}

func TestParseParams_ColonSeparator(t *testing.T) {
	params, err := ParseParams([]string{"KEY:value", "ANOTHER:val2"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["KEY"] != "value" {
		t.Fatalf("expected KEY=value, got %q", params["KEY"])
	}
	if params["ANOTHER"] != "val2" {
		t.Fatalf("expected ANOTHER=val2, got %q", params["ANOTHER"])
	}
}

func TestParseParams_EqualsSeparator(t *testing.T) {
	params, err := ParseParams([]string{"KEY=value"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["KEY"] != "value" {
		t.Fatalf("expected KEY=value, got %q", params["KEY"])
	}
}

func TestParseParams_ValueContainsSeparator(t *testing.T) {
	params, err := ParseParams([]string{"URL:https://example.com"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["URL"] != "https://example.com" {
		t.Fatalf("expected URL=https://example.com, got %q", params["URL"])
	}
}

func TestParseParams_EmptyValue(t *testing.T) {
	params, err := ParseParams([]string{"KEY:"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["KEY"] != "" {
		t.Fatalf("expected empty value, got %q", params["KEY"])
	}
}

func TestParseParams_NoSeparator(t *testing.T) {
	_, err := ParseParams([]string{"NOSEPARATOR"})
	if err == nil {
		t.Fatal("expected error for missing separator")
	}
}

func TestParseParams_EmptyArgs(t *testing.T) {
	params, err := ParseParams([]string{})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if len(params) != 0 {
		t.Fatalf("expected empty map, got %v", params)
	}
}

func TestParseParams_BothSeparators_UsesFirst(t *testing.T) {
	params, err := ParseParams([]string{"A:B=C"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["A"] != "B=C" {
		t.Fatalf("expected A='B=C', got %q", params["A"])
	}
}

func TestParseParams_EqualsBeforeColon(t *testing.T) {
	params, err := ParseParams([]string{"A=B:C"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["A"] != "B:C" {
		t.Fatalf("expected A='B:C', got %q", params["A"])
	}
}

func TestParseParams_WhitespaceKey(t *testing.T) {
	_, err := ParseParams([]string{" :value"})
	if err == nil {
		t.Fatal("expected error for whitespace-only key")
	}
}

func TestParseParams_DuplicateKey(t *testing.T) {
	params, err := ParseParams([]string{"A:1", "A:2"})
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params["A"] != "2" {
		t.Fatalf("expected last-wins A='2', got %q", params["A"])
	}
}

func TestParseParams_EqualsAtStart(t *testing.T) {
	_, err := ParseParams([]string{"=value"})
	if err == nil {
		t.Fatal("expected error for key starting with '='")
	}
}

func TestParseParams_ColonAtStart(t *testing.T) {
	_, err := ParseParams([]string{":value"})
	if err == nil {
		t.Fatal("expected error for key starting with ':'")
	}
}

func TestRunShellCommand_UsesBashWithPipefailWhenAvailable(t *testing.T) {
	resetShellCacheForTest()
	originalLookPathFn := lookPathFn
	originalCommandContextFn := commandContextFn
	t.Cleanup(func() {
		resetShellCacheForTest()
		lookPathFn = originalLookPathFn
		commandContextFn = originalCommandContextFn
	})

	lookPathFn = func(file string) (string, error) {
		if file == "bash" {
			return "/usr/bin/bash", nil
		}
		return "", exec.ErrNotFound
	}

	var gotName string
	var gotArgs []string
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.CommandContext(ctx, "go", "version")
	}

	if err := runShellCommand(context.Background(), "echo hi", nil, nil, nil); err != nil {
		t.Fatalf("runShellCommand() error: %v", err)
	}

	if gotName != "bash" {
		t.Fatalf("shell = %q, want bash", gotName)
	}
	wantArgs := []string{"-o", "pipefail", "-c", "echo hi"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestRunShellCommand_FallsBackToShWhenBashUnavailable(t *testing.T) {
	resetShellCacheForTest()
	originalLookPathFn := lookPathFn
	originalCommandContextFn := commandContextFn
	t.Cleanup(func() {
		resetShellCacheForTest()
		lookPathFn = originalLookPathFn
		commandContextFn = originalCommandContextFn
	})

	lookPathFn = func(file string) (string, error) {
		switch file {
		case "bash":
			return "", exec.ErrNotFound
		case "sh":
			return "/bin/sh", nil
		default:
			return "", errors.New("unexpected lookup: " + file)
		}
	}

	var gotName string
	var gotArgs []string
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return exec.CommandContext(ctx, "go", "version")
	}

	if err := runShellCommand(context.Background(), "echo hi", nil, nil, nil); err != nil {
		t.Fatalf("runShellCommand() error: %v", err)
	}

	if gotName != "sh" {
		t.Fatalf("shell = %q, want sh", gotName)
	}
	wantArgs := []string{"-c", "echo hi"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestRunShellCommand_NoSupportedShellFound(t *testing.T) {
	resetShellCacheForTest()
	originalLookPathFn := lookPathFn
	originalCommandContextFn := commandContextFn
	t.Cleanup(func() {
		resetShellCacheForTest()
		lookPathFn = originalLookPathFn
		commandContextFn = originalCommandContextFn
	})

	lookPathFn = func(file string) (string, error) {
		switch file {
		case "bash", "sh":
			return "", exec.ErrNotFound
		default:
			return "", errors.New("unexpected lookup: " + file)
		}
	}
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		t.Fatalf("commandContextFn should not be called when no shell is available (got name=%q args=%v)", name, args)
		return nil
	}

	err := runShellCommand(context.Background(), "echo hi", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error when no shell is available")
	}
	if !strings.Contains(err.Error(), "no supported shell") {
		t.Fatalf("expected 'no supported shell' error, got %v", err)
	}
}
