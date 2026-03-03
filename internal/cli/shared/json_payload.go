package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadJSONFilePayload loads a JSON object from a file path for commands that
// accept raw payload documents.
func ReadJSONFilePayload(path string) (json.RawMessage, error) {
	file, err := openJSONPayloadFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // Best effort close for read-only file

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("payload path must be a file")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("payload file is empty")
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return json.RawMessage(data), nil
}

func openJSONPayloadFile(path string) (*os.File, error) {
	file, err := OpenExistingNoFollow(path)
	if err == nil {
		return file, nil
	}

	info, statErr := os.Lstat(path)
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		return nil, err
	}

	// Keep compatibility with legacy command behavior: allow symlinked payload files.
	resolvedPath, resolveErr := filepath.EvalSymlinks(path)
	if resolveErr != nil {
		return nil, resolveErr
	}

	return OpenExistingNoFollow(resolvedPath)
}
