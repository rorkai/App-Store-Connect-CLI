package asc

// SigningProfileVerification separates CMS integrity from Apple signer trust.
// Statuses are verified, failed, or notChecked; an omitted record is unknown.
type SigningProfileVerification struct {
	CMSSignature string `json:"cmsSignature"`
	AppleTrust   string `json:"appleTrust"`
}
