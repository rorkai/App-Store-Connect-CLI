package asc

// PreReleaseVersionAppLinkageResponse is the response for pre-release version app relationships.
type PreReleaseVersionAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}
