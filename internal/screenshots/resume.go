package screenshots

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// FrameResumeFingerprint is the render-affecting resume key.
type FrameResumeFingerprint struct {
	SourceHash    string
	Device        string
	Title         string
	Subtitle      string
	TitleColor    string
	SubtitleColor string
	Background    string
	OverlayHash   string
}

// FingerprintFrameResume hashes every input that changes the framed image.
func FingerprintFrameResume(fp FrameResumeFingerprint) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		fp.SourceHash,
		fp.Device,
		fp.Title,
		fp.Subtitle,
		fp.TitleColor,
		fp.SubtitleColor,
		fp.Background,
		fp.OverlayHash,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// ResumeSkip reports whether output can be skipped because its recorded
// fingerprint still matches and the framed file exists.
func ResumeSkip(state FrameResumeState, outputPath, fingerprint string) bool {
	if state.Files == nil || fingerprint == "" {
		return false
	}
	recorded, ok := state.Files[outputPath]
	if !ok || recorded != fingerprint {
		return false
	}
	info, err := os.Stat(outputPath)
	return err == nil && !info.IsDir()
}
