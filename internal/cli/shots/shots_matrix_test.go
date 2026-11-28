package shots

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestShotsMatrixRejectsInvalidOutputBeforeLoadingOrRunning(t *testing.T) {
	var loadCalls, runCalls int
	d, stdoutBuf, stderrBuf := newShotsTestDeps()
	command := shotsMatrixCommandWithDependencies(
		d,
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

	stdout, stderr, err := shotsMatrixOutput(stdoutBuf, stderrBuf, func() error {
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
	d, stdoutBuf, stderrBuf := newShotsTestDeps()
	command := shotsMatrixCommandWithDependencies(
		d,
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

	stdout, stderr, err := shotsMatrixOutput(stdoutBuf, stderrBuf, func() error {
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
	d, stdoutBuf, stderrBuf := newShotsTestDeps()
	command := shotsMatrixCommandWithDependencies(
		d,
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
	_, _, err := shotsMatrixOutput(stdoutBuf, stderrBuf, func() error {
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

func shotsMatrixOutput(stdout, stderr *bytes.Buffer, run func() error) (string, string, error) {
	err := run()
	return stdout.String(), stderr.String(), err
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
			d, stdoutBuf, stderrBuf := newShotsTestDeps()
			command := shotsMatrixCommandWithDependencies(
				d,
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

			_, stderr, err := shotsMatrixOutput(stdoutBuf, stderrBuf, func() error {
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
			d, stdoutBuf, stderrBuf := newShotsTestDeps()
			command := shotsMatrixCommandWithDependencies(
				d,
				shotsMatrixCommandDependencies{
					loadPlan: func(string) (*screenshots.MatrixPlan, error) { return &screenshots.MatrixPlan{}, nil },
					runMatrix: func(context.Context, string, *screenshots.MatrixPlan, screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
						runCalls++
						return &screenshots.MatrixResult{}, nil
					},
				},
			)
			args := append([]string{"--plan", filepath.Join(t.TempDir(), "matrix.json")}, tt.args...)
			_, stderr, err := shotsMatrixOutput(stdoutBuf, stderrBuf, func() error {
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

func TestShotsMatrixProgressFileRefusalDoesNotStartRun(t *testing.T) {
	for _, name := range []string{"existing", "missing parent", "empty"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "existing")
			if err := os.WriteFile(path, []byte("preserved"), 0o600); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "missing parent":
				path = filepath.Join(dir, "missing", "progress.jsonl")
			case "empty":
				path = ""
			}
			runCalls := 0
			d, stdout, stderr := newShotsTestDeps()
			command := shotsMatrixCommandWithDependencies(d, shotsMatrixCommandDependencies{
				loadPlan: func(string) (*screenshots.MatrixPlan, error) { return &screenshots.MatrixPlan{}, nil },
				runMatrix: func(_ context.Context, _ string, _ *screenshots.MatrixPlan, options screenshots.MatrixOptions) (*screenshots.MatrixResult, error) {
					if options.PrepareProgress != nil {
						if err := options.PrepareProgress(nil); err != nil {
							return nil, err
						}
					}
					runCalls++
					return nil, nil
				},
			})
			err := command.ParseAndRun(context.Background(), []string{"--progress-file", path})
			if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr.String(), "--progress-file") || stdout.Len() != 0 || runCalls != 0 {
				t.Fatalf("error=%v stdout=%q stderr=%q runs=%d", err, stdout.String(), stderr.String(), runCalls)
			}
			if data, err := os.ReadFile(filepath.Join(dir, "existing")); err != nil || string(data) != "preserved" {
				t.Fatalf("destination changed: %q, %v", data, err)
			}
		})
	}
}
