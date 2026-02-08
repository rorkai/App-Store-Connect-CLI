package asc

import "fmt"

type routingAppCoverageField struct {
	Name  string
	Value string
}

func routingAppCoverageRows(resp *RoutingAppCoverageResponse) ([]string, [][]string) {
	fields := routingAppCoverageFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	return headers, rows
}

func routingAppCoverageFields(resp *RoutingAppCoverageResponse) []routingAppCoverageField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	return []routingAppCoverageField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "File Name", Value: fallbackValue(attrs.FileName)},
		{Name: "File Size", Value: formatAttachmentFileSize(attrs.FileSize)},
		{Name: "Source File Checksum", Value: fallbackValue(attrs.SourceFileChecksum)},
		{Name: "Delivery State", Value: formatAssetDeliveryState(attrs.AssetDeliveryState)},
	}
}

func routingAppCoverageDeleteResultRows(result *RoutingAppCoverageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
