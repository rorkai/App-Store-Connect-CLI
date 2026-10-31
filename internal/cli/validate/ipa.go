package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// uiDeviceFamilyIPad is the UIDeviceFamily value for iPad.
const uiDeviceFamilyIPad = 2

// LocalIPA is the device-family evidence read from a local IPA passed with
// --ipa. The App Store Connect API does not expose a build's device family.
type LocalIPA struct {
	Path           string
	BundleID       string
	Version        string
	BuildNumber    string
	DeviceFamilies []int
	SDKName        string
}

// SupportsIPad reports whether UIDeviceFamily includes iPad.
func (ipa *LocalIPA) SupportsIPad() bool {
	return ipa != nil && slices.Contains(ipa.DeviceFamilies, uiDeviceFamilyIPad)
}

// loadLocalIPA reads the top-level app Info.plist of an IPA without
// contacting App Store Connect.
func loadLocalIPA(path string) (*LocalIPA, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	manifest, err := artifacts.InspectIPAInfoPlist(file, info.Size())
	if err != nil {
		return nil, err
	}
	bundleID := strings.TrimSpace(manifest.BundleID)
	if bundleID == "" {
		return nil, fmt.Errorf("IPA app Info.plist has no CFBundleIdentifier")
	}
	return &LocalIPA{
		Path:           path,
		BundleID:       bundleID,
		Version:        strings.TrimSpace(manifest.Version),
		BuildNumber:    strings.TrimSpace(manifest.BuildNumber),
		DeviceFamilies: manifest.DeviceFamilies,
		SDKName:        strings.TrimSpace(manifest.SDKName),
	}, nil
}

// checkLocalIPAMatchesVersion rejects an IPA that cannot be the selected
// version's binary, so its device family is never applied to another app or
// to a different build of the same app. Its CFBundleShortVersionString must
// match the version string, and with a build attached, its CFBundleVersion
// must match that build.
func checkLocalIPAMatchesVersion(ipa *LocalIPA, platform, appBundleID, versionString string, build *validation.Build) error {
	if ipa == nil {
		return nil
	}
	if normalized := strings.ToUpper(strings.TrimSpace(platform)); normalized != "IOS" {
		return fmt.Errorf("--ipa applies only to IOS App Store versions; the selected version's platform is %s", normalized)
	}
	appBundleID = strings.TrimSpace(appBundleID)
	if appBundleID != "" && ipa.BundleID != appBundleID {
		return fmt.Errorf("--ipa bundle ID %q does not match the app's bundle ID %q", ipa.BundleID, appBundleID)
	}
	if versionString = strings.TrimSpace(versionString); versionString != "" && ipa.Version != versionString {
		return fmt.Errorf("--ipa version (CFBundleShortVersionString) %q does not match App Store version %q", ipa.Version, versionString)
	}
	if build != nil {
		if attached := strings.TrimSpace(build.Version); attached != "" && ipa.BuildNumber != attached {
			return fmt.Errorf("--ipa build number (CFBundleVersion) %q does not match the attached build %q; pass the IPA of the attached build", ipa.BuildNumber, attached)
		}
	}
	return nil
}
