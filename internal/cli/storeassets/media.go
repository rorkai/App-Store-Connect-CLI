package storeassets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Validate snapshots each selected media file before inspecting it. Uploads use
// the snapshot, so a repository path swap after validation cannot change bytes.
func Validate(ctx context.Context, clip *AppClipLayout, previews []PreviewLayout) (cleanup func(), err error) {
	var dirs []string
	cleanup = func() {
		for _, dir := range dirs {
			os.RemoveAll(dir)
		}
	}
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	stage := func(file *os.File, name string, max int64) (string, error) {
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > max {
			return "", fmt.Errorf("%s must be a nonempty regular file of at most %d bytes", name, max)
		}
		dir, err := os.MkdirTemp("", "asc-store-import-*")
		if err != nil {
			return "", err
		}
		dirs = append(dirs, dir)
		path := filepath.Join(dir, filepath.Base(name))
		out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return "", err
		}
		n, copyErr := io.Copy(out, io.LimitReader(file, max+1))
		closeErr := out.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if n > max {
			return "", fmt.Errorf("%s grew beyond file-size limit", name)
		}
		return path, nil
	}
	if clip != nil {
		clip.stagedHeaders = map[string]string{}
		clip.headerChecksums = map[string]string{}
		for locale, path := range clip.HeaderImages {
			file, err := openSource(clip.sourceRoot, path)
			if err != nil {
				return cleanup, err
			}
			staged, err := stage(file, path, 1<<30)
			if err != nil {
				return cleanup, err
			}
			f, err := os.Open(staged)
			if err != nil {
				return cleanup, err
			}
			cfg, format, err := image.DecodeConfig(f)
			f.Close()
			if err != nil {
				return cleanup, fmt.Errorf("app clip header %s is not a decodable image: %w", path, err)
			}
			if format != "png" || cfg.Width != 1800 || cfg.Height != 1200 {
				return cleanup, fmt.Errorf("app clip header %s must be PNG at 1800 x 1200 pixels", path)
			}
			clip.stagedHeaders[locale] = staged
			hash, err := asc.ComputeFileChecksum(staged, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return cleanup, err
			}
			clip.headerChecksums[locale] = hash.Hash
		}
	}
	previewContent := map[string]string{}
	for i := range previews {
		p := &previews[i]
		file, err := openSource(p.sourceRoot, p.Path)
		if err != nil {
			return cleanup, err
		}
		staged, err := stage(file, p.Path, maxPreviewBytes)
		if err != nil {
			return cleanup, err
		}
		if err := validateVideo(ctx, staged, p.DeviceType, p.PosterFrame); err != nil {
			return cleanup, fmt.Errorf("preview %s: %w", p.Path, err)
		}
		p.stagedPath = staged
		hash, err := asc.ComputeFileChecksum(staged, asc.ChecksumAlgorithmMD5)
		if err != nil {
			return cleanup, err
		}
		p.checksum = hash.Hash
		key := p.Locale + "/" + strings.ToUpper(p.DeviceType) + "/" + p.checksum
		if previous, ok := previewContent[key]; ok {
			return cleanup, fmt.Errorf("duplicate preview content in %s/%s: %q and %q", p.Locale, p.DeviceType, previous, p.FileName)
		}
		previewContent[key] = p.FileName
	}
	success = true
	return cleanup, nil
}

var (
	lookupProbe     = exec.LookPath
	runProbeCommand = exec.CommandContext
)

func validateVideo(ctx context.Context, path, device, poster string) error {
	executable, err := lookupProbe("ffprobe")
	if err != nil {
		return fmt.Errorf("ffprobe is required to validate App Preview dimensions and duration; install FFmpeg and ensure ffprobe is on PATH")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := runProbeCommand(bounded, executable, "-v", "error", "-protocol_whitelist", "file,pipe", "-select_streams", "v:0", "-show_entries", "stream=width,height:stream_tags=rotate:stream_side_data=rotation:format=duration", "-of", "json", path)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		cmd.Env = append(cmd.Env, "SystemRoot="+systemRoot)
	}
	output := &probeBuffer{}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if bounded.Err() != nil {
			return fmt.Errorf("ffprobe: %w", bounded.Err())
		}
		return fmt.Errorf("ffprobe could not inspect video: %w", err)
	}
	if output.truncated {
		return fmt.Errorf("ffprobe output exceeded 64 KiB")
	}
	var report struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
			Tags   struct {
				Rotate string `json:"rotate"`
			} `json:"tags"`
			SideData []struct {
				Rotation int `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		return fmt.Errorf("invalid ffprobe response: %w", err)
	}
	duration, err := strconv.ParseFloat(report.Format.Duration, 64)
	if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 15 || duration > 30 {
		return fmt.Errorf("app preview duration must be between 15 and 30 seconds")
	}
	if len(report.Streams) != 1 {
		return fmt.Errorf("preview must contain a video stream")
	}
	stream := report.Streams[0]
	rotation, _ := strconv.Atoi(stream.Tags.Rotate)
	for _, side := range stream.SideData {
		rotation = side.Rotation
	}
	w, h := stream.Width, stream.Height
	if rotation%180 != 0 {
		w, h = h, w
	}
	if !validPreviewDimensions(strings.ToUpper(device), w, h) {
		return fmt.Errorf("unsupported %s preview dimensions %d x %d", device, w, h)
	}
	if poster != "" && posterSeconds(poster) >= duration {
		return fmt.Errorf("poster frame must fall within the preview duration")
	}
	return nil
}

type probeBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *probeBuffer) Write(p []byte) (int, error) {
	size := len(p)
	remaining := (64 << 10) - b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.Buffer.Write(p)
	return size, nil
}

// Accepted delivery resolutions from Apple's App Preview specifications.
func validPreviewDimensions(device string, w, h int) bool {
	match := func(a, b int) bool { return (w == a && h == b) || (w == b && h == a) }
	switch device {
	case "IPHONE_67", "IPHONE_65", "IPHONE_61", "IPHONE_58":
		return match(886, 1920)
	case "IPHONE_55", "IPHONE_40":
		return match(1080, 1920)
	case "IPHONE_47":
		return match(750, 1334)
	case "IPAD_PRO_3GEN_129", "IPAD_PRO_3GEN_11", "IPAD_105":
		return match(1200, 1600)
	case "IPAD_PRO_129":
		return match(1200, 1600) || match(900, 1200)
	case "IPAD_97":
		return match(900, 1200)
	case "DESKTOP", "APPLE_TV":
		return w == 1920 && h == 1080
	case "APPLE_VISION_PRO":
		return w == 3840 && h == 2160
	default:
		return false
	}
}

func posterSeconds(value string) float64 {
	parts := strings.Split(value, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	second, _ := strconv.ParseFloat(parts[2], 64)
	total := float64(hour*3600+minute*60) + second
	if len(parts) == 4 {
		frames, _ := strconv.Atoi(parts[3])
		total += float64(frames) / 30
	}
	return total
}
