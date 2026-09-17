package screenshots

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// LoadOverlayConfig reads and validates an overlay JSON file.
func LoadOverlayConfig(path string) (OverlayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OverlayConfig{}, fmt.Errorf("read overlay config: %w", err)
	}
	var config OverlayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return OverlayConfig{}, fmt.Errorf("parse overlay config: %w", err)
	}
	return config, nil
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
