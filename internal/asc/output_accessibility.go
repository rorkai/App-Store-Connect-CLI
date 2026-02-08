package asc

import "fmt"

type accessibilityDeclarationField struct {
	Name  string
	Value string
}

func accessibilityDeclarationsRows(resp *AccessibilityDeclarationsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAccessibilityDeclarationsTable(resp *AccessibilityDeclarationsResponse) error {
	h, r := accessibilityDeclarationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAccessibilityDeclarationsMarkdown(resp *AccessibilityDeclarationsResponse) error {
	h, r := accessibilityDeclarationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func accessibilityDeclarationRows(resp *AccessibilityDeclarationResponse) ([]string, [][]string) {
	fields := accessibilityDeclarationFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	return headers, rows
}

func printAccessibilityDeclarationTable(resp *AccessibilityDeclarationResponse) error {
	h, r := accessibilityDeclarationRows(resp)
	RenderTable(h, r)
	return nil
}

func printAccessibilityDeclarationMarkdown(resp *AccessibilityDeclarationResponse) error {
	h, r := accessibilityDeclarationRows(resp)
	RenderMarkdown(h, r)
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

func accessibilityDeclarationDeleteResultRows(result *AccessibilityDeclarationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAccessibilityDeclarationDeleteResultTable(result *AccessibilityDeclarationDeleteResult) error {
	h, r := accessibilityDeclarationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAccessibilityDeclarationDeleteResultMarkdown(result *AccessibilityDeclarationDeleteResult) error {
	h, r := accessibilityDeclarationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
