package asc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const bundleIDsIdentifierFilterMaxLength = 3900

type bundleIDsPaginationRequiredError struct {
	err error
}

func (e *bundleIDsPaginationRequiredError) Error() string {
	return e.err.Error()
}

func (e *bundleIDsPaginationRequiredError) Unwrap() error {
	return e.err
}

// IsBundleIDsPaginationRequired reports whether a request can only be
// represented by following multiple continuation URLs.
func IsBundleIDsPaginationRequired(err error) bool {
	var paginationErr *bundleIDsPaginationRequiredError
	return errors.As(err, &paginationErr)
}

// ValidateBundleIDsRequest validates URL capacity and split-request behavior
// without requiring an authenticated client.
func ValidateBundleIDsRequest(opts ...BundleIDsOption) error {
	query := &bundleIDsQuery{splitPagination: true}
	for _, opt := range opts {
		opt(query)
	}
	return validateBundleIDsRequest(query)
}

func validateBundleIDsRequest(query *bundleIDsQuery) error {
	if query.nextURL != "" {
		return nil
	}

	if len(bundleIDsRequestPath(query)) > bundleIDsIdentifierFilterMaxLength && !strings.Contains(strings.TrimSpace(query.identifier), ",") {
		return fmt.Errorf("bundleIds: request exceeds %d-byte URL limit and cannot be split without multiple filter[identifier] values", bundleIDsIdentifierFilterMaxLength)
	}

	if !shouldSplitBundleIDsIdentifierFilter(query) {
		return nil
	}
	if query.splitPaginationSet && !query.splitPagination {
		return &bundleIDsPaginationRequiredError{err: fmt.Errorf("bundleIds: split identifier filter requires --paginate because multiple continuation URLs cannot be represented; use pagination")}
	}
	if err := validateBundleIDsSplitSort(query); err != nil {
		return err
	}
	_, err := splitBundleIDsIdentifierFilter(query)
	return err
}

// BundleIDsRequestRequiresSplit reports whether the request must split its
// identifier filter to stay within the supported request URL length.
func BundleIDsRequestRequiresSplit(opts ...BundleIDsOption) bool {
	query := &bundleIDsQuery{splitPagination: true}
	for _, opt := range opts {
		opt(query)
	}
	return query.nextURL == "" && shouldSplitBundleIDsIdentifierFilter(query)
}

// GetBundleIDs retrieves the list of bundle IDs.
func (c *Client) GetBundleIDs(ctx context.Context, opts ...BundleIDsOption) (*BundleIDsResponse, error) {
	query := &bundleIDsQuery{splitPagination: true}
	for _, opt := range opts {
		opt(query)
	}

	if err := validateBundleIDsRequest(query); err != nil {
		return nil, err
	}

	if shouldSplitBundleIDsIdentifierFilter(query) {
		return c.getBundleIDsWithSplitIdentifierFilter(ctx, query)
	}

	path := bundleIDsRequestPath(query)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIds: %w", err)
		}
		path = query.nextURL
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

// PaginateBundleIDs fetches and aggregates bundle ID pages while preserving
// unique included resources. It is intentionally endpoint-specific because
// generic pagination cannot know how to merge an Included JSON:API member.
func PaginateBundleIDs(ctx context.Context, firstPage *BundleIDsResponse, fetchNext func(context.Context, string) (*BundleIDsResponse, error)) (*BundleIDsResponse, error) {
	if firstPage == nil {
		return nil, nil
	}
	if fetchNext == nil {
		return nil, fmt.Errorf("bundleIds pagination requires a next-page fetcher")
	}

	result := &BundleIDsResponse{
		Data:  make([]Resource[BundleIDAttributes], 0, len(firstPage.Data)),
		Links: firstPage.Links,
		Meta:  firstPage.Meta,
	}
	included := make([]json.RawMessage, 0)
	includedSeen := make(map[string]struct{})
	includedPresent := false
	page := firstPage
	pageNumber := 1
	seenNext := make(map[string]struct{})

	for {
		result.Data = append(result.Data, page.Data...)
		includedPresent = includedPresent || bundleIDsIncludedArrayPresent(page.Included)
		if err := appendBundleIDsIncluded(&included, includedSeen, page.Included); err != nil {
			return result, fmt.Errorf("page %d: %w", pageNumber, err)
		}

		next := strings.TrimSpace(page.Links.Next)
		if next == "" {
			break
		}
		if _, ok := seenNext[next]; ok {
			return result, fmt.Errorf("page %d: %w", pageNumber+1, ErrRepeatedPaginationURL)
		}
		seenNext[next] = struct{}{}
		pageNumber++

		nextPage, err := fetchNext(ctx, next)
		if err != nil {
			return result, fmt.Errorf("page %d: %w", pageNumber, err)
		}
		if nextPage == nil {
			return result, fmt.Errorf("page %d: received nil response", pageNumber)
		}
		page = nextPage
		result.Links = Links{}
		result.Meta = nil
	}

	result.Links.Next = ""
	if includedPresent {
		mergedIncluded, err := json.Marshal(included)
		if err != nil {
			return result, fmt.Errorf("failed to merge included resources: %w", err)
		}
		result.Included = mergedIncluded
	}
	return result, nil
}

func shouldSplitBundleIDsIdentifierFilter(query *bundleIDsQuery) bool {
	identifier := strings.TrimSpace(query.identifier)
	return strings.Contains(identifier, ",") && len(bundleIDsRequestPath(query)) > bundleIDsIdentifierFilterMaxLength
}

func validateBundleIDsSplitSort(query *bundleIDsQuery) error {
	terms, ok := parseBundleIDSort(query.sort)
	if !ok || len(terms) == 0 || len(query.fields) == 0 {
		return nil
	}

	// Sparse fieldsets can omit sort keys, so local reordering would be wrong.
	fields := make(map[string]struct{}, len(query.fields))
	for _, field := range query.fields {
		fields[strings.TrimSpace(field)] = struct{}{}
	}
	for _, term := range terms {
		if term.field == "id" {
			continue
		}
		if _, ok := fields[term.field]; !ok {
			return fmt.Errorf("bundleIds: cannot preserve sort %q across split identifier requests because fields[bundleIds] omits %q", strings.TrimSpace(query.sort), term.field)
		}
	}
	return nil
}

func (c *Client) getBundleIDsWithSplitIdentifierFilter(ctx context.Context, query *bundleIDsQuery) (*BundleIDsResponse, error) {
	chunks, err := splitBundleIDsIdentifierFilter(query)
	if err != nil {
		return nil, err
	}
	combined := &BundleIDsResponse{
		Data: make([]Resource[BundleIDAttributes], 0),
	}
	included := make([]json.RawMessage, 0)
	includedSeen := make(map[string]struct{})
	includedPresent := false
	dataSeen := make(map[string]struct{})

	for _, chunk := range chunks {
		chunkQuery := *query
		chunkQuery.identifier = strings.Join(chunk, ",")
		page := 1
		seenNext := make(map[string]struct{})

		for {
			resp, err := c.getBundleIDsPage(ctx, &chunkQuery)
			if err != nil {
				return nil, err
			}
			appendBundleIDsData(&combined.Data, dataSeen, resp.Data)
			includedPresent = includedPresent || bundleIDsIncludedArrayPresent(resp.Included)
			if err := appendBundleIDsIncluded(&included, includedSeen, resp.Included); err != nil {
				return nil, err
			}
			next := strings.TrimSpace(resp.Links.Next)
			if next == "" || !query.splitPagination {
				break
			}
			if _, ok := seenNext[next]; ok {
				return nil, fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
			}
			seenNext[next] = struct{}{}
			page++
			chunkQuery = bundleIDsQuery{listQuery: listQuery{nextURL: next}}
		}
	}
	// Each identifier chunk is sorted independently by ASC. Re-sort the merged
	// resources so a large filter keeps the endpoint's documented ordering.
	sortBundleIDsData(combined.Data, query.sort)
	if includedPresent {
		mergedIncluded, err := json.Marshal(included)
		if err != nil {
			return nil, fmt.Errorf("failed to merge included resources: %w", err)
		}
		combined.Included = mergedIncluded
	}

	return combined, nil
}

func appendBundleIDsData(resources *[]Resource[BundleIDAttributes], seen map[string]struct{}, page []Resource[BundleIDAttributes]) {
	for _, resource := range page {
		if resource.Type == "" || resource.ID == "" {
			*resources = append(*resources, resource)
			continue
		}
		key := string(resource.Type) + "\x00" + resource.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		*resources = append(*resources, resource)
	}
}

type bundleIDSortTerm struct {
	field      string
	descending bool
}

func sortBundleIDsData(resources []Resource[BundleIDAttributes], sortValue string) {
	terms, ok := parseBundleIDSort(sortValue)
	if !ok || len(terms) == 0 {
		return
	}

	sort.SliceStable(resources, func(i, j int) bool {
		for _, term := range terms {
			left := bundleIDSortFieldValue(resources[i], term.field)
			right := bundleIDSortFieldValue(resources[j], term.field)
			if left == right {
				continue
			}
			if term.descending {
				return left > right
			}
			return left < right
		}
		return false
	})
}

func parseBundleIDSort(value string) ([]bundleIDSortTerm, bool) {
	terms := make([]bundleIDSortTerm, 0)
	for _, expression := range strings.Split(value, ",") {
		expression = strings.TrimSpace(expression)
		if expression == "" {
			continue
		}
		descending := strings.HasPrefix(expression, "-")
		field := strings.TrimPrefix(expression, "-")
		switch field {
		case "name", "platform", "identifier", "seedId", "id":
			terms = append(terms, bundleIDSortTerm{field: field, descending: descending})
		default:
			return nil, false
		}
	}
	return terms, true
}

func bundleIDSortFieldValue(resource Resource[BundleIDAttributes], field string) string {
	switch field {
	case "name":
		return resource.Attributes.Name
	case "platform":
		return string(resource.Attributes.Platform)
	case "identifier":
		return resource.Attributes.Identifier
	case "seedId":
		return resource.Attributes.SeedID
	case "id":
		return resource.ID
	default:
		return ""
	}
}

func appendBundleIDsIncluded(resources *[]json.RawMessage, seen map[string]struct{}, included json.RawMessage) error {
	trimmed := strings.TrimSpace(string(included))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	var next []json.RawMessage
	if err := json.Unmarshal(included, &next); err != nil {
		return fmt.Errorf("failed to parse included resources: %w", err)
	}
	for _, resource := range next {
		var identity struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(resource, &identity); err != nil {
			return fmt.Errorf("failed to parse included resource: %w", err)
		}
		key := identity.Type + "\x00" + identity.ID
		if identity.Type == "" || identity.ID == "" {
			key = string(resource)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		*resources = append(*resources, resource)
	}
	return nil
}

func bundleIDsIncludedArrayPresent(included json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(included))
	return trimmed != "" && trimmed != "null"
}

func (c *Client) getBundleIDsPage(ctx context.Context, query *bundleIDsQuery) (*BundleIDsResponse, error) {
	path := bundleIDsRequestPath(query)
	if strings.TrimSpace(query.nextURL) != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIds: %w", err)
		}
		path = query.nextURL
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

func bundleIDsRequestPath(query *bundleIDsQuery) string {
	path := "/v1/bundleIds"
	if queryString := buildBundleIDsQuery(query); queryString != "" {
		path += "?" + queryString
	}
	return path
}

func splitBundleIDsIdentifierFilter(query *bundleIDsQuery) ([][]string, error) {
	parts := strings.Split(query.identifier, ",")
	chunks := make([][]string, 0, 1)
	current := make([]string, 0)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		candidate := append(append([]string{}, current...), part)
		candidateQuery := *query
		candidateQuery.identifier = strings.Join(candidate, ",")
		if len(current) > 0 && len(bundleIDsRequestPath(&candidateQuery)) > bundleIDsIdentifierFilterMaxLength {
			chunks = append(chunks, current)
			current = []string{part}
			singleQuery := *query
			singleQuery.identifier = part
			if len(bundleIDsRequestPath(&singleQuery)) > bundleIDsIdentifierFilterMaxLength {
				return nil, fmt.Errorf("bundleIds: cannot split bundleIds identifier filter because fixed query parameters leave no room for a single identifier under the %d-byte URL limit", bundleIDsIdentifierFilterMaxLength)
			}
			continue
		}
		if len(current) == 0 && len(bundleIDsRequestPath(&candidateQuery)) > bundleIDsIdentifierFilterMaxLength {
			return nil, fmt.Errorf("bundleIds: cannot split bundleIds identifier filter because fixed query parameters leave no room for a single identifier under the %d-byte URL limit", bundleIDsIdentifierFilterMaxLength)
		}

		current = candidate
	}

	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks, nil
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

// UpdateBundleIDCapability updates an existing bundle ID capability.
func (c *Client) UpdateBundleIDCapability(ctx context.Context, capabilityID string, attrs BundleIDCapabilityUpdateAttributes) (*BundleIDCapabilityResponse, error) {
	capabilityID = strings.TrimSpace(capabilityID)
	if capabilityID == "" {
		return nil, fmt.Errorf("capability ID is required")
	}
	request := BundleIDCapabilityUpdateRequest{
		Data: BundleIDCapabilityUpdateData{
			Type:       ResourceTypeBundleIdCapabilities,
			ID:         capabilityID,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/bundleIdCapabilities/%s", capabilityID), body)
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
	if err := validateCertificateDetailQuery(query); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/certificates/%s", id)
	if queryString := buildCertificateDetailQuery(query); queryString != "" {
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

func validateCertificateDetailQuery(query *certificatesQuery) error {
	switch {
	case query.limit > 0:
		return fmt.Errorf("certificates: limit option cannot be used with GetCertificate")
	case query.nextURL != "":
		return fmt.Errorf("certificates: next URL option cannot be used with GetCertificate")
	case len(query.certificateTypes) > 0:
		return fmt.Errorf("certificates: certificate type option cannot be used with GetCertificate")
	case len(query.displayNames) > 0:
		return fmt.Errorf("certificates: display name option cannot be used with GetCertificate")
	case len(query.serialNumbers) > 0:
		return fmt.Errorf("certificates: serial number option cannot be used with GetCertificate")
	case len(query.ids) > 0:
		return fmt.Errorf("certificates: ID option cannot be used with GetCertificate")
	case strings.TrimSpace(query.sort) != "":
		return fmt.Errorf("certificates: sort option cannot be used with GetCertificate")
	default:
		return nil
	}
}

type certificateCreateOptions struct {
	passTypeID string
	merchantID string
}

// CertificateCreateOption configures certificate creation.
type CertificateCreateOption func(*certificateCreateOptions)

// WithCertificatePassTypeID associates a pass type ID with a pass certificate.
func WithCertificatePassTypeID(passTypeID string) CertificateCreateOption {
	return func(options *certificateCreateOptions) {
		options.passTypeID = strings.TrimSpace(passTypeID)
	}
}

// WithCertificateMerchantID associates a merchant ID with an Apple Pay
// certificate.
func WithCertificateMerchantID(merchantID string) CertificateCreateOption {
	return func(options *certificateCreateOptions) {
		options.merchantID = strings.TrimSpace(merchantID)
	}
}

// CreateCertificate creates a new certificate.
func (c *Client) CreateCertificate(ctx context.Context, csrContent string, certType string, opts ...CertificateCreateOption) (*CertificateResponse, error) {
	options := &certificateCreateOptions{}
	for _, opt := range opts {
		opt(options)
	}

	request := CertificateCreateRequest{
		Data: CertificateCreateData{
			Type: ResourceTypeCertificates,
			Attributes: CertificateCreateAttributes{
				CertificateType: strings.TrimSpace(certType),
				CSRContent:      strings.TrimSpace(csrContent),
			},
		},
	}
	if options.passTypeID != "" || options.merchantID != "" {
		relationships := &CertificateCreateRelationships{}
		if options.passTypeID != "" {
			relationships.PassTypeID = &Relationship{
				Data: ResourceData{
					Type: ResourceTypePassTypeIds,
					ID:   options.passTypeID,
				},
			}
		}
		if options.merchantID != "" {
			relationships.MerchantID = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeMerchantIds,
					ID:   options.merchantID,
				},
			}
		}
		request.Data.Relationships = relationships
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

// UpdateCertificate updates a certificate's attributes.
func (c *Client) UpdateCertificate(ctx context.Context, id string, attrs CertificateUpdateAttributes) (*CertificateResponse, error) {
	id = strings.TrimSpace(id)
	request := CertificateUpdateRequest{
		Data: CertificateUpdateData{
			Type:       ResourceTypeCertificates,
			ID:         id,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/certificates/%s", id), body)
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
	return getTypedResourceLinkages[ProfileCertificatesLinkagesResponse](
		c,
		ctx,
		profileID,
		"certificates",
		"profile ID",
		"/v1/profiles/%s/relationships/%s",
		"profileCertificatesRelationships",
		opts...,
	)
}

// GetProfileDevicesRelationships retrieves device linkages for a profile.
func (c *Client) GetProfileDevicesRelationships(ctx context.Context, profileID string, opts ...LinkagesOption) (*ProfileDevicesLinkagesResponse, error) {
	return getTypedResourceLinkages[ProfileDevicesLinkagesResponse](
		c,
		ctx,
		profileID,
		"devices",
		"profile ID",
		"/v1/profiles/%s/relationships/%s",
		"profileDevicesRelationships",
		opts...,
	)
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
