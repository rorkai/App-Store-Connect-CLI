package versions

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestVersionsMissingRequiredInputExposesStructuredDiagnostic(t *testing.T) {
	d, _ := newVersionsTestDeps(nil)
	err := newVersionsViewCommand(d).ParseAndRun(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp contract", err)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--version-id" {
		t.Fatalf("diagnostic = %+v, want required_input_missing for --version-id", diagnostic)
	}
}

func TestVersionsLinksInvalidInputExposesStructuredDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantError  string
		wantStderr string
		wantUsage  bool
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name:       "limit out of range",
			args:       []string{"--version-id", "version-1", "--type", "customerReviews", "--limit", "201"},
			wantError:  "versions links: --limit must be between 1 and 200",
			wantStderr: "Error: versions links: --limit must be between 1 and 200\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--limit",
		},
		{
			name:       "next url is not app store connect",
			args:       []string{"--type", "customerReviews", "--next", "https://example.com/v1/apps"},
			wantError:  "versions links: --next must be an App Store Connect URL",
			wantStderr: "Error: versions links: --next must be an App Store Connect URL\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--next",
		},
		{
			name:       "missing relationship type",
			args:       []string{"--version-id", "version-1"},
			wantStderr: "Error: --type is required; must be one of: " + shared.RelationshipTypeValues(appStoreVersionRelationshipList()) + "\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--type",
		},
		{
			name:       "unknown relationship type",
			args:       []string{"--version-id", "version-1", "--type", "notARelationship"},
			wantStderr: `Error: --type "notARelationship" is not a valid relationship type; must be one of: ` + shared.RelationshipTypeValues(appStoreVersionRelationshipList()) + "\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--type",
		},
		{
			name:       "pagination flags on to-one relationship",
			args:       []string{"--version-id", "version-1", "--type", "appStoreReviewDetail", "--paginate"},
			wantStderr: "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "--paginate",
		},
		{
			name:       "limit on to-one relationship",
			args:       []string{"--version-id", "version-1", "--type", "appStoreReviewDetail", "--limit", "10"},
			wantStderr: "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "--limit",
		},
		{
			name:       "next on to-one relationship",
			args:       []string{"--version-id", "version-1", "--type", "appStoreReviewDetail", "--next", "https://api.appstoreconnect.apple.com/v1/apps"},
			wantStderr: "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "--next",
		},
		{
			name:       "multiple pagination flags on to-one relationship",
			args:       []string{"--version-id", "version-1", "--type", "appStoreReviewDetail", "--limit", "10", "--paginate"},
			wantStderr: "Error: --limit, --next, and --paginate are only valid for to-many relationships\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, stderrBuf := newVersionsTestDeps(nil)
			command := newVersionsRelationshipsCommand(d)
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			err := command.Exec(context.Background(), nil)
			stderr := stderrBuf.String()
			if err == nil {
				t.Fatal("expected error")
			}
			if test.wantError != "" && err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if test.wantStderr != "" && stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if got := errors.Is(err, flag.ErrHelp); got != test.wantUsage {
				t.Fatalf("errors.Is(err, flag.ErrHelp) = %t, want %t", got, test.wantUsage)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}
