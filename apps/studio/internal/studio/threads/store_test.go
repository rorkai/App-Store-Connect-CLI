package threads

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSaveThreadRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	now := time.Now().UTC()
	thread := Thread{
		ID:        "thread-1",
		Title:     "Release Prep",
		CreatedAt: now,
		UpdatedAt: now,
		Messages: []Message{
			{ID: "msg-1", Role: RoleUser, Kind: KindMessage, Content: "Validate 2.3.0", CreatedAt: now},
		},
	}

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread() error = %v", err)
	}

	got, err := store.Get("thread-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Title != thread.Title {
		t.Fatalf("Title = %q, want %q", got.Title, thread.Title)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(got.Messages))
	}
}

func TestSaveThreadUsesOwnerOnlyPermissions(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Now().UTC()

	if err := store.SaveThread(Thread{
		ID:        "thread-1",
		Title:     "Release Prep",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("SaveThread() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(root, "threads.json"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("permissions = %#o, want 0o600", got)
	}
}

func TestSaveThreadSerializesConcurrentUpdates(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	originalReadFile := store.readFile
	var readCalls atomic.Int32
	firstReadEntered := make(chan struct{})
	allowFirstRead := make(chan struct{})
	secondReadEntered := make(chan struct{})

	store.readFile = func(path string) ([]byte, error) {
		switch readCalls.Add(1) {
		case 1:
			close(firstReadEntered)
			<-allowFirstRead
		case 2:
			close(secondReadEntered)
		}
		return originalReadFile(path)
	}

	now := time.Now().UTC()
	threadOne := Thread{ID: "thread-1", Title: "One", CreatedAt: now, UpdatedAt: now}
	threadTwo := Thread{ID: "thread-2", Title: "Two", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- store.SaveThread(threadOne)
	}()

	select {
	case <-firstReadEntered:
	case <-time.After(time.Second):
		t.Fatal("first SaveThread() did not reach readFile")
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- store.SaveThread(threadTwo)
	}()

	select {
	case <-secondReadEntered:
		t.Fatal("second SaveThread() reached readFile before the first update completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(allowFirstRead)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("SaveThread() error = %v", err)
		}
	}

	all, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(LoadAll()) = %d, want 2", len(all))
	}

	seen := map[string]bool{}
	for _, thread := range all {
		seen[thread.ID] = true
	}
	if !seen["thread-1"] || !seen["thread-2"] {
		t.Fatalf("stored thread IDs = %#v, want both thread-1 and thread-2", seen)
	}
}
