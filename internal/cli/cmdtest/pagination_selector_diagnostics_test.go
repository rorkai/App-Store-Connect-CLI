package cmdtest

import (
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestPaginationSelectorsReportCanonicalMissingParameter(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantParameter string
	}{
		{
			name:          "bundle ID capabilities",
			args:          []string{"bundle-ids", "capabilities", "list"},
			wantParameter: "--bundle",
		},
		{
			name:          "bundle ID profiles",
			args:          []string{"bundle-ids", "profiles", "list"},
			wantParameter: "--id",
		},
		{
			name:          "category subcategories",
			args:          []string{"categories", "subcategories"},
			wantParameter: "--category-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), test.args...)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("expected structured diagnostic, got %v", err)
			}
			if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != test.wantParameter {
				t.Fatalf("diagnostic = %+v, want required_input_missing for %q", diagnostic, test.wantParameter)
			}
		})
	}
}
