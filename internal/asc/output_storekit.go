package asc

import (
	"strconv"
)

// StoreKitAccountTokenResult acknowledges a successful account association.
type StoreKitAccountTokenResult struct {
	OriginalTransactionID string `json:"originalTransactionId"`
	AppAccountToken       string `json:"appAccountToken"`
	Updated               bool   `json:"updated"`
}

func storeKitAccountTokenRows(result *StoreKitAccountTokenResult) ([]string, [][]string) {
	return []string{"Original Transaction ID", "App Account Token", "Updated"}, [][]string{{result.OriginalTransactionID, result.AppAccountToken, strconv.FormatBool(result.Updated)}}
}
