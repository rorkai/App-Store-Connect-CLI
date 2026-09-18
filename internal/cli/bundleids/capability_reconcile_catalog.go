package bundleids

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

type entitlementCapability struct {
	Key        string
	Capability string
	WebCommand string
	Settings   func(any) []asc.CapabilitySetting
}

func entitlementCapabilityCatalog() []entitlementCapability {
	return []entitlementCapability{
		{Key: "aps-environment", Capability: "PUSH_NOTIFICATIONS"},
		{Key: "com.apple.developer.aps-environment", Capability: "PUSH_NOTIFICATIONS"},
		{Key: "com.apple.developer.healthkit", Capability: "HEALTHKIT"},
		{Key: "com.apple.developer.icloud-container-identifiers", Capability: "ICLOUD"},
		{Key: "com.apple.developer.icloud-services", Capability: "ICLOUD"},
		{Key: "com.apple.developer.ubiquity-kvstore-identifier", Capability: "ICLOUD"},
		{Key: "com.apple.security.application-groups", Capability: "APP_GROUPS"},
		{Key: "com.apple.developer.associated-domains", Capability: "ASSOCIATED_DOMAINS"},
		{Key: "com.apple.developer.networking.vpn.api", Capability: "PERSONAL_VPN"},
		{Key: "com.apple.external-accessory.wireless-configuration", Capability: "WIRELESS_ACCESSORY_CONFIGURATION"},
		{Key: "com.apple.developer.in-app-payments", Capability: "APPLE_PAY"},
		{Key: "com.apple.developer.default-data-protection", Capability: "DATA_PROTECTION", Settings: dataProtectionSettings},
		{Key: "com.apple.developer.siri", Capability: "SIRIKIT"},
		{Key: "com.apple.developer.networking.networkextension", Capability: "NETWORK_EXTENSIONS"},
		{Key: "com.apple.developer.networking.multipath", Capability: "MULTIPATH"},
		{Key: "com.apple.developer.networking.HotspotConfiguration", Capability: "HOT_SPOT"},
		{Key: "com.apple.developer.nfc.readersession.formats", Capability: "NFC_TAG_READING"},
		{Key: "com.apple.developer.ClassKit-environment", Capability: "CLASSKIT"},
		{Key: "com.apple.developer.authentication-services.autofill-credential-provider", Capability: "AUTOFILL_CREDENTIAL_PROVIDER"},
		{Key: "com.apple.developer.networking.wifi-info", Capability: "ACCESS_WIFI_INFORMATION"},
		{Key: "com.apple.developer.networking.custom-protocol", Capability: "NETWORK_CUSTOM_PROTOCOL"},
		{Key: "com.apple.developer.coremedia.hls.low-latency", Capability: "COREMEDIA_HLS_LOW_LATENCY"},
		{Key: "com.apple.developer.system-extension.install", Capability: "SYSTEM_EXTENSION_INSTALL"},
		{Key: "com.apple.developer.user-management", Capability: "USER_MANAGEMENT"},
		{Key: "com.apple.developer.applesignin", Capability: "APPLE_ID_AUTH"},
		{Key: "com.apple.developer.game-center", Capability: "GAME_CENTER"},
		{Key: "com.apple.developer.pass-type-identifiers", Capability: "WALLET"},
		{Key: "com.apple.developer.maps", Capability: "MAPS"},
		{Key: "com.apple.developer.inter-app-audio", Capability: "INTER_APP_AUDIO"},
		{Key: "com.apple.InAppPurchase", Capability: "IN_APP_PURCHASE"},
		{Key: "com.apple.developer.homekit", Capability: "HOMEKIT"},
		{Key: "com.apple.developer.kernel.increased-memory-limit", WebCommand: "asc web bundle-ids capabilities enable --capability INCREASED_MEMORY_LIMIT"},
	}
}

func dataProtectionSettings(value any) []asc.CapabilitySetting {
	text, _ := value.(string)
	if text == "" {
		return nil
	}
	enabled := true
	return []asc.CapabilitySetting{{
		Key:     "DATA_PROTECTION_PERMISSION_LEVEL",
		Options: []asc.CapabilityOption{{Key: text, Enabled: &enabled}},
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
			for _, option := range want.Options {
				if !capabilityOptionPresent(current.Options, option.Key) {
					merged[index].Options = append(merged[index].Options, option)
					changed = true
				}
			}
		}
		if !found {
			merged = append(merged, want)
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
