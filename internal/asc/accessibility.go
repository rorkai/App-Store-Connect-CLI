package asc

// DeviceFamily represents a device family used in accessibility declarations.
type DeviceFamily string

const (
	DeviceFamilyIPhone     DeviceFamily = "IPHONE"
	DeviceFamilyIPad       DeviceFamily = "IPAD"
	DeviceFamilyAppleTV    DeviceFamily = "APPLE_TV"
	DeviceFamilyAppleWatch DeviceFamily = "APPLE_WATCH"
	DeviceFamilyMac        DeviceFamily = "MAC"
	DeviceFamilyVision     DeviceFamily = "VISION"
)

// DeviceConnectionType represents the network connection type.
type DeviceConnectionType string

const (
	DeviceConnectionTypeWiFi       DeviceConnectionType = "WIFI"
	DeviceConnectionTypeMobileData DeviceConnectionType = "MOBILE_DATA"
	DeviceConnectionTypeWire       DeviceConnectionType = "WIRE"
	DeviceConnectionTypeUnknown    DeviceConnectionType = "UNKNOWN"
	DeviceConnectionTypeNone       DeviceConnectionType = "NONE"
)

// DeviceFamilyOsVersionFilter represents device family OS constraints.
type DeviceFamilyOsVersionFilter struct {
	DeviceFamily       DeviceFamily `json:"deviceFamily,omitempty"`
	MinimumOsInclusive string       `json:"minimumOsInclusive,omitempty"`
	MaximumOsInclusive string       `json:"maximumOsInclusive,omitempty"`
}

// AccessibilityDeclarationState represents the publishing state of a declaration.
type AccessibilityDeclarationState string

const (
	AccessibilityDeclarationStateDraft     AccessibilityDeclarationState = "DRAFT"
	AccessibilityDeclarationStatePublished AccessibilityDeclarationState = "PUBLISHED"
	AccessibilityDeclarationStateReplaced  AccessibilityDeclarationState = "REPLACED"
)

// AccessibilityDeclarationAttributes describes accessibility declaration attributes.
type AccessibilityDeclarationAttributes struct {
	DeviceFamily                           DeviceFamily                  `json:"deviceFamily,omitempty"`
	State                                  AccessibilityDeclarationState `json:"state,omitempty"`
	SupportsAudioDescriptions              *bool                         `json:"supportsAudioDescriptions,omitempty"`
	SupportsCaptions                       *bool                         `json:"supportsCaptions,omitempty"`
	SupportsDarkInterface                  *bool                         `json:"supportsDarkInterface,omitempty"`
	SupportsDifferentiateWithoutColorAlone *bool                         `json:"supportsDifferentiateWithoutColorAlone,omitempty"`
	SupportsLargerText                     *bool                         `json:"supportsLargerText,omitempty"`
	SupportsReducedMotion                  *bool                         `json:"supportsReducedMotion,omitempty"`
	SupportsSufficientContrast             *bool                         `json:"supportsSufficientContrast,omitempty"`
	SupportsVoiceControl                   *bool                         `json:"supportsVoiceControl,omitempty"`
	SupportsVoiceover                      *bool                         `json:"supportsVoiceover,omitempty"`
}

// AccessibilityDeclarationsResponse is the response from accessibility declarations list.
type AccessibilityDeclarationsResponse = Response[AccessibilityDeclarationAttributes]

// AccessibilityDeclarationCreateAttributes describes create attributes.
type AccessibilityDeclarationCreateAttributes struct {
	DeviceFamily                           DeviceFamily `json:"deviceFamily"`
	SupportsAudioDescriptions              *bool        `json:"supportsAudioDescriptions,omitempty"`
	SupportsCaptions                       *bool        `json:"supportsCaptions,omitempty"`
	SupportsDarkInterface                  *bool        `json:"supportsDarkInterface,omitempty"`
	SupportsDifferentiateWithoutColorAlone *bool        `json:"supportsDifferentiateWithoutColorAlone,omitempty"`
	SupportsLargerText                     *bool        `json:"supportsLargerText,omitempty"`
	SupportsReducedMotion                  *bool        `json:"supportsReducedMotion,omitempty"`
	SupportsSufficientContrast             *bool        `json:"supportsSufficientContrast,omitempty"`
	SupportsVoiceControl                   *bool        `json:"supportsVoiceControl,omitempty"`
	SupportsVoiceover                      *bool        `json:"supportsVoiceover,omitempty"`
}

// AccessibilityDeclarationRelationships describes relationships for requests.
type AccessibilityDeclarationRelationships struct {
	App *Relationship `json:"app"`
}

// AccessibilityDeclarationCreateData is the data portion of a create request.
type AccessibilityDeclarationCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    AccessibilityDeclarationCreateAttributes `json:"attributes"`
	Relationships *AccessibilityDeclarationRelationships   `json:"relationships"`
}

// AccessibilityDeclarationDeleteResult represents CLI output for deletions.
type AccessibilityDeclarationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
