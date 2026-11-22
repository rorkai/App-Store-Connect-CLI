package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// appComplianceRequirementNames maps the form codes App Store Connect uses for
// App Compliance requirements to stable names. The service sends no other
// name, and an unmapped code is reported as-is.
var appComplianceRequirementNames = map[string]string{
	"SECURITYASF395E": "SECURITY_ASSESSMENT",
	"AIGENERATEFD1DE": "AI_GENERATED_CONTENT",
}

// AppComplianceRequirement reports one requirement App Store Connect tracks
// for an app on its App Compliance page.
type AppComplianceRequirement struct {
	AppID              string
	RequirementID      string
	RequirementName    string
	FormName           string
	Status             string
	Required           bool
	CountriesOrRegions []string
	FormID             string
	DateSigned         string
	LastUpdatedDate    string
	RequiredActions    []string
}

// ListAppComplianceRequirements lists the App Compliance requirements for an
// app, answered or not. An app with none gets an empty list.
func (c *Client) ListAppComplianceRequirements(ctx context.Context, accountID, appID string) ([]AppComplianceRequirement, error) {
	accountID = strings.TrimSpace(accountID)
	appID = strings.TrimSpace(appID)
	if accountID == "" {
		return nil, fmt.Errorf("account id is required")
	}
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}

	path := "/ppm/compliancerequirements/v1/accounts/" + url.PathEscape(accountID) + "/requirements?entity_id=" + url.QueryEscape(appID)
	responseBody, err := c.doAppComplianceRequest(ctx, appID, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []struct {
			ID              string   `json:"id"`
			FormName        string   `json:"formName"`
			Status          string   `json:"status"`
			IsRequired      bool     `json:"isRequired"`
			Countries       []string `json:"countries"`
			FormID          string   `json:"formId"`
			DateSigned      string   `json:"dateSigned"`
			LastUpdatedDate string   `json:"lastUpdatedDate"`
			RequiredActions []string `json:"requiredActions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app compliance requirements response: %w", err)
	}

	requirements := make([]AppComplianceRequirement, 0, len(payload.Data))
	for _, item := range payload.Data {
		name, ok := appComplianceRequirementNames[item.FormName]
		if !ok {
			name = item.FormName
		}
		requirements = append(requirements, AppComplianceRequirement{
			AppID:              appID,
			RequirementID:      item.ID,
			RequirementName:    name,
			FormName:           item.FormName,
			Status:             item.Status,
			Required:           item.IsRequired,
			CountriesOrRegions: item.Countries,
			FormID:             item.FormID,
			DateSigned:         item.DateSigned,
			LastUpdatedDate:    item.LastUpdatedDate,
			RequiredActions:    item.RequiredActions,
		})
	}
	return requirements, nil
}
