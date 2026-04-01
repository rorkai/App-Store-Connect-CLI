package threads

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type (
	Role string
	Kind string
)

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"

	KindMessage Kind = "message"
	KindStatus  Kind = "status"
)

type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Kind      Kind      `json:"kind"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type Thread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	SessionID string    `json:"sessionId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Messages  []Message `json:"messages"`
}

type Store struct {
	mu       sync.RWMutex
	path     string
	readFile func(string) ([]byte, error)
	write    func(string, []byte, os.FileMode) error
	mkdirAll func(string, os.FileMode) error
}

func NewStore(root string) *Store {
	return &Store{
		path:     filepath.Join(root, "threads.json"),
		readFile: os.ReadFile,
		write:    os.WriteFile,
		mkdirAll: os.MkdirAll,
	}
}

func (s *Store) LoadAll() ([]Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadAllUnlocked()
}

func (s *Store) loadAllUnlocked() ([]Thread, error) {
	data, err := s.readFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Thread{}, nil
		}
		return nil, fmt.Errorf("read thread store: %w", err)
	}

	var threads []Thread
	if err := json.Unmarshal(data, &threads); err != nil {
		return nil, fmt.Errorf("decode thread store: %w", err)
	}

	sort.SliceStable(threads, func(i, j int) bool {
		return threads[i].UpdatedAt.After(threads[j].UpdatedAt)
	})

	return threads, nil
}

func (s *Store) Get(id string) (Thread, error) {
	all, err := s.LoadAll()
	if err != nil {
		return Thread{}, err
	}
	for _, thread := range all {
		if thread.ID == id {
			return thread, nil
		}
	}
	return Thread{}, fmt.Errorf("thread %q not found", id)
}

func (s *Store) SaveThread(next Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllUnlocked()
	if err != nil {
		return err
	}

	replaced := false
	for index, thread := range all {
		if thread.ID == next.ID {
			all[index] = next
			replaced = true
			break
		}
	}
	if !replaced {
		all = append(all, next)
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode thread store: %w", err)
	}
	data = append(data, '\n')
	if err := s.mkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create thread directory: %w", err)
	}
	if err := s.write(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write thread store: %w", err)
	}
	return nil
}
