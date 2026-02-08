package asc

import (
	"fmt"
	"os"
)

type accessibilityDeclarationField struct {
	Name  string
	Value string
}

func printAccessibilityDeclarationsTable(resp *AccessibilityDeclarationsResponse) error {
	headers := []string{"ID", "Device Family", "State", "Audio Descriptions", "Captions", "Dark Interface", "Differentiate Without Color", "Larger Text", "Reduced Motion", "Sufficient Contrast", "Voice Control", "Voiceover"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(fallbackValue(string(attrs.DeviceFamily))),
			sanitizeTerminal(fallbackValue(string(attrs.State))),
			formatOptionalBool(attrs.SupportsAudioDescriptions),
			formatOptionalBool(attrs.SupportsCaptions),
			formatOptionalBool(attrs.SupportsDarkInterface),
			formatOptionalBool(attrs.SupportsDifferentiateWithoutColorAlone),
			formatOptionalBool(attrs.SupportsLargerText),
			formatOptionalBool(attrs.SupportsReducedMotion),
			formatOptionalBool(attrs.SupportsSufficientContrast),
			formatOptionalBool(attrs.SupportsVoiceControl),
			formatOptionalBool(attrs.SupportsVoiceover),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAccessibilityDeclarationsMarkdown(resp *AccessibilityDeclarationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Device Family | State | Audio Descriptions | Captions | Dark Interface | Differentiate Without Color | Larger Text | Reduced Motion | Sufficient Contrast | Voice Control | Voiceover |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(fallbackValue(string(attrs.DeviceFamily))),
			escapeMarkdown(fallbackValue(string(attrs.State))),
			escapeMarkdown(formatOptionalBool(attrs.SupportsAudioDescriptions)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsCaptions)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsDarkInterface)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsDifferentiateWithoutColorAlone)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsLargerText)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsReducedMotion)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsSufficientContrast)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsVoiceControl)),
			escapeMarkdown(formatOptionalBool(attrs.SupportsVoiceover)),
		)
	}
	return nil
}

func printAccessibilityDeclarationTable(resp *AccessibilityDeclarationResponse) error {
	fields := accessibilityDeclarationFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	RenderTable(headers, rows)
	return nil
}

func printAccessibilityDeclarationMarkdown(resp *AccessibilityDeclarationResponse) error {
	fields := accessibilityDeclarationFields(resp)
	fmt.Fprintln(os.Stdout, "| Field | Value |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, field := range fields {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n", escapeMarkdown(field.Name), escapeMarkdown(field.Value))
	}
	return nil
}

func accessibilityDeclarationFields(resp *AccessibilityDeclarationResponse) []accessibilityDeclarationField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	return []accessibilityDeclarationField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "Device Family", Value: fallbackValue(string(attrs.DeviceFamily))},
		{Name: "State", Value: fallbackValue(string(attrs.State))},
		{Name: "Supports Audio Descriptions", Value: formatOptionalBool(attrs.SupportsAudioDescriptions)},
		{Name: "Supports Captions", Value: formatOptionalBool(attrs.SupportsCaptions)},
		{Name: "Supports Dark Interface", Value: formatOptionalBool(attrs.SupportsDarkInterface)},
		{Name: "Supports Differentiate Without Color", Value: formatOptionalBool(attrs.SupportsDifferentiateWithoutColorAlone)},
		{Name: "Supports Larger Text", Value: formatOptionalBool(attrs.SupportsLargerText)},
		{Name: "Supports Reduced Motion", Value: formatOptionalBool(attrs.SupportsReducedMotion)},
		{Name: "Supports Sufficient Contrast", Value: formatOptionalBool(attrs.SupportsSufficientContrast)},
		{Name: "Supports Voice Control", Value: formatOptionalBool(attrs.SupportsVoiceControl)},
		{Name: "Supports Voiceover", Value: formatOptionalBool(attrs.SupportsVoiceover)},
	}
}

func printAccessibilityDeclarationDeleteResultTable(result *AccessibilityDeclarationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAccessibilityDeclarationDeleteResultMarkdown(result *AccessibilityDeclarationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}
