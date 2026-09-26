package asc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesNotarySubmissionsButAllowsStatusReads(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("mutating request left the notary client: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"submissions","id":"` + testSubmissionID + `","attributes":{"status":"Accepted"}}}`))
	}))
	t.Cleanup(server.Close)
	client := newTestNotaryClient(t, server.URL)

	_, err := client.SubmitNotarization(context.Background(), testNotarySHA256, "app.zip")
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("SubmitNotarization() error = %v, want readonly.ErrRefused", err)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("server received %d requests after refused submission, want 0", got)
	}

	if _, err := client.GetNotarizationStatus(context.Background(), testSubmissionID); err != nil {
		t.Fatalf("GetNotarizationStatus() error = %v, want nil", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("server received %d requests after GET, want 1", got)
	}
}

func TestReadOnlyModeRefusesNotaryS3UploadBeforeSigning(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	creds := S3Credentials{
		AccessKeyID:     "AKIA_TEST",
		SecretAccessKey: "secret",
		SessionToken:    "token",
		Bucket:          "notary-submissions-test",
		Object:          "prod/abc/app.zip",
	}
	err := UploadToS3(context.Background(), creds, strings.NewReader("payload"), sha256Hex([]byte("payload")), 7, "application/zip")
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("UploadToS3() error = %v, want readonly.ErrRefused", err)
	}
	if !strings.Contains(err.Error(), "refusing PUT https://notary-submissions-test.s3.") {
		t.Fatalf("UploadToS3() error = %q, want the refused S3 PUT target", err.Error())
	}
}
