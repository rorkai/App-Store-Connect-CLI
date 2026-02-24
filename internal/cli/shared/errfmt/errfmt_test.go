package errfmt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestClassify_MissingAuth(t *testing.T) {
	err := errors.New("wrapped")
	err = wrap(err, shared.ErrMissingAuth)
	ce := Classify(err)
	if ce.Hint == "" {
		t.Fatalf("expected hint, got empty")
	}
}

func TestClassify_Forbidden(t *testing.T) {
	apiErr := &asc.APIError{Code: "FORBIDDEN", Title: "Forbidden", Detail: "Nope"}
	ce := Classify(apiErr)
	if ce.Hint == "" {
		t.Fatalf("expected hint, got empty")
	}
}

func TestClassify_Timeout(t *testing.T) {
	ce := Classify(context.DeadlineExceeded)
	if ce.Hint == "" {
		t.Fatalf("expected hint, got empty")
	}
}

func TestClassify_PrivacyDataUsages(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantHit bool
	}{
		{
			name:    "associated error with appDataUsages path",
			err:     errors.New("submit create: failed to submit for review: Associated errors for /v1/appDataUsages/: missing required data"),
			wantHit: true,
		},
		{
			name:    "associated error with appDataUsagesPublications",
			err:     errors.New("submit create: /v1/appDataUsagesPublications/ not published"),
			wantHit: true,
		},
		{
			name:    "unrelated error",
			err:     errors.New("submit create: failed to attach build"),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := Classify(tt.err)
			if tt.wantHit && ce.Hint == "" {
				t.Fatalf("expected privacy hint, got empty")
			}
			if tt.wantHit && !strings.Contains(ce.Hint, "App Store Connect web UI") {
				t.Fatalf("expected web UI hint, got: %s", ce.Hint)
			}
			if !tt.wantHit && ce.Hint != "" {
				t.Fatalf("did not expect hint for unrelated error, got: %s", ce.Hint)
			}
		})
	}
}

func TestClassify_PrivacyDataUsages_TakesPrecedenceOverForbidden(t *testing.T) {
	err := &asc.APIError{
		Code:   "FORBIDDEN",
		Title:  "Forbidden",
		Detail: "Associated resources failed validation",
		AssociatedErrors: map[string][]asc.APIAssociatedError{
			"/v1/appDataUsages/": {
				{
					Code:   "ENTITY_ERROR.ATTRIBUTE.REQUIRED",
					Detail: "Missing required privacy answers",
				},
			},
		},
	}

	ce := Classify(err)
	if !strings.Contains(ce.Hint, "App Store Connect web UI") {
		t.Fatalf("expected privacy hint to take precedence, got: %s", ce.Hint)
	}
	if strings.Contains(ce.Hint, "role/permissions") {
		t.Fatalf("expected privacy hint, got permissions hint: %s", ce.Hint)
	}
}

// wrap creates an error that Is() matches target without altering the base string.
type isWrapper struct {
	target error
}

func (e isWrapper) Error() string { return "x" }
func (e isWrapper) Is(t error) bool {
	return t == e.target
}

func wrap(base error, target error) error {
	_ = base
	return isWrapper{target: target}
}
