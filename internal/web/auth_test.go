package web

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPreparePasswordForProtocol(t *testing.T) {
	t.Run("s2k", func(t *testing.T) {
		prepared, err := preparePasswordForProtocol("example", "s2k")
		if err != nil {
			t.Fatalf("preparePasswordForProtocol returned error: %v", err)
		}
		if len(prepared) != 32 {
			t.Fatalf("expected 32-byte digest for s2k, got %d", len(prepared))
		}
	})

	t.Run("s2k_fo", func(t *testing.T) {
		prepared, err := preparePasswordForProtocol("example", "s2k_fo")
		if err != nil {
			t.Fatalf("preparePasswordForProtocol returned error: %v", err)
		}
		if len(prepared) != 64 {
			t.Fatalf("expected 64-byte hex digest for s2k_fo, got %d", len(prepared))
		}
	})

	t.Run("unsupported protocol", func(t *testing.T) {
		if _, err := preparePasswordForProtocol("example", "unknown"); err == nil {
			t.Fatal("expected error for unsupported protocol")
		}
	})
}

func TestMakeHashcash(t *testing.T) {
	now := time.Date(2026, 2, 24, 18, 0, 0, 0, time.UTC)
	hashcash := makeHashcash(10, "4d74fb15eb23f465f1f6fcbf534e5877", now)
	parts := strings.Split(hashcash, ":")
	if len(parts) != 6 {
		t.Fatalf("expected 6 hashcash fields, got %d (%q)", len(parts), hashcash)
	}
	if parts[0] != "1" {
		t.Fatalf("unexpected hashcash version: %q", parts[0])
	}
	if parts[1] != "10" {
		t.Fatalf("unexpected bits field: %q", parts[1])
	}
	if parts[2] != "20260224180000" {
		t.Fatalf("unexpected date field: %q", parts[2])
	}
	if parts[3] != "4d74fb15eb23f465f1f6fcbf534e5877" {
		t.Fatalf("unexpected challenge field: %q", parts[3])
	}
	sum := sha1.Sum([]byte(hashcash))
	if !hasLeadingZeroBits(sum[:], 10) {
		t.Fatalf("hashcash does not satisfy required leading-zero bits: %q", hashcash)
	}
}

func TestParseSigninInitResponseChallengeObject(t *testing.T) {
	input := []byte(`{
		"iteration": 21000,
		"salt": "c2FsdA==",
		"protocol": "s2k_fo",
		"b": "AQIDBA==",
		"c": {"v":1,"n":"test","u":"user@example.com"}
	}`)

	parsed, err := parseSigninInitResponse(input)
	if err != nil {
		t.Fatalf("parseSigninInitResponse error: %v", err)
	}
	if len(parsed.Challenge) == 0 {
		t.Fatal("expected non-empty challenge")
	}

	var challenge map[string]any
	if err := json.Unmarshal(parsed.Challenge, &challenge); err != nil {
		t.Fatalf("expected challenge to be JSON object, got decode error: %v", err)
	}
	if challenge["n"] != "test" {
		t.Fatalf("expected challenge.n=test, got %#v", challenge["n"])
	}
}

func TestParseSigninInitResponseMissingChallenge(t *testing.T) {
	input := []byte(`{
		"iteration": 21000,
		"salt": "c2FsdA==",
		"protocol": "s2k_fo",
		"b": "AQIDBA=="
	}`)
	if _, err := parseSigninInitResponse(input); err == nil {
		t.Fatal("expected missing challenge error")
	}
}

func TestClientDoRequestHonorsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.doRequest(ctx, "GET", "/apps", nil)
	if err == nil {
		t.Fatal("expected canceled-context error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "context canceled") {
		t.Fatalf("expected context canceled in error, got %v", err)
	}
}
