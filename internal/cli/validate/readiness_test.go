package validate

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func readinessTestDeps(client *asc.Client) *shared.Deps {
	return &shared.Deps{
		Client:  func() (*asc.Client, error) { return client, nil },
		Now:     time.Now,
		Timeout: shared.ContextWithTimeout,
	}
}

func validateTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		IO:      &shared.IO{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
		Timeout: shared.ContextWithTimeout,
	}, &stdout, &stderr
}

func TestReadinessPricingSkipReason_DowngradesTransientFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
		},
		{
			name: "retryable api error",
			err: &asc.RetryableError{
				Err: &asc.APIError{
					StatusCode: http.StatusTooManyRequests,
					Code:       "RATE_LIMIT_EXCEEDED",
					Title:      "Too Many Requests",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := readinessPricingSkipReason(tt.err); !ok {
				t.Fatalf("expected transient pricing failure to downgrade to a warning, got ok=false for %v", tt.err)
			}
		})
	}
}

func TestReadinessAvailabilitySkipReason_DowngradesTransientFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
		},
		{
			name: "retryable api error",
			err: &asc.RetryableError{
				Err: &asc.APIError{
					StatusCode: http.StatusServiceUnavailable,
					Code:       "SERVICE_UNAVAILABLE",
					Title:      "Service Unavailable",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := readinessAvailabilitySkipReason(tt.err); !ok {
				t.Fatalf("expected transient availability failure to downgrade to a warning, got ok=false for %v", tt.err)
			}
		})
	}
}
