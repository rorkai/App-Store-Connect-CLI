package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWaitForBuildProcessing_ReturnsValid(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		state := BuildProcessingStateProcessing
		if calls > 1 {
			state = BuildProcessingStateValid
		}
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, state)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	build, err := client.WaitForBuildProcessing(ctx, "build-1", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildProcessing() error: %v", err)
	}
	if build.Data.Attributes.ProcessingState != BuildProcessingStateValid {
		t.Fatalf("expected processing state %q, got %q", BuildProcessingStateValid, build.Data.Attributes.ProcessingState)
	}
}

func TestWaitForBuildProcessing_InvalidReturnsError(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, BuildProcessingStateInvalid)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := client.WaitForBuildProcessing(ctx, "build-1", 1*time.Millisecond); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestWaitForBuildProcessing_FailedReturnsError(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, BuildProcessingStateFailed)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.WaitForBuildProcessing(ctx, "build-1", 1*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), BuildProcessingStateFailed) {
		t.Fatalf("expected FAILED error, got %v", err)
	}
}
