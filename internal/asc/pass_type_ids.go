package asc

// PassTypeIDAttributes describes a pass type ID resource.
type PassTypeIDAttributes struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

// PassTypeIDUpdateAttributes describes attributes for updating a pass type ID.
type PassTypeIDUpdateAttributes struct {
	Name *NullableString `json:"name,omitempty"`
}

// PassTypeIDCertificatesLinkagesResponse is the response from pass type ID certificates linkages endpoints.
type PassTypeIDCertificatesLinkagesResponse = LinkagesResponse

// CertificatePassTypeIDLinkageResponse is the response from certificate pass type ID linkage endpoint.
type CertificatePassTypeIDLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}
