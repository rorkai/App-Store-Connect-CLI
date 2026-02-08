package asc

import (
	"fmt"
	"os"
)

// DeviceLocalUDIDResult represents CLI output for local device UDID lookup.
type DeviceLocalUDIDResult struct {
	UDID     string `json:"udid"`
	Platform string `json:"platform"`
}

func printDeviceLocalUDIDTable(result *DeviceLocalUDIDResult) error {
	headers := []string{"UDID", "Platform"}
	rows := [][]string{{result.UDID, result.Platform}}
	RenderTable(headers, rows)
	return nil
}

func printDeviceLocalUDIDMarkdown(result *DeviceLocalUDIDResult) error {
	fmt.Fprintln(os.Stdout, "| UDID | Platform |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s |\n",
		escapeMarkdown(result.UDID),
		escapeMarkdown(result.Platform),
	)
	return nil
}

func printDevicesTable(resp *DevicesResponse) error {
	headers := []string{"ID", "Name", "UDID", "Platform", "Status", "Class", "Model", "Added"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.UDID),
			compactWhitespace(string(item.Attributes.Platform)),
			compactWhitespace(string(item.Attributes.Status)),
			compactWhitespace(string(item.Attributes.DeviceClass)),
			compactWhitespace(item.Attributes.Model),
			compactWhitespace(item.Attributes.AddedDate),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printDevicesMarkdown(resp *DevicesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | UDID | Platform | Status | Class | Model | Added |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.UDID),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(string(item.Attributes.Status)),
			escapeMarkdown(string(item.Attributes.DeviceClass)),
			escapeMarkdown(item.Attributes.Model),
			escapeMarkdown(item.Attributes.AddedDate),
		)
	}
	return nil
}
