//go:build darwin || linux

package uploadlock

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

const holderDirEnv = "ASC_UPLOADLOCK_TEST_HOLDER_DIR"

// TestHelperHoldUploadLock is the other asc process: it takes the lock, says
// so on stdout, and holds it until it is killed.
func TestHelperHoldUploadLock(t *testing.T) {
	dir := os.Getenv(holderDirEnv)
	if dir == "" {
		t.Skip("helper process for TestAcquireWaitsForAnotherProcessAndTakesTheLockWhenItDies")
	}
	lockDir = func() (string, error) { return dir, nil }
	if _, err := Acquire(context.Background(), os.Stderr); err != nil {
		t.Fatalf("helper Acquire: %v", err)
	}
	_, _ = os.Stdout.WriteString("locked\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestDefaultLockDirIsSharedAcrossHOMEValues(t *testing.T) {
	want, err := defaultLockDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	got, err := defaultLockDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("lock dir with another HOME = %q, want %q", got, want)
	}
}

type notifyingWriter struct {
	once    sync.Once
	written chan struct{}
	mu      sync.Mutex
	text    strings.Builder
}

func (w *notifyingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.text.Write(p)
	w.mu.Unlock()
	w.once.Do(func() { close(w.written) })
	return len(p), nil
}

func TestAcquireWaitsForAnotherProcessAndTakesTheLockWhenItDies(t *testing.T) {
	dir := useTestLock(t)

	holder := exec.Command(os.Args[0], "-test.run=^TestHelperHoldUploadLock$")
	holder.Env = append(os.Environ(), holderDirEnv+"="+dir)
	holderStdin, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer holderStdin.Close()
	holderStdout, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Process.Kill(); _ = holder.Wait() }()
	line, err := bufio.NewReader(holderStdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("holder said %q, %v; want locked", line, err)
	}

	stderr := &notifyingWriter{written: make(chan struct{})}
	type result struct {
		release func()
		err     error
	}
	acquired := make(chan result, 1)
	go func() {
		release, err := Acquire(context.Background(), stderr)
		acquired <- result{release, err}
	}()

	select {
	case got := <-acquired:
		t.Fatalf("Acquire returned while another process held the lock: %v", got.err)
	case <-stderr.written:
	}
	stderr.mu.Lock()
	notice := stderr.text.String()
	stderr.mu.Unlock()
	if !strings.HasPrefix(notice, "Waiting for another asc upload to finish") {
		t.Fatalf("stderr = %q, want the waiting notice", notice)
	}

	// A crashed holder never calls release; the kernel drops its lock.
	if err := holder.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	got := <-acquired
	if got.err != nil {
		t.Fatalf("Acquire after the holder died: %v", got.err)
	}
	got.release()
}
