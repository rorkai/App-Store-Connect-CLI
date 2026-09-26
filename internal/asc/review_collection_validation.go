package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type reviewCollectionResourceSpec struct {
	resourceType      ResourceType
	relationshipTypes map[string]reviewRelationshipSpec
}

type reviewRelationshipSpec struct {
	resourceType ResourceType
	many         bool
}

var reviewSubmissionCollectionResourceSpec = reviewCollectionResourceSpec{
	resourceType: ResourceTypeReviewSubmissions,
	relationshipTypes: map[string]reviewRelationshipSpec{
		"app":                      {resourceType: ResourceTypeApps},
		"items":                    {resourceType: ResourceTypeReviewSubmissionItems, many: true},
		"appStoreVersionForReview": {resourceType: ResourceTypeAppStoreVersions},
		"submittedByActor":         {resourceType: ResourceTypeActors},
		"lastUpdatedByActor":       {resourceType: ResourceTypeActors},
	},
}

var reviewSubmissionItemCollectionResourceSpec = reviewCollectionResourceSpec{
	resourceType: ResourceTypeReviewSubmissionItems,
	relationshipTypes: map[string]reviewRelationshipSpec{
		"appStoreVersion":                 {resourceType: ResourceTypeAppStoreVersions},
		"appCustomProductPageVersion":     {resourceType: ResourceTypeAppCustomProductPageVersions},
		"appEvent":                        {resourceType: ResourceTypeAppEvents},
		"appStoreVersionExperiment":       {resourceType: ResourceTypeAppStoreVersionExperiments},
		"appStoreVersionExperimentV2":     {resourceType: ResourceTypeAppStoreVersionExperiments},
		"backgroundAssetVersion":          {resourceType: ResourceTypeBackgroundAssetVersions},
		"gameCenterAchievementVersion":    {resourceType: ResourceTypeGameCenterAchievementVersions},
		"gameCenterActivityVersion":       {resourceType: ResourceTypeGameCenterActivityVersions},
		"gameCenterChallengeVersion":      {resourceType: ResourceTypeGameCenterChallengeVersions},
		"gameCenterLeaderboardSetVersion": {resourceType: ResourceTypeGameCenterLeaderboardSetVersions},
		"gameCenterLeaderboardVersion":    {resourceType: ResourceTypeGameCenterLeaderboardVersions},
		"inAppPurchaseVersion":            {resourceType: ResourceTypeInAppPurchaseVersions},
		"subscriptionVersion":             {resourceType: ResourceTypeSubscriptionVersions},
		"subscriptionGroupVersion":        {resourceType: ResourceTypeSubscriptionGroupVersions},
	},
}

// validateReviewSubmissionCollectionEnvelope validates the required JSON:API
// envelope and resource linkage fields for review-submission collection
// responses before callers use a page as evidence that it is safe to create or
// reuse a submission. The OpenAPI contract requires data and links; meta is
// optional, but when present its paging object and field types must remain valid.
func validateReviewSubmissionCollectionEnvelope(data []byte, collectionName string, spec reviewCollectionResourceSpec) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil || envelope == nil {
		return fmt.Errorf("%s response must be a JSON object", collectionName)
	}
	if err := rejectReviewSubmissionTopLevelErrors(envelope, collectionName); err != nil {
		return err
	}

	dataValue, ok := envelope["data"]
	if !ok || jsonType(dataValue) != '[' {
		return fmt.Errorf("%s response data must be an array", collectionName)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(dataValue, &items); err != nil || items == nil {
		return fmt.Errorf("%s response data must be an array", collectionName)
	}
	for index, item := range items {
		if err := validateReviewSubmissionResource(item, fmt.Sprintf("%s response data[%d]", collectionName, index), spec); err != nil {
			return err
		}
	}

	linksValue, ok := envelope["links"]
	if !ok {
		return fmt.Errorf("%s response links are required", collectionName)
	}
	links, err := requiredJSONObject(linksValue, collectionName+" response links")
	if err != nil {
		return err
	}
	if err := requiredNonEmptyString(links, "self", collectionName+" response links"); err != nil {
		return err
	}
	for _, field := range []string{"first", "next"} {
		if err := optionalString(links, field, collectionName+" response links"); err != nil {
			return err
		}
	}

	if metaValue, ok := envelope["meta"]; ok {
		if err := validateReviewSubmissionPagingMeta(metaValue, collectionName+" response meta"); err != nil {
			return err
		}
	}
	return nil
}

func rejectReviewSubmissionTopLevelErrors(envelope map[string]json.RawMessage, responseName string) error {
	if _, ok := envelope["errors"]; ok {
		return fmt.Errorf("%s response must not contain top-level errors", responseName)
	}
	return nil
}

func rejectReviewSubmissionTopLevelErrorsInDocument(data []byte, responseName string) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil || envelope == nil {
		return fmt.Errorf("%s response must be a JSON object", responseName)
	}
	return rejectReviewSubmissionTopLevelErrors(envelope, responseName)
}

func validateReviewSubmissionResource(value json.RawMessage, resourceName string, spec reviewCollectionResourceSpec) error {
	resource, err := requiredJSONObject(value, resourceName)
	if err != nil {
		return err
	}

	resourceType, err := requiredString(resource, "type", resourceName)
	if err != nil {
		return err
	}
	if resourceType != string(spec.resourceType) {
		return fmt.Errorf("%s type must be %q, got %q", resourceName, spec.resourceType, resourceType)
	}
	if _, err := requiredString(resource, "id", resourceName); err != nil {
		return err
	}

	relationshipsValue, ok := resource["relationships"]
	if !ok {
		return nil
	}
	relationships, err := requiredJSONObject(relationshipsValue, resourceName+" relationships")
	if err != nil {
		return err
	}
	for relationshipName, relationshipSpec := range spec.relationshipTypes {
		relationshipValue, ok := relationships[relationshipName]
		if !ok {
			continue
		}
		relationship, err := requiredJSONObject(relationshipValue, resourceName+" relationship "+relationshipName)
		if err != nil {
			return err
		}
		dataValue, ok := relationship["data"]
		if relationshipSpec.many {
			if !ok {
				// OpenAPI permits a to-many relationship to expose only links
				// and/or meta when linkage data was not requested.
				continue
			}
			if isJSONNull(dataValue) {
				return fmt.Errorf("%s data must be an array", resourceName+" relationship "+relationshipName)
			}
			if err := validateReviewSubmissionLinkageArray(dataValue, resourceName+" relationship "+relationshipName, relationshipSpec.resourceType); err != nil {
				return err
			}
			continue
		}
		if !ok || isJSONNull(dataValue) {
			// A relationship may be represented by links only, and ASC can use
			// data:null for an unset to-one relationship.
			continue
		}
		if err := validateReviewSubmissionLinkage(dataValue, resourceName+" relationship "+relationshipName, relationshipSpec.resourceType); err != nil {
			return err
		}
	}
	return nil
}

func validateReviewSubmissionLinkage(value json.RawMessage, fieldName string, expectedType ResourceType) error {
	linkage, err := requiredJSONObject(value, fieldName+" data")
	if err != nil {
		return err
	}
	typeValue, err := requiredString(linkage, "type", fieldName+" data")
	if err != nil {
		return err
	}
	if typeValue != string(expectedType) {
		return fmt.Errorf("%s data type must be %q, got %q", fieldName, expectedType, typeValue)
	}
	_, err = requiredString(linkage, "id", fieldName+" data")
	return err
}

func validateReviewSubmissionLinkageArray(value json.RawMessage, fieldName string, expectedType ResourceType) error {
	if jsonType(value) != '[' {
		return fmt.Errorf("%s data must be an array", fieldName)
	}
	var linkages []json.RawMessage
	if err := json.Unmarshal(value, &linkages); err != nil || linkages == nil {
		return fmt.Errorf("%s data must be an array", fieldName)
	}
	for index, linkage := range linkages {
		if err := validateReviewSubmissionLinkage(linkage, fmt.Sprintf("%s[%d]", fieldName, index), expectedType); err != nil {
			return err
		}
	}
	return nil
}

func validateReviewSubmissionPagingMeta(value json.RawMessage, fieldName string) error {
	meta, err := requiredJSONObject(value, fieldName)
	if err != nil {
		return err
	}
	pagingValue, ok := meta["paging"]
	if !ok {
		return fmt.Errorf("%s paging is required", fieldName)
	}
	paging, err := requiredJSONObject(pagingValue, fieldName+" paging")
	if err != nil {
		return err
	}
	if err := requiredInteger(paging, "limit", fieldName+" paging"); err != nil {
		return err
	}
	if _, ok := paging["total"]; ok {
		if err := integerValue(paging["total"], "total", fieldName+" paging"); err != nil {
			return err
		}
	}
	if _, ok := paging["nextCursor"]; ok {
		if err := validateStringValue(paging["nextCursor"], "nextCursor", fieldName+" paging"); err != nil {
			return err
		}
	}
	return nil
}

func requiredJSONObject(value json.RawMessage, fieldName string) (map[string]json.RawMessage, error) {
	if jsonType(value) != '{' {
		return nil, fmt.Errorf("%s must be an object", fieldName)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(value, &object); err != nil || object == nil {
		return nil, fmt.Errorf("%s must be an object", fieldName)
	}
	return object, nil
}

func requiredNonEmptyString(object map[string]json.RawMessage, field, objectName string) error {
	value, ok := object[field]
	if !ok {
		return fmt.Errorf("%s %s is required", objectName, field)
	}
	if err := validateStringValue(value, field, objectName); err != nil {
		return err
	}
	var decoded string
	_ = json.Unmarshal(value, &decoded)
	if strings.TrimSpace(decoded) == "" {
		return fmt.Errorf("%s %s must not be empty", objectName, field)
	}
	return nil
}

func requiredString(object map[string]json.RawMessage, field, objectName string) (string, error) {
	if err := requiredNonEmptyString(object, field, objectName); err != nil {
		return "", err
	}
	var decoded string
	_ = json.Unmarshal(object[field], &decoded)
	return strings.TrimSpace(decoded), nil
}

func optionalString(object map[string]json.RawMessage, field, objectName string) error {
	if value, ok := object[field]; ok {
		return validateStringValue(value, field, objectName)
	}
	return nil
}

func validateStringValue(value json.RawMessage, field, objectName string) error {
	if jsonType(value) != '"' {
		return fmt.Errorf("%s %s must be a string", objectName, field)
	}
	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("%s %s must be a string", objectName, field)
	}
	return nil
}

func requiredInteger(object map[string]json.RawMessage, field, objectName string) error {
	value, ok := object[field]
	if !ok {
		return fmt.Errorf("%s %s is required", objectName, field)
	}
	return integerValue(value, field, objectName)
}

func integerValue(value json.RawMessage, field, objectName string) error {
	if jsonType(value) != '-' && (jsonType(value) < '0' || jsonType(value) > '9') {
		return fmt.Errorf("%s %s must be an integer", objectName, field)
	}
	var decoded int
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("%s %s must be an integer", objectName, field)
	}
	return nil
}

func jsonType(value json.RawMessage) byte {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}
	return value[0]
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}
