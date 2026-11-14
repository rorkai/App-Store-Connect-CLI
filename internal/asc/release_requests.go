package asc

// ResourceTypeAppStoreVersionReleaseRequests is the resource type for release requests.
const ResourceTypeAppStoreVersionReleaseRequests ResourceType = "appStoreVersionReleaseRequests"

// AppStoreVersionReleaseRequest represents a release request resource.
type AppStoreVersionReleaseRequest struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// AppStoreVersionReleaseRequestResponse represents a release request API response.
type AppStoreVersionReleaseRequestResponse struct {
	Data  AppStoreVersionReleaseRequest `json:"data"`
	Links Links                         `json:"links"`
}
