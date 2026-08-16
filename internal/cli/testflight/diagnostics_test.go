package testflight

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestFlightValidationDiagnosticsPreserveContracts(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "testflight.yaml")

	tests := []struct {
		name       string
		command    func() *ffcli.Command
		args       []string
		wantStderr string
		wantError  string
		wantCode   shared.DiagnosticCode
		wantParam  string
		noOutput   bool
		setup      func(*testing.T)
	}{
		{
			name:       "beta groups internal and external conflict",
			command:    BetaGroupsListCommand,
			args:       []string{"--internal", "--external"},
			wantStderr: "Error: --internal and --external are mutually exclusive\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "beta groups empty build id",
			command:    BetaGroupsListCommand,
			args:       []string{"--build-id", "   "},
			wantStderr: "Error: --build-id cannot be empty\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--build-id",
		},
		{
			name:       "beta groups empty app with build id",
			command:    BetaGroupsListCommand,
			args:       []string{"--build-id", "build-1", "--app", "   "},
			wantStderr: "Error: --app cannot be empty when used with --build-id\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--app",
		},
		{
			name:       "beta groups membership controls with build id",
			command:    BetaGroupsListCommand,
			args:       []string{"--build-id", "build-1", "--limit", "10"},
			wantStderr: "Error: --global, --limit, --next, and --paginate cannot be used with --build-id; membership lookup always fetches all required pages\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "beta groups global and app conflict",
			command:    BetaGroupsListCommand,
			args:       []string{"--global", "--app", "app-1"},
			wantStderr: "Error: --global and --app are mutually exclusive\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "beta group public link limit range",
			command:    BetaGroupsUpdateCommand,
			args:       []string{"--id", "group-1", "--public-link-limit", "10001"},
			wantStderr: "Error: --public-link-limit must be between 1 and 10000\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--public-link-limit",
		},
		{
			name:       "beta group relationship enum",
			command:    BetaGroupsRelationshipsGetCommand,
			args:       []string{"--group-id", "group-1", "--type", "unknown"},
			wantStderr: "Error: --type must be one of: betaTesters, builds\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--type",
		},
		{
			name:       "beta group relationship single conflict",
			command:    BetaGroupsRelationshipsGetCommand,
			args:       []string{"--group-id", "group-1", "--type", "betaTesters", "--paginate"},
			wantStderr: "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantCode:   shared.DiagnosticConflictingInput,
			setup: func(t *testing.T) {
				previous := betaGroupRelationshipKinds["betaTesters"]
				betaGroupRelationshipKinds["betaTesters"] = relationshipSingle
				t.Cleanup(func() { betaGroupRelationshipKinds["betaTesters"] = previous })
			},
		},
		{
			name:       "beta license agreement id and app conflict",
			command:    BetaLicenseAgreementsGetCommand,
			args:       []string{"--id", "agreement-1", "--app", "app-1"},
			wantStderr: "Error: --id and --app are mutually exclusive\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "beta license agreement app-only fields conflict",
			command:    BetaLicenseAgreementsGetCommand,
			args:       []string{"--app", "app-1", "--include", "app"},
			wantStderr: "Error: --app-fields and --include are only valid with --id\n",
			wantCode:   shared.DiagnosticConflictingInput,
		},
		{
			name:       "beta tester metrics limit range",
			command:    BetaTestersMetricsCommand,
			args:       []string{"--limit", "201"},
			wantStderr: "Error: --limit must be between 1 and 200\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--limit",
		},
		{
			name:      "beta tester metrics period enum",
			command:   BetaTestersMetricsCommand,
			args:      []string{"--period", "P10D"},
			wantError: "--period must be one of: P7D, P30D, P90D, P365D",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--period",
			noOutput:  true,
		},
		{
			name:       "beta tester relationship enum",
			command:    BetaTestersRelationshipsGetCommand,
			args:       []string{"--tester-id", "tester-1", "--type", "unknown"},
			wantStderr: "Error: --type must be one of: apps, betaGroups, builds\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--type",
		},
		{
			name:       "beta tester relationship single conflict",
			command:    BetaTestersRelationshipsGetCommand,
			args:       []string{"--tester-id", "tester-1", "--type", "apps", "--paginate"},
			wantStderr: "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantCode:   shared.DiagnosticConflictingInput,
			setup: func(t *testing.T) {
				previous := betaTesterRelationshipKinds["apps"]
				betaTesterRelationshipKinds["apps"] = relationshipSingle
				t.Cleanup(func() { betaTesterRelationshipKinds["apps"] = previous })
			},
		},
		{
			name:       "app tester usage limit range",
			command:    TestFlightMetricsBetaTesterUsagesCommand,
			args:       []string{"--limit", "201"},
			wantStderr: "Error: --limit must be between 1 and 200\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--limit",
		},
		{
			name:       "app tester usage period enum",
			command:    TestFlightMetricsBetaTesterUsagesCommand,
			args:       []string{"--period", "P10D"},
			wantStderr: "Error: --period must be one of: P7D, P30D, P90D, P365D\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--period",
		},
		{
			name:       "sync build filter requires include builds",
			command:    TestFlightSyncPullCommand,
			args:       []string{"--app", "app-1", "--output", outputPath, "--build-id", "build-1"},
			wantStderr: "Error: --build-id requires --include-builds\n\n",
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "--build-id",
			noOutput:   true,
		},
		{
			name:       "sync tester filter requires include testers",
			command:    TestFlightSyncPullCommand,
			args:       []string{"--app", "app-1", "--output", outputPath, "--tester", "tester-1"},
			wantStderr: "Error: --tester requires --include-testers\n\n",
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "--tester",
			noOutput:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setup != nil {
				test.setup(t)
			}
			cmd := test.command()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stderr := captureTestFlightDiagnosticStderr(t, func() {
				runErr = cmd.Exec(context.Background(), nil)
			})
			if runErr == nil {
				t.Fatal("expected validation error")
			}
			if test.wantError != "" {
				if errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("errors.Is(flag.ErrHelp) = true, error = %v", runErr)
				}
				if runErr.Error() != test.wantError {
					t.Fatalf("error = %q, want %q", runErr, test.wantError)
				}
			} else {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("errors.Is(flag.ErrHelp) = false, error = %v", runErr)
				}
				if runErr.Error() != flag.ErrHelp.Error() {
					t.Fatalf("error = %q, want %q", runErr, flag.ErrHelp.Error())
				}
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
			if test.noOutput {
				if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("output path exists after validation: %v", err)
				}
			}
		})
	}
}

func captureTestFlightDiagnosticStderr(t *testing.T, fn func()) string {
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
