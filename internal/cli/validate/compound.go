package validate

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const compoundLocalizationLimit = 50

type compoundLinkageState uint8

const (
	compoundLinkageFallback compoundLinkageState = iota
	compoundLinkageEmpty
	compoundLinkageResolved
)

type includedResourceKey struct {
	resourceType asc.ResourceType
	id           string
}

type includedResourceIndex struct {
	resources map[includedResourceKey]json.RawMessage
	ambiguous map[includedResourceKey]struct{}
	decodable bool
}

func newIncludedResourceIndex(raw json.RawMessage) includedResourceIndex {
	index := includedResourceIndex{
		resources: make(map[includedResourceKey]json.RawMessage),
		ambiguous: make(map[includedResourceKey]struct{}),
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return index
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return index
	}
	index.decodable = true

	for _, item := range items {
		var identity struct {
			Type asc.ResourceType `json:"type"`
			ID   string           `json:"id"`
		}
		if err := json.Unmarshal(item, &identity); err != nil {
			continue
		}
		identity.ID = strings.TrimSpace(identity.ID)
		if identity.Type == "" || identity.ID == "" {
			continue
		}

		key := includedResourceKey{resourceType: identity.Type, id: identity.ID}
		if _, exists := index.resources[key]; exists {
			index.ambiguous[key] = struct{}{}
			continue
		}
		index.resources[key] = append(json.RawMessage(nil), item...)
	}

	return index
}

func decodeIncludedResource[T any](index includedResourceIndex, resourceType asc.ResourceType, id string) (asc.Resource[T], bool) {
	var zero asc.Resource[T]
	id = strings.TrimSpace(id)
	if !index.decodable || id == "" {
		return zero, false
	}

	key := includedResourceKey{resourceType: resourceType, id: id}
	if _, ambiguous := index.ambiguous[key]; ambiguous {
		return zero, false
	}
	raw, ok := index.resources[key]
	if !ok {
		return zero, false
	}

	var resource asc.Resource[T]
	if err := json.Unmarshal(raw, &resource); err != nil {
		return zero, false
	}
	if resource.Type != resourceType || strings.TrimSpace(resource.ID) != id {
		return zero, false
	}
	return resource, true
}

func resolveCompoundToOne[T any](relationships json.RawMessage, included includedResourceIndex, name string, resourceType asc.ResourceType) (asc.Resource[T], compoundLinkageState) {
	var zero asc.Resource[T]
	id, state := compoundToOneID(relationships, name, resourceType)
	if state != compoundLinkageResolved {
		return zero, state
	}
	resource, ok := decodeIncludedResource[T](included, resourceType, id)
	if !ok {
		return zero, compoundLinkageFallback
	}
	return resource, compoundLinkageResolved
}

func resolveCompoundToMany[T any](relationships json.RawMessage, included includedResourceIndex, name string, resourceType asc.ResourceType) ([]asc.Resource[T], bool) {
	ids, ok := compoundToManyIDs(relationships, name, resourceType)
	if !ok {
		return nil, false
	}

	resources := make([]asc.Resource[T], 0, len(ids))
	for _, id := range ids {
		resource, found := decodeIncludedResource[T](included, resourceType, id)
		if !found {
			return nil, false
		}
		resources = append(resources, resource)
	}
	return resources, true
}

func compoundToOneID(relationships json.RawMessage, name string, resourceType asc.ResourceType) (string, compoundLinkageState) {
	relationship, ok := compoundRelationship(relationships, name)
	if !ok || len(bytes.TrimSpace(relationship.Data)) == 0 {
		return "", compoundLinkageFallback
	}
	if bytes.Equal(bytes.TrimSpace(relationship.Data), []byte("null")) {
		return "", compoundLinkageEmpty
	}

	var linkage asc.ResourceData
	if err := json.Unmarshal(relationship.Data, &linkage); err != nil {
		return "", compoundLinkageFallback
	}
	linkage.ID = strings.TrimSpace(linkage.ID)
	if linkage.Type != resourceType || linkage.ID == "" {
		return "", compoundLinkageFallback
	}
	return linkage.ID, compoundLinkageResolved
}

func compoundToManyIDs(relationships json.RawMessage, name string, resourceType asc.ResourceType) ([]string, bool) {
	relationship, ok := compoundRelationship(relationships, name)
	if !ok || len(bytes.TrimSpace(relationship.Data)) == 0 || bytes.Equal(bytes.TrimSpace(relationship.Data), []byte("null")) {
		return nil, false
	}

	var linkages []asc.ResourceData
	if err := json.Unmarshal(relationship.Data, &linkages); err != nil {
		return nil, false
	}

	ids := make([]string, 0, len(linkages))
	seen := make(map[string]struct{}, len(linkages))
	for _, linkage := range linkages {
		linkage.ID = strings.TrimSpace(linkage.ID)
		if linkage.Type != resourceType || linkage.ID == "" {
			return nil, false
		}
		if _, duplicate := seen[linkage.ID]; duplicate {
			return nil, false
		}
		seen[linkage.ID] = struct{}{}
		ids = append(ids, linkage.ID)
	}

	if !compoundRelationshipIsComplete(relationship.Meta, len(ids)) {
		return nil, false
	}
	return ids, true
}

type rawCompoundRelationship struct {
	Data json.RawMessage `json:"data"`
	Meta json.RawMessage `json:"meta"`
}

func compoundRelationship(relationships json.RawMessage, name string) (rawCompoundRelationship, bool) {
	var zero rawCompoundRelationship
	if len(bytes.TrimSpace(relationships)) == 0 {
		return zero, false
	}

	var rawRelationships map[string]json.RawMessage
	if err := json.Unmarshal(relationships, &rawRelationships); err != nil {
		return zero, false
	}
	raw, ok := rawRelationships[name]
	if !ok {
		return zero, false
	}

	var relationship rawCompoundRelationship
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return zero, false
	}
	return relationship, true
}

func compoundRelationshipIsComplete(meta json.RawMessage, linkedCount int) bool {
	if len(bytes.TrimSpace(meta)) == 0 || bytes.Equal(bytes.TrimSpace(meta), []byte("null")) {
		return linkedCount < compoundLocalizationLimit
	}

	var paging struct {
		Paging struct {
			Total      *int   `json:"total"`
			NextCursor string `json:"nextCursor"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(meta, &paging); err != nil {
		return false
	}
	if strings.TrimSpace(paging.Paging.NextCursor) != "" {
		return false
	}
	if paging.Paging.Total == nil {
		return linkedCount < compoundLocalizationLimit
	}
	return *paging.Paging.Total == linkedCount
}

func mapReviewDetails(resource asc.Resource[asc.AppStoreReviewDetailAttributes]) *validation.ReviewDetails {
	attrs := resource.Attributes
	return &validation.ReviewDetails{
		ID:                  resource.ID,
		ContactFirstName:    attrs.ContactFirstName,
		ContactLastName:     attrs.ContactLastName,
		ContactEmail:        attrs.ContactEmail,
		ContactPhone:        attrs.ContactPhone,
		DemoAccountName:     attrs.DemoAccountName,
		DemoAccountPassword: attrs.DemoAccountPassword,
		DemoAccountRequired: attrs.DemoAccountRequired,
		Notes:               attrs.Notes,
	}
}

func mapBuild(resource asc.Resource[asc.BuildAttributes]) *validation.Build {
	attrs := resource.Attributes
	return &validation.Build{
		ID:                      strings.TrimSpace(resource.ID),
		Version:                 attrs.Version,
		ProcessingState:         attrs.ProcessingState,
		Expired:                 attrs.Expired,
		UsesNonExemptEncryption: attrs.UsesNonExemptEncryption,
		MinOSVersion:            strings.TrimSpace(attrs.MinOSVersion),
	}
}

func copyBuild(build *validation.Build) *validation.Build {
	if build == nil {
		return nil
	}
	return &validation.Build{
		ID:                            strings.TrimSpace(build.ID),
		Version:                       build.Version,
		ProcessingState:               build.ProcessingState,
		Expired:                       build.Expired,
		UsesNonExemptEncryption:       build.UsesNonExemptEncryption,
		AppEncryptionDeclarationID:    strings.TrimSpace(build.AppEncryptionDeclarationID),
		AppEncryptionDeclarationState: strings.TrimSpace(build.AppEncryptionDeclarationState),
		MinOSVersion:                  strings.TrimSpace(build.MinOSVersion),
		Bundles:                       slices.Clone(build.Bundles),
		BundlesUnreadReason:           strings.TrimSpace(build.BundlesUnreadReason),
	}
}
