package asc

// DevicePlatform represents the platform of a device.
type DevicePlatform = BundleIDPlatform

const (
	DevicePlatformIOS       = BundleIDPlatformIOS
	DevicePlatformMacOS     = BundleIDPlatformMacOS
	DevicePlatformUniversal = BundleIDPlatformUniversal
)

// DeviceStatus represents the status of a device.
type DeviceStatus string

const (
	DeviceStatusEnabled  DeviceStatus = "ENABLED"
	DeviceStatusDisabled DeviceStatus = "DISABLED"
)

// DeviceClass represents the device class reported by ASC.
type DeviceClass string

const (
	DeviceClassAppleWatch DeviceClass = "APPLE_WATCH"
	DeviceClassIPad       DeviceClass = "IPAD"
	DeviceClassIPhone     DeviceClass = "IPHONE"
	DeviceClassIPod       DeviceClass = "IPOD"
	DeviceClassAppleTV    DeviceClass = "APPLE_TV"
	DeviceClassMac        DeviceClass = "MAC"
)

// DeviceAttributes describes an App Store Connect device.
type DeviceAttributes struct {
	Name        string         `json:"name"`
	Platform    DevicePlatform `json:"platform"`
	UDID        string         `json:"udid"`
	DeviceClass DeviceClass    `json:"deviceClass,omitempty"`
	Status      DeviceStatus   `json:"status,omitempty"`
	Model       string         `json:"model,omitempty"`
	AddedDate   string         `json:"addedDate,omitempty"`
}

// DeviceCreateAttributes describes attributes for creating a device.
type DeviceCreateAttributes struct {
	Name     string         `json:"name"`
	UDID     string         `json:"udid"`
	Platform DevicePlatform `json:"platform"`
}

// DeviceUpdateAttributes describes attributes for updating a device.
type DeviceUpdateAttributes struct {
	Name   *string       `json:"name,omitempty"`
	Status *DeviceStatus `json:"status,omitempty"`
}
