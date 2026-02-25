package web

import (
	"errors"
	"strings"
	"testing"
)

func TestAPIErrorRedactsRawBodyInErrorString(t *testing.T) {
	err := &APIError{
		Status:         422,
		AppleRequestID: "abc-request-id",
		CorrelationKey: "abc-correlation-key",
		rawBody:        []byte(`{"detail":"super-secret-token-123"}`),
	}
	message := err.Error()
	if strings.Contains(message, "super-secret-token-123") {
		t.Fatalf("expected redacted error string, got %q", message)
	}
	if !strings.Contains(message, "status 422") {
		t.Fatalf("expected status in error message, got %q", message)
	}
}

func TestIsDuplicateAppNameError(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantDup bool
	}{
		{
			name: "duplicate by code and detail",
			err: &APIError{rawBody: []byte(`{
				"errors":[{
					"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT",
					"detail":"The app name you entered is already being used."
				}]
			}`)},
			wantDup: true,
		},
		{
			name: "non-duplicate code",
			err: &APIError{rawBody: []byte(`{
				"errors":[{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"invalid value"}]
			}`)},
			wantDup: false,
		},
		{
			name:    "non api error",
			err:     errors.New("nope"),
			wantDup: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDuplicateAppNameError(tc.err); got != tc.wantDup {
				t.Fatalf("IsDuplicateAppNameError()=%v want %v", got, tc.wantDup)
			}
		})
	}
}
