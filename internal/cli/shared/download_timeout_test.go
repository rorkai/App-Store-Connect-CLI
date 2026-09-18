package shared

import (
	"context"
	"testing"
	"time"
)

func TestContextWithDownloadTimeoutUsesUploadBudget(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "30s")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "5m")

	downloadCtx, cancel := ContextWithDownloadTimeout(context.Background())
	defer cancel()
	deadline, ok := downloadCtx.Deadline()
	if !ok {
		t.Fatal("download context should have a deadline")
	}

	requestCtx, requestCancel := ContextWithTimeout(context.Background())
	defer requestCancel()
	requestDeadline, ok := requestCtx.Deadline()
	if !ok {
		t.Fatal("request context should have a deadline")
	}
	if !deadline.After(requestDeadline.Add(time.Minute)) {
		t.Fatalf("download deadline %s should outlast request deadline %s", deadline, requestDeadline)
	}
}
