package screenshots

import (
	"fmt"
	"strings"
)

// FrameDevice identifies a supported frame profile.
type FrameDevice string

const (
	FrameDeviceIPhoneAir     FrameDevice = "iphone-air"
	FrameDeviceIPhone17Pro   FrameDevice = "iphone-17-pro"
	FrameDeviceIPhone17PM    FrameDevice = "iphone-17-pro-max"
	FrameDeviceIPhone16e     FrameDevice = "iphone-16e"
	FrameDeviceIPhone17      FrameDevice = "iphone-17"
	FrameDeviceMac           FrameDevice = "mac"
	FrameDeviceIPadPro13     FrameDevice = "ipad-pro-13"
	FrameDeviceIPadPro11     FrameDevice = "ipad-pro-11"
	FrameDeviceIPadAir13     FrameDevice = "ipad-air-13"
	FrameDeviceIPadAir11     FrameDevice = "ipad-air-11"
	FrameDeviceIPadMini      FrameDevice = "ipad-mini"
	FrameDeviceWatchSeries11 FrameDevice = "watch-series-11"
	FrameDeviceWatchUltra3   FrameDevice = "watch-ultra-3"
	FrameDeviceAppleTV       FrameDevice = "apple-tv"

	FrameDeviceIPhoneDuoOuter          FrameDevice = "iphone-duo-outer"
	FrameDeviceIPhoneDuoOuterLandscape FrameDevice = "iphone-duo-outer-landscape"
	FrameDeviceIPhoneDuoInner          FrameDevice = "iphone-duo-inner"
	FrameDeviceIPhoneDuoInnerLandscape FrameDevice = "iphone-duo-inner-landscape"
)

const (
	frameFamilyIPhone = "iphone"
	frameFamilyIPad   = "ipad"
	frameFamilyWatch  = "watch"
	frameFamilyTV     = "tv"
	frameFamilyMac    = "mac"
)

// supportedFrameDevices is the CLI display order. New devices are appended so
// the established order of list-frame-devices output stays stable.
var supportedFrameDevices = []FrameDevice{
	FrameDeviceIPhoneAir,
	FrameDeviceIPhone17Pro,
	FrameDeviceIPhone17PM,
	FrameDeviceIPhone16e,
	FrameDeviceIPhone17,
	FrameDeviceMac,
	FrameDeviceIPadPro13,
	FrameDeviceIPadPro11,
	FrameDeviceIPadAir13,
	FrameDeviceIPadAir11,
	FrameDeviceIPadMini,
	FrameDeviceWatchSeries11,
	FrameDeviceWatchUltra3,
	FrameDeviceAppleTV,
	FrameDeviceIPhoneDuoOuter,
	FrameDeviceIPhoneDuoOuterLandscape,
	FrameDeviceIPhoneDuoInner,
	FrameDeviceIPhoneDuoInnerLandscape,
}

type frameColorVariant struct {
	ID        string
	FrameName string
}

type frameDeviceKoubouSpec struct {
	FrameName string // default Koubou frame name; equals Colors[0].FrameName when Colors is set
	Aliases   []string
	// OutputSize is a Koubou named size (e.g. "iPhone6_9"). When Koubou has
	// no named size for the App Store upload size, OutputWidth and
	// OutputHeight carry explicit pixel dimensions instead.
	OutputSize   string
	OutputWidth  int
	OutputHeight int
	DisplayType  string
	Canvas       bool // true = plain canvas, no device bezel; screenshot scaled to fill
	Family       string
	// FrameWidth and FrameHeight are the native pixel dimensions of the
	// pinned Koubou frame PNGs. Every color variant of a device shares them.
	// They let text layouts size the framed device without reading Koubou's
	// frame cache.
	FrameWidth  int
	FrameHeight int
	Colors      []frameColorVariant // first entry is the default color
}

// outputDimensions returns the upload pixel size Koubou renders for spec.
func (spec frameDeviceKoubouSpec) outputDimensions() (int, int, bool) {
	if spec.OutputWidth > 0 && spec.OutputHeight > 0 {
		return spec.OutputWidth, spec.OutputHeight, true
	}
	return resolveKoubouOutputSize(spec.OutputSize)
}

// koubouOutputSize returns the YAML output_size value for spec.
func (spec frameDeviceKoubouSpec) koubouOutputSize() koubouOutputSize {
	if spec.OutputWidth > 0 && spec.OutputHeight > 0 {
		return dimsOutputSize(spec.OutputWidth, spec.OutputHeight)
	}
	return namedOutputSize(spec.OutputSize)
}

// frameColors expands a Koubou frame-name template for each color label. The
// template contains one %s placeholder for the label.
func frameColors(template string, labels ...string) []frameColorVariant {
	variants := make([]frameColorVariant, 0, len(labels))
	for _, label := range labels {
		variants = append(variants, frameColorVariant{
			ID:        normalizeFrameDevice(strings.ReplaceAll(label, "+", " ")),
			FrameName: fmt.Sprintf(template, label),
		})
	}
	return variants
}

// Keeps the existing asc device slugs while delegating rendering to pinned
// Koubou v0.21.0 frame names. Every frame name and color variant below is
// checked against testdata/koubou-0.21.0-frames.txt.
var frameDeviceKoubouSpecs = map[FrameDevice]frameDeviceKoubouSpec{
	FrameDeviceIPhoneAir: {
		FrameName:   "iPhone Air - Light Gold - Portrait",
		Aliases:     []string{"iPhone 16 Pro - White Titanium - Portrait"},
		OutputSize:  "iPhone6_9_alt",
		DisplayType: "APP_IPHONE_69",
		Family:      frameFamilyIPhone,
		FrameWidth:  1380,
		FrameHeight: 2880,
		Colors:      frameColors("iPhone Air - %s - Portrait", "Light Gold", "Cloud White", "Sky Blue", "Space Black"),
	},
	FrameDeviceIPhone17PM: {
		FrameName:   "iPhone 17 Pro Max - Silver - Portrait",
		Aliases:     []string{"iPhone 16 Pro Max - White Titanium - Portrait"},
		OutputSize:  "iPhone6_9",
		DisplayType: "APP_IPHONE_69",
		Family:      frameFamilyIPhone,
		FrameWidth:  1470,
		FrameHeight: 3000,
		Colors:      frameColors("iPhone 17 Pro Max - %s - Portrait", "Silver", "Cosmic Orange", "Deep Blue"),
	},
	FrameDeviceIPhone17Pro: {
		FrameName:   "iPhone 17 Pro - Silver - Portrait",
		Aliases:     []string{"iPhone 15 Pro - White Titanium - Portrait"},
		OutputSize:  "iPhone6_3",
		DisplayType: "APP_IPHONE_61",
		Family:      frameFamilyIPhone,
		FrameWidth:  1350,
		FrameHeight: 2760,
		Colors:      frameColors("iPhone 17 Pro - %s - Portrait", "Silver", "Cosmic Orange", "Deep Blue"),
	},
	FrameDeviceIPhone17: {
		FrameName:   "iPhone 17 - White - Portrait",
		Aliases:     []string{"iPhone 17 - Teal - Portrait", "iPhone 14 Pro Portrait"},
		OutputSize:  "iPhone6_3",
		DisplayType: "APP_IPHONE_61",
		Family:      frameFamilyIPhone,
		FrameWidth:  1350,
		FrameHeight: 2760,
		Colors:      frameColors("iPhone 17 - %s - Portrait", "White", "Black", "Lavender", "Mist Blue", "Sage"),
	},
	FrameDeviceIPhone16e: {
		// Koubou ships no iPhone 16e frame; the iPhone 16 frame matches its
		// front profile. Only the colors the 16e ships in are offered.
		FrameName:   "iPhone 16 - White - Portrait",
		Aliases:     []string{"iPhone 16e - White - Portrait"},
		OutputSize:  "iPhone6_1",
		DisplayType: "APP_IPHONE_61",
		Family:      frameFamilyIPhone,
		FrameWidth:  1359,
		FrameHeight: 2736,
		Colors:      frameColors("iPhone 16 - %s - Portrait", "White", "Black"),
	},
	FrameDeviceMac: {
		FrameName:   "Mac",
		OutputSize:  "AppDesktop_2880",
		DisplayType: "APP_DESKTOP",
		Canvas:      true,
		Family:      frameFamilyMac,
	},
	FrameDeviceIPadPro13: {
		FrameName:   "iPad Pro 13 - M4 - Silver - Portrait",
		OutputSize:  "iPadPro13",
		DisplayType: "APP_IPAD_PRO_3GEN_129",
		Family:      frameFamilyIPad,
		FrameWidth:  2300,
		FrameHeight: 3000,
		Colors:      frameColors("iPad Pro 13 - M4 - %s - Portrait", "Silver", "Space Gray"),
	},
	FrameDeviceIPadPro11: {
		FrameName:   "iPad Pro 11 - M4 - Silver - Portrait",
		OutputSize:  "iPadPro11",
		DisplayType: "APP_IPAD_PRO_3GEN_11",
		Family:      frameFamilyIPad,
		FrameWidth:  1880,
		FrameHeight: 2640,
		Colors:      frameColors("iPad Pro 11 - M4 - %s - Portrait", "Silver", "Space Gray"),
	},
	FrameDeviceIPadAir13: {
		FrameName:   "iPad Air 13 - M2 - Space Gray - Portrait",
		OutputSize:  "iPadPro13",
		DisplayType: "APP_IPAD_PRO_3GEN_129",
		Family:      frameFamilyIPad,
		FrameWidth:  2300,
		FrameHeight: 2980,
		Colors:      frameColors("iPad Air 13 - M2 - %s - Portrait", "Space Gray", "Blue", "Purple", "Stardust"),
	},
	FrameDeviceIPadAir11: {
		FrameName:   "iPad Air 11 - M2 - Space Gray - Portrait",
		OutputSize:  "iPadPro11",
		DisplayType: "APP_IPAD_PRO_3GEN_11",
		Family:      frameFamilyIPad,
		FrameWidth:  1900,
		FrameHeight: 2620,
		Colors:      frameColors("iPad Air 11 - M2 - %s - Portrait", "Space Gray", "Blue", "Purple", "Stardust"),
	},
	FrameDeviceIPadMini: {
		// Koubou has no named iPad mini size; 1488x2266 is the iPad mini
		// (A17 Pro) screen and an accepted 11-inch iPad upload size.
		FrameName:    "iPad mini - Starlight - Portrait",
		OutputWidth:  1488,
		OutputHeight: 2266,
		DisplayType:  "APP_IPAD_PRO_3GEN_11",
		Family:       frameFamilyIPad,
		FrameWidth:   1780,
		FrameHeight:  2550,
		Colors:       frameColors("iPad mini - %s - Portrait", "Starlight"),
	},
	FrameDeviceWatchSeries11: {
		// Koubou locates the screen by flood-filling transparent pixels from
		// the frame edge. Titanium Milanese Loop and some Titanium Sport Band
		// variants enclose transparent band pixels, so the detected screen
		// exceeds the 416x496 display and the screenshot is misplaced. Only
		// variants whose detected screen is exactly 416x496 are offered.
		FrameName:    "Apple Watch S11 - 46mm - Aluminum Jet Black + Sport Band Black",
		OutputWidth:  416,
		OutputHeight: 496,
		DisplayType:  "APP_WATCH_SERIES_10",
		Family:       frameFamilyWatch,
		FrameWidth:   560,
		FrameHeight:  880,
		Colors: frameColors(
			"Apple Watch S11 - 46mm - %s",
			"Aluminum Jet Black + Sport Band Black",
			"Aluminum Jet Black + Sport Loop Dark Gray",
			"Aluminum Rose Gold + Sport Band Light Blush",
			"Aluminum Rose Gold + Sport Loop Purple Fog",
			"Aluminum Silver + Sport Band Neon Yellow",
			"Aluminum Silver + Sport Band Purple Fog",
			"Aluminum Silver + Sport Loop Forest",
			"Aluminum Silver + Sport Loop Neon Yellow",
			"Aluminum Space Gray + Sport Band Anchor Blue",
			"Aluminum Space Gray + Sport Band Black",
			"Aluminum Space Gray + Sport Loop Anchor Blue",
			"Aluminum Space Gray + Sport Loop Forest",
			"Titanium Gold + Magnetic Link Sage Gray",
			"Titanium Natural + Magnetic Link Caramel",
			"Titanium Slate + Magnetic Link Navy",
		),
	},
	FrameDeviceWatchUltra3: {
		// Alpine, Trail, and Milanese Loop variants enclose transparent band
		// pixels that Koubou's screen detection treats as screen; only the
		// Ocean Band variants detect the exact 422x514 display.
		FrameName:    "AW Ultra 3 - Black + Ocean Band Black",
		OutputWidth:  422,
		OutputHeight: 514,
		DisplayType:  "APP_WATCH_ULTRA",
		Family:       frameFamilyWatch,
		FrameWidth:   600,
		FrameHeight:  960,
		Colors: frameColors(
			"AW Ultra 3 - %s",
			"Black + Ocean Band Black",
			"Black + Ocean Band Anchor Blue",
			"Natural + Ocean Band Anchor Blue",
			"Natural + Ocean Band Neon Green",
		),
	},
	FrameDeviceAppleTV: {
		FrameName:    "Apple TV - 4K",
		OutputWidth:  3840,
		OutputHeight: 2160,
		DisplayType:  "APP_APPLE_TV",
		Family:       frameFamilyTV,
		FrameWidth:   4300,
		FrameHeight:  2780,
	},
	FrameDeviceIPhoneDuoOuter: {
		FrameName:   "iPhone Duo - Night Sky - Outer Closed Portrait",
		OutputSize:  "iPhoneDuo_Outer",
		DisplayType: "APP_IPHONE_DUO",
		Family:      frameFamilyIPhone,
		FrameWidth:  1574,
		FrameHeight: 2194,
		Colors:      frameColors("iPhone Duo - %s - Outer Closed Portrait", "Night Sky", "Star White"),
	},
	FrameDeviceIPhoneDuoOuterLandscape: {
		FrameName:   "iPhone Duo - Night Sky - Outer Closed Landscape",
		OutputSize:  "iPhoneDuo_Outer_Landscape",
		DisplayType: "APP_IPHONE_DUO",
		Family:      frameFamilyIPhone,
		FrameWidth:  2194,
		FrameHeight: 1574,
		Colors:      frameColors("iPhone Duo - %s - Outer Closed Landscape", "Night Sky", "Star White"),
	},
	FrameDeviceIPhoneDuoInner: {
		FrameName:   "iPhone Duo - Night Sky - Inner Open Portrait",
		OutputSize:  "iPhoneDuo_Inner",
		DisplayType: "APP_IPHONE_DUO",
		Family:      frameFamilyIPhone,
		FrameWidth:  2247,
		FrameHeight: 3093,
		Colors:      frameColors("iPhone Duo - %s - Inner Open Portrait", "Night Sky", "Star White"),
	},
	FrameDeviceIPhoneDuoInnerLandscape: {
		FrameName:   "iPhone Duo - Night Sky - Inner Open Landscape",
		OutputSize:  "iPhoneDuo_Inner_Landscape",
		DisplayType: "APP_IPHONE_DUO",
		Family:      frameFamilyIPhone,
		FrameWidth:  3093,
		FrameHeight: 2247,
		Colors:      frameColors("iPhone Duo - %s - Inner Open Landscape", "Night Sky", "Star White"),
	},
}

// FrameDeviceOption describes one supported frame device value.
type FrameDeviceOption struct {
	ID                string   `json:"id"`
	Default           bool     `json:"default"`
	Family            string   `json:"family,omitempty"`
	DefaultFrameColor string   `json:"defaultFrameColor,omitempty"`
	FrameColors       []string `json:"frameColors,omitempty"`
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
		spec := frameDeviceKoubouSpecs[device]
		option := FrameDeviceOption{
			ID:          string(device),
			Default:     device == defaultDevice,
			Family:      spec.Family,
			FrameColors: FrameColorValues(device),
		}
		if len(spec.Colors) > 0 {
			option.DefaultFrameColor = spec.Colors[0].ID
		}
		options = append(options, option)
	}
	return options
}

// FrameColorValues returns the --frame-color values for device, default first.
func FrameColorValues(device FrameDevice) []string {
	spec := frameDeviceKoubouSpecs[device]
	values := make([]string, 0, len(spec.Colors))
	for _, variant := range spec.Colors {
		values = append(values, variant.ID)
	}
	return values
}

// ResolveFrameColor normalizes and validates a --frame-color value for device.
// An empty value selects the device's default color.
func ResolveFrameColor(device FrameDevice, raw string) (string, error) {
	_, colorID, err := resolveFrameSpec(device, raw)
	return colorID, err
}

// resolveFrameSpec returns the device spec with FrameName set to the selected
// color variant, plus the normalized color ID.
func resolveFrameSpec(device FrameDevice, rawColor string) (frameDeviceKoubouSpec, string, error) {
	spec, ok := frameDeviceKoubouSpecs[device]
	if !ok {
		return frameDeviceKoubouSpec{}, "", fmt.Errorf("no Koubou mapping configured for device %q", device)
	}
	wanted := normalizeFrameDevice(rawColor)
	if len(spec.Colors) == 0 {
		if wanted != "" {
			return frameDeviceKoubouSpec{}, "", fmt.Errorf("device %s has no frame color variants", device)
		}
		return spec, "", nil
	}
	if wanted == "" {
		return spec, spec.Colors[0].ID, nil
	}
	for _, variant := range spec.Colors {
		if variant.ID == wanted {
			spec.FrameName = variant.FrameName
			return spec, variant.ID, nil
		}
	}
	return frameDeviceKoubouSpec{}, "", fmt.Errorf(
		"unsupported frame color %q for device %s (allowed: %s)",
		rawColor,
		device,
		strings.Join(FrameColorValues(device), ", "),
	)
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

func frameDeviceFamily(device FrameDevice) string {
	if spec, ok := frameDeviceKoubouSpecs[device]; ok && spec.Family != "" {
		return spec.Family
	}
	return frameFamilyIPhone
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
	for _, variant := range spec.Colors {
		if strings.EqualFold(variant.FrameName, frameRef) {
			return true
		}
	}
	return false
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
