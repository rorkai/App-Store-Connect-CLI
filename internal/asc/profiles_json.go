package asc

import (
	"encoding/json"
	"reflect"
	"strings"
)

// UnmarshalJSON records that the profile attributes member was present while
// retaining the existing value-shaped ProfileAttributes API.
func (a *ProfileAttributes) UnmarshalJSON(data []byte) error {
	type profileAttributes ProfileAttributes
	var decoded profileAttributes
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = ProfileAttributes(decoded)
	a.attributesPresent = true
	a.attributesNull = strings.TrimSpace(string(data)) == "null"
	a.nameJSON = fields["name"]
	a.platformJSON = fields["platform"]
	a.profileTypeJSON = fields["profileType"]
	a.profileStateJSON = fields["profileState"]
	a.profileContentJSON = fields["profileContent"]
	a.uuidJSON = fields["uuid"]
	a.createdDateJSON = fields["createdDate"]
	a.expirationDateJSON = fields["expirationDate"]
	a.profileVersionJSON = fields["profileVersion"]
	a.nameValue = a.Name
	a.platformValue = a.Platform
	a.profileTypeValue = a.ProfileType
	a.profileStateValue = a.ProfileState
	a.profileContentValue = a.ProfileContent
	a.uuidValue = a.UUID
	a.createdDateValue = a.CreatedDate
	a.expirationDateValue = a.ExpirationDate
	a.profileVersionValue = a.ProfileVersion
	return nil
}

// MarshalJSON preserves explicitly returned empty and null profile attributes
// while retaining the value-shaped ProfileAttributes API.
func (a ProfileAttributes) MarshalJSON() ([]byte, error) {
	if a.attributesNull {
		return []byte("null"), nil
	}

	type attributesJSON struct {
		Name           json.RawMessage `json:"name,omitempty"`
		Platform       json.RawMessage `json:"platform,omitempty"`
		ProfileType    json.RawMessage `json:"profileType,omitempty"`
		ProfileState   json.RawMessage `json:"profileState,omitempty"`
		ProfileContent json.RawMessage `json:"profileContent,omitempty"`
		UUID           json.RawMessage `json:"uuid,omitempty"`
		CreatedDate    json.RawMessage `json:"createdDate,omitempty"`
		ExpirationDate json.RawMessage `json:"expirationDate,omitempty"`
		ProfileVersion json.RawMessage `json:"profileVersion,omitempty"`
	}
	attributes := attributesJSON{}
	var err error
	if len(a.nameJSON) > 0 || a.Name != "" {
		attributes.Name, err = profileAttributeJSON(a.nameJSON, a.nameValue, a.Name)
		if err != nil {
			return nil, err
		}
	}
	if len(a.platformJSON) > 0 || a.Platform != "" {
		attributes.Platform, err = profileAttributeJSON(a.platformJSON, a.platformValue, a.Platform)
		if err != nil {
			return nil, err
		}
	}
	if len(a.profileTypeJSON) > 0 || a.ProfileType != "" {
		attributes.ProfileType, err = profileAttributeJSON(a.profileTypeJSON, a.profileTypeValue, a.ProfileType)
		if err != nil {
			return nil, err
		}
	}
	if len(a.profileStateJSON) > 0 || a.ProfileState != "" {
		attributes.ProfileState, err = profileAttributeJSON(a.profileStateJSON, a.profileStateValue, a.ProfileState)
		if err != nil {
			return nil, err
		}
	}
	if len(a.profileContentJSON) > 0 || a.ProfileContent != "" {
		attributes.ProfileContent, err = profileAttributeJSON(a.profileContentJSON, a.profileContentValue, a.ProfileContent)
		if err != nil {
			return nil, err
		}
	}
	if len(a.uuidJSON) > 0 || a.UUID != "" {
		attributes.UUID, err = profileAttributeJSON(a.uuidJSON, a.uuidValue, a.UUID)
		if err != nil {
			return nil, err
		}
	}
	if len(a.createdDateJSON) > 0 || a.CreatedDate != "" {
		attributes.CreatedDate, err = profileAttributeJSON(a.createdDateJSON, a.createdDateValue, a.CreatedDate)
		if err != nil {
			return nil, err
		}
	}
	if len(a.expirationDateJSON) > 0 || a.ExpirationDate != "" {
		attributes.ExpirationDate, err = profileAttributeJSON(a.expirationDateJSON, a.expirationDateValue, a.ExpirationDate)
		if err != nil {
			return nil, err
		}
	}
	if len(a.profileVersionJSON) > 0 || a.ProfileVersion != "" {
		attributes.ProfileVersion, err = profileAttributeJSON(a.profileVersionJSON, a.profileVersionValue, a.ProfileVersion)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(attributes)
}

func profileAttributeJSON(raw json.RawMessage, decoded, current any) (json.RawMessage, error) {
	if len(raw) > 0 && reflect.DeepEqual(decoded, current) {
		return raw, nil
	}
	return json.Marshal(current)
}

// GetLinks returns the links field for pagination.
func (r *ProfilesResponse) GetLinks() *Links {
	return &r.Links
}

// GetMeta returns the raw metadata field.
func (r *ProfilesResponse) GetMeta() json.RawMessage {
	return r.Meta
}

// GetData returns the data field for aggregation.
func (r *ProfilesResponse) GetData() any {
	return r.Data
}

// MarshalJSON omits attributes when Apple omitted them from a relationship-only
// sparse profile resource, while preserving the existing response envelope.
func (r ProfilesResponse) MarshalJSON() ([]byte, error) {
	type profileResource struct {
		Type          ResourceType       `json:"type"`
		ID            string             `json:"id"`
		Attributes    *ProfileAttributes `json:"attributes,omitempty"`
		Relationships json.RawMessage    `json:"relationships,omitempty"`
		Links         json.RawMessage    `json:"links,omitempty"`
	}
	var data []profileResource
	if r.Data != nil {
		data = make([]profileResource, len(r.Data))
	}
	for i, resource := range r.Data {
		data[i] = profileResource{
			Type:          resource.Type,
			ID:            resource.ID,
			Relationships: resource.Relationships,
			Links:         resource.Links,
		}
		if profileAttributesHaveValues(resource.Attributes) {
			attributes := resource.Attributes
			data[i].Attributes = &attributes
		}
	}

	type profilesResponse struct {
		Data     []profileResource `json:"data"`
		Links    Links             `json:"links"`
		Included json.RawMessage   `json:"included,omitempty"`
		Meta     json.RawMessage   `json:"meta,omitempty"`
	}
	return json.Marshal(profilesResponse{
		Data:     data,
		Links:    r.Links,
		Included: r.Included,
		Meta:     r.Meta,
	})
}

func profileAttributesHaveValues(attributes ProfileAttributes) bool {
	return attributes.attributesPresent ||
		attributes.Name != "" ||
		attributes.Platform != "" ||
		attributes.ProfileType != "" ||
		attributes.ProfileState != "" ||
		attributes.ProfileContent != "" ||
		attributes.UUID != "" ||
		attributes.CreatedDate != "" ||
		attributes.ExpirationDate != "" ||
		attributes.ProfileVersion != ""
}
