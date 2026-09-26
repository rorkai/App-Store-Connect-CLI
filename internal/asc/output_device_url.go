package asc

import "fmt"

// DeviceURLRegistration describes a device callback outcome.
type DeviceURLRegistration struct {
	Name     string `json:"name"`
	UDID     string `json:"udid"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	DeviceID string `json:"deviceId,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DeviceURLRegistrationResult is the final registration session receipt.
type DeviceURLRegistrationResult struct {
	URL         string                  `json:"url"`
	Token       string                  `json:"token"`
	CollectOnly bool                    `json:"collectOnly"`
	OutputFile  string                  `json:"outputFile,omitempty"`
	Devices     []DeviceURLRegistration `json:"devices"`
	Failures    []DeviceURLRegistration `json:"failures,omitempty"`
}

func deviceURLRegistrationSummaryRows(result *DeviceURLRegistrationResult) ([]string, [][]string) {
	return []string{"URL", "Collect Only", "Output File", "Devices", "Failed"}, [][]string{{compactWhitespace(result.URL), fmt.Sprint(result.CollectOnly), compactWhitespace(result.OutputFile), fmt.Sprint(len(result.Devices)), fmt.Sprint(len(result.Failures))}}
}

func deviceURLRegistrationRows(records []DeviceURLRegistration) ([]string, [][]string) {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{compactWhitespace(record.Name), compactWhitespace(record.UDID), record.Platform, record.Status, record.DeviceID, compactWhitespace(record.Error)})
	}
	return []string{"Name", "UDID", "Platform", "Status", "Device ID", "Error"}, rows
}
