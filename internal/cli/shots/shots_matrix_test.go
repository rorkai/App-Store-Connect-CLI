package shots

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestShotsMatrixRejectsInvalidOutputBeforeLoadingOrRunning(t *testing.T) {
	var loadCalls, runCalls int
	command := shotsMatrixCommandWithDependencies(
		shotsMatrixCommandDependencies{
			loadPlan: func(string) (*screenshots.MatrixPlan, error) {
				loadCalls++
				return nil, errors.New("load guard called")
			},
			runMatrix: func(context.Context, string, *screenshots.MatrixPlan, screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
				runCalls++
				return nil, errors.New("run guard called")
			},
		},
	)

	stdout, stderr, err := captureShotsMatrixOutput(t, func() error {
		return command.ParseAndRun(context.Background(), []string{
			"--plan", filepath.Join(t.TempDir(), "matrix.json"),
			"--output", "yaml",
		})
	})
	if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--output must be one of") {
		t.Fatalf("error = %v, want invalid-output usage error", err)
	}
	if loadCalls != 0 || runCalls != 0 {
		t.Fatalf("guards called for invalid output: load=%d run=%d", loadCalls, runCalls)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--output must be one of") {
		t.Fatalf("stderr = %q, want invalid-output diagnostic", stderr)
	}
}

func TestShotsMatrixRejectsPrettyForNonJSONBeforeLoadingOrRunning(t *testing.T) {
	var loadCalls, runCalls int
	command := shotsMatrixCommandWithDependencies(
		shotsMatrixCommandDependencies{
			loadPlan: func(string) (*screenshots.MatrixPlan, error) {
				loadCalls++
				return nil, errors.New("load guard called")
			},
			runMatrix: func(context.Context, string, *screenshots.MatrixPlan, screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
				runCalls++
				return nil, errors.New("run guard called")
			},
		},
	)

	stdout, stderr, err := captureShotsMatrixOutput(t, func() error {
		return command.ParseAndRun(context.Background(), []string{
			"--plan", filepath.Join(t.TempDir(), "matrix.json"),
			"--output", "table",
			"--pretty",
		})
	})
	if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("error = %v, want invalid-pretty usage error", err)
	}
	if loadCalls != 0 || runCalls != 0 {
		t.Fatalf("guards called for invalid pretty: load=%d run=%d", loadCalls, runCalls)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
		t.Fatalf("stderr = %q, want invalid-pretty diagnostic", stderr)
	}
}

func TestShotsMatrixPreservesLiteralPlanPath(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "matrix plan ")
	var loadedPath, runPath string
	command := shotsMatrixCommandWithDependencies(
		shotsMatrixCommandDependencies{
			loadPlan: func(path string) (*screenshots.MatrixPlan, error) {
				loadedPath = path
				return &screenshots.MatrixPlan{}, nil
			},
			runMatrix: func(_ context.Context, path string, _ *screenshots.MatrixPlan, _ screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
				runPath = path
				return &screenshots.MatrixResult{Status: screenshots.MatrixCellSuccess}, nil
			},
		},
	)
	_, _, err := captureShotsMatrixOutput(t, func() error {
		return command.ParseAndRun(context.Background(), []string{"--plan", planPath, "--output", "json"})
	})
	if err != nil {
		t.Fatalf("ParseAndRun() error = %v", err)
	}
	want, err := filepath.Abs(planPath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if loadedPath != want || runPath != want {
		t.Fatalf("plan paths = loaded %q, run %q, want %q", loadedPath, runPath, want)
	}
}

func captureShotsMatrixOutput(t *testing.T, run func() error) (stdout, stderr string, runErr error) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("create stderr pipe: %v", err)
	}
	stdoutDone := make(chan string, 1)
	stderrDone := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(stdoutReader)
		stdoutDone <- string(data)
	}()
	go func() {
		data, _ := io.ReadAll(stderrReader)
		stderrDone <- string(data)
	}()

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	runErr = run()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	stdout = <-stdoutDone
	stderr = <-stderrDone
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	return stdout, stderr, runErr
}

func TestShotsMatrixHonorsDocumentedZeroOverrideSentinel(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{name: "max-concurrency", flag: "--max-concurrency"},
		{name: "max-attempts", flag: "--max-attempts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotOptions screenshots.MatrixOptions
			var runCalls int
			command := shotsMatrixCommandWithDependencies(
				shotsMatrixCommandDependencies{
					loadPlan: func(string) (*screenshots.MatrixPlan, error) {
						return &screenshots.MatrixPlan{}, nil
					},
					runMatrix: func(_ context.Context, _ string, _ *screenshots.MatrixPlan, options screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
						runCalls++
						gotOptions = options
						return &screenshots.MatrixResult{}, nil
					},
				},
			)

			_, stderr, err := captureShotsMatrixOutput(t, func() error {
				return command.ParseAndRun(context.Background(), []string{
					"--plan", filepath.Join(t.TempDir(), "matrix.json"),
					tt.flag, "0",
					"--output", "json",
				})
			})
			if err != nil {
				t.Fatalf("error = %v, want explicit zero accepted as the documented plan-value sentinel", err)
			}
			if runCalls != 1 {
				t.Fatalf("runCalls = %d, want the matrix to run", runCalls)
			}
			if strings.Contains(stderr, "must be between") {
				t.Fatalf("stderr = %q, want no range diagnostic for the documented sentinel", stderr)
			}
			if gotOptions.MaxConcurrencySet || gotOptions.MaxAttemptsSet {
				t.Fatalf("options = %+v, want explicit zero to defer to the plan value", gotOptions)
			}
		})
	}
}

func TestShotsMatrixStillRejectsNegativeAndOutOfRangeOverrides(t *testing.T) {
	tests := []struct {
		name string
		args []string
		diag string
	}{
		{name: "negative concurrency", args: []string{"--max-concurrency", "-1"}, diag: "--max-concurrency must be between 1 and 8"},
		{name: "concurrency above max", args: []string{"--max-concurrency", "9"}, diag: "--max-concurrency must be between 1 and 8"},
		{name: "negative attempts", args: []string{"--max-attempts", "-1"}, diag: "--max-attempts must be between 1 and 3"},
		{name: "attempts above max", args: []string{"--max-attempts", "4"}, diag: "--max-attempts must be between 1 and 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runCalls int
			command := shotsMatrixCommandWithDependencies(
				shotsMatrixCommandDependencies{
					loadPlan: func(string) (*screenshots.MatrixPlan, error) { return &screenshots.MatrixPlan{}, nil },
					runMatrix: func(context.Context, string, *screenshots.MatrixPlan, screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
						runCalls++
						return &screenshots.MatrixResult{}, nil
					},
				},
			)
			args := append([]string{"--plan", filepath.Join(t.TempDir(), "matrix.json")}, tt.args...)
			_, stderr, err := captureShotsMatrixOutput(t, func() error {
				return command.ParseAndRun(context.Background(), args)
			})
			if err == nil {
				t.Fatalf("error = nil, want out-of-range rejection")
			}
			if !strings.Contains(stderr, tt.diag) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.diag)
			}
			if runCalls != 0 {
				t.Fatalf("runCalls = %d, want no run for an invalid override", runCalls)
			}
		})
	}
}
