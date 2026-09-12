package shared

import "testing"

func TestProvisioningProfileExtension(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		profileType string
		want        string
	}{
		{name: "macOS platform", platform: "MAC_OS", profileType: "IOS_APP_STORE", want: ".provisionprofile"},
		{name: "signed macOS platform", platform: "OSX", profileType: "IOS_APP_STORE", want: ".provisionprofile"},
		{name: "native Mac fallback", profileType: "MAC_APP_STORE", want: ".provisionprofile"},
		{name: "Mac Catalyst fallback", profileType: "MAC_CATALYST_APP_STORE", want: ".provisionprofile"},
		{name: "iOS", platform: "IOS", profileType: "IOS_APP_STORE", want: ".mobileprovision"},
		{name: "tvOS", platform: "TV_OS", profileType: "TVOS_APP_STORE", want: ".mobileprovision"},
		{name: "unknown defaults to legacy", platform: "UNKNOWN", profileType: "IOS_APP_STORE", want: ".mobileprovision"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ProvisioningProfileExtension(test.platform, test.profileType); got != test.want {
				t.Fatalf("ProvisioningProfileExtension(%q, %q) = %q, want %q", test.platform, test.profileType, got, test.want)
			}
		})
	}
}

func TestProvisioningProfileExtensionForPlatforms(t *testing.T) {
	if got := ProvisioningProfileExtensionForPlatforms([]string{"iOS", "macOS"}, ""); got != MacOSProvisioningProfileExtension {
		t.Fatalf("ProvisioningProfileExtensionForPlatforms() = %q, want %q", got, MacOSProvisioningProfileExtension)
	}
	if got := ProvisioningProfileExtensionForPlatforms(nil, "IOS_APP_STORE"); got != IOSProvisioningProfileExtension {
		t.Fatalf("ProvisioningProfileExtensionForPlatforms(nil) = %q, want %q", got, IOSProvisioningProfileExtension)
	}
}

func TestIsProvisioningProfilePath(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{path: "profile.mobileprovision", want: true},
		{path: "profile.MOBILEPROVISION", want: true},
		{path: "profile.provisionprofile", want: true},
		{path: "profile.PROVISIONPROFILE", want: true},
		{path: "profile.mobileprovision.enc", want: false},
		{path: "profile.plist", want: false},
		{path: "profile", want: false},
	} {
		if got := IsProvisioningProfilePath(test.path); got != test.want {
			t.Fatalf("IsProvisioningProfilePath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}
