package asc

// ArtifactIPAInfo is the offline IPA inspection receipt.
type ArtifactIPAInfo struct {
	Path             string                  `json:"path"`
	BundleID         string                  `json:"bundleId,omitempty"`
	Name             string                  `json:"name,omitempty"`
	Version          string                  `json:"version,omitempty"`
	BuildNumber      string                  `json:"buildNumber,omitempty"`
	MinimumOSVersion string                  `json:"minimumOSVersion,omitempty"`
	Platforms        []string                `json:"platforms,omitempty"`
	TeamID           string                  `json:"teamId,omitempty"`
	SignerCommonName string                  `json:"signerCommonName,omitempty"`
	Status           string                  `json:"status"`
	NestedBundles    []ArtifactNestedBundle  `json:"nestedBundles"`
	Entitlements     map[string]any          `json:"entitlements,omitempty"`
	Profile          *ArtifactProfileSummary `json:"profile,omitempty"`
}

// ArtifactNestedBundle is an extension or App Clip found in an IPA.
type ArtifactNestedBundle struct {
	BundleID string `json:"bundleId,omitempty"`
	Name     string `json:"name,omitempty"`
	Path     string `json:"path"`
}

// ArtifactProfileSummary is the embedded provisioning profile summary.
type ArtifactProfileSummary struct {
	Name           string `json:"name,omitempty"`
	UUID           string `json:"uuid,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	ProfileType    string `json:"profileType,omitempty"`
}

// ArtifactPKGInfo is the offline flat package inspection receipt.
type ArtifactPKGInfo struct {
	Path             string   `json:"path"`
	ProductID        string   `json:"productId,omitempty"`
	Version          string   `json:"version,omitempty"`
	InstallLocation  string   `json:"installLocation,omitempty"`
	BundleIDs        []string `json:"bundleIds,omitempty"`
	SignerCommonName string   `json:"signerCommonName,omitempty"`
	Status           string   `json:"status"`
}

func artifactIPAInfoRows(result *ArtifactIPAInfo) ([]string, [][]string) {
	headers := []string{"Bundle ID", "Version", "Build", "Status"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{result.BundleID, result.Version, result.BuildNumber, result.Status}}
}

func artifactPKGInfoRows(result *ArtifactPKGInfo) ([]string, [][]string) {
	headers := []string{"Product ID", "Version", "Install Location", "Status"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{result.ProductID, result.Version, result.InstallLocation, result.Status}}
}
