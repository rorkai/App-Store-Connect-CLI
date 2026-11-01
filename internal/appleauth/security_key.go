package appleauth

// SecurityKeyChallenge is Apple's FSA challenge returned by the web sign-in
// service. AllowedCredentials is a comma-separated list of credential IDs.
type SecurityKeyChallenge struct {
	Challenge          string   `json:"challenge"`
	KeyHandles         []string `json:"keyHandles"`
	AllowedCredentials string   `json:"allowedCredentials"`
}
