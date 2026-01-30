package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBundleIDs retrieves the list of bundle IDs.
func (c *Client) GetBundleIDs(ctx context.Context, opts ...BundleIDsOption) (*BundleIDsResponse, error) {
	query := &bundleIDsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/bundleIds"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIds: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBundleIDsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BundleIDsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBundleID retrieves a single bundle ID by ID.
func (c *Client) GetBundleID(ctx context.Context, id string) (*BundleIDResponse, error) {
	id = strings.TrimSpace(id)
	path := fmt.Sprintf("/v1/bundleIds/%s", id)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BundleIDResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBundleID creates a new bundle ID.
func (c *Client) CreateBundleID(ctx context.Context, attrs BundleIDCreateAttributes) (*BundleIDResponse, error) {
	request := BundleIDCreateRequest{
		Data: BundleIDCreateData{
			Type:       ResourceTypeBundleIds,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/bundleIds", body)
	if err != nil {
		return nil, err
	}

	var response BundleIDResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBundleID updates an existing bundle ID.
func (c *Client) UpdateBundleID(ctx context.Context, id string, attrs BundleIDUpdateAttributes) (*BundleIDResponse, error) {
	id = strings.TrimSpace(id)
	request := BundleIDUpdateRequest{
		Data: BundleIDUpdateData{
			Type:       ResourceTypeBundleIds,
			ID:         id,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/bundleIds/%s", id), body)
	if err != nil {
		return nil, err
	}

	var response BundleIDResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBundleID deletes a bundle ID by ID.
func (c *Client) DeleteBundleID(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	path := fmt.Sprintf("/v1/bundleIds/%s", id)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetBundleIDCapabilities retrieves capabilities for a bundle ID.
func (c *Client) GetBundleIDCapabilities(ctx context.Context, bundleID string, opts ...BundleIDCapabilitiesOption) (*BundleIDCapabilitiesResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	query := &bundleIDCapabilitiesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/bundleIds/%s/bundleIdCapabilities", bundleID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIdCapabilities: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBundleIDCapabilitiesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BundleIDCapabilitiesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBundleIDCapability adds a capability to a bundle ID.
func (c *Client) CreateBundleIDCapability(ctx context.Context, bundleID string, attrs BundleIDCapabilityCreateAttributes) (*BundleIDCapabilityResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	request := BundleIDCapabilityCreateRequest{
		Data: BundleIDCapabilityCreateData{
			Type:       ResourceTypeBundleIdCapabilities,
			Attributes: attrs,
			Relationships: &BundleIDCapabilityRelationships{
				BundleID: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBundleIds,
						ID:   bundleID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/bundleIdCapabilities", body)
	if err != nil {
		return nil, err
	}

	var response BundleIDCapabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBundleIDCapability deletes a bundle ID capability by ID.
func (c *Client) DeleteBundleIDCapability(ctx context.Context, capabilityID string) error {
	capabilityID = strings.TrimSpace(capabilityID)
	path := fmt.Sprintf("/v1/bundleIdCapabilities/%s", capabilityID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetCertificates retrieves the list of certificates.
func (c *Client) GetCertificates(ctx context.Context, opts ...CertificatesOption) (*CertificatesResponse, error) {
	query := &certificatesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/certificates"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("certificates: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCertificatesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CertificatesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCertificate retrieves a single certificate by ID.
func (c *Client) GetCertificate(ctx context.Context, id string, opts ...CertificatesOption) (*CertificateResponse, error) {
	id = strings.TrimSpace(id)
	query := &certificatesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/certificates/%s", id)
	if queryString := buildCertificatesQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CertificateResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateCertificate creates a new certificate.
func (c *Client) CreateCertificate(ctx context.Context, csrContent string, certType string) (*CertificateResponse, error) {
	request := CertificateCreateRequest{
		Data: CertificateCreateData{
			Type: ResourceTypeCertificates,
			Attributes: CertificateCreateAttributes{
				CertificateType: strings.TrimSpace(certType),
				CSRContent:      strings.TrimSpace(csrContent),
			},
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/certificates", body)
	if err != nil {
		return nil, err
	}

	var response CertificateResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// RevokeCertificate revokes a certificate by ID.
func (c *Client) RevokeCertificate(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	path := fmt.Sprintf("/v1/certificates/%s", id)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// RegisterDevice registers a new device.
func (c *Client) RegisterDevice(ctx context.Context, attrs DeviceCreateAttributes) (*DeviceResponse, error) {
	request := DeviceCreateRequest{
		Data: DeviceCreateData{
			Type:       ResourceTypeDevices,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/devices", body)
	if err != nil {
		return nil, err
	}

	var response DeviceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfiles retrieves the list of profiles.
func (c *Client) GetProfiles(ctx context.Context, opts ...ProfilesOption) (*ProfilesResponse, error) {
	query := &profilesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/profiles"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("profiles: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildProfilesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfilesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfile retrieves a single profile by ID.
func (c *Client) GetProfile(ctx context.Context, id string, opts ...ProfilesOption) (*ProfileResponse, error) {
	id = strings.TrimSpace(id)
	query := &profilesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/profiles/%s", id)
	if queryString := buildProfilesQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfileResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfileBundleID retrieves the bundle ID for a profile.
func (c *Client) GetProfileBundleID(ctx context.Context, profileID string) (*BundleIDResponse, error) {
	profileID = strings.TrimSpace(profileID)
	path := fmt.Sprintf("/v1/profiles/%s/bundleId", profileID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BundleIDResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfileCertificates retrieves certificates for a profile.
func (c *Client) GetProfileCertificates(ctx context.Context, profileID string, opts ...ProfileCertificatesOption) (*CertificatesResponse, error) {
	profileID = strings.TrimSpace(profileID)
	query := &profileCertificatesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/profiles/%s/certificates", profileID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("profileCertificates: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildProfileCertificatesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CertificatesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfileDevices retrieves devices for a profile.
func (c *Client) GetProfileDevices(ctx context.Context, profileID string, opts ...ProfileDevicesOption) (*DevicesResponse, error) {
	profileID = strings.TrimSpace(profileID)
	query := &profileDevicesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/profiles/%s/devices", profileID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("profileDevices: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildProfileDevicesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response DevicesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfileBundleIDRelationship retrieves the bundle ID linkage for a profile.
func (c *Client) GetProfileBundleIDRelationship(ctx context.Context, profileID string) (*ProfileBundleIDLinkageResponse, error) {
	profileID = strings.TrimSpace(profileID)
	path := fmt.Sprintf("/v1/profiles/%s/relationships/bundleId", profileID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfileBundleIDLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfileCertificatesRelationships retrieves certificate linkages for a profile.
func (c *Client) GetProfileCertificatesRelationships(ctx context.Context, profileID string, opts ...LinkagesOption) (*ProfileCertificatesLinkagesResponse, error) {
	profileID = strings.TrimSpace(profileID)
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/profiles/%s/relationships/certificates", profileID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("profileCertificatesRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfileCertificatesLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetProfileDevicesRelationships retrieves device linkages for a profile.
func (c *Client) GetProfileDevicesRelationships(ctx context.Context, profileID string, opts ...LinkagesOption) (*ProfileDevicesLinkagesResponse, error) {
	profileID = strings.TrimSpace(profileID)
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/profiles/%s/relationships/devices", profileID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("profileDevicesRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfileDevicesLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateProfile creates a new provisioning profile.
func (c *Client) CreateProfile(ctx context.Context, attrs ProfileCreateAttributes, bundleID string, certificateIDs []string, deviceIDs []string) (*ProfileResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	certificateIDs = normalizeList(certificateIDs)
	deviceIDs = normalizeList(deviceIDs)

	relationships := &ProfileCreateRelationships{
		BundleID: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeBundleIds,
				ID:   bundleID,
			},
		},
		Certificates: &RelationshipList{
			Data: make([]ResourceData, 0, len(certificateIDs)),
		},
	}
	for _, certificateID := range certificateIDs {
		relationships.Certificates.Data = append(relationships.Certificates.Data, ResourceData{
			Type: ResourceTypeCertificates,
			ID:   certificateID,
		})
	}
	if len(deviceIDs) > 0 {
		relationships.Devices = &RelationshipList{
			Data: make([]ResourceData, 0, len(deviceIDs)),
		}
		for _, deviceID := range deviceIDs {
			relationships.Devices.Data = append(relationships.Devices.Data, ResourceData{
				Type: ResourceTypeDevices,
				ID:   deviceID,
			})
		}
	}

	request := ProfileCreateRequest{
		Data: ProfileCreateData{
			Type:          ResourceTypeProfiles,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/profiles", body)
	if err != nil {
		return nil, err
	}

	var response ProfileResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteProfile deletes a profile by ID.
func (c *Client) DeleteProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	path := fmt.Sprintf("/v1/profiles/%s", id)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}
