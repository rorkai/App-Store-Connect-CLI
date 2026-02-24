package shared

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteProfileFile writes provisioning profile data to disk securely.
func WriteProfileFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := OpenNewFileNoFollow(path, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file already exists: %w", err)
		}
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(content); err != nil {
		return err
	}
	return file.Sync()
}
