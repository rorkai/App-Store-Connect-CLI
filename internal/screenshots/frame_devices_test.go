package screenshots

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func loadPinnedKoubouFrameCatalog(t *testing.T) map[string]struct{} {
	t.Helper()

	file, err := os.Open(filepath.Join("testdata", "koubou-"+pinnedKoubouVersion+"-frames.txt"))
	if err != nil {
		t.Fatalf("open pinned Koubou frame catalog: %v", err)
	}
	defer file.Close()

	catalog := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		catalog[line] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read pinned Koubou frame catalog: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("pinned Koubou frame catalog is empty")
	}
	return catalog
}

func TestFrameDeviceValuesIncludeTabletWatchAndTVFrames(t *testing.T) {
	want := []string{
		"iphone-air",
		"iphone-17-pro",
		"iphone-17-pro-max",
		"iphone-16e",
		"iphone-17",
		"mac",
		"ipad-pro-13",
		"ipad-pro-11",
		"ipad-air-13",
		"ipad-air-11",
		"ipad-mini",
		"watch-series-11",
		"watch-ultra-3",
		"apple-tv",
		"iphone-duo-outer",
		"iphone-duo-outer-landscape",
		"iphone-duo-inner",
		"iphone-duo-inner-landscape",
	}
	if got := FrameDeviceValues(); !slices.Equal(got, want) {
		t.Fatalf("FrameDeviceValues() = %v, want %v", got, want)
	}
}

func TestFrameDeviceSpecsReferencePinnedKoubouCatalog(t *testing.T) {
	catalog := loadPinnedKoubouFrameCatalog(t)

	for _, device := range supportedFrameDevices {
		spec, ok := frameDeviceKoubouSpecs[device]
		if !ok {
			t.Fatalf("missing Koubou spec for %q", device)
		}
		width, height, ok := spec.outputDimensions()
		if !ok {
			t.Fatalf("%q output size %q/%dx%d does not resolve", device, spec.OutputSize, spec.OutputWidth, spec.OutputHeight)
		}
		dimensions, ok := asc.ScreenshotDimensions(spec.DisplayType)
		if !ok {
			t.Fatalf("%q display type %q is not in the screenshot catalog", device, spec.DisplayType)
		}
		if !slices.Contains(dimensions, asc.ScreenshotDimension{Width: width, Height: height}) {
			t.Fatalf("%q output %dx%d is not an accepted %s size %v", device, width, height, spec.DisplayType, dimensions)
		}
		if spec.Family == "" {
			t.Fatalf("%q has no device family", device)
		}
		if spec.Canvas {
			if len(spec.Colors) != 0 {
				t.Fatalf("canvas device %q must not advertise frame colors", device)
			}
			continue
		}
		if _, ok := catalog[spec.FrameName]; !ok {
			t.Fatalf("%q frame %q is not shipped by Koubou %s", device, spec.FrameName, pinnedKoubouVersion)
		}
		if spec.FrameWidth <= 0 || spec.FrameHeight <= 0 {
			t.Fatalf("%q is missing native frame dimensions", device)
		}
		if len(spec.Colors) == 0 {
			continue
		}
		if spec.Colors[0].FrameName != spec.FrameName {
			t.Fatalf("%q default color frame = %q, want default frame %q", device, spec.Colors[0].FrameName, spec.FrameName)
		}
		seen := map[string]bool{}
		for _, variant := range spec.Colors {
			if seen[variant.ID] {
				t.Fatalf("%q repeats frame color %q", device, variant.ID)
			}
			seen[variant.ID] = true
			if variant.ID != normalizeFrameDevice(variant.ID) {
				t.Fatalf("%q frame color ID %q is not normalized", device, variant.ID)
			}
			if _, ok := catalog[variant.FrameName]; !ok {
				t.Fatalf("%q color %q frame %q is not shipped by Koubou %s", device, variant.ID, variant.FrameName, pinnedKoubouVersion)
			}
		}
	}
}

func TestFrameDeviceSpecsForExpandedFamilies(t *testing.T) {
	tests := []struct {
		device      FrameDevice
		frameName   string
		family      string
		displayType string
		width       int
		height      int
	}{
		{FrameDeviceIPadPro13, "iPad Pro 13 - M4 - Silver - Portrait", "ipad", "APP_IPAD_PRO_3GEN_129", 2064, 2752},
		{FrameDeviceIPadPro11, "iPad Pro 11 - M4 - Silver - Portrait", "ipad", "APP_IPAD_PRO_3GEN_11", 1668, 2388},
		{FrameDeviceIPadAir13, "iPad Air 13 - M2 - Space Gray - Portrait", "ipad", "APP_IPAD_PRO_3GEN_129", 2064, 2752},
		{FrameDeviceIPadAir11, "iPad Air 11 - M2 - Space Gray - Portrait", "ipad", "APP_IPAD_PRO_3GEN_11", 1668, 2388},
		{FrameDeviceIPadMini, "iPad mini - Starlight - Portrait", "ipad", "APP_IPAD_PRO_3GEN_11", 1488, 2266},
		{FrameDeviceWatchSeries11, "Apple Watch S11 - 46mm - Aluminum Jet Black + Sport Band Black", "watch", "APP_WATCH_SERIES_10", 416, 496},
		{FrameDeviceWatchUltra3, "AW Ultra 3 - Black + Ocean Band Black", "watch", "APP_WATCH_ULTRA", 422, 514},
		{FrameDeviceAppleTV, "Apple TV - 4K", "tv", "APP_APPLE_TV", 3840, 2160},
		{"iphone-duo-outer", "iPhone Duo - Night Sky - Outer Closed Portrait", "iphone", "APP_IPHONE_DUO", 1398, 2034},
		{"iphone-duo-outer-landscape", "iPhone Duo - Night Sky - Outer Closed Landscape", "iphone", "APP_IPHONE_DUO", 2034, 1398},
		{"iphone-duo-inner", "iPhone Duo - Night Sky - Inner Open Portrait", "iphone", "APP_IPHONE_DUO", 2007, 2853},
		{"iphone-duo-inner-landscape", "iPhone Duo - Night Sky - Inner Open Landscape", "iphone", "APP_IPHONE_DUO", 2853, 2007},
	}
	for _, test := range tests {
		spec := frameDeviceKoubouSpecs[test.device]
		width, height, _ := spec.outputDimensions()
		if spec.FrameName != test.frameName || spec.Family != test.family || spec.DisplayType != test.displayType || width != test.width || height != test.height {
			t.Fatalf("%q spec = frame %q family %q display %q size %dx%d; want %q %q %q %dx%d",
				test.device, spec.FrameName, spec.Family, spec.DisplayType, width, height,
				test.frameName, test.family, test.displayType, test.width, test.height)
		}
	}
}

func TestResolveFrameColor(t *testing.T) {
	tests := []struct {
		name      string
		device    FrameDevice
		raw       string
		wantID    string
		wantFrame string
	}{
		{name: "default", device: FrameDeviceIPhoneAir, raw: "", wantID: "light-gold", wantFrame: "iPhone Air - Light Gold - Portrait"},
		{name: "normalized", device: FrameDeviceIPhoneAir, raw: " Space_Black ", wantID: "space-black", wantFrame: "iPhone Air - Space Black - Portrait"},
		{name: "ipad", device: FrameDeviceIPadPro13, raw: "space gray", wantID: "space-gray", wantFrame: "iPad Pro 13 - M4 - Space Gray - Portrait"},
		{name: "watch case and band", device: FrameDeviceWatchUltra3, raw: "natural-ocean-band-neon-green", wantID: "natural-ocean-band-neon-green", wantFrame: "AW Ultra 3 - Natural + Ocean Band Neon Green"},
		{name: "no variants", device: FrameDeviceAppleTV, raw: "", wantID: "", wantFrame: "Apple TV - 4K"},
		{name: "canvas", device: FrameDeviceMac, raw: "", wantID: "", wantFrame: "Mac"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec, colorID, err := resolveFrameSpec(test.device, test.raw)
			if err != nil {
				t.Fatalf("resolveFrameSpec() error = %v", err)
			}
			if colorID != test.wantID || spec.FrameName != test.wantFrame {
				t.Fatalf("resolveFrameSpec() = %q/%q, want %q/%q", colorID, spec.FrameName, test.wantID, test.wantFrame)
			}
			if test.raw == "" {
				return
			}
			normalized, err := ResolveFrameColor(test.device, test.raw)
			if err != nil || normalized != test.wantID {
				t.Fatalf("ResolveFrameColor() = %q, %v; want %q", normalized, err, test.wantID)
			}
		})
	}
}

func TestResolveFrameColorRejectsUnknownAndUnsupported(t *testing.T) {
	_, err := ResolveFrameColor(FrameDeviceIPhone17Pro, "gold")
	if err == nil || !strings.Contains(err.Error(), "allowed: silver, cosmic-orange, deep-blue") {
		t.Fatalf("ResolveFrameColor(unknown) error = %v", err)
	}
	for _, device := range []FrameDevice{FrameDeviceMac, FrameDeviceAppleTV} {
		_, err := ResolveFrameColor(device, "black")
		if err == nil || !strings.Contains(err.Error(), "has no frame color variants") {
			t.Fatalf("ResolveFrameColor(%q) error = %v", device, err)
		}
	}
}

func TestFrameDeviceOptionsListFrameColors(t *testing.T) {
	var ipad FrameDeviceOption
	for _, option := range FrameDeviceOptions() {
		if option.ID == string(FrameDeviceIPadAir11) {
			ipad = option
		}
	}
	if ipad.Family != "ipad" || ipad.DefaultFrameColor != "space-gray" {
		t.Fatalf("ipad-air-11 option = %+v", ipad)
	}
	if want := []string{"space-gray", "blue", "purple", "stardust"}; !slices.Equal(ipad.FrameColors, want) {
		t.Fatalf("ipad-air-11 colors = %v, want %v", ipad.FrameColors, want)
	}
}

func TestFrameSpecMatchesColorVariantFrameRef(t *testing.T) {
	if got := resolveFrameDeviceForConfig("iPad Pro 11 - M4 - Space Gray - Portrait", "iphone-air"); got != string(FrameDeviceIPadPro11) {
		t.Fatalf("resolveFrameDeviceForConfig(color variant) = %q", got)
	}
}

func TestFrameDeviceFamilyUsesSpecFamily(t *testing.T) {
	tests := map[FrameDevice]string{
		FrameDeviceIPhoneAir:     "iphone",
		FrameDeviceMac:           "mac",
		FrameDeviceIPadMini:      "ipad",
		FrameDeviceWatchUltra3:   "watch",
		FrameDeviceAppleTV:       "tv",
		FrameDeviceIPhone17Pro:   "iphone",
		FrameDeviceWatchSeries11: "watch",
	}
	for device, want := range tests {
		if got := frameDeviceFamily(device); got != want {
			t.Fatalf("frameDeviceFamily(%q) = %q, want %q", device, got, want)
		}
	}
}

func TestCheckFrameInputAspect(t *testing.T) {
	tests := []struct {
		name          string
		device        FrameDevice
		width, height int
		wantMismatch  bool
	}{
		{name: "native watch series 11 capture", device: FrameDeviceWatchSeries11, width: 416, height: 496},
		{name: "native watch ultra 3 capture", device: FrameDeviceWatchUltra3, width: 422, height: 514},
		{name: "native apple tv 4k capture", device: FrameDeviceAppleTV, width: 3840, height: 2160},
		{name: "apple tv hd capture", device: FrameDeviceAppleTV, width: 1920, height: 1080},
		{name: "iphone 17 pro max capture on iphone air", device: FrameDeviceIPhoneAir, width: 1320, height: 2868},
		{name: "ipad air 11 capture on ipad air 11", device: FrameDeviceIPadAir11, width: 1640, height: 2360},
		{name: "mac capture", device: FrameDeviceMac, width: 1440, height: 900},
		{name: "iphone capture on watch", device: FrameDeviceWatchUltra3, width: 1206, height: 2622, wantMismatch: true},
		{name: "ipad capture on tv", device: FrameDeviceAppleTV, width: 2064, height: 2752, wantMismatch: true},
		{name: "4:3 capture on mac canvas", device: FrameDeviceMac, width: 1200, height: 900, wantMismatch: true},
		{name: "landscape iphone capture on portrait frame", device: FrameDeviceIPhone17Pro, width: 2622, height: 1206, wantMismatch: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "raw.png")
			writeMinimalPNG(t, path, test.width, test.height)
			mismatch, err := CheckFrameInputAspect(path, test.device)
			if err != nil {
				t.Fatalf("CheckFrameInputAspect() error = %v", err)
			}
			if got := mismatch != nil; got != test.wantMismatch {
				t.Fatalf("CheckFrameInputAspect() mismatch = %+v, want mismatch %v", mismatch, test.wantMismatch)
			}
			if mismatch == nil {
				return
			}
			if mismatch.InputWidth != test.width || mismatch.InputHeight != test.height {
				t.Fatalf("input dimensions = %dx%d, want %dx%d", mismatch.InputWidth, mismatch.InputHeight, test.width, test.height)
			}
			wantWidth, wantHeight, _ := frameDeviceKoubouSpecs[test.device].outputDimensions()
			if mismatch.ScreenWidth != wantWidth || mismatch.ScreenHeight != wantHeight {
				t.Fatalf("screen dimensions = %dx%d, want %dx%d", mismatch.ScreenWidth, mismatch.ScreenHeight, wantWidth, wantHeight)
			}
		})
	}
}

func TestCheckFrameInputAspectReportsUnreadableInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.png")
	if _, err := CheckFrameInputAspect(path, FrameDeviceWatchUltra3); err == nil {
		t.Fatal("CheckFrameInputAspect() error = nil, want missing input error")
	}
}
