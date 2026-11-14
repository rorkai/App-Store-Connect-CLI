package asc

// BuildBetaDetailAttributes describes build beta detail attributes.
type BuildBetaDetailAttributes struct {
	AutoNotifyEnabled  bool   `json:"autoNotifyEnabled,omitempty"`
	InternalBuildState string `json:"internalBuildState,omitempty"`
	ExternalBuildState string `json:"externalBuildState,omitempty"`

	api apiAttributes
}
