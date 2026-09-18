package migrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	appClipDirName      = "app_clip"
	appClipActionFile   = "action.txt"
	appClipSubtitleFile = "subtitle.txt"
	appClipHeaderFile   = "header_image.png"
	appPreviewsDirName  = "app_previews"
)

type AppClipLayout struct {
	Action       string            `json:"action,omitempty"`
	Subtitles    map[string]string `json:"subtitles,omitempty"`
	HeaderImages map[string]string `json:"headerImages,omitempty"`
}

type PreviewLayout struct {
	Locale      string `json:"locale"`
	DeviceType  string `json:"deviceType"`
	FileName    string `json:"fileName"`
	Path        string `json:"path"`
	PosterFrame string `json:"posterFrame,omitempty"`
}

func readAppClipLayout(metadataDir string) (AppClipLayout, bool, error) {
	layout := AppClipLayout{Subtitles: map[string]string{}, HeaderImages: map[string]string{}}
	actionPath := filepath.Join(metadataDir, appClipDirName, appClipActionFile)
	if action, err := readOptionalText(actionPath); err != nil {
		return layout, false, err
	} else if action != "" {
		normalized := strings.ToUpper(strings.TrimSpace(action))
		if normalized != "OPEN" && normalized != "PLAY" && normalized != "VIEW" {
			return layout, true, fmt.Errorf("app clip action must be OPEN, PLAY, or VIEW")
		}
		layout.Action = normalized
	}
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return layout, false, nil
		}
		return layout, false, err
	}
	present := layout.Action != ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subtitlePath := filepath.Join(metadataDir, entry.Name(), appClipDirName, appClipSubtitleFile)
		headerPath := filepath.Join(metadataDir, entry.Name(), appClipDirName, appClipHeaderFile)
		subtitle, err := readOptionalText(subtitlePath)
		if err != nil {
			return layout, present, err
		}
		if subtitle != "" {
			layout.Subtitles[entry.Name()] = subtitle
			present = true
		}
		if info, statErr := os.Stat(headerPath); statErr == nil && info.Mode().IsRegular() {
			layout.HeaderImages[entry.Name()] = headerPath
			present = true
		}
	}
	return layout, present, nil
}

func readPreviewLayout(root string) ([]PreviewLayout, error) {
	dir := filepath.Join(root, appPreviewsDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var previews []PreviewLayout
	for _, locale := range entries {
		if !locale.IsDir() {
			continue
		}
		devices, err := os.ReadDir(filepath.Join(dir, locale.Name()))
		if err != nil {
			return nil, err
		}
		for _, device := range devices {
			if !device.IsDir() {
				continue
			}
			files, err := os.ReadDir(filepath.Join(dir, locale.Name(), device.Name()))
			if err != nil {
				return nil, err
			}
			for _, file := range files {
				name := file.Name()
				ext := strings.ToLower(filepath.Ext(name))
				if ext != ".mp4" && ext != ".mov" {
					continue
				}
				item := PreviewLayout{Locale: locale.Name(), DeviceType: device.Name(), FileName: name, Path: filepath.Join(dir, locale.Name(), device.Name(), name)}
				poster := strings.TrimSuffix(item.Path, ext) + ".poster_frame.txt"
				frame, err := readOptionalText(poster)
				if err != nil {
					return nil, err
				}
				if frame != "" && !validPosterFrame(frame) {
					return nil, fmt.Errorf("invalid poster frame %s", poster)
				}
				item.PosterFrame = frame
				previews = append(previews, item)
			}
		}
	}
	sort.Slice(previews, func(i, j int) bool {
		if previews[i].Locale != previews[j].Locale {
			return previews[i].Locale < previews[j].Locale
		}
		if previews[i].DeviceType != previews[j].DeviceType {
			return previews[i].DeviceType < previews[j].DeviceType
		}
		return previews[i].FileName < previews[j].FileName
	})
	return previews, nil
}

func writeAppClipLayout(metadataDir string, layout AppClipLayout) error {
	if layout.Action != "" {
		if err := writeOptionalText(filepath.Join(metadataDir, appClipDirName, appClipActionFile), layout.Action); err != nil {
			return err
		}
	}
	for locale, subtitle := range layout.Subtitles {
		if err := writeOptionalText(filepath.Join(metadataDir, locale, appClipDirName, appClipSubtitleFile), subtitle); err != nil {
			return err
		}
	}
	for locale, source := range layout.HeaderImages {
		dest := filepath.Join(metadataDir, locale, appClipDirName, appClipHeaderFile)
		if err := copyRegularFile(source, dest); err != nil {
			return err
		}
	}
	return nil
}

func writePreviewLayout(root string, previews []PreviewLayout) error {
	for _, preview := range previews {
		destDir := filepath.Join(root, appPreviewsDirName, preview.Locale, preview.DeviceType)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		if err := copyRegularFile(preview.Path, filepath.Join(destDir, preview.FileName)); err != nil {
			return err
		}
		if preview.PosterFrame != "" {
			poster := strings.TrimSuffix(preview.FileName, filepath.Ext(preview.FileName)) + ".poster_frame.txt"
			if err := writeOptionalText(filepath.Join(destDir, poster), preview.PosterFrame); err != nil {
				return err
			}
		}
	}
	return nil
}

func missingAppClipWarning(folderExists bool) *SkippedItem {
	if folderExists {
		return nil
	}
	return &SkippedItem{Path: appClipDirName, Reason: "app has no App Clip; no app_clip folder to import"}
}

func readOptionalText(path string) (string, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeOptionalText(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := shared.OpenNewFileNoFollow(path, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteString(content + "\n"); err != nil {
		return err
	}
	return file.Sync()
}

func copyRegularFile(source, dest string) error {
	input, err := shared.OpenExistingNoFollow(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	output, err := shared.OpenNewFileNoFollow(dest, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}

func validPosterFrame(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 3 && len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
