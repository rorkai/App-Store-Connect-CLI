package asc

// MerchantIDAttributes describes a merchant ID resource.
type MerchantIDAttributes struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

// MerchantIDUpdateAttributes describes attributes for updating a merchant ID.
type MerchantIDUpdateAttributes struct {
	Name *string `json:"name"`
}

// MerchantIDCertificatesLinkagesResponse is the response from merchant ID certificate linkages endpoints.
type MerchantIDCertificatesLinkagesResponse = LinkagesResponse
