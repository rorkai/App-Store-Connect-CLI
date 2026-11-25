package asc

import "strconv"

// WebDSAStatus summarizes the selected account's Digital Services Act status.
type WebDSAStatus struct {
	TraderDeclaration string               `json:"traderDeclaration"`
	SellerInfo        []WebDSASellerStatus `json:"sellerInfo"`
}

// WebDSASellerStatus contains the verification state without seller identity.
type WebDSASellerStatus struct {
	VerificationStatus string `json:"verificationStatus"`
	IsEuCovered        *bool  `json:"isEuCovered"`
}

func webDSAStatusRows(result *WebDSAStatus) ([]string, [][]string) {
	headers := []string{"Trader Declaration", "Verification Status", "EU Covered"}
	if result == nil {
		return headers, nil
	}
	if len(result.SellerInfo) == 0 {
		return headers, [][]string{{webDeclarationText(result.TraderDeclaration), "n/a", "n/a"}}
	}
	rows := make([][]string, 0, len(result.SellerInfo))
	for _, seller := range result.SellerInfo {
		covered := "n/a"
		if seller.IsEuCovered != nil {
			covered = strconv.FormatBool(*seller.IsEuCovered)
		}
		rows = append(rows, []string{webDeclarationText(result.TraderDeclaration), webDeclarationText(seller.VerificationStatus), covered})
	}
	return headers, rows
}
