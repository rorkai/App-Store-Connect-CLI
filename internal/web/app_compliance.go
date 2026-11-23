package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
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

// appComplianceFieldRole is what one form field asks for. The declaration
// fields are saved in this order, which is the order the form shows them.
type appComplianceFieldRole int

const (
	appComplianceFieldDeclared appComplianceFieldRole = iota + 1
	appComplianceFieldProof
	appComplianceFieldReport
	appComplianceFieldFilingType
	appComplianceFieldFilingName
	appComplianceFieldFilingNumber
	appComplianceFieldThirdParty
	appComplianceFieldAuthorization
)

// appComplianceFormFields names the fields of each form by the key in the
// field's label. Field IDs are read from the form itself for every save.
var appComplianceFormFields = map[string]map[string]appComplianceFieldRole{
	"SECURITYASF395E": {
		"doesyourap2852f":  appComplianceFieldDeclared,
		"proofofacocb097":  appComplianceFieldProof,
		"securityascf4ff1": appComplianceFieldReport,
	},
	"AIGENERATEFD1DE": {
		"doesyourap268df2": appComplianceFieldDeclared,
		"whichfilinc6deb2": appComplianceFieldFilingType,
		"llmname2":         appComplianceFieldFilingName,
		"llmfilingnumber4": appComplianceFieldFilingNumber,
		"algorithmname2":   appComplianceFieldFilingName,
		"algorithmf737252": appComplianceFieldFilingNumber,
		"areyourappd75c12": appComplianceFieldThirdParty,
		"authorizat9286a3": appComplianceFieldAuthorization,
	},
}

var appComplianceFileTypes = map[string]string{
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".pdf":  "application/pdf",
	".png":  "image/png",
}

// AppComplianceFile is a document attached to an affirmative declaration.
type AppComplianceFile struct {
	Name string
	Data []byte
}

// AppComplianceDeclaration is the answer to one App Compliance requirement.
// Requirement is SECURITY_ASSESSMENT or AI_GENERATED_CONTENT. Proof and Report
// belong to an affirmative Security Assessment; the filing and third-party
// fields belong to an affirmative AI Generated or Synthetic Content answer,
// whose FilingType is LLM or ALGORITHM.
type AppComplianceDeclaration struct {
	Requirement             string
	Declared                bool
	Proof                   *AppComplianceFile
	Report                  *AppComplianceFile
	FilingType              string
	FilingName              string
	FilingNumber            string
	ThirdPartyTechnology    bool
	ThirdPartyAuthorization *AppComplianceFile
	DryRun                  bool
}

// AppComplianceDeclarationResult reports the saved declaration, or with DryRun
// the requirement as it stands.
type AppComplianceDeclarationResult struct {
	AppID           string
	RequirementID   string
	RequirementName string
	Declared        bool
	Status          string
	FormID          string
	DryRun          bool
}

type appComplianceFileContent struct {
	FileData string `json:"fileData"`
	FileName string `json:"fileName"`
	FileType string `json:"fileType"`
}

type appComplianceField struct {
	ID          string                    `json:"id"`
	SValue      string                    `json:"sValue,omitempty"`
	FileContent *appComplianceFileContent `json:"fileContent,omitempty"`
	role        appComplianceFieldRole
}

type appComplianceConstraint struct {
	Presentation struct {
		Label   string `json:"label"`
		Options []struct {
			ValueID string `json:"valueId"`
		} `json:"options"`
	} `json:"presentation"`
	Validation struct {
		Editable         bool     `json:"editable"`
		AllowedFileTypes []string `json:"allowedFileTypes"`
	} `json:"validation"`
}

func yesNo(value bool) string {
	if value {
		return "YES"
	}
	return "NO"
}

// field answers one form field, or refuses a field the declaration cannot
// answer so that nothing is saved.
func (d AppComplianceDeclaration) field(formName, id string, constraint appComplianceConstraint) (appComplianceField, error) {
	_, key, _ := strings.Cut(constraint.Presentation.Label, ".field.")
	key, _, _ = strings.Cut(key, ".")
	role, ok := appComplianceFormFields[formName][key]
	if !ok {
		return appComplianceField{}, fmt.Errorf("app compliance form has unrecognized field %q; answer this requirement in App Store Connect", key)
	}
	if !constraint.Validation.Editable {
		return appComplianceField{}, fmt.Errorf("app compliance form does not allow editing this declaration")
	}

	field := appComplianceField{ID: id, role: role}
	var file *AppComplianceFile
	switch role {
	case appComplianceFieldDeclared:
		field.SValue = yesNo(d.Declared)
	case appComplianceFieldFilingType:
		field.SValue = map[string]string{"LLM": "LARGE", "ALGORITHM": "ALGOR"}[d.FilingType]
	case appComplianceFieldFilingName:
		field.SValue = d.FilingName
	case appComplianceFieldFilingNumber:
		field.SValue = d.FilingNumber
	case appComplianceFieldThirdParty:
		field.SValue = yesNo(d.ThirdPartyTechnology)
	case appComplianceFieldProof:
		file = d.Proof
	case appComplianceFieldReport:
		file = d.Report
	case appComplianceFieldAuthorization:
		file = d.ThirdPartyAuthorization
	}

	if file != nil {
		fileType, err := appComplianceFileType(file, constraint.Validation.AllowedFileTypes)
		if err != nil {
			return appComplianceField{}, err
		}
		field.FileContent = &appComplianceFileContent{
			FileData: base64.StdEncoding.EncodeToString(file.Data),
			FileName: file.Name,
			FileType: fileType,
		}
		return field, nil
	}
	if field.SValue == "" {
		return appComplianceField{}, fmt.Errorf("app compliance form asks for %q, which this declaration does not provide", key)
	}
	offered := len(constraint.Presentation.Options) == 0
	for _, option := range constraint.Presentation.Options {
		offered = offered || option.ValueID == field.SValue
	}
	if !offered {
		return appComplianceField{}, fmt.Errorf("app compliance form does not offer %q for %q", field.SValue, key)
	}
	return field, nil
}

// appComplianceFileType returns the content type of an evidence file, or
// refuses one whose extension the form does not accept. With no list from the
// form, every type this command knows is accepted.
func appComplianceFileType(file *AppComplianceFile, allowed []string) (string, error) {
	extension := strings.ToLower(path.Ext(file.Name))
	fileType, known := appComplianceFileTypes[extension]
	if len(allowed) == 0 {
		allowed = slices.Sorted(maps.Keys(appComplianceFileTypes))
	}
	if !known || !slices.Contains(allowed, extension) {
		return "", fmt.Errorf("%q is not an accepted file type (%s)", file.Name, strings.Join(allowed, ", "))
	}
	return fileType, nil
}

// SetAppComplianceDeclaration answers one App Compliance requirement. It
// fills the form the way the website does: each answer is posted as a draft,
// the form replies with the fields that answer opens, and the declaration is
// saved once the form asks for nothing more. A form it cannot fill completely
// is not saved.
//
// A dry run only reads the requirement and checks the evidence file types
// locally. Every form step posts the draft answer and the last one carries the
// evidence files, so a dry run must not walk the form. For the same reason the
// form steps are writes to read-only mode, which refuses them.
func (c *Client) SetAppComplianceDeclaration(ctx context.Context, accountID, appID string, declaration AppComplianceDeclaration) (*AppComplianceDeclarationResult, error) {
	requirements, err := c.ListAppComplianceRequirements(ctx, accountID, appID)
	if err != nil {
		return nil, err
	}
	accountID = strings.TrimSpace(accountID)
	appID = strings.TrimSpace(appID)
	index := slices.IndexFunc(requirements, func(requirement AppComplianceRequirement) bool {
		return requirement.RequirementName == declaration.Requirement
	})
	if index < 0 {
		return nil, fmt.Errorf("app compliance requirement %q was not found for app %q", declaration.Requirement, appID)
	}
	requirement := requirements[index]
	if requirement.Status == "COLLECTED_VERIFY_PENDING" {
		return nil, fmt.Errorf("app compliance requirement %q is pending verification and cannot be changed", declaration.Requirement)
	}
	result := &AppComplianceDeclarationResult{
		AppID:           appID,
		RequirementID:   requirement.RequirementID,
		RequirementName: requirement.RequirementName,
		Declared:        declaration.Declared,
		Status:          requirement.Status,
		FormID:          requirement.FormID,
		DryRun:          declaration.DryRun,
	}
	if declaration.DryRun {
		for _, file := range []*AppComplianceFile{declaration.Proof, declaration.Report, declaration.ThirdPartyAuthorization} {
			if file == nil {
				continue
			}
			if _, err := appComplianceFileType(file, nil); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	// Evidence travels base64 in the last form-walk request and again in the
	// save, so a caller that bounds the whole upload governs those requests
	// instead of the per-request client timeout.
	httpClient := c.httpClient
	if _, bounded := ctx.Deadline(); bounded && httpClient != nil &&
		(declaration.Proof != nil || declaration.Report != nil || declaration.ThirdPartyAuthorization != nil) {
		uploadClient := *httpClient
		uploadClient.Timeout = 0
		httpClient = &uploadClient
	}

	entityPath := "/ppm/compliancerequirements/v1/accounts/" + url.PathEscape(accountID) + "/entities/" + url.PathEscape(appID)
	fields := []appComplianceField{}
	answered := map[string]bool{}
	for opened := true; opened; {
		if len(fields) > len(appComplianceFormFields[requirement.FormName]) {
			return nil, fmt.Errorf("app compliance form for %q keeps asking for more fields", declaration.Requirement)
		}
		responseBody, err := c.doAppComplianceRequestWithHTTPClient(httpClient, ctx, appID, http.MethodPost,
			entityPath+"/requirements/"+url.PathEscape(requirement.RequirementID)+"/metadata",
			map[string]any{
				"accountId":     accountID,
				"entityId":      appID,
				"requirementId": requirement.RequirementID,
				"form":          map[string]any{"fields": fields},
			})
		if err != nil {
			return nil, err
		}
		var metadata struct {
			Constraints map[string]appComplianceConstraint `json:"constraints"`
		}
		if err := json.Unmarshal(responseBody, &metadata); err != nil {
			return nil, fmt.Errorf("failed to parse app compliance form response: %w", err)
		}

		opened = false
		for id, constraint := range metadata.Constraints {
			if answered[id] {
				continue
			}
			field, err := declaration.field(requirement.FormName, id, constraint)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field)
			answered[id] = true
			opened = true
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("app compliance form for %q has no fields to answer", declaration.Requirement)
	}
	slices.SortStableFunc(fields, func(a, b appComplianceField) int { return int(a.role) - int(b.role) })
	// A branch that never opened would save an answer without its evidence.
	for role, given := range map[appComplianceFieldRole]bool{
		appComplianceFieldProof:         declaration.Proof != nil,
		appComplianceFieldReport:        declaration.Report != nil,
		appComplianceFieldFilingType:    declaration.FilingType != "",
		appComplianceFieldFilingName:    declaration.FilingName != "",
		appComplianceFieldFilingNumber:  declaration.FilingNumber != "",
		appComplianceFieldAuthorization: declaration.ThirdPartyAuthorization != nil,
	} {
		if given && !slices.ContainsFunc(fields, func(field appComplianceField) bool { return field.role == role }) {
			return nil, fmt.Errorf("app compliance form for %q did not ask for all of the evidence in this answer; nothing was saved", declaration.Requirement)
		}
	}

	request := struct {
		AccountID     string               `json:"accountId"`
		EntityID      string               `json:"entityId"`
		RequirementID string               `json:"requirementId"`
		FormID        string               `json:"formId,omitempty"`
		Fields        []appComplianceField `json:"fields"`
	}{accountID, appID, requirement.RequirementID, requirement.FormID, fields}
	// A first answer creates the declaration; a later one replaces it.
	var responseBody []byte
	if requirement.FormID == "" {
		responseBody, err = c.doAppComplianceRequestWithHTTPClient(httpClient, ctx, appID, http.MethodPost, entityPath+"/forms", request)
	} else {
		responseBody, err = c.doAppComplianceRequestWithHTTPClient(httpClient, ctx, appID, http.MethodPut, entityPath+"/forms/"+url.PathEscape(requirement.FormID), request)
	}
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) || errors.Is(err, readonly.ErrRefused) {
			return nil, err
		}
		return nil, fmt.Errorf("app compliance declaration save outcome is unknown; check asc web apps compliance list --app %s before retrying: %w", appID, err)
	}
	var saved struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &saved); err != nil {
		return nil, fmt.Errorf("app compliance declaration was sent, but the response could not be read: %w", err)
	}
	result.Status, result.FormID = saved.Data.Status, saved.Data.ID
	return result, nil
}
