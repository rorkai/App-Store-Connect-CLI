package assetlibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type libraryVideoProperties struct {
	Width, Height int
	FPS, Duration float64
	Audio         bool
	Codec         string
	MIMEType      string
}
type videoProbeBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *videoProbeBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (64 << 10) - b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, err := b.Buffer.Write(p)
	return n, err
}

func inspectLibraryVideo(ctx context.Context, path string) (libraryVideoProperties, error) {
	var properties libraryVideoProperties
	executable, err := exec.LookPath("ffprobe")
	if err != nil {
		return properties, fmt.Errorf("ffprobe is required; install FFmpeg and put ffprobe on PATH")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, executable, "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "stream=codec_type,codec_name,width,height,avg_frame_rate:stream_tags=rotate:stream_side_data=rotation:format=duration,format_name:format_tags=major_brand,compatible_brands", "-of", "json", path)
	command.WaitDelay = 250 * time.Millisecond
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		command.Env = append(command.Env, "SystemRoot="+systemRoot)
	}
	buffer := &videoProbeBuffer{}
	command.Stdout = buffer
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if bounded.Err() != nil {
			return properties, fmt.Errorf("ffprobe: %w", bounded.Err())
		}
		return properties, fmt.Errorf("ffprobe could not inspect video: %w", err)
	}
	if buffer.truncated {
		return properties, fmt.Errorf("ffprobe output exceeded 64 KiB")
	}
	var response struct {
		Streams []struct {
			Type          string `json:"codec_type"`
			Codec         string `json:"codec_name"`
			Width, Height int
			FPS           string `json:"avg_frame_rate"`
			Tags          struct {
				Rotate string `json:"rotate"`
			}
			Side []struct {
				Rotation int `json:"rotation"`
			} `json:"side_data_list"`
		}
		Format struct {
			Duration string
			Name     string `json:"format_name"`
			Tags     struct {
				MajorBrand       string `json:"major_brand"`
				CompatibleBrands string `json:"compatible_brands"`
			} `json:"tags"`
		}
	}
	if err := json.Unmarshal(buffer.Bytes(), &response); err != nil {
		return properties, fmt.Errorf("invalid ffprobe response")
	}
	containerSupported := false
	for _, name := range strings.Split(response.Format.Name, ",") {
		if name == "mov" || name == "mp4" || name == "m4v" {
			containerSupported = true
		}
	}
	if !containerSupported {
		return properties, fmt.Errorf("unsupported video container: use MOV, MP4 or M4V")
	}
	properties.MIMEType, err = libraryVideoContainerMIME(response.Format.Tags.MajorBrand, response.Format.Tags.CompatibleBrands)
	if err != nil {
		return properties, err
	}
	videoCount := 0
	for _, stream := range response.Streams {
		if stream.Type == "audio" {
			properties.Audio = true
		}
		if stream.Type != "video" {
			continue
		}
		videoCount++
		properties.Width, properties.Height, properties.Codec = stream.Width, stream.Height, stream.Codec
		rate := strings.Split(stream.FPS, "/")
		if len(rate) != 2 {
			return properties, fmt.Errorf("invalid video frame rate")
		}
		num, e1 := strconv.ParseFloat(rate[0], 64)
		den, e2 := strconv.ParseFloat(rate[1], 64)
		if e1 != nil || e2 != nil || den <= 0 {
			return properties, fmt.Errorf("invalid video frame rate")
		}
		properties.FPS = num / den
		rotation, err := strconv.Atoi(stream.Tags.Rotate)
		if stream.Tags.Rotate != "" && err != nil {
			return properties, fmt.Errorf("invalid video rotation")
		}
		for _, side := range stream.Side {
			rotation = side.Rotation
		}
		if rotation%90 != 0 {
			return properties, fmt.Errorf("unsupported video rotation")
		}
		if rotation%180 != 0 {
			properties.Width, properties.Height = properties.Height, properties.Width
		}
	}
	properties.Duration, err = strconv.ParseFloat(response.Format.Duration, 64)
	if videoCount != 1 || err != nil || math.IsNaN(properties.Duration) || math.IsInf(properties.Duration, 0) || math.IsNaN(properties.FPS) || math.IsInf(properties.FPS, 0) || properties.Width <= 0 || properties.Height <= 0 || properties.FPS <= 0 {
		return properties, fmt.Errorf("video must contain one valid video stream with duration and frame rate")
	}
	return properties, nil
}

func matchLibraryVideoSpec(raw []byte, category string, p libraryVideoProperties, size int64, extension string) error {
	return matchLibraryVideoSpecIn(raw, category, nil, p, size, extension)
}

// matchLibraryVideoSpecIn limits matching to allowed spec IDs when allowed is non-nil.
func matchLibraryVideoSpecIn(raw []byte, category string, allowed map[string]bool, p libraryVideoProperties, size int64, extension string) error {
	var envelope struct {
		Data []struct {
			Attributes struct {
				Specs []struct {
					SpecID     string `json:"specId"`
					Dimensions struct{ MinWidth, MaxWidth, MinHeight, MaxHeight int }
					Aspect     json.RawMessage `json:"aspectRatio"`
					Placements []string        `json:"compatiblePlacementTypes"`
					Rates      []struct {
						MinFPS float64 `json:"minFps"`
						MaxFPS float64 `json:"maxFps"`
					} `json:"frameRates"`
					Duration   struct{ Min, Max string }
					Audio      bool     `json:"audioRequired"`
					Extensions []string `json:"fileExtensions"`
					MIMETypes  []string `json:"mimeTypes"`
					MaxSize    int64    `json:"maxFileSize"`
				} `json:"videoSpecs"`
			}
		}
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("invalid asset reference data: %w", err)
	}
	for _, item := range envelope.Data {
		for _, spec := range item.Attributes.Specs {
			if allowed != nil && !allowed[spec.SpecID] {
				continue
			}
			mimeAllowed := false
			for _, mimeType := range spec.MIMETypes {
				if mimeType == p.MIMEType {
					mimeAllowed = true
				}
			}
			if !mimeAllowed {
				continue
			}
			preview := false
			for _, placement := range spec.Placements {
				if placement == "APP_PREVIEW" {
					preview = true
				}
			}
			if (category == "APP_SCREENSHOTS_AND_PREVIEWS") != preview {
				continue
			}
			dims := spec.Dimensions
			if p.Width < dims.MinWidth || p.Width > dims.MaxWidth || p.Height < dims.MinHeight || p.Height > dims.MaxHeight || spec.MaxSize <= 0 || size > spec.MaxSize {
				continue
			}
			// Fixed dimensions can be rounded to whole pixels (3840x1646 / 21:9).
			ratio, ratioErr := normalizeLibraryVideoAspectRatio(spec.Aspect)
			provided := len(bytes.TrimSpace(spec.Aspect)) > 0 && !bytes.Equal(bytes.TrimSpace(spec.Aspect), []byte("null"))
			if provided && ratioErr != nil {
				continue
			}
			if dims.MinWidth != dims.MaxWidth || dims.MinHeight != dims.MaxHeight {
				if ratioErr != nil {
					continue
				}
				actual := new(big.Rat).SetFrac(big.NewInt(int64(p.Width)), big.NewInt(int64(p.Height)))
				if actual.Cmp(ratio) != 0 {
					continue
				}
			}
			minDuration, e1 := time.ParseDuration(strings.ToLower(strings.TrimPrefix(spec.Duration.Min, "PT")))
			maxDuration, e2 := time.ParseDuration(strings.ToLower(strings.TrimPrefix(spec.Duration.Max, "PT")))
			if e1 != nil || e2 != nil || p.Duration < minDuration.Seconds() || p.Duration > maxDuration.Seconds() {
				continue
			}
			rateOK := false
			for _, r := range spec.Rates {
				if p.FPS >= r.MinFPS-0.001 && p.FPS <= r.MaxFPS+0.001 {
					rateOK = true
				}
			}
			if !rateOK || spec.Audio && !p.Audio {
				continue
			}
			for _, ext := range spec.Extensions {
				if strings.EqualFold(ext, extension) {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("video does not match a current %s specification (dimensions, duration, frame rate, audio or file size)", category)
}

// ffprobe shares a demuxer across QuickTime, MPEG-4 and unsupported 3GPP/MJ2.
// File brands distinguish those containers without consulting the filename.
func libraryVideoContainerMIME(major, compatible string) (string, error) {
	classify := func(brand string) string {
		switch strings.ToLower(strings.TrimSpace(brand)) {
		case "qt":
			return "video/quicktime"
		case "m4v", "m4vh", "m4vp":
			return "video/x-m4v"
		case "isom", "iso2", "iso3", "iso4", "iso5", "iso6", "iso7", "iso8", "iso9", "mp41", "mp42", "avc1", "avc2":
			return "video/mp4"
		}
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(major))
	if strings.HasPrefix(normalized, "3g") || normalized == "mjp2" || normalized == "mj2" {
		return "", fmt.Errorf("unsupported video container brand %q", major)
	}
	if mimeType := classify(major); mimeType != "" {
		return mimeType, nil
	}
	// An unfamiliar major brand can still declare MPEG-4 compatibility.
	for offset := 0; offset+4 <= len(compatible); offset += 4 {
		if classify(compatible[offset:offset+4]) == "video/mp4" {
			return "video/mp4", nil
		}
	}
	return "", fmt.Errorf("unsupported or unknown video container brand %q", major)
}

// Preserve decimal ratios exactly while rejecting nonfinite/nonpositive values.
func normalizeLibraryVideoAspectRatio(raw json.RawMessage) (*big.Rat, error) {
	var width, height string
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		components := strings.Split(text, ":")
		if len(components) != 2 {
			return nil, fmt.Errorf("invalid aspect ratio")
		}
		width, height = strings.TrimSpace(components[0]), strings.TrimSpace(components[1])
	} else {
		var object struct {
			Width  json.Number `json:"width"`
			Height json.Number `json:"height"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("invalid aspect ratio")
		}
		width, height = object.Width.String(), object.Height.String()
	}
	parse := func(value string) (*big.Rat, error) {
		if len(value) > 64 {
			return nil, fmt.Errorf("invalid aspect ratio component")
		}
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 {
			return nil, fmt.Errorf("aspect ratio component must be finite and positive")
		}
		rational, ok := new(big.Rat).SetString(value)
		if !ok || rational.Sign() <= 0 {
			return nil, fmt.Errorf("invalid aspect ratio component")
		}
		return rational, nil
	}
	w, err := parse(width)
	if err != nil {
		return nil, err
	}
	h, err := parse(height)
	if err != nil {
		return nil, err
	}
	return new(big.Rat).Quo(w, h), nil
}
