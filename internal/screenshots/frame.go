package screenshots

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// FrameDevice identifies a supported frame profile.
type FrameDevice string

const (
	FrameDeviceIPhoneAir   FrameDevice = "iphone-air"
	FrameDeviceIPhone17Pro FrameDevice = "iphone-17-pro"
	FrameDeviceIPhone17PM  FrameDevice = "iphone-17-pro-max"
	FrameDeviceIPhone16e   FrameDevice = "iphone-16e"
	FrameDeviceIPhone17    FrameDevice = "iphone-17"
	FrameDeviceMac         FrameDevice = "mac"

	pinnedKoubouVersion = "0.18.0"
)

const (
	canvasTitleFontSize    = 60
	canvasSubtitleFontSize = 28
	canvasWindowHeightFrac = 0.70 // max window height as fraction of canvas height when text overlays are present

	canvasTitleY        = "12%"
	canvasSubtitleY     = "16%"
	canvasSubtitleSoloY = "12%" // subtitle Y when no title is present
	canvasWindowCenterY = "50%"
	canvasWindowTextY   = "60%" // window pushed down to make room for text overlays

	canvasBGColorFrom = "#0d0c1e"
	canvasBGColorTo   = "#140f2d"
	canvasBGAngle     = 135.0

	canvasDefaultTitleColor    = "#ffffff"
	canvasDefaultSubtitleColor = "#aaaaaa"
)

var koubouVersionPattern = regexp.MustCompile(`(?i)\bv?(\d+\.\d+\.\d+)\b`)

var (
	koubouVersionCacheMu      sync.Mutex
	cachedKoubouBinaryPath    string
	cachedKoubouResolvedPATH  string
	cachedKoubouVersionIsGood bool
	cachedKoubouFramesReady   bool
)

var supportedFrameDevices = []FrameDevice{
	FrameDeviceIPhoneAir,
	FrameDeviceIPhone17Pro,
	FrameDeviceIPhone17PM,
	FrameDeviceIPhone16e,
	FrameDeviceIPhone17,
	FrameDeviceMac,
}

type frameDeviceKoubouSpec struct {
	FrameName   string
	Aliases     []string
	OutputSize  string // Koubou named size (e.g. "iPhone6_9" or "AppDesktop_2880")
	DisplayType string
	Canvas      bool // true = plain canvas, no device bezel; screenshot scaled to fill
}

// Keeps the existing asc device slugs while delegating rendering to pinned
// Koubou v0.18.0 frame names.
var frameDeviceKoubouSpecs = map[FrameDevice]frameDeviceKoubouSpec{
	FrameDeviceIPhoneAir: {
		FrameName:   "iPhone Air - Light Gold - Portrait",
		Aliases:     []string{"iPhone 16 Pro - White Titanium - Portrait"},
		OutputSize:  "iPhone6_9_alt",
		DisplayType: "APP_IPHONE_69",
	},
	FrameDeviceIPhone17PM: {
		FrameName:   "iPhone 17 Pro Max - Silver - Portrait",
		Aliases:     []string{"iPhone 16 Pro Max - White Titanium - Portrait"},
		OutputSize:  "iPhone6_9",
		DisplayType: "APP_IPHONE_69",
	},
	FrameDeviceIPhone17Pro: {
		FrameName:   "iPhone 17 Pro - Silver - Portrait",
		Aliases:     []string{"iPhone 15 Pro - White Titanium - Portrait"},
		OutputSize:  "iPhone6_3",
		DisplayType: "APP_IPHONE_61",
	},
	FrameDeviceIPhone17: {
		FrameName:   "iPhone 17 - White - Portrait",
		Aliases:     []string{"iPhone 17 - Teal - Portrait", "iPhone 14 Pro Portrait"},
		OutputSize:  "iPhone6_3",
		DisplayType: "APP_IPHONE_61",
	},
	FrameDeviceIPhone16e: {
		FrameName:   "iPhone 16 - White - Portrait",
		Aliases:     []string{"iPhone 16e - White - Portrait"},
		OutputSize:  "iPhone6_1",
		DisplayType: "APP_IPHONE_61",
	},
	FrameDeviceMac: {
		FrameName:   "Mac",
		OutputSize:  "AppDesktop_2880",
		DisplayType: "APP_DESKTOP",
		Canvas:      true,
	},
}

// CanvasOptions controls title/subtitle/color overlays for canvas-mode devices
// (e.g. --device mac). All fields are optional; zero values use defaults.
type CanvasOptions struct {
	Title         string
	Subtitle      string
	BGColor       string // solid background hex color (e.g. "#ffffff"); defaults to dark gradient
	TitleColor    string // title text color; defaults to canvasDefaultTitleColor
	SubtitleColor string // subtitle text color; defaults to canvasDefaultSubtitleColor
}

func (o CanvasOptions) hasText() bool { return o.Title != "" || o.Subtitle != "" }

// FrameRequest holds options for composing one screenshot.
type FrameRequest struct {
	InputPath  string         // required when ConfigPath is empty
	OutputPath string         // optional for custom config mode; required for input mode
	Device     string         // device slug; defaults to iphone-air when empty
	ConfigPath string         // optional Koubou YAML config path
	Canvas     *CanvasOptions // nil for bezel devices; non-nil for canvas devices (e.g. mac)

	// Kept for backwards compatibility; ignored in Koubou mode.
	FrameRoot   string
	ScreenBleed int
}

// FrameResult is the structured output for one composed frame image.
type FrameResult struct {
	Path         string `json:"path"`
	FramePath    string `json:"frame_path"`
	Device       string `json:"device"`
	DisplayType  string `json:"display_type,omitempty"`
	UploadWidth  int    `json:"upload_width,omitempty"`
	UploadHeight int    `json:"upload_height,omitempty"`
	Normalized   bool   `json:"normalized"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// FrameDeviceOption describes one supported frame device value.
type FrameDeviceOption struct {
	ID      string `json:"id"`
	Default bool   `json:"default"`
}

type koubouGenerateResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type frameExecutionMetadata struct {
	FrameRef     string
	DisplayType  string
	UploadWidth  int
	UploadHeight int
}

type koubouDefaultConfig struct {
	Project     koubouProjectConfig                    `yaml:"project"`
	Screenshots map[string]koubouDefaultScreenshotSpec `yaml:"screenshots"`
}

// koubouOutputSize is either a named size string (e.g. "iPhone6_9") or an explicit
// [width, height] pixel list (used for canvas devices like Mac). It implements
// yaml.Marshaler so the correct YAML type is always emitted — no any needed.
type koubouOutputSize struct {
	named string // non-empty for named sizes (iOS)
	w, h  int    // non-zero for explicit pixel dimensions (Mac canvas)
}

func namedOutputSize(name string) koubouOutputSize { return koubouOutputSize{named: name} }
func dimsOutputSize(w, h int) koubouOutputSize     { return koubouOutputSize{w: w, h: h} }

func (s koubouOutputSize) MarshalYAML() (interface{}, error) {
	if s.named != "" {
		return s.named, nil
	}
	return []int{s.w, s.h}, nil
}

type koubouProjectConfig struct {
	Name       string           `yaml:"name"`
	OutputDir  string           `yaml:"output_dir"`
	Device     string           `yaml:"device"`
	OutputSize koubouOutputSize `yaml:"output_size"`
}

type koubouGradientConfig struct {
	Type      string   `yaml:"type"`
	Colors    []string `yaml:"colors"`
	Direction float64  `yaml:"direction,omitempty"`
}

type koubouDefaultScreenshotSpec struct {
	Background *koubouGradientConfig      `yaml:"background,omitempty"`
	Content    []koubouDefaultContentItem `yaml:"content"`
}

type koubouDefaultContentItem struct {
	Type      string    `yaml:"type"`
	Asset     string    `yaml:"asset,omitempty"`
	Content   string    `yaml:"content,omitempty"`
	Position  [2]string `yaml:"position"`
	Scale     float64   `yaml:"scale,omitempty"`
	Frame     *bool     `yaml:"frame,omitempty"`
	Color     string    `yaml:"color,omitempty"`
	Size      int       `yaml:"size,omitempty"`
	Weight    string    `yaml:"weight,omitempty"`
	Alignment string    `yaml:"alignment,omitempty"`
}

// DefaultFrameDevice returns the default frame device.
func DefaultFrameDevice() FrameDevice {
	return FrameDeviceIPhoneAir
}

// FrameDeviceValues returns allowed --device values in CLI display order.
func FrameDeviceValues() []string {
	values := make([]string, 0, len(supportedFrameDevices))
	for _, device := range supportedFrameDevices {
		values = append(values, string(device))
	}
	return values
}

// FrameDeviceOptions returns supported values with default marker.
func FrameDeviceOptions() []FrameDeviceOption {
	options := make([]FrameDeviceOption, 0, len(supportedFrameDevices))
	defaultDevice := DefaultFrameDevice()
	for _, device := range supportedFrameDevices {
		options = append(options, FrameDeviceOption{
			ID:      string(device),
			Default: device == defaultDevice,
		})
	}
	return options
}

// IsCanvasDevice returns true if the device uses canvas mode (no device bezel).
func IsCanvasDevice(device FrameDevice) bool {
	spec, ok := frameDeviceKoubouSpecs[device]
	return ok && spec.Canvas
}

// ParseFrameDevice normalizes and validates a frame device value.
func ParseFrameDevice(raw string) (FrameDevice, error) {
	normalized := normalizeFrameDevice(raw)
	if normalized == "" {
		return DefaultFrameDevice(), nil
	}

	candidate := FrameDevice(normalized)
	for _, allowed := range supportedFrameDevices {
		if candidate == allowed {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"unsupported frame device %q (allowed: %s)",
		raw,
		strings.Join(FrameDeviceValues(), ", "),
	)
}

// Frame composes screenshots through Koubou's YAML pipeline.
func Frame(ctx context.Context, req FrameRequest) (*FrameResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	device, err := ParseFrameDevice(req.Device)
	if err != nil {
		return nil, err
	}

	outputPath := strings.TrimSpace(req.OutputPath)
	configPath := strings.TrimSpace(req.ConfigPath)
	resultDevice := string(device)
	metadata := frameExecutionMetadata{
		FrameRef: string(device),
	}

	if configPath == "" {
		inputPath := strings.TrimSpace(req.InputPath)
		if inputPath == "" {
			return nil, fmt.Errorf("input path is required")
		}
		if outputPath == "" {
			return nil, fmt.Errorf("output path is required")
		}

		spec, ok := frameDeviceKoubouSpecs[device]
		if !ok {
			return nil, fmt.Errorf("no Koubou mapping configured for device %q", device)
		}
		if req.Canvas != nil && !spec.Canvas {
			return nil, fmt.Errorf("canvas options require a canvas device; %q uses a device bezel", device)
		}

		absInputPath, err := filepath.Abs(inputPath)
		if err != nil {
			return nil, fmt.Errorf("resolve input path: %w", err)
		}
		if err := asc.ValidateImageFile(absInputPath); err != nil {
			return nil, fmt.Errorf("read input screenshot: %w", err)
		}

		generatedConfigPath, generatedMetadata, generatedWorkDir, err := createDefaultKoubouConfig(absInputPath, spec, req.Canvas)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = os.RemoveAll(generatedWorkDir)
		}()
		configPath = generatedConfigPath
		metadata = generatedMetadata
	} else {
		absConfigPath, err := filepath.Abs(configPath)
		if err != nil {
			return nil, fmt.Errorf("resolve config path: %w", err)
		}
		configPath = absConfigPath
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
		if parsed := parseKoubouConfigMetadata(configPath); parsed != nil {
			metadata = *parsed
			resultDevice = resolveFrameDeviceForConfig(metadata.FrameRef, resultDevice)
		}
	}

	generatedResults, err := runKoubouGenerate(ctx, configPath)
	if err != nil {
		return nil, err
	}
	generatedPath, err := selectGeneratedScreenshot(configPath, generatedResults)
	if err != nil {
		return nil, err
	}

	finalPath := generatedPath
	if outputPath != "" {
		absOutputPath, err := filepath.Abs(outputPath)
		if err != nil {
			return nil, fmt.Errorf("resolve output path: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(absOutputPath), 0o755); err != nil {
			return nil, fmt.Errorf("create output directory: %w", err)
		}
		if err := copyFile(generatedPath, absOutputPath); err != nil {
			return nil, err
		}
		finalPath = absOutputPath
	}

	if err := asc.ValidateImageFile(finalPath); err != nil {
		return nil, fmt.Errorf("koubou output invalid: %w", err)
	}
	dimensions, err := asc.ReadImageDimensions(finalPath)
	if err != nil {
		return nil, fmt.Errorf("read output image dimensions: %w", err)
	}
	if metadata.UploadWidth == 0 || metadata.UploadHeight == 0 {
		metadata.UploadWidth = dimensions.Width
		metadata.UploadHeight = dimensions.Height
	}

	normalized := dimensions.Width == metadata.UploadWidth && dimensions.Height == metadata.UploadHeight
	absFinalPath, _ := filepath.Abs(finalPath)
	return &FrameResult{
		Path:         absFinalPath,
		FramePath:    metadata.FrameRef,
		Device:       resultDevice,
		DisplayType:  metadata.DisplayType,
		UploadWidth:  metadata.UploadWidth,
		UploadHeight: metadata.UploadHeight,
		Normalized:   normalized,
		Width:        dimensions.Width,
		Height:       dimensions.Height,
	}, nil
}

// boolPtr returns a pointer to b. Used for YAML fields that require *bool for omitempty.
func boolPtr(b bool) *bool { return &b }

func createDefaultKoubouConfig(
	absInputPath string,
	spec frameDeviceKoubouSpec,
	canvas *CanvasOptions,
) (string, frameExecutionMetadata, string, error) {
	workDir, err := os.MkdirTemp("", "asc-shots-kou-*")
	if err != nil {
		return "", frameExecutionMetadata{}, "", fmt.Errorf("create temp config directory: %w", err)
	}

	kouOutputDir := filepath.Join(workDir, "output")
	if err := os.MkdirAll(kouOutputDir, 0o755); err != nil {
		return "", frameExecutionMetadata{}, "", fmt.Errorf("create temp output directory: %w", err)
	}

	scale := 1.0
	kouOutputSize := namedOutputSize(spec.OutputSize)
	opts := canvas
	if opts == nil {
		opts = &CanvasOptions{}
	}

	if spec.Canvas {
		if cw, ch, ok := resolveKoubouOutputSize(spec.OutputSize); ok {
			kouOutputSize = dimsOutputSize(cw, ch)
			if dims, err := asc.ReadImageDimensions(absInputPath); err == nil && dims.Width > 0 && dims.Height > 0 {
				maxH := float64(ch)
				if opts.hasText() {
					maxH = float64(ch) * canvasWindowHeightFrac
				}
				scaleByW := float64(cw) / float64(dims.Width)
				scaleByH := maxH / float64(dims.Height)
				if scaleByW < scaleByH {
					scale = scaleByW
				} else {
					scale = scaleByH
				}
			}
		}
	}

	var background *koubouGradientConfig
	var contentItems []koubouDefaultContentItem
	windowY := canvasWindowCenterY

	if spec.Canvas {
		if opts.BGColor != "" {
			background = &koubouGradientConfig{
				Type:   "linear",
				Colors: []string{opts.BGColor, opts.BGColor},
			}
		} else {
			background = &koubouGradientConfig{
				Type:      "linear",
				Colors:    []string{canvasBGColorFrom, canvasBGColorTo},
				Direction: canvasBGAngle,
			}
		}

		if opts.hasText() {
			windowY = canvasWindowTextY
		}

		if opts.Title != "" {
			tc := opts.TitleColor
			if tc == "" {
				tc = canvasDefaultTitleColor
			}
			contentItems = append(contentItems, koubouDefaultContentItem{
				Type:      "text",
				Content:   opts.Title,
				Position:  [2]string{"50%", canvasTitleY},
				Size:      canvasTitleFontSize,
				Weight:    "bold",
				Color:     tc,
				Alignment: "center",
			})
		}

		subtitleY := canvasSubtitleY
		if opts.Title == "" {
			subtitleY = canvasSubtitleSoloY
		}
		if opts.Subtitle != "" {
			sc := opts.SubtitleColor
			if sc == "" {
				sc = canvasDefaultSubtitleColor
			}
			contentItems = append(contentItems, koubouDefaultContentItem{
				Type:      "text",
				Content:   opts.Subtitle,
				Position:  [2]string{"50%", subtitleY},
				Size:      canvasSubtitleFontSize,
				Color:     sc,
				Alignment: "center",
			})
		}

		contentItems = append(contentItems, koubouDefaultContentItem{
			Type:     "image",
			Asset:    absInputPath,
			Position: [2]string{"50%", windowY},
			Scale:    scale,
			Frame:    boolPtr(false),
		})
	} else {
		contentItems = []koubouDefaultContentItem{
			{
				Type:     "image",
				Asset:    absInputPath,
				Position: [2]string{"50%", "50%"},
				Scale:    scale,
				Frame:    boolPtr(true),
			},
		}
	}

	configPath := filepath.Join(workDir, "frame.yaml")
	config := koubouDefaultConfig{
		Project: koubouProjectConfig{
			Name:       "ASC Shots Frame",
			OutputDir:  kouOutputDir,
			Device:     spec.FrameName,
			OutputSize: kouOutputSize,
		},
		Screenshots: map[string]koubouDefaultScreenshotSpec{
			"framed": {
				Background: background,
				Content:    contentItems,
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return "", frameExecutionMetadata{}, "", fmt.Errorf("marshal default Koubou YAML: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return "", frameExecutionMetadata{}, "", fmt.Errorf("write default Koubou YAML: %w", err)
	}

	metadata := frameExecutionMetadata{
		FrameRef:    spec.FrameName,
		DisplayType: spec.DisplayType,
	}
	if width, height, ok := resolveKoubouOutputSize(spec.OutputSize); ok {
		metadata.UploadWidth = width
		metadata.UploadHeight = height
	}
	return configPath, metadata, workDir, nil
}

func resolveFrameDeviceForConfig(frameRef, fallback string) string {
	trimmedFrameRef := strings.TrimSpace(frameRef)
	if trimmedFrameRef == "" {
		return fallback
	}
	for device, spec := range frameDeviceKoubouSpecs {
		if frameSpecMatchesFrameRef(spec, trimmedFrameRef) {
			return string(device)
		}
	}
	return trimmedFrameRef
}

func frameSpecMatchesFrameRef(spec frameDeviceKoubouSpec, frameRef string) bool {
	if strings.EqualFold(strings.TrimSpace(spec.FrameName), frameRef) {
		return true
	}
	for _, alias := range spec.Aliases {
		if strings.EqualFold(strings.TrimSpace(alias), frameRef) {
			return true
		}
	}
	return false
}

// ResolveFrameDeviceFromConfig resolves the config device to a supported CLI slug.
func ResolveFrameDeviceFromConfig(configPath, fallback string) string {
	parsed := parseKoubouConfigMetadata(strings.TrimSpace(configPath))
	if parsed == nil {
		return fallback
	}
	resolved := resolveFrameDeviceForConfig(parsed.FrameRef, fallback)
	device, err := ParseFrameDevice(resolved)
	if err != nil {
		return fallback
	}
	return string(device)
}

func parseKoubouConfigMetadata(configPath string) *frameExecutionMetadata {
	type project struct {
		Device     string `yaml:"device"`
		OutputSize any    `yaml:"output_size"`
	}
	type parsedConfig struct {
		Project project `yaml:"project"`
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var parsed parsedConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil
	}

	metadata := &frameExecutionMetadata{
		FrameRef: strings.TrimSpace(parsed.Project.Device),
	}
	if width, height, ok := resolveKoubouOutputSize(parsed.Project.OutputSize); ok {
		metadata.UploadWidth = width
		metadata.UploadHeight = height
	}
	if outputSizeName, ok := parsed.Project.OutputSize.(string); ok {
		if displayType, mapped := koubouDisplayTypeForSizeName(outputSizeName); mapped {
			metadata.DisplayType = displayType
		}
	}
	if metadata.DisplayType == "" {
		if displayType, ok := displayTypeForDimensions(metadata.UploadWidth, metadata.UploadHeight); ok {
			metadata.DisplayType = displayType
		}
	}
	return metadata
}

func koubouDisplayTypeForSizeName(sizeName string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(sizeName)) {
	case "iphone6_9", "iphone6_9_alt":
		return "APP_IPHONE_69", true
	case "iphone6_7":
		return "APP_IPHONE_67", true
	case "iphone6_3":
		return "APP_IPHONE_61", true
	case "iphone6_1":
		return "APP_IPHONE_61", true
	case "iphone5_8":
		return "APP_IPHONE_58", true
	case "iphone5_5":
		return "APP_IPHONE_55", true
	default:
		return "", false
	}
}

func resolveKoubouOutputSize(value any) (int, int, bool) {
	namedSizes := map[string]struct {
		Width  int
		Height int
	}{
		"iphone6_9":     {Width: 1320, Height: 2868},
		"iphone6_9_alt": {Width: 1260, Height: 2736},
		"iphone6_3":     {Width: 1206, Height: 2622},
		"iphone6_7":     {Width: 1290, Height: 2796},
		"iphone6_1":     {Width: 1179, Height: 2556},
		"iphone5_8":     {Width: 1170, Height: 2532},
		"iphone5_5":     {Width: 1242, Height: 2208},
		// Mac App Store desktop (16:10)
		"appdesktop_1280": {Width: 1280, Height: 800},
		"appdesktop_1440": {Width: 1440, Height: 900},
		"appdesktop_2560": {Width: 2560, Height: 1600},
		"appdesktop_2880": {Width: 2880, Height: 1800},
	}

	switch typed := value.(type) {
	case string:
		entry, ok := namedSizes[strings.ToLower(strings.TrimSpace(typed))]
		if !ok {
			return 0, 0, false
		}
		return entry.Width, entry.Height, true
	case []any:
		if len(typed) != 2 {
			return 0, 0, false
		}
		width, ok := toInt(typed[0])
		if !ok {
			return 0, 0, false
		}
		height, ok := toInt(typed[1])
		if !ok {
			return 0, 0, false
		}
		return width, height, true
	default:
		return 0, 0, false
	}
}

func displayTypeForDimensions(width, height int) (string, bool) {
	// Mac — Apple's four required 16:10 screenshot sizes
	macSizes := [][2]int{{1280, 800}, {1440, 900}, {2560, 1600}, {2880, 1800}}
	for _, sz := range macSizes {
		if width == sz[0] && height == sz[1] {
			return "APP_DESKTOP", true
		}
	}

	iphoneDisplayTypes := []string{
		"APP_IPHONE_69",
		"APP_IPHONE_67",
		"APP_IPHONE_61",
		"APP_IPHONE_58",
		"APP_IPHONE_55",
		"APP_IPHONE_47",
		"APP_IPHONE_40",
		"APP_IPHONE_35",
	}
	for _, displayType := range iphoneDisplayTypes {
		dimensions, ok := asc.ScreenshotDimensions(displayType)
		if !ok {
			continue
		}
		for _, dimension := range dimensions {
			if dimension.Width == width && dimension.Height == height {
				return displayType, true
			}
		}
	}
	return "", false
}

func toInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

func runKoubouGenerate(ctx context.Context, configPath string) ([]koubouGenerateResult, error) {
	kouBinaryPath, err := ensurePinnedKoubouVersion(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensurePinnedKoubouFrames(ctx, kouBinaryPath); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, kouBinaryPath, "generate", configPath, "--output", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf(
				"kou binary not found; install pinned Koubou %s first (%s)",
				pinnedKoubouVersion,
				pinnedKoubouInstallCommand(),
			)
		}
		errorOutput := strings.TrimSpace(stderr.String())
		if errorOutput == "" {
			errorOutput = strings.TrimSpace(string(output))
		}
		return nil, fmt.Errorf("kou: %w (output: %s)", err, errorOutput)
	}

	// Koubou may emit log lines to stdout before the JSON array.
	// Extract just the JSON portion (first '[' to last ']').
	jsonBytes := extractJSONArray(output)
	if jsonBytes == nil {
		return nil, fmt.Errorf("kou: no JSON array found in output: %s", strings.TrimSpace(string(output)))
	}

	var results []koubouGenerateResult
	if err := json.Unmarshal(jsonBytes, &results); err != nil {
		return nil, fmt.Errorf("kou: parse JSON output: %w", err)
	}
	return results, nil
}

func ensurePinnedKoubouVersion(ctx context.Context) (string, error) {
	resolvedPATH := os.Getenv("PATH")
	kouBinaryPath, err := exec.LookPath("kou")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf(
				"kou binary not found; install pinned Koubou %s first (%s)",
				pinnedKoubouVersion,
				pinnedKoubouInstallCommand(),
			)
		}
		return "", fmt.Errorf("kou lookup failed: %w", err)
	}

	koubouVersionCacheMu.Lock()
	if cachedKoubouVersionIsGood &&
		cachedKoubouResolvedPATH == resolvedPATH &&
		cachedKoubouBinaryPath == kouBinaryPath {
		koubouVersionCacheMu.Unlock()
		return kouBinaryPath, nil
	}
	koubouVersionCacheMu.Unlock()

	cmd := exec.CommandContext(ctx, kouBinaryPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf(
				"kou binary not found; install pinned Koubou %s first (%s)",
				pinnedKoubouVersion,
				pinnedKoubouInstallCommand(),
			)
		}
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput == "" {
			return "", fmt.Errorf("kou --version: %w", err)
		}
		return "", fmt.Errorf("kou --version: %w (output: %s)", err, trimmedOutput)
	}

	detectedVersion, ok := parseKoubouVersion(output)
	if !ok {
		return "", fmt.Errorf("kou --version output does not include a semantic version: %q", strings.TrimSpace(string(output)))
	}
	if detectedVersion != pinnedKoubouVersion {
		return "", fmt.Errorf(
			"unsupported Koubou version %s; this ASC release is pinned to %s. Install with: %s",
			detectedVersion,
			pinnedKoubouVersion,
			pinnedKoubouInstallCommand(),
		)
	}

	koubouVersionCacheMu.Lock()
	cacheTargetChanged := cachedKoubouResolvedPATH != resolvedPATH || cachedKoubouBinaryPath != kouBinaryPath
	cachedKoubouBinaryPath = kouBinaryPath
	cachedKoubouResolvedPATH = resolvedPATH
	cachedKoubouVersionIsGood = true
	if cacheTargetChanged {
		cachedKoubouFramesReady = false
	}
	koubouVersionCacheMu.Unlock()
	return kouBinaryPath, nil
}

func ensurePinnedKoubouFrames(ctx context.Context, kouBinaryPath string) error {
	resolvedPATH := os.Getenv("PATH")

	koubouVersionCacheMu.Lock()
	if cachedKoubouFramesReady &&
		cachedKoubouResolvedPATH == resolvedPATH &&
		cachedKoubouBinaryPath == kouBinaryPath {
		koubouVersionCacheMu.Unlock()
		return nil
	}
	koubouVersionCacheMu.Unlock()

	cmd := exec.CommandContext(ctx, kouBinaryPath, "setup-frames")
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		setupHint := fmt.Sprintf(
			"Koubou %s requires downloaded device frames; run `%s` with network access once before framing",
			pinnedKoubouVersion,
			pinnedKoubouSetupFramesCommand(),
		)
		if trimmedOutput == "" {
			return fmt.Errorf("kou setup-frames: %w. %s", err, setupHint)
		}
		return fmt.Errorf("kou setup-frames: %w (output: %s). %s", err, trimmedOutput, setupHint)
	}

	koubouVersionCacheMu.Lock()
	cachedKoubouBinaryPath = kouBinaryPath
	cachedKoubouResolvedPATH = resolvedPATH
	cachedKoubouFramesReady = true
	koubouVersionCacheMu.Unlock()
	return nil
}

func parseKoubouVersion(output []byte) (string, bool) {
	matches := koubouVersionPattern.FindSubmatch(output)
	if len(matches) < 2 {
		return "", false
	}
	raw := strings.TrimSpace(string(matches[1]))
	normalized := "v" + strings.TrimPrefix(raw, "v")
	if !semver.IsValid(normalized) {
		return "", false
	}
	return strings.TrimPrefix(normalized, "v"), true
}

func pinnedKoubouInstallCommand() string {
	return fmt.Sprintf("pip install koubou==%s", pinnedKoubouVersion)
}

func pinnedKoubouSetupFramesCommand() string {
	return "kou setup-frames"
}

// extractJSONArray finds the JSON array of objects in raw output that may
// contain interleaved log lines with their own brackets (e.g. "[07:59:06]").
// It looks for "[{" which marks the start of a JSON array of objects, then
// finds the matching "]".
func extractJSONArray(raw []byte) []byte {
	// Look for "[{" — the start of a JSON array of objects.
	start := bytes.Index(raw, []byte("[{"))
	if start < 0 {
		// Fall back to looking for an empty array "[]".
		start = bytes.Index(raw, []byte("[]"))
		if start < 0 {
			return nil
		}
		return raw[start : start+2]
	}
	end := bytes.LastIndexByte(raw, ']')
	if end < 0 || end <= start {
		return nil
	}
	return raw[start : end+1]
}

func selectGeneratedScreenshot(configPath string, results []koubouGenerateResult) (string, error) {
	failures := make([]string, 0)
	for _, result := range results {
		if result.Success && strings.TrimSpace(result.Path) != "" {
			path := strings.TrimSpace(result.Path)
			if !filepath.IsAbs(path) {
				cleanPath := filepath.Clean(path)
				parentPrefix := ".." + string(filepath.Separator)
				if cleanPath == ".." || strings.HasPrefix(cleanPath, parentPrefix) {
					return "", fmt.Errorf("koubou output path %q escapes config directory", path)
				}
				path = filepath.Join(filepath.Dir(configPath), cleanPath)
			}
			return path, nil
		}
		if !result.Success && strings.TrimSpace(result.Error) != "" {
			failures = append(failures, strings.TrimSpace(result.Error))
		}
	}

	if len(failures) > 0 {
		return "", fmt.Errorf("koubou generation failed: %s", strings.Join(failures, "; "))
	}
	return "", fmt.Errorf("koubou generation produced no successful output")
}

func copyFile(sourcePath, destinationPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open generated screenshot: %w", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("create final screenshot: %w", err)
	}
	defer destinationFile.Close()

	buffer := make([]byte, 256*1024)
	if _, err := io.CopyBuffer(destinationFile, sourceFile, buffer); err != nil {
		return fmt.Errorf("copy generated screenshot: %w", err)
	}
	return nil
}

func resetKoubouVersionCacheForTest() {
	koubouVersionCacheMu.Lock()
	defer koubouVersionCacheMu.Unlock()

	cachedKoubouBinaryPath = ""
	cachedKoubouResolvedPATH = ""
	cachedKoubouVersionIsGood = false
	cachedKoubouFramesReady = false
}

func normalizeFrameDevice(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})
	return strings.Join(parts, "-")
}
