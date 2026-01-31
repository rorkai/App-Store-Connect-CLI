package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// InAppPurchaseDeleteResult represents CLI output for IAP deletions.
type InAppPurchaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func printInAppPurchasesTable(resp *InAppPurchasesV2Response) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tProduct ID\tType\tState")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.ProductID,
			item.Attributes.InAppPurchaseType,
			item.Attributes.State,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tReference Name\tProduct ID\tType\tState")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.ProductID,
			item.Attributes.InAppPurchaseType,
			item.Attributes.State,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLocale\tName\tDescription")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.Description),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFile Name\tFile Size\tState")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
			item.ID,
			item.Attributes.FileName,
			item.Attributes.FileSize,
			item.Attributes.State,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCustomer Price\tProceeds")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			item.ID,
			item.Attributes.CustomerPrice,
			item.Attributes.Proceeds,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTerritory\tPrice Point\tStart Date\tEnd Date\tManual")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := inAppPurchasePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%t\n",
			item.ID,
			territoryID,
			pricePointID,
			item.Attributes.StartDate,
			item.Attributes.EndDate,
			item.Attributes.Manual,
		)
	}
	return w.Flush()
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

func printInAppPurchaseOfferCodesTable(resp *InAppPurchaseOfferCodesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tActive\tProd Codes\tSandbox Codes")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%t\t%d\t%d\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.Active,
			item.Attributes.ProductionCodeCount,
			item.Attributes.SandboxCodeCount,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCustom Code\tCodes\tExpires\tCreated\tActive")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%t\n",
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.CustomCode),
			attrs.NumberOfCodes,
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			attrs.Active,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCodes\tExpires\tCreated\tActive\tEnvironment")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%t\t%s\n",
			sanitizeTerminal(item.ID),
			attrs.NumberOfCodes,
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			attrs.Active,
			sanitizeTerminal(attrs.Environment),
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tAvailable In New Territories")
	fmt.Fprintf(w, "%s\t%t\n", resp.Data.ID, resp.Data.Attributes.AvailableInNewTerritories)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFile Name\tFile Size\tLast Modified\tURL")
	fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		resp.Data.Attributes.FileSize,
		resp.Data.Attributes.LastModifiedDate,
		resp.Data.Attributes.URL,
	)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID")
	fmt.Fprintf(w, "%s\n", resp.Data.ID)
	return w.Flush()
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

func printInAppPurchaseReviewScreenshotTable(resp *InAppPurchaseAppStoreReviewScreenshotResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFile Name\tFile Size\tAsset Type")
	fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		resp.Data.Attributes.FileSize,
		resp.Data.Attributes.AssetType,
	)
	return w.Flush()
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
