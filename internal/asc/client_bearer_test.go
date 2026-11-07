package asc

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestNewClientFromBearerTokenSendsTheTokenAsIs(t *testing.T) {
	token, err := GenerateJWT("KEY123", "ISS456", testJWTPrivateKey(t))
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}

	client, err := NewClientFromBearerToken(" "+token+"\n", 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}
	req, err := client.newRequest(context.Background(), http.MethodGet, "/v1/apps", nil)
	if err != nil {
		t.Fatalf("newRequest() error: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer "+token {
		t.Fatalf("Authorization = %q, want the given token", got)
	}
}

func TestNewClientFromBearerTokenRejectsExpiredAndMalformedTokens(t *testing.T) {
	if _, err := NewClientFromBearerToken("not-a-jwt", 0); err == nil {
		t.Fatal("NewClientFromBearerToken(not-a-jwt) error = nil, want invalid bearer token")
	}

	client, err := NewClientFromBearerToken(mustGenerateJWT(t), 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}
	if _, err := client.bearerJWT(time.Now().Add(tokenLifetime + time.Second)); !errors.Is(err, ErrBearerTokenExpired) {
		t.Fatalf("bearerJWT() after expiry error = %v, want ErrBearerTokenExpired", err)
	}
}

func TestBearerTokenClientRefusesNotarization(t *testing.T) {
	client, err := NewClientFromBearerToken(mustGenerateJWT(t), 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}

	if _, err := client.newNotaryRequest(context.Background(), http.MethodGet, "/notary/v2/submissions", nil); !errors.Is(err, ErrNotaryNeedsPrivateKey) {
		t.Fatalf("newNotaryRequest() error = %v, want ErrNotaryNeedsPrivateKey", err)
	}
}

func mustGenerateJWT(t *testing.T) string {
	t.Helper()
	token, err := GenerateJWT("KEY123", "ISS456", testJWTPrivateKey(t))
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}
	return token
}
