package environment

import (
	"errors"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestSnapshotKeepsLoadingWhenKeychainProbeFails(t *testing.T) {
	originalConfigPathFunc := configPathFunc
	originalConfigLoadFunc := configLoadFunc
	originalKeychainAvailableFunc := keychainAvailableFunc
	originalGetwdFunc := getwdFunc
	t.Cleanup(func() {
		configPathFunc = originalConfigPathFunc
		configLoadFunc = originalConfigLoadFunc
		keychainAvailableFunc = originalKeychainAvailableFunc
		getwdFunc = originalGetwdFunc
	})

	configPathFunc = func() (string, error) {
		return "/tmp/.asc/config.json", nil
	}
	configLoadFunc = func() (*config.Config, error) {
		return nil, config.ErrNotFound
	}
	keychainAvailableFunc = func() (bool, error) {
		return false, errors.New("keychain temporarily locked")
	}
	getwdFunc = func() (string, error) {
		return "/tmp/workspace", nil
	}

	snapshot, err := NewService().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.KeychainAvailable {
		t.Fatal("snapshot.KeychainAvailable = true, want false when probe fails")
	}
	if snapshot.KeychainWarning != "keychain temporarily locked" {
		t.Fatalf("snapshot.KeychainWarning = %q, want keychain temporarily locked", snapshot.KeychainWarning)
	}
	if snapshot.WorkflowPath != "/tmp/workspace/.asc/workflow.json" {
		t.Fatalf("snapshot.WorkflowPath = %q, want /tmp/workspace/.asc/workflow.json", snapshot.WorkflowPath)
	}
}
