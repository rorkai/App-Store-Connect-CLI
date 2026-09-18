package shots

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestShotsValidationDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		command func() interface {
			ParseAndRun(context.Context, []string) error
		}
		args           []string
		wantError      string
		wantStderr     string
		wantUsage      bool
		wantValidation bool
		wantCode       shared.DiagnosticCode
		wantParameter  string
	}{
		{
			name: "capture missing bundle id",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsCaptureCommand()
			},
			args:          []string{"--name", "home"},
			wantError:     "--bundle-id",
			wantStderr:    "Error: --bundle-id is required\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticRequiredInputMissing,
			wantParameter: "--bundle-id",
		},
		{
			name: "capture missing name",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsCaptureCommand()
			},
			args:          []string{"--bundle-id", "com.example.app"},
			wantError:     "--name",
			wantStderr:    "Error: --name is required\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticRequiredInputMissing,
			wantParameter: "--name",
		},
		{
			name: "capture name with path separators",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsCaptureCommand()
			},
			args:          []string{"--bundle-id", "com.example.app", "--name", "../home"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: --name must be a file name without path separators\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--name",
		},
		{
			name: "capture invalid provider",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsCaptureCommand()
			},
			args:          []string{"--bundle-id", "com.example.app", "--name", "home", "--provider", "invalid"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: --provider must be",
			wantUsage:     true,
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--provider",
		},
		{
			name: "frame missing input",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			wantError:     "--input",
			wantStderr:    "Error: --input is required when --config is not set\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticRequiredInputMissing,
			wantParameter: "--input",
		},
		{
			name: "frame input and config",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--config", "/tmp/frame.yaml"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: use either --input or --config, not both\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--config",
		},
		{
			name: "frame watch without config",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--watch"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: --watch requires --config\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--watch",
		},
		{
			name: "frame invalid device",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--device", "iphone-se"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: --device must be one of:",
			wantUsage:     true,
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--device",
		},
		{
			name: "frame canvas flags with config",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--config", "/tmp/frame.yaml", "--title", "Hello"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: --title, --subtitle, --bg-color, --title-color, --subtitle-color cannot be used with --config; set these in the YAML config instead\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--config",
		},
		{
			name: "frame bg-color with noncanvas device",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--bg-color", "#000000"},
			wantError:     flag.ErrHelp.Error(),
			wantStderr:    "Error: --bg-color only applies to canvas devices (e.g. --device mac)\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--bg-color",
		},
		{
			name: "frame name with path separators",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:           []string{"--input", "/tmp/raw.png", "--name", "../home"},
			wantError:      "screenshots frame: --name must be a file name without path separators",
			wantValidation: true,
			wantCode:       shared.DiagnosticInvalidInput,
			wantParameter:  "--name",
		},
		{
			name: "review approve missing selector",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsReviewApproveCommand()
			},
			wantError:  flag.ErrHelp.Error(),
			wantStderr: "Error: provide at least one selector: --all-ready, --key, --id, --locale, or --device\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticRequiredInputMissing,
		},
		{
			name: "frame unsupported watch flag",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--config", "/tmp/frame.yaml", "--watch", "--title", "Hello"},
			wantError:     "--title cannot be used with --watch; watch mode regenerates from the Koubou YAML config",
			wantStderr:    "Error: --title cannot be used with --watch; watch mode regenerates from the Koubou YAML config\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--title",
		},
		{
			name: "frame multiple unsupported watch flags",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:       []string{"--config", "/tmp/frame.yaml", "--watch", "--title", "Hello", "--subtitle", "World"},
			wantError:  "--subtitle, --title cannot be used with --watch; watch mode regenerates from the Koubou YAML config",
			wantStderr: "Error: --subtitle, --title cannot be used with --watch; watch mode regenerates from the Koubou YAML config\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name: "frame debounce without watch",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--watch-debounce", "1s"},
			wantError:     "--watch-debounce requires --watch",
			wantStderr:    "Error: --watch-debounce requires --watch\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--watch-debounce",
		},
		{
			name: "frame review directory without watch",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--watch-review-dir", "/tmp/review"},
			wantError:     "--watch-review-dir requires --watch",
			wantStderr:    "Error: --watch-review-dir requires --watch\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--watch-review-dir",
		},
		{
			name: "frame raw directory without watch",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--input", "/tmp/raw.png", "--watch-raw-dir", "/tmp/raw"},
			wantError:     "--watch-raw-dir requires --watch",
			wantStderr:    "Error: --watch-raw-dir requires --watch\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--watch-raw-dir",
		},
		{
			name: "frame raw directory without review directory",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--config", "/tmp/frame.yaml", "--watch", "--watch-raw-dir", "/tmp/raw"},
			wantError:     "--watch-raw-dir requires --watch-review-dir",
			wantStderr:    "Error: --watch-raw-dir requires --watch-review-dir\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--watch-raw-dir",
		},
		{
			name: "frame empty review directory",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--config", "/tmp/frame.yaml", "--watch", "--watch-review-dir", "   "},
			wantError:     "--watch-review-dir must not be empty",
			wantStderr:    "Error: --watch-review-dir must not be empty\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--watch-review-dir",
		},
		{
			name: "frame nonpositive debounce",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return ShotsFrameCommand()
			},
			args:          []string{"--config", "/tmp/frame.yaml", "--watch", "--watch-debounce", "0s"},
			wantError:     "--watch-debounce must be greater than 0",
			wantStderr:    "Error: --watch-debounce must be greater than 0\n",
			wantUsage:     true,
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--watch-debounce",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error
			stderr := captureShotsDiagnosticStderr(t, func() {
				err = test.command().ParseAndRun(context.Background(), test.args)
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if test.wantStderr != "" && !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if got := errors.Is(err, flag.ErrHelp); got != test.wantUsage {
				t.Fatalf("errors.Is(flag.ErrHelp) = %t, want %t", got, test.wantUsage)
			}
			if got := shared.IsValidationError(err); got != test.wantValidation {
				t.Fatalf("shared.IsValidationError() = %t, want %t", got, test.wantValidation)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParameter {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParameter)
			}
		})
	}
}

func captureShotsDiagnosticStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	readResult := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		readResult <- data
		readError <- readErr
	}()

	os.Stderr = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stderr = original
	data := <-readResult
	if err := <-readError; err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(data)
}
