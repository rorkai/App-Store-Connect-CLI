package asc

import (
	"fmt"
	"os"
	"strings"
)

type appEncryptionDeclarationField struct {
	Name  string
	Value string
}

type appEncryptionDeclarationDocumentField struct {
	Name  string
	Value string
}

func printAppEncryptionDeclarationsTable(resp *AppEncryptionDeclarationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppEncryptionDeclarationsMarkdown(resp *AppEncryptionDeclarationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | State | Exempt | Proprietary Crypto | Third-Party Crypto | French Store | Created | Code |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(fallbackValue(string(attrs.AppEncryptionDeclarationState))),
			escapeMarkdown(formatOptionalBool(attrs.Exempt)),
			escapeMarkdown(formatOptionalBool(attrs.ContainsProprietaryCryptography)),
			escapeMarkdown(formatOptionalBool(attrs.ContainsThirdPartyCryptography)),
			escapeMarkdown(formatOptionalBool(attrs.AvailableOnFrenchStore)),
			escapeMarkdown(fallbackValue(attrs.CreatedDate)),
			escapeMarkdown(fallbackValue(attrs.CodeValue)),
		)
	}
	return nil
}

func printAppEncryptionDeclarationTable(resp *AppEncryptionDeclarationResponse) error {
	fields := appEncryptionDeclarationFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppEncryptionDeclarationMarkdown(resp *AppEncryptionDeclarationResponse) error {
	fields := appEncryptionDeclarationFields(resp)
	fmt.Fprintln(os.Stdout, "| Field | Value |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, field := range fields {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n", escapeMarkdown(field.Name), escapeMarkdown(field.Value))
	}
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

func printAppEncryptionDeclarationDocumentTable(resp *AppEncryptionDeclarationDocumentResponse) error {
	fields := appEncryptionDeclarationDocumentFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppEncryptionDeclarationDocumentMarkdown(resp *AppEncryptionDeclarationDocumentResponse) error {
	fields := appEncryptionDeclarationDocumentFields(resp)
	fmt.Fprintln(os.Stdout, "| Field | Value |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, field := range fields {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n", escapeMarkdown(field.Name), escapeMarkdown(field.Value))
	}
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

func printAppEncryptionDeclarationBuildsUpdateResultTable(result *AppEncryptionDeclarationBuildsUpdateResult) error {
	headers := []string{"Declaration ID", "Build IDs", "Action"}
	rows := [][]string{{
		sanitizeTerminal(result.DeclarationID),
		sanitizeTerminal(strings.Join(result.BuildIDs, ",")),
		sanitizeTerminal(result.Action),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppEncryptionDeclarationBuildsUpdateResultMarkdown(result *AppEncryptionDeclarationBuildsUpdateResult) error {
	fmt.Fprintln(os.Stdout, "| Declaration ID | Build IDs | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.DeclarationID),
		escapeMarkdown(strings.Join(result.BuildIDs, ",")),
		escapeMarkdown(result.Action),
	)
	return nil
}
