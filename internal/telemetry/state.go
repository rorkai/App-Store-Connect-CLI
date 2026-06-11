package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	stateDirName  = ".asc"
	stateFileName = "telemetry.json"
	lockTimeout   = 2 * time.Second
	staleLockAge  = 30 * time.Second
)

type State struct {
	InstallID string `json:"install_id,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type Status struct {
	Path      string `json:"path"`
	Enabled   bool   `json:"enabled"`
	InstallID string `json:"install_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}

func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("telemetry: failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, stateDirName, stateFileName), nil
}

func ReadStatus() (Status, error) {
	path, err := StatePath()
	if err != nil {
		return Status{}, err
	}
	st, err := loadState(path)
	if err != nil {
		return Status{}, err
	}
	enabled, reason := enabledFromState(st)
	return Status{
		Path:      path,
		Enabled:   enabled,
		InstallID: st.InstallID,
		Reason:    reason,
		Endpoint:  endpoint(),
	}, nil
}

func EnsureInstallID() (string, error) {
	path, err := StatePath()
	if err != nil {
		return "", err
	}

	var installID string
	if err := updateState(path, func(st *State) error {
		if strings.TrimSpace(st.InstallID) == "" {
			st.InstallID = uuid.NewString()
		}
		installID = st.InstallID
		return nil
	}); err != nil {
		return "", err
	}
	return installID, nil
}

func SetEnabled(enabled bool) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	return updateState(path, func(st *State) error {
		st.Disabled = !enabled
		return nil
	})
}

func ResetInstallID() (string, error) {
	path, err := StatePath()
	if err != nil {
		return "", err
	}

	var installID string
	if err := updateState(path, func(st *State) error {
		st.InstallID = uuid.NewString()
		installID = st.InstallID
		return nil
	}); err != nil {
		return "", err
	}
	return installID, nil
}

func loadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("telemetry: failed to read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("telemetry: failed to parse state: %w", err)
	}
	if strings.TrimSpace(st.InstallID) != "" {
		if _, err := uuid.Parse(st.InstallID); err != nil {
			st.InstallID = ""
		}
	}
	return st, nil
}

func updateState(path string, mutate func(*State) error) error {
	unlock, err := lockState(path)
	if err != nil {
		return err
	}
	defer unlock()

	st, err := loadState(path)
	if err != nil {
		return err
	}
	before := st
	if err := mutate(&st); err != nil {
		return err
	}
	if st == before {
		return nil
	}
	return saveState(path, st)
}

func lockState(path string) (func(), error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("telemetry: failed to create state directory: %w", err)
	}
	lockPath := path + ".lock"
	deadline := time.Now().Add(lockTimeout)
	for {
		if err := os.Mkdir(lockPath, 0o700); err == nil {
			return func() { _ = os.Remove(lockPath) }, nil
		} else if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("telemetry: failed to lock state: %w", err)
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			if removeErr := os.Remove(lockPath); removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("telemetry: timed out locking state")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func saveState(path string, st State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("telemetry: failed to create state directory: %w", err)
	}
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("telemetry: failed to encode state: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, stateFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("telemetry: failed to create temp state: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("telemetry: failed to write state: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("telemetry: failed to set state permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("telemetry: failed to close state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("telemetry: failed to replace state: %w", err)
	}
	return nil
}

func enabledFromState(st State) (bool, string) {
	if envTruthy("ASC_TELEMETRY_DISABLED") {
		return false, "ASC_TELEMETRY_DISABLED"
	}
	if envTruthy("DO_NOT_TRACK") {
		return false, "DO_NOT_TRACK"
	}
	if st.Disabled {
		return false, "state"
	}
	return true, ""
}
