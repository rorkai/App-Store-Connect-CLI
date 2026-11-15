package testflight

import (
	"bytes"
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
		name    string
		command func() *ffcli.Command
		// depsCommand builds a command on shared.Deps; its stderr is read
		// from the deps and the process stderr must stay empty.
		depsCommand func(*shared.Deps) *ffcli.Command
		args        []string
		wantStderr  string
		wantError   string
		wantCode    shared.DiagnosticCode
		wantParam   string
		// usageError marks a pre-request flag check that reports its own
		// diagnostic and exits with usage semantics: the error renders the
		// message and still wraps flag.ErrHelp.
		usageError bool
		noOutput   bool
		setup      func(*testing.T)
	}{
		{
			name:        "beta groups internal and external conflict",
			depsCommand: newBetaGroupsListCommand,
			args:        []string{"--app", "app-1", "--internal", "--external"},
			wantStderr:  "Error: --internal and --external are mutually exclusive\n",
			wantCode:    shared.DiagnosticConflictingInput,
		},
		{
			name:        "beta groups empty build id",
			depsCommand: newBetaGroupsListCommand,
			args:        []string{"--build-id", "   "},
			wantStderr:  "Error: --build-id cannot be empty\n",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--build-id",
		},
		{
			name:        "beta groups empty app with build id",
			depsCommand: newBetaGroupsListCommand,
			args:        []string{"--build-id", "build-1", "--app", "   "},
			wantStderr:  "Error: --app cannot be empty when used with --build-id\n",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--app",
		},
		{
			name:        "beta groups membership controls with build id",
			depsCommand: newBetaGroupsListCommand,
			args:        []string{"--build-id", "build-1", "--limit", "10"},
			wantStderr:  "Error: --global, --limit, --next, and --paginate cannot be used with --build-id; membership lookup always fetches all required pages\n",
			wantCode:    shared.DiagnosticConflictingInput,
		},
		{
			name:        "beta groups query surface with build id",
			depsCommand: newBetaGroupsListCommand,
			args:        []string{"--build-id", "build-1", "--fields", "name"},
			wantStderr:  "Error: testflight groups query filters, sparse fields, includes, and relationship limits cannot be used with --build-id; membership lookup queries the build's app relationships directly\n",
			wantCode:    shared.DiagnosticConflictingInput,
		},
		{
			name:        "beta groups global and app conflict",
			depsCommand: newBetaGroupsListCommand,
			args:        []string{"--global", "--app", "app-1"},
			wantStderr:  "Error: --global and --app are mutually exclusive\n",
			wantCode:    shared.DiagnosticConflictingInput,
		},
		{
			name:        "beta group public link limit range",
			depsCommand: newBetaGroupsUpdateCommand,
			args:        []string{"--id", "group-1", "--public-link-limit", "10001"},
			wantStderr:  "Error: --public-link-limit must be between 1 and 10000\n",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--public-link-limit",
		},
		{
			name:        "beta group relationship type missing",
			depsCommand: newBetaGroupsRelationshipsGetCommand,
			args:        []string{"--group-id", "group-1"},
			wantStderr:  "Error: --type is required; must be one of: betaTesters, builds\n",
			wantError:   "--type",
			wantCode:    shared.DiagnosticRequiredInputMissing,
			wantParam:   "--type",
			usageError:  true,
		},
		{
			name:        "beta group relationship enum",
			depsCommand: newBetaGroupsRelationshipsGetCommand,
			args:        []string{"--group-id", "group-1", "--type", "unknown"},
			wantStderr:  `Error: --type "unknown" is not a valid relationship type; must be one of: betaTesters, builds` + "\n",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--type",
		},
		{
			name:        "beta group relationship single conflict",
			depsCommand: newBetaGroupsRelationshipsGetCommand,
			args:        []string{"--group-id", "group-1", "--type", "betaTesters", "--paginate"},
			wantStderr:  "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantCode:    shared.DiagnosticConflictingInput,
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
			name:        "beta tester metrics limit range",
			depsCommand: newBetaTestersMetricsCommand,
			args:        []string{"--limit", "201"},
			wantStderr:  "Error: testflight testers metrics: --limit must be between 1 and 200\n",
			wantError:   "testflight testers metrics: --limit must be between 1 and 200",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--limit",
			usageError:  true,
		},
		{
			name:        "beta tester metrics period enum",
			depsCommand: newBetaTestersMetricsCommand,
			args:        []string{"--tester-id", "tester-1", "--app", "app-1", "--period", "P10D"},
			wantStderr:  "Error: --period must be one of: P7D, P30D, P90D, P365D\n",
			wantError:   "--period must be one of: P7D, P30D, P90D, P365D",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--period",
			usageError:  true,
			noOutput:    true,
		},
		{
			name:        "beta tester relationship type missing",
			depsCommand: newBetaTestersRelationshipsGetCommand,
			args:        []string{"--tester-id", "tester-1"},
			wantStderr:  "Error: --type is required; must be one of: apps, betaGroups, builds\n",
			wantError:   "--type",
			wantCode:    shared.DiagnosticRequiredInputMissing,
			wantParam:   "--type",
			usageError:  true,
		},
		{
			name:        "beta tester relationship enum",
			depsCommand: newBetaTestersRelationshipsGetCommand,
			args:        []string{"--tester-id", "tester-1", "--type", "unknown"},
			wantStderr:  `Error: --type "unknown" is not a valid relationship type; must be one of: apps, betaGroups, builds` + "\n",
			wantCode:    shared.DiagnosticInvalidInput,
			wantParam:   "--type",
		},
		{
			name:        "beta tester relationship single conflict",
			depsCommand: newBetaTestersRelationshipsGetCommand,
			args:        []string{"--tester-id", "tester-1", "--type", "apps", "--paginate"},
			wantStderr:  "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantCode:    shared.DiagnosticConflictingInput,
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
			wantStderr: "Error: testflight metrics app-testers: --limit must be between 1 and 200\n",
			wantError:  "testflight metrics app-testers: --limit must be between 1 and 200",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--limit",
			usageError: true,
		},
		{
			name:       "app tester usage period enum",
			command:    TestFlightMetricsBetaTesterUsagesCommand,
			args:       []string{"--app", "app-1", "--period", "P10D"},
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
			stderr, runErr := runTestFlightDiagnosticCommand(t, test.command, test.depsCommand, test.args)
			if runErr == nil {
				t.Fatal("expected validation error")
			}
			if test.wantError != "" {
				if errors.Is(runErr, flag.ErrHelp) != test.usageError {
					t.Fatalf("errors.Is(flag.ErrHelp) = %t, want %t, error = %v",
						!test.usageError, test.usageError, runErr)
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

// runTestFlightDiagnosticCommand parses args on the command that build or
// depsBuild returns, runs it, and returns its stderr and error. A command
// built on shared.Deps writes stderr to the deps and nothing to the process.
func runTestFlightDiagnosticCommand(t *testing.T, build func() *ffcli.Command, depsBuild func(*shared.Deps) *ffcli.Command, args []string) (string, error) {
	t.Helper()

	var depsStdout, depsStderr bytes.Buffer
	var cmd *ffcli.Command
	if depsBuild != nil {
		cmd = depsBuild(&shared.Deps{IO: &shared.IO{Out: &depsStdout, ErrOut: &depsStderr}})
	} else {
		cmd = build()
	}
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stderr := captureTestFlightDiagnosticStderr(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if depsBuild == nil {
		return stderr, runErr
	}
	if stderr != "" {
		t.Fatalf("process stderr = %q, want empty", stderr)
	}
	if depsStdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", depsStdout.String())
	}
	return depsStderr.String(), runErr
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
