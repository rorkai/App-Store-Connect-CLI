package asc

import "fmt"

// WebSignInKeyReceipt contains only non-secret key metadata and a local P8 path.
type WebSignInKeyReceipt struct {
	KeyID    string `json:"keyId"`
	Name     string `json:"name,omitempty"`
	BundleID string `json:"bundleId,omitempty"`
	P8Path   string `json:"p8Path"`
}

func webSignInKeyReceiptRows(r *WebSignInKeyReceipt) ([]string, [][]string) {
	headers := []string{"Key ID", "Name", "Bundle ID", "P8 Path"}
	if r == nil {
		return headers, nil
	}
	return headers, [][]string{{r.KeyID, r.Name, r.BundleID, r.P8Path}}
}

// WebDeveloperKeyRevokeResult confirms a Developer Portal key is no longer listed.
type WebDeveloperKeyRevokeResult struct {
	KeyID   string `json:"keyId"`
	Revoked bool   `json:"revoked"`
}

func webDeveloperKeyRevokeRows(r *WebDeveloperKeyRevokeResult) ([]string, [][]string) {
	headers := []string{"Key ID", "Revoked"}
	if r == nil {
		return headers, nil
	}
	return headers, [][]string{{r.KeyID, fmt.Sprintf("%t", r.Revoked)}}
}
