package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type developerCapabilityAssociation struct {
	RelationshipName string `json:"relationshipName"`
	RelationshipType string `json:"relationshipType"`
	MinInstances     int    `json:"minInstances"`
	MaxLimit         *int   `json:"maxLimit"`
	AllowedInstances string `json:"allowedInstances"`
}

func developerCapabilityAssociationDefinitions(metadata developerCapabilityMetadataAttributes) (map[string]developerCapabilityAssociation, error) {
	var associations []developerCapabilityAssociation
	if len(metadata.Associations) > 0 {
		if err := json.Unmarshal(metadata.Associations, &associations); err != nil {
			return nil, fmt.Errorf("invalid Developer Portal capability associations metadata: %w", err)
		}
	}
	definitions := make(map[string]developerCapabilityAssociation, len(associations))
	for _, association := range associations {
		if association.RelationshipName == "" || association.RelationshipType == "" || association.MinInstances < 0 || (association.MaxLimit != nil && *association.MaxLimit < 0) {
			return nil, fmt.Errorf("incomplete Developer Portal capability association metadata")
		}
		if _, exists := definitions[association.RelationshipName]; exists {
			return nil, fmt.Errorf("duplicate capability association metadata %q", association.RelationshipName)
		}
		definitions[association.RelationshipName] = association
	}
	return definitions, nil
}

func applyDeveloperCapabilityAssociations(metadata developerCapabilityMetadataAttributes, capability developerResource, requested map[string][]string) (developerResource, error) {
	definitions, err := developerCapabilityAssociationDefinitions(metadata)
	if err != nil {
		return capability, err
	}
	capability.Relationships = cloneRawMessageMap(capability.Relationships)
	if capability.Relationships == nil {
		capability.Relationships = make(map[string]json.RawMessage)
	}
	for name, ids := range requested {
		definition, ok := definitions[name]
		if !ok {
			return capability, fmt.Errorf("unknown capability association %q", name)
		}
		if ids == nil {
			return capability, fmt.Errorf("capability association %q must be an array of resource ids, got null", name)
		}
		existing, many, err := developerCapabilityAssociationResources(capability.Relationships[name])
		if err != nil {
			return capability, fmt.Errorf("capability association %q: %w", name, err)
		}
		switch name {
		case "associatedBundleIds", "mediaSharingProtocolIds", "appGroups", "cloudContainers", "certificates", "merchantIds", "relatedAppConsentBundleIds":
			many = true
		case "parentBundleId", "appConsentBundleId":
			many = false
		default:
			if !many && len(existing) == 0 {
				return capability, fmt.Errorf("capability association %q has an unknown relationship shape; configure it in Developer Portal first", name)
			}
		}
		resources := make([]developerResource, 0, len(ids))
		for _, id := range ids {
			resources = append(resources, developerResource{Type: definition.RelationshipType, ID: strings.TrimSpace(id)})
		}
		var data any = resources
		if !many {
			if len(resources) > 1 {
				return capability, fmt.Errorf("capability association %q allows at most one resource", name)
			}
			data = nil
			if len(resources) == 1 {
				data = resources[0]
			}
		}
		encoded, err := json.Marshal(struct {
			Data any `json:"data"`
		}{Data: data})
		if err != nil {
			return capability, fmt.Errorf("encode capability association %q: %w", name, err)
		}
		capability.Relationships[name] = encoded
	}
	return capability, validateDeveloperCapabilityAssociationValues(definitions, capability)
}

func validateDeveloperCapabilityAssociations(metadata developerCapabilityMetadataAttributes, capability developerResource) error {
	definitions, err := developerCapabilityAssociationDefinitions(metadata)
	if err != nil {
		return err
	}
	return validateDeveloperCapabilityAssociationValues(definitions, capability)
}

func validateDeveloperCapabilityAssociationValues(definitions map[string]developerCapabilityAssociation, capability developerResource) error {
	for name, definition := range definitions {
		resources, _, err := developerCapabilityAssociationResources(capability.Relationships[name])
		if err != nil {
			return fmt.Errorf("capability association %q: %w", name, err)
		}
		if len(resources) < definition.MinInstances {
			return fmt.Errorf("capability association %q requires at least %d resource(s); provide --associations-json or configure the association in Developer Portal", name, definition.MinInstances)
		}
		if definition.AllowedInstances == "SINGLE" && len(resources) > 1 {
			return fmt.Errorf("capability association %q allows at most one resource", name)
		}
		if definition.MaxLimit != nil && len(resources) > *definition.MaxLimit {
			return fmt.Errorf("capability association %q allows at most %d resources", name, *definition.MaxLimit)
		}
		seen := make(map[string]struct{}, len(resources))
		for _, resource := range resources {
			if strings.TrimSpace(resource.ID) == "" {
				return fmt.Errorf("capability association %q has an empty resource id", name)
			}
			if resource.Type != definition.RelationshipType {
				return fmt.Errorf("capability association %q expected resource type %q, received %q", name, definition.RelationshipType, resource.Type)
			}
			if _, duplicate := seen[resource.ID]; duplicate {
				return fmt.Errorf("capability association %q repeats resource %q", name, resource.ID)
			}
			seen[resource.ID] = struct{}{}
		}
	}
	return nil
}

func developerCapabilityAssociationResources(raw json.RawMessage) ([]developerResource, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	var relationship struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return nil, false, fmt.Errorf("invalid relationship: %w", err)
	}
	data := bytes.TrimSpace(relationship.Data)
	if len(data) == 0 {
		return nil, false, fmt.Errorf("relationship has no resource data")
	}
	if bytes.Equal(data, []byte("null")) {
		return nil, false, nil
	}
	if data[0] == '[' {
		var resources []developerResource
		err := json.Unmarshal(data, &resources)
		return resources, true, err
	}
	var resource developerResource
	if err := json.Unmarshal(data, &resource); err != nil {
		return nil, false, fmt.Errorf("invalid relationship resource: %w", err)
	}
	return []developerResource{resource}, false, nil
}
