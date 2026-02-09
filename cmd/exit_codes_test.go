package cmd

import (
	"errors"
	"flag"
	"io"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestExitCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			expected: ExitSuccess,
		},
		{
			name:     "flag.ErrHelp returns usage",
			err:      flag.ErrHelp,
			expected: ExitUsage,
		},
		{
			name:     "ErrMissingAuth returns auth failure",
			err:      shared.ErrMissingAuth,
			expected: ExitAuth,
		},
		{
			name:     "ErrUnauthorized returns auth failure",
			err:      asc.ErrUnauthorized,
			expected: ExitAuth,
		},
		{
			name:     "ErrForbidden returns auth failure",
			err:      asc.ErrForbidden,
			expected: ExitAuth,
		},
		{
			name:     "ErrNotFound returns not found",
			err:      asc.ErrNotFound,
			expected: ExitNotFound,
		},
		{
			name:     "generic error returns generic error",
			err:      errors.New("something went wrong"),
			expected: ExitError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExitCodeFromError(tt.err)
			if result != tt.expected {
				t.Errorf("ExitCodeFromError(%v) = %d, want %d", tt.err, result, tt.expected)
			}
		})
	}
}

func TestExitCodeFromError_Conflict(t *testing.T) {
	conflictErr := &asc.APIError{
		Code:   "CONFLICT",
		Title:  "Conflict",
		Detail: "Resource already exists",
	}
	result := ExitCodeFromError(conflictErr)
	if result != ExitConflict {
		t.Errorf("ExitCodeFromError(conflict) = %d, want %d (Conflict)", result, ExitConflict)
	}
}

func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d, want 0", ExitSuccess)
	}
	if ExitError != 1 {
		t.Errorf("ExitError = %d, want 1", ExitError)
	}
	if ExitUsage != 2 {
		t.Errorf("ExitUsage = %d, want 2", ExitUsage)
	}
	if ExitAuth != 3 {
		t.Errorf("ExitAuth = %d, want 3", ExitAuth)
	}
	if ExitNotFound != 4 {
		t.Errorf("ExitNotFound = %d, want 4", ExitNotFound)
	}
	if ExitConflict != 5 {
		t.Errorf("ExitConflict = %d, want 5", ExitConflict)
	}
}

var _ = io.Discard
