package asc

import "fmt"

// SigningKeychainInstallResult is the public, non-secret receipt for a
// persistent signing keychain installation.
type SigningKeychainInstallResult struct {
	Action            string `json:"action"`
	KeychainPath      string `json:"keychainPath"`
	CertificateSHA256 string `json:"certificateSha256"`
	CertificateSHA1   string `json:"certificateSha1"`
	TeamID            string `json:"teamId"`
	SearchListUpdated bool   `json:"searchListUpdated"`
}

// SigningKeychainActionResult is the non-secret receipt for a keychain mutation.
type SigningKeychainActionResult struct {
	Action       string `json:"action"`
	KeychainPath string `json:"keychainPath"`
	Partition    string `json:"partition,omitempty"`
}

// SigningKeychainListResult lists user search-list keychains.
type SigningKeychainListResult struct {
	Keychains []SigningKeychainInfo `json:"keychains"`
}

// SigningKeychainInfo is one keychain in the user search list.
type SigningKeychainInfo struct {
	Path         string                    `json:"path"`
	InSearchList bool                      `json:"inSearchList"`
	Locked       bool                      `json:"locked"`
	Identities   []SigningKeychainIdentity `json:"identities,omitempty"`
}

// SigningKeychainIdentity is a public certificate summary. It never includes a key.
type SigningKeychainIdentity struct {
	SHA256     string `json:"sha256"`
	CommonName string `json:"commonName,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

func signingKeychainInstallRows(result *SigningKeychainInstallResult) ([]string, [][]string) {
	headers := []string{"Action", "Keychain Path", "Certificate SHA-256", "Certificate SHA-1", "Team ID", "Search List Updated"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{
		result.Action,
		result.KeychainPath,
		result.CertificateSHA256,
		result.CertificateSHA1,
		result.TeamID,
		formatBool(result.SearchListUpdated),
	}}
}

func signingKeychainActionRows(result *SigningKeychainActionResult) ([]string, [][]string) {
	headers := []string{"Action", "Keychain Path"}
	if result == nil {
		return headers, nil
	}
	return headers, [][]string{{result.Action, result.KeychainPath}}
}

func signingKeychainListRows(result *SigningKeychainListResult) ([]string, [][]string) {
	headers := []string{"Path", "Search List", "Locked", "Identities"}
	if result == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(result.Keychains))
	for _, keychain := range result.Keychains {
		rows = append(rows, []string{keychain.Path, formatBool(keychain.InSearchList), formatBool(keychain.Locked), fmt.Sprintf("%d", len(keychain.Identities))})
	}
	return headers, rows
}
