package shared

import (
	"path/filepath"
	"strings"
)

const (
	// IOSProvisioningProfileExtension is the conventional extension for iOS,
	// tvOS, and other non-macOS provisioning profiles.
	IOSProvisioningProfileExtension = ".mobileprovision"
	// MacOSProvisioningProfileExtension is the conventional extension for
	// provisioning profiles used by macOS applications.
	MacOSProvisioningProfileExtension = ".provisionprofile"
)

// IsProvisioningProfilePath reports whether path has a recognized
// provisioning-profile extension. The extension is only a file-type hint;
// callers must still parse and validate the profile contents.
func IsProvisioningProfilePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case IOSProvisioningProfileExtension, MacOSProvisioningProfileExtension:
		return true
	default:
		return false
	}
}

// ProvisioningProfileExtension returns the conventional extension for a
// profile. A known platform takes precedence; the profile type is used as a
// fallback because profile creation preflight runs before the API returns the
// created resource's platform attribute.
func ProvisioningProfileExtension(platform, profileType string) string {
	switch strings.ToUpper(strings.TrimSpace(platform)) {
	case "MAC_OS", "OSX", "MACOS", "MACOSX":
		return MacOSProvisioningProfileExtension
	case "IOS", "TV_OS", "TVOS", "VISION_OS":
		return IOSProvisioningProfileExtension
	}

	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(profileType)), "MAC_") {
		return MacOSProvisioningProfileExtension
	}
	return IOSProvisioningProfileExtension
}

// ProvisioningProfileExtensionForPlatforms returns the conventional extension
// for a profile whose parsed payload may advertise more than one platform.
// macOS wins when present; an unknown or empty platform preserves the legacy
// iOS-compatible default.
func ProvisioningProfileExtensionForPlatforms(platforms []string, profileType string) string {
	for _, platform := range platforms {
		if ProvisioningProfileExtension(platform, "") == MacOSProvisioningProfileExtension {
			return MacOSProvisioningProfileExtension
		}
	}
	return ProvisioningProfileExtension("", profileType)
}
