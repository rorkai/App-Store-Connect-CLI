package testflight

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestFlightUsageErrorDiagnosticsPreserveContracts(t *testing.T) {
	outputDirectory := filepath.Join(t.TempDir(), "testers") + string(filepath.Separator)

	tests := []struct {
		name       string
		command    func() *ffcli.Command
		args       []string
		wantError  string
		wantStderr string
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name:       "deprecated external testing true",
			command:    TestFlightBetaDetailsUpdateCommand,
			args:       []string{"--external-testing=true"},
			wantError:  `--external-testing=true cannot select a beta group or safely infer review submission. Use asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID" --submit --confirm.`,
			wantStderr: "Warning: `--external-testing` is deprecated and cannot be applied safely; App Store Connect does not support editing `externalBuildState`.\nError: --external-testing=true cannot select a beta group or safely infer review submission. Use asc builds add-groups --build-id \"BUILD_ID\" --group \"GROUP_ID\" --submit --confirm.\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--external-testing",
		},
		{
			name:       "deprecated external testing false",
			command:    TestFlightBetaDetailsUpdateCommand,
			args:       []string{"--external-testing=false"},
			wantError:  `--external-testing=false cannot identify which beta groups to remove. Use asc builds remove-groups --build-id "BUILD_ID" --group "GROUP_ID" --confirm.`,
			wantStderr: "Warning: `--external-testing` is deprecated and cannot be applied safely; App Store Connect does not support editing `externalBuildState`.\nError: --external-testing=false cannot identify which beta groups to remove. Use asc builds remove-groups --build-id \"BUILD_ID\" --group \"GROUP_ID\" --confirm.\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--external-testing",
		},
		{
			name:       "legacy build alias conflict",
			command:    BetaTestersExportCommand,
			args:       []string{"--build-id", "build-1", "--build", "build-2"},
			wantError:  "--build conflicts with --build-id; use only --build-id",
			wantStderr: "Error: --build conflicts with --build-id; use only --build-id\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "export group and build conflict",
			command:    BetaTestersExportCommand,
			args:       []string{"--group", "group-1", "--build-id", "build-1"},
			wantError:  "--group cannot be combined with --build-id",
			wantStderr: "Error: --group cannot be combined with --build-id\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "export output must be a file",
			command:    BetaTestersExportCommand,
			args:       []string{"--app", "app-1", "--output", outputDirectory},
			wantError:  "--output must be a file path",
			wantStderr: "Error: --output must be a file path\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--output",
		},
		{
			name:       "list group and build conflict",
			command:    BetaTestersListCommand,
			args:       []string{"--group", "group-1", "--build-id", "build-1"},
			wantError:  "--group cannot be combined with --build-id",
			wantStderr: "Error: --group cannot be combined with --build-id\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stderr := captureTestFlightDiagnosticStderr(t, func() {
				runErr = cmd.Exec(context.Background(), nil)
			})
			if runErr == nil {
				t.Fatal("expected usage error")
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("errors.Is(flag.ErrHelp) = false, error = %v", runErr)
			}
			if runErr.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", runErr, test.wantError)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}

			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}

func TestBetaTestersCSVUsageErrorDiagnostics(t *testing.T) {
	emptyPath := filepath.Join(t.TempDir(), "empty.csv")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("write empty CSV: %v", err)
	}

	tests := []struct {
		name      string
		run       func() error
		wantError string
	}{
		{
			name: "empty file",
			run: func() error {
				_, err := readBetaTestersCSV(emptyPath)
				return err
			},
			wantError: "CSV file is empty",
		},
		{
			name: "empty header",
			run: func() error {
				_, err := validateBetaTestersCSVHeader(nil)
				return err
			},
			wantError: "CSV header row is required",
		},
		{
			name: "empty column name",
			run: func() error {
				_, err := validateBetaTestersCSVHeader([]string{"email", ""})
				return err
			},
			wantError: "CSV header contains an empty column name",
		},
		{
			name: "unknown column",
			run: func() error {
				_, err := validateBetaTestersCSVHeader([]string{"email", "mystery"})
				return err
			},
			wantError: "unknown CSV column \"mystery\" (allowed: email, first_name, last_name, groups, _asc_formula_escaping)",
		},
		{
			name: "duplicate column",
			run: func() error {
				_, err := validateBetaTestersCSVHeader([]string{"email", "Email"})
				return err
			},
			wantError: "duplicate CSV column \"email\"",
		},
		{
			name: "missing email column",
			run: func() error {
				_, err := validateBetaTestersCSVHeader([]string{"first_name"})
				return err
			},
			wantError: "CSV header must include required column \"email\"",
		},
		{
			name: "empty parsed header",
			run: func() error {
				_, _, err := parseBetaTestersCSVHeader(nil)
				return err
			},
			wantError: "CSV header row is required",
		},
		{
			name: "malformed legacy row",
			run: func() error {
				_, err := parseLegacyBetaTesterCSVRow([]string{"First", "Last"})
				return err
			},
			wantError: "legacy CSV rows must have 3 or 4 columns: first_name,last_name,email[,groups]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var runErr error
			stderr := captureTestFlightDiagnosticStderr(t, func() {
				runErr = test.run()
			})
			if runErr == nil {
				t.Fatal("expected usage error")
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("errors.Is(flag.ErrHelp) = false, error = %v", runErr)
			}
			if runErr.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", runErr, test.wantError)
			}
			if stderr != "Error: "+test.wantError+"\n" {
				t.Fatalf("stderr = %q, want %q", stderr, "Error: "+test.wantError+"\\n")
			}

			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != shared.DiagnosticFileInvalidFormat || diagnostic.Parameter != "--input" {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, shared.DiagnosticFileInvalidFormat, "--input")
			}
		})
	}
}

func TestBetaTestersCSVUsageDiagnosticDoesNotLeakInputPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-testers.csv")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write empty CSV: %v", err)
	}
	_, err := readBetaTestersCSV(path)
	if err == nil {
		t.Fatal("expected usage error")
	}
	if strings.Contains(err.Error(), path) {
		t.Fatalf("error leaked input path: %q", err)
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatal("expected structured diagnostic")
	}
	if diagnostic.Parameter != "--input" {
		t.Fatalf("diagnostic parameter = %q, want --input", diagnostic.Parameter)
	}
}
