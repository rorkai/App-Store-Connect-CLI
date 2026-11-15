package storeassets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ReadAppClip reads only files below the selected metadata root. A canonical
// asset file expresses intent; empty folders left by exports do not.
func ReadAppClip(metadataDir string) (layout AppClipLayout, present bool, err error) {
	layout = AppClipLayout{Subtitles: map[string]string{}, HeaderImages: map[string]string{}}
	root, err := rootfs.New(metadataDir)
	if err != nil {
		return layout, false, err
	}
	keep := false
	defer func() {
		if !keep {
			root.Close()
		}
	}()
	entries, err := readDir(root, ".")
	if errors.Is(err, os.ErrNotExist) {
		return layout, false, nil
	}
	if err != nil {
		return layout, false, err
	}
	action, exists, err := readText(root, filepath.Join("app_clip", "action.txt"))
	if err != nil {
		return layout, present, err
	}
	if exists {
		action = strings.ToUpper(action)
		if action != "OPEN" && action != "PLAY" && action != "VIEW" {
			return layout, true, fmt.Errorf("app clip action must be OPEN, PLAY, or VIEW")
		}
		layout.Action = action
		present = true
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "app_clip" {
			continue
		}
		clipDir := filepath.Join(entry.Name(), "app_clip")
		_, exists, err := readDirOptional(root, clipDir)
		if err != nil {
			return layout, present, err
		}
		if !exists {
			continue
		}
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(entry.Name())
		if err != nil {
			return layout, true, err
		}
		if locale != entry.Name() {
			return layout, true, fmt.Errorf("app clip locale folder %q must use canonical locale %q", entry.Name(), locale)
		}
		subtitle, exists, err := readText(root, filepath.Join(clipDir, "subtitle.txt"))
		if err != nil {
			return layout, true, err
		}
		if exists {
			present = true
			layout.Subtitles[locale] = subtitle
		}
		header := filepath.Join(clipDir, "header_image.png")
		f, err := root.OpenFile(header)
		if err == nil {
			info, statErr := f.Stat()
			f.Close()
			if statErr != nil {
				return layout, true, statErr
			}
			if !info.Mode().IsRegular() {
				return layout, true, fmt.Errorf("header must be a regular file")
			}
			present = true
			layout.HeaderImages[locale] = filepath.Join(root.Path(), header)
		} else if !errors.Is(err, os.ErrNotExist) {
			return layout, true, err
		}
	}
	if present {
		layout.sourceRoot = root
		keep = true
	}
	return layout, present, nil
}

func ReadPreviews(base string) (previews []PreviewLayout, err error) {
	root, err := rootfs.New(base)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			root.Close()
		}
	}()
	locales, exists, err := readDirOptional(root, "app_previews")
	if err != nil || !exists {
		return nil, err
	}
	for _, localeEntry := range locales {
		if !localeEntry.IsDir() {
			if localeEntry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("preview locale may not be a symlink")
			}
			continue
		}
		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(localeEntry.Name())
		if err != nil {
			return nil, err
		}
		if locale != localeEntry.Name() {
			return nil, fmt.Errorf("preview locale folder %q must use canonical locale %q", localeEntry.Name(), locale)
		}
		localeDir := filepath.Join("app_previews", localeEntry.Name())
		devices, err := readDir(root, localeDir)
		if err != nil {
			return nil, err
		}
		seenDevices := map[string]bool{}
		for _, deviceEntry := range devices {
			if !deviceEntry.IsDir() {
				if deviceEntry.Type()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf("preview device may not be a symlink")
				}
				continue
			}
			device, err := assets.NormalizePreviewType(deviceEntry.Name())
			if err != nil {
				return nil, err
			}
			if seenDevices[device] {
				return nil, fmt.Errorf("duplicate preview device folder %s", device)
			}
			seenDevices[device] = true
			deviceDir := filepath.Join(localeDir, deviceEntry.Name())
			entries, err := readDir(root, deviceDir)
			if err != nil {
				return nil, err
			}
			var group []PreviewLayout
			names := map[string]bool{}
			for _, entry := range entries {
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext != ".mp4" && ext != ".mov" && ext != ".m4v" && ext != ".m3u8" {
					continue
				}
				if err := segment(entry.Name()); err != nil {
					return nil, err
				}
				name := filepath.Join(deviceDir, entry.Name())
				file, err := root.OpenFile(name)
				if err != nil {
					return nil, err
				}
				info, err := file.Stat()
				file.Close()
				if err != nil {
					return nil, err
				}
				if !info.Mode().IsRegular() || info.Size() == 0 {
					return nil, fmt.Errorf("preview %s must be a nonempty regular file", name)
				}
				preview := PreviewLayout{Locale: locale, DeviceType: strings.ToLower(device), FileName: entry.Name(), Path: filepath.Join(root.Path(), name), sourceRoot: root}
				if ext == ".m3u8" {
					if err := readPreviewReference(root, strings.TrimSuffix(name, filepath.Ext(name))+".preview.json", &preview); err != nil {
						return nil, err
					}
				}
				poster := filepath.Join(deviceDir, strings.TrimSuffix(preview.FileName, filepath.Ext(preview.FileName))+".poster_frame.txt")
				preview.PosterFrame, _, err = readText(root, poster)
				if err != nil {
					return nil, err
				}
				if preview.PosterFrame != "" && !assets.ValidPreviewFrameTimeCode(preview.PosterFrame) {
					return nil, fmt.Errorf("invalid poster frame %s; use HH:MM:SS:FF or HH:MM:SS.mmm", poster)
				}
				if names[preview.FileName] {
					return nil, fmt.Errorf("%s has more than one preview named %q", deviceDir, preview.FileName)
				}
				names[preview.FileName] = true
				group = append(group, preview)
			}
			if len(group) > 3 {
				return nil, fmt.Errorf("%s contains more than three preview files", deviceDir)
			}
			orderText, exists, err := readText(root, filepath.Join(deviceDir, "order.json"))
			if err != nil {
				return nil, err
			}
			if exists {
				var order []string
				if err := json.Unmarshal([]byte(orderText), &order); err != nil {
					return nil, fmt.Errorf("preview order.json: %w", err)
				}
				if len(order) != len(group) {
					return nil, fmt.Errorf("preview order.json must list every video exactly once")
				}
				byName := map[string]PreviewLayout{}
				for _, p := range group {
					byName[p.FileName] = p
				}
				ordered := make([]PreviewLayout, 0, len(group))
				for _, name := range order {
					p, ok := byName[name]
					if !ok {
						return nil, fmt.Errorf("preview order.json has unknown or repeated file %q", name)
					}
					delete(byName, name)
					ordered = append(ordered, p)
				}
				group = ordered
			}
			previews = append(previews, group...)
		}
	}
	if len(previews) > 0 {
		keep = true
	}
	return previews, nil
}

// PreviewsOutsideDeviceFolders lists, relative to base, the video files kept
// directly in a locale folder, which ReadPreviews does not read.
func PreviewsOutsideDeviceFolders(base string) ([]string, error) {
	root, err := rootfs.New(base)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	locales, _, err := readDirOptional(root, "app_previews")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, localeEntry := range locales {
		if !localeEntry.IsDir() {
			continue
		}
		localeDir := filepath.Join("app_previews", localeEntry.Name())
		entries, err := readDir(root, localeDir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if !entry.IsDir() && (ext == ".mp4" || ext == ".mov" || ext == ".m4v") {
				paths = append(paths, filepath.Join(localeDir, entry.Name()))
			}
		}
	}
	return paths, nil
}

// previewReference identifies a remote preview whose original video App Store
// Connect does not expose; exports save its HLS playlist beside this sidecar.
type previewReference struct {
	ID                 string `json:"id"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	SourceFileChecksum string `json:"sourceFileChecksum"`
}

func readPreviewReference(root rootfs.Root, name string, preview *PreviewLayout) error {
	text, exists, err := readText(root, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s is an App Store Connect streaming playlist without its %s sidecar; App Store Connect does not provide original preview videos, so replace it with the original video file", preview.Path, filepath.Base(name))
	}
	var ref previewReference
	if err := json.Unmarshal([]byte(text), &ref); err != nil {
		return fmt.Errorf("preview reference %s: %w", name, err)
	}
	ext := strings.ToLower(filepath.Ext(ref.FileName))
	if segment(ref.FileName) != nil || (ext != ".mp4" && ext != ".mov" && ext != ".m4v") || ref.ID == "" {
		return fmt.Errorf("preview reference %s needs an id and a video fileName", name)
	}
	preview.FileName, preview.checksum, preview.reference, preview.referenceID = ref.FileName, ref.SourceFileChecksum, true, ref.ID
	return nil
}

func readDir(root rootfs.Root, name string) ([]os.DirEntry, error) {
	f, err := root.OpenDir(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, err
}

func readDirOptional(root rootfs.Root, name string) ([]os.DirEntry, bool, error) {
	entries, err := readDir(root, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return entries, err == nil, err
}

func readText(root rootfs.Root, name string) (string, bool, error) {
	f, err := root.OpenFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return "", true, err
	}
	if len(data) > 1<<20 {
		return "", true, fmt.Errorf("asset metadata %s exceeds 1 MiB", name)
	}
	return strings.TrimSpace(string(data)), true, nil
}

func (clip *AppClipLayout) Close() {
	if clip != nil {
		clip.sourceRoot.Close()
	}
}

func ClosePreviews(previews []PreviewLayout) {
	if len(previews) > 0 {
		previews[0].sourceRoot.Close()
	}
}

func openSource(root rootfs.Root, path string) (*os.File, error) {
	relative, err := filepath.Rel(root.Path(), path)
	if err != nil {
		return nil, err
	}
	return root.OpenFile(relative)
}
