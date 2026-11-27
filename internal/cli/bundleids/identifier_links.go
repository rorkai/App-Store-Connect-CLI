package bundleids

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

func unverifiedEntitlementIdentifiers(key string, value any) []asc.CapabilityIdentifierLink {
	switch key {
	case "com.apple.security.application-groups", "com.apple.developer.in-app-payments", "com.apple.developer.icloud-container-identifiers", "com.apple.developer.ubiquity-container-identifiers":
	default:
		return nil
	}
	identifiers, _ := entitlementStringArray(value)
	seen := map[string]bool{}
	var result []asc.CapabilityIdentifierLink
	for _, identifier := range identifiers {
		if seen[identifier] {
			continue
		}
		seen[identifier] = true
		result = append(result, asc.CapabilityIdentifierLink{
			Entitlement: key, Identifier: identifier, Status: "unverified",
			Reason:  "The public API cannot read this identifier relationship. Select the matching Developer Portal team, exact bundle identifier and platform, then inspect its Portal resource ID with asc web bundle-ids view --bundle-id PORTAL_RESOURCE_ID --output json. Only complete matching relationship data can prove linkage or absence; an enabled capability is not proof.",
			Command: "asc web bundle-ids list --output json",
		})
	}
	return result
}
