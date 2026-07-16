package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const medicalDeviceRequirementName = "MEDICAL_DEVICE"

// MedicalDeviceDeclarationResult reports the resulting app-level declaration.
type MedicalDeviceDeclarationResult struct {
	AppID              string   `json:"appId"`
	RequirementID      string   `json:"requirementId"`
	RequirementName    string   `json:"requirementName"`
	Status             string   `json:"status,omitempty"`
	FormID             string   `json:"formId,omitempty"`
	Declared           bool     `json:"declared"`
	CountriesOrRegions []string `json:"countriesOrRegions,omitempty"`
}

type complianceRequirement struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Ref        string `json:"ref"`
	Status     string `json:"status"`
	DateSigned string `json:"dateSigned"`
	FormID     string `json:"formId"`
	IsRequired bool   `json:"isRequired"`
}

type complianceRequirementsResponse struct {
	AccountID       string `json:"accountId"`
	RequirementData []struct {
		ContentID    string                  `json:"contentId"`
		Requirements []complianceRequirement `json:"requirements"`
	} `json:"requirementData"`
}

type complianceConstraintOption struct {
	Value      string   `json:"value"`
	ListValues []string `json:"listValues"`
}

type complianceConstraint struct {
	AttributeName string                       `json:"attributeName"`
	Options       []complianceConstraintOption `json:"options"`
}

type medicalDeviceFormResponse struct {
	Constraints map[string]complianceConstraint `json:"constraints"`
}

func trimComplianceRequirement(req complianceRequirement) complianceRequirement {
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.Ref = strings.TrimSpace(req.Ref)
	req.Status = strings.TrimSpace(req.Status)
	req.DateSigned = strings.TrimSpace(req.DateSigned)
	req.FormID = strings.TrimSpace(req.FormID)
	return req
}

func trimComplianceRequirements(requirements []complianceRequirement) []complianceRequirement {
	trimmed := make([]complianceRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		trimmed = append(trimmed, trimComplianceRequirement(requirement))
	}
	return trimmed
}

func (c *Client) complianceFormBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
	switch {
	case strings.HasSuffix(baseURL, "/iris/v1"):
		return strings.TrimSuffix(baseURL, "/iris/v1")
	case strings.HasSuffix(baseURL, "/ci/api"):
		return strings.TrimSuffix(baseURL, "/ci/api")
	case baseURL == "":
		return appStoreBaseURL
	default:
		return baseURL
	}
}

func (c *Client) doAppComplianceRequest(ctx context.Context, appID, method, path string, body any) ([]byte, error) {
	baseURL := c.complianceFormBaseURL()
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	// The ppm/complianceform service rejects mutating requests with an empty
	// 403 unless this App Store Connect UI CSRF header is present; GETs work
	// without it, so send it unconditionally to match the web client.
	headers.Set("X-Csrf-Itc", "itc")
	headers.Set("Origin", baseURL)
	headers.Set("Referer", strings.TrimRight(baseURL, "/")+"/apps/"+url.PathEscape(strings.TrimSpace(appID))+"/distribution/info")
	return c.doRequestBase(ctx, baseURL, method, path, body, headers)
}

func normalizeMedicalDeviceRegion(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "":
		return ""
	case "EU":
		return "EEA"
	default:
		return value
	}
}

func medicalDeviceRegionsFromConstraints(constraints map[string]complianceConstraint) ([]string, error) {
	if len(constraints) == 0 {
		return nil, fmt.Errorf("medical device form constraints are missing")
	}

	seen := map[string]struct{}{}
	regions := make([]string, 0, 4)
	for _, constraint := range constraints {
		if strings.TrimSpace(constraint.AttributeName) != "countriesOrRegions" {
			continue
		}
		for _, option := range constraint.Options {
			if normalized := normalizeMedicalDeviceRegion(option.Value); normalized != "" {
				if _, ok := seen[normalized]; !ok {
					seen[normalized] = struct{}{}
					regions = append(regions, normalized)
				}
			}
			for _, listValue := range option.ListValues {
				if normalized := normalizeMedicalDeviceRegion(listValue); normalized != "" {
					if _, ok := seen[normalized]; !ok {
						seen[normalized] = struct{}{}
						regions = append(regions, normalized)
					}
				}
			}
		}
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("medical device countries/regions are missing from form metadata")
	}
	slices.Sort(regions)
	return regions, nil
}

func findComplianceRequirement(requirements []complianceRequirement, name string) *complianceRequirement {
	name = strings.TrimSpace(name)
	for _, requirement := range requirements {
		trimmed := trimComplianceRequirement(requirement)
		if trimmed.Name == name {
			copy := trimmed
			return &copy
		}
	}
	return nil
}

func (c *Client) listComplianceRequirements(ctx context.Context, accountID, appID string) ([]complianceRequirement, error) {
	accountID = strings.TrimSpace(accountID)
	appID = strings.TrimSpace(appID)
	if accountID == "" {
		return nil, fmt.Errorf("account id is required")
	}
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}

	path := "/ppm/complianceform/v1/accounts/" + url.PathEscape(accountID) + "/requirements?contentId=" + url.QueryEscape(appID)
	responseBody, err := c.doAppComplianceRequest(ctx, appID, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload complianceRequirementsResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse compliance requirements response: %w", err)
	}

	var fallback []complianceRequirement
	for _, item := range payload.RequirementData {
		switch strings.TrimSpace(item.ContentID) {
		case appID:
			return trimComplianceRequirements(item.Requirements), nil
		case "":
			if fallback == nil {
				fallback = trimComplianceRequirements(item.Requirements)
			}
		}
	}

	if fallback != nil {
		return fallback, nil
	}

	return nil, fmt.Errorf("no compliance requirements found for app %q", appID)
}

func (c *Client) getMedicalDeviceForm(ctx context.Context, accountID, appID, requirementID string) (*medicalDeviceFormResponse, error) {
	path := "/ppm/complianceform/v1/accounts/" + url.PathEscape(strings.TrimSpace(accountID)) +
		"/requirements/" + url.PathEscape(strings.TrimSpace(requirementID)) +
		"/forms?contentId=" + url.QueryEscape(strings.TrimSpace(appID))
	responseBody, err := c.doAppComplianceRequest(ctx, appID, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload medicalDeviceFormResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse medical device form response: %w", err)
	}
	return &payload, nil
}

// SetMedicalDeviceDeclaration sets the regulated medical device declaration.
//
// Only the false/no path is currently supported because Apple requires
// additional region-specific metadata for the true/yes flow.
func (c *Client) SetMedicalDeviceDeclaration(ctx context.Context, accountID, appID string, declared bool) (*MedicalDeviceDeclarationResult, error) {
	if declared {
		return nil, fmt.Errorf("only false is currently supported for the regulated medical device declaration")
	}

	requirements, err := c.listComplianceRequirements(ctx, accountID, appID)
	if err != nil {
		return nil, err
	}
	requirement := findComplianceRequirement(requirements, medicalDeviceRequirementName)
	if requirement == nil {
		return nil, fmt.Errorf("regulated medical device requirement was not found for app %q", strings.TrimSpace(appID))
	}

	form, err := c.getMedicalDeviceForm(ctx, accountID, appID, requirement.ID)
	if err != nil {
		return nil, err
	}
	countriesOrRegions, err := medicalDeviceRegionsFromConstraints(form.Constraints)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"accountId":          strings.TrimSpace(accountID),
		"contentId":          strings.TrimSpace(appID),
		"requirementId":      requirement.ID,
		"requirementName":    requirement.Name,
		"countriesOrRegions": countriesOrRegions,
		"medicalDeviceData": map[string]string{
			"declaration": "no",
		},
	}
	path := "/ppm/complianceform/v1/accounts/" + url.PathEscape(strings.TrimSpace(accountID)) +
		"/contents/" + url.PathEscape(strings.TrimSpace(appID)) +
		"/requirements/" + url.PathEscape(requirement.ID) +
		"/forms"
	if _, err := c.doAppComplianceRequest(ctx, appID, http.MethodPost, path, requestBody); err != nil {
		return nil, err
	}

	updatedRequirement := requirement
	if updatedRequirements, err := c.listComplianceRequirements(ctx, accountID, appID); err == nil {
		if refreshed := findComplianceRequirement(updatedRequirements, medicalDeviceRequirementName); refreshed != nil {
			updatedRequirement = refreshed
		}
	}

	return &MedicalDeviceDeclarationResult{
		AppID:              strings.TrimSpace(appID),
		RequirementID:      updatedRequirement.ID,
		RequirementName:    updatedRequirement.Name,
		Status:             updatedRequirement.Status,
		FormID:             updatedRequirement.FormID,
		Declared:           false,
		CountriesOrRegions: countriesOrRegions,
	}, nil
}
