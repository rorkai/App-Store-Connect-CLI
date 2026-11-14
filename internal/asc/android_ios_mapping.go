package asc

import "encoding/json"

// AndroidToIosAppMappingDetailAttributes describes an android-to-iOS mapping resource.
type AndroidToIosAppMappingDetailAttributes struct {
	PackageName                                      string   `json:"packageName,omitempty"`
	AppSigningKeyPublicCertificateSha256Fingerprints []string `json:"appSigningKeyPublicCertificateSha256Fingerprints,omitempty"`
}

// AndroidToIosAppMappingDetailCreateAttributes describes attributes for creating a mapping.
type AndroidToIosAppMappingDetailCreateAttributes struct {
	PackageName                                      string   `json:"packageName"`
	AppSigningKeyPublicCertificateSha256Fingerprints []string `json:"appSigningKeyPublicCertificateSha256Fingerprints"`
}

// NullableStringSlice represents a nullable string array for update requests.
type NullableStringSlice struct {
	Value []string
}

// MarshalJSON outputs a JSON array or null.
func (n NullableStringSlice) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

// UnmarshalJSON parses a JSON array or null.
func (n *NullableStringSlice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var value []string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = value
	return nil
}

// AndroidToIosAppMappingDetailUpdateAttributes describes attributes for updating a mapping.
type AndroidToIosAppMappingDetailUpdateAttributes struct {
	PackageName                                      *NullableString      `json:"packageName,omitempty"`
	AppSigningKeyPublicCertificateSha256Fingerprints *NullableStringSlice `json:"appSigningKeyPublicCertificateSha256Fingerprints,omitempty"`
}

// AndroidToIosAppMappingDetailCreateRelationships describes relationships for creating a mapping.
type AndroidToIosAppMappingDetailCreateRelationships struct {
	App *Relationship `json:"app"`
}

// AndroidToIosAppMappingDetailCreateData is the data portion of a create request.
type AndroidToIosAppMappingDetailCreateData struct {
	Type          ResourceType                                    `json:"type"`
	Attributes    AndroidToIosAppMappingDetailCreateAttributes    `json:"attributes"`
	Relationships AndroidToIosAppMappingDetailCreateRelationships `json:"relationships"`
}

// AndroidToIosAppMappingDetailsResponse is the response from the mapping list endpoint.
type AndroidToIosAppMappingDetailsResponse = Response[AndroidToIosAppMappingDetailAttributes]
