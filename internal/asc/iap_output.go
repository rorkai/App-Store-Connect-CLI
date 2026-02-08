package asc

import (
	"encoding/json"
	"fmt"
)

// InAppPurchaseDeleteResult represents CLI output for IAP deletions.
type InAppPurchaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func inAppPurchasesRows(resp *InAppPurchasesV2Response) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Product ID", "Type", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.ProductID,
			item.Attributes.InAppPurchaseType,
			item.Attributes.State,
		})
	}
	return headers, rows
}

func printInAppPurchasesTable(resp *InAppPurchasesV2Response) error {
	h, r := inAppPurchasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchasesMarkdown(resp *InAppPurchasesV2Response) error {
	h, r := inAppPurchasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func legacyInAppPurchasesRows(resp *InAppPurchasesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Product ID", "Type", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.ProductID,
			item.Attributes.InAppPurchaseType,
			item.Attributes.State,
		})
	}
	return headers, rows
}

func printLegacyInAppPurchasesTable(resp *InAppPurchasesResponse) error {
	h, r := legacyInAppPurchasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printLegacyInAppPurchasesMarkdown(resp *InAppPurchasesResponse) error {
	h, r := legacyInAppPurchasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseLocalizationsRows(resp *InAppPurchaseLocalizationsResponse) ([]string, [][]string) {
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

func printInAppPurchaseLocalizationsTable(resp *InAppPurchaseLocalizationsResponse) error {
	h, r := inAppPurchaseLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseLocalizationsMarkdown(resp *InAppPurchaseLocalizationsResponse) error {
	h, r := inAppPurchaseLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseDeleteResultRows(result *InAppPurchaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printInAppPurchaseDeleteResultTable(result *InAppPurchaseDeleteResult) error {
	h, r := inAppPurchaseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseDeleteResultMarkdown(result *InAppPurchaseDeleteResult) error {
	h, r := inAppPurchaseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseImagesRows(resp *InAppPurchaseImagesResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			item.Attributes.State,
		})
	}
	return headers, rows
}

func printInAppPurchaseImagesTable(resp *InAppPurchaseImagesResponse) error {
	h, r := inAppPurchaseImagesRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseImagesMarkdown(resp *InAppPurchaseImagesResponse) error {
	h, r := inAppPurchaseImagesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchasePricePointsRows(resp *InAppPurchasePricePointsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Customer Price", "Proceeds"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CustomerPrice,
			item.Attributes.Proceeds,
		})
	}
	return headers, rows
}

func printInAppPurchasePricePointsTable(resp *InAppPurchasePricePointsResponse) error {
	h, r := inAppPurchasePricePointsRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchasePricePointsMarkdown(resp *InAppPurchasePricePointsResponse) error {
	h, r := inAppPurchasePricePointsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchasePricesRows(resp *InAppPurchasePricesResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "Price Point", "Start Date", "End Date", "Manual"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchasePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{
			item.ID,
			territoryID,
			pricePointID,
			item.Attributes.StartDate,
			item.Attributes.EndDate,
			fmt.Sprintf("%t", item.Attributes.Manual),
		})
	}
	return headers, rows, nil
}

func printInAppPurchasePricesTable(resp *InAppPurchasePricesResponse) error {
	h, r, err := inAppPurchasePricesRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printInAppPurchasePricesMarkdown(resp *InAppPurchasePricesResponse) error {
	h, r, err := inAppPurchasePricesRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseOfferCodePricesRows(resp *InAppPurchaseOfferPricesResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchaseOfferPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(territoryID),
			sanitizeTerminal(pricePointID),
		})
	}
	return headers, rows, nil
}

func printInAppPurchaseOfferCodePricesTable(resp *InAppPurchaseOfferPricesResponse) error {
	h, r, err := inAppPurchaseOfferCodePricesRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseOfferCodePricesMarkdown(resp *InAppPurchaseOfferPricesResponse) error {
	h, r, err := inAppPurchaseOfferCodePricesRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseOfferCodesRows(resp *InAppPurchaseOfferCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Active", "Prod Codes", "Sandbox Codes"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			fmt.Sprintf("%t", item.Attributes.Active),
			fmt.Sprintf("%d", item.Attributes.ProductionCodeCount),
			fmt.Sprintf("%d", item.Attributes.SandboxCodeCount),
		})
	}
	return headers, rows
}

func printInAppPurchaseOfferCodesTable(resp *InAppPurchaseOfferCodesResponse) error {
	h, r := inAppPurchaseOfferCodesRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseOfferCodesMarkdown(resp *InAppPurchaseOfferCodesResponse) error {
	h, r := inAppPurchaseOfferCodesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseOfferCodeCustomCodesRows(resp *InAppPurchaseOfferCodeCustomCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Custom Code", "Codes", "Expires", "Created", "Active"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.CustomCode),
			fmt.Sprintf("%d", attrs.NumberOfCodes),
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			fmt.Sprintf("%t", attrs.Active),
		})
	}
	return headers, rows
}

func printInAppPurchaseOfferCodeCustomCodesTable(resp *InAppPurchaseOfferCodeCustomCodesResponse) error {
	h, r := inAppPurchaseOfferCodeCustomCodesRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseOfferCodeCustomCodesMarkdown(resp *InAppPurchaseOfferCodeCustomCodesResponse) error {
	h, r := inAppPurchaseOfferCodeCustomCodesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseOfferCodeOneTimeUseCodesRows(resp *InAppPurchaseOfferCodeOneTimeUseCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Codes", "Expires", "Created", "Active", "Environment"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			fmt.Sprintf("%d", attrs.NumberOfCodes),
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			fmt.Sprintf("%t", attrs.Active),
			sanitizeTerminal(attrs.Environment),
		})
	}
	return headers, rows
}

func printInAppPurchaseOfferCodeOneTimeUseCodesTable(resp *InAppPurchaseOfferCodeOneTimeUseCodesResponse) error {
	h, r := inAppPurchaseOfferCodeOneTimeUseCodesRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseOfferCodeOneTimeUseCodesMarkdown(resp *InAppPurchaseOfferCodeOneTimeUseCodesResponse) error {
	h, r := inAppPurchaseOfferCodeOneTimeUseCodesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseAvailabilityRows(resp *InAppPurchaseAvailabilityResponse) ([]string, [][]string) {
	headers := []string{"ID", "Available In New Territories"}
	rows := [][]string{{resp.Data.ID, fmt.Sprintf("%t", resp.Data.Attributes.AvailableInNewTerritories)}}
	return headers, rows
}

func printInAppPurchaseAvailabilityTable(resp *InAppPurchaseAvailabilityResponse) error {
	h, r := inAppPurchaseAvailabilityRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseAvailabilityMarkdown(resp *InAppPurchaseAvailabilityResponse) error {
	h, r := inAppPurchaseAvailabilityRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchaseContentRows(resp *InAppPurchaseContentResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Last Modified", "URL"}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		resp.Data.Attributes.LastModifiedDate,
		resp.Data.Attributes.URL,
	}}
	return headers, rows
}

func printInAppPurchaseContentTable(resp *InAppPurchaseContentResponse) error {
	h, r := inAppPurchaseContentRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseContentMarkdown(resp *InAppPurchaseContentResponse) error {
	h, r := inAppPurchaseContentRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchasePriceScheduleRows(resp *InAppPurchasePriceScheduleResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}

func printInAppPurchasePriceScheduleTable(resp *InAppPurchasePriceScheduleResponse) error {
	h, r := inAppPurchasePriceScheduleRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchasePriceScheduleMarkdown(resp *InAppPurchasePriceScheduleResponse) error {
	h, r := inAppPurchasePriceScheduleRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func inAppPurchasePriceRelationshipIDs(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var relationships struct {
		Territory               *Relationship `json:"territory"`
		InAppPurchasePricePoint *Relationship `json:"inAppPurchasePricePoint"`
	}
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", "", fmt.Errorf("decode in-app purchase price relationships: %w", err)
	}
	territoryID := ""
	pricePointID := ""
	if relationships.Territory != nil {
		territoryID = relationships.Territory.Data.ID
	}
	if relationships.InAppPurchasePricePoint != nil {
		pricePointID = relationships.InAppPurchasePricePoint.Data.ID
	}
	return territoryID, pricePointID, nil
}

func inAppPurchaseOfferPriceRelationshipIDs(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var relationships InAppPurchaseOfferPriceInlineRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", "", fmt.Errorf("decode in-app purchase offer price relationships: %w", err)
	}
	return relationships.Territory.Data.ID, relationships.PricePoint.Data.ID, nil
}

func inAppPurchaseReviewScreenshotRows(resp *InAppPurchaseAppStoreReviewScreenshotResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Asset Type"}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		resp.Data.Attributes.AssetType,
	}}
	return headers, rows
}

func printInAppPurchaseReviewScreenshotTable(resp *InAppPurchaseAppStoreReviewScreenshotResponse) error {
	h, r := inAppPurchaseReviewScreenshotRows(resp)
	RenderTable(h, r)
	return nil
}

func printInAppPurchaseReviewScreenshotMarkdown(resp *InAppPurchaseAppStoreReviewScreenshotResponse) error {
	h, r := inAppPurchaseReviewScreenshotRows(resp)
	RenderMarkdown(h, r)
	return nil
}
