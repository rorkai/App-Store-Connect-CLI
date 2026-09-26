package asc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SubscriptionGroupDeleteResult represents CLI output for group deletions.
type SubscriptionGroupDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// SubscriptionDeleteResult represents CLI output for subscription deletions.
type SubscriptionDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// SubscriptionPriceDeleteResult represents CLI output for subscription price deletions.
type SubscriptionPriceDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// SubscriptionIntroductoryOfferCreateSummary describes a single- or
// all-territories introductory-offer create operation.
type SubscriptionIntroductoryOfferCreateSummary struct {
	SubscriptionID  string                                              `json:"subscriptionId"`
	AvailabilityID  string                                              `json:"availabilityId,omitempty"`
	Territory       string                                              `json:"territory,omitempty"`
	AllTerritories  bool                                                `json:"allTerritories"`
	DryRun          bool                                                `json:"dryRun"`
	ContinueOnError bool                                                `json:"continueOnError"`
	Total           int                                                 `json:"total"`
	Created         int                                                 `json:"created"`
	Skipped         int                                                 `json:"skipped"`
	Failed          int                                                 `json:"failed"`
	Skips           []SubscriptionIntroductoryOfferCreateSummarySkip    `json:"skips,omitempty"`
	Failures        []SubscriptionIntroductoryOfferCreateSummaryFailure `json:"failures,omitempty"`
}

// SubscriptionIntroductoryOfferCreateSummarySkip records an existing offer
// that was not recreated in an all-territories operation.
type SubscriptionIntroductoryOfferCreateSummarySkip struct {
	Territory string `json:"territory"`
	Reason    string `json:"reason"`
}

// SubscriptionIntroductoryOfferCreateSummaryFailure records a territory that
// could not be processed.
type SubscriptionIntroductoryOfferCreateSummaryFailure struct {
	Territory string `json:"territory"`
	Error     string `json:"error"`
}

func subscriptionGroupsRows(resp *SubscriptionGroupsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
		})
	}
	return headers, rows
}

func subscriptionGroupVersionsRows(resp *SubscriptionGroupVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			item.Attributes.State,
		})
	}
	return headers, rows
}

func subscriptionGroupLocalizationsV2Rows(resp *SubscriptionGroupLocalizationsV2Response) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Custom App Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.CustomAppName),
		})
	}
	return headers, rows
}

func subscriptionsRows(resp *SubscriptionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Product ID", "Period", "State", "Markets", "Multi-seat Status"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		markets := ""
		if item.Attributes.MarketSettings != nil {
			markets = strings.Join(*item.Attributes.MarketSettings, ", ")
		}
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.ProductID,
			item.Attributes.SubscriptionPeriod,
			item.Attributes.State,
			markets,
			formatOptionalString(item.Attributes.MultiSeatStatus),
		})
	}
	return headers, rows
}

func subscriptionVersionsRows(resp *SubscriptionVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, fmt.Sprintf("%d", item.Attributes.Version), string(item.Attributes.State)})
	}
	return headers, rows
}

func subscriptionLocalizationsV2Rows(resp *SubscriptionLocalizationsV2Response) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.Description),
		})
	}
	return headers, rows
}

func subscriptionImagesV2Rows(resp *SubscriptionImagesV2Response) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil && item.Attributes.AssetDeliveryState.State != nil {
			state = *item.Attributes.AssetDeliveryState.State
		}
		rows = append(rows, []string{item.ID, item.Attributes.FileName, fmt.Sprintf("%d", item.Attributes.FileSize), state})
	}
	return headers, rows
}

func subscriptionPriceRows(resp *SubscriptionPriceResponse) ([]string, [][]string) {
	headers := []string{"ID", "Start Date", "Preserved"}
	preserved := ""
	if value, ok := resp.Data.Attributes.PreservedValue(); ok {
		preserved = fmt.Sprintf("%t", value)
	}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.StartDate,
		preserved,
	}}
	return headers, rows
}

func subscriptionPricesRows(resp *SubscriptionPricesResponse) ([]string, [][]string, error) {
	included, err := subscriptionPriceIncludedAttributes(resp.Included)
	if err != nil {
		return nil, nil, err
	}
	showCurrency := includedHasAttribute(included, ResourceTypeTerritories, "currency")
	showCustomerPrice := includedHasAttribute(included, ResourceTypeSubscriptionPricePoints, "customerPrice")
	showProceeds := includedHasAttribute(included, ResourceTypeSubscriptionPricePoints, "proceeds")
	showProceedsYear2 := includedHasAttribute(included, ResourceTypeSubscriptionPricePoints, "proceedsYear2")
	showPricePointTerritory := false
	showEqualizations := false
	showAdjustedEqualizations := false
	pricePointRelationships := make([]subscriptionPricePointIncludedRelationships, len(resp.Data))

	for index, item := range resp.Data {
		_, pricePointID, err := subscriptionPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		pricePointRelationships[index].Territory, pricePointRelationships[index].HasTerritory = includedPricePointRelationshipValue(included, pricePointID, "territory")
		pricePointRelationships[index].Equalizations, pricePointRelationships[index].HasEqualizations = includedPricePointRelationshipValue(included, pricePointID, "equalizations")
		pricePointRelationships[index].AdjustedEqualizations, pricePointRelationships[index].HasAdjustedEqualizations = includedPricePointRelationshipValue(included, pricePointID, "adjustedEqualizations")
		showPricePointTerritory = pricePointRelationships[index].HasTerritory || showPricePointTerritory
		showEqualizations = pricePointRelationships[index].HasEqualizations || showEqualizations
		showAdjustedEqualizations = pricePointRelationships[index].HasAdjustedEqualizations || showAdjustedEqualizations
	}

	headers := []string{"ID", "Territory", "Price Point", "Plan Type", "Start Date", "Preserved"}
	if showCurrency {
		headers = append(headers, "Currency")
	}
	if showCustomerPrice {
		headers = append(headers, "Customer Price")
	}
	if showProceeds {
		headers = append(headers, "Proceeds")
	}
	if showProceedsYear2 {
		headers = append(headers, "Proceeds Y2")
	}
	if showPricePointTerritory {
		headers = append(headers, "Price Point Territory ID")
	}
	if showEqualizations {
		headers = append(headers, "Equalizations URL")
	}
	if showAdjustedEqualizations {
		headers = append(headers, "Adjusted Equalizations URL")
	}

	rows := make([][]string, 0, len(resp.Data))
	for index, item := range resp.Data {
		territoryID, pricePointID, err := subscriptionPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		preserved := ""
		if value, ok := item.Attributes.PreservedValue(); ok {
			preserved = fmt.Sprintf("%t", value)
		}
		row := []string{
			item.ID,
			territoryID,
			pricePointID,
			string(item.Attributes.PlanType),
			item.Attributes.StartDate,
			preserved,
		}
		if showCurrency {
			row = append(row, includedAttributeValue(included, ResourceTypeTerritories, territoryID, "currency"))
		}
		if showCustomerPrice {
			row = append(row, includedAttributeValue(included, ResourceTypeSubscriptionPricePoints, pricePointID, "customerPrice"))
		}
		if showProceeds {
			row = append(row, includedAttributeValue(included, ResourceTypeSubscriptionPricePoints, pricePointID, "proceeds"))
		}
		if showProceedsYear2 {
			row = append(row, includedAttributeValue(included, ResourceTypeSubscriptionPricePoints, pricePointID, "proceedsYear2"))
		}
		if showPricePointTerritory {
			row = append(row, pricePointRelationships[index].Territory)
		}
		if showEqualizations {
			row = append(row, pricePointRelationships[index].Equalizations)
		}
		if showAdjustedEqualizations {
			row = append(row, pricePointRelationships[index].AdjustedEqualizations)
		}
		rows = append(rows, row)
	}
	return headers, rows, nil
}

func subscriptionAvailabilityRows(resp *SubscriptionAvailabilityResponse) ([]string, [][]string) {
	headers := []string{"ID", "Available In New Territories"}
	rows := [][]string{{
		resp.Data.ID,
		fmt.Sprintf("%t", resp.Data.Attributes.AvailableInNewTerritories),
	}}
	return headers, rows
}

func subscriptionGracePeriodRows(resp *SubscriptionGracePeriodResponse) ([]string, [][]string) {
	headers := []string{"ID", "Opt In", "Sandbox Opt In", "Duration", "Renewal Type"}
	rows := [][]string{{
		resp.Data.ID,
		fmt.Sprintf("%t", resp.Data.Attributes.OptIn),
		fmt.Sprintf("%t", resp.Data.Attributes.SandboxOptIn),
		resp.Data.Attributes.Duration,
		resp.Data.Attributes.RenewalType,
	}}
	return headers, rows
}

func subscriptionGroupDeleteResultRows(result *SubscriptionGroupDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func subscriptionDeleteResultRows(result *SubscriptionDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func subscriptionPriceDeleteResultRows(result *SubscriptionPriceDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func subscriptionIntroductoryOfferCreateSummaryRows(summary *SubscriptionIntroductoryOfferCreateSummary) ([]string, [][]string) {
	headers := []string{"Subscription ID"}
	row := []string{summary.SubscriptionID}
	if summary.AvailabilityID != "" {
		headers = append(headers, "Availability ID")
		row = append(row, summary.AvailabilityID)
	}
	if summary.Territory != "" {
		headers = append(headers, "Territory")
		row = append(row, summary.Territory)
	}
	headers = append(headers, "Dry Run", "Total", "Created", "Skipped", "Failed")
	row = append(
		row,
		fmt.Sprintf("%t", summary.DryRun),
		fmt.Sprintf("%d", summary.Total),
		fmt.Sprintf("%d", summary.Created),
		fmt.Sprintf("%d", summary.Skipped),
		fmt.Sprintf("%d", summary.Failed),
	)
	return headers, [][]string{row}
}

func subscriptionIntroductoryOfferCreateSummarySkipRows(summary *SubscriptionIntroductoryOfferCreateSummary) ([]string, [][]string) {
	rows := make([][]string, 0, len(summary.Skips))
	for _, skip := range summary.Skips {
		rows = append(rows, []string{skip.Territory, skip.Reason})
	}
	return []string{"Skipped Territory", "Reason"}, rows
}

func subscriptionIntroductoryOfferCreateSummaryFailureRows(summary *SubscriptionIntroductoryOfferCreateSummary) ([]string, [][]string) {
	rows := make([][]string, 0, len(summary.Failures))
	for _, failure := range summary.Failures {
		rows = append(rows, []string{failure.Territory, failure.Error})
	}
	return []string{"Failed Territory", "Error"}, rows
}

func subscriptionPriceRelationshipIDs(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var relationships SubscriptionPriceRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", "", fmt.Errorf("decode subscription price relationships: %w", err)
	}
	territoryID := ""
	pricePointID := ""
	if relationships.Territory != nil {
		territoryID = relationships.Territory.Data.ID
	}
	if relationships.SubscriptionPricePoint != nil {
		pricePointID = relationships.SubscriptionPricePoint.Data.ID
	}
	return territoryID, pricePointID, nil
}

// subscriptionPriceIncludedAttributes indexes included resource attributes
// and relationships by resource type and ID so human-readable output can
// retain fields requested for included territories and price points.
type subscriptionPriceIncludedResource struct {
	Attributes    map[string]json.RawMessage
	Relationships json.RawMessage
}

type subscriptionPriceIncludedAttributesMap map[ResourceType]map[string]subscriptionPriceIncludedResource

type subscriptionPricePointIncludedRelationships struct {
	Territory                string
	Equalizations            string
	AdjustedEqualizations    string
	HasTerritory             bool
	HasEqualizations         bool
	HasAdjustedEqualizations bool
}

func subscriptionPriceIncludedAttributes(raw json.RawMessage) (subscriptionPriceIncludedAttributesMap, error) {
	resources := make(subscriptionPriceIncludedAttributesMap)
	if len(raw) == 0 {
		return resources, nil
	}

	var included []struct {
		Type          ResourceType               `json:"type"`
		ID            string                     `json:"id"`
		Attributes    map[string]json.RawMessage `json:"attributes"`
		Relationships json.RawMessage            `json:"relationships"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return nil, fmt.Errorf("decode subscription price included resources: %w", err)
	}
	for _, resource := range included {
		if _, ok := resources[resource.Type]; !ok {
			resources[resource.Type] = make(map[string]subscriptionPriceIncludedResource)
		}
		resources[resource.Type][resource.ID] = subscriptionPriceIncludedResource{
			Attributes:    resource.Attributes,
			Relationships: resource.Relationships,
		}
	}
	return resources, nil
}

func includedHasAttribute(included subscriptionPriceIncludedAttributesMap, resourceType ResourceType, name string) bool {
	for _, resource := range included[resourceType] {
		if _, ok := resource.Attributes[name]; ok {
			return true
		}
	}
	return false
}

func includedAttributeValue(included subscriptionPriceIncludedAttributesMap, resourceType ResourceType, id, name string) string {
	resource := included[resourceType][id]
	raw, ok := resource.Attributes[name]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func includedPricePointRelationshipValue(included subscriptionPriceIncludedAttributesMap, id, name string) (string, bool) {
	resource, ok := included[ResourceTypeSubscriptionPricePoints][id]
	if !ok || len(resource.Relationships) == 0 {
		return "", false
	}

	var relationships map[string]json.RawMessage
	if err := json.Unmarshal(resource.Relationships, &relationships); err != nil {
		return "", false
	}
	raw, ok := relationships[name]
	if !ok {
		return "", false
	}

	switch name {
	case "territory":
		var relationship Relationship
		if err := json.Unmarshal(raw, &relationship); err != nil {
			return "", true
		}
		return relationship.Data.ID, true
	case "equalizations", "adjustedEqualizations":
		var relationship SubscriptionPricePointLinkRelationship
		if err := json.Unmarshal(raw, &relationship); err != nil {
			return "", true
		}
		if relationship.Links.Related != "" {
			return relationship.Links.Related, true
		}
		return relationship.Links.Self, true
	default:
		return "", false
	}
}
