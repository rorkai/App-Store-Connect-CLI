package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	medicalDeviceRequirementName = "MEDICAL_DEVICE"
	medicalDeviceDeclarationYes  = "yes"
	medicalDeviceDeclarationNo   = "no"
	medicalDeviceCollectedStatus = "COLLECTED"
)

var medicalDeviceSupportedRegions = []string{"EEA", "GBR", "USA"}

// MedicalDeviceDeclarationOptions controls the app-level regulated
// medical-device answer. Apple currently exposes only these three regions in
// the web form; the detailed registration/support/contact subform is a
// separate operation and is intentionally not represented here.
type MedicalDeviceDeclarationOptions struct {
	CountriesOrRegions []string
}

// MedicalDeviceDeclarationResult reports the resulting app-level declaration.
type MedicalDeviceDeclarationResult struct {
	AppID              string   `json:"appId"`
	RequirementID      string   `json:"requirementId"`
	RequirementName    string   `json:"requirementName"`
	Status             string   `json:"status,omitempty"`
	FormID             string   `json:"formId,omitempty"`
	Declared           bool     `json:"declared"`
	Changed            bool     `json:"changed"`
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
	Constraints        map[string]complianceConstraint `json:"constraints"`
	Data               json.RawMessage                 `json:"data"`
	CountriesOrRegions []string                        `json:"countriesOrRegions"`
	MedicalDeviceData  struct {
		Declaration string `json:"declaration"`
	} `json:"medicalDeviceData"`
	rawPayload map[string]any
}

func (r *medicalDeviceFormResponse) UnmarshalJSON(data []byte) error {
	type response medicalDeviceFormResponse
	var decoded response
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var rawPayload map[string]any
	if err := json.Unmarshal(data, &rawPayload); err != nil {
		return err
	}
	*r = medicalDeviceFormResponse(decoded)
	r.rawPayload = rawPayload
	return nil
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
	return c.doAppComplianceRequestWithHTTPClient(c.httpClient, ctx, appID, method, path, body)
}

func (c *Client) doAppComplianceRequestWithHTTPClient(client *http.Client, ctx context.Context, appID, method, path string, body any) ([]byte, error) {
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
	return c.doRequestBaseWithHTTPClient(client, ctx, baseURL, method, path, body, headers)
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

func normalizeMedicalDeviceRegions(values []string) ([]string, error) {
	if len(values) == 0 {
		return append([]string(nil), medicalDeviceSupportedRegions...), nil
	}

	allowed := make(map[string]struct{}, len(medicalDeviceSupportedRegions))
	for _, region := range medicalDeviceSupportedRegions {
		allowed[region] = struct{}{}
	}

	seen := make(map[string]struct{}, len(values))
	regions := make([]string, 0, len(values))
	for _, value := range values {
		region := normalizeMedicalDeviceRegion(value)
		if region == "" {
			continue
		}
		if _, ok := allowed[region]; !ok {
			return nil, fmt.Errorf("unsupported medical device country/region %q (supported: EEA, GBR, USA)", strings.TrimSpace(value))
		}
		if _, ok := seen[region]; ok {
			continue
		}
		seen[region] = struct{}{}
		regions = append(regions, region)
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("at least one medical device country/region is required")
	}
	slices.Sort(regions)
	return regions, nil
}

// NormalizeMedicalDeviceDeclarationRegions validates and canonicalizes the
// region values accepted by the medical-device web form. An empty list means
// Apple's captured default region set.
func NormalizeMedicalDeviceDeclarationRegions(values []string) ([]string, error) {
	return normalizeMedicalDeviceRegions(values)
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

func medicalDeviceFormObject(form *medicalDeviceFormResponse) (map[string]any, error) {
	if form == nil {
		return map[string]any{}, nil
	}

	trimmedData := bytes.TrimSpace(form.Data)
	if len(trimmedData) > 0 && !bytes.Equal(trimmedData, []byte("null")) {
		var object map[string]any
		if err := json.Unmarshal(trimmedData, &object); err == nil && object != nil {
			return object, nil
		}

		var list []map[string]any
		if err := json.Unmarshal(trimmedData, &list); err == nil {
			if len(list) == 0 {
				return map[string]any{}, nil
			}
			if list[0] != nil {
				return list[0], nil
			}
		}

		return nil, fmt.Errorf("medical device form data must be an object or a non-empty object array")
	}

	if len(form.rawPayload) == 0 {
		return map[string]any{}, nil
	}
	// Some compliance-form responses put the stored answer fields at the
	// top level instead of under `data`. Preserve that complete answer shape,
	// while omitting the response envelope's metadata fields.
	object := cloneMedicalDeviceMap(form.rawPayload)
	delete(object, "constraints")
	delete(object, "data")
	if len(object) == 0 {
		return map[string]any{}, nil
	}
	return object, nil
}

func cloneMedicalDeviceMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func medicalDeviceFormMedicalData(form *medicalDeviceFormResponse, formData map[string]any) (map[string]any, bool, error) {
	if raw, ok := formData["medicalDeviceData"]; ok && raw != nil {
		medicalData, ok := raw.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("medical device form medicalDeviceData must be an object")
		}
		medicalData = cloneMedicalDeviceMap(medicalData)
		declaration, _ := medicalData["declaration"].(string)
		return medicalData, strings.TrimSpace(declaration) != "", nil
	}

	if form != nil {
		if declaration := strings.TrimSpace(form.MedicalDeviceData.Declaration); declaration != "" {
			return map[string]any{"declaration": declaration}, true, nil
		}
	}
	return map[string]any{}, false, nil
}

func medicalDeviceRegistrationInfo(medicalData map[string]any) ([]any, error) {
	raw, ok := medicalData["registrationInfo"]
	if !ok || raw == nil {
		return nil, nil
	}
	registrationInfo, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("medical device registrationInfo must be an array")
	}
	return registrationInfo, nil
}

func medicalDeviceRegistrationRegion(value any) string {
	row, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	switch countries := row["countriesOrRegions"].(type) {
	case []any:
		if len(countries) == 0 {
			return ""
		}
		region, _ := countries[0].(string)
		return normalizeMedicalDeviceRegion(region)
	case []string:
		if len(countries) == 0 {
			return ""
		}
		return normalizeMedicalDeviceRegion(countries[0])
	case string:
		return normalizeMedicalDeviceRegion(countries)
	default:
		return ""
	}
}

func buildMedicalDeviceRegistrationInfo(medicalData map[string]any, declared bool, selected []string) ([]any, error) {
	existing, err := medicalDeviceRegistrationInfo(medicalData)
	if err != nil {
		return nil, err
	}
	if !declared {
		// Apple's No branch preserves existing regional rows; the app-level
		// declaration and requirement status define the current answer, while
		// historical regional details remain available for future edits.
		if existing == nil {
			return []any{}, nil
		}
		return existing, nil
	}

	selectedRegions := make(map[string]struct{}, len(selected))
	for _, region := range selected {
		selectedRegions[region] = struct{}{}
	}
	existingByRegion := make(map[string]map[string]any, len(existing))
	for _, value := range existing {
		region := medicalDeviceRegistrationRegion(value)
		row, ok := value.(map[string]any)
		if region == "" || !ok {
			continue
		}
		if _, already := existingByRegion[region]; !already {
			existingByRegion[region] = row
		}
	}

	updated := make([]any, 0, len(medicalDeviceSupportedRegions))
	for _, region := range medicalDeviceSupportedRegions {
		row := cloneMedicalDeviceMap(existingByRegion[region])
		if row == nil {
			row = make(map[string]any)
		}
		row["countriesOrRegions"] = []string{region}
		declaration := medicalDeviceDeclarationNo
		if _, ok := selectedRegions[region]; ok {
			declaration = medicalDeviceDeclarationYes
		}
		row["declaration"] = declaration
		updated = append(updated, row)
	}
	return updated, nil
}

func medicalDevicePersistedAffirmativeRegions(form *medicalDeviceFormResponse) ([]string, error) {
	formData, err := medicalDeviceFormObject(form)
	if err != nil {
		return nil, err
	}
	medicalData, _, err := medicalDeviceFormMedicalData(form, formData)
	if err != nil {
		return nil, err
	}

	rawRegistrationInfo, hasRegistrationInfo := medicalData["registrationInfo"]
	if !hasRegistrationInfo {
		regions := form.countriesOrRegions()
		if len(regions) == 0 {
			return nil, fmt.Errorf("medical device persisted region selections are missing")
		}
		return regions, nil
	}
	if rawRegistrationInfo == nil {
		return []string{}, nil
	}

	registrationInfo, err := medicalDeviceRegistrationInfo(medicalData)
	if err != nil {
		return nil, err
	}
	seenDeclarations := make(map[string]string, len(registrationInfo))
	regions := make([]string, 0, len(registrationInfo))
	for _, value := range registrationInfo {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("medical device registrationInfo contains a non-object region row")
		}
		region := medicalDeviceRegistrationRegion(row)
		if region == "" {
			return nil, fmt.Errorf("medical device registrationInfo contains a row without a country/region")
		}
		declaration, ok := row["declaration"].(string)
		if !ok {
			return nil, fmt.Errorf("medical device registrationInfo row for %q has no declaration", region)
		}
		declaration = strings.ToLower(strings.TrimSpace(declaration))
		if declaration != medicalDeviceDeclarationYes && declaration != medicalDeviceDeclarationNo {
			return nil, fmt.Errorf("medical device registrationInfo row for %q has unsupported declaration %q", region, declaration)
		}
		if previous, exists := seenDeclarations[region]; exists && previous != declaration {
			return nil, fmt.Errorf("medical device registrationInfo has conflicting declarations for %q", region)
		}
		seenDeclarations[region] = declaration
		if declaration == medicalDeviceDeclarationYes {
			regions = append(regions, region)
		}
	}
	slices.Sort(regions)
	return slices.Compact(regions), nil
}

func medicalDeviceDeclarationRequestBody(form *medicalDeviceFormResponse, accountID, appID string, requirement *complianceRequirement, declared bool, selected []string) (map[string]any, bool, error) {
	formData, err := medicalDeviceFormObject(form)
	if err != nil {
		return nil, false, err
	}
	medicalData, existingDeclaration, err := medicalDeviceFormMedicalData(form, formData)
	if err != nil {
		return nil, false, err
	}

	body := cloneMedicalDeviceMap(formData)
	delete(body, "medicalDeviceData")
	if declared {
		medicalData["declaration"] = medicalDeviceDeclarationYes
	} else {
		medicalData["declaration"] = medicalDeviceDeclarationNo
	}

	if !existingDeclaration {
		body["countriesOrRegions"] = append([]string(nil), selected...)
	} else {
		registrationInfo, err := buildMedicalDeviceRegistrationInfo(medicalData, declared, selected)
		if err != nil {
			return nil, false, err
		}
		medicalData["registrationInfo"] = registrationInfo
		if _, ok := body["countriesOrRegions"]; !ok {
			if regions := form.countriesOrRegions(); len(regions) > 0 {
				body["countriesOrRegions"] = regions
			}
		}
	}

	fillMedicalDeviceFormIdentity(body, accountID, appID, requirement)
	body["medicalDeviceData"] = medicalData
	return body, existingDeclaration, nil
}

func fillMedicalDeviceFormIdentity(body map[string]any, accountID, appID string, requirement *complianceRequirement) {
	// Older form responses omitted these identity fields, while the captured
	// UI payload includes them when present. Add them only as a compatibility
	// fallback so opaque fields from form.data remain unchanged.
	if _, ok := body["accountId"]; !ok {
		body["accountId"] = strings.TrimSpace(accountID)
	}
	if _, ok := body["contentId"]; !ok {
		body["contentId"] = strings.TrimSpace(appID)
	}
	if _, ok := body["requirementId"]; !ok {
		body["requirementId"] = requirement.ID
	}
	if _, ok := body["requirementName"]; !ok {
		body["requirementName"] = requirement.Name
	}
}

// SetMedicalDeviceDeclaration sets the regulated medical device declaration.
// The compatibility wrapper retains the existing method shape and defaults an
// affirmative answer to Apple's captured EEA/GBR/USA region set.
func (c *Client) SetMedicalDeviceDeclaration(ctx context.Context, accountID, appID string, declared bool) (*MedicalDeviceDeclarationResult, error) {
	return c.SetMedicalDeviceDeclarationWithOptions(ctx, accountID, appID, declared, MedicalDeviceDeclarationOptions{})
}

// SetMedicalDeviceDeclarationWithOptions sets the app-level medical-device
// answer using Apple's captured form contract. It does not attempt the
// region-specific registration, support-information, or contact-information
// subform; those fields must already be present and are preserved.
func (c *Client) SetMedicalDeviceDeclarationWithOptions(ctx context.Context, accountID, appID string, declared bool, options MedicalDeviceDeclarationOptions) (*MedicalDeviceDeclarationResult, error) {
	var selected []string
	var err error
	if declared || len(options.CountriesOrRegions) > 0 {
		selected, err = normalizeMedicalDeviceRegions(options.CountriesOrRegions)
		if err != nil {
			return nil, err
		}
	}

	requirement, form, err := c.medicalDeviceRequirementAndForm(ctx, accountID, appID)
	if err != nil {
		return nil, err
	}

	if !declared && form.declaration() == medicalDeviceDeclarationNo && requirement.Status == medicalDeviceCollectedStatus {
		return &MedicalDeviceDeclarationResult{
			AppID:              strings.TrimSpace(appID),
			RequirementID:      requirement.ID,
			RequirementName:    requirement.Name,
			Status:             requirement.Status,
			FormID:             requirement.FormID,
			Declared:           false,
			Changed:            false,
			CountriesOrRegions: form.countriesOrRegions(),
		}, nil
	}
	if !declared && form.declaration() == "" && len(options.CountriesOrRegions) == 0 {
		selected, err = medicalDeviceRegionsFromConstraints(form.Constraints)
		if err != nil {
			return nil, err
		}
	}
	if declared && form.declaration() == medicalDeviceDeclarationYes {
		persistedRegions, err := medicalDevicePersistedAffirmativeRegions(form)
		if err != nil {
			return nil, fmt.Errorf("medical device declaration region verification failed: %w", err)
		}
		if slices.Equal(persistedRegions, selected) {
			return &MedicalDeviceDeclarationResult{
				AppID:              strings.TrimSpace(appID),
				RequirementID:      requirement.ID,
				RequirementName:    requirement.Name,
				Status:             requirement.Status,
				FormID:             requirement.FormID,
				Declared:           true,
				Changed:            false,
				CountriesOrRegions: persistedRegions,
			}, nil
		}
	}

	requestBody, existingDeclaration, err := medicalDeviceDeclarationRequestBody(form, accountID, appID, requirement, declared, selected)
	if err != nil {
		return nil, err
	}
	path := "/ppm/complianceform/v1/accounts/" + url.PathEscape(strings.TrimSpace(accountID)) +
		"/contents/" + url.PathEscape(strings.TrimSpace(appID)) +
		"/requirements/" + url.PathEscape(requirement.ID) +
		"/forms"
	method := http.MethodPost
	if existingDeclaration {
		method = http.MethodPut
	}
	if _, err := c.doAppComplianceRequest(ctx, appID, method, path, requestBody); err != nil {
		return nil, err
	}

	updatedRequirement, updatedForm, err := c.medicalDeviceRequirementAndForm(ctx, accountID, appID)
	if err != nil {
		return nil, fmt.Errorf("medical device declaration verification failed: %w", err)
	}
	wantDeclaration := medicalDeviceDeclarationNo
	if declared {
		wantDeclaration = "yes"
	}
	if updatedForm.declaration() != wantDeclaration {
		return nil, fmt.Errorf("medical device declaration verification failed: Apple returned %q, want %q", updatedForm.declaration(), wantDeclaration)
	}
	if !declared && updatedRequirement.Status != medicalDeviceCollectedStatus {
		return nil, fmt.Errorf("medical device declaration verification failed: Apple returned status %q, want %q", updatedRequirement.Status, medicalDeviceCollectedStatus)
	}
	countriesOrRegions := updatedForm.countriesOrRegions()
	if declared {
		persistedRegions, err := medicalDevicePersistedAffirmativeRegions(updatedForm)
		if err != nil {
			return nil, fmt.Errorf("medical device declaration verification failed: %w", err)
		}
		if !slices.Equal(persistedRegions, selected) {
			return nil, fmt.Errorf("medical device declaration verification failed: persisted regions %v, want %v", persistedRegions, selected)
		}
		countriesOrRegions = persistedRegions
	} else if len(countriesOrRegions) == 0 {
		countriesOrRegions = append([]string(nil), selected...)
	}

	return &MedicalDeviceDeclarationResult{
		AppID:              strings.TrimSpace(appID),
		RequirementID:      updatedRequirement.ID,
		RequirementName:    updatedRequirement.Name,
		Status:             updatedRequirement.Status,
		FormID:             updatedRequirement.FormID,
		Declared:           declared,
		Changed:            true,
		CountriesOrRegions: countriesOrRegions,
	}, nil
}
