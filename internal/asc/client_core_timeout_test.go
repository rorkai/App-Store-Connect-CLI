package asc

import (
	"testing"
	"time"
)

func TestResolveUploadTimeout_DefaultIsFiveMinutes(t *testing.T) {
	t.Setenv("ASC_UPLOAD_TIMEOUT", "")
	t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

	if got := ResolveUploadTimeout(); got != 300*time.Second {
		t.Fatalf("ResolveUploadTimeout() = %s, want 5m0s", got)
	}
}

func TestResolveUploadTimeout_UsesUploadTimeoutEnv(t *testing.T) {
	t.Setenv("ASC_UPLOAD_TIMEOUT", "17s")
	t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

	if got := ResolveUploadTimeout(); got != 17*time.Second {
		t.Fatalf("ResolveUploadTimeout() = %s, want 17s", got)
	}
}
