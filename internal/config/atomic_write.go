package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

var syncDirFn = syncDir

func writeConfigFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := ensureConfigDirPath(dir); err != nil {
		return err
	}
	if parentDir := filepath.Dir(dir); parentDir != dir {
		if err := syncDirFn(parentDir); err != nil {
			return fmt.Errorf("failed to sync config parent directory: %w", err)
		}
	}

	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink %q", path)
		}
		if info.IsDir() {
			return fmt.Errorf("config path %q is a directory", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tempFile, err := createTempConfigFile(dir, ".asc-config-*", 0o600)
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := syncDirFn(dir); err != nil {
		return fmt.Errorf("failed to sync config directory: %w", err)
	}

	if err := replaceConfigFile(tempPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

func ensureConfigDirPath(dir string) error {
	cleanDir := filepath.Clean(dir)
	absDir, err := filepath.Abs(cleanDir)
	if err != nil {
		return err
	}

	for _, component := range configDirComponents(absDir) {
		if err := ensureConfigDirComponent(component); err != nil {
			return err
		}
	}
	return nil
}

func configDirComponents(absDir string) []string {
	clean := filepath.Clean(absDir)
	volume := filepath.VolumeName(clean)
	remainder := clean[len(volume):]
	root := volume
	if strings.HasPrefix(remainder, string(filepath.Separator)) {
		root += string(filepath.Separator)
		remainder = strings.TrimPrefix(remainder, string(filepath.Separator))
	}
	if root == "" {
		root = string(filepath.Separator)
	}

	components := []string{root}
	current := root
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		components = append(components, current)
	}
	return components
}

func ensureConfigDirComponent(path string) error {
	err := validateConfigDirComponent(path)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return err
	}

	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return validateConfigDirComponent(path)
}

func validateConfigDirComponent(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if isAllowedConfigDirSymlink(path) {
			return nil
		}
		return fmt.Errorf("refusing to follow symlink component %q", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("config directory component %q is not a directory", path)
	}
	return nil
}

func isAllowedConfigDirSymlink(path string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	switch filepath.Clean(path) {
	case "/var", "/tmp", "/etc":
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return false
		}
		expected := filepath.Join("/private", strings.TrimPrefix(filepath.Clean(path), "/"))
		return filepath.Clean(resolved) == expected
	default:
		return false
	}
}

func createTempConfigFile(dir, pattern string, perm os.FileMode) (*os.File, error) {
	prefix := pattern
	suffix := ""
	if idx := strings.LastIndex(pattern, "*"); idx != -1 {
		prefix = pattern[:idx]
		suffix = pattern[idx+1:]
	}

	const maxAttempts = 10_000
	var randBytes [12]byte
	for i := 0; i < maxAttempts; i++ {
		if _, err := rand.Read(randBytes[:]); err != nil {
			return nil, err
		}

		name := prefix + hex.EncodeToString(randBytes[:]) + suffix
		file, err := secureopen.OpenNewFileNoFollow(filepath.Join(dir, name), perm)
		if err == nil {
			return file, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, err
	}

	return nil, fmt.Errorf("failed to create temporary config file in %q", dir)
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	if err := dir.Sync(); err != nil {
		if runtime.GOOS == "windows" {
			return nil
		}
		return err
	}
	return nil
}

func replaceConfigFile(tempPath, path string) error {
	dir := filepath.Dir(path)
	if err := os.Rename(tempPath, path); err == nil {
		if err := syncDirFn(dir); err != nil {
			return fmt.Errorf("failed to sync config directory: %w", err)
		}
		return nil
	} else if errors.Is(err, os.ErrNotExist) {
		return err
	}

	backupFile, err := createTempConfigFile(dir, ".asc-config-backup-*", 0o600)
	if err != nil {
		return err
	}

	backupPath := backupFile.Name()
	if closeErr := backupFile.Close(); closeErr != nil {
		return closeErr
	}
	if removeErr := os.Remove(backupPath); removeErr != nil {
		return removeErr
	}

	if err := os.Rename(path, backupPath); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return err
	}
	_ = os.Remove(backupPath)
	if err := syncDirFn(dir); err != nil {
		return fmt.Errorf("failed to sync config directory: %w", err)
	}
	return nil
}
