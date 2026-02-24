package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		if req.Sha256 != "abc123def456" {
			t.Errorf("expected sha256 abc123def456, got %s", req.Sha256)
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
	defer func() { server.Close() }()

	client := newTestNotaryClient(t, server.URL)
	ctx := context.Background()

	resp, err := client.SubmitNotarization(ctx, "abc123def456", "MyApp.zip")
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
	defer func() { server.Close() }()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.SubmitNotarization(context.Background(), "abc123", "test.zip")
	if err == nil {
		t.Fatal("expected error for 403 response")
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
				if !strings.HasSuffix(r.URL.Path, "/notary/v2/submissions/sub-456") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				resp := NotarySubmissionStatusResponse{
					Data: NotarySubmissionStatusData{
						ID:   "sub-456",
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
			defer func() { server.Close() }()

			client := newTestNotaryClient(t, server.URL)
			resp, err := client.GetNotarizationStatus(context.Background(), "sub-456")
			if err != nil {
				t.Fatalf("GetNotarizationStatus() error: %v", err)
			}

			if resp.Data.Attributes.Status != tt.status {
				t.Errorf("expected status %s, got %s", tt.status, resp.Data.Attributes.Status)
			}
			if resp.Data.ID != "sub-456" {
				t.Errorf("expected ID sub-456, got %s", resp.Data.ID)
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
	defer func() { server.Close() }()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.GetNotarizationStatus(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGetNotarizationLogs_SendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/notary/v2/submissions/sub-789/logs") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := NotarySubmissionLogsResponse{
			Data: NotarySubmissionLogsData{
				ID:   "sub-789",
				Type: "submissionsLog",
				Attributes: NotarySubmissionLogsAttributes{
					DeveloperLogURL: "https://example.com/logs/sub-789.json",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, resp)
	}))
	defer func() { server.Close() }()

	client := newTestNotaryClient(t, server.URL)
	resp, err := client.GetNotarizationLogs(context.Background(), "sub-789")
	if err != nil {
		t.Fatalf("GetNotarizationLogs() error: %v", err)
	}

	if resp.Data.Attributes.DeveloperLogURL != "https://example.com/logs/sub-789.json" {
		t.Errorf("unexpected log URL: %s", resp.Data.Attributes.DeveloperLogURL)
	}
	if resp.Data.ID != "sub-789" {
		t.Errorf("unexpected ID: %s", resp.Data.ID)
	}
}

func TestGetNotarizationLogs_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustWriteBody(t, w, `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"Logs not available"}]}`)
	}))
	defer func() { server.Close() }()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.GetNotarizationLogs(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 response")
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
	defer func() { server.Close() }()

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
	defer func() { server.Close() }()

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
	defer func() { server.Close() }()

	client := newTestNotaryClient(t, server.URL)
	_, err := client.ListNotarizations(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestComputeFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ComputeFileSHA256(path)
	if err != nil {
		t.Fatalf("ComputeFileSHA256() error: %v", err)
	}

	// Expected SHA-256 of "hello world"
	h := sha256.Sum256(content)
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestComputeFileSHA256_FileNotFound(t *testing.T) {
	_, err := ComputeFileSHA256("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestComputeFileSHA256_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ComputeFileSHA256(path)
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
	// Empty credentials
	err := UploadToS3(context.Background(), S3Credentials{}, strings.NewReader("data"), "hash", 4, "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for empty credentials")
	}

	// Empty bucket
	err = UploadToS3(context.Background(), S3Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Object:          "obj",
	}, strings.NewReader("data"), "hash", 4, "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for empty bucket")
	}

	// Empty object
	err = UploadToS3(context.Background(), S3Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "bucket",
	}, strings.NewReader("data"), "hash", 4, "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for empty object")
	}

	// Empty payload hash
	err = UploadToS3(context.Background(), S3Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "bucket",
		Object:          "object",
	}, strings.NewReader("data"), "", 4, "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for empty payload hash")
	}

	// Invalid content length
	err = UploadToS3(context.Background(), S3Credentials{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "bucket",
		Object:          "object",
	}, strings.NewReader("data"), "hash", 0, "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for invalid content length")
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

	_, err = client.SubmitNotarization(ctx, "abc123", "")
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
		Sha256:         "deadbeef",
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

	if parsed["sha256"] != "deadbeef" {
		t.Errorf("expected sha256 deadbeef, got %s", parsed["sha256"])
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
