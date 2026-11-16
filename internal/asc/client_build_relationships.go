package asc

// BuildAppLinkageResponse is the response for build app relationships.
type BuildAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildAppStoreVersionLinkageResponse is the response for build app store version relationships.
type BuildAppStoreVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildBuildBetaDetailLinkageResponse is the response for build beta detail relationships.
type BuildBuildBetaDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildPreReleaseVersionLinkageResponse is the response for build pre-release version relationships.
type BuildPreReleaseVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}
