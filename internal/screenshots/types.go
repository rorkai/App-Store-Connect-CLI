package screenshots

// CaptureRequest holds parameters for a single screenshot capture.
type CaptureRequest struct {
	Provider  string // ProviderAXe or ProviderMacOS
	BundleID  string
	UDID      string // simulator UDID or "booted"
	Name      string // output file name (without extension)
	OutputDir string // directory to write PNG
}

// CaptureResult is the structured result of a successful capture.
type CaptureResult struct {
	Path     string `json:"path"`
	Provider string `json:"provider"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	BundleID string `json:"bundle_id"`
	UDID     string `json:"udid"`
}
