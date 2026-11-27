package asc

import "fmt"

// XcodeInstallResult is the privacy-safe result of installing and verifying a
// local IPA on one connected iOS device. Device identity is represented only
// by a one-way digest; source and temporary paths are intentionally omitted.
type XcodeInstallResult struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Operation     string               `json:"operation"`
	Success       bool                 `json:"success"`
	Installed     bool                 `json:"installed"`
	Verified      bool                 `json:"verified"`
	Launch        *XcodeInstallLaunch  `json:"launch,omitempty"`
	IPA           XcodeInstallArtifact `json:"ipa"`
	Device        *XcodeInstallDevice  `json:"device,omitempty"`
	FailureStage  string               `json:"failureStage,omitempty"`
	FailureCode   string               `json:"failureCode,omitempty"`
	DurationMS    int64                `json:"durationMs"`
}

// XcodeInstallLaunch records CoreDevice launch acknowledgement only.
// It does not claim that the application is healthy or remains running.
type XcodeInstallLaunch struct {
	Started bool `json:"started"`
}

// XcodeInstallArtifact contains only stable, non-path IPA identity fields.
type XcodeInstallArtifact struct {
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"sizeBytes"`
	BundleID    string `json:"bundleId"`
	Version     string `json:"version"`
	BuildNumber string `json:"buildNumber"`
}

// XcodeInstallDevice contains redacted connected-device state. The identifier
// is a one-way digest rather than the CoreDevice identifier or hardware UDID.
type XcodeInstallDevice struct {
	IdentifierSHA256 string `json:"identifierSha256"`
	Platform         string `json:"platform"`
	PairingState     string `json:"pairingState"`
	ConnectionState  string `json:"connectionState"`
}

func xcodeInstallResultRows(result *XcodeInstallResult) ([]string, [][]string) {
	if result == nil {
		result = &XcodeInstallResult{}
	}
	rows := [][]string{
		{"schema_version", fmt.Sprintf("%d", result.SchemaVersion)},
		{"operation", result.Operation},
		{"success", fmt.Sprintf("%t", result.Success)},
		{"installed", fmt.Sprintf("%t", result.Installed)},
		{"verified", fmt.Sprintf("%t", result.Verified)},
		{"ipa_bundle_id", result.IPA.BundleID},
		{"ipa_version", result.IPA.Version},
		{"ipa_build_number", result.IPA.BuildNumber},
		{"ipa_size_bytes", fmt.Sprintf("%d", result.IPA.SizeBytes)},
		{"ipa_sha256", result.IPA.SHA256},
	}
	if result.Device != nil {
		rows = append(
			rows,
			[]string{"device_identifier_sha256", result.Device.IdentifierSHA256},
			[]string{"device_platform", result.Device.Platform},
			[]string{"pairing_state", result.Device.PairingState},
			[]string{"connection_state", result.Device.ConnectionState},
		)
	}
	if result.Launch != nil {
		rows = append(rows, []string{"launch_started", fmt.Sprintf("%t", result.Launch.Started)})
	}
	if result.FailureStage != "" {
		rows = append(rows, []string{"failure_stage", result.FailureStage})
	}
	if result.FailureCode != "" {
		rows = append(rows, []string{"failure_code", result.FailureCode})
	}
	rows = append(rows, []string{"duration_ms", fmt.Sprintf("%d", result.DurationMS)})
	return []string{"Field", "Value"}, rows
}
