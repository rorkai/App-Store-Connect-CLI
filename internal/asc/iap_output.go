package asc

import (
	"encoding/json"
	"fmt"
	"os"
)

// InAppPurchaseDeleteResult represents CLI output for IAP deletions.
type InAppPurchaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func printInAppPurchasesTable(resp *InAppPurchasesV2Response) error {
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchasesMarkdown(resp *InAppPurchasesV2Response) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Product ID | Type | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.ProductID),
			escapeMarkdown(item.Attributes.InAppPurchaseType),
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printLegacyInAppPurchasesTable(resp *InAppPurchasesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printLegacyInAppPurchasesMarkdown(resp *InAppPurchasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Product ID | Type | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.ProductID),
			escapeMarkdown(item.Attributes.InAppPurchaseType),
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printInAppPurchaseLocalizationsTable(resp *InAppPurchaseLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseLocalizationsMarkdown(resp *InAppPurchaseLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name | Description |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.Description),
		)
	}
	return nil
}

func printInAppPurchaseDeleteResultTable(result *InAppPurchaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseDeleteResultMarkdown(result *InAppPurchaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printInAppPurchaseImagesTable(resp *InAppPurchaseImagesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseImagesMarkdown(resp *InAppPurchaseImagesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.FileName),
			item.Attributes.FileSize,
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printInAppPurchasePricePointsTable(resp *InAppPurchasePricePointsResponse) error {
	headers := []string{"ID", "Customer Price", "Proceeds"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CustomerPrice,
			item.Attributes.Proceeds,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchasePricePointsMarkdown(resp *InAppPurchasePricePointsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Customer Price | Proceeds |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.CustomerPrice),
			escapeMarkdown(item.Attributes.Proceeds),
		)
	}
	return nil
}

func printInAppPurchasePricesTable(resp *InAppPurchasePricesResponse) error {
	headers := []string{"ID", "Territory", "Price Point", "Start Date", "End Date", "Manual"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchasePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchasePricesMarkdown(resp *InAppPurchasePricesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Territory | Price Point | Start Date | End Date | Manual |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchasePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(territoryID),
			escapeMarkdown(pricePointID),
			escapeMarkdown(item.Attributes.StartDate),
			escapeMarkdown(item.Attributes.EndDate),
			item.Attributes.Manual,
		)
	}
	return nil
}

func printInAppPurchaseOfferCodePricesTable(resp *InAppPurchaseOfferPricesResponse) error {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchaseOfferPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(territoryID),
			sanitizeTerminal(pricePointID),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseOfferCodePricesMarkdown(resp *InAppPurchaseOfferPricesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Territory | Price Point |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchaseOfferPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(territoryID),
			escapeMarkdown(pricePointID),
		)
	}
	return nil
}

func printInAppPurchaseOfferCodesTable(resp *InAppPurchaseOfferCodesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseOfferCodesMarkdown(resp *InAppPurchaseOfferCodesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Active | Prod Codes | Sandbox Codes |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %t | %d | %d |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			item.Attributes.Active,
			item.Attributes.ProductionCodeCount,
			item.Attributes.SandboxCodeCount,
		)
	}
	return nil
}

func printInAppPurchaseOfferCodeCustomCodesTable(resp *InAppPurchaseOfferCodeCustomCodesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseOfferCodeCustomCodesMarkdown(resp *InAppPurchaseOfferCodeCustomCodesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Custom Code | Codes | Expires | Created | Active |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.CustomCode),
			attrs.NumberOfCodes,
			escapeMarkdown(attrs.ExpirationDate),
			escapeMarkdown(attrs.CreatedDate),
			attrs.Active,
		)
	}
	return nil
}

func printInAppPurchaseOfferCodeOneTimeUseCodesTable(resp *InAppPurchaseOfferCodeOneTimeUseCodesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseOfferCodeOneTimeUseCodesMarkdown(resp *InAppPurchaseOfferCodeOneTimeUseCodesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Codes | Expires | Created | Active | Environment |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s | %t | %s |\n",
			escapeMarkdown(item.ID),
			attrs.NumberOfCodes,
			escapeMarkdown(attrs.ExpirationDate),
			escapeMarkdown(attrs.CreatedDate),
			attrs.Active,
			escapeMarkdown(attrs.Environment),
		)
	}
	return nil
}

func printInAppPurchaseAvailabilityTable(resp *InAppPurchaseAvailabilityResponse) error {
	headers := []string{"ID", "Available In New Territories"}
	rows := [][]string{{resp.Data.ID, fmt.Sprintf("%t", resp.Data.Attributes.AvailableInNewTerritories)}}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseAvailabilityMarkdown(resp *InAppPurchaseAvailabilityResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Available In New Territories |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(resp.Data.ID),
		resp.Data.Attributes.AvailableInNewTerritories,
	)
	return nil
}

func printInAppPurchaseContentTable(resp *InAppPurchaseContentResponse) error {
	headers := []string{"ID", "File Name", "File Size", "Last Modified", "URL"}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		resp.Data.Attributes.LastModifiedDate,
		resp.Data.Attributes.URL,
	}}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseContentMarkdown(resp *InAppPurchaseContentResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Last Modified | URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.FileName),
		resp.Data.Attributes.FileSize,
		escapeMarkdown(resp.Data.Attributes.LastModifiedDate),
		escapeMarkdown(resp.Data.Attributes.URL),
	)
	return nil
}

func printInAppPurchasePriceScheduleTable(resp *InAppPurchasePriceScheduleResponse) error {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchasePriceScheduleMarkdown(resp *InAppPurchasePriceScheduleResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(resp.Data.ID))
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

func printInAppPurchaseReviewScreenshotTable(resp *InAppPurchaseAppStoreReviewScreenshotResponse) error {
	headers := []string{"ID", "File Name", "File Size", "Asset Type"}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		resp.Data.Attributes.AssetType,
	}}
	RenderTable(headers, rows)
	return nil
}

func printInAppPurchaseReviewScreenshotMarkdown(resp *InAppPurchaseAppStoreReviewScreenshotResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Asset Type |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.FileName),
		resp.Data.Attributes.FileSize,
		escapeMarkdown(resp.Data.Attributes.AssetType),
	)
	return nil
}
