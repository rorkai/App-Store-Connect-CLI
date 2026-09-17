package screenshots

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// OverlayEntry is one title and keyword overlay.
type OverlayEntry struct {
	Filter     string `json:"filter,omitempty"`
	Title      string `json:"title,omitempty"`
	Keyword    string `json:"keyword,omitempty"`
	Background string `json:"background,omitempty"`
}

// OverlayConfig is the --overlay-config schema.
type OverlayConfig struct {
	Default OverlayEntry   `json:"default"`
	Data    []OverlayEntry `json:"data"`
}

// LoadOverlayConfig reads and validates an overlay JSON file without following
// a symlink at the final path. The returned hash is of those exact bytes.
func LoadOverlayConfig(path string) (OverlayConfig, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return OverlayConfig{}, "", fmt.Errorf("read overlay config: %w", err)
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return OverlayConfig{}, "", fmt.Errorf("read overlay config: %w", err)
	}
	defer root.Close()
	data, err := root.ReadFile(filepath.Base(absolute))
	if err != nil {
		return OverlayConfig{}, "", fmt.Errorf("read overlay config: %w", err)
	}
	config, err := ParseOverlayConfig(data)
	if err != nil {
		return OverlayConfig{}, "", err
	}
	sum := sha256.Sum256(data)
	return config, hex.EncodeToString(sum[:]), nil
}

// ParseOverlayConfig validates overlay JSON bytes.
func ParseOverlayConfig(data []byte) (OverlayConfig, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config OverlayConfig
	if err := decoder.Decode(&config); err != nil {
		return OverlayConfig{}, fmt.Errorf("parse overlay config: %w", err)
	}
	var trailing struct{}
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return config, nil
	}
	if err != nil {
		return OverlayConfig{}, fmt.Errorf("parse overlay config: %w", err)
	}
	return OverlayConfig{}, fmt.Errorf("parse overlay config: unexpected trailing value")
}

// MatchOverlay selects the first data entry whose filter is a substring of
// name, otherwise the default entry.
func MatchOverlay(config OverlayConfig, name string) OverlayEntry {
	base := strings.ToLower(filepath.Base(name))
	for _, entry := range config.Data {
		filter := strings.ToLower(strings.TrimSpace(entry.Filter))
		if filter != "" && strings.Contains(base, filter) {
			return entry
		}
	}
	return config.Default
}

// OverlayToCanvas converts a matched overlay into canvas options.
func OverlayToCanvas(entry OverlayEntry) CanvasOptions {
	return CanvasOptions{
		Title:    strings.TrimSpace(entry.Title),
		Subtitle: strings.TrimSpace(entry.Keyword),
		BGColor:  strings.TrimSpace(entry.Background),
	}
}
