package screenshots

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// FrameDevice identifies a cached Apple device frame family.
type FrameDevice string

const (
	FrameDeviceIPhoneAir    FrameDevice = "iphone-air"
	FrameDeviceIPhone17Pro  FrameDevice = "iphone-17-pro"
	FrameDeviceIPhone17PM   FrameDevice = "iphone-17-pro-max"
	FrameDeviceIPhone16e    FrameDevice = "iphone-16e"
	FrameDeviceIPhone17     FrameDevice = "iphone-17"
	defaultFrameCacheSubdir             = ".asc/frames/apple"
	defaultFrameBleedPX                 = 24
	outerAlphaThreshold     uint8       = 8
	innerAlphaThreshold     uint8       = 32
)

const defaultIPhoneAirPortrait = "iPhone Air - Light Gold - Portrait.png"

var supportedFrameDevices = []FrameDevice{
	FrameDeviceIPhoneAir,
	FrameDeviceIPhone17Pro,
	FrameDeviceIPhone17PM,
	FrameDeviceIPhone16e,
	FrameDeviceIPhone17,
}

// FrameRequest holds options for composing one screenshot into an Apple frame.
type FrameRequest struct {
	InputPath   string // required source screenshot path
	OutputPath  string // required destination PNG path
	Device      string // device slug; defaults to iphone-air when empty
	FrameRoot   string // optional override; defaults to ~/.asc/frames/apple
	ScreenBleed int    // optional extra pixels to extend under bezel AA edge
}

// FrameResult is the structured output for a composed frame image.
type FrameResult struct {
	Path      string `json:"path"`
	FramePath string `json:"frame_path"`
	Device    string `json:"device"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// FrameDeviceOption describes one supported frame device value.
type FrameDeviceOption struct {
	ID      string `json:"id"`
	Default bool   `json:"default"`
}

// DefaultFrameDevice returns the default device used by frame composition.
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

// Frame composes a screenshot under a selected Apple hardware frame.
func Frame(ctx context.Context, req FrameRequest) (*FrameResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	inputPath := strings.TrimSpace(req.InputPath)
	if inputPath == "" {
		return nil, fmt.Errorf("input path is required")
	}
	outputPath := strings.TrimSpace(req.OutputPath)
	if outputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}

	device, err := ParseFrameDevice(req.Device)
	if err != nil {
		return nil, err
	}

	framePath, err := resolveFramePath(req.FrameRoot, device)
	if err != nil {
		return nil, err
	}

	raw, err := decodeImageFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read input screenshot: %w", err)
	}
	frame, err := decodeImageFile(framePath)
	if err != nil {
		return nil, fmt.Errorf("read frame image: %w", err)
	}

	bleed := req.ScreenBleed
	if bleed == 0 {
		bleed = defaultFrameBleedPX
	}
	if bleed < 0 {
		return nil, fmt.Errorf("screen bleed must be >= 0")
	}

	composed, err := composeFramedImage(raw, frame, bleed)
	if err != nil {
		return nil, err
	}

	absOut, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}
	outFile, err := os.Create(absOut)
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()
	if err := png.Encode(outFile, composed); err != nil {
		return nil, fmt.Errorf("encode output png: %w", err)
	}

	absFrame, _ := filepath.Abs(framePath)
	return &FrameResult{
		Path:      absOut,
		FramePath: absFrame,
		Device:    string(device),
		Width:     composed.Bounds().Dx(),
		Height:    composed.Bounds().Dy(),
	}, nil
}

func resolveFramePath(frameRoot string, device FrameDevice) (string, error) {
	root, err := resolveFrameRoot(frameRoot)
	if err != nil {
		return "", err
	}
	pngDir := filepath.Join(root, string(device), "png")

	entries, err := os.ReadDir(pngDir)
	if err != nil {
		return "", fmt.Errorf("read frame directory %q: %w", pngDir, err)
	}

	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".png") {
			continue
		}
		if !strings.Contains(lower, "portrait") {
			continue
		}
		candidates = append(candidates, name)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no portrait PNG frames found for %q in %q", device, pngDir)
	}
	sort.Strings(candidates)

	preferred := defaultPortraitForDevice(device)
	if preferred != "" {
		for _, name := range candidates {
			if strings.EqualFold(name, preferred) {
				return filepath.Join(pngDir, name), nil
			}
		}
	}
	return filepath.Join(pngDir, candidates[0]), nil
}

func defaultPortraitForDevice(device FrameDevice) string {
	if device == FrameDeviceIPhoneAir {
		return defaultIPhoneAirPortrait
	}
	return ""
}

func resolveFrameRoot(override string) (string, error) {
	trimmed := strings.TrimSpace(override)
	if trimmed != "" {
		return filepath.Abs(trimmed)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(homeDir, defaultFrameCacheSubdir), nil
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

func decodeImageFile(path string) (image.Image, error) {
	if err := asc.ValidateImageFile(path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

type point struct {
	x int
	y int
}

func composeFramedImage(raw image.Image, frame image.Image, bleed int) (*image.RGBA, error) {
	rawFlat := flattenOnWhite(raw)
	frameRGBA := toRGBA(frame)
	frameBounds := frameRGBA.Bounds()
	fw, fh := frameBounds.Dx(), frameBounds.Dy()

	screenMask, screenRect, err := detectScreenMask(frameRGBA)
	if err != nil {
		return nil, err
	}

	base := image.NewRGBA(image.Rect(0, 0, fw, fh))
	draw.Draw(base, base.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	for y := 0; y < fh; y++ {
		for x := 0; x < fw; x++ {
			if shouldDarkenBaseAt(frameRGBA, screenMask, x, y, fw, fh) {
				base.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}

	targetW := screenRect.Dx() + 2*bleed
	targetH := screenRect.Dy() + 2*bleed
	if targetW <= 0 || targetH <= 0 {
		return nil, fmt.Errorf("invalid target fit area %dx%d", targetW, targetH)
	}

	sw, sh := rawFlat.Bounds().Dx(), rawFlat.Bounds().Dy()
	scale := math.Max(float64(targetW)/float64(sw), float64(targetH)/float64(sh))
	fitW := int(math.Ceil(float64(sw) * scale))
	fitH := int(math.Ceil(float64(sh) * scale))
	if fitW <= 0 || fitH <= 0 {
		return nil, fmt.Errorf("invalid resized image dimensions %dx%d", fitW, fitH)
	}
	fit := resizeNearest(rawFlat, fitW, fitH)

	screenCX := screenRect.Min.X + screenRect.Dx()/2
	screenCY := screenRect.Min.Y + screenRect.Dy()/2
	dstX := screenCX - fitW/2
	dstY := screenCY - fitH/2

	layer := image.NewRGBA(base.Bounds())
	draw.Draw(layer, image.Rect(dstX, dstY, dstX+fitW, dstY+fitH), fit, image.Point{}, draw.Src)

	for y := 0; y < fh; y++ {
		for x := 0; x < fw; x++ {
			if screenMask[maskIndex(x, y, fw)] {
				continue
			}
			i := y*layer.Stride + x*4
			layer.Pix[i+3] = 0
		}
	}

	draw.Draw(base, base.Bounds(), layer, image.Point{}, draw.Over)
	draw.Draw(base, base.Bounds(), frameRGBA, image.Point{}, draw.Over)
	return base, nil
}

func detectScreenMask(frame *image.RGBA) ([]bool, image.Rectangle, error) {
	fw, fh := frame.Bounds().Dx(), frame.Bounds().Dy()
	outside := make([]bool, fw*fh)
	queue := make([]point, 0, fw*4+fh*4)

	push := func(x, y int) {
		if x < 0 || x >= fw || y < 0 || y >= fh {
			return
		}
		i := maskIndex(x, y, fw)
		if outside[i] {
			return
		}
		if alphaAt(frame, x, y) > outerAlphaThreshold {
			return
		}
		outside[i] = true
		queue = append(queue, point{x: x, y: y})
	}

	for x := 0; x < fw; x++ {
		push(x, 0)
		push(x, fh-1)
	}
	for y := 0; y < fh; y++ {
		push(0, y)
		push(fw-1, y)
	}
	for head := 0; head < len(queue); head++ {
		p := queue[head]
		push(p.x+1, p.y)
		push(p.x-1, p.y)
		push(p.x, p.y+1)
		push(p.x, p.y-1)
	}

	screenMask := make([]bool, fw*fh)
	minX, minY := fw, fh
	maxX, maxY := -1, -1
	for y := 0; y < fh; y++ {
		for x := 0; x < fw; x++ {
			i := maskIndex(x, y, fw)
			if outside[i] || alphaAt(frame, x, y) > innerAlphaThreshold {
				continue
			}

			screenMask[i] = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	if maxX < minX || maxY < minY {
		return nil, image.Rectangle{}, fmt.Errorf("failed to detect inner screen area from frame alpha")
	}
	return screenMask, image.Rect(minX, minY, maxX+1, maxY+1), nil
}

func flattenOnWhite(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)
	return dst
}

func toRGBA(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

func resizeNearest(src image.Image, dstW, dstH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	for y := 0; y < dstH; y++ {
		srcY := int(float64(y) * float64(srcH) / float64(dstH))
		if srcY >= srcH {
			srcY = srcH - 1
		}
		for x := 0; x < dstW; x++ {
			srcX := int(float64(x) * float64(srcW) / float64(dstW))
			if srcX >= srcW {
				srcX = srcW - 1
			}
			dst.Set(x, y, src.At(srcBounds.Min.X+srcX, srcBounds.Min.Y+srcY))
		}
	}

	return dst
}

func alphaAt(img *image.RGBA, x, y int) uint8 {
	return img.Pix[y*img.Stride+x*4+3]
}

func shouldDarkenBaseAt(frame *image.RGBA, screenMask []bool, x, y, width, height int) bool {
	alpha := alphaAt(frame, x, y)
	if alpha == 0 {
		return false
	}
	if alpha == 255 {
		return true
	}
	return touchesScreen(screenMask, x, y, width, height)
}

func touchesScreen(screenMask []bool, x, y, width, height int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx < 0 || nx >= width || ny < 0 || ny >= height {
				continue
			}
			if screenMask[maskIndex(nx, ny, width)] {
				return true
			}
		}
	}
	return false
}

func maskIndex(x, y, width int) int {
	return y*width + x
}
