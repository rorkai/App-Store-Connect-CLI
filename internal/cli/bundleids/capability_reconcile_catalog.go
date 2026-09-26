package bundleids

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

var dataProtectionOptions = map[string]string{
	"NSFileProtectionComplete":                             "COMPLETE_PROTECTION",
	"NSFileProtectionCompleteUnlessOpen":                   "PROTECTED_UNLESS_OPEN",
	"NSFileProtectionCompleteUntilFirstUserAuthentication": "PROTECTED_UNTIL_FIRST_USER_AUTH",
}

type entitlementCapability struct {
	Key                   string
	Capability            string
	WebCommand            string
	UnsupportedCapability string
	ValueKind             entitlementValueKind
	Settings              func(any) []asc.CapabilitySetting
}

type entitlementValueKind uint8

const (
	entitlementValueAny entitlementValueKind = iota
	entitlementValueBoolean
	entitlementValueString
	entitlementValueStringArray
)

func entitlementCapabilityCatalog() []entitlementCapability {
	return []entitlementCapability{
		{Key: "aps-environment", Capability: "PUSH_NOTIFICATIONS", ValueKind: entitlementValueString},
		{Key: "com.apple.developer.aps-environment", Capability: "PUSH_NOTIFICATIONS", ValueKind: entitlementValueString},
		{Key: "com.apple.developer.healthkit", Capability: "HEALTHKIT", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.icloud-container-identifiers", Capability: "ICLOUD", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.icloud-services", Capability: "ICLOUD", ValueKind: entitlementValueStringArray, Settings: iCloudServicesSettings},
		{Key: "com.apple.developer.ubiquity-container-identifiers", Capability: "ICLOUD", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.ubiquity-kvstore-identifier", Capability: "ICLOUD", ValueKind: entitlementValueString},
		{Key: "com.apple.security.application-groups", Capability: "APP_GROUPS", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.associated-domains", Capability: "ASSOCIATED_DOMAINS", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.networking.vpn.api", Capability: "PERSONAL_VPN", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.external-accessory.wireless-configuration", Capability: "WIRELESS_ACCESSORY_CONFIGURATION", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.in-app-payments", Capability: "APPLE_PAY", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.default-data-protection", Capability: "DATA_PROTECTION", ValueKind: entitlementValueString, Settings: dataProtectionSettings},
		{Key: "com.apple.developer.siri", Capability: "SIRIKIT", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.networking.networkextension", Capability: "NETWORK_EXTENSIONS", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.networking.multipath", Capability: "MULTIPATH", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.networking.HotspotConfiguration", Capability: "HOT_SPOT", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.nfc.readersession.formats", Capability: "NFC_TAG_READING", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.ClassKit-environment", Capability: "CLASSKIT", ValueKind: entitlementValueString},
		{Key: "com.apple.developer.authentication-services.autofill-credential-provider", Capability: "AUTOFILL_CREDENTIAL_PROVIDER", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.networking.wifi-info", Capability: "ACCESS_WIFI_INFORMATION", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.networking.custom-protocol", Capability: "NETWORK_CUSTOM_PROTOCOL"},
		{Key: "com.apple.developer.coremedia.hls.low-latency", Capability: "COREMEDIA_HLS_LOW_LATENCY"},
		{Key: "com.apple.developer.system-extension.install", Capability: "SYSTEM_EXTENSION_INSTALL", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.user-management", Capability: "USER_MANAGEMENT", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.applesignin", Capability: "APPLE_ID_AUTH", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.game-center", Capability: "GAME_CENTER", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.pass-type-identifiers", Capability: "WALLET", ValueKind: entitlementValueStringArray},
		{Key: "com.apple.developer.maps", Capability: "MAPS", ValueKind: entitlementValueBoolean},
		{Key: "inter-app-audio", Capability: "INTER_APP_AUDIO", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.homekit", Capability: "HOMEKIT", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.private-cloud-compute", WebCommand: "asc web bundle-ids capabilities enable --capability PRIVATE_CLOUD_COMPUTE", ValueKind: entitlementValueBoolean},
		{Key: "com.apple.developer.kernel.increased-memory-limit", UnsupportedCapability: "INCREASED_MEMORY_LIMIT", ValueKind: entitlementValueBoolean},
	}
}

func iCloudServicesSettings(value any) []asc.CapabilitySetting {
	services, ok := entitlementStringArray(value)
	if !ok {
		return nil // readDesiredEntitlements validates this before planning.
	}
	for _, service := range services {
		if service == "CloudKit" {
			enabled := true
			return []asc.CapabilitySetting{{Key: "ICLOUD_VERSION", Options: []asc.CapabilityOption{{Key: "XCODE_6", Enabled: &enabled}}}}
		}
	}
	return nil
}

func entitlementStringArray(value any) ([]string, bool) {
	switch values := value.(type) {
	case []string:
		return values, true
	case []any:
		strings := make([]string, len(values))
		for index, entry := range values {
			text, ok := entry.(string)
			if !ok {
				return nil, false
			}
			strings[index] = text
		}
		return strings, true
	default:
		return nil, false
	}
}

func dataProtectionSettings(value any) []asc.CapabilitySetting {
	text, _ := value.(string)
	optionKey, ok := dataProtectionOptions[text]
	if !ok {
		return nil
	}
	enabled := true
	return []asc.CapabilitySetting{{
		Key:     "DATA_PROTECTION_PERMISSION_LEVEL",
		Options: []asc.CapabilityOption{{Key: optionKey, Enabled: &enabled}},
	}}
}

func mergeCapabilitySettings(existing, desired []asc.CapabilitySetting) ([]asc.CapabilitySetting, bool) {
	if len(desired) == 0 {
		return append([]asc.CapabilitySetting(nil), existing...), false
	}
	merged := append([]asc.CapabilitySetting(nil), existing...)
	changed := false
	for _, want := range desired {
		found := false
		for index, current := range merged {
			if current.Key != want.Key {
				continue
			}
			found = true
			options, optionsChanged := reconcileCapabilityOptions(current.Options, want.Options)
			if optionsChanged {
				merged[index].Options = options
				changed = true
			}
		}
		if !found {
			merged = append(merged, want)
			changed = true
		}
	}
	return merged, changed
}

func reconcileCapabilityOptions(current, desired []asc.CapabilityOption) ([]asc.CapabilityOption, bool) {
	selected := make(map[string]bool, len(desired))
	for _, option := range desired {
		selected[option.Key] = option.Enabled != nil && *option.Enabled
	}
	merged := append([]asc.CapabilityOption(nil), current...)
	changed := false
	for index, option := range current {
		wasEnabled := option.Enabled != nil && *option.Enabled
		shouldEnable := selected[option.Key]
		if wasEnabled != shouldEnable {
			merged[index].Enabled = &shouldEnable
			changed = true
		}
	}
	for _, option := range desired {
		if !capabilityOptionPresent(current, option.Key) {
			merged = append(merged, option)
			changed = true
		}
	}
	return merged, changed
}

func capabilityOptionPresent(options []asc.CapabilityOption, key string) bool {
	for _, option := range options {
		if option.Key == key {
			return true
		}
	}
	return false
}

func mappedCapability(capability string) bool {
	for _, item := range entitlementCapabilityCatalog() {
		if item.Capability == capability {
			return true
		}
	}
	return false
}
