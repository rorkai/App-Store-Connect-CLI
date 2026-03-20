package validate

import (
	"context"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

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
