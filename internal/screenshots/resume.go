package screenshots

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type FrameResumeEntry struct {
	Fingerprint string      `json:"fingerprint"`
	Result      FrameResult `json:"result"`
}

// FrameResumeState records completed framed outputs.
type FrameResumeState struct {
	Files map[string]FrameResumeEntry `json:"files"`
}

const (
	// FrameResumeStateRel is the repo-local resume file.
	FrameResumeStateRel = ".asc/reports/screenshots-frame/state.json"

	maxFrameResumeStateBytes = 16 << 20
	frameResumeLockName      = ".asc-screenshots-frame.lock"
)

// HashFile returns the SHA-256 hex digest of path.
func HashFile(path string) (digest string, returnErr error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	defer func() {
		returnErr = errors.Join(returnErr, root.Close())
	}()
	artifact, err := inspectMatrixArtifactWithContext(context.Background(), root, root.Path(), filepath.Join(root.Path(), filepath.Base(absolute)))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(artifact.digest[:]), nil
}

// LoadFrameResumeState reads state through root, returning an empty map when
// the file is missing. The path must stay inside the operator-selected root.
func LoadFrameResumeState(root rootfs.Root, name string) (FrameResumeState, error) {
	data, err := root.ReadFileLimited(name, maxFrameResumeStateBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return FrameResumeState{Files: map[string]FrameResumeEntry{}}, nil
		}
		return FrameResumeState{}, err
	}
	var state FrameResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return FrameResumeState{}, err
	}
	if state.Files == nil {
		state.Files = map[string]FrameResumeEntry{}
	}
	return state, nil
}

// SaveFrameResumeState writes state beneath root without following a symlink
// at the destination.
func SaveFrameResumeState(root rootfs.Root, name string, state FrameResumeState) error {
	if state.Files == nil {
		state.Files = map[string]FrameResumeEntry{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxFrameResumeStateBytes {
		return fmt.Errorf("frame resume state exceeds the %d-byte size limit", maxFrameResumeStateBytes)
	}
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
const (
	// frameResumeSchema changes when framed output generation changes without
	// a corresponding Koubou pin bump.
	frameResumeSchema = "2"
)

func FingerprintFrameResume(fp FrameResumeFingerprint) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		frameResumeSchema,
		pinnedKoubouVersion,
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

// WithFrameResumeLock serializes resume-state read-modify-write for one
// working tree.
func WithFrameResumeLock(ctx context.Context, root rootfs.Root, fn func() error) (returnErr error) {
	if ctx == nil {
		return errors.New("matrix lock context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(frameResumeLockName), 0o755); err != nil {
		return err
	}
	release, err := acquireMatrixNamedLock(ctx, root, frameResumeLockName)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, release())
	}()
	return fn()
}

// ResumeEntry returns the stored frame result when the fingerprint still
// matches and the framed file exists.
func ResumeEntry(state FrameResumeState, outputPath, fingerprint string) (FrameResult, bool) {
	if state.Files == nil || fingerprint == "" {
		return FrameResult{}, false
	}
	entry, ok := state.Files[outputPath]
	if !ok || entry.Fingerprint != fingerprint {
		return FrameResult{}, false
	}
	info, err := os.Lstat(outputPath)
	if err != nil || !info.Mode().IsRegular() {
		return FrameResult{}, false
	}
	result := entry.Result
	result.Path = outputPath
	result.Skipped = true
	return result, true
}
