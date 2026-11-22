package asc

import "testing"

func TestValidateIPhoneDuoDimensions(t *testing.T) {
	for _, displayType := range []string{"APP_IPHONE_DUO", "IMESSAGE_APP_IPHONE_DUO"} {
		for _, dimensions := range [][2]int{{1398, 2034}, {2034, 1398}, {2007, 2853}, {2853, 2007}} {
			if err := ValidateScreenshotDimensionsForSize("duo.png", dimensions[0], dimensions[1], displayType); err != nil {
				t.Fatalf("%s: %v", displayType, err)
			}
		}
		if err := ValidateScreenshotDimensionsForSize("other.png", 1320, 2868, displayType); err == nil {
			t.Fatalf("%s accepted a standard iPhone screenshot", displayType)
		}
		if got := CanonicalScreenshotDisplayTypeForAPI(displayType); got != displayType {
			t.Fatalf("canonical type = %q", got)
		}
	}
}
