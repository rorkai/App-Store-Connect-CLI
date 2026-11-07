package uploadlock

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func useTestLock(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	previousDir, previousWait, previousPoll := lockDir, waitLimit, pollInterval
	lockDir = func() (string, error) { return dir, nil }
	pollInterval = time.Millisecond
	t.Cleanup(func() { lockDir, waitLimit, pollInterval = previousDir, previousWait, previousPoll })
	t.Setenv(EnvVar, "")
	return dir
}

func TestAcquireFailsWhenAnotherUploadHoldsTheLockPastTheWaitLimit(t *testing.T) {
	useTestLock(t)
	release, err := Acquire(context.Background(), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	waitLimit = 0
	var stderr bytes.Buffer
	if _, err := Acquire(context.Background(), &stderr); err == nil || !strings.Contains(err.Error(), "ASC_UPLOAD_LOCK=0") {
		t.Fatalf("second Acquire error = %v, want a held-lock error naming ASC_UPLOAD_LOCK=0", err)
	}
	if !strings.HasPrefix(stderr.String(), "Waiting for another asc upload to finish") {
		t.Fatalf("stderr = %q, want the waiting notice", stderr.String())
	}

	release()
	releaseAgain, err := Acquire(context.Background(), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	releaseAgain()
}

func TestAcquireSkipsTheLockWhenDisabled(t *testing.T) {
	useTestLock(t)
	release, err := Acquire(context.Background(), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()

	t.Setenv(EnvVar, "0")
	var stderr bytes.Buffer
	skip, err := Acquire(context.Background(), &stderr)
	if err != nil {
		t.Fatalf("Acquire with %s=0: %v", EnvVar, err)
	}
	skip()
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no waiting notice", stderr.String())
	}

	t.Setenv(EnvVar, "sometimes")
	if _, err := Acquire(context.Background(), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "ASC_UPLOAD_LOCK must be 0 or 1") {
		t.Fatalf("Acquire with an invalid %s error = %v", EnvVar, err)
	}
}
