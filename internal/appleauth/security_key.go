package appleauth

import "encoding/json"

// SecurityKeyChallenge is Apple's FSA challenge returned by the web sign-in
// service. AllowedCredentials is a comma-separated list of credential IDs.
type SecurityKeyChallenge struct {
	Challenge          string `json:"challenge"`
	AllowedCredentials string `json:"allowedCredentials"`
}

// parseSecurityKeyChallenge returns nil unless raw is a usable challenge, so an
// unexpected value from Apple leaves ordinary code sign-in working.
func parseSecurityKeyChallenge(raw json.RawMessage) *SecurityKeyChallenge {
	var challenge SecurityKeyChallenge
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return nil
	}
	if challenge.Challenge == "" || challenge.AllowedCredentials == "" {
		return nil
	}
	return &challenge
}
