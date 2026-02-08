package asc

import "strings"

type appEncryptionDeclarationField struct {
	Name  string
	Value string
}

type appEncryptionDeclarationDocumentField struct {
	Name  string
	Value string
}

func appEncryptionDeclarationsRows(resp *AppEncryptionDeclarationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Exempt", "Proprietary Crypto", "Third-Party Crypto", "French Store", "Created", "Code"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(fallbackValue(string(attrs.AppEncryptionDeclarationState))),
			formatOptionalBool(attrs.Exempt),
			formatOptionalBool(attrs.ContainsProprietaryCryptography),
			formatOptionalBool(attrs.ContainsThirdPartyCryptography),
			formatOptionalBool(attrs.AvailableOnFrenchStore),
			sanitizeTerminal(fallbackValue(attrs.CreatedDate)),
			sanitizeTerminal(fallbackValue(attrs.CodeValue)),
		})
	}
	return headers, rows
}

func printAppEncryptionDeclarationsTable(resp *AppEncryptionDeclarationsResponse) error {
	h, r := appEncryptionDeclarationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEncryptionDeclarationsMarkdown(resp *AppEncryptionDeclarationsResponse) error {
	h, r := appEncryptionDeclarationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEncryptionDeclarationRows(resp *AppEncryptionDeclarationResponse) ([]string, [][]string) {
	fields := appEncryptionDeclarationFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	return headers, rows
}

func printAppEncryptionDeclarationTable(resp *AppEncryptionDeclarationResponse) error {
	h, r := appEncryptionDeclarationRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEncryptionDeclarationMarkdown(resp *AppEncryptionDeclarationResponse) error {
	h, r := appEncryptionDeclarationRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEncryptionDeclarationFields(resp *AppEncryptionDeclarationResponse) []appEncryptionDeclarationField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	return []appEncryptionDeclarationField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "App Description", Value: compactWhitespace(attrs.AppDescription)},
		{Name: "State", Value: fallbackValue(string(attrs.AppEncryptionDeclarationState))},
		{Name: "Uses Encryption", Value: formatOptionalBool(attrs.UsesEncryption)},
		{Name: "Exempt", Value: formatOptionalBool(attrs.Exempt)},
		{Name: "Contains Proprietary Cryptography", Value: formatOptionalBool(attrs.ContainsProprietaryCryptography)},
		{Name: "Contains Third-Party Cryptography", Value: formatOptionalBool(attrs.ContainsThirdPartyCryptography)},
		{Name: "Available On French Store", Value: formatOptionalBool(attrs.AvailableOnFrenchStore)},
		{Name: "Code Value", Value: fallbackValue(attrs.CodeValue)},
		{Name: "Created Date", Value: fallbackValue(attrs.CreatedDate)},
		{Name: "Uploaded Date", Value: fallbackValue(attrs.UploadedDate)},
		{Name: "Document Name", Value: fallbackValue(attrs.DocumentName)},
		{Name: "Document URL", Value: fallbackValue(attrs.DocumentURL)},
		{Name: "Document Type", Value: fallbackValue(attrs.DocumentType)},
		{Name: "Platform", Value: fallbackValue(string(attrs.Platform))},
	}
}

func appEncryptionDeclarationDocumentRows(resp *AppEncryptionDeclarationDocumentResponse) ([]string, [][]string) {
	fields := appEncryptionDeclarationDocumentFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	return headers, rows
}

func printAppEncryptionDeclarationDocumentTable(resp *AppEncryptionDeclarationDocumentResponse) error {
	h, r := appEncryptionDeclarationDocumentRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEncryptionDeclarationDocumentMarkdown(resp *AppEncryptionDeclarationDocumentResponse) error {
	h, r := appEncryptionDeclarationDocumentRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEncryptionDeclarationDocumentFields(resp *AppEncryptionDeclarationDocumentResponse) []appEncryptionDeclarationDocumentField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	return []appEncryptionDeclarationDocumentField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "File Name", Value: fallbackValue(attrs.FileName)},
		{Name: "File Size", Value: formatAttachmentFileSize(attrs.FileSize)},
		{Name: "Download URL", Value: fallbackValue(attrs.DownloadURL)},
		{Name: "Source File Checksum", Value: fallbackValue(attrs.SourceFileChecksum)},
		{Name: "Asset Token", Value: fallbackValue(attrs.AssetToken)},
		{Name: "Delivery State", Value: formatAssetDeliveryState(attrs.AssetDeliveryState)},
	}
}

func appEncryptionDeclarationBuildsUpdateResultRows(result *AppEncryptionDeclarationBuildsUpdateResult) ([]string, [][]string) {
	headers := []string{"Declaration ID", "Build IDs", "Action"}
	rows := [][]string{{
		sanitizeTerminal(result.DeclarationID),
		sanitizeTerminal(strings.Join(result.BuildIDs, ",")),
		sanitizeTerminal(result.Action),
	}}
	return headers, rows
}

func printAppEncryptionDeclarationBuildsUpdateResultTable(result *AppEncryptionDeclarationBuildsUpdateResult) error {
	h, r := appEncryptionDeclarationBuildsUpdateResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppEncryptionDeclarationBuildsUpdateResultMarkdown(result *AppEncryptionDeclarationBuildsUpdateResult) error {
	h, r := appEncryptionDeclarationBuildsUpdateResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
