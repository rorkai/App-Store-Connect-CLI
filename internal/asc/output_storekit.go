package asc

import "strconv"

// StoreKitAccountTokenResult acknowledges a successful account association.
type StoreKitAccountTokenResult struct {
	OriginalTransactionID string `json:"originalTransactionId"`
	AppAccountToken       string `json:"appAccountToken"`
	Updated               bool   `json:"updated"`
}

// StoreKitConsumptionResult acknowledges accepted consumption data, not a
// refund decision.
type StoreKitConsumptionResult struct {
	TransactionID string `json:"transactionId"`
	Submitted     bool   `json:"submitted"`
}

// StoreKitFinishResult acknowledges that Apple accepted transaction finishing.
type StoreKitFinishResult struct {
	TransactionID string `json:"transactionId"`
	Finished      bool   `json:"finished"`
}

func storeKitAccountTokenRows(result *StoreKitAccountTokenResult) ([]string, [][]string) {
	return []string{"Original Transaction ID", "App Account Token", "Updated"}, [][]string{{result.OriginalTransactionID, result.AppAccountToken, strconv.FormatBool(result.Updated)}}
}

func storeKitConsumptionRows(result *StoreKitConsumptionResult) ([]string, [][]string) {
	return []string{"Transaction ID", "Submitted"}, [][]string{{result.TransactionID, strconv.FormatBool(result.Submitted)}}
}

func storeKitFinishRows(result *StoreKitFinishResult) ([]string, [][]string) {
	return []string{"Transaction ID", "Finished"}, [][]string{{result.TransactionID, strconv.FormatBool(result.Finished)}}
}
