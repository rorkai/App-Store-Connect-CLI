package screenshots

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// FrameResumeState records source hashes for completed framed outputs.
type FrameResumeState struct {
	Files map[string]string `json:"files"`
}

// FrameResumeStateRel is the repo-local resume file.
const FrameResumeStateRel = ".asc/reports/screenshots-frame/state.json"

// HashFile returns the SHA-256 hex digest of path.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// LoadFrameResumeState reads state, returning an empty map when the file is
// missing.
func LoadFrameResumeState(path string) (FrameResumeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FrameResumeState{Files: map[string]string{}}, nil
		}
		return FrameResumeState{}, err
	}
	var state FrameResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return FrameResumeState{}, err
	}
	if state.Files == nil {
		state.Files = map[string]string{}
	}
	return state, nil
}

// SaveFrameResumeState writes state atomically enough for a local report.
func SaveFrameResumeState(path string, state FrameResumeState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if state.Files == nil {
		state.Files = map[string]string{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// ResumeSkip reports whether output can be skipped because its recorded
// source hash still matches inputHash.
func ResumeSkip(state FrameResumeState, outputPath, inputHash string) bool {
	if state.Files == nil || inputHash == "" {
		return false
	}
	recorded, ok := state.Files[outputPath]
	if !ok || recorded != inputHash {
		return false
	}
	info, err := os.Stat(outputPath)
	return err == nil && !info.IsDir()
}
