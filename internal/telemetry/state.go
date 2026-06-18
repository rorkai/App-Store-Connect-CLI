package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	stateDirName       = ".asc"
	stateFileName      = "telemetry.json"
	lockTimeout        = 2 * time.Second
	lockPollInterval   = 10 * time.Millisecond
	legacyLockStaleAge = 30 * time.Second
	maxStateFileBytes  = 64 * 1024
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
	return ensureInstallID(lockTimeout)
}

func ensureInstallID(wait time.Duration) (string, error) {
	path, err := StatePath()
	if err != nil {
		return "", err
	}

	var installID string
	if err := updateStateWithLockTimeout(path, wait, func(st *State) error {
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
	file, err := openStateFileForRead(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("telemetry: failed to read state: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxStateFileBytes+1))
	if err != nil {
		return State{}, fmt.Errorf("telemetry: failed to read state: %w", err)
	}
	if len(data) > maxStateFileBytes {
		return State{}, fmt.Errorf("telemetry: state file is too large")
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
	return updateStateWithLockTimeout(path, lockTimeout, mutate)
}

func updateStateWithLockTimeout(path string, wait time.Duration, mutate func(*State) error) error {
	deadline := time.Now().Add(wait)
	unlock, err := lockState(path, wait)
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
	replaceWait := wait
	if wait > 0 {
		replaceWait = max(time.Duration(0), time.Until(deadline))
	}
	return saveState(path, st, replaceWait)
}

func lockState(path string, wait time.Duration) (func(), error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("telemetry: failed to create state directory: %w", err)
	}
	lockPath := path + ".lock"
	deadline := time.Now().Add(wait)
	for {
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			info, statErr := statStateLock(lockPath)
			if errors.Is(statErr, os.ErrNotExist) {
				if !waitForStateLockRetry(wait, deadline) {
					return nil, fmt.Errorf("telemetry: timed out locking state")
				}
				continue
			}
			if statErr != nil || !info.IsDir() {
				return nil, fmt.Errorf("telemetry: failed to open state lock: %w", err)
			}
			if time.Since(info.ModTime()) > legacyLockStaleAge {
				migrated, migrateErr := migrateLegacyStateLockDirectory(lockPath, info)
				if migrateErr != nil {
					return nil, fmt.Errorf("telemetry: failed to migrate legacy state lock: %w", migrateErr)
				}
				if migrated {
					continue
				}
			}
			if wait <= 0 || time.Now().After(deadline) {
				return nil, fmt.Errorf("telemetry: timed out locking state")
			}
			if !waitForStateLockRetry(wait, deadline) {
				return nil, fmt.Errorf("telemetry: timed out locking state")
			}
			continue
		}
		if wait > 0 && time.Until(deadline) <= 0 {
			_ = lockFile.Close()
			return nil, fmt.Errorf("telemetry: timed out locking state")
		}

		locked, lockErr := tryLockStateFile(lockFile)
		if lockErr != nil {
			_ = lockFile.Close()
			return nil, fmt.Errorf("telemetry: failed to lock state: %w", lockErr)
		}
		if locked {
			return func() {
				_ = unlockStateFile(lockFile)
				_ = lockFile.Close()
			}, nil
		}
		if wait <= 0 || time.Now().After(deadline) {
			_ = lockFile.Close()
			return nil, fmt.Errorf("telemetry: timed out locking state")
		}
		if !waitForStateLockRetry(wait, deadline) {
			_ = lockFile.Close()
			return nil, fmt.Errorf("telemetry: timed out locking state")
		}
		_ = lockFile.Close()
	}
}

func migrateLegacyStateLockDirectory(lockPath string, expected os.FileInfo) (bool, error) {
	markerPath := filepath.Join(lockPath, ".asc-migrating")
	marker, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			markerInfo, statErr := os.Stat(markerPath)
			if statErr == nil && time.Since(markerInfo.ModTime()) > legacyLockStaleAge {
				current, currentErr := statStateLock(lockPath)
				if currentErr != nil {
					if errors.Is(currentErr, os.ErrNotExist) {
						return true, nil
					}
					return false, currentErr
				}
				if os.SameFile(expected, current) {
					return quarantineLegacyStateLockDirectory(lockPath, markerPath)
				}
			}
			return false, nil
		}
		current, statErr := os.Stat(lockPath)
		if errors.Is(statErr, os.ErrNotExist) || statErr == nil && !current.IsDir() {
			return true, nil
		}
		return false, err
	}
	if closeErr := marker.Close(); closeErr != nil {
		_ = os.Remove(markerPath)
		return false, closeErr
	}

	current, err := statStateLock(lockPath)
	if err != nil {
		_ = os.Remove(markerPath)
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if !os.SameFile(expected, current) {
		_ = os.Remove(markerPath)
		return false, nil
	}

	return quarantineLegacyStateLockDirectory(lockPath, markerPath)
}

func quarantineLegacyStateLockDirectory(lockPath, markerPath string) (bool, error) {
	quarantinePath := lockPath + ".legacy-" + uuid.NewString()
	if err := os.Rename(lockPath, quarantinePath); err != nil {
		_ = os.Remove(markerPath)
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if err := os.Remove(filepath.Join(quarantinePath, filepath.Base(markerPath))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := removeLegacyStateLockDirectory(quarantinePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}

func statStateLock(path string) (os.FileInfo, error) {
	file, err := openStateLockForStat(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

func waitForStateLockRetry(wait time.Duration, deadline time.Time) bool {
	if wait <= 0 {
		return false
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return false
	}
	time.Sleep(min(lockPollInterval, remaining))
	return true
}

func saveState(path string, st State, wait time.Duration) error {
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
	if err := replaceStateFile(tmpPath, path, wait); err != nil {
		return fmt.Errorf("telemetry: failed to replace state: %w", err)
	}
	return nil
}

func enabledFromState(st State) (bool, string) {
	if reason := environmentOptOutReason(); reason != "" {
		return false, reason
	}
	if st.Disabled {
		return false, "state"
	}
	return true, ""
}

func environmentOptOutReason() string {
	if envTruthy("ASC_TELEMETRY_DISABLED") {
		return "ASC_TELEMETRY_DISABLED"
	}
	if envTruthy("DO_NOT_TRACK") {
		return "DO_NOT_TRACK"
	}
	return ""
}
