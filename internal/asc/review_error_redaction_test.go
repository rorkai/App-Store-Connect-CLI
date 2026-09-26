package asc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

const submittedPasswordErrorSentinel = "asc-submitted-password-sentinel-91a47f"

func echoedPasswordErrorResponse() *http.Response {
	return jsonResponse(
		http.StatusBadRequest,
		`{"errors":[{"code":"BAD_REQUEST","title":"Invalid password `+
			submittedPasswordErrorSentinel+
			`","detail":"demoAccountPassword `+
			submittedPasswordErrorSentinel+
			` is rejected","meta":{"associatedErrors":{"reviewDetail":[{"detail":"bad `+
			submittedPasswordErrorSentinel+
			`"}]}}}]}`,
	)
}

func assertSubmittedPasswordRedacted(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected API error")
	}
	if strings.Contains(err.Error(), submittedPasswordErrorSentinel) {
		t.Fatalf("API error exposed submitted password: %v", err)
	}
	if !strings.Contains(err.Error(), RedactedValuePlaceholder) {
		t.Fatalf("API error did not mark redacted password: %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("API error classification was not preserved: %T: %v", err, err)
	}
	if strings.Contains(apiErr.Error(), submittedPasswordErrorSentinel) {
		t.Fatalf("errors.As exposed submitted password: %v", apiErr)
	}
	for _, detail := range apiErr.AllDetails {
		if strings.Contains(detail, submittedPasswordErrorSentinel) {
			t.Fatalf("errors.As exposed submitted password through AllDetails: %q", detail)
		}
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("bad-request classification was not preserved: %v", err)
	}
}

func TestCreateAppStoreReviewDetailRedactsSubmittedPasswordFromAPIError(t *testing.T) {
	client := newTestClient(t, nil, echoedPasswordErrorResponse())
	password := submittedPasswordErrorSentinel
	_, err := client.CreateAppStoreReviewDetail(
		context.Background(),
		"version-1",
		&AppStoreReviewDetailCreateAttributes{DemoAccountPassword: &password},
	)
	assertSubmittedPasswordRedacted(t, err)
}

func TestUpdateAppStoreReviewDetailRedactsSubmittedPasswordFromAPIError(t *testing.T) {
	client := newTestClient(t, nil, echoedPasswordErrorResponse())
	password := submittedPasswordErrorSentinel
	_, err := client.UpdateAppStoreReviewDetail(
		context.Background(),
		"detail-1",
		AppStoreReviewDetailUpdateAttributes{DemoAccountPassword: &password},
	)
	assertSubmittedPasswordRedacted(t, err)
}

func TestUpdateBetaAppReviewDetailRedactsSubmittedPasswordFromAPIError(t *testing.T) {
	client := newTestClient(t, nil, echoedPasswordErrorResponse())
	password := submittedPasswordErrorSentinel
	_, err := client.UpdateBetaAppReviewDetail(
		context.Background(),
		"detail-1",
		BetaAppReviewDetailUpdateAttributes{DemoAccountPassword: &password},
	)
	assertSubmittedPasswordRedacted(t, err)
}

func TestSubmittedPasswordRedactionPreservesProtocolCodeClassification(t *testing.T) {
	password := "BAD"
	response := jsonResponse(
		http.StatusBadRequest,
		`{"errors":[{"code":"BAD_REQUEST","title":"Invalid submitted credential","detail":"credential rejected"}]}`,
	)
	client := newTestClient(t, nil, response)
	_, err := client.UpdateAppStoreReviewDetail(
		context.Background(),
		"detail-1",
		AppStoreReviewDetailUpdateAttributes{DemoAccountPassword: &password},
	)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("errors.Is(err, ErrBadRequest) = false: %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("errors.As(err, *APIError) = false: %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Fatalf("safe API error code = %q, want BAD_REQUEST", apiErr.Code)
	}
}

func TestReviewPasswordRedactionKeepsRetryableChainSafe(t *testing.T) {
	response := jsonResponse(
		http.StatusInternalServerError,
		`{"errors":[{"code":"INTERNAL_ERROR","title":"temporary failure `+
			submittedPasswordErrorSentinel+
			`","detail":"retry `+
			submittedPasswordErrorSentinel+
			`"}]}`,
	)
	client := newTestClient(t, nil, response)
	password := submittedPasswordErrorSentinel
	_, err := client.UpdateAppStoreReviewDetail(
		context.Background(),
		"detail-1",
		AppStoreReviewDetailUpdateAttributes{DemoAccountPassword: &password},
	)
	if err == nil {
		t.Fatal("expected retryable API error")
	}
	if !IsRetryable(err) {
		t.Fatalf("retryable classification was not preserved: %v", err)
	}
	var retryable *RetryableError
	if !errors.As(err, &retryable) {
		t.Fatalf("retryable error was not inspectable: %v", err)
	}
	if strings.Contains(retryable.Error(), submittedPasswordErrorSentinel) {
		t.Fatalf("errors.As exposed submitted password through RetryableError: %v", retryable)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("API error was not inspectable: %v", err)
	}
	if strings.Contains(apiErr.Error(), submittedPasswordErrorSentinel) {
		t.Fatalf("errors.As exposed submitted password through APIError: %v", apiErr)
	}
}
