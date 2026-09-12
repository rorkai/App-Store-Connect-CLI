package xcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStapleRunsResolutionThenStapleThenValidation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "My App;$(touch should-not-run).dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")

	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_STDOUT", "staple stdout\n")
	t.Setenv("ASC_STAPLER_STAPLE_STDERR", "staple stderr\n")
	t.Setenv("ASC_STAPLER_VALIDATE_STDOUT", "validate stdout\n")
	t.Setenv("ASC_STAPLER_VALIDATE_STDERR", "validate stderr\n")

	var diagnostics bytes.Buffer
	result, err := Staple(context.Background(), target, &diagnostics)
	if err != nil {
		t.Fatalf("Staple() error = %v", err)
	}
	if result == nil || result.Path != target || result.Operation != string(StaplerOperationStaple) || !result.Stapled || !result.Validated {
		t.Fatalf("Staple() result = %#v, want a verified staple result", result)
	}
	if !strings.Contains(diagnostics.String(), "staple stdout") ||
		!strings.Contains(diagnostics.String(), "staple stderr") ||
		!strings.Contains(diagnostics.String(), "validate stdout") ||
		!strings.Contains(diagnostics.String(), "validate stderr") {
		t.Fatalf("diagnostics = %q, want child stdout/stderr", diagnostics.String())
	}

	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
		"xcrun|stapler|validate|" + target,
	})
}

func TestStapleRunsStageVerifierAroundEachOperation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)

	var stages []string
	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		position := "after"
		if before {
			position = "before"
		}
		stages = append(stages, position+" "+string(operation))
		return nil
	})
	if err != nil {
		t.Fatalf("StapleWithVerifier() error = %v", err)
	}
	if result == nil || !result.Stapled || !result.Validated {
		t.Fatalf("StapleWithVerifier() result = %#v, want verified result", result)
	}
	want := []string{"before staple", "after staple", "before validate", "after validate"}
	if !reflect.DeepEqual(stages, want) {
		t.Fatalf("verified stages = %#v, want %#v", stages, want)
	}
}

func TestStapleWithVerifierMarksPostStapleVerifierFailureAsPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	wantErr := errors.New("target changed after staple")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationStaple && !before {
			return wantErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after verifier failure", result)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
		t.Fatalf("StapleWithVerifier() error = %T %v, want post-staple partial marker", err, err)
	}
	if !strings.Contains(err.Error(), "post-staple") {
		t.Fatalf("StapleWithVerifier() error = %v, want post-staple phase", err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierJoinsStapleChildAndPostStapleVerifierFailures(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_EXIT_CODE", "66")
	verifierErr := errors.New("target changed after staple")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationStaple && !before {
			return verifierErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after post-staple verification failure", result)
	}
	if !errors.Is(err, verifierErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
		t.Fatalf("StapleWithVerifier() error = %T %v, want staple partial marker", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != 66 {
		t.Fatalf("StapleWithVerifier() error = %T %v, want joined staple/66 child error", err, err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) || stageErr.Before {
		t.Fatalf("StapleWithVerifier() error = %T %v, want post-staple stage error", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierMarksPreValidationVerifierFailureAsPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	verifierErr := errors.New("target changed before validation")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationValidate && before {
			return verifierErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after post-staple verification failure", result)
	}
	if !errors.Is(err, verifierErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
		t.Fatalf("StapleWithVerifier() error = %T %v, want staple partial marker", err, err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) || !stageErr.Before || stageErr.Operation != StaplerOperationValidate {
		t.Fatalf("StapleWithVerifier() error = %T %v, want pre-validation stage error", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierMarksPostValidationVerifierFailureAsPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	wantErr := errors.New("target changed after validation")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationValidate && !before {
			return wantErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after verifier failure", result)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationValidate {
		t.Fatalf("StapleWithVerifier() error = %T %v, want post-validation partial marker", err, err)
	}
	if !strings.Contains(err.Error(), "after staple") {
		t.Fatalf("StapleWithVerifier() error = %v, want post-staple phase", err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
		"xcrun|stapler|validate|" + target,
	})
}

func TestStapleWithVerifierJoinsPostValidationChildAndVerifierFailures(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_EXIT_CODE", "65")
	verifierErr := errors.New("target changed after validation")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationValidate && !before {
			return verifierErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after verifier failure", result)
	}
	if !errors.Is(err, verifierErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationValidate {
		t.Fatalf("StapleWithVerifier() error = %T %v, want post-validation partial marker", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != 65 {
		t.Fatalf("StapleWithVerifier() error = %T %v, want joined validate/65 child error", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
		"xcrun|stapler|validate|" + target,
	})
}

func TestStapleWithVerifierKeepsPreStapleVerifierFailureOrdinary(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	wantErr := errors.New("target unavailable before staple")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationStaple && before {
			return wantErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil before verifier failure", result)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if errors.As(err, &partialErr) {
		t.Fatalf("StapleWithVerifier() error = %v, pre-staple failure must not be partial mutation", err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) || !stageErr.Before {
		t.Fatalf("StapleWithVerifier() error = %T %v, want pre-staple stage error", err, err)
	}
	assertStaplerCommands(t, logPath, []string{"xcrun|--find|stapler"})
}

func TestStapleWithVerifierDoesNotMarkStartFailureAsPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	lookupCommandContext := commandContextFn
	previousCommandContext := commandContextFn
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if filepath.Base(name) == "xcrun" && len(args) == 2 && args[0] == "--find" && args[1] == "stapler" {
			return lookupCommandContext(ctx, name, args...)
		}
		return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-stapler"), args...)
	}
	t.Cleanup(func() { commandContextFn = previousCommandContext })
	verifierErr := errors.New("target changed after a child start failure")

	result, err := StapleWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationStaple && !before {
			return verifierErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after child start failure", result)
	}
	if err == nil || !errors.Is(err, verifierErr) {
		t.Fatalf("StapleWithVerifier() error = %v, want verifier cause", err)
	}
	var partialErr *StaplerPartialMutationError
	if errors.As(err, &partialErr) {
		t.Fatalf("StapleWithVerifier() error = %v, child that never started must not be partial mutation", err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) || stageErr.Before {
		t.Fatalf("StapleWithVerifier() error = %T %v, want joined post-staple stage error", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationStaple) {
		t.Fatalf("StapleWithVerifier() error = %T %v, want retained staple start failure", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
	})
}

func TestStaplerPreservesStartFailureWhenContextCancelsDuringFormatting(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	previousCommandContext := commandContextFn
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "stapler" {
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-stapler"), args...)
		}
		return previousCommandContext(ctx, name, args...)
	}
	t.Cleanup(func() { commandContextFn = previousCommandContext })

	// The start failure is followed by a cancellation observed while formatting
	// that failure.
	ctx := newStaplerCancelAfterContextChecks(1)
	result, err := Staple(ctx, target, nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after child start failure", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationStaple) {
		t.Fatalf("Staple() error = %T %v, want preserved staple start failure", err, err)
	}
	if commandErr.ExitCode != -1 {
		t.Fatalf("Staple() command error = %#v, want no process exit status", commandErr)
	}
	if !isStaplerOperationNotStarted(err) {
		t.Fatalf("Staple() error = %T %v, want not-started marker", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
	})
}

func TestStapleWithVerifierMarksCancellationBeforeFollowUpValidation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stages []string
	result, err := StapleWithVerifier(ctx, target, nil, func(operation StaplerOperation, before bool) error {
		position := "after"
		if before {
			position = "before"
		}
		stages = append(stages, position+" "+string(operation))
		if operation == StaplerOperationStaple && !before {
			cancel()
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after cancellation", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StapleWithVerifier() error = %v, want context cancellation", err)
	}
	if !strings.Contains(err.Error(), "after staple") {
		t.Fatalf("StapleWithVerifier() error = %v, want post-staple validation phase", err)
	}
	if want := []string{"before staple", "after staple", "before validate", "after validate"}; !reflect.DeepEqual(stages, want) {
		t.Fatalf("verified stages = %#v, want %#v", stages, want)
	}
}

func TestStapleWithVerifierMarksCancellationDuringFollowUpValidation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	readyPath := filepath.Join(t.TempDir(), "validate-ready")
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_WAIT", "1")
	t.Setenv("ASC_STAPLER_VALIDATE_READY_PATH", readyPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		result *StaplerResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := StapleWithVerifier(ctx, target, nil, func(StaplerOperation, bool) error {
			return nil
		})
		done <- outcome{result: result, err: err}
	}()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			cancel()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("validation helper did not report readiness")
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case got := <-done:
		if got.result != nil {
			t.Fatalf("StapleWithVerifier() result = %#v, want nil after cancellation", got.result)
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("StapleWithVerifier() error = %v, want context cancellation", got.err)
		}
		if !strings.Contains(got.err.Error(), "after staple") {
			t.Fatalf("StapleWithVerifier() error = %v, want post-staple validation phase", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StapleWithVerifier() did not return after cancellation")
	}
}

func TestValidateStapleRunsOnlyValidationAfterResolution(t *testing.T) {
	target := filepath.Join(t.TempDir(), "My App.pkg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)

	result, err := ValidateStaple(context.Background(), target, ioDiscardForStaplerTest{})
	if err != nil {
		t.Fatalf("ValidateStaple() error = %v", err)
	}
	if result == nil || result.Path != target || result.Operation != string(StaplerOperationValidate) || result.Stapled || !result.Validated {
		t.Fatalf("ValidateStaple() result = %#v, want a validation-only result", result)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|validate|" + target,
	})
}

func TestValidateWithVerifierRunsVerifierAroundValidationChild(t *testing.T) {
	target := filepath.Join(t.TempDir(), "My App.pkg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)

	var stages []string
	result, err := ValidateWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		position := "after"
		if before {
			position = "before"
		}
		stages = append(stages, position+" "+string(operation))
		return nil
	})
	if err != nil {
		t.Fatalf("ValidateWithVerifier() error = %v", err)
	}
	if result == nil || result.Path != target || result.Operation != string(StaplerOperationValidate) || result.Stapled || !result.Validated {
		t.Fatalf("ValidateWithVerifier() result = %#v, want a validation-only result", result)
	}
	if want := []string{"before validate", "after validate"}; !reflect.DeepEqual(stages, want) {
		t.Fatalf("verified stages = %#v, want %#v", stages, want)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|validate|" + target,
	})
}

func TestValidateWithVerifierWrapsStageErrorsAndSkipsValidationChild(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.pkg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	wantErr := errors.New("target identity mismatch")

	result, err := ValidateWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation != StaplerOperationValidate || !before {
			t.Fatalf("verifier called for %s before=%t, want validate before", operation, before)
		}
		return wantErr
	})
	if result != nil {
		t.Fatalf("ValidateWithVerifier() result = %#v, want nil on verifier failure", result)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("ValidateWithVerifier() error = %v, want wrapped verifier error", err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) {
		t.Fatalf("ValidateWithVerifier() error = %T %v, want stage verification error", err, err)
	}
	if stageErr.Operation != StaplerOperationValidate || !stageErr.Before {
		t.Fatalf("stage verification error = %#v, want validate/before", stageErr)
	}
	assertStaplerCommands(t, logPath, []string{"xcrun|--find|stapler"})
}

func TestValidateWithVerifierRunsAfterVerifierWhenValidationFails(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.pkg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_EXIT_CODE", "65")

	var stages []string
	result, err := ValidateWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		position := "after"
		if before {
			position = "before"
		}
		stages = append(stages, position+" "+string(operation))
		return nil
	})
	if result != nil {
		t.Fatalf("ValidateWithVerifier() result = %#v, want nil on child failure", result)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != 65 {
		t.Fatalf("ValidateWithVerifier() error = %#v, want validate/65 command error", err)
	}
	if want := []string{"before validate", "after validate"}; !reflect.DeepEqual(stages, want) {
		t.Fatalf("verified stages = %#v, want %#v", stages, want)
	}
}

func TestValidateWithVerifierJoinsChildAndPostValidationVerifierFailures(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.pkg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_EXIT_CODE", "65")
	verifierErr := errors.New("target changed after validation")

	result, err := ValidateWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationValidate && !before {
			return verifierErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("ValidateWithVerifier() result = %#v, want nil after post-validation verifier failure", result)
	}
	if !errors.Is(err, verifierErr) {
		t.Fatalf("ValidateWithVerifier() error = %v, want verifier cause", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != 65 {
		t.Fatalf("ValidateWithVerifier() error = %T %v, want joined validate/65 child error", err, err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) || stageErr.Before || stageErr.Operation != StaplerOperationValidate {
		t.Fatalf("ValidateWithVerifier() error = %T %v, want post-validation stage error", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|validate|" + target,
	})
}

func TestValidateWithVerifierRejectsReplacementDetectedAfterLookup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.pkg")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	originalInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_SWAP_ON_FIND", target)
	restored := false
	t.Cleanup(func() {
		if restored {
			return
		}
		replacementPath := target + ".replacement"
		if _, statErr := os.Stat(target); statErr == nil {
			_ = os.Rename(target, replacementPath)
		}
		_ = os.Rename(target+".original", target)
		_ = os.Remove(replacementPath)
	})

	wantErr := errors.New("target identity changed")
	result, err := ValidateWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		if operation != StaplerOperationValidate || !before {
			t.Fatalf("verifier called for %s before=%t, want validate before", operation, before)
		}
		current, statErr := os.Stat(target)
		if statErr != nil {
			return statErr
		}
		if !os.SameFile(originalInfo, current) {
			return wantErr
		}
		return nil
	})
	if result != nil {
		t.Fatalf("ValidateWithVerifier() result = %#v, want nil on replacement", result)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("ValidateWithVerifier() error = %v, want replacement failure", err)
	}
	assertStaplerCommands(t, logPath, []string{"xcrun|--find|stapler"})

	if err := os.Rename(target, target+".replacement"); err != nil {
		t.Fatalf("move replacement: %v", err)
	}
	if err := os.Rename(target+".original", target); err != nil {
		t.Fatalf("restore original target: %v", err)
	}
	restored = true
}

func TestValidateWithVerifierRestoresLookupSwapBeforeValidationChild(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.pkg")
	original := []byte("original")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	observedPath := filepath.Join(t.TempDir(), "observed-by-child")
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_SWAP_AND_RESTORE_ON_FIND", target)
	t.Setenv("ASC_STAPLER_VALIDATE_OBSERVED_PATH", observedPath)

	var stages []string
	result, err := ValidateWithVerifier(context.Background(), target, nil, func(operation StaplerOperation, before bool) error {
		position := "after"
		if before {
			position = "before"
		}
		stages = append(stages, position+" "+string(operation))
		if operation == StaplerOperationValidate && before {
			current, readErr := os.ReadFile(target)
			if readErr != nil {
				return readErr
			}
			if !bytes.Equal(current, original) {
				return fmt.Errorf("pre-child target = %q, want original", current)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ValidateWithVerifier() error = %v, want success after lookup restoration", err)
	}
	if result == nil || result.Path != target || result.Operation != string(StaplerOperationValidate) || !result.Validated {
		t.Fatalf("ValidateWithVerifier() result = %#v, want truthful validation result", result)
	}
	observed, err := os.ReadFile(observedPath)
	if err != nil {
		t.Fatalf("read child observation: %v", err)
	}
	if !bytes.Equal(observed, original) {
		t.Fatalf("validation child observed %q, want original bytes", observed)
	}
	if want := []string{"before validate", "after validate"}; !reflect.DeepEqual(stages, want) {
		t.Fatalf("verified stages = %#v, want %#v", stages, want)
	}
	final, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read final target: %v", err)
	}
	if !bytes.Equal(final, original) {
		t.Fatalf("final target = %q, want original bytes", final)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|validate|" + target,
	})
}

func TestValidateWithVerifierRejectsReplacementByValidationHelper(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.pkg")
	original := []byte("original")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	originalInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	observedPath := filepath.Join(t.TempDir(), "observed-by-child")
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_REPLACE_AFTER_VALIDATE", target)
	t.Setenv("ASC_STAPLER_VALIDATE_OBSERVED_PATH", observedPath)
	restored := false
	t.Cleanup(func() {
		if restored {
			return
		}
		replacementPath := target + ".replacement"
		if _, statErr := os.Stat(target); statErr == nil {
			_ = os.Rename(target, replacementPath)
		}
		_ = os.Rename(target+".original", target)
		_ = os.Remove(replacementPath)
	})

	wantErr := errors.New("target identity changed after validation child")
	var diagnostics bytes.Buffer
	result, err := ValidateWithVerifier(context.Background(), target, &diagnostics, func(operation StaplerOperation, before bool) error {
		if operation != StaplerOperationValidate {
			return nil
		}
		current, statErr := os.Stat(target)
		if statErr != nil {
			return statErr
		}
		if before {
			if !os.SameFile(originalInfo, current) {
				return wantErr
			}
			return nil
		}
		if os.SameFile(originalInfo, current) {
			return nil
		}
		return wantErr
	})
	if result != nil {
		t.Fatalf("ValidateWithVerifier() result = %#v, want nil after helper replacement", result)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("ValidateWithVerifier() error = %v, want post-child identity failure", err)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("child diagnostics = %q, want no success output or diagnostics", diagnostics.String())
	}
	observed, err := os.ReadFile(observedPath)
	if err != nil {
		t.Fatalf("read child observation: %v", err)
	}
	if !bytes.Equal(observed, original) {
		t.Fatalf("validation helper observed %q, want original bytes before replacement", observed)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|validate|" + target,
	})

	if err := os.Rename(target, target+".replacement"); err != nil {
		t.Fatalf("move replacement: %v", err)
	}
	if err := os.Rename(target+".original", target); err != nil {
		t.Fatalf("restore original target: %v", err)
	}
	if err := os.Remove(target + ".replacement"); err != nil {
		t.Fatalf("remove replacement: %v", err)
	}
	restored = true
}

func TestStapleStopsBeforeValidationWhenStapleFails(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_EXIT_CODE", "66")
	t.Setenv("ASC_STAPLER_STAPLE_STDERR", "not a supported artifact\n")

	var diagnostics bytes.Buffer
	result, err := Staple(context.Background(), target, &diagnostics)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil on staple failure", result)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want StaplerCommandError", err, err)
	}
	if commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != 66 {
		t.Fatalf("Staple() command error = %#v, want staple/66", commandErr)
	}
	if !strings.Contains(diagnostics.String(), "not a supported artifact") {
		t.Fatalf("diagnostics = %q, want child diagnostic", diagnostics.String())
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierMarksFailedStapleChildAsPossibleMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_EXIT_CODE", "66")

	// A post-staple verifier that recaptures a new baseline cannot prove that a
	// failed staple child left the artifact untouched, so the runner must keep
	// the unverified-mutation classification.
	result, err := StapleWithVerifier(context.Background(), target, nil, func(StaplerOperation, bool) error {
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil on staple failure", result)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) {
		t.Fatalf("StapleWithVerifier() error = %T %v, want partial mutation for a started staple child", err, err)
	}
	if partialErr.Interrupted {
		t.Fatalf("StapleWithVerifier() error = %v, must not report an ordinary child failure as interrupted", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("StapleWithVerifier() error = %T %v, want preserved child command error", err, err)
	}
	if commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != 66 {
		t.Fatalf("StapleWithVerifier() command error = %#v, want staple/66", commandErr)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStaplePreservesChildExitWhenContextCancelsAfterWait(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_EXIT_CODE", "65")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousWaitHook := afterStaplerCommandWaitFn
	afterStaplerCommandWaitFn = cancel
	t.Cleanup(func() { afterStaplerCommandWaitFn = previousWaitHook })

	result, err := Staple(ctx, target, nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after canceled failed child", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want preserved child command error", err, err)
	}
	if commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != 65 {
		t.Fatalf("Staple() command error = %#v, want staple/65", commandErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 65 {
		t.Fatalf("Staple() error = %T %v, want underlying exit 65", err, err)
	}
}

func TestStaplePreservesSignaledChildWhenContextCancelsAfterWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt is not implemented for child processes on Windows")
	}
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_SIGNAL", "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousWaitHook := afterStaplerCommandWaitFn
	afterStaplerCommandWaitFn = cancel
	t.Cleanup(func() { afterStaplerCommandWaitFn = previousWaitHook })

	result, err := Staple(ctx, target, nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after signaled child", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || !partialErr.Interrupted {
		t.Fatalf("Staple() error = %T %v, want interrupted partial-mutation marker", err, err)
	}
	if !isStaplerOperationAttemptedSignal(err) {
		t.Fatalf("Staple() error = %T %v, want attempted-signal marker", err, err)
	}
	if isStaplerOperationAttemptedCancellation(err) {
		t.Fatalf("Staple() error = %T %v, want signal marker instead of cancellation marker", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != -1 {
		t.Fatalf("Staple() error = %T %v, want preserved signaled staple command cause", err, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !staplerExitWasSignaled(exitErr) {
		t.Fatalf("Staple() error = %T %v, want underlying signaled child cause", err, err)
	}
}

func TestValidatePreservesSignaledChildWhenContextCancelsAfterWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt is not implemented for child processes on Windows")
	}
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_SIGNAL", "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousWaitHook := afterStaplerCommandWaitFn
	afterStaplerCommandWaitFn = cancel
	t.Cleanup(func() { afterStaplerCommandWaitFn = previousWaitHook })

	result, err := ValidateStaple(ctx, target, nil)
	if result != nil {
		t.Fatalf("ValidateStaple() result = %#v, want nil after signaled child", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ValidateStaple() error = %v, want context cancellation", err)
	}
	if !isStaplerOperationAttemptedSignal(err) {
		t.Fatalf("ValidateStaple() error = %T %v, want attempted-signal marker", err, err)
	}
	if isStaplerOperationAttemptedCancellation(err) {
		t.Fatalf("ValidateStaple() error = %T %v, want signal marker instead of cancellation marker", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != -1 {
		t.Fatalf("ValidateStaple() error = %T %v, want preserved signaled validate command cause", err, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !staplerExitWasSignaled(exitErr) {
		t.Fatalf("ValidateStaple() error = %T %v, want underlying signaled child cause", err, err)
	}
}

func TestValidateClassifiesContextKilledChildAsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("child process cancellation is not implemented on Windows")
	}
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_WAIT", "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct {
		result *StaplerResult
		err    error
	}, 1)
	go func() {
		result, err := ValidateStaple(ctx, target, nil)
		done <- struct {
			result *StaplerResult
			err    error
		}{result: result, err: err}
	}()
	waitForStaplerCommand(t, logPath, "xcrun|stapler|validate|")
	cancel()

	select {
	case outcome := <-done:
		if outcome.result != nil {
			t.Fatalf("ValidateStaple() result = %#v, want nil after context cancellation", outcome.result)
		}
		if !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("ValidateStaple() error = %v, want context cancellation", outcome.err)
		}
		if !isStaplerOperationAttemptedCancellation(outcome.err) {
			t.Fatalf("ValidateStaple() error = %T %v, want attempted-cancellation marker", outcome.err, outcome.err)
		}
		if isStaplerOperationAttemptedSignal(outcome.err) {
			t.Fatalf("ValidateStaple() error = %T %v, want cancellation marker instead of signal marker", outcome.err, outcome.err)
		}
		var commandErr *StaplerCommandError
		if !errors.As(outcome.err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != -1 {
			t.Fatalf("ValidateStaple() error = %T %v, want preserved canceled validate command cause", outcome.err, outcome.err)
		}
		var exitErr *exec.ExitError
		if !errors.As(outcome.err, &exitErr) || !staplerExitWasSignaled(exitErr) {
			t.Fatalf("ValidateStaple() error = %T %v, want underlying context-killed child cause", outcome.err, outcome.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ValidateStaple() did not return after context cancellation")
	}
}

func TestStaplePreservesChildExitWhenContextCancelsAfterStartBeforeWait(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_EXIT_CODE", "65")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousStartHook := afterStaplerCommandStartFn
	afterStaplerCommandStartFn = func(cmd *exec.Cmd) {
		if len(cmd.Args) < 4 || cmd.Args[len(cmd.Args)-4] != "xcrun" ||
			cmd.Args[len(cmd.Args)-3] != "stapler" || cmd.Args[len(cmd.Args)-2] != "staple" ||
			cmd.Args[len(cmd.Args)-1] != target {
			return
		}
		// CommandContext normally kills the process as soon as cancellation is
		// observed. Keep this child alive long enough to return its concrete
		// status so the post-Start/pre-Wait race is deterministic.
		cmd.Cancel = func() error { return nil }
		cancel()
	}
	t.Cleanup(func() { afterStaplerCommandStartFn = previousStartHook })

	result, err := Staple(ctx, target, nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after canceled failed child", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want preserved child command error", err, err)
	}
	if commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != 65 {
		t.Fatalf("Staple() command error = %#v, want staple/65", commandErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 65 {
		t.Fatalf("Staple() error = %T %v, want underlying exit 65", err, err)
	}
}

func TestValidatePreservesChildExitWhenContextCancelSucceedsBeforeWait(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_EXIT_CODE", "65")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelCalled := make(chan struct{}, 1)
	previousCancelHook := beforeStaplerCommandCancelFn
	beforeStaplerCommandCancelFn = func(cmd *exec.Cmd) {
		if len(cmd.Args) < 4 || cmd.Args[len(cmd.Args)-4] != "xcrun" ||
			cmd.Args[len(cmd.Args)-3] != "stapler" || cmd.Args[len(cmd.Args)-2] != "validate" ||
			cmd.Args[len(cmd.Args)-1] != target {
			return
		}
		// Keep the child alive after cancellation so Wait observes its concrete
		// exit status even though CommandContext's cancellation callback succeeds.
		cmd.Cancel = func() error {
			select {
			case cancelCalled <- struct{}{}:
			default:
			}
			return nil
		}
	}
	t.Cleanup(func() { beforeStaplerCommandCancelFn = previousCancelHook })
	previousStartHook := afterStaplerCommandStartFn
	afterStaplerCommandStartFn = func(cmd *exec.Cmd) {
		if len(cmd.Args) < 4 || cmd.Args[len(cmd.Args)-4] != "xcrun" ||
			cmd.Args[len(cmd.Args)-3] != "stapler" || cmd.Args[len(cmd.Args)-2] != "validate" ||
			cmd.Args[len(cmd.Args)-1] != target {
			return
		}
		cancel()
		select {
		case <-cancelCalled:
		case <-time.After(2 * time.Second):
			t.Fatal("context cancellation callback was not invoked")
		}
	}
	t.Cleanup(func() { afterStaplerCommandStartFn = previousStartHook })

	result, err := ValidateStaple(ctx, target, nil)
	if result != nil {
		t.Fatalf("ValidateStaple() result = %#v, want nil after canceled failed child", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ValidateStaple() error = %v, want context cancellation", err)
	}
	if isStaplerOperationAttemptedCancellation(err) {
		t.Fatalf("ValidateStaple() error = %T %v, concrete child status must not be classified as cancellation", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != 65 {
		t.Fatalf("ValidateStaple() error = %T %v, want validate/65 command error", err, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 65 {
		t.Fatalf("ValidateStaple() error = %T %v, want underlying exit 65", err, err)
	}
}

func TestStapleReturnsValidationFailureAfterMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_EXIT_CODE", "65")
	t.Setenv("ASC_STAPLER_VALIDATE_STDERR", "ticket mismatch\n")

	var diagnostics bytes.Buffer
	result, err := Staple(context.Background(), target, &diagnostics)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil when follow-up validation fails", result)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want StaplerCommandError", err, err)
	}
	if commandErr.Operation != string(StaplerOperationValidate) || commandErr.ExitCode != 65 {
		t.Fatalf("Staple() command error = %#v, want validate/65", commandErr)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationValidate {
		t.Fatalf("Staple() error = %#v, want post-staple validation marker", err)
	}
	if !strings.Contains(diagnostics.String(), "ticket mismatch") {
		t.Fatalf("diagnostics = %q, want validation diagnostic", diagnostics.String())
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
		"xcrun|stapler|validate|" + target,
	})
}

func TestStaplerRejectsUnsupportedPlatformBeforeToolLookup(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = previousOS })
	previousLookPath := lookPathFn
	lookPathFn = func(string) (string, error) {
		t.Fatal("lookPathFn called on unsupported platform")
		return "", nil
	}
	t.Cleanup(func() { lookPathFn = previousLookPath })

	result, err := Staple(context.Background(), "/tmp/MyApp.dmg", nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil", result)
	}
	if err == nil || !strings.Contains(err.Error(), "macOS only") {
		t.Fatalf("Staple() error = %v, want macOS-only failure", err)
	}
}

func TestStaplerReportsMissingXcrunWithoutStartingChild(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousOS })
	previousTrustedXcrunPath := trustedXcrunPathFn
	trustedXcrunPathFn = func() (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { trustedXcrunPathFn = previousTrustedXcrunPath })
	previousCommandContext := commandContextFn
	commandContextFn = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("commandContextFn called when xcrun is missing")
		return nil
	}
	t.Cleanup(func() { commandContextFn = previousCommandContext })

	_, err := Staple(context.Background(), "/tmp/MyApp.dmg", nil)
	if err == nil || !strings.Contains(err.Error(), "xcrun not available") {
		t.Fatalf("Staple() error = %v, want missing-xcrun failure", err)
	}
}

func TestStaplerRedactsNonNotFoundXcrunLookupFailure(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousOS })
	const canary = "STAPLER_LOOKUP_PATH_CANARY_2242"
	wantErr := errors.New("lookpath /private/tmp/" + canary + "/xcrun: permission denied")
	previousTrustedXcrunPath := trustedXcrunPathFn
	trustedXcrunPathFn = func() (string, error) { return "", wantErr }
	t.Cleanup(func() { trustedXcrunPathFn = previousTrustedXcrunPath })

	result, err := Staple(context.Background(), "/tmp/MyApp.dmg", nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil", result)
	}
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("Staple() error = %v, want lookup cause", err)
	}
	var resolutionErr *StaplerResolutionError
	if !errors.As(err, &resolutionErr) {
		t.Fatalf("Staple() error = %T %v, want StaplerResolutionError", err, err)
	}
	if strings.Contains(err.Error(), canary) || strings.Contains(err.Error(), "/private/tmp/") {
		t.Fatalf("Staple() error = %q, must redact lookup path", err.Error())
	}
}

func TestStaplerPreservesResolutionExitStatus(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_FIND_EXIT_CODE", "64")

	_, err := Staple(context.Background(), target, nil)
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want StaplerCommandError", err, err)
	}
	if commandErr.Operation != string(StaplerOperationResolve) || commandErr.ExitCode != 64 {
		t.Fatalf("Staple() command error = %#v, want resolve/64", commandErr)
	}
}

func TestStaplerDoesNotStreamResolverDiagnosticsToLogWriter(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_FIND_EXIT_CODE", "64")
	const canary = "RESOLVER_DIAGNOSTIC_PATH_CANARY_2242"
	t.Setenv("ASC_STAPLER_FIND_STDERR", "/private/tmp/"+canary+"/developer-dir\n")

	var diagnostics bytes.Buffer
	_, err := Staple(context.Background(), target, &diagnostics)
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want resolver command error", err, err)
	}
	if strings.Contains(diagnostics.String(), canary) {
		t.Fatalf("resolver diagnostics = %q, must not stream raw resolver output", diagnostics.String())
	}
}

func TestStaplerPreservesResolutionExitStatusWhenContextCancelsAfterLookup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_FIND_EXIT_CODE", "64")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousHook := afterStaplerResolutionFn
	afterStaplerResolutionFn = cancel
	t.Cleanup(func() { afterStaplerResolutionFn = previousHook })

	_, err := Staple(ctx, target, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want preserved lookup command error", err, err)
	}
	if commandErr.Operation != string(StaplerOperationResolve) || commandErr.ExitCode != 64 {
		t.Fatalf("Staple() command error = %#v, want resolve/64", commandErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 64 {
		t.Fatalf("Staple() error = %T %v, want underlying lookup exit 64", err, err)
	}
}

func TestStaplerPreservesResolutionExitStatusWhenCancellationCleanupWinsRace(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousOS })
	previousTrustedXcrunPath := trustedXcrunPathFn
	trustedXcrunPathFn = func() (string, error) { return "/usr/bin/xcrun", nil }
	t.Cleanup(func() { trustedXcrunPathFn = previousTrustedXcrunPath })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousCommandContext := commandContextFn
	commandContextFn = func(commandCtx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(commandCtx, "/bin/sh", "-c", "sleep 0.1; exit 64")
	}
	t.Cleanup(func() { commandContextFn = previousCommandContext })
	previousHook := beforeStaplerResolutionRunFn
	beforeStaplerResolutionRunFn = func(cmd *exec.Cmd) {
		// Simulate CommandContext's cancellation callback reporting success for
		// a process that has already exited, while Wait still returns its status.
		cmd.Cancel = func() error { return nil }
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
	}
	t.Cleanup(func() { beforeStaplerResolutionRunFn = previousHook })

	_, err := Staple(ctx, "/tmp/MyApp.dmg", nil)
	if err == nil {
		t.Fatal("Staple() error = nil, want resolver failure")
	}
	if IsStaplerOperationAttemptedCancellation(err) {
		t.Fatalf("Staple() error = %v, concrete resolver status must not be cancellation-marked", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationResolve) || commandErr.ExitCode != 64 {
		t.Fatalf("Staple() error = %T %v, want resolve/64", err, err)
	}
}

func TestStaplerClassifiesContextKilledResolverAsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("child process cancellation is not implemented on Windows")
	}
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	readyPath := filepath.Join(t.TempDir(), "resolver-ready")
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_FIND_WAIT", "1")
	t.Setenv("ASC_STAPLER_FIND_READY_PATH", readyPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		result *StaplerResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := Staple(ctx, target, nil)
		done <- outcome{result: result, err: err}
	}()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("resolver helper did not report readiness")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case got := <-done:
		if got.result != nil {
			t.Fatalf("Staple() result = %#v, want nil after resolver cancellation", got.result)
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("Staple() error = %v, want context cancellation", got.err)
		}
		if !isStaplerOperationAttemptedCancellation(got.err) {
			t.Fatalf("Staple() error = %T %v, want attempted-cancellation marker", got.err, got.err)
		}
		if isStaplerOperationAttemptedSignal(got.err) {
			t.Fatalf("Staple() error = %T %v, want cancellation marker instead of signal marker", got.err, got.err)
		}
		var commandErr *StaplerCommandError
		if !errors.As(got.err, &commandErr) || commandErr.Operation != string(StaplerOperationResolve) || commandErr.ExitCode != -1 {
			t.Fatalf("Staple() error = %T %v, want preserved canceled resolver command cause", got.err, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Staple() did not return after resolver cancellation")
	}
}

func TestStaplerPreservesSignaledResolutionWhenContextCancelsAfterLookup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt is not implemented for child processes on Windows")
	}
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_FIND_SIGNAL", "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousHook := afterStaplerResolutionFn
	afterStaplerResolutionFn = cancel
	t.Cleanup(func() { afterStaplerResolutionFn = previousHook })

	_, err := Staple(ctx, target, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want preserved lookup command error", err, err)
	}
	if commandErr.Operation != string(StaplerOperationResolve) || commandErr.ExitCode != -1 {
		t.Fatalf("Staple() command error = %#v, want signaled resolve/-1", commandErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !staplerExitWasSignaled(exitErr) {
		t.Fatalf("Staple() error = %T %v, want underlying signaled lookup exit", err, err)
	}
}

func TestStaplerPreservesResolutionStartFailureWhenContextCancelsAfterLookup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	missingTool := filepath.Join(t.TempDir(), "missing-stapler-resolver")
	commandContextFn = func(commandCtx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(commandCtx, missingTool)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousHook := afterStaplerResolutionFn
	afterStaplerResolutionFn = cancel
	t.Cleanup(func() { afterStaplerResolutionFn = previousHook })

	_, err := Staple(ctx, target, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want context cancellation", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Staple() error = %T %v, want preserved resolver command error", err, err)
	}
	if commandErr.Operation != string(StaplerOperationResolve) || commandErr.ExitCode != -1 {
		t.Fatalf("Staple() command error = %#v, want resolve/-1", commandErr)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Staple() error = %T %v, want preserved resolver launch cause", err, err)
	}
}

func TestStaplerPropagatesCancellationWithoutSuccess(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_WAIT", "1")

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	result, err := Staple(ctx, target, nil)
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after cancellation", result)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Staple() error = %v, want context deadline", err)
	}
}

func configureStaplerTestEnvironment(t *testing.T, logPath string) {
	t.Helper()
	previousOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousOS })
	previousLookPath := lookPathFn
	lookPathFn = func(file string) (string, error) {
		if file != "xcrun" && file != "stapler" {
			return "", fmt.Errorf("unexpected lookup %q", file)
		}
		return "/usr/bin/" + file, nil
	}
	t.Cleanup(func() { lookPathFn = previousLookPath })
	previousTrustedXcrunPath := trustedXcrunPathFn
	trustedXcrunPathFn = func() (string, error) {
		return lookPathFn("xcrun")
	}
	t.Cleanup(func() { trustedXcrunPathFn = previousTrustedXcrunPath })
	previousTrustedXcodeToolPath := trustedXcodeToolPathFn
	trustedXcodeToolPathFn = func(_ context.Context, tool string, _ []string) (string, error) {
		return lookPathFn(tool)
	}
	t.Cleanup(func() { trustedXcodeToolPathFn = previousTrustedXcodeToolPath })
	previousCommandContext := commandContextFn
	commandContextFn = staplerHelperCommandContext(t, logPath)
	t.Cleanup(func() { commandContextFn = previousCommandContext })
	t.Setenv("GO_WANT_STAPLER_HELPER", "1")
}

func staplerHelperCommandContext(t *testing.T, logPath string) func(context.Context, string, ...string) *exec.Cmd {
	t.Helper()
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		commandArgs := []string{"-test.run=TestStaplerHelperProcess", "--", filepath.Base(name)}
		commandArgs = append(commandArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_STAPLER_HELPER=1", "ASC_STAPLER_HELPER_LOG="+logPath)
		return cmd
	}
}

func TestStaplerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_STAPLER_HELPER") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		fmt.Fprintln(os.Stderr, "missing helper arguments")
		os.Exit(2)
	}
	args := os.Args[separator+1:]
	if err := appendStaplerHelperLog(os.Getenv("ASC_STAPLER_HELPER_LOG"), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	commandName := filepath.Base(args[0])

	if len(args) == 3 && commandName == "xcrun" && args[1] == "--find" && args[2] == "stapler" {
		if target := os.Getenv("ASC_STAPLER_SWAP_ON_FIND"); target != "" {
			if err := os.Rename(target, target+".original"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := os.WriteFile(target, []byte("replacement"), 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
		if target := os.Getenv("ASC_STAPLER_SWAP_AND_RESTORE_ON_FIND"); target != "" {
			if err := os.Rename(target, target+".original"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := os.WriteFile(target, []byte("replacement"), 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := os.Rename(target, target+".lookup-replacement"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := os.Rename(target+".original", target); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := os.Remove(target + ".lookup-replacement"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
		if os.Getenv("ASC_STAPLER_FIND_SIGNAL") == "1" {
			process, err := os.FindProcess(os.Getpid())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := process.Signal(os.Interrupt); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			time.Sleep(100 * time.Millisecond)
			os.Exit(125)
		}
		if os.Getenv("ASC_STAPLER_FIND_WAIT") == "1" {
			if readyPath := os.Getenv("ASC_STAPLER_FIND_READY_PATH"); readyPath != "" {
				if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
			}
			for {
				time.Sleep(time.Second)
			}
		}
		if code := staplerHelperExitCode("ASC_STAPLER_FIND_EXIT_CODE"); code >= 0 {
			if output := os.Getenv("ASC_STAPLER_FIND_STDERR"); output != "" {
				fmt.Fprint(os.Stderr, output)
			} else {
				fmt.Fprintln(os.Stderr, "stapler lookup failed")
			}
			os.Exit(code)
		}
		fmt.Fprintln(os.Stdout, "/usr/bin/stapler")
		os.Exit(0)
	}
	if len(args) >= 4 && commandName == "xcrun" && args[1] == "stapler" {
		operation := strings.ToUpper(args[2][:1]) + args[2][1:]
		if args[2] == "validate" {
			if observationPath := os.Getenv("ASC_STAPLER_VALIDATE_OBSERVED_PATH"); observationPath != "" {
				observed, err := os.ReadFile(args[3])
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
				if err := os.WriteFile(observationPath, observed, 0o600); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
			}
			if replacementTarget := os.Getenv("ASC_STAPLER_REPLACE_AFTER_VALIDATE"); replacementTarget != "" {
				if err := os.Rename(replacementTarget, replacementTarget+".original"); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
				if err := os.WriteFile(replacementTarget, []byte("replacement"), 0o600); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
			}
			if os.Getenv("ASC_STAPLER_VALIDATE_WAIT") == "1" {
				if readyPath := os.Getenv("ASC_STAPLER_VALIDATE_READY_PATH"); readyPath != "" {
					if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(2)
					}
				}
				for {
					time.Sleep(time.Second)
				}
			}
		}
		if os.Getenv("ASC_STAPLER_"+strings.ToUpper(args[2])+"_SIGNAL") == "1" {
			process, err := os.FindProcess(os.Getpid())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := process.Signal(os.Interrupt); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			// Keep the helper bounded if a platform ignores Interrupt. The
			// test is skipped on platforms where child interruption is absent.
			time.Sleep(100 * time.Millisecond)
			os.Exit(125)
		}
		if output := os.Getenv("ASC_STAPLER_" + strings.ToUpper(args[2]) + "_STDOUT"); output != "" {
			fmt.Fprint(os.Stdout, output)
		}
		if output := os.Getenv("ASC_STAPLER_" + strings.ToUpper(args[2]) + "_STDERR"); output != "" {
			fmt.Fprint(os.Stderr, output)
		}
		if os.Getenv("ASC_STAPLER_WAIT") == "1" {
			for {
				time.Sleep(time.Second)
			}
		}
		if code := staplerHelperExitCode("ASC_STAPLER_" + strings.ToUpper(args[2]) + "_EXIT_CODE"); code >= 0 {
			fmt.Fprintf(os.Stderr, "%s failed\n", operation)
			os.Exit(code)
		}
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "unexpected helper invocation: %v\n", args)
	os.Exit(2)
}

func staplerHelperExitCode(name string) int {
	value := os.Getenv(name)
	if value == "" {
		return -1
	}
	code, err := strconv.Atoi(value)
	if err != nil {
		return 2
	}
	return code
}

func appendStaplerHelperLog(path string, args []string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	loggedArgs := append([]string(nil), args...)
	if len(loggedArgs) > 0 {
		loggedArgs[0] = filepath.Base(loggedArgs[0])
	}
	_, err = fmt.Fprintln(file, strings.Join(loggedArgs, "|"))
	return err
}

type ioDiscardForStaplerTest struct{}

func (ioDiscardForStaplerTest) Write(p []byte) (int, error) { return len(p), nil }

type staplerDiagnosticFailureWriter struct {
	err error
}

func (writer staplerDiagnosticFailureWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func TestStapleDiagnosticWriterFailurePreservesPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_STDOUT", "staple diagnostic\n")
	writerErr := errors.New("diagnostic sink unavailable")

	result, err := Staple(context.Background(), target, staplerDiagnosticFailureWriter{err: writerErr})
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after diagnostic writer failure", result)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
		t.Fatalf("Staple() error = %T %v, want staple partial-mutation marker", err, err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationStaple) {
		t.Fatalf("Staple() error = %T %v, want staple command cause", err, err)
	}
	if !errors.Is(err, writerErr) {
		t.Fatalf("Staple() error = %v, want diagnostic writer cause", err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleDiagnosticWriterFailurePreservesLateCancellationCause(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_STDOUT", "staple diagnostic\n")
	writerErr := errors.New("diagnostic sink unavailable")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousHook := afterStaplerCommandWaitFn
	afterStaplerCommandWaitFn = cancel
	t.Cleanup(func() { afterStaplerCommandWaitFn = previousHook })

	result, err := Staple(ctx, target, staplerDiagnosticFailureWriter{err: writerErr})
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after diagnostic writer failure", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Staple() error = %v, want late cancellation cause", err)
	}
	if !errors.Is(err, writerErr) {
		t.Fatalf("Staple() error = %v, want diagnostic writer cause preserved with cancellation", err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || !partialErr.Interrupted {
		t.Fatalf("Staple() error = %T %v, want interrupted partial-mutation marker", err, err)
	}
}

func TestValidateStapleDiagnosticWriterFailurePreservesLateCancellationCause(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_VALIDATE_STDOUT", "validation diagnostic\n")
	writerErr := errors.New("diagnostic sink unavailable")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousHook := afterStaplerCommandWaitFn
	afterStaplerCommandWaitFn = cancel
	t.Cleanup(func() { afterStaplerCommandWaitFn = previousHook })

	result, err := ValidateStaple(ctx, target, staplerDiagnosticFailureWriter{err: writerErr})
	if result != nil {
		t.Fatalf("ValidateStaple() result = %#v, want nil after diagnostic writer failure", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ValidateStaple() error = %v, want late cancellation cause", err)
	}
	if !errors.Is(err, writerErr) {
		t.Fatalf("ValidateStaple() error = %v, want diagnostic writer cause preserved with cancellation", err)
	}
	var partialErr *StaplerPartialMutationError
	if errors.As(err, &partialErr) {
		t.Fatalf("ValidateStaple() error = %T %v, validation-only failure must not be partial mutation", err, err)
	}
}

func assertStaplerCommands(t *testing.T, logPath string, want []string) {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(got) != len(want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("command %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestStapleWithVerifierMarksCancellationDuringInitialStapleAsPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_WAIT", "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		result *StaplerResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := StapleWithVerifier(ctx, target, nil, nil)
		done <- outcome{result: result, err: err}
	}()
	waitForStaplerCommand(t, logPath, "xcrun|stapler|staple|")
	cancel()

	select {
	case got := <-done:
		if got.result != nil {
			t.Fatalf("StapleWithVerifier() result = %#v, want nil after cancellation", got.result)
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("StapleWithVerifier() error = %v, want context cancellation", got.err)
		}
		var partialErr *StaplerPartialMutationError
		if !errors.As(got.err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
			t.Fatalf("StapleWithVerifier() error = %T %v, want initial-staple partial marker", got.err, got.err)
		}
		if !partialErr.Interrupted || !strings.Contains(got.err.Error(), "staple was interrupted") {
			t.Fatalf("StapleWithVerifier() error = %v, want interrupted-staple classification", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StapleWithVerifier() did not return after cancellation")
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierPreservesInterruptedWhenPostStapleVerifierAlsoFails(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_WAIT", "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	verifierErr := errors.New("target changed after interrupted staple")
	type outcome struct {
		result *StaplerResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := StapleWithVerifier(ctx, target, nil, func(operation StaplerOperation, before bool) error {
			if operation == StaplerOperationStaple && !before {
				return verifierErr
			}
			return nil
		})
		done <- outcome{result: result, err: err}
	}()
	waitForStaplerCommand(t, logPath, "xcrun|stapler|staple|")
	cancel()

	select {
	case got := <-done:
		if got.result != nil {
			t.Fatalf("StapleWithVerifier() result = %#v, want nil after cancellation", got.result)
		}
		if !errors.Is(got.err, context.Canceled) || !errors.Is(got.err, verifierErr) {
			t.Fatalf("StapleWithVerifier() error = %v, want cancellation and verifier causes", got.err)
		}
		var partialErr *StaplerPartialMutationError
		if !errors.As(got.err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
			t.Fatalf("StapleWithVerifier() error = %T %v, want initial-staple partial marker", got.err, got.err)
		}
		if !partialErr.Interrupted || !strings.Contains(got.err.Error(), "staple was interrupted") {
			t.Fatalf("StapleWithVerifier() error = %v, want interrupted-staple classification", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StapleWithVerifier() did not return after cancellation")
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierMarksDeadlineDuringInitialStapleAsPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_WAIT", "1")

	ctx := newStaplerDeadlineTestContext()
	t.Cleanup(ctx.close)
	type outcome struct {
		result *StaplerResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := StapleWithVerifier(ctx, target, nil, nil)
		done <- outcome{result: result, err: err}
	}()
	waitForStaplerCommand(t, logPath, "xcrun|stapler|staple|")
	ctx.close()

	select {
	case got := <-done:
		if got.result != nil {
			t.Fatalf("StapleWithVerifier() result = %#v, want nil after deadline", got.result)
		}
		if !errors.Is(got.err, context.DeadlineExceeded) {
			t.Fatalf("StapleWithVerifier() error = %v, want context deadline", got.err)
		}
		var partialErr *StaplerPartialMutationError
		if !errors.As(got.err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
			t.Fatalf("StapleWithVerifier() error = %T %v, want initial-staple partial marker", got.err, got.err)
		}
		if !partialErr.Interrupted || !strings.Contains(got.err.Error(), "staple was interrupted") {
			t.Fatalf("StapleWithVerifier() error = %v, want interrupted-staple classification", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StapleWithVerifier() did not return after deadline")
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func TestStapleWithVerifierMarksSignalDuringInitialStapleAsPartialMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt is not implemented for child processes on Windows")
	}
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_SIGNAL", "1")

	result, err := StapleWithVerifier(context.Background(), target, nil, nil)
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil after signal termination", result)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationStaple {
		t.Fatalf("StapleWithVerifier() error = %T %v, want initial-staple partial marker", err, err)
	}
	if !partialErr.Interrupted || !strings.Contains(err.Error(), "staple was interrupted") {
		t.Fatalf("StapleWithVerifier() error = %v, want interrupted-staple classification", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationStaple) || commandErr.ExitCode != -1 {
		t.Fatalf("StapleWithVerifier() error = %T %v, want signaled staple command cause", err, err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !staplerExitWasSignaled(exitErr) {
		t.Fatalf("StapleWithVerifier() error = %T %v, want signaled child cause", err, err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

func staplerExitWasSignaled(exitErr *exec.ExitError) bool {
	if exitErr == nil || exitErr.ProcessState == nil {
		return false
	}
	status, ok := exitErr.Sys().(interface{ Signaled() bool })
	return ok && status.Signaled()
}

func TestStapleWithVerifierCancellationBeforeInitialChildStartIsNotPartial(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	helperCommandContext := commandContextFn
	commandContextFn = func(commandCtx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "stapler" {
			cancel()
		}
		return helperCommandContext(commandCtx, name, args...)
	}

	result, err := StapleWithVerifier(ctx, target, nil, nil)
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil before child start", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StapleWithVerifier() error = %v, want context cancellation", err)
	}
	var partialErr *StaplerPartialMutationError
	if errors.As(err, &partialErr) {
		t.Fatalf("StapleWithVerifier() error = %T %v, must not mark a child that never started as partial", err, err)
	}
	assertStaplerCommands(t, logPath, []string{"xcrun|--find|stapler"})
}

func TestStapleWithVerifierCancellationBeforeInitialChildStartPreservesStageFailureWithoutPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	helperCommandContext := commandContextFn
	commandContextFn = func(commandCtx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "stapler" {
			cancel()
		}
		return helperCommandContext(commandCtx, name, args...)
	}
	stageCause := errors.New("target identity changed")
	result, err := StapleWithVerifier(ctx, target, nil, func(operation StaplerOperation, before bool) error {
		if operation == StaplerOperationStaple && !before {
			return stageCause
		}
		return nil
	})
	if result != nil {
		t.Fatalf("StapleWithVerifier() result = %#v, want nil before child start", result)
	}
	if !errors.Is(err, context.Canceled) || !errors.Is(err, stageCause) {
		t.Fatalf("StapleWithVerifier() error = %v, want joined cancellation and stage failure", err)
	}
	var stageErr *StaplerStageVerificationError
	if !errors.As(err, &stageErr) || stageErr.Operation != StaplerOperationStaple || stageErr.Before {
		t.Fatalf("StapleWithVerifier() error = %T %v, want post-staple stage failure", err, err)
	}
	var partialErr *StaplerPartialMutationError
	if errors.As(err, &partialErr) {
		t.Fatalf("StapleWithVerifier() error = %T %v, child that never started must not be partial", err, err)
	}
	assertStaplerCommands(t, logPath, []string{"xcrun|--find|stapler"})
}

func waitForStaplerCommand(t *testing.T, logPath, prefix string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(data), prefix) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("stapler helper did not report command %q", prefix)
}

type staplerDeadlineTestContext struct {
	done chan struct{}
}

type staplerCancelAfterContextChecks struct {
	cancelOn int
	checks   int
	done     chan struct{}
	closed   bool
}

func newStaplerCancelAfterContextChecks(cancelOn int) *staplerCancelAfterContextChecks {
	return &staplerCancelAfterContextChecks{cancelOn: cancelOn, done: make(chan struct{})}
}

func (ctx *staplerCancelAfterContextChecks) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (ctx *staplerCancelAfterContextChecks) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *staplerCancelAfterContextChecks) Err() error {
	ctx.checks++
	if ctx.checks >= ctx.cancelOn {
		if !ctx.closed {
			close(ctx.done)
			ctx.closed = true
		}
		return context.Canceled
	}
	return nil
}

func (ctx *staplerCancelAfterContextChecks) Value(any) any {
	return nil
}

func newStaplerDeadlineTestContext() *staplerDeadlineTestContext {
	return &staplerDeadlineTestContext{done: make(chan struct{})}
}

func (ctx *staplerDeadlineTestContext) Deadline() (time.Time, bool) {
	return time.Now().Add(time.Hour), true
}

func (ctx *staplerDeadlineTestContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *staplerDeadlineTestContext) Err() error {
	select {
	case <-ctx.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (ctx *staplerDeadlineTestContext) Value(any) any {
	return nil
}

func (ctx *staplerDeadlineTestContext) close() {
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
}

func TestStapleMarksDiagnosticCopyFailureAfterSuccessfulChild(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	t.Setenv("ASC_STAPLER_STAPLE_STDERR", "stapler progress\n")

	result, err := Staple(context.Background(), target, failingStaplerDiagnosticWriter{})
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after diagnostic copy failure", result)
	}
	if !errors.Is(err, ErrStaplerDiagnosticOutput) {
		t.Fatalf("Staple() error = %T %v, want diagnostic-output marker", err, err)
	}
	var partialErr *StaplerPartialMutationError
	if !errors.As(err, &partialErr) || partialErr.Operation != StaplerOperationStaple || partialErr.Interrupted {
		t.Fatalf("Staple() error = %#v, want non-interrupted staple partial-mutation marker", err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
	})
}

type failingStaplerDiagnosticWriter struct{}

func (failingStaplerDiagnosticWriter) Write([]byte) (int, error) {
	return 0, errors.New("diagnostic sink closed")
}

func TestStapleKeepsValidationDiagnosticCopyFailureOutOfPartialMutation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "MyApp.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "commands.log")
	configureStaplerTestEnvironment(t, logPath)
	// Only the validation child emits output, so the staple child completes
	// without touching the failing diagnostic writer.
	t.Setenv("ASC_STAPLER_VALIDATE_STDERR", "validation detail\n")

	result, err := Staple(context.Background(), target, failingStaplerDiagnosticWriter{})
	if result != nil {
		t.Fatalf("Staple() result = %#v, want nil after diagnostic copy failure", result)
	}
	if !errors.Is(err, ErrStaplerDiagnosticOutput) {
		t.Fatalf("Staple() error = %T %v, want diagnostic-output marker", err, err)
	}
	var partialErr *StaplerPartialMutationError
	if errors.As(err, &partialErr) {
		t.Fatalf("Staple() error = %#v, must not claim partial mutation when validation succeeded", err)
	}
	var commandErr *StaplerCommandError
	if !errors.As(err, &commandErr) || commandErr.Operation != string(StaplerOperationValidate) {
		t.Fatalf("Staple() error = %#v, want validate command error", err)
	}
	assertStaplerCommands(t, logPath, []string{
		"xcrun|--find|stapler",
		"xcrun|stapler|staple|" + target,
		"xcrun|stapler|validate|" + target,
	})
}
