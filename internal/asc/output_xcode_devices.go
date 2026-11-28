package asc

import "fmt"

// XcodeDevicesResult lists physical iOS devices known to CoreDevice.
type XcodeDevicesResult struct {
	Devices []XcodeDevice `json:"devices"`
}

// XcodeDevice exposes the CoreDevice selector accepted by xcode install.
// Hardware UDIDs, serial numbers and unrelated native properties are omitted.
type XcodeDevice struct {
	DeviceID        string `json:"deviceId"`
	Name            string `json:"name,omitempty"`
	Platform        string `json:"platform"`
	PairingState    string `json:"pairingState"`
	ConnectionState string `json:"connectionState"`
	Available       bool   `json:"available"`
}

func xcodeDevicesResultRows(result *XcodeDevicesResult) ([]string, [][]string) {
	rows := make([][]string, 0, len(result.Devices))
	for _, device := range result.Devices {
		rows = append(rows, []string{device.DeviceID, device.Name, device.Platform, device.PairingState, device.ConnectionState, fmt.Sprintf("%t", device.Available)})
	}
	return []string{"Device ID", "Name", "Platform", "Pairing", "Connection", "Available"}, rows
}
