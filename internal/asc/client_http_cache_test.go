package asc

import (
	"context"
	"testing"
	"time"
)

func TestGenerateJWT_UsesCachedTokenWhenStillValid(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(200, `{"data":[]}`))
	client.cachedJWT = "cached-token"
	client.cachedJWTExpiresAt = time.Now().Add(2 * time.Minute)

	token, err := client.generateJWT(context.Background())
	if err != nil {
		t.Fatalf("generateJWT() error: %v", err)
	}
	if token != "cached-token" {
		t.Fatalf("expected cached token, got %q", token)
	}
}

func TestGenerateJWT_RefreshesExpiredOrNearExpiryToken(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(200, `{"data":[]}`))
	client.cachedJWT = "stale-token"
	client.cachedJWTExpiresAt = time.Now().Add(5 * time.Second)

	token, err := client.generateJWT(context.Background())
	if err != nil {
		t.Fatalf("generateJWT() error: %v", err)
	}
	if token == "stale-token" {
		t.Fatal("expected stale token to be refreshed")
	}
	if client.cachedJWT != token {
		t.Fatalf("expected client cache to update with fresh token, got %q", client.cachedJWT)
	}
}
