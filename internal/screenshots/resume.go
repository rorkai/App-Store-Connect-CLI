package screenshots

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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

// LoadFrameResumeState reads state through root, returning an empty map when
// the file is missing. The path must stay inside the operator-selected root.
func LoadFrameResumeState(root rootfs.Root, name string) (FrameResumeState, error) {
	data, found, err := root.ReadFileOptional(name)
	if err != nil {
		return FrameResumeState{}, err
	}
	if !found {
		return FrameResumeState{Files: map[string]string{}}, nil
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

// SaveFrameResumeState writes state beneath root without following a symlink
// at the destination.
func SaveFrameResumeState(root rootfs.Root, name string, state FrameResumeState) error {
	if state.Files == nil {
		state.Files = map[string]string{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return root.WriteFile(name, data, 0o644)
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
