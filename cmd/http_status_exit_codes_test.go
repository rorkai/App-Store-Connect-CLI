package cmd

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestExitCodeFromError_WebAPIStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected int
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, expected: ExitAuth},
		{name: "forbidden", status: http.StatusForbidden, expected: ExitAuth},
		{name: "not found", status: http.StatusNotFound, expected: ExitNotFound},
		{name: "conflict", status: http.StatusConflict, expected: ExitConflict},
		{name: "server error", status: http.StatusInternalServerError, expected: ExitHTTPInternalServer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("web command failed: %w", &webcore.APIError{Status: tt.status})
			if got := ExitCodeFromError(err); got != tt.expected {
				t.Fatalf("ExitCodeFromError() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestExitCodeFromError_StoreKitAPIStatusRemainsGeneric(t *testing.T) {
	err := fmt.Errorf("storekit command failed: %w", &storekit.APIError{StatusCode: http.StatusInternalServerError})
	if got := ExitCodeFromError(err); got != ExitError {
		t.Fatalf("ExitCodeFromError() = %d, want %d", got, ExitError)
	}
}

type publicStorefrontStatusError struct{ status int }

func (e publicStorefrontStatusError) Error() string               { return "storefront request failed" }
func (e publicStorefrontStatusError) HTTPStatusCode() int         { return e.status }
func (e publicStorefrontStatusError) PublicStorefrontError() bool { return true }

func TestExitCodeFromError_StatusErrorsOutsideASC(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "apple ads 400", err: &appleads.APIError{StatusCode: http.StatusBadRequest}, expected: ExitHTTPBadRequest},
		{name: "apple ads 401", err: &appleads.APIError{StatusCode: http.StatusUnauthorized}, expected: ExitAuth},
		{name: "apple ads 404", err: &appleads.APIError{StatusCode: http.StatusNotFound}, expected: ExitNotFound},
		{name: "apple ads 503", err: &appleads.APIError{StatusCode: http.StatusServiceUnavailable}, expected: ExitHTTPServiceUnavailable},
		{name: "web sign-in 401", err: &webcore.SigninServiceError{Status: http.StatusUnauthorized}, expected: ExitAuth},
		{name: "web sign-in 503", err: &webcore.SigninServiceError{Status: http.StatusServiceUnavailable}, expected: ExitHTTPServiceUnavailable},
		{name: "web 2fa finalization 403", err: &webcore.TwoFactorFinalizationError{Status: http.StatusForbidden}, expected: ExitAuth},
		{name: "web security key rejected 401", err: &webcore.SecurityKeyAssertionRejectedError{Status: http.StatusUnauthorized}, expected: ExitAuth},
		{name: "public storefront 403 stays generic", err: publicStorefrontStatusError{status: http.StatusForbidden}, expected: ExitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCodeFromError(fmt.Errorf("command failed: %w", tt.err)); got != tt.expected {
				t.Fatalf("ExitCodeFromError() = %d, want %d", got, tt.expected)
			}
		})
	}
}
