package web

import (
	"encoding/json"
	"fmt"
)

func validateDeveloperCapabilitySettings(metadata developerCapabilityMetadataAttributes, settings []BundleIDCapabilitySetting) error {
	if settings == nil {
		return nil
	}
	var available []BundleIDCapabilitySetting
	if len(metadata.Settings) > 0 {
		if err := json.Unmarshal(metadata.Settings, &available); err != nil {
			return fmt.Errorf("invalid Developer Portal capability settings metadata: %w", err)
		}
	}
	requested := make(map[string]BundleIDCapabilitySetting, len(settings))
	for _, setting := range settings {
		if _, duplicate := requested[setting.Key]; duplicate {
			return fmt.Errorf("duplicate capability setting %q", setting.Key)
		}
		requested[setting.Key] = setting
	}
	known := make(map[string]BundleIDCapabilitySetting, len(available))
	for _, setting := range available {
		known[setting.Key] = setting
	}
	for _, setting := range settings {
		definition, ok := known[setting.Key]
		if !ok {
			return fmt.Errorf("unknown capability setting %q", setting.Key)
		}
		if err := validateDeveloperCapabilitySettingOptions(definition, setting.Options); err != nil {
			return err
		}
	}
	for _, definition := range available {
		if _, supplied := requested[definition.Key]; supplied {
			continue
		}
		if definition.MinInstances != nil && *definition.MinInstances > 0 {
			return fmt.Errorf("capability setting %q requires at least %d enabled option(s)", definition.Key, *definition.MinInstances)
		}
	}
	return nil
}

func validateDeveloperCapabilitySettingOptions(definition BundleIDCapabilitySetting, options []BundleIDCapabilityOption) error {
	known := make(map[string]struct{}, len(definition.Options))
	for _, option := range definition.Options {
		known[option.Key] = struct{}{}
	}
	seen := make(map[string]struct{}, len(options))
	enabled := 0
	for _, option := range options {
		if _, duplicate := seen[option.Key]; duplicate {
			return fmt.Errorf("duplicate option %q for capability setting %q", option.Key, definition.Key)
		}
		seen[option.Key] = struct{}{}
		if _, ok := known[option.Key]; !ok {
			return fmt.Errorf("unknown option %q for capability setting %q", option.Key, definition.Key)
		}
		if option.Enabled == nil {
			return fmt.Errorf("option %q for capability setting %q requires an enabled value", option.Key, definition.Key)
		}
		if *option.Enabled {
			enabled++
		}
	}
	if definition.MinInstances != nil && enabled < *definition.MinInstances {
		return fmt.Errorf("capability setting %q requires at least %d enabled option(s)", definition.Key, *definition.MinInstances)
	}
	switch definition.AllowedInstances {
	case "SINGLE":
		if enabled > 1 {
			return fmt.Errorf("capability setting %q allows at most one enabled option", definition.Key)
		}
	case "", "MULTIPLE":
	default:
		return fmt.Errorf("capability setting %q uses unsupported allowedInstances %q", definition.Key, definition.AllowedInstances)
	}
	return nil
}
