package asc

import "fmt"

// XcodeSimulatorInstallResult reports installation separately from verification.
// Local paths and raw simulator identifiers are omitted.
type XcodeSimulatorInstallResult struct {
	BundleID                  string `json:"bundleId"`
	Version                   string `json:"version"`
	BuildNumber               string `json:"buildNumber"`
	SimulatorIdentifierSHA256 string `json:"simulatorIdentifierSha256"`
	Installed                 bool   `json:"installed"`
	Verified                  bool   `json:"verified"`
}

func xcodeSimulatorInstallResultRows(result *XcodeSimulatorInstallResult) ([]string, [][]string) {
	return []string{"Field", "Value"}, [][]string{
		{"bundle_id", result.BundleID},
		{"version", result.Version},
		{"build_number", result.BuildNumber},
		{"simulator_identifier_sha256", result.SimulatorIdentifierSHA256},
		{"installed", fmt.Sprintf("%t", result.Installed)},
		{"verified", fmt.Sprintf("%t", result.Verified)},
	}
}
