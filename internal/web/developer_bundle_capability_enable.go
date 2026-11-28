package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func validateDeveloperBundleIDEnableSettings(current developerBundleIDResponse, metadata developerCapabilityMetadataAttributes, req DeveloperBundleIDCapabilityEnableRequest) error {
	if req.Settings != nil {
		return validateDeveloperCapabilitySettings(metadata, req.Settings)
	}
	capabilities, err := developerBundleIDCapabilities(current)
	if err != nil {
		return err
	}
	for _, capability := range capabilities {
		id, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return err
		}
		if strings.EqualFold(id, req.Capability) {
			enabled, err := developerBundleIDCapabilityEnabled(capability)
			if err != nil {
				return err
			}
			if enabled || len(metadata.Settings) == 0 || string(metadata.Settings) == "null" {
				return nil
			}
			var attributes struct {
				Settings []BundleIDCapabilitySetting `json:"settings"`
			}
			if err := json.Unmarshal(capability.Attributes, &attributes); err != nil {
				return fmt.Errorf("parse current capability settings: %w", err)
			}
			if attributes.Settings == nil {
				attributes.Settings = []BundleIDCapabilitySetting{}
			}
			return validateDeveloperCapabilitySettings(metadata, attributes.Settings)
		}
	}
	return validateDeveloperCapabilitySettings(metadata, []BundleIDCapabilitySetting{})
}

func setDeveloperCapabilitySettings(attributes json.RawMessage, settings []BundleIDCapabilitySetting) (json.RawMessage, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(attributes, &values); err != nil {
		return nil, fmt.Errorf("parse capability attributes: %w", err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("encode capability settings: %w", err)
	}
	values["settings"] = encoded
	return json.Marshal(values)
}

func developerCapabilityResourceSettingsApplied(capability developerResource, requested []BundleIDCapabilitySetting) bool {
	var attributes struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(capability.Attributes, &attributes); err != nil {
		return false
	}
	return requestedSettingsApplied(requested, attributes.Settings)
}

func verifyDeveloperBundleIDCapabilityGraph(payload developerBundleIDPatchRequest, after developerBundleIDResponse, skipCapability string) error {
	var desired developerResourceRelationship
	if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &desired); err != nil {
		return fmt.Errorf("parse requested capability graph: %w", err)
	}
	actual, err := developerBundleIDCapabilities(after)
	if err != nil {
		return err
	}
	for _, wanted := range desired.Data {
		wantedCapability, err := developerBundleIDCapabilityID(wanted)
		if err != nil {
			return err
		}
		if strings.EqualFold(wantedCapability, skipCapability) {
			continue
		}
		matched := false
		for _, found := range actual {
			foundCapability, err := developerBundleIDCapabilityID(found)
			if err != nil {
				return err
			}
			if !strings.EqualFold(foundCapability, wantedCapability) || (wanted.ID != "" && wanted.ID != found.ID) {
				continue
			}
			if !requestedSettingsApplied(wanted.Attributes, found.Attributes) || !developerCapabilityRelationshipsApplied(wanted.Relationships, found.Relationships) {
				continue
			}
			matched = true
			break
		}
		if !matched {
			return fmt.Errorf("capability %q resource %q did not retain its requested state and relationships", wantedCapability, wanted.ID)
		}
	}
	return nil
}

func developerCapabilityRelationshipsApplied(requested, current map[string]json.RawMessage) bool {
	for name, requestedRelationship := range requested {
		currentRelationship, exists := current[name]
		if !exists {
			return false
		}
		wanted, ok := normalizeDeveloperCapabilityRelationship(requestedRelationship)
		if !ok {
			return false
		}
		found, ok := normalizeDeveloperCapabilityRelationship(currentRelationship)
		if !ok || !requestedSettingsApplied(wanted, found) {
			return false
		}
	}
	return true
}

func normalizeDeveloperCapabilityRelationship(raw json.RawMessage) (json.RawMessage, bool) {
	var relationship map[string]json.RawMessage
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return nil, false
	}
	data := bytes.TrimSpace(relationship["data"])
	if len(data) == 0 || data[0] != '[' {
		return raw, true
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, false
	}
	type linkage struct {
		reference relationshipData
		raw       json.RawMessage
	}
	links := make([]linkage, 0, len(values))
	seen := make(map[relationshipData]struct{}, len(values))
	known := true
	for _, value := range values {
		var reference relationshipData
		if err := json.Unmarshal(value, &reference); err != nil || reference.ID == "" || reference.Type == "" {
			known = false
			continue
		}
		if _, duplicate := seen[reference]; duplicate {
			return nil, false
		}
		seen[reference] = struct{}{}
		links = append(links, linkage{reference: reference, raw: value})
	}
	if !known {
		return raw, true
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].reference.Type != links[j].reference.Type {
			return links[i].reference.Type < links[j].reference.Type
		}
		return links[i].reference.ID < links[j].reference.ID
	})
	for index, link := range links {
		values[index] = link.raw
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, false
	}
	relationship["data"] = encoded
	encoded, err = json.Marshal(relationship)
	return encoded, err == nil
}
