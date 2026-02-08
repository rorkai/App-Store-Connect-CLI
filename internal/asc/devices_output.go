package asc

// DeviceLocalUDIDResult represents CLI output for local device UDID lookup.
type DeviceLocalUDIDResult struct {
	UDID     string `json:"udid"`
	Platform string `json:"platform"`
}

func deviceLocalUDIDRows(result *DeviceLocalUDIDResult) ([]string, [][]string) {
	headers := []string{"UDID", "Platform"}
	rows := [][]string{{result.UDID, result.Platform}}
	return headers, rows
}

func printDeviceLocalUDIDTable(result *DeviceLocalUDIDResult) error {
	h, r := deviceLocalUDIDRows(result)
	RenderTable(h, r)
	return nil
}

func printDeviceLocalUDIDMarkdown(result *DeviceLocalUDIDResult) error {
	h, r := deviceLocalUDIDRows(result)
	RenderMarkdown(h, r)
	return nil
}

func devicesRows(resp *DevicesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printDevicesTable(resp *DevicesResponse) error {
	h, r := devicesRows(resp)
	RenderTable(h, r)
	return nil
}

func printDevicesMarkdown(resp *DevicesResponse) error {
	h, r := devicesRows(resp)
	RenderMarkdown(h, r)
	return nil
}
