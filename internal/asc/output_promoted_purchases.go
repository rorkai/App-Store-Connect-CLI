package asc

import (
	"fmt"
	"strconv"
	"strings"
)

// PromotedPurchaseDeleteResult represents CLI output for promoted purchase deletions.
type PromotedPurchaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppPromotedPurchasesLinkResult represents CLI output for linking promoted purchases.
type AppPromotedPurchasesLinkResult struct {
	AppID               string   `json:"appId"`
	PromotedPurchaseIDs []string `json:"promotedPurchaseIds"`
	Action              string   `json:"action"`
}

func promotedPurchaseBool(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}

func promotedPurchasesRows(resp *PromotedPurchasesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Visible For All Users", "Enabled", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			promotedPurchaseBool(item.Attributes.VisibleForAllUsers),
			promotedPurchaseBool(item.Attributes.Enabled),
			item.Attributes.State,
		})
	}
	return headers, rows
}

func promotedPurchaseDeleteResultRows(result *PromotedPurchaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func appPromotedPurchasesLinkResultRows(result *AppPromotedPurchasesLinkResult) ([]string, [][]string) {
	headers := []string{"App ID", "Promoted Purchase IDs", "Action"}
	rows := [][]string{{
		result.AppID,
		strings.Join(result.PromotedPurchaseIDs, ", "),
		result.Action,
	}}
	return headers, rows
}
