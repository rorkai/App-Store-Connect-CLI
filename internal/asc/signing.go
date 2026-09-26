package asc

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// BundleIDPlatform represents the platform of a bundle ID or registered device.
type BundleIDPlatform string

const (
	BundleIDPlatformIOS       BundleIDPlatform = "IOS"
	BundleIDPlatformMacOS     BundleIDPlatform = "MAC_OS"
	BundleIDPlatformUniversal BundleIDPlatform = "UNIVERSAL"
)

// BundleIDAttributes describes a bundle ID resource.
type BundleIDAttributes struct {
	Name       string           `json:"name"`
	Identifier string           `json:"identifier"`
	Platform   BundleIDPlatform `json:"platform"`
	SeedID     string           `json:"seedId,omitempty"`

	attributesDecoded bool
	attributesNull    bool
	nameJSON          json.RawMessage
	identifierJSON    json.RawMessage
	platformJSON      json.RawMessage
	seedIDJSON        json.RawMessage
	nameValue         string
	identifierValue   string
	platformValue     BundleIDPlatform
	seedIDValue       string
}

// UnmarshalJSON retains sparse field presence without changing the public,
// value-shaped BundleIDAttributes API.
func (a *BundleIDAttributes) UnmarshalJSON(data []byte) error {
	type alias BundleIDAttributes
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = BundleIDAttributes(decoded)
	a.attributesDecoded = true
	a.attributesNull = bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	a.nameJSON = fields["name"]
	a.identifierJSON = fields["identifier"]
	a.platformJSON = fields["platform"]
	a.seedIDJSON = fields["seedId"]
	a.nameValue = a.Name
	a.identifierValue = a.Identifier
	a.platformValue = a.Platform
	a.seedIDValue = a.SeedID
	return nil
}

// MarshalJSON preserves fields Apple supplied in sparse responses while still
// serializing values changed by callers after decoding.
func (a BundleIDAttributes) MarshalJSON() ([]byte, error) {
	if !a.attributesDecoded {
		type alias BundleIDAttributes
		return json.Marshal(alias(a))
	}
	if a.attributesNull {
		return []byte("null"), nil
	}

	type attributesJSON struct {
		Name       json.RawMessage `json:"name,omitempty"`
		Identifier json.RawMessage `json:"identifier,omitempty"`
		Platform   json.RawMessage `json:"platform,omitempty"`
		SeedID     json.RawMessage `json:"seedId,omitempty"`
	}
	fields := attributesJSON{}
	var err error
	if len(a.nameJSON) > 0 || a.Name != "" {
		fields.Name, err = sparseAttributeJSON(a.nameJSON, a.nameValue, a.Name)
		if err != nil {
			return nil, err
		}
	}
	if len(a.identifierJSON) > 0 || a.Identifier != "" {
		fields.Identifier, err = sparseAttributeJSON(a.identifierJSON, a.identifierValue, a.Identifier)
		if err != nil {
			return nil, err
		}
	}
	if len(a.platformJSON) > 0 || a.Platform != "" {
		fields.Platform, err = sparseAttributeJSON(a.platformJSON, a.platformValue, a.Platform)
		if err != nil {
			return nil, err
		}
	}
	if len(a.seedIDJSON) > 0 || a.SeedID != "" {
		fields.SeedID, err = sparseAttributeJSON(a.seedIDJSON, a.seedIDValue, a.SeedID)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

// BundleIDCreateAttributes describes attributes for creating a bundle ID.
type BundleIDCreateAttributes struct {
	Name       string           `json:"name"`
	Identifier string           `json:"identifier"`
	Platform   BundleIDPlatform `json:"platform"`
}

// BundleIDUpdateAttributes describes attributes for updating a bundle ID.
type BundleIDUpdateAttributes struct {
	Name string `json:"name,omitempty"`
}

// BundleIDCreateData is the data portion of a bundle ID create request.
type BundleIDCreateData struct {
	Type       ResourceType             `json:"type"`
	Attributes BundleIDCreateAttributes `json:"attributes"`
}

// BundleIDCreateRequest is a request to create a bundle ID.
type BundleIDCreateRequest struct {
	Data BundleIDCreateData `json:"data"`
}

// BundleIDUpdateData is the data portion of a bundle ID update request.
type BundleIDUpdateData struct {
	Type       ResourceType              `json:"type"`
	ID         string                    `json:"id"`
	Attributes *BundleIDUpdateAttributes `json:"attributes,omitempty"`
}

// BundleIDUpdateRequest is a request to update a bundle ID.
type BundleIDUpdateRequest struct {
	Data BundleIDUpdateData `json:"data"`
}

// BundleIDCapabilityAttributes describes a bundle ID capability resource.
type BundleIDCapabilityAttributes struct {
	CapabilityType string              `json:"capabilityType"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// BundleIDCapabilityCreateAttributes describes attributes for creating a capability.
type BundleIDCapabilityCreateAttributes struct {
	CapabilityType string              `json:"capabilityType"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// CapabilitySetting describes a capability setting.
type CapabilitySetting struct {
	Key              string             `json:"key"`
	Name             string             `json:"name,omitempty"`
	Description      string             `json:"description,omitempty"`
	EnabledByDefault *bool              `json:"enabledByDefault,omitempty"`
	Visible          *bool              `json:"visible,omitempty"`
	AllowedInstances string             `json:"allowedInstances,omitempty"`
	MinInstances     *int               `json:"minInstances,omitempty"`
	Options          []CapabilityOption `json:"options,omitempty"`
}

// CapabilityOption describes a capability option.
type CapabilityOption struct {
	Key              string `json:"key"`
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	EnabledByDefault *bool  `json:"enabledByDefault,omitempty"`
	Enabled          *bool  `json:"enabled,omitempty"`
	SupportsWildcard *bool  `json:"supportsWildcard,omitempty"`
}

// BundleIDCapabilityRelationships describes relationships for bundle ID capabilities.
type BundleIDCapabilityRelationships struct {
	BundleID *Relationship `json:"bundleId"`
}

// BundleIDCapabilityCreateData is the data portion of a capability create request.
type BundleIDCapabilityCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    BundleIDCapabilityCreateAttributes `json:"attributes"`
	Relationships *BundleIDCapabilityRelationships   `json:"relationships"`
}

// BundleIDCapabilityCreateRequest is a request to create a bundle ID capability.
type BundleIDCapabilityCreateRequest struct {
	Data BundleIDCapabilityCreateData `json:"data"`
}

// BundleIDCapabilityUpdateAttributes describes attributes for updating a capability.
type BundleIDCapabilityUpdateAttributes struct {
	CapabilityType string              `json:"capabilityType,omitempty"`
	Settings       []CapabilitySetting `json:"settings,omitempty"`
}

// BundleIDCapabilityUpdateData is the data portion of a capability update request.
type BundleIDCapabilityUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *BundleIDCapabilityUpdateAttributes `json:"attributes,omitempty"`
}

// BundleIDCapabilityUpdateRequest is a request to update a bundle ID capability.
type BundleIDCapabilityUpdateRequest struct {
	Data BundleIDCapabilityUpdateData `json:"data"`
}

// BundleIDsResponse is the response from bundle IDs list endpoint.
type BundleIDsResponse = Response[BundleIDAttributes]

// BundleIDResponse is the response from bundle ID detail endpoint.
type BundleIDResponse = SingleResponse[BundleIDAttributes]

// BundleIDCapabilitiesResponse is the response from bundle ID capabilities endpoint.
type BundleIDCapabilitiesResponse = Response[BundleIDCapabilityAttributes]

// BundleIDCapabilityResponse is the response from bundle ID capability detail endpoint.
type BundleIDCapabilityResponse = SingleResponse[BundleIDCapabilityAttributes]

// CertificateAttributes describes a certificate resource.
type CertificateAttributes struct {
	Name               string `json:"name"`
	CertificateType    string `json:"certificateType"`
	DisplayName        string `json:"displayName,omitempty"`
	SerialNumber       string `json:"serialNumber,omitempty"`
	Platform           string `json:"platform,omitempty"`
	ExpirationDate     string `json:"expirationDate,omitempty"`
	CertificateContent string `json:"certificateContent,omitempty"`
	Activated          *bool  `json:"activated,omitempty"`

	attributesDecoded       bool
	attributesNull          bool
	nameJSON                json.RawMessage
	certificateTypeJSON     json.RawMessage
	displayNameJSON         json.RawMessage
	serialNumberJSON        json.RawMessage
	platformJSON            json.RawMessage
	expirationDateJSON      json.RawMessage
	certificateContentJSON  json.RawMessage
	activatedJSON           json.RawMessage
	nameValue               string
	certificateTypeValue    string
	displayNameValue        string
	serialNumberValue       string
	platformValue           string
	expirationDateValue     string
	certificateContentValue string
	activatedValue          *bool
}

// UnmarshalJSON retains sparse field presence so list responses can be
// rendered without filling omitted certificate fields with zero values.
func (a *CertificateAttributes) UnmarshalJSON(data []byte) error {
	type alias CertificateAttributes
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = CertificateAttributes(decoded)
	a.attributesDecoded = true
	a.attributesNull = bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	a.nameJSON = fields["name"]
	a.certificateTypeJSON = fields["certificateType"]
	a.displayNameJSON = fields["displayName"]
	a.serialNumberJSON = fields["serialNumber"]
	a.platformJSON = fields["platform"]
	a.expirationDateJSON = fields["expirationDate"]
	a.certificateContentJSON = fields["certificateContent"]
	a.activatedJSON = fields["activated"]
	a.nameValue = a.Name
	a.certificateTypeValue = a.CertificateType
	a.displayNameValue = a.DisplayName
	a.serialNumberValue = a.SerialNumber
	a.platformValue = a.Platform
	a.expirationDateValue = a.ExpirationDate
	a.certificateContentValue = a.CertificateContent
	a.activatedValue = cloneBool(a.Activated)
	return nil
}

// MarshalJSON preserves the fields Apple supplied for decoded sparse
// responses while keeping ordinary struct construction unchanged.
func (a CertificateAttributes) MarshalJSON() ([]byte, error) {
	if !a.attributesDecoded {
		type alias CertificateAttributes
		return json.Marshal(alias(a))
	}
	if a.attributesNull {
		return []byte("null"), nil
	}
	fields := struct {
		Name               json.RawMessage `json:"name,omitempty"`
		CertificateType    json.RawMessage `json:"certificateType,omitempty"`
		DisplayName        json.RawMessage `json:"displayName,omitempty"`
		SerialNumber       json.RawMessage `json:"serialNumber,omitempty"`
		Platform           json.RawMessage `json:"platform,omitempty"`
		ExpirationDate     json.RawMessage `json:"expirationDate,omitempty"`
		CertificateContent json.RawMessage `json:"certificateContent,omitempty"`
		Activated          json.RawMessage `json:"activated,omitempty"`
	}{}
	var err error
	if len(a.nameJSON) > 0 || a.Name != "" {
		fields.Name, err = sparseAttributeJSON(a.nameJSON, a.nameValue, a.Name)
		if err != nil {
			return nil, err
		}
	}
	if len(a.certificateTypeJSON) > 0 || a.CertificateType != "" {
		fields.CertificateType, err = sparseAttributeJSON(a.certificateTypeJSON, a.certificateTypeValue, a.CertificateType)
		if err != nil {
			return nil, err
		}
	}
	if len(a.displayNameJSON) > 0 || a.DisplayName != "" {
		fields.DisplayName, err = sparseAttributeJSON(a.displayNameJSON, a.displayNameValue, a.DisplayName)
		if err != nil {
			return nil, err
		}
	}
	if len(a.serialNumberJSON) > 0 || a.SerialNumber != "" {
		fields.SerialNumber, err = sparseAttributeJSON(a.serialNumberJSON, a.serialNumberValue, a.SerialNumber)
		if err != nil {
			return nil, err
		}
	}
	if len(a.platformJSON) > 0 || a.Platform != "" {
		fields.Platform, err = sparseAttributeJSON(a.platformJSON, a.platformValue, a.Platform)
		if err != nil {
			return nil, err
		}
	}
	if len(a.expirationDateJSON) > 0 || a.ExpirationDate != "" {
		fields.ExpirationDate, err = sparseAttributeJSON(a.expirationDateJSON, a.expirationDateValue, a.ExpirationDate)
		if err != nil {
			return nil, err
		}
	}
	if len(a.certificateContentJSON) > 0 || a.CertificateContent != "" {
		fields.CertificateContent, err = sparseAttributeJSON(a.certificateContentJSON, a.certificateContentValue, a.CertificateContent)
		if err != nil {
			return nil, err
		}
	}
	if len(a.activatedJSON) > 0 || a.Activated != nil {
		fields.Activated, err = sparseAttributeJSON(a.activatedJSON, a.activatedValue, a.Activated)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

func sparseAttributeJSON(raw json.RawMessage, decoded, current any) (json.RawMessage, error) {
	if len(raw) > 0 && reflect.DeepEqual(decoded, current) {
		return raw, nil
	}
	return json.Marshal(current)
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// CertificateCreateAttributes describes attributes for creating a certificate.
type CertificateCreateAttributes struct {
	CertificateType string `json:"certificateType"`
	CSRContent      string `json:"csrContent"`
}

// CertificateCreateRelationships describes relationships for certificate creation.
type CertificateCreateRelationships struct {
	MerchantID *Relationship `json:"merchantId,omitempty"`
	PassTypeID *Relationship `json:"passTypeId,omitempty"`
}

// CertificateCreateData is the data portion of a certificate create request.
type CertificateCreateData struct {
	Type          ResourceType                    `json:"type"`
	Attributes    CertificateCreateAttributes     `json:"attributes"`
	Relationships *CertificateCreateRelationships `json:"relationships,omitempty"`
}

// CertificateCreateRequest is a request to create a certificate.
type CertificateCreateRequest struct {
	Data CertificateCreateData `json:"data"`
}

// CertificateUpdateAttributes describes attributes for updating a certificate.
type CertificateUpdateAttributes struct {
	Activated *bool `json:"activated,omitempty"`
}

// CertificateUpdateData is the data portion of a certificate update request.
type CertificateUpdateData struct {
	Type       ResourceType                 `json:"type"`
	ID         string                       `json:"id"`
	Attributes *CertificateUpdateAttributes `json:"attributes,omitempty"`
}

// CertificateUpdateRequest is a request to update a certificate.
type CertificateUpdateRequest struct {
	Data CertificateUpdateData `json:"data"`
}

// CertificatesResponse is the response from certificates list endpoint.
type CertificatesResponse = Response[CertificateAttributes]

// CertificateResponse is the response from certificate detail endpoint.
type CertificateResponse = SingleResponse[CertificateAttributes]

// ProfileState represents profile state values.
type ProfileState string

const (
	ProfileStateActive  ProfileState = "ACTIVE"
	ProfileStateInvalid ProfileState = "INVALID"
)

// ProfileAttributes describes a profile resource.
type ProfileAttributes struct {
	Name                string       `json:"name,omitempty"`
	Platform            Platform     `json:"platform,omitempty"`
	ProfileType         string       `json:"profileType,omitempty"`
	ProfileState        ProfileState `json:"profileState,omitempty"`
	ProfileContent      string       `json:"profileContent,omitempty"`
	UUID                string       `json:"uuid,omitempty"`
	CreatedDate         string       `json:"createdDate,omitempty"`
	ExpirationDate      string       `json:"expirationDate,omitempty"`
	attributesPresent   bool
	attributesNull      bool
	nameJSON            json.RawMessage
	platformJSON        json.RawMessage
	profileTypeJSON     json.RawMessage
	profileStateJSON    json.RawMessage
	profileContentJSON  json.RawMessage
	uuidJSON            json.RawMessage
	createdDateJSON     json.RawMessage
	expirationDateJSON  json.RawMessage
	nameValue           string
	platformValue       Platform
	profileTypeValue    string
	profileStateValue   ProfileState
	profileContentValue string
	uuidValue           string
	createdDateValue    string
	expirationDateValue string
}

// ProfileCreateAttributes describes attributes for creating a profile.
type ProfileCreateAttributes struct {
	Name        string   `json:"name"`
	Platform    Platform `json:"platform,omitempty"`
	ProfileType string   `json:"profileType"`
}

// ProfileCreateRelationships describes relationships for profile creation.
type ProfileCreateRelationships struct {
	BundleID     *Relationship     `json:"bundleId"`
	Certificates *RelationshipList `json:"certificates"`
	Devices      *RelationshipList `json:"devices,omitempty"`
}

// ProfileCreateData is the data portion of a profile create request.
type ProfileCreateData struct {
	Type          ResourceType                `json:"type"`
	Attributes    ProfileCreateAttributes     `json:"attributes"`
	Relationships *ProfileCreateRelationships `json:"relationships"`
}

// ProfileCreateRequest is a request to create a profile.
type ProfileCreateRequest struct {
	Data ProfileCreateData `json:"data"`
}

// ProfilesResponse is the response from profiles list endpoint.
type ProfilesResponse Response[ProfileAttributes]

// ProfileResponse is the response from profile detail endpoint.
type ProfileResponse = SingleResponse[ProfileAttributes]

// ProfileCertificatesLinkagesResponse is the response from profile certificates linkage endpoint.
type ProfileCertificatesLinkagesResponse = LinkagesResponse

// ProfileDevicesLinkagesResponse is the response from profile devices linkage endpoint.
type ProfileDevicesLinkagesResponse = LinkagesResponse

// ProfileBundleIDLinkageResponse is the response from profile bundle ID linkage endpoint.
type ProfileBundleIDLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}
