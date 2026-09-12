package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestNotaryClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	c := &Client{
		httpClient: &http.Client{},
		keyID:      "TEST_KEY",
		issuerID:   "TEST_ISSUER",
		privateKey: key,
	}
	if serverURL != "" {
		c.SetNotaryBaseURL(serverURL)
	}
	return c
}

const (
	testNotarySHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testSubmissionID = "550e8400-e29b-41d4-a716-446655440000"
)

type cancelAfterReadSeeker struct {
	reader *bytes.Reader
	cancel context.CancelFunc
}

func (r *cancelAfterReadSeeker) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.cancel()
	}
	return n, err
}

func (r *cancelAfterReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return r.reader.Seek(offset, whence)
}

func TestGenerateNotaryJWT(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	token, err := GenerateNotaryJWT("KEY_ID", "ISSUER_ID", key)
	if err != nil {
		t.Fatalf("GenerateNotaryJWT() error: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Token should have 3 parts (header.payload.signature)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 token parts, got %d", len(parts))
	}
}

func TestGenerateNotaryJWT_BackdatesIssuedAtForClockSkew(t *testing.T) {
	privateKey := testJWTPrivateKey(t)

	before := time.Now()
	tokenString, err := GenerateNotaryJWT("KEY123", "ISS456", privateKey)
	if err != nil {
		t.Fatalf("GenerateNotaryJWT() error: %v", err)
	}
	after := time.Now()

	claims := parseJWTClaims(t, tokenString, privateKey)
	assertJWTIssuedAtSkew(t, claims.IssuedAt, claims.ExpiresAt, before, after, tokenLifetime)
}

func TestGenerateNotaryJWT_NormalizesIdentifiers(t *testing.T) {
	privateKey := testJWTPrivateKey(t)

	tokenString, err := GenerateNotaryJWT("  KEY123\n", "\tISS456  ", privateKey)
	if err != nil {
		t.Fatalf("GenerateNotaryJWT() error: %v", err)
	}

	token, claims := parseJWT(t, tokenString, privateKey)
	if claims.Issuer != "ISS456" {
		t.Fatalf("issuer claim = %q, want ISS456", claims.Issuer)
	}
	if keyID, ok := token.Header["kid"].(string); !ok || keyID != "KEY123" {
		t.Fatalf("key ID header = %#v, want KEY123", token.Header["kid"])
	}
}

func TestGenerateNotaryJWT_WhitespaceIssuerUsesUserSubjectClaim(t *testing.T) {
	privateKey := testJWTPrivateKey(t)

	tokenString, err := GenerateNotaryJWT("KEY123", " \t\n ", privateKey)
	if err != nil {
		t.Fatalf("GenerateNotaryJWT() error: %v", err)
	}

	_, claims := parseJWT(t, tokenString, privateKey)
	if claims.Issuer != "" {
		t.Fatalf("issuer claim = %q, want empty", claims.Issuer)
	}
	if claims.Subject != "user" {
		t.Fatalf("subject claim = %q, want user", claims.Subject)
	}
}

func TestGenerateNotaryJWT_RejectsWhitespaceKeyID(t *testing.T) {
	privateKey := testJWTPrivateKey(t)

	_, err := GenerateNotaryJWT(" \t\n ", "ISS456", privateKey)
	if !errors.Is(err, ErrMissingKeyID) {
		t.Fatalf("GenerateNotaryJWT() error = %v, want ErrMissingKeyID", err)
	}
}

func TestSubmitNotarization_SendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/notary/v2/submissions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify Authorization header is present
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Bearer auth, got %q", auth)
		}

		body, _ := io.ReadAll(r.Body)
		var req NotarySubmissionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("parse request: %v", err)
		}
		if req.Sha256 != testNotarySHA256 {
			t.Errorf("expected sha256 %s, got %s", testNotarySHA256, req.Sha256)
		}
		if req.SubmissionName != "MyApp.zip" {
			t.Errorf("expected name MyApp.zip, got %s", req.SubmissionName)
		}

		resp := NotarySubmissionResponse{
			Data: NotarySubmissionResponseData{
				Type: "newSubmissions",
				ID:   "sub-123",
				Attributes: NotarySubmissionResponseAttributes{
					AwsAccessKeyID:     "AKID",
					AwsSecretAccessKey: "SECRET",
					AwsSessionToken:    "TOKEN",
					Bucket:             "notary-submissions-prod",
					Object:             "obj-key",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, resp)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	ctx := context.Background()

	resp, err := client.SubmitNotarization(ctx, "  "+strings.ToUpper(testNotarySHA256)+"  ", "MyApp.zip")
	if err != nil {
		t.Fatalf("SubmitNotarization() error: %v", err)
	}

	if resp.Data.ID != "sub-123" {
		t.Errorf("expected ID sub-123, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.AwsAccessKeyID != "AKID" {
		t.Errorf("expected AKID, got %s", resp.Data.Attributes.AwsAccessKeyID)
	}
	if resp.Data.Attributes.Bucket != "notary-submissions-prod" {
		t.Errorf("expected bucket notary-submissions-prod, got %s", resp.Data.Attributes.Bucket)
	}
	if resp.Data.Attributes.Object != "obj-key" {
		t.Errorf("expected object obj-key, got %s", resp.Data.Attributes.Object)
	}
}

func TestSubmitNotarization_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		mustEncodeJSON(t, w, map[string]any{
			"errors": []map[string]string{
				{"code": "FORBIDDEN", "title": "Forbidden", "detail": "Invalid credentials"},
			},
		})
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.SubmitNotarization(context.Background(), testNotarySHA256, "test.zip")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestSubmitNotarization_RejectsInvalidSHA256BeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.SubmitNotarization(context.Background(), "not-a-sha256", "test.zip")
	if err == nil || !strings.Contains(err.Error(), "64 hexadecimal") {
		t.Fatalf("SubmitNotarization() error = %v, want strict SHA-256 validation error", err)
	}
	if requests != 0 {
		t.Fatalf("got %d requests for invalid hash, want 0", requests)
	}
}

func TestValidateNotarySHA256(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantError bool
	}{
		{name: "empty", value: "", wantError: true},
		{name: "short", value: "abc123", wantError: true},
		{name: "long", value: testNotarySHA256 + "0", wantError: true},
		{name: "non hexadecimal", value: strings.Repeat("g", 64), wantError: true},
		{name: "uppercase and whitespace", value: "  " + strings.ToUpper(testNotarySHA256) + "  ", want: testNotarySHA256},
		{name: "valid", value: testNotarySHA256, want: testNotarySHA256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateNotarySHA256(tt.value)
			if tt.wantError {
				if err == nil {
					t.Fatal("validateNotarySHA256() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateNotarySHA256() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("validateNotarySHA256() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetNotarizationStatus_SendsRequest(t *testing.T) {
	tests := []struct {
		name   string
		status NotarySubmissionStatus
	}{
		{"accepted", NotaryStatusAccepted},
		{"in progress", NotaryStatusInProgress},
		{"invalid", NotaryStatusInvalid},
		{"rejected", NotaryStatusRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/notary/v2/submissions/"+testSubmissionID) {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				resp := NotarySubmissionStatusResponse{
					Data: NotarySubmissionStatusData{
						ID:   testSubmissionID,
						Type: "submissions",
						Attributes: NotarySubmissionStatusAttributes{
							Status:      tt.status,
							Name:        "test.zip",
							CreatedDate: "2026-01-15T10:00:00Z",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				mustEncodeJSON(t, w, resp)
			}))
			defer server.Close()

			client := newTestNotaryClient(t, server.URL)
			resp, err := client.GetNotarizationStatus(context.Background(), "  "+testSubmissionID+"  ")
			if err != nil {
				t.Fatalf("GetNotarizationStatus() error: %v", err)
			}

			if resp.Data.Attributes.Status != tt.status {
				t.Errorf("expected status %s, got %s", tt.status, resp.Data.Attributes.Status)
			}
			if resp.Data.ID != testSubmissionID {
				t.Errorf("expected ID %s, got %s", testSubmissionID, resp.Data.ID)
			}
			if resp.Data.Attributes.Name != "test.zip" {
				t.Errorf("expected name test.zip, got %s", resp.Data.Attributes.Name)
			}
		})
	}
}

func TestGetNotarizationStatus_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustWriteBody(t, w, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"Submission not found"}]}`)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.GetNotarizationStatus(context.Background(), testSubmissionID)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGetNotarizationLogs_SendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/notary/v2/submissions/"+testSubmissionID+"/logs") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := NotarySubmissionLogsResponse{
			Data: NotarySubmissionLogsData{
				ID:   testSubmissionID,
				Type: "submissionsLog",
				Attributes: NotarySubmissionLogsAttributes{
					DeveloperLogURL: "https://example.com/logs/" + testSubmissionID + ".json",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, resp)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	resp, err := client.GetNotarizationLogs(context.Background(), "  "+testSubmissionID+"  ")
	if err != nil {
		t.Fatalf("GetNotarizationLogs() error: %v", err)
	}

	if resp.Data.Attributes.DeveloperLogURL != "https://example.com/logs/"+testSubmissionID+".json" {
		t.Errorf("unexpected log URL: %s", resp.Data.Attributes.DeveloperLogURL)
	}
	if resp.Data.ID != testSubmissionID {
		t.Errorf("unexpected ID: %s", resp.Data.ID)
	}
}

func TestGetNotarizationLogs_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustWriteBody(t, w, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"Logs not available"}]}`)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.GetNotarizationLogs(context.Background(), testSubmissionID)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestNotarizationRequestIDsRejectInvalidValuesBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
	}))
	defer server.Close()

	invalidIDs := []string{
		"",
		"sub-456",
		"550e8400e29b41d4a716446655440000",
		"urn:uuid:" + testSubmissionID,
		"{" + testSubmissionID + "}",
		"550e8400-e29b-41d4-a716-44665544000g",
		"550e8400-e29b-41d4-a716-446655440000?admin=true",
		"550e8400-e29b-41d4-a716-446655440000/logs",
	}

	for _, id := range invalidIDs {
		client := newTestNotaryClient(t, server.URL)
		if _, err := client.GetNotarizationStatus(context.Background(), id); err == nil {
			t.Errorf("GetNotarizationStatus(%q) error = nil, want validation error", id)
		}
		if _, err := client.GetNotarizationLogs(context.Background(), id); err == nil {
			t.Errorf("GetNotarizationLogs(%q) error = nil, want validation error", id)
		}
	}
	if requests != 0 {
		t.Fatalf("got %d requests for invalid IDs, want 0", requests)
	}
}

func TestValidateNotarySubmissionID(t *testing.T) {
	got, err := validateNotarySubmissionID("  " + testSubmissionID + "  ")
	if err != nil {
		t.Fatalf("validateNotarySubmissionID() error = %v", err)
	}
	if got != testSubmissionID {
		t.Fatalf("validateNotarySubmissionID() = %q, want %q", got, testSubmissionID)
	}

	if _, err := validateNotarySubmissionID(strings.ToUpper(testSubmissionID)); err != nil {
		t.Fatalf("uppercase canonical UUID rejected: %v", err)
	}
}

func TestListNotarizations_SendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/notary/v2/submissions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := NotarySubmissionsListResponse{
			Data: []NotarySubmissionStatusData{
				{
					ID:   "sub-1",
					Type: "submissions",
					Attributes: NotarySubmissionStatusAttributes{
						Status:      NotaryStatusAccepted,
						Name:        "app1.zip",
						CreatedDate: "2026-01-10T10:00:00Z",
					},
				},
				{
					ID:   "sub-2",
					Type: "submissions",
					Attributes: NotarySubmissionStatusAttributes{
						Status:      NotaryStatusInProgress,
						Name:        "app2.zip",
						CreatedDate: "2026-01-15T10:00:00Z",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, resp)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	resp, err := client.ListNotarizations(context.Background())
	if err != nil {
		t.Fatalf("ListNotarizations() error: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 submissions, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "sub-1" {
		t.Errorf("expected ID sub-1, got %s", resp.Data[0].ID)
	}
	if resp.Data[0].Attributes.Status != NotaryStatusAccepted {
		t.Errorf("expected Accepted, got %s", resp.Data[0].Attributes.Status)
	}
	if resp.Data[1].ID != "sub-2" {
		t.Errorf("expected ID sub-2, got %s", resp.Data[1].ID)
	}
	if resp.Data[1].Attributes.Status != NotaryStatusInProgress {
		t.Errorf("expected In Progress, got %s", resp.Data[1].Attributes.Status)
	}
}

func TestListNotarizations_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := NotarySubmissionsListResponse{
			Data: []NotarySubmissionStatusData{},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, resp)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	resp, err := client.ListNotarizations(context.Background())
	if err != nil {
		t.Fatalf("ListNotarizations() error: %v", err)
	}

	if len(resp.Data) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp.Data))
	}
}

func TestListNotarizations_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		mustWriteBody(t, w, `{"errors":[{"code":"UNAUTHORIZED","title":"Unauthorized"}]}`)
	}))
	defer server.Close()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.ListNotarizations(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestComputeFileSHA256(t *testing.T) {
	content := []byte("hello world")
	file := bytes.NewReader(content)
	got, err := ComputeFileSHA256(context.Background(), file)
	if err != nil {
		t.Fatalf("ComputeFileSHA256() error: %v", err)
	}

	// Expected SHA-256 of "hello world"
	h := sha256.Sum256(content)
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	replayed, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read rewound file: %v", err)
	}
	if !bytes.Equal(replayed, content) {
		t.Fatalf("rewound contents = %q, want %q", replayed, content)
	}
}

func TestComputeFileSHA256_EmptyFile(t *testing.T) {
	got, err := ComputeFileSHA256(context.Background(), bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("ComputeFileSHA256() error: %v", err)
	}

	// SHA-256 of empty data
	h := sha256.Sum256(nil)
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestComputeFileSHA256_ContextCancellationBeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := ComputeFileSHA256(ctx, bytes.NewReader([]byte("data")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ComputeFileSHA256() error = %v, want context.Canceled", err)
	}
	if got != "" {
		t.Fatalf("ComputeFileSHA256() hash = %q, want empty hash on cancellation", got)
	}
}

func TestComputeFileSHA256_ContextCancellationDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	file := &cancelAfterReadSeeker{
		reader: bytes.NewReader(bytes.Repeat([]byte("x"), 128*1024)),
		cancel: cancel,
	}

	got, err := ComputeFileSHA256(ctx, file)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ComputeFileSHA256() error = %v, want context.Canceled", err)
	}
	if got != "" {
		t.Fatalf("ComputeFileSHA256() hash = %q, want empty hash on cancellation", got)
	}
}

func TestUploadToS3Helpers(t *testing.T) {
	// We can't easily test UploadToS3 against a mock since it builds URLs from bucket/object,
	// but we can test the crypto helpers and validation.

	// Test sha256Hex
	data := []byte("test data")
	hash := sha256Hex(data)
	h := sha256.Sum256(data)
	expected := hex.EncodeToString(h[:])
	if hash != expected {
		t.Errorf("sha256Hex: got %s, want %s", hash, expected)
	}

	// Test deriveSigningKey produces non-empty result
	sigKey := deriveSigningKey("secret", "20260206", "us-west-2", "s3")
	if len(sigKey) == 0 {
		t.Fatal("deriveSigningKey returned empty key")
	}

	// Test hmacSHA256 produces consistent results
	mac1 := hmacSHA256([]byte("key"), []byte("data"))
	mac2 := hmacSHA256([]byte("key"), []byte("data"))
	if hex.EncodeToString(mac1) != hex.EncodeToString(mac2) {
		t.Fatal("hmacSHA256 not deterministic")
	}

	// Test different keys produce different MACs
	mac3 := hmacSHA256([]byte("other-key"), []byte("data"))
	if hex.EncodeToString(mac1) == hex.EncodeToString(mac3) {
		t.Fatal("different keys should produce different MACs")
	}

	encoded, err := encodeS3ObjectPath("AROARQRX7CZS3PRF6ZA5L:22390004-2418-4edc-bb06-661cca8cf6e0")
	if err != nil {
		t.Fatalf("encodeS3ObjectPath() error: %v", err)
	}
	if !strings.Contains(encoded, "%3A") {
		t.Fatalf("expected encoded path to contain %%3A, got %q", encoded)
	}
	if !strings.HasPrefix(encoded, "/") {
		t.Fatalf("expected encoded path to start with '/', got %q", encoded)
	}
	if strings.Contains(encoded, "//") {
		t.Fatalf("unexpected double slash in encoded path: %q", encoded)
	}
}

func mustEncodeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func mustWriteBody(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func TestUploadToS3_Validation(t *testing.T) {
	validCredentials := S3Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Bucket:          "bucket",
		Object:          "object",
	}

	tests := []struct {
		name   string
		mutate func(*S3Credentials)
		want   string
	}{
		{name: "access key ID", mutate: func(creds *S3Credentials) { creds.AccessKeyID = "" }, want: "S3 access key ID is required"},
		{name: "secret access key", mutate: func(creds *S3Credentials) { creds.SecretAccessKey = "" }, want: "S3 secret access key is required"},
		{name: "session token", mutate: func(creds *S3Credentials) { creds.SessionToken = "" }, want: "S3 session token is required"},
		{name: "bucket", mutate: func(creds *S3Credentials) { creds.Bucket = "" }, want: "S3 bucket is required"},
		{name: "object", mutate: func(creds *S3Credentials) { creds.Object = "" }, want: "S3 object is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := validCredentials
			tt.mutate(&creds)
			err := UploadToS3(context.Background(), creds, strings.NewReader("data"), testNotarySHA256, 4, "application/octet-stream")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("UploadToS3() error = %v, want %q", err, tt.want)
			}
		})
	}

	err := UploadToS3(context.Background(), validCredentials, strings.NewReader("data"), "", 4, "application/octet-stream")
	if err == nil || !strings.Contains(err.Error(), "payload hash") {
		t.Fatalf("UploadToS3() error = %v, want payload hash error", err)
	}

	err = UploadToS3(context.Background(), validCredentials, strings.NewReader("data"), testNotarySHA256, 0, "application/octet-stream")
	if err == nil || !strings.Contains(err.Error(), "content length") {
		t.Fatalf("UploadToS3() error = %v, want content length error", err)
	}
}

func TestUploadToS3_RejectsInvalidPayloadHashBeforeRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	creds := S3Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Bucket:          "bucket",
		Object:          "object",
	}
	err := UploadToS3(ctx, creds, strings.NewReader("data"), "not-a-sha256", 4, "application/octet-stream")
	if err == nil || !strings.Contains(err.Error(), "64 hexadecimal") {
		t.Fatalf("UploadToS3() error = %v, want strict SHA-256 validation error", err)
	}
}

func TestCompleteMultipartUploadClassifiesResponseBody(t *testing.T) {
	const diagnosticLimit = 200
	const omittedMarker = "OMITTED-DIAGNOSTIC-MARKER"

	tests := []struct {
		name            string
		body            string
		wantErrorPrefix string
		checkDiagnostic bool
	}{
		{
			name: "embedded error after keepalive whitespace and prolog",
			body: " \r\n\t<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
				"<Error><Code>InternalError</Code><Message>retry the upload</Message></Error>" +
				"\x1b" + strings.Repeat("x", diagnosticLimit) + omittedMarker,
			wantErrorPrefix: "complete multipart upload failed: ",
			checkDiagnostic: true,
		},
		{
			name: "normal completion result",
			body: "\n<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
				"<CompleteMultipartUploadResult xmlns=\"http://s3.amazonaws.com/doc/2006-03-01/\">" +
				"<Bucket>example</Bucket><Key>archive.zip</Key><ETag>\"etag\"</ETag>" +
				"</CompleteMultipartUploadResult>\n<!-- completed -->\n ",
		},
		{
			name:            "unexpected response root",
			body:            "<?xml version=\"1.0\"?><UnexpectedResult><Message>unknown</Message></UnexpectedResult>",
			wantErrorPrefix: "unexpected complete multipart upload response: ",
		},
		{
			name: "truncated completion result",
			body: "<?xml version=\"1.0\"?>" +
				"<CompleteMultipartUploadResult><Bucket>example</Bucket>",
			wantErrorPrefix: "parse complete multipart upload response: ",
		},
		{
			name: "second root after completion result",
			body: "<CompleteMultipartUploadResult></CompleteMultipartUploadResult>" +
				"<Error><Code>InternalError</Code></Error>",
			wantErrorPrefix: "parse complete multipart upload response: ",
		},
		{
			name: "non-whitespace data after completion result",
			body: "<CompleteMultipartUploadResult></CompleteMultipartUploadResult>" +
				"unexpected trailing data",
			wantErrorPrefix: "parse complete multipart upload response: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", req.Method)
				}
				if req.URL.Path != "/archive.zip" {
					t.Fatalf("path = %q, want /archive.zip", req.URL.Path)
				}
				if req.URL.Query().Get("uploadId") != "upload-123" {
					t.Fatalf("uploadId = %q, want upload-123", req.URL.Query().Get("uploadId"))
				}
				mustWriteBody(t, w, tt.body)
			}))
			t.Cleanup(server.Close)
			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse test server URL: %v", err)
			}

			err = completeMultipartUploadWithClient(
				context.Background(),
				server.Client(),
				serverURL.Host,
				"/archive.zip",
				S3Credentials{AccessKeyID: "key", SecretAccessKey: "secret"},
				"upload-123",
				[]s3CompletedPart{{PartNumber: 1, ETag: "\"etag\""}},
			)
			if tt.wantErrorPrefix == "" {
				if err != nil {
					t.Fatalf("completeMultipartUpload() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatal("completeMultipartUpload() error = nil, want response classification error")
			}
			if !strings.HasPrefix(err.Error(), tt.wantErrorPrefix) {
				t.Fatalf("error = %q, want prefix %q", err, tt.wantErrorPrefix)
			}
			if !tt.checkDiagnostic {
				return
			}

			const errorPrefix = "complete multipart upload failed: "
			diagnostic := strings.TrimPrefix(err.Error(), errorPrefix)
			if !strings.Contains(diagnostic, "<Code>InternalError</Code>") || !strings.Contains(diagnostic, "<Message>retry the upload</Message>") {
				t.Fatalf("diagnostic = %q, want S3 code and message", diagnostic)
			}
			if strings.Contains(diagnostic, "\x1b") {
				t.Fatalf("diagnostic contains control character: %q", diagnostic)
			}
			if strings.Contains(diagnostic, omittedMarker) {
				t.Fatalf("diagnostic contains content beyond limit: %q", diagnostic)
			}
			if len(diagnostic) > diagnosticLimit {
				t.Fatalf("diagnostic length = %d, want <= %d", len(diagnostic), diagnosticLimit)
			}
		})
	}
}

func TestCompleteMultipartUploadReportsPartialBodyReadError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "128")
		w.WriteHeader(http.StatusBadGateway)
		mustWriteBody(t, w, "<Error><Code>InternalError</Code>")
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	err = completeMultipartUploadWithClient(
		context.Background(),
		server.Client(),
		serverURL.Host,
		"/archive.zip",
		S3Credentials{AccessKeyID: "key", SecretAccessKey: "secret"},
		"upload-123",
		[]s3CompletedPart{{PartNumber: 1, ETag: "\"etag\""}},
	)
	if err == nil || !strings.Contains(err.Error(), "read complete multipart upload response") {
		t.Fatalf("completeMultipartUpload() error = %v, want response read error", err)
	}
}

func TestEncodeS3Query(t *testing.T) {
	values := url.Values{}
	values.Set("uploads", "")
	if got := encodeS3Query(values); got != "uploads=" {
		t.Fatalf("expected uploads=, got %q", got)
	}

	values = url.Values{}
	values.Add("partNumber", "2")
	values.Add("partNumber", "1")
	values.Set("uploadId", "foo:bar")
	got := encodeS3Query(values)
	want := "partNumber=1&partNumber=2&uploadId=foo%3Abar"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestCalculateMultipartPartSize(t *testing.T) {
	defaultSize := int64(notaryS3DefaultPartSizeBytes)
	size := calculateMultipartPartSize(notaryS3MaxSingleUploadBytes + 1)
	if size != defaultSize {
		t.Fatalf("expected default size %d, got %d", defaultSize, size)
	}

	huge := defaultSize*notaryS3MaxParts + 1
	size = calculateMultipartPartSize(huge)
	if size <= defaultSize {
		t.Fatalf("expected size > %d for huge payload, got %d", defaultSize, size)
	}
	if size*notaryS3MaxParts < huge {
		t.Fatalf("expected size to cover payload, got %d for %d bytes", size, huge)
	}
}

func TestNormalizeETag(t *testing.T) {
	if got := normalizeETag("\"abc\""); got != "\"abc\"" {
		t.Fatalf("expected quoted etag to be preserved, got %q", got)
	}
	if got := normalizeETag("abc"); got != "\"abc\"" {
		t.Fatalf("expected etag to be quoted, got %q", got)
	}
}

func TestSubmitNotarization_EmptyInputs(t *testing.T) {
	client := newTestNotaryClient(t, "")

	ctx := context.Background()

	_, err := client.SubmitNotarization(ctx, "", "name.zip")
	if err == nil {
		t.Fatal("expected error for empty sha256")
	}

	_, err = client.SubmitNotarization(ctx, testNotarySHA256, "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetNotarizationStatus_EmptyID(t *testing.T) {
	client := newTestNotaryClient(t, "")

	_, err := client.GetNotarizationStatus(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestGetNotarizationLogs_EmptyID(t *testing.T) {
	client := newTestNotaryClient(t, "")

	_, err := client.GetNotarizationLogs(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestNotarySubmissionStatusConstants(t *testing.T) {
	if NotaryStatusAccepted != "Accepted" {
		t.Errorf("unexpected Accepted value: %s", NotaryStatusAccepted)
	}
	if NotaryStatusInProgress != "In Progress" {
		t.Errorf("unexpected In Progress value: %s", NotaryStatusInProgress)
	}
	if NotaryStatusInvalid != "Invalid" {
		t.Errorf("unexpected Invalid value: %s", NotaryStatusInvalid)
	}
	if NotaryStatusRejected != "Rejected" {
		t.Errorf("unexpected Rejected value: %s", NotaryStatusRejected)
	}
}

func TestNotarySubmissionRequestJSON(t *testing.T) {
	req := NotarySubmissionRequest{
		Sha256:         testNotarySHA256,
		SubmissionName: "app.zip",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["sha256"] != testNotarySHA256 {
		t.Errorf("expected sha256 %s, got %s", testNotarySHA256, parsed["sha256"])
	}
	if parsed["submissionName"] != "app.zip" {
		t.Errorf("expected submissionName app.zip, got %s", parsed["submissionName"])
	}
}

func TestNotarySubmissionResponseJSON(t *testing.T) {
	raw := `{
		"data": {
			"type": "newSubmissions",
			"id": "sub-abc",
			"attributes": {
				"awsAccessKeyId": "AKID",
				"awsSecretAccessKey": "SECRET",
				"awsSessionToken": "TOKEN",
				"bucket": "my-bucket",
				"object": "my-object"
			}
		}
	}`

	var resp NotarySubmissionResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Data.ID != "sub-abc" {
		t.Errorf("unexpected ID: %s", resp.Data.ID)
	}
	if resp.Data.Type != "newSubmissions" {
		t.Errorf("unexpected type: %s", resp.Data.Type)
	}
	if resp.Data.Attributes.AwsAccessKeyID != "AKID" {
		t.Errorf("unexpected access key: %s", resp.Data.Attributes.AwsAccessKeyID)
	}
	if resp.Data.Attributes.AwsSecretAccessKey != "SECRET" {
		t.Errorf("unexpected secret: %s", resp.Data.Attributes.AwsSecretAccessKey)
	}
	if resp.Data.Attributes.AwsSessionToken != "TOKEN" {
		t.Errorf("unexpected token: %s", resp.Data.Attributes.AwsSessionToken)
	}
	if resp.Data.Attributes.Bucket != "my-bucket" {
		t.Errorf("unexpected bucket: %s", resp.Data.Attributes.Bucket)
	}
	if resp.Data.Attributes.Object != "my-object" {
		t.Errorf("unexpected object: %s", resp.Data.Attributes.Object)
	}
}

func TestNotaryStatusResponseJSON(t *testing.T) {
	raw := `{
		"data": {
			"id": "sub-status",
			"type": "submissions",
			"attributes": {
				"status": "Accepted",
				"name": "myapp.zip",
				"createdDate": "2026-01-15T10:30:00Z"
			}
		}
	}`

	var resp NotarySubmissionStatusResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Data.Attributes.Status != NotaryStatusAccepted {
		t.Errorf("unexpected status: %s", resp.Data.Attributes.Status)
	}
	if resp.Data.Attributes.Name != "myapp.zip" {
		t.Errorf("unexpected name: %s", resp.Data.Attributes.Name)
	}
	if resp.Data.Attributes.CreatedDate != "2026-01-15T10:30:00Z" {
		t.Errorf("unexpected date: %s", resp.Data.Attributes.CreatedDate)
	}
}

func TestNotaryLogsResponseJSON(t *testing.T) {
	raw := `{
		"data": {
			"id": "sub-log",
			"type": "submissionsLog",
			"attributes": {
				"developerLogUrl": "https://example.com/log.json"
			}
		}
	}`

	var resp NotarySubmissionLogsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Data.ID != "sub-log" {
		t.Errorf("unexpected ID: %s", resp.Data.ID)
	}
	if resp.Data.Type != "submissionsLog" {
		t.Errorf("unexpected type: %s", resp.Data.Type)
	}
	if resp.Data.Attributes.DeveloperLogURL != "https://example.com/log.json" {
		t.Errorf("unexpected log URL: %s", resp.Data.Attributes.DeveloperLogURL)
	}
}

func TestNotaryListResponseJSON(t *testing.T) {
	raw := `{
		"data": [
			{
				"id": "sub-1",
				"type": "submissions",
				"attributes": {
					"status": "Accepted",
					"name": "first.zip",
					"createdDate": "2026-01-10T08:00:00Z"
				}
			},
			{
				"id": "sub-2",
				"type": "submissions",
				"attributes": {
					"status": "Rejected",
					"name": "second.zip",
					"createdDate": "2026-01-12T12:00:00Z"
				}
			}
		]
	}`

	var resp NotarySubmissionsListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "sub-1" {
		t.Errorf("unexpected ID: %s", resp.Data[0].ID)
	}
	if resp.Data[0].Attributes.Status != NotaryStatusAccepted {
		t.Errorf("unexpected status: %s", resp.Data[0].Attributes.Status)
	}
	if resp.Data[1].Attributes.Status != NotaryStatusRejected {
		t.Errorf("unexpected status: %s", resp.Data[1].Attributes.Status)
	}
}

func TestResolveNotaryBaseURL(t *testing.T) {
	client := newTestNotaryClient(t, "")

	// Default should be NotaryBaseURL
	if got := client.resolveNotaryBaseURL(); got != NotaryBaseURL {
		t.Errorf("expected %s, got %s", NotaryBaseURL, got)
	}

	// Override
	client.SetNotaryBaseURL("http://localhost:9999")
	if got := client.resolveNotaryBaseURL(); got != "http://localhost:9999" {
		t.Errorf("expected http://localhost:9999, got %s", got)
	}
}
